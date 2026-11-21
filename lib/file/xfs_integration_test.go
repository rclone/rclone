//go:build linux && !mips && !mipsle && !mips64 && !mips64le && !ppc64 && !ppc64le

package file

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func newXFSFile(t *testing.T, dir string, data []byte) *os.File {
	t.Helper()
	f, err := os.CreateTemp(dir, "rclone-xfs-")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = f.Close()
		assert.NoError(t, os.Remove(f.Name()))
	})
	_, err = f.Write(data)
	require.NoError(t, err)
	return f
}

func checkXFSContents(t *testing.T, f *os.File, want []byte) {
	t.Helper()
	got, err := os.ReadFile(f.Name())
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestXFSCommitRangeIntegration(t *testing.T) {
	dir := os.Getenv("RCLONE_TEST_XFS_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	onXFS, err := IsXFS(dir)
	require.NoError(t, err)
	if !onXFS {
		if os.Getenv("RCLONE_TEST_XFS_DIR") != "" {
			t.Fatal("RCLONE_TEST_XFS_DIR must be on XFS")
		}
		t.Skip("requires XFS")
	}

	// START_COMMIT alone does not detect a missing on-disk exchange feature.
	target := newXFSFile(t, dir, []byte("old"))
	staging := newXFSFile(t, dir, []byte("new"))
	token, err := StartXFSCommit(target)
	if err == nil {
		err = CommitXFSRange(target, staging, token, true)
	}
	if errors.Is(err, ErrXFSCommitRangeUnsupported) {
		require.NotEqual(t, "1", os.Getenv("RCLONE_TEST_XFS_EXCHANGE"), "exchange support required")
		require.Empty(t, os.Getenv("RCLONE_TEST_XFS_REQUIRED"), "exchange support required")
		checkXFSContents(t, target, []byte("old"))
		checkXFSContents(t, staging, []byte("new"))
		t.Log("unsupported exchange leaves both files unchanged")
		return
	}
	require.NoError(t, err)
	require.NotEqual(t, "0", os.Getenv("RCLONE_TEST_XFS_EXCHANGE"), "exchange was expected to be disabled")

	for _, dsync := range []bool{false, true} {
		for _, sizes := range [][2]int{{0, 0}, {0, 1}, {1, 0}, {1, 1}, {4095, 4096}, {4096, 4095}, {4096, 4097}, {65537, 23}, {23, 65537}, {1048579, 2097161}} {
			t.Run(fmt.Sprintf("Sizes/%d-%d/DSync=%t", sizes[0], sizes[1], dsync), func(t *testing.T) {
				old := bytes.Repeat([]byte("a"), sizes[0])
				want := bytes.Repeat([]byte("b"), sizes[1])
				target := newXFSFile(t, dir, old)
				staging := newXFSFile(t, dir, want)
				require.NoError(t, staging.Chmod(0640))
				targetInfo, err := target.Stat()
				require.NoError(t, err)
				stagingInfo, err := staging.Stat()
				require.NoError(t, err)
				token, err := StartXFSCommit(target)
				require.NoError(t, err)
				sample := *token
				_, err = target.Seek(3, io.SeekStart)
				require.NoError(t, err)
				_, err = staging.Seek(7, io.SeekStart)
				require.NoError(t, err)
				require.NoError(t, CommitXFSRange(target, staging, token, dsync))
				assert.Equal(t, sample, *token, "committing must not modify the freshness token")
				checkXFSContents(t, target, want)
				checkXFSContents(t, staging, old)
				for _, item := range []struct {
					f      *os.File
					before os.FileInfo
					offset int64
				}{{target, targetInfo, 3}, {staging, stagingInfo, 7}} {
					after, err := item.f.Stat()
					require.NoError(t, err)
					assert.True(t, os.SameFile(item.before, after))
					assert.Equal(t, item.before.Mode(), after.Mode())
					offset, err := item.f.Seek(0, io.SeekCurrent)
					require.NoError(t, err)
					assert.Equal(t, item.offset, offset)
				}
			})
		}
	}

	for _, test := range []struct {
		name   string
		mutate func(*testing.T, *os.File)
	}{
		{"Overwrite", func(t *testing.T, f *os.File) {
			_, err := f.WriteAt([]byte("new"), 0)
			require.NoError(t, err)
		}},
		{"Truncate", func(t *testing.T, f *os.File) { require.NoError(t, f.Truncate(1)) }},
		{"Append", func(t *testing.T, f *os.File) {
			_, err := f.WriteAt([]byte("!"), 3)
			require.NoError(t, err)
		}},
		{"Chmod", func(t *testing.T, f *os.File) { require.NoError(t, f.Chmod(0640)) }},
		{"Mtime", func(t *testing.T, f *os.File) {
			require.NoError(t, os.Chtimes(f.Name(), time.Unix(1, 0), time.Unix(1, 0)))
		}},
		{"RestoreMtime", func(t *testing.T, f *os.File) {
			info, err := f.Stat()
			require.NoError(t, err)
			_, err = f.WriteAt([]byte("new"), 0)
			require.NoError(t, err)
			require.NoError(t, os.Chtimes(f.Name(), info.ModTime(), info.ModTime()))
		}},
	} {
		t.Run("Stale/"+test.name, func(t *testing.T) {
			target := newXFSFile(t, dir, []byte("old"))
			staging := newXFSFile(t, dir, []byte("replacement"))
			token, err := StartXFSCommit(target)
			require.NoError(t, err)
			test.mutate(t, target)
			want, err := os.ReadFile(target.Name())
			require.NoError(t, err)
			assert.ErrorIs(t, CommitXFSRange(target, staging, token, true), unix.EBUSY)
			checkXFSContents(t, target, want)
			checkXFSContents(t, staging, []byte("replacement"))
		})
	}

	t.Run("WrongTarget", func(t *testing.T) {
		first := newXFSFile(t, dir, []byte("first"))
		second := newXFSFile(t, dir, []byte("second"))
		staging := newXFSFile(t, dir, []byte("staging"))
		token, err := StartXFSCommit(first)
		require.NoError(t, err)
		assert.ErrorIs(t, CommitXFSRange(second, staging, token, false), unix.EBUSY)
		checkXFSContents(t, first, []byte("first"))
		checkXFSContents(t, second, []byte("second"))
		checkXFSContents(t, staging, []byte("staging"))
	})

	t.Run("TokenReuse", func(t *testing.T) {
		target := newXFSFile(t, dir, []byte("old"))
		staging := newXFSFile(t, dir, []byte("new"))
		token, err := StartXFSCommit(target)
		require.NoError(t, err)
		require.NoError(t, CommitXFSRange(target, staging, token, false))
		assert.ErrorIs(t, CommitXFSRange(target, staging, token, false), unix.EBUSY)
		checkXFSContents(t, target, []byte("new"))
		checkXFSContents(t, staging, []byte("old"))
	})

	for _, readOnlyTarget := range []bool{false, true} {
		t.Run(fmt.Sprintf("ReadOnly/Target=%t", readOnlyTarget), func(t *testing.T) {
			target := newXFSFile(t, dir, []byte("old"))
			staging := newXFSFile(t, dir, []byte("new"))
			token, err := StartXFSCommit(target)
			require.NoError(t, err)
			name := staging.Name()
			if readOnlyTarget {
				name = target.Name()
			}
			readOnly, err := os.Open(name)
			require.NoError(t, err)
			defer func() { _ = readOnly.Close() }()
			if readOnlyTarget {
				err = CommitXFSRange(readOnly, staging, token, false)
			} else {
				err = CommitXFSRange(target, readOnly, token, false)
			}
			require.Error(t, err)
			assert.False(t, errors.Is(err, ErrXFSCommitRangeUnsupported))
			checkXFSContents(t, target, []byte("old"))
			checkXFSContents(t, staging, []byte("new"))
		})
	}

	t.Run("CrossFilesystem", func(t *testing.T) {
		other := os.Getenv("RCLONE_TEST_XFS_OTHER_DIR")
		if other == "" {
			t.Skip("set RCLONE_TEST_XFS_OTHER_DIR to test cross-filesystem rejection")
		}
		target := newXFSFile(t, dir, []byte("old"))
		staging := newXFSFile(t, other, []byte("new"))
		token, err := StartXFSCommit(target)
		require.NoError(t, err)
		assert.ErrorIs(t, CommitXFSRange(target, staging, token, false), unix.EXDEV)
		checkXFSContents(t, target, []byte("old"))
		checkXFSContents(t, staging, []byte("new"))
	})

	t.Run("Concurrent", func(t *testing.T) {
		const writers = 8
		target := newXFSFile(t, dir, []byte("old"))
		staging := make([]*os.File, writers)
		tokens := make([]*XFSCommitToken, writers)
		for i := range writers {
			staging[i] = newXFSFile(t, dir, []byte(fmt.Sprint(i)))
			var err error
			tokens[i], err = StartXFSCommit(target)
			require.NoError(t, err)
		}
		type result struct {
			index int
			err   error
		}
		results := make(chan result, writers)
		start := make(chan struct{})
		for i := range writers {
			go func() {
				<-start
				results <- result{i, CommitXFSRange(target, staging[i], tokens[i], true)}
			}()
		}
		close(start)
		winner, successes := -1, 0
		for range writers {
			r := <-results
			if r.err == nil {
				winner = r.index
				successes++
			} else {
				assert.ErrorIs(t, r.err, unix.EBUSY)
			}
		}
		require.Equal(t, 1, successes)
		checkXFSContents(t, target, []byte(fmt.Sprint(winner)))
		for i, f := range staging {
			want := []byte(fmt.Sprint(i))
			if i == winner {
				want = []byte("old")
			}
			checkXFSContents(t, f, want)
		}
	})

	t.Run("HardLink", func(t *testing.T) {
		target := newXFSFile(t, dir, []byte("old"))
		staging := newXFSFile(t, dir, []byte("new"))
		link := filepath.Join(dir, filepath.Base(target.Name())+"-link")
		require.NoError(t, os.Link(target.Name(), link))
		t.Cleanup(func() { assert.NoError(t, os.Remove(link)) })
		token, err := StartXFSCommit(target)
		require.NoError(t, err)
		require.NoError(t, CommitXFSRange(target, staging, token, true))
		got, err := os.ReadFile(link)
		require.NoError(t, err)
		assert.Equal(t, "new", string(got))
		checkXFSContents(t, staging, []byte("old"))
	})

	t.Run("Sparse", func(t *testing.T) {
		target := newXFSFile(t, dir, nil)
		staging := newXFSFile(t, dir, nil)
		require.NoError(t, target.Truncate(2<<20))
		require.NoError(t, staging.Truncate(4<<20))
		_, err := target.WriteAt([]byte("old"), 65536)
		require.NoError(t, err)
		_, err = staging.WriteAt([]byte("new"), 3<<20)
		require.NoError(t, err)
		old, err := os.ReadFile(target.Name())
		require.NoError(t, err)
		want, err := os.ReadFile(staging.Name())
		require.NoError(t, err)
		token, err := StartXFSCommit(target)
		require.NoError(t, err)
		require.NoError(t, CommitXFSRange(target, staging, token, true))
		checkXFSContents(t, target, want)
		checkXFSContents(t, staging, old)
	})

	t.Run("Reflink", func(t *testing.T) {
		old := bytes.Repeat([]byte("old"), 32768)
		target := newXFSFile(t, dir, old)
		staging := newXFSFile(t, dir, nil)
		require.NoError(t, unix.IoctlFileClone(int(staging.Fd()), int(target.Fd())))
		token, err := StartXFSCommit(target)
		require.NoError(t, err)
		_, err = staging.WriteAt([]byte("new"), 0)
		require.NoError(t, err)
		checkXFSContents(t, target, old)
		want := bytes.Clone(old)
		copy(want, "new")
		require.NoError(t, CommitXFSRange(target, staging, token, true))
		checkXFSContents(t, target, want)
		checkXFSContents(t, staging, old)
	})
}
