package domain

import "time"

type CertificateStatus string

const (
	CertificateStatusPending CertificateStatus = "pending"
	CertificateStatusActive  CertificateStatus = "active"
	CertificateStatusWarning CertificateStatus = "warning"
	CertificateStatusExpired CertificateStatus = "expired"
	CertificateStatusFailed  CertificateStatus = "failed"
)

type Certificate struct {
	ID              int64
	Name            string
	PrimaryDomain   string
	Domains         []string
	Email           string
	SNIs            []string
	CertPath        string
	KeyPath         string
	ExpiresAt       *time.Time
	AutoRenew       bool
	RenewBeforeDays int
	Status          CertificateStatus
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CertificateKongTarget struct {
	ID                int64
	CertificateID     int64
	KongTargetID      int64
	KongCertificateID string
	SyncStatus        SyncStatus
	LastSyncedAt      *time.Time
	LastError         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type SyncStatus string

const (
	SyncStatusPending   SyncStatus = "pending"
	SyncStatusSynced    SyncStatus = "synced"
	SyncStatusFailed    SyncStatus = "failed"
	SyncStatusNotSynced SyncStatus = "not_synced"
)
