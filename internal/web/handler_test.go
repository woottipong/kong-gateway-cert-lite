package web

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	acmeadapter "kong-cert-lite/internal/adapter/acme"
	kongadapter "kong-cert-lite/internal/adapter/kong"
	sqliteadapter "kong-cert-lite/internal/adapter/sqlite"
	"kong-cert-lite/internal/db"
	"kong-cert-lite/internal/usecase"

	"github.com/gofiber/fiber/v2"
)

func TestHealthz(t *testing.T) {
	app := testApp(t)
	resp, body := doRequest(t, app, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if contentType := resp.Header.Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	if strings.TrimSpace(body) != `{"status":"ok"}` {
		t.Fatalf("unexpected body %q", body)
	}
}

func TestPlaceholderPagesRenderLayout(t *testing.T) {
	tests := []struct {
		path       string
		wantTitle  string
		wantActive string
	}{
		{path: "/certificates", wantTitle: "Certificates", wantActive: "Certificates"},
		{path: "/jobs", wantTitle: "Jobs and logs", wantActive: "Jobs / Logs"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			app := testApp(t)
			resp, body := doRequest(t, app, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if resp.StatusCode != fiber.StatusOK {
				t.Fatalf("expected status 200, got %d", resp.StatusCode)
			}
			for _, want := range []string{
				`data-bs-theme="dark"`,
				`class="navbar navbar-vertical navbar-expand-lg d-print-none app-sidebar"`,
				`class="brand-logo"`,
				`class="page-header d-print-none"`,
				`class="card"`,
				`href="/static/tabler/tabler.min.css"`,
				`src="/static/tabler/tabler.min.js"`,
				`src="/static/js/actions.js"`,
				tt.wantTitle,
				tt.wantActive,
			} {
				if !strings.Contains(body, want) {
					t.Fatalf("expected body to contain %q", want)
				}
			}
		})
	}
}

func TestHomeRedirectsToCertificates(t *testing.T) {
	app := testApp(t)
	resp, _ := doRequest(t, app, httptest.NewRequest(http.MethodGet, "/", nil))

	if resp.StatusCode != fiber.StatusFound {
		t.Fatalf("expected status 302, got %d", resp.StatusCode)
	}
	if location := resp.Header.Get("Location"); location != "/certificates" {
		t.Fatalf("expected redirect to /certificates, got %q", location)
	}
}

func TestStaticAssets(t *testing.T) {
	tests := []string{
		"/static/tabler/tabler.min.css",
		"/static/tabler/tabler.min.js",
		"/static/css/app.css",
		"/static/js/actions.js",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			app := testApp(t)
			resp, body := doRequest(t, app, httptest.NewRequest(http.MethodGet, path, nil))

			if resp.StatusCode != fiber.StatusOK {
				t.Fatalf("expected status 200, got %d", resp.StatusCode)
			}
			if body == "" {
				t.Fatal("expected static asset body")
			}
		})
	}
}

func TestCreateCertificateAndRenderDetail(t *testing.T) {
	database, app := testServer(t)
	form := url.Values{
		"name":              {"Production wildcard"},
		"email":             {"admin@example.com"},
		"domains":           {"example.com\n*.example.com"},
		"snis":              {"example.com\n*.example.com"},
		"auto_renew":        {"on"},
		"renew_before_days": {"30"},
	}
	req := httptest.NewRequest(http.MethodPost, "/certificates", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, _ := doRequest(t, app, req)

	if resp.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", resp.StatusCode)
	}
	if location := resp.Header.Get("Location"); location != "/certificates/1" {
		t.Fatalf("expected detail redirect, got %q", location)
	}

	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM certificates WHERE name = ?", "Production wildcard").Scan(&count); err != nil {
		t.Fatalf("count certificates: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one certificate, got %d", count)
	}

	detailResp, detailBody := doRequest(t, app, httptest.NewRequest(http.MethodGet, "/certificates/1", nil))
	if detailResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected detail status 200, got %d", detailResp.StatusCode)
	}
	for _, want := range []string{"Production wildcard", "example.com", "*.example.com", "Enabled", "30 days"} {
		if !strings.Contains(detailBody, want) {
			t.Fatalf("expected detail body to contain %q", want)
		}
	}
	for _, want := range []string{
		`class="list-group list-group-flush"`,
		`class="badge bg-secondary-lt text-secondary">example.com<`,
		`class="avatar avatar-sm bg-success-lt text-success">1<`,
	} {
		if !strings.Contains(detailBody, want) {
			t.Fatalf("expected detail body to contain %q", want)
		}
	}
	for _, unwanted := range []string{"app-config-strip", "app-workflow-list", "app-detail-note", "app-coverage-grid"} {
		if strings.Contains(detailBody, unwanted) {
			t.Fatalf("expected detail body to omit legacy custom class %q", unwanted)
		}
	}

	listResp, listBody := doRequest(t, app, httptest.NewRequest(http.MethodGet, "/certificates", nil))
	if listResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected list status 200, got %d", listResp.StatusCode)
	}
	for _, want := range []string{
		"Production wildcard",
		`<th scope="col" class="d-none d-md-table-cell">SNIs</th>`,
		`class="badge bg-secondary-lt text-secondary">*.example.com<`,
		"action=\"/certificates/1/delete\"",
		">Delete<",
		`class="table table-hover card-table table-vcenter align-middle mb-0"`,
		`class="badge bg-secondary-lt text-secondary me-auto">Pending<`,
	} {
		if !strings.Contains(listBody, want) {
			t.Fatalf("expected list body to contain %q", want)
		}
	}
	for _, unwanted := range []string{"app-status-line", "app-status-dot", "app-target-count", "app-empty-state", "app-row"} {
		if strings.Contains(listBody, unwanted) {
			t.Fatalf("expected certificates list to omit legacy custom class %q", unwanted)
		}
	}
}

func TestDeleteCertificateRemovesCertificateAndLinks(t *testing.T) {
	database, app := testServer(t)
	_, err := database.Exec(`
		INSERT INTO certificates (
			name, primary_domain, domains_json, email, snis_json, status
		) VALUES (?, ?, ?, ?, ?, ?)
	`, "Production wildcard", "example.com", `["example.com"]`, "admin@example.com", `["example.com"]`, "active")
	if err != nil {
		t.Fatalf("insert certificate: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO kong_targets (
			name, environment, admin_url, auth_type,
			auth_header_name, auth_header_value, status
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "Production Kong", "production", "https://prod-kong.internal:8444", "none", "", "", "online")
	if err != nil {
		t.Fatalf("insert kong target: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO certificate_kong_targets (
			certificate_id, kong_target_id, kong_certificate_id, sync_status
		) VALUES (?, ?, ?, ?)
	`, 1, 1, "kong-cert-1", "synced")
	if err != nil {
		t.Fatalf("insert certificate link: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO jobs (certificate_id, type, status, message, log)
		VALUES (?, ?, ?, ?, ?)
	`, 1, "delete", "success", "Certificate metadata created", "log output")
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/certificates/1/delete", nil)
	resp, _ := doRequest(t, app, req)

	if resp.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", resp.StatusCode)
	}
	if location := resp.Header.Get("Location"); location != "/certificates" {
		t.Fatalf("expected list redirect, got %q", location)
	}

	var certificateCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM certificates WHERE id = ?`, 1).Scan(&certificateCount); err != nil {
		t.Fatalf("count certificates: %v", err)
	}
	if certificateCount != 0 {
		t.Fatalf("expected certificate to be deleted, got %d rows", certificateCount)
	}

	var linkCount int
	if err := database.QueryRow(`
		SELECT COUNT(*) FROM certificate_kong_targets
		WHERE certificate_id = ?
	`, 1).Scan(&linkCount); err != nil {
		t.Fatalf("count certificate links: %v", err)
	}
	if linkCount != 0 {
		t.Fatalf("expected certificate links to be deleted, got %d rows", linkCount)
	}

	var jobCertificateID sql.NullInt64
	if err := database.QueryRow(`SELECT certificate_id FROM jobs WHERE id = ?`, 1).Scan(&jobCertificateID); err != nil {
		t.Fatalf("read job certificate reference: %v", err)
	}
	if jobCertificateID.Valid {
		t.Fatalf("expected job certificate reference to be NULL, got %d", jobCertificateID.Int64)
	}
}

func TestIssueCertificateWithoutCloudflareTokenMarksCertificateFailedAndCreatesFailedJob(t *testing.T) {
	database, app := testServer(t)
	_, err := database.Exec(`
		INSERT INTO certificates (
			name, primary_domain, domains_json, email, snis_json, status
		) VALUES (?, ?, ?, ?, ?, ?)
	`, "Caption wildcard", "caption.rtt.in.th", `["caption.rtt.in.th","*.caption.rtt.in.th"]`, "ops@rtt.in.th", `["caption.rtt.in.th","*.caption.rtt.in.th"]`, "pending")
	if err != nil {
		t.Fatalf("insert certificate: %v", err)
	}

	resp, _ := doRequest(t, app, httptest.NewRequest(http.MethodPost, "/certificates/1/issue", nil))

	if resp.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", resp.StatusCode)
	}
	if location := resp.Header.Get("Location"); location != "/certificates/1" {
		t.Fatalf("expected detail redirect, got %q", location)
	}

	var status string
	if err := database.QueryRow(`SELECT status FROM certificates WHERE id = ?`, 1).Scan(&status); err != nil {
		t.Fatalf("read certificate status: %v", err)
	}
	if status != "failed" {
		t.Fatalf("expected certificate status failed, got %q", status)
	}

	var jobStatus string
	var message string
	var logOutput string
	if err := database.QueryRow(`
		SELECT status, message, log
		FROM jobs
		WHERE type = 'issue' AND certificate_id = ?
	`, 1).Scan(&jobStatus, &message, &logOutput); err != nil {
		t.Fatalf("read issue job: %v", err)
	}
	if jobStatus != "failed" {
		t.Fatalf("expected failed issue job, got %q", jobStatus)
	}
	if !strings.Contains(strings.ToLower(message), "cloudflare") {
		t.Fatalf("expected failure message to mention cloudflare token, got %q", message)
	}
	if !strings.Contains(strings.ToLower(logOutput), "cloudflare") {
		t.Fatalf("expected failure log to mention cloudflare token, got %q", logOutput)
	}
}

func TestRenewCertificateWithoutCloudflareTokenMarksCertificateFailedAndCreatesFailedJob(t *testing.T) {
	database, app := testServer(t)
	certDir := t.TempDir()
	certPath := filepath.Join(certDir, "fullchain.pem")
	keyPath := filepath.Join(certDir, "privkey.pem")
	if err := os.WriteFile(certPath, []byte("existing certificate"), 0o600); err != nil {
		t.Fatalf("write existing certificate: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("existing key"), 0o600); err != nil {
		t.Fatalf("write existing key: %v", err)
	}

	_, err := database.Exec(`
		INSERT INTO certificates (
			name, primary_domain, domains_json, email, snis_json,
			cert_path, key_path, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "Issued wildcard", "issued.example.com", `["issued.example.com","*.issued.example.com"]`, "ops@example.com", `["issued.example.com","*.issued.example.com"]`, certPath, keyPath, "active")
	if err != nil {
		t.Fatalf("insert certificate: %v", err)
	}

	resp, _ := doRequest(t, app, httptest.NewRequest(http.MethodPost, "/certificates/1/renew", nil))

	if resp.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", resp.StatusCode)
	}
	if location := resp.Header.Get("Location"); location != "/certificates/1" {
		t.Fatalf("expected detail redirect, got %q", location)
	}

	var status string
	if err := database.QueryRow(`SELECT status FROM certificates WHERE id = ?`, 1).Scan(&status); err != nil {
		t.Fatalf("read certificate status: %v", err)
	}
	if status != "failed" {
		t.Fatalf("expected certificate status failed, got %q", status)
	}

	var jobStatus string
	var message string
	var logOutput string
	if err := database.QueryRow(`
		SELECT status, message, log
		FROM jobs
		WHERE type = 'renew' AND certificate_id = ?
	`, 1).Scan(&jobStatus, &message, &logOutput); err != nil {
		t.Fatalf("read renew job: %v", err)
	}
	if jobStatus != "failed" {
		t.Fatalf("expected failed renew job, got %q", jobStatus)
	}
	if !strings.Contains(strings.ToLower(message), "cloudflare") {
		t.Fatalf("expected failure message to mention cloudflare token, got %q", message)
	}
	if !strings.Contains(strings.ToLower(logOutput), "cloudflare") {
		t.Fatalf("expected failure log to mention cloudflare token, got %q", logOutput)
	}
}

func TestEditCertificateRendersEditableDomainsAndSNIsBeforeIssue(t *testing.T) {
	database, app := testServer(t)
	_, err := database.Exec(`
		INSERT INTO certificates (
			name, primary_domain, domains_json, email, snis_json,
			auto_renew, renew_before_days, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "Caption wildcard", "caption.rtt.in.th", `["caption.rtt.in.th","*.caption.rtt.in.th"]`, "ops@rtt.in.th", `["caption.rtt.in.th","*.caption.rtt.in.th"]`, 1, 30, "pending")
	if err != nil {
		t.Fatalf("insert certificate: %v", err)
	}

	resp, body := doRequest(t, app, httptest.NewRequest(http.MethodGet, "/certificates/1/edit", nil))

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected edit status 200, got %d", resp.StatusCode)
	}
	for _, want := range []string{
		"Edit certificate",
		"action=\"/certificates/1\"",
		"Caption wildcard",
		"ops@rtt.in.th",
		"caption.rtt.in.th",
		"*.caption.rtt.in.th",
		"Create pending metadata",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected edit body to contain %q", want)
		}
	}
	if strings.Contains(body, "Domains are locked after the certificate has been issued.") {
		t.Fatal("expected pending certificate edit form to allow domain and SNI editing")
	}
}

func TestUpdateCertificateAllowsEditingMetadataAndDomainsBeforeIssue(t *testing.T) {
	database, app := testServer(t)
	_, err := database.Exec(`
		INSERT INTO certificates (
			name, primary_domain, domains_json, email, snis_json,
			auto_renew, renew_before_days, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "Caption wildcard", "caption.rtt.in.th", `["caption.rtt.in.th","*.caption.rtt.in.th"]`, "ops@rtt.in.th", `["caption.rtt.in.th","*.caption.rtt.in.th"]`, 1, 30, "pending")
	if err != nil {
		t.Fatalf("insert certificate: %v", err)
	}

	form := url.Values{
		"name":              {"Caption updated"},
		"email":             {"certs@rtt.in.th"},
		"domains":           {"api.caption.rtt.in.th\ncaption.rtt.in.th"},
		"snis":              {"api.caption.rtt.in.th\ncaption.rtt.in.th"},
		"renew_before_days": {"14"},
	}
	req := httptest.NewRequest(http.MethodPost, "/certificates/1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, _ := doRequest(t, app, req)

	if resp.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", resp.StatusCode)
	}
	if location := resp.Header.Get("Location"); location != "/certificates/1" {
		t.Fatalf("expected detail redirect, got %q", location)
	}

	var name string
	var primaryDomain string
	var email string
	var domainsJSON string
	var snisJSON string
	var autoRenew int
	var renewBeforeDays int
	if err := database.QueryRow(`
		SELECT name, primary_domain, email, domains_json, snis_json, auto_renew, renew_before_days
		FROM certificates WHERE id = ?
	`, 1).Scan(&name, &primaryDomain, &email, &domainsJSON, &snisJSON, &autoRenew, &renewBeforeDays); err != nil {
		t.Fatalf("read certificate: %v", err)
	}
	if name != "Caption updated" {
		t.Fatalf("expected updated name, got %q", name)
	}
	if primaryDomain != "api.caption.rtt.in.th" {
		t.Fatalf("expected updated primary domain, got %q", primaryDomain)
	}
	if email != "certs@rtt.in.th" {
		t.Fatalf("expected updated email, got %q", email)
	}
	if domainsJSON != `["api.caption.rtt.in.th","caption.rtt.in.th"]` {
		t.Fatalf("unexpected domains json %q", domainsJSON)
	}
	if snisJSON != `["api.caption.rtt.in.th","caption.rtt.in.th"]` {
		t.Fatalf("unexpected snis json %q", snisJSON)
	}
	if autoRenew != 0 {
		t.Fatalf("expected auto renew disabled, got %d", autoRenew)
	}
	if renewBeforeDays != 14 {
		t.Fatalf("expected renew_before_days 14, got %d", renewBeforeDays)
	}
}

func TestUpdateCertificateKeepsDomainsAndSNIsLockedAfterIssue(t *testing.T) {
	database, app := testServer(t)
	_, err := database.Exec(`
		INSERT INTO certificates (
			name, primary_domain, domains_json, email, snis_json,
			cert_path, key_path, auto_renew, renew_before_days, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "Caption wildcard", "caption.rtt.in.th", `["caption.rtt.in.th","*.caption.rtt.in.th"]`, "ops@rtt.in.th", `["caption.rtt.in.th","*.caption.rtt.in.th"]`, "/data/certs/caption.crt", "/data/certs/caption.key", 1, 30, "active")
	if err != nil {
		t.Fatalf("insert issued certificate: %v", err)
	}

	editResp, editBody := doRequest(t, app, httptest.NewRequest(http.MethodGet, "/certificates/1/edit", nil))
	if editResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected edit status 200, got %d", editResp.StatusCode)
	}
	for _, want := range []string{
		"Edit certificate",
		"Domains are locked after the certificate has been issued.",
		"SNI values are locked after the certificate has been issued.",
		"disabled",
	} {
		if !strings.Contains(editBody, want) {
			t.Fatalf("expected issued edit body to contain %q", want)
		}
	}

	form := url.Values{
		"name":              {"Caption active"},
		"email":             {"renew@rtt.in.th"},
		"domains":           {"other.rtt.in.th"},
		"snis":              {"other.rtt.in.th"},
		"auto_renew":        {"on"},
		"renew_before_days": {"10"},
	}
	req := httptest.NewRequest(http.MethodPost, "/certificates/1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, _ := doRequest(t, app, req)
	if resp.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", resp.StatusCode)
	}

	var name string
	var primaryDomain string
	var email string
	var domainsJSON string
	var snisJSON string
	var renewBeforeDays int
	if err := database.QueryRow(`
		SELECT name, primary_domain, email, domains_json, snis_json, renew_before_days
		FROM certificates WHERE id = ?
	`, 1).Scan(&name, &primaryDomain, &email, &domainsJSON, &snisJSON, &renewBeforeDays); err != nil {
		t.Fatalf("read issued certificate: %v", err)
	}
	if name != "Caption active" {
		t.Fatalf("expected updated name, got %q", name)
	}
	if email != "renew@rtt.in.th" {
		t.Fatalf("expected updated email, got %q", email)
	}
	if primaryDomain != "caption.rtt.in.th" {
		t.Fatalf("expected primary domain to stay locked, got %q", primaryDomain)
	}
	if domainsJSON != `["caption.rtt.in.th","*.caption.rtt.in.th"]` {
		t.Fatalf("expected original domains to remain, got %q", domainsJSON)
	}
	if snisJSON != `["caption.rtt.in.th","*.caption.rtt.in.th"]` {
		t.Fatalf("expected original snis to remain, got %q", snisJSON)
	}
	if renewBeforeDays != 10 {
		t.Fatalf("expected renew_before_days 10, got %d", renewBeforeDays)
	}
}

func TestCertificateDetailRendersLinkedKongTargetsSelection(t *testing.T) {
	database, app := testServer(t)
	_, err := database.Exec(`
		INSERT INTO certificates (
			name, primary_domain, domains_json, email, snis_json, status
		) VALUES (?, ?, ?, ?, ?, ?)
	`, "Production wildcard", "example.com", `["example.com"]`, "admin@example.com", `["example.com"]`, "active")
	if err != nil {
		t.Fatalf("insert certificate: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO kong_targets (
			name, environment, admin_url, auth_type,
			auth_header_name, auth_header_value, status
		) VALUES
			(?, ?, ?, ?, ?, ?, ?),
			(?, ?, ?, ?, ?, ?, ?)
	`,
		"Production Kong", "production", "https://prod-kong.internal:8444", "none", "", "", "online",
		"Staging Kong", "staging", "https://staging-kong.internal:8444", "none", "", "", "unknown",
	)
	if err != nil {
		t.Fatalf("insert kong targets: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO certificate_kong_targets (
			certificate_id, kong_target_id, kong_certificate_id, sync_status
		) VALUES (?, ?, ?, ?)
	`, 1, 1, "kong-cert-1", "synced")
	if err != nil {
		t.Fatalf("insert certificate link: %v", err)
	}

	resp, body := doRequest(t, app, httptest.NewRequest(http.MethodGet, "/certificates/1", nil))

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected detail status 200, got %d", resp.StatusCode)
	}
	for _, want := range []string{
		"Linked Kong targets",
		"action=\"/certificates/1/targets\"",
		"name=\"kong_target_ids\"",
		"Production Kong",
		"Staging Kong",
		"Synced",
		"Not linked",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected detail body to contain %q", want)
		}
	}
}

func TestCertificateDetailShowsIssueThenSyncWorkflowForPendingCertificate(t *testing.T) {
	database, app := testServer(t)
	_, err := database.Exec(`
		INSERT INTO certificates (
			name, primary_domain, domains_json, email, snis_json, status
		) VALUES (?, ?, ?, ?, ?, ?)
	`, "Pending wildcard", "pending.example.com", `["pending.example.com","*.pending.example.com"]`, "ops@example.com", `["pending.example.com","*.pending.example.com"]`, "pending")
	if err != nil {
		t.Fatalf("insert certificate: %v", err)
	}

	resp, body := doRequest(t, app, httptest.NewRequest(http.MethodGet, "/certificates/1", nil))

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected detail status 200, got %d", resp.StatusCode)
	}
	for _, want := range []string{
		"Issue and sync workflow",
		"Metadata",
		"Issue certificate",
		"action=\"/certificates/1/issue\"",
		">Issue certificate<",
		"Sync becomes available after the certificate has been issued.",
		">Sync to Kong<",
		"disabled",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected detail body to contain %q", want)
		}
	}
	if strings.Contains(body, `action="/certificates/1/sync"`) {
		t.Fatal("expected pending certificate detail to keep sync action unavailable")
	}
}

func TestCertificateDetailShowsSyncReadyWorkflowForIssuedCertificate(t *testing.T) {
	database, app := testServer(t)
	_, err := database.Exec(`
		INSERT INTO certificates (
			name, primary_domain, domains_json, email, snis_json,
			cert_path, key_path, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "Issued wildcard", "issued.example.com", `["issued.example.com"]`, "ops@example.com", `["issued.example.com"]`, "/data/certs/issued/fullchain.pem", "/data/certs/issued/privkey.pem", "active")
	if err != nil {
		t.Fatalf("insert certificate: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO kong_targets (
			name, environment, admin_url, auth_type,
			auth_header_name, auth_header_value, status
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "Production Kong", "production", "https://prod-kong.internal:8444", "none", "", "", "online")
	if err != nil {
		t.Fatalf("insert kong target: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO certificate_kong_targets (
			certificate_id, kong_target_id, sync_status
		) VALUES (?, ?, ?)
	`, 1, 1, "not_synced")
	if err != nil {
		t.Fatalf("insert certificate link: %v", err)
	}

	resp, body := doRequest(t, app, httptest.NewRequest(http.MethodGet, "/certificates/1", nil))

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected detail status 200, got %d", resp.StatusCode)
	}
	for _, want := range []string{
		"Issue and sync workflow",
		"Certificate files are available.",
		"Certificate already issued",
		`action="/certificates/1/renew"`,
		">Renew now<",
		`action="/certificates/1/sync"`,
		">Sync to Kong<",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected detail body to contain %q", want)
		}
	}
	if strings.Contains(body, `action="/certificates/1/issue"`) {
		t.Fatal("expected issued certificate detail to stop presenting issue action as available")
	}
}

func TestUpdateCertificateLinkedKongTargetsPersistsSelection(t *testing.T) {
	database, app := testServer(t)
	_, err := database.Exec(`
		INSERT INTO certificates (
			name, primary_domain, domains_json, email, snis_json, status
		) VALUES (?, ?, ?, ?, ?, ?)
	`, "Production wildcard", "example.com", `["example.com"]`, "admin@example.com", `["example.com"]`, "active")
	if err != nil {
		t.Fatalf("insert certificate: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO kong_targets (
			name, environment, admin_url, auth_type,
			auth_header_name, auth_header_value, status
		) VALUES
			(?, ?, ?, ?, ?, ?, ?),
			(?, ?, ?, ?, ?, ?, ?)
	`,
		"Production Kong", "production", "https://prod-kong.internal:8444", "none", "", "", "online",
		"Staging Kong", "staging", "https://staging-kong.internal:8444", "none", "", "", "unknown",
	)
	if err != nil {
		t.Fatalf("insert kong targets: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO certificate_kong_targets (
			certificate_id, kong_target_id, kong_certificate_id, sync_status
		) VALUES (?, ?, ?, ?)
	`, 1, 1, "kong-cert-1", "synced")
	if err != nil {
		t.Fatalf("insert certificate link: %v", err)
	}

	form := url.Values{}
	form.Add("kong_target_ids", "2")
	req := httptest.NewRequest(http.MethodPost, "/certificates/1/targets", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, _ := doRequest(t, app, req)

	if resp.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", resp.StatusCode)
	}
	if location := resp.Header.Get("Location"); location != "/certificates/1" {
		t.Fatalf("expected detail redirect, got %q", location)
	}

	var countTarget1 int
	if err := database.QueryRow(`
		SELECT COUNT(*) FROM certificate_kong_targets
		WHERE certificate_id = ? AND kong_target_id = ?
	`, 1, 1).Scan(&countTarget1); err != nil {
		t.Fatalf("count target 1 links: %v", err)
	}
	if countTarget1 != 0 {
		t.Fatalf("expected target 1 to be unlinked, got %d rows", countTarget1)
	}

	var countTarget2 int
	var syncStatus string
	if err := database.QueryRow(`
		SELECT COUNT(*), COALESCE(MAX(sync_status), '')
		FROM certificate_kong_targets
		WHERE certificate_id = ? AND kong_target_id = ?
	`, 1, 2).Scan(&countTarget2, &syncStatus); err != nil {
		t.Fatalf("read target 2 link: %v", err)
	}
	if countTarget2 != 1 {
		t.Fatalf("expected target 2 to be linked once, got %d rows", countTarget2)
	}
	if syncStatus != "not_synced" {
		t.Fatalf("expected new target link to start as not_synced, got %q", syncStatus)
	}
}

func TestCreateCertificateValidationErrors(t *testing.T) {
	_, app := testServer(t)
	form := url.Values{
		"name":              {""},
		"email":             {"not-an-email"},
		"domains":           {""},
		"snis":              {""},
		"renew_before_days": {"0"},
	}
	req := httptest.NewRequest(http.MethodPost, "/certificates", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, body := doRequest(t, app, req)

	if resp.StatusCode != fiber.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", resp.StatusCode)
	}
	for _, want := range []string{
		"Certificate name is required.",
		"A valid email address is required.",
		"Add at least one domain.",
		"Add at least one SNI value.",
		"Renew before days must be greater than 0.",
		`<h1 class="page-title">Add certificate</h1>`,
		`data-tag-for="domains" data-tag-label="Domains" data-tag-invalid="true" data-tag-describedby="error-domains hint-domains"`,
		`id="domains" name="domains" aria-label="Domains" tabindex="-1"`,
		`data-tag-for="snis"`,
		`data-tag-label="SNI values" data-tag-invalid="true" data-tag-describedby="error-snis hint-snis"`,
		`id="snis" name="snis" aria-label="SNI values" tabindex="-1"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected validation body to contain %q", want)
		}
	}
}

func TestCreateKongTargetAndRenderList(t *testing.T) {
	database, app := testServer(t)
	form := url.Values{
		"name":              {"Production Kong"},
		"environment":       {"production"},
		"admin_url":         {"https://kong-admin.internal:8444"},
		"auth_type":         {"custom-header"},
		"auth_header_name":  {"Kong-Admin-Token"},
		"auth_header_value": {"super-secret-token"},
	}
	req := httptest.NewRequest(http.MethodPost, "/kong-targets", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, _ := doRequest(t, app, req)

	if resp.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", resp.StatusCode)
	}
	if location := resp.Header.Get("Location"); location != "/kong-targets" {
		t.Fatalf("expected list redirect, got %q", location)
	}

	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM kong_targets WHERE name = ?", "Production Kong").Scan(&count); err != nil {
		t.Fatalf("count kong targets: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one kong target, got %d", count)
	}

	listResp, listBody := doRequest(t, app, httptest.NewRequest(http.MethodGet, "/kong-targets", nil))
	if listResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected list status 200, got %d", listResp.StatusCode)
	}
	for _, want := range []string{
		"Production Kong",
		`<th scope="col" class="d-none d-md-table-cell">Environment</th>`,
		`class="badge bg-secondary-lt text-secondary">production<`,
		"production",
		"https://kong-admin.internal:8444",
		"Custom header",
		"Kong-Admin-Token",
		"Unknown",
		"action=\"/kong-targets/1/delete\"",
		">Delete<",
		`class="table table-hover card-table table-vcenter align-middle mb-0"`,
		`class="badge bg-secondary-lt text-secondary me-auto">Unknown<`,
	} {
		if !strings.Contains(listBody, want) {
			t.Fatalf("expected list body to contain %q", want)
		}
	}
	for _, unwanted := range []string{"app-status-line", "app-status-dot", "app-empty-state", "app-row"} {
		if strings.Contains(listBody, unwanted) {
			t.Fatalf("expected kong targets list to omit legacy custom class %q", unwanted)
		}
	}
	if strings.Contains(listBody, "super-secret-token") {
		t.Fatal("expected list body not to render secret header value")
	}
}

func TestCreateKongTargetValidationErrors(t *testing.T) {
	_, app := testServer(t)
	form := url.Values{
		"name":              {""},
		"environment":       {""},
		"admin_url":         {"not-a-url"},
		"auth_type":         {"custom-header"},
		"auth_header_name":  {""},
		"auth_header_value": {""},
	}
	req := httptest.NewRequest(http.MethodPost, "/kong-targets", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, body := doRequest(t, app, req)

	if resp.StatusCode != fiber.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", resp.StatusCode)
	}
	for _, want := range []string{
		"Target name is required.",
		"Environment is required.",
		"Enter a valid Admin API URL.",
		"Header name is required for custom-header auth.",
		"Header value is required for custom-header auth.",
		`id="name"`,
		`aria-describedby="error-name" aria-invalid="true"`,
		`id="environment"`,
		`aria-describedby="error-environment" aria-invalid="true"`,
		`id="admin_url"`,
		`aria-describedby="hint-admin-url error-admin-url" aria-invalid="true"`,
		`id="auth_header_name"`,
		`aria-describedby="error-auth-header-name" aria-invalid="true"`,
		`id="auth_header_value"`,
		`aria-describedby="error-auth-header-value" aria-invalid="true"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected validation body to contain %q", want)
		}
	}
}

func TestCreateKongTargetRejectsUnsafeURLAndHeaderName(t *testing.T) {
	_, app := testServer(t)
	form := url.Values{
		"name":              {"Production Kong"},
		"environment":       {"production"},
		"admin_url":         {"ftp://kong-admin.internal"},
		"auth_type":         {"custom-header"},
		"auth_header_name":  {"Bad Header"},
		"auth_header_value": {"secret"},
	}
	req := httptest.NewRequest(http.MethodPost, "/kong-targets", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, body := doRequest(t, app, req)

	if resp.StatusCode != fiber.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", resp.StatusCode)
	}
	for _, want := range []string{
		"Admin API URL must start with http:// or https://.",
		"Enter a valid HTTP header name.",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected validation body to contain %q", want)
		}
	}
}

func TestEditKongTargetDoesNotRenderSecretValue(t *testing.T) {
	database, app := testServer(t)
	_, err := database.Exec(`
		INSERT INTO kong_targets (
			name, environment, admin_url, auth_type,
			auth_header_name, auth_header_value, status
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "Staging Kong", "staging", "https://staging-kong.internal:8444", "custom-header", "X-Admin-Token", "do-not-render", "unknown")
	if err != nil {
		t.Fatalf("insert kong target: %v", err)
	}

	editResp, editBody := doRequest(t, app, httptest.NewRequest(http.MethodGet, "/kong-targets/1/edit", nil))
	if editResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected edit status 200, got %d", editResp.StatusCode)
	}
	for _, want := range []string{"Staging Kong", "staging", "https://staging-kong.internal:8444", "X-Admin-Token", "Leave blank to keep existing value"} {
		if !strings.Contains(editBody, want) {
			t.Fatalf("expected edit body to contain %q", want)
		}
	}
	if strings.Contains(editBody, "do-not-render") {
		t.Fatal("expected edit body not to render secret header value")
	}

	form := url.Values{
		"name":             {"Staging Kong Updated"},
		"environment":      {"staging"},
		"admin_url":        {"https://staging-kong.internal:8445"},
		"auth_type":        {"custom-header"},
		"auth_header_name": {"X-Admin-Token"},
	}
	req := httptest.NewRequest(http.MethodPost, "/kong-targets/1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	updateResp, _ := doRequest(t, app, req)
	if updateResp.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("expected update status 303, got %d", updateResp.StatusCode)
	}

	var secret string
	if err := database.QueryRow("SELECT auth_header_value FROM kong_targets WHERE id = 1").Scan(&secret); err != nil {
		t.Fatalf("read preserved secret: %v", err)
	}
	if secret != "do-not-render" {
		t.Fatalf("expected existing secret to be preserved, got %q", secret)
	}
}

func TestDeleteKongTargetRemovesTargetAndCertificateLinks(t *testing.T) {
	database, app := testServer(t)
	_, err := database.Exec(`
		INSERT INTO certificates (
			name, primary_domain, domains_json, email, snis_json, status
		) VALUES (?, ?, ?, ?, ?, ?)
	`, "Production wildcard", "example.com", `["example.com"]`, "admin@example.com", `["example.com"]`, "active")
	if err != nil {
		t.Fatalf("insert certificate: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO kong_targets (
			name, environment, admin_url, auth_type,
			auth_header_name, auth_header_value, status
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "Production Kong", "production", "https://prod-kong.internal:8444", "none", "", "", "online")
	if err != nil {
		t.Fatalf("insert kong target: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO certificate_kong_targets (
			certificate_id, kong_target_id, kong_certificate_id, sync_status
		) VALUES (?, ?, ?, ?)
	`, 1, 1, "kong-cert-1", "synced")
	if err != nil {
		t.Fatalf("insert certificate link: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/kong-targets/1/delete", nil)
	resp, _ := doRequest(t, app, req)

	if resp.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", resp.StatusCode)
	}
	if location := resp.Header.Get("Location"); location != "/kong-targets" {
		t.Fatalf("expected list redirect, got %q", location)
	}

	var targetCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM kong_targets WHERE id = ?`, 1).Scan(&targetCount); err != nil {
		t.Fatalf("count kong targets: %v", err)
	}
	if targetCount != 0 {
		t.Fatalf("expected target to be deleted, got %d rows", targetCount)
	}

	var linkCount int
	if err := database.QueryRow(`
		SELECT COUNT(*) FROM certificate_kong_targets
		WHERE certificate_id = ? AND kong_target_id = ?
	`, 1, 1).Scan(&linkCount); err != nil {
		t.Fatalf("count certificate links: %v", err)
	}
	if linkCount != 0 {
		t.Fatalf("expected certificate link to be deleted, got %d rows", linkCount)
	}
}

func TestTestKongTargetMarksReachableTargetOnlineAndCreatesJob(t *testing.T) {
	database, app := testServer(t)
	adminAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Kong-Admin-Token") != "super-secret-token" {
			t.Fatalf("expected auth header to be forwarded")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"3.0.0"}`))
	}))
	defer adminAPI.Close()

	_, err := database.Exec(`
		INSERT INTO kong_targets (
			name, environment, admin_url, auth_type,
			auth_header_name, auth_header_value, status
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "Production Kong", "production", adminAPI.URL, "custom-header", "Kong-Admin-Token", "super-secret-token", "unknown")
	if err != nil {
		t.Fatalf("insert kong target: %v", err)
	}

	resp, _ := doRequest(t, app, httptest.NewRequest(http.MethodPost, "/kong-targets/1/test", nil))

	if resp.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", resp.StatusCode)
	}
	if location := resp.Header.Get("Location"); location != "/kong-targets" {
		t.Fatalf("expected list redirect, got %q", location)
	}

	var status string
	var lastCheckedAt sql.NullString
	if err := database.QueryRow("SELECT status, last_checked_at FROM kong_targets WHERE id = 1").Scan(&status, &lastCheckedAt); err != nil {
		t.Fatalf("read kong target status: %v", err)
	}
	if status != "online" {
		t.Fatalf("expected target status online, got %q", status)
	}
	if !lastCheckedAt.Valid || strings.TrimSpace(lastCheckedAt.String) == "" {
		t.Fatal("expected last_checked_at to be recorded")
	}

	var jobType string
	var jobStatus string
	var message string
	var logOutput string
	if err := database.QueryRow("SELECT type, status, message, log FROM jobs WHERE kong_target_id = ?", 1).Scan(&jobType, &jobStatus, &message, &logOutput); err != nil {
		t.Fatalf("read test job: %v", err)
	}
	if jobType != "test_kong" {
		t.Fatalf("expected test_kong job, got %q", jobType)
	}
	if jobStatus != "success" {
		t.Fatalf("expected success job, got %q", jobStatus)
	}
	if !strings.Contains(message, "reachable") {
		t.Fatalf("expected success message to mention reachable target, got %q", message)
	}
	if !strings.Contains(logOutput, adminAPI.URL) {
		t.Fatalf("expected log output to mention tested URL, got %q", logOutput)
	}
}

func TestTestKongTargetMarksUnreachableTargetOfflineAndCreatesFailedJob(t *testing.T) {
	database, app := testServer(t)
	adminAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	adminURL := adminAPI.URL
	adminAPI.Close()

	_, err := database.Exec(`
		INSERT INTO kong_targets (
			name, environment, admin_url, auth_type,
			auth_header_name, auth_header_value, status
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "Staging Kong", "staging", adminURL, "none", "", "", "unknown")
	if err != nil {
		t.Fatalf("insert kong target: %v", err)
	}

	resp, _ := doRequest(t, app, httptest.NewRequest(http.MethodPost, "/kong-targets/1/test", nil))

	if resp.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", resp.StatusCode)
	}

	var status string
	var lastCheckedAt sql.NullString
	if err := database.QueryRow("SELECT status, last_checked_at FROM kong_targets WHERE id = 1").Scan(&status, &lastCheckedAt); err != nil {
		t.Fatalf("read kong target status: %v", err)
	}
	if status != "offline" {
		t.Fatalf("expected target status offline, got %q", status)
	}
	if !lastCheckedAt.Valid || strings.TrimSpace(lastCheckedAt.String) == "" {
		t.Fatal("expected last_checked_at to be recorded")
	}

	var jobStatus string
	var message string
	var logOutput string
	if err := database.QueryRow("SELECT status, message, log FROM jobs WHERE kong_target_id = ?", 1).Scan(&jobStatus, &message, &logOutput); err != nil {
		t.Fatalf("read failed test job: %v", err)
	}
	if jobStatus != "failed" {
		t.Fatalf("expected failed job, got %q", jobStatus)
	}
	if strings.TrimSpace(message) == "" {
		t.Fatal("expected failure message")
	}
	if !strings.Contains(logOutput, adminURL) {
		t.Fatalf("expected log output to mention tested URL, got %q", logOutput)
	}
}

func TestSyncCertificateCreatesKongCertificateAndStoresMapping(t *testing.T) {
	database, app := testServer(t)
	certDir := t.TempDir()
	certPath := filepath.Join(certDir, "tls.crt")
	keyPath := filepath.Join(certDir, "tls.key")
	if err := os.WriteFile(certPath, []byte("CERT-PEM"), 0o600); err != nil {
		t.Fatalf("write cert file: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("KEY-PEM"), 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}

	adminAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/certificates" {
			t.Fatalf("expected /certificates path, got %s", r.URL.Path)
		}
		var payload struct {
			Cert string   `json:"cert"`
			Key  string   `json:"key"`
			SNIs []string `json:"snis"`
			Tags []string `json:"tags"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request payload: %v", err)
		}
		if payload.Cert != "CERT-PEM" || payload.Key != "KEY-PEM" {
			t.Fatalf("unexpected certificate payload: %+v", payload)
		}
		if len(payload.SNIs) != 2 || payload.SNIs[0] != "example.com" || payload.SNIs[1] != "*.example.com" {
			t.Fatalf("unexpected SNI payload: %+v", payload.SNIs)
		}
		if len(payload.Tags) != 2 || payload.Tags[0] != "source:kong-cert-lite" || payload.Tags[1] != "wildcard:true" {
			t.Fatalf("unexpected tag payload: %+v", payload.Tags)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"kong-cert-1"}`))
	}))
	defer adminAPI.Close()

	_, err := database.Exec(`
		INSERT INTO certificates (
			name, primary_domain, domains_json, email, snis_json,
			cert_path, key_path, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "Production wildcard", "example.com", `["example.com","*.example.com"]`, "admin@example.com", `["example.com","*.example.com"]`, certPath, keyPath, "active")
	if err != nil {
		t.Fatalf("insert certificate: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO kong_targets (
			name, environment, admin_url, auth_type,
			auth_header_name, auth_header_value, status
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "Production Kong", "production", adminAPI.URL, "none", "", "", "online")
	if err != nil {
		t.Fatalf("insert kong target: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO certificate_kong_targets (
			certificate_id, kong_target_id, sync_status
		) VALUES (?, ?, ?)
	`, 1, 1, "not_synced")
	if err != nil {
		t.Fatalf("insert certificate sync link: %v", err)
	}

	resp, _ := doRequest(t, app, httptest.NewRequest(http.MethodPost, "/certificates/1/sync", nil))

	if resp.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", resp.StatusCode)
	}
	if location := resp.Header.Get("Location"); location != "/certificates/1" {
		t.Fatalf("expected detail redirect, got %q", location)
	}

	var kongCertificateID string
	var syncStatus string
	var lastError string
	var lastSyncedAt sql.NullString
	if err := database.QueryRow(`
		SELECT kong_certificate_id, sync_status, last_error, last_synced_at
		FROM certificate_kong_targets
		WHERE certificate_id = ? AND kong_target_id = ?
	`, 1, 1).Scan(&kongCertificateID, &syncStatus, &lastError, &lastSyncedAt); err != nil {
		t.Fatalf("read certificate sync mapping: %v", err)
	}
	if kongCertificateID != "kong-cert-1" {
		t.Fatalf("expected kong certificate id kong-cert-1, got %q", kongCertificateID)
	}
	if syncStatus != "synced" {
		t.Fatalf("expected sync status synced, got %q", syncStatus)
	}
	if lastError != "" {
		t.Fatalf("expected empty sync error, got %q", lastError)
	}
	if !lastSyncedAt.Valid || strings.TrimSpace(lastSyncedAt.String) == "" {
		t.Fatal("expected last_synced_at to be recorded")
	}

	var jobStatus string
	var message string
	var logOutput string
	if err := database.QueryRow(`
		SELECT status, message, log
		FROM jobs
		WHERE type = 'sync' AND certificate_id = ? AND kong_target_id = ?
	`, 1, 1).Scan(&jobStatus, &message, &logOutput); err != nil {
		t.Fatalf("read sync job: %v", err)
	}
	if jobStatus != "success" {
		t.Fatalf("expected success sync job, got %q", jobStatus)
	}
	if !strings.Contains(message, "synced") {
		t.Fatalf("expected success message to mention synced, got %q", message)
	}
	if !strings.Contains(logOutput, "kong-cert-1") {
		t.Fatalf("expected sync log to mention created kong certificate id, got %q", logOutput)
	}
}

func TestSyncCertificateHandlesLargeKongResponseWithoutLoggingSecrets(t *testing.T) {
	database, app := testServer(t)
	certDir := t.TempDir()
	certPath := filepath.Join(certDir, "tls.crt")
	keyPath := filepath.Join(certDir, "tls.key")
	if err := os.WriteFile(certPath, []byte("CERT-PEM"), 0o600); err != nil {
		t.Fatalf("write cert file: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("KEY-PEM"), 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}

	largeCertificate := strings.Repeat("A", 6000)
	secretKey := "SECRET-PRIVATE-KEY"

	adminAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST request, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{
			"id":   "kong-cert-large",
			"cert": largeCertificate,
			"key":  secretKey,
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer adminAPI.Close()

	_, err := database.Exec(`
		INSERT INTO certificates (
			name, primary_domain, domains_json, email, snis_json,
			cert_path, key_path, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "Production wildcard", "example.com", `["example.com","*.example.com"]`, "admin@example.com", `["example.com","*.example.com"]`, certPath, keyPath, "active")
	if err != nil {
		t.Fatalf("insert certificate: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO kong_targets (
			name, environment, admin_url, auth_type,
			auth_header_name, auth_header_value, status
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "Production Kong", "production", adminAPI.URL, "none", "", "", "online")
	if err != nil {
		t.Fatalf("insert kong target: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO certificate_kong_targets (
			certificate_id, kong_target_id, sync_status
		) VALUES (?, ?, ?)
	`, 1, 1, "not_synced")
	if err != nil {
		t.Fatalf("insert certificate sync link: %v", err)
	}

	resp, body := doRequest(t, app, httptest.NewRequest(http.MethodPost, "/certificates/1/sync", nil))

	if resp.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("expected status 303, got %d with body %q", resp.StatusCode, body)
	}

	var kongCertificateID string
	var syncStatus string
	if err := database.QueryRow(`
		SELECT kong_certificate_id, sync_status
		FROM certificate_kong_targets
		WHERE certificate_id = ? AND kong_target_id = ?
	`, 1, 1).Scan(&kongCertificateID, &syncStatus); err != nil {
		t.Fatalf("read certificate sync mapping: %v", err)
	}
	if kongCertificateID != "kong-cert-large" {
		t.Fatalf("expected kong certificate id kong-cert-large, got %q", kongCertificateID)
	}
	if syncStatus != "synced" {
		t.Fatalf("expected sync status synced, got %q", syncStatus)
	}

	var logOutput string
	if err := database.QueryRow(`
		SELECT log
		FROM jobs
		WHERE type = 'sync' AND certificate_id = ? AND kong_target_id = ?
	`, 1, 1).Scan(&logOutput); err != nil {
		t.Fatalf("read sync job log: %v", err)
	}
	if strings.Contains(logOutput, secretKey) {
		t.Fatalf("expected sync log to omit secret key material, got %q", logOutput)
	}
	if strings.Contains(logOutput, largeCertificate) {
		t.Fatal("expected sync log to omit certificate material from Kong response")
	}
}

func TestSyncCertificateUpdatesExistingKongCertificate(t *testing.T) {
	database, app := testServer(t)
	certDir := t.TempDir()
	certPath := filepath.Join(certDir, "tls.crt")
	keyPath := filepath.Join(certDir, "tls.key")
	if err := os.WriteFile(certPath, []byte("UPDATED-CERT"), 0o600); err != nil {
		t.Fatalf("write cert file: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("UPDATED-KEY"), 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}

	adminAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH request, got %s", r.Method)
		}
		if r.URL.Path != "/certificates/existing-kong-cert" {
			t.Fatalf("expected update path, got %s", r.URL.Path)
		}
		var payload struct {
			SNIs []string `json:"snis"`
			Tags []string `json:"tags"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request payload: %v", err)
		}
		if len(payload.SNIs) != 1 || payload.SNIs[0] != "example.com" {
			t.Fatalf("unexpected SNI payload: %+v", payload.SNIs)
		}
		if len(payload.Tags) != 2 || payload.Tags[0] != "source:kong-cert-lite" || payload.Tags[1] != "wildcard:false" {
			t.Fatalf("unexpected tag payload: %+v", payload.Tags)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"existing-kong-cert"}`))
	}))
	defer adminAPI.Close()

	_, err := database.Exec(`
		INSERT INTO certificates (
			name, primary_domain, domains_json, email, snis_json,
			cert_path, key_path, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "Production cert", "example.com", `["example.com"]`, "admin@example.com", `["example.com"]`, certPath, keyPath, "active")
	if err != nil {
		t.Fatalf("insert certificate: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO kong_targets (
			name, environment, admin_url, auth_type,
			auth_header_name, auth_header_value, status
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "Production Kong", "production", adminAPI.URL, "none", "", "", "online")
	if err != nil {
		t.Fatalf("insert kong target: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO certificate_kong_targets (
			certificate_id, kong_target_id, kong_certificate_id, sync_status
		) VALUES (?, ?, ?, ?)
	`, 1, 1, "existing-kong-cert", "not_synced")
	if err != nil {
		t.Fatalf("insert certificate sync mapping: %v", err)
	}

	resp, _ := doRequest(t, app, httptest.NewRequest(http.MethodPost, "/certificates/1/sync", nil))

	if resp.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", resp.StatusCode)
	}

	var kongCertificateID string
	var syncStatus string
	if err := database.QueryRow(`
		SELECT kong_certificate_id, sync_status
		FROM certificate_kong_targets
		WHERE certificate_id = ? AND kong_target_id = ?
	`, 1, 1).Scan(&kongCertificateID, &syncStatus); err != nil {
		t.Fatalf("read updated certificate sync mapping: %v", err)
	}
	if kongCertificateID != "existing-kong-cert" {
		t.Fatalf("expected existing kong certificate id to be kept, got %q", kongCertificateID)
	}
	if syncStatus != "synced" {
		t.Fatalf("expected sync status synced, got %q", syncStatus)
	}
}

func TestSyncCertificateMissingFilesCreatesFailedJob(t *testing.T) {
	database, app := testServer(t)
	_, err := database.Exec(`
		INSERT INTO certificates (
			name, primary_domain, domains_json, email, snis_json,
			cert_path, key_path, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "Broken cert", "example.com", `["example.com"]`, "admin@example.com", `["example.com"]`, "/missing/tls.crt", "/missing/tls.key", "active")
	if err != nil {
		t.Fatalf("insert certificate: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO kong_targets (
			name, environment, admin_url, auth_type,
			auth_header_name, auth_header_value, status
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "Production Kong", "production", "http://127.0.0.1:65534", "none", "", "", "online")
	if err != nil {
		t.Fatalf("insert kong target: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO certificate_kong_targets (
			certificate_id, kong_target_id, sync_status
		) VALUES (?, ?, ?)
	`, 1, 1, "not_synced")
	if err != nil {
		t.Fatalf("insert certificate sync link: %v", err)
	}

	resp, _ := doRequest(t, app, httptest.NewRequest(http.MethodPost, "/certificates/1/sync", nil))

	if resp.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", resp.StatusCode)
	}

	var syncStatus string
	var lastError string
	if err := database.QueryRow(`
		SELECT sync_status, last_error
		FROM certificate_kong_targets
		WHERE certificate_id = ? AND kong_target_id = ?
	`, 1, 1).Scan(&syncStatus, &lastError); err != nil {
		t.Fatalf("read failed certificate sync mapping: %v", err)
	}
	if syncStatus != "failed" {
		t.Fatalf("expected sync status failed, got %q", syncStatus)
	}
	if !strings.Contains(lastError, "/missing/tls.crt") {
		t.Fatalf("expected missing cert path in last_error, got %q", lastError)
	}

	var jobStatus string
	var message string
	var logOutput string
	if err := database.QueryRow(`
		SELECT status, message, log
		FROM jobs
		WHERE type = 'sync' AND certificate_id = ? AND kong_target_id = ?
	`, 1, 1).Scan(&jobStatus, &message, &logOutput); err != nil {
		t.Fatalf("read failed sync job: %v", err)
	}
	if jobStatus != "failed" {
		t.Fatalf("expected failed sync job, got %q", jobStatus)
	}
	if !strings.Contains(message, "missing") {
		t.Fatalf("expected failure message to mention missing file, got %q", message)
	}
	if !strings.Contains(logOutput, "/missing/tls.crt") {
		t.Fatalf("expected failed sync log to mention missing cert path, got %q", logOutput)
	}
}

func TestSyncCertificateOnlySyncsLinkedKongTargets(t *testing.T) {
	database, app := testServer(t)
	certDir := t.TempDir()
	certPath := filepath.Join(certDir, "tls.crt")
	keyPath := filepath.Join(certDir, "tls.key")
	if err := os.WriteFile(certPath, []byte("CERT-PEM"), 0o600); err != nil {
		t.Fatalf("write cert file: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("KEY-PEM"), 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}

	firstCalls := 0
	firstAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"kong-cert-1"}`))
	}))
	defer firstAPI.Close()

	secondCalls := 0
	secondAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"kong-cert-2"}`))
	}))
	defer secondAPI.Close()

	_, err := database.Exec(`
		INSERT INTO certificates (
			name, primary_domain, domains_json, email, snis_json,
			cert_path, key_path, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "Production wildcard", "example.com", `["example.com"]`, "admin@example.com", `["example.com"]`, certPath, keyPath, "active")
	if err != nil {
		t.Fatalf("insert certificate: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO kong_targets (
			name, environment, admin_url, auth_type,
			auth_header_name, auth_header_value, status
		) VALUES
			(?, ?, ?, ?, ?, ?, ?),
			(?, ?, ?, ?, ?, ?, ?)
	`,
		"Production Kong", "production", firstAPI.URL, "none", "", "", "online",
		"Staging Kong", "staging", secondAPI.URL, "none", "", "", "online",
	)
	if err != nil {
		t.Fatalf("insert kong targets: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO certificate_kong_targets (
			certificate_id, kong_target_id, sync_status
		) VALUES (?, ?, ?)
	`, 1, 1, "not_synced")
	if err != nil {
		t.Fatalf("insert linked target: %v", err)
	}

	resp, _ := doRequest(t, app, httptest.NewRequest(http.MethodPost, "/certificates/1/sync", nil))

	if resp.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", resp.StatusCode)
	}
	if firstCalls != 1 {
		t.Fatalf("expected linked target to be synced once, got %d calls", firstCalls)
	}
	if secondCalls != 0 {
		t.Fatalf("expected unlinked target to remain untouched, got %d calls", secondCalls)
	}

	var syncJobCount int
	if err := database.QueryRow(`
		SELECT COUNT(*) FROM jobs WHERE type = 'sync' AND certificate_id = ?
	`, 1).Scan(&syncJobCount); err != nil {
		t.Fatalf("count sync jobs: %v", err)
	}
	if syncJobCount != 1 {
		t.Fatalf("expected one sync job for linked targets only, got %d", syncJobCount)
	}
}

func TestJobsListOrdersLatestFirstAndRendersEmptyState(t *testing.T) {
	database, app := testServer(t)
	emptyResp, emptyBody := doRequest(t, app, httptest.NewRequest(http.MethodGet, "/jobs", nil))
	if emptyResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected empty list status 200, got %d", emptyResp.StatusCode)
	}
	if !strings.Contains(emptyBody, "No jobs") {
		t.Fatal("expected empty list body to contain useful empty state")
	}
	if !strings.Contains(emptyBody, `class="empty m-0 w-100"`) {
		t.Fatal("expected empty list body to use Tabler empty state markup")
	}

	_, err := database.Exec(`
		INSERT INTO jobs (type, status, message, log, started_at, finished_at)
		VALUES
			('sync', 'success', 'Old sync complete', 'old sync log', '2026-04-25 10:00:00', '2026-04-25 10:01:00'),
			('test_kong', 'running', 'Testing Kong target', 'testing log', '2026-04-26 11:00:00', NULL),
			('renew', 'failed', 'Renew failed', 'renew failed log', '2026-04-26 10:00:00', '2026-04-26 10:03:00')
	`)
	if err != nil {
		t.Fatalf("insert jobs: %v", err)
	}

	resp, body := doRequest(t, app, httptest.NewRequest(http.MethodGet, "/jobs", nil))
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected list status 200, got %d", resp.StatusCode)
	}
	for _, want := range []string{"Testing Kong target", "Renew failed", "Old sync complete", "Test Kong", "Running", "Failed", "Success"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected jobs body to contain %q", want)
		}
	}
	for _, want := range []string{
		`class="table table-hover card-table table-vcenter align-middle mb-0"`,
		`class="badge bg-purple-lt text-purple text-uppercase me-auto">Test Kong<`,
		`class="badge bg-warning-lt text-warning me-auto">Running<`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected jobs body to contain %q", want)
		}
	}
	for _, unwanted := range []string{"app-log-type", "app-log-table", "app-log-detail", "app-log-msg"} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("expected jobs body to omit legacy custom class %q", unwanted)
		}
	}
	first := strings.Index(body, "Testing Kong target")
	second := strings.Index(body, "Renew failed")
	third := strings.Index(body, "Old sync complete")
	if first == -1 || second == -1 || third == -1 || !(first < second && second < third) {
		t.Fatal("expected jobs to be ordered by latest first")
	}
}

func TestJobsListShowsOnlyTwentyLatestItems(t *testing.T) {
	database, app := testServer(t)

	for i := 1; i <= 21; i++ {
		startedAt := fmt.Sprintf("2026-04-%02d 10:00:00", i)
		message := fmt.Sprintf("Job message %02d", i)
		if _, err := database.Exec(`
			INSERT INTO jobs (type, status, message, log, started_at, finished_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`, "sync", "success", message, "job log", startedAt, startedAt); err != nil {
			t.Fatalf("insert job %d: %v", i, err)
		}
	}

	resp, body := doRequest(t, app, httptest.NewRequest(http.MethodGet, "/jobs", nil))
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected list status 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(body, "Job message 21") {
		t.Fatal("expected latest job to be rendered")
	}
	if strings.Contains(body, "Job message 01") {
		t.Fatal("expected oldest job outside latest 20 to be omitted")
	}
	if got := strings.Count(body, `class="btn btn-outline-secondary btn-sm" href="/jobs/`); got != 20 {
		t.Fatalf("expected 20 job rows, got %d", got)
	}
	if !strings.Contains(body, "20 of 21 records") {
		t.Fatal("expected jobs summary count to reflect the 20 latest items")
	}
	first := strings.Index(body, "Job message 21")
	second := strings.Index(body, "Job message 20")
	if first == -1 || second == -1 || !(first < second) {
		t.Fatal("expected latest jobs to remain ordered newest first")
	}
}

func TestJobDetailRendersStatusTimingMessageAndLogs(t *testing.T) {
	database, app := testServer(t)
	certificateResult, err := database.Exec(`
		INSERT INTO certificates (
			name, primary_domain, domains_json, email, snis_json
		) VALUES (?, ?, ?, ?, ?)
	`, "Production wildcard", "example.com", `["example.com"]`, "admin@example.com", `["example.com"]`)
	if err != nil {
		t.Fatalf("insert certificate: %v", err)
	}
	certificateID, err := certificateResult.LastInsertId()
	if err != nil {
		t.Fatalf("read certificate id: %v", err)
	}

	kongTargetResult, err := database.Exec(`
		INSERT INTO kong_targets (
			name, environment, admin_url, auth_type,
			auth_header_name, auth_header_value, status
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "Production Kong", "production", "https://kong-admin.internal:8444", "none", "", "", "unknown")
	if err != nil {
		t.Fatalf("insert kong target: %v", err)
	}
	kongTargetID, err := kongTargetResult.LastInsertId()
	if err != nil {
		t.Fatalf("read kong target id: %v", err)
	}

	_, err = database.Exec(`
		INSERT INTO jobs (
			certificate_id, kong_target_id, type, status,
			message, log, started_at, finished_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		certificateID,
		kongTargetID,
		"sync",
		"failed",
		"Kong certificate update failed",
		"Started sync\nKong returned 401\nSync failed",
		"2026-04-26 09:00:00",
		"2026-04-26 09:02:00",
	)
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}

	resp, body := doRequest(t, app, httptest.NewRequest(http.MethodGet, "/jobs/1", nil))
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected detail status 200, got %d", resp.StatusCode)
	}
	for _, want := range []string{
		"Job #1",
		"Failed",
		"Sync",
		"2026-04-26 09:00",
		"2026-04-26 09:02",
		"Certificate #1",
		"Kong target #1",
		"Kong certificate update failed",
		"Started sync",
		"Kong returned 401",
		"Sync failed",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected detail body to contain %q", want)
		}
	}
	for _, want := range []string{
		`class="badge bg-success-lt text-success text-uppercase">Sync<`,
		`class="badge bg-danger-lt text-danger">Failed<`,
		`class="bg-body-secondary border rounded p-3 mb-0 font-monospace small text-body overflow-auto"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected detail body to contain %q", want)
		}
	}
	for _, unwanted := range []string{"app-log-type", "app-log-block", "app-empty-state"} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("expected job detail to omit legacy custom class %q", unwanted)
		}
	}
}

func testApp(t *testing.T) *fiber.App {
	t.Helper()

	_, app := testServer(t)
	return app
}

func testServer(t *testing.T) (*sql.DB, *fiber.App) {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("close test database: %v", err)
		}
	})

	certificateRepository := sqliteadapter.NewCertificateRepository(database)
	certificateUseCase := usecase.NewCertificateUseCase(certificateRepository)
	kongTargetRepository := sqliteadapter.NewKongTargetRepository(database)
	jobRepository := sqliteadapter.NewJobRepository(database)
	jobUseCase := usecase.NewJobUseCase(jobRepository)
	kongAdminClient := kongadapter.NewAdminClient(nil)
	kongSyncUseCase := usecase.NewKongSyncUseCase(certificateRepository, kongTargetRepository, jobUseCase, kongAdminClient)
	acmeClient := acmeadapter.NewLegoClient(filepath.Join(t.TempDir(), "accounts"), "staging", "")
	acmeUseCase := usecase.NewACMEUseCase(certificateRepository, jobUseCase, acmeClient, filepath.Join(t.TempDir(), "certs"), kongSyncUseCase)
	kongTargetUseCase := usecase.NewKongTargetUseCase(kongTargetRepository, kongAdminClient, jobUseCase)

	return database, NewApp(nil, certificateUseCase, acmeUseCase, kongSyncUseCase, kongTargetUseCase, jobUseCase)
}

func doRequest(t *testing.T, app *fiber.App, req *http.Request) (*http.Response, string) {
	t.Helper()

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app test request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	return resp, string(body)
}
