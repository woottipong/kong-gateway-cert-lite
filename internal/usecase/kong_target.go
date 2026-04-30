package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"kong-cert-lite/internal/domain"
)

type KongTargetRepository interface {
	List(ctx context.Context) ([]domain.KongTarget, error)
	Get(ctx context.Context, id int64) (domain.KongTarget, error)
	Create(ctx context.Context, target domain.KongTarget) (int64, error)
	Update(ctx context.Context, target domain.KongTarget) error
	Delete(ctx context.Context, id int64) error
}

type KongAdminClient interface {
	CheckConnection(ctx context.Context, target domain.KongTarget) (string, error)
}

type KongTargetUseCase struct {
	repository KongTargetRepository
	tester     KongAdminClient
	jobs       *JobUseCase
}

type KongTargetInput struct {
	ID              int64
	Name            string
	Environment     string
	AdminURL        string
	AuthType        string
	AuthHeaderName  string
	AuthHeaderValue string
}

type KongTargetFormData struct {
	ID              int64
	Name            string
	Environment     string
	AdminURL        string
	AuthType        string
	AuthHeaderName  string
	AuthHeaderValue string
	HasSecret       bool
}

type KongTargetView struct {
	Target      domain.KongTarget
	StatusLabel string
	AuthLabel   string
	LastChecked string
}

func NewKongTargetUseCase(repository KongTargetRepository, tester KongAdminClient, jobs *JobUseCase) *KongTargetUseCase {
	return &KongTargetUseCase{repository: repository, tester: tester, jobs: jobs}
}

func (uc *KongTargetUseCase) List(ctx context.Context) ([]KongTargetView, error) {
	targets, err := uc.repository.List(ctx)
	if err != nil {
		return nil, err
	}

	views := make([]KongTargetView, 0, len(targets))
	for _, target := range targets {
		views = append(views, buildKongTargetView(target))
	}

	return views, nil
}

func (uc *KongTargetUseCase) Get(ctx context.Context, id int64) (KongTargetView, error) {
	target, err := uc.repository.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return KongTargetView{}, ErrNotFound
		}
		return KongTargetView{}, err
	}

	return buildKongTargetView(target), nil
}

func (uc *KongTargetUseCase) Create(ctx context.Context, input KongTargetInput) (int64, error) {
	target, err := validateKongTargetInput(input, false)
	if err != nil {
		return 0, err
	}

	return uc.repository.Create(ctx, target)
}

func (uc *KongTargetUseCase) Update(ctx context.Context, input KongTargetInput) error {
	existing, err := uc.repository.Get(ctx, input.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}

	target, err := validateKongTargetInput(input, true)
	if err != nil {
		return err
	}
	target.ID = input.ID
	target.Status = existing.Status
	target.LastCheckedAt = existing.LastCheckedAt
	if target.AuthType == domain.KongTargetAuthTypeCustomHeader && target.AuthHeaderValue == "" {
		target.AuthHeaderValue = existing.AuthHeaderValue
	}

	return uc.repository.Update(ctx, target)
}

func (uc *KongTargetUseCase) Delete(ctx context.Context, id int64) error {
	if err := uc.repository.Delete(ctx, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}

	return nil
}

func (uc *KongTargetUseCase) TestConnection(ctx context.Context, id int64) error {
	if uc.tester == nil || uc.jobs == nil {
		return fmt.Errorf("kong target test dependencies are not configured")
	}

	target, err := uc.repository.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}

	jobID, err := uc.jobs.Create(ctx, JobInput{
		KongTargetID: &target.ID,
		Type:         string(domain.JobTypeTestKong),
		Message:      "Testing Kong Admin API connectivity",
		Log:          "Starting Kong Admin API connectivity test for " + target.AdminURL,
	})
	if err != nil {
		return err
	}

	detail, testErr := uc.tester.CheckConnection(ctx, target)
	checkedAt := time.Now().UTC()
	target.LastCheckedAt = &checkedAt

	jobStatus := string(domain.JobStatusSuccess)
	jobMessage := "Kong Admin API reachable"
	jobLog := strings.TrimSpace("Testing Kong target " + target.AdminURL + "\n" + detail)
	if testErr != nil {
		target.Status = domain.KongTargetStatusOffline
		jobStatus = string(domain.JobStatusFailed)
		jobMessage = testErr.Error()
		jobLog = strings.TrimSpace(jobLog + "\n" + testErr.Error())
	} else {
		target.Status = domain.KongTargetStatusOnline
	}

	if err := uc.repository.Update(ctx, target); err != nil {
		return err
	}

	if err := uc.jobs.Complete(ctx, JobCompleteInput{
		ID:      jobID,
		Status:  jobStatus,
		Message: jobMessage,
		Log:     jobLog,
	}); err != nil {
		return err
	}

	return nil
}

func ParseKongTargetID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid id")
	}
	return id, nil
}

func validateKongTargetInput(input KongTargetInput, allowExistingSecret bool) (domain.KongTarget, error) {
	fields := make(map[string]string)
	authType := strings.TrimSpace(input.AuthType)
	if authType == "" {
		authType = string(domain.KongTargetAuthTypeNone)
	}

	target := domain.KongTarget{
		Name:            strings.TrimSpace(input.Name),
		Environment:     strings.TrimSpace(input.Environment),
		AdminURL:        strings.TrimSpace(input.AdminURL),
		AuthType:        domain.KongTargetAuthType(authType),
		AuthHeaderName:  strings.TrimSpace(input.AuthHeaderName),
		AuthHeaderValue: strings.TrimSpace(input.AuthHeaderValue),
		Status:          domain.KongTargetStatusUnknown,
	}

	if target.Name == "" {
		fields["name"] = "Target name is required."
	}
	if target.Environment == "" {
		fields["environment"] = "Environment is required."
	}
	if target.AdminURL == "" {
		fields["admin_url"] = "Admin API URL is required."
	} else if parsed, err := url.ParseRequestURI(target.AdminURL); err != nil || parsed.Scheme == "" || parsed.Host == "" {
		fields["admin_url"] = "Enter a valid Admin API URL."
	} else if parsed.Scheme != "http" && parsed.Scheme != "https" {
		fields["admin_url"] = "Admin API URL must start with http:// or https://."
	}

	switch target.AuthType {
	case domain.KongTargetAuthTypeNone:
		target.AuthHeaderName = ""
		target.AuthHeaderValue = ""
	case domain.KongTargetAuthTypeCustomHeader:
		if target.AuthHeaderName == "" {
			fields["auth_header_name"] = "Header name is required for custom-header auth."
		} else if !isHTTPHeaderName(target.AuthHeaderName) {
			fields["auth_header_name"] = "Enter a valid HTTP header name."
		}
		if target.AuthHeaderValue == "" && !allowExistingSecret {
			fields["auth_header_value"] = "Header value is required for custom-header auth."
		}
	default:
		fields["auth_type"] = "Auth type must be none or custom-header."
	}

	if len(fields) > 0 {
		return target, ValidationError{Fields: fields}
	}

	return target, nil
}

func isHTTPHeaderName(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		switch {
		case char >= 'A' && char <= 'Z':
		case char >= 'a' && char <= 'z':
		case char >= '0' && char <= '9':
		case strings.ContainsRune("!#$%&'*+-.^_`|~", char):
		default:
			return false
		}
	}
	return true
}

func buildKongTargetView(target domain.KongTarget) KongTargetView {
	lastChecked := "Never"
	if target.LastCheckedAt != nil {
		lastChecked = target.LastCheckedAt.Format("2006-01-02 15:04")
	}

	return KongTargetView{
		Target:      target,
		StatusLabel: statusLabelForValue(string(target.Status)),
		AuthLabel:   authLabel(target),
		LastChecked: lastChecked,
	}
}

func authLabel(target domain.KongTarget) string {
	if target.AuthType == domain.KongTargetAuthTypeCustomHeader {
		return "Custom header"
	}
	return "None"
}

func statusLabelForValue(value string) string {
	words := strings.Fields(strings.ReplaceAll(value, "_", " "))
	for index, word := range words {
		words[index] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}
