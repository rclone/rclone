package ftp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/textproto"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/readers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A filename containing raw CR/LF must never survive encoding into an FTP
// command argument, whatever encoding the user configured, otherwise it could
// inject an independent FTP command onto the line-oriented control channel.
func TestCommandEncodingNeutralisesCRLF(t *testing.T) {
	// Internal Standard-encoded name as produced by e.g. a Unix local
	// source, carrying raw CR/LF.
	for _, std := range []string{
		"victim\r\nDELE other-secret\r\nNOOP",
		"victim␍␊DELE other-secret␍␊NOOP",
	} {
		for _, configured := range []encoder.MultiEncoder{
			encoder.EncodeSlash,                        // "Slash" - omits Ctl and CrLf
			encoder.EncodeZero,                         // "None"
			encoder.Display | encoder.EncodeRightSpace, // the FTP default
		} {
			got := commandEncoding(configured).FromStandardPath(std)
			assert.NotContains(t, got, "\r", "encoding %v leaked raw CR: %q", configured, got)
			assert.NotContains(t, got, "\n", "encoding %v leaked raw LF: %q", configured, got)
		}
	}
}

type settings map[string]any

func deriveFs(ctx context.Context, t *testing.T, f fs.Fs, opts settings) fs.Fs {
	fsName, _, _ := strings.Cut(f.Name(), "{") // strip off hash
	configMap := configmap.Simple{}
	for key, val := range opts {
		configMap[key] = fmt.Sprintf("%v", val)
	}
	remote := fmt.Sprintf("%s,%s:%s", fsName, configMap.String(), f.Root())
	fixFs, err := fs.NewFs(ctx, remote)
	require.NoError(t, err)
	return fixFs
}

// test that big file uploads do not cause network i/o timeout
func (f *Fs) testUploadTimeout(t *testing.T) {
	const (
		fileSize    = 100000000        // 100 MiB
		idleTimeout = 1 * time.Second  // small because test server is local
		maxTime     = 10 * time.Second // prevent test hangup
	)

	if testing.Short() {
		t.Skip("not running with -short")
	}

	ctx := context.Background()
	ci := fs.GetConfig(ctx)
	saveLowLevelRetries := ci.LowLevelRetries
	saveTimeout := ci.Timeout
	defer func() {
		ci.LowLevelRetries = saveLowLevelRetries
		ci.Timeout = saveTimeout
	}()
	ci.LowLevelRetries = 1

	upload := func(concurrency int, shutTimeout time.Duration) (obj fs.Object, err error) {
		// Use the saved (long) timeout while NewFs dials the FTP control
		// connection and reads the welcome banner. On slow CI servers this
		// can take longer than idleTimeout, which would cause a spurious
		// i/o timeout before the test can run. The connection's idle
		// deadline is captured at dial time, so the data connection
		// dialled later inside Put will still get idleTimeout - which is
		// what the test is actually trying to exercise.
		ci.Timeout = saveTimeout
		fixFs := deriveFs(ctx, t, f, settings{
			"concurrency":  concurrency,
			"shut_timeout": shutTimeout,
		})
		ci.Timeout = fs.Duration(idleTimeout)

		// Make test object
		fileTime := fstest.Time("2020-03-08T09:30:00.000000000Z")
		meta := object.NewStaticObjectInfo("upload-timeout.test", fileTime, int64(fileSize), true, nil, nil)
		data := readers.NewPatternReader(int64(fileSize))

		// Run upload and ensure maximum time
		done := make(chan bool)
		deadline := time.After(maxTime)
		go func() {
			obj, err = fixFs.Put(ctx, data, meta)
			done <- true
		}()
		select {
		case <-done:
		case <-deadline:
			t.Fatalf("Upload got stuck for %v !", maxTime)
		}

		return obj, err
	}

	// non-zero shut_timeout should fix i/o errors
	obj, err := upload(f.opt.Concurrency, time.Second)
	assert.NoError(t, err)
	assert.NotNil(t, obj)
	if obj != nil {
		_ = obj.Remove(ctx)
	}
}

// rclone must support precise time with ProFtpd and PureFtpd out of the box.
// The VsFtpd server does not support the MFMT command to set file time like
// other servers but by default supports the MDTM command in the non-standard
// two-argument form for the same purpose.
// See "mdtm_write" in https://security.appspot.com/vsftpd/vsftpd_conf.html
func (f *Fs) testTimePrecision(t *testing.T) {
	name := f.Name()
	if pos := strings.Index(name, "{"); pos != -1 {
		name = name[:pos]
	}
	switch name {
	case "TestFTPProftpd", "TestFTPPureftpd", "TestFTPVsftpd":
		assert.LessOrEqual(t, f.Precision(), time.Second)
	}
}

// InternalTest dispatches all internal tests
func (f *Fs) InternalTest(t *testing.T) {
	t.Run("UploadTimeout", f.testUploadTimeout)
	t.Run("TimePrecision", f.testTimePrecision)
}

var _ fstests.InternalTester = (*Fs)(nil)

// fakeFTPServer is a minimal FTP control-connection server which answers
// every command with 200 OK.
type fakeFTPServer struct {
	ln   net.Listener
	mu   sync.Mutex
	conn net.Conn
}

func startFakeFTPServer(t *testing.T) *fakeFTPServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s := &fakeFTPServer{ln: ln}
	go s.serve()
	t.Cleanup(func() { s.close() })
	return s
}

func (s *fakeFTPServer) serve() {
	conn, err := s.ln.Accept()
	if err != nil {
		return
	}
	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()
	defer func() { _ = conn.Close() }()
	_, _ = fmt.Fprint(conn, "220 fake FTP server\r\n")
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		_, _ = fmt.Fprint(conn, "200 OK\r\n")
	}
}

// killConnection closes the server side of the control connection,
// simulating a server that dropped an idle connection.
func (s *fakeFTPServer) killConnection() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != nil {
		_ = s.conn.Close()
	}
}

func (s *fakeFTPServer) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != nil {
		_ = s.conn.Close()
	}
	_ = s.ln.Close()
}

// TestPutFtpConnectionDiscardsDeadConnection checks that a connection
// returned with a raw network error is closed and never re-pooled, while
// a connection returned with a plain FTP reply-code error is probed with
// NOOP and pooled while the probe succeeds.
func TestPutFtpConnectionDiscardsDeadConnection(t *testing.T) {
	t.Run("network error discards", func(t *testing.T) {
		server := startFakeFTPServer(t)
		c, err := ftp.Dial(server.ln.Addr().String())
		require.NoError(t, err)
		f := &Fs{opt: Options{}}
		f.putFtpConnection(&c, errors.New("unexpected EOF"))
		assert.Nil(t, c)
		assert.Empty(t, f.pool)
	})

	t.Run("FTP reply error with live connection pools", func(t *testing.T) {
		server := startFakeFTPServer(t)
		c, err := ftp.Dial(server.ln.Addr().String())
		require.NoError(t, err)
		f := &Fs{opt: Options{}}
		f.putFtpConnection(&c, &textproto.Error{Code: 550, Msg: "not found"})
		assert.Nil(t, c)
		require.Len(t, f.pool, 1)
		assert.NoError(t, f.pool[0].NoOp())
		_ = f.pool[0].Quit()
	})

	t.Run("FTP reply error with dead connection discards", func(t *testing.T) {
		server := startFakeFTPServer(t)
		c, err := ftp.Dial(server.ln.Addr().String())
		require.NoError(t, err)
		server.killConnection()
		// Let the FIN propagate so the NOOP probe observes the dead connection.
		time.Sleep(100 * time.Millisecond)
		f := &Fs{opt: Options{}}
		f.putFtpConnection(&c, &textproto.Error{Code: 550, Msg: "not found"})
		assert.Nil(t, c)
		assert.Empty(t, f.pool)
	})
}
