package protondrive

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataRateLimiterOffDoesNotRestrictReadSize(t *testing.T) {
	input := bytes.Repeat([]byte("x"), 2*dataLimitChunkSize)
	reader := limitDataReader(context.Background(), bytes.NewReader(input), newDataRateLimiter(-1))
	buffer := make([]byte, len(input))

	n, err := reader.Read(buffer)
	require.NoError(t, err)
	assert.Equal(t, len(input), n)
}

func TestDataRateLimiterIsShared(t *testing.T) {
	limiter := newDataRateLimiter(fs.Mebi)
	started := time.Now()
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reader := limitDataReader(
				context.Background(),
				bytes.NewReader(bytes.Repeat([]byte("x"), dataLimitChunkSize)),
				limiter,
			)
			_, err := io.Copy(io.Discard, reader)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	assert.GreaterOrEqual(t, time.Since(started), 100*time.Millisecond)
}

func TestDataRateLimiterHonorsContext(t *testing.T) {
	limiter := newDataRateLimiter(fs.SizeSuffix(1))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	reader := limitDataReader(ctx, bytes.NewReader([]byte("payload")), limiter)

	_, err := reader.Read(make([]byte, len("payload")))
	require.Error(t, err)
}

func TestDataRateLimiterRuntimeDisableAffectsExistingReader(t *testing.T) {
	limiter := newDataRateLimiter(fs.SizeSuffix(1))
	reader := limitDataReader(context.Background(), bytes.NewReader([]byte("payload")), limiter)

	limiter.set(-1)
	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, []byte("payload"), got)
}

func TestDataBandwidthCommandIsAtomic(t *testing.T) {
	f := &Fs{
		uploadLimit:   newDataRateLimiter(fs.Mebi),
		downloadLimit: newDataRateLimiter(2 * fs.Mebi),
	}

	_, err := f.Command(context.Background(), "data-bandwidth", nil, map[string]string{
		"upload":   "3M",
		"download": "invalid",
	})
	require.Error(t, err)
	upload, _ := f.uploadLimit.get()
	download, _ := f.downloadLimit.get()
	assert.Equal(t, fs.Mebi, upload)
	assert.Equal(t, 2*fs.Mebi, download)
}

func TestDataBandwidthCommandUpdatesExistingLimiters(t *testing.T) {
	f := &Fs{
		uploadLimit:   newDataRateLimiter(-1),
		downloadLimit: newDataRateLimiter(-1),
	}

	result, err := f.Command(context.Background(), "data-bandwidth", nil, map[string]string{
		"upload":   "4.8M",
		"download": "0",
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"upload":                 "4.800Mi",
		"download":               "off",
		"uploadBytesPerSecond":   int64(5033164),
		"downloadBytesPerSecond": int64(-1),
	}, result)
}
