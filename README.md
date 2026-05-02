# kong-cert-lite

Lightweight internal tool for managing TLS certificate metadata for Kong Gateway OSS.

The current implementation has:

- Go single binary application
- Go Fiber HTTP adapter
- SQLite database and embedded migrations
- Clean Architecture package layout
- Server-rendered HTML with Tabler on top of Bootstrap 5
- Modern premium minimal light-first UI
- Certificate create, edit, delete, list, and detail flow
- Kong target create, edit, delete, list, and connectivity test flow
- Job list/detail pages backed by SQLite
- Linked Kong target selection per certificate
- Manual ACME issue flow through `lego` + Cloudflare DNS-01
- Manual certificate renew flow
- Manual certificate sync to linked Kong targets
- Auto renew scheduler through `AUTO_RENEW_CRON`

## Quick Start

Production-like Docker Compose:

```bash
APP_USERNAME=operator \
APP_PASSWORD=change-me \
CF_DNS_API_TOKEN=your_token \
docker compose up --build
```

Docker dev with hot reload:

```bash
docker compose -f compose.dev.yaml up --build
```

Open:

```text
http://127.0.0.1:8080/certificates
```

Native Go:

```bash
APP_ADDR=127.0.0.1:8080 APP_DB_PATH=./data/app.db go run ./cmd/kong-cert-lite
```

Open:

```text
http://127.0.0.1:8080/certificates
```

Other current routes:

```text
GET  /healthz
GET  /certificates
GET  /certificates/new
POST /certificates
GET  /certificates/:id/edit
GET  /certificates/:id
POST /certificates/:id
POST /certificates/:id/issue
POST /certificates/:id/renew
POST /certificates/:id/delete
POST /certificates/:id/targets
POST /certificates/:id/sync
GET  /kong-targets
GET  /kong-targets/new
POST /kong-targets
GET  /kong-targets/:id/edit
POST /kong-targets/:id
POST /kong-targets/:id/delete
POST /kong-targets/:id/test
GET  /jobs
GET  /jobs/:id
```

## Configuration

Current supported environment variables:

```text
APP_ADDR           Default: :8080
APP_DB_PATH        Default: /data/app.db
APP_CERT_DIR       Default: /data/certs
APP_ACCOUNT_DIR    Default: /data/accounts
APP_USERNAME       Enables operator login when set with APP_PASSWORD
APP_PASSWORD       Enables operator login when set with APP_USERNAME
CF_DNS_API_TOKEN   Required for ACME issue and renew
LETSENCRYPT_ENV    Default: staging
AUTO_RENEW_CRON    Default: 0 3 * * *
```

`APP_USERNAME` and `APP_PASSWORD` must be set together. When both are set, all UI and action routes require the built-in login page. Successful login creates an HttpOnly session cookie that expires after 12 hours. `/healthz` remains unauthenticated for container health checks.

`LETSENCRYPT_ENV` must be `staging` or `production`. Keep `staging` until a deployment has been tested end to end.

`AUTO_RENEW_CRON` uses a 5-field cron expression in UTC:

```text
minute hour day month weekday
```

The default `0 3 * * *` checks renewal windows every day at 03:00 UTC.

## Production Docker Compose

The production image is built from `Dockerfile` and runs as a non-root user on a distroless Debian runtime. Runtime state is mounted at `/data` through the `kong-cert-data` named volume.

```bash
APP_USERNAME=operator \
APP_PASSWORD=change-me \
CF_DNS_API_TOKEN=your_token \
LETSENCRYPT_ENV=staging \
AUTO_RENEW_CRON="0 3 * * *" \
docker compose up --build
```

Open:

```text
http://127.0.0.1:8080/certificates
```

Check health:

```bash
docker compose ps
docker compose exec app /healthcheck http://127.0.0.1:8080/healthz
```

Persistent paths inside the container:

```text
/data/app.db
/data/certs
/data/accounts
```

### Backup And Restore

Back up the named volume:

```bash
docker run --rm \
  -v kong-cert-lite_kong-cert-data:/data:ro \
  -v "$PWD:/backup" \
  busybox \
  tar czf /backup/kong-cert-data-backup.tgz -C /data .
```

Restore into a fresh named volume:

```bash
docker compose down
docker volume rm kong-cert-lite_kong-cert-data
docker volume create kong-cert-lite_kong-cert-data
docker run --rm \
  -v kong-cert-lite_kong-cert-data:/data \
  -v "$PWD:/backup:ro" \
  busybox \
  tar xzf /backup/kong-cert-data-backup.tgz -C /data
docker compose up --build
```

The backup includes the SQLite database, issued certificate files, private keys, and ACME account data. Treat backups as sensitive secrets.

### Secret Handling

- Provide `CF_DNS_API_TOKEN` through the environment, not in committed files.
- Kong custom header values are stored in SQLite for runtime use but are not rendered in the UI.
- Job logs include file paths and API status details, not certificate private key material or Kong custom header values.
- Restrict access to `/data` backups because they contain private keys and ACME account material.

## Development

### Docker Dev With Hot Reload

The dev container uses [Air](https://github.com/air-verse/air) to rebuild and restart the Go binary when files under `cmd/` or `internal/` change. Template, CSS, JavaScript, migration, and Go edits are watched.

```bash
docker compose -f compose.dev.yaml up --build
```

The container stores runtime state in the local `./data` directory:

```text
/data/app.db
/data/certs
/data/accounts
```

Use Cloudflare DNS-01 from Docker dev:

```bash
CF_DNS_API_TOKEN=your_token \
LETSENCRYPT_ENV=staging \
docker compose -f compose.dev.yaml up --build
```

Reset dev runtime containers and build caches:

```bash
docker compose -f compose.dev.yaml down -v
```

Local app data remains under `./data`.

Run a one-off test command in the dev image:

```bash
docker compose -f compose.dev.yaml run --rm app go test ./...
```

### Native Go

Run tests:

```bash
go test ./...
```

Format code:

```bash
gofmt -w cmd internal
```

Use a local database:

```bash
mkdir -p data
APP_ADDR=127.0.0.1:8080 APP_DB_PATH=./data/app.db go run ./cmd/kong-cert-lite
```

Issue certificates locally with Cloudflare DNS-01:

```bash
CF_DNS_API_TOKEN=your_token \
LETSENCRYPT_ENV=staging \
APP_ADDR=127.0.0.1:8080 \
APP_DB_PATH=./data/app.db \
APP_CERT_DIR=./data/certs \
APP_ACCOUNT_DIR=./data/accounts \
go run ./cmd/kong-cert-lite
```

Inspect SQLite:

```bash
sqlite3 ./data/app.db ".tables"
```

## Architecture

The app follows a small Clean Architecture layout. Dependencies point inward:

```text
cmd -> app -> web / adapter / usecase
web -> usecase
adapter/sqlite -> domain
usecase -> domain
domain -> no application dependencies
```

Package responsibilities:

```text
internal/domain
  Entities and domain errors.

internal/usecase
  Application business rules, validation, DTOs, and repository ports.

internal/adapter/sqlite
  SQLite repository implementations and row mapping.

internal/db
  SQLite connection and embedded migrations.

internal/web
  Fiber routes, handlers, templates, static assets.

internal/app
  Composition root. Wires config, DB, repositories, use cases, and Fiber app.
```

Full architecture notes live in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## UI

The UI uses a modern premium minimal design direction:

- Light-first operational console
- Tabler compiled CSS and JS served locally
- Restrained green accent for primary actions and important status
- Small JavaScript only for focused interaction polish

Static assets are embedded from:

```text
internal/web/static/
```

Templates are embedded from:

```text
internal/web/templates/
```

## Project Breakdown

Planning and task execution are tracked in:

```text
.breakdown/STATUS.md
.breakdown/task-*.md
```

Current next task:

```text
task-009-acme-certificate-issue verification
```

## Current MVP Status

Done:

- Project scaffold
- SQLite schema and migrations
- Bootstrap layout
- Certificate CRUD UI
- Fiber web adapter migration
- Kong target CRUD UI
- Job log UI
- Kong target connectivity test
- Certificate sync to linked Kong targets
- Manual ACME issue flow

Next:

- Verify live ACME issue flow and then implement manual renew

Not implemented yet:

- Manual renew
- Auto renew scheduler
- Docker Compose hardening
