//go:build !plan9

package sftp

import "io"

// Interfaces for ssh client and session implemented in ssh_internal.go and ssh_external.go

// An interface for an ssh client to abstract over internal ssh library and external binary
type sshClient interface {
	// Wait blocks until the connection has shut down, and returns the
	// error causing the shutdown.
	Wait() error

	// SendKeepAlive sends a keepalive message to keep the connection open
	SendKeepAlive()

	// Close the connection
	Close() error

	// NewSession opens a new sshSession for this sshClient. (A
	// session is a remote execution of a program.)
	NewSession() (sshSession, error)

	// CanReuse indicates if this client can be reused
	CanReuse() bool
}

// An interface for an ssh session to abstract over internal ssh library and external binary
type sshSession interface {
	// Setenv sets an environment variable that will be applied to any
	// command executed by Shell or Run.
	Setenv(name, value string) error

	// Start runs cmd on the remote host. Typically, the remote
	// server passes cmd to the shell for interpretation.
	// A Session only accepts one call to Run, Start or Shell.
	Start(cmd string) error

	// StdinPipe returns a pipe that will be connected to the
	// remote command's standard input when the command starts.
	StdinPipe() (io.WriteCloser, error)

	// StdoutPipe returns a pipe that will be connected to the
	// remote command's standard output when the command starts.
	// There is a fixed amount of buffering that is shared between
	// stdout and stderr streams. If the StdoutPipe reader is
	// not serviced fast enough it may eventually cause the
	// remote command to block.
	StdoutPipe() (io.Reader, error)

	// RequestSubsystem requests the association of a subsystem
	// with the session on the remote host. A subsystem is a
	// predefined command that runs in the background when the ssh
	// session is initiated
	RequestSubsystem(subsystem string) error

	// Run runs cmd on the remote host. Typically, the remote
	// server passes cmd to the shell for interpretation.
	// A Session only accepts one call to Run, Start, Shell, Output,
	// or CombinedOutput.
	Run(cmd string) error

	// Close the session
	Close() error

	// Set the stdout
	SetStdout(io.Writer)

	// Set the stderr
	SetStderr(io.Writer)
}
