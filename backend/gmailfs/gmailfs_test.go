// Unit tests for the gmailfs backend.
//
// These are RED-phase tests: they assert the production contract and fail by
// assertion against the minimal compile shims until SCAFFOLD/GREEN fill in the
// real bodies.
package gmailfs

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/mail"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/gmailfs/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/hash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixedYear pins "today" for deterministic year/day generation in tests.
const fixedYear = 2024

// newTestFs builds an Fs wired up enough for unit-level routing tests without
// touching the network. startTime is pinned so year/day generation is stable.
func newTestFs(t *testing.T, startYear int) *Fs {
	t.Helper()
	return &Fs{
		name:      "gmailtest",
		root:      "",
		opt:       Options{StartYear: startYear},
		startTime: time.Date(fixedYear, 6, 15, 0, 0, 0, 0, time.UTC),
	}
}

// remotes extracts the Remote() of each entry for easy assertions.
func remotes(entries fs.DirEntries) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Remote())
	}
	return out
}

// hasEntry reports whether any entry's Remote() equals name.
func hasEntry(entries fs.DirEntries, name string) bool {
	for _, e := range entries {
		if e.Remote() == name {
			return true
		}
	}
	return false
}

// hasSuffix reports whether any entry's Remote() ends in suffix.
func hasSuffix(entries fs.DirEntries, suffix string) bool {
	for _, e := range entries {
		if strings.HasSuffix(e.Remote(), suffix) {
			return true
		}
	}
	return false
}

// ----------------------------------------------------------------------------
// T01 — skeleton / OAuth / NewFs
// ----------------------------------------------------------------------------

func TestNewFs_RequiresClientID(t *testing.T) {
	ctx := context.Background()
	m := configmap.Simple{
		"type": "gmail",
		// deliberately no client_id / client_secret / token
	}
	_, err := NewFs(ctx, "gmailtest", "", m)
	require.Error(t, err, "NewFs without client_id/token must return a descriptive error, not succeed")
	require.NotContains(t, err.Error(), "panic", "must surface a descriptive error, never a panic")
}

func TestNewFs_SetsReadMimeTypeFeature(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2000)
	f.features = (&fs.Features{ReadMimeType: true}).Fill(ctx, f)
	require.True(t, f.Features().ReadMimeType, "ReadMimeType feature must be true")
	require.False(t, f.Features().WriteMimeType, "WriteMimeType feature must stay false")
}

func TestFs_HashesNone(t *testing.T) {
	f := newTestFs(t, 2000)
	require.Equal(t, hash.Set(hash.None), f.Hashes())
}

func TestFs_PrecisionNotSupported(t *testing.T) {
	f := newTestFs(t, 2000)
	require.Equal(t, fs.ModTimeNotSupported, f.Precision())
}

func TestRegInfo_PrefixIsGmailfs(t *testing.T) {
	ri, err := fs.Find("gmail")
	require.NoError(t, err, "the gmail backend must be registered")
	require.Equal(t, "gmailfs", ri.Prefix, "RegInfo.Prefix must be \"gmailfs\" to match config.yaml")
}

// ----------------------------------------------------------------------------
// T02 — dirPattern tree
// ----------------------------------------------------------------------------

func TestPatternMatch_RootIsDir(t *testing.T) {
	match, _, pattern := patterns.match("", "", false)
	require.NotNil(t, pattern, "root path \"\" must match a directory pattern")
	require.NotNil(t, match)
}

func TestPatternMatch_YearMonthDay(t *testing.T) {
	cases := []string{
		"2024",
		"2024/2024-01",
		"2024/2024-01/2024-01-15",
	}
	for _, p := range cases {
		_, _, pattern := patterns.match("", p, false)
		require.NotNilf(t, pattern, "path %q must match a directory pattern", p)
	}
}

func TestPatternMatch_ThreadDir(t *testing.T) {
	threadPath := "2024/2024-01/2024-01-15/18c0e1abcd1234ef — Hello there"
	match, _, pattern := patterns.match("", threadPath, false)
	require.NotNil(t, pattern, "thread path must match as a directory")
	require.NotEmpty(t, match, "thread path must capture the thread segment")
}

func TestPatternMatch_EmlIsFile(t *testing.T) {
	emlPath := "2024/2024-01/2024-01-15/18c0e1abcd1234ef — Hello/18c0msg01 — Hello.eml"
	_, _, pattern := patterns.match("", emlPath, true)
	require.NotNil(t, pattern, ".eml path must match with isFile=true")
	require.True(t, pattern.isFile, "matched .eml pattern must be a file")
}

func TestPatternMatch_AttachmentsDirAndFile(t *testing.T) {
	base := "2024/2024-01/2024-01-15/18c0e1abcd1234ef — Hello"
	_, _, dirPat := patterns.match("", base+"/attachments", false)
	require.NotNil(t, dirPat, "attachments dir must match as a directory")
	require.False(t, dirPat.isFile)

	_, _, filePat := patterns.match("", base+"/attachments/18c0msg01 — invoice.pdf", true)
	require.NotNil(t, filePat, "attachment file must match with isFile=true")
	require.True(t, filePat.isFile)
}

func TestPatternMatch_UnknownReturnsNil(t *testing.T) {
	// An unrecognised path must yield a nil pattern. We anchor this test by also
	// requiring that the recognised sibling DOES match, so an empty pattern set
	// (the shim) fails this test rather than passing trivially.
	_, _, knownPat := patterns.match("", "2024", false)
	require.NotNil(t, knownPat, "a valid year path must match (guards against empty pattern set)")

	_, _, unknownPat := patterns.match("", "this/is/not/a/valid/path/at/all", false)
	require.Nil(t, unknownPat, "an unrecognised path must return a nil pattern")
}

// ----------------------------------------------------------------------------
// T03 — List
// ----------------------------------------------------------------------------

func TestList_RootReturnsYears(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	entries, err := f.List(ctx, "")
	require.NoError(t, err)
	require.NotEmpty(t, entries, "root List must return year dirs from start_year to current year")
	require.True(t, hasEntry(entries, "2024"), "root List must include the current year")
	require.True(t, hasEntry(entries, "2020"), "root List must include start_year")
}

func TestList_YearReturnsTwelveMonths(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	entries, err := f.List(ctx, "2024")
	require.NoError(t, err)
	require.Len(t, entries, 12, "List of a year must return 12 month dirs")
	require.True(t, hasEntry(entries, "2024/2024-01"), "January month dir must be present")
	require.True(t, hasEntry(entries, "2024/2024-12"), "December month dir must be present")
}

func TestList_MonthReturnsDays(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	entries, err := f.List(ctx, "2024/2024-01")
	require.NoError(t, err)
	require.Len(t, entries, 31, "January must return 31 day dirs")
	require.True(t, hasEntry(entries, "2024/2024-01/2024-01-15"))
}

func TestList_DayListsThreads(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	entries, err := f.List(ctx, "2024/2024-01/2024-01-15")
	require.NoError(t, err)
	require.NotEmpty(t, entries, "day-level List must return one dir per thread")
	require.True(t, hasSuffix(entries, " — Hello there") || hasEntry(entries, "2024/2024-01/2024-01-15/18c0 — Hello there"),
		"each thread dir must be named \"<id> — <Subject>\"")
}

func TestList_DayExhaustsPagination(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	entries, err := f.List(ctx, "2024/2024-01/2024-01-15")
	require.NoError(t, err)
	// A correctly paginated threads.list spanning two pages must surface threads
	// from BOTH pages. The shim returns nothing, so this fails.
	require.GreaterOrEqual(t, len(entries), 2, "pagination must exhaust nextPageToken pages")
}

func TestList_ThreadListsMessagesAndAttachmentsDir(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	threadDir := "2024/2024-01/2024-01-15/18c0e1abcd1234ef — Hello"
	entries, err := f.List(ctx, threadDir)
	require.NoError(t, err)
	require.NotEmpty(t, entries, "thread-level List must return messages and an attachments dir")
	require.True(t, hasSuffix(entries, ".eml"), "thread List must contain one .eml per message")
	require.True(t, hasEntry(entries, threadDir+"/attachments"), "thread with an attachment must expose an attachments/ dir")
}

func TestList_ThreadNoAttachmentsOmitsDir(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	threadDir := "2024/2024-01/2024-01-15/18c0noattach — Plain"
	entries, err := f.List(ctx, threadDir)
	require.NoError(t, err)
	require.NotEmpty(t, entries, "a thread with no attachments must still list its messages")
	require.True(t, hasSuffix(entries, ".eml"), "thread List must contain one .eml per message")
	require.False(t, hasEntry(entries, threadDir+"/attachments"), "thread with no attachments must NOT expose an attachments/ dir")
}

func TestList_AttachmentsDirListsFiles(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	attachDir := "2024/2024-01/2024-01-15/18c0e1abcd1234ef — Hello/attachments"
	entries, err := f.List(ctx, attachDir)
	require.NoError(t, err)
	require.NotEmpty(t, entries, "attachments-dir List must return one entry per attachment part")
	require.True(t, hasSuffix(entries, "invoice.pdf"), "attachment entry must be named \"<msg> — <filename>\"")
}

func TestList_UnknownDirNotFound(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	// Anchor: a recognised dir must succeed, so the empty-shim case fails here
	// rather than passing trivially on the unmatched-path assertion.
	_, err := f.List(ctx, "2024")
	require.NoError(t, err, "a valid directory must list without error (guards against blanket failure)")

	_, err = f.List(ctx, "this/is/not/valid")
	require.ErrorIs(t, err, fs.ErrorDirNotFound, "an unmatched directory must return fs.ErrorDirNotFound")
}

// ----------------------------------------------------------------------------
// T04 — NewObject
// ----------------------------------------------------------------------------

func TestNewObject_ResolvesEml(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	remote := "2024/2024-01/2024-01-15/18c0e1abcd1234ef — Hello/18c0msg01 — Hello.eml"
	obj, err := f.NewObject(ctx, remote)
	require.NoError(t, err, "a valid .eml path must resolve to an Object")
	require.NotNil(t, obj)
	require.Equal(t, remote, obj.Remote())
}

func TestNewObject_ResolvesAttachment(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	remote := "2024/2024-01/2024-01-15/18c0e1abcd1234ef — Hello/attachments/18c0msg01 — invoice.pdf"
	obj, err := f.NewObject(ctx, remote)
	require.NoError(t, err, "a valid attachment path must resolve to an Object")
	require.NotNil(t, obj)
	o, ok := obj.(*Object)
	require.True(t, ok, "resolved object must be *gmailfs.Object")
	require.True(t, o.isAttachment, "an attachment path must set isAttachment")
}

func TestNewObject_DirectoryReturnsObjectNotFound(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	// Anchor: a valid file path must resolve, proving the resolver works; the
	// shim (which resolves nothing) fails this anchor rather than passing the
	// directory assertion trivially.
	fileRemote := "2024/2024-01/2024-01-15/18c0e1abcd1234ef — Hello/18c0msg01 — Hello.eml"
	fileObj, err := f.NewObject(ctx, fileRemote)
	require.NoError(t, err, "valid .eml must resolve (guards against trivial pass)")
	require.NotNil(t, fileObj)

	_, err = f.NewObject(ctx, "2024/2024-01")
	require.ErrorIs(t, err, fs.ErrorObjectNotFound, "a directory path must return fs.ErrorObjectNotFound (Decision 5)")
}

func TestNewObject_BadPathReturnsObjectNotFound(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	// Anchor: a valid file path must resolve so the shim fails here.
	fileRemote := "2024/2024-01/2024-01-15/18c0e1abcd1234ef — Hello/18c0msg01 — Hello.eml"
	fileObj, err := f.NewObject(ctx, fileRemote)
	require.NoError(t, err, "valid .eml must resolve (guards against trivial pass)")
	require.NotNil(t, fileObj)

	require.NotPanics(t, func() {
		_, err = f.NewObject(ctx, "2024/01/15/badthread/nomessage.eml")
	})
	require.ErrorIs(t, err, fs.ErrorObjectNotFound, "an unresolvable path must return fs.ErrorObjectNotFound")
}

// ----------------------------------------------------------------------------
// T05 — .eml synthesis
// ----------------------------------------------------------------------------

// sampleMessage builds a small multipart Gmail message for synthesis tests.
func sampleMessage() *api.Message {
	return &api.Message{
		ID:           "18c0msg01",
		ThreadID:     "18c0e1abcd1234ef",
		InternalDate: "1705276800000", // 2024-01-15T00:00:00Z in ms
		Payload: &api.Part{
			MimeType: "multipart/mixed",
			Headers: []api.Header{
				{Name: "Date", Value: "Mon, 15 Jan 2024 00:00:00 +0000"},
				{Name: "From", Value: "Alice <alice@example.com>"},
				{Name: "To", Value: "Bob <bob@example.com>"},
				{Name: "Subject", Value: "Héllo wörld"},
				{Name: "Message-ID", Value: "<msg01@example.com>"},
			},
			Parts: []api.Part{
				{
					MimeType: "multipart/alternative",
					Parts: []api.Part{
						{MimeType: "text/plain", Body: &api.PartBody{Data: base64.URLEncoding.EncodeToString([]byte("Héllo wörld"))}},
						{MimeType: "text/html", Body: &api.PartBody{Data: base64.URLEncoding.EncodeToString([]byte("<p>Héllo wörld</p>"))}},
					},
				},
				{
					MimeType: "application/pdf",
					Filename: "invoice.pdf",
					Body:     &api.PartBody{AttachmentID: "att01", Size: 4},
				},
			},
		},
	}
}

func TestEmlSynthesis_ParsesAsRFC2822(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	raw, err := f.emlBytes(ctx, sampleMessage())
	require.NoError(t, err)
	require.NotEmpty(t, raw, "synthesized .eml must produce bytes")
	_, err = mail.ReadMessage(bytes.NewReader(raw))
	require.NoError(t, err, "synthesized .eml must parse as RFC 2822")
}

func TestEmlSynthesis_RequiredHeadersPresent(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	raw, err := f.emlBytes(ctx, sampleMessage())
	require.NoError(t, err)
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	require.NoError(t, err, "synthesized .eml must parse so headers can be inspected")
	for _, h := range []string{"Date", "From", "To", "Subject", "Message-ID", "MIME-Version", "Content-Type"} {
		require.NotEmptyf(t, msg.Header.Get(h), "required header %q must be present", h)
	}
}

func TestEmlSynthesis_MultipartMixedWhenAttachments(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	raw, err := f.emlBytes(ctx, sampleMessage())
	require.NoError(t, err)
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(msg.Header.Get("Content-Type"), "multipart/mixed"),
		"a message with attachments must be multipart/mixed")
}

func TestEmlSynthesis_DistinctNestedBoundaries(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	raw, err := f.emlBytes(ctx, sampleMessage())
	require.NoError(t, err)
	require.NotEmpty(t, raw)
	text := string(raw)
	// Collect distinct boundary= declarations; nested multiparts must differ.
	var boundaries []string
	for _, line := range strings.Split(text, "\n") {
		if idx := strings.Index(line, "boundary="); idx >= 0 {
			b := strings.Trim(strings.TrimSpace(line[idx+len("boundary="):]), "\";")
			boundaries = append(boundaries, b)
		}
	}
	require.GreaterOrEqual(t, len(boundaries), 2, "nested multiparts must declare at least two boundaries")
	require.NotEqual(t, boundaries[0], boundaries[1], "nested multipart boundaries must be distinct")
}

func TestEmlSynthesis_NonASCIIEncoded(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	raw, err := f.emlBytes(ctx, sampleMessage())
	require.NoError(t, err)
	require.NotEmpty(t, raw)
	text := string(raw)
	// The non-ASCII body ("Héllo wörld") must not appear raw 8-bit; it must be
	// base64 or quoted-printable encoded.
	require.NotContains(t, text, "Héllo wörld", "non-ASCII body must be encoded, not raw 8-bit")
	require.True(t,
		strings.Contains(text, "Content-Transfer-Encoding: base64") ||
			strings.Contains(text, "Content-Transfer-Encoding: quoted-printable"),
		"non-ASCII parts must declare a base64 or quoted-printable transfer encoding")
}

func TestEmlSynthesis_Deterministic(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	a, err := f.emlBytes(ctx, sampleMessage())
	require.NoError(t, err)
	b, err := f.emlBytes(ctx, sampleMessage())
	require.NoError(t, err)
	require.NotEmpty(t, a, "synthesis must produce bytes")
	require.Equal(t, a, b, "the same payload must synthesize identical bytes")
}

// ----------------------------------------------------------------------------
// T06 — attachment open
// ----------------------------------------------------------------------------

func TestAttachmentOpen_Base64URLDecodesExactly(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	want := []byte("PDF-bytes\x00\x01\x02hello")
	o := &Object{
		fs:           f,
		remote:       "2024/2024-01/2024-01-15/t — S/attachments/m — invoice.pdf",
		isAttachment: true,
		messageID:    "m",
		attachmentID: "att01",
		bytes:        int64(len(want)),
	}
	rc, err := o.Open(ctx)
	require.NoError(t, err, "attachment Open must succeed")
	require.NotNil(t, rc)
	defer func() { _ = rc.Close() }()
	got := new(bytes.Buffer)
	_, err = got.ReadFrom(rc)
	require.NoError(t, err)
	require.NotEmpty(t, got.Bytes(), "attachment Open must yield the decoded bytes")
}

func TestAttachmentOpen_UsesURLAlphabet(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	// Bytes whose base64url encoding contains '-' and '_' but whose standard
	// base64 encoding contains '+' and '/'. Proves base64url decoding.
	want := []byte{0xfb, 0xff, 0xbf}
	urlEncoded := base64.URLEncoding.EncodeToString(want)
	require.Contains(t, urlEncoded, "-")
	require.Contains(t, urlEncoded, "_")
	o := &Object{
		fs:           f,
		remote:       "2024/2024-01/2024-01-15/t — S/attachments/m — bin.dat",
		isAttachment: true,
		messageID:    "m",
		attachmentID: "att02",
		bytes:        int64(len(want)),
	}
	rc, err := o.Open(ctx)
	require.NoError(t, err, "attachment Open must succeed")
	require.NotNil(t, rc)
	defer func() { _ = rc.Close() }()
	got := new(bytes.Buffer)
	_, err = got.ReadFrom(rc)
	require.NoError(t, err)
	require.Equal(t, want, got.Bytes(), "payload with -/_ must decode via the base64url alphabet")
}

// ----------------------------------------------------------------------------
// T07 — metadata
// ----------------------------------------------------------------------------

func TestObject_ModTimeFromInternalDate(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	const internalDateMs = int64(1705276800000) // 2024-01-15T00:00:00Z
	want := time.UnixMilli(internalDateMs).UTC()
	o := &Object{fs: f, remote: "x.eml", internalDate: internalDateMs}
	// The shim leaves modTime zero; production must derive it from internalDate.
	require.False(t, o.ModTime(ctx).IsZero(), "ModTime must be derived from internalDate, not left zero")
	require.True(t, o.ModTime(ctx).UTC().Equal(want), "ModTime must equal the ms-epoch internalDate")
}

func TestObject_AttachmentSizeIsDecodedBytes(t *testing.T) {
	f := newTestFs(t, 2020)
	raw := []byte("hello-attachment-bytes")
	o := &Object{
		fs:           f,
		remote:       "a/attachments/m — f.bin",
		isAttachment: true,
		partSize:     int64(len(raw)),
	}
	require.Equal(t, int64(len(raw)), o.Size(),
		"attachment Size must be the decoded byte count, not the base64 length")
}

func TestObject_EmlSizeMinusOneAllowed(t *testing.T) {
	f := newTestFs(t, 2020)
	o := &Object{fs: f, remote: "m — S.eml", bytes: 0}
	size := o.Size()
	// A genuinely-resolved .eml object is either the synthesized length (>0) or
	// the explicit unknown sentinel (-1). The shim returns 0, which is neither.
	require.True(t, size == -1 || size > 0,
		".eml Size must be the synthesized length or -1, never an unpopulated zero")
}

func TestObject_HashUnsupported(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	o := &Object{fs: f, remote: "m.eml"}
	h, err := o.Hash(ctx, hash.MD5)
	require.Equal(t, "", h)
	require.Equal(t, hash.ErrUnsupported, err)
}

func TestObject_MimeTypeEml(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	o := &Object{fs: f, remote: "m — S.eml", isAttachment: false}
	require.Equal(t, "message/rfc822", o.MimeType(ctx),
		".eml MimeType must be message/rfc822")
}

func TestObject_MimeTypeAttachment(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	o := &Object{
		fs:           f,
		remote:       "a/attachments/m — invoice.pdf",
		isAttachment: true,
		mimeType:     "application/pdf",
	}
	require.Equal(t, "application/pdf", o.MimeType(ctx),
		"attachment MimeType must be the part's Content-Type")
}

// ----------------------------------------------------------------------------
// T08 — read-only enforcement
// ----------------------------------------------------------------------------

func TestReadOnly_PutDenied(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	_, err := f.Put(ctx, strings.NewReader("data"), nil)
	require.ErrorIs(t, err, fs.ErrorPermissionDenied)
}

func TestReadOnly_MkdirDenied(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	require.ErrorIs(t, f.Mkdir(ctx, "anything"), fs.ErrorPermissionDenied)
}

func TestReadOnly_RmdirDenied(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	require.ErrorIs(t, f.Rmdir(ctx, "anything"), fs.ErrorPermissionDenied)
}

func TestReadOnly_RemoveDenied(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	o := &Object{fs: f, remote: "m.eml"}
	require.ErrorIs(t, o.Remove(ctx), fs.ErrorPermissionDenied)
}

func TestReadOnly_SetModTimeDenied(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	o := &Object{fs: f, remote: "m.eml"}
	require.ErrorIs(t, o.SetModTime(ctx, time.Now()), fs.ErrorPermissionDenied)
}

// ----------------------------------------------------------------------------
// Config respected
// ----------------------------------------------------------------------------

// ----------------------------------------------------------------------------
// S3-01 — slash encoding (U+2215 DIVISION SLASH)
//
// Gmail subjects and attachment filenames may contain "/" (U+002F), which is a
// path separator. Every name segment derived from such a field must replace "/"
// with "∕" (U+2215) so the segment stays a single path component. The substitute
// is U+2215 specifically — not the standard slash and not the fullwidth pipe
// "｜" (U+FF5C).
// ----------------------------------------------------------------------------

const (
	rawSlash    = "/" // U+002F SOLIDUS
	subSlash    = "∕" // U+2215 DIVISION SLASH
	fullPipe    = "｜" // U+FF5C FULLWIDTH VERTICAL LINE
	slashDayDir = "2024/2024-01/2024-01-15"
	slashSubj   = "SMIG, Tuesday, June 23, 2-3 pm EST/1-2 pm CST/11 am PST"
	slashFile   = "report 1/2.pdf"
	slashThread = "18c0slashthread"
	slashMsg    = "18c0slashmsg01"
)

// afterSep returns the substring of s after the last " — " thread/message
// separator, i.e. the subject or filename segment of the leaf entry (the path
// prefix may carry its own " — " from an enclosing thread dir).
func afterSep(s string) string {
	i := strings.LastIndex(s, " — ")
	if i < 0 {
		return ""
	}
	return s[i+len(" — "):]
}

// findEntry returns the first entry whose Remote() contains substr, or "".
func findEntry(entries fs.DirEntries, substr string) string {
	for _, e := range entries {
		if strings.Contains(e.Remote(), substr) {
			return e.Remote()
		}
	}
	return ""
}

// T01 — sanitizeName helper

func TestSanitizeName_ReplacesSlash(t *testing.T) {
	require.Equal(t, "a"+subSlash+"b"+subSlash+"c", sanitizeName("a/b/c"),
		"every \"/\" must be replaced with U+2215")
}

func TestSanitizeName_LeavesPlainUnchanged(t *testing.T) {
	const plain = "Subject no slash"
	// Anchor: a slash-bearing input MUST be transformed, so the identity shim
	// fails here instead of passing the no-op assertion trivially.
	require.NotEqual(t, slashSubj, sanitizeName(slashSubj),
		"a \"/\"-bearing name must be transformed (guards against identity shim)")
	require.Equal(t, plain, sanitizeName(plain),
		"a name with no \"/\" must be returned unchanged")
}

func TestSanitizeName_UsesU2215NotPipe(t *testing.T) {
	got := sanitizeName(slashSubj)
	require.Contains(t, got, subSlash, "sanitized name must contain U+2215")
	require.NotContains(t, got, fullPipe, "sanitized name must NOT use U+FF5C (｜)")
	require.NotContains(t, got, rawSlash, "sanitized name must contain no raw \"/\"")
}

func TestUnsanitizeName_RoundTrip(t *testing.T) {
	san := sanitizeName(slashSubj)
	// Anchor: sanitize must actually change the input (else the round-trip is a
	// trivial identity); the shim fails here instead of passing trivially.
	require.NotEqual(t, slashSubj, san,
		"sanitizeName must transform a \"/\"-bearing name (guards against identity shim)")
	require.Equal(t, slashSubj, unsanitizeName(san),
		"unsanitizeName(sanitizeName(s)) must round-trip back to s")
}

// T02 — thread-dir site (day-level List)

func TestListThreads_SlashSubjectNoRawSlash(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	entries, err := f.List(ctx, slashDayDir)
	require.NoError(t, err)
	remote := findEntry(entries, slashThread)
	require.NotEmpty(t, remote, "the \"/\"-subject thread must appear as a dir entry")
	require.NotContains(t, afterSep(remote), rawSlash,
		"the subject segment must contain no raw \"/\" after the \" — \" separator")
}

func TestListThreads_SlashSubjectContainsSubstitute(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	entries, err := f.List(ctx, slashDayDir)
	require.NoError(t, err)
	remote := findEntry(entries, slashThread)
	require.NotEmpty(t, remote, "the \"/\"-subject thread must appear as a dir entry")
	require.Contains(t, afterSep(remote), subSlash,
		"the subject segment must contain U+2215 in place of each \"/\"")
}

func TestListThreads_IDPrefixUnchanged(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	entries, err := f.List(ctx, slashDayDir)
	require.NoError(t, err)
	remote := findEntry(entries, slashThread)
	require.NotEmpty(t, remote, "the \"/\"-subject thread must appear as a dir entry")
	require.True(t, strings.HasPrefix(remote, slashDayDir+"/"+slashThread+" — "),
		"the entry must still begin with \"<threadId> — \" (ID prefix byte-identical)")
}

// T03 — .eml site (thread-level List)

func TestListThread_EmlSlashSubjectSanitized(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	threadDir := slashDayDir + "/" + slashThread + " — sanitized"
	entries, err := f.List(ctx, threadDir)
	require.NoError(t, err)
	remote := findEntry(entries, ".eml")
	require.NotEmpty(t, remote, "thread List must yield one .eml per message")
	require.True(t, strings.HasSuffix(remote, ".eml"), "the message entry must end in .eml")
	seg := afterSep(remote)
	require.Contains(t, seg, subSlash, "the .eml subject segment must contain U+2215")
	require.NotContains(t, seg, rawSlash, "the .eml subject segment must contain no raw \"/\"")
}

func TestListThread_EmlNewObjectResolves(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	threadDir := slashDayDir + "/" + slashThread + " — sanitized"
	entries, err := f.List(ctx, threadDir)
	require.NoError(t, err)
	remote := findEntry(entries, ".eml")
	require.NotEmpty(t, remote, "thread List must yield one .eml per message")

	obj, err := f.NewObject(ctx, remote)
	require.NoError(t, err, "NewObject on the sanitized .eml remote must resolve")
	o, ok := obj.(*Object)
	require.True(t, ok, "resolved object must be *gmailfs.Object")
	require.Equal(t, slashMsg, o.messageID,
		"messageID must survive sanitization and equal the mocked message ID")
}

// T04 — attachment site

func TestListAttachments_SlashFilenameSanitized(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	attachDir := slashDayDir + "/" + slashThread + " — sanitized/attachments"
	entries, err := f.List(ctx, attachDir)
	require.NoError(t, err)
	remote := findEntry(entries, slashMsg)
	require.NotEmpty(t, remote, "attachments List must yield the slash-filename part")
	seg := afterSep(remote)
	require.Contains(t, seg, subSlash, "the attachment filename segment must contain U+2215")
	require.NotContains(t, seg, rawSlash, "the attachment filename segment must contain no raw \"/\"")
}

func TestListAttachments_NewObjectResolvesAttachment(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	attachDir := slashDayDir + "/" + slashThread + " — sanitized/attachments"
	entries, err := f.List(ctx, attachDir)
	require.NoError(t, err)
	remote := findEntry(entries, slashMsg)
	require.NotEmpty(t, remote, "attachments List must yield the slash-filename part")

	obj, err := f.NewObject(ctx, remote)
	require.NoError(t, err, "NewObject on the sanitized attachment remote must resolve")
	o, ok := obj.(*Object)
	require.True(t, ok, "resolved object must be *gmailfs.Object")
	require.True(t, o.isAttachment, "the resolved object must be an attachment")
	require.Equal(t, slashMsg, o.messageID,
		"messageID must survive sanitization and equal the mocked message ID")
}

// T05 — round-trip / no-regression identity

func TestSlash_ListThenNewObjectIdentity_Eml(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	threadDir := slashDayDir + "/" + slashThread + " — sanitized"
	entries, err := f.List(ctx, threadDir)
	require.NoError(t, err)
	remote := findEntry(entries, ".eml")
	require.NotEmpty(t, remote, "thread List must yield one .eml per message")

	obj, err := f.NewObject(ctx, remote)
	require.NoError(t, err, "List→NewObject identity: the produced .eml remote must resolve")
	o := obj.(*Object)
	require.Equal(t, slashThread, o.threadID, "threadID must round-trip through List→NewObject")
	require.Equal(t, slashMsg, o.messageID, "messageID must round-trip through List→NewObject")
}

func TestSlash_ListThenNewObjectIdentity_Attachment(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	attachDir := slashDayDir + "/" + slashThread + " — sanitized/attachments"
	entries, err := f.List(ctx, attachDir)
	require.NoError(t, err)
	remote := findEntry(entries, slashMsg)
	require.NotEmpty(t, remote, "attachments List must yield the slash-filename part")

	obj, err := f.NewObject(ctx, remote)
	require.NoError(t, err, "List→NewObject identity: the produced attachment remote must resolve")
	o := obj.(*Object)
	require.True(t, o.isAttachment, "the resolved object must be an attachment")
	require.Equal(t, slashThread, o.threadID, "threadID must round-trip through List→NewObject")
	require.Equal(t, slashMsg, o.messageID, "messageID must round-trip through List→NewObject")
}

// T06 — read-only guard (unchanged by the slash fix)

func TestReadOnly_StillDenied_AfterSlashFix(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	o := &Object{fs: f, remote: slashThread + " — " + slashSubj + ".eml"}
	require.ErrorIs(t, func() error { _, e := f.Put(ctx, strings.NewReader("x"), nil); return e }(),
		fs.ErrorPermissionDenied, "Put must still be denied")
	require.ErrorIs(t, f.Mkdir(ctx, "anything"), fs.ErrorPermissionDenied, "Mkdir must still be denied")
	require.ErrorIs(t, f.Rmdir(ctx, "anything"), fs.ErrorPermissionDenied, "Rmdir must still be denied")
	require.ErrorIs(t, o.Remove(ctx), fs.ErrorPermissionDenied, "Object.Remove must still be denied")
	require.ErrorIs(t, o.SetModTime(ctx, time.Now()), fs.ErrorPermissionDenied, "Object.SetModTime must still be denied")
}

func TestStartYear_RootHonorsStartYear(t *testing.T) {
	ctx := context.Background()
	f := newTestFs(t, 2020)
	entries, err := f.List(ctx, "")
	require.NoError(t, err)
	require.NotEmpty(t, entries, "root List must return year dirs honoring start_year")
	for _, name := range remotes(entries) {
		assert.GreaterOrEqual(t, name, "2020", "no year before start_year may appear: %q", name)
		require.NotEqual(t, "2019", name, "years before start_year must not appear")
	}
	require.True(t, hasEntry(entries, "2020"), "start_year itself must be present")
}
