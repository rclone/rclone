package bcosv1

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Fixture: a small, fixed set of (key, version) rows modelled on real BCOS v1
// responses. Shapes and quirks (LastModified format, ETag quoting, DeleteMarker
// having no ETag/Size, x-bbg-bcos-meta-* for custom metadata) were captured
// against a live BCOS dev bucket at bcos.dev.blpprofessional.com:8443 and
// cross-checked against the server implementation that generates them,
// bbgithub.dev.bloomberg.com/bcos/bcos-storage-service ->
// src/bloomberg/bcos/v1/storage_service/blueprint/v1/bucket.py, and the API
// reference at https://tutti.prod.bloomberg.com/bcos/docs/v1/api.
// ---------------------------------------------------------------------------

const (
	testBucket  = "testbucket"
	testAccount = "test-account"
	testSecret  = "test-secret"
)

type fakeVersion struct {
	key          string
	versionID    string
	isLatest     bool
	deleteMarker bool
	modTime      time.Time
	body         []byte
	contentType  string
	customMeta   map[string]string // bare names; sent on the wire as x-bbg-bcos-meta-<name>
}

func md5Hex(b []byte) string {
	sum := md5.Sum(b)
	return hex.EncodeToString(sum[:])
}

// newFixture builds the bucket contents used by most tests below:
//   - file1.txt, dir/file2.txt: plain single-version objects (one under a
//     "directory" prefix, to exercise delimiter/CommonPrefixes)
//   - meta.txt: carries x-bbg-bcos-meta-foo, to exercise custom metadata
//   - versioned.txt: 3 revisions, to exercise --bcosv1-versions naming
//   - deleted.txt: one real revision followed by a delete marker as the new
//     latest — absent from the latest-only listing, present (suffixed) in
//     versions mode, matching BCOS's non-destructive DELETE semantics
//     (see the API doc's "DELETE Object (non-destructive)" section).
func newFixture(base time.Time) []fakeVersion {
	at := func(offset time.Duration) time.Time { return base.Add(offset).UTC() }
	return []fakeVersion{
		{key: "file1.txt", versionID: "v-file1-1", isLatest: true, modTime: at(0), body: []byte("hello world"), contentType: "text/plain"},
		{key: "dir/file2.txt", versionID: "v-file2-1", isLatest: true, modTime: at(time.Second), body: []byte("under dir"), contentType: "text/plain"},
		{key: "meta.txt", versionID: "v-meta-1", isLatest: true, modTime: at(2 * time.Second), body: []byte(`{"k":"v"}`), contentType: "application/json", customMeta: map[string]string{"foo": "bar-value"}},
		{key: "versioned.txt", versionID: "v-versioned-1", isLatest: false, modTime: at(3 * time.Second), body: []byte("revision 1")},
		{key: "versioned.txt", versionID: "v-versioned-2", isLatest: false, modTime: at(4 * time.Second), body: []byte("revision 2")},
		{key: "versioned.txt", versionID: "v-versioned-3", isLatest: true, modTime: at(5 * time.Second), body: []byte("revision 3 (latest)")},
		{key: "deleted.txt", versionID: "v-deleted-1", isLatest: false, modTime: at(6 * time.Second), body: []byte("gone but fetchable by version")},
		{key: "deleted.txt", versionID: "v-deleted-2", isLatest: true, deleteMarker: true, modTime: at(7 * time.Second)},
	}
}

// mockBCOS is a minimal httptest-backed stand-in for a BCOS v1 bucket,
// implementing just enough of GET Bucket / GET Bucket Object Versions /
// GET Object / HEAD Object to drive the backend under test.
type mockBCOS struct {
	t        *testing.T
	versions []fakeVersion
	pageSize int // 0 = return everything in one page
}

func (m *mockBCOS) start() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(m.handle))
}

func (m *mockBCOS) handle(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("x-bbg-bcos-account") != testAccount || r.Header.Get("x-bbg-bcos-secret-key") != testSecret {
		writeBCOSError(w, http.StatusForbidden, "InvalidSecurity")
		return
	}
	base := "/v1/" + testBucket
	switch {
	case r.URL.Path == base:
		m.handleBucket(w, r)
	case strings.HasPrefix(r.URL.Path, base+"/"):
		key, err := url.PathUnescape(strings.TrimPrefix(r.URL.Path, base+"/"))
		require.NoError(m.t, err)
		m.handleObject(w, r, key)
	default:
		writeBCOSError(w, http.StatusNotFound, "NoSuchBucket")
	}
}

func writeBCOSError(w http.ResponseWriter, status int, code string) {
	body, _ := xml.Marshal(bcosError{Code: code})
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// bucketEntry is either a real/delete-marker version row or a synthesised
// "directory" row standing in for a run of keys collapsed by the delimiter.
type bucketEntry struct {
	sortKey string // the value used for prefix/marker filtering and pagination
	isDir   bool
	dirName string // set when isDir
	v       fakeVersion
}

// listEntries applies the versions/latest-only, prefix and delimiter rules a
// real BCOS bucket listing would, returning entries in the pagination order.
func (m *mockBCOS) listEntries(versionsMode bool, prefix, delimiter string) []bucketEntry {
	var rows []fakeVersion
	if versionsMode {
		rows = append(rows, m.versions...)
		sort.SliceStable(rows, func(i, j int) bool {
			if rows[i].key != rows[j].key {
				return rows[i].key < rows[j].key
			}
			return rows[i].modTime.After(rows[j].modTime) // newest first within a key, like real BCOS
		})
	} else {
		latest := map[string]fakeVersion{}
		for _, v := range m.versions {
			if v.isLatest {
				latest[v.key] = v
			}
		}
		for _, v := range latest {
			if !v.deleteMarker {
				rows = append(rows, v)
			}
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].key < rows[j].key })
	}

	seenDirs := map[string]bool{}
	var entries []bucketEntry
	for _, v := range rows {
		if !strings.HasPrefix(v.key, prefix) {
			continue
		}
		rest := strings.TrimPrefix(v.key, prefix)
		if delimiter != "" {
			if i := strings.Index(rest, delimiter); i >= 0 {
				dir := prefix + rest[:i+len(delimiter)]
				if !seenDirs[dir] {
					seenDirs[dir] = true
					entries = append(entries, bucketEntry{sortKey: dir, isDir: true, dirName: dir})
				}
				continue
			}
		}
		entries = append(entries, bucketEntry{sortKey: v.key, v: v})
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].sortKey < entries[j].sortKey })
	return entries
}

func (m *mockBCOS) handleBucket(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	prefix := q.Get("prefix")
	delimiter := q.Get("delimiter")
	_, versionsMode := q["versions"]

	entries := m.listEntries(versionsMode, prefix, delimiter)

	marker := q.Get("marker")
	if versionsMode {
		// Simplified: matches by key only, not the composite (key, versionID)
		// pair a real BCOS server uses. This is only safe because tests choose
		// pageSize so no page boundary falls inside one key's run of versions
		// (see TestListVersionsPagination) -- otherwise two rows sharing a key
		// would be indistinguishable to this marker check and pagination could
		// never advance past them.
		marker = q.Get("key-marker")
	}
	start := 0
	if marker != "" {
		start = sort.Search(len(entries), func(i int) bool { return entries[i].sortKey >= marker })
	}
	page := entries[start:]
	truncated := false
	var nextMarker string
	if m.pageSize > 0 && len(page) > m.pageSize {
		truncated = true
		nextMarker = page[m.pageSize].sortKey
		page = page[:m.pageSize]
	}

	if versionsMode {
		res := listVersionsResult{IsTruncated: truncated, NextKeyMarker: nextMarker}
		for _, e := range page {
			if e.isDir {
				res.CommonPrefixes = append(res.CommonPrefixes, xmlPrefix{Prefix: e.dirName})
				continue
			}
			obj := e.v.toXMLObject()
			if e.v.deleteMarker {
				res.DeleteMarkers = append(res.DeleteMarkers, obj)
			} else {
				res.Versions = append(res.Versions, obj)
			}
		}
		writeXML(w, res)
		return
	}
	res := listBucketResult{IsTruncated: truncated, NextMarker: nextMarker}
	for _, e := range page {
		if e.isDir {
			res.CommonPrefixes = append(res.CommonPrefixes, xmlPrefix{Prefix: e.dirName})
			continue
		}
		res.Contents = append(res.Contents, e.v.toXMLObject())
	}
	writeXML(w, res)
}

func writeXML(w http.ResponseWriter, v interface{}) {
	body, err := xml.Marshal(v)
	if err != nil {
		panic(err) // test fixture bug, not a runtime condition to handle gracefully
	}
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (v fakeVersion) toXMLObject() xmlObject {
	o := xmlObject{
		Key:          v.key,
		VersionId:    v.versionID,
		IsLatest:     v.isLatest,
		LastModified: v.modTime.Format("2006-01-02T15:04:05.000Z"),
	}
	if !v.deleteMarker {
		o.ETag = `"` + md5Hex(v.body) + `"`
		o.Size = int64(len(v.body))
		o.StorageClass = "STANDARD"
	}
	return o
}

// findVersion looks up one exact (key, versionID) row, or the latest
// non-deleted row for key when versionID is empty.
func (m *mockBCOS) findVersion(key, versionID string) *fakeVersion {
	if versionID != "" {
		for i := range m.versions {
			if m.versions[i].key == key && m.versions[i].versionID == versionID {
				return &m.versions[i]
			}
		}
		return nil
	}
	for i := range m.versions {
		v := &m.versions[i]
		if v.key == key && v.isLatest {
			if v.deleteMarker {
				return nil // GET/HEAD on the bare key 404s once the latest is a delete marker
			}
			return v
		}
	}
	return nil
}

func (m *mockBCOS) handleObject(w http.ResponseWriter, r *http.Request, key string) {
	v := m.findVersion(key, r.URL.Query().Get("versionId"))
	if v == nil {
		writeBCOSError(w, http.StatusNotFound, "NoSuchKey")
		return
	}
	w.Header().Set("ETag", `"`+md5Hex(v.body)+`"`)
	w.Header().Set("Last-Modified", v.modTime.Format(http.TimeFormat))
	w.Header().Set("x-amz-version-id", v.versionID)
	if v.contentType != "" {
		w.Header().Set("Content-Type", v.contentType)
	}
	// Real BCOS serves custom metadata as x-bbg-bcos-meta-*, never x-amz-meta-*
	// (see the package doc comment and TestMetadata, which fails if the
	// backend ever goes back to reading the wrong prefix).
	for name, val := range v.customMeta {
		w.Header().Set("x-bbg-bcos-meta-"+name, val)
	}

	http.ServeContent(w, r, key, v.modTime, strings.NewReader(string(v.body)))
}

// ---------------------------------------------------------------------------
// Test setup
// ---------------------------------------------------------------------------

func prepare(t *testing.T, m *mockBCOS, extraOpt configmap.Simple) (fs.Fs, *httptest.Server) {
	ts := m.start()
	t.Cleanup(ts.Close)

	opt := configmap.Simple{
		"type":       "bcosv1",
		"endpoint":   ts.URL,
		"account":    testAccount,
		"secret_key": testSecret,
		// NewFs is called directly here (bypassing the config system, same as
		// backend/http's own internal tests do), so registered Option defaults
		// are never injected -- restate the real default explicitly.
		"metadata_source_timestamp": "true",
	}
	for k, v := range extraOpt {
		opt[k] = v
	}

	f, err := NewFs(context.Background(), "TestBcosv1", testBucket, opt)
	require.NoError(t, err)
	return f, ts
}

func remotes(entries fs.DirEntries) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Remote()
	}
	sort.Strings(names)
	return names
}

// ---------------------------------------------------------------------------
// Listing
// ---------------------------------------------------------------------------

func TestListLatestOnly(t *testing.T) {
	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	m := &mockBCOS{t: t, versions: newFixture(base)}
	f, _ := prepare(t, m, nil)

	entries, err := f.List(context.Background(), "")
	require.NoError(t, err)

	// deleted.txt's latest is a delete marker -> absent; versioned.txt shows
	// only its latest (plain-named) revision; dir/ collapses to one Dir entry.
	assert.Equal(t, []string{"dir", "file1.txt", "meta.txt", "versioned.txt"}, remotes(entries))

	for _, e := range entries {
		if e.Remote() == "versioned.txt" {
			o := e.(fs.Object)
			assert.Equal(t, int64(len("revision 3 (latest)")), o.Size())
			hashVal, err := o.Hash(context.Background(), hash.MD5)
			require.NoError(t, err)
			assert.Equal(t, md5Hex([]byte("revision 3 (latest)")), hashVal)
		}
	}
}

func TestListVersions(t *testing.T) {
	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	fixture := newFixture(base)
	m := &mockBCOS{t: t, versions: fixture}
	f, _ := prepare(t, m, configmap.Simple{"versions": "true"})

	entries, err := f.List(context.Background(), "")
	require.NoError(t, err)

	var versionedRev1, versionedRev2 fakeVersion
	for _, v := range fixture {
		if v.key == "versioned.txt" && v.versionID == "v-versioned-1" {
			versionedRev1 = v
		}
		if v.key == "versioned.txt" && v.versionID == "v-versioned-2" {
			versionedRev2 = v
		}
	}
	suffixedRev1 := version.Add("versioned.txt", versionedRev1.modTime)
	suffixedRev2 := version.Add("versioned.txt", versionedRev2.modTime)

	var deletedRev1 fakeVersion
	for _, v := range fixture {
		if v.key == "deleted.txt" && v.versionID == "v-deleted-1" {
			deletedRev1 = v
		}
	}
	suffixedDeleted := version.Add("deleted.txt", deletedRev1.modTime)

	want := []string{"dir", "file1.txt", "meta.txt", "versioned.txt", suffixedDeleted, suffixedRev1, suffixedRev2}
	sort.Strings(want)
	assert.Equal(t, want, remotes(entries))

	// The delete marker itself never becomes an object, only the version
	// underneath it does.
	for _, e := range entries {
		assert.NotContains(t, e.Remote(), "-vTIMESTAMP")
	}
}

func TestListPagination(t *testing.T) {
	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	m := &mockBCOS{t: t, versions: newFixture(base), pageSize: 1}
	f, _ := prepare(t, m, nil)

	entries, err := f.List(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, []string{"dir", "file1.txt", "meta.txt", "versioned.txt"}, remotes(entries))
}

func TestListVersionsPagination(t *testing.T) {
	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	// pageSize=4 splits the 8 raw rows (deleted.txt x2, dir/, file1.txt,
	// meta.txt, versioned.txt x3) into two pages of 4 without a boundary
	// falling inside versioned.txt's 3-row run -- see the mock's key-marker
	// comment in handleBucket for why that constraint matters here.
	m := &mockBCOS{t: t, versions: newFixture(base), pageSize: 4}
	f, _ := prepare(t, m, configmap.Simple{"versions": "true"})

	entries, err := f.List(context.Background(), "")
	require.NoError(t, err)
	assert.Len(t, entries, 7) // same 7 as TestListVersions, just paginated to get there
}

func TestListSubdirDelimiter(t *testing.T) {
	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	m := &mockBCOS{t: t, versions: newFixture(base)}
	f, _ := prepare(t, m, nil)

	entries, err := f.List(context.Background(), "dir")
	require.NoError(t, err)
	assert.Equal(t, []string{"dir/file2.txt"}, remotes(entries))
}

func TestListNonexistentBucket(t *testing.T) {
	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	m := &mockBCOS{t: t, versions: newFixture(base)}
	ts := m.start()
	t.Cleanup(ts.Close)

	opt := configmap.Simple{
		"type":       "bcosv1",
		"endpoint":   ts.URL,
		"account":    testAccount,
		"secret_key": testSecret,
	}
	f, err := NewFs(context.Background(), "TestBcosv1", "no-such-bucket", opt)
	require.NoError(t, err) // NewFs itself never talks to BCOS eagerly here (root == "")

	_, err = f.List(context.Background(), "")
	assert.Equal(t, fs.ErrorDirNotFound, err)
}

// ---------------------------------------------------------------------------
// NewObject / "root is a file"
// ---------------------------------------------------------------------------

func TestNewObjectPlain(t *testing.T) {
	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	m := &mockBCOS{t: t, versions: newFixture(base)}
	f, _ := prepare(t, m, nil)

	o, err := f.NewObject(context.Background(), "file1.txt")
	require.NoError(t, err)
	assert.Equal(t, int64(len("hello world")), o.Size())

	_, err = f.NewObject(context.Background(), "does-not-exist.txt")
	assert.Equal(t, fs.ErrorObjectNotFound, err)
}

func TestNewObjectVersionSuffixed(t *testing.T) {
	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	fixture := newFixture(base)
	m := &mockBCOS{t: t, versions: fixture}
	f, _ := prepare(t, m, configmap.Simple{"versions": "true"})

	var rev1 fakeVersion
	for _, v := range fixture {
		if v.key == "versioned.txt" && v.versionID == "v-versioned-1" {
			rev1 = v
		}
	}
	suffixed := version.Add("versioned.txt", rev1.modTime)

	o, err := f.NewObject(context.Background(), suffixed)
	require.NoError(t, err)
	rc, err := o.Open(context.Background())
	require.NoError(t, err)
	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	assert.Equal(t, "revision 1", string(data))
}

// TestRootIsFile and TestRootIsVersionSuffixedFile guard against a regression
// where NewFs's "root is a file" detection built an Object directly (a literal
// HEAD on the root string) instead of going through NewObject. That always
// 404s for a version-suffixed root, since the suffixed string is never a real
// BCOS key -- only version.Remove recovers the real key. The symptom was
// silent empty results (no error) from `rclone cat`/`copyto` on such a path.
func TestRootIsFile(t *testing.T) {
	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	m := &mockBCOS{t: t, versions: newFixture(base)}
	ts := m.start()
	t.Cleanup(ts.Close)

	opt := configmap.Simple{
		"type":       "bcosv1",
		"endpoint":   ts.URL,
		"account":    testAccount,
		"secret_key": testSecret,
	}
	f, err := NewFs(context.Background(), "TestBcosv1", testBucket+"/file1.txt", opt)
	assert.Equal(t, fs.ErrorIsFile, err)
	require.NotNil(t, f)
	assert.Equal(t, "", f.Root())

	o, err := f.NewObject(context.Background(), "file1.txt")
	require.NoError(t, err)
	rc, err := o.Open(context.Background())
	require.NoError(t, err)
	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	assert.Equal(t, "hello world", string(data))
}

func TestRootIsVersionSuffixedFile(t *testing.T) {
	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	fixture := newFixture(base)
	m := &mockBCOS{t: t, versions: fixture}
	ts := m.start()
	t.Cleanup(ts.Close)

	var rev1 fakeVersion
	for _, v := range fixture {
		if v.key == "versioned.txt" && v.versionID == "v-versioned-1" {
			rev1 = v
		}
	}
	suffixed := version.Add("versioned.txt", rev1.modTime)

	opt := configmap.Simple{
		"type":       "bcosv1",
		"endpoint":   ts.URL,
		"account":    testAccount,
		"secret_key": testSecret,
		"versions":   "true",
	}
	f, err := NewFs(context.Background(), "TestBcosv1", testBucket+"/"+suffixed, opt)
	assert.Equal(t, fs.ErrorIsFile, err)
	require.NotNil(t, f)
	assert.Equal(t, "", f.Root())

	o, err := f.NewObject(context.Background(), suffixed)
	require.NoError(t, err)
	rc, err := o.Open(context.Background())
	require.NoError(t, err)
	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	assert.Equal(t, "revision 1", string(data))
}

// ---------------------------------------------------------------------------
// Open / Range / errors
// ---------------------------------------------------------------------------

func TestOpenRange(t *testing.T) {
	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	m := &mockBCOS{t: t, versions: newFixture(base)}
	f, _ := prepare(t, m, nil)

	o, err := f.NewObject(context.Background(), "file1.txt")
	require.NoError(t, err)

	rc, err := o.Open(context.Background(), &fs.RangeOption{Start: 0, End: 4})
	require.NoError(t, err)
	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	assert.Equal(t, "hello", string(data))
}

func TestOpenNotFound(t *testing.T) {
	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	m := &mockBCOS{t: t, versions: newFixture(base)}
	f, _ := prepare(t, m, nil)

	// deleted.txt's latest is a delete marker, so Open (like a real BCOS GET
	// on the bare key) 404s even though older content still exists.
	bf := f.(*Fs)
	o := &Object{fs: bf, remote: "deleted.txt", key: "deleted.txt"}
	_, err := o.Open(context.Background())
	assert.Equal(t, fs.ErrorObjectNotFound, err)
}

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

func TestMetadata(t *testing.T) {
	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	m := &mockBCOS{t: t, versions: newFixture(base)}
	f, _ := prepare(t, m, nil)

	o, err := f.NewObject(context.Background(), "meta.txt")
	require.NoError(t, err)

	md, err := o.(fs.Metadataer).Metadata(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "application/json", md["content-type"])
	assert.Equal(t, "bar-value", md["foo"]) // x-bbg-bcos-meta-foo -> bare "foo"
	assert.NotEmpty(t, md["source-version-id"])
	assert.NotEmpty(t, md["source-timestamp"])
	assert.NotEmpty(t, md["mtime"])
}

func TestMetadataCustomKeyDoesNotClobberSystemKeys(t *testing.T) {
	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	fixture := newFixture(base)
	for i := range fixture {
		if fixture[i].key == "meta.txt" {
			fixture[i].customMeta = map[string]string{"mtime": "attacker-controlled"}
		}
	}
	m := &mockBCOS{t: t, versions: fixture}
	f, _ := prepare(t, m, nil)

	o, err := f.NewObject(context.Background(), "meta.txt")
	require.NoError(t, err)
	md, err := o.(fs.Metadataer).Metadata(context.Background())
	require.NoError(t, err)
	assert.NotEqual(t, "attacker-controlled", md["mtime"])
}

func TestMetadataSourceTimestampDisabled(t *testing.T) {
	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	m := &mockBCOS{t: t, versions: newFixture(base)}
	f, _ := prepare(t, m, configmap.Simple{"metadata_source_timestamp": "false"})

	o, err := f.NewObject(context.Background(), "file1.txt")
	require.NoError(t, err)
	md, err := o.(fs.Metadataer).Metadata(context.Background())
	require.NoError(t, err)
	_, ok := md["source-timestamp"]
	assert.False(t, ok)
}

// ---------------------------------------------------------------------------
// Read-only enforcement
// ---------------------------------------------------------------------------

func TestReadOnly(t *testing.T) {
	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	m := &mockBCOS{t: t, versions: newFixture(base)}
	f, _ := prepare(t, m, nil)

	_, err := f.Put(context.Background(), strings.NewReader(""), nil)
	assert.Equal(t, fs.ErrorPermissionDenied, err)
	assert.Equal(t, fs.ErrorPermissionDenied, f.Mkdir(context.Background(), ""))
	assert.Equal(t, fs.ErrorPermissionDenied, f.Rmdir(context.Background(), ""))

	o, err := f.NewObject(context.Background(), "file1.txt")
	require.NoError(t, err)
	assert.Equal(t, fs.ErrorCantSetModTime, o.SetModTime(context.Background(), time.Now()))
	assert.Equal(t, fs.ErrorPermissionDenied, o.Update(context.Background(), strings.NewReader(""), nil))
	assert.Equal(t, fs.ErrorPermissionDenied, o.Remove(context.Background()))
}

// ---------------------------------------------------------------------------
// Pure-function unit tests: XML/header parsing, naming, retry classification
// ---------------------------------------------------------------------------

func TestUnquoteETag(t *testing.T) {
	assert.Equal(t, "abc123", unquoteETag(`"ABC123"`))
	assert.Equal(t, "abc123", unquoteETag("abc123"))
}

func TestParseTime(t *testing.T) {
	// BCOS's own listing/header format (millisecond precision, always Z).
	got := parseTime("2026-07-20T17:03:21.121Z")
	assert.Equal(t, 2026, got.Year())
	assert.Equal(t, 121000000, got.Nanosecond())

	// HTTP-date, as returned in Last-Modified on GET/HEAD.
	got = parseTime("Mon, 20 Jul 2026 17:03:21 GMT")
	assert.Equal(t, 2026, got.Year())

	assert.True(t, parseTime("not a time").IsZero())
}

func TestEscapePath(t *testing.T) {
	assert.Equal(t, "a/b%20c/d", escapePath("a/b c/d"))
	assert.Equal(t, "", escapePath(""))
}

func TestSplitBucketPath(t *testing.T) {
	bucket, prefix := splitBucketPath("sandbox/rclone-bcosv1/dir")
	assert.Equal(t, "sandbox", bucket)
	assert.Equal(t, "rclone-bcosv1/dir", prefix)

	bucket, prefix = splitBucketPath("sandbox")
	assert.Equal(t, "sandbox", bucket)
	assert.Equal(t, "", prefix)

	bucket, _ = splitBucketPath("")
	assert.Equal(t, "", bucket)
}

func TestShouldRetry(t *testing.T) {
	for _, status := range []int{429, 500, 502, 503, 504} {
		retry, err := shouldRetry(context.Background(), &http.Response{StatusCode: status}, nil)
		assert.True(t, retry, "status %d should be retryable", status)
		assert.Error(t, err)
	}
	for _, status := range []int{200, 403, 404} {
		retry, err := shouldRetry(context.Background(), &http.Response{StatusCode: status}, nil)
		assert.False(t, retry, "status %d should not be retryable", status)
		assert.NoError(t, err)
	}
}

func TestErrorHandlerParsesXML(t *testing.T) {
	body := `<?xml version="1.0" encoding="UTF-8"?><Error><Code>NoSuchBucket</Code><Message>gone</Message></Error>`
	resp := &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	err := errorHandler(resp)
	var apiErr *bcosError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "NoSuchBucket", apiErr.Code)
	assert.Equal(t, "gone", apiErr.Message)
}

func TestErrorHandlerFallsBackOnNonXMLBody(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Status:     "500 Internal Server Error",
		Body:       io.NopCloser(strings.NewReader("not xml")),
	}
	err := errorHandler(resp)
	var apiErr *bcosError
	assert.False(t, errors.As(err, &apiErr))
	assert.Error(t, err)
}
