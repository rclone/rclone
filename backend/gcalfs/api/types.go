// Package api provides types used by the Google Calendar API.
package api

import "time"

// Error returned by the Calendar API
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// Error satisfies the error interface
func (e *Error) Error() string { return e.Message }

// CalendarListEntry represents a calendar in the user's list
type CalendarListEntry struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	Primary bool   `json:"primary"`
}

// CalendarList is returned by calendarList.list
type CalendarList struct {
	Items         []CalendarListEntry `json:"items"`
	NextPageToken string              `json:"nextPageToken"`
}

// Event represents a Google Calendar event
type Event struct {
	ID          string        `json:"id"`
	Summary     string        `json:"summary"`
	Description string        `json:"description"`
	Location    string        `json:"location"`
	Updated     time.Time     `json:"updated"`
	Start       EventDateTime `json:"start"`
	End         EventDateTime `json:"end"`
}

// EventDateTime holds the start or end of an event
type EventDateTime struct {
	DateTime string `json:"dateTime"` // RFC 3339 for timed events
	Date     string `json:"date"`     // YYYY-MM-DD for all-day events
	TimeZone string `json:"timeZone"`
}

// EventList is returned by events.list
type EventList struct {
	Items         []Event `json:"items"`
	NextPageToken string  `json:"nextPageToken"`
}

// ensure time is used
var _ = time.Now
