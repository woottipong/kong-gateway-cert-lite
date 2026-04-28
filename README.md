# kong-cert-lite

Lightweight internal tool for managing TLS certificate metadata for Kong Gateway OSS.

The current implementation has:

- Go single binary application
- Go Fiber HTTP adapter
- SQLite database and embedded migrations
- Clean Architecture package layout
- Server-rendered HTML with Bootstrap 5
- Modern premium minimal light-first UI
- Certificate create, edit, delete, list, and detail flow
- Kong target create, edit, delete, list, and connectivity test flow
- Job list/detail pages backed by SQLite
- Linked Kong target selection per certificate
- Manual ACME issue flow through `lego` + Cloudflare DNS-01
- Manual certificate sync to linked Kong targets

Manual renew, auto renew, and scheduler work remain in later breakdown tasks.

## Quick Start

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
GET  /settings
```

## Configuration

Current supported environment variables:

```text
APP_ADDR           Default: :8080
APP_DB_PATH        Default: /data/app.db
APP_CERT_DIR       Default: /data/certs
APP_ACCOUNT_DIR    Default: /data/accounts
CF_DNS_API_TOKEN   Required for ACME issue
LETSENCRYPT_ENV    Default: staging
AUTO_RENEW_CRON
```

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
- Restrained green accent for primary actions and important status
- Plus Jakarta Sans and JetBrains Mono from Google Fonts
- Bootstrap 5 local static assets
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
