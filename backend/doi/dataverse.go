// Implementation for Dataverse

package doi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/doi/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/rest"
)

// IngestFormat selects which form of a Dataverse tabular-ingest file the
// mount surfaces.
type IngestFormat string

const (
	// IngestFormatOriginal surfaces the original upload bytes (fetched
	// with ?format=original) and, on the whole-version listing, the
	// original upload name (.csv, .sav, …).
	IngestFormatOriginal IngestFormat = "original"
	// IngestFormatArchival surfaces the post-ingest archival bytes that
	// Dataverse stores natively (typically .tab).
	IngestFormatArchival IngestFormat = "archival"
)

// treeListLimit is the page size requested from /tree: large enough to
// keep the round-trip count low on big directories, small enough to let
// ListP stream incrementally.
const treeListLimit = 1000

// Returns true if resolvedURL is likely a DOI hosted on a Dataverse intallation
func activateDataverse(resolvedURL *url.URL) (isActive bool) {
	queryValues := resolvedURL.Query()
	persistentID := queryValues.Get("persistentId")
	return persistentID != ""
}

// dataverseVersionEndpoint builds the dataset-version endpoint
// /api/datasets/:persistentId/versions/{version} relative to base. The
// real PID rides in the persistentId query parameter (the path
// placeholder is literal).
func dataverseVersionEndpoint(base *url.URL, pid, version string) *url.URL {
	if version == "" {
		version = api.LatestVersion
	}
	query := url.Values{}
	query.Add("persistentId", pid)
	return base.ResolveReference(&url.URL{
		Path:     "/api/datasets/:persistentId/versions/" + version,
		RawQuery: query.Encode(),
	})
}

// Resolve the main API endpoint for a DOI hosted on a Dataverse installation
func resolveDataverseEndpoint(resolvedURL *url.URL, version string) (provider Provider, endpoint *url.URL, err error) {
	pid := resolvedURL.Query().Get("persistentId")
	return Dataverse, dataverseVersionEndpoint(resolvedURL, pid, version), nil
}

// resolveDataverseDirectEndpoint builds the dataset-version endpoint from
// host + dataset_pid (direct mode), skipping doi.org resolution.
func resolveDataverseDirectEndpoint(opt *Options) (provider Provider, endpoint *url.URL, err error) {
	base, err := url.Parse(strings.TrimRight(opt.Host, "/"))
	if err != nil {
		return "", nil, fmt.Errorf("invalid host %q: %w", opt.Host, err)
	}
	return Dataverse, dataverseVersionEndpoint(base, opt.DatasetPID, opt.Version), nil
}

// datasetPID returns the dataset persistent ID, from the option (direct
// mode) or the resolved endpoint's query (resolved mode).
func (f *Fs) datasetPID() string {
	if f.opt.DatasetPID != "" {
		return f.opt.DatasetPID
	}
	if f.endpoint != nil {
		return f.endpoint.Query().Get("persistentId")
	}
	return ""
}

// dataverseProvider implements the doiProvider interface for Dataverse installations
type dataverseProvider struct {
	f *Fs
}

// ListEntries returns the full list of entries found at the remote,
// regardless of root. This is the whole-version (non-/tree) listing path;
// it is also the connection/auth check at NewFs time. The /tree path
// (when available) never calls this.
func (dp *dataverseProvider) ListEntries(ctx context.Context) (entries []*Object, err error) {
	// Use the cache if populated
	cachedEntries, found := dp.f.cache.GetMaybe("files")
	if found {
		parsedEntries, ok := cachedEntries.([]Object)
		if ok {
			for _, entry := range parsedEntries {
				newEntry := entry
				entries = append(entries, &newEntry)
			}
			return entries, nil
		}
	}

	version, err := dp.fetchVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("readDir failed: %w", err)
	}
	modTime := parseDataverseTime(version.LastUpdateTime)
	for i := range version.Files {
		file := &version.Files[i]
		// Defence-in-depth: a malformed dataset whose directoryLabel
		// contains ".." could otherwise project files outside the dataset
		// root. Drop any such entry and continue.
		if !isSafeDirLabel(file.DirectoryLabel) {
			fs.Logf(dp.f, "skipping file with unsafe directoryLabel %q", file.DirectoryLabel)
			continue
		}
		entries = append(entries, dp.f.newObjectDataverseFile(file, modTime))
	}
	// Populate the cache
	cacheEntries := []Object{}
	for _, entry := range entries {
		cacheEntries = append(cacheEntries, *entry)
	}
	dp.f.cache.Put("files", cacheEntries)
	return entries, nil
}

// callDataverseJSON runs a JSON API call through the pacer and surfaces
// the Dataverse {status,message} error envelope as an error.
func (f *Fs) callDataverseJSON(ctx context.Context, opts *rest.Opts, result interface{ Err() error }) error {
	err := f.pacer.Call(func() (bool, error) {
		res, callErr := f.srv.CallJSON(ctx, opts, nil, result)
		return shouldRetry(ctx, res, callErr)
	})
	if err != nil {
		return err
	}
	return result.Err()
}

// fetchVersion fetches the dataset version's file list + timestamp from
// the /versions/{version} endpoint (same shape in direct and resolved
// mode).
func (dp *dataverseProvider) fetchVersion(ctx context.Context) (*api.DataverseDatasetVersion, error) {
	opts := rest.Opts{
		Method:     "GET",
		Path:       strings.TrimLeft(dp.f.endpoint.EscapedPath(), "/"),
		Parameters: dp.f.endpoint.Query(),
	}
	var result api.DataverseVersionResponse
	if err := dp.f.callDataverseJSON(ctx, &opts, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// newObjectDataverseFile builds an Object from a whole-version file
// entry, applying the ingest_format preference (see the effective*
// helpers). The content URL is built from the file ID + access format.
func (f *Fs) newObjectDataverseFile(file *api.DataverseFile, modTime time.Time) *Object {
	df := &file.DataFile
	ref := &url.URL{Path: fmt.Sprintf("/api/access/datafile/%d", df.ID)}
	if format := f.accessFormat(df); format != "" {
		query := url.Values{}
		query.Add("format", format)
		ref.RawQuery = query.Encode()
	}
	contentURL := f.endpoint.ResolveReference(ref)
	accessStatus := ""
	if file.Restricted {
		accessStatus = "restricted"
	}
	return &Object{
		fs:           f,
		remote:       path.Join(file.DirectoryLabel, f.effectiveLabel(df)),
		contentURL:   contentURL.String(),
		size:         f.effectiveSize(df),
		modTime:      modTime,
		md5:          f.effectiveMD5(df),
		contentType:  f.effectiveContentType(df),
		accessStatus: accessStatus,
	}
}

func newDataverseProvider(f *Fs) doiProvider {
	return &dataverseProvider{
		f: f,
	}
}

// effectiveLabel returns the filename to surface for a file. For ingested
// files under IngestFormatOriginal it is the original upload name;
// otherwise the stored filename.
func (f *Fs) effectiveLabel(df *api.DataverseDataFile) string {
	if df.IsIngested() && IngestFormat(f.opt.IngestFormat) == IngestFormatOriginal {
		return df.OriginalFileName
	}
	return df.Filename
}

// effectiveSize returns the byte length the backend will stream. After
// ingest Dataverse sets dataFile.filesize to the SERVED (archival .tab)
// size and keeps the original upload's size in originalFileSize, so:
//   - ingested + original: the original upload's size (originalFileSize)
//   - ingested + archival, or non-ingested: filesize (the served form)
func (f *Fs) effectiveSize(df *api.DataverseDataFile) int64 {
	if df.IsIngested() && IngestFormat(f.opt.IngestFormat) == IngestFormatOriginal && df.OriginalFileSize > 0 {
		return df.OriginalFileSize
	}
	return df.FileSize
}

// effectiveContentType returns the MIME type of the streamed bytes.
func (f *Fs) effectiveContentType(df *api.DataverseDataFile) string {
	if df.IsIngested() && IngestFormat(f.opt.IngestFormat) == IngestFormatOriginal && df.OriginalFileFormat != "" {
		return df.OriginalFileFormat
	}
	return df.ContentType
}

// effectiveMD5 returns the MD5 matching the streamed bytes, or "" when
// none matches. Dataverse stores the ORIGINAL upload's MD5; in archival
// mode that does not match the .tab bytes, so it is suppressed to keep
// rclone check from comparing mismatched hashes.
func (f *Fs) effectiveMD5(df *api.DataverseDataFile) string {
	if df.IsIngested() && IngestFormat(f.opt.IngestFormat) == IngestFormatArchival {
		return ""
	}
	return df.StoredMD5()
}

// accessFormat returns the ?format= value for fetching the file's bytes.
// Only ingested files under IngestFormatOriginal need ?format=original;
// everything else is fetched in its stored form.
func (f *Fs) accessFormat(df *api.DataverseDataFile) string {
	if df.IsIngested() && IngestFormat(f.opt.IngestFormat) == IngestFormatOriginal {
		return "original"
	}
	return ""
}

// isSafeTreeName reports whether a /tree item name is a single, clean
// path segment — the /tree counterpart of isSafeDirLabel below.
func isSafeTreeName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.Contains(name, "/")
}

// isSafeDirLabel rejects directory labels with a ".." path segment,
// which would traverse outside the dataset root. Dataverse should never
// emit these, but a corrupted or hostile dataset shouldn't be able to
// escape the mount. ".." is only rejected as a whole segment: names
// merely containing consecutive dots (e.g. "results..final") are fine.
func isSafeDirLabel(dir string) bool {
	for _, seg := range strings.Split(dir, "/") {
		if seg == ".." {
			return false
		}
	}
	return true
}

// parseDataverseTime parses Dataverse's RFC3339-ish timestamps, returning
// timeUnset on error so callers can fall back to "unknown".
func parseDataverseTime(s string) time.Time {
	if s == "" {
		return timeUnset
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t
	}
	return timeUnset
}

// treeRequestPath builds the /tree request path for a dataset version.
// The :persistentId placeholder is literal; the real PID rides in the
// query string (set by the caller).
func treeRequestPath(version string) string {
	if version == "" {
		version = api.LatestVersion
	}
	return "/api/datasets/:persistentId/versions/" + url.PathEscape(version) + "/tree"
}

// treeSupported feature-detects the /tree endpoint with a cheap root
// probe (path="", limit=1). It returns true only when the probe answers
// 2xx with a well-formed /tree envelope — a catch-all front-end could
// answer 200 to an unknown API path — and false on a 404 (endpoint
// absent) or any other error. The caller falls back to the whole-version
// listing, so a false here is always safe. The probe also validates the
// connection/auth.
func treeSupported(ctx context.Context, f *Fs, datasetPID, version string) bool {
	if datasetPID == "" {
		return false
	}
	opts := rest.Opts{
		Method: "GET",
		Path:   strings.TrimLeft(treeRequestPath(version), "/"),
		Parameters: url.Values{
			"persistentId": []string{datasetPID},
			"path":         []string{""},
			"limit":        []string{"1"},
		},
		ExtraHeaders: map[string]string{"Accept": "application/json"},
		// Inspect the status ourselves: a 404 must be a clean "false".
		IgnoreStatus: true,
	}
	var res *http.Response
	err := f.pacer.Call(func() (bool, error) {
		var callErr error
		res, callErr = f.srv.Call(ctx, &opts)
		retry, retryErr := shouldRetry(ctx, res, callErr)
		if res != nil && (retry || retryErr != nil) {
			// IgnoreStatus leaves the body open even on error statuses;
			// close it before the pacer re-issues the request.
			_ = res.Body.Close()
		}
		return retry, retryErr
	})
	if err != nil || res == nil {
		return false
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return false
	}
	var tr api.TreeResponse
	return json.NewDecoder(res.Body).Decode(&tr) == nil && tr.Status == "OK"
}

// listTree fetches one page of a single directory level via /tree.
//   - dir: directory to list ("" = root).
//   - cursor: opaque continuation token; "" for the first page.
//   - originals: when true, ingested tabular files report their
//     original-upload downloadUrl/size/checksum instead of the archival form.
func listTree(ctx context.Context, f *Fs, datasetPID, version, dir, cursor string, limit int, originals bool) (*api.TreePage, error) {
	if datasetPID == "" {
		return nil, errors.New("dataset persistent ID is required")
	}
	params := url.Values{
		"persistentId": []string{datasetPID},
		"path":         []string{dir},
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	if originals {
		params.Set("originals", "true")
	}
	opts := rest.Opts{
		Method:       "GET",
		Path:         strings.TrimLeft(treeRequestPath(version), "/"),
		Parameters:   params,
		ExtraHeaders: map[string]string{"Accept": "application/json"},
	}
	var tr api.TreeResponse
	if err := f.callDataverseJSON(ctx, &opts, &tr); err != nil {
		return nil, fmt.Errorf("list tree: %w", err)
	}
	return &tr.Data, nil
}

// listTreeP lists one directory level via the lazy /tree endpoint, paging
// through nextCursor and emitting each page through callback. Folders map
// to fs.Dir; files map to an Object the shared Open path consumes.
// ingest_format selects the form: IngestFormatOriginal requests
// originals=true so files carry their original-upload downloadUrl, size
// and MD5.
func (f *Fs) listTreeP(ctx context.Context, dir string, callback fs.ListRCallback) error {
	treePath := path.Join(f.root, dir)
	if treePath == "." {
		treePath = ""
	}
	originals := IngestFormat(f.opt.IngestFormat) == IngestFormatOriginal

	cursor := ""
	total := 0
	for {
		page, err := listTree(ctx, f, f.datasetPID(), f.opt.Version, treePath, cursor, treeListLimit, originals)
		if err != nil {
			return err
		}
		var entries fs.DirEntries
		for i := range page.Items {
			item := &page.Items[i]
			// Defence-in-depth mirroring isSafeDirLabel on the flat path:
			// a name that is not a single clean path segment could project
			// entries outside the listed directory.
			if !isSafeTreeName(item.Name) {
				fs.Logf(f, "skipping tree item with unsafe name %q", item.Name)
				continue
			}
			if item.IsFolder() {
				entries = append(entries, fs.NewDir(path.Join(dir, item.Name), time.Time{}))
				continue
			}
			entries = append(entries, f.newObjectTreeItem(item, path.Join(dir, item.Name)))
		}
		total += len(entries)
		if len(entries) > 0 {
			if err := callback(entries); err != nil {
				return err
			}
		}
		if page.NextCursor == nil || *page.NextCursor == "" {
			break
		}
		cursor = *page.NextCursor
	}
	// Dataverse has no empty directories (folders exist only by virtue of
	// containing files), so an empty listing for a non-root path means the
	// directory doesn't exist.
	if total == 0 && treePath != "" {
		return fs.ErrorDirNotFound
	}
	return nil
}

// newObjectTreeItem builds an Object for a /tree file entry, carrying the
// endpoint-provided downloadUrl (which already encodes ?format=…) and
// access status so Open fetches exactly the bytes the item's size/MD5
// describe and a 403 can be attributed. remote is the object's path
// relative to the Fs root.
func (f *Fs) newObjectTreeItem(item *api.TreeItem, remote string) *Object {
	contentURL := item.DownloadURL
	if contentURL == "" && item.ID != 0 {
		// Some items (e.g. restricted files) may omit downloadUrl; fall
		// back to the ID-based access endpoint like the whole-version
		// path does.
		ref := &url.URL{Path: fmt.Sprintf("/api/access/datafile/%d", item.ID)}
		if IngestFormat(f.opt.IngestFormat) == IngestFormatOriginal {
			query := url.Values{}
			query.Add("format", "original")
			ref.RawQuery = query.Encode()
		}
		contentURL = ref.String()
	}
	if contentURL != "" && f.endpoint != nil {
		if ref, err := url.Parse(contentURL); err == nil {
			contentURL = f.endpoint.ResolveReference(ref).String()
		}
	}
	return &Object{
		fs:           f,
		remote:       remote,
		contentURL:   contentURL,
		size:         item.Size,
		modTime:      time.Time{},
		md5:          item.MD5(),
		contentType:  item.ContentType,
		accessStatus: item.Access,
	}
}

// treeLevel fetches all pages of one directory level, memoised in the
// Fs cache so per-file NewObject lookups don't re-list the level.
func (f *Fs) treeLevel(ctx context.Context, dir string) ([]api.TreeItem, error) {
	key := "tree:" + dir
	if cached, found := f.cache.GetMaybe(key); found {
		if items, ok := cached.([]api.TreeItem); ok {
			return items, nil
		}
	}
	originals := IngestFormat(f.opt.IngestFormat) == IngestFormatOriginal
	var items []api.TreeItem
	cursor := ""
	for {
		page, err := listTree(ctx, f, f.datasetPID(), f.opt.Version, dir, cursor, treeListLimit, originals)
		if err != nil {
			return nil, err
		}
		items = append(items, page.Items...)
		if page.NextCursor == nil || *page.NextCursor == "" {
			break
		}
		cursor = *page.NextCursor
	}
	f.cache.Put(key, items)
	return items, nil
}

// newObjectTree resolves a single object via /tree: list the parent
// directory level and find the file whose name matches the leaf.
func (f *Fs) newObjectTree(ctx context.Context, remote string) (fs.Object, error) {
	full := path.Join(f.root, remote)
	if full == "" || full == "." {
		// The dataset root is a directory, never an object.
		return nil, fs.ErrorObjectNotFound
	}
	parent := path.Dir(full)
	if parent == "." {
		parent = ""
	}
	leaf := path.Base(full)

	items, err := f.treeLevel(ctx, parent)
	if err != nil {
		return nil, err
	}
	for i := range items {
		item := &items[i]
		if item.IsFolder() || item.Name != leaf {
			continue
		}
		return f.newObjectTreeItem(item, remote), nil
	}
	return nil, fs.ErrorObjectNotFound
}
