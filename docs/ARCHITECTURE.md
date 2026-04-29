# ARCHITECTURE

## Overview

`kong-cert-lite` is a lightweight internal tool for managing TLS certificates for Kong Gateway OSS.

The application issues and renews Let's Encrypt certificates, stores certificate metadata in SQLite, stores certificate files on a local volume, and syncs certificates plus SNI values into one or more Kong Gateway targets through the Kong Admin API.

The product is intended to run as a single Go binary in one Docker container. The HTTP adapter uses Go Fiber. The UI is server-rendered HTML styled with Tabler, compiled Bootstrap 5 assets, and a modern premium minimal light-first design direction.

## Goals

- Run as one Go application with no separate frontend build pipeline.
- Use SQLite for metadata and job logs.
- Store runtime data under `/data` for simple backup and restore.
- Issue and renew Let's Encrypt certificates with `lego`.
- Support Cloudflare DNS-01 challenge for wildcard certificates.
- Manage multiple Kong Gateway targets.
- Sync one certificate to multiple Kong targets.
- Provide a simple web UI for certificates, Kong targets, and job logs.
- Support manual issue, manual renew, manual sync, and scheduled auto renew.
- Deploy with Docker or Docker Compose.

## Non-Goals

- Do not replace Kong Gateway.
- Do not manage Kong services, routes, plugins, or consumers.
- Do not implement user management or RBAC in the MVP.
- Do not support multiple DNS providers in the MVP.
- Do not support multiple Cloudflare accounts in the MVP.
- Do not expose the application publicly without a protective layer such as VPN, Cloudflare Access, or reverse proxy auth.
- Do not create a heavy analytics dashboard.
- Do not build a Kubernetes controller.

## Stack

```text
Language:        Go
Database:        SQLite
ACME Client:     lego
DNS Challenge:   Cloudflare DNS-01
HTTP Adapter:    Go Fiber
Web UI:          Server-rendered HTML
UI Framework:    Tabler (Bootstrap 5-based)
Design System:   Light-first premium minimal
Interactivity:   HTMX or small JavaScript only when needed
Scheduler:       Internal cron scheduler
Deployment:      Docker / Docker Compose
Dev Runtime:     Docker Compose + Air hot reload
Storage:         Local volume /data
```

## Components

The code follows a small Clean Architecture layout. Dependencies point inward:

```text
cmd -> app -> web / adapter / usecase
web -> usecase
adapter/sqlite -> domain
usecase -> domain
domain -> no application dependencies
```

Current package responsibilities:

```text
internal/domain
  Enterprise entities and shared domain errors.

internal/usecase
  Application business rules, validation, DTOs, and repository ports.

internal/adapter/sqlite
  SQLite repository implementations and row mapping.

internal/db
  Database connection and migrations.

internal/web
  Fiber routes, handlers, templates, and static assets.

internal/app
  Composition root for wiring config, database, repositories, use cases, and HTTP routes.
```

### Web UI

Server-rendered pages for:

- Certificates list, detail, and form
- Kong targets list and form
- Jobs list and detail
- Settings and system information

Current implemented routes:

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

Status badge tones:

```text
active      -> success
success     -> success
synced      -> success
online      -> success
warning     -> warning
running     -> warning
expired     -> danger
failed      -> danger
offline     -> danger
pending     -> secondary
not_synced  -> secondary
unknown     -> secondary
```

### Development Runtime

Local Docker development uses `compose.dev.yaml` and `Dockerfile.dev`.

- The dev image is based on the Go toolchain image and installs Air for hot reload.
- Source is bind-mounted at `/app`, so changes to Go code, templates, CSS, JavaScript, and migrations trigger rebuilds.
- Runtime data is bind-mounted from `./data` to `/data` so local SQLite, certificates, and ACME accounts are shared with native Go runs.
- Go module and build caches use named volumes to avoid repeated dependency downloads.
- The app uses the default `:8080` bind address in Docker dev and maps port `8080` to the host.

The development container is intentionally separate from future production hardening. Production Docker Compose concerns remain in the later hardening task.

### Certificate Use Case

Owns certificate application logic:

- Validate domains, SNI values, email, and renew settings
- Create and update certificate metadata
- Calculate remaining days and status
- Coordinate issue, renew, and sync workflows

### SQLite Adapters

Own persistence details:

- Implement repository ports defined by use cases
- Map database rows to domain entities
- Keep SQL and JSON storage details out of use cases

### ACME Service

Owns Let's Encrypt integration:

- Create or load ACME account data
- Perform Cloudflare DNS-01 challenge
- Issue certificates
- Renew certificates
- Save certificate and private key files
- Parse certificate expiry

### Kong Sync Service

Owns Kong Admin API integration:

- Test Kong target connectivity
- Create or update Kong certificate entities
- Create or update SNI values
- Attach source and wildcard tags to Kong certificate entities
- Track `kong_certificate_id` per Kong target
- Store per-target sync status and errors

### Scheduler

Owns automatic renewal:

- Runs on `AUTO_RENEW_CRON`
- Finds certificates with `auto_renew = true`
- Renews certificates when remaining days are less than or equal to `renew_before_days`
- Syncs renewed certificates to linked Kong targets

### Job Log Service

Owns execution history:

- Records issue, renew, sync, test, and delete jobs
- Stores status, short messages, and detailed logs
- Provides enough context to debug failures

## Flow

### Issue Certificate

```text
User submits certificate form
  -> validate input
  -> create certificate record as pending
User starts manual issue
  -> create issue job
  -> ACME DNS-01 challenge through Cloudflare
  -> Let's Encrypt issues certificate
  -> save cert/key under /data/certs
  -> update expires_at and status
  -> update issue job logs
```

### Renew Certificate

```text
User or scheduler starts renew
  -> create renew job
  -> ACME renews certificate
  -> save new cert/key files
  -> update expires_at and status
  -> sync to linked Kong targets
  -> update job logs and sync status
```

### Sync Certificate

```text
User starts sync
  -> read cert/key files
  -> create or update Kong certificate entity
  -> attach source and wildcard tags
  -> create or update SNI values
  -> save kong_certificate_id
  -> update sync status per target
  -> write job logs
```

## Assumptions

- The app is used as an internal tool, not a public SaaS product.
- The first supported DNS provider is Cloudflare only.
- Cloudflare API token is provided through `CF_DNS_API_TOKEN`.
- Kong Admin API is reachable from the app over a private network.
- Tabler and Bootstrap 5 assets are served locally by the Go app or embedded in the binary.
- The UI is light-first with no theme toggle in the MVP.
- MVP can start with simple POST forms for state-changing actions.
- SQLite is sufficient for MVP scale.
- `/data` is mounted as a persistent Docker volume.
