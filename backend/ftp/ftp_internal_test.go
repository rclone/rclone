package ftp

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/readers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type settings map[string]interface{}

func deriveFs(ctx context.Context, t *testing.T, f fs.Fs, opts settings) fs.Fs {
	fsName := strings.Split(f.Name(), "{")[0] // strip off hash
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
	ci.Timeout = idleTimeout

	upload := func(concurrency int, shutTimeout time.Duration) (obj fs.Object, err error) {
		fixFs := deriveFs(ctx, t, f, settings{
			"concurrency":  concurrency,
			"shut_timeout": shutTimeout,
		})

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
