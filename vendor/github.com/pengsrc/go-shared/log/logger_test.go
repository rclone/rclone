package log

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/pengsrc/go-shared/buffer"
)

func TestSetAndGetLevel(t *testing.T) {
	l, err := NewTerminalLogger()
	assert.NoError(t, err)

	l.SetLevel("ERROR")
	assert.Equal(t, "ERROR", l.GetLevel())
}

func TestNewLogger(t *testing.T) {
	buf := buffer.GlobalBytesPool().Get()
	defer buf.Free()

	l, err := NewLogger(buf, "INFO")
	assert.NoError(t, err)

	l.Debug(context.Background(), "DEBUG message")
	l.Info(context.Background(), "INFO message")

	assert.NotContains(t, buf.String(), "DEBUG message")
	assert.Contains(t, buf.String(), "INFO message")
	buf.Reset()

	// Test logging for context.
	type contextKey string
	const traceID contextKey = "trace_id"
	ctx := context.WithValue(nil, traceID, "60b725f10c9c85c70d97880dfe8191b3")

	l.SetInterestContextKeys([]interface{}{traceID})

	l.Info(ctx, "Hello World!")
	assert.Contains(t, buf.String(), "trace_id=60b725f10c9c85c70d97880dfe8191b3")
	t.Log(buf.String())
	buf.Reset()

	l.SetCallerFlag(true)

	l.Info(ctx, "Hello World!")
	assert.Contains(t, buf.String(), "source=log/logger_test.go")
	t.Log(buf.String())
	buf.Reset()

	l.Infof(ctx, "Hello %s!", "World")
	assert.Contains(t, buf.String(), "source=log/logger_test.go")
	t.Log(buf.String())
	buf.Reset()

	l.InfoEvent(ctx).Messagef("Hello %s!", "World")
	assert.Contains(t, buf.String(), "source=log/logger_test.go")
	t.Log(buf.String())
	buf.Reset()

	l.InfoEvent(ctx).Int("count", 1024).Messagef("Hello %s!", "World")
	assert.Contains(t, buf.String(), "source=log/logger_test.go")
	t.Log(buf.String())
	buf.Reset()
}

func TestNewLoggerWithError(t *testing.T) {
	buf := buffer.GlobalBytesPool().Get()
	errBuf := buffer.GlobalBytesPool().Get()
	defer buf.Free()
	defer errBuf.Free()

	l, err := NewLoggerWithError(buf, errBuf, "INFO")
	assert.NoError(t, err)

	l.Debug(context.Background(), "DEBUG message")
	l.Info(context.Background(), "INFO message")
	l.Error(context.Background(), "ERROR message")

	assert.NotContains(t, buf.String(), "DEBUG message")
	assert.Contains(t, buf.String(), "INFO message")
	assert.Contains(t, buf.String(), "ERROR message")

	assert.NotContains(t, errBuf.String(), "DEBUG message")
	assert.NotContains(t, errBuf.String(), "INFO message")
	assert.Contains(t, errBuf.String(), "ERROR message")
}

func TestNewFileLogger(t *testing.T) {
	t.Skip()

	lFile := path.Join(os.TempDir(), "test.log")
	defer os.Remove(lFile)

	// Create logger failed.
	_, err := NewLoggerWithError(nil, nil, "DEBUG")
	assert.Error(t, err)
	_, err = NewLoggerWithError(ioutil.Discard, ioutil.Discard, "INVALID")
	assert.Error(t, err)

	// Create logger success.
	l, err := NewFileLogger(lFile, "INFO")
	assert.NoError(t, err)

	l.Debug(context.Background(), "file - DEBUG message")
	l.Info(context.Background(), "file - INFO message")
	l.Warn(context.Background(), "file - WARN message")
	l.Error(context.Background(), "file - ERROR message")

	data, err := ioutil.ReadFile(lFile)
	assert.NoError(t, err)
	assert.Equal(t, 4, len(strings.Split(string(data), "\n")))

	// Move log file.
	movedLFile := fmt.Sprintf(`%s.move`, lFile)
	err = os.Rename(lFile, movedLFile)
	assert.NoError(t, err)
	defer os.Remove(movedLFile)

	l.Error(context.Background(), "file - ERROR message")

	data, err = ioutil.ReadFile(movedLFile)
	assert.NoError(t, err)
	assert.Equal(t, 5, len(strings.Split(string(data), "\n")))

	// Reopen.
	syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
	time.Sleep(1 * time.Second)

	l.Warn(context.Background(), "file - WARN message")
	l.Error(context.Background(), "file - ERROR message")

	data, err = ioutil.ReadFile(lFile)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(strings.Split(string(data), "\n")))
}

func TestNewFileLoggerWithError(t *testing.T) {
	t.Skip()

	lFile := path.Join(os.TempDir(), "test.log")
	errLFile := path.Join(os.TempDir(), "test.log.wf")
	defer os.Remove(lFile)
	defer os.Remove(errLFile)

	// Create logger failed.
	_, err := NewFileLoggerWithError("/not/exists/dir", "/not/exists/dir", "DEBUG")
	assert.Error(t, err)
	_, err = NewFileLoggerWithError(lFile, "/not/exists/dir", "DEBUG")
	assert.Error(t, err)
	_, err = NewFileLoggerWithError(os.TempDir(), os.TempDir(), "DEBUG")
	assert.Error(t, err)
	_, err = NewFileLoggerWithError(lFile, os.TempDir(), "DEBUG")
	assert.Error(t, err)

	// Create logger success.
	l, err := NewFileLoggerWithError(lFile, errLFile, "DEBUG")
	assert.NoError(t, err)

	l.Debug(context.Background(), "file - DEBUG message")
	l.Info(context.Background(), "file - INFO message")
	l.Warn(context.Background(), "file - WARN message")
	l.Error(context.Background(), "file - ERROR message")

	data, err := ioutil.ReadFile(lFile)
	assert.NoError(t, err)
	assert.Equal(t, 5, len(strings.Split(string(data), "\n")))

	errLog, err := ioutil.ReadFile(errLFile)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(strings.Split(string(errLog), "\n")))

	// Move data file.
	movedLogFile := fmt.Sprintf(`%s.move`, lFile)
	err = os.Rename(lFile, movedLogFile)
	assert.NoError(t, err)
	defer os.Remove(movedLogFile)

	movedErrLogFile := fmt.Sprintf(`%s.move`, errLFile)
	err = os.Rename(errLFile, movedErrLogFile)
	assert.NoError(t, err)
	defer os.Remove(movedErrLogFile)

	l.Error(context.Background(), "file - ERROR message")

	data, err = ioutil.ReadFile(movedLogFile)
	assert.NoError(t, err)
	assert.Equal(t, 6, len(strings.Split(string(data), "\n")))

	errLog, err = ioutil.ReadFile(movedErrLogFile)
	assert.NoError(t, err)
	assert.Equal(t, 4, len(strings.Split(string(errLog), "\n")))

	// Reopen.
	syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
	time.Sleep(1 * time.Second)

	l.Warn(context.Background(), "file - WARN message")
	l.Error(context.Background(), "file - ERROR message")

	data, err = ioutil.ReadFile(lFile)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(strings.Split(string(data), "\n")))

	errLog, err = ioutil.ReadFile(errLFile)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(strings.Split(string(errLog), "\n")))
}

func TestBufferedFileLogger(t *testing.T) {
	lFile := path.Join(os.TempDir(), "test.log")
	defer os.Remove(lFile)

	l, err := NewBufferedFileLogger(lFile, 1, "DEBUG")
	assert.NoError(t, err)

	l.Debug(context.Background(), "file - DEBUG message")
	l.Info(context.Background(), "file - INFO message")
	l.Warn(context.Background(), "file - WARN message")
	l.Error(context.Background(), "file - ERROR message")

	data, err := ioutil.ReadFile(lFile)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(strings.Split(string(data), "\n")))

	// Wait timeout.
	time.Sleep(2 * time.Second)

	data, err = ioutil.ReadFile(lFile)
	assert.NoError(t, err)
	assert.Equal(t, 5, len(strings.Split(string(data), "\n")))
}

func TestBufferedFileLoggerWithError(t *testing.T) {
	lFile := path.Join(os.TempDir(), "test.log")
	errLFile := path.Join(os.TempDir(), "test.log.wf")
	defer os.Remove(lFile)
	defer os.Remove(errLFile)

	// Create logger failed.
	_, err := NewBufferedFileLoggerWithError("/not/exists/dir", "/not/exists/dir", 0, "DEBUG")
	assert.Error(t, err)
	_, err = NewBufferedFileLoggerWithError(lFile, "/not/exists/dir", 0, "DEBUG")
	assert.Error(t, err)
	_, err = NewBufferedFileLoggerWithError(os.TempDir(), os.TempDir(), 0, "DEBUG")
	assert.Error(t, err)
	_, err = NewBufferedFileLoggerWithError(lFile, os.TempDir(), 0, "DEBUG")
	assert.Error(t, err)

	// Create logger success.
	errL, err := NewBufferedFileLoggerWithError(lFile, errLFile, 1, "DEBUG")
	assert.NoError(t, err)

	errL.Debug(context.Background(), "file - DEBUG message")
	errL.Info(context.Background(), "file - INFO message")
	errL.Warn(context.Background(), "file - WARN message")
	errL.Error(context.Background(), "file - ERROR message")

	data, err := ioutil.ReadFile(lFile)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(strings.Split(string(data), "\n")))

	errData, err := ioutil.ReadFile(errLFile)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(strings.Split(string(errData), "\n")))

	// Flush log.
	errL.Flush()

	data, err = ioutil.ReadFile(lFile)
	assert.NoError(t, err)
	assert.Equal(t, 5, len(strings.Split(string(data), "\n")))

	errData, err = ioutil.ReadFile(errLFile)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(strings.Split(string(errData), "\n")))

	// Wait timeout to improve test coverage.
	time.Sleep(2 * time.Second)

	// Reopen using signal to improve test coverage.
	syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
}

func TestTerminalLogger(t *testing.T) {
	l, err := NewTerminalLogger("DEBUG")
	assert.NoError(t, err)

	l.Debug(context.Background(), "terminal - DEBUG message")
	l.Info(context.Background(), "terminal - INFO message")
	l.Warn(context.Background(), "terminal - WARN message")
	l.Error(context.Background(), "terminal - ERROR message")

	l.Debugf(context.Background(), "terminal - DEBUG message - %d", time.Now().Unix())
	l.Infof(context.Background(), "terminal - INFO message - %d", time.Now().Unix())
	l.Warnf(context.Background(), "terminal - WARN message - %d", time.Now().Unix())
	l.Errorf(context.Background(), "terminal - ERROR message - %d", time.Now().Unix())
}

func TestBufferedTerminalLogger(t *testing.T) {
	l, err := NewBufferedTerminalLogger("DEBUG")
	assert.NoError(t, err)

	l.Debug(context.Background(), "terminal - DEBUG message")
	l.Info(context.Background(), "terminal - INFO message")
	l.Warn(context.Background(), "terminal - WARN message")
	l.Error(context.Background(), "terminal - ERROR message")

	l.Flush()
}

func BenchmarkLogger(b *testing.B) {
	l, err := NewLogger(ioutil.Discard, "DEBUG")
	assert.NoError(b, err)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.DebugEvent(ctx).String("key", "value").Messagef("Hello %s!", "World")
	}
}
