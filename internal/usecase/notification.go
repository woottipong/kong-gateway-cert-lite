package usecase

import (
	"context"
	"math"
	"strings"
	"time"

	"kong-cert-lite/internal/domain"
)

type NotificationSeverity string

const (
	NotificationSeverityInfo     NotificationSeverity = "info"
	NotificationSeverityWarning  NotificationSeverity = "warning"
	NotificationSeverityCritical NotificationSeverity = "critical"
	NotificationSeveritySuccess  NotificationSeverity = "success"
)

type NotificationEvent struct {
	Severity      NotificationSeverity
	Event         string
	Certificate   *domain.Certificate
	KongTarget    string
	Environment   string
	JobID         int64
	JobType       domain.JobType
	JobStatus     domain.JobStatus
	Message       string
	RemainingDays *int
	OccurredAt    time.Time
}

type Notifier interface {
	Notify(ctx context.Context, event NotificationEvent) error
}

func notifyJobResult(ctx context.Context, notifier Notifier, notifySuccess bool, event NotificationEvent) {
	if notifier == nil {
		return
	}
	if event.JobStatus == domain.JobStatusSuccess && !notifySuccess {
		return
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	_ = notifier.Notify(ctx, event)
}

func certificateExpiryNotification(certificate domain.Certificate, now time.Time) (NotificationEvent, bool) {
	if certificate.ExpiresAt == nil {
		return NotificationEvent{}, false
	}
	if strings.TrimSpace(certificate.CertPath) == "" {
		return NotificationEvent{}, false
	}

	remainingDays := int(math.Ceil(certificate.ExpiresAt.UTC().Sub(now.UTC()).Hours() / 24))
	severity := NotificationSeverityCritical
	event := ""
	switch {
	case remainingDays >= 13 && remainingDays <= 14:
		severity = NotificationSeverityWarning
		event = "certificate_expiring_14_days"
	case remainingDays >= 6 && remainingDays <= 7:
		event = "certificate_expiring_7_days"
	case remainingDays >= 2 && remainingDays <= 3:
		event = "certificate_expiring_3_days"
	case remainingDays >= -1 && remainingDays <= 0:
		event = "certificate_expired"
	default:
		return NotificationEvent{}, false
	}

	return NotificationEvent{
		Severity:      severity,
		Event:         event,
		Certificate:   &certificate,
		Message:       "Certificate expiry threshold reached",
		RemainingDays: &remainingDays,
		OccurredAt:    now.UTC(),
	}, true
}
