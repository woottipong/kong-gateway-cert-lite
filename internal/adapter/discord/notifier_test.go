package discord

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	if embed.Color != 0xD63939 {
		t.Fatalf("expected critical color, got %d", embed.Color)
	}
	if embed.Timestamp != eventTime.Format(time.RFC3339) {
		t.Fatalf("expected timestamp %q, got %q", eventTime.Format(time.RFC3339), embed.Timestamp)
	}
	if !fieldExists(embed.Fields, "Certificate", "Production wildcard") {
		t.Fatal("expected certificate field")
	}
	if !fieldExists(embed.Fields, "Message", "DNS challenge failed") {
		t.Fatal("expected message field")
	}
}

func TestNotifierNoopsWithoutWebhookURL(t *testing.T) {
	notifier := NewNotifier("", nil)

	if err := notifier.Notify(context.Background(), usecase.NotificationEvent{Event: "renew_failed"}); err != nil {
		t.Fatalf("expected empty webhook to no-op, got %v", err)
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
