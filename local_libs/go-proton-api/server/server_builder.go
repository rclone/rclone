package server

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rclone/go-proton-api"
	"github.com/rclone/go-proton-api/server/backend"
)

type serverBuilder struct {
	config         *http.Server
	listener       net.Listener
	withTLS        bool
	domain         string
	logger         io.Writer
	origin         string
	proxyTransport *http.Transport
	cacher         AuthCacher
	rateLimiter    *rateLimiter
	enableDedup    bool
}

func newServerBuilder() *serverBuilder {
	var logger io.Writer

	if os.Getenv("GO_PROTON_API_SERVER_LOGGER_ENABLED") != "" {
		logger = gin.DefaultWriter
	} else {
		logger = io.Discard
	}

	return &serverBuilder{
		config:         &http.Server{},
		withTLS:        true,
		domain:         "proton.local",
		logger:         logger,
		origin:         proton.DefaultHostURL,
		proxyTransport: &http.Transport{},
	}
}

func (builder *serverBuilder) build() *Server {
	gin.SetMode(gin.ReleaseMode)

	s := &Server{
		r: gin.New(),
		b: backend.New(time.Hour, builder.domain, builder.enableDedup),

		domain:         builder.domain,
		proxyOrigin:    builder.origin,
		authCacher:     builder.cacher,
		rateLimit:      builder.rateLimiter,
		proxyTransport: builder.proxyTransport,
	}

	s.r.Use(gin.CustomRecovery(func(c *gin.Context, recovered any) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"Code":    http.StatusInternalServerError,
			"Error":   "Internal server error",
			"Details": recovered,
		})
	}))

	var l net.Listener

	if builder.listener == nil {
		var err error

		if l, err = net.Listen("tcp", "127.0.0.1:0"); err != nil {
			panic(err)
		}
	} else {
		l = builder.listener
	}

	// Create the test server.
	s.s = &httptest.Server{
		Listener: l,
		Config:   builder.config,
	}

	// Set the server to use the custom handler.
	s.s.Config.Handler = s.r

	// Start the server.
	if builder.withTLS {
		s.s.StartTLS()
	} else {
		s.s.Start()
	}

	s.r.Use(
		gin.LoggerWithConfig(gin.LoggerConfig{Output: builder.logger}),
		gin.Recovery(),
		s.logCalls(),
		s.handleOffline(),
	)

	initRouter(s)

	return s
}

// Option represents a type that can be used to configure the server.
type Option interface {
	config(*serverBuilder)
}

// WithTLS controls whether the server should serve over TLS.
func WithTLS(tls bool) Option {
	return &withTLS{
		withTLS: tls,
	}
}

type withTLS struct {
	withTLS bool
}

func (opt withTLS) config(builder *serverBuilder) {
	builder.withTLS = opt.withTLS
}

// WithDomain controls the domain of the server.
func WithDomain(domain string) Option {
	return &withDomain{
		domain: domain,
	}
}

type withDomain struct {
	domain string
}

func (opt withDomain) config(builder *serverBuilder) {
	builder.domain = opt.domain
}

// WithLogger controls where Gin logs to.
func WithLogger(logger io.Writer) Option {
	return &withLogger{
		logger: logger,
	}
}

type withLogger struct {
	logger io.Writer
}

func (opt withLogger) config(builder *serverBuilder) {
	builder.logger = opt.logger
}

func WithProxyOrigin(origin string) Option {
	return &withProxyOrigin{
		origin: origin,
	}
}

type withProxyOrigin struct {
	origin string
}

func (opt withProxyOrigin) config(builder *serverBuilder) {
	builder.origin = opt.origin
}

func WithAuthCacher(cacher AuthCacher) Option {
	return &withAuthCache{
		cacher: cacher,
	}
}

type withAuthCache struct {
	cacher AuthCacher
}

func (opt withAuthCache) config(builder *serverBuilder) {
	builder.cacher = opt.cacher
}

func WithRateLimit(limit int, window time.Duration) Option {
	return &withRateLimit{
		limit:      limit,
		window:     window,
		statusCode: http.StatusTooManyRequests,
	}
}

func WithRateLimitAndCustomStatusCode(limit int, window time.Duration, code int) Option {
	return &withRateLimit{
		limit:      limit,
		window:     window,
		statusCode: code,
	}
}

type withRateLimit struct {
	limit      int
	statusCode int
	window     time.Duration
}

func (opt withRateLimit) config(builder *serverBuilder) {
	builder.rateLimiter = newRateLimiter(opt.limit, opt.window, opt.statusCode)
}

func WithProxyTransport(transport *http.Transport) Option {
	return &withProxyTransport{
		transport: transport,
	}
}

type withProxyTransport struct {
	transport *http.Transport
}

func (opt withProxyTransport) config(builder *serverBuilder) {
	builder.proxyTransport = opt.transport
}

type withServerConfig struct {
	cfg *http.Server
}

func (opt withServerConfig) config(builder *serverBuilder) {
	builder.config = opt.cfg
}

// WithServerConfig allows you to configure the underlying HTTP server.
func WithServerConfig(cfg *http.Server) Option {
	return withServerConfig{
		cfg: cfg,
	}
}

type withNetListener struct {
	listener net.Listener
}

func (opt withNetListener) config(builder *serverBuilder) {
	builder.listener = opt.listener
}

// WithListener allows you to set the net.Listener to use.
func WithListener(listener net.Listener) Option {
	return withNetListener{
		listener: listener,
	}
}

type withMessageDedup struct{}

func (withMessageDedup) config(builder *serverBuilder) {
	builder.enableDedup = true
}

func WithMessageDedup() Option {
	return &withMessageDedup{}
}
