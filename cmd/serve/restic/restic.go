// Package restic serves a remote suitable for use with restic
package restic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/walk"
	libhttp "github.com/rclone/rclone/lib/http"
	"github.com/rclone/rclone/lib/http/serve"
	"github.com/rclone/rclone/lib/systemd"
	"github.com/rclone/rclone/lib/terminal"
	"github.com/spf13/cobra"
	"golang.org/x/net/http2"
)

// Options required for http server
type Options struct {
	Auth         libhttp.AuthConfig
	HTTP         libhttp.Config
	Stdio        bool
	AppendOnly   bool
	PrivateRepos bool
	CacheObjects bool
}

// DefaultOpt is the default values used for Options
var DefaultOpt = Options{
	Auth: libhttp.DefaultAuthCfg(),
	HTTP: libhttp.DefaultCfg(),
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
	flags.BoolVarP(flagSet, &Opt.Stdio, "stdio", "", false, "Run an HTTP2 server on stdin/stdout", "")
	flags.BoolVarP(flagSet, &Opt.AppendOnly, "append-only", "", false, "Disallow deletion of repository data", "")
	flags.BoolVarP(flagSet, &Opt.PrivateRepos, "private-repos", "", false, "Users can only access their private repo", "")
	flags.BoolVarP(flagSet, &Opt.CacheObjects, "cache-objects", "", true, "Cache listed objects", "")
}

// Command definition for cobra
var Command = &cobra.Command{
	Use:   "restic remote:path",
	Short: `Serve the remote for restic's REST API.`,
	Long: `Run a basic web server to serve a remote over restic's REST backend
API over HTTP.  This allows restic to use rclone as a data storage
mechanism for cloud providers that restic does not support directly.

[Restic](https://restic.net/) is a command-line program for doing
backups.

The server will log errors.  Use -v to see access logs.

` + "`--bwlimit`" + ` will be respected for file transfers.
Use ` + "`--stats`" + ` to control the stats printing.

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
with use of the ` + "`--addr`" + ` flag.

You might wish to start this server on boot.

Adding ` + "`--cache-objects=false`" + ` will cause rclone to stop caching objects
returned from the List call. Caching is normally desirable as it speeds
up downloading objects, saves transactions and uses very little memory.

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

#### Private repositories ####

The` + "`--private-repos`" + ` flag can be used to limit users to repositories starting
with a path of ` + "`/<username>/`" + `.

` + libhttp.Help(flagPrefix) + libhttp.AuthHelp(flagPrefix),
	Annotations: map[string]string{
		"versionIntroduced": "v1.40",
	},
	Run: func(command *cobra.Command, args []string) {
		ctx := context.Background()
		cmd.CheckArgs(1, 1, command, args)
		f := cmd.NewFsSrc(args)
		cmd.Run(false, true, command, func() error {
			s, err := newServer(ctx, f, &Opt)
			if err != nil {
				return err
			}
			if s.opt.Stdio {
				if terminal.IsTerminal(int(os.Stdout.Fd())) {
					return errors.New("refusing to run HTTP2 server directly on a terminal, please let restic start rclone")
				}

				conn := &StdioConn{
					stdin:  os.Stdin,
					stdout: os.Stdout,
				}

				httpSrv := &http2.Server{}
				opts := &http2.ServeConnOpts{
					Handler: s.Server.Router(),
				}
				httpSrv.ServeConn(conn, opts)
				return nil
			}
			fs.Logf(s.f, "Serving restic REST API on %s", s.URLs())

			defer systemd.Notify()()
			s.Wait()

			return nil
		})
	},
}

const (
	resticAPIV2 = "application/vnd.x.restic.rest.v2"
)

type contextRemoteType struct{}

// ContextRemoteKey is a simple context key for storing the username of the request
var ContextRemoteKey = &contextRemoteType{}

// WithRemote makes a remote from a URL path.  This implements the backend layout
// required by restic.
func WithRemote(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var urlpath string
		rctx := chi.RouteContext(r.Context())
		if rctx != nil && rctx.RoutePath != "" {
			urlpath = rctx.RoutePath
		} else {
			urlpath = r.URL.Path
		}
		urlpath = strings.Trim(urlpath, "/")
		parts := matchData.FindStringSubmatch(urlpath)
		// if no data directory, layout is flat
		if parts != nil {
			// otherwise map
			// data/2159dd48 to
			// data/21/2159dd48
			fileName := parts[1]
			prefix := urlpath[:len(urlpath)-len(fileName)]
			urlpath = prefix + fileName[:2] + "/" + fileName
		}
		ctx := context.WithValue(r.Context(), ContextRemoteKey, urlpath)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Middleware to ensure authenticated user is accessing their own private folder
func checkPrivate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := chi.URLParam(r, "userID")
		userID, ok := libhttp.CtxGetUser(r.Context())
		if ok && user != "" && user == userID {
			next.ServeHTTP(w, r)
		} else {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		}
	})
}

// server contains everything to run the server
type server struct {
	*libhttp.Server
	f     fs.Fs
	cache *cache
	opt   Options
}

func newServer(ctx context.Context, f fs.Fs, opt *Options) (s *server, err error) {
	s = &server{
		f:     f,
		cache: newCache(opt.CacheObjects),
		opt:   *opt,
	}
	// Don't bind any HTTP listeners if running with --stdio
	if opt.Stdio {
		opt.HTTP.ListenAddr = nil
	}
	s.Server, err = libhttp.NewServer(ctx,
		libhttp.WithConfig(opt.HTTP),
		libhttp.WithAuth(opt.Auth),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to init server: %w", err)
	}
	router := s.Router()
	s.Bind(router)
	s.Server.Serve()
	return s, nil
}

// bind helper for main Bind method
func (s *server) bind(router chi.Router) {
	router.MethodFunc("GET", "/*", func(w http.ResponseWriter, r *http.Request) {
		urlpath := chi.URLParam(r, "*")
		if urlpath == "" || strings.HasSuffix(urlpath, "/") {
			s.listObjects(w, r)
		} else {
			s.serveObject(w, r)
		}
	})
	router.MethodFunc("POST", "/*", func(w http.ResponseWriter, r *http.Request) {
		urlpath := chi.URLParam(r, "*")
		if urlpath == "" || strings.HasSuffix(urlpath, "/") {
			s.createRepo(w, r)
		} else {
			s.postObject(w, r)
		}
	})
	router.MethodFunc("HEAD", "/*", s.serveObject)
	router.MethodFunc("DELETE", "/*", s.deleteObject)
}

// Bind restic server routes to passed router
func (s *server) Bind(router chi.Router) {
	// FIXME
	// if m := authX.Auth(authX.Opt); m != nil {
	// 	router.Use(m)
	// }
	router.Use(
		middleware.SetHeader("Accept-Ranges", "bytes"),
		middleware.SetHeader("Server", "rclone/"+fs.Version),
		WithRemote,
	)

	if s.opt.PrivateRepos {
		router.Route("/{userID}", func(r chi.Router) {
			r.Use(checkPrivate)
			s.bind(r)
		})
		router.NotFound(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		})
	} else {
		s.bind(router)
	}
}

var matchData = regexp.MustCompile("(?:^|/)data/([^/]{2,})$")

// newObject returns an object with the remote given either from the
// cache or directly
func (s *server) newObject(ctx context.Context, remote string) (fs.Object, error) {
	o := s.cache.find(remote)
	if o != nil {
		return o, nil
	}
	o, err := s.f.NewObject(ctx, remote)
	if err != nil {
		return o, err
	}
	s.cache.add(remote, o)
	return o, nil
}

// get the remote
func (s *server) serveObject(w http.ResponseWriter, r *http.Request) {
	remote, ok := r.Context().Value(ContextRemoteKey).(string)
	if !ok {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	o, err := s.newObject(r.Context(), remote)
	if err != nil {
		fs.Debugf(remote, "%s request error: %v", r.Method, err)
		if errors.Is(err, fs.ErrorObjectNotFound) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		} else {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}
	serve.Object(w, r, o)
}

// postObject posts an object to the repository
func (s *server) postObject(w http.ResponseWriter, r *http.Request) {
	remote, ok := r.Context().Value(ContextRemoteKey).(string)
	if !ok {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if s.opt.AppendOnly {
		// make sure the file does not exist yet
		_, err := s.newObject(r.Context(), remote)
		if err == nil {
			fs.Errorf(remote, "Post request: file already exists, refusing to overwrite in append-only mode")
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)

			return
		}
	}

	o, err := operations.RcatSize(r.Context(), s.f, remote, r.Body, r.ContentLength, time.Now(), nil)
	if err != nil {
		err = accounting.Stats(r.Context()).Error(err)
		fs.Errorf(remote, "Post request rcat error: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

		return
	}

	// if successfully uploaded add to cache
	s.cache.add(remote, o)
}

// delete the remote
func (s *server) deleteObject(w http.ResponseWriter, r *http.Request) {
	remote, ok := r.Context().Value(ContextRemoteKey).(string)
	if !ok {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if s.opt.AppendOnly {
		parts := strings.Split(r.URL.Path, "/")

		// if path doesn't end in "/locks/:name", disallow the operation
		if len(parts) < 2 || parts[len(parts)-2] != "locks" {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
	}

	o, err := s.newObject(r.Context(), remote)
	if err != nil {
		fs.Debugf(remote, "Delete request error: %v", err)
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	if err := o.Remove(r.Context()); err != nil {
		fs.Errorf(remote, "Delete request remove error: %v", err)
		if errors.Is(err, fs.ErrorObjectNotFound) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		} else {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	// remove object from cache
	s.cache.remove(remote)
}

// listItem is an element returned for the restic v2 list response
type listItem struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// return type for list
type listItems []listItem

// add an fs.Object to the listItems
func (ls *listItems) add(o fs.Object) {
	*ls = append(*ls, listItem{
		Name: path.Base(o.Remote()),
		Size: o.Size(),
	})
}

// listObjects lists all Objects of a given type in an arbitrary order.
func (s *server) listObjects(w http.ResponseWriter, r *http.Request) {
	remote, ok := r.Context().Value(ContextRemoteKey).(string)
	if !ok {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if r.Header.Get("Accept") != resticAPIV2 {
		fs.Errorf(remote, "Restic v2 API required for List Objects")
		http.Error(w, "Restic v2 API required for List Objects", http.StatusBadRequest)
		return
	}
	fs.Debugf(remote, "list request")

	// make sure an empty list is returned, and not a 'nil' value
	ls := listItems{}

	// Remove all existing values from the cache
	s.cache.removePrefix(remote)

	// if remote supports ListR use that directly, otherwise use recursive Walk
	err := walk.ListR(r.Context(), s.f, remote, true, -1, walk.ListObjects, func(entries fs.DirEntries) error {
		for _, entry := range entries {
			if o, ok := entry.(fs.Object); ok {
				ls.add(o)
				s.cache.add(o.Remote(), o)
			}
		}
		return nil
	})
	if err != nil {
		if !errors.Is(err, fs.ErrorDirNotFound) {
			fs.Errorf(remote, "list failed: %#v %T", err, err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/vnd.x.restic.rest.v2")
	enc := json.NewEncoder(w)
	err = enc.Encode(ls)
	if err != nil {
		fs.Errorf(remote, "failed to write list: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

// createRepo creates repository directories.
//
// We don't bother creating the data dirs as rclone will create them on the fly
func (s *server) createRepo(w http.ResponseWriter, r *http.Request) {
	remote, ok := r.Context().Value(ContextRemoteKey).(string)
	if !ok {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	fs.Infof(remote, "Creating repository")

	if r.URL.Query().Get("create") != "true" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	err := s.f.Mkdir(r.Context(), remote)
	if err != nil {
		fs.Errorf(remote, "Create repo failed to Mkdir: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	for _, name := range []string{"data", "index", "keys", "locks", "snapshots"} {
		dirRemote := path.Join(remote, name)
		err := s.f.Mkdir(r.Context(), dirRemote)
		if err != nil {
			fs.Errorf(dirRemote, "Create repo failed to Mkdir: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
}
