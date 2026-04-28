package usecase

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"kong-cert-lite/internal/domain"
)

type IssueCertificateRepository interface {
	Get(ctx context.Context, id int64) (domain.Certificate, error)
	MarkIssued(ctx context.Context, id int64, certPath string, keyPath string, expiresAt time.Time) error
	UpdateStatus(ctx context.Context, id int64, status domain.CertificateStatus) error
}

type ACMEIssueRequest struct {
	Email   string
	Domains []string
}

type ACMEIssueResult struct {
	FullChainPEM  []byte
	PrivateKeyPEM []byte
}

type ACMEIssueClient interface {
	Issue(ctx context.Context, request ACMEIssueRequest) (ACMEIssueResult, error)
}

type ACMEUseCase struct {
	certificates IssueCertificateRepository
	jobs         *JobUseCase
	client       ACMEIssueClient
	certDir      string
}

func NewACMEUseCase(certificates IssueCertificateRepository, jobs *JobUseCase, client ACMEIssueClient, certDir string) *ACMEUseCase {
	return &ACMEUseCase{
		certificates: certificates,
		jobs:         jobs,
		client:       client,
		certDir:      certDir,
	}
}

func (uc *ACMEUseCase) IssueCertificate(ctx context.Context, certificateID int64) error {
	if uc.certificates == nil || uc.jobs == nil || uc.client == nil {
		return fmt.Errorf("acme issue dependencies are not configured")
	}

	certificate, err := uc.certificates.Get(ctx, certificateID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}

	jobID, err := uc.jobs.Create(ctx, JobInput{
		CertificateID: &certificate.ID,
		Type:          string(domain.JobTypeIssue),
		Message:       "Issuing certificate via Let's Encrypt",
		Log:           "Starting ACME DNS-01 issue for " + strings.Join(certificate.Domains, ", "),
	})
	if err != nil {
		return err
	}

	result, issueErr := uc.client.Issue(ctx, ACMEIssueRequest{
		Email:   certificate.Email,
		Domains: certificate.Domains,
	})
	if issueErr != nil {
		return uc.failIssue(ctx, certificate.ID, jobID, issueErr)
	}

	certPath, keyPath, err := uc.writeIssuedFiles(certificate.ID, result.FullChainPEM, result.PrivateKeyPEM)
	if err != nil {
		return uc.failIssue(ctx, certificate.ID, jobID, err)
	}

	expiresAt, err := parseCertificateExpiry(result.FullChainPEM)
	if err != nil {
		return uc.failIssue(ctx, certificate.ID, jobID, err)
	}

	if err := uc.certificates.MarkIssued(ctx, certificate.ID, certPath, keyPath, expiresAt); err != nil {
		return err
	}

	logOutput := strings.Join([]string{
		"Issued certificate for " + strings.Join(certificate.Domains, ", "),
		"Saved fullchain to " + certPath,
		"Saved private key to " + keyPath,
		"Expires at " + expiresAt.UTC().Format(time.RFC3339),
	}, "\n")

	if err := uc.jobs.Complete(ctx, JobCompleteInput{
		ID:      jobID,
		Status:  string(domain.JobStatusSuccess),
		Message: "Certificate issued successfully",
		Log:     logOutput,
	}); err != nil {
		return err
	}

	return nil
}

func (uc *ACMEUseCase) failIssue(ctx context.Context, certificateID int64, jobID int64, cause error) error {
	// Always attempt both operations so the job is never left in a running state.
	// Combine any persistence errors with errors.Join rather than returning early.
	statusErr := uc.certificates.UpdateStatus(ctx, certificateID, domain.CertificateStatusFailed)

	logOutput := strings.Join([]string{
		"Certificate issue failed",
		cause.Error(),
	}, "\n")

	jobErr := uc.jobs.Complete(ctx, JobCompleteInput{
		ID:      jobID,
		Status:  string(domain.JobStatusFailed),
		Message: cause.Error(),
		Log:     logOutput,
	})

	return errors.Join(statusErr, jobErr)
}

func (uc *ACMEUseCase) writeIssuedFiles(certificateID int64, fullChainPEM []byte, privateKeyPEM []byte) (string, string, error) {
	if strings.TrimSpace(uc.certDir) == "" {
		return "", "", fmt.Errorf("certificate directory is not configured")
	}
	if len(fullChainPEM) == 0 {
		return "", "", fmt.Errorf("issued certificate is empty")
	}
	if len(privateKeyPEM) == 0 {
		return "", "", fmt.Errorf("issued private key is empty")
	}

	targetDir := filepath.Join(uc.certDir, strconv.FormatInt(certificateID, 10))
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create certificate directory %s: %w", targetDir, err)
	}

	certPath := filepath.Join(targetDir, "fullchain.pem")
	keyPath := filepath.Join(targetDir, "privkey.pem")

	if err := os.WriteFile(certPath, fullChainPEM, 0o600); err != nil {
		return "", "", fmt.Errorf("write certificate file %s: %w", certPath, err)
	}
	if err := os.WriteFile(keyPath, privateKeyPEM, 0o600); err != nil {
		return "", "", fmt.Errorf("write private key file %s: %w", keyPath, err)
	}

	return certPath, keyPath, nil
}

func parseCertificateExpiry(fullChainPEM []byte) (time.Time, error) {
	rest := fullChainPEM
	for len(rest) > 0 {
		block, remaining := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = remaining
		if block.Type != "CERTIFICATE" {
			continue
		}
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return time.Time{}, fmt.Errorf("parse issued certificate: %w", err)
		}
		return certificate.NotAfter.UTC(), nil
	}

	return time.Time{}, fmt.Errorf("issued certificate does not contain a valid PEM certificate")
}
