# AGENTS.md

Instructions for agents and contributors working in this repository.

## Project Context

`kong-cert-lite` is an internal Go tool for managing TLS certificate lifecycle metadata and, in later tasks, issuing certificates and syncing them to Kong Gateway OSS.

The project currently uses:

- Go 1.26
- Go Fiber for HTTP routing
- SQLite for persistence
- Server-rendered HTML templates
- Bootstrap 5 static assets
- Green Deck UI styling
- Clean Architecture package boundaries

## Source of Truth

Use these files in this order:

1. `docs/ARCHITECTURE.md` for system intent, MVP boundaries, stack, and architecture.
2. `.breakdown/STATUS.md` for current progress and next task.
3. The active `.breakdown/task-*.md` file for implementation scope.

Work on one task at a time.

## Architecture Rules

Respect the dependency direction:

```text
domain <- usecase <- web
domain <- adapter/sqlite
app wires everything together
```

Package responsibilities:

- `internal/domain`: entities and domain errors only.
- `internal/usecase`: business rules, validation, DTOs, and port interfaces.
- `internal/adapter/sqlite`: SQL queries and persistence mapping.
- `internal/db`: database opening and migrations.
- `internal/web`: Fiber handlers, routes, templates, and static files.
- `internal/app`: composition root.

Do not put SQL in `internal/web`.
Do not import Fiber from `internal/usecase` or `internal/domain`.
Do not make `internal/domain` depend on application, database, or HTTP packages.

## Current Development Flow

Before editing:

```bash
go test ./...
```

After editing:

```bash
gofmt -w cmd internal
go test ./...
```

Run locally:

```bash
APP_ADDR=127.0.0.1:8080 APP_DB_PATH=./data/app.db go run ./cmd/kong-cert-lite
```

Open:

```text
http://127.0.0.1:8080/certificates
```

Current implemented UI areas:

- Certificates list, detail, and create form
- Kong targets list, create form, and edit form
- Jobs list and detail
- Settings placeholder

## Fiber Web Adapter Rules

Use Fiber for web routes and handlers.

Current route style:

```go
app.Get("/certificates", handler.Certificates)
app.Post("/certificates", handler.CreateCertificate)
app.Get("/kong-targets", handler.KongTargets)
app.Post("/kong-targets", handler.CreateKongTarget)
app.Get("/jobs", handler.Jobs)
```

Use `*fiber.Ctx` only inside `internal/web`.
Use `c.UserContext()` when calling use cases.
Use `app.Test(req)` in web tests.

## UI Rules

Follow the Green Deck visual direction:

- Dark-first by default.
- Preserve the light/dark toggle.
- Use `#1DB954` only for interactive accents and important status.
- Keep pages dense enough for operational work.
- Avoid oversized hero-style headings.
- Do not add a frontend build step.
- Keep custom JavaScript small and local to `internal/web/static/js`.

Bootstrap assets are local and embedded. Do not use CDN for Bootstrap.

## Scope Control

Do not implement future tasks early.

Examples:

- While working on Kong target CRUD, do not implement Kong connectivity testing.
- While working on job log UI, do not implement ACME issue.
- While working on ACME issue, do not add notifications or webhooks.

If a change requires expanding scope, update the active task or ask first.

## Secrets

Never log or render secrets.

Current and planned sensitive values include:

- `CF_DNS_API_TOKEN`
- Kong custom auth header values
- certificate private keys

## Files to Avoid Committing

Local runtime files should not be committed:

```text
data/
*.db
*.db-shm
*.db-wal
```

If needed, add ignore rules before introducing generated runtime files.
