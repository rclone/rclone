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
	"os"
	"reflect"
	"testing"
	"time"
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

func testCheckClose(rd io.ReadCloser, e error) (err error) {
	err = e
	defer checkClose(rd, &err)
	return
}

// Make a closer which returns the error of our choice
type myCloser struct {
	err error
}

func (c *myCloser) Read([]byte) (int, error) {
	return 0, io.EOF
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

func TestSetFromEnv(t *testing.T) {
	// String
	s := ""

	os.Setenv("POTATO", "")
	err := setFromEnv(&s, "POTATO")
	if err != nil {
		t.Fatal(err)
	}

	os.Setenv("POTATO", "this is a test")
	err = setFromEnv(&s, "POTATO")
	if err != nil {
		t.Fatal(err)
	}
	if s != "this is a test" {
		t.Fatal("incorrect", s)
	}

	os.Setenv("POTATO", "new")
	err = setFromEnv(&s, "POTATO")
	if err != nil {
		t.Fatal(err)
	}
	if s != "this is a test" {
		t.Fatal("was reset when it shouldn't have been")
	}

	// Integer
	i := 0

	os.Setenv("POTATO", "42")
	err = setFromEnv(&i, "POTATO")
	if err != nil {
		t.Fatal(err)
	}
	if i != 42 {
		t.Fatal("incorrect", i)
	}

	os.Setenv("POTATO", "43")
	err = setFromEnv(&i, "POTATO")
	if err != nil {
		t.Fatal(err)
	}
	if i != 42 {
		t.Fatal("was reset when it shouldn't have been")
	}

	i = 0
	os.Setenv("POTATO", "not a number")
	err = setFromEnv(&i, "POTATO")
	if err == nil {
		t.Fatal("expecting error but didn't get one")
	}

	// bool
	var b bool
	os.Setenv("POTATO", "1")
	err = setFromEnv(&b, "POTATO")
	if err != nil {
		t.Fatal(err)
	}
	if b != true {
		t.Fatal("incorrect", b)
	}

	// time.Duration
	var dt time.Duration
	os.Setenv("POTATO", "5s")
	err = setFromEnv(&dt, "POTATO")
	if err != nil {
		t.Fatal(err)
	}
	if dt != 5*time.Second {
		t.Fatal("incorrect", dt)
	}

	// EndpointType
	var e EndpointType
	os.Setenv("POTATO", "internal")
	err = setFromEnv(&e, "POTATO")
	if err != nil {
		t.Fatal(err)
	}
	if e != EndpointType("internal") {
		t.Fatal("incorrect", e)
	}

	// Unknown
	var unknown struct{}
	err = setFromEnv(&unknown, "POTATO")
	if err == nil {
		t.Fatal("expecting error")
	}

	os.Setenv("POTATO", "")
}

func TestApplyEnvironment(t *testing.T) {
	// We've tested all the setting logic above, so just do a quick test here
	c := new(Connection)
	os.Setenv("GOSWIFT_CONNECT_TIMEOUT", "100s")
	err := c.ApplyEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	if c.ConnectTimeout != 100*time.Second {
		t.Fatal("timeout incorrect", c.ConnectTimeout)
	}

	c.ConnectTimeout = 0
	os.Setenv("GOSWIFT_CONNECT_TIMEOUT", "parse error")
	err = c.ApplyEnvironment()
	if err == nil {
		t.Fatal("expecting error")
	}
	if c.ConnectTimeout != 0 {
		t.Fatal("timeout incorrect", c.ConnectTimeout)
	}

	os.Setenv("GOSWIFT_CONNECT_TIMEOUT", "")
}

func TestApplyEnvironmentAll(t *testing.T) {
	// we do this in two phases because some of the variable set the same thing
	for phase := 1; phase <= 2; phase++ {
		c := new(Connection)

		items := []struct {
			phase    int
			result   interface{}
			name     string
			value    string
			want     interface{}
			oldValue string
		}{
			// Copied and amended from ApplyEnvironment
			// Environment variables - keep in same order as Connection
			{1, &c.Domain, "OS_USER_DOMAIN_NAME", "os_user_domain_name", "os_user_domain_name", ""},
			{1, &c.DomainId, "OS_USER_DOMAIN_ID", "os_user_domain_id", "os_user_domain_id", ""},
			{1, &c.UserName, "OS_USERNAME", "os_username", "os_username", ""},
			{1, &c.UserId, "OS_USER_ID", "os_user_id", "os_user_id", ""},
			{1, &c.ApiKey, "OS_PASSWORD", "os_password", "os_password", ""},
			{1, &c.AuthUrl, "OS_AUTH_URL", "os_auth_url", "os_auth_url", ""},
			{1, &c.Retries, "GOSWIFT_RETRIES", "4", 4, ""},
			{1, &c.UserAgent, "GOSWIFT_USER_AGENT", "goswift_user_agent", "goswift_user_agent", ""},
			{1, &c.ConnectTimeout, "GOSWIFT_CONNECT_TIMEOUT", "98s", 98 * time.Second, ""},
			{1, &c.Timeout, "GOSWIFT_TIMEOUT", "99s", 99 * time.Second, ""},
			{1, &c.Region, "OS_REGION_NAME", "os_region_name", "os_region_name", ""},
			{1, &c.AuthVersion, "ST_AUTH_VERSION", "3", 3, ""},
			{1, &c.Internal, "GOSWIFT_INTERNAL", "true", true, ""},
			{1, &c.Tenant, "OS_TENANT_NAME", "os_tenant_name", "os_tenant_name", ""},
			{2, &c.Tenant, "OS_PROJECT_NAME", "os_project_name", "os_project_name", ""},
			{1, &c.TenantId, "OS_TENANT_ID", "os_tenant_id", "os_tenant_id", ""},
			{1, &c.EndpointType, "OS_ENDPOINT_TYPE", "internal", EndpointTypeInternal, ""},
			{1, &c.TenantDomain, "OS_PROJECT_DOMAIN_NAME", "os_project_domain_name", "os_project_domain_name", ""},
			{1, &c.TenantDomainId, "OS_PROJECT_DOMAIN_ID", "os_project_domain_id", "os_project_domain_id", ""},
			{1, &c.TrustId, "OS_TRUST_ID", "os_trust_id", "os_trust_id", ""},
			{1, &c.StorageUrl, "OS_STORAGE_URL", "os_storage_url", "os_storage_url", ""},
			{1, &c.AuthToken, "OS_AUTH_TOKEN", "os_auth_token", "os_auth_token", ""},
			// v1 auth alternatives
			{2, &c.ApiKey, "ST_KEY", "st_key", "st_key", ""},
			{2, &c.UserName, "ST_USER", "st_user", "st_user", ""},
			{2, &c.AuthUrl, "ST_AUTH", "st_auth", "st_auth", ""},
		}

		for i := range items {
			item := &items[i]
			if item.phase == phase {
				item.oldValue = os.Getenv(item.name) // save old value
				os.Setenv(item.name, item.value)     // set new value
			}
		}

		err := c.ApplyEnvironment()
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}

		for i := range items {
			item := &items[i]
			if item.phase == phase {
				got := reflect.Indirect(reflect.ValueOf(item.result)).Interface()
				if !reflect.DeepEqual(item.want, got) {
					t.Errorf("%s: %v != %v", item.name, item.want, got)
				}
				os.Setenv(item.name, item.oldValue) // restore old value
			}
		}
	}

}
