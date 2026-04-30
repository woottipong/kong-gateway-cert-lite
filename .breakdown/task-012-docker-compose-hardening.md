# Task: Docker Compose and hardening

## Task ID
task-012-docker-compose-hardening

## Epic
epic-05-renewal

## Area
infra / docs

## Status
done

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
- [x] Run `docker build`.
- [x] Run `docker compose up`.
- [x] Call `/healthz` from the container.
- [x] Verify `/data` volume paths.
- [x] Review logs for secret leakage.

## Outcome
Implemented production Docker packaging and Compose hardening. The production image now builds from a multi-stage Dockerfile, runs as a non-root distroless container, includes an internal healthcheck binary, and persists runtime state through the `/data` named volume. README and architecture docs now cover production Compose usage, required environment variables, health checks, backup/restore, and secret handling.

## Completion Evidence
- `go test ./...`
- `docker build --check .`
- `CF_DNS_API_TOKEN=dummy docker compose config --quiet`
- `docker build --pull --progress=plain -t kong-cert-lite:local .`
- `CF_DNS_API_TOKEN=dummy LETSENCRYPT_ENV=staging docker compose up -d --build`
- `CF_DNS_API_TOKEN=dummy docker compose exec -T app /healthcheck http://127.0.0.1:8080/healthz`
- `docker run --rm -v kong-cert-lite_kong-cert-data:/data:ro busybox find /data -maxdepth 2 -type f -o -type d`
- `docker history --no-trunc kong-cert-lite:local | grep -iE 'password|secret|token|key' || true`
- `CF_DNS_API_TOKEN=dummy docker compose logs --no-color app | grep -i dummy || true`
- `docker scout quickview kong-cert-lite:local`
- `docker scout cves kong-cert-lite:local --only-severity critical,high`

## Completed At
2026-05-01
