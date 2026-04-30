# Task: Auto renew scheduler

## Task ID
task-011-auto-renew-scheduler

## Epic
epic-05-renewal

## Area
backend

## Status
done

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
- [x] Run `go test ./...`.
- [x] Test renewal window logic.
- [x] Test invalid cron config.
- [x] Run scheduler with a short test cron or direct trigger.

## Outcome
Implemented the auto renew scheduler. The app now reads and validates `AUTO_RENEW_CRON`, starts an internal renewal scheduler with the application, filters auto-renew certificates by renewal window, and calls the verified manual renew path so renewed certificates are synced to linked Kong targets and renew/sync job logs are recorded.

## Completion Evidence
- `go test ./...` passes.
- `TestRenewalSchedulerRunOnceRenewsOnlyCertificatesInsideWindow` covers direct scheduler trigger and renewal window selection.
- `TestParseCronExpressionRejectsInvalidConfig` and `TestConfigValidateRejectsInvalidAutoRenewCron` cover invalid cron handling.
- Manual renew tests from task 010 cover renewed file writes, expiry updates, post-renew sync, and renew failure logs used by scheduler-triggered renewals.

## Completed At
2026-05-01
