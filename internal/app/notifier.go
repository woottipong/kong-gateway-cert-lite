package app

import (
	"context"
	"log/slog"

	"kong-cert-lite/internal/usecase"
)

type loggingNotifier struct {
	logger *slog.Logger
	next   usecase.Notifier
}

func (n loggingNotifier) Notify(ctx context.Context, event usecase.NotificationEvent) error {
	if n.next == nil {
		return nil
	}
	if err := n.next.Notify(ctx, event); err != nil {
		logger := n.logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.Warn("send notification failed", "event", event.Event, "severity", event.Severity, "error", err)
	}
	return nil
}
