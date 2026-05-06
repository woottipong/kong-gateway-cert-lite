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
	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	presentation := notificationPresentationFor(event)
	title := strings.TrimSpace(eventIcon(event) + " " + presentation.Title)

	fields := []discordField{
		{Name: "Severity", Value: severityLabel(event.Severity), Inline: true},
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
	if strings.TrimSpace(presentation.Action) != "" {
		fields = append(fields, discordField{Name: "Action", Value: presentation.Action, Inline: false})
	}

	return discordEmbed{
		Title:     title,
		Color:     colorForSeverity(event.Severity),
		Fields:    fields,
		Timestamp: occurredAt.UTC().Format(time.RFC3339),
	}
}

type notificationPresentation struct {
	Title  string
	Action string
}

func notificationPresentationFor(event usecase.NotificationEvent) notificationPresentation {
	switch event.Event {
	case "issue_succeeded":
		return notificationPresentation{Title: "Certificate issued", Action: "No action required. Verify linked Kong targets if this certificate should be served immediately."}
	case "issue_failed":
		return notificationPresentation{Title: "Certificate issue failed", Action: jobAction(event, "Review the ACME job log, then check DNS challenge configuration and Cloudflare token permissions.")}
	case "renew_succeeded":
		return notificationPresentation{Title: "Certificate renewed", Action: "No action required. Linked Kong targets will sync after renew when configured."}
	case "renew_failed":
		return notificationPresentation{Title: "Certificate renew failed", Action: jobAction(event, "Review the renew job log, then check ACME DNS-01, Cloudflare zone access, and recent retry cooldown.")}
	case "sync_succeeded":
		return notificationPresentation{Title: "Kong certificate sync completed", Action: "No action required. Confirm the target status if traffic still serves the old certificate."}
	case "sync_failed":
		return notificationPresentation{Title: "Kong certificate sync failed", Action: jobAction(event, "Review the sync job log, then check Kong Admin URL, authentication, network access, and certificate file paths.")}
	case "kong_target_test_succeeded":
		return notificationPresentation{Title: "Kong target connectivity passed", Action: "No action required."}
	case "kong_target_test_failed":
		return notificationPresentation{Title: "Kong target connectivity failed", Action: jobAction(event, "Check the Kong Admin URL, authentication header, DNS/network route, and firewall access.")}
	case "certificate_expiring_14_days":
		return notificationPresentation{Title: "Certificate expires in about 14 days", Action: "Verify auto renew is enabled and the certificate has linked Kong targets if it should be synced after renewal."}
	case "certificate_expiring_7_days":
		return notificationPresentation{Title: "Certificate expires in about 7 days", Action: "Prioritize renewal soon. Check recent renew jobs before manually retrying."}
	case "certificate_expiring_3_days":
		return notificationPresentation{Title: "Certificate expires in about 3 days", Action: "Renew immediately or verify that auto renew completed successfully."}
	case "certificate_expired":
		return notificationPresentation{Title: "Certificate expired", Action: "Renew or replace the certificate immediately, then sync the linked Kong targets."}
	default:
		return notificationPresentation{Title: fallbackTitle(event), Action: ""}
	}
}

func eventIcon(event usecase.NotificationEvent) string {
	switch event.Event {
	case "issue_succeeded":
		return "✅"
	case "issue_failed":
		return "🚨"
	case "renew_succeeded":
		return "✅"
	case "renew_failed":
		return "🚨"
	case "sync_succeeded":
		return "✅"
	case "sync_failed":
		return "🚨"
	case "kong_target_test_succeeded":
		return "✅"
	case "kong_target_test_failed":
		return "⚠️"
	case "certificate_expiring_14_days":
		return "⚠️"
	case "certificate_expiring_7_days", "certificate_expiring_3_days":
		return "🚨"
	case "certificate_expired":
		return "🛑"
	default:
		return severityIcon(event.Severity)
	}
}

func severityIcon(severity usecase.NotificationSeverity) string {
	switch severity {
	case usecase.NotificationSeveritySuccess:
		return "✅"
	case usecase.NotificationSeverityWarning:
		return "⚠️"
	case usecase.NotificationSeverityCritical:
		return "🚨"
	case usecase.NotificationSeverityInfo:
		return "ℹ️"
	default:
		return "ℹ️"
	}
}

func jobAction(event usecase.NotificationEvent, action string) string {
	if event.JobID <= 0 {
		return action
	}
	return fmt.Sprintf("%s Job #%d has the full log.", action, event.JobID)
}

func fallbackTitle(event usecase.NotificationEvent) string {
	value := strings.TrimSpace(strings.ReplaceAll(event.Event, "_", " "))
	if value == "" {
		value = "notification"
	}
	words := strings.Fields(value)
	for index, word := range words {
		if word == "" {
			continue
		}
		words[index] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

func severityLabel(severity usecase.NotificationSeverity) string {
	switch severity {
	case usecase.NotificationSeveritySuccess:
		return "Success"
	case usecase.NotificationSeverityWarning:
		return "Warning"
	case usecase.NotificationSeverityCritical:
		return "Critical"
	case usecase.NotificationSeverityInfo:
		return "Info"
	default:
		return nonEmpty(string(severity), "Info")
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
