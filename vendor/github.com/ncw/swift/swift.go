package swift

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultUserAgent    = "goswift/1.0"         // Default user agent
	DefaultRetries      = 3                     // Default number of retries on token expiry
	TimeFormat          = "2006-01-02T15:04:05" // Python date format for json replies parsed as UTC
	UploadTar           = "tar"                 // Data format specifier for Connection.BulkUpload().
	UploadTarGzip       = "tar.gz"              // Data format specifier for Connection.BulkUpload().
	UploadTarBzip2      = "tar.bz2"             // Data format specifier for Connection.BulkUpload().
	allContainersLimit  = 10000                 // Number of containers to fetch at once
	allObjectsLimit     = 10000                 // Number objects to fetch at once
	allObjectsChanLimit = 1000                  // ...when fetching to a channel
)

// ObjectType is the type of the swift object, regular, static large,
// or dynamic large.
type ObjectType int

// Values that ObjectType can take
const (
	RegularObjectType ObjectType = iota
	StaticLargeObjectType
	DynamicLargeObjectType
)

// Connection holds the details of the connection to the swift server.
//
// You need to provide UserName, ApiKey and AuthUrl when you create a
// connection then call Authenticate on it.
//
// The auth version in use will be detected from the AuthURL - you can
// override this with the AuthVersion parameter.
//
// If using v2 auth you can also set Region in the Connection
// structure.  If you don't set Region you will get the default region
// which may not be what you want.
//
// For reference some common AuthUrls looks like this:
//
//  Rackspace US        https://auth.api.rackspacecloud.com/v1.0
//  Rackspace UK        https://lon.auth.api.rackspacecloud.com/v1.0
//  Rackspace v2        https://identity.api.rackspacecloud.com/v2.0
//  Memset Memstore UK  https://auth.storage.memset.com/v1.0
//  Memstore v2         https://auth.storage.memset.com/v2.0
//
// When using Google Appengine you must provide the Connection with an
// appengine-specific Transport:
//
//	import (
//		"appengine/urlfetch"
//		"fmt"
//		"github.com/ncw/swift"
//	)
//
//	func handler(w http.ResponseWriter, r *http.Request) {
//		ctx := appengine.NewContext(r)
//		tr := urlfetch.Transport{Context: ctx}
//		c := swift.Connection{
//			UserName:  "user",
//			ApiKey:    "key",
//			AuthUrl:   "auth_url",
//			Transport: tr,
//		}
//		_ := c.Authenticate()
//		containers, _ := c.ContainerNames(nil)
//		fmt.Fprintf(w, "containers: %q", containers)
//	}
//
// If you don't supply a Transport, one is made which relies on
// http.ProxyFromEnvironment (http://golang.org/pkg/net/http/#ProxyFromEnvironment).
// This means that the connection will respect the HTTP proxy specified by the
// environment variables $HTTP_PROXY and $NO_PROXY.
type Connection struct {
	// Parameters - fill these in before calling Authenticate
	// They are all optional except UserName, ApiKey and AuthUrl
	Domain                      string            // User's domain name
	DomainId                    string            // User's domain Id
	UserName                    string            // UserName for api
	UserId                      string            // User Id
	ApiKey                      string            // Key for api access
	ApplicationCredentialId     string            // Application Credential ID
	ApplicationCredentialName   string            // Application Credential Name
	ApplicationCredentialSecret string            // Application Credential Secret
	AuthUrl                     string            // Auth URL
	Retries                     int               // Retries on error (default is 3)
	UserAgent                   string            // Http User agent (default goswift/1.0)
	ConnectTimeout              time.Duration     // Connect channel timeout (default 10s)
	Timeout                     time.Duration     // Data channel timeout (default 60s)
	Region                      string            // Region to use eg "LON", "ORD" - default is use first region (v2,v3 auth only)
	AuthVersion                 int               // Set to 1, 2 or 3 or leave at 0 for autodetect
	Internal                    bool              // Set this to true to use the the internal / service network
	Tenant                      string            // Name of the tenant (v2,v3 auth only)
	TenantId                    string            // Id of the tenant (v2,v3 auth only)
	EndpointType                EndpointType      // Endpoint type (v2,v3 auth only) (default is public URL unless Internal is set)
	TenantDomain                string            // Name of the tenant's domain (v3 auth only), only needed if it differs from the user domain
	TenantDomainId              string            // Id of the tenant's domain (v3 auth only), only needed if it differs the from user domain
	TrustId                     string            // Id of the trust (v3 auth only)
	Transport                   http.RoundTripper `json:"-" xml:"-"` // Optional specialised http.Transport (eg. for Google Appengine)
	// These are filled in after Authenticate is called as are the defaults for above
	StorageUrl string
	AuthToken  string
	Expires    time.Time // time the token expires, may be Zero if unknown
	client     *http.Client
	Auth       Authenticator `json:"-" xml:"-"` // the current authenticator
	authLock   sync.Mutex    // lock when R/W StorageUrl, AuthToken, Auth
	// swiftInfo is filled after QueryInfo is called
	swiftInfo SwiftInfo
}

// setFromEnv reads the value that param points to (it must be a
// pointer), if it isn't the zero value then it reads the environment
// variable name passed in, parses it according to the type and writes
// it to the pointer.
func setFromEnv(param interface{}, name string) (err error) {
	val := os.Getenv(name)
	if val == "" {
		return
	}
	switch result := param.(type) {
	case *string:
		if *result == "" {
			*result = val
		}
	case *int:
		if *result == 0 {
			*result, err = strconv.Atoi(val)
		}
	case *bool:
		if *result == false {
			*result, err = strconv.ParseBool(val)
		}
	case *time.Duration:
		if *result == 0 {
			*result, err = time.ParseDuration(val)
		}
	case *EndpointType:
		if *result == EndpointType("") {
			*result = EndpointType(val)
		}
	default:
		return newErrorf(0, "can't set var of type %T", param)
	}
	return err
}

// ApplyEnvironment reads environment variables and applies them to
// the Connection structure.  It won't overwrite any parameters which
// are already set in the Connection struct.
//
// To make a new Connection object entirely from the environment you
// would do:
//
//    c := new(Connection)
//    err := c.ApplyEnvironment()
//    if err != nil { log.Fatal(err) }
//
// The naming of these variables follows the official Openstack naming
// scheme so it should be compatible with OpenStack rc files.
//
// For v1 authentication (obsolete)
//     ST_AUTH - Auth URL
//     ST_USER - UserName for api
//     ST_KEY - Key for api access
//
// For v2 authentication
//     OS_AUTH_URL - Auth URL
//     OS_USERNAME - UserName for api
//     OS_PASSWORD - Key for api access
//     OS_TENANT_NAME - Name of the tenant
//     OS_TENANT_ID   - Id of the tenant
//     OS_REGION_NAME - Region to use - default is use first region
//
// For v3 authentication
//     OS_AUTH_URL - Auth URL
//     OS_USERNAME - UserName for api
//     OS_USER_ID - User Id
//     OS_PASSWORD - Key for api access
//     OS_APPLICATION_CREDENTIAL_ID - Application Credential ID
//     OS_APPLICATION_CREDENTIAL_NAME - Application Credential Name
//     OS_APPLICATION_CREDENTIAL_SECRET - Application Credential Secret
//     OS_USER_DOMAIN_NAME - User's domain name
//     OS_USER_DOMAIN_ID - User's domain Id
//     OS_PROJECT_NAME - Name of the project
//     OS_PROJECT_DOMAIN_NAME - Name of the tenant's domain, only needed if it differs from the user domain
//     OS_PROJECT_DOMAIN_ID - Id of the tenant's domain, only needed if it differs the from user domain
//     OS_TRUST_ID - If of the trust
//     OS_REGION_NAME - Region to use - default is use first region
//
// Other
//     OS_ENDPOINT_TYPE - Endpoint type public, internal or admin
//     ST_AUTH_VERSION - Choose auth version - 1, 2 or 3 or leave at 0 for autodetect
//
// For manual authentication
//     OS_STORAGE_URL - storage URL from alternate authentication
//     OS_AUTH_TOKEN - Auth Token from alternate authentication
//
// Library specific
//     GOSWIFT_RETRIES - Retries on error (default is 3)
//     GOSWIFT_USER_AGENT - HTTP User agent (default goswift/1.0)
//     GOSWIFT_CONNECT_TIMEOUT - Connect channel timeout with unit, eg "10s", "100ms" (default "10s")
//     GOSWIFT_TIMEOUT - Data channel timeout with unit, eg "10s", "100ms" (default "60s")
//     GOSWIFT_INTERNAL - Set this to "true" to use the the internal network (obsolete - use OS_ENDPOINT_TYPE)
func (c *Connection) ApplyEnvironment() (err error) {
	for _, item := range []struct {
		result interface{}
		name   string
	}{
		// Environment variables - keep in same order as Connection
		{&c.Domain, "OS_USER_DOMAIN_NAME"},
		{&c.DomainId, "OS_USER_DOMAIN_ID"},
		{&c.UserName, "OS_USERNAME"},
		{&c.UserId, "OS_USER_ID"},
		{&c.ApiKey, "OS_PASSWORD"},
		{&c.ApplicationCredentialId, "OS_APPLICATION_CREDENTIAL_ID"},
		{&c.ApplicationCredentialName, "OS_APPLICATION_CREDENTIAL_NAME"},
		{&c.ApplicationCredentialSecret, "OS_APPLICATION_CREDENTIAL_SECRET"},
		{&c.AuthUrl, "OS_AUTH_URL"},
		{&c.Retries, "GOSWIFT_RETRIES"},
		{&c.UserAgent, "GOSWIFT_USER_AGENT"},
		{&c.ConnectTimeout, "GOSWIFT_CONNECT_TIMEOUT"},
		{&c.Timeout, "GOSWIFT_TIMEOUT"},
		{&c.Region, "OS_REGION_NAME"},
		{&c.AuthVersion, "ST_AUTH_VERSION"},
		{&c.Internal, "GOSWIFT_INTERNAL"},
		{&c.Tenant, "OS_TENANT_NAME"},  //v2
		{&c.Tenant, "OS_PROJECT_NAME"}, // v3
		{&c.TenantId, "OS_TENANT_ID"},
		{&c.EndpointType, "OS_ENDPOINT_TYPE"},
		{&c.TenantDomain, "OS_PROJECT_DOMAIN_NAME"},
		{&c.TenantDomainId, "OS_PROJECT_DOMAIN_ID"},
		{&c.TrustId, "OS_TRUST_ID"},
		{&c.StorageUrl, "OS_STORAGE_URL"},
		{&c.AuthToken, "OS_AUTH_TOKEN"},
		// v1 auth alternatives
		{&c.ApiKey, "ST_KEY"},
		{&c.UserName, "ST_USER"},
		{&c.AuthUrl, "ST_AUTH"},
	} {
		err = setFromEnv(item.result, item.name)
		if err != nil {
			return newErrorf(0, "failed to read env var %q: %v", item.name, err)
		}
	}
	return nil
}

// Error - all errors generated by this package are of this type.  Other error
// may be passed on from library functions though.
type Error struct {
	StatusCode int // HTTP status code if relevant or 0 if not
	Text       string
}

// Error satisfy the error interface.
func (e *Error) Error() string {
	return e.Text
}

// newError make a new error from a string.
func newError(StatusCode int, Text string) *Error {
	return &Error{
		StatusCode: StatusCode,
		Text:       Text,
	}
}

// newErrorf makes a new error from sprintf parameters.
func newErrorf(StatusCode int, Text string, Parameters ...interface{}) *Error {
	return newError(StatusCode, fmt.Sprintf(Text, Parameters...))
}

// errorMap defines http error codes to error mappings.
type errorMap map[int]error

var (
	// Specific Errors you might want to check for equality
	NotModified         = newError(304, "Not Modified")
	BadRequest          = newError(400, "Bad Request")
	AuthorizationFailed = newError(401, "Authorization Failed")
	ContainerNotFound   = newError(404, "Container Not Found")
	ContainerNotEmpty   = newError(409, "Container Not Empty")
	ObjectNotFound      = newError(404, "Object Not Found")
	ObjectCorrupted     = newError(422, "Object Corrupted")
	TimeoutError        = newError(408, "Timeout when reading or writing data")
	Forbidden           = newError(403, "Operation forbidden")
	TooLargeObject      = newError(413, "Too Large Object")
	RateLimit           = newError(498, "Rate Limit")
	TooManyRequests     = newError(429, "TooManyRequests")

	// Mappings for authentication errors
	authErrorMap = errorMap{
		400: BadRequest,
		401: AuthorizationFailed,
		403: Forbidden,
	}

	// Mappings for container errors
	ContainerErrorMap = errorMap{
		400: BadRequest,
		403: Forbidden,
		404: ContainerNotFound,
		409: ContainerNotEmpty,
		498: RateLimit,
	}

	// Mappings for object errors
	objectErrorMap = errorMap{
		304: NotModified,
		400: BadRequest,
		403: Forbidden,
		404: ObjectNotFound,
		413: TooLargeObject,
		422: ObjectCorrupted,
		429: TooManyRequests,
		498: RateLimit,
	}
)

// checkClose is used to check the return from Close in a defer
// statement.
func checkClose(c io.Closer, err *error) {
	cerr := c.Close()
	if *err == nil {
		*err = cerr
	}
}

// drainAndClose discards all data from rd and closes it.
// If an error occurs during Read, it is discarded.
func drainAndClose(rd io.ReadCloser, err *error) {
	if rd == nil {
		return
	}

	_, _ = io.Copy(ioutil.Discard, rd)
	cerr := rd.Close()
	if err != nil && *err == nil {
		*err = cerr
	}
}

// parseHeaders checks a response for errors and translates into
// standard errors if necessary. If an error is returned, resp.Body
// has been drained and closed.
func (c *Connection) parseHeaders(resp *http.Response, errorMap errorMap) error {
	if errorMap != nil {
		if err, ok := errorMap[resp.StatusCode]; ok {
			drainAndClose(resp.Body, nil)
			return err
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		drainAndClose(resp.Body, nil)
		return newErrorf(resp.StatusCode, "HTTP Error: %d: %s", resp.StatusCode, resp.Status)
	}
	return nil
}

// readHeaders returns a Headers object from the http.Response.
//
// If it receives multiple values for a key (which should never
// happen) it will use the first one
func readHeaders(resp *http.Response) Headers {
	headers := Headers{}
	for key, values := range resp.Header {
		headers[key] = values[0]
	}
	return headers
}

// Headers stores HTTP headers (can only have one of each header like Swift).
type Headers map[string]string

// Does an http request using the running timer passed in
func (c *Connection) doTimeoutRequest(timer *time.Timer, req *http.Request) (*http.Response, error) {
	// Do the request in the background so we can check the timeout
	type result struct {
		resp *http.Response
		err  error
	}
	done := make(chan result, 1)
	go func() {
		resp, err := c.client.Do(req)
		done <- result{resp, err}
	}()
	// Wait for the read or the timeout
	select {
	case r := <-done:
		return r.resp, r.err
	case <-timer.C:
		// Kill the connection on timeout so we don't leak sockets or goroutines
		cancelRequest(c.Transport, req)
		return nil, TimeoutError
	}
	panic("unreachable") // For Go 1.0
}

// Set defaults for any unset values
//
// Call with authLock held
func (c *Connection) setDefaults() {
	if c.UserAgent == "" {
		c.UserAgent = DefaultUserAgent
	}
	if c.Retries == 0 {
		c.Retries = DefaultRetries
	}
	if c.ConnectTimeout == 0 {
		c.ConnectTimeout = 10 * time.Second
	}
	if c.Timeout == 0 {
		c.Timeout = 60 * time.Second
	}
	if c.Transport == nil {
		t := &http.Transport{
			//		TLSClientConfig:    &tls.Config{RootCAs: pool},
			//		DisableCompression: true,
			Proxy: http.ProxyFromEnvironment,
			// Half of linux's default open files limit (1024).
			MaxIdleConnsPerHost: 512,
		}
		SetExpectContinueTimeout(t, 5*time.Second)
		c.Transport = t
	}
	if c.client == nil {
		c.client = &http.Client{
			//		CheckRedirect: redirectPolicyFunc,
			Transport: c.Transport,
		}
	}
}

// Authenticate connects to the Swift server.
//
// If you don't call it before calling one of the connection methods
// then it will be called for you on the first access.
func (c *Connection) Authenticate() (err error) {
	c.authLock.Lock()
	defer c.authLock.Unlock()
	return c.authenticate()
}

// Internal implementation of Authenticate
//
// Call with authLock held
func (c *Connection) authenticate() (err error) {
	c.setDefaults()

	// Flush the keepalives connection - if we are
	// re-authenticating then stuff has gone wrong
	flushKeepaliveConnections(c.Transport)

	if c.Auth == nil {
		c.Auth, err = newAuth(c)
		if err != nil {
			return
		}
	}

	retries := 1
again:
	var req *http.Request
	req, err = c.Auth.Request(c)
	if err != nil {
		return
	}
	if req != nil {
		timer := time.NewTimer(c.ConnectTimeout)
		defer timer.Stop()
		var resp *http.Response
		resp, err = c.doTimeoutRequest(timer, req)
		if err != nil {
			return
		}
		defer func() {
			drainAndClose(resp.Body, &err)
			// Flush the auth connection - we don't want to keep
			// it open if keepalives were enabled
			flushKeepaliveConnections(c.Transport)
		}()
		if err = c.parseHeaders(resp, authErrorMap); err != nil {
			// Try again for a limited number of times on
			// AuthorizationFailed or BadRequest. This allows us
			// to try some alternate forms of the request
			if (err == AuthorizationFailed || err == BadRequest) && retries > 0 {
				retries--
				goto again
			}
			return
		}
		err = c.Auth.Response(resp)
		if err != nil {
			return
		}
	}
	if customAuth, isCustom := c.Auth.(CustomEndpointAuthenticator); isCustom && c.EndpointType != "" {
		c.StorageUrl = customAuth.StorageUrlForEndpoint(c.EndpointType)
	} else {
		c.StorageUrl = c.Auth.StorageUrl(c.Internal)
	}
	c.AuthToken = c.Auth.Token()
	if do, ok := c.Auth.(Expireser); ok {
		c.Expires = do.Expires()
	} else {
		c.Expires = time.Time{}
	}

	if !c.authenticated() {
		err = newError(0, "Response didn't have storage url and auth token")
		return
	}
	return
}

// Get an authToken and url
//
// The Url may be updated if it needed to authenticate using the OnReAuth function
func (c *Connection) getUrlAndAuthToken(targetUrlIn string, OnReAuth func() (string, error)) (targetUrlOut, authToken string, err error) {
	c.authLock.Lock()
	defer c.authLock.Unlock()
	targetUrlOut = targetUrlIn
	if !c.authenticated() {
		err = c.authenticate()
		if err != nil {
			return
		}
		if OnReAuth != nil {
			targetUrlOut, err = OnReAuth()
			if err != nil {
				return
			}
		}
	}
	authToken = c.AuthToken
	return
}

// flushKeepaliveConnections is called to flush pending requests after an error.
func flushKeepaliveConnections(transport http.RoundTripper) {
	if tr, ok := transport.(interface {
		CloseIdleConnections()
	}); ok {
		tr.CloseIdleConnections()
	}
}

// UnAuthenticate removes the authentication from the Connection.
func (c *Connection) UnAuthenticate() {
	c.authLock.Lock()
	c.StorageUrl = ""
	c.AuthToken = ""
	c.authLock.Unlock()
}

// Authenticated returns a boolean to show if the current connection
// is authenticated.
//
// Doesn't actually check the credentials against the server.
func (c *Connection) Authenticated() bool {
	c.authLock.Lock()
	defer c.authLock.Unlock()
	return c.authenticated()
}

// Internal version of Authenticated()
//
// Call with authLock held
func (c *Connection) authenticated() bool {
	if c.StorageUrl == "" || c.AuthToken == "" {
		return false
	}
	if c.Expires.IsZero() {
		return true
	}
	timeUntilExpiry := c.Expires.Sub(time.Now())
	return timeUntilExpiry >= 60*time.Second
}

// SwiftInfo contains the JSON object returned by Swift when the /info
// route is queried. The object contains, among others, the Swift version,
// the enabled middlewares and their configuration
type SwiftInfo map[string]interface{}

func (i SwiftInfo) SupportsBulkDelete() bool {
	_, val := i["bulk_delete"]
	return val
}

func (i SwiftInfo) SupportsSLO() bool {
	_, val := i["slo"]
	return val
}

func (i SwiftInfo) SLOMinSegmentSize() int64 {
	if slo, ok := i["slo"].(map[string]interface{}); ok {
		val, _ := slo["min_segment_size"].(float64)
		return int64(val)
	}
	return 1
}

// Discover Swift configuration by doing a request against /info
func (c *Connection) QueryInfo() (infos SwiftInfo, err error) {
	infoUrl, err := url.Parse(c.StorageUrl)
	if err != nil {
		return nil, err
	}
	infoUrl.Path = path.Join(infoUrl.Path, "..", "..", "info")
	resp, err := c.client.Get(infoUrl.String())
	if err == nil {
		if resp.StatusCode != http.StatusOK {
			drainAndClose(resp.Body, nil)
			return nil, fmt.Errorf("Invalid status code for info request: %d", resp.StatusCode)
		}
		err = readJson(resp, &infos)
		if err == nil {
			c.authLock.Lock()
			c.swiftInfo = infos
			c.authLock.Unlock()
		}
		return infos, err
	}
	return nil, err
}

func (c *Connection) cachedQueryInfo() (infos SwiftInfo, err error) {
	c.authLock.Lock()
	infos = c.swiftInfo
	c.authLock.Unlock()
	if infos == nil {
		infos, err = c.QueryInfo()
		if err != nil {
			return
		}
	}
	return infos, nil
}

// RequestOpts contains parameters for Connection.storage.
type RequestOpts struct {
	Container  string
	ObjectName string
	Operation  string
	Parameters url.Values
	Headers    Headers
	ErrorMap   errorMap
	NoResponse bool
	Body       io.Reader
	Retries    int
	// if set this is called on re-authentication to refresh the targetUrl
	OnReAuth func() (string, error)
}

// Call runs a remote command on the targetUrl, returns a
// response, headers and possible error.
//
// operation is GET, HEAD etc
// container is the name of a container
// Any other parameters (if not None) are added to the targetUrl
//
// Returns a response or an error.  If response is returned then
// the resp.Body must be read completely and
// resp.Body.Close() must be called on it, unless noResponse is set in
// which case the body will be closed in this function
//
// If "Content-Length" is set in p.Headers it will be used - this can
// be used to override the default chunked transfer encoding for
// uploads.
//
// This will Authenticate if necessary, and re-authenticate if it
// receives a 401 error which means the token has expired
//
// This method is exported so extensions can call it.
func (c *Connection) Call(targetUrl string, p RequestOpts) (resp *http.Response, headers Headers, err error) {
	c.authLock.Lock()
	c.setDefaults()
	c.authLock.Unlock()
	retries := p.Retries
	if retries == 0 {
		retries = c.Retries
	}
	var req *http.Request
	for {
		var authToken string
		if targetUrl, authToken, err = c.getUrlAndAuthToken(targetUrl, p.OnReAuth); err != nil {
			return //authentication failure
		}
		var URL *url.URL
		URL, err = url.Parse(targetUrl)
		if err != nil {
			return
		}
		if p.Container != "" {
			URL.Path += "/" + p.Container
			if p.ObjectName != "" {
				URL.Path += "/" + p.ObjectName
			}
		}
		if p.Parameters != nil {
			URL.RawQuery = p.Parameters.Encode()
		}
		timer := time.NewTimer(c.ConnectTimeout)
		defer timer.Stop()
		reader := p.Body
		if reader != nil {
			reader = newWatchdogReader(reader, c.Timeout, timer)
		}
		req, err = http.NewRequest(p.Operation, URL.String(), reader)
		if err != nil {
			return
		}
		if p.Headers != nil {
			for k, v := range p.Headers {
				// Set ContentLength in req if the user passed it in in the headers
				if k == "Content-Length" {
					req.ContentLength, err = strconv.ParseInt(v, 10, 64)
					if err != nil {
						err = fmt.Errorf("Invalid %q header %q: %v", k, v, err)
						return
					}
				} else {
					req.Header.Add(k, v)
				}
			}
		}
		req.Header.Add("User-Agent", c.UserAgent)
		req.Header.Add("X-Auth-Token", authToken)

		_, hasCL := p.Headers["Content-Length"]
		AddExpectAndTransferEncoding(req, hasCL)

		resp, err = c.doTimeoutRequest(timer, req)
		if err != nil {
			if (p.Operation == "HEAD" || p.Operation == "GET") && retries > 0 {
				retries--
				continue
			}
			return
		}
		// Check to see if token has expired
		if resp.StatusCode == 401 && retries > 0 {
			drainAndClose(resp.Body, nil)
			c.UnAuthenticate()
			retries--
		} else {
			break
		}
	}

	headers = readHeaders(resp)
	if err = c.parseHeaders(resp, p.ErrorMap); err != nil {
		return
	}
	if p.NoResponse {
		drainAndClose(resp.Body, &err)
		if err != nil {
			return
		}
	} else {
		// Cancel the request on timeout
		cancel := func() {
			cancelRequest(c.Transport, req)
		}
		// Wrap resp.Body to make it obey an idle timeout
		resp.Body = newTimeoutReader(resp.Body, c.Timeout, cancel)
	}
	return
}

// storage runs a remote command on a the storage url, returns a
// response, headers and possible error.
//
// operation is GET, HEAD etc
// container is the name of a container
// Any other parameters (if not None) are added to the storage url
//
// Returns a response or an error.  If response is returned then
// resp.Body.Close() must be called on it, unless noResponse is set in
// which case the body will be closed in this function
//
// This will Authenticate if necessary, and re-authenticate if it
// receives a 401 error which means the token has expired
func (c *Connection) storage(p RequestOpts) (resp *http.Response, headers Headers, err error) {
	p.OnReAuth = func() (string, error) {
		return c.StorageUrl, nil
	}
	c.authLock.Lock()
	url := c.StorageUrl
	c.authLock.Unlock()
	return c.Call(url, p)
}

// readLines reads the response into an array of strings.
//
// Closes the response when done
func readLines(resp *http.Response) (lines []string, err error) {
	defer drainAndClose(resp.Body, &err)
	reader := bufio.NewReader(resp.Body)
	buffer := bytes.NewBuffer(make([]byte, 0, 128))
	var part []byte
	var prefix bool
	for {
		if part, prefix, err = reader.ReadLine(); err != nil {
			break
		}
		buffer.Write(part)
		if !prefix {
			lines = append(lines, buffer.String())
			buffer.Reset()
		}
	}
	if err == io.EOF {
		err = nil
	}
	return
}

// readJson reads the response into the json type passed in
//
// Closes the response when done
func readJson(resp *http.Response, result interface{}) (err error) {
	defer drainAndClose(resp.Body, &err)
	decoder := json.NewDecoder(resp.Body)
	return decoder.Decode(result)
}

/* ------------------------------------------------------------ */

// ContainersOpts is options for Containers() and ContainerNames()
type ContainersOpts struct {
	Limit     int     // For an integer value n, limits the number of results to at most n values.
	Prefix    string  // Given a string value x, return container names matching the specified prefix.
	Marker    string  // Given a string value x, return container names greater in value than the specified marker.
	EndMarker string  // Given a string value x, return container names less in value than the specified marker.
	Headers   Headers // Any additional HTTP headers - can be nil
}

// parse the ContainerOpts
func (opts *ContainersOpts) parse() (url.Values, Headers) {
	v := url.Values{}
	var h Headers
	if opts != nil {
		if opts.Limit > 0 {
			v.Set("limit", strconv.Itoa(opts.Limit))
		}
		if opts.Prefix != "" {
			v.Set("prefix", opts.Prefix)
		}
		if opts.Marker != "" {
			v.Set("marker", opts.Marker)
		}
		if opts.EndMarker != "" {
			v.Set("end_marker", opts.EndMarker)
		}
		h = opts.Headers
	}
	return v, h
}

// ContainerNames returns a slice of names of containers in this account.
func (c *Connection) ContainerNames(opts *ContainersOpts) ([]string, error) {
	v, h := opts.parse()
	resp, _, err := c.storage(RequestOpts{
		Operation:  "GET",
		Parameters: v,
		ErrorMap:   ContainerErrorMap,
		Headers:    h,
	})
	if err != nil {
		return nil, err
	}
	lines, err := readLines(resp)
	return lines, err
}

// Container contains information about a container
type Container struct {
	Name  string // Name of the container
	Count int64  // Number of objects in the container
	Bytes int64  // Total number of bytes used in the container
}

// Containers returns a slice of structures with full information as
// described in Container.
func (c *Connection) Containers(opts *ContainersOpts) ([]Container, error) {
	v, h := opts.parse()
	v.Set("format", "json")
	resp, _, err := c.storage(RequestOpts{
		Operation:  "GET",
		Parameters: v,
		ErrorMap:   ContainerErrorMap,
		Headers:    h,
	})
	if err != nil {
		return nil, err
	}
	var containers []Container
	err = readJson(resp, &containers)
	return containers, err
}

// containersAllOpts makes a copy of opts if set or makes a new one and
// overrides Limit and Marker
func containersAllOpts(opts *ContainersOpts) *ContainersOpts {
	var newOpts ContainersOpts
	if opts != nil {
		newOpts = *opts
	}
	if newOpts.Limit == 0 {
		newOpts.Limit = allContainersLimit
	}
	newOpts.Marker = ""
	return &newOpts
}

// ContainersAll is like Containers but it returns all the Containers
//
// It calls Containers multiple times using the Marker parameter
//
// It has a default Limit parameter but you may pass in your own
func (c *Connection) ContainersAll(opts *ContainersOpts) ([]Container, error) {
	opts = containersAllOpts(opts)
	containers := make([]Container, 0)
	for {
		newContainers, err := c.Containers(opts)
		if err != nil {
			return nil, err
		}
		containers = append(containers, newContainers...)
		if len(newContainers) < opts.Limit {
			break
		}
		opts.Marker = newContainers[len(newContainers)-1].Name
	}
	return containers, nil
}

// ContainerNamesAll is like ContainerNamess but it returns all the Containers
//
// It calls ContainerNames multiple times using the Marker parameter
//
// It has a default Limit parameter but you may pass in your own
func (c *Connection) ContainerNamesAll(opts *ContainersOpts) ([]string, error) {
	opts = containersAllOpts(opts)
	containers := make([]string, 0)
	for {
		newContainers, err := c.ContainerNames(opts)
		if err != nil {
			return nil, err
		}
		containers = append(containers, newContainers...)
		if len(newContainers) < opts.Limit {
			break
		}
		opts.Marker = newContainers[len(newContainers)-1]
	}
	return containers, nil
}

/* ------------------------------------------------------------ */

// ObjectOpts is options for Objects() and ObjectNames()
type ObjectsOpts struct {
	Limit      int     // For an integer value n, limits the number of results to at most n values.
	Marker     string  // Given a string value x, return object names greater in value than the  specified marker.
	EndMarker  string  // Given a string value x, return object names less in value than the specified marker
	Prefix     string  // For a string value x, causes the results to be limited to object names beginning with the substring x.
	Path       string  // For a string value x, return the object names nested in the pseudo path
	Delimiter  rune    // For a character c, return all the object names nested in the container
	Headers    Headers // Any additional HTTP headers - can be nil
	KeepMarker bool    // Do not reset Marker when using ObjectsAll or ObjectNamesAll
}

// parse reads values out of ObjectsOpts
func (opts *ObjectsOpts) parse() (url.Values, Headers) {
	v := url.Values{}
	var h Headers
	if opts != nil {
		if opts.Limit > 0 {
			v.Set("limit", strconv.Itoa(opts.Limit))
		}
		if opts.Marker != "" {
			v.Set("marker", opts.Marker)
		}
		if opts.EndMarker != "" {
			v.Set("end_marker", opts.EndMarker)
		}
		if opts.Prefix != "" {
			v.Set("prefix", opts.Prefix)
		}
		if opts.Path != "" {
			v.Set("path", opts.Path)
		}
		if opts.Delimiter != 0 {
			v.Set("delimiter", string(opts.Delimiter))
		}
		h = opts.Headers
	}
	return v, h
}

// ObjectNames returns a slice of names of objects in a given container.
func (c *Connection) ObjectNames(container string, opts *ObjectsOpts) ([]string, error) {
	v, h := opts.parse()
	resp, _, err := c.storage(RequestOpts{
		Container:  container,
		Operation:  "GET",
		Parameters: v,
		ErrorMap:   ContainerErrorMap,
		Headers:    h,
	})
	if err != nil {
		return nil, err
	}
	return readLines(resp)
}

// Object contains information about an object
type Object struct {
	Name               string     `json:"name"`          // object name
	ContentType        string     `json:"content_type"`  // eg application/directory
	Bytes              int64      `json:"bytes"`         // size in bytes
	ServerLastModified string     `json:"last_modified"` // Last modified time, eg '2011-06-30T08:20:47.736680' as a string supplied by the server
	LastModified       time.Time  // Last modified time converted to a time.Time
	Hash               string     `json:"hash"`     // MD5 hash, eg "d41d8cd98f00b204e9800998ecf8427e"
	SLOHash            string     `json:"slo_etag"` // MD5 hash of all segments' MD5 hash, eg "d41d8cd98f00b204e9800998ecf8427e"
	PseudoDirectory    bool       // Set when using delimiter to show that this directory object does not really exist
	SubDir             string     `json:"subdir"` // returned only when using delimiter to mark "pseudo directories"
	ObjectType         ObjectType // type of this object
}

// Objects returns a slice of Object with information about each
// object in the container.
//
// If Delimiter is set in the opts then PseudoDirectory may be set,
// with ContentType 'application/directory'.  These are not real
// objects but represent directories of objects which haven't had an
// object created for them.
func (c *Connection) Objects(container string, opts *ObjectsOpts) ([]Object, error) {
	v, h := opts.parse()
	v.Set("format", "json")
	resp, _, err := c.storage(RequestOpts{
		Container:  container,
		Operation:  "GET",
		Parameters: v,
		ErrorMap:   ContainerErrorMap,
		Headers:    h,
	})
	if err != nil {
		return nil, err
	}
	var objects []Object
	err = readJson(resp, &objects)
	// Convert Pseudo directories and dates
	for i := range objects {
		object := &objects[i]
		if object.SubDir != "" {
			object.Name = object.SubDir
			object.PseudoDirectory = true
			object.ContentType = "application/directory"
		}
		if object.ServerLastModified != "" {
			// 2012-11-11T14:49:47.887250
			//
			// Remove fractional seconds if present. This
			// then keeps it consistent with Object
			// which can only return timestamps accurate
			// to 1 second
			//
			// The TimeFormat will parse fractional
			// seconds if desired though
			datetime := strings.SplitN(object.ServerLastModified, ".", 2)[0]
			object.LastModified, err = time.Parse(TimeFormat, datetime)
			if err != nil {
				return nil, err
			}
		}
		if object.SLOHash != "" {
			object.ObjectType = StaticLargeObjectType
		}
	}
	return objects, err
}

// objectsAllOpts makes a copy of opts if set or makes a new one and
// overrides Limit and Marker
// Marker is not overriden if KeepMarker is set
func objectsAllOpts(opts *ObjectsOpts, Limit int) *ObjectsOpts {
	var newOpts ObjectsOpts
	if opts != nil {
		newOpts = *opts
	}
	if newOpts.Limit == 0 {
		newOpts.Limit = Limit
	}
	if !newOpts.KeepMarker {
		newOpts.Marker = ""
	}
	return &newOpts
}

// A closure defined by the caller to iterate through all objects
//
// Call Objects or ObjectNames from here with the *ObjectOpts passed in
//
// Do whatever is required with the results then return them
type ObjectsWalkFn func(*ObjectsOpts) (interface{}, error)

// ObjectsWalk is uses to iterate through all the objects in chunks as
// returned by Objects or ObjectNames using the Marker and Limit
// parameters in the ObjectsOpts.
//
// Pass in a closure `walkFn` which calls Objects or ObjectNames with
// the *ObjectsOpts passed to it and does something with the results.
//
// Errors will be returned from this function
//
// It has a default Limit parameter but you may pass in your own
func (c *Connection) ObjectsWalk(container string, opts *ObjectsOpts, walkFn ObjectsWalkFn) error {
	opts = objectsAllOpts(opts, allObjectsChanLimit)
	for {
		objects, err := walkFn(opts)
		if err != nil {
			return err
		}
		var n int
		var last string
		switch objects := objects.(type) {
		case []string:
			n = len(objects)
			if n > 0 {
				last = objects[len(objects)-1]
			}
		case []Object:
			n = len(objects)
			if n > 0 {
				last = objects[len(objects)-1].Name
			}
		default:
			panic("Unknown type returned to ObjectsWalk")
		}
		if n < opts.Limit {
			break
		}
		opts.Marker = last
	}
	return nil
}

// ObjectsAll is like Objects but it returns an unlimited number of Objects in a slice
//
// It calls Objects multiple times using the Marker parameter
func (c *Connection) ObjectsAll(container string, opts *ObjectsOpts) ([]Object, error) {
	objects := make([]Object, 0)
	err := c.ObjectsWalk(container, opts, func(opts *ObjectsOpts) (interface{}, error) {
		newObjects, err := c.Objects(container, opts)
		if err == nil {
			objects = append(objects, newObjects...)
		}
		return newObjects, err
	})
	return objects, err
}

// ObjectNamesAll is like ObjectNames but it returns all the Objects
//
// It calls ObjectNames multiple times using the Marker parameter. Marker is
// reset unless KeepMarker is set
//
// It has a default Limit parameter but you may pass in your own
func (c *Connection) ObjectNamesAll(container string, opts *ObjectsOpts) ([]string, error) {
	objects := make([]string, 0)
	err := c.ObjectsWalk(container, opts, func(opts *ObjectsOpts) (interface{}, error) {
		newObjects, err := c.ObjectNames(container, opts)
		if err == nil {
			objects = append(objects, newObjects...)
		}
		return newObjects, err
	})
	return objects, err
}

// Account contains information about this account.
type Account struct {
	BytesUsed  int64 // total number of bytes used
	Containers int64 // total number of containers
	Objects    int64 // total number of objects
}

// getInt64FromHeader is a helper function to decode int64 from header.
func getInt64FromHeader(resp *http.Response, header string) (result int64, err error) {
	value := resp.Header.Get(header)
	result, err = strconv.ParseInt(value, 10, 64)
	if err != nil {
		err = newErrorf(0, "Bad Header '%s': '%s': %s", header, value, err)
	}
	return
}

// Account returns info about the account in an Account struct.
func (c *Connection) Account() (info Account, headers Headers, err error) {
	var resp *http.Response
	resp, headers, err = c.storage(RequestOpts{
		Operation:  "HEAD",
		ErrorMap:   ContainerErrorMap,
		NoResponse: true,
	})
	if err != nil {
		return
	}
	// Parse the headers into a dict
	//
	//    {'Accept-Ranges': 'bytes',
	//     'Content-Length': '0',
	//     'Date': 'Tue, 05 Jul 2011 16:37:06 GMT',
	//     'X-Account-Bytes-Used': '316598182',
	//     'X-Account-Container-Count': '4',
	//     'X-Account-Object-Count': '1433'}
	if info.BytesUsed, err = getInt64FromHeader(resp, "X-Account-Bytes-Used"); err != nil {
		return
	}
	if info.Containers, err = getInt64FromHeader(resp, "X-Account-Container-Count"); err != nil {
		return
	}
	if info.Objects, err = getInt64FromHeader(resp, "X-Account-Object-Count"); err != nil {
		return
	}
	return
}

// AccountUpdate adds, replaces or remove account metadata.
//
// Add or update keys by mentioning them in the Headers.
//
// Remove keys by setting them to an empty string.
func (c *Connection) AccountUpdate(h Headers) error {
	_, _, err := c.storage(RequestOpts{
		Operation:  "POST",
		ErrorMap:   ContainerErrorMap,
		NoResponse: true,
		Headers:    h,
	})
	return err
}

// ContainerCreate creates a container.
//
// If you don't want to add Headers just pass in nil
//
// No error is returned if it already exists but the metadata if any will be updated.
func (c *Connection) ContainerCreate(container string, h Headers) error {
	_, _, err := c.storage(RequestOpts{
		Container:  container,
		Operation:  "PUT",
		ErrorMap:   ContainerErrorMap,
		NoResponse: true,
		Headers:    h,
	})
	return err
}

// ContainerDelete deletes a container.
//
// May return ContainerDoesNotExist or ContainerNotEmpty
func (c *Connection) ContainerDelete(container string) error {
	_, _, err := c.storage(RequestOpts{
		Container:  container,
		Operation:  "DELETE",
		ErrorMap:   ContainerErrorMap,
		NoResponse: true,
	})
	return err
}

// Container returns info about a single container including any
// metadata in the headers.
func (c *Connection) Container(container string) (info Container, headers Headers, err error) {
	var resp *http.Response
	resp, headers, err = c.storage(RequestOpts{
		Container:  container,
		Operation:  "HEAD",
		ErrorMap:   ContainerErrorMap,
		NoResponse: true,
	})
	if err != nil {
		return
	}
	// Parse the headers into the struct
	info.Name = container
	if info.Bytes, err = getInt64FromHeader(resp, "X-Container-Bytes-Used"); err != nil {
		return
	}
	if info.Count, err = getInt64FromHeader(resp, "X-Container-Object-Count"); err != nil {
		return
	}
	return
}

// ContainerUpdate adds, replaces or removes container metadata.
//
// Add or update keys by mentioning them in the Metadata.
//
// Remove keys by setting them to an empty string.
//
// Container metadata can only be read with Container() not with Containers().
func (c *Connection) ContainerUpdate(container string, h Headers) error {
	_, _, err := c.storage(RequestOpts{
		Container:  container,
		Operation:  "POST",
		ErrorMap:   ContainerErrorMap,
		NoResponse: true,
		Headers:    h,
	})
	return err
}

// ------------------------------------------------------------

// ObjectCreateFile represents a swift object open for writing
type ObjectCreateFile struct {
	checkHash  bool           // whether we are checking the hash
	pipeReader *io.PipeReader // pipe for the caller to use
	pipeWriter *io.PipeWriter
	hash       hash.Hash      // hash being build up as we go along
	done       chan struct{}  // signals when the upload has finished
	resp       *http.Response // valid when done has signalled
	err        error          // ditto
	headers    Headers        // ditto
}

// Write bytes to the object - see io.Writer
func (file *ObjectCreateFile) Write(p []byte) (n int, err error) {
	n, err = file.pipeWriter.Write(p)
	if err == io.ErrClosedPipe {
		if file.err != nil {
			return 0, file.err
		}
		return 0, newError(500, "Write on closed file")
	}
	if err == nil && file.checkHash {
		_, _ = file.hash.Write(p)
	}
	return
}

// Close the object and checks the md5sum if it was required.
//
// Also returns any other errors from the server (eg container not
// found) so it is very important to check the errors on this method.
func (file *ObjectCreateFile) Close() error {
	// Close the body
	err := file.pipeWriter.Close()
	if err != nil {
		return err
	}

	// Wait for the HTTP operation to complete
	<-file.done

	// Check errors
	if file.err != nil {
		return file.err
	}
	if file.checkHash {
		receivedMd5 := strings.ToLower(file.headers["Etag"])
		calculatedMd5 := fmt.Sprintf("%x", file.hash.Sum(nil))
		if receivedMd5 != calculatedMd5 {
			return ObjectCorrupted
		}
	}
	return nil
}

// Headers returns the response headers from the created object if the upload
// has been completed. The Close() method must be called on an ObjectCreateFile
// before this method.
func (file *ObjectCreateFile) Headers() (Headers, error) {
	// error out if upload is not complete.
	select {
	case <-file.done:
	default:
		return nil, fmt.Errorf("Cannot get metadata, object upload failed or has not yet completed.")
	}
	return file.headers, nil
}

// Check it satisfies the interface
var _ io.WriteCloser = &ObjectCreateFile{}

// objectPutHeaders create a set of headers for a PUT
//
// It guesses the contentType from the objectName if it isn't set
//
// checkHash may be changed
func objectPutHeaders(objectName string, checkHash *bool, Hash string, contentType string, h Headers) Headers {
	if contentType == "" {
		contentType = mime.TypeByExtension(path.Ext(objectName))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	}
	// Meta stuff
	extraHeaders := map[string]string{
		"Content-Type": contentType,
	}
	for key, value := range h {
		extraHeaders[key] = value
	}
	if Hash != "" {
		extraHeaders["Etag"] = Hash
		*checkHash = false // the server will do it
	}
	return extraHeaders
}

// ObjectCreate creates or updates the object in the container.  It
// returns an io.WriteCloser you should write the contents to.  You
// MUST call Close() on it and you MUST check the error return from
// Close().
//
// If checkHash is True then it will calculate the MD5 Hash of the
// file as it is being uploaded and check it against that returned
// from the server.  If it is wrong then it will return
// ObjectCorrupted on Close()
//
// If you know the MD5 hash of the object ahead of time then set the
// Hash parameter and it will be sent to the server (as an Etag
// header) and the server will check the MD5 itself after the upload,
// and this will return ObjectCorrupted on Close() if it is incorrect.
//
// If you don't want any error protection (not recommended) then set
// checkHash to false and Hash to "".
//
// If contentType is set it will be used, otherwise one will be
// guessed from objectName using mime.TypeByExtension
func (c *Connection) ObjectCreate(container string, objectName string, checkHash bool, Hash string, contentType string, h Headers) (file *ObjectCreateFile, err error) {
	extraHeaders := objectPutHeaders(objectName, &checkHash, Hash, contentType, h)
	pipeReader, pipeWriter := io.Pipe()
	file = &ObjectCreateFile{
		hash:       md5.New(),
		checkHash:  checkHash,
		pipeReader: pipeReader,
		pipeWriter: pipeWriter,
		done:       make(chan struct{}),
	}
	// Run the PUT in the background piping it data
	go func() {
		opts := RequestOpts{
			Container:  container,
			ObjectName: objectName,
			Operation:  "PUT",
			Headers:    extraHeaders,
			Body:       pipeReader,
			NoResponse: true,
			ErrorMap:   objectErrorMap,
		}
		file.resp, file.headers, file.err = c.storage(opts)
		// Signal finished
		pipeReader.Close()
		close(file.done)
	}()
	return
}

func (c *Connection) objectPut(container string, objectName string, contents io.Reader, checkHash bool, Hash string, contentType string, h Headers, parameters url.Values) (headers Headers, err error) {
	extraHeaders := objectPutHeaders(objectName, &checkHash, Hash, contentType, h)
	hash := md5.New()
	var body io.Reader = contents
	if checkHash {
		body = io.TeeReader(contents, hash)
	}
	_, headers, err = c.storage(RequestOpts{
		Container:  container,
		ObjectName: objectName,
		Operation:  "PUT",
		Headers:    extraHeaders,
		Body:       body,
		NoResponse: true,
		ErrorMap:   objectErrorMap,
		Parameters: parameters,
	})
	if err != nil {
		return
	}
	if checkHash {
		receivedMd5 := strings.ToLower(headers["Etag"])
		calculatedMd5 := fmt.Sprintf("%x", hash.Sum(nil))
		if receivedMd5 != calculatedMd5 {
			err = ObjectCorrupted
			return
		}
	}
	return
}

// ObjectPut creates or updates the path in the container from
// contents.  contents should be an open io.Reader which will have all
// its contents read.
//
// This is a low level interface.
//
// If checkHash is True then it will calculate the MD5 Hash of the
// file as it is being uploaded and check it against that returned
// from the server.  If it is wrong then it will return
// ObjectCorrupted.
//
// If you know the MD5 hash of the object ahead of time then set the
// Hash parameter and it will be sent to the server (as an Etag
// header) and the server will check the MD5 itself after the upload,
// and this will return ObjectCorrupted if it is incorrect.
//
// If you don't want any error protection (not recommended) then set
// checkHash to false and Hash to "".
//
// If contentType is set it will be used, otherwise one will be
// guessed from objectName using mime.TypeByExtension
func (c *Connection) ObjectPut(container string, objectName string, contents io.Reader, checkHash bool, Hash string, contentType string, h Headers) (headers Headers, err error) {
	return c.objectPut(container, objectName, contents, checkHash, Hash, contentType, h, nil)
}

// ObjectPutBytes creates an object from a []byte in a container.
//
// This is a simplified interface which checks the MD5.
func (c *Connection) ObjectPutBytes(container string, objectName string, contents []byte, contentType string) (err error) {
	buf := bytes.NewBuffer(contents)
	h := Headers{"Content-Length": strconv.Itoa(len(contents))}
	_, err = c.ObjectPut(container, objectName, buf, true, "", contentType, h)
	return
}

// ObjectPutString creates an object from a string in a container.
//
// This is a simplified interface which checks the MD5
func (c *Connection) ObjectPutString(container string, objectName string, contents string, contentType string) (err error) {
	buf := strings.NewReader(contents)
	h := Headers{"Content-Length": strconv.Itoa(len(contents))}
	_, err = c.ObjectPut(container, objectName, buf, true, "", contentType, h)
	return
}

// ObjectOpenFile represents a swift object open for reading
type ObjectOpenFile struct {
	connection *Connection    // stored copy of Connection used in Open
	container  string         // stored copy of container used in Open
	objectName string         // stored copy of objectName used in Open
	headers    Headers        // stored copy of headers used in Open
	resp       *http.Response // http connection
	body       io.Reader      // read data from this
	checkHash  bool           // true if checking MD5
	hash       hash.Hash      // currently accumulating MD5
	bytes      int64          // number of bytes read on this connection
	eof        bool           // whether we have read end of file
	pos        int64          // current position when reading
	lengthOk   bool           // whether length is valid
	length     int64          // length of the object if read
	seeked     bool           // whether we have seeked this file or not
	overSeeked bool           // set if we have seeked to the end or beyond
}

// Read bytes from the object - see io.Reader
func (file *ObjectOpenFile) Read(p []byte) (n int, err error) {
	if file.overSeeked {
		return 0, io.EOF
	}
	n, err = file.body.Read(p)
	file.bytes += int64(n)
	file.pos += int64(n)
	if err == io.EOF {
		file.eof = true
	}
	return
}

// Seek sets the offset for the next Read to offset, interpreted
// according to whence: 0 means relative to the origin of the file, 1
// means relative to the current offset, and 2 means relative to the
// end. Seek returns the new offset and an Error, if any.
//
// Seek uses HTTP Range headers which, if the file pointer is moved,
// will involve reopening the HTTP connection.
//
// Note that you can't seek to the end of a file or beyond; HTTP Range
// requests don't support the file pointer being outside the data,
// unlike os.File
//
// Seek(0, 1) will return the current file pointer.
func (file *ObjectOpenFile) Seek(offset int64, whence int) (newPos int64, err error) {
	file.overSeeked = false
	switch whence {
	case 0: // relative to start
		newPos = offset
	case 1: // relative to current
		newPos = file.pos + offset
	case 2: // relative to end
		if !file.lengthOk {
			return file.pos, newError(0, "Length of file unknown so can't seek from end")
		}
		newPos = file.length + offset
		if offset >= 0 {
			file.overSeeked = true
			return
		}
	default:
		panic("Unknown whence in ObjectOpenFile.Seek")
	}
	// If at correct position (quite likely), do nothing
	if newPos == file.pos {
		return
	}
	// Close the file...
	file.seeked = true
	err = file.Close()
	if err != nil {
		return
	}
	// ...and re-open with a Range header
	if file.headers == nil {
		file.headers = Headers{}
	}
	if newPos > 0 {
		file.headers["Range"] = fmt.Sprintf("bytes=%d-", newPos)
	} else {
		delete(file.headers, "Range")
	}
	newFile, _, err := file.connection.ObjectOpen(file.container, file.objectName, false, file.headers)
	if err != nil {
		return
	}
	// Update the file
	file.resp = newFile.resp
	file.body = newFile.body
	file.checkHash = false
	file.pos = newPos
	return
}

// Length gets the objects content length either from a cached copy or
// from the server.
func (file *ObjectOpenFile) Length() (int64, error) {
	if !file.lengthOk {
		info, _, err := file.connection.Object(file.container, file.objectName)
		file.length = info.Bytes
		file.lengthOk = (err == nil)
		return file.length, err
	}
	return file.length, nil
}

// Close the object and checks the length and md5sum if it was
// required and all the object was read
func (file *ObjectOpenFile) Close() (err error) {
	// Close the body at the end
	defer checkClose(file.resp.Body, &err)

	// If not end of file or seeked then can't check anything
	if !file.eof || file.seeked {
		return
	}

	// Check the MD5 sum if requested
	if file.checkHash {
		receivedMd5 := strings.ToLower(file.resp.Header.Get("Etag"))
		calculatedMd5 := fmt.Sprintf("%x", file.hash.Sum(nil))
		if receivedMd5 != calculatedMd5 {
			err = ObjectCorrupted
			return
		}
	}

	// Check to see we read the correct number of bytes
	if file.lengthOk && file.length != file.bytes {
		err = ObjectCorrupted
		return
	}
	return
}

// Check it satisfies the interfaces
var _ io.ReadCloser = &ObjectOpenFile{}
var _ io.Seeker = &ObjectOpenFile{}

func (c *Connection) objectOpenBase(container string, objectName string, checkHash bool, h Headers, parameters url.Values) (file *ObjectOpenFile, headers Headers, err error) {
	var resp *http.Response
	opts := RequestOpts{
		Container:  container,
		ObjectName: objectName,
		Operation:  "GET",
		ErrorMap:   objectErrorMap,
		Headers:    h,
		Parameters: parameters,
	}
	resp, headers, err = c.storage(opts)
	if err != nil {
		return
	}
	// Can't check MD5 on an object with X-Object-Manifest or X-Static-Large-Object set
	if checkHash && headers.IsLargeObject() {
		// log.Printf("swift: turning off md5 checking on object with manifest %v", objectName)
		checkHash = false
	}
	file = &ObjectOpenFile{
		connection: c,
		container:  container,
		objectName: objectName,
		headers:    h,
		resp:       resp,
		checkHash:  checkHash,
		body:       resp.Body,
	}
	if checkHash {
		file.hash = md5.New()
		file.body = io.TeeReader(resp.Body, file.hash)
	}
	// Read Content-Length
	if resp.Header.Get("Content-Length") != "" {
		file.length, err = getInt64FromHeader(resp, "Content-Length")
		file.lengthOk = (err == nil)
	}
	return
}

func (c *Connection) objectOpen(container string, objectName string, checkHash bool, h Headers, parameters url.Values) (file *ObjectOpenFile, headers Headers, err error) {
	err = withLORetry(0, func() (Headers, int64, error) {
		file, headers, err = c.objectOpenBase(container, objectName, checkHash, h, parameters)
		if err != nil {
			return headers, 0, err
		}
		return headers, file.length, nil
	})
	return
}

// ObjectOpen returns an ObjectOpenFile for reading the contents of
// the object.  This satisfies the io.ReadCloser and the io.Seeker
// interfaces.
//
// You must call Close() on contents when finished
//
// Returns the headers of the response.
//
// If checkHash is true then it will calculate the md5sum of the file
// as it is being received and check it against that returned from the
// server.  If it is wrong then it will return ObjectCorrupted. It
// will also check the length returned. No checking will be done if
// you don't read all the contents.
//
// Note that objects with X-Object-Manifest or X-Static-Large-Object
// set won't ever have their md5sum's checked as the md5sum reported
// on the object is actually the md5sum of the md5sums of the
// parts. This isn't very helpful to detect a corrupted download as
// the size of the parts aren't known without doing more operations.
// If you want to ensure integrity of an object with a manifest then
// you will need to download everything in the manifest separately.
//
// headers["Content-Type"] will give the content type if desired.
func (c *Connection) ObjectOpen(container string, objectName string, checkHash bool, h Headers) (file *ObjectOpenFile, headers Headers, err error) {
	return c.objectOpen(container, objectName, checkHash, h, nil)
}

// ObjectGet gets the object into the io.Writer contents.
//
// Returns the headers of the response.
//
// If checkHash is true then it will calculate the md5sum of the file
// as it is being received and check it against that returned from the
// server.  If it is wrong then it will return ObjectCorrupted.
//
// headers["Content-Type"] will give the content type if desired.
func (c *Connection) ObjectGet(container string, objectName string, contents io.Writer, checkHash bool, h Headers) (headers Headers, err error) {
	file, headers, err := c.ObjectOpen(container, objectName, checkHash, h)
	if err != nil {
		return
	}
	defer checkClose(file, &err)
	_, err = io.Copy(contents, file)
	return
}

// ObjectGetBytes returns an object as a []byte.
//
// This is a simplified interface which checks the MD5
func (c *Connection) ObjectGetBytes(container string, objectName string) (contents []byte, err error) {
	var buf bytes.Buffer
	_, err = c.ObjectGet(container, objectName, &buf, true, nil)
	contents = buf.Bytes()
	return
}

// ObjectGetString returns an object as a string.
//
// This is a simplified interface which checks the MD5
func (c *Connection) ObjectGetString(container string, objectName string) (contents string, err error) {
	var buf bytes.Buffer
	_, err = c.ObjectGet(container, objectName, &buf, true, nil)
	contents = buf.String()
	return
}

// ObjectDelete deletes the object.
//
// May return ObjectNotFound if the object isn't found
func (c *Connection) ObjectDelete(container string, objectName string) error {
	_, _, err := c.storage(RequestOpts{
		Container:  container,
		ObjectName: objectName,
		Operation:  "DELETE",
		ErrorMap:   objectErrorMap,
	})
	return err
}

// ObjectTempUrl returns a temporary URL for an object
func (c *Connection) ObjectTempUrl(container string, objectName string, secretKey string, method string, expires time.Time) string {
	mac := hmac.New(sha1.New, []byte(secretKey))
	prefix, _ := url.Parse(c.StorageUrl)
	body := fmt.Sprintf("%s\n%d\n%s/%s/%s", method, expires.Unix(), prefix.Path, container, objectName)
	mac.Write([]byte(body))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s/%s/%s?temp_url_sig=%s&temp_url_expires=%d", c.StorageUrl, container, objectName, sig, expires.Unix())
}

// parseResponseStatus parses string like "200 OK" and returns Error.
//
// For status codes beween 200 and 299, this returns nil.
func parseResponseStatus(resp string, errorMap errorMap) error {
	code := 0
	reason := resp
	t := strings.SplitN(resp, " ", 2)
	if len(t) == 2 {
		ncode, err := strconv.Atoi(t[0])
		if err == nil {
			code = ncode
			reason = t[1]
		}
	}
	if errorMap != nil {
		if err, ok := errorMap[code]; ok {
			return err
		}
	}
	if 200 <= code && code <= 299 {
		return nil
	}
	return newError(code, reason)
}

// BulkDeleteResult stores results of BulkDelete().
//
// Individual errors may (or may not) be returned by Errors.
// Errors is a map whose keys are a full path of where the object was
// to be deleted, and whose values are Error objects.  A full path of
// object looks like "/API_VERSION/USER_ACCOUNT/CONTAINER/OBJECT_PATH".
type BulkDeleteResult struct {
	NumberNotFound int64            // # of objects not found.
	NumberDeleted  int64            // # of deleted objects.
	Errors         map[string]error // Mapping between object name and an error.
	Headers        Headers          // Response HTTP headers.
}

func (c *Connection) doBulkDelete(objects []string) (result BulkDeleteResult, err error) {
	var buffer bytes.Buffer
	for _, s := range objects {
		u := url.URL{Path: s}
		buffer.WriteString(u.String() + "\n")
	}
	resp, headers, err := c.storage(RequestOpts{
		Operation:  "DELETE",
		Parameters: url.Values{"bulk-delete": []string{"1"}},
		Headers: Headers{
			"Accept":         "application/json",
			"Content-Type":   "text/plain",
			"Content-Length": strconv.Itoa(buffer.Len()),
		},
		ErrorMap: ContainerErrorMap,
		Body:     &buffer,
	})
	if err != nil {
		return
	}
	var jsonResult struct {
		NotFound int64  `json:"Number Not Found"`
		Status   string `json:"Response Status"`
		Errors   [][]string
		Deleted  int64 `json:"Number Deleted"`
	}
	err = readJson(resp, &jsonResult)
	if err != nil {
		return
	}

	err = parseResponseStatus(jsonResult.Status, objectErrorMap)
	result.NumberNotFound = jsonResult.NotFound
	result.NumberDeleted = jsonResult.Deleted
	result.Headers = headers
	el := make(map[string]error, len(jsonResult.Errors))
	for _, t := range jsonResult.Errors {
		if len(t) != 2 {
			continue
		}
		el[t[0]] = parseResponseStatus(t[1], objectErrorMap)
	}
	result.Errors = el
	return
}

// BulkDelete deletes multiple objectNames from container in one operation.
//
// Some servers may not accept bulk-delete requests since bulk-delete is
// an optional feature of swift - these will return the Forbidden error.
//
// See also:
// * http://docs.openstack.org/trunk/openstack-object-storage/admin/content/object-storage-bulk-delete.html
// * http://docs.rackspace.com/files/api/v1/cf-devguide/content/Bulk_Delete-d1e2338.html
func (c *Connection) BulkDelete(container string, objectNames []string) (result BulkDeleteResult, err error) {
	if len(objectNames) == 0 {
		result.Errors = make(map[string]error)
		return
	}
	fullPaths := make([]string, len(objectNames))
	for i, name := range objectNames {
		fullPaths[i] = fmt.Sprintf("/%s/%s", container, name)
	}
	return c.doBulkDelete(fullPaths)
}

// BulkUploadResult stores results of BulkUpload().
//
// Individual errors may (or may not) be returned by Errors.
// Errors is a map whose keys are a full path of where an object was
// to be created, and whose values are Error objects.  A full path of
// object looks like "/API_VERSION/USER_ACCOUNT/CONTAINER/OBJECT_PATH".
type BulkUploadResult struct {
	NumberCreated int64            // # of created objects.
	Errors        map[string]error // Mapping between object name and an error.
	Headers       Headers          // Response HTTP headers.
}

// BulkUpload uploads multiple files in one operation.
//
// uploadPath can be empty, a container name, or a pseudo-directory
// within a container.  If uploadPath is empty, new containers may be
// automatically created.
//
// Files are read from dataStream.  The format of the stream is specified
// by the format parameter.  Available formats are:
// * UploadTar       - Plain tar stream.
// * UploadTarGzip   - Gzip compressed tar stream.
// * UploadTarBzip2  - Bzip2 compressed tar stream.
//
// Some servers may not accept bulk-upload requests since bulk-upload is
// an optional feature of swift - these will return the Forbidden error.
//
// See also:
// * http://docs.openstack.org/trunk/openstack-object-storage/admin/content/object-storage-extract-archive.html
// * http://docs.rackspace.com/files/api/v1/cf-devguide/content/Extract_Archive-d1e2338.html
func (c *Connection) BulkUpload(uploadPath string, dataStream io.Reader, format string, h Headers) (result BulkUploadResult, err error) {
	extraHeaders := Headers{"Accept": "application/json"}
	for key, value := range h {
		extraHeaders[key] = value
	}
	// The following code abuses Container parameter intentionally.
	// The best fix might be to rename Container to UploadPath.
	resp, headers, err := c.storage(RequestOpts{
		Container:  uploadPath,
		Operation:  "PUT",
		Parameters: url.Values{"extract-archive": []string{format}},
		Headers:    extraHeaders,
		ErrorMap:   ContainerErrorMap,
		Body:       dataStream,
	})
	if err != nil {
		return
	}
	// Detect old servers which don't support this feature
	if headers["Content-Type"] != "application/json" {
		err = Forbidden
		return
	}
	var jsonResult struct {
		Created int64  `json:"Number Files Created"`
		Status  string `json:"Response Status"`
		Errors  [][]string
	}
	err = readJson(resp, &jsonResult)
	if err != nil {
		return
	}

	err = parseResponseStatus(jsonResult.Status, objectErrorMap)
	result.NumberCreated = jsonResult.Created
	result.Headers = headers
	el := make(map[string]error, len(jsonResult.Errors))
	for _, t := range jsonResult.Errors {
		if len(t) != 2 {
			continue
		}
		el[t[0]] = parseResponseStatus(t[1], objectErrorMap)
	}
	result.Errors = el
	return
}

// Object returns info about a single object including any metadata in the header.
//
// May return ObjectNotFound.
//
// Use headers.ObjectMetadata() to read the metadata in the Headers.
func (c *Connection) Object(container string, objectName string) (info Object, headers Headers, err error) {
	err = withLORetry(0, func() (Headers, int64, error) {
		info, headers, err = c.objectBase(container, objectName)
		if err != nil {
			return headers, 0, err
		}
		return headers, info.Bytes, nil
	})
	return
}

func (c *Connection) objectBase(container string, objectName string) (info Object, headers Headers, err error) {
	var resp *http.Response
	resp, headers, err = c.storage(RequestOpts{
		Container:  container,
		ObjectName: objectName,
		Operation:  "HEAD",
		ErrorMap:   objectErrorMap,
		NoResponse: true,
	})
	if err != nil {
		return
	}
	// Parse the headers into the struct
	// HTTP/1.1 200 OK
	// Date: Thu, 07 Jun 2010 20:59:39 GMT
	// Server: Apache
	// Last-Modified: Fri, 12 Jun 2010 13:40:18 GMT
	// ETag: 8a964ee2a5e88be344f36c22562a6486
	// Content-Length: 512000
	// Content-Type: text/plain; charset=UTF-8
	// X-Object-Meta-Meat: Bacon
	// X-Object-Meta-Fruit: Bacon
	// X-Object-Meta-Veggie: Bacon
	// X-Object-Meta-Dairy: Bacon
	info.Name = objectName
	info.ContentType = resp.Header.Get("Content-Type")
	if resp.Header.Get("Content-Length") != "" {
		if info.Bytes, err = getInt64FromHeader(resp, "Content-Length"); err != nil {
			return
		}
	}
	// Currently ceph doesn't return a Last-Modified header for DLO manifests without any segments
	// See ceph http://tracker.ceph.com/issues/15812
	if resp.Header.Get("Last-Modified") != "" {
		info.ServerLastModified = resp.Header.Get("Last-Modified")
		if info.LastModified, err = time.Parse(http.TimeFormat, info.ServerLastModified); err != nil {
			return
		}
	}

	info.Hash = resp.Header.Get("Etag")
	if resp.Header.Get("X-Object-Manifest") != "" {
		info.ObjectType = DynamicLargeObjectType
	} else if resp.Header.Get("X-Static-Large-Object") != "" {
		info.ObjectType = StaticLargeObjectType
	}

	return
}

// ObjectUpdate adds, replaces or removes object metadata.
//
// Add or Update keys by mentioning them in the Metadata.  Use
// Metadata.ObjectHeaders and Headers.ObjectMetadata to convert your
// Metadata to and from normal HTTP headers.
//
// This removes all metadata previously added to the object and
// replaces it with that passed in so to delete keys, just don't
// mention them the headers you pass in.
//
// Object metadata can only be read with Object() not with Objects().
//
// This can also be used to set headers not already assigned such as
// X-Delete-At or X-Delete-After for expiring objects.
//
// You cannot use this to change any of the object's other headers
// such as Content-Type, ETag, etc.
//
// Refer to copying an object when you need to update metadata or
// other headers such as Content-Type or CORS headers.
//
// May return ObjectNotFound.
func (c *Connection) ObjectUpdate(container string, objectName string, h Headers) error {
	_, _, err := c.storage(RequestOpts{
		Container:  container,
		ObjectName: objectName,
		Operation:  "POST",
		ErrorMap:   objectErrorMap,
		NoResponse: true,
		Headers:    h,
	})
	return err
}

// urlPathEscape escapes URL path the in string using URL escaping rules
//
// This mimics url.PathEscape which only available from go 1.8
func urlPathEscape(in string) string {
	var u url.URL
	u.Path = in
	return u.String()
}

// ObjectCopy does a server side copy of an object to a new position
//
// All metadata is preserved.  If metadata is set in the headers then
// it overrides the old metadata on the copied object.
//
// The destination container must exist before the copy.
//
// You can use this to copy an object to itself - this is the only way
// to update the content type of an object.
func (c *Connection) ObjectCopy(srcContainer string, srcObjectName string, dstContainer string, dstObjectName string, h Headers) (headers Headers, err error) {
	// Meta stuff
	extraHeaders := map[string]string{
		"Destination": urlPathEscape(dstContainer + "/" + dstObjectName),
	}
	for key, value := range h {
		extraHeaders[key] = value
	}
	_, headers, err = c.storage(RequestOpts{
		Container:  srcContainer,
		ObjectName: srcObjectName,
		Operation:  "COPY",
		ErrorMap:   objectErrorMap,
		NoResponse: true,
		Headers:    extraHeaders,
	})
	return
}

// ObjectMove does a server side move of an object to a new position
//
// This is a convenience method which calls ObjectCopy then ObjectDelete
//
// All metadata is preserved.
//
// The destination container must exist before the copy.
func (c *Connection) ObjectMove(srcContainer string, srcObjectName string, dstContainer string, dstObjectName string) (err error) {
	_, err = c.ObjectCopy(srcContainer, srcObjectName, dstContainer, dstObjectName, nil)
	if err != nil {
		return
	}
	return c.ObjectDelete(srcContainer, srcObjectName)
}

// ObjectUpdateContentType updates the content type of an object
//
// This is a convenience method which calls ObjectCopy
//
// All other metadata is preserved.
func (c *Connection) ObjectUpdateContentType(container string, objectName string, contentType string) (err error) {
	h := Headers{"Content-Type": contentType}
	_, err = c.ObjectCopy(container, objectName, container, objectName, h)
	return
}

// ------------------------------------------------------------

// VersionContainerCreate is a helper method for creating and enabling version controlled containers.
//
// It builds the current object container, the non-current object version container, and enables versioning.
//
// If the server doesn't support versioning then it will return
// Forbidden however it will have created both the containers at that point.
func (c *Connection) VersionContainerCreate(current, version string) error {
	if err := c.ContainerCreate(version, nil); err != nil {
		return err
	}
	if err := c.ContainerCreate(current, nil); err != nil {
		return err
	}
	if err := c.VersionEnable(current, version); err != nil {
		return err
	}
	return nil
}

// VersionEnable enables versioning on the current container with version as the tracking container.
//
// May return Forbidden if this isn't supported by the server
func (c *Connection) VersionEnable(current, version string) error {
	h := Headers{"X-Versions-Location": version}
	if err := c.ContainerUpdate(current, h); err != nil {
		return err
	}
	// Check to see if the header was set properly
	_, headers, err := c.Container(current)
	if err != nil {
		return err
	}
	// If failed to set versions header, return Forbidden as the server doesn't support this
	if headers["X-Versions-Location"] != version {
		return Forbidden
	}
	return nil
}

// VersionDisable disables versioning on the current container.
func (c *Connection) VersionDisable(current string) error {
	h := Headers{"X-Versions-Location": ""}
	if err := c.ContainerUpdate(current, h); err != nil {
		return err
	}
	return nil
}

// VersionObjectList returns a list of older versions of the object.
//
// Objects are returned in the format <length><object_name>/<timestamp>
func (c *Connection) VersionObjectList(version, object string) ([]string, error) {
	opts := &ObjectsOpts{
		// <3-character zero-padded hexadecimal character length><object name>/
		Prefix: fmt.Sprintf("%03x", len(object)) + object + "/",
	}
	return c.ObjectNames(version, opts)
}
