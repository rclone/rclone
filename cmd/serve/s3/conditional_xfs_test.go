//go:build linux && !mips && !mipsle && !mips64 && !mips64le && !ppc64 && !ppc64le

package s3

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rclone/gofakes3"
	"github.com/rclone/rclone/fs"
	filelib "github.com/rclone/rclone/lib/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

type mutateOnRead struct {
	reader *strings.Reader
	mutate func()
}

func (r *mutateOnRead) Read(p []byte) (int, error) {
	if r.mutate != nil {
		r.mutate()
		r.mutate = nil
	}
	return r.reader.Read(p)
}

func newConditionalXFSBackend(t *testing.T) (*conditionalBackend, string) {
	t.Helper()
	b, root := newTestBackend(t)
	cb := newConditionalBackend(b)
	if !cb.xfs {
		if os.Getenv("RCLONE_TEST_XFS_REQUIRED") != "" {
			t.Fatal("XFS required")
		}
		t.Skip("requires an XFS temporary directory")
	}
	files := make([]*os.File, 2)
	for i := range files {
		f, err := os.CreateTemp(root, "probe-")
		require.NoError(t, err)
		defer func() {
			_ = f.Close()
			_ = os.Remove(f.Name())
		}()
		_, err = f.WriteString("probe")
		require.NoError(t, err)
		files[i] = f
	}
	token, err := filelib.StartXFSCommit(files[0])
	if err == nil {
		err = filelib.CommitXFSRange(files[0], files[1], token, true)
	}
	if errors.Is(err, filelib.ErrXFSCommitRangeUnsupported) {
		if os.Getenv("RCLONE_TEST_XFS_REQUIRED") != "" {
			t.Fatalf("XFS exchange required: %v", err)
		}
		t.Skip("kernel or filesystem lacks XFS exchange support")
	}
	require.NoError(t, err)
	return cb, root
}

func checkNoConditionalStaging(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "bucket"))
	require.NoError(t, err)
	for _, entry := range entries {
		assert.False(t, strings.HasPrefix(entry.Name(), tempObjectPrefix), "staging object left behind: %s", entry.Name())
	}
}

func TestConditionalXFSConflict(t *testing.T) {
	for _, mutation := range []string{"Write", "Truncate", "Rename", "Delete", "Symlink", "Chmod", "RestoreMtime"} {
		t.Run(mutation, func(t *testing.T) {
			cb, root := newConditionalXFSBackend(t)
			name := filepath.Join(root, "bucket", "object.txt")
			ctx := conditionalContext(objectETag(t, cb.s3Backend, "bucket", "object.txt"), "")
			var want []byte
			input := &mutateOnRead{reader: strings.NewReader("replacement"), mutate: func() {
				switch mutation {
				case "Write":
					require.NoError(t, os.WriteFile(name, []byte("external writer"), 0600))
				case "Truncate":
					require.NoError(t, os.Truncate(name, 0))
				case "Rename":
					require.NoError(t, os.Rename(name, name+".old"))
					require.NoError(t, os.WriteFile(name, []byte("external writer"), 0600))
				case "Delete":
					require.NoError(t, os.Remove(name))
					return
				case "Symlink":
					require.NoError(t, os.Rename(name, name+".old"))
					require.NoError(t, os.Symlink(name+".old", name))
				case "Chmod":
					require.NoError(t, os.Chmod(name, 0600))
				case "RestoreMtime":
					info, err := os.Stat(name)
					require.NoError(t, err)
					require.NoError(t, os.WriteFile(name, bytes.Repeat([]byte("x"), int(info.Size())), 0600))
					require.NoError(t, os.Chtimes(name, info.ModTime(), info.ModTime()))
				}
				var err error
				want, err = os.ReadFile(name)
				require.NoError(t, err)
			}}
			_, err := cb.PutObject(ctx, "bucket", "object.txt", map[string]string{}, input, 11)
			require.ErrorIs(t, err, gofakes3.ErrConditionalRequestConflict)
			got, err := os.ReadFile(name)
			if mutation == "Delete" {
				assert.True(t, os.IsNotExist(err))
			} else {
				require.NoError(t, err)
				assert.Equal(t, want, got)
			}
			checkNoConditionalStaging(t, root)
		})
	}
}

func TestConditionalXFSPublication(t *testing.T) {
	for _, method := range []string{"Put", "Copy", "Multipart"} {
		t.Run(method, func(t *testing.T) {
			b, root := newTestBackend(t)
			cb := newConditionalBackend(b)
			mode := os.Getenv("RCLONE_TEST_XFS_EXCHANGE")
			if mode != "" {
				require.Equal(t, mode == "1", cb.xfs, "XFS exchange capability")
			}
			name := filepath.Join(root, "bucket", "object.txt")
			before, err := os.Stat(name)
			require.NoError(t, err)
			ctx := conditionalContext(objectETag(t, b, "bucket", "object.txt"), "")
			switch method {
			case "Put":
				_, err = cb.PutObject(ctx, "bucket", "object.txt", map[string]string{}, strings.NewReader("replacement"), 11)
			case "Copy":
				_, err = b.PutObject(context.Background(), "bucket", "source", map[string]string{}, strings.NewReader("replacement"), 11)
				require.NoError(t, err)
				_, err = cb.CopyObject(ctx, "bucket", "source", "bucket", "object.txt", map[string]string{})
			case "Multipart":
				id, createErr := cb.CreateMultipartUpload(context.Background(), "bucket", "object.txt", map[string]string{})
				require.NoError(t, createErr)
				etag, putErr := cb.UploadPart(context.Background(), "bucket", "object.txt", id, 1, 11, strings.NewReader("replacement"))
				require.NoError(t, putErr)
				var input gofakes3.CompleteMultipartUploadRequest
				require.NoError(t, xml.Unmarshal([]byte(`<CompleteMultipartUpload><Part><PartNumber>1</PartNumber><ETag>`+etag+`</ETag></Part></CompleteMultipartUpload>`), &input))
				_, _, err = cb.CompleteMultipartUpload(ctx, "bucket", "object.txt", id, &input)
			}
			require.NoError(t, err)
			after, err := os.Stat(name)
			require.NoError(t, err)
			if mode == "1" || os.Getenv("RCLONE_TEST_XFS_REQUIRED") != "" {
				assert.True(t, os.SameFile(before, after), "exchange must preserve the target inode")
			} else if mode == "0" {
				assert.False(t, os.SameFile(before, after), "unsupported exchange must use rename")
			}
			got, err := os.ReadFile(name)
			require.NoError(t, err)
			assert.Equal(t, "replacement", string(got))
			checkNoConditionalStaging(t, root)
		})
	}
}

func TestConditionalXFSFallback(t *testing.T) {
	for _, kind := range []string{"HardLink", "ReadOnly"} {
		t.Run(kind, func(t *testing.T) {
			cb, root := newConditionalXFSBackend(t)
			name := filepath.Join(root, "bucket", "object.txt")
			before, err := os.Stat(name)
			require.NoError(t, err)
			if kind == "HardLink" {
				require.NoError(t, os.Link(name, name+".link"))
			} else {
				if os.Geteuid() == 0 {
					t.Skip("requires an unprivileged process")
				}
				require.NoError(t, os.Chmod(name, 0444))
			}
			ctx := conditionalContext(objectETag(t, cb.s3Backend, "bucket", "object.txt"), "")
			_, err = cb.PutObject(ctx, "bucket", "object.txt", map[string]string{}, strings.NewReader("replacement"), 11)
			require.NoError(t, err)
			after, err := os.Stat(name)
			require.NoError(t, err)
			assert.False(t, os.SameFile(before, after))
			got, err := os.ReadFile(name)
			require.NoError(t, err)
			assert.Equal(t, "replacement", string(got))
			if kind == "HardLink" {
				got, err := os.ReadFile(name + ".link")
				require.NoError(t, err)
				assert.Equal(t, "normal object", string(got), "replacing one key must not update its aliases")
			}
			checkNoConditionalStaging(t, root)
		})
	}
}

func TestConditionalXFSStagingAliases(t *testing.T) {
	for _, kind := range []string{"Symlink", "HardLink"} {
		t.Run(kind, func(t *testing.T) {
			cb, root := newConditionalXFSBackend(t)
			v := cb.s.provider.VFS()
			guard, err := startXFSCommitGuard(v.Fs(), "bucket/object.txt")
			require.NoError(t, err)
			defer func() { require.NoError(t, guard.Close()) }()
			victim := filepath.Join(root, "bucket", "victim")
			staging := filepath.Join(root, "bucket", "staging")
			require.NoError(t, os.WriteFile(victim, []byte("not staging"), 0600))
			if kind == "HardLink" {
				require.NoError(t, os.Link(victim, staging))
			} else {
				require.NoError(t, os.Symlink(victim, staging))
			}
			require.Error(t, guard.Publish("bucket/staging"))
			got, err := os.ReadFile(filepath.Join(root, "bucket", "object.txt"))
			require.NoError(t, err)
			assert.Equal(t, "normal object", string(got))
			got, err = os.ReadFile(victim)
			require.NoError(t, err)
			assert.Equal(t, "not staging", string(got))
		})
	}
}

func TestConditionalXFSFreshHash(t *testing.T) {
	cb, root := newConditionalXFSBackend(t)
	etag := objectETag(t, cb.s3Backend, "bucket", "object.txt")
	name := filepath.Join(root, "bucket", "object.txt")
	info, err := os.Stat(name)
	require.NoError(t, err)
	want := bytes.Repeat([]byte("x"), int(info.Size()))
	require.NoError(t, os.WriteFile(name, want, 0600))
	require.NoError(t, os.Chtimes(name, info.ModTime(), info.ModTime()))
	ctx := conditionalContext(etag, "")
	_, err = cb.PutObject(ctx, "bucket", "object.txt", map[string]string{}, strings.NewReader("replacement"), 11)
	require.ErrorIs(t, err, gofakes3.ErrPreconditionFailed)
	got, err := os.ReadFile(name)
	require.NoError(t, err)
	assert.Equal(t, want, got)
	checkNoConditionalStaging(t, root)
}

func TestConditionalXFSFailedUpload(t *testing.T) {
	for _, kind := range []string{"ShortBody", "Canceled"} {
		t.Run(kind, func(t *testing.T) {
			cb, root := newConditionalXFSBackend(t)
			conditional := conditionalContext(objectETag(t, cb.s3Backend, "bucket", "object.txt"), "")
			ctx, cancel := context.WithCancel(conditional)
			defer cancel()
			var input io.Reader = strings.NewReader("short")
			wantErr := error(gofakes3.ErrIncompleteBody)
			if kind == "Canceled" {
				input = &mutateOnRead{reader: strings.NewReader("replacement"), mutate: cancel}
				wantErr = context.Canceled
			}
			_, err := cb.PutObject(ctx, "bucket", "object.txt", map[string]string{}, input, 11)
			require.ErrorIs(t, err, wantErr)
			got, err := os.ReadFile(filepath.Join(root, "bucket", "object.txt"))
			require.NoError(t, err)
			assert.Equal(t, "normal object", string(got))
			checkNoConditionalStaging(t, root)
		})
	}
}

func TestConditionalXFSReader(t *testing.T) {
	cb, _ := newConditionalXFSBackend(t)
	ctx := conditionalContext(objectETag(t, cb.s3Backend, "bucket", "object.txt"), "")
	obj, err := cb.GetObject(context.Background(), "bucket", "object.txt", nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = obj.Contents.Close() })
	prefix := make([]byte, 3)
	_, err = io.ReadFull(obj.Contents, prefix)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		_, err := cb.PutObject(ctx, "bucket", "object.txt", map[string]string{}, strings.NewReader("replacement"), 11)
		done <- err
	}()
	require.Eventually(t, func() bool {
		cb.locks.mu.Lock()
		defer cb.locks.mu.Unlock()
		return cb.locks.locks[cb.objectLockKey(ctx, "bucket", "object.txt")].refs == 2
	}, 5*time.Second, time.Millisecond)
	select {
	case err := <-done:
		t.Fatalf("writer passed an open reader: %v", err)
	default:
	}
	rest, err := io.ReadAll(obj.Contents)
	require.NoError(t, err)
	assert.Equal(t, "normal object", string(append(prefix, rest...)))
	require.NoError(t, obj.Contents.Close())
	require.NoError(t, <-done)
	obj, err = cb.GetObject(context.Background(), "bucket", "object.txt", nil)
	require.NoError(t, err)
	rest, err = io.ReadAll(obj.Contents)
	require.NoError(t, obj.Contents.Close())
	require.NoError(t, err)
	assert.Equal(t, "replacement", string(rest))
}

func TestConditionalXFSLinkOptions(t *testing.T) {
	for _, option := range []string{"links", "copy_links"} {
		t.Run(option, func(t *testing.T) {
			f, err := fs.NewFs(context.Background(), ":local,"+option+"=true:"+t.TempDir())
			require.NoError(t, err)
			got, err := xfsCommitRangeCapability(f)
			require.NoError(t, err)
			assert.False(t, got)
		})
	}
}

func TestConditionalXFSMultipartRetry(t *testing.T) {
	cb, root := newConditionalXFSBackend(t)
	b := cb.s3Backend
	ctx := context.Background()
	name := filepath.Join(root, "bucket", "object.txt")
	id, err := cb.CreateMultipartUpload(ctx, "bucket", "object.txt", map[string]string{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = cb.AbortMultipartUpload(ctx, "bucket", "object.txt", id) })
	part, err := cb.UploadPart(ctx, "bucket", "object.txt", id, 1, 11, strings.NewReader("replacement"))
	require.NoError(t, err)
	var input gofakes3.CompleteMultipartUploadRequest
	require.NoError(t, xml.Unmarshal([]byte(`<CompleteMultipartUpload><Part><PartNumber>1</PartNumber><ETag>`+part+`</ETag></Part></CompleteMultipartUpload>`), &input))
	up, err := cb.loadUpload(ctx, id)
	require.NoError(t, err)
	up.fh = &mutateOnClose{WriteCloser: up.fh, mutate: func() {
		require.NoError(t, os.WriteFile(name, []byte("changed elsewhere"), 0600))
	}}
	conditional := conditionalContext(objectETag(t, b, "bucket", "object.txt"), "")
	_, _, err = cb.CompleteMultipartUpload(conditional, "bucket", "object.txt", id, &input)
	require.ErrorIs(t, err, gofakes3.ErrConditionalRequestConflict)
	got, err := os.ReadFile(name)
	require.NoError(t, err)
	assert.Equal(t, "changed elsewhere", string(got))
	_, err = cb.loadUpload(ctx, id)
	require.NoError(t, err)
	got, err = os.ReadFile(filepath.Join(root, up.streamFp))
	require.NoError(t, err)
	assert.Equal(t, "replacement", string(got))
	b.forgetPath(up.vfs, up.fp)
	conditional = conditionalContext(objectETag(t, b, "bucket", "object.txt"), "")
	_, _, err = cb.CompleteMultipartUpload(conditional, "bucket", "object.txt", id, &input)
	require.NoError(t, err)
	_, err = cb.loadUpload(ctx, id)
	assert.ErrorIs(t, err, gofakes3.ErrNoSuchUpload)
	got, err = os.ReadFile(name)
	require.NoError(t, err)
	assert.Equal(t, "replacement", string(got))
	checkNoConditionalStaging(t, root)
}

type mutateOnClose struct {
	io.WriteCloser
	mutate func()
}

func (w *mutateOnClose) Close() error {
	if err := w.WriteCloser.Close(); err != nil {
		return err
	}
	w.mutate()
	return nil
}

func TestConditionalXFSRootConfinement(t *testing.T) {
	cb, root := newConditionalXFSBackend(t)
	b := cb.s3Backend
	outside := t.TempDir()
	victim := filepath.Join(outside, "victim")
	require.NoError(t, os.WriteFile(victim, []byte("outside"), 0600))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "bucket", "escape")))
	guard, err := startXFSCommitGuard(b.s.provider.VFS().Fs(), "bucket/escape/victim")
	require.Error(t, err)
	assert.Nil(t, guard)
	guard, err = startXFSCommitGuard(b.s.provider.VFS().Fs(), "bucket/object.txt")
	require.NoError(t, err)
	t.Cleanup(func() { _ = guard.Close() })
	assert.Error(t, guard.Publish("bucket/escape/victim"))
	got, err := os.ReadFile(victim)
	require.NoError(t, err)
	assert.Equal(t, "outside", string(got))
	require.NoError(t, guard.Close())
	require.NoError(t, guard.Close())
}

func TestConditionalXFSMetadata(t *testing.T) {
	for _, kind := range []string{"Mode", "Xattr", "ACL", "Owner"} {
		t.Run(kind, func(t *testing.T) {
			cb, root := newConditionalXFSBackend(t)
			for _, key := range []string{"exchange", "rename"} {
				name := filepath.Join(root, "bucket", key)
				require.NoError(t, os.WriteFile(name, []byte("old"), 0666))
				switch kind {
				case "Mode":
					require.NoError(t, os.Chmod(name, 0600))
				case "Xattr":
					require.NoError(t, unix.Setxattr(name, "user.test", []byte("old"), 0))
				case "ACL":
					tool, err := exec.LookPath("setfacl")
					if err != nil {
						t.Skip("requires setfacl")
					}
					out, err := exec.Command(tool, "-m", "u:65534:r--", name).CombinedOutput()
					require.NoError(t, err, "%s", out)
				case "Owner":
					if os.Geteuid() != 0 {
						t.Skip("requires chown")
					}
					require.NoError(t, os.Chown(name, 65534, 65534))
				}
				before, err := os.Stat(name)
				require.NoError(t, err)
				cb.xfs = key == "exchange"
				cb.forgetPath(cb.s.provider.VFS(), "bucket/"+key)
				ctx := conditionalContext("*", "")
				_, err = cb.PutObject(ctx, "bucket", key, map[string]string{}, strings.NewReader("replacement"), 11)
				require.NoError(t, err)
				after, err := os.Stat(name)
				require.NoError(t, err)
				require.False(t, os.SameFile(before, after))
			}
			path := func(key string) string { return filepath.Join(root, "bucket", key) }
			var x, r unix.Stat_t
			require.NoError(t, unix.Stat(path("exchange"), &x))
			require.NoError(t, unix.Stat(path("rename"), &r))
			assert.Equal(t, r.Mode, x.Mode)
			assert.Equal(t, r.Uid, x.Uid)
			assert.Equal(t, r.Gid, x.Gid)
			for _, name := range []string{"user.test", "system.posix_acl_access"} {
				for _, key := range []string{"exchange", "rename"} {
					_, err := unix.Getxattr(path(key), name, nil)
					require.ErrorIs(t, err, unix.ENODATA)
				}
			}
		})
	}
}
