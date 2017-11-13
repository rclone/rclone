package http

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/vfs"
	"github.com/ncw/rclone/vfs/vfsflags"
	"github.com/spf13/cobra"
)

// Globals
var (
	bindAddress = "localhost:8080"
)

func init() {
	Command.Flags().StringVarP(&bindAddress, "addr", "", bindAddress, "IPaddress:Port to bind server to.")
	vfsflags.AddFlags(Command.Flags())
}

// Command definition for cobra
var Command = &cobra.Command{
	Use:   "http remote:path",
	Short: `Serve the remote over HTTP.`,
	Long: `rclone serve http implements a basic web server to serve the remote
over HTTP.  This can be viewed in a web browser or you can make a
remote of type http read from it.

Use --addr to specify which IP address and port the server should
listen on, eg --addr 1.2.3.4:8000 or --addr :8080 to listen to all
IPs.  By default it only listens on localhost.

You can use the filter flags (eg --include, --exclude) to control what
is served.

The server will log errors.  Use -v to see access logs.

--bwlimit will be respected for file transfers.  Use --stats to
control the stats printing.
` + vfs.Help,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		f := cmd.NewFsSrc(args)
		cmd.Run(false, true, command, func() error {
			s := newServer(f, bindAddress)
			s.serve()
			return nil
		})
	},
}

// server contains everything to run the server
type server struct {
	f           fs.Fs
	bindAddress string
	vfs         *vfs.VFS
}

func newServer(f fs.Fs, bindAddress string) *server {
	s := &server{
		f:           f,
		bindAddress: bindAddress,
		vfs:         vfs.New(f, &vfsflags.Opt),
	}
	return s
}

// serve creates the http server
func (s *server) serve() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handler)
	// FIXME make a transport?
	httpServer := &http.Server{
		Addr:           s.bindAddress,
		Handler:        mux,
		MaxHeaderBytes: 1 << 20,
	}
	initServer(httpServer)
	fs.Logf(s.f, "Serving on http://%s/", bindAddress)
	log.Fatal(httpServer.ListenAndServe())
}

// handler reads incoming requests and dispatches them
func (s *server) handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Server", "rclone/"+fs.Version)

	urlPath := r.URL.Path
	isDir := strings.HasSuffix(urlPath, "/")
	remote := strings.Trim(urlPath, "/")
	if isDir {
		s.serveDir(w, r, remote)
	} else {
		s.serveFile(w, r, remote)
	}
}

// entry is a directory entry
type entry struct {
	remote string
	URL    string
	Leaf   string
}

// entries represents a directory
type entries []entry

// indexPage is a directory listing template
var indexPage = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>{{ .Title }}</title>
</head>
<body>
<h1>{{ .Title }}</h1>
{{ range $i := .Entries }}<a href="{{ $i.URL }}">{{ $i.Leaf }}</a><br />
{{ end }}</body>
</html>
`

// indexTemplate is the instantiated indexPage
var indexTemplate = template.Must(template.New("index").Parse(indexPage))

// indexData is used to fill in the indexTemplate
type indexData struct {
	Title   string
	Entries entries
}

// error returns an http.StatusInternalServerError and logs the error
func internalError(what interface{}, w http.ResponseWriter, text string, err error) {
	fs.Stats.Error(err)
	fs.Errorf(what, "%s: %v", text, err)
	http.Error(w, text+".", http.StatusInternalServerError)
}

// serveDir serves a directory index at dirRemote
func (s *server) serveDir(w http.ResponseWriter, r *http.Request, dirRemote string) {
	// List the directory
	node, err := s.vfs.Stat(dirRemote)
	if err == vfs.ENOENT {
		http.Error(w, "Directory not found", http.StatusNotFound)
		return
	} else if err != nil {
		internalError(dirRemote, w, "Failed to list directory", err)
		return
	}
	if !node.IsDir() {
		http.Error(w, "Not a directory", http.StatusNotFound)
		return
	}
	dir := node.(*vfs.Dir)
	dirEntries, err := dir.ReadDirAll()
	if err != nil {
		internalError(dirRemote, w, "Failed to list directory", err)
		return
	}

	var out entries
	for _, node := range dirEntries {
		obj := node.DirEntry()
		remote := strings.Trim(obj.Remote(), "/")
		leaf := path.Base(remote)
		urlRemote := leaf
		if node.IsDir() {
			leaf += "/"
			urlRemote += "/"
		}
		out = append(out, entry{remote: remote, URL: urlRemote, Leaf: leaf})
	}

	// Account the transfer
	fs.Stats.Transferring(dirRemote)
	defer fs.Stats.DoneTransferring(dirRemote, true)

	fs.Infof(dirRemote, "%s: Serving directory", r.RemoteAddr)
	err = indexTemplate.Execute(w, indexData{
		Entries: out,
		Title:   fmt.Sprintf("Directory listing of /%s", dirRemote),
	})
	if err != nil {
		internalError(dirRemote, w, "Failed to render template", err)
		return
	}
}

// serveFile serves a file object at remote
func (s *server) serveFile(w http.ResponseWriter, r *http.Request, remote string) {
	node, err := s.vfs.Stat(remote)
	if err == vfs.ENOENT {
		fs.Infof(remote, "%s: File not found", r.RemoteAddr)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	} else if err != nil {
		internalError(remote, w, "Failed to find file", err)
		return
	}
	if !node.IsFile() {
		http.Error(w, "Not a file", http.StatusNotFound)
		return
	}
	obj := node.DirEntry().(fs.Object)
	file := node.(*vfs.File)

	// Set content length since we know how long the object is
	w.Header().Set("Content-Length", strconv.FormatInt(node.Size(), 10))

	// Set content type
	mimeType := fs.MimeType(obj)
	if mimeType == "application/octet-stream" && path.Ext(remote) == "" {
		// Leave header blank so http server guesses
	} else {
		w.Header().Set("Content-Type", mimeType)
	}

	// If HEAD no need to read the object since we have set the headers
	if r.Method == "HEAD" {
		return
	}

	// open the object
	in, err := file.OpenRead()
	if err != nil {
		internalError(remote, w, "Failed to open file", err)
		return
	}
	defer func() {
		err := in.Close()
		if err != nil {
			fs.Errorf(remote, "Failed to close file: %v", err)
		}
	}()

	// Account the transfer
	fs.Stats.Transferring(remote)
	defer fs.Stats.DoneTransferring(remote, true)
	// FIXME in = fs.NewAccount(in, obj).WithBuffer() // account the transfer

	// Serve the file
	http.ServeContent(w, r, remote, node.ModTime(), in)
}
