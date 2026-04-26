# Task: Project scaffold

## Task ID
task-001-project-scaffold

## Epic
epic-01-foundation

## Area
infra / backend

## Status
done

## Priority
high

## Depends On
- none

## Summary
Create the initial Go project scaffold with a runnable HTTP server, configuration loading, and health check endpoint.

## Scope
- Initialize the Go module.
- Create `cmd/kong-cert-lite/main.go`.
- Add basic app/config package.
- Read initial environment variables with defaults.
- Start an HTTP server on `APP_ADDR`.
- Add `GET /healthz` returning JSON status.

## Out of Scope
- Database schema.
- HTML templates.
- Kong integration.
- ACME integration.
- Docker image.

## Acceptance Criteria
- `go run ./cmd/kong-cert-lite` starts the server.
- `GET /healthz` returns HTTP 200.
- Health response includes `{"status":"ok"}` or equivalent JSON.
- `APP_ADDR` can override the listen address.
- Config errors fail startup with a clear message.

## Files Likely Affected
- `go.mod`
- `go.sum`
- `cmd/kong-cert-lite/main.go`
- `internal/app/app.go`
- `internal/app/config.go`
- `internal/web/routes.go`
- `internal/web/handler.go`

## Test Checklist
- [x] Run `go test ./...`.
- [x] Run the app locally.
- [x] Call `GET /healthz`.

## Outcome
Initial Go scaffold is implemented with a runnable HTTP server, environment-based config loading, and `GET /healthz`.

## Completion Evidence
- `go test ./...` passed.
- `APP_ADDR=127.0.0.1:18080 go run ./cmd/kong-cert-lite` started the server.
- `curl -i http://127.0.0.1:18080/healthz` returned HTTP 200 and `{"status":"ok"}`.
- `APP_ADDR= go run ./cmd/kong-cert-lite` failed startup with `APP_ADDR must not be empty`.

## Completed At
2026-04-26 18:34 Asia/Bangkok
