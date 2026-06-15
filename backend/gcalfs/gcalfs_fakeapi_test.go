package gcalfs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/gcalfs/api"
	"github.com/rclone/rclone/lib/rest"
)

// TestMain wires a fake Google Calendar API server into testCalSrv so the
// API-dependent tests run without network or OAuth.
func TestMain(m *testing.M) {
	server := httptest.NewServer(http.HandlerFunc(fakeCalendarHandler))
	testCalSrv = rest.NewClient(server.Client()).SetRoot(server.URL)
	code := m.Run()
	server.Close()
	os.Exit(code)
}

func fakeCalendarHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	enc := json.NewEncoder(w)
	switch {
	case strings.Contains(path, "calendarList"):
		_ = enc.Encode(api.CalendarList{
			Items: []api.CalendarListEntry{
				{ID: "abcdef1234567890", Summary: "Shared"},
				{ID: "zyxwvu9876543210", Summary: "Shared"},
			},
		})
	case strings.Contains(path, "calendars/work@x/events/evt123"):
		_ = enc.Encode(api.Event{
			ID:      "evt123",
			Summary: "Team Sync",
			Updated: time.Date(2024, 1, 15, 9, 30, 0, 0, time.UTC),
			Start:   api.EventDateTime{DateTime: "2024-01-15T10:00:00Z"},
			End:     api.EventDateTime{DateTime: "2024-01-15T11:00:00Z"},
		})
	case strings.Contains(path, "calendars/work@x/events"):
		if r.URL.Query().Get("pageToken") == "" {
			_ = enc.Encode(api.EventList{
				Items: []api.Event{
					{ID: "evt1", Summary: "Event One", Updated: time.Now(), Start: api.EventDateTime{DateTime: "2024-01-15T09:00:00Z"}, End: api.EventDateTime{DateTime: "2024-01-15T10:00:00Z"}},
					{ID: "evt2", Summary: "Event Two", Updated: time.Now(), Start: api.EventDateTime{DateTime: "2024-01-15T10:00:00Z"}, End: api.EventDateTime{DateTime: "2024-01-15T11:00:00Z"}},
					{ID: "evt3", Summary: "Event Three", Updated: time.Now(), Start: api.EventDateTime{DateTime: "2024-01-15T11:00:00Z"}, End: api.EventDateTime{DateTime: "2024-01-15T12:00:00Z"}},
				},
				NextPageToken: "page2",
			})
		} else {
			_ = enc.Encode(api.EventList{
				Items: []api.Event{
					{ID: "evt4", Summary: "Event Four", Updated: time.Now(), Start: api.EventDateTime{DateTime: "2024-01-15T13:00:00Z"}, End: api.EventDateTime{DateTime: "2024-01-15T14:00:00Z"}},
					{ID: "evt5", Summary: "Event Five", Updated: time.Now(), Start: api.EventDateTime{DateTime: "2024-01-15T14:00:00Z"}, End: api.EventDateTime{DateTime: "2024-01-15T15:00:00Z"}},
				},
			})
		}
	case strings.Contains(path, "calendars/empty@x/events"):
		_ = enc.Encode(api.EventList{Items: nil})
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}
