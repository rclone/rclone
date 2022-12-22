// Package http provides common functionality for http servers
package http

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	libhttp "github.com/rclone/rclone/lib/http"
	"github.com/rclone/rclone/lib/http/serve"
	"github.com/rclone/rclone/vfs"
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

func init() {
	flagSet := Command.Flags()
	libhttp.AddAuthFlagsPrefix(flagSet, "", &Opt.Auth)
	libhttp.AddHTTPFlagsPrefix(flagSet, "", &Opt.HTTP)
	libhttp.AddTemplateFlagsPrefix(flagSet, "", &Opt.Template)
	vfsflags.AddFlags(flagSet)
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
` + libhttp.Help + libhttp.TemplateHelp + libhttp.AuthHelp + vfs.Help,
	Annotations: map[string]string{
		"versionIntroduced": "v1.39",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		f := cmd.NewFsSrc(args)

		cmd.Run(false, true, command, func() error {
			ctx := context.Background()

			s, err := run(ctx, f, Opt)
			if err != nil {
				log.Fatal(err)
			}

			s.server.Wait()
			return nil
		})
	},
}

// server contains everything to run the server
type serveCmd struct {
	f      fs.Fs
	vfs    *vfs.VFS
	server *libhttp.Server
}

func run(ctx context.Context, f fs.Fs, opt Options) (*serveCmd, error) {
	var err error

	s := &serveCmd{
		f:   f,
		vfs: vfs.New(f, &vfsflags.Opt),
	}

	s.server, err = libhttp.NewServer(ctx,
		libhttp.WithConfig(opt.HTTP),
		libhttp.WithAuth(opt.Auth),
		libhttp.WithTemplate(opt.Template),
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
func (s *serveCmd) handler(w http.ResponseWriter, r *http.Request) {
	isDir := strings.HasSuffix(r.URL.Path, "/")
	remote := strings.Trim(r.URL.Path, "/")
	if isDir {
		s.serveDir(w, r, remote)
	} else {
		s.serveFile(w, r, remote)
	}
}

// serveDir serves a directory index at dirRemote
func (s *serveCmd) serveDir(w http.ResponseWriter, r *http.Request, dirRemote string) {
	// List the directory
	node, err := s.vfs.Stat(dirRemote)
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
		if vfsflags.Opt.NoModTime {
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
func (s *serveCmd) serveFile(w http.ResponseWriter, r *http.Request, remote string) {
	node, err := s.vfs.Stat(remote)
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
	tr := accounting.Stats(r.Context()).NewTransfer(obj)
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
