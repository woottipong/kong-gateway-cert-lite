# Task: Job log UI

## Task ID
task-006-job-log-ui

## Epic
epic-02-data-ui

## Area
frontend / backend

## Status
done

## Priority
medium

## Depends On
- task-002-database-schema
- task-003-bootstrap-layout

## Summary
Build job list and detail pages backed by the jobs table.

## Scope
- Add job service helpers for creating and updating jobs.
- List jobs ordered by latest first.
- Show job detail with full log text.
- Render status badges consistently.

## Out of Scope
- Real ACME job generation.
- Real Kong sync job generation.
- Log streaming.

## Acceptance Criteria
- Jobs page renders rows from the database.
- Job detail page shows status, timing, message, and full logs.
- Empty job list has a useful empty state.
- Status badge mapping follows the architecture doc.

## Files Likely Affected
- `internal/domain/job.go`
- `internal/usecase/job.go`
- `internal/adapter/sqlite/job_repository.go`
- `internal/web/handler.go`
- `internal/web/routes.go`
- `internal/web/templates/jobs.html`
- `internal/web/templates/job_detail.html`

## Test Checklist
- [x] Run `go test ./...`.
- [x] Insert or create sample jobs.
- [x] Verify list ordering.
- [x] Verify detail rendering.

## Outcome
Implemented job repository/usecase helpers, latest-first job list, job detail route, full log rendering, empty state, and status badge mapping for current job states.

## Completion Evidence
`go test ./...` passed. Smoke test against a local Fiber server verified `/jobs` empty and populated states plus `/jobs/1` detail with status, timing, message, and full logs.

## Completed At
2026-04-26
