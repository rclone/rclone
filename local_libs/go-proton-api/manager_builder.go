package proton

import (
	"net/http"
	"time"

	"github.com/ProtonMail/gluon/async"
	"github.com/go-resty/resty/v2"
)

const (
	// DefaultHostURL is the default host of the API.
	DefaultHostURL = "https://mail.proton.me/api"

	// DefaultAppVersion is the default app version used to communicate with the API.
	// This must be changed (using the WithAppVersion option) for production use.
	DefaultAppVersion = "go-proton-api"

	// DefaultUserAgent is the default user agent used to communicate with the API.
	// See: https://github.com/emersion/hydroxide/issues/252
	DefaultUserAgent = ""
)

type managerBuilder struct {
	hostURL      string
	appVersion   string
	userAgent    string
	transport    http.RoundTripper
	verifyProofs bool
	cookieJar    http.CookieJar
	retryCount   int
	logger       resty.Logger
	debug        bool
	panicHandler async.PanicHandler
}

func newManagerBuilder() *managerBuilder {
	return &managerBuilder{
		hostURL:      DefaultHostURL,
		appVersion:   DefaultAppVersion,
		userAgent:    DefaultUserAgent,
		transport:    http.DefaultTransport,
		verifyProofs: true,
		cookieJar:    nil,
		retryCount:   3,
		logger:       nil,
		debug:        false,
		panicHandler: async.NoopPanicHandler{},
	}
}

func (builder *managerBuilder) build() *Manager {
	m := &Manager{
		rc: resty.New(),

		errHandlers: make(map[Code][]Handler),

		verifyProofs: builder.verifyProofs,

		panicHandler: builder.panicHandler,
	}

	// Set the API host.
	m.rc.SetBaseURL(builder.hostURL)

	// Set the transport.
	m.rc.SetTransport(builder.transport)

	// Set the cookie jar.
	m.rc.SetCookieJar(builder.cookieJar)

	// Set the logger.
	if builder.logger != nil {
		m.rc.SetLogger(builder.logger)
	}

	// Set the debug flag.
	m.rc.SetDebug(builder.debug)

	// Set app version in header.
	m.rc.OnBeforeRequest(func(_ *resty.Client, req *resty.Request) error {
		req.SetHeader("x-pm-appversion", builder.appVersion)
		req.SetHeader("User-Agent", builder.userAgent)
		return nil
	})

	// Set middleware.
	m.rc.OnAfterResponse(catchAPIError)
	m.rc.OnAfterResponse(updateTime)
	m.rc.OnAfterResponse(m.checkConnUp)
	m.rc.OnError(m.checkConnDown)
	m.rc.OnError(m.handleError)

	// Configure retry mechanism.
	m.rc.SetRetryCount(builder.retryCount)
	m.rc.SetRetryMaxWaitTime(time.Minute)
	m.rc.AddRetryCondition(catchTooManyRequests)
	m.rc.AddRetryCondition(catchDialError)
	m.rc.AddRetryCondition(catchDropError)
	m.rc.SetRetryAfter(catchRetryAfter)

	// Set the data type of API errors.
	m.rc.SetError(&APIError{})

	return m
}
