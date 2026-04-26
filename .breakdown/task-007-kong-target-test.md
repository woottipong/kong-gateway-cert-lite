# Task: Kong target connectivity test

## Task ID
task-007-kong-target-test

## Epic
epic-03-kong

## Area
backend

## Status
done

## Priority
high

## Depends On
- task-005-kong-target-crud-ui
- task-006-job-log-ui

## Summary
Implement Kong Admin API connectivity testing for saved Kong targets.

## Scope
- Add Kong Admin API client.
- Support `none` and `custom-header` auth types.
- Add `POST /kong-targets/{id}/test`.
- Update target status to online or offline.
- Create a `test_kong` job with result details.

## Out of Scope
- Certificate sync.
- Kong route/service/plugin operations.

## Acceptance Criteria
- Testing an available Kong Admin API marks the target online.
- Testing an unavailable Kong Admin API marks the target offline.
- Failures store a clear error message.
- A job log is created for each test.

## Files Likely Affected
- `internal/domain/kong_target.go`
- `internal/usecase/kong_target.go`
- `internal/usecase/job.go`
- `internal/adapter/kong/admin_client.go`
- `internal/adapter/sqlite/kong_target_repository.go`
- `internal/adapter/sqlite/job_repository.go`
- `internal/web/handler.go`
- `internal/web/routes.go`

## Test Checklist
- [ ] Run `go test ./...`.
- [ ] Test against a reachable Kong Admin API or mock server.
- [ ] Test against an unreachable URL.
- [ ] Verify job logs.

## Outcome
Implemented Kong Admin API connectivity testing for saved Kong targets. The app now sends a real HTTP request to the configured Admin API endpoint, supports both `none` and `custom-header` auth modes, records a `test_kong` job for each run, and updates the target status to `online` or `offline` with a fresh `last_checked_at` timestamp.

## Completion Evidence
- Added a Kong Admin API client in `internal/adapter/kong/admin_client.go`.
- Added `POST /kong-targets/:id/test` and exposed a `Test` action from the Kong targets list.
- Reachable target tests now mark the target `online` and create a successful `test_kong` job.
- Unreachable target tests now mark the target `offline` and create a failed `test_kong` job with a clear error message.
- `gofmt -w internal` completed successfully.
- `go test ./...` passed.

## Completed At
2026-04-26
