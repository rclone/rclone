package bcosv1

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"github.com/rclone/rclone/lib/version"
)

const (
	minSleep      = 10 * time.Millisecond
	maxSleep      = 2 * time.Second
	decayConstant = 2
	listChunk     = 1000
)

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "bcosv1",
		Description: "BCOS v1 (Bloomberg Cloud Object Store, legacy) — read-only migration source",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "endpoint",
			Help:     "BCOS v1 endpoint, e.g. https://bcos.dev.blpprofessional.com:8443",
			Required: true,
		}, {
			Name: "account",
			Help: "BCOS account, sent as the x-bbg-bcos-account header.",
		}, {
			Name:       "secret_key",
			Help:       "BCOS secret key, sent as the x-bbg-bcos-secret-key header.",
			IsPassword: true,
		}, {
			Name:    "versions",
			Help:    "Enumerate every object version (non-latest versions get a -vTIMESTAMP suffixed name). Default: latest only.",
			Default: false,
		}, {
			Name:     "metadata_source_timestamp",
			Help:     "Emit the original BCOS timestamp as source-timestamp metadata (-> x-amz-meta-source-timestamp).",
			Default:  true,
			Advanced: true,
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Endpoint                string `config:"endpoint"`
	Account                 string `config:"account"`
	SecretKey               string `config:"secret_key"`
	Versions                bool   `config:"versions"`
	MetadataSourceTimestamp bool   `config:"metadata_source_timestamp"`
}

// Fs represents a read-only view over one BCOS v1 bucket (+ optional prefix)
type Fs struct {
	name     string
	root     string // prefix within the bucket, no leading/trailing slash
	bucket   string
	opt      Options
	features *fs.Features
	srv      *rest.Client
	pacer    *fs.Pacer
}

// NewFs constructs a bcos Fs from name, root ("bucket[/prefix]") and config
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	if err := configstruct.Set(m, opt); err != nil {
		return nil, err
	}
	if opt.Endpoint == "" {
		return nil, errors.New("bcosv1: endpoint is required")
	}

	bucket, prefix := splitBucketPath(root)
	if bucket == "" {
		return nil, errors.New("bcosv1: remote must include a bucket, e.g. bcosv1:mybucket/path")
	}

	secret := opt.SecretKey
	if secret != "" {
		if revealed, err := obscure.Reveal(secret); err == nil {
			secret = revealed // stored obscured (IsPassword); use plaintext on the wire
		}
	}

	client := fshttp.NewClient(ctx)
	srv := rest.NewClient(client).SetRoot(strings.TrimRight(opt.Endpoint, "/"))
	// x-bbg-bcos-account / x-bbg-bcos-secret-key: the only two auth headers BCOS
	// checks, in validate_account_credentials:
	// https://bbgithub.dev.bloomberg.com/bcos/bcos-storage-service/blob/1.22.19/src/bloomberg/bcos/v1/storage_service/blueprint/v1/__init__.py#L87-L107
	// — a plaintext account name plus a secret key compared against a stored
	// SHA-256 hash (`account_info.secret_key_hash != sha256(sec_key)`), unlike
	// S3's SigV4 request signing. Set once here since they're constant for the
	// Fs's lifetime; every route (list/head/get) goes through the same
	// before_request chain, so there's no per-operation auth variant to
	// special-case.
	srv.SetHeader("x-bbg-bcos-account", opt.Account)
	srv.SetHeader("x-bbg-bcos-secret-key", secret)
	srv.SetErrorHandler(errorHandler)

	f := &Fs{
		name:   name,
		root:   prefix,
		bucket: bucket,
		opt:    *opt,
		srv:    srv,
		pacer:  fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}
	f.features = (&fs.Features{
		ReadMetadata:            true,
		UserMetadata:            true,
		BucketBased:             true,
		CanHaveEmptyDirectories: false,
		SlowModTime:             true,
	}).Fill(ctx, f)

	// Detect "root is a file" so `rclone copy bcos:bkt/dir/file dst` works.
	// Goes through f.NewObject (rather than a bare HEAD on f.root) so a root that
	// is itself a version-suffixed name (e.g. bcos:bkt/dir/file-v2026-...ext) is
	// resolved via the same version.Match/newObjectFromVersionName path NewObject
	// uses elsewhere — a literal HEAD on that suffixed string always 404s, since
	// it's not a real BCOS key.
	if f.root != "" {
		remote := path.Base(f.root)
		newRoot := path.Dir(f.root)
		if newRoot == "." {
			newRoot = ""
		}
		saveRoot := f.root
		f.root = newRoot
		if _, err := f.NewObject(ctx, remote); err == nil {
			return f, fs.ErrorIsFile
		}
		f.root = saveRoot
	}
	return f, nil
}

func splitBucketPath(root string) (bucket, prefix string) {
	root = strings.Trim(root, "/")
	if root == "" {
		return "", ""
	}
	if i := strings.IndexByte(root, '/'); i >= 0 {
		return root[:i], strings.Trim(root[i+1:], "/")
	}
	return root, ""
}

func (f *Fs) joinPath(remote string) string {
	if f.root == "" {
		return remote
	}
	if remote == "" {
		return f.root
	}
	return f.root + "/" + remote
}

func (f *Fs) relative(key string) string {
	if f.root == "" {
		return key
	}
	return strings.TrimPrefix(key, f.root+"/")
}

func (f *Fs) objectPath(key string) string {
	return "/v1/" + f.bucket + "/" + escapePath(key)
}

// Name of the remote
func (f *Fs) Name() string { return f.name }

// Root of the remote (prefix within the bucket)
func (f *Fs) Root() string { return f.root }

// String description
func (f *Fs) String() string {
	if f.root == "" {
		return fmt.Sprintf("BCOS bucket %s", f.bucket)
	}
	return fmt.Sprintf("BCOS bucket %s path %s", f.bucket, f.root)
}

// Precision of BCOS timestamps (milliseconds)
func (f *Fs) Precision() time.Duration { return time.Millisecond }

// Hashes supported (MD5 via ETag)
func (f *Fs) Hashes() hash.Set { return hash.Set(hash.MD5) }

// Features of this Fs
func (f *Fs) Features() *fs.Features { return f.features }

// errorHandler parses BCOS's XML <Error> body (present on GET/list failures,
// never on HEAD) into a *bcosError; falls back to a plain status-based error
// when the body isn't parseable XML.
func errorHandler(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if err == nil {
		var apiErr bcosError
		if xml.Unmarshal(body, &apiErr) == nil && apiErr.Code != "" {
			return &apiErr
		}
	}
	return fmt.Errorf("bcosv1: HTTP error %d: %s", resp.StatusCode, resp.Status)
}

// shouldRetry classifies 429/5xx as transient. BCOS's own application errors,
// https://bbgithub.dev.bloomberg.com/bcos/bcos-v1-common/blob/0.1.6/src/bloomberg/bcos/v1/common/storage_service/exceptions.py#L114-L161,
// never raise 429, and only InternalError maps to 500 — so these codes are
// realistically produced by whatever sits in front of BCOS (load balancer,
// proxy) rather than BCOS's Flask handlers themselves, but they're the
// standard set any HTTP client should treat as worth a retry regardless of
// origin.
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	if err != nil {
		return fserrors.ShouldRetry(err), err
	}
	if resp != nil {
		switch resp.StatusCode {
		case 429, 500, 502, 503, 504:
			return true, fmt.Errorf("bcosv1: retryable HTTP status %d", resp.StatusCode)
		}
	}
	return false, nil
}

func (f *Fs) callXML(ctx context.Context, path string, params url.Values, out interface{}) error {
	opts := rest.Opts{Method: "GET", Path: path, Parameters: params}
	return f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallXML(ctx, &opts, nil, out)
		return shouldRetry(ctx, resp, err)
	})
}

// listAll paginates a listing under prefix and invokes fn per entry.
// isDir==true means remoteKey is a common-prefix (directory) and item is nil.
func (f *Fs) listAll(ctx context.Context, prefix, delimiter string, fn func(key string, isDir bool, item *listItem) error) error {
	if f.opt.Versions {
		keyMarker, verMarker := "", ""
		for {
			params := url.Values{}
			params.Set("versions", "")
			if prefix != "" {
				params.Set("prefix", prefix)
			}
			if delimiter != "" {
				params.Set("delimiter", delimiter)
			}
			if keyMarker != "" {
				params.Set("key-marker", keyMarker)
			}
			if verMarker != "" {
				params.Set("version-id-marker", verMarker)
			}
			params.Set("max-keys", fmt.Sprint(listChunk))
			var res listVersionsResult
			if err := f.callXML(ctx, "/v1/"+f.bucket, params, &res); err != nil {
				return err
			}
			for _, cp := range res.CommonPrefixes {
				if err := fn(cp.Prefix, true, nil); err != nil {
					return err
				}
			}
			for _, v := range res.Versions {
				it := toItem(v, false)
				if err := fn(v.Key, false, &it); err != nil {
					return err
				}
			}
			for _, d := range res.DeleteMarkers {
				it := toItem(d, true)
				if err := fn(d.Key, false, &it); err != nil {
					return err
				}
			}
			if !res.IsTruncated || res.NextKeyMarker == "" {
				return nil
			}
			keyMarker, verMarker = res.NextKeyMarker, res.NextVersionIdMarker
		}
	}
	marker := ""
	for {
		params := url.Values{}
		if prefix != "" {
			params.Set("prefix", prefix)
		}
		if delimiter != "" {
			params.Set("delimiter", delimiter)
		}
		if marker != "" {
			params.Set("marker", marker)
		}
		params.Set("max-keys", fmt.Sprint(listChunk))
		var res listBucketResult
		if err := f.callXML(ctx, "/v1/"+f.bucket, params, &res); err != nil {
			return err
		}
		for _, cp := range res.CommonPrefixes {
			if err := fn(cp.Prefix, true, nil); err != nil {
				return err
			}
		}
		for _, c := range res.Contents {
			it := toItem(c, false)
			if err := fn(c.Key, false, &it); err != nil {
				return err
			}
		}
		if !res.IsTruncated || res.NextMarker == "" {
			return nil
		}
		marker = res.NextMarker
	}
}

func toItem(x xmlObject, isDelete bool) listItem {
	return listItem{
		key:            x.Key,
		versionID:      x.VersionId,
		isLatest:       x.IsLatest,
		isDeleteMarker: isDelete,
		size:           x.Size,
		md5:            unquoteETag(x.ETag),
		modTime:        parseTime(x.LastModified),
	}
}

// remoteName maps a listItem to its rclone-visible name (relative to f.root),
// suffixing non-latest versions so they don't collide on the destination.
func (f *Fs) remoteName(item *listItem) string {
	name := f.relative(item.key)
	if f.opt.Versions && !item.isLatest && item.versionID != "" {
		name = version.Add(name, item.modTime)
	}
	return name
}

// List the objects and directories in dir into entries
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	dirPath := f.joinPath(dir)
	prefix := ""
	if dirPath != "" {
		prefix = dirPath + "/"
	}
	var entries fs.DirEntries
	err := f.listAll(ctx, prefix, "/", func(key string, isDir bool, item *listItem) error {
		if isDir {
			d := f.relative(strings.TrimSuffix(key, "/"))
			if d == "" {
				return nil
			}
			entries = append(entries, fs.NewDir(d, time.Time{}))
			return nil
		}
		if item.isDeleteMarker || strings.HasSuffix(item.key, "/") {
			return nil
		}
		entries = append(entries, f.newObjectFromItem(f.remoteName(item), item))
		return nil
	})
	if err != nil {
		var apiErr *bcosError
		if errors.As(err, &apiErr) && apiErr.Code == "NoSuchBucket" {
			return nil, fs.ErrorDirNotFound
		}
		return nil, err
	}
	return entries, nil
}

// NewObject finds the Object at remote
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	// Version-suffixed name: resolve the matching version by listing
	if f.opt.Versions && version.Match(remote) {
		return f.newObjectFromVersionName(ctx, remote)
	}
	o := &Object{fs: f, remote: remote, key: f.joinPath(remote)}
	if err := o.readMetaData(ctx); err != nil {
		return nil, err
	}
	return o, nil
}

func (f *Fs) newObjectFromVersionName(ctx context.Context, remote string) (fs.Object, error) {
	_, base := version.Remove(remote)
	key := f.joinPath(base)
	var found *Object
	err := f.listAll(ctx, key, "", func(k string, isDir bool, item *listItem) error {
		if isDir || item == nil || item.isDeleteMarker || item.key != key {
			return nil
		}
		if f.remoteName(item) == remote {
			found = f.newObjectFromItem(remote, item)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, fs.ErrorObjectNotFound
	}
	return found, nil
}

func (f *Fs) newObjectFromItem(remote string, item *listItem) *Object {
	return &Object{
		fs:        f,
		remote:    remote,
		key:       item.key,
		versionID: item.versionID,
		size:      item.size,
		modTime:   item.modTime,
		md5:       item.md5,
	}
}

// Put is not supported — this backend is read-only
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return nil, fs.ErrorPermissionDenied
}

// Mkdir is not supported — this backend is read-only
func (f *Fs) Mkdir(ctx context.Context, dir string) error { return fs.ErrorPermissionDenied }

// Rmdir is not supported — this backend is read-only
func (f *Fs) Rmdir(ctx context.Context, dir string) error { return fs.ErrorPermissionDenied }

// ------------------------------------------------------------------- Object

// Object describes one BCOS object version
type Object struct {
	fs          *Fs
	remote      string
	key         string
	versionID   string // "" == latest
	size        int64
	modTime     time.Time
	md5         string
	contentType string
	customMeta  map[string]string
	headLoaded  bool
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info { return o.fs }

// Remote returns the rclone-visible name
func (o *Object) Remote() string { return o.remote }

// String returns the remote name
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Size of the object
func (o *Object) Size() int64 { return o.size }

// ModTime of the object
func (o *Object) ModTime(ctx context.Context) time.Time { return o.modTime }

// Hash returns the MD5 (from the BCOS ETag)
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	return o.md5, nil
}

// Storable — everything is copyable
func (o *Object) Storable() bool { return true }

// MimeType of the object
func (o *Object) MimeType(ctx context.Context) string {
	_ = o.readMetaData(ctx)
	return o.contentType
}

// SetModTime is not supported — read-only
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return fs.ErrorCantSetModTime
}

// Update is not supported — read-only
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return fs.ErrorPermissionDenied
}

// Remove is not supported — read-only
func (o *Object) Remove(ctx context.Context) error { return fs.ErrorPermissionDenied }

func (o *Object) readMetaData(ctx context.Context) error {
	if o.headLoaded {
		return nil
	}
	opts := rest.Opts{Method: "HEAD", Path: o.fs.objectPath(o.key), NoResponse: true}
	if o.versionID != "" {
		opts.Parameters = url.Values{"versionId": {o.versionID}}
	}
	var resp *http.Response
	err := o.fs.pacer.Call(func() (bool, error) {
		var e error
		resp, e = o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, e)
	})
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return fs.ErrorObjectNotFound
		}
		return err
	}
	o.contentType = resp.Header.Get("Content-Type")
	if et := resp.Header.Get("ETag"); et != "" {
		o.md5 = unquoteETag(et)
	}
	if resp.ContentLength >= 0 {
		o.size = resp.ContentLength
	}
	if o.versionID == "" {
		if v := resp.Header.Get("x-amz-version-id"); v != "" {
			o.versionID = v
		}
	}
	if o.modTime.IsZero() {
		if lm := resp.Header.Get("Last-Modified"); lm != "" {
			o.modTime = parseTime(lm)
		}
	}
	o.customMeta = map[string]string{}
	for k, vs := range resp.Header {
		lk := strings.ToLower(k)
		// BCOS v1 does not support x-amz-meta-*; user metadata round-trips as
		// x-bbg-bcos-meta-* instead (confirmed against BCOS dev: an object PUT
		// with x-amz-meta-foo comes back with neither header at all, while
		// x-bbg-bcos-meta-foo survives GET/HEAD unchanged). put_object,
		// https://bbgithub.dev.bloomberg.com/bcos/bcos-storage-service/blob/1.22.19/src/bloomberg/bcos/v1/storage_service/blueprint/v1/object.py#L152-L158,
		// is the definitive source: it only persists a request header as user
		// metadata when `header_name.startswith("X-Bbg-Bcos-Meta-")` (plus a
		// short allowlist of system headers like Content-Type) — x-amz-meta-*
		// was never in that allowlist, so BCOS silently drops it on the way in.
		if strings.HasPrefix(lk, "x-bbg-bcos-meta-") && len(vs) > 0 {
			o.customMeta[strings.TrimPrefix(lk, "x-bbg-bcos-meta-")] = vs[0]
		}
	}
	o.headLoaded = true
	return nil
}

// Open the object for read. A GET on a bare key (no versionId) whose latest
// version is a delete marker 404s here too — get_object,
// https://bbgithub.dev.bloomberg.com/bcos/bcos-storage-service/blob/1.22.19/src/bloomberg/bcos/v1/storage_service/blueprint/v1/object.py#L492-L497,
// explicitly returns `("", 404, response_headers)` with
// x-amz-delete-marker: true when `row["delete_marker"]` is set, rather than
// serving the marker as if it were content. GET with an explicit versionId
// bypasses "latest" entirely and is unaffected, since that key still exists
// and stays fetchable by version even after a delete marker is written over
// it as the new latest (BCOS's non-destructive DELETE).
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	fs.FixRangeOption(options, o.size)
	opts := rest.Opts{Method: "GET", Path: o.fs.objectPath(o.key), Options: options}
	if o.versionID != "" {
		opts.Parameters = url.Values{"versionId": {o.versionID}}
	}
	var resp *http.Response
	err := o.fs.pacer.Call(func() (bool, error) {
		var e error
		resp, e = o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, e)
	})
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}
	return resp.Body, nil
}

// Metadata returns per-object metadata; the s3 destination maps non-system
// keys (source-version-id, source-timestamp, custom) to x-amz-meta-*.
func (o *Object) Metadata(ctx context.Context) (fs.Metadata, error) {
	if err := o.readMetaData(ctx); err != nil {
		return nil, err
	}
	m := fs.Metadata{}
	if o.contentType != "" {
		m["content-type"] = o.contentType
	}
	if !o.modTime.IsZero() {
		m["mtime"] = o.modTime.Format(time.RFC3339Nano)
	}
	if o.versionID != "" {
		m["source-version-id"] = o.versionID
	}
	if o.fs.opt.MetadataSourceTimestamp && !o.modTime.IsZero() {
		m["source-timestamp"] = o.modTime.UTC().Format("2006-01-02T15:04:05.000Z")
	}
	for k, v := range o.customMeta {
		if _, clash := m[k]; !clash {
			m[k] = v
		}
	}
	return m, nil
}

// Check interfaces
var (
	_ fs.Fs         = &Fs{}
	_ fs.Object     = &Object{}
	_ fs.Metadataer = &Object{}
	_ fs.MimeTyper  = &Object{}
)
