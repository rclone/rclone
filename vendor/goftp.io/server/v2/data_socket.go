// Copyright 2018 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package server

import (
	"crypto/tls"
	"io"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"goftp.io/server/v2/ratelimit"
)

// DataSocket describes a data socket is used to send non-control data between the client and
// server.
type DataSocket interface {
	Host() string

	Port() int

	// the standard io.Reader interface
	Read(p []byte) (n int, err error)

	// the standard io.ReaderFrom interface
	ReadFrom(r io.Reader) (int64, error)

	// the standard io.Writer interface
	Write(p []byte) (n int, err error)

	// the standard io.Closer interface
	Close() error
}

type activeSocket struct {
	conn *net.TCPConn
	reader io.Reader
	writer io.Writer
	sess *Session
	host string
	port int
}

func newActiveSocket(sess *Session, remote string, port int) (DataSocket, error) {
	connectTo := net.JoinHostPort(remote, strconv.Itoa(port))

	sess.log("Opening active data connection to " + connectTo)

	raddr, err := net.ResolveTCPAddr("tcp", connectTo)

	if err != nil {
		sess.log(err)
		return nil, err
	}

	tcpConn, err := net.DialTCP("tcp", nil, raddr)

	if err != nil {
		sess.log(err)
		return nil, err
	}

	socket := new(activeSocket)
	socket.sess = sess
	socket.conn = tcpConn
	socket.reader = ratelimit.Reader(tcpConn, sess.server.rateLimiter)
	socket.writer = ratelimit.Writer(tcpConn, sess.server.rateLimiter)
	socket.host = remote
	socket.port = port

	return socket, nil
}

func (socket *activeSocket) Host() string {
	return socket.host
}

func (socket *activeSocket) Port() int {
	return socket.port
}

func (socket *activeSocket) Read(p []byte) (n int, err error) {
	return socket.reader.Read(p)
}

func (socket *activeSocket) ReadFrom(r io.Reader) (int64, error) {
	return io.Copy(socket.writer, r)
}

func (socket *activeSocket) Write(p []byte) (n int, err error) {
	return socket.writer.Write(p)
}

func (socket *activeSocket) Close() error {
	return socket.conn.Close()
}

type passiveSocket struct {
	sess    *Session
	conn    net.Conn
	reader io.Reader
	writer io.Writer
	port    int
	host    string
	ingress chan []byte
	egress  chan []byte
	lock    sync.Mutex // protects conn and err
	err     error
}

// Detect if an error is "bind: address already in use"
//
// Originally from https://stackoverflow.com/a/52152912/164234
func isErrorAddressAlreadyInUse(err error) bool {
	errOpError, ok := err.(*net.OpError)
	if !ok {
		return false
	}
	errSyscallError, ok := errOpError.Err.(*os.SyscallError)
	if !ok {
		return false
	}
	errErrno, ok := errSyscallError.Err.(syscall.Errno)
	if !ok {
		return false
	}
	if errErrno == syscall.EADDRINUSE {
		return true
	}
	const WSAEADDRINUSE = 10048
	if runtime.GOOS == "windows" && errErrno == WSAEADDRINUSE {
		return true
	}
	return false
}

func (sess *Session) newPassiveSocket() (DataSocket, error) {
	socket := new(passiveSocket)
	socket.ingress = make(chan []byte)
	socket.egress = make(chan []byte)
	socket.sess = sess
	socket.host = sess.passiveListenIP()

	const retries = 10
	var err error
	for i := 1; i <= retries; i++ {
		socket.port = sess.PassivePort()
		err = socket.ListenAndServe()
		if err != nil && socket.port != 0 && isErrorAddressAlreadyInUse(err) {
			// choose a different port on error already in use
			continue
		}
		break
	}
	sess.dataConn = socket
	return socket, err
}

func (socket *passiveSocket) Host() string {
	return socket.host
}

func (socket *passiveSocket) Port() int {
	return socket.port
}

func (socket *passiveSocket) Read(p []byte) (n int, err error) {
	socket.lock.Lock()
	defer socket.lock.Unlock()
	if socket.err != nil {
		return 0, socket.err
	}
	return socket.reader.Read(p)
}

func (socket *passiveSocket) ReadFrom(r io.Reader) (int64, error) {
	socket.lock.Lock()
	defer socket.lock.Unlock()
	if socket.err != nil {
		return 0, socket.err
	}

	// For normal TCPConn, this will use sendfile syscall; if not,
	// it will just downgrade to normal read/write procedure
	return io.Copy(socket.writer, r)
}

func (socket *passiveSocket) Write(p []byte) (n int, err error) {
	socket.lock.Lock()
	defer socket.lock.Unlock()
	if socket.err != nil {
		return 0, socket.err
	}
	return socket.writer.Write(p)
}

func (socket *passiveSocket) Close() error {
	socket.lock.Lock()
	defer socket.lock.Unlock()
	if socket.conn != nil {
		return socket.conn.Close()
	}
	return nil
}

func (socket *passiveSocket) ListenAndServe() (err error) {
	laddr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort("", strconv.Itoa(socket.port)))
	if err != nil {
		socket.sess.log(err)
		return err
	}

	var tcplistener *net.TCPListener
	tcplistener, err = net.ListenTCP("tcp", laddr)
	if err != nil {
		socket.sess.log(err)
		return err
	}

	// The timeout, for a remote client to establish connection
	// with a PASV style data connection.
	const acceptTimeout = 60 * time.Second
	err = tcplistener.SetDeadline(time.Now().Add(acceptTimeout))
	if err != nil {
		socket.sess.log(err)
		return err
	}

	var listener net.Listener = tcplistener
	add := listener.Addr()
	parts := strings.Split(add.String(), ":")
	port, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		socket.sess.log(err)
		return err
	}

	socket.port = port
	if socket.sess.server.tlsConfig != nil {
		listener = tls.NewListener(listener, socket.sess.server.tlsConfig)
	}

	socket.lock.Lock()
	go func() {
		defer socket.lock.Unlock()

		conn, err := listener.Accept()
		if err != nil {
			socket.err = err
			return
		}
		socket.err = nil
		socket.conn = conn
		socket.reader = ratelimit.Reader(socket.conn, socket.sess.server.rateLimiter)
		socket.writer = ratelimit.Writer(socket.conn, socket.sess.server.rateLimiter)
		_ = listener.Close()
	}()
	return nil
}
