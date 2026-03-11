// Package calendar implements calendar port adapters for external calendar APIs.
package calendar

import (
	"context"
	"fmt"
	"time"

	domain "alex/internal/domain/calendar"
	"alex/internal/shared/logging"
)

// larkCalendarProvider implements CalendarPort using the Lark Calendar API.
// This is a stub implementation — the actual Lark API integration will be
// added when API credentials and SDK bindings are available.
type larkCalendarProvider struct {
	appID      string
	appSecret  string
	baseDomain string
	logger     logging.Logger
}

// LarkCalendarConfig holds configuration for the Lark calendar provider.
type LarkCalendarConfig struct {
	AppID      string
	AppSecret  string
	BaseDomain string
}

// NewLarkCalendarProvider creates a new Lark calendar provider.
func NewLarkCalendarProvider(cfg LarkCalendarConfig, logger logging.Logger) *larkCalendarProvider {
	return &larkCalendarProvider{
		appID:      cfg.AppID,
		appSecret:  cfg.AppSecret,
		baseDomain: cfg.BaseDomain,
		logger:     logging.OrNop(logger),
	}
}

// ListUpcoming1on1s queries the Lark Calendar API for 1:1 meetings within
// the given window for the specified member.
func (p *larkCalendarProvider) ListUpcoming1on1s(ctx context.Context, memberID string, window time.Duration) ([]domain.Meeting, error) {
	if p.appID == "" || p.appSecret == "" {
		return nil, fmt.Errorf("lark calendar: missing app credentials")
	}

	p.logger.Info("Lark calendar: listing upcoming 1:1s for %s (window=%s) [stub]", memberID, window)

	return nil, nil
}
