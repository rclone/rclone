package adb

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/thinkhy/go-adb/internal/errors"
	"github.com/thinkhy/go-adb/wire"
)

const (
	AdbExecutableName = "adb"

	// Default port the adb server listens on.
	AdbPort = 5037
)

type ServerConfig struct {
	// Path to the adb executable. If empty, the PATH environment variable will be searched.
	PathToAdb string

	// Host and port the adb server is listening on.
	// If not specified, will use the default port on localhost.
	Host string
	Port int

	// Dialer used to connect to the adb server.
	Dialer

	fs *filesystem
}

// Server knows how to start the adb server and connect to it.
type server interface {
	Start() error
	Dial() (*wire.Conn, error)
}

func roundTripSingleResponse(s server, req string) ([]byte, error) {
	conn, err := s.Dial()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	return conn.RoundTripSingleResponse([]byte(req))
}

func roundTripSingleNoResponse(s server, req string) error {
	conn, err := s.Dial()
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.RoundTripSingleNoResponse([]byte(req))
}

type realServer struct {
	config ServerConfig

	// Caches Host:Port so they don't have to be concatenated for every dial.
	address string
}

func newServer(config ServerConfig) (server, error) {
	if config.Dialer == nil {
		config.Dialer = tcpDialer{}
	}

	if config.Host == "" {
		// localhost becames very slow on windows(1s dial delay), use 127.0.0.1 works well
		config.Host = "127.0.0.1"
	}
	if config.Port == 0 {
		config.Port = AdbPort
	}

	if config.fs == nil {
		config.fs = localFilesystem
	}

	if config.PathToAdb == "" {
		path, err := config.fs.LookPath(AdbExecutableName)
		if err != nil {
			return nil, errors.WrapErrorf(err, errors.ServerNotAvailable, "could not find %s in PATH", AdbExecutableName)
		}
		config.PathToAdb = path
	}

	return &realServer{
		config:  config,
		address: fmt.Sprintf("%s:%d", config.Host, config.Port),
	}, nil
}

// Dial tries to connect to the server. If the first attempt fails, tries starting the server before
// retrying. If the second attempt fails, returns the error.
func (s *realServer) Dial() (*wire.Conn, error) {
	conn, err := s.config.Dial(s.address)
	if err != nil {
		// Attempt to start the server and try again.
		if err = s.Start(); err != nil {
			return nil, errors.WrapErrorf(err, errors.ServerNotAvailable, "error starting server for dial")
		}

		conn, err = s.config.Dial(s.address)
		if err != nil {
			return nil, err
		}
	}
	return conn, nil
}

// StartServer ensures there is a server running.
func (s *realServer) Start() error {
	output, err := s.config.fs.CmdCombinedOutput(s.config.PathToAdb, "start-server")
	outputStr := strings.TrimSpace(string(output))
	return errors.WrapErrorf(err, errors.ServerNotAvailable, "error starting server: %s\noutput:\n%s", err, outputStr)
}

// filesystem abstracts interactions with the local filesystem for testability.
type filesystem struct {
	// Wraps exec.LookPath.
	LookPath func(string) (string, error)

	// Wraps exec.Command().CombinedOutput()
	CmdCombinedOutput func(name string, arg ...string) ([]byte, error)
}

var localFilesystem = &filesystem{
	LookPath: exec.LookPath,
	CmdCombinedOutput: func(name string, arg ...string) ([]byte, error) {
		return exec.Command(name, arg...).CombinedOutput()
	},
}
