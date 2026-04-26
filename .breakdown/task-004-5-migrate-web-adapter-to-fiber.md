# Task: Migrate web adapter to Fiber

## Task ID
task-004.5-migrate-web-adapter-to-fiber

## Epic
epic-02-data-ui

## Area
frontend / backend

## Status
done

## Priority
high

## Depends On
- task-004-certificate-crud-ui

## Summary
Replace the `net/http` web adapter with Go Fiber while preserving existing routes, templates, static assets, and certificate CRUD behavior.

## Scope
- Add Fiber dependency.
- Migrate web routes to Fiber.
- Migrate handlers to `*fiber.Ctx`.
- Serve embedded static assets through Fiber middleware.
- Update application startup and graceful shutdown.
- Update web tests to use `app.Test(req)`.

## Out of Scope
- Changing domain or usecase behavior.
- Replacing templates.
- Adding new CRUD features.
- Starting task-005 behavior.

## Acceptance Criteria
- Existing routes keep the same paths and status behavior.
- Static assets still return HTTP 200.
- Certificate create/list/detail flow still works.
- Validation errors still return HTTP 422.
- `go test ./...` passes.

## Files Likely Affected
- `go.mod`
- `go.sum`
- `cmd/kong-cert-lite/main.go`
- `internal/app/app.go`
- `internal/web/routes.go`
- `internal/web/handler.go`
- `internal/web/handler_test.go`

## Test Checklist
- [x] Run `go test ./...`.
- [x] Verify health route.
- [x] Verify static assets.
- [x] Verify certificate create/list/detail.
- [x] Verify validation errors.

## Outcome
The web adapter now uses Go Fiber. Clean Architecture boundaries are preserved: Fiber remains in `internal/web`, while business logic stays in `internal/usecase`.

## Completion Evidence
- `go test ./...` passed.
- Web tests now use Fiber `app.Test(req)`.
- Existing certificate CRUD tests pass without behavior changes.

## Completed At
2026-04-26 23:00 Asia/Bangkok
