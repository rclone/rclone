// A logging http transport

package fs

import (
	"log"
	"net/http"
	"net/http/httputil"
)

const (
	separatorReq  = ">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>"
	separatorResp = "<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<"
)

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
	buf, _ := httputil.DumpRequestOut(req, t.logBody)
	log.Println(separatorReq)
	log.Println("HTTP REQUEST")
	log.Println(string(buf))
	log.Println(separatorReq)
	resp, err = t.wrapped.RoundTrip(req)
	log.Println(separatorResp)
	log.Println("HTTP RESPONSE")
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		buf, _ = httputil.DumpResponse(resp, t.logBody)
		log.Println(string(buf))
	}
	log.Println(separatorResp)
	return resp, err
}
