//go:build go1.21

package dlna

import (
	"crypto/md5"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"

	"github.com/anacrolix/dms/soap"
	"github.com/anacrolix/dms/upnp"
	"github.com/rclone/rclone/fs"
)

// Return a default "friendly name" for the server.
func makeDefaultFriendlyName() string {
	hostName, err := os.Hostname()
	if err != nil {
		hostName = ""
	} else {
		hostName = " (" + hostName + ")"
	}
	return "rclone" + hostName
}

func makeDeviceUUID(unique string) string {
	h := md5.New()
	if _, err := io.WriteString(h, unique); err != nil {
		log.Panicf("makeDeviceUUID write failed: %s", err)
	}
	buf := h.Sum(nil)
	return upnp.FormatUUID(buf)
}

// Get all available active network interfaces.
func listInterfaces() []net.Interface {
	ifs, err := net.Interfaces()
	if err != nil {
		log.Printf("list network interfaces: %v", err)
		return []net.Interface{}
	}

	var active []net.Interface
	for _, intf := range ifs {
		if isAppropriatelyConfigured(intf) {
			active = append(active, intf)
		}
	}
	return active
}

func isAppropriatelyConfigured(intf net.Interface) bool {
	return intf.Flags&net.FlagUp != 0 && intf.Flags&net.FlagMulticast != 0 && intf.MTU > 0
}

func didlLite(chardata string) string {
	return `<DIDL-Lite` +
		` xmlns:dc="http://purl.org/dc/elements/1.1/"` +
		` xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/"` +
		` xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/"` +
		` xmlns:dlna="urn:schemas-dlna-org:metadata-1-0/">` +
		chardata +
		`</DIDL-Lite>`
}

func mustMarshalXML(value interface{}) []byte {
	ret, err := xml.MarshalIndent(value, "", "  ")
	if err != nil {
		log.Panicf("mustMarshalXML failed to marshal %v: %s", value, err)
	}
	return ret
}

// Marshal SOAP response arguments into a response XML snippet.
func marshalSOAPResponse(sa upnp.SoapAction, args map[string]string) []byte {
	soapArgs := make([]soap.Arg, 0, len(args))
	for argName, value := range args {
		soapArgs = append(soapArgs, soap.Arg{
			XMLName: xml.Name{Local: argName},
			Value:   value,
		})
	}
	return []byte(fmt.Sprintf(`<u:%[1]sResponse xmlns:u="%[2]s">%[3]s</u:%[1]sResponse>`,
		sa.Action, sa.ServiceURN.String(), mustMarshalXML(soapArgs)))
}

type loggingResponseWriter struct {
	http.ResponseWriter
	request   *http.Request
	committed bool
}

func (lrw *loggingResponseWriter) logRequest(code int, err interface{}) {
	// Choose appropriate log level based on response status code.
	var level fs.LogLevel
	if code < 400 && err == nil {
		level = fs.LogLevelInfo
	} else {
		level = fs.LogLevelError
	}

	if err == nil {
		err = ""
	}

	fs.LogPrintf(level, lrw.request.URL, "%s %s %d %s %s",
		lrw.request.RemoteAddr, lrw.request.Method, code,
		lrw.request.Header.Get("SOAPACTION"), err)
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.committed = true
	lrw.logRequest(code, nil)
	lrw.ResponseWriter.WriteHeader(code)
}

// HTTP handler that logs requests and any errors or panics.
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lrw := &loggingResponseWriter{ResponseWriter: w, request: r}
		defer func() {
			err := recover()
			if err != nil {
				if !lrw.committed {
					lrw.logRequest(http.StatusInternalServerError, err)
					http.Error(w, fmt.Sprint(err), http.StatusInternalServerError)
				} else {
					// Too late to send the error to client, but at least log it.
					fs.Errorf(r.URL.Path, "Recovered panic: %v", err)
				}
			}
		}()
		next.ServeHTTP(lrw, r)
	})
}

// HTTP handler that logs complete request and response bodies for debugging.
// Error recovery and general request logging are left to logging().
func traceLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dump, err := httputil.DumpRequest(r, true)
		if err != nil {
			serveError(nil, w, "error dumping request", err)
			return
		}
		fs.Debugf(nil, "%s", dump)

		recorder := httptest.NewRecorder()
		next.ServeHTTP(recorder, r)

		dump, err = httputil.DumpResponse(recorder.Result(), true)
		if err != nil {
			// log the error but ignore it
			fs.Errorf(nil, "error dumping response: %v", err)
		} else {
			fs.Debugf(nil, "%s", dump)
		}

		// copy from recorder to the real response writer
		for k, v := range recorder.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(recorder.Code)
		_, err = recorder.Body.WriteTo(w)
		if err != nil {
			// Network error
			fs.Debugf(nil, "Error writing response: %v", err)
		}
	})
}

// HTTP handler that sets headers.
func withHeader(name string, value string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(name, value)
		next.ServeHTTP(w, r)
	})
}

// serveError returns an http.StatusInternalServerError and logs the error
func serveError(what interface{}, w http.ResponseWriter, text string, err error) {
	err = fs.CountError(err)
	fs.Errorf(what, "%s: %v", text, err)
	http.Error(w, text+".", http.StatusInternalServerError)
}

// Splits a path into (root, ext) such that root + ext == path, and ext is empty
// or begins with a period.  Extended version of path.Ext().
func splitExt(path string) (string, string) {
	for i := len(path) - 1; i >= 0 && path[i] != '/'; i-- {
		if path[i] == '.' {
			return path[:i], path[i:]
		}
	}
	return path, ""
}
