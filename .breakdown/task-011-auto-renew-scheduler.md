# Task: Auto renew scheduler

## Task ID
task-011-auto-renew-scheduler

## Epic
epic-05-renewal

## Area
backend

## Status
planned

## Priority
high

## Depends On
- task-010-manual-renew

## Summary
Add scheduled auto renewal for certificates that are close to expiry.

## Scope
- Read `AUTO_RENEW_CRON`.
- Start internal scheduler with the app.
- Find certificates where `auto_renew = true`.
- Renew certificates when remaining days are less than or equal to `renew_before_days`.
- Sync after successful renew.
- Record scheduler job logs.

## Out of Scope
- Distributed locking.
- Multi-instance scheduling.
- Notifications.

## Acceptance Criteria
- Scheduler starts with the application.
- Invalid cron config fails startup with a clear message.
- Certificates outside the renew window are skipped.
- Certificates inside the renew window are renewed.
- Scheduler actions produce job logs.

## Files Likely Affected
- `internal/usecase/scheduler.go`
- `internal/usecase/certificate.go`
- `internal/usecase/job.go`
- `internal/app/app.go`
- `internal/app/config.go`

## Test Checklist
- [ ] Run `go test ./...`.
- [ ] Test renewal window logic.
- [ ] Test invalid cron config.
- [ ] Run scheduler with a short test cron or direct trigger.

## Outcome
Not started.

## Completion Evidence
Not completed.

## Completed At
Not completed.
