# Task: ACME certificate issue

## Task ID
task-009-acme-certificate-issue

## Epic
epic-04-acme

## Area
backend

## Status
done

## Priority
high

## Depends On
- task-004-certificate-crud-ui
- task-006-job-log-ui

## Summary
Implement Let's Encrypt certificate issuance with lego and Cloudflare DNS-01.

## Scope
- Add `lego` integration.
- Load Cloudflare token from `CF_DNS_API_TOKEN`.
- Create or load ACME account under `APP_ACCOUNT_DIR`.
- Issue certificates for configured domains.
- Save fullchain and private key under `APP_CERT_DIR`.
- Parse and store expiry.
- Update certificate status and issue job logs.

## Out of Scope
- Manual renew.
- Auto renew.
- Multi-provider DNS.
- Multiple Cloudflare tokens.

## Acceptance Criteria
- Issue flow works against Let's Encrypt staging.
- Certificate files are saved under the configured cert directory.
- Expiry is parsed and stored.
- Certificate status changes from pending to active on success.
- Failure creates a clear job log and marks certificate failed.

## Files Likely Affected
- `internal/domain/certificate.go`
- `internal/usecase/acme.go`
- `internal/usecase/certificate.go`
- `internal/usecase/job.go`
- `internal/adapter/acme/lego_client.go`
- `internal/adapter/sqlite/certificate_repository.go`
- `internal/adapter/sqlite/job_repository.go`
- `internal/app/config.go`
- `internal/web/handler.go`
- `internal/web/routes.go`

## Test Checklist
- [x] Run `go test ./...`.
- [x] Add opt-in live staging verification test.
- [x] Run issue flow with `LETSENCRYPT_ENV=staging`.
- [x] Verify cert/key files exist.
- [x] Verify expiry in SQLite.
- [x] Verify failure behavior with missing Cloudflare token.

## Outcome
Implemented the ACME issue workflow in code: added `lego` + Cloudflare DNS-01 integration, ACME account persistence under `APP_ACCOUNT_DIR`, certificate/key file persistence under `APP_CERT_DIR`, expiry parsing/storage, issue job logging, and the `/certificates/:id/issue` web action. The UI now keeps issue and Kong sync as separate operator actions for clarity.

Added an opt-in live staging verification test that exercises the full use case with a real Cloudflare DNS-01 challenge and verifies saved files, stored expiry, active certificate status, and success job logging. The live test is skipped unless `CF_DNS_API_TOKEN`, `ACME_LIVE_EMAIL`, and `ACME_LIVE_DOMAINS` are set.

Verified the live Let's Encrypt staging issue flow with Cloudflare DNS-01 for `sandbox2.rtt.in.th` and `*.sandbox2.rtt.in.th`.

## Completion Evidence
- `go test ./...` passes.
- Web regression test covers missing `CF_DNS_API_TOKEN` and verifies failed job + failed certificate status.
- ACME use case test covers success path file writes, expiry parsing, active status, and success job logging.
- Live staging verification test exists at `internal/usecase/acme_live_test.go`.
- `go test ./internal/usecase -run TestLiveStagingIssueCertificateWithCloudflareDNS01 -v` passed against Let's Encrypt staging on 2026-05-01 for `sandbox2.rtt.in.th` and `*.sandbox2.rtt.in.th`.

## Completed At
2026-05-01
