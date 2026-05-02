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

const acmeRetryCooldown = 15 * time.Minute

var acmeJobTypes = []domain.JobType{domain.JobTypeIssue, domain.JobTypeRenew}

type OperationBlockedError struct {
	Message string
}

func (e OperationBlockedError) Error() string {
	return e.Message
}

type ACMEOperationState struct {
	Blocked    bool
	Reason     string
	RetryAfter string
}

type IssueCertificateRepository interface {
	Get(ctx context.Context, id int64) (domain.Certificate, error)
	MarkIssued(ctx context.Context, id int64, certPath string, keyPath string, expiresAt time.Time) error
	UpdateStatus(ctx context.Context, id int64, status domain.CertificateStatus) error
}

type ACMEIssueRequest struct {
	Email   string
	Domains []string
}

type ACMERenewRequest struct {
	Email                 string
	Domains               []string
	ExistingFullChainPEM  []byte
	ExistingPrivateKeyPEM []byte
}

type ACMEIssueResult struct {
	FullChainPEM  []byte
	PrivateKeyPEM []byte
}

type ACMEIssueClient interface {
	Issue(ctx context.Context, request ACMEIssueRequest) (ACMEIssueResult, error)
	Renew(ctx context.Context, request ACMERenewRequest) (ACMEIssueResult, error)
}

type ACMEUseCase struct {
	certificates  IssueCertificateRepository
	jobs          *JobUseCase
	client        ACMEIssueClient
	certDir       string
	kongSync      *KongSyncUseCase
	notifier      Notifier
	notifySuccess bool
}

func NewACMEUseCase(certificates IssueCertificateRepository, jobs *JobUseCase, client ACMEIssueClient, certDir string, kongSync ...*KongSyncUseCase) *ACMEUseCase {
	uc := &ACMEUseCase{
		certificates: certificates,
		jobs:         jobs,
		client:       client,
		certDir:      certDir,
	}
	if len(kongSync) > 0 {
		uc.kongSync = kongSync[0]
	}
	return uc
}

func (uc *ACMEUseCase) SetNotifier(notifier Notifier, notifySuccess bool) {
	uc.notifier = notifier
	uc.notifySuccess = notifySuccess
}

func (uc *ACMEUseCase) OperationState(ctx context.Context, certificateID int64) (ACMEOperationState, error) {
	return uc.acmeOperationState(ctx, certificateID, time.Now().UTC())
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
	if err := uc.ensureACMEOperationAllowed(ctx, certificate.ID); err != nil {
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
		return uc.failIssue(ctx, certificate, jobID, issueErr)
	}

	certPath, keyPath, err := uc.writeIssuedFiles(certificate.ID, result.FullChainPEM, result.PrivateKeyPEM)
	if err != nil {
		return uc.failIssue(ctx, certificate, jobID, err)
	}

	expiresAt, err := parseCertificateExpiry(result.FullChainPEM)
	if err != nil {
		return uc.failIssue(ctx, certificate, jobID, err)
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
	notifyJobResult(ctx, uc.notifier, uc.notifySuccess, NotificationEvent{
		Severity:    NotificationSeveritySuccess,
		Event:       "issue_succeeded",
		Certificate: &certificate,
		JobID:       jobID,
		JobType:     domain.JobTypeIssue,
		JobStatus:   domain.JobStatusSuccess,
		Message:     "Certificate issued successfully",
	})

	return nil
}

func (uc *ACMEUseCase) RenewCertificate(ctx context.Context, certificateID int64) error {
	if uc.certificates == nil || uc.jobs == nil || uc.client == nil {
		return fmt.Errorf("acme renew dependencies are not configured")
	}

	certificate, err := uc.certificates.Get(ctx, certificateID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	if err := uc.ensureACMEOperationAllowed(ctx, certificate.ID); err != nil {
		return err
	}

	jobID, err := uc.jobs.Create(ctx, JobInput{
		CertificateID: &certificate.ID,
		Type:          string(domain.JobTypeRenew),
		Message:       "Renewing certificate via Let's Encrypt",
		Log:           "Starting ACME DNS-01 renew for " + strings.Join(certificate.Domains, ", "),
	})
	if err != nil {
		return err
	}

	existingFullChainPEM, existingPrivateKeyPEM, err := readExistingCertificateFiles(certificate.CertPath, certificate.KeyPath)
	if err != nil {
		return uc.failRenew(ctx, certificate, jobID, err)
	}

	result, renewErr := uc.client.Renew(ctx, ACMERenewRequest{
		Email:                 certificate.Email,
		Domains:               certificate.Domains,
		ExistingFullChainPEM:  existingFullChainPEM,
		ExistingPrivateKeyPEM: existingPrivateKeyPEM,
	})
	if renewErr != nil {
		return uc.failRenew(ctx, certificate, jobID, renewErr)
	}

	certPath, keyPath, err := uc.writeIssuedFiles(certificate.ID, result.FullChainPEM, result.PrivateKeyPEM)
	if err != nil {
		return uc.failRenew(ctx, certificate, jobID, err)
	}

	expiresAt, err := parseCertificateExpiry(result.FullChainPEM)
	if err != nil {
		return uc.failRenew(ctx, certificate, jobID, err)
	}

	if err := uc.certificates.MarkIssued(ctx, certificate.ID, certPath, keyPath, expiresAt); err != nil {
		return err
	}

	logOutput := strings.Join([]string{
		"Renewed certificate for " + strings.Join(certificate.Domains, ", "),
		"Saved fullchain to " + certPath,
		"Saved private key to " + keyPath,
		"Expires at " + expiresAt.UTC().Format(time.RFC3339),
	}, "\n")

	if err := uc.jobs.Complete(ctx, JobCompleteInput{
		ID:      jobID,
		Status:  string(domain.JobStatusSuccess),
		Message: "Certificate renewed successfully",
		Log:     logOutput,
	}); err != nil {
		return err
	}
	notifyJobResult(ctx, uc.notifier, uc.notifySuccess, NotificationEvent{
		Severity:    NotificationSeveritySuccess,
		Event:       "renew_succeeded",
		Certificate: &certificate,
		JobID:       jobID,
		JobType:     domain.JobTypeRenew,
		JobStatus:   domain.JobStatusSuccess,
		Message:     "Certificate renewed successfully",
	})

	if uc.kongSync != nil {
		return uc.kongSync.SyncCertificate(ctx, certificate.ID)
	}

	return nil
}

func (uc *ACMEUseCase) ensureACMEOperationAllowed(ctx context.Context, certificateID int64) error {
	state, err := uc.acmeOperationState(ctx, certificateID, time.Now().UTC())
	if err != nil {
		return err
	}
	if state.Blocked {
		return OperationBlockedError{Message: state.Reason}
	}
	return nil
}

func (uc *ACMEUseCase) acmeOperationState(ctx context.Context, certificateID int64, now time.Time) (ACMEOperationState, error) {
	running, err := uc.jobs.HasRunningCertificateJob(ctx, certificateID, acmeJobTypes)
	if err != nil {
		return ACMEOperationState{}, err
	}
	if running {
		return ACMEOperationState{
			Blocked: true,
			Reason:  "An issue or renew job is already running for this certificate.",
		}, nil
	}

	recentFailure, err := uc.jobs.LatestFailedCertificateJobSince(ctx, certificateID, acmeJobTypes, now.UTC().Add(-acmeRetryCooldown))
	if err != nil {
		return ACMEOperationState{}, err
	}
	if recentFailure != nil {
		retryAt := recentFailure.StartedAt.UTC().Add(acmeRetryCooldown)
		return ACMEOperationState{
			Blocked:    true,
			Reason:     "The last issue or renew attempt failed recently. Wait 15 minutes before retrying.",
			RetryAfter: retryAt.Format("2006-01-02 15:04 UTC"),
		}, nil
	}

	return ACMEOperationState{}, nil
}

func (uc *ACMEUseCase) failIssue(ctx context.Context, certificate domain.Certificate, jobID int64, cause error) error {
	// Always attempt both operations so the job is never left in a running state.
	// Combine any persistence errors with errors.Join rather than returning early.
	statusErr := uc.certificates.UpdateStatus(ctx, certificate.ID, domain.CertificateStatusFailed)

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
	notifyJobResult(ctx, uc.notifier, uc.notifySuccess, NotificationEvent{
		Severity:    NotificationSeverityCritical,
		Event:       "issue_failed",
		Certificate: &certificate,
		JobID:       jobID,
		JobType:     domain.JobTypeIssue,
		JobStatus:   domain.JobStatusFailed,
		Message:     cause.Error(),
	})

	return errors.Join(statusErr, jobErr)
}

func (uc *ACMEUseCase) failRenew(ctx context.Context, certificate domain.Certificate, jobID int64, cause error) error {
	statusErr := uc.certificates.UpdateStatus(ctx, certificate.ID, domain.CertificateStatusFailed)

	logOutput := strings.Join([]string{
		"Certificate renew failed",
		cause.Error(),
	}, "\n")

	jobErr := uc.jobs.Complete(ctx, JobCompleteInput{
		ID:      jobID,
		Status:  string(domain.JobStatusFailed),
		Message: cause.Error(),
		Log:     logOutput,
	})
	notifyJobResult(ctx, uc.notifier, uc.notifySuccess, NotificationEvent{
		Severity:    NotificationSeverityCritical,
		Event:       "renew_failed",
		Certificate: &certificate,
		JobID:       jobID,
		JobType:     domain.JobTypeRenew,
		JobStatus:   domain.JobStatusFailed,
		Message:     cause.Error(),
	})

	return errors.Join(statusErr, jobErr)
}

func readExistingCertificateFiles(certPath string, keyPath string) ([]byte, []byte, error) {
	certPEM, err := readExistingCertificateFile(certPath, "certificate")
	if err != nil {
		return nil, nil, err
	}
	keyPEM, err := readExistingCertificateFile(keyPath, "private key")
	if err != nil {
		return nil, nil, err
	}
	return certPEM, keyPEM, nil
}

func readExistingCertificateFile(path string, label string) ([]byte, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("missing existing %s file path", label)
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("missing existing %s file: %s", label, path)
		}
		return nil, fmt.Errorf("read existing %s file %s: %w", label, path, err)
	}
	return bytes, nil
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
