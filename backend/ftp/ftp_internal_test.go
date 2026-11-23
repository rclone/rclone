package ftp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
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

// fakeDataReader stands in for the RETR data stream of ftpReadCloser.
type fakeDataReader struct {
	data     []byte
	closeErr error
}

func (r *fakeDataReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

func (r *fakeDataReader) Close() error { return r.closeErr }

// TestFtpReadCloserClose checks that an early-closed RETR answered with 425
// is masked only when nothing was read, that a connection which got QUIT is
// never pooled, and that a second Close does not touch the pool.
func TestFtpReadCloserClose(t *testing.T) {
	err425 := &textproto.Error{Code: ftp.StatusCanNotOpenDataConnection, Msg: "Can't open data connection."}
	newReader := func(t *testing.T, data []byte, closeErr error) (*ftpReadCloser, *Fs) {
		server := startFakeFTPServer(t)
		c, err := ftp.Dial(server.ln.Addr().String())
		require.NoError(t, err)
		f := &Fs{opt: Options{}}
		return &ftpReadCloser{rc: &fakeDataReader{data: data, closeErr: closeErr}, c: c, f: f}, f
	}

	t.Run("425 before any read is masked and the connection is not pooled", func(t *testing.T) {
		rc, f := newReader(t, nil, err425)
		assert.NoError(t, rc.Close())
		assert.Empty(t, f.pool)
		assert.NoError(t, rc.Close())
		assert.Empty(t, f.pool)
	})

	t.Run("425 after data was read stays an error", func(t *testing.T) {
		rc, f := newReader(t, []byte("partial"), err425)
		buf := make([]byte, 4)
		n, err := rc.Read(buf)
		require.NoError(t, err)
		require.Equal(t, 4, n)
		err = rc.Close()
		require.Error(t, err)
		assert.Equal(t, ftp.StatusCanNotOpenDataConnection, textprotoError(err).Code)
		assert.Empty(t, f.pool)
	})

	t.Run("clean close pools the connection once", func(t *testing.T) {
		rc, f := newReader(t, []byte("data"), nil)
		_, err := io.ReadAll(rc)
		require.NoError(t, err)
		assert.NoError(t, rc.Close())
		require.Len(t, f.pool, 1)
		assert.NoError(t, rc.Close())
		assert.Len(t, f.pool, 1)
		_ = f.pool[0].Quit()
	})
}

// startPasvFTPServer starts an FTP server on 127.0.0.1 whose PASV reply
// sends data connections to 127.0.0.2, returning its port.
func startPasvFTPServer(t *testing.T) string {
	t.Helper()
	data, err := net.Listen("tcp", "127.0.0.2:0")
	if err != nil {
		t.Skipf("can't listen on 127.0.0.2: %v", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = ln.Close()
		_ = data.Close()
	})
	dataPort := data.Addr().(*net.TCPAddr).Port
	session := func(conn net.Conn) {
		defer func() { _ = conn.Close() }()
		_, _ = fmt.Fprint(conn, "220 fake FTP server\r\n")
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			cmd, _, _ := strings.Cut(scanner.Text(), " ")
			switch strings.ToUpper(cmd) {
			case "USER":
				_, _ = fmt.Fprint(conn, "331 need password\r\n")
			case "PASS":
				_, _ = fmt.Fprint(conn, "230 logged in\r\n")
			case "FEAT":
				_, _ = fmt.Fprint(conn, "211-Features:\r\n PASV\r\n211 End\r\n")
			case "EPSV":
				_, _ = fmt.Fprint(conn, "500 unknown command\r\n")
			case "PASV":
				_, _ = fmt.Fprintf(conn, "227 Entering Passive Mode (127,0,0,2,%d,%d)\r\n", dataPort/256, dataPort%256)
			case "LIST":
				_, _ = fmt.Fprint(conn, "150 opening\r\n")
				dataConn, err := data.Accept()
				if err != nil {
					return
				}
				_, _ = fmt.Fprint(dataConn, "-rw-r--r-- 1 user group 5 Jan 01 00:00 file.txt\r\n")
				_ = dataConn.Close()
				_, _ = fmt.Fprint(conn, "226 done\r\n")
			case "QUIT":
				_, _ = fmt.Fprint(conn, "221 bye\r\n")
				return
			default:
				_, _ = fmt.Fprint(conn, "200 OK\r\n")
			}
		}
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go session(conn)
		}
	}()
	return fmt.Sprint(ln.Addr().(*net.TCPAddr).Port)
}

// TestTrustPasvIP checks that the PASV reply IP is only used with
// trust_pasv_ip and that the error suggests it when it isn't set.
func TestTrustPasvIP(t *testing.T) {
	ctx := context.Background()
	port := startPasvFTPServer(t)
	newFs := func(trust bool) fs.Fs {
		f, err := NewFs(ctx, "TestTrustPasvIP", "", configmap.Simple{
			"host":          "127.0.0.1",
			"port":          port,
			"user":          "user",
			"pass":          obscure.MustObscure("pass"),
			"trust_pasv_ip": fmt.Sprint(trust),
		})
		require.NoError(t, err)
		t.Cleanup(func() { _ = f.Features().Shutdown(ctx) })
		return f
	}

	t.Run("untrusted", func(t *testing.T) {
		_, err := newFs(false).List(ctx, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "127.0.0.2")
		assert.Contains(t, err.Error(), "--ftp-trust-pasv-ip")
	})

	t.Run("trusted", func(t *testing.T) {
		entries, err := newFs(true).List(ctx, "")
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, "file.txt", entries[0].Remote())
	})
}
