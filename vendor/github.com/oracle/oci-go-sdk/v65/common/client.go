// Copyright (c) 2016, 2018, 2023, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.

// Package common provides supporting functions and structs used by service packages
package common

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// DefaultHostURLTemplate The default url template for service hosts
	DefaultHostURLTemplate = "%s.%s.oraclecloud.com"

	// requestHeaderAccept The key for passing a header to indicate Accept
	requestHeaderAccept = "Accept"

	// requestHeaderAuthorization The key for passing a header to indicate Authorization
	requestHeaderAuthorization = "Authorization"

	// requestHeaderContentLength The key for passing a header to indicate Content Length
	requestHeaderContentLength = "Content-Length"

	// requestHeaderContentType The key for passing a header to indicate Content Type
	requestHeaderContentType = "Content-Type"

	// requestHeaderExpect The key for passing a header to indicate Expect/100-Continue
	requestHeaderExpect = "Expect"

	// requestHeaderDate The key for passing a header to indicate Date
	requestHeaderDate = "Date"

	// requestHeaderIfMatch The key for passing a header to indicate If Match
	requestHeaderIfMatch = "if-match"

	// requestHeaderOpcClientInfo The key for passing a header to indicate OPC Client Info
	requestHeaderOpcClientInfo = "opc-client-info"

	// requestHeaderOpcRetryToken The key for passing a header to indicate OPC Retry Token
	requestHeaderOpcRetryToken = "opc-retry-token"

	// requestHeaderOpcRequestID The key for unique Oracle-assigned identifier for the request.
	requestHeaderOpcRequestID = "opc-request-id"

	// requestHeaderOpcClientRequestID The key for unique Oracle-assigned identifier for the request.
	requestHeaderOpcClientRequestID = "opc-client-request-id"

	// requestHeaderUserAgent The key for passing a header to indicate User Agent
	requestHeaderUserAgent = "User-Agent"

	// requestHeaderXContentSHA256 The key for passing a header to indicate SHA256 hash
	requestHeaderXContentSHA256 = "X-Content-SHA256"

	// requestHeaderOpcOboToken The key for passing a header to use obo token
	requestHeaderOpcOboToken = "opc-obo-token"

	// private constants
	defaultScheme            = "https"
	defaultSDKMarker         = "Oracle-GoSDK"
	defaultUserAgentTemplate = "%s/%s (%s/%s; go/%s)" //SDK/SDKVersion (OS/OSVersion; Lang/LangVersion)
	// http.Client.Timeout includes Dial, TLSHandshake, Request, Response header and body
	defaultTimeout           = 60 * time.Second
	defaultConfigFileName    = "config"
	defaultConfigDirName     = ".oci"
	configFilePathEnvVarName = "OCI_CONFIG_FILE"

	secondaryConfigDirName = ".oraclebmc"
	maxBodyLenForDebug     = 1024 * 1000

	// appendUserAgentEnv The key for retrieving append user agent value from env var
	appendUserAgentEnv = "OCI_SDK_APPEND_USER_AGENT"

	// requestHeaderOpcClientRetries The key for passing a header to set client retries info
	requestHeaderOpcClientRetries = "opc-client-retries"

	// isDefaultRetryEnabled The key for set default retry disabled from env var
	isDefaultRetryEnabled = "OCI_SDK_DEFAULT_RETRY_ENABLED"

	// isDefaultCircuitBreakerEnabled is the key for set default circuit breaker disabled from env var
	isDefaultCircuitBreakerEnabled = "OCI_SDK_DEFAULT_CIRCUITBREAKER_ENABLED"

	//circuitBreakerNumberOfHistoryResponseEnv is the number of recorded history responses
	circuitBreakerNumberOfHistoryResponseEnv = "OCI_SDK_CIRCUITBREAKER_NUM_HISTORY_RESPONSE"

	// ociDefaultCertsPath is the env var for the path to the SSL cert file
	ociDefaultCertsPath = "OCI_DEFAULT_CERTS_PATH"

	//maxAttemptsForRefreshableRetry is the number of retry when 401 happened on a refreshable auth type
	maxAttemptsForRefreshableRetry = 3
)

// RequestInterceptor function used to customize the request before calling the underlying service
type RequestInterceptor func(*http.Request) error

// HTTPRequestDispatcher wraps the execution of a http request, it is generally implemented by
// http.Client.Do, but can be customized for testing
type HTTPRequestDispatcher interface {
	Do(req *http.Request) (*http.Response, error)
}

// CustomClientConfiguration contains configurations set at client level, currently it only includes RetryPolicy
type CustomClientConfiguration struct {
	RetryPolicy                                 *RetryPolicy
	CircuitBreaker                              *OciCircuitBreaker
	RealmSpecificServiceEndpointTemplateEnabled *bool
}

// BaseClient struct implements all basic operations to call oci web services.
type BaseClient struct {
	//HTTPClient performs the http network operations
	HTTPClient HTTPRequestDispatcher

	//Signer performs auth operation
	Signer HTTPRequestSigner

	//A request interceptor can be used to customize the request before signing and dispatching
	Interceptor RequestInterceptor

	//The host of the service
	Host string

	//The user agent
	UserAgent string

	//Base path for all operations of this client
	BasePath string

	Configuration CustomClientConfiguration
}

// SetCustomClientConfiguration sets client with retry and other custom configurations
func (client *BaseClient) SetCustomClientConfiguration(config CustomClientConfiguration) {
	client.Configuration = config
}

// RetryPolicy returns the retryPolicy configured for client
func (client *BaseClient) RetryPolicy() *RetryPolicy {
	return client.Configuration.RetryPolicy
}

// Endpoint returns the endpoint configured for client
func (client *BaseClient) Endpoint() string {
	host := client.Host
	if !strings.Contains(host, "http") &&
		!strings.Contains(host, "https") {
		host = fmt.Sprintf("%s://%s", defaultScheme, host)
	}
	return host
}

func defaultUserAgent() string {
	userAgent := fmt.Sprintf(defaultUserAgentTemplate, defaultSDKMarker, Version(), runtime.GOOS, runtime.GOARCH, runtime.Version())
	appendUA := os.Getenv(appendUserAgentEnv)
	if appendUA != "" {
		userAgent = fmt.Sprintf("%s %s", userAgent, appendUA)
	}
	return userAgent
}

var clientCounter int64

func getNextSeed() int64 {
	newCounterValue := atomic.AddInt64(&clientCounter, 1)
	return newCounterValue + time.Now().UnixNano()
}

func newBaseClient(signer HTTPRequestSigner, dispatcher HTTPRequestDispatcher) BaseClient {
	rand.Seed(getNextSeed())

	baseClient := BaseClient{
		UserAgent:   defaultUserAgent(),
		Interceptor: nil,
		Signer:      signer,
		HTTPClient:  dispatcher,
	}

	// check the default retry environment variable setting
	if IsEnvVarTrue(isDefaultRetryEnabled) {
		defaultRetry := DefaultRetryPolicy()
		baseClient.Configuration.RetryPolicy = &defaultRetry
	} else if IsEnvVarFalse(isDefaultRetryEnabled) {
		policy := NoRetryPolicy()
		baseClient.Configuration.RetryPolicy = &policy
	}
	// check if user defined global retry is configured
	if GlobalRetry != nil {
		baseClient.Configuration.RetryPolicy = GlobalRetry
	}

	return baseClient
}

func defaultHTTPDispatcher() http.Client {
	var httpClient http.Client
	var tp = http.DefaultTransport.(*http.Transport)
	if isExpectHeaderDisabled := IsEnvVarFalse(UsingExpectHeaderEnvVar); !isExpectHeaderDisabled {
		tp.Proxy = http.ProxyFromEnvironment
		tp.DialContext = (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext
		tp.ForceAttemptHTTP2 = true
		tp.MaxIdleConns = 100
		tp.IdleConnTimeout = 90 * time.Second
		tp.TLSHandshakeTimeout = 10 * time.Second
		tp.ExpectContinueTimeout = 3 * time.Second
	}
	if certFile, ok := os.LookupEnv(ociDefaultCertsPath); ok {
		pool := x509.NewCertPool()
		pemCert := readCertPem(certFile)
		cert, err := x509.ParseCertificate(pemCert)
		if err != nil {
			Logf("unable to parse content to cert fallback to pem format from env var value: %s", certFile)
			pool.AppendCertsFromPEM(pemCert)
		} else {
			Logf("using custom cert parsed from env var value: %s", certFile)
			pool.AddCert(cert)
		}
		tp.TLSClientConfig = &tls.Config{RootCAs: pool}
	}
	httpClient = http.Client{
		Timeout:   defaultTimeout,
		Transport: tp,
	}
	return httpClient
}

func defaultBaseClient(provider KeyProvider) BaseClient {
	dispatcher := defaultHTTPDispatcher()
	signer := DefaultRequestSigner(provider)
	return newBaseClient(signer, &dispatcher)
}

// DefaultBaseClientWithSigner creates a default base client with a given signer
func DefaultBaseClientWithSigner(signer HTTPRequestSigner) BaseClient {
	dispatcher := defaultHTTPDispatcher()
	return newBaseClient(signer, &dispatcher)
}

// NewClientWithConfig Create a new client with a configuration provider, the configuration provider
// will be used for the default signer as well as reading the region
// This function does not check for valid regions to implement forward compatibility
func NewClientWithConfig(configProvider ConfigurationProvider) (client BaseClient, err error) {
	var ok bool
	if ok, err = IsConfigurationProviderValid(configProvider); !ok {
		err = fmt.Errorf("can not create client, bad configuration: %s", err.Error())
		return
	}

	client = defaultBaseClient(configProvider)

	if authConfig, e := configProvider.AuthType(); e == nil && authConfig.OboToken != nil {
		Debugf("authConfig's authType is %s, and token content is %s", authConfig.AuthType, *authConfig.OboToken)
		signOboToken(&client, *authConfig.OboToken, configProvider)
	}

	return
}

// NewClientWithOboToken Create a new client that will use oboToken for auth
func NewClientWithOboToken(configProvider ConfigurationProvider, oboToken string) (client BaseClient, err error) {
	client, err = NewClientWithConfig(configProvider)
	if err != nil {
		return
	}

	signOboToken(&client, oboToken, configProvider)

	return
}

// Add obo token header to Interceptor and sign to client
func signOboToken(client *BaseClient, oboToken string, configProvider ConfigurationProvider) {
	// Interceptor to add obo token header
	client.Interceptor = func(request *http.Request) error {
		request.Header.Add(requestHeaderOpcOboToken, oboToken)
		return nil
	}
	// Obo token will also be signed
	defaultHeaders := append(DefaultGenericHeaders(), requestHeaderOpcOboToken)
	client.Signer = RequestSigner(configProvider, defaultHeaders, DefaultBodyHeaders())
}

func getHomeFolder() string {
	current, e := user.Current()
	if e != nil {
		//Give up and try to return something sensible
		home := os.Getenv("HOME")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return current.HomeDir
}

// DefaultConfigProvider returns the default config provider. The default config provider
// will look for configurations in 3 places: file in $HOME/.oci/config, HOME/.obmcs/config and
// variables names starting with the string TF_VAR. If the same configuration is found in multiple
// places the provider will prefer the first one.
// If the config file is not placed in the default location, the environment variable
// OCI_CONFIG_FILE can provide the config file location.
func DefaultConfigProvider() ConfigurationProvider {
	defaultConfigFile := getDefaultConfigFilePath()
	homeFolder := getHomeFolder()
	secondaryConfigFile := filepath.Join(homeFolder, secondaryConfigDirName, defaultConfigFileName)

	defaultFileProvider, _ := ConfigurationProviderFromFile(defaultConfigFile, "")
	secondaryFileProvider, _ := ConfigurationProviderFromFile(secondaryConfigFile, "")
	environmentProvider := environmentConfigurationProvider{EnvironmentVariablePrefix: "TF_VAR"}

	provider, _ := ComposingConfigurationProvider([]ConfigurationProvider{defaultFileProvider, secondaryFileProvider, environmentProvider})
	Debugf("Configuration provided by: %s", provider)
	return provider
}

func getDefaultConfigFilePath() string {
	homeFolder := getHomeFolder()
	defaultConfigFile := filepath.Join(homeFolder, defaultConfigDirName, defaultConfigFileName)
	if _, err := os.Stat(defaultConfigFile); err == nil {
		return defaultConfigFile
	}
	Debugf("The %s does not exist, will check env var %s for file path.", defaultConfigFile, configFilePathEnvVarName)
	// Read configuration file path from OCI_CONFIG_FILE env var
	fallbackConfigFile, existed := os.LookupEnv(configFilePathEnvVarName)
	if !existed {
		Debugf("The env var %s does not exist...", configFilePathEnvVarName)
		return defaultConfigFile
	}
	if _, err := os.Stat(fallbackConfigFile); os.IsNotExist(err) {
		Debugf("The specified cfg file path in the env var %s does not exist: %s", configFilePathEnvVarName, fallbackConfigFile)
		return defaultConfigFile
	}
	return fallbackConfigFile
}

// setRawPath sets the Path and RawPath fields of the URL based on the provided
// escaped path p. It maintains the invariant that RawPath is only specified
// when it differs from the default encoding of the path.
// For example:
// - setPath("/foo/bar")   will set Path="/foo/bar" and RawPath=""
// - setPath("/foo%2fbar") will set Path="/foo/bar" and RawPath="/foo%2fbar"
func setRawPath(u *url.URL) error {
	oldPath := u.Path
	path, err := url.PathUnescape(u.Path)
	if err != nil {
		return err
	}
	u.Path = path
	if escp := u.EscapedPath(); oldPath == escp {
		// Default encoding is fine.
		u.RawPath = ""
	} else {
		u.RawPath = oldPath
	}
	return nil
}

// CustomProfileConfigProvider returns the config provider of given profile. The custom profile config provider
// will look for configurations in 2 places: file in $HOME/.oci/config,  and variables names starting with the
// string TF_VAR. If the same configuration is found in multiple places the provider will prefer the first one.
func CustomProfileConfigProvider(customConfigPath string, profile string) ConfigurationProvider {
	homeFolder := getHomeFolder()
	if customConfigPath == "" {
		customConfigPath = filepath.Join(homeFolder, defaultConfigDirName, defaultConfigFileName)
	}
	customFileProvider, _ := ConfigurationProviderFromFileWithProfile(customConfigPath, profile, "")
	defaultFileProvider, _ := ConfigurationProviderFromFileWithProfile(customConfigPath, "DEFAULT", "")
	environmentProvider := environmentConfigurationProvider{EnvironmentVariablePrefix: "TF_VAR"}
	provider, _ := ComposingConfigurationProvider([]ConfigurationProvider{customFileProvider, defaultFileProvider, environmentProvider})
	Debugf("Configuration provided by: %s", provider)
	return provider
}

func (client *BaseClient) prepareRequest(request *http.Request) (err error) {
	if client.UserAgent == "" {
		return fmt.Errorf("user agent can not be blank")
	}

	if request.Header == nil {
		request.Header = http.Header{}
	}
	request.Header.Set(requestHeaderUserAgent, client.UserAgent)
	request.Header.Set(requestHeaderDate, time.Now().UTC().Format(http.TimeFormat))

	if !strings.Contains(client.Host, "http") &&
		!strings.Contains(client.Host, "https") {
		client.Host = fmt.Sprintf("%s://%s", defaultScheme, client.Host)
	}

	clientURL, err := url.Parse(client.Host)
	if err != nil {
		return fmt.Errorf("host is invalid. %s", err.Error())
	}
	request.URL.Host = clientURL.Host
	request.URL.Scheme = clientURL.Scheme
	currentPath := request.URL.Path
	if !strings.Contains(currentPath, fmt.Sprintf("/%s", client.BasePath)) {
		request.URL.Path = path.Clean(fmt.Sprintf("/%s/%s", client.BasePath, currentPath))
		err := setRawPath(request.URL)
		if err != nil {
			return err
		}
	}
	return
}

func (client BaseClient) intercept(request *http.Request) (err error) {
	if client.Interceptor != nil {
		err = client.Interceptor(request)
	}
	return
}

// checkForSuccessfulResponse checks if the response is successful
// If Error Code is 4XX/5XX and debug level is set to info, will log the request and response
func checkForSuccessfulResponse(res *http.Response, requestBody *io.ReadCloser) error {
	familyStatusCode := res.StatusCode / 100
	if familyStatusCode == 4 || familyStatusCode == 5 {
		IfInfo(func() {
			// If debug level is set to verbose, the request and request body will be dumped and logged under debug level, this is to avoid duplicate logging
			if defaultLogger.LogLevel() < verboseLogging {
				logRequest(res.Request, Logf, noLogging)
				if requestBody != nil && *requestBody != http.NoBody {
					bodyContent, _ := ioutil.ReadAll(*requestBody)
					Logf("Dump Request Body: \n%s", string(bodyContent))
				}
			}
			logResponse(res, Logf, infoLogging)
		})
		return newServiceFailureFromResponse(res)
	}
	IfDebug(func() {
		logResponse(res, Debugf, verboseLogging)
	})
	return nil
}

func logRequest(request *http.Request, fn func(format string, v ...interface{}), bodyLoggingLevel int) {
	if request == nil {
		return
	}
	dumpBody := true
	if checkBodyLengthExceedLimit(request.ContentLength) {
		fn("not dumping body too big\n")
		dumpBody = false
	}

	dumpBody = dumpBody && defaultLogger.LogLevel() >= bodyLoggingLevel && bodyLoggingLevel != noLogging
	if dump, e := httputil.DumpRequestOut(request, dumpBody); e == nil {
		fn("Dump Request %s", string(dump))
	} else {
		fn("%v\n", e)
	}
}

func logResponse(response *http.Response, fn func(format string, v ...interface{}), bodyLoggingLevel int) {
	if response == nil {
		return
	}
	dumpBody := true
	if checkBodyLengthExceedLimit(response.ContentLength) {
		fn("not dumping body too big\n")
		dumpBody = false
	}
	dumpBody = dumpBody && defaultLogger.LogLevel() >= bodyLoggingLevel && bodyLoggingLevel != noLogging
	if dump, e := httputil.DumpResponse(response, dumpBody); e == nil {
		fn("Dump Response %s", string(dump))
	} else {
		fn("%v\n", e)
	}
}

func checkBodyLengthExceedLimit(contentLength int64) bool {
	if contentLength > maxBodyLenForDebug {
		return true
	}
	return false
}

// OCIRequest is any request made to an OCI service.
type OCIRequest interface {
	// HTTPRequest assembles an HTTP request.
	HTTPRequest(method, path string, binaryRequestBody *OCIReadSeekCloser, extraHeaders map[string]string) (http.Request, error)
}

// RequestMetadata is metadata about an OCIRequest. This structure represents the behavior exhibited by the SDK when
// issuing (or reissuing) a request.
type RequestMetadata struct {
	// RetryPolicy is the policy for reissuing the request. If no retry policy is set on the request,
	// then the request will be issued exactly once.
	RetryPolicy *RetryPolicy
}

// OCIReadSeekCloser is a thread-safe io.ReadSeekCloser to prevent racing with retrying binary requests
type OCIReadSeekCloser struct {
	rc       io.ReadCloser
	lock     sync.Mutex
	isClosed bool
}

// NewOCIReadSeekCloser constructs OCIReadSeekCloser, the only input is binary request body
func NewOCIReadSeekCloser(rc io.ReadCloser) *OCIReadSeekCloser {
	rsc := OCIReadSeekCloser{}
	rsc.rc = rc
	return &rsc
}

// Seek is a thread-safe operation, it implements io.seek() interface, if the original request body implements io.seek()
// interface, or implements "well-known" data type like os.File, io.SectionReader, or wrapped by ioutil.NopCloser can be supported
func (rsc *OCIReadSeekCloser) Seek(offset int64, whence int) (int64, error) {
	rsc.lock.Lock()
	defer rsc.lock.Unlock()

	if _, ok := rsc.rc.(io.Seeker); ok {
		return rsc.rc.(io.Seeker).Seek(offset, whence)
	}
	// once the binary request body is wrapped with ioutil.NopCloser:
	if rsc.isNopCloser() {
		unwrappedInterface := reflect.ValueOf(rsc.rc).Field(0).Interface()
		if _, ok := unwrappedInterface.(io.Seeker); ok {
			return unwrappedInterface.(io.Seeker).Seek(offset, whence)
		}
	}
	return 0, fmt.Errorf("current binary request body type is not seekable, if want to use retry feature, please make sure the request body implements seek() method")
}

// Close is a thread-safe operation, it closes the instance of the OCIReadSeekCloser's access to the underlying io.ReadCloser.
func (rsc *OCIReadSeekCloser) Close() error {
	rsc.lock.Lock()
	defer rsc.lock.Unlock()
	rsc.isClosed = true
	return nil
}

// Read is a thread-safe operation, it implements io.Read() interface
func (rsc *OCIReadSeekCloser) Read(p []byte) (n int, err error) {
	rsc.lock.Lock()
	defer rsc.lock.Unlock()

	if rsc.isClosed {
		return 0, io.EOF
	}

	return rsc.rc.Read(p)
}

// Seekable is used for check if the binary request body can be seek or no
func (rsc *OCIReadSeekCloser) Seekable() bool {
	if rsc == nil {
		return false
	}
	if _, ok := rsc.rc.(io.Seeker); ok {
		return true
	}
	// once the binary request body is wrapped with ioutil.NopCloser:
	if rsc.isNopCloser() {
		if _, ok := reflect.ValueOf(rsc.rc).Field(0).Interface().(io.Seeker); ok {
			return true
		}
	}
	return false
}

// Helper function to judge if this struct is a nopCloser or nopCloserWriterTo
func (rsc *OCIReadSeekCloser) isNopCloser() bool {
	if reflect.TypeOf(rsc.rc) == reflect.TypeOf(ioutil.NopCloser(nil)) || reflect.TypeOf(rsc.rc) == reflect.TypeOf(ioutil.NopCloser(bytes.NewReader(nil))) {
		return true
	}
	return false
}

// OCIResponse is the response from issuing a request to an OCI service.
type OCIResponse interface {
	// HTTPResponse returns the raw HTTP response.
	HTTPResponse() *http.Response
}

// OCIOperation is the generalization of a request-response cycle undergone by an OCI service.
type OCIOperation func(context.Context, OCIRequest, *OCIReadSeekCloser, map[string]string) (OCIResponse, error)

// ClientCallDetails a set of settings used by the a single Call operation of the http Client
type ClientCallDetails struct {
	Signer HTTPRequestSigner
}

// Call executes the http request with the given context
func (client BaseClient) Call(ctx context.Context, request *http.Request) (response *http.Response, err error) {
	if client.IsRefreshableAuthType() {
		return client.RefreshableTokenWrappedCallWithDetails(ctx, request, ClientCallDetails{Signer: client.Signer})
	}
	return client.CallWithDetails(ctx, request, ClientCallDetails{Signer: client.Signer})
}

// RefreshableTokenWrappedCallWithDetails wraps the CallWithDetails with retry on 401 for Refreshable Toekn (Instance Principal, Resource Principal etc.)
// This is to intimitate the race condition on refresh
func (client BaseClient) RefreshableTokenWrappedCallWithDetails(ctx context.Context, request *http.Request, details ClientCallDetails) (response *http.Response, err error) {
	for i := 0; i < maxAttemptsForRefreshableRetry; i++ {
		response, err = client.CallWithDetails(ctx, request, ClientCallDetails{Signer: client.Signer})
		if response != nil && response.StatusCode != 401 {
			return response, err
		}
		time.Sleep(1 * time.Second)
	}
	return
}

// CallWithDetails executes the http request, the given context using details specified in the parameters, this function
// provides a way to override some settings present in the client
func (client BaseClient) CallWithDetails(ctx context.Context, request *http.Request, details ClientCallDetails) (response *http.Response, err error) {
	Debugln("Attempting to call downstream service")
	request = request.WithContext(ctx)
	err = client.prepareRequest(request)
	if err != nil {
		return
	}
	//Intercept
	err = client.intercept(request)
	if err != nil {
		return
	}
	//Sign the request
	err = details.Signer.Sign(request)
	if err != nil {
		return
	}

	//Execute the http request
	if ociGoBreaker := client.Configuration.CircuitBreaker; ociGoBreaker != nil {
		resp, cbErr := ociGoBreaker.Cb.Execute(func() (interface{}, error) {
			return client.httpDo(request)
		})
		if httpResp, ok := resp.(*http.Response); ok {
			if httpResp != nil && httpResp.StatusCode != 200 {
				if failure, ok := IsServiceError(cbErr); ok {
					ociGoBreaker.AddToHistory(resp.(*http.Response), failure)
				}
			}
		}
		if cbErr != nil && IsCircuitBreakerError(cbErr) {
			cbErr = getCircuitBreakerError(request, cbErr, ociGoBreaker)
		}
		if _, ok := resp.(*http.Response); !ok {
			return nil, cbErr
		}
		return resp.(*http.Response), cbErr
	}
	return client.httpDo(request)
}

// IsRefreshableAuthType validates if a signer is from a refreshable config provider
func (client BaseClient) IsRefreshableAuthType() bool {
	if signer, ok := client.Signer.(ociRequestSigner); ok {
		if provider, ok := signer.KeyProvider.(RefreshableConfigurationProvider); ok {
			return provider.Refreshable()
		}
	}
	return false
}

func (client BaseClient) httpDo(request *http.Request) (response *http.Response, err error) {

	//Copy request body and save for logging
	dumpRequestBody := ioutil.NopCloser(bytes.NewBuffer(nil))
	if request.Body != nil && !checkBodyLengthExceedLimit(request.ContentLength) {
		if dumpRequestBody, request.Body, err = drainBody(request.Body); err != nil {
			dumpRequestBody = ioutil.NopCloser(bytes.NewBuffer(nil))
		}
	}
	IfDebug(func() {
		logRequest(request, Debugf, verboseLogging)
	})

	//Execute the http request
	response, err = client.HTTPClient.Do(request)

	if err != nil {
		IfInfo(func() {
			Logf("%v\n", err)
		})
		return response, err
	}

	err = checkForSuccessfulResponse(response, &dumpRequestBody)
	return response, err
}

// CloseBodyIfValid closes the body of an http response if the response and the body are valid
func CloseBodyIfValid(httpResponse *http.Response) {
	if httpResponse != nil && httpResponse.Body != nil {
		httpResponse.Body.Close()
	}
}

// IsOciRealmSpecificServiceEndpointTemplateEnabled returns true if the client is configured to use realm specific service endpoint template
// it will first check the client configuration, if not set, it will check the environment variable
func (client BaseClient) IsOciRealmSpecificServiceEndpointTemplateEnabled() bool {
	if client.Configuration.RealmSpecificServiceEndpointTemplateEnabled != nil {
		return *client.Configuration.RealmSpecificServiceEndpointTemplateEnabled
	}
	return IsEnvVarTrue(OciRealmSpecificServiceEndpointTemplateEnabledEnvVar)
}
