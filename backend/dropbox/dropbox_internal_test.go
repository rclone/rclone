package dropbox

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox/async"
	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox/files"
	"github.com/dropbox/dropbox-sdk-go-unofficial/v6/dropbox/sharing"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInternalCheckPathLength(t *testing.T) {
	rep := func(n int, r rune) (out string) {
		rs := make([]rune, n)
		for i := range rs {
			rs[i] = r
		}
		return string(rs)
	}
	for _, test := range []struct {
		in string
		ok bool
	}{
		{in: "", ok: true},
		{in: rep(maxFileNameLength, 'a'), ok: true},
		{in: rep(maxFileNameLength+1, 'a'), ok: false},
		{in: rep(maxFileNameLength, '£'), ok: true},
		{in: rep(maxFileNameLength+1, '£'), ok: false},
		{in: rep(maxFileNameLength, '☺'), ok: true},
		{in: rep(maxFileNameLength+1, '☺'), ok: false},
		{in: rep(maxFileNameLength, '你'), ok: true},
		{in: rep(maxFileNameLength+1, '你'), ok: false},
		{in: "/ok/ok", ok: true},
		{in: "/ok/" + rep(maxFileNameLength, 'a') + "/ok", ok: true},
		{in: "/ok/" + rep(maxFileNameLength+1, 'a') + "/ok", ok: false},
		{in: "/ok/" + rep(maxFileNameLength, '£') + "/ok", ok: true},
		{in: "/ok/" + rep(maxFileNameLength+1, '£') + "/ok", ok: false},
		{in: "/ok/" + rep(maxFileNameLength, '☺') + "/ok", ok: true},
		{in: "/ok/" + rep(maxFileNameLength+1, '☺') + "/ok", ok: false},
		{in: "/ok/" + rep(maxFileNameLength, '你') + "/ok", ok: true},
		{in: "/ok/" + rep(maxFileNameLength+1, '你') + "/ok", ok: false},
	} {

		err := checkPathLength(test.in)
		assert.Equal(t, test.ok, err == nil, test.in)
	}
}

func (f *Fs) importPaperForTest(t *testing.T) {
	content := `# test doc

Lorem ipsum __dolor__ sit amet
[link](http://google.com)
`

	arg := files.PaperCreateArg{
		Path:         f.slashRootSlash + "export.paper",
		ImportFormat: &files.ImportFormat{Tagged: dropbox.Tagged{Tag: files.ImportFormatMarkdown}},
	}
	var err error
	err = f.pacer.Call(func() (bool, error) {
		reader := strings.NewReader(content)
		_, err = f.srv.PaperCreate(&arg, reader)
		return shouldRetry(context.Background(), err)
	})
	require.NoError(t, err)
}

func (f *Fs) InternalTestPaperExport(t *testing.T) {
	ctx := context.Background()
	f.importPaperForTest(t)

	f.exportExts = []exportExtension{"html"}

	obj, err := f.NewObject(ctx, "export.html")
	require.NoError(t, err)

	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, rc.Close()) }()

	buf, err := io.ReadAll(rc)
	require.NoError(t, err)
	text := string(buf)

	for _, excerpt := range []string{
		"Lorem ipsum",
		"<b>dolor</b>",
		`href="http://google.com"`,
	} {
		require.Contains(t, text, excerpt)
	}
}

// InternalTestSharedFolder tests both read and write operations on shared folders.
// It uses a single shared folder setup to avoid re-sharing between subtests
// (Dropbox has eventual consistency issues with share/unshare cycles).
func (f *Fs) InternalTestSharedFolder(t *testing.T) {
	ctx := context.Background()
	testDir := f.slashRoot

	// Share the test root folder
	folderName := f.opt.Enc.ToStandardName(strings.TrimPrefix(testDir, "/"))
	shareArg := sharing.NewShareFolderArg(testDir)
	var sharedFolderID string
	var shareLaunch *sharing.ShareFolderLaunch
	var err error
	err = f.pacer.Call(func() (bool, error) {
		shareLaunch, err = f.sharing.ShareFolder(shareArg)
		return shouldRetry(ctx, err)
	})
	if err != nil {
		sharedFolderID, err = f.findSharedFolder(ctx, folderName)
		require.NoError(t, err, "ShareFolder failed and couldn't find existing share")
	} else {
		switch shareLaunch.Tag {
		case sharing.ShareFolderLaunchComplete:
			sharedFolderID = shareLaunch.Complete.SharedFolderId
		case sharing.ShareFolderLaunchAsyncJobId:
			pollArg := async.PollArg{AsyncJobId: shareLaunch.AsyncJobId}
			for range 30 {
				time.Sleep(time.Second)
				var status *sharing.ShareFolderJobStatus
				err = f.pacer.Call(func() (bool, error) {
					status, err = f.sharing.CheckShareJobStatus(&pollArg)
					return shouldRetry(ctx, err)
				})
				require.NoError(t, err)
				if status.Tag == sharing.ShareFolderJobStatusComplete {
					sharedFolderID = status.Complete.SharedFolderId
					break
				}
			}
			require.NotEmpty(t, sharedFolderID, "share folder job did not complete")
		}
	}
	require.NotEmpty(t, sharedFolderID)

	defer func() {
		unshareArg := sharing.NewUnshareFolderArg(sharedFolderID)
		unshareArg.LeaveACopy = true
		_ = f.pacer.Call(func() (bool, error) {
			_, err = f.sharing.UnshareFolder(unshareArg)
			return shouldRetry(ctx, err)
		})
	}()

	sf := *f //nolint:govet // intentional copy; mutex is zero-valued and unused before this point
	sf.opt.SharedFolders = true
	sf.features = (&fs.Features{
		CaseInsensitive:         true,
		ReadMimeType:            false,
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, &sf)
	sharedFs := &sf

	// Upload a test file using the normal API for listing tests
	testFileName := "shared-folder-test-file.txt"
	testFilePath := testDir + "/" + testFileName
	content := "hello shared folder"
	uploadArg := files.NewUploadArg(testFilePath)
	uploadArg.Mode = &files.WriteMode{Tagged: dropbox.Tagged{Tag: files.WriteModeOverwrite}}
	err = f.pacer.Call(func() (bool, error) {
		_, err = f.srv.Upload(uploadArg, strings.NewReader(content))
		return shouldRetry(ctx, err)
	})
	require.NoError(t, err)
	defer func() {
		_, _ = f.srv.DeleteV2(files.NewDeleteArg(testFilePath))
	}()

	t.Run("List", func(t *testing.T) {
		// listing root should include the shared folder
		var rootEntries fs.DirEntries
		err = sharedFs.ListP(ctx, "", func(entries fs.DirEntries) error {
			rootEntries = append(rootEntries, entries...)
			return nil
		})
		require.NoError(t, err)

		found := false
		for _, entry := range rootEntries {
			if entry.Remote() == folderName {
				found = true
				break
			}
		}
		assert.True(t, found, "shared folder %q not found in root listing (have %v)", folderName, rootEntries)

		// listing the shared folder should show its contents
		var dirEntries fs.DirEntries
		err = sharedFs.ListP(ctx, folderName, func(entries fs.DirEntries) error {
			dirEntries = append(dirEntries, entries...)
			return nil
		})
		require.NoError(t, err)

		foundFile := false
		expectedRemote := folderName + "/" + testFileName
		for _, entry := range dirEntries {
			if entry.Remote() == expectedRemote {
				foundFile = true
				break
			}
		}
		assert.True(t, foundFile, "file %q not found in shared folder listing (have %v)", expectedRemote, dirEntries)
	})

	t.Run("NewObject", func(t *testing.T) {
		expectedRemote := folderName + "/" + testFileName
		obj, err := sharedFs.NewObject(ctx, expectedRemote)
		require.NoError(t, err, "NewObject failed for shared folder file")
		assert.Equal(t, expectedRemote, obj.Remote())
		assert.Equal(t, int64(len(content)), obj.Size())
	})

	t.Run("Open", func(t *testing.T) {
		expectedRemote := folderName + "/" + testFileName
		obj, err := sharedFs.NewObject(ctx, expectedRemote)
		require.NoError(t, err)
		rc, err := obj.Open(ctx)
		require.NoError(t, err, "Open failed for shared folder file")
		buf, err := io.ReadAll(rc)
		require.NoError(t, rc.Close())
		require.NoError(t, err)
		assert.Equal(t, content, string(buf))
	})

	t.Run("Put", func(t *testing.T) {
		// Upload a file via Put
		content := "hello from shared folder put"
		remote := folderName + "/test-put-file.txt"
		src := object.NewStaticObjectInfo(remote, time.Now(), int64(len(content)), true, nil, nil)
		obj, err := sharedFs.Put(ctx, bytes.NewReader([]byte(content)), src)
		require.NoError(t, err)
		assert.Equal(t, remote, obj.Remote())
		assert.Equal(t, int64(len(content)), obj.Size())

		// Verify we can read back the content
		rc, err := obj.Open(ctx)
		require.NoError(t, err)
		buf, err := io.ReadAll(rc)
		require.NoError(t, rc.Close())
		require.NoError(t, err)
		assert.Equal(t, content, string(buf))

		// Clean up
		require.NoError(t, obj.Remove(ctx))
	})

	t.Run("Hash", func(t *testing.T) {
		// Upload a file and check its hash
		content := "hash test content"
		remote := folderName + "/test-hash-file.txt"
		src := object.NewStaticObjectInfo(remote, time.Now(), int64(len(content)), true, nil, nil)
		obj, err := sharedFs.Put(ctx, bytes.NewReader([]byte(content)), src)
		require.NoError(t, err)
		defer func() { _ = obj.Remove(ctx) }()

		hash, err := obj.Hash(ctx, DbHashType)
		require.NoError(t, err)
		assert.NotEmpty(t, hash, "hash should not be empty")
	})

	t.Run("Mkdir", func(t *testing.T) {
		// Create a subdirectory within the shared folder
		subDir := folderName + "/test-mkdir-subdir"
		err := sharedFs.Mkdir(ctx, subDir)
		require.NoError(t, err)

		// Verify it appears in the listing
		var entries fs.DirEntries
		err = sharedFs.ListP(ctx, folderName, func(e fs.DirEntries) error {
			entries = append(entries, e...)
			return nil
		})
		require.NoError(t, err)
		found := false
		for _, entry := range entries {
			if entry.Remote() == subDir {
				found = true
				break
			}
		}
		assert.True(t, found, "subdirectory %q not found in listing", subDir)

		// Clean up
		require.NoError(t, sharedFs.Rmdir(ctx, subDir))
	})

	t.Run("Rmdir", func(t *testing.T) {
		// Create and then remove a subdirectory
		subDir := folderName + "/test-rmdir-subdir"
		require.NoError(t, sharedFs.Mkdir(ctx, subDir))
		require.NoError(t, sharedFs.Rmdir(ctx, subDir))

		// Verify it's gone
		var entries fs.DirEntries
		err := sharedFs.ListP(ctx, folderName, func(e fs.DirEntries) error {
			entries = append(entries, e...)
			return nil
		})
		require.NoError(t, err)
		for _, entry := range entries {
			assert.NotEqual(t, subDir, entry.Remote(), "subdirectory should have been removed")
		}
	})

	t.Run("RmdirSharedFolderRoot", func(t *testing.T) {
		// Removing a shared folder itself should fail
		err := sharedFs.Rmdir(ctx, folderName)
		assert.Error(t, err, "should not be able to remove shared folder itself")
	})

	t.Run("Remove", func(t *testing.T) {
		// Upload and then delete a file
		content := "file to remove"
		remote := folderName + "/test-remove-file.txt"
		src := object.NewStaticObjectInfo(remote, time.Now(), int64(len(content)), true, nil, nil)
		obj, err := sharedFs.Put(ctx, bytes.NewReader([]byte(content)), src)
		require.NoError(t, err)

		err = obj.Remove(ctx)
		require.NoError(t, err)

		// Verify it's gone
		_, err = sharedFs.NewObject(ctx, remote)
		assert.ErrorIs(t, err, fs.ErrorObjectNotFound)
	})

	t.Run("Copy", func(t *testing.T) {
		// Upload a file, then server-side copy it within the same shared folder
		content := "file to copy"
		srcRemote := folderName + "/test-copy-src.txt"
		dstRemote := folderName + "/test-copy-dst.txt"

		src := object.NewStaticObjectInfo(srcRemote, time.Now(), int64(len(content)), true, nil, nil)
		srcObj, err := sharedFs.Put(ctx, bytes.NewReader([]byte(content)), src)
		require.NoError(t, err)
		defer func() { _ = srcObj.Remove(ctx) }()

		dstObj, err := sharedFs.Copy(ctx, srcObj, dstRemote)
		require.NoError(t, err)
		defer func() { _ = dstObj.Remove(ctx) }()

		assert.Equal(t, dstRemote, dstObj.Remote())
		assert.Equal(t, int64(len(content)), dstObj.Size())

		// Verify content
		rc, err := dstObj.Open(ctx)
		require.NoError(t, err)
		buf, err := io.ReadAll(rc)
		require.NoError(t, rc.Close())
		require.NoError(t, err)
		assert.Equal(t, content, string(buf))
	})

	t.Run("Move", func(t *testing.T) {
		// Upload a file, then server-side move it within the same shared folder
		content := "file to move"
		srcRemote := folderName + "/test-move-src.txt"
		dstRemote := folderName + "/test-move-dst.txt"

		src := object.NewStaticObjectInfo(srcRemote, time.Now(), int64(len(content)), true, nil, nil)
		srcObj, err := sharedFs.Put(ctx, bytes.NewReader([]byte(content)), src)
		require.NoError(t, err)

		dstObj, err := sharedFs.Move(ctx, srcObj, dstRemote)
		require.NoError(t, err)
		defer func() { _ = dstObj.Remove(ctx) }()

		assert.Equal(t, dstRemote, dstObj.Remote())

		// Source should be gone
		_, err = sharedFs.NewObject(ctx, srcRemote)
		assert.ErrorIs(t, err, fs.ErrorObjectNotFound)
	})

	t.Run("Purge", func(t *testing.T) {
		// Create a subdir with a file, then purge
		subDir := folderName + "/test-purge-subdir"
		require.NoError(t, sharedFs.Mkdir(ctx, subDir))

		content := "file in purge dir"
		remote := subDir + "/file.txt"
		src := object.NewStaticObjectInfo(remote, time.Now(), int64(len(content)), true, nil, nil)
		_, err := sharedFs.Put(ctx, bytes.NewReader([]byte(content)), src)
		require.NoError(t, err)

		err = sharedFs.Purge(ctx, subDir)
		require.NoError(t, err)
	})
}

func (f *Fs) InternalTest(t *testing.T) {
	t.Run("PaperExport", f.InternalTestPaperExport)
	t.Run("SharedFolder", f.InternalTestSharedFolder)
}

var _ fstests.InternalTester = (*Fs)(nil)
