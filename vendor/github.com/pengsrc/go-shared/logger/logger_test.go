package logger

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCheckLevel(t *testing.T) {
	assert.NoError(t, CheckLevel("warn"))
	assert.Error(t, CheckLevel("invalid"))
}

func TestSetAndGetLevel(t *testing.T) {
	l, err := NewTerminalLogger()
	assert.NoError(t, err)

	l.SetLevel("error")
	assert.Equal(t, "error", l.GetLevel())
}

func TestNewFileLogger(t *testing.T) {
	logFile := "/tmp/logger-test/test.log"
	dir := path.Dir(logFile)
	err := os.MkdirAll(dir, 0775)
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	l, err := NewFileLogger(logFile, "debug")
	assert.NoError(t, err)

	l.Debug("file - debug")
	l.Info("file - info")
	l.Warn("file - warn")
	l.Error("file - error")

	log, err := ioutil.ReadFile(logFile)
	assert.NoError(t, err)
	assert.Equal(t, 5, len(strings.Split(string(log), "\n")))

	// Move log file.
	movedLogFile := fmt.Sprintf(`%s.move`, logFile)
	os.Rename(logFile, movedLogFile)

	l.Error("file - error")

	log, err = ioutil.ReadFile(movedLogFile)
	assert.NoError(t, err)
	assert.Equal(t, 6, len(strings.Split(string(log), "\n")))

	// Reopen.
	syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
	time.Sleep(10 * time.Millisecond)

	l.Warn("file - warn")
	l.Error("file - error")

	log, err = ioutil.ReadFile(logFile)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(strings.Split(string(log), "\n")))
}

func TestNewFileLoggerWithWf(t *testing.T) {
	logFile := "/tmp/logger-test/test.log"
	errLogFile := "/tmp/logger-test/test.log.wf"
	dir := path.Dir(logFile)
	err := os.MkdirAll(dir, 0775)
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	l, err := NewFileLoggerWithErr(logFile, errLogFile, "debug")
	assert.NoError(t, err)

	l.Debug("file - debug")
	l.Info("file - info")
	l.Warn("file - warn")
	l.Error("file - error")

	log, err := ioutil.ReadFile(logFile)
	assert.NoError(t, err)
	assert.Equal(t, 5, len(strings.Split(string(log), "\n")))

	errLog, err := ioutil.ReadFile(errLogFile)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(strings.Split(string(errLog), "\n")))

	// Move log file.
	movedLogFile := fmt.Sprintf(`%s.move`, logFile)
	os.Rename(logFile, movedLogFile)

	movedErrLogFile := fmt.Sprintf(`%s.move`, errLogFile)
	os.Rename(errLogFile, movedErrLogFile)

	l.Error("file - error")

	log, err = ioutil.ReadFile(movedLogFile)
	assert.NoError(t, err)
	assert.Equal(t, 6, len(strings.Split(string(log), "\n")))

	errLog, err = ioutil.ReadFile(movedErrLogFile)
	assert.NoError(t, err)
	assert.Equal(t, 4, len(strings.Split(string(errLog), "\n")))

	// Reopen.
	syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
	time.Sleep(10 * time.Millisecond)

	l.Warn("file - warn")
	l.Error("file - error")

	log, err = ioutil.ReadFile(logFile)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(strings.Split(string(log), "\n")))

	errLog, err = ioutil.ReadFile(errLogFile)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(strings.Split(string(errLog), "\n")))
}

func TestBufferedFileLogger(t *testing.T) {
	logFile := "/tmp/logger-test/test.log"
	dir := path.Dir(logFile)
	err := os.MkdirAll(dir, 0775)
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	l, err := NewBufferedFileLogger(logFile, "debug")
	assert.NoError(t, err)

	l.Debug("file - debug")
	l.Info("file - info")
	l.Warn("file - warn")
	l.Error("file - error")

	log, err := ioutil.ReadFile(logFile)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(strings.Split(string(log), "\n")))

	// Wait timeout.
	//time.Sleep(10*time.Second + 10*time.Millisecond)
	//
	//log, err = ioutil.ReadFile(logFile)
	//assert.NoError(t, err)
	//assert.Equal(t, 5, len(strings.Split(string(log), "\n")))
}

func TestBufferedFileLoggerWithErr(t *testing.T) {
	logFile := "/tmp/logger-test/test.log"
	errLogFile := "/tmp/logger-test/test.log.wf"
	dir := path.Dir(logFile)
	err := os.MkdirAll(dir, 0775)
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	errL, err := NewBufferedFileLoggerWithErr(logFile, errLogFile, "debug")
	assert.NoError(t, err)

	errL.Debug("file - debug")
	errL.Info("file - info")
	errL.Warn("file - warn")
	errL.Error("file - error")

	log, err := ioutil.ReadFile(logFile)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(strings.Split(string(log), "\n")))

	errLog, err := ioutil.ReadFile(errLogFile)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(strings.Split(string(errLog), "\n")))

	// Wait timeout.
	//time.Sleep(10*time.Second + 10*time.Millisecond)
	//
	//log, err = ioutil.ReadFile(logFile)
	//assert.NoError(t, err)
	//assert.Equal(t, 5, len(strings.Split(string(log), "\n")))
	//
	//errLog, err = ioutil.ReadFile(errLogFile)
	//assert.NoError(t, err)
	//assert.Equal(t, 3, len(strings.Split(string(errLog), "\n")))
}

func TestTerminalLogger(t *testing.T) {
	l, err := NewTerminalLogger("debug")
	assert.NoError(t, err)

	l.Debug("terminal - debug")
	l.Info("terminal - info")
	l.Warn("terminal - warn")
	l.Error("terminal - error")

	l.Debugf("terminal - debug - %d", time.Now().Unix())
	l.Infof("terminal - info - %d", time.Now().Unix())
	l.Warnf("terminal - warn - %d", time.Now().Unix())
	l.Errorf("terminal - error - %d", time.Now().Unix())
}

func TestTerminalLoggerWithErr(t *testing.T) {
	errL, err := NewTerminalLoggerWithErr("debug")
	assert.NoError(t, err)

	errL.Debug("terminal - debug - err")
	errL.Info("terminal - info - err")
	errL.Warn("terminal - warn - err")
	errL.Error("terminal - error - err")

	errL.Debugf("terminal - debug - err - %d", time.Now().Unix())
	errL.Infof("terminal - info - err - %d", time.Now().Unix())
	errL.Warnf("terminal - warn - err - %d", time.Now().Unix())
	errL.Errorf("terminal - error - err - %d", time.Now().Unix())
}
