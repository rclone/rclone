package pixeldrain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/rest"
)

type FilesystemPath struct {
	Path      []FilesystemNode `json:"path"`
	BaseIndex int              `json:"base_index"`
	Children  []FilesystemNode `json:"children"`
}

func (fsp *FilesystemPath) Base() FilesystemNode {
	return fsp.Path[fsp.BaseIndex]
}

// FilesystemNode is the return value of the GET /filesystem/ API
type FilesystemNode struct {
	Type      string    `json:"type"`
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	Created   time.Time `json:"created"`
	Modified  time.Time `json:"modified"`
	ModeStr   string    `json:"mode_string"`
	ModeOctal string    `json:"mode_octal"`

	// File params
	FileSize  int64  `json:"file_size"`
	FileType  string `json:"file_type"`
	SHA256Sum string `json:"sha256_sum"`

	// Meta params
	ID            string            `json:"id,omitempty"`
	ReadPassword  string            `json:"read_password,omitempty"`
	WritePassword string            `json:"write_password,omitempty"`
	Properties    map[string]string `json:"properties,omitempty"`
}

// UserInfo contains information about the logged in user
type UserInfo struct {
	Username            string            `json:"username"`
	Email               string            `json:"email"`
	Subscription        SubscriptionType  `json:"subscription"`
	StorageSpaceUsed    int64             `json:"storage_space_used"`
	IsAdmin             bool              `json:"is_admin"`
	BalanceMicroEUR     int64             `json:"balance_micro_eur"`
	Hotlinking          bool              `json:"hotlinking_enabled"`
	MonthlyTransferCap  int64             `json:"monthly_transfer_cap"`
	MonthlyTransferUsed int64             `json:"monthly_transfer_used"`
	FileViewerBranding  map[string]string `json:"file_viewer_branding"`
	FileEmbedDomains    string            `json:"file_embed_domains"`
	SkipFileViewer      bool              `json:"skip_file_viewer"`
}

// SubscriptionType contains information about a subscription type. It's not the
// active subscription itself, only the properties of the subscription. Like the
// perks and cost
type SubscriptionType struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Type                string `json:"type"`
	FileSizeLimit       int64  `json:"file_size_limit"`
	FileExpiryDays      int64  `json:"file_expiry_days"`
	StorageSpace        int64  `json:"storage_space"`
	PricePerTBStorage   int64  `json:"price_per_tb_storage"`
	PricePerTBBandwidth int64  `json:"price_per_tb_bandwidth"`
	MonthlyTransferCap  int64  `json:"monthly_transfer_cap"`
	FileViewerBranding  bool   `json:"file_viewer_branding"`
}

type ApiError struct {
	StatusCode string `json:"value"`
	Message    string `json:"message"`
}

func (e ApiError) Error() string { return e.StatusCode }

// Generalized errors which are caught in our own handlers and translated to
// more specific errors from the fs package.
var (
	errNotFound             = errors.New("pd api: path not found")
	errExists               = errors.New("pd api: node already exists")
	errAuthenticationFailed = errors.New("pd api: authentication failed")
)

func apiErrorHandler(resp *http.Response) (err error) {
	var e ApiError
	if err = json.NewDecoder(resp.Body).Decode(&e); err != nil {
		return fmt.Errorf("failed to parse error json: %w", err)
	}

	if e.StatusCode == "path_not_found" {
		return errNotFound
	} else if e.StatusCode == "directory_not_empty" {
		return fs.ErrorDirectoryNotEmpty
	} else if e.StatusCode == "node_already_exists" {
		return errExists
	} else if e.StatusCode == "authentication_failed" {
		return errAuthenticationFailed
	} else if e.StatusCode == "permission_denied" {
		return fs.ErrorPermissionDenied
	}

	return e
}

// nodeToObject converts a single FilesystemNode API response to an object. The
// node is usually a single element from a directory listing
func (f *Fs) nodeToObject(node FilesystemNode) (o *Object) {
	// Trim the path prefix. The path prefix is hidden from rclone during all
	// operations. Saving it here would confuse rclone a lot. So instead we
	// strip it here and add it back for every API request we need to perform
	node.Path = strings.TrimPrefix(node.Path, f.pathPrefix)
	return &Object{fs: f, base: node}
}

func (f *Fs) nodeToDirectory(node FilesystemNode) fs.DirEntry {
	return fs.NewDir(strings.TrimPrefix(node.Path, f.pathPrefix), node.Modified)
}

func (f *Fs) put(ctx context.Context, path string, body io.Reader, options []fs.OpenOption) (node FilesystemNode, err error) {
	resp, err := f.srv.CallJSON(
		ctx,
		&rest.Opts{
			Method: "PUT",
			Path:   f.pathPrefix + url.PathEscape(path),
			Body:   body,
			Parameters: url.Values{
				// Tell the server to automatically create parent directories if
				// they don't exist yet
				"make_parents": []string{"true"},
			},
			Options: options,
		},
		nil,
		&node,
	)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	return node, err
}

func (f *Fs) read(ctx context.Context, path string, options []fs.OpenOption) (in io.ReadCloser, err error) {
	resp, err := f.srv.Call(ctx, &rest.Opts{
		Method:  "GET",
		Path:    f.pathPrefix + url.PathEscape(path),
		Options: options,
	})
	if err != nil {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		return nil, err
	}
	return resp.Body, err
}

func (f *Fs) stat(ctx context.Context, path string) (fsp FilesystemPath, err error) {
	resp, err := f.srv.CallJSON(
		ctx,
		&rest.Opts{
			Method: "GET",
			Path:   f.pathPrefix + url.PathEscape(path),
			// To receive node info from the pixeldrain API you need to add the
			// ?stat query. Without it pixeldrain will return the file contents
			// in the URL points to a file
			Parameters: url.Values{"stat": []string{""}},
		},
		nil,
		&fsp,
	)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	return fsp, err
}

func (f *Fs) update(ctx context.Context, path string, fields map[string]any) (node FilesystemNode, err error) {
	var params = make(url.Values)
	params.Set("action", "update")

	if created, ok := fields["created"]; ok {
		params.Set("created", created.(time.Time).Format(time.RFC3339Nano))
	}
	if modified, ok := fields["modified"]; ok {
		params.Set("modified", modified.(time.Time).Format(time.RFC3339Nano))
	}

	resp, err := f.srv.CallJSON(
		ctx,
		&rest.Opts{
			Method:          "POST",
			Path:            f.pathPrefix + url.PathEscape(path),
			MultipartParams: params,
		},
		nil,
		&node,
	)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	return node, err
}

func (f *Fs) mkdir(ctx context.Context, dir string) (err error) {
	resp, err := f.srv.CallJSON(
		ctx,
		&rest.Opts{
			Method: "POST",
			Path:   f.pathPrefix + url.PathEscape(dir),
			MultipartParams: url.Values{
				"action": []string{"mkdirall"},
			},
			NoResponse: true,
		},
		nil, nil,
	)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	return err
}

var errIncompatibleSourceFS = errors.New("source filesystem is not the same as target")

// Renames a file on the server side. Can be used for both directories and files
func (f *Fs) rename(ctx context.Context, src fs.Fs, from, to string) (err error) {
	srcFs, ok := src.(*Fs)
	if !ok {
		// This is not a pixeldrain FS, can't move
		return errIncompatibleSourceFS
	} else if srcFs.opt.BucketID != f.opt.BucketID {
		// Path is not in the same bucket, can't move
		return errIncompatibleSourceFS
	}

	resp, err := f.srv.CallJSON(
		ctx,
		&rest.Opts{
			Method: "POST",
			// Important: We use the source FS path prefix here
			Path: srcFs.pathPrefix + url.PathEscape(from),
			MultipartParams: url.Values{
				"action": []string{"rename"},
				// The target is always in our own filesystem so here we use our
				// own pathPrefix
				"target": []string{f.pathPrefix + to},
			},
			NoResponse: true,
		},
		nil, nil,
	)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	return err
}

func (f *Fs) delete(ctx context.Context, path string, recursive bool) (err error) {
	var params url.Values = nil
	if recursive {
		// Tell the server to recursively delete all child files
		params = url.Values{"recursive": []string{"true"}}
	}

	resp, err := f.srv.CallJSON(
		ctx,
		&rest.Opts{
			Method:     "DELETE",
			Path:       f.pathPrefix + url.PathEscape(path),
			Parameters: params,
			NoResponse: true,
		},
		nil, nil,
	)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	return err
}

func (f *Fs) userInfo(ctx context.Context) (user UserInfo, err error) {
	resp, err := f.srv.CallJSON(
		ctx,
		&rest.Opts{
			Method: "GET",
			// The default RootURL points at the filesystem endpoint. We can't
			// use that to request user information. So here we override it to
			// the user endpoint
			RootURL: f.opt.APIURL + userEndpoint,
		},
		nil,
		&user,
	)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	return user, err
}
