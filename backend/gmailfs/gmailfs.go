// Package gmailfs provides an interface to Gmail
package gmailfs

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/gmailfs/api"
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

// enc is the standard encoder for Google-family backends
const enc = encoder.Base | encoder.EncodeCrLf | encoder.EncodeInvalidUtf8

const (
	rootURL  = "https://gmail.googleapis.com/gmail/v1"
	minSleep = 10 * time.Millisecond
)

// retryErrorCodes is the list of HTTP status codes worth retrying.
var retryErrorCodes = []int{429, 500, 502, 503, 504}

// testSrv, when non-nil, replaces f.srv for API calls so unit tests can point
// the backend at a fake HTTP server. It is nil in production.
var testSrv *rest.Client

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "gmail",
		Prefix:      "gmailfs",
		Description: "Gmail",
		NewFs:       NewFs,
		Options: append(oauthutil.SharedOptions, []fs.Option{{
			Name:    "start_year",
			Help:    "Year limits the photos to be downloaded to those which are uploaded after the given year.",
			Default: 2000,
		}, {
			Name:    "encoding",
			Help:    "The encoding for the backend.",
			Default: enc,
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

// Options for the gmailfs backend
type Options struct {
	StartYear                 int                  `config:"start_year"`
	Enc                       encoder.MultiEncoder `config:"encoding"`
	ServiceAccountFile        string               `config:"service_account_file"`
	ServiceAccountCredentials string               `config:"service_account_credentials"`
	Impersonate               string               `config:"impersonate"`
	EnvAuth                   bool                 `config:"env_auth"`
}

// Fs represents a remote Gmail filesystem
type Fs struct {
	name      string
	root      string
	opt       Options
	features  *fs.Features
	srv       *rest.Client
	pacer     *fs.Pacer
	startTime time.Time
}

// Object describes a Gmail object
type Object struct {
	fs           *Fs
	remote       string
	id           string
	threadID     string
	messageID    string
	internalDate int64
	modTime      time.Time
	bytes        int64
	partSize     int64
	mimeType     string
	isAttachment bool
	attachmentID string
}

var oauthConfig = &oauthutil.Config{
	Scopes:       []string{"https://www.googleapis.com/auth/gmail.readonly"},
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
		return nil, fmt.Errorf("gmail: client_id is required when not using service_account_file or env_auth")
	}
	oAuthClient, _, err := oauthutil.NewClientWithBaseClient(ctx, name, m, oauthConfig, fshttp.NewClient(ctx))
	if err != nil {
		return nil, fmt.Errorf("gmail: failed to configure OAuth: %w", err)
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

	f := &Fs{
		name:      name,
		root:      strings.Trim(root, "/"),
		opt:       *opt,
		srv:       rest.NewClient(oAuthClient).SetRoot(rootURL),
		pacer:     fs.NewPacer(ctx, pacer.NewGoogleDrive(pacer.MinSleep(minSleep))),
		startTime: time.Now(),
	}
	f.features = (&fs.Features{
		ReadMimeType:  true,
		WriteMimeType: false,
	}).Fill(ctx, f)
	return f, nil
}

// Name returns the name of the remote
func (f *Fs) Name() string { return f.name }

// Root returns the root path
func (f *Fs) Root() string { return f.root }

// String converts this Fs to a string
func (f *Fs) String() string { return "Gmail path " + f.root }

// Features returns the optional features
func (f *Fs) Features() *fs.Features { return f.features }

// Precision returns the modtime precision
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// Hashes returns the supported hash sets
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// apiSrv returns the REST client to use for API calls, honoring the test hook.
func (f *Fs) apiSrv() *rest.Client {
	if testSrv != nil {
		return testSrv
	}
	return f.srv
}

// callJSON makes a JSON API call, retrying through the pacer when one is
// configured (production) or directly with a small retry loop otherwise (tests).
func (f *Fs) callJSON(ctx context.Context, opts *rest.Opts, response any) error {
	srv := f.apiSrv()
	call := func() (bool, error) {
		resp, err := srv.CallJSON(ctx, opts, nil, response)
		return shouldRetry(ctx, resp, err)
	}
	if f.pacer != nil {
		return f.pacer.Call(call)
	}
	_, err := srv.CallJSON(ctx, opts, nil, response)
	return err
}

// List lists a directory
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	match, prefix, pattern := patterns.match(f.root, dir, false)
	if pattern == nil || pattern.toEntries == nil {
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
	o := &Object{
		fs:     f,
		remote: remote,
		bytes:  -1,
	}
	// Year patterns produce 6 captures: [full, year, month, day, threadDir, file].
	// Period patterns produce 4 captures: [full, period, threadDir, file].
	var threadDir, fileName string
	if len(match) >= 6 {
		threadDir = match[4]
		fileName = match[5]
	} else {
		threadDir = match[2]
		fileName = match[3]
	}
	o.threadID = strings.SplitN(threadDir, " — ", 2)[0]
	if strings.Contains(remote, "/attachments/") {
		o.isAttachment = true
		o.messageID = strings.SplitN(fileName, " — ", 2)[0]
	} else {
		name := strings.TrimSuffix(fileName, ".eml")
		o.messageID = strings.SplitN(name, " — ", 2)[0]
	}
	return o, nil
}

// Put uploads an object — read-only backend, always denied
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return nil, fs.ErrorPermissionDenied
}

// Mkdir creates a directory — read-only backend, always denied
func (f *Fs) Mkdir(ctx context.Context, dir string) error { return fs.ErrorPermissionDenied }

// Rmdir removes a directory — read-only backend, always denied
func (f *Fs) Rmdir(ctx context.Context, dir string) error { return fs.ErrorPermissionDenied }

// dirTime returns the time to set a directory to
func (f *Fs) dirTime() time.Time { return f.startTime }

// startYear returns the start year
func (f *Fs) startYear() int { return f.opt.StartYear }

// nextDay returns the day after year-month-day as a YYYY/M/D Gmail query date.
func gmailDate(year, month, day string) string {
	return fmt.Sprintf("%s/%s/%s", year, strings.TrimPrefix(month, "0"), strings.TrimPrefix(day, "0"))
}

// gmailDateFromTime formats a time.Time as a Gmail query date (YYYY/M/D).
func gmailDateFromTime(t time.Time) string {
	return gmailDate(
		fmt.Sprintf("%04d", t.Year()),
		fmt.Sprintf("%02d", int(t.Month())),
		fmt.Sprintf("%02d", t.Day()),
	)
}

// listThreadsForRange queries Gmail for threads in [start, end) and returns
// thread dirs under prefix.
func (f *Fs) listThreadsForRange(ctx context.Context, prefix string, start, end time.Time) (fs.DirEntries, error) {
	q := fmt.Sprintf("after:%s before:%s", gmailDateFromTime(start), gmailDateFromTime(end))
	var entries fs.DirEntries
	pageToken := ""
	for {
		params := url.Values{}
		params.Set("q", q)
		params.Set("maxResults", "100")
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}
		opts := rest.Opts{
			Method:     "GET",
			Path:       "/users/me/threads",
			Parameters: params,
		}
		var result api.ThreadList
		if err := f.callJSON(ctx, &opts, &result); err != nil {
			return nil, fmt.Errorf("threads.list: %w", err)
		}
		for _, t := range result.Threads {
			subject := sanitizeName(f.threadSubject(ctx, t.ID))
			entries = append(entries, fs.NewDir(prefix+t.ID+" — "+subject, f.dirTime()))
		}
		if result.NextPageToken == "" {
			break
		}
		pageToken = result.NextPageToken
	}
	return entries, nil
}

// listThreads lists threads for a single day.
func (f *Fs) listThreads(ctx context.Context, prefix, year, month, day string) (fs.DirEntries, error) {
	start, err := time.Parse("2006-01-02", fmt.Sprintf("%s-%s-%s", year, month, day))
	if err != nil {
		return nil, err
	}
	return f.listThreadsForRange(ctx, prefix, start, start.AddDate(0, 0, 1))
}

// periodRange returns the [start, end) time range for the named period.
func periodRange(now time.Time, period string) (start, end time.Time, err error) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	switch period {
	case "today":
		return today, today.AddDate(0, 0, 1), nil
	case "this-week":
		// ISO week: Monday is the first day of the week.
		offset := int(today.Weekday()) - 1
		if offset < 0 {
			offset = 6 // Sunday
		}
		monday := today.AddDate(0, 0, -offset)
		return monday, monday.AddDate(0, 0, 7), nil
	case "last-week":
		offset := int(today.Weekday()) - 1
		if offset < 0 {
			offset = 6
		}
		thisMonday := today.AddDate(0, 0, -offset)
		lastMonday := thisMonday.AddDate(0, 0, -7)
		return lastMonday, thisMonday, nil
	case "this-month":
		first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		return first, first.AddDate(0, 1, 0), nil
	case "this-year":
		first := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
		return first, first.AddDate(1, 0, 0), nil
	}
	return time.Time{}, time.Time{}, fmt.Errorf("unknown period: %s", period)
}

// listPeriodThreads lists all threads in the named time period.
func (f *Fs) listPeriodThreads(ctx context.Context, prefix, period string) (fs.DirEntries, error) {
	start, end, err := periodRange(time.Now(), period)
	if err != nil {
		return nil, err
	}
	return f.listThreadsForRange(ctx, prefix, start, end)
}

// threadSubject fetches a thread's first-message Subject header; on error it
// falls back to the thread ID.
func (f *Fs) threadSubject(ctx context.Context, threadID string) string {
	thread, err := f.getThread(ctx, threadID, "metadata")
	if err != nil || len(thread.Messages) == 0 || thread.Messages[0].Payload == nil {
		return threadID
	}
	if s := headerValue(thread.Messages[0].Payload.Headers, "Subject"); s != "" {
		return s
	}
	return threadID
}

// getThread fetches a thread with the given format.
func (f *Fs) getThread(ctx context.Context, threadID, format string) (*api.Thread, error) {
	params := url.Values{}
	params.Set("format", format)
	if format == "metadata" {
		params.Set("metadataHeaders", "Subject")
	}
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/users/me/threads/" + threadID,
		Parameters: params,
	}
	var thread api.Thread
	if err := f.callJSON(ctx, &opts, &thread); err != nil {
		return nil, fmt.Errorf("threads.get: %w", err)
	}
	return &thread, nil
}

// listThread lists messages in a thread
func (f *Fs) listThread(ctx context.Context, prefix, threadDir string) (fs.DirEntries, error) {
	threadID := strings.SplitN(threadDir, " — ", 2)[0]
	thread, err := f.getThread(ctx, threadID, "full")
	if err != nil {
		return nil, err
	}
	var entries fs.DirEntries
	hasAttachments := false
	for i := range thread.Messages {
		msg := &thread.Messages[i]
		subject := ""
		if msg.Payload != nil {
			subject = sanitizeName(headerValue(msg.Payload.Headers, "Subject"))
		}
		o := &Object{
			fs:           f,
			remote:       prefix + msg.ID + " — " + subject + ".eml",
			threadID:     thread.ID,
			messageID:    msg.ID,
			internalDate: parseInternalDate(msg.InternalDate),
			bytes:        -1,
		}
		// Payload is already in memory (format=full); compute eml size so
		// macFUSE can report a non-zero size in stat() and the kernel issues reads.
		if emlData, err2 := f.emlBytes(ctx, msg); err2 == nil {
			o.bytes = int64(len(emlData))
		}
		entries = append(entries, o)
		if hasAttachmentParts(msg.Payload) {
			hasAttachments = true
		}
	}
	if hasAttachments {
		entries = append(entries, fs.NewDir(prefix+"attachments", f.dirTime()))
	}
	return entries, nil
}

// listAttachments lists attachments
func (f *Fs) listAttachments(ctx context.Context, prefix, threadDir string) (fs.DirEntries, error) {
	threadID := strings.SplitN(threadDir, " — ", 2)[0]
	thread, err := f.getThread(ctx, threadID, "full")
	if err != nil {
		return nil, err
	}
	var entries fs.DirEntries
	for i := range thread.Messages {
		msg := &thread.Messages[i]
		walkAttachments(msg.Payload, func(p *api.Part) {
			o := &Object{
				fs:           f,
				remote:       prefix + msg.ID + " — " + sanitizeName(p.Filename),
				threadID:     thread.ID,
				messageID:    msg.ID,
				internalDate: parseInternalDate(msg.InternalDate),
				isAttachment: true,
				mimeType:     p.MimeType,
			}
			if p.Body != nil {
				o.attachmentID = p.Body.AttachmentID
				o.partSize = p.Body.Size
				o.bytes = p.Body.Size
			}
			entries = append(entries, o)
		})
	}
	return entries, nil
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info { return o.fs }

// String returns a string version
func (o *Object) String() string { return o.remote }

// Remote returns the remote path
func (o *Object) Remote() string { return o.remote }

// Hash returns the hash — unsupported for Gmail
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Size returns the size
func (o *Object) Size() int64 {
	if o.isAttachment {
		return o.partSize
	}
	if o.bytes != 0 {
		return o.bytes
	}
	return -1
}

// ModTime returns the mod time
func (o *Object) ModTime(ctx context.Context) time.Time {
	return time.UnixMilli(o.internalDate).UTC()
}

// SetModTime sets the mod time — read-only backend, always denied
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorPermissionDenied
}

// Storable returns true
func (o *Object) Storable() bool { return true }

// Open opens the object
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	if o.isAttachment {
		opts := rest.Opts{
			Method: "GET",
			Path:   "/users/me/messages/" + o.messageID + "/attachments/" + o.attachmentID,
		}
		var body api.AttachmentBody
		if err := o.fs.callJSON(ctx, &opts, &body); err != nil {
			return nil, fmt.Errorf("attachments.get: %w", err)
		}
		decoded, err := base64.URLEncoding.DecodeString(body.Data)
		if err != nil {
			return nil, fmt.Errorf("attachment decode: %w", err)
		}
		return io.NopCloser(bytes.NewReader(decoded)), nil
	}

	params := url.Values{}
	params.Set("format", "full")
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/users/me/messages/" + o.messageID,
		Parameters: params,
	}
	var msg api.Message
	if err := o.fs.callJSON(ctx, &opts, &msg); err != nil {
		return nil, fmt.Errorf("messages.get: %w", err)
	}
	eml, err := o.fs.emlBytes(ctx, &msg)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(eml)), nil
}

// Update updates the object — read-only backend, always denied
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return fs.ErrorPermissionDenied
}

// Remove removes the object — read-only backend, always denied
func (o *Object) Remove(ctx context.Context) error { return fs.ErrorPermissionDenied }

// MimeType returns the MIME type
func (o *Object) MimeType(ctx context.Context) string {
	if o.isAttachment {
		return o.mimeType
	}
	return "message/rfc822"
}

// headerValue extracts a header value by name (case-insensitive).
func headerValue(headers []api.Header, name string) string {
	for _, h := range headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

// hasAttachmentParts reports whether any part in the tree has a filename.
func hasAttachmentParts(p *api.Part) bool {
	if p == nil {
		return false
	}
	if p.Filename != "" {
		return true
	}
	for i := range p.Parts {
		if hasAttachmentParts(&p.Parts[i]) {
			return true
		}
	}
	return false
}

// walkAttachments calls fn for every part in the tree that has a filename.
func walkAttachments(p *api.Part, fn func(*api.Part)) {
	if p == nil {
		return
	}
	if p.Filename != "" {
		fn(p)
	}
	for i := range p.Parts {
		walkAttachments(&p.Parts[i], fn)
	}
}

// isASCII reports whether s contains only printable/whitespace ASCII bytes.
func isASCII(b []byte) bool {
	for _, c := range b {
		if c > 127 {
			return false
		}
	}
	return true
}

// emlBytes synthesizes the RFC 2822 .eml bytes for a message.
//
// Output is deterministic for identical input: boundaries are derived from each
// part's position in the tree, never from randomness or the clock.
func (f *Fs) emlBytes(ctx context.Context, msg *api.Message) ([]byte, error) {
	var buf bytes.Buffer
	payload := msg.Payload
	if payload == nil {
		payload = &api.Part{MimeType: "text/plain"}
	}

	// Top-level RFC 2822 envelope headers come from the payload. Content-* and
	// MIME-Version are emitted by writePart so they stay consistent with the body.
	for _, h := range payload.Headers {
		if strings.EqualFold(h.Name, "MIME-Version") ||
			strings.EqualFold(h.Name, "Content-Type") ||
			strings.EqualFold(h.Name, "Content-Transfer-Encoding") ||
			strings.EqualFold(h.Name, "Content-Disposition") {
			continue
		}
		val := h.Value
		if strings.EqualFold(h.Name, "Subject") && !isASCII([]byte(val)) {
			val = mime.BEncoding.Encode("utf-8", val)
		}
		fmt.Fprintf(&buf, "%s: %s\r\n", h.Name, val)
	}
	buf.WriteString("MIME-Version: 1.0\r\n")

	writePart(&buf, payload, "0")
	return buf.Bytes(), nil
}

// writePart writes the Content-* headers, the blank separator, and the body of
// one MIME part. boundaryID seeds a deterministic boundary string so that
// nested multiparts each declare a distinct boundary.
func writePart(buf *bytes.Buffer, p *api.Part, boundaryID string) {
	mimeType := p.MimeType
	if mimeType == "" {
		mimeType = "text/plain"
	}

	if strings.HasPrefix(mimeType, "multipart/") {
		boundary := "boundary_" + boundaryID
		fmt.Fprintf(buf, "Content-Type: %s\r\n",
			mime.FormatMediaType(mimeType, map[string]string{"boundary": boundary}))
		buf.WriteString("\r\n")
		for i := range p.Parts {
			fmt.Fprintf(buf, "--%s\r\n", boundary)
			writePart(buf, &p.Parts[i], fmt.Sprintf("%s_%d", boundaryID, i))
			buf.WriteString("\r\n")
		}
		fmt.Fprintf(buf, "--%s--\r\n", boundary)
		return
	}

	// Leaf part.
	raw := decodePartData(p)
	cte := "7bit"
	if !isASCII(raw) {
		cte = "base64"
	}
	ct := mimeType
	switch {
	case p.Filename != "":
		ct = mime.FormatMediaType(mimeType, map[string]string{"name": p.Filename})
	case strings.HasPrefix(mimeType, "text/"):
		ct = mime.FormatMediaType(mimeType, map[string]string{"charset": "utf-8"})
	}
	fmt.Fprintf(buf, "Content-Type: %s\r\n", ct)
	fmt.Fprintf(buf, "Content-Transfer-Encoding: %s\r\n", cte)
	if p.Filename != "" {
		fmt.Fprintf(buf, "Content-Disposition: %s\r\n",
			mime.FormatMediaType("attachment", map[string]string{"filename": p.Filename}))
	}
	buf.WriteString("\r\n")
	buf.Write(encodeBody(raw, cte))
}

// decodePartData returns the raw (decoded) body bytes of a leaf part.
func decodePartData(p *api.Part) []byte {
	if p.Body == nil || p.Body.Data == "" {
		return nil
	}
	decoded, err := base64.URLEncoding.DecodeString(p.Body.Data)
	if err != nil {
		return []byte(p.Body.Data)
	}
	return decoded
}

// encodeBody encodes raw bytes per the given Content-Transfer-Encoding.
func encodeBody(raw []byte, cte string) []byte {
	if cte == "base64" {
		enc := base64.StdEncoding.EncodeToString(raw)
		var out bytes.Buffer
		for len(enc) > 76 {
			out.WriteString(enc[:76])
			out.WriteString("\r\n")
			enc = enc[76:]
		}
		out.WriteString(enc)
		out.WriteString("\r\n")
		return out.Bytes()
	}
	return raw
}

// parseInternalDate converts a Gmail internalDate string (epoch ms) to int64.
func parseInternalDate(s string) int64 {
	ms, _ := strconv.ParseInt(s, 10, 64)
	return ms
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

// Interface satisfaction checks
var (
	_ fs.Fs        = &Fs{}
	_ fs.Object    = &Object{}
	_ fs.MimeTyper = &Object{}
	_ lister       = &Fs{}
)
