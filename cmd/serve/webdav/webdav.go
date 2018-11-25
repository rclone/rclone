//+build go1.9

package webdav

import (
	"net/http"
	"os"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/cmd/serve/httplib"
	"github.com/ncw/rclone/cmd/serve/httplib/httpflags"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/hash"
	"github.com/ncw/rclone/fs/log"
	"github.com/ncw/rclone/vfs"
	"github.com/ncw/rclone/vfs/vfsflags"
	"github.com/spf13/cobra"
	"golang.org/x/net/context" // switch to "context" when we stop supporting go1.8
	"golang.org/x/net/webdav"
)

var (
	hashName string
	hashType = hash.None
)

func init() {
	httpflags.AddFlags(Command.Flags())
	vfsflags.AddFlags(Command.Flags())
	Command.Flags().StringVar(&hashName, "etag-hash", "", "Which hash to use for the ETag, or auto or blank for off")
}

// Command definition for cobra
var Command = &cobra.Command{
	Use:   "webdav remote:path",
	Short: `Serve remote:path over webdav.`,
	Long: `
rclone serve webdav implements a basic webdav server to serve the
remote over HTTP via the webdav protocol. This can be viewed with a
webdav client or you can make a remote of type webdav to read and
write it.

### Webdav options

#### --etag-hash 

This controls the ETag header.  Without this flag the ETag will be
based on the ModTime and Size of the object.

If this flag is set to "auto" then rclone will choose the first
supported hash on the backend or you can use a named hash such as
"MD5" or "SHA-1".

Use "rclone hashsum" to see the full list.

` + httplib.Help + vfs.Help,
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(1, 1, command, args)
		f := cmd.NewFsSrc(args)
		hashType = hash.None
		if hashName == "auto" {
			hashType = f.Hashes().GetOne()
		} else if hashName != "" {
			err := hashType.Set(hashName)
			if err != nil {
				return err
			}
		}
		if hashType != hash.None {
			fs.Debugf(f, "Using hash %v for ETag", hashType)
		}
		cmd.Run(false, false, command, func() error {
			s := newWebDAV(f, &httpflags.Opt)
			err := s.serve()
			if err != nil {
				return err
			}
			s.Wait()
			return nil
		})
		return nil
	},
}

// WebDAV is a webdav.FileSystem interface
//
// A FileSystem implements access to a collection of named files. The elements
// in a file path are separated by slash ('/', U+002F) characters, regardless
// of host operating system convention.
//
// Each method has the same semantics as the os package's function of the same
// name.
//
// Note that the os.Rename documentation says that "OS-specific restrictions
// might apply". In particular, whether or not renaming a file or directory
// overwriting another existing file or directory is an error is OS-dependent.
type WebDAV struct {
	*httplib.Server
	f   fs.Fs
	vfs *vfs.VFS
}

// check interface
var _ webdav.FileSystem = (*WebDAV)(nil)

// Make a new WebDAV to serve the remote
func newWebDAV(f fs.Fs, opt *httplib.Options) *WebDAV {
	w := &WebDAV{
		f:   f,
		vfs: vfs.New(f, &vfsflags.Opt),
	}

	handler := &webdav.Handler{
		FileSystem: w,
		LockSystem: webdav.NewMemLS(),
		Logger:     w.logRequest, // FIXME
	}

	w.Server = httplib.NewServer(handler, opt)
	return w
}

// serve runs the http server in the background.
//
// Use s.Close() and s.Wait() to shutdown server
func (w *WebDAV) serve() error {
	err := w.Serve()
	if err != nil {
		return err
	}
	fs.Logf(w.f, "WebDav Server started on %s", w.URL())
	return nil
}

// logRequest is called by the webdav module on every request
func (w *WebDAV) logRequest(r *http.Request, err error) {
	fs.Infof(r.URL.Path, "%s from %s", r.Method, r.RemoteAddr)
}

// Mkdir creates a directory
func (w *WebDAV) Mkdir(ctx context.Context, name string, perm os.FileMode) (err error) {
	defer log.Trace(name, "perm=%v", perm)("err = %v", &err)
	dir, leaf, err := w.vfs.StatParent(name)
	if err != nil {
		return err
	}
	_, err = dir.Mkdir(leaf)
	return err
}

// OpenFile opens a file or a directory
func (w *WebDAV) OpenFile(ctx context.Context, name string, flags int, perm os.FileMode) (file webdav.File, err error) {
	defer log.Trace(name, "flags=%v, perm=%v", flags, perm)("err = %v", &err)
	f, err := w.vfs.OpenFile(name, flags, perm)
	if err != nil {
		return nil, err
	}
	return Handle{f}, nil
}

// RemoveAll removes a file or a directory and its contents
func (w *WebDAV) RemoveAll(ctx context.Context, name string) (err error) {
	defer log.Trace(name, "")("err = %v", &err)
	node, err := w.vfs.Stat(name)
	if err != nil {
		return err
	}
	err = node.RemoveAll()
	if err != nil {
		return err
	}
	return nil
}

// Rename a file or a directory
func (w *WebDAV) Rename(ctx context.Context, oldName, newName string) (err error) {
	defer log.Trace(oldName, "newName=%q", newName)("err = %v", &err)
	return w.vfs.Rename(oldName, newName)
}

// Stat returns info about the file or directory
func (w *WebDAV) Stat(ctx context.Context, name string) (fi os.FileInfo, err error) {
	defer log.Trace(name, "")("fi=%+v, err = %v", &fi, &err)
	fi, err = w.vfs.Stat(name)
	if err != nil {
		return nil, err
	}
	return FileInfo{fi}, nil
}

// Handle represents an open file
type Handle struct {
	vfs.Handle
}

// Readdir reads directory entries from the handle
func (h Handle) Readdir(count int) (fis []os.FileInfo, err error) {
	fis, err = h.Handle.Readdir(count)
	if err != nil {
		return nil, err
	}
	// Wrap each FileInfo
	for i := range fis {
		fis[i] = FileInfo{fis[i]}
	}
	return fis, nil
}

// Stat the handle
func (h Handle) Stat() (fi os.FileInfo, err error) {
	fi, err = h.Handle.Stat()
	if err != nil {
		return nil, err
	}
	return FileInfo{fi}, nil
}

// FileInfo represents info about a file satisfying os.FileInfo and
// also some additional interfaces for webdav for ETag and ContentType
type FileInfo struct {
	os.FileInfo
}

// ETag returns an ETag for the FileInfo
func (fi FileInfo) ETag(ctx context.Context) (etag string, err error) {
	defer log.Trace(fi, "")("etag=%q, err=%v", &etag, &err)
	if hashType == hash.None {
		return "", webdav.ErrNotImplemented
	}
	node, ok := (fi.FileInfo).(vfs.Node)
	if !ok {
		fs.Errorf(fi, "Expecting vfs.Node, got %T", fi.FileInfo)
		return "", webdav.ErrNotImplemented
	}
	entry := node.DirEntry()
	o, ok := entry.(fs.Object)
	if !ok {
		return "", webdav.ErrNotImplemented
	}
	hash, err := o.Hash(hashType)
	if err != nil || hash == "" {
		return "", webdav.ErrNotImplemented
	}
	return `"` + hash + `"`, nil
}

// ContentType returns a content type for the FileInfo
func (fi FileInfo) ContentType(ctx context.Context) (contentType string, err error) {
	defer log.Trace(fi, "")("etag=%q, err=%v", &contentType, &err)
	node, ok := (fi.FileInfo).(vfs.Node)
	if !ok {
		fs.Errorf(fi, "Expecting vfs.Node, got %T", fi.FileInfo)
		return "application/octet-stream", nil
	}
	entry := node.DirEntry()
	switch x := entry.(type) {
	case fs.Object:
		return fs.MimeType(x), nil
	case fs.Directory:
		return "inode/directory", nil
	}
	fs.Errorf(fi, "Expecting fs.Object or fs.Directory, got %T", entry)
	return "application/octet-stream", nil
}
