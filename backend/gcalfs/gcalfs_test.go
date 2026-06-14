// Tests for the Google Calendar backend core.
//
// RED phase: every test fails by assertion against the minimal compile shims.
package gcalfs

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/gcalfs/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/hash"
)

// newTestFs builds an Fs directly with the given options, bypassing OAuth.
func newTestFs(t *testing.T, startYear int) *Fs {
	t.Helper()
	return &Fs{
		name:      "gcalfs",
		root:      "",
		opt:       Options{StartYear: startYear},
		features:  &fs.Features{ReadMimeType: true},
		startTime: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
		calendars: make(map[string]*api.CalendarListEntry),
	}
}

// timedEvent is a fixture timed event.
func timedEvent() *api.Event {
	return &api.Event{
		ID:          "evt123",
		Summary:     "Team Sync",
		Description: "Weekly team sync meeting",
		Location:    "Room 1",
		Updated:     time.Date(2024, 1, 15, 9, 30, 0, 0, time.UTC),
		Start:       api.EventDateTime{DateTime: "2024-01-15T10:00:00Z"},
		End:         api.EventDateTime{DateTime: "2024-01-15T11:00:00Z"},
	}
}

// allDayEvent is a fixture all-day event.
func allDayEvent() *api.Event {
	return &api.Event{
		ID:      "allday1",
		Summary: "Holiday",
		Updated: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Start:   api.EventDateTime{Date: "2024-01-15"},
		End:     api.EventDateTime{Date: "2024-01-16"},
	}
}

// ---- T01 — skeleton / OAuth / NewFs ----

func TestNewFs_RequiresClientID(t *testing.T) {
	ctx := context.Background()
	m := configmap.Simple{} // no client_id, no token
	_, err := NewFs(ctx, "gcalfs", "", m)
	if err == nil {
		t.Fatal("expected a descriptive non-nil error when client_id is absent, got nil")
	}
}

func TestNewFs_SetsReadMimeTypeFeature(t *testing.T) {
	f := newTestFs(t, 2000)
	feat := f.Features()
	if !feat.ReadMimeType {
		t.Error("expected Features().ReadMimeType to be true")
	}
	if feat.WriteMimeType {
		t.Error("expected Features().WriteMimeType to be false")
	}
}

func TestFs_HashesNone(t *testing.T) {
	f := newTestFs(t, 2000)
	if got := f.Hashes(); got != hash.Set(hash.None) {
		t.Errorf("Hashes() = %v, want %v", got, hash.Set(hash.None))
	}
}

func TestFs_PrecisionNotSupported(t *testing.T) {
	f := newTestFs(t, 2000)
	if got := f.Precision(); got != fs.ModTimeNotSupported {
		t.Errorf("Precision() = %v, want %v", got, fs.ModTimeNotSupported)
	}
}

func TestRegInfo_PrefixIsGcalfs(t *testing.T) {
	info, err := fs.Find("calendar")
	if err != nil {
		t.Fatalf("could not find registered backend: %v", err)
	}
	if info.Prefix != "gcalfs" {
		t.Errorf("RegInfo.Prefix = %q, want %q", info.Prefix, "gcalfs")
	}
}

// ---- T02 — dirPattern tree ----

func TestPatternMatch_RootIsDir(t *testing.T) {
	m, _, p := patterns.match("", "", false)
	if p == nil {
		t.Fatal("expected root pattern to match \"\" as a directory, got nil")
	}
	_ = m
}

func TestPatternMatch_CalendarCaptured(t *testing.T) {
	_, _, p := patterns.match("", "My Calendar", false)
	if p == nil {
		t.Fatal("expected dynamic root capture to match calendar name as a directory")
	}
	if p.isFile {
		t.Error("calendar level must be a directory, not a file")
	}
}

func TestPatternMatch_YearMonthDay(t *testing.T) {
	cases := []string{
		"My Calendar/2024",
		"My Calendar/2024/2024-01",
		"My Calendar/2024/2024-01/2024-01-15",
	}
	for _, p := range cases {
		_, _, pat := patterns.match("", p, false)
		if pat == nil {
			t.Errorf("expected %q to match a directory level, got nil", p)
			continue
		}
		if pat.isFile {
			t.Errorf("%q must be a directory level, got isFile=true", p)
		}
	}
}

func TestPatternMatch_IcsIsFile(t *testing.T) {
	icsPath := "My Calendar/2024/2024-01/2024-01-15/evt123 — Team Sync.ics"
	_, _, pat := patterns.match("", icsPath, true)
	if pat == nil {
		t.Fatal("expected .ics leaf to match as a file, got nil")
	}
	if !pat.isFile {
		t.Error("expected .ics leaf pattern isFile=true")
	}
}

func TestPatternMatch_UnknownReturnsNil(t *testing.T) {
	_, _, pat := patterns.match("", "no/such/garbage/path.txt", false)
	if pat != nil {
		t.Errorf("expected nil pattern for unrecognized path, got %+v", pat)
	}
}

// ---- T03 — root List / caching / disambiguation ----

func TestRootList_OneDirPerCalendar(t *testing.T) {
	f := newTestFs(t, 2000)
	f.calendars["Work"] = &api.CalendarListEntry{ID: "work@x", Summary: "Work"}
	f.calendars["Home"] = &api.CalendarListEntry{ID: "home@x", Summary: "Home"}
	entries, err := f.List(context.Background(), "")
	if err != nil {
		t.Fatalf("List(\"\") returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected one dir per calendar (2), got %d", len(entries))
	}
}

func TestRootList_PaginationExhausted(t *testing.T) {
	f := newTestFs(t, 2000)
	// Simulate a two-page calendar list materialized into the cache.
	for _, id := range []string{"a", "b", "c", "d"} {
		f.calendars[id] = &api.CalendarListEntry{ID: id, Summary: id}
	}
	entries, err := f.List(context.Background(), "")
	if err != nil {
		t.Fatalf("List(\"\") returned error: %v", err)
	}
	if len(entries) != 4 {
		t.Errorf("expected all 4 calendars across pages, got %d", len(entries))
	}
}

func TestRootList_DisambiguatesDuplicateSummary(t *testing.T) {
	f := newTestFs(t, 2000)
	cals := []api.CalendarListEntry{
		{ID: "abcdef1234567890", Summary: "Shared"},
		{ID: "zyxwvu9876543210", Summary: "Shared"},
	}
	entries, err := f.listCalendars(context.Background(), "")
	if err != nil {
		t.Fatalf("listCalendars returned error: %v", err)
	}
	names := map[string]bool{}
	for _, e := range entries {
		names[e.Remote()] = true
	}
	if len(names) != 2 {
		t.Errorf("expected 2 distinct disambiguated names, got %d: %v", len(names), names)
	}
	for _, c := range cals {
		want := "Shared " + c.ID[:8]
		if !names[want] {
			t.Errorf("expected disambiguated name %q present in %v", want, names)
		}
	}
}

func TestCache_ResolvesNameToCalendarID(t *testing.T) {
	f := newTestFs(t, 2000)
	f.calendars["Work"] = &api.CalendarListEntry{ID: "work@group", Summary: "Work"}
	id, ok := f.calendarIDForName("Work")
	if !ok {
		t.Fatal("expected cache to resolve directory name to calendar ID")
	}
	if id != "work@group" {
		t.Errorf("calendarIDForName = %q, want %q", id, "work@group")
	}
}

// ---- T04 — day List ----

func TestDayList_OneIcsPerEvent(t *testing.T) {
	f := newTestFs(t, 2000)
	f.calendars["Work"] = &api.CalendarListEntry{ID: "work@x", Summary: "Work"}
	entries, err := f.listEvents(context.Background(), "Work/2024/2024-01/2024-01-15/", "work@x", "2024", "01", "15")
	if err != nil {
		t.Fatalf("listEvents returned error: %v", err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Remote(), ".ics") {
			t.Errorf("expected each entry to be a .ics file, got %q", e.Remote())
		}
	}
	if len(entries) == 0 {
		t.Error("expected one .ics entry per event, got none")
	}
}

func TestDayList_SendsSingleEventsTrue(t *testing.T) {
	f := newTestFs(t, 2000)
	f.calendars["Work"] = &api.CalendarListEntry{ID: "work@x", Summary: "Work"}
	_, err := f.listEvents(context.Background(), "Work/2024/2024-01/2024-01-15/", "work@x", "2024", "01", "15")
	if err != nil {
		t.Fatalf("listEvents returned error: %v", err)
	}
	if !lastEventsRequestSingleEvents(f) {
		t.Error("expected events.list request to carry singleEvents=true")
	}
}

func TestDayList_PaginationExhausted(t *testing.T) {
	f := newTestFs(t, 2000)
	f.calendars["Work"] = &api.CalendarListEntry{ID: "work@x", Summary: "Work"}
	entries, err := f.listEvents(context.Background(), "Work/2024/2024-01/2024-01-15/", "work@x", "2024", "01", "15")
	if err != nil {
		t.Fatalf("listEvents returned error: %v", err)
	}
	// A two-page event list (3 + 2) should yield all 5 events.
	if len(entries) != 5 {
		t.Errorf("expected 5 events across two pages, got %d", len(entries))
	}
}

func TestDayList_EmptyDayReturnsEmptySlice(t *testing.T) {
	f := newTestFs(t, 2000)
	f.calendars["Work"] = &api.CalendarListEntry{ID: "empty@x", Summary: "Work"}
	entries, err := f.listEvents(context.Background(), "Work/2024/2024-01/2024-01-20/", "empty@x", "2024", "01", "20")
	if err != nil {
		t.Fatalf("a day with no events must return nil error, got %v", err)
	}
	if entries == nil {
		t.Error("expected an empty (non-nil) slice for a day with no events")
	}
}

// lastEventsRequestSingleEvents reports whether the most recent events.list
// request issued by f carried singleEvents=true. In RED there is no request
// recording, so this is always false (test fails by assertion).
func lastEventsRequestSingleEvents(f *Fs) bool {
	_ = f
	return lastSingleEvents
}

// ---- T05 — NewObject ----

func TestNewObject_ResolvesIcs(t *testing.T) {
	f := newTestFs(t, 2000)
	f.calendars["Work"] = &api.CalendarListEntry{ID: "work@x", Summary: "Work"}
	remote := "Work/2024/2024-01/2024-01-15/evt123 — Team Sync.ics"
	o, err := f.NewObject(context.Background(), remote)
	if err != nil {
		t.Fatalf("NewObject for a valid .ics path returned error: %v", err)
	}
	if o.Remote() != remote {
		t.Errorf("Remote() = %q, want %q", o.Remote(), remote)
	}
}

func TestNewObject_DirectoryReturnsObjectNotFound(t *testing.T) {
	f := newTestFs(t, 2000)
	_, err := f.NewObject(context.Background(), "My Calendar/2024")
	if err != fs.ErrorObjectNotFound {
		t.Errorf("NewObject on directory path err = %v, want %v", err, fs.ErrorObjectNotFound)
	}
}

func TestNewObject_BadPathReturnsObjectNotFound(t *testing.T) {
	f := newTestFs(t, 2000)
	_, err := f.NewObject(context.Background(), "Work/2024/2024-01/2024-01-15/nope — none.ics")
	if err != fs.ErrorObjectNotFound {
		t.Errorf("NewObject on nonexistent event err = %v, want %v", err, fs.ErrorObjectNotFound)
	}
}

// ---- T06 — .ics synthesis ----

func icsBytes(t *testing.T, ev *api.Event) []byte {
	t.Helper()
	b := synthesizeICS(ev)
	if b == nil {
		t.Fatal("synthesizeICS returned nil")
	}
	return b
}

func TestIcsSynthesis_AllLinesCRLF(t *testing.T) {
	b := icsBytes(t, timedEvent())
	// strip trailing CRLF then ensure every remaining line ended with CRLF.
	trimmed := bytes.TrimSuffix(b, []byte("\r\n"))
	if bytes.Contains(bytes.ReplaceAll(trimmed, []byte("\r\n"), []byte("")), []byte("\n")) {
		t.Error("found a bare LF — every line must end with CRLF")
	}
	if !bytes.Contains(b, []byte("\r\n")) {
		t.Error("expected CRLF line endings throughout")
	}
}

func TestIcsSynthesis_MandatoryProperties(t *testing.T) {
	s := string(icsBytes(t, timedEvent()))
	for _, want := range []string{
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID", "BEGIN:VEVENT",
		"UID:", "DTSTAMP:", "DTSTART", "DTEND", "SUMMARY:",
		"END:VEVENT", "END:VCALENDAR",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("synthesized .ics missing mandatory property %q", want)
		}
	}
}

func TestIcsSynthesis_LineFoldingAt75Octets(t *testing.T) {
	ev := timedEvent()
	ev.Description = strings.Repeat("A", 200)
	b := icsBytes(t, ev)
	for _, line := range bytes.Split(b, []byte("\r\n")) {
		if len(line) > 75 {
			t.Errorf("line exceeds 75 octets (%d): %q", len(line), line)
		}
	}
	// folded continuation lines begin with a single space.
	if !bytes.Contains(b, []byte("\r\n ")) {
		t.Error("expected folded continuation lines (CRLF + space)")
	}
}

func TestIcsSynthesis_AllDayUsesValueDate(t *testing.T) {
	s := string(icsBytes(t, allDayEvent()))
	if !strings.Contains(s, "DTSTART;VALUE=DATE:20240115") {
		t.Errorf("all-day event must render DTSTART;VALUE=DATE:20240115, got:\n%s", s)
	}
	if strings.Contains(s, "DTSTART;VALUE=DATE:20240115T") {
		t.Error("all-day DTSTART must not carry a time component")
	}
}

func TestIcsSynthesis_TimedEventUsesUTC(t *testing.T) {
	s := string(icsBytes(t, timedEvent()))
	if !strings.Contains(s, "DTSTART:20240115T100000Z") {
		t.Errorf("timed event must render DTSTART:20240115T100000Z, got:\n%s", s)
	}
}

func TestIcsSynthesis_DeterministicExceptDtstamp(t *testing.T) {
	ev := timedEvent()
	a := icsBytes(t, ev)
	b := icsBytes(t, ev)
	strip := func(in []byte) string {
		var out []string
		for _, line := range strings.Split(string(in), "\r\n") {
			if strings.HasPrefix(line, "DTSTAMP") {
				continue
			}
			out = append(out, line)
		}
		return strings.Join(out, "\r\n")
	}
	if strip(a) != strip(b) {
		t.Error("synthesized .ics must be deterministic apart from DTSTAMP")
	}
}

// ---- T07 — metadata ----

func TestObject_ModTimeFromUpdated(t *testing.T) {
	f := newTestFs(t, 2000)
	ev := timedEvent()
	o := &Object{fs: f, remote: "Work/2024/2024-01/2024-01-15/evt123 — Team Sync.ics", event: ev}
	got := o.ModTime(context.Background())
	if !got.Equal(ev.Updated) {
		t.Errorf("ModTime = %v, want event.Updated %v", got, ev.Updated)
	}
}

func TestObject_SizeMinusOneAllowed(t *testing.T) {
	f := newTestFs(t, 2000)
	o := &Object{fs: f, event: timedEvent()}
	got := o.Size()
	if got == 0 {
		t.Errorf("Size() = 0, want synthesized length or -1")
	}
}

func TestObject_HashUnsupported(t *testing.T) {
	f := newTestFs(t, 2000)
	o := &Object{fs: f, event: timedEvent()}
	h, err := o.Hash(context.Background(), hash.MD5)
	if h != "" || err != hash.ErrUnsupported {
		t.Errorf("Hash = (%q, %v), want (\"\", %v)", h, err, hash.ErrUnsupported)
	}
}

func TestObject_MimeTypeCalendar(t *testing.T) {
	f := newTestFs(t, 2000)
	o := &Object{fs: f, event: timedEvent()}
	if got := o.MimeType(context.Background()); got != "text/calendar" {
		t.Errorf("MimeType = %q, want %q", got, "text/calendar")
	}
}

// ---- T08 — read-only enforcement ----

func TestReadOnly_PutDenied(t *testing.T) {
	f := newTestFs(t, 2000)
	_, err := f.Put(context.Background(), strings.NewReader("x"), nil)
	if err != fs.ErrorPermissionDenied {
		t.Errorf("Put err = %v, want %v", err, fs.ErrorPermissionDenied)
	}
}

func TestReadOnly_MkdirDenied(t *testing.T) {
	f := newTestFs(t, 2000)
	if err := f.Mkdir(context.Background(), "x"); err != fs.ErrorPermissionDenied {
		t.Errorf("Mkdir err = %v, want %v", err, fs.ErrorPermissionDenied)
	}
}

func TestReadOnly_RmdirDenied(t *testing.T) {
	f := newTestFs(t, 2000)
	if err := f.Rmdir(context.Background(), "x"); err != fs.ErrorPermissionDenied {
		t.Errorf("Rmdir err = %v, want %v", err, fs.ErrorPermissionDenied)
	}
}

func TestReadOnly_RemoveDenied(t *testing.T) {
	f := newTestFs(t, 2000)
	o := &Object{fs: f, event: timedEvent()}
	if err := o.Remove(context.Background()); err != fs.ErrorPermissionDenied {
		t.Errorf("Remove err = %v, want %v", err, fs.ErrorPermissionDenied)
	}
}

func TestReadOnly_SetModTimeDenied(t *testing.T) {
	f := newTestFs(t, 2000)
	o := &Object{fs: f, event: timedEvent()}
	if err := o.SetModTime(context.Background(), time.Now()); err != fs.ErrorPermissionDenied {
		t.Errorf("SetModTime err = %v, want %v", err, fs.ErrorPermissionDenied)
	}
}

// ---- Config respected ----

func TestStartYear_YearListHonorsStartYear(t *testing.T) {
	f := newTestFs(t, 2022)
	f.calendars["Work"] = &api.CalendarListEntry{ID: "work@x", Summary: "Work"}
	entries, err := f.listYears(context.Background(), "Work/", "work@x")
	if err != nil {
		t.Fatalf("listYears returned error: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected year directory entries, got none")
	}
	for _, e := range entries {
		name := e.Remote()
		// each entry is a 4-digit year; none may precede start_year.
		if name < "2022" {
			t.Errorf("year %q precedes start_year 2022", name)
		}
	}
}

// ensure io is used (Open reader assertions in future GREEN reads)
var _ = io.EOF
