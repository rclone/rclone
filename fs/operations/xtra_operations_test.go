// Extra operations tests (xtra_operations_test.go).
//
// This group contains tests, which involve streaming uploads.
// Currently they are TestRcat and TestRcatSize, which directly
// or implicitly invoke ioutil.NopCloser().
// Indeterminate upload size triggers multi-upload in few backends.
//
// The S3 backend additionally triggers extra large upload buffers.
// Namely, multiupload track in the Object.upload() method of S3
// backend (rclone/backends/s3.go) selects PartSize about 512M,
// upload.init() of AWS SDK (vendor/.../s3/s3manager/upload.go)
// allocates upload.bufferPool of that size for each concurrent
// upload goroutine. Given default concurrency of 4, this results
// in 2G buffers persisting until the test executable ends.
//
// As the rclone test suite parallelizes test runs, this may
// create memory pressure on a test box and trigger kernel swap,
// which extremely slows down the test and makes probability
// of memory contention between test processes even higher.
//
// Since name of this source file deliberately starts with `x`,
// its tests will run lattermost isolating high memory at the
// very end to somewhat reduce the contention probability.
//
package operations_test

import (
	"context"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRcat(t *testing.T) {
	checkSumBefore := fs.Config.CheckSum
	defer func() { fs.Config.CheckSum = checkSumBefore }()

	check := func(withChecksum bool) {
		fs.Config.CheckSum = withChecksum
		prefix := "no_checksum_"
		if withChecksum {
			prefix = "with_checksum_"
		}

		r := fstest.NewRun(t)
		defer r.Finalise()

		if *fstest.SizeLimit > 0 && int64(fs.Config.StreamingUploadCutoff) > *fstest.SizeLimit {
			savedCutoff := fs.Config.StreamingUploadCutoff
			defer func() {
				fs.Config.StreamingUploadCutoff = savedCutoff
			}()
			fs.Config.StreamingUploadCutoff = fs.SizeSuffix(*fstest.SizeLimit)
			t.Logf("Adjust StreamingUploadCutoff to size limit %s (was %s)", fs.Config.StreamingUploadCutoff, savedCutoff)
		}

		fstest.CheckListing(t, r.Fremote, []fstest.Item{})

		data1 := "this is some really nice test data"
		path1 := prefix + "small_file_from_pipe"

		data2 := string(make([]byte, fs.Config.StreamingUploadCutoff+1))
		path2 := prefix + "big_file_from_pipe"

		in := ioutil.NopCloser(strings.NewReader(data1))
		_, err := operations.Rcat(context.Background(), r.Fremote, path1, in, t1)
		require.NoError(t, err)

		in = ioutil.NopCloser(strings.NewReader(data2))
		_, err = operations.Rcat(context.Background(), r.Fremote, path2, in, t2)
		require.NoError(t, err)

		file1 := fstest.NewItem(path1, data1, t1)
		file2 := fstest.NewItem(path2, data2, t2)
		fstest.CheckItems(t, r.Fremote, file1, file2)
	}

	check(true)
	check(false)
}

func TestRcatSize(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	const body = "------------------------------------------------------------"
	file1 := r.WriteFile("potato1", body, t1)
	file2 := r.WriteFile("potato2", body, t2)
	// Test with known length
	bodyReader := ioutil.NopCloser(strings.NewReader(body))
	obj, err := operations.RcatSize(context.Background(), r.Fremote, file1.Path, bodyReader, int64(len(body)), file1.ModTime)
	require.NoError(t, err)
	assert.Equal(t, int64(len(body)), obj.Size())
	assert.Equal(t, file1.Path, obj.Remote())

	// Test with unknown length
	bodyReader = ioutil.NopCloser(strings.NewReader(body)) // reset Reader
	ioutil.NopCloser(strings.NewReader(body))
	obj, err = operations.RcatSize(context.Background(), r.Fremote, file2.Path, bodyReader, -1, file2.ModTime)
	require.NoError(t, err)
	assert.Equal(t, int64(len(body)), obj.Size())
	assert.Equal(t, file2.Path, obj.Remote())

	// Check files exist
	fstest.CheckItems(t, r.Fremote, file1, file2)
}
