// +build go1.7

package httpserver

import (
	"sync"

	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config/flags"
	"github.com/ncw/rclone/lib/atexit"
)

var (
	registerMu sync.Mutex
	router     *mux.Router
	// DefaultHTTPPort is the the default port of the HTTP server
	DefaultHTTPPort = 8083
	// DefaultHTTPAddr is the the default IP address of the HTTP server
	DefaultHTTPAddr = "localhost"
	// Flags
	httpPort = flags.IntP("http-port", "", DefaultHTTPPort, "Port of the HTTP server")
	httpAddr = flags.StringP("http-address", "", DefaultHTTPAddr, "IP address to bind on for the HTTP server")
)

// SupportsResponse is a generic response to the /supports handler
type SupportsResponse struct {
	Routes []string
	Error  string
}

// Callback is the signature of a callback method registered as a handler
// params will contain any optional query params on GET requests
// writer is the data that needs to be returned to the client. For simple map JSON use WriteJsonMap
// reader is the data sent by the request in case of POST or PUT requests
type Callback func(params map[string]string, writer io.Writer, reader io.Reader) error

// bootstrap will be run once only and starts the http server
func bootstrap(startErr chan error) {
	fs.Infof("httpserver", "Listening on %s:%d", *httpAddr, *httpPort)
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", *httpAddr, *httpPort),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	atexit.Register(func() {
		stopServer(server)
	})

	err := server.ListenAndServe()
	if err != nil {
		startErr <- err
	}
}

// Register a callback as a handler and the first call will bootstrap the http server too
// handler name should be unique for all the remotes. It is recommended to start with: <remote name>/...
// fn is the actual callback
func Register(handler string, fn Callback) error {
	var err error
	registerMu.Lock()
	defer registerMu.Unlock()

	// start the http server and register a new router
	// thread safe as Register is locked
	if router == nil {
		router = mux.NewRouter()
		router.HandleFunc("/", supportsHandler).Name("/")

		startErr := make(chan error)
		go bootstrap(startErr)

		select {
		case err = <-startErr:
		case <-time.After(1 * time.Second):
		}
	}
	// if an error was registered at startup, we fail the registration
	if err != nil {
		return err
	}

	if !strings.HasPrefix(handler, "/") {
		handler = "/" + handler
	}
	router.HandleFunc(handler, func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			err := r.Body.Close()
			if err != nil {
				fs.Debugf("httpserver", "request close error: %v", err)
			}
		}()

		// build the query params map
		params := make(map[string]string)
		if r.Method == "GET" {
			for k, v := range r.URL.Query() {
				if len(v) == 0 {
					params[k] = ""
					continue
				}
				params[k] = v[0]
			}
		} else {
			params = mux.Vars(r)
		}

		// callback
		err := fn(params, w, r.Body)
		// handle errors in a standard way
		if err != nil {
			data, _ := json.Marshal(map[string]string{"status": "error", "message": err.Error()})
			http.Error(w, string(data), http.StatusInternalServerError)
			return
		}
	}).Name(handler)

	return nil
}

// supportsHandler returns all the registered routes
func supportsHandler(w http.ResponseWriter, r *http.Request) {
	resp := &SupportsResponse{
		Routes: make([]string, 0),
		Error:  "",
	}
	// this should never be true but let's check it to be sure
	if router == nil {
		http.Error(w, "server not started", http.StatusInternalServerError)
		return
	}

	err := router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		resp.Routes = append(resp.Routes, route.GetName())
		return nil
	})
	if err != nil {
		resp.Error = err.Error()
	}

	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// WriteJSONMap is an utility function that writes the passed map as a JSON to the provided writer
func WriteJSONMap(w io.Writer, m map[string]interface{}) error {
	return json.NewEncoder(w).Encode(m)
}
