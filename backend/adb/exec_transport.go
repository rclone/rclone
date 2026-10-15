package adb

// exec_transport.go provides a custom exec: service transport for the ADB
// backend.
//
// gadb dials only the shell: service. The exec: service bypasses PTY
// processing and returns binary-clean stdout. B4dM4n measured 50 KiB/s on
// shell: vs 5 MiB/s on exec: for file reads. This file implements exec: by
// dialing the ADB server TCP socket directly.
//
// Protocol (both frames use the same 4-byte hex length + payload shape):
//   1. TCP connect to ADB server at host:port.
//   2. Send host:transport:<serial> framed as %04x%s.
//   3. Read 4-byte response: OKAY or FAIL.
//   4. Send exec:<command> framed the same way.
//   5. Read 4-byte response: OKAY or FAIL.
//   6. Remaining bytes from the connection are raw stdout. Close = EOF.
//
// Path handling notes:
//   - Paths with spaces: the shell command uses shell variable expansion so
//     dd receives the path without word-splitting.
//   - Paths with single-quotes: escaped by the caller before Dial is invoked.
//   - Paths with newlines: NOT safe. Document only; no code change needed.
//   - UTF-8 paths pass through as bytes; ADB handles them on modern Android.
//   - If dd is missing on the device (stripped ROMs): exec: returns OKAY but
//     stdout is empty. adbReader promotes the short read to ErrUnexpectedEOF.
//   - Device disconnect mid-read: conn.Read returns an io error which bubbles
//     through adbReader to the rclone transfer engine.

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/rclone/rclone/fs"
)

// execTransport dials the ADB exec: service for a specific device.
// gadb does not expose exec:. Used for binary-clean range reads in Object.Open.
type execTransport struct {
	host    string
	port    int
	serial  string
	timeout time.Duration
}

func newExecTransport(host string, port int, serial string) *execTransport {
	return &execTransport{
		host:    host,
		port:    port,
		serial:  serial,
		timeout: 60 * time.Second,
	}
}

// sendFrame writes a length-prefixed ADB protocol message.
// Format: 4 ASCII hex digits for the byte length, then the payload bytes.
func sendFrame(conn net.Conn, msg string) error {
	frame := fmt.Sprintf("%04x%s", len(msg), msg)
	_, err := fmt.Fprint(conn, frame)
	return err
}

// readOKAY reads a 4-byte ADB response status and returns nil on "OKAY".
// On "FAIL" it reads the length-prefixed error body and returns it as an error.
func readOKAY(r *bufio.Reader) error {
	var status [4]byte
	if _, err := io.ReadFull(r, status[:]); err != nil {
		return fmt.Errorf("adb exec: read status: %w", err)
	}
	switch string(status[:]) {
	case "OKAY":
		return nil
	case "FAIL":
		var lenHex [4]byte
		if _, err := io.ReadFull(r, lenHex[:]); err != nil {
			return fmt.Errorf("adb exec: read error length: %w", err)
		}
		n, err := strconv.ParseUint(string(lenHex[:]), 16, 32)
		if err != nil {
			return fmt.Errorf("adb exec: parse error length %q: %w", string(lenHex[:]), err)
		}
		msg := make([]byte, n)
		if _, err := io.ReadFull(r, msg); err != nil {
			return fmt.Errorf("adb exec: read error body: %w", err)
		}
		return fmt.Errorf("adb exec: device error: %s", msg)
	default:
		return fmt.Errorf("adb exec: unexpected status %q", string(status[:]))
	}
}

// execConn is an io.ReadCloser backed by a net.Conn.
// Read routes through a Reader that may include bytes buffered during the
// protocol handshake (via bufio). Close signals the ctx goroutine to exit.
type execConn struct {
	reader io.Reader // bufio drain + conn, or conn alone
	conn   net.Conn  // raw conn; Close calls conn.Close
	done   chan<- struct{}
}

func (c *execConn) Read(b []byte) (int, error) {
	return c.reader.Read(b)
}

func (c *execConn) Close() error {
	err := c.conn.Close()
	// Signal the goroutine. Non-blocking; capacity-1 channel prevents deadlock
	// if the ctx goroutine already exited after ctx.Done fired.
	select {
	case c.done <- struct{}{}:
	default:
	}
	return err
}

// Dial sends "host:transport:<serial>" then "exec:<command>" and returns a
// ReadCloser of raw stdout. Caller must Close the returned ReadCloser.
//
// Ctx cancellation closes the underlying TCP connection so in-progress Read
// calls unblock within one network round-trip. The caller must still call
// Close to release the ctx goroutine even after cancellation.
func (t *execTransport) Dial(ctx context.Context, command string) (io.ReadCloser, error) {
	var d net.Dialer
	addr := net.JoinHostPort(t.host, strconv.Itoa(t.port))
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("adb exec: dial %s: %w", addr, err)
	}

	// Handshake deadline: fail fast if the ADB server is unresponsive.
	if t.timeout > 0 {
		_ = conn.SetDeadline(time.Now().Add(t.timeout))
	}

	// done has capacity 1 so Close can always send even if ctx has already fired.
	done := make(chan struct{}, 1)

	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()

	// cleanup closes conn and signals done; called on any handshake error.
	cleanup := func() {
		_ = conn.Close()
		select {
		case done <- struct{}{}:
		default:
		}
	}

	br := bufio.NewReader(conn)

	if err := sendFrame(conn, fmt.Sprintf("host:transport:%s", t.serial)); err != nil {
		cleanup()
		return nil, fmt.Errorf("adb exec: send transport: %w", err)
	}
	if err := readOKAY(br); err != nil {
		cleanup()
		return nil, fmt.Errorf("adb exec: transport handshake: %w", err)
	}

	if err := sendFrame(conn, fmt.Sprintf("exec:%s", command)); err != nil {
		cleanup()
		return nil, fmt.Errorf("adb exec: send exec: %w", err)
	}
	if err := readOKAY(br); err != nil {
		cleanup()
		return nil, fmt.Errorf("adb exec: exec handshake: %w", err)
	}

	// Handshake complete. Clear the deadline; long file reads must not time out.
	_ = conn.SetDeadline(time.Time{})

	// bufio.Reader may hold bytes it read ahead during OKAY status parsing.
	// Drain those first, then fall through to conn for the remaining stream.
	var reader io.Reader
	if br.Buffered() > 0 {
		reader = io.MultiReader(br, conn)
	} else {
		reader = conn
	}

	return &execConn{
		reader: reader,
		conn:   conn,
		done:   done,
	}, nil
}

// adbReader wraps an io.ReadCloser from the exec: connection and handles:
//
//  1. Sub-block skip: dd reads in blockSize (4096) chunks. When the requested
//     file offset is not block-aligned, dd starts at the previous block boundary.
//     adbReader discards the leading (offset % blockSize) bytes.
//
//  2. Unexpected EOF detection: if the connection closes before expected bytes
//     are delivered, io.EOF is upgraded to io.ErrUnexpectedEOF so rclone can
//     distinguish a complete read from a truncated one and retry if needed.
//
// adbReader lives here alongside execTransport for cohesion.
// It is used only by Object.Open.
type adbReader struct {
	io.ReadCloser
	skip     int64 // bytes to discard at start of stream (sub-block offset remainder)
	read     int64 // total payload bytes delivered to caller so far
	expected int64 // total payload bytes the caller requested (= count param)
}

func (r *adbReader) Read(b []byte) (n int, err error) {
	n, err = r.ReadCloser.Read(b)
	if s := r.skip; n > 0 && s > 0 {
		_n := int64(n)
		if _n <= s {
			// Entire read buffer is still in the skip region. Discard and recurse.
			r.skip -= _n
			return r.Read(b)
		}
		// Read buffer straddles the skip boundary. Slide payload bytes to front.
		r.skip = 0
		copy(b, b[s:n])
		n -= int(s)
	}
	r.read += int64(n)
	if err == io.EOF && r.read < r.expected {
		fs.Debugf("adb", "adbReader: short read: got %d expected %d", r.read, r.expected)
		return n, io.ErrUnexpectedEOF
	}
	return n, err
}
