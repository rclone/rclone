//go:build !plan9 && !solaris && !js

package azureblob

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/fstest/testserver"
	"github.com/rclone/rclone/lib/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockIDCreator(t *testing.T) {
	// Check creation and random number
	bic, err := newBlockIDCreator()
	require.NoError(t, err)
	bic2, err := newBlockIDCreator()
	require.NoError(t, err)
	assert.NotEqual(t, bic.random, bic2.random)
	assert.NotEqual(t, bic.random, [8]byte{})

	// Set random to known value for tests
	bic.random = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	chunkNumber := uint64(0xFEDCBA9876543210)

	// Check creation of ID
	want := base64.StdEncoding.EncodeToString([]byte{0xFE, 0xDC, 0xBA, 0x98, 0x76, 0x54, 0x32, 0x10, 1, 2, 3, 4, 5, 6, 7, 8})
	assert.Equal(t, "/ty6mHZUMhABAgMEBQYHCA==", want)
	got := bic.newBlockID(chunkNumber)
	assert.Equal(t, want, got)
	assert.Equal(t, "/ty6mHZUMhABAgMEBQYHCA==", got)

	// Test checkID is working
	assert.NoError(t, bic.checkID(chunkNumber, got))
	assert.ErrorContains(t, bic.checkID(chunkNumber, "$"+got), "illegal base64")
	assert.ErrorContains(t, bic.checkID(chunkNumber, "AAAA"+got), "bad block ID length")
	assert.ErrorContains(t, bic.checkID(chunkNumber+1, got), "expecting decoded")
	assert.ErrorContains(t, bic2.checkID(chunkNumber, got), "random bytes")
}

func (f *Fs) testFeatures(t *testing.T) {
	// Check first feature flags are set on this remote
	enabled := f.Features().SetTier
	assert.True(t, enabled)
	enabled = f.Features().GetTier
	assert.True(t, enabled)
}

type ReadSeekCloser struct {
	*strings.Reader
}

func (r *ReadSeekCloser) Close() error {
	return nil
}

// Stage a block at remote but don't commit it
func (f *Fs) stageBlockWithoutCommit(ctx context.Context, t *testing.T, remote string) {
	var (
		containerName, blobPath = f.split(remote)
		containerClient         = f.cntSVC(containerName)
		blobClient              = containerClient.NewBlockBlobClient(blobPath)
		data                    = "uncommitted data"
		blockID                 = "1"
		blockIDBase64           = base64.StdEncoding.EncodeToString([]byte(blockID))
	)
	r := &ReadSeekCloser{strings.NewReader(data)}
	_, err := blobClient.StageBlock(ctx, blockIDBase64, r, nil)
	require.NoError(t, err)

	// Verify the block is staged but not committed
	blockList, err := blobClient.GetBlockList(ctx, blockblob.BlockListTypeAll, nil)
	require.NoError(t, err)
	found := false
	for _, block := range blockList.UncommittedBlocks {
		if *block.Name == blockIDBase64 {
			found = true
			break
		}
	}
	require.True(t, found, "Block ID not found in uncommitted blocks")
}

// This tests uploading a blob where it has uncommitted blocks with a different ID size.
//
// https://gauravmantri.com/2013/05/18/windows-azure-blob-storage-dealing-with-the-specified-blob-or-block-content-is-invalid-error/
//
// TestIntegration/FsMkdir/FsPutFiles/Internal/WriteUncommittedBlocks
func (f *Fs) testWriteUncommittedBlocks(t *testing.T) {
	var (
		ctx    = context.Background()
		remote = "testBlob"
	)

	// Multipart copy the blob please
	oldUseCopyBlob, oldCopyCutoff := f.opt.UseCopyBlob, f.opt.CopyCutoff
	f.opt.UseCopyBlob = false
	f.opt.CopyCutoff = f.opt.ChunkSize
	defer func() {
		f.opt.UseCopyBlob, f.opt.CopyCutoff = oldUseCopyBlob, oldCopyCutoff
	}()

	// Create a blob with uncommitted blocks
	f.stageBlockWithoutCommit(ctx, t, remote)

	// Now attempt to overwrite the block with a different sized block ID to provoke this error

	// Check the object does not exist
	_, err := f.NewObject(ctx, remote)
	require.Equal(t, fs.ErrorObjectNotFound, err)

	// Upload a multipart file over the block with uncommitted chunks of a different ID size
	size := 4*int(f.opt.ChunkSize) - 1
	contents := random.String(size)
	item := fstest.NewItem(remote, contents, fstest.Time("2001-05-06T04:05:06.499Z"))
	o := fstests.PutTestContents(ctx, t, f, &item, contents, true)

	// Check size
	assert.Equal(t, int64(size), o.Size())

	// Create a new blob with uncommitted blocks
	newRemote := "testBlob2"
	f.stageBlockWithoutCommit(ctx, t, newRemote)

	// Copy over that block
	dst, err := f.Copy(ctx, o, newRemote)
	require.NoError(t, err)

	// Check basics
	assert.Equal(t, int64(size), dst.Size())
	assert.Equal(t, newRemote, dst.Remote())

	// Check contents
	gotContents := fstests.ReadObject(ctx, t, dst, -1)
	assert.Equal(t, contents, gotContents)

	// Remove the object
	require.NoError(t, dst.Remove(ctx))
}

func (f *Fs) InternalTest(t *testing.T) {
	t.Run("Features", f.testFeatures)
	t.Run("WriteUncommittedBlocks", f.testWriteUncommittedBlocks)
	t.Run("Metadata", f.testMetadataPaths)
}

// Standalone runner for metadata path tests to allow easy filtering with -run
func TestAzureMetadataPaths(t *testing.T) {
	remoteName := "TestAzureBlob:"
	fstest.Initialise()
	finish, err := testserver.Start(remoteName)
	require.NoError(t, err)
	defer finish()

	subRemoteName, _, err := fstest.RandomRemoteName(remoteName)
	require.NoError(t, err)
	fsi, err := fs.NewFs(context.Background(), subRemoteName)
	if err == fs.ErrorNotFoundInConfigFile {
		t.Skipf("Didn't find %q in config file - skipping tests", remoteName)
		return
	}
	require.NoError(t, err)
	f := fsi.(*Fs)
	f.testMetadataPaths(t)
}

// helper to read blob properties for an object
func getProps(ctx context.Context, t *testing.T, o fs.Object) *blob.GetPropertiesResponse {
	ao := o.(*Object)
	props, err := ao.readMetaDataAlways(ctx)
	require.NoError(t, err)
	return props
}

// helper to assert select headers and user metadata
func assertHeadersAndMetadata(t *testing.T, props *blob.GetPropertiesResponse, want map[string]string, wantUserMeta map[string]string) {
	// Headers
	get := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	if v, ok := want["content-type"]; ok {
		assert.Equal(t, v, get(props.ContentType), "content-type")
	}
	if v, ok := want["cache-control"]; ok {
		assert.Equal(t, v, get(props.CacheControl), "cache-control")
	}
	if v, ok := want["content-disposition"]; ok {
		assert.Equal(t, v, get(props.ContentDisposition), "content-disposition")
	}
	if v, ok := want["content-encoding"]; ok {
		assert.Equal(t, v, get(props.ContentEncoding), "content-encoding")
	}
	if v, ok := want["content-language"]; ok {
		assert.Equal(t, v, get(props.ContentLanguage), "content-language")
	}
	// User metadata (case-insensitive keys from service)
	norm := make(map[string]*string, len(props.Metadata))
	for kk, vv := range props.Metadata {
		norm[strings.ToLower(kk)] = vv
	}
	for k, v := range wantUserMeta {
		pv, ok := norm[strings.ToLower(k)]
		if assert.True(t, ok, fmt.Sprintf("missing user metadata key %q", k)) {
			if pv == nil {
				assert.Equal(t, v, "", k)
			} else {
				assert.Equal(t, v, *pv, k)
			}
		} else {
			// Log available keys for diagnostics
			keys := make([]string, 0, len(props.Metadata))
			for kk := range props.Metadata {
				keys = append(keys, kk)
			}
			t.Logf("available user metadata keys: %v", keys)
		}
	}
}

// helper to read blob tags for an object
func getTagsMap(ctx context.Context, t *testing.T, o fs.Object) map[string]string {
	ao := o.(*Object)
	blb := ao.getBlobSVC()
	resp, err := blb.GetTags(ctx, nil)
	require.NoError(t, err)
	out := make(map[string]string)
	for _, tag := range resp.BlobTagSet {
		if tag.Key != nil {
			k := *tag.Key
			v := ""
			if tag.Value != nil {
				v = *tag.Value
			}
			out[k] = v
		}
	}
	return out
}

// Test metadata across different write paths
func (f *Fs) testMetadataPaths(t *testing.T) {
	ctx := context.Background()
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	// Common expected metadata and headers
	baseMeta := fs.Metadata{
		"cache-control":       "no-cache",
		"content-disposition": "inline",
		"content-language":    "en-US",
		// Note: Don't set content-encoding here to avoid download decoding differences
		// We will set a custom user metadata key
		"potato": "royal",
		// and modtime
		"mtime": fstest.Time("2009-05-06T04:05:06.499999999Z").Format(time.RFC3339Nano),
	}

	// Singlepart upload
	t.Run("PutSinglepart", func(t *testing.T) {
		// size less than chunk size
		contents := random.String(int(f.opt.ChunkSize / 2))
		item := fstest.NewItem("meta-single.txt", contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
		// override content-type via metadata mapping
		meta := fs.Metadata{}
		meta.Merge(baseMeta)
		meta["content-type"] = "text/plain"
		obj := fstests.PutTestContentsMetadata(ctx, t, f, &item, true, contents, true, "text/html", meta)
		defer func() { _ = obj.Remove(ctx) }()

		props := getProps(ctx, t, obj)
		assertHeadersAndMetadata(t, props, map[string]string{
			"content-type":        "text/plain",
			"cache-control":       "no-cache",
			"content-disposition": "inline",
			"content-language":    "en-US",
		}, map[string]string{
			"potato": "royal",
		})
		_ = http.StatusOK // keep import for parity but don't inspect RawResponse
	})

	// Multipart upload
	t.Run("PutMultipart", func(t *testing.T) {
		// size greater than chunk size to force multipart
		contents := random.String(int(f.opt.ChunkSize + 1024))
		item := fstest.NewItem("meta-multipart.txt", contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
		meta := fs.Metadata{}
		meta.Merge(baseMeta)
		meta["content-type"] = "application/json"
		obj := fstests.PutTestContentsMetadata(ctx, t, f, &item, true, contents, true, "text/html", meta)
		defer func() { _ = obj.Remove(ctx) }()

		props := getProps(ctx, t, obj)
		assertHeadersAndMetadata(t, props, map[string]string{
			"content-type":        "application/json",
			"cache-control":       "no-cache",
			"content-disposition": "inline",
			"content-language":    "en-US",
		}, map[string]string{
			"potato": "royal",
		})

		// Tags: Singlepart upload
		t.Run("PutSinglepartTags", func(t *testing.T) {
			contents := random.String(int(f.opt.ChunkSize / 2))
			item := fstest.NewItem("tags-single.txt", contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
			meta := fs.Metadata{
				"x-ms-tags": "env=dev,team=sync",
			}
			obj := fstests.PutTestContentsMetadata(ctx, t, f, &item, true, contents, true, "text/plain", meta)
			defer func() { _ = obj.Remove(ctx) }()

			tags := getTagsMap(ctx, t, obj)
			assert.Equal(t, "dev", tags["env"])
			assert.Equal(t, "sync", tags["team"])
		})

		// Tags: Multipart upload
		t.Run("PutMultipartTags", func(t *testing.T) {
			contents := random.String(int(f.opt.ChunkSize + 2048))
			item := fstest.NewItem("tags-multipart.txt", contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
			meta := fs.Metadata{
				"x-ms-tags": "project=alpha,release=2025-08",
			}
			obj := fstests.PutTestContentsMetadata(ctx, t, f, &item, true, contents, true, "application/octet-stream", meta)
			defer func() { _ = obj.Remove(ctx) }()

			tags := getTagsMap(ctx, t, obj)
			assert.Equal(t, "alpha", tags["project"])
			assert.Equal(t, "2025-08", tags["release"])
		})
	})

	// Singlepart copy with metadata-set mapping; omit content-type to exercise fallback
	t.Run("CopySinglepart", func(t *testing.T) {
		// create small source
		contents := random.String(int(f.opt.ChunkSize / 2))
		srcItem := fstest.NewItem("meta-copy-single-src.txt", contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
		srcObj := fstests.PutTestContentsMetadata(ctx, t, f, &srcItem, true, contents, true, "text/plain", nil)
		defer func() { _ = srcObj.Remove(ctx) }()

		// set mapping via MetadataSet
		ctx2, ci := fs.AddConfig(ctx)
		ci.Metadata = true
		ci.MetadataSet = fs.Metadata{
			"cache-control":       "private, max-age=60",
			"content-disposition": "attachment; filename=foo.txt",
			"content-language":    "fr",
			// no content-type: should fallback to source
			"potato": "maris",
		}

		// do copy
		dstName := "meta-copy-single-dst.txt"
		dst, err := f.Copy(ctx2, srcObj, dstName)
		require.NoError(t, err)
		defer func() { _ = dst.Remove(ctx2) }()

		props := getProps(ctx2, t, dst)
		// content-type should fallback to source (text/plain)
		assertHeadersAndMetadata(t, props, map[string]string{
			"content-type":        "text/plain",
			"cache-control":       "private, max-age=60",
			"content-disposition": "attachment; filename=foo.txt",
			"content-language":    "fr",
		}, map[string]string{
			"potato": "maris",
		})
		// mtime should be populated on copy when --metadata is used
		// and should equal the source ModTime (RFC3339Nano)
		// Read user metadata (case-insensitive)
		m := props.Metadata
		var gotMtime string
		for k, v := range m {
			if strings.EqualFold(k, "mtime") && v != nil {
				gotMtime = *v
				break
			}
		}
		if assert.NotEmpty(t, gotMtime, "mtime not set on destination metadata") {
			// parse and compare times ignoring formatting differences
			parsed, err := time.Parse(time.RFC3339Nano, gotMtime)
			require.NoError(t, err)
			assert.True(t, srcObj.ModTime(ctx2).Equal(parsed), "dst mtime should equal src ModTime")
		}
	})

	// CopySinglepart with only --metadata (no MetadataSet) must inject mtime and preserve src content-type
	t.Run("CopySinglepart_MetadataOnly", func(t *testing.T) {
		contents := random.String(int(f.opt.ChunkSize / 2))
		srcItem := fstest.NewItem("meta-copy-single-only-src.txt", contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
		srcObj := fstests.PutTestContentsMetadata(ctx, t, f, &srcItem, true, contents, true, "text/plain", nil)
		defer func() { _ = srcObj.Remove(ctx) }()

		ctx2, ci := fs.AddConfig(ctx)
		ci.Metadata = true

		dstName := "meta-copy-single-only-dst.txt"
		dst, err := f.Copy(ctx2, srcObj, dstName)
		require.NoError(t, err)
		defer func() { _ = dst.Remove(ctx2) }()

		props := getProps(ctx2, t, dst)
		assertHeadersAndMetadata(t, props, map[string]string{
			"content-type": "text/plain",
		}, map[string]string{})
		// Assert mtime injected
		m := props.Metadata
		var gotMtime string
		for k, v := range m {
			if strings.EqualFold(k, "mtime") && v != nil {
				gotMtime = *v
				break
			}
		}
		if assert.NotEmpty(t, gotMtime, "mtime not set on destination metadata") {
			parsed, err := time.Parse(time.RFC3339Nano, gotMtime)
			require.NoError(t, err)
			assert.True(t, srcObj.ModTime(ctx2).Equal(parsed), "dst mtime should equal src ModTime")
		}
	})

	// Multipart copy with metadata-set mapping; omit content-type to exercise fallback
	t.Run("CopyMultipart", func(t *testing.T) {
		// create large source to force multipart
		contents := random.String(int(f.opt.CopyCutoff + 1024))
		srcItem := fstest.NewItem("meta-copy-multi-src.txt", contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
		srcObj := fstests.PutTestContentsMetadata(ctx, t, f, &srcItem, true, contents, true, "application/octet-stream", nil)
		defer func() { _ = srcObj.Remove(ctx) }()

		// set mapping via MetadataSet
		ctx2, ci := fs.AddConfig(ctx)
		ci.Metadata = true
		ci.MetadataSet = fs.Metadata{
			"cache-control": "max-age=0, no-cache",
			// omit content-type to trigger fallback
			"content-language": "de",
			"potato":           "desiree",
		}

		dstName := "meta-copy-multi-dst.txt"
		dst, err := f.Copy(ctx2, srcObj, dstName)
		require.NoError(t, err)
		defer func() { _ = dst.Remove(ctx2) }()

		props := getProps(ctx2, t, dst)
		// content-type should fallback to source (application/octet-stream)
		assertHeadersAndMetadata(t, props, map[string]string{
			"content-type":     "application/octet-stream",
			"cache-control":    "max-age=0, no-cache",
			"content-language": "de",
		}, map[string]string{
			"potato": "desiree",
		})
		// mtime should be populated on copy when --metadata is used
		m := props.Metadata
		var gotMtime string
		for k, v := range m {
			if strings.EqualFold(k, "mtime") && v != nil {
				gotMtime = *v
				break
			}
		}
		if assert.NotEmpty(t, gotMtime, "mtime not set on destination metadata") {
			parsed, err := time.Parse(time.RFC3339Nano, gotMtime)
			require.NoError(t, err)
			assert.True(t, srcObj.ModTime(ctx2).Equal(parsed), "dst mtime should equal src ModTime")
		}
	})

	// CopyMultipart with only --metadata must inject mtime and preserve src content-type
	t.Run("CopyMultipart_MetadataOnly", func(t *testing.T) {
		contents := random.String(int(f.opt.CopyCutoff + 2048))
		srcItem := fstest.NewItem("meta-copy-multi-only-src.txt", contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
		srcObj := fstests.PutTestContentsMetadata(ctx, t, f, &srcItem, true, contents, true, "application/octet-stream", nil)
		defer func() { _ = srcObj.Remove(ctx) }()

		ctx2, ci := fs.AddConfig(ctx)
		ci.Metadata = true

		dstName := "meta-copy-multi-only-dst.txt"
		dst, err := f.Copy(ctx2, srcObj, dstName)
		require.NoError(t, err)
		defer func() { _ = dst.Remove(ctx2) }()

		props := getProps(ctx2, t, dst)
		assertHeadersAndMetadata(t, props, map[string]string{
			"content-type": "application/octet-stream",
		}, map[string]string{})
		m := props.Metadata
		var gotMtime string
		for k, v := range m {
			if strings.EqualFold(k, "mtime") && v != nil {
				gotMtime = *v
				break
			}
		}
		if assert.NotEmpty(t, gotMtime, "mtime not set on destination metadata") {
			parsed, err := time.Parse(time.RFC3339Nano, gotMtime)
			require.NoError(t, err)
			assert.True(t, srcObj.ModTime(ctx2).Equal(parsed), "dst mtime should equal src ModTime")
		}
	})

	// Tags: Singlepart copy
	t.Run("CopySinglepartTags", func(t *testing.T) {
		// create small source
		contents := random.String(int(f.opt.ChunkSize / 2))
		srcItem := fstest.NewItem("tags-copy-single-src.txt", contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
		srcObj := fstests.PutTestContentsMetadata(ctx, t, f, &srcItem, true, contents, true, "text/plain", nil)
		defer func() { _ = srcObj.Remove(ctx) }()

		// set mapping via MetadataSet including tags
		ctx2, ci := fs.AddConfig(ctx)
		ci.Metadata = true
		ci.MetadataSet = fs.Metadata{
			"x-ms-tags": "copy=single,mode=test",
		}

		dstName := "tags-copy-single-dst.txt"
		dst, err := f.Copy(ctx2, srcObj, dstName)
		require.NoError(t, err)
		defer func() { _ = dst.Remove(ctx2) }()

		tags := getTagsMap(ctx2, t, dst)
		assert.Equal(t, "single", tags["copy"])
		assert.Equal(t, "test", tags["mode"])
	})

	// Tags: Multipart copy
	t.Run("CopyMultipartTags", func(t *testing.T) {
		// create large source to force multipart
		contents := random.String(int(f.opt.CopyCutoff + 4096))
		srcItem := fstest.NewItem("tags-copy-multi-src.txt", contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
		srcObj := fstests.PutTestContentsMetadata(ctx, t, f, &srcItem, true, contents, true, "application/octet-stream", nil)
		defer func() { _ = srcObj.Remove(ctx) }()

		ctx2, ci := fs.AddConfig(ctx)
		ci.Metadata = true
		ci.MetadataSet = fs.Metadata{
			"x-ms-tags": "copy=multi,mode=test",
		}

		dstName := "tags-copy-multi-dst.txt"
		dst, err := f.Copy(ctx2, srcObj, dstName)
		require.NoError(t, err)
		defer func() { _ = dst.Remove(ctx2) }()

		tags := getTagsMap(ctx2, t, dst)
		assert.Equal(t, "multi", tags["copy"])
		assert.Equal(t, "test", tags["mode"])
	})

	// Negative: invalid x-ms-tags must error
	t.Run("InvalidXMsTags", func(t *testing.T) {
		contents := random.String(32)
		item := fstest.NewItem("tags-invalid.txt", contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
		// construct ObjectInfo with invalid x-ms-tags
		buf := strings.NewReader(contents)
		// Build obj info with metadata
		meta := fs.Metadata{
			"x-ms-tags": "badpair-without-equals",
		}
		// force metadata on
		ctx2, ci := fs.AddConfig(ctx)
		ci.Metadata = true
		obji := object.NewStaticObjectInfo(item.Path, item.ModTime, int64(len(contents)), true, nil, nil)
		obji = obji.WithMetadata(meta).WithMimeType("text/plain")
		_, err := f.Put(ctx2, buf, obji)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid tag")
	})
}
