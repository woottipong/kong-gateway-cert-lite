package usecase

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sqliteadapter "kong-cert-lite/internal/adapter/sqlite"
	"kong-cert-lite/internal/db"
	"kong-cert-lite/internal/domain"
)

func TestACMEUseCaseIssueCertificateStoresFilesAndMarksActive(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})

	certificateRepository := sqliteadapter.NewCertificateRepository(database)
	jobRepository := sqliteadapter.NewJobRepository(database)
	jobUseCase := NewJobUseCase(jobRepository)

	expiresAt := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Second)
	certificatePEM, privateKeyPEM := issuedCertificateFixture(t, expiresAt)
	fakeClient := &fakeACMEIssueClient{
		result: ACMEIssueResult{
			FullChainPEM:  certificatePEM,
			PrivateKeyPEM: privateKeyPEM,
		},
	}

	certificateID, err := certificateRepository.Create(ctx, domain.Certificate{
		Name:            "Caption wildcard",
		PrimaryDomain:   "caption.rtt.in.th",
		Domains:         []string{"caption.rtt.in.th", "*.caption.rtt.in.th"},
		Email:           "ops@rtt.in.th",
		SNIs:            []string{"caption.rtt.in.th", "*.caption.rtt.in.th"},
		AutoRenew:       true,
		RenewBeforeDays: 30,
		Status:          domain.CertificateStatusPending,
	})
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certDir := filepath.Join(t.TempDir(), "certs")
	useCase := NewACMEUseCase(certificateRepository, jobUseCase, fakeClient, certDir)

	if err := useCase.IssueCertificate(ctx, certificateID); err != nil {
		t.Fatalf("issue certificate: %v", err)
	}

	stored, err := certificateRepository.Get(ctx, certificateID)
	if err != nil {
		t.Fatalf("get certificate: %v", err)
	}
	if stored.Status != domain.CertificateStatusActive {
		t.Fatalf("expected active certificate status, got %q", stored.Status)
	}
	if stored.CertPath == "" {
		t.Fatal("expected certificate path to be stored")
	}
	if stored.KeyPath == "" {
		t.Fatal("expected private key path to be stored")
	}
	if stored.ExpiresAt == nil {
		t.Fatal("expected expiry to be stored")
	}
	if !stored.ExpiresAt.UTC().Equal(expiresAt) {
		t.Fatalf("expected expiry %s, got %s", expiresAt.Format(time.RFC3339), stored.ExpiresAt.UTC().Format(time.RFC3339))
	}

	storedCertificatePEM, err := os.ReadFile(stored.CertPath)
	if err != nil {
		t.Fatalf("read stored certificate file: %v", err)
	}
	if string(storedCertificatePEM) != string(certificatePEM) {
		t.Fatal("stored certificate PEM does not match issued certificate")
	}

	storedPrivateKeyPEM, err := os.ReadFile(stored.KeyPath)
	if err != nil {
		t.Fatalf("read stored key file: %v", err)
	}
	if string(storedPrivateKeyPEM) != string(privateKeyPEM) {
		t.Fatal("stored private key PEM does not match issued key")
	}

	jobs, err := jobRepository.List(ctx)
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Type != domain.JobTypeIssue {
		t.Fatalf("expected issue job type, got %q", jobs[0].Type)
	}
	if jobs[0].Status != domain.JobStatusSuccess {
		t.Fatalf("expected success job status, got %q", jobs[0].Status)
	}
	if jobs[0].Message != "Certificate issued successfully" {
		t.Fatalf("unexpected job message %q", jobs[0].Message)
	}
	if !strings.Contains(jobs[0].Log, stored.CertPath) {
		t.Fatalf("expected job log to mention certificate path, got %q", jobs[0].Log)
	}
	if fakeClient.request.Email != "ops@rtt.in.th" {
		t.Fatalf("expected issue request email to be forwarded, got %q", fakeClient.request.Email)
	}
	if len(fakeClient.request.Domains) != 2 {
		t.Fatalf("expected issue request domains to be forwarded, got %v", fakeClient.request.Domains)
	}
}

func TestACMEUseCaseRenewCertificateStoresFilesAndSyncsLinkedTargets(t *testing.T) {
	ctx := context.Background()
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
	jobUseCase := NewJobUseCase(jobRepository)

	oldExpiresAt := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Second)
	renewedExpiresAt := time.Now().UTC().Add(45 * 24 * time.Hour).Truncate(time.Second)
	oldCertificatePEM, oldPrivateKeyPEM := issuedCertificateFixture(t, oldExpiresAt)
	renewedCertificatePEM, renewedPrivateKeyPEM := issuedCertificateFixture(t, renewedExpiresAt)

	fakeACMEClient := &fakeACMEIssueClient{
		renewResult: ACMEIssueResult{
			FullChainPEM:  renewedCertificatePEM,
			PrivateKeyPEM: renewedPrivateKeyPEM,
		},
	}
	fakeKongClient := &fakeKongCertificateSyncClient{
		kongCertificateID: "kong-cert-renewed",
		detail:            "updated certificate in kong",
	}

	certificateID, err := certificateRepository.Create(ctx, domain.Certificate{
		Name:            "Renewed wildcard",
		PrimaryDomain:   "caption.rtt.in.th",
		Domains:         []string{"caption.rtt.in.th", "*.caption.rtt.in.th"},
		Email:           "ops@rtt.in.th",
		SNIs:            []string{"caption.rtt.in.th", "*.caption.rtt.in.th"},
		AutoRenew:       true,
		RenewBeforeDays: 30,
		Status:          domain.CertificateStatusPending,
	})
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certDir := filepath.Join(t.TempDir(), "certs")
	oldCertDir := filepath.Join(certDir, "old")
	if err := os.MkdirAll(oldCertDir, 0o755); err != nil {
		t.Fatalf("create old cert dir: %v", err)
	}
	oldCertPath := filepath.Join(oldCertDir, "fullchain.pem")
	oldKeyPath := filepath.Join(oldCertDir, "privkey.pem")
	if err := os.WriteFile(oldCertPath, oldCertificatePEM, 0o600); err != nil {
		t.Fatalf("write old certificate: %v", err)
	}
	if err := os.WriteFile(oldKeyPath, oldPrivateKeyPEM, 0o600); err != nil {
		t.Fatalf("write old key: %v", err)
	}
	if err := certificateRepository.MarkIssued(ctx, certificateID, oldCertPath, oldKeyPath, oldExpiresAt); err != nil {
		t.Fatalf("mark issued certificate: %v", err)
	}

	targetID, err := kongTargetRepository.Create(ctx, domain.KongTarget{
		Name:        "Production Kong",
		Environment: "production",
		AdminURL:    "https://kong-admin.internal:8444",
		AuthType:    domain.KongTargetAuthTypeNone,
		Status:      domain.KongTargetStatusOnline,
	})
	if err != nil {
		t.Fatalf("create kong target: %v", err)
	}
	if err := certificateRepository.SetLinkedTargets(ctx, certificateID, []int64{targetID}); err != nil {
		t.Fatalf("link kong target: %v", err)
	}

	kongSyncUseCase := NewKongSyncUseCase(certificateRepository, kongTargetRepository, jobUseCase, fakeKongClient)
	acmeUseCase := NewACMEUseCase(certificateRepository, jobUseCase, fakeACMEClient, certDir, kongSyncUseCase)

	if err := acmeUseCase.RenewCertificate(ctx, certificateID); err != nil {
		t.Fatalf("renew certificate: %v", err)
	}

	stored, err := certificateRepository.Get(ctx, certificateID)
	if err != nil {
		t.Fatalf("get certificate: %v", err)
	}
	if stored.Status != domain.CertificateStatusActive {
		t.Fatalf("expected active certificate status, got %q", stored.Status)
	}
	if stored.ExpiresAt == nil || !stored.ExpiresAt.UTC().Equal(renewedExpiresAt) {
		t.Fatalf("expected renewed expiry %s, got %v", renewedExpiresAt.Format(time.RFC3339), stored.ExpiresAt)
	}
	if stored.CertPath == oldCertPath {
		t.Fatal("expected renewed certificate path to replace the previous path")
	}
	storedCertificatePEM, err := os.ReadFile(stored.CertPath)
	if err != nil {
		t.Fatalf("read renewed certificate: %v", err)
	}
	if string(storedCertificatePEM) != string(renewedCertificatePEM) {
		t.Fatal("stored renewed certificate PEM does not match ACME result")
	}
	if fakeACMEClient.renewRequest.Email != "ops@rtt.in.th" {
		t.Fatalf("expected renew request email to be forwarded, got %q", fakeACMEClient.renewRequest.Email)
	}
	if string(fakeACMEClient.renewRequest.ExistingFullChainPEM) != string(oldCertificatePEM) {
		t.Fatal("expected existing certificate PEM to be sent to ACME renew client")
	}
	if fakeKongClient.calls != 1 {
		t.Fatalf("expected one Kong sync call after renew, got %d", fakeKongClient.calls)
	}
	if fakeKongClient.certPEM != string(renewedCertificatePEM) {
		t.Fatal("expected Kong sync to use renewed certificate PEM")
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
		t.Fatal("expected successful sync job after renew")
	}
}

type fakeACMEIssueClient struct {
	request      ACMEIssueRequest
	renewRequest ACMERenewRequest
	result       ACMEIssueResult
	renewResult  ACMEIssueResult
	err          error
	renewErr     error
}

func (f *fakeACMEIssueClient) Issue(ctx context.Context, request ACMEIssueRequest) (ACMEIssueResult, error) {
	_ = ctx
	f.request = request
	if f.err != nil {
		return ACMEIssueResult{}, f.err
	}
	return f.result, nil
}

func (f *fakeACMEIssueClient) Renew(ctx context.Context, request ACMERenewRequest) (ACMEIssueResult, error) {
	_ = ctx
	f.renewRequest = request
	if f.renewErr != nil {
		return ACMEIssueResult{}, f.renewErr
	}
	return f.renewResult, nil
}

type fakeKongCertificateSyncClient struct {
	calls             int
	certPEM           string
	kongCertificateID string
	detail            string
	err               error
}

func (f *fakeKongCertificateSyncClient) SyncCertificate(ctx context.Context, target domain.KongTarget, existingKongCertificateID string, certPEM string, keyPEM string, snis []string) (string, string, error) {
	_ = ctx
	_ = target
	_ = existingKongCertificateID
	_ = keyPEM
	_ = snis
	f.calls++
	f.certPEM = certPEM
	return f.kongCertificateID, f.detail, f.err
}

func issuedCertificateFixture(t *testing.T, expiresAt time.Time) ([]byte, []byte) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate private key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "caption.rtt.in.th",
		},
		DNSNames:              []string{"caption.rtt.in.th", "*.caption.rtt.in.th"},
		NotBefore:             expiresAt.Add(-24 * time.Hour),
		NotAfter:              expiresAt,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	return certificatePEM, privateKeyPEM
}
