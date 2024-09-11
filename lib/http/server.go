// Package http provides a registration interface for http services
package http

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/lib/atexit"
	sdActivation "github.com/rclone/rclone/lib/sdactivation"
	"github.com/spf13/pflag"
)

// Help returns text describing the http server to add to the command
// help.
func Help(prefix string) string {
	help := `### Server options

Use ` + "`--{{ .Prefix }}addr`" + ` to specify which IP address and port the server should
listen on, eg ` + "`--{{ .Prefix }}addr 1.2.3.4:8000` or `--{{ .Prefix }}addr :8080`" + ` to listen to all
IPs.  By default it only listens on localhost.  You can use port
:0 to let the OS choose an available port.

If you set ` + "`--{{ .Prefix }}addr`" + ` to listen on a public or LAN accessible IP address
then using Authentication is advised - see the next section for info.

You can use a unix socket by setting the url to ` + "`unix:///path/to/socket`" + `
or just by using an absolute path name. Note that unix sockets bypass the
authentication - this is expected to be done with file system permissions.

` + "`--{{ .Prefix }}addr`" + ` may be repeated to listen on multiple IPs/ports/sockets.
Socket activation, described further below, can also be used to accomplish the same.

` + "`--{{ .Prefix }}server-read-timeout` and `--{{ .Prefix }}server-write-timeout`" + ` can be used to
control the timeouts on the server.  Note that this is the total time
for a transfer.

` + "`--{{ .Prefix }}max-header-bytes`" + ` controls the maximum number of bytes the server will
accept in the HTTP header.

` + "`--{{ .Prefix }}baseurl`" + ` controls the URL prefix that rclone serves from.  By default
rclone will serve from the root.  If you used ` + "`--{{ .Prefix }}baseurl \"/rclone\"`" + ` then
rclone would serve from a URL starting with "/rclone/".  This is
useful if you wish to proxy rclone serve.  Rclone automatically
inserts leading and trailing "/" on ` + "`--{{ .Prefix }}baseurl`" + `, so ` + "`--{{ .Prefix }}baseurl \"rclone\"`" + `,
` + "`--{{ .Prefix }}baseurl \"/rclone\"` and `--{{ .Prefix }}baseurl \"/rclone/\"`" + ` are all treated
identically.

#### TLS (SSL)

By default this will serve over http.  If you want you can serve over
https.  You will need to supply the ` + "`--{{ .Prefix }}cert` and `--{{ .Prefix }}key`" + ` flags.
If you wish to do client side certificate validation then you will need to
supply ` + "`--{{ .Prefix }}client-ca`" + ` also.

` + "`--{{ .Prefix }}cert`" + ` should be a either a PEM encoded certificate or a concatenation
of that with the CA certificate.  ` + "`--k{{ .Prefix }}ey`" + ` should be the PEM encoded
private key and ` + "`--{{ .Prefix }}client-ca`" + ` should be the PEM encoded client
certificate authority certificate.

` + "`--{{ .Prefix }}min-tls-version`" + ` is minimum TLS version that is acceptable. Valid
  values are "tls1.0", "tls1.1", "tls1.2" and "tls1.3" (default
  "tls1.0").

### Socket activation

Instead of the listening addresses specified above, rclone will listen to all
FDs passed by the service manager, if any (and ignore any arguments passed by ` +
		"--{{ .Prefix }}addr`" + `).

This allows rclone to be a socket-activated service.
It can be configured with .socket and .service unit files as described in
https://www.freedesktop.org/software/systemd/man/latest/systemd.socket.html

Socket activation can be tested ad-hoc with the ` + "`systemd-socket-activate`" + `command

       systemd-socket-activate -l 8000 -- rclone serve

This will socket-activate rclone on the first connection to port 8000 over TCP.
`
	tmpl, err := template.New("server help").Parse(help)
	if err != nil {
		fs.Fatal(nil, fmt.Sprint("Fatal error parsing template", err))
	}

	data := struct {
		Prefix string
	}{
		Prefix: prefix,
	}
	buf := &bytes.Buffer{}
	err = tmpl.Execute(buf, data)
	if err != nil {
		fs.Fatal(nil, fmt.Sprint("Fatal error executing template", err))
	}
	return buf.String()
}

// Middleware function signature required by chi.Router.Use()
type Middleware func(http.Handler) http.Handler

// ConfigInfo descripts the Options in use
var ConfigInfo = fs.Options{{
	Name:    "addr",
	Default: []string{"127.0.0.1:8080"},
	Help:    "IPaddress:Port or :Port to bind server to",
}, {
	Name:    "server_read_timeout",
	Default: 1 * time.Hour,
	Help:    "Timeout for server reading data",
}, {
	Name:    "server_write_timeout",
	Default: 1 * time.Hour,
	Help:    "Timeout for server writing data",
}, {
	Name:    "max_header_bytes",
	Default: 4096,
	Help:    "Maximum size of request header",
}, {
	Name:    "cert",
	Default: "",
	Help:    "TLS PEM key (concatenation of certificate and CA certificate)",
}, {
	Name:    "key",
	Default: "",
	Help:    "TLS PEM Private key",
}, {
	Name:    "client_ca",
	Default: "",
	Help:    "Client certificate authority to verify clients with",
}, {
	Name:    "baseurl",
	Default: "",
	Help:    "Prefix for URLs - leave blank for root",
}, {
	Name:    "min_tls_version",
	Default: "tls1.0",
	Help:    "Minimum TLS version that is acceptable",
}, {
	Name:    "allow_origin",
	Default: "",
	Help:    "Origin which cross-domain request (CORS) can be executed from",
}}

// Config contains options for the http Server
type Config struct {
	ListenAddr         []string      `config:"addr"`                 // Port to listen on
	BaseURL            string        `config:"baseurl"`              // prefix to strip from URLs
	ServerReadTimeout  time.Duration `config:"server_read_timeout"`  // Timeout for server reading data
	ServerWriteTimeout time.Duration `config:"server_write_timeout"` // Timeout for server writing data
	MaxHeaderBytes     int           `config:"max_header_bytes"`     // Maximum size of request header
	TLSCert            string        `config:"cert"`                 // Path to TLS PEM key (concatenation of certificate and CA certificate)
	TLSKey             string        `config:"key"`                  // Path to TLS PEM Private key
	TLSCertBody        []byte        `config:"-"`                    // TLS PEM key (concatenation of certificate and CA certificate) body, ignores TLSCert
	TLSKeyBody         []byte        `config:"-"`                    // TLS PEM Private key body, ignores TLSKey
	ClientCA           string        `config:"client_ca"`            // Client certificate authority to verify clients with
	MinTLSVersion      string        `config:"min_tls_version"`      // MinTLSVersion contains the minimum TLS version that is acceptable.
	AllowOrigin        string        `config:"allow_origin"`         // AllowOrigin sets the Access-Control-Allow-Origin header
}

// AddFlagsPrefix adds flags for the httplib
func (cfg *Config) AddFlagsPrefix(flagSet *pflag.FlagSet, prefix string) {
	flags.StringArrayVarP(flagSet, &cfg.ListenAddr, prefix+"addr", "", cfg.ListenAddr, "IPaddress:Port, :Port or [unix://]/path/to/socket to bind server to", prefix)
	flags.DurationVarP(flagSet, &cfg.ServerReadTimeout, prefix+"server-read-timeout", "", cfg.ServerReadTimeout, "Timeout for server reading data", prefix)
	flags.DurationVarP(flagSet, &cfg.ServerWriteTimeout, prefix+"server-write-timeout", "", cfg.ServerWriteTimeout, "Timeout for server writing data", prefix)
	flags.IntVarP(flagSet, &cfg.MaxHeaderBytes, prefix+"max-header-bytes", "", cfg.MaxHeaderBytes, "Maximum size of request header", prefix)
	flags.StringVarP(flagSet, &cfg.TLSCert, prefix+"cert", "", cfg.TLSCert, "TLS PEM key (concatenation of certificate and CA certificate)", prefix)
	flags.StringVarP(flagSet, &cfg.TLSKey, prefix+"key", "", cfg.TLSKey, "TLS PEM Private key", prefix)
	flags.StringVarP(flagSet, &cfg.ClientCA, prefix+"client-ca", "", cfg.ClientCA, "Client certificate authority to verify clients with", prefix)
	flags.StringVarP(flagSet, &cfg.BaseURL, prefix+"baseurl", "", cfg.BaseURL, "Prefix for URLs - leave blank for root", prefix)
	flags.StringVarP(flagSet, &cfg.MinTLSVersion, prefix+"min-tls-version", "", cfg.MinTLSVersion, "Minimum TLS version that is acceptable", prefix)
	flags.StringVarP(flagSet, &cfg.AllowOrigin, prefix+"allow-origin", "", cfg.AllowOrigin, "Origin which cross-domain request (CORS) can be executed from", prefix)
}

// AddHTTPFlagsPrefix adds flags for the httplib
func AddHTTPFlagsPrefix(flagSet *pflag.FlagSet, prefix string, cfg *Config) {
	cfg.AddFlagsPrefix(flagSet, prefix)
}

// DefaultCfg is the default values used for Config
//
// Note that this needs to be kept in sync with ConfigInfo above and
// can be removed when all callers have been converted.
func DefaultCfg() Config {
	return Config{
		ListenAddr:         []string{"127.0.0.1:8080"},
		ServerReadTimeout:  1 * time.Hour,
		ServerWriteTimeout: 1 * time.Hour,
		MaxHeaderBytes:     4096,
		MinTLSVersion:      "tls1.0",
	}
}

type instance struct {
	url        string
	listener   net.Listener
	httpServer *http.Server
}

func (s instance) serve(wg *sync.WaitGroup) {
	defer wg.Done()
	err := s.httpServer.Serve(s.listener)
	if err != http.ErrServerClosed && err != nil {
		fs.Logf(nil, "%s: unexpected error: %s", s.listener.Addr(), err.Error())
	}
}

// Server contains info about the running http server
type Server struct {
	wg           sync.WaitGroup
	mux          chi.Router
	tlsConfig    *tls.Config
	instances    []instance
	auth         AuthConfig
	cfg          Config
	template     *TemplateConfig
	htmlTemplate *template.Template
	usingAuth    bool // set if we are using auth middleware
	atexitHandle atexit.FnHandle
}

// Option allows customizing the server
type Option func(*Server)

// WithAuth option initializes the appropriate auth middleware
func WithAuth(cfg AuthConfig) Option {
	return func(s *Server) {
		s.auth = cfg
	}
}

// WithConfig option applies the Config to the server, overriding defaults
func WithConfig(cfg Config) Option {
	return func(s *Server) {
		s.cfg = cfg
	}
}

// WithTemplate option allows the parsing of a template
func WithTemplate(cfg TemplateConfig) Option {
	return func(s *Server) {
		s.template = &cfg
	}
}

// For a given listener, and optional tlsConfig, construct a instance.
// The url string ends up in the `url` field of the `instance`.
// This unconditionally wraps the listener with the provided TLS config if one
// is specified, so all decision logic on whether to use TLS needs to live at
// the callsite.
func newInstance(ctx context.Context, s *Server, listener net.Listener, tlsCfg *tls.Config, url string) *instance {
	if tlsCfg != nil {
		listener = tls.NewListener(listener, tlsCfg)
	}

	return &instance{
		url:      url,
		listener: listener,
		httpServer: &http.Server{
			Handler:           s.mux,
			ReadTimeout:       s.cfg.ServerReadTimeout,
			WriteTimeout:      s.cfg.ServerWriteTimeout,
			MaxHeaderBytes:    s.cfg.MaxHeaderBytes,
			ReadHeaderTimeout: 10 * time.Second, // time to send the headers
			IdleTimeout:       60 * time.Second, // time to keep idle connections open
			TLSConfig:         tlsCfg,
			BaseContext:       NewBaseContext(ctx, url),
		},
	}
}

// NewServer instantiates a new http server using provided listeners and options
// This function is provided if the default http server does not meet a services requirements and should not generally be used
// A http server can listen using multiple listeners. For example, a listener for port 80, and a listener for port 443.
// tlsListeners are ignored if opt.TLSKey is not provided
func NewServer(ctx context.Context, options ...Option) (*Server, error) {
	s := &Server{
		mux: chi.NewRouter(),
		cfg: DefaultCfg(),
	}

	// Make sure default logger is logging where everything else is
	// middleware.DefaultLogger = middleware.RequestLogger(&middleware.DefaultLogFormatter{Logger: log.Default(), NoColor: true})
	// Log requests
	// s.mux.Use(middleware.Logger)

	for _, opt := range options {
		opt(s)
	}

	// Build base router
	s.mux.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	})
	s.mux.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	})

	// Ignore passing "/" for BaseURL
	s.cfg.BaseURL = strings.Trim(s.cfg.BaseURL, "/")
	if s.cfg.BaseURL != "" {
		s.cfg.BaseURL = "/" + s.cfg.BaseURL
		s.mux.Use(MiddlewareStripPrefix(s.cfg.BaseURL))
	}

	err := s.initTemplate()
	if err != nil {
		return nil, err
	}

	err = s.initTLS()
	if err != nil {
		return nil, err
	}

	s.mux.Use(MiddlewareCORS(s.cfg.AllowOrigin))

	s.initAuth()

	// (Only) listen on FDs provided by the service manager, if any.
	sdListeners, err := sdActivation.ListenersWithNames()
	if err != nil {
		return nil, fmt.Errorf("unable to acquire listeners: %w", err)
	}

	if len(sdListeners) != 0 {
		for listenerName, listeners := range sdListeners {
			for i, listener := range listeners {
				url := fmt.Sprintf("sd-listen:%s-%d/%s", listenerName, i, s.cfg.BaseURL)
				if s.tlsConfig != nil {
					url = fmt.Sprintf("sd-listen+tls:%s-%d/%s", listenerName, i, s.cfg.BaseURL)
				}

				instance := newInstance(ctx, s, listener, s.tlsConfig, url)

				s.instances = append(s.instances, *instance)
			}
		}

		return s, nil
	}

	// Process all listeners specified in the CLI Args.
	for _, addr := range s.cfg.ListenAddr {
		var instance *instance

		if strings.HasPrefix(addr, "unix://") || filepath.IsAbs(addr) {
			addr = strings.TrimPrefix(addr, "unix://")

			listener, err := net.Listen("unix", addr)
			if err != nil {
				return nil, err
			}
			instance = newInstance(ctx, s, listener, s.tlsConfig, addr)
		} else if strings.HasPrefix(addr, "tls://") || (len(s.cfg.ListenAddr) == 1 && s.tlsConfig != nil) {
			addr = strings.TrimPrefix(addr, "tls://")
			listener, err := net.Listen("tcp", addr)
			if err != nil {
				return nil, err
			}
			instance = newInstance(ctx, s, listener, s.tlsConfig, fmt.Sprintf("https://%s%s/", listener.Addr().String(), s.cfg.BaseURL))
		} else {
			// HTTP case
			addr = strings.TrimPrefix(addr, "http://")
			listener, err := net.Listen("tcp", addr)
			if err != nil {
				return nil, err
			}
			instance = newInstance(ctx, s, listener, nil, fmt.Sprintf("http://%s%s/", listener.Addr().String(), s.cfg.BaseURL))

		}

		s.instances = append(s.instances, *instance)
	}

	return s, nil
}

func (s *Server) initAuth() {
	s.usingAuth = false

	authCertificateUserEnabled := s.tlsConfig != nil && s.tlsConfig.ClientAuth != tls.NoClientCert && s.auth.HtPasswd == "" && s.auth.BasicUser == ""
	if authCertificateUserEnabled {
		s.usingAuth = true
		s.mux.Use(MiddlewareAuthCertificateUser())
	}

	if s.auth.CustomAuthFn != nil {
		s.usingAuth = true
		s.mux.Use(MiddlewareAuthCustom(s.auth.CustomAuthFn, s.auth.Realm, authCertificateUserEnabled))
		return
	}

	if s.auth.HtPasswd != "" {
		s.usingAuth = true
		s.mux.Use(MiddlewareAuthHtpasswd(s.auth.HtPasswd, s.auth.Realm))
		return
	}

	if s.auth.BasicUser != "" {
		s.usingAuth = true
		s.mux.Use(MiddlewareAuthBasic(s.auth.BasicUser, s.auth.BasicPass, s.auth.Realm, s.auth.Salt))
		return
	}
}

func (s *Server) initTemplate() error {
	if s.template == nil {
		return nil
	}

	var err error
	s.htmlTemplate, err = GetTemplate(s.template.Path)
	if err != nil {
		err = fmt.Errorf("failed to get template: %w", err)
	}

	return err
}

var (
	// ErrInvalidMinTLSVersion - hard coded errors, allowing for easier testing
	ErrInvalidMinTLSVersion = errors.New("invalid value for --min-tls-version")
	// ErrTLSBodyMismatch - hard coded errors, allowing for easier testing
	ErrTLSBodyMismatch = errors.New("need both TLSCertBody and TLSKeyBody to use TLS")
	// ErrTLSFileMismatch - hard coded errors, allowing for easier testing
	ErrTLSFileMismatch = errors.New("need both --cert and --key to use TLS")
	// ErrTLSParseCA - hard coded errors, allowing for easier testing
	ErrTLSParseCA = errors.New("unable to parse client certificate authority")
)

func (s *Server) initTLS() error {
	if s.cfg.TLSCert == "" && s.cfg.TLSKey == "" && len(s.cfg.TLSCertBody) == 0 && len(s.cfg.TLSKeyBody) == 0 {
		return nil
	}

	if (len(s.cfg.TLSCertBody) > 0) != (len(s.cfg.TLSKeyBody) > 0) {
		return ErrTLSBodyMismatch
	}

	if (s.cfg.TLSCert != "") != (s.cfg.TLSKey != "") {
		return ErrTLSFileMismatch
	}

	var cert tls.Certificate
	var err error
	if len(s.cfg.TLSCertBody) > 0 {
		cert, err = tls.X509KeyPair(s.cfg.TLSCertBody, s.cfg.TLSKeyBody)
	} else {
		cert, err = tls.LoadX509KeyPair(s.cfg.TLSCert, s.cfg.TLSKey)
	}
	if err != nil {
		return err
	}

	var minTLSVersion uint16
	switch s.cfg.MinTLSVersion {
	case "tls1.0":
		minTLSVersion = tls.VersionTLS10
	case "tls1.1":
		minTLSVersion = tls.VersionTLS11
	case "tls1.2":
		minTLSVersion = tls.VersionTLS12
	case "tls1.3":
		minTLSVersion = tls.VersionTLS13
	default:
		return fmt.Errorf("%w: %s", ErrInvalidMinTLSVersion, s.cfg.MinTLSVersion)
	}

	s.tlsConfig = &tls.Config{
		MinVersion:   minTLSVersion,
		Certificates: []tls.Certificate{cert},
	}

	if s.cfg.ClientCA != "" {
		// if !useTLS {
		// 	err := errors.New("can't use --client-ca without --cert and --key")
		// 	log.Fatalf(err.Error())
		// }
		certpool := x509.NewCertPool()
		pem, err := os.ReadFile(s.cfg.ClientCA)
		if err != nil {
			return err
		}

		if !certpool.AppendCertsFromPEM(pem) {
			return ErrTLSParseCA
		}

		s.tlsConfig.ClientCAs = certpool
		s.tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return nil
}

// Serve starts the HTTP server on each listener
func (s *Server) Serve() {
	s.wg.Add(len(s.instances))
	for _, ii := range s.instances {
		// TODO: decide how/when to log listening url
		// log.Printf("listening on %s", ii.url)
		go ii.serve(&s.wg)
	}
	// Install an atexit handler to shutdown gracefully
	s.atexitHandle = atexit.Register(func() { _ = s.Shutdown() })
}

// Wait blocks while the server is serving requests
func (s *Server) Wait() {
	s.wg.Wait()
}

// Router returns the server base router
func (s *Server) Router() chi.Router {
	return s.mux
}

// Time to wait to Shutdown an HTTP server
const gracefulShutdownTime = 10 * time.Second

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	// Stop the atexit handler
	if s.atexitHandle != nil {
		atexit.Unregister(s.atexitHandle)
		s.atexitHandle = nil
	}
	for _, ii := range s.instances {
		expiry := time.Now().Add(gracefulShutdownTime)
		ctx, cancel := context.WithDeadline(context.Background(), expiry)
		if err := ii.httpServer.Shutdown(ctx); err != nil {
			fs.Logf(nil, "error shutting down server: %s", err)
		}
		cancel()
	}
	s.wg.Wait()
	return nil
}

// HTMLTemplate returns the parsed template, if WithTemplate option was passed.
func (s *Server) HTMLTemplate() *template.Template {
	return s.htmlTemplate
}

// URLs returns all configured URLS
func (s *Server) URLs() []string {
	var out []string
	for _, ii := range s.instances {
		if ii.listener.Addr().Network() == "unix" {
			continue
		}
		out = append(out, ii.url)
	}
	return out
}

// UsingAuth returns true if authentication is required
func (s *Server) UsingAuth() bool {
	return s.usingAuth
}
