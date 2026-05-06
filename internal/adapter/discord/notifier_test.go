package discord

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kong-cert-lite/internal/domain"
	"kong-cert-lite/internal/usecase"
)

func TestNotifierSendsDiscordEmbed(t *testing.T) {
	var payload discordPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	certificate := domain.Certificate{ID: 12, Name: "Production wildcard", PrimaryDomain: "*.example.com"}
	eventTime := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	notifier := NewNotifier(server.URL, server.Client())

	if err := notifier.Notify(context.Background(), usecase.NotificationEvent{
		Severity:    usecase.NotificationSeverityCritical,
		Event:       "renew_failed",
		Certificate: &certificate,
		JobID:       42,
		JobType:     domain.JobTypeRenew,
		JobStatus:   domain.JobStatusFailed,
		Message:     "DNS challenge failed",
		OccurredAt:  eventTime,
	}); err != nil {
		t.Fatalf("notify discord: %v", err)
	}

	if payload.Username != "Kong CertOps" {
		t.Fatalf("expected username Kong CertOps, got %q", payload.Username)
	}
	if len(payload.Embeds) != 1 {
		t.Fatalf("expected one embed, got %d", len(payload.Embeds))
	}
	embed := payload.Embeds[0]
	if embed.Title != "🚨 Certificate renew failed" {
		t.Fatalf("expected friendly title, got %q", embed.Title)
	}
	if embed.Color != 0xD63939 {
		t.Fatalf("expected critical color, got %d", embed.Color)
	}
	if embed.Timestamp != eventTime.Format(time.RFC3339) {
		t.Fatalf("expected timestamp %q, got %q", eventTime.Format(time.RFC3339), embed.Timestamp)
	}
	if !fieldExists(embed.Fields, "Certificate", "Production wildcard") {
		t.Fatal("expected certificate field")
	}
	if fieldNameExists(embed.Fields, "Message") {
		t.Fatal("expected message field to be omitted from discord embed")
	}
	if !fieldExists(embed.Fields, "Severity", "Critical") {
		t.Fatal("expected severity field")
	}
	if !fieldContains(embed.Fields, "Action", "Job #42 has the full log.") {
		t.Fatal("expected action field to mention job log")
	}
}

func TestNotifierNoopsWithoutWebhookURL(t *testing.T) {
	notifier := NewNotifier("", nil)

	if err := notifier.Notify(context.Background(), usecase.NotificationEvent{Event: "renew_failed"}); err != nil {
		t.Fatalf("expected empty webhook to no-op, got %v", err)
	}
}

func TestNotificationPresentationsCoverKnownEvents(t *testing.T) {
	events := []struct {
		event    usecase.NotificationEvent
		title    string
		action   string
		color    int
		severity string
		icon     string
	}{
		{
			event:    usecase.NotificationEvent{Severity: usecase.NotificationSeveritySuccess, Event: "issue_succeeded", JobStatus: domain.JobStatusSuccess},
			title:    "Certificate issued",
			action:   "No action required.",
			color:    0x1DB954,
			severity: "Success",
			icon:     "✅",
		},
		{
			event:    usecase.NotificationEvent{Severity: usecase.NotificationSeverityCritical, Event: "issue_failed", JobID: 101, JobType: domain.JobTypeIssue, JobStatus: domain.JobStatusFailed},
			title:    "Certificate issue failed",
			action:   "DNS challenge configuration",
			color:    0xD63939,
			severity: "Critical",
			icon:     "🚨",
		},
		{
			event:    usecase.NotificationEvent{Severity: usecase.NotificationSeveritySuccess, Event: "renew_succeeded", JobStatus: domain.JobStatusSuccess},
			title:    "Certificate renewed",
			action:   "Linked Kong targets will sync",
			color:    0x1DB954,
			severity: "Success",
			icon:     "✅",
		},
		{
			event:    usecase.NotificationEvent{Severity: usecase.NotificationSeverityCritical, Event: "renew_failed", JobID: 102, JobType: domain.JobTypeRenew, JobStatus: domain.JobStatusFailed},
			title:    "Certificate renew failed",
			action:   "Cloudflare zone access",
			color:    0xD63939,
			severity: "Critical",
			icon:     "🚨",
		},
		{
			event:    usecase.NotificationEvent{Severity: usecase.NotificationSeveritySuccess, Event: "sync_succeeded", JobStatus: domain.JobStatusSuccess},
			title:    "Kong certificate sync completed",
			action:   "No action required.",
			color:    0x1DB954,
			severity: "Success",
			icon:     "✅",
		},
		{
			event:    usecase.NotificationEvent{Severity: usecase.NotificationSeverityCritical, Event: "sync_failed", JobID: 103, JobType: domain.JobTypeSync, JobStatus: domain.JobStatusFailed},
			title:    "Kong certificate sync failed",
			action:   "Kong Admin URL",
			color:    0xD63939,
			severity: "Critical",
			icon:     "🚨",
		},
		{
			event:    usecase.NotificationEvent{Severity: usecase.NotificationSeveritySuccess, Event: "kong_target_test_succeeded", JobStatus: domain.JobStatusSuccess},
			title:    "Kong target connectivity passed",
			action:   "No action required.",
			color:    0x1DB954,
			severity: "Success",
			icon:     "✅",
		},
		{
			event:    usecase.NotificationEvent{Severity: usecase.NotificationSeverityWarning, Event: "kong_target_test_failed", JobID: 104, JobType: domain.JobTypeTestKong, JobStatus: domain.JobStatusFailed},
			title:    "Kong target connectivity failed",
			action:   "authentication header",
			color:    0xF59F00,
			severity: "Warning",
			icon:     "⚠️",
		},
		{
			event:    usecase.NotificationEvent{Severity: usecase.NotificationSeverityWarning, Event: "certificate_expiring_14_days"},
			title:    "Certificate expires in about 14 days",
			action:   "Verify auto renew",
			color:    0xF59F00,
			severity: "Warning",
			icon:     "⚠️",
		},
		{
			event:    usecase.NotificationEvent{Severity: usecase.NotificationSeverityCritical, Event: "certificate_expiring_7_days"},
			title:    "Certificate expires in about 7 days",
			action:   "Prioritize renewal soon",
			color:    0xD63939,
			severity: "Critical",
			icon:     "🚨",
		},
		{
			event:    usecase.NotificationEvent{Severity: usecase.NotificationSeverityCritical, Event: "certificate_expiring_3_days"},
			title:    "Certificate expires in about 3 days",
			action:   "Renew immediately",
			color:    0xD63939,
			severity: "Critical",
			icon:     "🚨",
		},
		{
			event:    usecase.NotificationEvent{Severity: usecase.NotificationSeverityCritical, Event: "certificate_expired"},
			title:    "Certificate expired",
			action:   "sync the linked Kong targets",
			color:    0xD63939,
			severity: "Critical",
			icon:     "🛑",
		},
	}

	for _, tt := range events {
		t.Run(tt.event.Event, func(t *testing.T) {
			embed := buildEmbed(tt.event)
			wantTitle := tt.icon + " " + tt.title
			if embed.Title != wantTitle {
				t.Fatalf("expected title %q, got %q", wantTitle, embed.Title)
			}
			if embed.Color != tt.color {
				t.Fatalf("expected color %d, got %d", tt.color, embed.Color)
			}
			if !fieldExists(embed.Fields, "Severity", tt.severity) {
				t.Fatalf("expected severity %q in fields", tt.severity)
			}
			if !fieldContains(embed.Fields, "Action", tt.action) {
				t.Fatalf("expected action field to contain %q", tt.action)
			}
		})
	}
}

func fieldExists(fields []discordField, name string, value string) bool {
	for _, field := range fields {
		if field.Name == name && field.Value == value {
			return true
		}
	}
	return false
}

func fieldNameExists(fields []discordField, name string) bool {
	for _, field := range fields {
		if field.Name == name {
			return true
		}
	}
	return false
}

func fieldContains(fields []discordField, name string, value string) bool {
	for _, field := range fields {
		if field.Name == name && strings.Contains(field.Value, value) {
			return true
		}
	}
	return false
}
