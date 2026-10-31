//go:build linux

package azureblob

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	fssync "github.com/rclone/rclone/fs/sync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestMapPOSIXMetadata(t *testing.T) {
	meta := fs.Metadata{
		"mode": "104751", "uid": "1000", "gid": "1001",
		"nlink": "2", "ino": "1234", "dev": "2050",
		"atime":      "2020-01-02T03:04:05.123456789Z",
		"mtime":      "2020-01-03T03:04:05.987654321Z",
		"ctime":      "2020-01-04T03:04:05Z",
		"btime":      "2020-01-01T03:04:05Z",
		"statx-mask": "4095", "attributes": "16", "attributes-mask": "48",
		"custom": "value",
	}
	got, err := mapPOSIXMetadata(meta)
	require.NoError(t, err)
	assert.Equal(t, "35305", got["permissions"])
	assert.Equal(t, "1000", got["posix_owner"])
	assert.Equal(t, "1001", got["posix_group"])
	assert.Equal(t, "2", got["posix_nlink"])
	assert.Equal(t, "1234", got["posix_ino"])
	assert.Equal(t, "2050", got["posix_dev"])
	assert.Equal(t, "4095", got["linux_statx_mask"])
	assert.Equal(t, "16", got["linux_attribute"])
	assert.Equal(t, "48", got["linux_attribute_mask"])
	for _, key := range []string{"atime", "mtime", "ctime", "btime"} {
		when, err := time.Parse(time.RFC3339Nano, meta[key])
		require.NoError(t, err)
		dest := map[string]string{"atime": "posix_atime", "mtime": "modtime", "ctime": "posix_ctime", "btime": "linux_btime"}[key]
		assert.Equal(t, strconv.FormatInt(when.UnixNano(), 10), got[dest])
	}
	for _, key := range []string{"mode", "uid", "gid", "nlink", "ino", "dev", "ctime", "btime", "statx-mask", "attributes", "attributes-mask"} {
		assert.NotContains(t, got, key)
	}
	assert.Equal(t, "value", got["custom"])
	assert.Equal(t, "104751", meta["mode"], "mapping must not mutate source metadata")

	for _, tc := range []struct{ mode, marker string }{
		{"40755", "hdi_isfolder"}, {"120777", "is_symlink"},
		{"20600", "is_char_dev"}, {"60600", "is_block_dev"},
		{"10600", "is_fifo"}, {"140600", "is_socket"},
	} {
		t.Run(tc.marker, func(t *testing.T) {
			input := fs.Metadata{"mode": tc.mode, "uid": "1", "gid": "2", "rdev": "abc"}
			got, err := mapPOSIXMetadata(input)
			require.NoError(t, err)
			assert.Equal(t, "true", got[tc.marker])
			if tc.marker == "is_char_dev" || tc.marker == "is_block_dev" {
				assert.Equal(t, "2748", got["posix_rdev"])
			} else {
				assert.NotContains(t, got, "posix_rdev")
			}
			for _, marker := range []string{"hdi_isfolder", "is_symlink", "is_char_dev", "is_block_dev", "is_fifo", "is_socket"} {
				if marker != tc.marker {
					assert.NotContains(t, got, marker)
				}
			}
		})
	}
	for _, key := range []string{"mode", "uid", "atime", "dev", "statx-mask"} {
		input := fs.Metadata{"mode": "100600", "uid": "1", "gid": "2"}
		input[key] = "invalid"
		_, err := mapPOSIXMetadata(input)
		assert.Error(t, err, key)
	}
	_, err = mapPOSIXMetadata(nil)
	assert.Error(t, err, "missing POSIX metadata must not silently succeed")
	_, err = mapPOSIXMetadata(fs.Metadata{"mode": "100600", "uid": "4294967296"})
	assert.Error(t, err, "UIDs must fit AzCopy's uint32 representation")
}

type preservationBlob struct {
	body []byte
	meta fs.Metadata
}

// preservationServer implements only the Azure requests used by the upload tests.
type preservationServer struct {
	mu             sync.Mutex
	blobs          map[string]preservationBlob
	blocks         map[string][]byte
	uploads        int
	metadataWrites int
}

func (s *preservationServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name := strings.TrimPrefix(r.URL.Path, "/container/")
	w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
	w.Header().Set("ETag", `"test"`)
	if r.Method == http.MethodGet && r.URL.Query().Get("comp") == "list" {
		w.Header().Set("Content-Type", "application/xml")
		type listedBlob struct {
			Name       string
			Properties struct {
				LastModified  string `xml:"Last-Modified"`
				ContentLength int    `xml:"Content-Length"`
				BlobType      string `xml:"BlobType"`
			}
			Metadata struct {
				Folder string `xml:"hdi_isfolder,omitempty"`
			}
		}
		response := struct {
			XMLName    xml.Name                `xml:"EnumerationResults"`
			Blobs      []listedBlob            `xml:"Blobs>Blob"`
			Prefixes   []struct{ Name string } `xml:"Blobs>BlobPrefix"`
			NextMarker string
		}{}
		prefix := r.URL.Query().Get("prefix")
		delimiter := r.URL.Query().Get("delimiter")
		seenPrefixes := map[string]bool{}
		for name, stored := range s.blobs {
			if !strings.HasPrefix(name, prefix) {
				continue
			}
			if delimiter != "" {
				if before, _, found := strings.Cut(strings.TrimPrefix(name, prefix), delimiter); found {
					dir := prefix + before + delimiter
					if !seenPrefixes[dir] {
						seenPrefixes[dir] = true
						response.Prefixes = append(response.Prefixes, struct{ Name string }{dir})
					}
					continue
				}
			}
			entry := listedBlob{Name: name}
			entry.Properties.LastModified = time.Now().UTC().Format(http.TimeFormat)
			entry.Properties.ContentLength = len(stored.body)
			entry.Properties.BlobType = "BlockBlob"
			entry.Metadata.Folder = stored.meta[dirMetaKey]
			response.Blobs = append(response.Blobs, entry)
		}
		sort.Slice(response.Blobs, func(i, j int) bool { return response.Blobs[i].Name < response.Blobs[j].Name })
		_ = xml.NewEncoder(w).Encode(response)
		return
	}
	if r.Method == http.MethodPut {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		switch r.URL.Query().Get("comp") {
		case "metadata":
			stored, found := s.blobs[name]
			if !found {
				w.Header().Set("x-ms-error-code", "BlobNotFound")
				w.WriteHeader(http.StatusNotFound)
				return
			}
			body = stored.body
			s.metadataWrites++
		case "block":
			s.blocks[name+"/"+r.URL.Query().Get("blockid")] = body
			w.WriteHeader(http.StatusCreated)
			return
		case "blocklist":
			var blocks struct {
				Latest []string
			}
			if err := xml.Unmarshal(body, &blocks); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			body = nil
			for _, id := range blocks.Latest {
				body = append(body, s.blocks[name+"/"+id]...)
			}
		}
		meta := fs.Metadata{}
		for key, values := range r.Header {
			if suffix, ok := strings.CutPrefix(strings.ToLower(key), "x-ms-meta-"); ok {
				meta[suffix] = values[0]
			}
		}
		s.blobs[name] = preservationBlob{body: body, meta: meta}
		if r.URL.Query().Get("comp") == "metadata" {
			w.WriteHeader(http.StatusOK)
		} else {
			s.uploads++
			w.WriteHeader(http.StatusCreated)
		}
		return
	}
	if r.Method == http.MethodDelete {
		if _, found := s.blobs[name]; !found {
			w.Header().Set("x-ms-error-code", "BlobNotFound")
			w.WriteHeader(http.StatusNotFound)
			return
		}
		delete(s.blobs, name)
		w.WriteHeader(http.StatusAccepted)
		return
	}
	stored, found := s.blobs[name]
	if !found {
		w.Header().Set("x-ms-error-code", "BlobNotFound")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	for key, value := range stored.meta {
		w.Header().Set("x-ms-meta-"+key, value)
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(stored.body)))
	w.Header().Set("x-ms-blob-type", "BlockBlob")
	sum := md5.Sum(stored.body)
	w.Header().Set("Content-MD5", base64.StdEncoding.EncodeToString(sum[:]))
	w.Header().Set("Content-Type", "application/octet-stream")
	if r.Method == http.MethodGet {
		_, _ = w.Write(stored.body)
	}
}

func newPreservationFs(t *testing.T, ctx context.Context) (*Fs, *preservationServer) {
	t.Helper()
	state := &preservationServer{blobs: map[string]preservationBlob{}, blocks: map[string][]byte{}}
	server := httptest.NewServer(state)
	t.Cleanup(server.Close)
	return newPreservationFsAt(t, ctx, server.URL, "container", false), state
}

func newPreservationFsAt(t *testing.T, ctx context.Context, endpoint, root string, directoryMarkers bool) *Fs {
	t.Helper()
	info, err := fs.Find("azureblob")
	require.NoError(t, err)
	m := fs.ConfigMap("azureblob", info.Options, "", configmap.Simple{
		"account": "test", "endpoint": endpoint, "hns": "false",
		"no_check_container": "true",
		"directory_markers":  strconv.FormatBool(directoryMarkers),
	})
	f, err := NewFs(ctx, "azureblob", root, m)
	require.NoError(t, err)
	return f.(*Fs)
}

func TestPreservationUpload(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		posix, symlinks, multipart bool
	}{
		{"Default", false, false, false},
		{"POSIX", true, false, false},
		{"Symlinks", false, true, false},
		{"Both", true, true, false},
		{"Multipart", true, true, true},
		{"SequentialMultipart", true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, ci := fs.AddConfig(context.Background())
			ci.PreservePOSIXProperties, ci.PreserveSymlinks = tc.posix, tc.symlinks
			root := t.TempDir()
			content := "file content"
			require.NoError(t, os.WriteFile(filepath.Join(root, "file"), []byte(content), 0751))
			require.NoError(t, os.Link(filepath.Join(root, "file"), filepath.Join(root, "hardlink")))
			require.NoError(t, os.Symlink("../missing", filepath.Join(root, "link")))
			fsrc, err := local.NewFs(ctx, "local", root, configmap.Simple{})
			require.NoError(t, err)
			fdst, server := newPreservationFs(t, ctx)
			if tc.multipart {
				fdst.opt.ChunkSize = 4
				ci.MultiThreadCutoff = 1
				ci.MultiThreadStreams = 2
				if tc.name == "SequentialMultipart" {
					ci.MultiThreadStreams = 0
				}
			}
			for _, name := range []string{"file", "hardlink", "link"} {
				src, err := fsrc.NewObject(ctx, name)
				if name == "link" && !tc.symlinks {
					// The default local backend does not expose an uploadable symlink.
					if err == nil {
						assert.False(t, src.Storable())
					}
					continue
				}
				require.NoError(t, err)
				var before unix.Stat_t
				require.NoError(t, unix.Lstat(filepath.Join(root, name), &before))
				_, err = operations.Copy(ctx, fdst, nil, "renamed-"+name, src)
				require.NoError(t, err)
				server.mu.Lock()
				stored := server.blobs["renamed-"+name]
				server.mu.Unlock()
				if name == "link" {
					assert.Equal(t, "../missing", string(stored.body))
					assert.Equal(t, "true", stored.meta["is_symlink"])
					assert.NotContains(t, stored.meta, "is_char_dev")
				} else {
					assert.Equal(t, content, string(stored.body))
					assert.NotContains(t, stored.meta, "is_symlink")
				}
				if tc.posix {
					assert.Equal(t, strconv.FormatUint(uint64(before.Mode), 10), stored.meta["permissions"])
					assert.Equal(t, strconv.FormatUint(uint64(before.Uid), 10), stored.meta["posix_owner"])
					assert.Equal(t, strconv.FormatUint(uint64(before.Gid), 10), stored.meta["posix_group"])
					assert.Equal(t, strconv.FormatUint(before.Ino, 10), stored.meta["posix_ino"])
					assert.Equal(t, strconv.FormatUint(uint64(before.Nlink), 10), stored.meta["posix_nlink"])
					assert.Equal(t, strconv.FormatUint(uint64(before.Dev), 10), stored.meta["posix_dev"])
					assert.Equal(t, strconv.FormatInt(time.Unix(before.Mtim.Unix()).UnixNano(), 10), stored.meta["modtime"])
					assert.Equal(t, strconv.FormatInt(time.Unix(before.Ctim.Unix()).UnixNano(), 10), stored.meta["posix_ctime"])
					assert.Equal(t, strconv.FormatInt(time.Unix(before.Atim.Unix()).UnixNano(), 10), stored.meta["posix_atime"])
					assert.NotEmpty(t, stored.meta["linux_statx_mask"])
				} else {
					assert.NotContains(t, stored.meta, "permissions")
				}
				assert.NotEmpty(t, stored.meta[modTimeKey])
			}
			if tc.multipart {
				server.mu.Lock()
				assert.NotEmpty(t, server.blocks, "the test must actually upload multipart blocks")
				server.mu.Unlock()
			}
		})
	}
}

func TestPreservationDirectories(t *testing.T) {
	ctx, ci := fs.AddConfig(context.Background())
	ci.PreservePOSIXProperties, ci.PreserveSymlinks = true, true
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "empty"), 0750))
	require.NoError(t, os.Mkdir(filepath.Join(root, "sub"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "sub", "file"), []byte("data"), 0640))
	require.NoError(t, os.Symlink("sub", filepath.Join(root, "directory-link")))
	fsrc, err := local.NewFs(ctx, "local", root, configmap.Simple{})
	require.NoError(t, err)
	fdst, server := newPreservationFs(t, ctx)
	require.NoError(t, fssync.CopyDir(ctx, fdst, fsrc, false))
	server.mu.Lock()
	for _, name := range []string{"empty", "sub"} {
		stored, ok := server.blobs[name]
		assert.True(t, ok, name)
		assert.Empty(t, stored.body)
		assert.Equal(t, "true", stored.meta["hdi_isfolder"])
		assert.NotEmpty(t, stored.meta["permissions"])
		assert.NotEmpty(t, stored.meta["posix_owner"])
	}
	assert.Equal(t, "data", string(server.blobs["sub/file"].body))
	assert.Equal(t, "sub", string(server.blobs["directory-link"].body))
	server.mu.Unlock()
	entries, err := fdst.List(ctx, "")
	require.NoError(t, err)
	counts := map[string]int{}
	for _, entry := range entries {
		counts[entry.Remote()]++
	}
	assert.Equal(t, map[string]int{"empty": 1, "sub": 1, "directory-link": 1}, counts)
	// A second pass must handle metadata on existing directory entries too.
	require.NoError(t, fssync.CopyDir(ctx, fdst, fsrc, false))
	plain := newPreservationFsAt(t, context.Background(), fdst.svc.URL(), "container", false)
	entries, err = plain.List(context.Background(), "")
	require.NoError(t, err)
	assert.Len(t, entries, 3, "ordinary listings must not duplicate preserved directories")
}

func TestPreservationCachedBackends(t *testing.T) {
	for _, tc := range []struct {
		name            string
		posix, symlinks bool
	}{
		{"POSIX", true, false},
		{"Symlinks", false, true},
		{"Both", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plainCtx, plainConfig := fs.AddConfig(context.Background())
			plainConfig.PreservePOSIXProperties, plainConfig.PreserveSymlinks = false, false
			ctx, ci := fs.AddConfig(plainCtx)
			ci.PreservePOSIXProperties, ci.PreserveSymlinks = tc.posix, tc.symlinks
			root := t.TempDir()
			require.NoError(t, os.Mkdir(filepath.Join(root, "empty"), 0750))
			require.NoError(t, os.Symlink("missing", filepath.Join(root, "link")))
			fdst, server := newPreservationFs(t, plainCtx)
			remote := fmt.Sprintf(":azureblob,account=test,endpoint=%q,hns=false,no_check_container=true:container", fdst.svc.URL())

			plainSrc, err := cache.Get(plainCtx, root)
			require.NoError(t, err)
			plainDst, err := cache.Get(plainCtx, remote)
			require.NoError(t, err)
			fsrc, err := cache.Get(ctx, root)
			require.NoError(t, err)
			preservedDst, err := cache.Get(ctx, remote)
			require.NoError(t, err)
			require.NotSame(t, plainSrc, fsrc)
			require.NotSame(t, plainDst, preservedDst)
			require.NoError(t, fssync.CopyDir(ctx, preservedDst, fsrc, false))
			server.mu.Lock()
			if tc.symlinks {
				assert.Equal(t, "missing", string(server.blobs["link"].body))
				assert.Equal(t, "true", server.blobs["link"].meta["is_symlink"])
			} else {
				assert.NotContains(t, server.blobs, "link")
			}
			if tc.posix {
				assert.Equal(t, "true", server.blobs["empty"].meta[dirMetaKey])
				assert.NotEmpty(t, server.blobs["empty"].meta["permissions"])
			} else {
				assert.NotContains(t, server.blobs, "empty")
				assert.NotContains(t, server.blobs["link"].meta, "permissions")
			}
			server.mu.Unlock()

			srcAgain, err := cache.Get(plainCtx, root)
			require.NoError(t, err)
			dstAgain, err := cache.Get(plainCtx, remote)
			require.NoError(t, err)
			assert.Same(t, plainSrc, srcAgain)
			assert.Same(t, plainDst, dstAgain)
			assert.Nil(t, dstAgain.Features().MkdirMetadata)
			link, err := srcAgain.NewObject(plainCtx, "link")
			require.NoError(t, err)
			assert.False(t, link.Storable())
		})
	}
}

func TestPreservationRejectsUnsupportedTransfers(t *testing.T) {
	ctx, ci := fs.AddConfig(context.Background())
	ci.PreserveSymlinks = true
	root := t.TempDir()
	fsrc, err := local.NewFs(ctx, "local", root, configmap.Simple{})
	require.NoError(t, err)
	fdst, server := newPreservationFs(t, ctx)
	src := object.NewMemoryObject("remote-source", time.Now(), []byte("data"))
	_, err = operations.Copy(ctx, fdst, nil, src.Remote(), src)
	require.ErrorContains(t, err, "local")
	err = fssync.CopyDir(ctx, fsrc, fdst, false)
	require.Error(t, err)
	err = operations.CopyFile(ctx, fsrc, fsrc, "missing", "missing")
	require.ErrorContains(t, err, "--preserve-symlinks")
	server.mu.Lock()
	assert.Empty(t, server.blobs)
	server.mu.Unlock()
	_, err = fdst.Put(ctx, strings.NewReader("data"), object.NewStaticObjectInfo("unknown", time.Now(), 4, true, map[hash.Type]string{}, fs.Unknown))
	assert.Error(t, err)
}

func TestPreservationOverwriteSymlinkWithFile(t *testing.T) {
	ctx, ci := fs.AddConfig(context.Background())
	ci.PreserveSymlinks = true
	root := t.TempDir()
	require.NoError(t, os.Symlink("missing", filepath.Join(root, "source")))
	fsrc, err := local.NewFs(ctx, "local", root, configmap.Simple{})
	require.NoError(t, err)
	fdst, server := newPreservationFs(t, ctx)
	src, err := fsrc.NewObject(ctx, "source")
	require.NoError(t, err)
	dst, err := operations.Copy(ctx, fdst, nil, "blob", src)
	require.NoError(t, err)
	require.NoError(t, os.Remove(filepath.Join(root, "source")))
	require.NoError(t, os.WriteFile(filepath.Join(root, "source"), []byte("regular"), 0600))
	src, err = fsrc.NewObject(ctx, "source")
	require.NoError(t, err)
	_, err = operations.Copy(ctx, fdst, dst, "blob", src)
	require.NoError(t, err)
	server.mu.Lock()
	defer server.mu.Unlock()
	assert.Equal(t, "regular", string(server.blobs["blob"].body))
	assert.NotContains(t, server.blobs["blob"].meta, "is_symlink")
}

func TestPreservationBackup(t *testing.T) {
	ctx, ci := fs.AddConfig(context.Background())
	ci.PreservePOSIXProperties, ci.PreserveSymlinks = true, true
	root := t.TempDir()
	require.NoError(t, os.Symlink("missing", filepath.Join(root, "source")))
	fsrc, err := local.NewFs(ctx, "local", root, configmap.Simple{})
	require.NoError(t, err)
	fdst, server := newPreservationFs(t, ctx)
	src, err := fsrc.NewObject(ctx, "source")
	require.NoError(t, err)
	_, err = operations.Copy(ctx, fdst, nil, "blob", src)
	require.NoError(t, err)
	require.NoError(t, os.Remove(filepath.Join(root, "source")))
	require.NoError(t, os.WriteFile(filepath.Join(root, "source"), []byte("replacement"), 0600))
	ci.Suffix = ".bak"
	// Also exercise the manual fallback, which must copy the existing blob metadata.
	fdst.Features().Copy = func(context.Context, fs.Object, string) (fs.Object, error) {
		return nil, fs.ErrorCantCopy
	}
	require.NoError(t, operations.CopyFile(ctx, fdst, fsrc, "blob", "source"))
	server.mu.Lock()
	defer server.mu.Unlock()
	assert.Equal(t, "replacement", string(server.blobs["blob"].body))
	assert.NotContains(t, server.blobs["blob"].meta, "is_symlink")
	assert.Equal(t, "missing", string(server.blobs["blob.bak"].body))
	assert.Equal(t, "true", server.blobs["blob.bak"].meta["is_symlink"])
	assert.Equal(t, strconv.FormatUint(0120777, 10), server.blobs["blob.bak"].meta["permissions"])
}

func TestPreservationTimestampOnly(t *testing.T) {
	ctx, ci := fs.AddConfig(context.Background())
	ci.PreservePOSIXProperties = true
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "source"), []byte("same data"), 0600))
	oldTime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, os.Chtimes(filepath.Join(root, "source"), oldTime, oldTime))
	fsrc, err := local.NewFs(ctx, "local", root, configmap.Simple{})
	require.NoError(t, err)
	fdst, server := newPreservationFs(t, ctx)
	require.NoError(t, operations.CopyFile(ctx, fdst, fsrc, "blob", "source"))
	server.mu.Lock()
	before := server.uploads
	server.mu.Unlock()
	newTime := oldTime.Add(time.Hour)
	require.NoError(t, os.Chtimes(filepath.Join(root, "source"), newTime, newTime))
	require.NoError(t, operations.CopyFile(ctx, fdst, fsrc, "blob", "source"))
	server.mu.Lock()
	defer server.mu.Unlock()
	assert.Equal(t, before, server.uploads, "identical content should not be uploaded again")
	assert.Equal(t, 1, server.metadataWrites)
	assert.Equal(t, newTime.Format(timeFormatOut), server.blobs["blob"].meta["mtime"])
	assert.Equal(t, strconv.FormatInt(newTime.UnixNano(), 10), server.blobs["blob"].meta["modtime"])
}

func TestPreservationDirectoryRemoval(t *testing.T) {
	for _, tc := range []struct {
		name, root, dir, marker string
		directoryMarkers        bool
	}{
		{"RawDefault", "container", "empty", "empty", false},
		{"RawMarkers", "container", "empty", "empty", true},
		{"SlashDefault", "container", "empty", "empty/", false},
		{"SlashMarkers", "container", "empty", "empty/", true},
		{"RawRoot", "container/prefix/empty", "", "prefix/empty", false},
		{"SlashRoot", "container/prefix/empty", "", "prefix/empty/", true},
		{"BothMarkers", "container", "empty", "empty", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			fdst, server := newPreservationFs(t, ctx)
			server.mu.Lock()
			server.blobs[tc.marker] = preservationBlob{meta: fs.Metadata{dirMetaKey: dirMetaValue}}
			if tc.name == "BothMarkers" {
				server.blobs[tc.marker+"/"] = preservationBlob{meta: fs.Metadata{dirMetaKey: dirMetaValue}}
			}
			server.mu.Unlock()
			plain := newPreservationFsAt(t, ctx, fdst.svc.URL(), tc.root, tc.directoryMarkers)
			require.NoError(t, plain.Rmdir(ctx, tc.dir))
			server.mu.Lock()
			assert.NotContains(t, server.blobs, tc.marker)
			if tc.name == "BothMarkers" {
				assert.NotContains(t, server.blobs, tc.marker+"/")
			}
			server.mu.Unlock()
		})
	}
	ctx := context.Background()
	fdst, server := newPreservationFs(t, ctx)
	server.blobs["not-empty"] = preservationBlob{meta: fs.Metadata{dirMetaKey: dirMetaValue}}
	server.blobs["not-empty/file"] = preservationBlob{body: []byte("data")}
	server.blobs["regular"] = preservationBlob{body: []byte("regular file")}
	server.blobs["regular/"] = preservationBlob{meta: fs.Metadata{dirMetaKey: dirMetaValue}}
	assert.ErrorIs(t, fdst.Rmdir(ctx, "not-empty"), fs.ErrorDirectoryNotEmpty)
	require.NoError(t, fdst.Rmdir(ctx, "regular"))
	server.mu.Lock()
	assert.Contains(t, server.blobs, "not-empty")
	assert.Contains(t, server.blobs, "not-empty/file")
	assert.Contains(t, server.blobs, "regular")
	assert.NotContains(t, server.blobs, "regular/")
	server.mu.Unlock()
}

func TestPreservationSymlinkMetadataOverride(t *testing.T) {
	ctx, ci := fs.AddConfig(context.Background())
	ci.PreserveSymlinks, ci.Metadata = true, true
	root := t.TempDir()
	require.NoError(t, os.Symlink("missing", filepath.Join(root, "link")))
	require.NoError(t, os.WriteFile(filepath.Join(root, "file"), []byte("regular"), 0600))
	fsrc, err := local.NewFs(ctx, "local", root, configmap.Simple{})
	require.NoError(t, err)
	fdst, server := newPreservationFs(t, ctx)
	for name, spoof := range map[string]string{"link": "false", "file": "true"} {
		ci.MetadataSet = fs.Metadata{"is_symlink": spoof}
		require.NoError(t, operations.CopyFile(ctx, fdst, fsrc, name, name))
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	assert.Equal(t, "true", server.blobs["link"].meta["is_symlink"])
	assert.NotContains(t, server.blobs["file"].meta, "is_symlink")
}

func Example_mapPOSIXMetadata() {
	meta, _ := mapPOSIXMetadata(fs.Metadata{"mode": "100644", "uid": "1000", "gid": "1000"})
	fmt.Println(meta["permissions"], meta["posix_owner"])
	// Output: 33188 1000
}
