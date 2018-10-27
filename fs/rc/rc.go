// Package rc implements a remote control server and registry for rclone
//
// To register your internal calls, call rc.Add(path, function).  Your
// function should take ane return a Param.  It can also return an
// error.  Use rc.NewError to wrap an existing error along with an
// http response type if another response other than 500 internal
// error is required on error.
package rc

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	_ "net/http/pprof" // install the pprof http handlers
	"strings"

	"github.com/ncw/rclone/cmd/serve/httplib"
	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
	"github.com/skratchdot/open-golang/open"
)

// Options contains options for the remote control server
type Options struct {
	HTTPOptions httplib.Options
	Enabled     bool   // set to enable the server
	Files       string // set to enable serving files
}

// DefaultOpt is the default values used for Options
var DefaultOpt = Options{
	HTTPOptions: httplib.DefaultOpt,
	Enabled:     false,
}

func init() {
	DefaultOpt.HTTPOptions.ListenAddr = "localhost:5572"
}

// Start the remote control server if configured
func Start(opt *Options) {
	if opt.Enabled {
		s := newServer(opt)
		go s.serve()
	}
}

// server contains everything to run the server
type server struct {
	srv   *httplib.Server
	files http.Handler
}

func newServer(opt *Options) *server {
	// Serve on the DefaultServeMux so can have global registrations appear
	mux := http.DefaultServeMux
	s := &server{
		srv: httplib.NewServer(mux, &opt.HTTPOptions),
	}
	mux.HandleFunc("/", s.handler)

	// Add some more mime types which are often missing
	mime.AddExtensionType(".wasm", "application/wasm")
	mime.AddExtensionType(".js", "application/javascript")

	// File handling
	s.files = http.NewServeMux()
	if opt.Files != "" {
		fs.Logf(nil, "Serving files from %q", opt.Files)
		s.files = http.FileServer(http.Dir(opt.Files))
	}
	return s
}

// serve runs the http server - doesn't return
func (s *server) serve() {
	err := s.srv.Serve()
	if err != nil {
		fs.Errorf(nil, "Opening listener: %v", err)
	}
	fs.Logf(nil, "Serving remote control on %s", s.srv.URL())
	// Open the files in the browser if set
	if s.files != nil {
		_ = open.Start(s.srv.URL())
	}
	s.srv.Wait()
}

// WriteJSON writes JSON in out to w
func WriteJSON(w io.Writer, out Params) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "\t")
	return enc.Encode(out)
}

// writeError writes a formatted error to the output
func writeError(path string, in Params, w http.ResponseWriter, err error, status int) {
	fs.Errorf(nil, "rc: %q: error: %v", path, err)
	// Adjust the error return for some well known errors
	switch errors.Cause(err) {
	case fs.ErrorDirNotFound, fs.ErrorObjectNotFound:
		status = http.StatusNotFound
	}
	w.WriteHeader(status)
	err = WriteJSON(w, Params{
		"error": err.Error(),
		"input": in,
	})
	if err != nil {
		// can't return the error at this point
		fs.Errorf(nil, "rc: failed to write JSON output: %v", err)
	}
}

// handler reads incoming requests and dispatches them
func (s *server) handler(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")

	w.Header().Add("Access-Control-Allow-Origin", "*")

	// echo back access control headers client needs
	reqAccessHeaders := r.Header.Get("Access-Control-Request-Headers")
	w.Header().Add("Access-Control-Allow-Headers", reqAccessHeaders)

	switch r.Method {
	case "POST":
		s.handlePost(w, r, path)
	case "OPTIONS":
		s.handleOptions(w, r, path)
	case "GET":
		s.handleGet(w, r, path)
	default:
		writeError(path, nil, w, errors.Errorf("method %q not allowed", r.Method), http.StatusMethodNotAllowed)
		return
	}
}

func (s *server) handlePost(w http.ResponseWriter, r *http.Request, path string) {
	// Parse the POST and URL parameters into r.Form, for others r.Form will be empty value
	err := r.ParseForm()
	if err != nil {
		writeError(path, nil, w, errors.Wrap(err, "failed to parse form/URL parameters"), http.StatusBadRequest)
		return
	}

	// Read the POST and URL parameters into in
	in := make(Params)
	for k, vs := range r.Form {
		if len(vs) > 0 {
			in[k] = vs[len(vs)-1]
		}
	}

	// Parse a JSON blob from the input
	if r.Header.Get("Content-Type") == "application/json" {
		err := json.NewDecoder(r.Body).Decode(&in)
		if err != nil {
			writeError(path, in, w, errors.Wrap(err, "failed to read input JSON"), http.StatusBadRequest)
			return
		}
	}

	// Find the call
	call := registry.get(path)
	if call == nil {
		writeError(path, in, w, errors.Errorf("couldn't find method %q", path), http.StatusMethodNotAllowed)
		return
	}

	// Check to see if it is async or not
	isAsync, err := in.GetBool("_async")
	if err != nil {
		writeError(path, in, w, err, http.StatusBadRequest)
		return
	}

	fs.Debugf(nil, "rc: %q: with parameters %+v", path, in)
	var out Params
	if isAsync {
		out, err = StartJob(call.Fn, in)
	} else {
		out, err = call.Fn(in)
	}
	if err != nil {
		writeError(path, in, w, err, http.StatusInternalServerError)
		return
	}
	if out == nil {
		out = make(Params)
	}

	fs.Debugf(nil, "rc: %q: reply %+v: %v", path, out, err)
	err = WriteJSON(w, out)
	if err != nil {
		// can't return the error at this point
		fs.Errorf(nil, "rc: failed to write JSON output: %v", err)
	}
}

func (s *server) handleOptions(w http.ResponseWriter, r *http.Request, path string) {
	w.WriteHeader(http.StatusOK)
}

func (s *server) handleGet(w http.ResponseWriter, r *http.Request, path string) {
	if s.files == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	s.files.ServeHTTP(w, r)
}
