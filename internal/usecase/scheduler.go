package usecase

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"kong-cert-lite/internal/domain"
)

type RenewalCertificateRepository interface {
	List(ctx context.Context) ([]domain.Certificate, error)
}

type CertificateRenewer interface {
	RenewCertificate(ctx context.Context, certificateID int64) error
}

type RenewalScheduler struct {
	certificates RenewalCertificateRepository
	renewer      CertificateRenewer
	schedule     CronSchedule
	notifier     Notifier
	notified     map[string]struct{}

	stopCh  chan struct{}
	doneCh  chan struct{}
	mu      sync.Mutex
	started bool
	stopped bool
}

func NewRenewalScheduler(certificates RenewalCertificateRepository, renewer CertificateRenewer, cronExpression string) (*RenewalScheduler, error) {
	schedule, err := ParseCronExpression(cronExpression)
	if err != nil {
		return nil, err
	}
	if certificates == nil || renewer == nil {
		return nil, fmt.Errorf("renewal scheduler dependencies are not configured")
	}

	return &RenewalScheduler{
		certificates: certificates,
		renewer:      renewer,
		schedule:     schedule,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
		notified:     make(map[string]struct{}),
	}, nil
}

func (s *RenewalScheduler) SetNotifier(notifier Notifier) {
	s.notifier = notifier
}

func (s *RenewalScheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.started || s.stopped {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.mu.Unlock()

	go s.loop(ctx)
}

func (s *RenewalScheduler) Stop() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	started := s.started
	close(s.stopCh)
	if !started {
		close(s.doneCh)
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	<-s.doneCh
}

func (s *RenewalScheduler) RunOnce(ctx context.Context, now time.Time) error {
	certificates, err := s.certificates.List(ctx)
	if err != nil {
		return err
	}

	var renewErrors []error
	for _, certificate := range certificates {
		if !shouldAutoRenew(certificate, now) {
			continue
		}
		if err := s.renewer.RenewCertificate(ctx, certificate.ID); err != nil {
			renewErrors = append(renewErrors, fmt.Errorf("auto renew certificate %d: %w", certificate.ID, err))
		}
	}

	if s.notifier != nil {
		refreshedCertificates, err := s.certificates.List(ctx)
		if err != nil {
			renewErrors = append(renewErrors, err)
		} else {
			for _, certificate := range refreshedCertificates {
				if event, ok := certificateExpiryNotification(certificate, now); ok {
					if s.expiryNotificationSent(event, now) {
						continue
					}
					_ = s.notifier.Notify(ctx, event)
				}
			}
		}
	}

	return errors.Join(renewErrors...)
}

func (s *RenewalScheduler) expiryNotificationSent(event NotificationEvent, now time.Time) bool {
	if event.Certificate == nil {
		return false
	}
	key := fmt.Sprintf("%d:%s:%s", event.Certificate.ID, event.Event, now.UTC().Format("2006-01-02"))

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.notified[key]; ok {
		return true
	}
	s.notified[key] = struct{}{}
	return false
}

func (s *RenewalScheduler) loop(ctx context.Context) {
	defer close(s.doneCh)

	for {
		now := time.Now().UTC()
		next := s.schedule.Next(now)
		timer := time.NewTimer(time.Until(next))
		select {
		case <-timer.C:
			_ = s.RunOnce(ctx, time.Now().UTC())
		case <-ctx.Done():
			timer.Stop()
			return
		case <-s.stopCh:
			timer.Stop()
			return
		}
	}
}

func shouldAutoRenew(certificate domain.Certificate, now time.Time) bool {
	if !certificate.AutoRenew || certificate.ExpiresAt == nil || certificate.RenewBeforeDays <= 0 {
		return false
	}
	if strings.TrimSpace(certificate.CertPath) == "" || strings.TrimSpace(certificate.KeyPath) == "" {
		return false
	}
	switch certificate.Status {
	case domain.CertificateStatusActive, domain.CertificateStatusWarning, domain.CertificateStatusExpired:
	default:
		return false
	}

	renewAt := certificate.ExpiresAt.UTC().AddDate(0, 0, -certificate.RenewBeforeDays)
	return !now.UTC().Before(renewAt)
}

type CronSchedule struct {
	minute  cronField
	hour    cronField
	day     cronField
	month   cronField
	weekday cronField
}

func ParseCronExpression(expression string) (CronSchedule, error) {
	fields := strings.Fields(expression)
	if len(fields) != 5 {
		return CronSchedule{}, fmt.Errorf("AUTO_RENEW_CRON must be a 5-field cron expression")
	}

	minute, err := parseCronField(fields[0], 0, 59)
	if err != nil {
		return CronSchedule{}, fmt.Errorf("invalid AUTO_RENEW_CRON minute field: %w", err)
	}
	hour, err := parseCronField(fields[1], 0, 23)
	if err != nil {
		return CronSchedule{}, fmt.Errorf("invalid AUTO_RENEW_CRON hour field: %w", err)
	}
	day, err := parseCronField(fields[2], 1, 31)
	if err != nil {
		return CronSchedule{}, fmt.Errorf("invalid AUTO_RENEW_CRON day field: %w", err)
	}
	month, err := parseCronField(fields[3], 1, 12)
	if err != nil {
		return CronSchedule{}, fmt.Errorf("invalid AUTO_RENEW_CRON month field: %w", err)
	}
	weekday, err := parseCronField(fields[4], 0, 6)
	if err != nil {
		return CronSchedule{}, fmt.Errorf("invalid AUTO_RENEW_CRON weekday field: %w", err)
	}

	return CronSchedule{minute: minute, hour: hour, day: day, month: month, weekday: weekday}, nil
}

func (s CronSchedule) Next(after time.Time) time.Time {
	candidate := after.UTC().Truncate(time.Minute).Add(time.Minute)
	for i := 0; i < 366*24*60*5; i++ {
		if s.matches(candidate) {
			return candidate
		}
		candidate = candidate.Add(time.Minute)
	}
	return candidate
}

func (s CronSchedule) matches(value time.Time) bool {
	return s.minute.matches(value.Minute()) &&
		s.hour.matches(value.Hour()) &&
		s.day.matches(value.Day()) &&
		s.month.matches(int(value.Month())) &&
		s.weekday.matches(int(value.Weekday()))
}

type cronField struct {
	allowed map[int]struct{}
}

func parseCronField(value string, min int, max int) (cronField, error) {
	allowed := make(map[int]struct{})
	parts := strings.Split(value, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return cronField{}, fmt.Errorf("empty cron field segment")
		}

		switch {
		case part == "*":
			for i := min; i <= max; i++ {
				allowed[i] = struct{}{}
			}
		case strings.HasPrefix(part, "*/"):
			step, err := strconv.Atoi(strings.TrimPrefix(part, "*/"))
			if err != nil || step <= 0 {
				return cronField{}, fmt.Errorf("invalid step %q", part)
			}
			for i := min; i <= max; i += step {
				allowed[i] = struct{}{}
			}
		default:
			number, err := strconv.Atoi(part)
			if err != nil {
				return cronField{}, fmt.Errorf("invalid value %q", part)
			}
			if number < min || number > max {
				return cronField{}, fmt.Errorf("value %d outside range %d-%d", number, min, max)
			}
			allowed[number] = struct{}{}
		}
	}

	return cronField{allowed: allowed}, nil
}

func (f cronField) matches(value int) bool {
	_, ok := f.allowed[value]
	return ok
}
