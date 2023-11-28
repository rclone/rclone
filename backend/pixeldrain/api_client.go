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

// nodeToObject converts a single FilesystemNode API response to an object. The
// node is usually a single element from a directory listing
func (f *Fs) nodeToObject(node FilesystemNode) (o *Object) {
	// Trim the path prefix. The path prefix is hidden from rclone during all
	// operations. Saving it here would confuse rclone a lot. So instead we
	// strip it here and add it back for every API request we need to perform
	node.Path = trimPrefix(node.Path, f.pathPrefix)
	return &Object{fs: f, base: node}
}

func (f *Fs) nodeToDirectory(node FilesystemNode) fs.DirEntry {
	return fs.NewDir(trimPrefix(node.Path, f.pathPrefix), node.Modified)
}

func trimPath(s string) string {
	return strings.Trim(s, "/")
}
func trimPrefix(s, prefix string) string {
	return strings.TrimPrefix(trimPath(s), prefix+"/")
}
func mergePath(inputs ...string) (out string) {
	var allParts []string
	for i := range inputs {
		// Strip the input of any preceding and trailing slashes and split all
		// the remaining parts and add them to the collection
		allParts = append(allParts, strings.Split(trimPath(inputs[i]), "/")...)
	}

	// Now encode all the parts and combine them together
	for i := range allParts {
		allParts[i] = url.PathEscape(allParts[i])
	}
	return strings.Join(allParts, "/")
}

func (f *Fs) put(ctx context.Context, path string, body io.Reader, options []fs.OpenOption) (node FilesystemNode, err error) {
	resp, err := f.srv.CallJSON(
		ctx,
		&rest.Opts{
			Method: "PUT",
			Path:   mergePath(f.pathPrefix, path),
			Body:   body,
			// Tell the server to automatically create parent directories if
			// they don't exist yet
			Parameters: url.Values{"make_parents": []string{"true"}},
			Options:    options,
		},
		nil,
		&node,
	)
	if err != nil {
		return node, err
	}
	return node, resp.Body.Close()
}

func (f *Fs) read(ctx context.Context, path string, options []fs.OpenOption) (in io.ReadCloser, err error) {
	resp, err := f.srv.Call(ctx, &rest.Opts{
		Method:  "GET",
		Path:    mergePath(f.pathPrefix, path),
		Options: options,
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, err
}

func (f *Fs) stat(ctx context.Context, path string) (fsp FilesystemPath, err error) {
	resp, err := f.srv.CallJSON(
		ctx,
		&rest.Opts{
			Method: "GET",
			Path:   mergePath(f.pathPrefix, path),
			// To receive node info from the pixeldrain API you need to add the
			// ?stat query. Without it pixeldrain will return the file contents
			// in the URL points to a file
			Parameters: url.Values{"stat": []string{""}},
		},
		nil,
		&fsp,
	)
	if err != nil {
		return fsp, err
	}
	return fsp, resp.Body.Close()
}

func (f *Fs) update(ctx context.Context, path string, fields fs.Metadata) (node FilesystemNode, err error) {
	var params = make(url.Values)
	params.Set("action", "update")

	if modified, ok := fields["mtime"]; ok {
		params.Set("modified", modified)
	}
	if created, ok := fields["btime"]; ok {
		params.Set("created", created)
	}
	if mode, ok := fields["mode"]; ok {
		params.Set("mode", mode)
	}
	if shared, ok := fields["shared"]; ok {
		params.Set("shared", shared)
	}

	resp, err := f.srv.CallJSON(
		ctx,
		&rest.Opts{
			Method:          "POST",
			Path:            mergePath(f.pathPrefix, path),
			MultipartParams: params,
		},
		nil,
		&node,
	)
	if err != nil {
		return node, err
	}
	return node, resp.Body.Close()
}

func (f *Fs) mkdir(ctx context.Context, dir string) (err error) {
	_, err = f.srv.CallJSON(
		ctx,
		&rest.Opts{
			Method:          "POST",
			Path:            mergePath(f.pathPrefix, dir),
			MultipartParams: url.Values{"action": []string{"mkdirall"}},
			NoResponse:      true,
		},
		nil, nil,
	)
	return err
}

var errIncompatibleSourceFS = errors.New("source filesystem is not the same as target")

// Renames a file on the server side. Can be used for both directories and files
func (f *Fs) rename(ctx context.Context, src fs.Fs, from, to string) (err error) {
	srcFs, ok := src.(*Fs)
	if !ok {
		// This is not a pixeldrain FS, can't move
		return errIncompatibleSourceFS
	} else if srcFs.opt.DirectoryID != f.opt.DirectoryID {
		// Path is not in the same root dir, can't move
		return errIncompatibleSourceFS
	}

	_, err = f.srv.CallJSON(
		ctx,
		&rest.Opts{
			Method: "POST",
			// Important: We use the source FS path prefix here
			Path: mergePath(srcFs.pathPrefix, from),
			MultipartParams: url.Values{
				"action": []string{"rename"},
				// The target is always in our own filesystem so here we use our
				// own pathPrefix
				"target": []string{f.pathPrefix + "/" + to},
				// Create parent directories if the parent directory of the file
				// does not exist yet
				"make_parents": []string{"true"},
			},
			NoResponse: true,
		},
		nil, nil,
	)
	return err
}

func (f *Fs) delete(ctx context.Context, path string, recursive bool) (err error) {
	var params url.Values
	if recursive {
		// Tell the server to recursively delete all child files
		params = url.Values{"recursive": []string{"true"}}
	}

	_, err = f.srv.CallJSON(
		ctx,
		&rest.Opts{
			Method:     "DELETE",
			Path:       mergePath(f.pathPrefix, path),
			Parameters: params,
			NoResponse: true,
		},
		nil, nil,
	)
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
			RootURL: f.opt.APIURL + "/user",
		},
		nil,
		&user,
	)
	if err != nil {
		return user, err
	}
	return user, resp.Body.Close()
}
