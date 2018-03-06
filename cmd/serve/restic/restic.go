// Package restic serves a remote suitable for use with restic
package restic

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/cmd/serve/httplib"
	"github.com/ncw/rclone/cmd/serve/httplib/httpflags"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/fserrors"
	"github.com/ncw/rclone/fs/object"
	"github.com/ncw/rclone/fs/operations"
	"github.com/ncw/rclone/fs/walk"
	"github.com/spf13/cobra"
)

func init() {
	httpflags.AddFlags(Command.Flags())
}

// Command definition for cobra
var Command = &cobra.Command{
	Use:   "restic remote:path",
	Short: `Serve the remote for restic's REST API.`,
	Long: `rclone serve restic implements restic's REST backend API
over HTTP.  This allows restic to use rclone as a data storage
mechanism for cloud providers that restic does not support directly.

[Restic](https://restic.net/) is a command line program for doing
backups.

The server will log errors.  Use -v to see access logs.

--bwlimit will be respected for file transfers.  Use --stats to
control the stats printing.

### Setting up rclone for use by restic ###

First [set up a remote for your chosen cloud provider](/docs/#configure).

Once you have set up the remote, check it is working with, for example
"rclone lsd remote:".  You may have called the remote something other
than "remote:" - just substitute whatever you called it in the
following instructions.

Now start the rclone restic server

    rclone serve restic -v remote:backup

Where you can replace "backup" in the above by whatever path in the
remote you wish to use.

By default this will serve on "localhost:8080" you can change this
with use of the "--addr" flag.

You might wish to start this server on boot.

### Setting up restic to use rclone ###

Now you can [follow the restic
instructions](http://restic.readthedocs.io/en/latest/030_preparing_a_new_repo.html#rest-server)
on setting up restic.

Note that you will need restic 0.8.2 or later to interoperate with
rclone.

For the example above you will want to use "http://localhost:8080/" as
the URL for the REST server.

For example:

    $ export RESTIC_REPOSITORY=rest:http://localhost:8080/
    $ export RESTIC_PASSWORD=yourpassword
    $ restic init
    created restic backend 8b1a4b56ae at rest:http://localhost:8080/
    
    Please note that knowledge of your password is required to access
    the repository. Losing your password means that your data is
    irrecoverably lost.
    $ restic backup /path/to/files/to/backup
    scan [/path/to/files/to/backup]
    scanned 189 directories, 312 files in 0:00
    [0:00] 100.00%  38.128 MiB / 38.128 MiB  501 / 501 items  0 errors  ETA 0:00 
    duration: 0:00
    snapshot 45c8fdd8 saved

#### Multiple repositories ####

Note that you can use the endpoint to host multiple repositories.  Do
this by adding a directory name or path after the URL.  Note that
these **must** end with /.  Eg

    $ export RESTIC_REPOSITORY=rest:http://localhost:8080/user1repo/
    # backup user1 stuff
    $ export RESTIC_REPOSITORY=rest:http://localhost:8080/user2repo/
    # backup user2 stuff

` + httplib.Help,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		f := cmd.NewFsSrc(args)
		cmd.Run(false, true, command, func() error {
			s := newServer(f, &httpflags.Opt)
			s.serve()
			return nil
		})
	},
}

// server contains everything to run the server
type server struct {
	f   fs.Fs
	srv *httplib.Server
}

func newServer(f fs.Fs, opt *httplib.Options) *server {
	mux := http.NewServeMux()
	s := &server{
		f:   f,
		srv: httplib.NewServer(mux, opt),
	}
	mux.HandleFunc("/", s.handler)
	return s
}

// serve runs the http server - doesn't return
func (s *server) serve() {
	fs.Logf(s.f, "Serving restic REST API on %s", s.srv.URL())
	s.srv.Serve()
}

var matchData = regexp.MustCompile("(?:^|/)data/([^/]{2,})$")

// Makes a remote from a URL path.  This implements the backend layout
// required by restic.
func makeRemote(path string) string {
	path = strings.Trim(path, "/")
	parts := matchData.FindStringSubmatch(path)
	// if no data directory, layout is flat
	if parts == nil {
		return path
	}
	// otherwise map
	// data/2159dd48 to
	// data/21/2159dd48
	fileName := parts[1]
	prefix := path[:len(path)-len(fileName)]
	return prefix + fileName[:2] + "/" + fileName
}

// handler reads incoming requests and dispatches them
func (s *server) handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Server", "rclone/"+fs.Version)

	path := r.URL.Path
	remote := makeRemote(path)
	fs.Debugf(s.f, "%s %s", r.Method, path)

	// Dispatch on path then method
	if strings.HasSuffix(path, "/") {
		switch r.Method {
		case "GET":
			s.listObjects(w, r, remote)
		case "POST":
			s.createRepo(w, r, remote)
		default:
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		}
	} else {
		switch r.Method {
		case "GET":
			s.getObject(w, r, remote)
		case "HEAD":
			s.headObject(w, r, remote)
		case "POST":
			s.postObject(w, r, remote)
		case "DELETE":
			s.deleteObject(w, r, remote)
		default:
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		}
	}
}

// head request the remote
func (s *server) headObject(w http.ResponseWriter, r *http.Request, remote string) {
	o, err := s.f.NewObject(remote)
	if err != nil {
		fs.Debugf(remote, "Head request error: %v", err)
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	// Set content length since we know how long the object is
	w.Header().Set("Content-Length", strconv.FormatInt(o.Size(), 10))
}

// get the remote
func (s *server) getObject(w http.ResponseWriter, r *http.Request, remote string) {
	o, err := s.f.NewObject(remote)
	if err != nil {
		fs.Debugf(remote, "Get request error: %v", err)
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	// Set content length since we know how long the object is
	w.Header().Set("Content-Length", strconv.FormatInt(o.Size(), 10))

	// Decode Range request if present
	code := http.StatusOK
	size := o.Size()
	var options []fs.OpenOption
	if rangeRequest := r.Header.Get("Range"); rangeRequest != "" {
		//fs.Debugf(nil, "Range: request %q", rangeRequest)
		option, err := fs.ParseRangeOption(rangeRequest)
		if err != nil {
			fs.Debugf(remote, "Get request parse range request error: %v", err)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		options = append(options, option)
		offset, limit := option.Decode(o.Size())
		end := o.Size() // exclusive
		if limit >= 0 {
			end = offset + limit
		}
		if end > o.Size() {
			end = o.Size()
		}
		size = end - offset
		// fs.Debugf(nil, "Range: offset=%d, limit=%d, end=%d, size=%d (object size %d)", offset, limit, end, size, o.Size())
		// Content-Range: bytes 0-1023/146515
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, end-1, o.Size()))
		// fs.Debugf(nil, "Range: Content-Range: %q", w.Header().Get("Content-Range"))
		code = http.StatusPartialContent
	}
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))

	file, err := o.Open(options...)
	if err != nil {
		fs.Debugf(remote, "Get request open error: %v", err)
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	defer func() {
		err := file.Close()
		if err != nil {
			fs.Errorf(remote, "Get request: close failed: %v", err)
		}
	}()

	w.WriteHeader(code)

	_, err = io.Copy(w, file)
	if err != nil {
		fs.Errorf(remote, "Didn't finish writing GET request: %v", err)
		return
	}
}

// postObject posts an object to the repository
func (s *server) postObject(w http.ResponseWriter, r *http.Request, remote string) {
	// fs.Debugf(s.f, "content length = %d", r.ContentLength)
	if r.ContentLength >= 0 {
		// Size known use Put
		info := object.NewStaticObjectInfo(remote, time.Now(), r.ContentLength, true, nil, s.f)
		_, err := s.f.Put(r.Body, info)
		if err != nil {
			fs.Errorf(remote, "Post request put error: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	} else {
		// Size unknown use Rcat
		_, err := operations.Rcat(s.f, remote, r.Body, time.Now())
		if err != nil {
			fs.Errorf(remote, "Post request rcat error: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
}

// delete the remote
func (s *server) deleteObject(w http.ResponseWriter, r *http.Request, remote string) {
	o, err := s.f.NewObject(remote)
	if err != nil {
		fs.Debugf(remote, "Delete request error: %v", err)
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	if err := o.Remove(); err != nil {
		fs.Errorf(remote, "Delete request remove error: %v", err)
		if err == fs.ErrorObjectNotFound {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		} else {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}
}

// listItem is an element returned for the restic v2 list response
type listItem struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// return type for list
type listItems []listItem

// add a DirEntry to the listItems
func (ls *listItems) add(entry fs.DirEntry) {
	if o, ok := entry.(fs.Object); ok {
		*ls = append(*ls, listItem{
			Name: path.Base(o.Remote()),
			Size: o.Size(),
		})
	}
}

// listObjects lists all Objects of a given type in an arbitrary order.
func (s *server) listObjects(w http.ResponseWriter, r *http.Request, remote string) {
	fs.Debugf(remote, "list request")

	var ls listItems
	err := walk.Walk(s.f, remote, true, -1, func(path string, entries fs.DirEntries, err error) error {
		if err == nil {
			for _, entry := range entries {
				ls.add(entry)
			}
		}
		return err
	})

	if err != nil {
		_, err = fserrors.Cause(err)
		if err != fs.ErrorDirNotFound {
			fs.Errorf(remote, "list failed: %#v %T", err, err)
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
	}

	data, err := json.Marshal(ls)
	if err != nil {
		fs.Errorf(remote, "list marshal failed: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	// fs.Debugf(remote, "ls result = %s", data)

	w.Header().Set("Content-Type", "application/vnd.x.restic.rest.v2")

	_, _ = w.Write(data)
}

// createRepo creates repository directories.
//
// We don't bother creating the data dirs as rclone will create them on the fly
func (s *server) createRepo(w http.ResponseWriter, r *http.Request, remote string) {
	fs.Infof(remote, "Creating repository")

	if r.URL.Query().Get("create") != "true" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	err := s.f.Mkdir(remote)
	if err != nil {
		fs.Errorf(remote, "Create repo failed to Mkdir: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	for _, name := range []string{"data", "index", "keys", "locks", "snapshots"} {
		dirRemote := path.Join(remote, name)
		err := s.f.Mkdir(dirRemote)
		if err != nil {
			fs.Errorf(dirRemote, "Create repo failed to Mkdir: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
}
