# Task: Docker Compose and hardening

## Task ID
task-012-docker-compose-hardening

## Epic
epic-05-renewal

## Area
infra / docs

## Status
planned

## Priority
medium

## Depends On
- task-011-auto-renew-scheduler

## Summary
Add production-ready Docker packaging, Docker Compose example, and final MVP hardening checks.

## Scope
- Add Dockerfile.
- Add Docker Compose example with `/data` volume.
- Document required environment variables.
- Confirm health check behavior.
- Review secret handling for Cloudflare token and Kong custom header.
- Add backup/restore notes.

## Out of Scope
- CI/CD pipeline.
- Kubernetes manifests.
- Public authentication system.

## Acceptance Criteria
- Docker image builds successfully.
- App runs through Docker Compose.
- `/data` is mounted as persistent storage.
- Required env vars are documented.
- Backup/restore notes cover database, certs, and accounts.
- Secrets are not logged or rendered in UI.

## Files Likely Affected
- `Dockerfile`
- `docker-compose.yml`
- `.dockerignore`
- `README.md`
- `docs/ARCHITECTURE.md`
- `internal/app/config.go`

## Test Checklist
- [ ] Run `docker build`.
- [ ] Run `docker compose up`.
- [ ] Call `/healthz` from the container.
- [ ] Verify `/data` volume paths.
- [ ] Review logs for secret leakage.

## Outcome
Not started.

## Completion Evidence
Not completed.

## Completed At
Not completed.
