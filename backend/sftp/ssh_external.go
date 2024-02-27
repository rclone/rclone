//go:build !plan9
// +build !plan9

package sftp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
)

// Implement the sshClient interface for external ssh programs
type sshClientExternal struct {
	f       *Fs
	session *sshSessionExternal
}

func (f *Fs) newSSHClientExternal() (sshClient, error) {
	return &sshClientExternal{f: f}, nil
}

// Wait for connection to close
func (s *sshClientExternal) Wait() error {
	if s.session == nil {
		return nil
	}
	return s.session.Wait()
}

// Send a keepalive over the ssh connection
func (s *sshClientExternal) SendKeepAlive() {
	// Up to the user to configure -o ServerAliveInterval=20 on their ssh connections
}

// Close the connection
func (s *sshClientExternal) Close() error {
	if s.session == nil {
		return nil
	}
	return s.session.Close()
}

// NewSession makes a new external SSH connection
func (s *sshClientExternal) NewSession() (sshSession, error) {
	session := s.f.newSSHSessionExternal()
	if s.session == nil {
		fs.Debugf(s.f, "ssh external: creating additional session")
	}
	return session, nil
}

// CanReuse indicates if this client can be reused
func (s *sshClientExternal) CanReuse() bool {
	if s.session == nil {
		return true
	}
	exited := s.session.exited()
	canReuse := !exited && s.session.runningSFTP
	// fs.Debugf(s.f, "ssh external: CanReuse %v, exited=%v runningSFTP=%v", canReuse, exited, s.session.runningSFTP)
	return canReuse
}

// Check interfaces
var _ sshClient = &sshClientExternal{}

// implement the sshSession interface for external ssh binary
type sshSessionExternal struct {
	f           *Fs
	cmd         *exec.Cmd
	cancel      func()
	startCalled bool
	runningSFTP bool
}

func (f *Fs) newSSHSessionExternal() *sshSessionExternal {
	s := &sshSessionExternal{
		f: f,
	}

	// Make a cancellation function for this to call in Close()
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	// Connect to a remote host and request the sftp subsystem via
	// the 'ssh' command. This assumes that passwordless login is
	// correctly configured.
	ssh := append([]string(nil), s.f.opt.SSH...)
	s.cmd = exec.CommandContext(ctx, ssh[0], ssh[1:]...)

	// Allow the command a short time only to shut down
	s.cmd.WaitDelay = time.Second

	return s
}

// Setenv sets an environment variable that will be applied to any
// command executed by Shell or Run.
func (s *sshSessionExternal) Setenv(name, value string) error {
	return errors.New("ssh external: can't set environment variables")
}

const requestSubsystem = "***Subsystem***:"

// Start runs cmd on the remote host. Typically, the remote
// server passes cmd to the shell for interpretation.
// A Session only accepts one call to Run, Start or Shell.
func (s *sshSessionExternal) Start(cmd string) error {
	if s.startCalled {
		return errors.New("internal error: ssh external: command already running")
	}
	s.startCalled = true

	// Adjust the args
	if strings.HasPrefix(cmd, requestSubsystem) {
		s.cmd.Args = append(s.cmd.Args, "-s", cmd[len(requestSubsystem):])
		s.runningSFTP = true
	} else {
		s.cmd.Args = append(s.cmd.Args, cmd)
		s.runningSFTP = false
	}

	fs.Debugf(s.f, "ssh external: running: %v", fs.SpaceSepList(s.cmd.Args))

	// start the process
	err := s.cmd.Start()
	if err != nil {
		return fmt.Errorf("ssh external: start process: %w", err)
	}

	return nil
}

// RequestSubsystem requests the association of a subsystem
// with the session on the remote host. A subsystem is a
// predefined command that runs in the background when the ssh
// session is initiated
func (s *sshSessionExternal) RequestSubsystem(subsystem string) error {
	return s.Start(requestSubsystem + subsystem)
}

// StdinPipe returns a pipe that will be connected to the
// remote command's standard input when the command starts.
func (s *sshSessionExternal) StdinPipe() (io.WriteCloser, error) {
	rd, err := s.cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("ssh external: stdin pipe: %w", err)
	}
	return rd, nil
}

// StdoutPipe returns a pipe that will be connected to the
// remote command's standard output when the command starts.
// There is a fixed amount of buffering that is shared between
// stdout and stderr streams. If the StdoutPipe reader is
// not serviced fast enough it may eventually cause the
// remote command to block.
func (s *sshSessionExternal) StdoutPipe() (io.Reader, error) {
	wr, err := s.cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("ssh external: stdout pipe: %w", err)
	}
	return wr, nil
}

// Return whether the command has finished or not
func (s *sshSessionExternal) exited() bool {
	return s.cmd.ProcessState != nil
}

// Wait for the command to exit
func (s *sshSessionExternal) Wait() error {
	if s.exited() {
		return nil
	}
	err := s.cmd.Wait()
	if err == nil {
		fs.Debugf(s.f, "ssh external: command exited OK")
	} else {
		fs.Debugf(s.f, "ssh external: command exited with error: %v", err)
	}
	return err
}

// Run runs cmd on the remote host. Typically, the remote
// server passes cmd to the shell for interpretation.
// A Session only accepts one call to Run, Start, Shell, Output,
// or CombinedOutput.
func (s *sshSessionExternal) Run(cmd string) error {
	err := s.Start(cmd)
	if err != nil {
		return err
	}
	return s.Wait()
}

// Close the external ssh
func (s *sshSessionExternal) Close() error {
	fs.Debugf(s.f, "ssh external: close")
	// Cancel the context which kills the process
	s.cancel()
	// Wait for it to finish
	_ = s.Wait()
	return nil
}

// Set the stdout
func (s *sshSessionExternal) SetStdout(wr io.Writer) {
	s.cmd.Stdout = wr
}

// Set the stderr
func (s *sshSessionExternal) SetStderr(wr io.Writer) {
	s.cmd.Stderr = wr
}

// Check interfaces
var _ sshSession = &sshSessionExternal{}
