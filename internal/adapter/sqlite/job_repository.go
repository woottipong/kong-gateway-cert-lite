package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"kong-cert-lite/internal/domain"
)

type JobRepository struct {
	db *sql.DB
}

func NewJobRepository(db *sql.DB) *JobRepository {
	return &JobRepository{db: db}
}

func (r *JobRepository) List(ctx context.Context) ([]domain.Job, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, certificate_id, kong_target_id, type, status,
		       message, log, started_at, finished_at, created_at
		FROM jobs
		ORDER BY started_at DESC, id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []domain.Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs: %w", err)
	}

	return jobs, nil
}

func (r *JobRepository) Get(ctx context.Context, id int64) (domain.Job, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, certificate_id, kong_target_id, type, status,
		       message, log, started_at, finished_at, created_at
		FROM jobs
		WHERE id = ?
	`, id)

	job, err := scanJob(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.Job{}, domain.ErrNotFound
		}
		return domain.Job{}, fmt.Errorf("get job: %w", err)
	}

	return job, nil
}

func (r *JobRepository) Create(ctx context.Context, job domain.Job) (int64, error) {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO jobs (
			certificate_id, kong_target_id, type, status, message, log
		) VALUES (?, ?, ?, ?, ?, ?)
	`,
		nullableInt64(job.CertificateID),
		nullableInt64(job.KongTargetID),
		string(job.Type),
		string(job.Status),
		job.Message,
		job.Log,
	)
	if err != nil {
		return 0, fmt.Errorf("create job: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read job id: %w", err)
	}

	return id, nil
}

func (r *JobRepository) Update(ctx context.Context, job domain.Job) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE jobs
		SET status = ?,
		    message = ?,
		    log = ?,
		    finished_at = ?
		WHERE id = ?
	`,
		string(job.Status),
		job.Message,
		job.Log,
		nullableTime(job.FinishedAt),
		job.ID,
	)
	if err != nil {
		return fmt.Errorf("update job: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read update rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func scanJob(scanner interface {
	Scan(dest ...any) error
}) (domain.Job, error) {
	var job domain.Job
	var certificateID sql.NullInt64
	var kongTargetID sql.NullInt64
	var jobType string
	var status string
	var startedAt string
	var finishedAt sql.NullString
	var createdAt string

	err := scanner.Scan(
		&job.ID,
		&certificateID,
		&kongTargetID,
		&jobType,
		&status,
		&job.Message,
		&job.Log,
		&startedAt,
		&finishedAt,
		&createdAt,
	)
	if err != nil {
		return domain.Job{}, err
	}

	if certificateID.Valid {
		job.CertificateID = &certificateID.Int64
	}
	if kongTargetID.Valid {
		job.KongTargetID = &kongTargetID.Int64
	}
	job.Type = domain.JobType(jobType)
	job.Status = domain.JobStatus(status)

	var errTime error
	job.StartedAt, errTime = parseSQLiteTime(startedAt)
	if errTime != nil {
		return domain.Job{}, fmt.Errorf("parse started_at: %w", errTime)
	}
	job.CreatedAt, errTime = parseSQLiteTime(createdAt)
	if errTime != nil {
		return domain.Job{}, fmt.Errorf("parse created_at: %w", errTime)
	}
	if finishedAt.Valid {
		parsed, err := parseSQLiteTime(finishedAt.String)
		if err != nil {
			return domain.Job{}, fmt.Errorf("parse finished_at: %w", err)
		}
		job.FinishedAt = &parsed
	}

	return job, nil
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format("2006-01-02 15:04:05")
}
