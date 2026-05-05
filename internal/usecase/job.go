package usecase

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"kong-cert-lite/internal/domain"
)

type JobRepository interface {
	List(ctx context.Context) ([]domain.Job, error)
	Get(ctx context.Context, id int64) (domain.Job, error)
	Create(ctx context.Context, job domain.Job) (int64, error)
	Update(ctx context.Context, job domain.Job) error
	DeleteCompleted(ctx context.Context) (int64, error)
	HasRunningCertificateJob(ctx context.Context, certificateID int64, types []domain.JobType) (bool, error)
	LatestFailedCertificateJobSince(ctx context.Context, certificateID int64, types []domain.JobType, since time.Time) (*domain.Job, error)
}

type JobUseCase struct {
	repository JobRepository
}

type JobInput struct {
	CertificateID *int64
	KongTargetID  *int64
	Type          string
	Status        string
	Message       string
	Log           string
}

type JobCompleteInput struct {
	ID      int64
	Status  string
	Message string
	Log     string
}

type JobView struct {
	Job              domain.Job
	TypeLabel        string
	StatusLabel      string
	StartedAt        string
	FinishedAt       string
	CertificateLabel string
	KongTargetLabel  string
}

func NewJobUseCase(repository JobRepository) *JobUseCase {
	return &JobUseCase{repository: repository}
}

func (uc *JobUseCase) List(ctx context.Context) ([]JobView, error) {
	jobs, err := uc.repository.List(ctx)
	if err != nil {
		return nil, err
	}

	views := make([]JobView, 0, len(jobs))
	for _, job := range jobs {
		views = append(views, buildJobView(job))
	}

	return views, nil
}

func (uc *JobUseCase) Get(ctx context.Context, id int64) (JobView, error) {
	job, err := uc.repository.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return JobView{}, ErrNotFound
		}
		return JobView{}, err
	}

	return buildJobView(job), nil
}

func (uc *JobUseCase) Create(ctx context.Context, input JobInput) (int64, error) {
	job, err := validateJobInput(input)
	if err != nil {
		return 0, err
	}

	return uc.repository.Create(ctx, job)
}

func (uc *JobUseCase) Complete(ctx context.Context, input JobCompleteInput) error {
	if input.ID <= 0 {
		return fmt.Errorf("invalid job id")
	}

	existing, err := uc.repository.Get(ctx, input.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}

	status := domain.JobStatus(strings.TrimSpace(input.Status))
	switch status {
	case domain.JobStatusSuccess, domain.JobStatusFailed:
	default:
		return ValidationError{Fields: map[string]string{"status": "Job completion status must be success or failed."}}
	}

	now := time.Now().UTC()
	existing.Status = status
	existing.Message = strings.TrimSpace(input.Message)
	existing.Log = strings.TrimSpace(input.Log)
	existing.FinishedAt = &now

	return uc.repository.Update(ctx, existing)
}

func (uc *JobUseCase) ClearCompleted(ctx context.Context) (int64, error) {
	if uc == nil || uc.repository == nil {
		return 0, fmt.Errorf("job dependencies are not configured")
	}
	return uc.repository.DeleteCompleted(ctx)
}

func (uc *JobUseCase) HasRunningCertificateJob(ctx context.Context, certificateID int64, types []domain.JobType) (bool, error) {
	if uc == nil || uc.repository == nil {
		return false, fmt.Errorf("job dependencies are not configured")
	}
	return uc.repository.HasRunningCertificateJob(ctx, certificateID, types)
}

func (uc *JobUseCase) LatestFailedCertificateJobSince(ctx context.Context, certificateID int64, types []domain.JobType, since time.Time) (*domain.Job, error) {
	if uc == nil || uc.repository == nil {
		return nil, fmt.Errorf("job dependencies are not configured")
	}
	return uc.repository.LatestFailedCertificateJobSince(ctx, certificateID, types, since)
}

func ParseJobID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid id")
	}
	return id, nil
}

func validateJobInput(input JobInput) (domain.Job, error) {
	fields := make(map[string]string)

	job := domain.Job{
		CertificateID: input.CertificateID,
		KongTargetID:  input.KongTargetID,
		Type:          domain.JobType(strings.TrimSpace(input.Type)),
		Status:        domain.JobStatus(strings.TrimSpace(input.Status)),
		Message:       strings.TrimSpace(input.Message),
		Log:           strings.TrimSpace(input.Log),
	}

	if job.Status == "" {
		job.Status = domain.JobStatusRunning
	}

	switch job.Type {
	case domain.JobTypeIssue, domain.JobTypeRenew, domain.JobTypeSync, domain.JobTypeTestKong, domain.JobTypeDelete:
	default:
		fields["type"] = "Job type is invalid."
	}

	switch job.Status {
	case domain.JobStatusRunning, domain.JobStatusSuccess, domain.JobStatusFailed:
	default:
		fields["status"] = "Job status is invalid."
	}

	if len(fields) > 0 {
		return job, ValidationError{Fields: fields}
	}

	return job, nil
}

func buildJobView(job domain.Job) JobView {
	finishedAt := "Not finished"
	if job.FinishedAt != nil {
		finishedAt = formatJobTime(*job.FinishedAt)
	}

	return JobView{
		Job:              job,
		TypeLabel:        statusLabelForValue(string(job.Type)),
		StatusLabel:      statusLabelForValue(string(job.Status)),
		StartedAt:        formatJobTime(job.StartedAt),
		FinishedAt:       finishedAt,
		CertificateLabel: optionalIDLabel("Certificate", job.CertificateID),
		KongTargetLabel:  optionalIDLabel("Kong target", job.KongTargetID),
	}
}

func optionalIDLabel(prefix string, id *int64) string {
	if id == nil {
		return "Not linked"
	}
	return prefix + " #" + strconv.FormatInt(*id, 10)
}

func formatJobTime(value time.Time) string {
	if value.IsZero() {
		return "Unknown"
	}
	return value.Format("2006-01-02 15:04")
}
