package usecase

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"kong-cert-lite/internal/domain"
)

type SyncCertificateRepository interface {
	Get(ctx context.Context, id int64) (domain.Certificate, error)
	ListSyncTargets(ctx context.Context, certificateID int64) ([]domain.CertificateKongTarget, error)
	UpsertSyncTarget(ctx context.Context, mapping domain.CertificateKongTarget) error
}

type SyncKongTargetRepository interface {
	Get(ctx context.Context, id int64) (domain.KongTarget, error)
}

type KongCertificateSyncClient interface {
	SyncCertificate(ctx context.Context, target domain.KongTarget, existingKongCertificateID string, certPEM string, keyPEM string, snis []string) (string, string, error)
}

type KongSyncUseCase struct {
	certificates SyncCertificateRepository
	targets      SyncKongTargetRepository
	jobs         *JobUseCase
	client       KongCertificateSyncClient
}

func NewKongSyncUseCase(certificates SyncCertificateRepository, targets SyncKongTargetRepository, jobs *JobUseCase, client KongCertificateSyncClient) *KongSyncUseCase {
	return &KongSyncUseCase{
		certificates: certificates,
		targets:      targets,
		jobs:         jobs,
		client:       client,
	}
}

func (uc *KongSyncUseCase) SyncCertificate(ctx context.Context, certificateID int64) error {
	if uc.certificates == nil || uc.targets == nil || uc.jobs == nil || uc.client == nil {
		return fmt.Errorf("kong sync dependencies are not configured")
	}

	certificate, err := uc.certificates.Get(ctx, certificateID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}

	linkedTargets, err := uc.certificates.ListSyncTargets(ctx, certificateID)
	if err != nil {
		return err
	}
	if len(linkedTargets) == 0 {
		return nil
	}

	certPEM, keyPEM, readErr := readCertificateFiles(certificate.CertPath, certificate.KeyPath)
	for _, mapping := range linkedTargets {
		target, err := uc.targets.Get(ctx, mapping.KongTargetID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				continue
			}
			return err
		}
		if err := uc.syncTarget(ctx, certificate, target, mapping, certPEM, keyPEM, readErr); err != nil {
			return err
		}
	}

	return nil
}

func (uc *KongSyncUseCase) syncTarget(ctx context.Context, certificate domain.Certificate, target domain.KongTarget, mapping domain.CertificateKongTarget, certPEM string, keyPEM string, readErr error) error {

	jobID, err := uc.jobs.Create(ctx, JobInput{
		CertificateID: &certificate.ID,
		KongTargetID:  &target.ID,
		Type:          string(domain.JobTypeSync),
		Message:       "Syncing certificate to Kong target",
		Log:           "Starting certificate sync for " + target.Name,
	})
	if err != nil {
		return err
	}

	status := string(domain.JobStatusSuccess)
	message := "Certificate synced to Kong target"
	logOutput := ""

	if readErr != nil {
		mapping.SyncStatus = domain.SyncStatusFailed
		mapping.LastError = readErr.Error()
		status = string(domain.JobStatusFailed)
		message = "Certificate sync failed: " + readErr.Error()
		logOutput = "Unable to read certificate files for sync\n" + readErr.Error()
	} else {
		kongCertificateID, detail, syncErr := uc.client.SyncCertificate(ctx, target, mapping.KongCertificateID, certPEM, keyPEM, certificate.SNIs)
		logOutput = strings.TrimSpace("Syncing certificate to " + target.AdminURL + "\n" + detail)
		if syncErr != nil {
			mapping.SyncStatus = domain.SyncStatusFailed
			mapping.LastError = syncErr.Error()
			status = string(domain.JobStatusFailed)
			message = "Certificate sync failed: " + syncErr.Error()
			logOutput = strings.TrimSpace(logOutput + "\n" + syncErr.Error())
		} else {
			now := time.Now().UTC()
			mapping.KongCertificateID = kongCertificateID
			mapping.SyncStatus = domain.SyncStatusSynced
			mapping.LastSyncedAt = &now
			mapping.LastError = ""
			message = "Certificate synced to Kong target with Kong certificate id " + kongCertificateID
			logOutput = strings.TrimSpace(logOutput + "\nKong certificate id: " + kongCertificateID)
		}
	}

	if err := uc.certificates.UpsertSyncTarget(ctx, mapping); err != nil {
		return err
	}

	if err := uc.jobs.Complete(ctx, JobCompleteInput{
		ID:      jobID,
		Status:  status,
		Message: message,
		Log:     logOutput,
	}); err != nil {
		return err
	}

	return nil
}

func readCertificateFiles(certPath string, keyPath string) (string, string, error) {
	certPEM, err := readRequiredFile(certPath, "certificate")
	if err != nil {
		return "", "", err
	}
	keyPEM, err := readRequiredFile(keyPath, "private key")
	if err != nil {
		return "", "", err
	}

	return certPEM, keyPEM, nil
}

func readRequiredFile(path string, label string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("missing %s file path", label)
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("missing %s file: %s", label, path)
		}
		return "", fmt.Errorf("read %s file %s: %w", label, path, err)
	}

	return string(bytes), nil
}
