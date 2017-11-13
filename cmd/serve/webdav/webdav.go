package webdav

// FIXME need to fix directory listings reading each file - make an
// override for getcontenttype property?

import (
	"net/http"
	"os"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/vfs"
	"github.com/ncw/rclone/vfs/vfsflags"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"golang.org/x/net/webdav"
)

// Globals
var (
	bindAddress = "localhost:8081"
)

func init() {
	Command.Flags().StringVarP(&bindAddress, "addr", "", bindAddress, "IPaddress:Port to bind server to.")
	vfsflags.AddFlags(Command.Flags())
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

NB at the moment each directory listing reads the start of each file
which is undesirable: see https://github.com/golang/go/issues/22577

` + vfs.Help,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			return serveWebDav(fsrc)
		})
	},
}

// serve the remote
func serveWebDav(f fs.Fs) error {
	fs.Logf(f, "WebDav Server started on %v", bindAddress)

	webdavFS := &WebDAV{
		f:   f,
		vfs: vfs.New(f, &vfsflags.Opt),
	}

	handler := &webdav.Handler{
		FileSystem: webdavFS,
		LockSystem: webdav.NewMemLS(),
		Logger:     webdavFS.logRequest, // FIXME
	}

	// FIXME use our HTTP transport
	http.Handle("/", handler)
	return http.ListenAndServe(bindAddress, nil)
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
	f   fs.Fs
	vfs *vfs.VFS
}

// check interface
var _ webdav.FileSystem = (*WebDAV)(nil)

// logRequest is called by the webdav module on every request
func (w *WebDAV) logRequest(r *http.Request, err error) {
	fs.Infof(r.URL.Path, "%s from %s", r.Method, r.RemoteAddr)
}

// Mkdir creates a directory
func (w *WebDAV) Mkdir(ctx context.Context, name string, perm os.FileMode) (err error) {
	defer fs.Trace(name, "perm=%v", perm)("err = %v", &err)
	dir, leaf, err := w.vfs.StatParent(name)
	if err != nil {
		return err
	}
	_, err = dir.Mkdir(leaf)
	return err
}

// OpenFile opens a file or a directory
func (w *WebDAV) OpenFile(ctx context.Context, name string, flags int, perm os.FileMode) (file webdav.File, err error) {
	defer fs.Trace(name, "flags=%v, perm=%v", flags, perm)("err = %v", &err)
	return w.vfs.OpenFile(name, flags, perm)
}

// RemoveAll removes a file or a directory and its contents
func (w *WebDAV) RemoveAll(ctx context.Context, name string) (err error) {
	defer fs.Trace(name, "")("err = %v", &err)
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
	defer fs.Trace(oldName, "newName=%q", newName)("err = %v", &err)
	return w.vfs.Rename(oldName, newName)
}

// Stat returns info about the file or directory
func (w *WebDAV) Stat(ctx context.Context, name string) (fi os.FileInfo, err error) {
	defer fs.Trace(name, "")("fi=%+v, err = %v", &fi, &err)
	return w.vfs.Stat(name)
}

// check interface
var _ os.FileInfo = vfs.Node(nil)
