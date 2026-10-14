//go:build !plan9

package archive

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// squashfsSuperblock builds a minimal 96-byte squashfs superblock with the
// given attacker-controllable fields. It mirrors the on-disk layout the
// go-diskfs parser reads, and is only used to construct deliberately malformed
// images for the tests below.
func squashfsSuperblock(blockSize uint32, blockLog, compression uint16, rootInode, inodeTableStart uint64) []byte {
	const u64max = ^uint64(0)
	b := make([]byte, 96)
	binary.LittleEndian.PutUint32(b[0:], 0x73717368) // "hsqs" magic
	binary.LittleEndian.PutUint32(b[4:], 1)          // inode count
	binary.LittleEndian.PutUint32(b[8:], 0)          // modtime
	binary.LittleEndian.PutUint32(b[12:], blockSize)
	binary.LittleEndian.PutUint32(b[16:], 0) // fragment count
	binary.LittleEndian.PutUint16(b[20:], compression)
	binary.LittleEndian.PutUint16(b[22:], blockLog)
	binary.LittleEndian.PutUint16(b[24:], 0x0210) // no fragments/xattrs
	binary.LittleEndian.PutUint16(b[26:], 0)      // id count
	binary.LittleEndian.PutUint16(b[28:], 4)      // version major
	binary.LittleEndian.PutUint16(b[30:], 0)      // version minor
	binary.LittleEndian.PutUint64(b[32:], rootInode)
	binary.LittleEndian.PutUint64(b[40:], 96) // bytes used
	binary.LittleEndian.PutUint64(b[48:], u64max)
	binary.LittleEndian.PutUint64(b[56:], u64max)
	binary.LittleEndian.PutUint64(b[64:], inodeTableStart)
	binary.LittleEndian.PutUint64(b[72:], inodeTableStart)
	binary.LittleEndian.PutUint64(b[80:], u64max)
	binary.LittleEndian.PutUint64(b[88:], u64max)
	return b
}

// TestArchiveSquashfsMalformed feeds the archive backend squashfs images whose
// superblock/metadata make the go-diskfs parser panic. See GHSA-6jcg-q3wp-x2f4.
func TestArchiveSquashfsMalformed(t *testing.T) {
	fstest.Initialise()
	ctx := context.Background()

	// Variant 1: zero block size reaches an integer division in the parser.
	zeroBlock := squashfsSuperblock(0, 0, 1, 0, 96)

	// Variant 2: an out-of-range root inode byte offset reaches an unchecked
	// slice against an empty, uncompressed metadata block.
	oob := append(squashfsSuperblock(65536, 16, 1, 12336, 96), 0x00, 0x80)
	binary.LittleEndian.PutUint64(oob[40:], uint64(len(oob)))

	for _, tc := range []struct {
		name  string
		image []byte
	}{
		{"ZeroBlockSize", zeroBlock},
		{"OutOfRangeInodeOffset", oob},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A directory per image: the fs cache keys on the parent
			// directory, so sharing one would hand the second subtest a
			// cached listing which does not contain its image.
			path := filepath.Join(t.TempDir(), tc.name+".sqfs")
			require.NoError(t, os.WriteFile(path, tc.image, 0o644))

			// The whole point is that this returns rather than panicking and
			// killing the test binary (and, in production, the serve daemon).
			f, err := cache.Get(ctx, ":archive:"+path)
			if err == nil {
				// The image parsed, so the panic must surface from a later
				// lazily-parsed operation instead.
				_, err = f.List(ctx, "")
			}
			require.Error(t, err, "a malformed squashfs image must error, not panic")
			assert.Contains(t, err.Error(), "invalid or corrupt squashfs image",
				"error should come from the recover, not an unrelated failure")
		})
	}
}
