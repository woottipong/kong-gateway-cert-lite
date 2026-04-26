# Task: Manual renew

## Task ID
task-010-manual-renew

## Epic
epic-04-acme

## Area
backend

## Status
planned

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
- [ ] Run `go test ./...`.
- [ ] Renew against Let's Encrypt staging.
- [ ] Verify expiry changes or remains valid.
- [ ] Verify sync is attempted after renew.
- [ ] Verify failure logs.

## Outcome
Not started.

## Completion Evidence
Not completed.

## Completed At
Not completed.
