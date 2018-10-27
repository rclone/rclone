// Package rcserver implements the HTTP endpoint to serve the remote control
package rcserver

import (
	"encoding/json"
	"mime"
	"net/http"
	"strings"

	"github.com/ncw/rclone/cmd/serve/httplib"
	"github.com/ncw/rclone/cmd/serve/httplib/serve"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/rc"
	"github.com/pkg/errors"
	"github.com/skratchdot/open-golang/open"
)

// Start the remote control server if configured
func Start(opt *rc.Options) {
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

func newServer(opt *rc.Options) *server {
	// Serve on the DefaultServeMux so can have global registrations appear
	mux := http.DefaultServeMux
	s := &server{
		srv: httplib.NewServer(mux, &opt.HTTPOptions),
	}
	mux.HandleFunc("/", s.handler)

	// Add some more mime types which are often missing
	_ = mime.AddExtensionType(".wasm", "application/wasm")
	_ = mime.AddExtensionType(".js", "application/javascript")

	// File handling
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

// writeError writes a formatted error to the output
func writeError(path string, in rc.Params, w http.ResponseWriter, err error, status int) {
	fs.Errorf(nil, "rc: %q: error: %v", path, err)
	// Adjust the error return for some well known errors
	errOrig := errors.Cause(err)
	switch {
	case errOrig == fs.ErrorDirNotFound || errOrig == fs.ErrorObjectNotFound:
		status = http.StatusNotFound
	case rc.IsErrParamInvalid(err) || rc.IsErrParamNotFound(err):
		status = http.StatusBadRequest
	}
	w.WriteHeader(status)
	err = rc.WriteJSON(w, rc.Params{
		"status": status,
		"error":  err.Error(),
		"input":  in,
		"path":   path,
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
	in := make(rc.Params)
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
	call := rc.Calls.Get(path)
	if call == nil {
		writeError(path, in, w, errors.Errorf("couldn't find method %q", path), http.StatusMethodNotAllowed)
		return
	}

	// Check to see if it is async or not
	isAsync, err := in.GetBool("_async")
	if rc.NotErrParamNotFound(err) {
		writeError(path, in, w, err, http.StatusBadRequest)
		return
	}

	fs.Debugf(nil, "rc: %q: with parameters %+v", path, in)
	var out rc.Params
	if isAsync {
		out, err = rc.StartJob(call.Fn, in)
	} else {
		out, err = call.Fn(in)
	}
	if err != nil {
		writeError(path, in, w, err, http.StatusInternalServerError)
		return
	}
	if out == nil {
		out = make(rc.Params)
	}

	fs.Debugf(nil, "rc: %q: reply %+v: %v", path, out, err)
	err = rc.WriteJSON(w, out)
	if err != nil {
		// can't return the error at this point
		fs.Errorf(nil, "rc: failed to write JSON output: %v", err)
	}
}

func (s *server) handleOptions(w http.ResponseWriter, r *http.Request, path string) {
	w.WriteHeader(http.StatusOK)
}

func (s *server) handleGet(w http.ResponseWriter, r *http.Request, path string) {
	// if we have an &fs parameter we are serving from a different fs
	fsName := r.URL.Query().Get("fs")
	if fsName != "" {
		f, err := rc.GetCachedFs(fsName)
		if err != nil {
			writeError(path, nil, w, errors.Wrap(err, "failed to make Fs"), http.StatusInternalServerError)
			return
		}
		o, err := f.NewObject(path)
		if err != nil {
			writeError(path, nil, w, errors.Wrap(err, "failed to find object"), http.StatusInternalServerError)
			return
		}
		serve.Object(w, r, o)
	} else if s.files == nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	} else {
		s.files.ServeHTTP(w, r)
	}
}
