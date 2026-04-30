# Task: Manual renew

## Task ID
task-010-manual-renew

## Epic
epic-04-acme

## Area
backend

## Status
done

## Priority
high

## Depends On
- task-008-kong-certificate-sync
- task-009-acme-certificate-issue

## Summary
Implement manual certificate renewal and post-renew sync.

## Scope
- Add `POST /certificates/{id}/renew`.
- Renew existing certificate through ACME service.
- Save updated cert/key files.
- Update expiry and status.
- Sync renewed certificate to linked Kong targets.
- Create detailed renew and sync job logs.

## Out of Scope
- Scheduled auto renew.
- Notification.
- Webhooks.

## Acceptance Criteria
- User can manually renew an existing certificate.
- Renewed certificate files replace or update the stored files.
- Expiry is updated after renew.
- Linked Kong targets are synced after successful renew.
- Failures are visible in job logs.

## Files Likely Affected
- `internal/usecase/acme.go`
- `internal/usecase/certificate.go`
- `internal/usecase/kong_sync.go`
- `internal/usecase/job.go`
- `internal/adapter/acme/lego_client.go`
- `internal/adapter/kong/admin_client.go`
- `internal/web/handler.go`
- `internal/web/routes.go`

## Test Checklist
- [x] Run `go test ./...`.
- [x] Renew against Let's Encrypt staging.
- [x] Verify expiry changes or remains valid.
- [x] Verify sync is attempted after renew.
- [x] Verify failure logs.

## Outcome
Implemented manual certificate renewal through `POST /certificates/{id}/renew`. Renew reads the existing certificate and private key, renews through the ACME client, writes updated certificate files, parses and stores the renewed expiry, records a renew job, and syncs linked Kong targets after successful renew. Failure paths mark the certificate failed and create clear renew job logs.

## Completion Evidence
- `go test ./...` passes.
- `go test ./internal/usecase -run TestLiveStagingRenewCertificateWithCloudflareDNS01 -v` passed against Let's Encrypt staging on 2026-05-01 for `sandbox2.rtt.in.th` and `*.sandbox2.rtt.in.th`.
- `TestACMEUseCaseRenewCertificateStoresFilesAndSyncsLinkedTargets` covers renewed file writes, expiry storage, renew job logging, and post-renew Kong sync.
- `TestRenewCertificateWithoutCloudflareTokenMarksCertificateFailedAndCreatesFailedJob` covers renew failure logging and failed certificate status.

## Completed At
2026-05-01
