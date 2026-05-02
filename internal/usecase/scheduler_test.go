package usecase

import (
	"context"
	"testing"
	"time"

	"kong-cert-lite/internal/domain"
)

func TestRenewalSchedulerRunOnceRenewsOnlyCertificatesInsideWindow(t *testing.T) {
	now := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	dueExpiry := now.Add(5 * 24 * time.Hour)
	laterExpiry := now.Add(20 * 24 * time.Hour)

	repository := &fakeRenewalCertificateRepository{
		certificates: []domain.Certificate{
			{
				ID:              1,
				AutoRenew:       true,
				RenewBeforeDays: 10,
				ExpiresAt:       &dueExpiry,
				Status:          domain.CertificateStatusActive,
				CertPath:        "/tmp/certs/1/fullchain.pem",
				KeyPath:         "/tmp/certs/1/privkey.pem",
			},
			{
				ID:              2,
				AutoRenew:       true,
				RenewBeforeDays: 10,
				ExpiresAt:       &laterExpiry,
				Status:          domain.CertificateStatusActive,
				CertPath:        "/tmp/certs/2/fullchain.pem",
				KeyPath:         "/tmp/certs/2/privkey.pem",
			},
			{
				ID:              3,
				AutoRenew:       false,
				RenewBeforeDays: 10,
				ExpiresAt:       &dueExpiry,
				Status:          domain.CertificateStatusActive,
				CertPath:        "/tmp/certs/3/fullchain.pem",
				KeyPath:         "/tmp/certs/3/privkey.pem",
			},
			{
				ID:              4,
				AutoRenew:       true,
				RenewBeforeDays: 10,
				Status:          domain.CertificateStatusPending,
			},
		},
	}
	renewer := &fakeCertificateRenewer{}
	scheduler, err := NewRenewalScheduler(repository, renewer, "0 3 * * *")
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}

	if err := scheduler.RunOnce(context.Background(), now); err != nil {
		t.Fatalf("run scheduler: %v", err)
	}

	if len(renewer.renewedIDs) != 1 || renewer.renewedIDs[0] != 1 {
		t.Fatalf("expected only due certificate 1 to renew, got %v", renewer.renewedIDs)
	}
}

func TestRenewalSchedulerRunOnceSendsExpiryNotifications(t *testing.T) {
	now := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	expiring := now.Add(13 * 24 * time.Hour)
	notYet := now.Add(12 * 24 * time.Hour)
	repository := &fakeRenewalCertificateRepository{
		certificates: []domain.Certificate{
			{
				ID:            1,
				Name:          "Production wildcard",
				PrimaryDomain: "*.example.com",
				ExpiresAt:     &expiring,
				Status:        domain.CertificateStatusActive,
				CertPath:      "/tmp/certs/1/fullchain.pem",
			},
			{
				ID:            2,
				Name:          "Outside window",
				PrimaryDomain: "api.example.com",
				ExpiresAt:     &notYet,
				Status:        domain.CertificateStatusActive,
				CertPath:      "/tmp/certs/2/fullchain.pem",
			},
		},
	}
	renewer := &fakeCertificateRenewer{}
	notifier := &fakeNotifier{}
	scheduler, err := NewRenewalScheduler(repository, renewer, "0 3 * * *")
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}
	scheduler.SetNotifier(notifier)

	if err := scheduler.RunOnce(context.Background(), now); err != nil {
		t.Fatalf("run scheduler: %v", err)
	}

	if len(notifier.events) != 1 {
		t.Fatalf("expected one notification, got %d", len(notifier.events))
	}
	event := notifier.events[0]
	if event.Event != "certificate_expiring_14_days" {
		t.Fatalf("expected 14-day expiry event, got %q", event.Event)
	}
	if event.RemainingDays == nil || *event.RemainingDays != 13 {
		t.Fatalf("expected remaining days 13, got %#v", event.RemainingDays)
	}
}

func TestRenewalSchedulerDoesNotRepeatExpiryNotificationSameDay(t *testing.T) {
	now := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	expiring := now.Add(13 * 24 * time.Hour)
	repository := &fakeRenewalCertificateRepository{
		certificates: []domain.Certificate{
			{
				ID:            1,
				Name:          "Production wildcard",
				PrimaryDomain: "*.example.com",
				ExpiresAt:     &expiring,
				Status:        domain.CertificateStatusActive,
				CertPath:      "/tmp/certs/1/fullchain.pem",
			},
		},
	}
	notifier := &fakeNotifier{}
	scheduler, err := NewRenewalScheduler(repository, &fakeCertificateRenewer{}, "0 3 * * *")
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}
	scheduler.SetNotifier(notifier)

	if err := scheduler.RunOnce(context.Background(), now); err != nil {
		t.Fatalf("run scheduler: %v", err)
	}
	if err := scheduler.RunOnce(context.Background(), now.Add(2*time.Hour)); err != nil {
		t.Fatalf("run scheduler again: %v", err)
	}

	if len(notifier.events) != 1 {
		t.Fatalf("expected one notification for same day, got %d", len(notifier.events))
	}
}

func TestParseCronExpressionRejectsInvalidConfig(t *testing.T) {
	if _, err := ParseCronExpression("not a cron"); err == nil {
		t.Fatal("expected invalid cron expression to fail")
	}
}

func TestRenewalSchedulerStopBeforeStartReturns(t *testing.T) {
	scheduler, err := NewRenewalScheduler(&fakeRenewalCertificateRepository{}, &fakeCertificateRenewer{}, "0 3 * * *")
	if err != nil {
		t.Fatalf("create scheduler: %v", err)
	}

	stopped := make(chan struct{})
	go func() {
		scheduler.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected scheduler stop before start to return")
	}
}

type fakeRenewalCertificateRepository struct {
	certificates []domain.Certificate
}

func (r *fakeRenewalCertificateRepository) List(ctx context.Context) ([]domain.Certificate, error) {
	_ = ctx
	return r.certificates, nil
}

type fakeCertificateRenewer struct {
	renewedIDs []int64
}

func (r *fakeCertificateRenewer) RenewCertificate(ctx context.Context, certificateID int64) error {
	_ = ctx
	r.renewedIDs = append(r.renewedIDs, certificateID)
	return nil
}

type fakeNotifier struct {
	events []NotificationEvent
}

func (n *fakeNotifier) Notify(ctx context.Context, event NotificationEvent) error {
	_ = ctx
	n.events = append(n.events, event)
	return nil
}
