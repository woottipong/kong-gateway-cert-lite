package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"sort"
	"strconv"
	"strings"
	"time"

	"kong-cert-lite/internal/domain"
)

var ErrNotFound = domain.ErrNotFound

type CertificateRepository interface {
	List(ctx context.Context) ([]domain.Certificate, error)
	Get(ctx context.Context, id int64) (domain.Certificate, error)
	Create(ctx context.Context, certificate domain.Certificate) (int64, error)
	Update(ctx context.Context, certificate domain.Certificate) error
	Delete(ctx context.Context, id int64) error
	ListKongTargets(ctx context.Context) ([]domain.KongTarget, error)
	ListSyncTargets(ctx context.Context, certificateID int64) ([]domain.CertificateKongTarget, error)
	CountLinkedTargetsByCertificate(ctx context.Context) (map[int64]int, error)
	SetLinkedTargets(ctx context.Context, certificateID int64, targetIDs []int64) error
}

type CertificateUseCase struct {
	repository CertificateRepository
}

type CertificateInput struct {
	Name            string
	Email           string
	DomainsText     string
	SNIsText        string
	AutoRenew       bool
	RenewBeforeDays int
}

type CertificateFormData struct {
	ID              int64
	Name            string
	Email           string
	DomainsText     string
	SNIsText        string
	AutoRenew       bool
	RenewBeforeDays int
}

type CertificateView struct {
	Certificate       domain.Certificate
	Type              string
	Expires           string
	Remaining         string
	StatusLabel       string
	LinkedTargetCount int
	LinkedTargets     []CertificateTargetLinkView
}

type CertificateTargetLinkView struct {
	Target            domain.KongTarget
	IsLinked          bool
	SyncStatusLabel   string
	SyncStatusTone    string
	KongCertificateID string
	LastError         string
}

type ValidationError struct {
	Fields map[string]string
}

func (e ValidationError) Error() string {
	return "validation failed"
}

func NewCertificateUseCase(repository CertificateRepository) *CertificateUseCase {
	return &CertificateUseCase{repository: repository}
}

func (uc *CertificateUseCase) List(ctx context.Context) ([]CertificateView, error) {
	certificates, err := uc.repository.List(ctx)
	if err != nil {
		return nil, err
	}

	// Fetch all linked target counts in a single query to avoid N+1 per certificate.
	counts, err := uc.repository.CountLinkedTargetsByCertificate(ctx)
	if err != nil {
		return nil, err
	}

	views := make([]CertificateView, 0, len(certificates))
	for _, certificate := range certificates {
		view := buildCertificateView(certificate)
		view.LinkedTargetCount = counts[certificate.ID]
		views = append(views, view)
	}

	return views, nil
}

func (uc *CertificateUseCase) Get(ctx context.Context, id int64) (CertificateView, error) {
	certificate, err := uc.repository.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return CertificateView{}, ErrNotFound
		}
		return CertificateView{}, err
	}

	targets, err := uc.repository.ListKongTargets(ctx)
	if err != nil {
		return CertificateView{}, err
	}
	links, err := uc.repository.ListSyncTargets(ctx, id)
	if err != nil {
		return CertificateView{}, err
	}

	view := buildCertificateView(certificate)
	view.LinkedTargetCount = len(links)
	view.LinkedTargets = buildCertificateTargetLinkViews(targets, links)

	return view, nil
}

func (uc *CertificateUseCase) Create(ctx context.Context, input CertificateInput) (int64, error) {
	certificate, err := validateCertificateInput(input)
	if err != nil {
		return 0, err
	}

	return uc.repository.Create(ctx, certificate)
}

func (uc *CertificateUseCase) Update(ctx context.Context, input CertificateInput, id int64) error {
	existing, err := uc.repository.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}

	if isIssuedCertificate(existing) {
		input.DomainsText = strings.Join(existing.Domains, "\n")
		input.SNIsText = strings.Join(existing.SNIs, "\n")
	}

	certificate, err := validateCertificateInput(input)
	if err != nil {
		return err
	}
	certificate.ID = existing.ID

	return uc.repository.Update(ctx, certificate)
}

func (uc *CertificateUseCase) Delete(ctx context.Context, id int64) error {
	if err := uc.repository.Delete(ctx, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}

	return nil
}

func (uc *CertificateUseCase) UpdateLinkedTargets(ctx context.Context, certificateID int64, targetIDs []int64) error {
	if _, err := uc.repository.Get(ctx, certificateID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}

	availableTargets, err := uc.repository.ListKongTargets(ctx)
	if err != nil {
		return err
	}
	allowed := make(map[int64]struct{}, len(availableTargets))
	for _, target := range availableTargets {
		allowed[target.ID] = struct{}{}
	}

	normalizedIDs := uniqueSortedIDs(targetIDs)
	for _, targetID := range normalizedIDs {
		if _, ok := allowed[targetID]; !ok {
			return ValidationError{Fields: map[string]string{"kong_target_ids": "Select valid Kong targets."}}
		}
	}

	return uc.repository.SetLinkedTargets(ctx, certificateID, normalizedIDs)
}

func ParseID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid id")
	}
	return id, nil
}

func validateCertificateInput(input CertificateInput) (domain.Certificate, error) {
	fields := make(map[string]string)
	certificate := domain.Certificate{
		Name:            strings.TrimSpace(input.Name),
		Email:           strings.TrimSpace(input.Email),
		Domains:         splitLines(input.DomainsText),
		SNIs:            splitLines(input.SNIsText),
		AutoRenew:       input.AutoRenew,
		RenewBeforeDays: input.RenewBeforeDays,
		Status:          domain.CertificateStatusPending,
	}

	if len(certificate.Domains) > 0 {
		certificate.PrimaryDomain = certificate.Domains[0]
	}

	if certificate.Name == "" {
		fields["name"] = "Certificate name is required."
	}
	if _, err := mail.ParseAddress(certificate.Email); certificate.Email == "" || err != nil {
		fields["email"] = "A valid email address is required."
	}
	if len(certificate.Domains) == 0 {
		fields["domains"] = "Add at least one domain."
	}
	if len(certificate.SNIs) == 0 {
		fields["snis"] = "Add at least one SNI value."
	}
	if certificate.RenewBeforeDays <= 0 {
		fields["renew_before_days"] = "Renew before days must be greater than 0."
	}

	if len(fields) > 0 {
		return certificate, ValidationError{Fields: fields}
	}

	return certificate, nil
}

func splitLines(value string) []string {
	seen := make(map[string]struct{})
	var output []string

	for _, line := range strings.Split(value, "\n") {
		item := strings.TrimSpace(line)
		if item == "" {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		output = append(output, item)
	}

	return output
}

func buildCertificateView(certificate domain.Certificate) CertificateView {
	view := CertificateView{
		Certificate: certificate,
		Type:        certificateType(certificate.Domains),
		Expires:     "Not issued",
		Remaining:   "-",
		StatusLabel: statusLabel(certificate.Status),
	}

	if certificate.ExpiresAt != nil {
		view.Expires = certificate.ExpiresAt.Format("2006-01-02")
		days := int(time.Until(*certificate.ExpiresAt).Hours() / 24)
		view.Remaining = fmt.Sprintf("%d days", days)
	}

	return view
}

func buildCertificateTargetLinkViews(targets []domain.KongTarget, links []domain.CertificateKongTarget) []CertificateTargetLinkView {
	linkMap := make(map[int64]domain.CertificateKongTarget, len(links))
	for _, link := range links {
		linkMap[link.KongTargetID] = link
	}

	views := make([]CertificateTargetLinkView, 0, len(targets))
	for _, target := range targets {
		link, linked := linkMap[target.ID]
		view := CertificateTargetLinkView{
			Target:          target,
			IsLinked:        linked,
			SyncStatusLabel: "Not linked",
			SyncStatusTone:  "secondary",
		}
		if linked {
			view.SyncStatusLabel = statusLabelForValue(string(link.SyncStatus))
			view.SyncStatusTone = statusClassForValue(string(link.SyncStatus))
			view.KongCertificateID = link.KongCertificateID
			view.LastError = link.LastError
		}
		views = append(views, view)
	}

	return views
}

func uniqueSortedIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})
	return result
}

func isIssuedCertificate(certificate domain.Certificate) bool {
	return strings.TrimSpace(certificate.CertPath) != "" || strings.TrimSpace(certificate.KeyPath) != "" || certificate.ExpiresAt != nil
}

func statusClassForValue(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
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

func certificateType(domains []string) string {
	for _, domain := range domains {
		if strings.HasPrefix(domain, "*.") {
			return "Wildcard"
		}
	}
	return "Single"
}

func statusLabel(status domain.CertificateStatus) string {
	value := strings.ReplaceAll(string(status), "_", " ")
	words := strings.Fields(value)
	for index, word := range words {
		words[index] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}
