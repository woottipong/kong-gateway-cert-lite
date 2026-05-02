package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"kong-cert-lite/internal/usecase"
)

type Notifier struct {
	webhookURL string
	client     *http.Client
}

func NewNotifier(webhookURL string, client *http.Client) *Notifier {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Notifier{
		webhookURL: strings.TrimSpace(webhookURL),
		client:     client,
	}
}

func (n *Notifier) Notify(ctx context.Context, event usecase.NotificationEvent) error {
	if strings.TrimSpace(n.webhookURL) == "" {
		return nil
	}

	payload := discordPayload{
		Username: "Kong CertOps",
		Embeds:   []discordEmbed{buildEmbed(event)},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
	}

	return nil
}

type discordPayload struct {
	Username string         `json:"username,omitempty"`
	Embeds   []discordEmbed `json:"embeds"`
}

type discordEmbed struct {
	Title     string         `json:"title"`
	Color     int            `json:"color"`
	Fields    []discordField `json:"fields"`
	Timestamp string         `json:"timestamp"`
}

type discordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

func buildEmbed(event usecase.NotificationEvent) discordEmbed {
	title := "[" + strings.ToUpper(string(event.Severity)) + "] " + strings.ReplaceAll(event.Event, "_", " ")
	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	fields := []discordField{
		{Name: "Event", Value: event.Event, Inline: true},
	}
	if event.JobID > 0 {
		fields = append(fields, discordField{Name: "Job", Value: fmt.Sprintf("#%d %s", event.JobID, event.JobType), Inline: true})
	}
	if event.Certificate != nil {
		fields = append(fields,
			discordField{Name: "Certificate", Value: nonEmpty(event.Certificate.Name, fmt.Sprintf("#%d", event.Certificate.ID)), Inline: true},
			discordField{Name: "Domain", Value: nonEmpty(event.Certificate.PrimaryDomain, "-"), Inline: true},
		)
	}
	if event.KongTarget != "" {
		fields = append(fields, discordField{Name: "Kong target", Value: event.KongTarget, Inline: true})
	}
	if event.Environment != "" {
		fields = append(fields, discordField{Name: "Environment", Value: event.Environment, Inline: true})
	}
	if event.RemainingDays != nil {
		fields = append(fields, discordField{Name: "Remaining", Value: fmt.Sprintf("%d days", *event.RemainingDays), Inline: true})
	}
	if strings.TrimSpace(event.Message) != "" {
		fields = append(fields, discordField{Name: "Message", Value: truncate(event.Message, 900), Inline: false})
	}

	return discordEmbed{
		Title:     title,
		Color:     colorForSeverity(event.Severity),
		Fields:    fields,
		Timestamp: occurredAt.UTC().Format(time.RFC3339),
	}
}

func colorForSeverity(severity usecase.NotificationSeverity) int {
	switch severity {
	case usecase.NotificationSeveritySuccess:
		return 0x1DB954
	case usecase.NotificationSeverityWarning:
		return 0xF59F00
	case usecase.NotificationSeverityCritical:
		return 0xD63939
	default:
		return 0x6B7280
	}
}

func nonEmpty(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func truncate(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	return value[:max-3] + "..."
}
