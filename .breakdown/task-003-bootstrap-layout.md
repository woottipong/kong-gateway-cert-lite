# Task: Bootstrap layout

## Task ID
task-003-bootstrap-layout

## Epic
epic-01-foundation

## Area
frontend

## Status
done

## Priority
high

## Depends On
- task-001-project-scaffold

## Summary
Create the base server-rendered HTML layout using Bootstrap 5.

## Scope
- Add local Bootstrap 5 CSS and JS assets.
- Add base layout template with sidebar and main content area.
- Serve static assets from the Go app.
- Add simple placeholder pages for main navigation.
- Keep custom CSS minimal.

## Out of Scope
- Full CRUD forms.
- Dynamic database-backed pages.
- HTMX behavior.

## Acceptance Criteria
- App serves Bootstrap CSS and JS locally.
- Main pages render with a consistent sidebar layout.
- Navigation includes Certificates, Kong Targets, Jobs / Logs, and Settings.
- Layout works without Node.js or a frontend build step.

## Files Likely Affected
- `internal/web/templates/layout.html`
- `internal/web/templates/*.html`
- `internal/web/static/bootstrap/*`
- `internal/web/static/css/app.css`
- `internal/web/routes.go`

## Test Checklist
- [x] Run `go test ./...`.
- [x] Start app locally.
- [x] Open/render pages through local HTTP routes.
- [x] Confirm static assets return HTTP 200.

## Outcome
Bootstrap 5 layout foundation is implemented. The app now serves local Bootstrap CSS/JS assets, renders a shared sidebar layout, and exposes placeholder pages for Certificates, Kong Targets, Jobs / Logs, and Settings.

## Completion Evidence
- `go test ./...` passed.
- `APP_ADDR=127.0.0.1:18082 APP_DB_PATH=/var/folders/tx/kzqlp8fs029gxj9q91vfxc780000gp/T/tmp.OjdFhfY0jD/app.db go run ./cmd/kong-cert-lite` started successfully.
- `/certificates`, `/kong-targets`, `/jobs`, and `/settings` returned HTTP 200.
- `/static/bootstrap/bootstrap.min.css`, `/static/bootstrap/bootstrap.bundle.min.js`, and `/static/css/app.css` returned HTTP 200.
- `/certificates` rendered the shared sidebar navigation and local Bootstrap asset links.

## Completed At
2026-04-26 22:20 Asia/Bangkok
