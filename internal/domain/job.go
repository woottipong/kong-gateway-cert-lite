package domain

import "time"

type JobType string

const (
	JobTypeIssue    JobType = "issue"
	JobTypeRenew    JobType = "renew"
	JobTypeSync     JobType = "sync"
	JobTypeTestKong JobType = "test_kong"
	JobTypeDelete   JobType = "delete"
)

type JobStatus string

const (
	JobStatusRunning JobStatus = "running"
	JobStatusSuccess JobStatus = "success"
	JobStatusFailed  JobStatus = "failed"
)

type Job struct {
	ID            int64
	CertificateID *int64
	KongTargetID  *int64
	Type          JobType
	Status        JobStatus
	Message       string
	Log           string
	StartedAt     time.Time
	FinishedAt    *time.Time
	CreatedAt     time.Time
}
