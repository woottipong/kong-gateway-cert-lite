# Task: Certificate CRUD UI

## Task ID
task-004-certificate-crud-ui

## Epic
epic-02-data-ui

## Area
frontend / backend

## Status
done

## Priority
high

## Depends On
- task-002-database-schema
- task-003-bootstrap-layout

## Summary
Build certificate list, detail, and create form backed by SQLite metadata.

## Scope
- List certificates from the database.
- Show certificate detail page.
- Add certificate create form.
- Validate name, email, domains, SNI, auto renew, and renew before days.
- Store certificate metadata as pending before real ACME issue is implemented.

## Out of Scope
- Actual Let's Encrypt issue.
- Renew.
- Kong sync.
- Delete confirmation modal unless needed for basic flow.

## Acceptance Criteria
- User can create a certificate metadata record.
- Invalid form input returns clear validation errors.
- Certificates list displays created records.
- Detail page displays domains, SNI values, status, and renew settings.

## Files Likely Affected
- `internal/domain/certificate.go`
- `internal/usecase/certificate.go`
- `internal/adapter/sqlite/certificate_repository.go`
- `internal/web/handler.go`
- `internal/web/routes.go`
- `internal/web/templates/certificates.html`
- `internal/web/templates/certificate_detail.html`
- `internal/web/templates/certificate_form.html`

## Test Checklist
- [x] Run `go test ./...`.
- [x] Create certificate through UI.
- [x] Verify record exists in SQLite.
- [x] Verify validation errors in UI.

## Outcome
Certificate metadata CRUD UI is implemented for the MVP scope. Users can create pending certificate records, see them in the certificate list, open detail pages, and receive field-level validation errors for invalid input.

## Completion Evidence
- `go test ./...` passed.
- `POST /certificates` with valid form data returned `303 See Other` to `/certificates/1`.
- `/certificates` rendered the created certificate row.
- `/certificates/1` rendered domains, SNI values, status, email, auto renew, and renew-before settings.
- SQLite query confirmed the created row in `certificates`.
- Invalid form submit returned HTTP `422` with validation messages for name, email, domains, SNI, and renew-before days.

## Completed At
2026-04-26 22:34 Asia/Bangkok
