// Test Swift filesystem interface
package swift

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/ncw/swift/v2"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/lib/readers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestSwiftAIO:",
		NilObject:  (*Object)(nil),
	})
}

func (f *Fs) SetUploadChunkSize(cs fs.SizeSuffix) (fs.SizeSuffix, error) {
	return f.setUploadChunkSize(cs)
}

var _ fstests.SetUploadChunkSizer = (*Fs)(nil)

// Check that PutStream works with NoChunk as it is the major code
// deviation
func (f *Fs) testNoChunk(t *testing.T) {
	ctx := context.Background()
	f.opt.NoChunk = true
	defer func() {
		f.opt.NoChunk = false
	}()

	file := fstest.Item{
		ModTime: fstest.Time("2001-02-03T04:05:06.499999999Z"),
		Path:    "piped data no chunk.txt",
		Size:    -1, // use unknown size during upload
	}

	const contentSize = 100

	contents := random.String(contentSize)
	buf := bytes.NewBufferString(contents)
	uploadHash := hash.NewMultiHasher()
	in := io.TeeReader(buf, uploadHash)

	// Track how much space is used before we put our object.
	usage, err := f.About(ctx)
	require.NoError(t, err)
	usedBeforePut := *usage.Used

	file.Size = -1
	obji := object.NewStaticObjectInfo(file.Path, file.ModTime, file.Size, true, nil, nil)
	obj, err := f.Features().PutStream(ctx, in, obji)
	require.NoError(t, err)

	file.Hashes = uploadHash.Sums()
	file.Size = int64(contentSize) // use correct size when checking
	file.Check(t, obj, f.Precision())

	// Re-read the object and check again
	obj, err = f.NewObject(ctx, file.Path)
	require.NoError(t, err)
	file.Check(t, obj, f.Precision())

	// Check how much space is used after the upload, should match the amount we
	// uploaded..
	usage, err = f.About(ctx)
	require.NoError(t, err)
	expectedUsed := usedBeforePut + obj.Size()
	require.EqualValues(t, expectedUsed, *usage.Used)

	// Delete the object
	assert.NoError(t, obj.Remove(ctx))
}

// Additional tests that aren't in the framework
func (f *Fs) InternalTest(t *testing.T) {
	t.Run("PolicyDiscovery", f.testPolicyDiscovery)
	t.Run("NoChunk", f.testNoChunk)
	t.Run("WithChunk", f.testWithChunk)
	t.Run("WithChunkFail", f.testWithChunkFail)
	t.Run("CopyLargeObject", f.testCopyLargeObject)
}

func (f *Fs) testWithChunk(t *testing.T) {
	preConfChunkSize := f.opt.ChunkSize
	preConfChunk := f.opt.NoChunk
	f.opt.NoChunk = false
	f.opt.ChunkSize = 1024 * fs.SizeSuffixBase
	defer func() {
		//restore old config after test
		f.opt.ChunkSize = preConfChunkSize
		f.opt.NoChunk = preConfChunk
	}()

	file := fstest.Item{
		ModTime: fstest.Time("2020-12-31T04:05:06.499999999Z"),
		Path:    "piped data chunk.txt",
		Size:    -1, // use unknown size during upload
	}
	const contentSize = 2048
	contents := random.String(contentSize)
	buf := bytes.NewBufferString(contents)
	uploadHash := hash.NewMultiHasher()
	in := io.TeeReader(buf, uploadHash)

	// Track how much space is used before we put our object.
	ctx := context.TODO()
	usage, err := f.About(ctx)
	require.NoError(t, err)
	usedBeforePut := *usage.Used

	file.Size = -1
	obji := object.NewStaticObjectInfo(file.Path, file.ModTime, file.Size, true, nil, nil)
	obj, err := f.Features().PutStream(ctx, in, obji)
	require.NoError(t, err)
	require.NotEmpty(t, obj)

	// Check how much space is used after the upload, should match the amount we
	// uploaded..
	usage, err = f.About(ctx)
	require.NoError(t, err)
	expectedUsed := usedBeforePut + obj.Size()
	require.EqualValues(t, expectedUsed, *usage.Used)
}

func (f *Fs) testWithChunkFail(t *testing.T) {
	preConfChunkSize := f.opt.ChunkSize
	preConfChunk := f.opt.NoChunk
	f.opt.NoChunk = false
	f.opt.ChunkSize = 1024 * fs.SizeSuffixBase
	segmentContainer := f.root + "_segments"
	if !f.opt.UseSegmentsContainer.Value {
		segmentContainer = f.root
	}
	defer func() {
		//restore config
		f.opt.ChunkSize = preConfChunkSize
		f.opt.NoChunk = preConfChunk
	}()
	path := "piped data chunk with error.txt"
	file := fstest.Item{
		ModTime: fstest.Time("2021-01-04T03:46:00.499999999Z"),
		Path:    path,
		Size:    -1, // use unknown size during upload
	}
	const contentSize = 4096
	const errPosition = 3072
	contents := random.String(contentSize)
	buf := bytes.NewBufferString(contents[:errPosition])
	errMessage := "potato"
	er := &readers.ErrorReader{Err: errors.New(errMessage)}
	in := io.NopCloser(io.MultiReader(buf, er))

	file.Size = contentSize
	obji := object.NewStaticObjectInfo(file.Path, file.ModTime, file.Size, true, nil, nil)
	ctx := context.TODO()
	_, err := f.Features().PutStream(ctx, in, obji)
	// error is potato
	require.NotNil(t, err)
	require.Equal(t, errMessage, err.Error())
	_, _, err = f.c.Object(ctx, f.rootContainer, path)
	assert.Equal(t, swift.ObjectNotFound, err)
	prefix := path
	if !f.opt.UseSegmentsContainer.Value {
		prefix = segmentsDirectory + "/" + prefix
	}
	objs, err := f.c.Objects(ctx, segmentContainer, &swift.ObjectsOpts{
		Prefix: prefix,
	})
	require.NoError(t, err)
	require.Empty(t, objs)
}

func (f *Fs) testCopyLargeObject(t *testing.T) {
	preConfChunkSize := f.opt.ChunkSize
	preConfChunk := f.opt.NoChunk
	f.opt.NoChunk = false
	f.opt.ChunkSize = 1024 * fs.SizeSuffixBase
	defer func() {
		//restore old config after test
		f.opt.ChunkSize = preConfChunkSize
		f.opt.NoChunk = preConfChunk
	}()

	file := fstest.Item{
		ModTime: fstest.Time("2020-12-31T04:05:06.499999999Z"),
		Path:    "large.txt",
		Size:    -1, // use unknown size during upload
	}
	const contentSize = 2048
	contents := random.String(contentSize)
	buf := bytes.NewBufferString(contents)
	uploadHash := hash.NewMultiHasher()
	in := io.TeeReader(buf, uploadHash)

	// Track how much space is used before we put our object.
	ctx := context.TODO()
	usage, err := f.About(ctx)
	require.NoError(t, err)
	usedBeforePut := *usage.Used

	file.Size = -1
	obji := object.NewStaticObjectInfo(file.Path, file.ModTime, file.Size, true, nil, nil)
	obj, err := f.Features().PutStream(ctx, in, obji)
	require.NoError(t, err)
	require.NotEmpty(t, obj)
	remoteTarget := "large.txt (copy)"
	objTarget, err := f.Features().Copy(ctx, obj, remoteTarget)
	require.NoError(t, err)
	require.NotEmpty(t, objTarget)
	require.Equal(t, obj.Size(), objTarget.Size())

	// Check how much space is used after the upload, should match the amount we
	// uploaded *and* the copy.
	usage, err = f.About(ctx)
	require.NoError(t, err)
	expectedUsed := usedBeforePut + obj.Size() + objTarget.Size()
	require.EqualValues(t, expectedUsed, *usage.Used)
}

func (f *Fs) testPolicyDiscovery(t *testing.T) {
	ctx := context.TODO()
	container := "testPolicyDiscovery-1"
	// Reset the policy so we can test if it is populated.
	f.opt.StoragePolicy = ""
	err := f.makeContainer(ctx, container)
	require.NoError(t, err)
	_, err = f.fetchStoragePolicy(ctx, container)
	require.NoError(t, err)

	// Default policy for SAIO image is 1replica.
	assert.Equal(t, "1replica", f.opt.StoragePolicy)

	// Create a container using a non-default policy, and check to ensure
	// that the created segments container uses the same non-default policy.
	policy := "Policy-1"
	container = "testPolicyDiscovery-2"

	f.opt.StoragePolicy = policy
	err = f.makeContainer(ctx, container)
	require.NoError(t, err)

	// Reset the policy so we can test if it is populated, and set to the
	// non-default policy.
	f.opt.StoragePolicy = ""
	_, err = f.fetchStoragePolicy(ctx, container)
	require.NoError(t, err)
	assert.Equal(t, policy, f.opt.StoragePolicy)

	// Test that when a segmented upload container is made, the newly
	// created container inherits the non-default policy of the base
	// container.
	f.opt.StoragePolicy = ""
	f.opt.UseSegmentsContainer.Value = true
	su, err := f.newSegmentedUpload(ctx, container, "")
	require.NoError(t, err)
	// The container name we expected?
	segmentsContainer := container + segmentsContainerSuffix
	assert.Equal(t, segmentsContainer, su.container)
	// The policy we expected?
	f.opt.StoragePolicy = ""
	_, err = f.fetchStoragePolicy(ctx, su.container)
	require.NoError(t, err)
	assert.Equal(t, policy, f.opt.StoragePolicy)
}

var _ fstests.InternalTester = (*Fs)(nil)
