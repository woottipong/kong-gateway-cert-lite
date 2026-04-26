# Task: Kong certificate sync

## Task ID
task-008-kong-certificate-sync

## Epic
epic-03-kong

## Area
backend

## Status
done

## Priority
high

## Depends On
- task-004-certificate-crud-ui
- task-005-kong-target-crud-ui
- task-007-kong-target-test

## Summary
Implement certificate and SNI sync from local certificate files into Kong targets.

## Scope
- Read certificate and private key files from stored paths.
- Create Kong certificate entity when `kong_certificate_id` is missing.
- Update Kong certificate entity when `kong_certificate_id` exists.
- Create or update SNI values for the certificate.
- Update per-target sync status, last synced time, and last error.
- Create sync jobs.

## Out of Scope
- Issuing real certificates.
- Renewing certificates.
- Managing Kong services, routes, or plugins.

## Acceptance Criteria
- Sync creates a Kong certificate when no Kong certificate id exists.
- Sync updates an existing Kong certificate when an id exists.
- SNI values from certificate metadata are applied.
- Per-target success and failure are persisted.
- Missing cert/key files fail with clear job logs.

## Files Likely Affected
- `internal/domain/certificate.go`
- `internal/domain/kong_target.go`
- `internal/usecase/kong_sync.go`
- `internal/usecase/certificate.go`
- `internal/usecase/job.go`
- `internal/adapter/kong/admin_client.go`
- `internal/adapter/sqlite/certificate_repository.go`
- `internal/adapter/sqlite/job_repository.go`
- `internal/web/handler.go`
- `internal/web/routes.go`

## Test Checklist
- [x] Run `go test ./...`.
- [x] Sync against Kong or mock Kong Admin API.
- [x] Verify certificate mapping is stored.
- [x] Verify SNI values are sent.
- [x] Verify failure logs for missing files.

## Outcome
Implemented certificate sync from local certificate files into linked Kong targets selected from the certificate detail page. The application now reads stored cert/key files, creates or updates Kong certificate entities, sends certificate SNI values plus Kong tags in the sync payload, records per-target sync results in `certificate_kong_targets`, and creates per-target sync jobs for both successful and failed runs.

## Completion Evidence
- Added `internal/usecase/kong_sync.go` to orchestrate certificate sync across Kong targets.
- Extended `internal/adapter/kong/admin_client.go` to create and update Kong certificate entities through the Admin API, handle large JSON responses safely, and avoid logging returned secret material.
- Extended `internal/adapter/sqlite/certificate_repository.go` to persist per-target sync mappings and error state.
- Added linked Kong target selection on the certificate detail page and persisted the selection in `certificate_kong_targets`.
- Added `POST /certificates/:id/sync` and a `Sync to Kong` action that syncs only the linked Kong targets for that certificate.
- Web tests cover create-sync, update-sync, large-response handling, and missing-file failure flows against a mock Kong Admin API.
- `go test ./...` passed.

## Completed At
2026-04-26
