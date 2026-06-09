package operations

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fstest/mockfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type checksumTestObject struct {
	name    string
	size    int64
	modTime time.Time
	f       fs.Fs
	hashVal string
}

func (o *checksumTestObject) Fs() fs.Info    { return o.f }
func (o *checksumTestObject) Remote() string { return o.name }
func (o *checksumTestObject) String() string { return o.name }

func (o *checksumTestObject) Hash(_ context.Context, _ hash.Type) (string, error) {
	return o.hashVal, nil
}

func (o *checksumTestObject) ModTime(_ context.Context) time.Time { return o.modTime }
func (o *checksumTestObject) Size() int64                         { return o.size }
func (o *checksumTestObject) Storable() bool                      { return true }

func (o *checksumTestObject) SetModTime(_ context.Context, _ time.Time) error {
	return errors.New("not implemented")
}

func (o *checksumTestObject) Open(_ context.Context, _ ...fs.OpenOption) (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}

func (o *checksumTestObject) Update(_ context.Context, _ io.Reader, _ fs.ObjectInfo, _ ...fs.OpenOption) error {
	return errors.New("not implemented")
}

func (o *checksumTestObject) Remove(_ context.Context) error {
	return errors.New("not implemented")
}

func TestEqualChecksumNoFallback(t *testing.T) {
	ctx := context.Background()

	srcFsI, err := mockfs.NewFs(ctx, "src", "", nil)
	require.NoError(t, err)
	dstFsI, err := mockfs.NewFs(ctx, "dst", "", nil)
	require.NoError(t, err)
	srcFs := srcFsI.(*mockfs.Fs)
	dstFs := dstFsI.(*mockfs.Fs)

	now := time.Now().Truncate(time.Second)
	src := &checksumTestObject{name: "file.txt", size: 10, modTime: now, f: srcFs}
	dst := &checksumTestObject{name: "file.txt", size: 10, modTime: now, f: dstFs}

	// When the source and destination cannot agree on a hash type to
	// compare, rclone normally falls back to comparing file sizes only.
	// A file that matches by size is treated as already up to date and
	// not re-uploaded.
	t.Run("no common hash type without no-fallback returns true", func(t *testing.T) {
		srcFs.SetHashes(hash.NewHashSet(hash.MD5))
		dstFs.SetHashes(hash.NewHashSet(hash.SHA1))

		opt := equalOpt{checkSum: true, checksumNoFallback: false}
		assert.True(t, equal(ctx, src, dst, opt))
	})

	// When the user has asked for strict checksum verification but no
	// shared hash type exists between source and destination, rclone cannot
	// actually verify the file. Rather than silently passing the file as equal,
	// the run should refuse to verify it.
	// The file is not transferred and the run reports an error so the
	// user knows verification could not be performed.
	t.Run("no common hash type with no-fallback skips and errors", func(t *testing.T) {
		srcFs.SetHashes(hash.NewHashSet(hash.MD5))
		dstFs.SetHashes(hash.NewHashSet(hash.SHA1))

		opt := equalOpt{checkSum: true, checksumNoFallback: true}
		assert.True(t, equal(ctx, src, dst, opt))
	})

	// Both source and destination support the same hash type and the
	// hash values match. This is the normal "everything is fine" case and the
	// flag should not interfere with it.
	// The file is recognised as already up to date and is not
	// re-uploaded.
	t.Run("matching hashes with no-fallback returns true", func(t *testing.T) {
		srcFs.SetHashes(hash.NewHashSet(hash.MD5))
		dstFs.SetHashes(hash.NewHashSet(hash.MD5))
		src.hashVal = "abc123"
		dst.hashVal = "abc123"

		opt := equalOpt{checkSum: true, checksumNoFallback: true}
		assert.True(t, equal(ctx, src, dst, opt))
	})

	// A file already on the destination has no checksum recorded against it,
	// even though the destination supports the same hash type as the source.
	// This is a common state when a file was previously uploaded outside
	// rclone, or when the hasher cache lost its entry.
	// The file is re-uploaded so the destination ends up with a verified
	// checksum recorded against it.
	t.Run("empty dst hash with no-fallback returns false", func(t *testing.T) {
		srcFs.SetHashes(hash.NewHashSet(hash.MD5))
		dstFs.SetHashes(hash.NewHashSet(hash.MD5))
		src.hashVal = "abc123"
		dst.hashVal = ""

		opt := equalOpt{checkSum: true, checksumNoFallback: true}
		assert.False(t, equal(ctx, src, dst, opt))
	})

	// The source side has no hash value, even though both source and
	// destination support the same hash type. Re-uploading would not help
	// here because the source would still be unable to produce a hash on
	// the next run, leaving the file forever unverifiable.
	// The file is not transferred and an error is reported so the user
	// knows it was not verified.
	t.Run("empty src hash with no-fallback skips and errors", func(t *testing.T) {
		srcFs.SetHashes(hash.NewHashSet(hash.MD5))
		dstFs.SetHashes(hash.NewHashSet(hash.MD5))
		src.hashVal = ""
		dst.hashVal = "abc123"

		opt := equalOpt{checkSum: true, checksumNoFallback: true}
		assert.True(t, equal(ctx, src, dst, opt))
		src.hashVal = "abc123"
	})

	// Both source and destination have hash values recorded for the
	// file, but the values are different. This means the contents have
	// genuinely changed.
	// The file is re-uploaded because the contents do not match.
	t.Run("both hashes populated and differing with no-fallback returns false", func(t *testing.T) {
		srcFs.SetHashes(hash.NewHashSet(hash.MD5))
		dstFs.SetHashes(hash.NewHashSet(hash.MD5))
		src.hashVal = "abc123"
		dst.hashVal = "different"

		opt := equalOpt{checkSum: true, checksumNoFallback: true}
		assert.False(t, equal(ctx, src, dst, opt))
	})

	// A user has enabled --checksum-no-fallback but forgotten to also
	// enable --checksum. The flag is documented as only meaningful alongside
	// --checksum, so on its own it should change nothing.
	// Comparison falls back to default behaviour and a file that matches
	// by size and modtime is treated as up to date.
	t.Run("no-fallback without --checksum is inert", func(t *testing.T) {
		srcFs.SetHashes(hash.NewHashSet(hash.MD5))
		dstFs.SetHashes(hash.NewHashSet(hash.SHA1))
		src.hashVal = "abc123"
		dst.hashVal = ""
		src.modTime = now
		dst.modTime = now

		opt := equalOpt{checkSum: false, checksumNoFallback: true}
		assert.True(t, equal(ctx, src, dst, opt))
	})

	// When a file cannot be verified because no shared hash type
	// exists, the user needs to see that something went wrong. Skipping
	// silently would let unverifiable files pile up unnoticed and would make
	// the run appear successful when it was not.
	// For each file that cannot be verified, the user sees one error
	// reported and the overall run finishes with a non-zero exit code.
	t.Run("no common hash with no-fallback increments error counter", func(t *testing.T) {
		ctx2, _ := fs.AddConfig(context.Background())
		stats := accounting.Stats(ctx2)
		before := stats.GetErrors()

		srcFs.SetHashes(hash.NewHashSet(hash.MD5))
		dstFs.SetHashes(hash.NewHashSet(hash.SHA1))
		src.modTime = now
		dst.modTime = now

		opt := equalOpt{checkSum: true, checksumNoFallback: true}
		assert.True(t, equal(ctx2, src, dst, opt))
		assert.Equal(t, before+1, stats.GetErrors(), "expected one error to be counted")
	})
}

func TestNeedTransferChecksumUploadMissing(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)

	srcFsI, err := mockfs.NewFs(ctx, "src", "", nil)
	require.NoError(t, err)
	dstFsI, err := mockfs.NewFs(ctx, "dst", "", nil)
	require.NoError(t, err)
	srcFs := srcFsI.(*mockfs.Fs)
	dstFs := dstFsI.(*mockfs.Fs)

	// Use no common hash type to match a real local-to-hasher scenario.
	srcFs.SetHashes(hash.NewHashSet(hash.SHA1))
	dstFs.SetHashes(hash.NewHashSet(hash.BLAKE3))

	now := time.Now().Truncate(time.Second)
	src := &checksumTestObject{name: "file.txt", size: 10, modTime: now, f: srcFs, hashVal: "abc123"}
	dst := &checksumTestObject{name: "file.txt", size: 10, modTime: now, f: dstFs, hashVal: ""}

	// A file already exists on the destination but has no checksum
	// recorded against it. This is the typical state when the file was
	// uploaded outside rclone (e.g. via a desktop client or browser), and is
	// exactly what this flag is designed to fix.
	// The file is uploaded again so the destination ends up with a
	// checksum recorded against it.
	t.Run("dst missing checksum with flag enabled triggers transfer", func(t *testing.T) {
		ci.CheckSum = true
		ci.ChecksumUploadMissing = true
		assert.True(t, NeedTransfer(ctx, dst, src))
	})

	// A user who has not enabled --checksum-upload-missing should not
	// suddenly start seeing files re-uploaded just because the destination is
	// missing a checksum. The flag must be opt-in.
	// Files are left alone and the user does not see unexpected
	// transfers.
	t.Run("dst missing checksum with flag disabled skips transfer", func(t *testing.T) {
		ci.CheckSum = true
		ci.ChecksumUploadMissing = false
		assert.False(t, NeedTransfer(ctx, dst, src))
	})

	// A user enables --checksum-upload-missing but forgets to also
	// enable --checksum. The flag is documented as only meaningful alongside
	// --checksum, so on its own it should change nothing.
	// The flag has no effect and normal comparison decides whether the
	// file is transferred.
	t.Run("dst missing checksum without --checksum active skips transfer", func(t *testing.T) {
		ci.CheckSum = false
		ci.ChecksumUploadMissing = true
		assert.False(t, NeedTransfer(ctx, dst, src))
		ci.CheckSum = true
	})

	// A destination file already has a checksum recorded against it.
	// The flag should only fire for files that lack a checksum, not for files
	// that already have one.
	// Files with a recorded checksum are left alone, avoiding wasteful
	// uploads.
	t.Run("dst has checksum with flag enabled skips transfer", func(t *testing.T) {
		ci.CheckSum = true
		ci.ChecksumUploadMissing = true
		dst.hashVal = "abc123"
		assert.False(t, NeedTransfer(ctx, dst, src))
		dst.hashVal = ""
	})

	// The destination is a backend that does not support storing
	// hashes at all. Asking it for a hash is meaningless, so the flag should
	// quietly do nothing rather than try to re-upload every file forever.
	// No files are re-uploaded; the flag has no effect on backends that
	// cannot store hashes.
	t.Run("dst backend supports no hashes is a no-op", func(t *testing.T) {
		ci.CheckSum = true
		ci.ChecksumUploadMissing = true
		dstFs.SetHashes(hash.NewHashSet())
		assert.False(t, NeedTransfer(ctx, dst, src))
		dstFs.SetHashes(hash.NewHashSet(hash.BLAKE3))
	})
}

func TestEqualChecksumIncludeModtime(t *testing.T) {
	ctx := context.Background()

	srcFsI, err := mockfs.NewFs(ctx, "src", "", nil)
	require.NoError(t, err)
	dstFsI, err := mockfs.NewFs(ctx, "dst", "", nil)
	require.NoError(t, err)
	srcFs := srcFsI.(*mockfs.Fs)
	dstFs := dstFsI.(*mockfs.Fs)

	srcFs.SetHashes(hash.NewHashSet(hash.MD5))
	dstFs.SetHashes(hash.NewHashSet(hash.MD5))

	now := time.Now().Truncate(time.Second)
	src := &checksumTestObject{name: "file.txt", size: 10, modTime: now, f: srcFs}
	dst := &checksumTestObject{name: "file.txt", size: 10, modTime: now, f: dstFs}

	// Size, modtime, and checksum all agree between source and
	// destination. This is the everything-is-fine case the flag is designed
	// around.
	// The file is recognised as already up to date and not re-uploaded.
	t.Run("size modtime and checksum all match returns true", func(t *testing.T) {
		src.hashVal = "abc123"
		dst.hashVal = "abc123"

		opt := equalOpt{checkSum: true, checksumIncludeModtime: true}
		assert.True(t, equal(ctx, src, dst, opt))
	})

	// Sizes and hashes would match, but the modtimes are different.
	// The whole purpose of the flag is to include modtime as a required check,
	// so a modtime difference must count as a mismatch.
	// The file is re-uploaded so the destination modtime is corrected.
	t.Run("modtime differs returns false", func(t *testing.T) {
		dst.modTime = now.Add(-time.Hour)

		opt := equalOpt{checkSum: true, checksumIncludeModtime: true}
		assert.False(t, equal(ctx, src, dst, opt))
		dst.modTime = now
	})

	// Sizes and modtimes match but the checksums differ. Adding
	// modtime as an extra check must not let a checksum mismatch slip
	// through.
	// The file is re-uploaded because the contents differ.
	t.Run("checksum differs returns false", func(t *testing.T) {
		src.hashVal = "abc123"
		dst.hashVal = "different"

		opt := equalOpt{checkSum: true, checksumIncludeModtime: true}
		assert.False(t, equal(ctx, src, dst, opt))
	})

	// Source and destination cannot agree on a hash type to compare,
	// and the user has not enabled --checksum-no-fallback. The checksum step
	// cannot run, so the file's identity must be decided on the remaining
	// checks (size and modtime).
	// A file matching by size and modtime is treated as up to date even
	// though the checksum could not be compared.
	t.Run("no common hash type skips checksum step returns true", func(t *testing.T) {
		srcFs.SetHashes(hash.NewHashSet(hash.MD5))
		dstFs.SetHashes(hash.NewHashSet(hash.SHA1))
		src.modTime = now
		dst.modTime = now

		opt := equalOpt{checkSum: true, checksumIncludeModtime: true}
		assert.True(t, equal(ctx, src, dst, opt))
		srcFs.SetHashes(hash.NewHashSet(hash.MD5))
		dstFs.SetHashes(hash.NewHashSet(hash.MD5))
	})

	// The destination is a backend that does not record modtimes at
	// all. The modtime step cannot be performed against such a backend, so it
	// should be quietly skipped rather than treating every file as different.
	// A file with matching checksums is treated as up to date even
	// though the destination cannot report a modtime.
	t.Run("no modtime support skips modtime step and checks hash", func(t *testing.T) {
		dstFs.SetPrecision(fs.ModTimeNotSupported)
		srcFs.SetHashes(hash.NewHashSet(hash.MD5))
		dstFs.SetHashes(hash.NewHashSet(hash.MD5))
		src.hashVal = "abc123"
		dst.hashVal = "abc123"
		src.modTime = now
		dst.modTime = now.Add(-time.Hour)

		opt := equalOpt{checkSum: true, checksumIncludeModtime: true}
		assert.True(t, equal(ctx, src, dst, opt))

		dstFs.SetPrecision(time.Second)
		dst.modTime = now
	})

	// The file's size differs between source and destination. Size
	// mismatch is the strongest signal that the contents have changed, and
	// the flag should not weaken that.
	// The file is re-uploaded because the sizes prove the contents
	// have changed.
	t.Run("size differs returns false with include-modtime", func(t *testing.T) {
		srcFs.SetHashes(hash.NewHashSet(hash.MD5))
		dstFs.SetHashes(hash.NewHashSet(hash.MD5))
		src.hashVal = "abc123"
		dst.hashVal = "abc123"
		src.modTime = now
		dst.modTime = now
		dst.size = 999

		opt := equalOpt{checkSum: true, checksumIncludeModtime: true}
		assert.False(t, equal(ctx, src, dst, opt))
		dst.size = src.size
	})

	// A user enables --checksum-include-modtime but forgets to also
	// enable --checksum. The flag is documented as only meaningful alongside
	// --checksum, so on its own it should change nothing.
	// Comparison falls back to default behaviour and a file that
	// matches by size and modtime is treated as up to date, regardless of
	// any hash values present on the objects.
	t.Run("include-modtime without --checksum is inert", func(t *testing.T) {
		srcFs.SetHashes(hash.NewHashSet(hash.MD5))
		dstFs.SetHashes(hash.NewHashSet(hash.MD5))
		src.hashVal = "abc123"
		dst.hashVal = "different"
		src.modTime = now
		dst.modTime = now

		opt := equalOpt{checkSum: false, checksumIncludeModtime: true}
		assert.True(t, equal(ctx, src, dst, opt))
	})

	// A user has enabled both flags together. The destination has no
	// checksum recorded for the file, even though size and modtime match.
	// Combining the flags should still catch this and re-upload the file.
	// The file is re-uploaded so the destination acquires a verified
	// checksum.
	t.Run("checksum-include-modtime with no-fallback and empty dst hash returns false", func(t *testing.T) {
		srcFs.SetHashes(hash.NewHashSet(hash.MD5))
		dstFs.SetHashes(hash.NewHashSet(hash.MD5))
		src.hashVal = "abc123"
		dst.hashVal = ""
		src.modTime = now
		dst.modTime = now

		opt := equalOpt{checkSum: true, checksumIncludeModtime: true, checksumNoFallback: true}
		assert.False(t, equal(ctx, src, dst, opt))
	})

	// A user has enabled both flags together. Source and destination
	// cannot agree on a hash type, so the file cannot be verified. The
	// combination should still refuse to weaken to size and modtime alone.
	// The file is not transferred and the user sees an error reported
	// so they know verification could not be done.
	t.Run("checksum-include-modtime with no-fallback and no common hash skips", func(t *testing.T) {
		srcFs.SetHashes(hash.NewHashSet(hash.MD5))
		dstFs.SetHashes(hash.NewHashSet(hash.SHA1))
		src.hashVal = "abc123"
		dst.hashVal = "abc123"
		src.modTime = now
		dst.modTime = now

		opt := equalOpt{checkSum: true, checksumIncludeModtime: true, checksumNoFallback: true}
		assert.True(t, equal(ctx, src, dst, opt))
		srcFs.SetHashes(hash.NewHashSet(hash.MD5))
		dstFs.SetHashes(hash.NewHashSet(hash.MD5))
	})
}

func TestNeedTransferAllChecksumFlagsCombined(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)

	srcFsI, err := mockfs.NewFs(ctx, "src", "", nil)
	require.NoError(t, err)
	dstFsI, err := mockfs.NewFs(ctx, "dst", "", nil)
	require.NoError(t, err)
	srcFs := srcFsI.(*mockfs.Fs)
	dstFs := dstFsI.(*mockfs.Fs)

	srcFs.SetHashes(hash.NewHashSet(hash.MD5))
	dstFs.SetHashes(hash.NewHashSet(hash.MD5))

	now := time.Now().Truncate(time.Second)
	src := &checksumTestObject{name: "file.txt", size: 10, modTime: now, f: srcFs, hashVal: "abc123"}
	dst := &checksumTestObject{name: "file.txt", size: 10, modTime: now, f: dstFs, hashVal: "abc123"}

	ci.CheckSum = true
	ci.ChecksumNoFallback = true
	ci.ChecksumUploadMissing = true
	ci.ChecksumIncludeModtime = true

	// A user has enabled all four data-integrity flags together and
	// the file genuinely matches on size, modtime, and checksum. The combined
	// mode should recognise this and not waste bandwidth.
	// The file is left alone, no upload is triggered.
	t.Run("all flags on with everything matching skips transfer", func(t *testing.T) {
		assert.False(t, NeedTransfer(ctx, dst, src))
	})

	// With all four flags on, the destination has matching size and
	// modtime but no checksum recorded. A missing checksum should take
	// priority over matching size and modtime, otherwise such files would
	// silently stay unverified forever.
	// The file is re-uploaded so the destination acquires a checksum.
	t.Run("empty dst hash triggers upload-missing before checksum-include-modtime runs", func(t *testing.T) {
		dst.hashVal = ""
		assert.True(t, NeedTransfer(ctx, dst, src))
		dst.hashVal = "abc123"
	})

	// With all four flags on, the modtime differs between source and
	// destination. The include-modtime check should still fire and the other
	// flags should not weaken it.
	// The file is re-uploaded so the destination modtime is corrected.
	t.Run("modtime differs fails checksum-include-modtime", func(t *testing.T) {
		dst.modTime = now.Add(-time.Hour)
		assert.True(t, NeedTransfer(ctx, dst, src))
		dst.modTime = now
	})

	// With all four flags on, the checksums differ between source and
	// destination, meaning the contents really have changed.
	// The file is re-uploaded because its contents differ.
	t.Run("checksum differs fails checksum-include-modtime", func(t *testing.T) {
		dst.hashVal = "different"
		assert.True(t, NeedTransfer(ctx, dst, src))
		dst.hashVal = "abc123"
	})
}