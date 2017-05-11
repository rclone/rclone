// This tests the swift package internals
//
// It does not require access to a swift server
//
// FIXME need to add more tests and to check URLs and parameters
package swift

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
)

const (
	TEST_ADDRESS = "localhost:5324"
	AUTH_URL     = "http://" + TEST_ADDRESS + "/v1.0"
	PROXY_URL    = "http://" + TEST_ADDRESS + "/proxy"
	USERNAME     = "test"
	APIKEY       = "apikey"
	AUTH_TOKEN   = "token"
)

// Globals
var (
	server *SwiftServer
	c      *Connection
)

// SwiftServer implements a test swift server
type SwiftServer struct {
	t      *testing.T
	checks []*Check
}

// Used to check and reply to http transactions
type Check struct {
	in  Headers
	out Headers
	rx  *string
	tx  *string
	err *Error
	url *string
}

// Add a in check
func (check *Check) In(in Headers) *Check {
	check.in = in
	return check
}

// Add an out check
func (check *Check) Out(out Headers) *Check {
	check.out = out
	return check
}

// Add an Error check
func (check *Check) Error(StatusCode int, Text string) *Check {
	check.err = newError(StatusCode, Text)
	return check
}

// Add a rx check
func (check *Check) Rx(rx string) *Check {
	check.rx = &rx
	return check
}

// Add an tx check
func (check *Check) Tx(tx string) *Check {
	check.tx = &tx
	return check
}

// Add an URL check
func (check *Check) Url(url string) *Check {
	check.url = &url
	return check
}

// Add a check
func (s *SwiftServer) AddCheck(t *testing.T) *Check {
	server.t = t
	check := &Check{
		in:  Headers{},
		out: Headers{},
		err: nil,
	}
	s.checks = append(s.checks, check)
	return check
}

// Responds to a request
func (s *SwiftServer) Respond(w http.ResponseWriter, r *http.Request) {
	if len(s.checks) < 1 {
		s.t.Fatal("Unexpected http transaction")
	}
	check := s.checks[0]
	s.checks = s.checks[1:]

	// Check URL
	if check.url != nil && *check.url != r.URL.String() {
		s.t.Errorf("Expecting URL %q but got %q", *check.url, r.URL)
	}

	// Check headers
	for k, v := range check.in {
		actual := r.Header.Get(k)
		if actual != v {
			s.t.Errorf("Expecting header %q=%q but got %q", k, v, actual)
		}
	}
	// Write output headers
	h := w.Header()
	for k, v := range check.out {
		h.Set(k, v)
	}
	// Return an error if required
	if check.err != nil {
		http.Error(w, check.err.Text, check.err.StatusCode)
	} else {
		if check.tx != nil {
			_, err := w.Write([]byte(*check.tx))
			if err != nil {
				s.t.Error("Write failed", err)
			}
		}
	}
}

// Checks to see all responses are used up
func (s *SwiftServer) Finished() {
	if len(s.checks) > 0 {
		s.t.Error("Unused checks", s.checks)
	}
}

func handle(w http.ResponseWriter, r *http.Request) {
	// out, _ := httputil.DumpRequest(r, true)
	// os.Stdout.Write(out)
	server.Respond(w, r)
}

func NewSwiftServer() *SwiftServer {
	server := &SwiftServer{}
	http.HandleFunc("/", handle)
	go http.ListenAndServe(TEST_ADDRESS, nil)
	fmt.Print("Waiting for server to start ")
	for {
		fmt.Print(".")
		conn, err := net.Dial("tcp", TEST_ADDRESS)
		if err == nil {
			conn.Close()
			fmt.Println(" Started")
			break
		}
	}
	return server
}

func init() {
	server = NewSwiftServer()
	c = &Connection{
		UserName: USERNAME,
		ApiKey:   APIKEY,
		AuthUrl:  AUTH_URL,
	}
}

// Check the error is a swift error
func checkError(t *testing.T, err error, StatusCode int, Text string) {
	if err == nil {
		t.Fatal("No error returned")
	}
	err2, ok := err.(*Error)
	if !ok {
		t.Fatal("Bad error type")
	}
	if err2.StatusCode != StatusCode {
		t.Fatalf("Bad status code, expecting %d got %d", StatusCode, err2.StatusCode)
	}
	if err2.Text != Text {
		t.Fatalf("Bad error string, expecting %q got %q", Text, err2.Text)
	}
}

// FIXME copied from swift_test.go
func compareMaps(t *testing.T, a, b map[string]string) {
	if len(a) != len(b) {
		t.Error("Maps different sizes", a, b)
	}
	for ka, va := range a {
		if vb, ok := b[ka]; !ok || va != vb {
			t.Error("Difference in key", ka, va, b[ka])
		}
	}
	for kb, vb := range b {
		if va, ok := a[kb]; !ok || vb != va {
			t.Error("Difference in key", kb, vb, a[kb])
		}
	}
}

func TestInternalError(t *testing.T) {
	e := newError(404, "Not Found!")
	if e.StatusCode != 404 || e.Text != "Not Found!" {
		t.Fatal("Bad error")
	}
	if e.Error() != "Not Found!" {
		t.Fatal("Bad error")
	}

}

func testCheckClose(c io.Closer, e error) (err error) {
	err = e
	defer checkClose(c, &err)
	return
}

// Make a closer which returns the error of our choice
type myCloser struct {
	err error
}

func (c *myCloser) Close() error {
	return c.err
}

func TestInternalCheckClose(t *testing.T) {
	if testCheckClose(&myCloser{nil}, nil) != nil {
		t.Fatal("bad 1")
	}
	if testCheckClose(&myCloser{nil}, ObjectCorrupted) != ObjectCorrupted {
		t.Fatal("bad 2")
	}
	if testCheckClose(&myCloser{ObjectNotFound}, nil) != ObjectNotFound {
		t.Fatal("bad 3")
	}
	if testCheckClose(&myCloser{ObjectNotFound}, ObjectCorrupted) != ObjectCorrupted {
		t.Fatal("bad 4")
	}
}

func TestInternalParseHeaders(t *testing.T) {
	resp := &http.Response{StatusCode: 200}
	if c.parseHeaders(resp, nil) != nil {
		t.Error("Bad 1")
	}
	if c.parseHeaders(resp, authErrorMap) != nil {
		t.Error("Bad 1")
	}

	resp = &http.Response{StatusCode: 299}
	if c.parseHeaders(resp, nil) != nil {
		t.Error("Bad 1")
	}

	resp = &http.Response{StatusCode: 199, Status: "BOOM"}
	checkError(t, c.parseHeaders(resp, nil), 199, "HTTP Error: 199: BOOM")

	resp = &http.Response{StatusCode: 300, Status: "BOOM"}
	checkError(t, c.parseHeaders(resp, nil), 300, "HTTP Error: 300: BOOM")

	resp = &http.Response{StatusCode: 404, Status: "BOOM"}
	checkError(t, c.parseHeaders(resp, nil), 404, "HTTP Error: 404: BOOM")
	if c.parseHeaders(resp, ContainerErrorMap) != ContainerNotFound {
		t.Error("Bad 1")
	}
	if c.parseHeaders(resp, objectErrorMap) != ObjectNotFound {
		t.Error("Bad 1")
	}
}

func TestInternalReadHeaders(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	compareMaps(t, readHeaders(resp), Headers{})

	resp = &http.Response{Header: http.Header{
		"one": []string{"1"},
		"two": []string{"2"},
	}}
	compareMaps(t, readHeaders(resp), Headers{"one": "1", "two": "2"})

	// FIXME this outputs a log which we should test and check
	resp = &http.Response{Header: http.Header{
		"one": []string{"1", "11", "111"},
		"two": []string{"2"},
	}}
	compareMaps(t, readHeaders(resp), Headers{"one": "1", "two": "2"})
}

func TestInternalStorage(t *testing.T) {
	// FIXME
}

// ------------------------------------------------------------

func TestInternalAuthenticate(t *testing.T) {
	server.AddCheck(t).In(Headers{
		"User-Agent":  DefaultUserAgent,
		"X-Auth-Key":  APIKEY,
		"X-Auth-User": USERNAME,
	}).Out(Headers{
		"X-Storage-Url": PROXY_URL,
		"X-Auth-Token":  AUTH_TOKEN,
	}).Url("/v1.0")
	defer server.Finished()

	err := c.Authenticate()
	if err != nil {
		t.Fatal(err)
	}
	if c.StorageUrl != PROXY_URL {
		t.Error("Bad storage url")
	}
	if c.AuthToken != AUTH_TOKEN {
		t.Error("Bad auth token")
	}
	if !c.Authenticated() {
		t.Error("Didn't authenticate")
	}
}

func TestInternalAuthenticateDenied(t *testing.T) {
	server.AddCheck(t).Error(400, "Bad request")
	server.AddCheck(t).Error(401, "DENIED")
	defer server.Finished()
	c.UnAuthenticate()
	err := c.Authenticate()
	if err != AuthorizationFailed {
		t.Fatal("Expecting AuthorizationFailed", err)
	}
	// FIXME
	// if c.Authenticated() {
	// 	t.Fatal("Expecting not authenticated")
	// }
}

func TestInternalAuthenticateBad(t *testing.T) {
	server.AddCheck(t).Out(Headers{
		"X-Storage-Url": PROXY_URL,
	})
	defer server.Finished()
	err := c.Authenticate()
	checkError(t, err, 0, "Response didn't have storage url and auth token")
	if c.Authenticated() {
		t.Fatal("Expecting not authenticated")
	}

	server.AddCheck(t).Out(Headers{
		"X-Auth-Token": AUTH_TOKEN,
	})
	err = c.Authenticate()
	checkError(t, err, 0, "Response didn't have storage url and auth token")
	if c.Authenticated() {
		t.Fatal("Expecting not authenticated")
	}

	server.AddCheck(t)
	err = c.Authenticate()
	checkError(t, err, 0, "Response didn't have storage url and auth token")
	if c.Authenticated() {
		t.Fatal("Expecting not authenticated")
	}

	server.AddCheck(t).Out(Headers{
		"X-Storage-Url": PROXY_URL,
		"X-Auth-Token":  AUTH_TOKEN,
	})
	err = c.Authenticate()
	if err != nil {
		t.Fatal(err)
	}
	if !c.Authenticated() {
		t.Fatal("Expecting authenticated")
	}
}

func testContainerNames(t *testing.T, rx string, expected []string) {
	server.AddCheck(t).In(Headers{
		"User-Agent":   DefaultUserAgent,
		"X-Auth-Token": AUTH_TOKEN,
	}).Tx(rx).Url("/proxy")
	containers, err := c.ContainerNames(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(containers) != len(expected) {
		t.Fatal("Wrong number of containers", len(containers), rx, len(expected), expected)
	}
	for i := range containers {
		if containers[i] != expected[i] {
			t.Error("Bad container", containers[i], expected[i])
		}
	}
}
func TestInternalContainerNames(t *testing.T) {
	defer server.Finished()
	testContainerNames(t, "", []string{})
	testContainerNames(t, "one", []string{"one"})
	testContainerNames(t, "one\n", []string{"one"})
	testContainerNames(t, "one\ntwo\nthree\n", []string{"one", "two", "three"})
}

func TestInternalObjectPutBytes(t *testing.T) {
	server.AddCheck(t).In(Headers{
		"User-Agent":     DefaultUserAgent,
		"X-Auth-Token":   AUTH_TOKEN,
		"Content-Length": "5",
		"Content-Type":   "text/plain",
	}).Rx("12345")
	defer server.Finished()
	c.ObjectPutBytes("container", "object", []byte{'1', '2', '3', '4', '5'}, "text/plain")
}

func TestInternalObjectPutString(t *testing.T) {
	server.AddCheck(t).In(Headers{
		"User-Agent":     DefaultUserAgent,
		"X-Auth-Token":   AUTH_TOKEN,
		"Content-Length": "5",
		"Content-Type":   "text/plain",
	}).Rx("12345")
	defer server.Finished()
	c.ObjectPutString("container", "object", "12345", "text/plain")
}
