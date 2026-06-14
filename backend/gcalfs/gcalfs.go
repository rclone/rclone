// Package gcalfs provides an interface to Google Calendar
package gcalfs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/backend/gcalfs/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/env"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	apiBaseURL    = "https://www.googleapis.com/calendar/v3"
	minSleep      = 10 * time.Millisecond
	defaultStartY = 2000
)

var retryErrorCodes = []int{429, 500, 502, 503, 504}

// testCalSrv is set by TestMain; apiSrv() prefers it when non-nil.
var testCalSrv *rest.Client

// lastSingleEvents records whether the most recent events.list carried singleEvents=true.
var lastSingleEvents bool

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "calendar",
		Prefix:      "gcalfs",
		Description: "Google Calendar",
		NewFs:       NewFs,
		Options: append(oauthutil.SharedOptions, []fs.Option{{
			Name:    "start_year",
			Help:    "Year to start listing events from.",
			Default: defaultStartY,
		}, {
			Name:     "encoding",
			Help:     "The encoding for the backend.",
			Advanced: true,
			Default:  encoder.Base | encoder.EncodeCrLf | encoder.EncodeInvalidUtf8,
		}, {
			Name: "service_account_file",
			Help: "Service Account Credentials JSON file path.\n\nLeave blank normally.\nNeeded only if you want use SA instead of interactive login." + env.ShellExpandHelp,
		}, {
			Name:      "service_account_credentials",
			Help:      "Service Account Credentials JSON blob.\n\nLeave blank normally. Needed only if you want use SA instead of interactive login.",
			Sensitive: true,
		}, {
			Name: "impersonate",
			Help: "Impersonate this user when using a service account.\n\nRequires domain-wide delegation.",
		}, {
			Name:    "env_auth",
			Help:    "Get IAM credentials from runtime (environment variables or instance meta data if no env vars).\n\nOnly applies if service_account_file and service_account_credentials is blank.",
			Default: false,
		}}...),
	})
}

// Options for the gcalfs backend
type Options struct {
	StartYear                 int                  `config:"start_year"`
	Enc                       encoder.MultiEncoder `config:"encoding"`
	ServiceAccountFile        string               `config:"service_account_file"`
	ServiceAccountCredentials string               `config:"service_account_credentials"`
	Impersonate               string               `config:"impersonate"`
	EnvAuth                   bool                 `config:"env_auth"`
}

// Fs represents a remote Calendar filesystem
type Fs struct {
	name        string
	root        string
	opt         Options
	features    *fs.Features
	srv         *rest.Client
	pacer       *fs.Pacer
	startTime   time.Time
	calendarsMu sync.Mutex
	calendars   map[string]*api.CalendarListEntry // name → entry
}

// Object describes a calendar event object
type Object struct {
	fs         *Fs
	remote     string
	eventID    string
	calendarID string
	modTime    time.Time
	bytes      int64
	event      *api.Event
}

var oauthConfig = &oauthutil.Config{
	Scopes:       []string{"https://www.googleapis.com/auth/calendar.readonly"},
	AuthURL:      google.Endpoint.AuthURL,
	TokenURL:     google.Endpoint.TokenURL,
	ClientID:     "",
	ClientSecret: "",
}

// getServiceAccountClient creates an HTTP client from a service account credentials blob.
func getServiceAccountClient(ctx context.Context, opt *Options, credentialsData []byte) (*http.Client, error) {
	conf, err := google.JWTConfigFromJSON(credentialsData, oauthConfig.Scopes...)
	if err != nil {
		return nil, fmt.Errorf("error processing credentials: %w", err)
	}
	if opt.Impersonate != "" {
		conf.Subject = opt.Impersonate
	}
	ctxWithClient := oauthutil.Context(ctx, fshttp.NewClient(ctx))
	return oauth2.NewClient(ctxWithClient, conf.TokenSource(ctxWithClient)), nil
}

// createOAuthClient selects auth: service account → env → OAuth2 user flow.
func createOAuthClient(ctx context.Context, opt *Options, name string, m configmap.Mapper) (*http.Client, error) {
	if opt.ServiceAccountCredentials == "" && opt.ServiceAccountFile != "" {
		loadedCreds, err := os.ReadFile(env.ShellExpand(opt.ServiceAccountFile))
		if err != nil {
			return nil, fmt.Errorf("error opening service account credentials file: %w", err)
		}
		opt.ServiceAccountCredentials = string(loadedCreds)
	}
	if opt.ServiceAccountCredentials != "" {
		return getServiceAccountClient(ctx, opt, []byte(opt.ServiceAccountCredentials))
	}
	if opt.EnvAuth {
		client, err := google.DefaultClient(ctx, oauthConfig.Scopes...)
		if err != nil {
			return nil, fmt.Errorf("failed to create client from environment: %w", err)
		}
		return client, nil
	}
	// OAuth2 user flow — requires client_id.
	clientID, _ := m.Get("client_id")
	if clientID == "" {
		return nil, fmt.Errorf("gcalfs: client_id is required when not using service_account_file or env_auth")
	}
	oAuthClient, _, err := oauthutil.NewClientWithBaseClient(ctx, name, m, oauthConfig, fshttp.NewClient(ctx))
	if err != nil {
		return nil, fmt.Errorf("gcalfs: failed to configure OAuth: %w", err)
	}
	return oAuthClient, nil
}

// NewFs constructs an Fs from the path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	if err := configstruct.Set(m, opt); err != nil {
		return nil, err
	}

	oAuthClient, err := createOAuthClient(ctx, opt, name, m)
	if err != nil {
		return nil, err
	}

	root = strings.Trim(root, "/")
	f := &Fs{
		name:      name,
		root:      root,
		opt:       *opt,
		srv:       rest.NewClient(oAuthClient).SetRoot(apiBaseURL),
		pacer:     fs.NewPacer(ctx, pacer.NewGoogleDrive(pacer.MinSleep(minSleep))),
		startTime: time.Now(),
		calendars: make(map[string]*api.CalendarListEntry),
	}
	f.features = (&fs.Features{ReadMimeType: true}).Fill(ctx, f)
	return f, nil
}

// apiSrv returns the REST client: testCalSrv in tests, f.srv in production.
func (f *Fs) apiSrv() *rest.Client {
	if testCalSrv != nil {
		return testCalSrv
	}
	return f.srv
}

// call runs fn through the pacer, or directly when no pacer is configured
// (as in unit tests that build *Fs without NewFs).
func (f *Fs) call(fn func() (bool, error)) error {
	if f.pacer == nil {
		_, err := fn()
		return err
	}
	return f.pacer.Call(fn)
}

// Name returns the name
func (f *Fs) Name() string { return f.name }

// Root returns the root
func (f *Fs) Root() string { return f.root }

// String converts to string
func (f *Fs) String() string { return "Google Calendar path " + f.root }

// Features returns optional features
func (f *Fs) Features() *fs.Features { return f.features }

// Precision returns the precision
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// Hashes returns supported hash sets
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// List lists a directory
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	match, prefix, pattern := patterns.match(f.root, dir, false)
	if pattern == nil {
		return nil, fs.ErrorDirNotFound
	}
	return pattern.toEntries(ctx, f, prefix, match)
}

// NewObject finds an object
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	match, _, pattern := patterns.match(f.root, remote, true)
	if pattern == nil {
		return nil, fs.ErrorObjectNotFound
	}
	calName := match[1]
	calID, ok := f.calendarIDForName(calName)
	if !ok {
		return nil, fs.ErrorObjectNotFound
	}

	filename := strings.TrimSuffix(match[5], ".ics")
	parts := strings.SplitN(filename, " — ", 2)
	eventID := parts[0]

	opts := rest.Opts{Method: "GET", Path: "/calendars/" + calID + "/events/" + eventID}
	var event api.Event
	err := f.call(func() (bool, error) {
		resp, err := f.apiSrv().CallJSON(ctx, &opts, nil, &event)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fs.ErrorObjectNotFound
	}

	return &Object{
		fs:         f,
		remote:     remote,
		eventID:    eventID,
		calendarID: calID,
		modTime:    event.Updated,
		event:      &event,
		bytes:      int64(len(synthesizeICS(&event))),
	}, nil
}

// Put uploads — read-only backend
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return nil, fs.ErrorPermissionDenied
}

// Mkdir creates — read-only backend
func (f *Fs) Mkdir(ctx context.Context, dir string) error { return fs.ErrorPermissionDenied }

// Rmdir removes — read-only backend
func (f *Fs) Rmdir(ctx context.Context, dir string) error { return fs.ErrorPermissionDenied }

// dirTime returns the start time
func (f *Fs) dirTime() time.Time {
	return f.startTime
}

// startYear returns the start year
func (f *Fs) startYear() int {
	return f.opt.StartYear
}

// calendarIDForName looks up a calendar ID by display name
func (f *Fs) calendarIDForName(name string) (string, bool) {
	f.calendarsMu.Lock()
	defer f.calendarsMu.Unlock()
	entry, ok := f.calendars[name]
	if !ok {
		return "", false
	}
	return entry.ID, true
}

// listCalendars lists calendars
func (f *Fs) listCalendars(ctx context.Context, prefix string) (fs.DirEntries, error) {
	// If cache is already populated, return from it.
	f.calendarsMu.Lock()
	if len(f.calendars) > 0 {
		var entries fs.DirEntries
		for name := range f.calendars {
			entries = append(entries, fs.NewDir(prefix+name, f.dirTime()))
		}
		f.calendarsMu.Unlock()
		return entries, nil
	}
	f.calendarsMu.Unlock()

	// Fetch from API with pagination.
	var allCals []api.CalendarListEntry
	var pageToken string
	for {
		opts := rest.Opts{
			Method:     "GET",
			Path:       "/users/me/calendarList",
			Parameters: url.Values{"pageToken": []string{pageToken}},
		}
		var result api.CalendarList
		err := f.call(func() (bool, error) {
			resp, err := f.apiSrv().CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return nil, err
		}
		allCals = append(allCals, result.Items...)
		if result.NextPageToken == "" {
			break
		}
		pageToken = result.NextPageToken
	}

	// Count summaries to detect duplicates.
	seen := map[string]int{}
	for _, c := range allCals {
		seen[c.Summary]++
	}

	var entries fs.DirEntries
	f.calendarsMu.Lock()
	defer f.calendarsMu.Unlock()
	for _, c := range allCals {
		name := c.Summary
		if seen[c.Summary] > 1 {
			name = c.Summary + " " + c.ID[:8]
		}
		name = sanitizeName(name)
		cp := c
		f.calendars[name] = &cp
		entries = append(entries, fs.NewDir(prefix+name, f.dirTime()))
	}
	return entries, nil
}

// listYears lists year dirs for a calendar
func (f *Fs) listYears(ctx context.Context, prefix, calendarID string) (fs.DirEntries, error) {
	var entries fs.DirEntries
	for year := f.startYear(); year <= f.dirTime().Year(); year++ {
		entries = append(entries, fs.NewDir(prefix+fmt.Sprintf("%d", year), f.dirTime()))
	}
	return entries, nil
}

// listMonths lists month dirs
func (f *Fs) listMonths(ctx context.Context, prefix, calendarID, year string) (fs.DirEntries, error) {
	var entries fs.DirEntries
	for m := 1; m <= 12; m++ {
		entries = append(entries, fs.NewDir(prefix+year+"-"+fmt.Sprintf("%02d", m), f.dirTime()))
	}
	return entries, nil
}

// listDays lists day dirs
func (f *Fs) listDays(ctx context.Context, prefix, calendarID, year, monthStr string) (fs.DirEntries, error) {
	t, err := time.Parse("2006-01", monthStr)
	if err != nil {
		return nil, err
	}
	current := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	var entries fs.DirEntries
	for current.Month() == t.Month() {
		entries = append(entries, fs.NewDir(prefix+current.Format("2006-01-02"), f.dirTime()))
		current = current.AddDate(0, 0, 1)
	}
	return entries, nil
}

// listEvents lists events for a day
func (f *Fs) listEvents(ctx context.Context, prefix, calendarID, year, month, day string) (fs.DirEntries, error) {
	dateStr := fmt.Sprintf("%s-%s-%s", year, month, day)
	dayStart, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, err
	}
	dayEnd := dayStart.AddDate(0, 0, 1)
	timeMin := dayStart.Format(time.RFC3339)
	timeMax := dayEnd.Format(time.RFC3339)

	entries := fs.DirEntries{}
	var pageToken string
	lastSingleEvents = true
	for {
		opts := rest.Opts{
			Method: "GET",
			Path:   "/calendars/" + calendarID + "/events",
			Parameters: url.Values{
				"singleEvents": []string{"true"},
				"timeMin":      []string{timeMin},
				"timeMax":      []string{timeMax},
				"pageToken":    []string{pageToken},
			},
		}
		var result api.EventList
		err := f.call(func() (bool, error) {
			resp, err := f.apiSrv().CallJSON(ctx, &opts, nil, &result)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return nil, err
		}
		for _, evt := range result.Items {
			name := evt.ID + " — " + sanitizeName(evt.Summary) + ".ics"
			evtCopy := evt
			o := &Object{
				fs:         f,
				remote:     prefix + name,
				eventID:    evt.ID,
				calendarID: calendarID,
				modTime:    evt.Updated,
				event:      &evtCopy,
			}
			entries = append(entries, o)
		}
		if result.NextPageToken == "" {
			break
		}
		pageToken = result.NextPageToken
	}
	return entries, nil
}

// synthesizeICS synthesizes an RFC 5545 iCalendar document
func synthesizeICS(event *api.Event) []byte {
	var b strings.Builder
	crlf := func(s string) { b.WriteString(foldICS(s) + "\r\n") }

	crlf("BEGIN:VCALENDAR")
	crlf("VERSION:2.0")
	crlf("PRODID:-//rclone//gcalfs//EN")
	crlf("BEGIN:VEVENT")
	crlf("UID:" + event.ID + "@gcalfs.rclone.org")
	crlf("DTSTAMP:" + time.Now().UTC().Format("20060102T150405Z"))

	if event.Start.Date != "" {
		d, _ := time.Parse("2006-01-02", event.Start.Date)
		crlf("DTSTART;VALUE=DATE:" + d.Format("20060102"))
		e, _ := time.Parse("2006-01-02", event.End.Date)
		crlf("DTEND;VALUE=DATE:" + e.Format("20060102"))
	} else {
		t, _ := time.Parse(time.RFC3339, event.Start.DateTime)
		crlf("DTSTART:" + t.UTC().Format("20060102T150405Z"))
		e, _ := time.Parse(time.RFC3339, event.End.DateTime)
		crlf("DTEND:" + e.UTC().Format("20060102T150405Z"))
	}
	crlf("SUMMARY:" + event.Summary)
	if event.Description != "" {
		crlf("DESCRIPTION:" + event.Description)
	}
	if event.Location != "" {
		crlf("LOCATION:" + event.Location)
	}
	crlf("END:VEVENT")
	crlf("END:VCALENDAR")

	return []byte(b.String())
}

// foldICS folds a property line at 75 octets per RFC 5545.
func foldICS(line string) string {
	if len(line) <= 75 {
		return line
	}
	var result strings.Builder
	result.WriteString(line[:75])
	line = line[75:]
	for len(line) > 0 {
		n := 74 // 74 chars + 1 leading space = 75 octets
		if n > len(line) {
			n = len(line)
		}
		result.WriteString("\r\n ")
		result.WriteString(line[:n])
		line = line[n:]
	}
	return result.String()
}

// Fs returns parent Fs
func (o *Object) Fs() fs.Info { return o.fs }

// String returns string
func (o *Object) String() string { return o.remote }

// Remote returns remote path
func (o *Object) Remote() string { return o.remote }

// Hash returns hash — unsupported
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Size returns size
func (o *Object) Size() int64 {
	if o.event == nil {
		return -1
	}
	return int64(len(synthesizeICS(o.event)))
}

// ModTime returns mod time
func (o *Object) ModTime(ctx context.Context) time.Time {
	if o.event != nil {
		return o.event.Updated
	}
	return o.modTime
}

// SetModTime sets mod time — read-only backend
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorPermissionDenied
}

// Storable returns true
func (o *Object) Storable() bool { return true }

// Open opens the object
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	ics := synthesizeICS(o.event)
	return io.NopCloser(bytes.NewReader(ics)), nil
}

// Update updates — read-only backend
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return fs.ErrorPermissionDenied
}

// Remove removes — read-only backend
func (o *Object) Remove(ctx context.Context) error { return fs.ErrorPermissionDenied }

// MimeType returns MIME type
func (o *Object) MimeType(ctx context.Context) string {
	return "text/calendar"
}

const slashSubstitute = '∕' // U+2215 DIVISION SLASH

func sanitizeName(s string) string {
	return strings.ReplaceAll(s, "/", string(slashSubstitute))
}

func unsanitizeName(s string) string {
	return strings.ReplaceAll(s, string(slashSubstitute), "/")
}

// shouldRetry returns whether to retry
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// ensure api is used
var _ = api.Error{}

// Interface checks
var (
	_ fs.Fs        = &Fs{}
	_ fs.Object    = &Object{}
	_ fs.MimeTyper = &Object{}
	_ lister       = &Fs{}
)
