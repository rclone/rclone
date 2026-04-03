// Package testserver starts and stops test servers if required
package testserver

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fspath"
)

var (
	findConfigOnce sync.Once
	configDir      string // where the config is stored
)

// Assume we are run somewhere within the rclone root
func findConfig() (string, error) {
	dir := filepath.Join("fstest", "testserver", "init.d")
	for range 5 {
		fi, err := os.Stat(dir)
		if err == nil && fi.IsDir() {
			return filepath.Abs(dir)
		} else if !os.IsNotExist(err) {
			return "", err
		}
		dir = filepath.Join("..", dir)
	}
	return "", errors.New("couldn't find testserver config files - run from within rclone source")
}

// returns path to a script to start this server
func cmdPath(name string) string {
	return filepath.Join(configDir, name)
}

// return true if the server with name has a start command
func hasStartCommand(name string) bool {
	fi, err := os.Stat(cmdPath(name))
	return err == nil && !fi.IsDir()
}

// run the command returning the output and an error
func run(name, command string) (out []byte, err error) {
	script := cmdPath(name)
	cmd := exec.Command(script, command)
	out, err = cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("failed to run %s %s\n%s: %w", script, command, string(out), err)
	}
	return out, err
}

// envKey returns the environment variable name to set name, key
func envKey(name, key string) string {
	return fmt.Sprintf("RCLONE_CONFIG_%s_%s", strings.ToUpper(name), strings.ToUpper(key))
}

// match a line of config var=value
var matchLine = regexp.MustCompile(`^([a-zA-Z_]+)=(.*)$`)

// Start the server and env vars so rclone can use it
func start(name string) error {
	fs.Logf(name, "Starting server")
	out, err := run(name, "start")
	if err != nil {
		return err
	}
	// parse the output and set environment vars from it
	var connect string
	var connectDelay time.Duration
	for line := range bytes.SplitSeq(out, []byte("\n")) {
		line = bytes.TrimSpace(line)
		part := matchLine.FindSubmatch(line)
		if part != nil {
			key, value := part[1], part[2]
			if string(key) == "_connect" {
				connect = string(value)
				continue
			} else if string(key) == "_connect_delay" {
				connectDelay, err = time.ParseDuration(string(value))
				if err != nil {
					return fmt.Errorf("bad _connect_delay: %w", err)
				}
				continue
			}

			// fs.Debugf(name, "key = %q, envKey = %q, value = %q", key, envKey(name, string(key)), value)
			err = os.Setenv(envKey(name, string(key)), string(value))
			if err != nil {
				return err
			}
		}
	}
	if connect == "" {
		fs.Logf(name, "Started server")
		return nil
	}
	// If we got a _connect value then try to connect to it
	const maxTries = 100
	var rdBuf = make([]byte, 1)
	for i := 1; i <= maxTries; i++ {
		if i != 0 {
			time.Sleep(time.Second)
		}
		fs.Logf(name, "Attempting to connect to %q try %d/%d", connect, i, maxTries)
		conn, err := net.DialTimeout("tcp", connect, time.Second)
		if err != nil {
			fs.Debugf(name, "Connection to %q failed try %d/%d: %v", connect, i, maxTries, err)
			continue
		}

		err = conn.SetReadDeadline(time.Now().Add(time.Second))
		if err != nil {
			return fmt.Errorf("failed to set deadline: %w", err)
		}
		n, err := conn.Read(rdBuf)
		_ = conn.Close()
		fs.Debugf(name, "Read %d, error: %v", n, err)
		if err != nil && !errors.Is(err, os.ErrDeadlineExceeded) {
			// Try again
			continue
		}
		if connectDelay > 0 {
			fs.Logf(name, "Connect delay %v", connectDelay)
			time.Sleep(connectDelay)
		}
		fs.Logf(name, "Started server and connected to %q", connect)
		return nil
	}
	return fmt.Errorf("failed to connect to %q on %q", name, connect)
}

// Stops the named test server
func stop(name string) {
	fs.Logf(name, "Stopping server")
	_, err := run(name, "stop")
	if err != nil {
		fs.Errorf(name, "Failed to stop server: %v", err)
	}
}

// No server to stop so do nothing
func stopNothing() {
}

// Start starts the test server for remoteName.
//
// This must be stopped by calling the function returned when finished.
func Start(remote string) (fn func(), err error) {
	// don't start the local backend
	if remote == "" {
		return stopNothing, nil
	}
	parsed, err := fspath.Parse(remote)
	if err != nil {
		return nil, err
	}
	name := parsed.ConfigString
	// don't start the local backend
	if name == "" {
		return stopNothing, nil
	}

	// Make sure we know where the config is
	findConfigOnce.Do(func() {
		configDir, err = findConfig()
	})
	if err != nil {
		return nil, err
	}

	// If remote has no start command then do nothing
	if !hasStartCommand(name) {
		return stopNothing, nil
	}

	// Start the server
	err = start(name)
	if err != nil {
		return nil, err
	}

	// And return a function to stop it
	return func() {
		stop(name)
	}, nil

}
