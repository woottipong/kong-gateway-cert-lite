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
