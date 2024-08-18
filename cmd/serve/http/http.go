// Package http provides common functionality for http servers
package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/cmd/serve/proxy/proxyflags"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	libhttp "github.com/rclone/rclone/lib/http"
	"github.com/rclone/rclone/lib/http/serve"
	"github.com/rclone/rclone/lib/systemd"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/rclone/rclone/vfs/vfsflags"
	"github.com/spf13/cobra"
)

// Options required for http server
type Options struct {
	Auth     libhttp.AuthConfig
	HTTP     libhttp.Config
	Template libhttp.TemplateConfig
}

// DefaultOpt is the default values used for Options
var DefaultOpt = Options{
	Auth:     libhttp.DefaultAuthCfg(),
	HTTP:     libhttp.DefaultCfg(),
	Template: libhttp.DefaultTemplateCfg(),
}

// Opt is options set by command line flags
var Opt = DefaultOpt

// flagPrefix is the prefix used to uniquely identify command line flags.
// It is intentionally empty for this package.
const flagPrefix = ""

func init() {
	flagSet := Command.Flags()
	libhttp.AddAuthFlagsPrefix(flagSet, flagPrefix, &Opt.Auth)
	libhttp.AddHTTPFlagsPrefix(flagSet, flagPrefix, &Opt.HTTP)
	libhttp.AddTemplateFlagsPrefix(flagSet, flagPrefix, &Opt.Template)
	vfsflags.AddFlags(flagSet)
	proxyflags.AddFlags(flagSet)
}

// Command definition for cobra
var Command = &cobra.Command{
	Use:   "http remote:path",
	Short: `Serve the remote over HTTP.`,
	Long: `Run a basic web server to serve a remote over HTTP.
This can be viewed in a web browser or you can make a remote of type
http read from it.

You can use the filter flags (e.g. ` + "`--include`, `--exclude`" + `) to control what
is served.

The server will log errors.  Use ` + "`-v`" + ` to see access logs.

` + "`--bwlimit`" + ` will be respected for file transfers.  Use ` + "`--stats`" + ` to
control the stats printing.

` + libhttp.Help(flagPrefix) + libhttp.TemplateHelp(flagPrefix) + libhttp.AuthHelp(flagPrefix) + vfs.Help() + proxy.Help,
	Annotations: map[string]string{
		"versionIntroduced": "v1.39",
		"groups":            "Filter",
	},
	Run: func(command *cobra.Command, args []string) {
		var f fs.Fs
		if proxyflags.Opt.AuthProxy == "" {
			cmd.CheckArgs(1, 1, command, args)
			f = cmd.NewFsSrc(args)
		} else {
			cmd.CheckArgs(0, 0, command, args)
		}

		cmd.Run(false, true, command, func() error {
			s, err := run(context.Background(), f, Opt)
			if err != nil {
				fs.Fatal(nil, fmt.Sprint(err))
			}

			defer systemd.Notify()()
			s.server.Wait()
			return nil
		})
	},
}

// HTTP contains everything to run the server
type HTTP struct {
	f      fs.Fs
	_vfs   *vfs.VFS // don't use directly, use getVFS
	server *libhttp.Server
	opt    Options
	proxy  *proxy.Proxy
	ctx    context.Context // for global config
}

// Gets the VFS in use for this request
func (s *HTTP) getVFS(ctx context.Context) (VFS *vfs.VFS, err error) {
	if s._vfs != nil {
		return s._vfs, nil
	}
	value := libhttp.CtxGetAuth(ctx)
	if value == nil {
		return nil, errors.New("no VFS found in context")
	}
	VFS, ok := value.(*vfs.VFS)
	if !ok {
		return nil, fmt.Errorf("context value is not VFS: %#v", value)
	}
	return VFS, nil
}

// auth does proxy authorization
func (s *HTTP) auth(user, pass string) (value interface{}, err error) {
	VFS, _, err := s.proxy.Call(user, pass, false)
	if err != nil {
		return nil, err
	}
	return VFS, err
}

func run(ctx context.Context, f fs.Fs, opt Options) (s *HTTP, err error) {
	s = &HTTP{
		f:   f,
		ctx: ctx,
		opt: opt,
	}

	if proxyflags.Opt.AuthProxy != "" {
		s.proxy = proxy.New(ctx, &proxyflags.Opt)
		// override auth
		s.opt.Auth.CustomAuthFn = s.auth
	} else {
		s._vfs = vfs.New(f, &vfscommon.Opt)
	}

	s.server, err = libhttp.NewServer(ctx,
		libhttp.WithConfig(s.opt.HTTP),
		libhttp.WithAuth(s.opt.Auth),
		libhttp.WithTemplate(s.opt.Template),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to init server: %w", err)
	}

	router := s.server.Router()
	router.Use(
		middleware.SetHeader("Accept-Ranges", "bytes"),
		middleware.SetHeader("Server", "rclone/"+fs.Version),
	)
	router.Get("/*", s.handler)
	router.Head("/*", s.handler)

	s.server.Serve()

	return s, nil
}

// handler reads incoming requests and dispatches them
func (s *HTTP) handler(w http.ResponseWriter, r *http.Request) {
	isDir := strings.HasSuffix(r.URL.Path, "/")
	remote := strings.Trim(r.URL.Path, "/")
	if isDir {
		s.serveDir(w, r, remote)
	} else {
		s.serveFile(w, r, remote)
	}
}

// serveDir serves a directory index at dirRemote
func (s *HTTP) serveDir(w http.ResponseWriter, r *http.Request, dirRemote string) {
	VFS, err := s.getVFS(r.Context())
	if err != nil {
		http.Error(w, "Root directory not found", http.StatusNotFound)
		fs.Errorf(nil, "Failed to serve directory: %v", err)
		return
	}
	// List the directory
	node, err := VFS.Stat(dirRemote)
	if err == vfs.ENOENT {
		http.Error(w, "Directory not found", http.StatusNotFound)
		return
	} else if err != nil {
		serve.Error(dirRemote, w, "Failed to list directory", err)
		return
	}
	if !node.IsDir() {
		http.Error(w, "Not a directory", http.StatusNotFound)
		return
	}
	dir := node.(*vfs.Dir)
	dirEntries, err := dir.ReadDirAll()
	if err != nil {
		serve.Error(dirRemote, w, "Failed to list directory", err)
		return
	}

	// Make the entries for display
	directory := serve.NewDirectory(dirRemote, s.server.HTMLTemplate())
	for _, node := range dirEntries {
		if vfscommon.Opt.NoModTime {
			directory.AddHTMLEntry(node.Path(), node.IsDir(), node.Size(), time.Time{})
		} else {
			directory.AddHTMLEntry(node.Path(), node.IsDir(), node.Size(), node.ModTime().UTC())
		}
	}

	sortParm := r.URL.Query().Get("sort")
	orderParm := r.URL.Query().Get("order")
	directory.ProcessQueryParams(sortParm, orderParm)

	// Set the Last-Modified header to the timestamp
	w.Header().Set("Last-Modified", dir.ModTime().UTC().Format(http.TimeFormat))

	directory.Serve(w, r)
}

// serveFile serves a file object at remote
func (s *HTTP) serveFile(w http.ResponseWriter, r *http.Request, remote string) {
	VFS, err := s.getVFS(r.Context())
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		fs.Errorf(nil, "Failed to serve file: %v", err)
		return
	}

	node, err := VFS.Stat(remote)
	if err == vfs.ENOENT {
		fs.Infof(remote, "%s: File not found", r.RemoteAddr)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	} else if err != nil {
		serve.Error(remote, w, "Failed to find file", err)
		return
	}
	if !node.IsFile() {
		http.Error(w, "Not a file", http.StatusNotFound)
		return
	}
	entry := node.DirEntry()
	if entry == nil {
		http.Error(w, "Can't open file being written", http.StatusNotFound)
		return
	}
	obj := entry.(fs.Object)
	file := node.(*vfs.File)

	// Set content length if we know how long the object is
	knownSize := obj.Size() >= 0
	if knownSize {
		w.Header().Set("Content-Length", strconv.FormatInt(node.Size(), 10))
	}

	// Set content type
	mimeType := fs.MimeType(r.Context(), obj)
	if mimeType == "application/octet-stream" && path.Ext(remote) == "" {
		// Leave header blank so http server guesses
	} else {
		w.Header().Set("Content-Type", mimeType)
	}

	// Set the Last-Modified header to the timestamp
	w.Header().Set("Last-Modified", file.ModTime().UTC().Format(http.TimeFormat))

	// If HEAD no need to read the object since we have set the headers
	if r.Method == "HEAD" {
		return
	}

	// open the object
	in, err := file.Open(os.O_RDONLY)
	if err != nil {
		serve.Error(remote, w, "Failed to open file", err)
		return
	}
	defer func() {
		err := in.Close()
		if err != nil {
			fs.Errorf(remote, "Failed to close file: %v", err)
		}
	}()

	// Account the transfer
	tr := accounting.Stats(r.Context()).NewTransfer(obj, nil)
	defer tr.Done(r.Context(), nil)
	// FIXME in = fs.NewAccount(in, obj).WithBuffer() // account the transfer

	// Serve the file
	if knownSize {
		http.ServeContent(w, r, remote, node.ModTime(), in)
	} else {
		// http.ServeContent can't serve unknown length files
		if rangeRequest := r.Header.Get("Range"); rangeRequest != "" {
			http.Error(w, "Can't use Range: on files of unknown length", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		n, err := io.Copy(w, in)
		if err != nil {
			fs.Errorf(obj, "Didn't finish writing GET request (wrote %d/unknown bytes): %v", n, err)
			return
		}
	}

}
