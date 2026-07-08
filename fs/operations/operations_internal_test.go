// Internal tests for operations

package operations

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/stretchr/testify/assert"
)

// equalTestInfo is a minimal fs.Info implementation for testing equal().
// It has configurable modtime precision and hash support.
type equalTestInfo struct {
	precision time.Duration
	hashes    hash.Set
}

func (f *equalTestInfo) Name() string             { return "equalTestFs" }
func (f *equalTestInfo) Root() string             { return "" }
func (f *equalTestInfo) String() string           { return "equalTestFs:" }
func (f *equalTestInfo) Precision() time.Duration { return f.precision }
func (f *equalTestInfo) Hashes() hash.Set         { return f.hashes }
func (f *equalTestInfo) Features() *fs.Features   { return &fs.Features{} }

// equalTestObject is a minimal fs.Object implementation for testing equal().
type equalTestObject struct {
	info    *equalTestInfo
	size    int64
	modTime time.Time
	hashes  map[hash.Type]string
}

func (o *equalTestObject) Fs() fs.Info                         { return o.info }
func (o *equalTestObject) Remote() string                      { return "test" }
func (o *equalTestObject) String() string                      { return "test" }
func (o *equalTestObject) ModTime(_ context.Context) time.Time { return o.modTime }
func (o *equalTestObject) Size() int64                         { return o.size }
func (o *equalTestObject) Storable() bool                      { return true }
func (o *equalTestObject) Hash(_ context.Context, ty hash.Type) (string, error) {
	return o.hashes[ty], nil
}

// fs.Object methods — not exercised by the equal() test paths below, but
// required to satisfy the interface.
func (o *equalTestObject) SetModTime(_ context.Context, _ time.Time) error {
	return fs.ErrorCantSetModTime
}
func (o *equalTestObject) Open(_ context.Context, _ ...fs.OpenOption) (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}
func (o *equalTestObject) Update(_ context.Context, _ io.Reader, _ fs.ObjectInfo, _ ...fs.OpenOption) error {
	return errors.New("not implemented")
}
func (o *equalTestObject) Remove(_ context.Context) error {
	return errors.New("not implemented")
}

func TestSizeDiffers(t *testing.T) {
	ctx := context.Background()
	ci := fs.GetConfig(ctx)
	when := time.Now()
	for _, test := range []struct {
		ignoreSize bool
		srcSize    int64
		dstSize    int64
		want       bool
	}{
		{false, 0, 0, false},
		{false, 1, 2, true},
		{false, 1, -1, false},
		{false, -1, 1, false},
		{true, 0, 0, false},
		{true, 1, 2, false},
		{true, 1, -1, false},
		{true, -1, 1, false},
	} {
		src := object.NewStaticObjectInfo("a", when, test.srcSize, true, nil, nil)
		dst := object.NewStaticObjectInfo("a", when, test.dstSize, true, nil, nil)
		oldIgnoreSize := ci.IgnoreSize
		ci.IgnoreSize = test.ignoreSize
		got := sizeDiffers(ctx, src, dst)
		ci.IgnoreSize = oldIgnoreSize
		assert.Equal(t, test.want, got, fmt.Sprintf("ignoreSize=%v, srcSize=%v, dstSize=%v", test.ignoreSize, test.srcSize, test.dstSize))
	}
}

// TestEqualModTimeNotSupported tests the behaviour of equal() when the
// destination backend does not support modtime (e.g. Google Photos via hasher).
//
// Without --checksum, the old code returned "Sizes identical" / equal=true for
// any two files with the same size, even if their content differed. The fix
// falls through to CheckHashes when a common hash type is available, so content
// changes are caught automatically without requiring --checksum.
//
// Branch coverage for the new logic in equal():
//
//	A. ModTimeNotSupported, no common hash         → equal  (size-only, old behaviour preserved)
//	B. ModTimeNotSupported, common hash, same hash → equal  (hash confirms match)
//	C. ModTimeNotSupported, common hash, diff hash → differ (THE BUG FIX)
func TestEqualModTimeNotSupported(t *testing.T) {
	ctx := context.Background()
	// updateModTime=false so SetModTime is never called on our stub object.
	opt := equalOpt{checkSum: false, sizeOnly: false, updateModTime: false}

	newObj := func(info *equalTestInfo, size int64, h map[hash.Type]string) *equalTestObject {
		return &equalTestObject{info: info, size: size, hashes: h}
	}
	withMD5 := &equalTestInfo{
		precision: fs.ModTimeNotSupported,
		hashes:    hash.NewHashSet(hash.MD5),
	}
	noHash := &equalTestInfo{
		precision: fs.ModTimeNotSupported,
		hashes:    hash.NewHashSet(), // no hash support
	}

	// Branch A: no common hash → size-only fallback (existing behaviour preserved)
	t.Run("NoCommonHash_SameSize_Equal", func(t *testing.T) {
		src := newObj(withMD5, 100, map[hash.Type]string{hash.MD5: "aaa"})
		dst := newObj(noHash, 100, map[hash.Type]string{})
		assert.True(t, equal(ctx, src, dst, opt),
			"same size, no common hash: should be treated as equal (size-only fallback)")
	})

	// Branch B: common hash, hashes match → equal
	t.Run("CommonHash_SameContent_Equal", func(t *testing.T) {
		src := newObj(withMD5, 100, map[hash.Type]string{hash.MD5: "abc123"})
		dst := newObj(withMD5, 100, map[hash.Type]string{hash.MD5: "abc123"})
		assert.True(t, equal(ctx, src, dst, opt),
			"same size + same MD5: should be equal")
	})

	// Branch C: common hash, hashes DIFFER → must detect as not equal.
	// Before the fix, equal() returned true here because it short-circuited at
	// "Sizes identical" without ever reaching the hash comparison.
	t.Run("CommonHash_DifferentContent_NotEqual", func(t *testing.T) {
		src := newObj(withMD5, 100, map[hash.Type]string{hash.MD5: "abc123"})
		dst := newObj(withMD5, 100, map[hash.Type]string{hash.MD5: "def456"})
		assert.False(t, equal(ctx, src, dst, opt),
			"same size but different MD5: must NOT be equal (content changed)")
	})

	// Sanity: different sizes always detected regardless of hash support.
	t.Run("DifferentSize_NotEqual", func(t *testing.T) {
		src := newObj(withMD5, 100, map[hash.Type]string{hash.MD5: "abc123"})
		dst := newObj(withMD5, 200, map[hash.Type]string{hash.MD5: "abc123"})
		assert.False(t, equal(ctx, src, dst, opt),
			"different sizes: must not be equal")
	})

	// Branch D: ModTimeNotSupported, common hash, same hash, updateModTime=true -> equal (should NOT call SetModTime / return false)
	t.Run("CommonHash_SameContent_UpdateModTime_Equal", func(t *testing.T) {
		src := newObj(withMD5, 100, map[hash.Type]string{hash.MD5: "abc123"})
		dst := newObj(withMD5, 100, map[hash.Type]string{hash.MD5: "abc123"})
		src.modTime = time.Now()
		dst.modTime = src.modTime.Add(1 * time.Hour)

		optWithUpdate := equalOpt{checkSum: false, sizeOnly: false, updateModTime: true}
		assert.True(t, equal(ctx, src, dst, optWithUpdate),
			"same size + same MD5 even with updateModTime=true: should be equal when modtime is unsupported")
	})
}
