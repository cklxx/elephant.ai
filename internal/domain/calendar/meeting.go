// Package calendar defines the domain model for calendar meetings.
package calendar

import "time"

// Meeting represents a calendar event with participant metadata.
type Meeting struct {
	ID           string
	Title        string
	Participants []string  // user/member IDs
	StartTime    time.Time
	EndTime      time.Time
	Is1on1       bool
}
