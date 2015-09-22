// A logging http transport

package fs

import (
	"log"
	"net/http"
	"net/http/httputil"
)

const separator = "------------------------------------------------------------"

// LoggedTransport is an http transport which logs the traffic
type LoggedTransport struct {
	wrapped http.RoundTripper
	logBody bool
}

// NewLoggedTransport wraps the transport passed in and logs all roundtrips
// including the body if logBody is set.
func NewLoggedTransport(transport http.RoundTripper, logBody bool) *LoggedTransport {
	return &LoggedTransport{
		wrapped: transport,
		logBody: logBody,
	}
}

// CancelRequest cancels an in-flight request by closing its
// connection. CancelRequest should only be called after RoundTrip has
// returned.
func (t *LoggedTransport) CancelRequest(req *http.Request) {
	if wrapped, ok := t.wrapped.(interface {
		CancelRequest(*http.Request)
	}); ok {
		log.Printf("CANCEL REQUEST %v", req)
		wrapped.CancelRequest(req)
	}
}

// RoundTrip implements the RoundTripper interface.
func (t *LoggedTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	buf, _ := httputil.DumpRequest(req, t.logBody)
	log.Println(separator)
	log.Println("HTTP REQUEST")
	log.Println(string(buf))
	log.Println(separator)
	resp, err = t.wrapped.RoundTrip(req)
	buf, _ = httputil.DumpResponse(resp, t.logBody)
	log.Println(separator)
	log.Println("HTTP RESPONSE")
	log.Println(string(buf))
	log.Println(separator)
	return resp, err
}
