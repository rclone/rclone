package copyurl

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetGlobals() {
	autoFilename = false
	headerFilename = false
	printFilename = false
	stdout = false
	noClobber = false
	urls = false
	copyURL = operations.CopyURL
}

func TestRun_RequiresTwoArgsWhenNotStdout(t *testing.T) {
	t.Cleanup(resetGlobals)
	resetGlobals()

	err := run([]string{"https://example.com/foo"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "need 2 arguments if not using --stdout")
}

func TestRun_CallsCopyURL_WithExplicitFilename_Success(t *testing.T) {
	t.Cleanup(resetGlobals)
	resetGlobals()

	tmp := t.TempDir()
	dstPath := filepath.Join(tmp, "out.txt")

	var called atomic.Int32

	copyURL = func(_ctx context.Context, _dst fs.Fs, dstFileName, url string, auto, header, noclobber bool) (fs.Object, error) {
		called.Add(1)
		assert.Equal(t, "https://example.com/file", url)
		assert.Equal(t, "out.txt", dstFileName)
		assert.False(t, auto)
		assert.False(t, header)
		assert.False(t, noclobber)
		return nil, nil
	}

	err := run([]string{"https://example.com/file", dstPath})
	require.NoError(t, err)
	assert.Equal(t, int32(1), called.Load())
}

func TestRun_CallsCopyURL_WithAutoFilename_AndPropagatesError(t *testing.T) {
	t.Cleanup(resetGlobals)
	resetGlobals()

	tmp := t.TempDir()
	autoFilename = true

	want := errors.New("boom")
	var called atomic.Int32

	copyURL = func(_ctx context.Context, _dst fs.Fs, dstFileName, url string, auto, header, noclobber bool) (fs.Object, error) {
		called.Add(1)
		assert.Equal(t, "", dstFileName) // auto filename -> empty
		assert.True(t, auto)
		return nil, want
	}

	err := run([]string{"https://example.com/auto/name", tmp})
	require.Error(t, err)
	assert.Equal(t, want, err)
	assert.Equal(t, int32(1), called.Load())
}

func TestRunURLS_ErrorsWithStdoutAndWithPrintFilename(t *testing.T) {
	t.Cleanup(resetGlobals)
	resetGlobals()

	stdout = true
	err := runURLS([]string{"dummy.csv", "destDir"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "can't use --stdout with --urls")

	resetGlobals()
	printFilename = true
	err = runURLS([]string{"dummy.csv", "destDir"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "can't use --print-filename with --urls")
}

func TestRunURLS_ProcessesCSV_ParallelCalls_AndAggregatesError(t *testing.T) {
	t.Cleanup(resetGlobals)
	resetGlobals()

	tmp := t.TempDir()
	csvPath := filepath.Join(tmp, "urls.csv")
	csvContent := []byte(
		"https://example.com/a,aaa.txt\n" + // success
			"https://example.com/b\n" + // auto filename
			"https://example.com/c,ccc.txt\n") // error
	require.NoError(t, os.WriteFile(csvPath, csvContent, 0o600))

	// destination dir (local backend)
	dest := t.TempDir()

	// mock copyURL: succeed for /a and /b, fail for /c

	var calls atomic.Int32
	var mu sync.Mutex
	var seen []string

	copyURL = func(_ctx context.Context, _dst fs.Fs, dstFileName, url string, auto, header, noclobber bool) (fs.Object, error) {
		calls.Add(1)
		mu.Lock()
		seen = append(seen, url+"|"+dstFileName)
		mu.Unlock()

		switch {
		case url == "https://example.com/a":
			require.Equal(t, "aaa.txt", dstFileName)
			return nil, nil
		case url == "https://example.com/b":
			require.Equal(t, "", dstFileName) // auto-name path
			return nil, nil
		case url == "https://example.com/c":
			return nil, errors.New("network down")
		default:
			return nil, nil
		}
	}

	err := runURLS([]string{csvPath, dest})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not all URLs copied successfully")
	// 3 lines => 3 calls
	assert.Equal(t, int32(3), calls.Load())

	// sanity: all expected URLs were seen
	assert.ElementsMatch(t,
		[]string{
			"https://example.com/a|aaa.txt",
			"https://example.com/b|",
			"https://example.com/c|ccc.txt",
		},
		seen,
	)
}
