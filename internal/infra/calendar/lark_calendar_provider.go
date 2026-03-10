// Package calendar implements calendar port adapters for external calendar APIs.
package calendar

import (
	"context"
	"fmt"
	"time"

	domain "alex/internal/domain/calendar"
	"alex/internal/shared/logging"
)

// LarkCalendarProvider implements CalendarPort using the Lark Calendar API.
// This is a stub implementation — the actual Lark API integration will be
// added when API credentials and SDK bindings are available.
type LarkCalendarProvider struct {
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
func NewLarkCalendarProvider(cfg LarkCalendarConfig, logger logging.Logger) *LarkCalendarProvider {
	return &LarkCalendarProvider{
		appID:      cfg.AppID,
		appSecret:  cfg.AppSecret,
		baseDomain: cfg.BaseDomain,
		logger:     logging.OrNop(logger),
	}
}

// ListUpcoming1on1s queries the Lark Calendar API for 1:1 meetings within
// the given window for the specified member.
//
// STUB: Returns an empty slice until Lark Calendar API integration is complete.
func (p *LarkCalendarProvider) ListUpcoming1on1s(ctx context.Context, memberID string, window time.Duration) ([]domain.Meeting, error) {
	if p.appID == "" || p.appSecret == "" {
		return nil, fmt.Errorf("lark calendar: missing app credentials")
	}

	p.logger.Info("Lark calendar: listing upcoming 1:1s for %s (window=%s) [stub]", memberID, window)

	// TODO: Implement Lark Calendar API call:
	// 1. Get tenant access token using appID/appSecret
	// 2. Call /open-apis/calendar/v4/calendars/{calendar_id}/events
	// 3. Filter events with exactly 2 attendees (1:1)
	// 4. Map to domain.Meeting

	return nil, nil
}
