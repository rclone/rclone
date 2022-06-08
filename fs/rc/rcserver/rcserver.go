// Package rcserver implements the HTTP endpoint to serve the remote control
package rcserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rclone/rclone/cmd/serve/httplib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/list"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fs/rc/jobs"
	"github.com/rclone/rclone/fs/rc/rcflags"
	"github.com/rclone/rclone/fs/rc/webgui"
	"github.com/rclone/rclone/lib/http/serve"
	"github.com/rclone/rclone/lib/random"
	"github.com/skratchdot/open-golang/open"
)

var promHandler http.Handler
var onlyOnceWarningAllowOrigin sync.Once

func init() {
	rcloneCollector := accounting.NewRcloneCollector(context.Background())
	prometheus.MustRegister(rcloneCollector)

	m := fshttp.NewMetrics("rclone")
	for _, c := range m.Collectors() {
		prometheus.MustRegister(c)
	}
	fshttp.DefaultMetrics = m

	promHandler = promhttp.Handler()
}

// Start the remote control server if configured
//
// If the server wasn't configured the *Server returned may be nil
func Start(ctx context.Context, opt *rc.Options) (*Server, error) {
	jobs.SetOpt(opt) // set the defaults for jobs
	if opt.Enabled {
		// Serve on the DefaultServeMux so can have global registrations appear
		s := newServer(ctx, opt, http.DefaultServeMux)
		return s, s.Serve()
	}
	return nil, nil
}

// Server contains everything to run the rc server
type Server struct {
	*httplib.Server
	ctx            context.Context // for global config
	files          http.Handler
	pluginsHandler http.Handler
	opt            *rc.Options
}

func newServer(ctx context.Context, opt *rc.Options, mux *http.ServeMux) *Server {
	fileHandler := http.Handler(nil)
	pluginsHandler := http.Handler(nil)
	// Add some more mime types which are often missing
	_ = mime.AddExtensionType(".wasm", "application/wasm")
	_ = mime.AddExtensionType(".js", "application/javascript")

	cachePath := filepath.Join(config.GetCacheDir(), "webgui")
	extractPath := filepath.Join(cachePath, "current/build")
	// File handling
	if opt.Files != "" {
		if opt.WebUI {
			fs.Logf(nil, "--rc-files overrides --rc-web-gui command\n")
		}
		fs.Logf(nil, "Serving files from %q", opt.Files)
		fileHandler = http.FileServer(http.Dir(opt.Files))
	} else if opt.WebUI {
		if err := webgui.CheckAndDownloadWebGUIRelease(opt.WebGUIUpdate, opt.WebGUIForceUpdate, opt.WebGUIFetchURL, config.GetCacheDir()); err != nil {
			fs.Errorf(nil, "Error while fetching the latest release of Web GUI: %v", err)
		}
		if opt.NoAuth {
			fs.Logf(nil, "It is recommended to use web gui with auth.")
		} else {
			if opt.HTTPOptions.BasicUser == "" {
				opt.HTTPOptions.BasicUser = "gui"
				fs.Infof(nil, "No username specified. Using default username: %s \n", rcflags.Opt.HTTPOptions.BasicUser)
			}
			if opt.HTTPOptions.BasicPass == "" {
				randomPass, err := random.Password(128)
				if err != nil {
					log.Fatalf("Failed to make password: %v", err)
				}
				opt.HTTPOptions.BasicPass = randomPass
				fs.Infof(nil, "No password specified. Using random password: %s \n", randomPass)
			}
		}
		opt.Serve = true

		fs.Logf(nil, "Serving Web GUI")
		fileHandler = http.FileServer(http.Dir(extractPath))

		pluginsHandler = http.FileServer(http.Dir(webgui.PluginsPath))
	}

	s := &Server{
		Server:         httplib.NewServer(mux, &opt.HTTPOptions),
		ctx:            ctx,
		opt:            opt,
		files:          fileHandler,
		pluginsHandler: pluginsHandler,
	}
	mux.HandleFunc("/", s.handler)

	return s
}

// Serve runs the http server in the background.
//
// Use s.Close() and s.Wait() to shutdown server
func (s *Server) Serve() error {
	err := s.Server.Serve()
	if err != nil {
		return err
	}
	fs.Logf(nil, "Serving remote control on %s", s.URL())
	// Open the files in the browser if set
	if s.files != nil {
		openURL, err := url.Parse(s.URL())
		if err != nil {
			return fmt.Errorf("invalid serving URL: %w", err)
		}
		// Add username, password into the URL if they are set
		user, pass := s.opt.HTTPOptions.BasicUser, s.opt.HTTPOptions.BasicPass
		if user != "" && pass != "" {
			openURL.User = url.UserPassword(user, pass)

			// Base64 encode username and password to be sent through url
			loginToken := user + ":" + pass
			parameters := url.Values{}
			encodedToken := base64.URLEncoding.EncodeToString([]byte(loginToken))
			fs.Debugf(nil, "login_token %q", encodedToken)
			parameters.Add("login_token", encodedToken)
			openURL.RawQuery = parameters.Encode()
			openURL.RawPath = "/#/login"
		}
		// Don't open browser if serving in testing environment or required not to do so.
		if flag.Lookup("test.v") == nil && !s.opt.WebGUINoOpenBrowser {
			if err := open.Start(openURL.String()); err != nil {
				fs.Errorf(nil, "Failed to open Web GUI in browser: %v. Manually access it at: %s", err, openURL.String())
			}
		} else {
			fs.Logf(nil, "Web GUI is not automatically opening browser. Navigate to %s to use.", openURL.String())
		}
	}
	return nil
}

// writeError writes a formatted error to the output
func writeError(path string, in rc.Params, w http.ResponseWriter, err error, status int) {
	fs.Errorf(nil, "rc: %q: error: %v", path, err)
	params, status := rc.Error(path, in, err, status)
	w.WriteHeader(status)
	err = rc.WriteJSON(w, params)
	if err != nil {
		// can't return the error at this point
		fs.Errorf(nil, "rc: writeError: failed to write JSON output from %#v: %v", in, err)
	}
}

// handler reads incoming requests and dispatches them
func (s *Server) handler(w http.ResponseWriter, r *http.Request) {
	urlPath, ok := s.Path(w, r)
	if !ok {
		return
	}
	path := strings.TrimLeft(urlPath, "/")

	allowOrigin := rcflags.Opt.AccessControlAllowOrigin
	if allowOrigin != "" {
		onlyOnceWarningAllowOrigin.Do(func() {
			if allowOrigin == "*" {
				fs.Logf(nil, "Warning: Allow origin set to *. This can cause serious security problems.")
			}
		})
		w.Header().Add("Access-Control-Allow-Origin", allowOrigin)
	} else {
		w.Header().Add("Access-Control-Allow-Origin", s.URL())
	}

	// echo back access control headers client needs
	//reqAccessHeaders := r.Header.Get("Access-Control-Request-Headers")
	w.Header().Add("Access-Control-Request-Method", "POST, OPTIONS, GET, HEAD")
	w.Header().Add("Access-Control-Allow-Headers", "authorization, Content-Type")

	switch r.Method {
	case "POST":
		s.handlePost(w, r, path)
	case "OPTIONS":
		s.handleOptions(w, r, path)
	case "GET", "HEAD":
		s.handleGet(w, r, path)
	default:
		writeError(path, nil, w, fmt.Errorf("method %q not allowed", r.Method), http.StatusMethodNotAllowed)
		return
	}
}

func (s *Server) handlePost(w http.ResponseWriter, r *http.Request, path string) {
	ctx := r.Context()
	contentType := r.Header.Get("Content-Type")

	values := r.URL.Query()
	if contentType == "application/x-www-form-urlencoded" {
		// Parse the POST and URL parameters into r.Form, for others r.Form will be empty value
		err := r.ParseForm()
		if err != nil {
			writeError(path, nil, w, fmt.Errorf("failed to parse form/URL parameters: %w", err), http.StatusBadRequest)
			return
		}
		values = r.Form
	}

	// Read the POST and URL parameters into in
	in := make(rc.Params)
	for k, vs := range values {
		if len(vs) > 0 {
			in[k] = vs[len(vs)-1]
		}
	}

	// Parse a JSON blob from the input
	if contentType == "application/json" {
		err := json.NewDecoder(r.Body).Decode(&in)
		if err != nil {
			writeError(path, in, w, fmt.Errorf("failed to read input JSON: %w", err), http.StatusBadRequest)
			return
		}
	}
	// Find the call
	call := rc.Calls.Get(path)
	if call == nil {
		writeError(path, in, w, fmt.Errorf("couldn't find method %q", path), http.StatusNotFound)
		return
	}

	// Check to see if it requires authorisation
	if !s.opt.NoAuth && call.AuthRequired && !s.UsingAuth() {
		writeError(path, in, w, fmt.Errorf("authentication must be set up on the rc server to use %q or the --rc-no-auth flag must be in use", path), http.StatusForbidden)
		return
	}

	inOrig := in.Copy()

	if call.NeedsRequest {
		// Add the request to RC
		in["_request"] = r
	}

	if call.NeedsResponse {
		in["_response"] = w
	}

	fs.Debugf(nil, "rc: %q: with parameters %+v", path, in)
	job, out, err := jobs.NewJob(ctx, call.Fn, in)
	if job != nil {
		w.Header().Add("x-rclone-jobid", fmt.Sprintf("%d", job.ID))
	}
	if err != nil {
		writeError(path, inOrig, w, err, http.StatusInternalServerError)
		return
	}
	if out == nil {
		out = make(rc.Params)
	}

	fs.Debugf(nil, "rc: %q: reply %+v: %v", path, out, err)
	err = rc.WriteJSON(w, out)
	if err != nil {
		// can't return the error at this point - but have a go anyway
		writeError(path, inOrig, w, err, http.StatusInternalServerError)
		fs.Errorf(nil, "rc: handlePost: failed to write JSON output: %v", err)
	}
}

func (s *Server) handleOptions(w http.ResponseWriter, r *http.Request, path string) {
	w.WriteHeader(http.StatusOK)
}

func (s *Server) serveRoot(w http.ResponseWriter, r *http.Request) {
	remotes := config.FileSections()
	sort.Strings(remotes)
	directory := serve.NewDirectory("", s.HTMLTemplate)
	directory.Name = "List of all rclone remotes."
	q := url.Values{}
	for _, remote := range remotes {
		q.Set("fs", remote)
		directory.AddHTMLEntry("["+remote+":]", true, -1, time.Time{})
	}
	sortParm := r.URL.Query().Get("sort")
	orderParm := r.URL.Query().Get("order")
	directory.ProcessQueryParams(sortParm, orderParm)

	directory.Serve(w, r)
}

func (s *Server) serveRemote(w http.ResponseWriter, r *http.Request, path string, fsName string) {
	f, err := cache.Get(s.ctx, fsName)
	if err != nil {
		writeError(path, nil, w, fmt.Errorf("failed to make Fs: %w", err), http.StatusInternalServerError)
		return
	}
	if path == "" || strings.HasSuffix(path, "/") {
		path = strings.Trim(path, "/")
		entries, err := list.DirSorted(r.Context(), f, false, path)
		if err != nil {
			writeError(path, nil, w, fmt.Errorf("failed to list directory: %w", err), http.StatusInternalServerError)
			return
		}
		// Make the entries for display
		directory := serve.NewDirectory(path, s.HTMLTemplate)
		for _, entry := range entries {
			_, isDir := entry.(fs.Directory)
			//directory.AddHTMLEntry(entry.Remote(), isDir, entry.Size(), entry.ModTime(r.Context()))
			directory.AddHTMLEntry(entry.Remote(), isDir, entry.Size(), time.Time{})
		}
		sortParm := r.URL.Query().Get("sort")
		orderParm := r.URL.Query().Get("order")
		directory.ProcessQueryParams(sortParm, orderParm)

		directory.Serve(w, r)
	} else {
		path = strings.Trim(path, "/")
		o, err := f.NewObject(r.Context(), path)
		if err != nil {
			writeError(path, nil, w, fmt.Errorf("failed to find object: %w", err), http.StatusInternalServerError)
			return
		}
		serve.Object(w, r, o)
	}
}

// Match URLS of the form [fs]/remote
var fsMatch = regexp.MustCompile(`^\[(.*?)\](.*)$`)

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request, path string) {
	// Look to see if this has an fs in the path
	fsMatchResult := fsMatch.FindStringSubmatch(path)

	switch {
	case fsMatchResult != nil && s.opt.Serve:
		// Serve /[fs]/remote files
		s.serveRemote(w, r, fsMatchResult[2], fsMatchResult[1])
		return
	case path == "metrics" && s.opt.EnableMetrics:
		promHandler.ServeHTTP(w, r)
		return
	case path == "*" && s.opt.Serve:
		// Serve /* as the remote listing
		s.serveRoot(w, r)
		return
	case s.files != nil:
		if s.opt.WebUI {
			pluginsMatchResult := webgui.PluginsMatch.FindStringSubmatch(path)

			if len(pluginsMatchResult) > 2 {
				ok := webgui.ServePluginOK(w, r, pluginsMatchResult)
				if !ok {
					r.URL.Path = fmt.Sprintf("/%s/%s/app/build/%s", pluginsMatchResult[1], pluginsMatchResult[2], pluginsMatchResult[3])
					s.pluginsHandler.ServeHTTP(w, r)
					return
				}
				return
			} else if webgui.ServePluginWithReferrerOK(w, r, path) {
				return
			}
		}
		// Serve the files
		r.URL.Path = "/" + path
		s.files.ServeHTTP(w, r)
		return
	case path == "" && s.opt.Serve:
		// Serve the root as a remote listing
		s.serveRoot(w, r)
		return
	}
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}
