# Task: Database schema

## Task ID
task-002-database-schema

## Epic
epic-01-foundation

## Area
backend

## Status
done

## Priority
high

## Depends On
- task-001-project-scaffold

## Summary
Add SQLite database initialization and MVP schema migrations.

## Scope
- Add SQLite dependency.
- Open database from `APP_DB_PATH`.
- Create migration runner.
- Add tables for certificates, Kong targets, certificate target mappings, and jobs.
- Add indexes for common lookups.

## Out of Scope
- Full CRUD handlers.
- ACME integration.
- Kong Admin API integration.

## Acceptance Criteria
- App creates the SQLite database when it does not exist.
- Migrations run once and are safe to rerun.
- Required MVP tables exist after startup.
- `go test ./...` passes.

## Files Likely Affected
- `internal/db/db.go`
- `internal/db/migrations/*.sql`
- `internal/domain/*.go`
- `internal/app/app.go`

## Test Checklist
- [x] Run `go test ./...`.
- [x] Start app with a temporary `APP_DB_PATH`.
- [x] Inspect database schema.

## Outcome
SQLite initialization and MVP schema migrations are implemented. The app opens `APP_DB_PATH` during startup, creates the database file when missing, runs embedded migrations safely, and exposes model structs for the MVP tables.

## Completion Evidence
- `go test ./...` passed.
- `APP_ADDR=127.0.0.1:18081 APP_DB_PATH=/var/folders/tx/kzqlp8fs029gxj9q91vfxc780000gp/T/tmp.LmU5x3jFjY/app.db go run ./cmd/kong-cert-lite` started successfully.
- `curl -s -i http://127.0.0.1:18081/healthz` returned HTTP 200 and `{"status":"ok"}`.
- `sqlite3 .../app.db ".tables"` showed `certificates`, `kong_targets`, `certificate_kong_targets`, `jobs`, and `schema_migrations`.
- `schema_migrations` contains `001_initial.sql`.

## Completed At
2026-04-26 22:16 Asia/Bangkok
