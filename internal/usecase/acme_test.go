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

type fakeACMEIssueClient struct {
	request ACMEIssueRequest
	result  ACMEIssueResult
	err     error
}

func (f *fakeACMEIssueClient) Issue(ctx context.Context, request ACMEIssueRequest) (ACMEIssueResult, error) {
	_ = ctx
	f.request = request
	if f.err != nil {
		return ACMEIssueResult{}, f.err
	}
	return f.result, nil
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
