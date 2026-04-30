# Project Status

## Current Phase

Phase 1 - Foundation.

Certificate CRUD UI, Kong target CRUD UI, job log UI, Kong connectivity testing, certificate sync, manual ACME issue, manual renew, auto renew scheduler, and Docker Compose hardening are implemented. ACME issue and renew are verified against Let's Encrypt staging.

## Overall Progress

```text
MVP progress: 12 / 12 feature tasks done, plus 1 refactor task done
Current status: MVP feature tasks completed
```

## Current Priorities

1. Review production deployment settings before first real production run.
2. Keep `LETSENCRYPT_ENV=staging` until the deployment is verified end to end.
3. Rotate and store operational secrets outside committed files.

## Blockers

No blockers recorded.

## Epic Summary

| Epic | Summary | Status |
|------|---------|--------|
| epic-01-foundation | Go app, config, HTTP server, Docker base | in progress |
| epic-02-data-ui | SQLite schema, Bootstrap layout, CRUD screens | in progress |
| epic-03-kong | Kong target testing and certificate sync | in progress |
| epic-04-acme | Let's Encrypt and Cloudflare DNS-01 integration | in progress |
| epic-05-renewal | Manual renew, auto renew, and hardening | done |

## Task Index

### Current Status Snapshot

- `✅ done`: task-001-project-scaffold, task-002-database-schema, task-003-bootstrap-layout, task-004-certificate-crud-ui, task-004.5-migrate-web-adapter-to-fiber, task-005-kong-target-crud-ui, task-006-job-log-ui, task-007-kong-target-test, task-008-kong-certificate-sync, task-009-acme-certificate-issue, task-010-manual-renew, task-011-auto-renew-scheduler, task-012-docker-compose-hardening
- `🚧 in progress`: none
- `🎯 next up`: none
- `⛔ blocked`: none

### In Progress

- None

### Ready To Start

- None

### Backlog

- None

### Phase 1 - Foundation

| Task | Area | Priority | Status |
|------|------|----------|--------|
| task-001-project-scaffold | infra/backend | high | ✅ done |
| task-002-database-schema | backend | high | ✅ done |
| task-003-bootstrap-layout | frontend | high | ✅ done |

### Phase 2 - UI and CRUD

| Task | Area | Priority | Status |
|------|------|----------|--------|
| task-004-certificate-crud-ui | frontend/backend | high | ✅ done |
| task-004.5-migrate-web-adapter-to-fiber | frontend/backend | high | ✅ done |
| task-005-kong-target-crud-ui | frontend/backend | high | ✅ done |
| task-006-job-log-ui | frontend/backend | medium | ✅ done |

### Phase 3 - Kong Integration

| Task | Area | Priority | Status |
|------|------|----------|--------|
| task-007-kong-target-test | backend | high | ✅ done |
| task-008-kong-certificate-sync | backend | high | ✅ done |

### Phase 4 - ACME Integration

| Task | Area | Priority | Status |
|------|------|----------|--------|
| task-009-acme-certificate-issue | backend | high | ✅ done |
| task-010-manual-renew | backend | high | ✅ done |

### Phase 5 - Renewal and Hardening

| Task | Area | Priority | Status |
|------|------|----------|--------|
| task-011-auto-renew-scheduler | backend | high | ✅ done |
| task-012-docker-compose-hardening | infra/docs | medium | ✅ done |

## Definition of Done

A task is done only when:

- The deliverable in the task file is implemented.
- Acceptance criteria are verified.
- Relevant tests or commands have been run, or the verification gap is documented.
- The task file is updated with outcome, completion evidence, and completed date.
- `.breakdown/STATUS.md` reflects the new status.

## Working Rules

- Use `docs/ARCHITECTURE.md` as the source of truth for MVP scope.
- Use `.breakdown/STATUS.md` as the source of truth for current progress.
- Use one `.breakdown/task-*.md` file as the scope boundary for execution.
- Work on one task at a time.
- Keep tasks small enough to finish in one focused session.
- Do not expand beyond MVP scope unless `docs/ARCHITECTURE.md` is intentionally updated.
- Preserve acceptance criteria when closing a task.
