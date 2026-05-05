package usecase_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	acmeadapter "kong-cert-lite/internal/adapter/acme"
	sqliteadapter "kong-cert-lite/internal/adapter/sqlite"
	"kong-cert-lite/internal/db"
	"kong-cert-lite/internal/domain"
	"kong-cert-lite/internal/usecase"
)

func TestLiveStagingIssueCertificateWithCloudflareDNS01(t *testing.T) {
	cloudflareToken := strings.TrimSpace(os.Getenv("CF_DNS_API_TOKEN"))
	email := strings.TrimSpace(os.Getenv("ACME_LIVE_EMAIL"))
	domains := liveDomainsFromEnv()

	if cloudflareToken == "" || email == "" || len(domains) == 0 {
		t.Skip("set CF_DNS_API_TOKEN, ACME_LIVE_EMAIL, and ACME_LIVE_DOMAINS to run live ACME staging issue")
	}

	letsencryptEnv := strings.ToLower(strings.TrimSpace(os.Getenv("LETSENCRYPT_ENV")))
	if letsencryptEnv == "" {
		letsencryptEnv = "staging"
	}
	if letsencryptEnv != "staging" {
		t.Fatalf("live ACME verification must use LETSENCRYPT_ENV=staging, got %q", letsencryptEnv)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	database, err := db.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})

	certificateRepository := sqliteadapter.NewCertificateRepository(database)
	jobRepository := sqliteadapter.NewJobRepository(database)
	jobUseCase := usecase.NewJobUseCase(jobRepository)

	certificateID, err := certificateRepository.Create(ctx, domain.Certificate{
		Name:            "Live staging verification",
		PrimaryDomain:   domains[0],
		Domains:         domains,
		Email:           email,
		SNIs:            domains,
		AutoRenew:       true,
		RenewBeforeDays: 30,
		Status:          domain.CertificateStatusPending,
	})
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certDir := filepath.Join(t.TempDir(), "certs")
	accountDir := filepath.Join(t.TempDir(), "accounts")
	acmeClient := acmeadapter.NewLegoClient(accountDir, letsencryptEnv, cloudflareToken)
	acmeUseCase := usecase.NewACMEUseCase(certificateRepository, jobUseCase, acmeClient, certDir)

	if err := acmeUseCase.IssueCertificate(ctx, certificateID); err != nil {
		t.Fatalf("issue live staging certificate: %v", err)
	}

	stored, err := certificateRepository.Get(ctx, certificateID)
	if err != nil {
		t.Fatalf("get certificate: %v", err)
	}
	if stored.Status != domain.CertificateStatusActive {
		t.Fatalf("expected active status, got %q", stored.Status)
	}
	if stored.CertPath == "" || stored.KeyPath == "" {
		t.Fatalf("expected cert and key paths to be stored, got cert=%q key=%q", stored.CertPath, stored.KeyPath)
	}
	if stored.ExpiresAt == nil || !stored.ExpiresAt.After(time.Now().UTC()) {
		t.Fatalf("expected future expiry, got %v", stored.ExpiresAt)
	}
	if _, err := os.Stat(stored.CertPath); err != nil {
		t.Fatalf("stat issued certificate file: %v", err)
	}
	if _, err := os.Stat(stored.KeyPath); err != nil {
		t.Fatalf("stat issued private key file: %v", err)
	}

	jobs, err := jobRepository.List(ctx)
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 issue job, got %d", len(jobs))
	}
	if jobs[0].Type != domain.JobTypeIssue || jobs[0].Status != domain.JobStatusSuccess {
		t.Fatalf("expected successful issue job, got type=%q status=%q", jobs[0].Type, jobs[0].Status)
	}
}

func TestLiveStagingRenewCertificateWithCloudflareDNS01(t *testing.T) {
	cloudflareToken := strings.TrimSpace(os.Getenv("CF_DNS_API_TOKEN"))
	email := strings.TrimSpace(os.Getenv("ACME_LIVE_EMAIL"))
	domains := liveDomainsFromEnv()

	if cloudflareToken == "" || email == "" || len(domains) == 0 {
		t.Skip("set CF_DNS_API_TOKEN, ACME_LIVE_EMAIL, and ACME_LIVE_DOMAINS to run live ACME staging renew")
	}

	letsencryptEnv := strings.ToLower(strings.TrimSpace(os.Getenv("LETSENCRYPT_ENV")))
	if letsencryptEnv == "" {
		letsencryptEnv = "staging"
	}
	if letsencryptEnv != "staging" {
		t.Fatalf("live ACME verification must use LETSENCRYPT_ENV=staging, got %q", letsencryptEnv)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	database, err := db.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})

	certificateRepository := sqliteadapter.NewCertificateRepository(database)
	kongTargetRepository := sqliteadapter.NewKongTargetRepository(database)
	jobRepository := sqliteadapter.NewJobRepository(database)
	jobUseCase := usecase.NewJobUseCase(jobRepository)

	certificateID, err := certificateRepository.Create(ctx, domain.Certificate{
		Name:            "Live staging renew verification",
		PrimaryDomain:   domains[0],
		Domains:         domains,
		Email:           email,
		SNIs:            domains,
		AutoRenew:       true,
		RenewBeforeDays: 30,
		Status:          domain.CertificateStatusPending,
	})
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	targetID, err := kongTargetRepository.Create(ctx, domain.KongTarget{
		Name:        "Live verification Kong",
		Environment: "staging",
		AdminURL:    "https://kong-admin.example.invalid:8444",
		AuthType:    domain.KongTargetAuthTypeNone,
		Status:      domain.KongTargetStatusOnline,
	})
	if err != nil {
		t.Fatalf("create kong target: %v", err)
	}
	if err := certificateRepository.SetLinkedTargets(ctx, certificateID, []int64{targetID}); err != nil {
		t.Fatalf("link kong target: %v", err)
	}

	certDir := filepath.Join(t.TempDir(), "certs")
	accountDir := filepath.Join(t.TempDir(), "accounts")
	acmeClient := acmeadapter.NewLegoClient(accountDir, letsencryptEnv, cloudflareToken)
	kongClient := &liveRenewKongSyncClient{}
	kongSyncUseCase := usecase.NewKongSyncUseCase(certificateRepository, kongTargetRepository, jobUseCase, kongClient)
	acmeUseCase := usecase.NewACMEUseCase(certificateRepository, jobUseCase, acmeClient, certDir, kongSyncUseCase)

	if err := acmeUseCase.IssueCertificate(ctx, certificateID); err != nil {
		t.Fatalf("issue live staging certificate before renew: %v", err)
	}
	issued, err := certificateRepository.Get(ctx, certificateID)
	if err != nil {
		t.Fatalf("get issued certificate: %v", err)
	}
	issuedPEM, err := os.ReadFile(issued.CertPath)
	if err != nil {
		t.Fatalf("read issued certificate: %v", err)
	}

	if err := acmeUseCase.RenewCertificate(ctx, certificateID); err != nil {
		t.Fatalf("renew live staging certificate: %v", err)
	}

	renewed, err := certificateRepository.Get(ctx, certificateID)
	if err != nil {
		t.Fatalf("get renewed certificate: %v", err)
	}
	if renewed.Status != domain.CertificateStatusActive {
		t.Fatalf("expected active status, got %q", renewed.Status)
	}
	if renewed.ExpiresAt == nil || !renewed.ExpiresAt.After(time.Now().UTC()) {
		t.Fatalf("expected future renewed expiry, got %v", renewed.ExpiresAt)
	}
	renewedPEM, err := os.ReadFile(renewed.CertPath)
	if err != nil {
		t.Fatalf("read renewed certificate: %v", err)
	}
	if len(renewedPEM) == 0 || string(renewedPEM) == string(issuedPEM) {
		t.Fatal("expected renewed certificate file to be updated")
	}
	if kongClient.calls != 1 {
		t.Fatalf("expected linked Kong target sync after renew, got %d calls", kongClient.calls)
	}
	if kongClient.certPEM != string(renewedPEM) {
		t.Fatal("expected Kong sync to receive renewed certificate PEM")
	}

	jobs, err := jobRepository.List(ctx)
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	var sawRenewJob bool
	var sawSyncJob bool
	for _, job := range jobs {
		if job.Type == domain.JobTypeRenew && job.Status == domain.JobStatusSuccess {
			sawRenewJob = true
		}
		if job.Type == domain.JobTypeSync && job.Status == domain.JobStatusSuccess {
			sawSyncJob = true
		}
	}
	if !sawRenewJob {
		t.Fatal("expected successful renew job")
	}
	if !sawSyncJob {
		t.Fatal("expected successful sync job after live renew")
	}
}

type liveRenewKongSyncClient struct {
	calls   int
	certPEM string
}

func (c *liveRenewKongSyncClient) SyncCertificate(ctx context.Context, target domain.KongTarget, existingKongCertificateID string, certPEM string, keyPEM string, snis []string, tags []string) (string, string, error) {
	_ = ctx
	_ = target
	_ = existingKongCertificateID
	_ = keyPEM
	_ = snis
	_ = tags
	c.calls++
	c.certPEM = certPEM
	return "live-renew-kong-certificate", "live renew sync verification", nil
}

func liveDomainsFromEnv() []string {
	rawDomains := strings.Split(os.Getenv("ACME_LIVE_DOMAINS"), ",")
	domains := make([]string, 0, len(rawDomains))
	for _, domain := range rawDomains {
		domain = strings.TrimSpace(domain)
		if domain != "" {
			domains = append(domains, domain)
		}
	}
	return domains
}
