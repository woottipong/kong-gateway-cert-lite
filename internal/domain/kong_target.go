package domain

import "time"

type KongTargetStatus string

const (
	KongTargetStatusOnline  KongTargetStatus = "online"
	KongTargetStatusOffline KongTargetStatus = "offline"
	KongTargetStatusUnknown KongTargetStatus = "unknown"
)

type KongTargetAuthType string

const (
	KongTargetAuthTypeNone         KongTargetAuthType = "none"
	KongTargetAuthTypeCustomHeader KongTargetAuthType = "custom-header"
)

type KongTarget struct {
	ID              int64
	Name            string
	Environment     string
	AdminURL        string
	AuthType        KongTargetAuthType
	AuthHeaderName  string
	AuthHeaderValue string
	Status          KongTargetStatus
	LastCheckedAt   *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
