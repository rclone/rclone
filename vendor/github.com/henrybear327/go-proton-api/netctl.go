package proton

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

// Listener wraps a net.Listener.
// It can be configured to spawn connections that drop all reads or writes.
type Listener struct {
	net.Listener

	canRead bool
	rlock   sync.RWMutex

	canWrite bool
	wlock    sync.RWMutex

	conns    []net.Conn
	connLock sync.RWMutex

	done     chan struct{}
	doneOnce sync.Once

	newConn func(net.Conn, *Listener) net.Conn
}

// NewListener returns a new DropListener.
func NewListener(l net.Listener, newConn func(net.Conn, *Listener) net.Conn) *Listener {
	return &Listener{
		Listener: l,
		canRead:  true,
		canWrite: true,
		done:     make(chan struct{}),
		newConn:  newConn,
	}
}

func (l *Listener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}

	l.connLock.Lock()
	defer l.connLock.Unlock()

	dropConn := l.newConn(conn, l)

	l.conns = append(l.conns, dropConn)

	return dropConn, nil
}

// SetCanRead sets whether the connections spawned by this listener can read.
func (l *Listener) SetCanRead(canRead bool) {
	l.rlock.Lock()
	defer l.rlock.Unlock()

	l.canRead = canRead
}

// SetCanWrite sets whether the connections spawned by this listener can write.
func (l *Listener) SetCanWrite(canWrite bool) {
	l.wlock.Lock()
	defer l.wlock.Unlock()

	l.canWrite = canWrite
}

// Close closes the listener.
func (l *Listener) Close() error {
	defer l.doneOnce.Do(func() {
		close(l.done)
	})

	return l.Listener.Close()
}

// Done returns a channel that is closed when the listener is closed.
func (l *Listener) Done() <-chan struct{} {
	return l.done
}

// DropAll closes all connections spawned by this listener.
func (l *Listener) DropAll() {
	l.connLock.RLock()
	defer l.connLock.RUnlock()

	for _, conn := range l.conns {
		_ = conn.Close()
	}
}

type hangConn struct {
	net.Conn

	l *Listener
}

func NewHangConn(c net.Conn, l *Listener) net.Conn {
	return &hangConn{
		Conn: c,
		l:    l,
	}
}

func (c *hangConn) Read(b []byte) (int, error) {
	c.l.rlock.RLock()
	defer c.l.rlock.RUnlock()

	if !c.l.canRead {
		c.l.rlock.RUnlock()
		<-c.l.Done()
		c.l.rlock.RLock()
	}

	return c.Conn.Read(b)
}

func (c *hangConn) Write(b []byte) (int, error) {
	c.l.wlock.RLock()
	defer c.l.wlock.RUnlock()

	if !c.l.canWrite {
		c.l.wlock.RUnlock()
		<-c.l.Done()
		c.l.wlock.RLock()
	}

	return c.Conn.Write(b)
}

type dropConn struct {
	net.Conn

	l *Listener
}

func NewDropConn(c net.Conn, l *Listener) net.Conn {
	return &dropConn{
		Conn: c,
		l:    l,
	}
}

func (c *dropConn) Read(b []byte) (int, error) {
	c.l.rlock.RLock()
	defer c.l.rlock.RUnlock()

	if c.l.canRead {
		return c.Conn.Read(b)
	}

	// Read half the length of the buffer.
	n, err := c.Conn.Read(b[:len(b)/2])
	if err != nil {
		return n, fmt.Errorf("read: %w", err)
	}

	if err := c.Close(); err != nil {
		return n, fmt.Errorf("close: %w", err)
	}

	return n, errors.New("read: connection closed")
}

func (c *dropConn) Write(b []byte) (int, error) {
	c.l.wlock.RLock()
	defer c.l.wlock.RUnlock()

	if c.l.canWrite {
		return c.Conn.Write(b)
	}

	// Write half the length of the buffer.
	n, err := c.Conn.Write(b[:len(b)/2])
	if err != nil {
		return n, fmt.Errorf("write: %w", err)
	}

	if err := c.Close(); err != nil {
		return n, fmt.Errorf("close: %w", err)
	}

	return n, errors.New("write: connection closed")
}

func (c *dropConn) Close() error {
	if tcpConn, ok := c.Conn.(*net.TCPConn); ok {
		if err := tcpConn.SetLinger(0); err != nil {
			return err
		}
	}

	return c.Conn.Close()
}

// InsecureTransport returns an http.Transport with InsecureSkipVerify set to true.
func InsecureTransport() *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
}

// ctl can be used to control whether a dialer can dial, and whether the resulting
// connection can read or write.
type NetCtl struct {
	canDial   bool
	dialLimit uint64
	dialCount uint64
	onDial    []func(net.Conn)
	dlock     sync.RWMutex

	canRead   bool
	readLimit uint64
	readCount uint64
	readSpeed int
	onRead    []func([]byte)
	rlock     sync.RWMutex

	canWrite   bool
	writeLimit uint64
	writeCount uint64
	writeSpeed int
	onWrite    []func([]byte)
	wlock      sync.RWMutex

	conns []net.Conn
}

// NewNetCtl returns a new ctl with all fields set to true.
func NewNetCtl() *NetCtl {
	return &NetCtl{
		canDial:  true,
		canRead:  true,
		canWrite: true,
	}
}

// SetCanDial sets whether the dialer can dial.
func (c *NetCtl) SetCanDial(canDial bool) {
	c.dlock.Lock()
	defer c.dlock.Unlock()

	c.canDial = canDial
}

// SetDialLimit sets the maximum number of times dialers using this controller can dial.
func (c *NetCtl) SetDialLimit(limit uint64) {
	c.dlock.Lock()
	defer c.dlock.Unlock()

	c.dialLimit = limit
}

// SetCanRead sets whether the connection can read.
func (c *NetCtl) SetCanRead(canRead bool) {
	c.dlock.Lock()
	defer c.dlock.Unlock()

	for _, conn := range c.conns {
		conn.Close()
	}

	c.rlock.Lock()
	defer c.rlock.Unlock()

	c.canRead = canRead
}

// SetReadLimit sets the maximum number of bytes that can be read.
func (c *NetCtl) SetReadLimit(limit uint64) {
	c.dlock.Lock()
	defer c.dlock.Unlock()

	for _, conn := range c.conns {
		conn.Close()
	}

	c.rlock.Lock()
	defer c.rlock.Unlock()

	c.readLimit = limit
	c.readCount = 0
}

// SetReadSpeed sets the maximum number of bytes that can be read per second.
func (c *NetCtl) SetReadSpeed(speed int) {
	c.dlock.Lock()
	defer c.dlock.Unlock()

	for _, conn := range c.conns {
		conn.Close()
	}

	c.rlock.Lock()
	defer c.rlock.Unlock()

	c.readSpeed = speed
}

// SetCanWrite sets whether the connection can write.
func (c *NetCtl) SetCanWrite(canWrite bool) {
	c.dlock.Lock()
	defer c.dlock.Unlock()

	for _, conn := range c.conns {
		conn.Close()
	}

	c.wlock.Lock()
	defer c.wlock.Unlock()

	c.canWrite = canWrite
}

// SetWriteLimit sets the maximum number of bytes that can be written.
func (c *NetCtl) SetWriteLimit(limit uint64) {
	c.dlock.Lock()
	defer c.dlock.Unlock()

	for _, conn := range c.conns {
		conn.Close()
	}

	c.wlock.Lock()
	defer c.wlock.Unlock()

	c.writeLimit = limit
	c.writeCount = 0
}

// SetWriteSpeed sets the maximum number of bytes that can be written per second.
func (c *NetCtl) SetWriteSpeed(speed int) {
	c.dlock.Lock()
	defer c.dlock.Unlock()

	for _, conn := range c.conns {
		conn.Close()
	}

	c.wlock.Lock()
	defer c.wlock.Unlock()

	c.writeSpeed = speed
}

// OnDial adds a callback that is called with the created connection when a dial is successful.
func (c *NetCtl) OnDial(f func(net.Conn)) {
	c.dlock.Lock()
	defer c.dlock.Unlock()

	c.onDial = append(c.onDial, f)
}

// OnRead adds a callback that is called with the read bytes when a read is successful.
func (c *NetCtl) OnRead(fn func([]byte)) {
	c.rlock.Lock()
	defer c.rlock.Unlock()

	c.onRead = append(c.onRead, fn)
}

// OnWrite adds a callback that is called with the written bytes when a write is successful.
func (c *NetCtl) OnWrite(fn func([]byte)) {
	c.wlock.Lock()
	defer c.wlock.Unlock()

	c.onWrite = append(c.onWrite, fn)
}

// Disable is equivalent to disallowing dial, read and write.
func (c *NetCtl) Disable() {
	c.SetCanDial(false)
	c.SetCanRead(false)
	c.SetCanWrite(false)
}

// Enable is equivalent to allowing dial, read and write.
func (c *NetCtl) Enable() {
	c.SetCanDial(true)
	c.SetCanRead(true)
	c.SetCanWrite(true)
}

// NewDialer returns a new dialer controlled by the ctl.
func (c *NetCtl) NewRoundTripper(tlsConfig *tls.Config) http.RoundTripper {
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return c.dial(ctx, &net.Dialer{}, network, addr)
		},
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return c.dial(ctx, &tls.Dialer{Config: tlsConfig}, network, addr)
		},
		TLSClientConfig:       tlsConfig,
		ResponseHeaderTimeout: time.Second,
		ExpectContinueTimeout: time.Second,
	}
}

// ctxDialer implements DialContext.
type ctxDialer interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// dial dials using d, but only if the controller allows it.
func (c *NetCtl) dial(ctx context.Context, dialer ctxDialer, network, addr string) (net.Conn, error) {
	c.dlock.Lock()
	defer c.dlock.Unlock()

	if !c.canDial {
		return nil, errors.New("dial failed (not allowed)")
	}

	if c.dialLimit > 0 && c.dialCount >= c.dialLimit {
		return nil, errors.New("dial failed (limit reached)")
	}

	conn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	c.dialCount++

	for _, fn := range c.onDial {
		fn(conn)
	}

	c.conns = append(c.conns, conn)

	return newConn(conn, c), nil
}

// read reads from r, but only if the controller allows it.
func (c *NetCtl) read(r io.Reader, b []byte) (int, error) {
	c.rlock.Lock()
	defer c.rlock.Unlock()

	if !c.canRead {
		return 0, errors.New("read failed (not allowed)")
	}

	if c.readLimit > 0 && c.readCount >= c.readLimit {
		return 0, errors.New("read failed (limit reached)")
	}

	var rem uint64

	if c.readLimit > 0 && c.readLimit-c.readCount < uint64(len(b)) {
		rem = c.readLimit - c.readCount
	} else {
		rem = uint64(len(b))
	}

	c.rlock.Unlock()
	n, err := newSlowReader(r, c.readSpeed).Read(b[:rem])
	c.rlock.Lock()

	c.readCount += uint64(n)

	for _, fn := range c.onRead {
		fn(b[:n])
	}

	return n, err
}

// write writes to w, but only if the controller allows it.
func (c *NetCtl) write(w io.Writer, b []byte) (int, error) {
	c.wlock.Lock()
	defer c.wlock.Unlock()

	if !c.canWrite {
		return 0, errors.New("write failed (not allowed)")
	}

	if c.writeLimit > 0 && c.writeCount >= c.writeLimit {
		return 0, errors.New("write failed (limit exceeded)")
	}

	var rem uint64

	if c.writeLimit > 0 && c.writeLimit-c.writeCount < uint64(len(b)) {
		rem = c.writeLimit - c.writeCount
	} else {
		rem = uint64(len(b))
	}

	c.wlock.Unlock()
	n, err := newSlowWriter(w, c.writeSpeed).Write(b[:rem])
	c.wlock.Lock()

	c.writeCount += uint64(n)

	for _, fn := range c.onWrite {
		fn(b[:n])
	}

	if uint64(n) < rem {
		return n, fmt.Errorf("write incomplete (limit reached)")
	}

	return n, err
}

// conn is a wrapper around net.conn that can be used to control whether a connection can read or write.
type conn struct {
	net.Conn

	ctl *NetCtl
}

func newConn(c net.Conn, ctl *NetCtl) *conn {
	return &conn{
		Conn: c,
		ctl:  ctl,
	}
}

// Read reads from the wrapped connection, but only if the controller allows it.
func (c *conn) Read(b []byte) (int, error) {
	return c.ctl.read(c.Conn, b)
}

// Write writes to the wrapped connection, but only if the controller allows it.
func (c *conn) Write(b []byte) (int, error) {
	return c.ctl.write(c.Conn, b)
}

// slowReader is an io.Reader that reads at a fixed rate.
type slowReader struct {
	r io.Reader

	// bytesPerSec is the number of bytes to read per second.
	bytesPerSec int
}

func newSlowReader(r io.Reader, bytesPerSec int) *slowReader {
	return &slowReader{
		r:           r,
		bytesPerSec: bytesPerSec,
	}
}

func (r *slowReader) Read(b []byte) (int, error) {
	start := time.Now()

	n, err := r.r.Read(b)

	if r.bytesPerSec > 0 {
		time.Sleep(time.Until(start.Add(time.Duration(n*r.bytesPerSec) * time.Second)))
	}

	return n, err
}

// slowWriter is an io.Writer that writes at a fixed rate.
type slowWriter struct {
	w io.Writer

	// bytesPerSec is the number of bytes to write per second.
	bytesPerSec int
}

func newSlowWriter(w io.Writer, bytesPerSec int) *slowWriter {
	return &slowWriter{
		w:           w,
		bytesPerSec: bytesPerSec,
	}
}

func (w *slowWriter) Write(b []byte) (int, error) {
	start := time.Now()

	n, err := w.w.Write(b)

	if w.bytesPerSec > 0 {
		time.Sleep(time.Until(start.Add(time.Duration(n*w.bytesPerSec) * time.Second)))
	}

	return n, err
}
