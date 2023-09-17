// Copyright 2018 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"

	"goftp.io/server/v2/ratelimit"
)

var (
	version = "2.0beta"
)

// Options contains parameters for server.NewServer()
type Options struct {
	// This server supported commands, if blank, it will be defaultCommands
	// So that users could override the Commands
	Commands map[string]Command

	// The driver that will be used to handle files persistent
	Driver Driver

	// How to hanle the authenticate requests
	Auth Auth

	// How to handle the perm controls
	Perm Perm

	// Server Name, Default is Go Ftp Server
	Name string

	// The hostname that the FTP server should listen on. Optional, defaults to
	// "::", which means all hostnames on ipv4 and ipv6.
	Hostname string

	// Public IP of the server
	PublicIP string

	// Passive ports
	PassivePorts string

	// The port that the FTP should listen on. Optional, defaults to 3000. In
	// a production environment you will probably want to change this to 21.
	Port int

	// use tls, default is false
	TLS bool

	// if tls used, cert file is required
	CertFile string

	// if tls used, key file is required
	KeyFile string

	// If ture TLS is used in RFC4217 mode
	ExplicitFTPS bool

	// If true, client must upgrade to TLS before sending any other command
	ForceTLS bool

	WelcomeMessage string

	// A logger implementation, if nil the StdLogger is used
	Logger Logger

	// Rate Limit per connection bytes per second, 0 means no limit
	RateLimit int64
}

// Server is the root of your FTP application. You should instantiate one
// of these and call ListenAndServe() to start accepting client connections.
//
// Always use the NewServer() method to create a new Server.
type Server struct {
	*Options
	listenTo  string
	logger    Logger
	listener  net.Listener
	tlsConfig *tls.Config
	ctx       context.Context
	cancel    context.CancelFunc
	feats     string
	notifiers notifierList
	// rate limiter per connection
	rateLimiter *ratelimit.Limiter
}

// ErrServerClosed is returned by ListenAndServe() or Serve() when a shutdown
// was requested.
var ErrServerClosed = errors.New("ftp: Server closed")

// optsWithDefaults copies an Options struct into a new struct,
// then adds any default values that are missing and returns the new data.
func optsWithDefaults(opts *Options) *Options {
	var newOpts Options
	if opts == nil {
		opts = &Options{}
	}
	if opts.Hostname == "" {
		newOpts.Hostname = "::"
	} else {
		newOpts.Hostname = opts.Hostname
	}
	if opts.Port == 0 {
		newOpts.Port = 2121
	} else {
		newOpts.Port = opts.Port
	}
	newOpts.Driver = opts.Driver
	if opts.Name == "" {
		newOpts.Name = "Go FTP Server"
	} else {
		newOpts.Name = opts.Name
	}

	if opts.WelcomeMessage == "" {
		newOpts.WelcomeMessage = defaultWelcomeMessage
	} else {
		newOpts.WelcomeMessage = opts.WelcomeMessage
	}

	if opts.Auth != nil {
		newOpts.Auth = opts.Auth
	}

	if opts.Logger != nil {
		newOpts.Logger = opts.Logger
	} else {
		newOpts.Logger = &StdLogger{}
	}

	if opts.Commands == nil {
		newOpts.Commands = defaultCommands
	} else {
		newOpts.Commands = opts.Commands
	}

	newOpts.Perm = opts.Perm
	newOpts.TLS = opts.TLS
	newOpts.KeyFile = opts.KeyFile
	newOpts.CertFile = opts.CertFile
	newOpts.ExplicitFTPS = opts.ExplicitFTPS

	newOpts.PublicIP = opts.PublicIP
	newOpts.PassivePorts = opts.PassivePorts
	newOpts.RateLimit = opts.RateLimit

	return &newOpts
}

// NewServer initialises a new FTP server. Configuration options are provided
// via an instance of Options. Calling this function in your code will
// probably look something like this:
//
//     driver := &MyDriver{}
//     opts    := &server.Options{
//       Driver: driver,
//       Auth: auth,
//       Port: 2000,
//       Perm: perm,
//       Hostname: "127.0.0.1",
//     }
//     server, err  := server.NewServer(opts)
//
func NewServer(opts *Options) (*Server, error) {
	opts = optsWithDefaults(opts)
	if opts.Perm == nil {
		return nil, errors.New("No perm implementation")
	}
	s := new(Server)
	s.Options = opts
	s.listenTo = net.JoinHostPort(opts.Hostname, strconv.Itoa(opts.Port))
	s.logger = opts.Logger

	var (
		feats    = "Extensions supported:\n%s"
		featCmds = " UTF8\n"
	)

	for k, v := range s.Commands {
		if v.IsExtend() {
			featCmds = featCmds + " " + k + "\n"
		}
	}

	if opts.TLS {
		featCmds += " AUTH TLS\n PBSZ\n PROT\n"
	}
	s.feats = fmt.Sprintf(feats, featCmds)
	s.rateLimiter = ratelimit.New(opts.RateLimit)

	return s, nil
}

// RegisterNotifer registers a notifier
func (server *Server) RegisterNotifer(notifier Notifier) {
	server.notifiers = append(server.notifiers, notifier)
}

// NewConn constructs a new object that will handle the FTP protocol over
// an active net.TCPConn. The TCP connection should already be open before
// it is handed to this functions. driver is an instance of FTPDriver that
// will handle all auth and persistence details.
func (server *Server) newSession(id string, tcpConn net.Conn) *Session {
	return &Session{
		id:            id,
		server:        server,
		conn:          tcpConn,
		controlReader: bufio.NewReader(tcpConn),
		controlWriter: bufio.NewWriter(tcpConn),
		curDir:        "/",
		reqUser:       "",
		user:          "",
		renameFrom:    "",
		lastFilePos:   -1,
		closed:        false,
		tls:           false,
		Data:          make(map[string]interface{}),
	}
}

func simpleTLSConfig(certFile, keyFile string) (*tls.Config, error) {
	config := &tls.Config{}
	if config.NextProtos == nil {
		config.NextProtos = []string{"ftp"}
	}

	var err error
	config.Certificates = make([]tls.Certificate, 1)
	config.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return config, nil
}

// ListenAndServe asks a new Server to begin accepting client connections. It
// accepts no arguments - all configuration is provided via the NewServer
// function.
//
// If the server fails to start for any reason, an error will be returned. Common
// errors are trying to bind to a privileged port or something else is already
// listening on the same port.
//
func (server *Server) ListenAndServe() error {
	var listener net.Listener
	var err error

	if server.Options.TLS {
		server.tlsConfig, err = simpleTLSConfig(server.CertFile, server.KeyFile)
		if err != nil {
			return err
		}

		if server.Options.ExplicitFTPS {
			listener, err = net.Listen("tcp", server.listenTo)
		} else {
			listener, err = tls.Listen("tcp", server.listenTo, server.tlsConfig)
		}
	} else {
		listener, err = net.Listen("tcp", server.listenTo)
	}
	if err != nil {
		return err
	}

	server.logger.Printf("", "%s listening on %d", server.Name, server.Port)

	return server.Serve(listener)
}

// Serve accepts connections on a given net.Listener and handles each
// request in a new goroutine.
//
func (server *Server) Serve(l net.Listener) error {
	server.listener = l
	server.ctx, server.cancel = context.WithCancel(context.Background())
	defer server.cancel()
	sessionID := newSessionID()
	for {
		tcpConn, err := server.listener.Accept()
		if err != nil {
			select {
			case <-server.ctx.Done():
				return ErrServerClosed
			default:
			}
			server.logger.Printf(sessionID, "listening error: %v", err)
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				continue
			}
			return err
		}

		ftpConn := server.newSession(sessionID, tcpConn)
		go ftpConn.Serve()
	}
}

// Shutdown will gracefully stop a server. Already connected clients will retain their connections
func (server *Server) Shutdown() error {
	if server.cancel != nil {
		server.cancel()
	}
	if server.listener != nil {
		return server.listener.Close()
	}
	// server wasnt even started
	return nil
}
