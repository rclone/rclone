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
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/rest"
)

// FilesystemPath is the object which is returned from the pixeldrain API when
// running the stat command on a path. It includes the node information for all
// the members of the path and for all the children of the requested directory.
type FilesystemPath struct {
	Path      []FilesystemNode `json:"path"`
	BaseIndex int              `json:"base_index"`
	Children  []FilesystemNode `json:"children"`
}

// Base returns the base node of the path, this is the node that the path points
// to
func (fsp *FilesystemPath) Base() FilesystemNode {
	return fsp.Path[fsp.BaseIndex]
}

// FilesystemNode is a single node in the pixeldrain filesystem. Usually part of
// a Path or Children slice. The Node is also returned as response from update
// commands, if requested
type FilesystemNode struct {
	Type      string    `json:"type"`
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	Created   time.Time `json:"created"`
	Modified  time.Time `json:"modified"`
	ModeOctal string    `json:"mode_octal"`

	// File params
	FileSize  int64  `json:"file_size"`
	FileType  string `json:"file_type"`
	SHA256Sum string `json:"sha256_sum"`

	// ID is only filled in when the file/directory is publicly shared
	ID string `json:"id,omitempty"`
}

// ChangeLog is a log of changes that happened in a filesystem. Changes returned
// from the API are on chronological order from old to new. A change log can be
// requested for any directory or file, but change logging needs to be enabled
// with the update API before any log entries will be made. Changes are logged
// for 24 hours after logging was enabled. Each time a change log is requested
// the timer is reset to 24 hours.
type ChangeLog []ChangeLogEntry

// ChangeLogEntry is a single entry in a directory's change log. It contains the
// time at which the change occurred. The path relative to the requested
// directory and the action that was performend (update, move or delete). In
// case of a move operation the new path of the file is stored in the path_new
// field
type ChangeLogEntry struct {
	Time    time.Time `json:"time"`
	Path    string    `json:"path"`
	PathNew string    `json:"path_new"`
	Action  string    `json:"action"`
	Type    string    `json:"type"`
}

// UserInfo contains information about the logged in user
type UserInfo struct {
	Username         string           `json:"username"`
	Subscription     SubscriptionType `json:"subscription"`
	StorageSpaceUsed int64            `json:"storage_space_used"`
}

// SubscriptionType contains information about a subscription type. It's not the
// active subscription itself, only the properties of the subscription. Like the
// perks and cost
type SubscriptionType struct {
	Name         string `json:"name"`
	StorageSpace int64  `json:"storage_space"`
}

// APIError is the error type returned by the pixeldrain API
type APIError struct {
	StatusCode string `json:"value"`
	Message    string `json:"message"`
}

func (e APIError) Error() string { return e.StatusCode }

// Generalized errors which are caught in our own handlers and translated to
// more specific errors from the fs package.
var (
	errNotFound             = errors.New("pd api: path not found")
	errExists               = errors.New("pd api: node already exists")
	errAuthenticationFailed = errors.New("pd api: authentication failed")
)

func apiErrorHandler(resp *http.Response) (err error) {
	var e APIError
	if err = json.NewDecoder(resp.Body).Decode(&e); err != nil {
		return fmt.Errorf("failed to parse error json: %w", err)
	}

	// We close the body here so that the API handlers can be sure that the
	// response body is not still open when an error was returned
	if err = resp.Body.Close(); err != nil {
		return fmt.Errorf("failed to close resp body: %w", err)
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

var retryErrorCodes = []int{
	429, // Too Many Requests.
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
}

// shouldRetry returns a boolean as to whether this resp and err deserve to be
// retried. It returns the err as a convenience so it can be used as the return
// value in the pacer function
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// paramsFromMetadata turns the fs.Metadata into instructions the pixeldrain API
// can understand.
func paramsFromMetadata(meta fs.Metadata) (params url.Values) {
	params = make(url.Values)

	if modified, ok := meta["mtime"]; ok {
		params.Set("modified", modified)
	}
	if created, ok := meta["btime"]; ok {
		params.Set("created", created)
	}
	if mode, ok := meta["mode"]; ok {
		params.Set("mode", mode)
	}
	if shared, ok := meta["shared"]; ok {
		params.Set("shared", shared)
	}
	if loggingEnabled, ok := meta["logging_enabled"]; ok {
		params.Set("logging_enabled", loggingEnabled)
	}

	return params
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
	return fs.NewDir(strings.TrimPrefix(node.Path, f.pathPrefix), node.Modified).SetID(node.ID)
}

func (f *Fs) escapePath(p string) (out string) {
	// Add the path prefix, encode all the parts and combine them together
	var parts = strings.Split(f.pathPrefix+p, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func (f *Fs) put(
	ctx context.Context,
	path string,
	body io.Reader,
	meta fs.Metadata,
	options []fs.OpenOption,
) (node FilesystemNode, err error) {
	var params = paramsFromMetadata(meta)

	// Tell the server to automatically create parent directories if they don't
	// exist yet
	params.Set("make_parents", "true")

	return node, f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(
			ctx,
			&rest.Opts{
				Method:     "PUT",
				Path:       f.escapePath(path),
				Body:       body,
				Parameters: params,
				Options:    options,
			},
			nil,
			&node,
		)
		return shouldRetry(ctx, resp, err)
	})
}

func (f *Fs) read(ctx context.Context, path string, options []fs.OpenOption) (in io.ReadCloser, err error) {
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &rest.Opts{
			Method:  "GET",
			Path:    f.escapePath(path),
			Options: options,
		})
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, err
}

func (f *Fs) stat(ctx context.Context, path string) (fsp FilesystemPath, err error) {
	return fsp, f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(
			ctx,
			&rest.Opts{
				Method: "GET",
				Path:   f.escapePath(path),
				// To receive node info from the pixeldrain API you need to add the
				// ?stat query. Without it pixeldrain will return the file contents
				// in the URL points to a file
				Parameters: url.Values{"stat": []string{""}},
			},
			nil,
			&fsp,
		)
		return shouldRetry(ctx, resp, err)
	})
}

func (f *Fs) changeLog(ctx context.Context, start, end time.Time) (changeLog ChangeLog, err error) {
	return changeLog, f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(
			ctx,
			&rest.Opts{
				Method: "GET",
				Path:   f.escapePath(""),
				Parameters: url.Values{
					"change_log": []string{""},
					"start":      []string{start.Format(time.RFC3339Nano)},
					"end":        []string{end.Format(time.RFC3339Nano)},
				},
			},
			nil,
			&changeLog,
		)
		return shouldRetry(ctx, resp, err)
	})
}

func (f *Fs) update(ctx context.Context, path string, fields fs.Metadata) (node FilesystemNode, err error) {
	var params = paramsFromMetadata(fields)
	params.Set("action", "update")

	return node, f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(
			ctx,
			&rest.Opts{
				Method:          "POST",
				Path:            f.escapePath(path),
				MultipartParams: params,
			},
			nil,
			&node,
		)
		return shouldRetry(ctx, resp, err)
	})
}

func (f *Fs) mkdir(ctx context.Context, dir string) (err error) {
	return f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(
			ctx,
			&rest.Opts{
				Method:          "POST",
				Path:            f.escapePath(dir),
				MultipartParams: url.Values{"action": []string{"mkdirall"}},
				NoResponse:      true,
			},
			nil,
			nil,
		)
		return shouldRetry(ctx, resp, err)
	})
}

var errIncompatibleSourceFS = errors.New("source filesystem is not the same as target")

// Renames a file on the server side. Can be used for both directories and files
func (f *Fs) rename(ctx context.Context, src fs.Fs, from, to string, meta fs.Metadata) (node FilesystemNode, err error) {
	srcFs, ok := src.(*Fs)
	if !ok {
		// This is not a pixeldrain FS, can't move
		return node, errIncompatibleSourceFS
	} else if srcFs.opt.RootFolderID != f.opt.RootFolderID {
		// Path is not in the same root dir, can't move
		return node, errIncompatibleSourceFS
	}

	var params = paramsFromMetadata(meta)
	params.Set("action", "rename")

	// The target is always in our own filesystem so here we use our
	// own pathPrefix
	params.Set("target", f.pathPrefix+to)

	// Create parent directories if the parent directory of the file
	// does not exist yet
	params.Set("make_parents", "true")

	return node, f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(
			ctx,
			&rest.Opts{
				Method: "POST",
				// Important: We use the source FS path prefix here
				Path:            srcFs.escapePath(from),
				MultipartParams: params,
			},
			nil,
			&node,
		)
		return shouldRetry(ctx, resp, err)
	})
}

func (f *Fs) delete(ctx context.Context, path string, recursive bool) (err error) {
	var params url.Values
	if recursive {
		// Tell the server to recursively delete all child files
		params = url.Values{"recursive": []string{"true"}}
	}

	return f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(
			ctx,
			&rest.Opts{
				Method:     "DELETE",
				Path:       f.escapePath(path),
				Parameters: params,
				NoResponse: true,
			},
			nil, nil,
		)
		return shouldRetry(ctx, resp, err)
	})
}

func (f *Fs) userInfo(ctx context.Context) (user UserInfo, err error) {
	return user, f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(
			ctx,
			&rest.Opts{
				Method: "GET",
				// The default RootURL points at the filesystem endpoint. We can't
				// use that to request user information. So here we override it to
				// the user endpoint
				RootURL: f.opt.APIURL + "/user",
			},
			nil,
			&user,
		)
		return shouldRetry(ctx, resp, err)
	})
}
