package web

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"kong-cert-lite/internal/domain"
	"kong-cert-lite/internal/usecase"
)

//go:embed templates/*.html
var templateFiles embed.FS

type Handler struct {
	logger       *slog.Logger
	certificates *usecase.CertificateUseCase
	acme         *usecase.ACMEUseCase
	kongSync     *usecase.KongSyncUseCase
	kongTargets  *usecase.KongTargetUseCase
	jobs         *usecase.JobUseCase
}

type PageData struct {
	Title         string
	Active        string
	Heading       string
	Description   string
	PrimaryAction string
	StatusLabel   string
	Metrics       []Metric
	Columns       []string
	EmptyTitle    string
	EmptyText     string
}

type CertificateListPage struct {
	PageData
	Certificates []usecase.CertificateView
}

type CertificateFormPage struct {
	PageData
	Form              usecase.CertificateFormData
	Errors            map[string]string
	IsEdit            bool
	Action            string
	DomainsAndSNILock bool
}

type CertificateDetailPage struct {
	PageData
	Certificate usecase.CertificateView
}

type KongTargetListPage struct {
	PageData
	Targets []usecase.KongTargetView
}

type KongTargetFormPage struct {
	PageData
	Form   usecase.KongTargetFormData
	Errors map[string]string
	IsEdit bool
	Action string
}

type JobListPage struct {
	PageData
	Jobs []usecase.JobView
}

type JobDetailPage struct {
	PageData
	Job usecase.JobView
}

type Metric struct {
	Label string
	Value string
	Tone  string
}

func NewHandler(logger *slog.Logger, certificates *usecase.CertificateUseCase, acme *usecase.ACMEUseCase, kongSync *usecase.KongSyncUseCase, kongTargets *usecase.KongTargetUseCase, jobs *usecase.JobUseCase) *Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{
		logger:       logger,
		certificates: certificates,
		acme:         acme,
		kongSync:     kongSync,
		kongTargets:  kongTargets,
		jobs:         jobs,
	}
}

func (h *Handler) Healthz(c *fiber.Ctx) error {
	c.Type("json")
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
}

func (h *Handler) Home(c *fiber.Ctx) error {
	return c.Redirect("/certificates", fiber.StatusFound)
}

func (h *Handler) Certificates(c *fiber.Ctx) error {
	certificates, err := h.certificates.List(c.UserContext())
	if err != nil {
		return h.serverError(c, "list certificates", err)
	}

	active, warning, failed := certificateMetrics(certificates)
	return h.render(c, fiber.StatusOK, "templates/certificates.html", CertificateListPage{
		PageData: PageData{
			Title:         "Certificates",
			Active:        "certificates",
			Heading:       "Certificates",
			Description:   "Track expiry, renewal windows, and Kong sync state from one workspace.",
			PrimaryAction: "Add certificate",
			StatusLabel:   "Ready",
			Metrics: []Metric{
				{Label: "Active", Value: strconv.Itoa(active), Tone: "success"},
				{Label: "Expiring soon", Value: strconv.Itoa(warning), Tone: "warning"},
				{Label: "Needs attention", Value: strconv.Itoa(failed), Tone: "danger"},
			},
			Columns:    []string{"Domain", "Type", "Expires", "Status", "Kong targets", "Actions"},
			EmptyTitle: "No certificates",
			EmptyText:  "Create the first certificate record to begin tracking expiry and Kong sync state.",
		},
		Certificates: certificates,
	})
}

func (h *Handler) NewCertificate(c *fiber.Ctx) error {
	return h.renderCertificateForm(c, fiber.StatusOK, usecase.CertificateFormData{
		AutoRenew:       true,
		RenewBeforeDays: 30,
	}, nil, false, false)
}

func (h *Handler) CreateCertificate(c *fiber.Ctx) error {
	renewBeforeDays, _ := strconv.Atoi(strings.TrimSpace(c.FormValue("renew_before_days")))
	form := usecase.CertificateFormData{
		Name:            c.FormValue("name"),
		Email:           c.FormValue("email"),
		DomainsText:     c.FormValue("domains"),
		SNIsText:        c.FormValue("snis"),
		AutoRenew:       c.FormValue("auto_renew") == "on",
		RenewBeforeDays: renewBeforeDays,
	}

	id, err := h.certificates.Create(c.UserContext(), certificateInputFromForm(form))
	if err != nil {
		var validationErr usecase.ValidationError
		if errors.As(err, &validationErr) {
			return h.renderCertificateForm(c, fiber.StatusUnprocessableEntity, form, validationErr.Fields, false, false)
		}
		return h.serverError(c, "create certificate", err)
	}

	return c.Redirect("/certificates/"+strconv.FormatInt(id, 10), fiber.StatusSeeOther)
}

func (h *Handler) EditCertificate(c *fiber.Ctx) error {
	id, err := usecase.ParseID(c.Params("id"))
	if err != nil {
		return fiber.ErrNotFound
	}

	certificate, err := h.certificates.Get(c.UserContext(), id)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			return fiber.ErrNotFound
		}
		return h.serverError(c, "get certificate", err)
	}

	form := usecase.CertificateFormData{
		ID:              certificate.Certificate.ID,
		Name:            certificate.Certificate.Name,
		Email:           certificate.Certificate.Email,
		DomainsText:     strings.Join(certificate.Certificate.Domains, "\n"),
		SNIsText:        strings.Join(certificate.Certificate.SNIs, "\n"),
		AutoRenew:       certificate.Certificate.AutoRenew,
		RenewBeforeDays: certificate.Certificate.RenewBeforeDays,
	}

	return h.renderCertificateForm(c, fiber.StatusOK, form, nil, true, certificateLockedForEdit(certificate.Certificate))
}

func (h *Handler) UpdateCertificate(c *fiber.Ctx) error {
	id, err := usecase.ParseID(c.Params("id"))
	if err != nil {
		return fiber.ErrNotFound
	}

	form := certificateFormFromRequest(c, id)
	certificate, err := h.certificates.Get(c.UserContext(), id)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			return fiber.ErrNotFound
		}
		return h.serverError(c, "get certificate", err)
	}
	locked := certificateLockedForEdit(certificate.Certificate)

	if err := h.certificates.Update(c.UserContext(), certificateInputFromForm(form), id); err != nil {
		var validationErr usecase.ValidationError
		if errors.As(err, &validationErr) {
			return h.renderCertificateForm(c, fiber.StatusUnprocessableEntity, form, validationErr.Fields, true, locked)
		}
		if errors.Is(err, usecase.ErrNotFound) {
			return fiber.ErrNotFound
		}
		return h.serverError(c, "update certificate", err)
	}

	return c.Redirect("/certificates/"+strconv.FormatInt(id, 10), fiber.StatusSeeOther)
}

func (h *Handler) CertificateDetail(c *fiber.Ctx) error {
	id, err := usecase.ParseID(c.Params("id"))
	if err != nil {
		return fiber.ErrNotFound
	}

	certificate, err := h.certificates.Get(c.UserContext(), id)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			return fiber.ErrNotFound
		}
		return h.serverError(c, "get certificate", err)
	}

	return h.render(c, fiber.StatusOK, "templates/certificate_detail.html", CertificateDetailPage{
		PageData: PageData{
			Title:         certificate.Certificate.Name,
			Active:        "certificates",
			Heading:       certificate.Certificate.Name,
			Description:   "Certificate metadata, renewal settings, domains, and SNI values.",
			PrimaryAction: "Renew now",
			StatusLabel:   certificate.StatusLabel,
		},
		Certificate: certificate,
	})
}

func (h *Handler) DeleteCertificate(c *fiber.Ctx) error {
	id, err := usecase.ParseID(c.Params("id"))
	if err != nil {
		return fiber.ErrNotFound
	}

	if err := h.certificates.Delete(c.UserContext(), id); err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			return fiber.ErrNotFound
		}
		return h.serverError(c, "delete certificate", err)
	}

	return c.Redirect("/certificates", fiber.StatusSeeOther)
}

func (h *Handler) SyncCertificate(c *fiber.Ctx) error {
	id, err := usecase.ParseID(c.Params("id"))
	if err != nil {
		return fiber.ErrNotFound
	}

	if err := h.kongSync.SyncCertificate(c.UserContext(), id); err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			return fiber.ErrNotFound
		}
		return h.serverError(c, "sync certificate", err)
	}

	return c.Redirect("/certificates/"+strconv.FormatInt(id, 10), fiber.StatusSeeOther)
}

func (h *Handler) IssueCertificate(c *fiber.Ctx) error {
	id, err := usecase.ParseID(c.Params("id"))
	if err != nil {
		return fiber.ErrNotFound
	}

	if err := h.acme.IssueCertificate(c.UserContext(), id); err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			return fiber.ErrNotFound
		}
		return h.serverError(c, "issue certificate", err)
	}

	return c.Redirect("/certificates/"+strconv.FormatInt(id, 10), fiber.StatusSeeOther)
}

func (h *Handler) UpdateCertificateTargets(c *fiber.Ctx) error {
	id, err := usecase.ParseID(c.Params("id"))
	if err != nil {
		return fiber.ErrNotFound
	}

	formValues, err := url.ParseQuery(string(c.Body()))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid form data")
	}
	targetIDs, err := parseSelectedTargetIDs(formValues["kong_target_ids"])
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid Kong target selection")
	}

	if err := h.certificates.UpdateLinkedTargets(c.UserContext(), id, targetIDs); err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			return fiber.ErrNotFound
		}
		var validationErr usecase.ValidationError
		if errors.As(err, &validationErr) {
			return fiber.NewError(fiber.StatusBadRequest, validationErr.Error())
		}
		return h.serverError(c, "update certificate targets", err)
	}

	return c.Redirect("/certificates/"+strconv.FormatInt(id, 10), fiber.StatusSeeOther)
}

func (h *Handler) KongTargets(c *fiber.Ctx) error {
	targets, err := h.kongTargets.List(c.UserContext())
	if err != nil {
		return h.serverError(c, "list kong targets", err)
	}

	online, offline, unknown := kongTargetMetrics(targets)
	return h.render(c, fiber.StatusOK, "templates/kong_targets.html", KongTargetListPage{
		PageData: PageData{
			Title:         "Kong Targets",
			Active:        "kong-targets",
			Heading:       "Kong Targets",
			Description:   "Manage the Kong Admin API endpoints that receive synced certificates.",
			PrimaryAction: "Add target",
			StatusLabel:   "Targets",
			Metrics: []Metric{
				{Label: "Online", Value: strconv.Itoa(online), Tone: "success"},
				{Label: "Offline", Value: strconv.Itoa(offline), Tone: "danger"},
				{Label: "Unknown", Value: strconv.Itoa(unknown), Tone: "secondary"},
			},
			Columns:    []string{"Name", "Environment", "Admin URL", "Auth", "Status", "Actions"},
			EmptyTitle: "No Kong targets",
			EmptyText:  "Add a target before syncing certificates to Kong Gateway.",
		},
		Targets: targets,
	})
}

func (h *Handler) NewKongTarget(c *fiber.Ctx) error {
	return h.renderKongTargetForm(c, fiber.StatusOK, usecase.KongTargetFormData{
		AuthType: "none",
	}, nil, false)
}

func (h *Handler) CreateKongTarget(c *fiber.Ctx) error {
	form := kongTargetFormFromRequest(c, 0)
	_, err := h.kongTargets.Create(c.UserContext(), kongTargetInputFromForm(form))
	if err != nil {
		var validationErr usecase.ValidationError
		if errors.As(err, &validationErr) {
			return h.renderKongTargetForm(c, fiber.StatusUnprocessableEntity, form, validationErr.Fields, false)
		}
		return h.serverError(c, "create kong target", err)
	}

	return c.Redirect("/kong-targets", fiber.StatusSeeOther)
}

func (h *Handler) EditKongTarget(c *fiber.Ctx) error {
	id, err := usecase.ParseKongTargetID(c.Params("id"))
	if err != nil {
		return fiber.ErrNotFound
	}

	target, err := h.kongTargets.Get(c.UserContext(), id)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			return fiber.ErrNotFound
		}
		return h.serverError(c, "get kong target", err)
	}

	form := usecase.KongTargetFormData{
		ID:             target.Target.ID,
		Name:           target.Target.Name,
		Environment:    target.Target.Environment,
		AdminURL:       target.Target.AdminURL,
		AuthType:       string(target.Target.AuthType),
		AuthHeaderName: target.Target.AuthHeaderName,
		HasSecret:      target.Target.AuthHeaderValue != "",
	}
	return h.renderKongTargetForm(c, fiber.StatusOK, form, nil, true)
}

func (h *Handler) UpdateKongTarget(c *fiber.Ctx) error {
	id, err := usecase.ParseKongTargetID(c.Params("id"))
	if err != nil {
		return fiber.ErrNotFound
	}

	form := kongTargetFormFromRequest(c, id)
	if err := h.kongTargets.Update(c.UserContext(), kongTargetInputFromForm(form)); err != nil {
		var validationErr usecase.ValidationError
		if errors.As(err, &validationErr) {
			return h.renderKongTargetForm(c, fiber.StatusUnprocessableEntity, form, validationErr.Fields, true)
		}
		if errors.Is(err, usecase.ErrNotFound) {
			return fiber.ErrNotFound
		}
		return h.serverError(c, "update kong target", err)
	}

	return c.Redirect("/kong-targets", fiber.StatusSeeOther)
}

func (h *Handler) DeleteKongTarget(c *fiber.Ctx) error {
	id, err := usecase.ParseKongTargetID(c.Params("id"))
	if err != nil {
		return fiber.ErrNotFound
	}

	if err := h.kongTargets.Delete(c.UserContext(), id); err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			return fiber.ErrNotFound
		}
		return h.serverError(c, "delete kong target", err)
	}

	return c.Redirect("/kong-targets", fiber.StatusSeeOther)
}

func (h *Handler) TestKongTarget(c *fiber.Ctx) error {
	id, err := usecase.ParseKongTargetID(c.Params("id"))
	if err != nil {
		return fiber.ErrNotFound
	}

	if err := h.kongTargets.TestConnection(c.UserContext(), id); err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			return fiber.ErrNotFound
		}
		return h.serverError(c, "test kong target", err)
	}

	return c.Redirect("/kong-targets", fiber.StatusSeeOther)
}

func (h *Handler) Jobs(c *fiber.Ctx) error {
	jobs, err := h.jobs.List(c.UserContext())
	if err != nil {
		return h.serverError(c, "list jobs", err)
	}

	running, succeeded, failed := jobMetrics(jobs)
	return h.render(c, fiber.StatusOK, "templates/jobs.html", JobListPage{
		PageData: PageData{
			Title:         "Jobs / Logs",
			Active:        "jobs",
			Heading:       "Jobs / Logs",
			Description:   "Review issue, renew, sync, and Kong connectivity runs.",
			PrimaryAction: "Refresh",
			StatusLabel:   "History",
			Metrics: []Metric{
				{Label: "Running", Value: strconv.Itoa(running), Tone: "primary"},
				{Label: "Succeeded", Value: strconv.Itoa(succeeded), Tone: "success"},
				{Label: "Failed", Value: strconv.Itoa(failed), Tone: "danger"},
			},
			Columns:    []string{"Time", "Type", "Certificate", "Target", "Status", "Message", "Actions"},
			EmptyTitle: "No jobs",
			EmptyText:  "Job history appears after certificate, sync, or Kong target actions run.",
		},
		Jobs: jobs,
	})
}

func (h *Handler) JobDetail(c *fiber.Ctx) error {
	id, err := usecase.ParseJobID(c.Params("id"))
	if err != nil {
		return fiber.ErrNotFound
	}

	job, err := h.jobs.Get(c.UserContext(), id)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			return fiber.ErrNotFound
		}
		return h.serverError(c, "get job", err)
	}

	return h.render(c, fiber.StatusOK, "templates/job_detail.html", JobDetailPage{
		PageData: PageData{
			Title:         "Job #" + strconv.FormatInt(job.Job.ID, 10),
			Active:        "jobs",
			Heading:       "Job #" + strconv.FormatInt(job.Job.ID, 10),
			Description:   "Execution timing, status, message, and detailed log output.",
			PrimaryAction: "Back to jobs",
			StatusLabel:   job.StatusLabel,
		},
		Job: job,
	})
}

func (h *Handler) Settings(c *fiber.Ctx) error {
	return h.render(c, fiber.StatusOK, "templates/placeholder.html", PageData{
		Title:         "Settings",
		Active:        "settings",
		Heading:       "Settings",
		Description:   "Review runtime mode, storage paths, and renewal defaults.",
		PrimaryAction: "Reload",
		StatusLabel:   "Local",
		Metrics: []Metric{
			{Label: "Database", Value: "SQLite", Tone: "primary"},
			{Label: "ACME", Value: "Staging", Tone: "warning"},
			{Label: "Storage", Value: "/data", Tone: "secondary"},
		},
		Columns:    []string{"Setting", "Value", "Source", "Status"},
		EmptyTitle: "Settings overview",
		EmptyText:  "Runtime settings will be surfaced as configuration support is expanded.",
	})
}

func (h *Handler) renderCertificateForm(c *fiber.Ctx, status int, form usecase.CertificateFormData, errors map[string]string, isEdit bool, domainsAndSNILock bool) error {
	title := "Add Certificate"
	action := "/certificates"
	description := "Create a pending certificate record before ACME issue support is enabled."
	if isEdit {
		title = "Edit Certificate"
		action = "/certificates/" + strconv.FormatInt(form.ID, 10)
	}

	return h.render(c, status, "templates/certificate_form.html", CertificateFormPage{
		PageData: PageData{
			Title:         title,
			Active:        "certificates",
			Heading:       title,
			Description:   description,
			PrimaryAction: "Issue certificate",
			StatusLabel:   "Metadata",
		},
		Form:              form,
		Errors:            errors,
		IsEdit:            isEdit,
		Action:            action,
		DomainsAndSNILock: domainsAndSNILock,
	})
}

func (h *Handler) renderKongTargetForm(c *fiber.Ctx, status int, form usecase.KongTargetFormData, errors map[string]string, isEdit bool) error {
	title := "Add Kong Target"
	action := "/kong-targets"
	if isEdit {
		title = "Edit Kong Target"
		action = "/kong-targets/" + strconv.FormatInt(form.ID, 10)
	}

	return h.render(c, status, "templates/kong_target_form.html", KongTargetFormPage{
		PageData: PageData{
			Title:         title,
			Active:        "kong-targets",
			Heading:       title,
			Description:   "Configure Kong Admin API metadata. Use the target list to run connectivity checks.",
			PrimaryAction: "Save target",
			StatusLabel:   "Target",
		},
		Form:   form,
		Errors: errors,
		IsEdit: isEdit,
		Action: action,
	})
}

func (h *Handler) render(c *fiber.Ctx, status int, contentTemplate string, data any) error {
	tmpl, err := template.New("layout.html").Funcs(template.FuncMap{
		"fieldError":  fieldError,
		"statusClass": statusClass,
		"join":        strings.Join,
	}).ParseFS(templateFiles, "templates/layout.html", contentTemplate)
	if err != nil {
		h.logger.Error("parse templates", "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, "template error")
	}

	var body bytes.Buffer
	if err := tmpl.ExecuteTemplate(&body, "layout", data); err != nil {
		h.logger.Error("render template", "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, "template error")
	}

	c.Type("html", "utf-8")
	return c.Status(status).SendString(body.String())
}

func (h *Handler) serverError(c *fiber.Ctx, message string, err error) error {
	h.logger.Error(message, "error", err)
	return fiber.NewError(fiber.StatusInternalServerError, "internal server error")
}

func fieldError(errors map[string]string, field string) string {
	if errors == nil {
		return ""
	}
	return errors[field]
}

func statusClass(status any) string {
	switch strings.ToLower(strings.TrimSpace(statusString(status))) {
	case "active", "synced", "online", "success":
		return "success"
	case "warning", "running":
		return "warning"
	case "expired", "failed", "offline":
		return "danger"
	default:
		return "secondary"
	}
}

func jobMetrics(jobs []usecase.JobView) (running int, succeeded int, failed int) {
	for _, job := range jobs {
		switch job.Job.Status {
		case "running":
			running++
		case "success":
			succeeded++
		case "failed":
			failed++
		}
	}
	return running, succeeded, failed
}

func statusString(status any) string {
	switch value := status.(type) {
	case string:
		return value
	case fmt.Stringer:
		return value.String()
	default:
		return fmt.Sprint(value)
	}
}

func certificateMetrics(certificates []usecase.CertificateView) (active int, warning int, failed int) {
	for _, certificate := range certificates {
		switch certificate.Certificate.Status {
		case "active":
			active++
		case "warning":
			warning++
		case "expired", "failed":
			failed++
		}
	}
	return active, warning, failed
}

func kongTargetFormFromRequest(c *fiber.Ctx, id int64) usecase.KongTargetFormData {
	authType := c.FormValue("auth_type")
	if authType == "" {
		authType = "none"
	}

	return usecase.KongTargetFormData{
		ID:              id,
		Name:            c.FormValue("name"),
		Environment:     c.FormValue("environment"),
		AdminURL:        c.FormValue("admin_url"),
		AuthType:        authType,
		AuthHeaderName:  c.FormValue("auth_header_name"),
		AuthHeaderValue: c.FormValue("auth_header_value"),
	}
}

func certificateFormFromRequest(c *fiber.Ctx, id int64) usecase.CertificateFormData {
	renewBeforeDays, _ := strconv.Atoi(strings.TrimSpace(c.FormValue("renew_before_days")))

	return usecase.CertificateFormData{
		ID:              id,
		Name:            c.FormValue("name"),
		Email:           c.FormValue("email"),
		DomainsText:     c.FormValue("domains"),
		SNIsText:        c.FormValue("snis"),
		AutoRenew:       c.FormValue("auto_renew") == "on",
		RenewBeforeDays: renewBeforeDays,
	}
}

func certificateInputFromForm(form usecase.CertificateFormData) usecase.CertificateInput {
	return usecase.CertificateInput{
		Name:            form.Name,
		Email:           form.Email,
		DomainsText:     form.DomainsText,
		SNIsText:        form.SNIsText,
		AutoRenew:       form.AutoRenew,
		RenewBeforeDays: form.RenewBeforeDays,
	}
}

func certificateLockedForEdit(certificate domain.Certificate) bool {
	return strings.TrimSpace(certificate.CertPath) != "" || strings.TrimSpace(certificate.KeyPath) != "" || certificate.ExpiresAt != nil
}

func parseSelectedTargetIDs(values []string) ([]int64, error) {
	ids := make([]int64, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		id, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("invalid target id")
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func kongTargetInputFromForm(form usecase.KongTargetFormData) usecase.KongTargetInput {
	return usecase.KongTargetInput{
		ID:              form.ID,
		Name:            form.Name,
		Environment:     form.Environment,
		AdminURL:        form.AdminURL,
		AuthType:        form.AuthType,
		AuthHeaderName:  form.AuthHeaderName,
		AuthHeaderValue: form.AuthHeaderValue,
	}
}

func kongTargetMetrics(targets []usecase.KongTargetView) (online int, offline int, unknown int) {
	for _, target := range targets {
		switch target.Target.Status {
		case "online":
			online++
		case "offline":
			offline++
		default:
			unknown++
		}
	}
	return online, offline, unknown
}
