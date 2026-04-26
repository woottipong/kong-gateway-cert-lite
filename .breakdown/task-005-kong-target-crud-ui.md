# Task: Kong target CRUD UI

## Task ID
task-005-kong-target-crud-ui

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
Build Kong target list and create/edit forms backed by SQLite.

## Scope
- List Kong targets.
- Add Kong target form.
- Edit Kong target form.
- Validate name, environment, admin URL, auth type, and optional custom header fields.
- Store auth header values without displaying them back in plain text.

## Out of Scope
- Testing Kong connectivity.
- Syncing certificates.
- Secret encryption at rest.

## Acceptance Criteria
- User can create and edit Kong targets.
- Invalid admin URL or auth settings return clear validation errors.
- Kong target list displays status and key metadata.
- Custom header value is not rendered back in plain text.

## Files Likely Affected
- `internal/domain/kong_target.go`
- `internal/usecase/kong_target.go`
- `internal/adapter/sqlite/kong_target_repository.go`
- `internal/web/handler.go`
- `internal/web/routes.go`
- `internal/web/templates/kong_targets.html`
- `internal/web/templates/kong_target_form.html`

## Test Checklist
- [x] Run `go test ./...`.
- [x] Create Kong target through UI.
- [x] Edit Kong target through UI.
- [x] Verify validation errors.
- [x] Verify secret value is not displayed.

## Outcome
Implemented Kong target list, create form, edit form, validation, SQLite persistence, and secret-safe rendering.

## Completion Evidence
`go test ./...` passed. Smoke test against a local Fiber server verified `/kong-targets`, `/kong-targets/new`, create redirect, edit page, validation 422, and no plain-text secret rendering.

## Completed At
2026-04-26
