package dropbox

import (
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
func (f *Fs) InternalTestSharedFolderList(t *testing.T) {
	ctx := context.Background()

	// Create a subfolder and a file inside it using the normal API.
	// The test Fs root is something like /rclone-test-xxxxx so the
	// shared folder will be the test root directory itself.
	testDir := f.slashRoot
	testFileName := "shared-folder-test-file.txt"
	testFilePath := testDir + "/" + testFileName

	// Upload a small test file
	content := "hello shared folder"
	uploadArg := files.NewUploadArg(testFilePath)
	uploadArg.Mode = &files.WriteMode{Tagged: dropbox.Tagged{Tag: files.WriteModeOverwrite}}
	var err error
	err = f.pacer.Call(func() (bool, error) {
		_, err = f.srv.Upload(uploadArg, strings.NewReader(content))
		return shouldRetry(ctx, err)
	})
	require.NoError(t, err)
	defer func() {
		// Clean up test file
		_, _ = f.srv.DeleteV2(files.NewDeleteArg(testFilePath))
	}()

	// Share the test root folder
	folderName := f.opt.Enc.ToStandardName(strings.TrimPrefix(testDir, "/"))
	shareArg := sharing.NewShareFolderArg(testDir)
	var sharedFolderID string
	var shareLaunch *sharing.ShareFolderLaunch
	err = f.pacer.Call(func() (bool, error) {
		shareLaunch, err = f.sharing.ShareFolder(shareArg)
		return shouldRetry(ctx, err)
	})
	if err != nil {
		// If the folder is already shared, find its ID
		sharedFolderID, err = f.findSharedFolder(ctx, folderName)
		require.NoError(t, err, "ShareFolder failed and couldn't find existing share")
	} else {
		switch shareLaunch.Tag {
		case sharing.ShareFolderLaunchComplete:
			sharedFolderID = shareLaunch.Complete.SharedFolderId
		case sharing.ShareFolderLaunchAsyncJobId:
			// Poll for completion
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
		// Unshare the folder when done
		unshareArg := sharing.NewUnshareFolderArg(sharedFolderID)
		unshareArg.LeaveACopy = true
		_ = f.pacer.Call(func() (bool, error) {
			_, err = f.sharing.UnshareFolder(unshareArg)
			return shouldRetry(ctx, err)
		})
	}()

	// Now test listing with shared_folders mode.
	// Create a copy of the Fs with SharedFolders enabled.
	sharedFs := *f
	sharedFs.opt.SharedFolders = true

	// Test 1: listing root should include the shared folder
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

	// Test 2: listing the shared folder should show its contents
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

	// Test 3: NewObject should find a file inside a shared folder
	obj, err := sharedFs.NewObject(ctx, expectedRemote)
	require.NoError(t, err, "NewObject failed for shared folder file")
	assert.Equal(t, expectedRemote, obj.Remote())
	assert.Equal(t, int64(len(content)), obj.Size())

	// Test 4: Open should be able to read the file contents
	rc, err := obj.Open(ctx)
	require.NoError(t, err, "Open failed for shared folder file")
	buf, err := io.ReadAll(rc)
	require.NoError(t, rc.Close())
	require.NoError(t, err)
	assert.Equal(t, content, string(buf))
}

func (f *Fs) InternalTest(t *testing.T) {
	t.Run("PaperExport", f.InternalTestPaperExport)
	t.Run("SharedFolderList", f.InternalTestSharedFolderList)
}

var _ fstests.InternalTester = (*Fs)(nil)
