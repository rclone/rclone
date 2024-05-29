package operations_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/diskusage"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func rcNewRun(t *testing.T, method string) (*fstest.Run, *rc.Call) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping test on non local remote")
	}
	r := fstest.NewRun(t)
	call := rc.Calls.Get(method)
	assert.NotNil(t, call)
	cache.Put(r.LocalName, r.Flocal)
	cache.Put(r.FremoteName, r.Fremote)
	return r, call
}

// operations/about: Return the space used on the remote
func TestRcAbout(t *testing.T) {
	r, call := rcNewRun(t, "operations/about")
	r.Mkdir(context.Background(), r.Fremote)

	// Will get an error if remote doesn't support About
	expectedErr := r.Fremote.Features().About == nil

	in := rc.Params{
		"fs": r.FremoteName,
	}
	out, err := call.Fn(context.Background(), in)
	if expectedErr {
		assert.Error(t, err)
		return
	}
	require.NoError(t, err)

	// Can't really check the output much!
	assert.NotEqual(t, int64(0), out["Total"])
}

// operations/cleanup: Remove trashed files in the remote or path
func TestRcCleanup(t *testing.T) {
	r, call := rcNewRun(t, "operations/cleanup")

	in := rc.Params{
		"fs": r.LocalName,
	}
	out, err := call.Fn(context.Background(), in)
	require.Error(t, err)
	assert.Equal(t, rc.Params(nil), out)
	assert.Contains(t, err.Error(), "doesn't support cleanup")
}

// operations/copyfile: Copy a file from source remote to destination remote
func TestRcCopyfile(t *testing.T) {
	r, call := rcNewRun(t, "operations/copyfile")
	file1 := r.WriteFile("file1", "file1 contents", t1)
	r.Mkdir(context.Background(), r.Fremote)
	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t)

	in := rc.Params{
		"srcFs":     r.LocalName,
		"srcRemote": "file1",
		"dstFs":     r.FremoteName,
		"dstRemote": "file1-renamed",
	}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params(nil), out)

	r.CheckLocalItems(t, file1)
	file1.Path = "file1-renamed"
	r.CheckRemoteItems(t, file1)
}

// operations/copyurl: Copy the URL to the object
func TestRcCopyurl(t *testing.T) {
	r, call := rcNewRun(t, "operations/copyurl")
	contents := "file1 contents\n"
	file1 := r.WriteFile("file1", contents, t1)
	r.Mkdir(context.Background(), r.Fremote)
	r.CheckRemoteItems(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(contents))
		assert.NoError(t, err)
	}))
	defer ts.Close()

	in := rc.Params{
		"fs":           r.FremoteName,
		"remote":       "file1",
		"url":          ts.URL,
		"autoFilename": false,
		"noClobber":    false,
	}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params(nil), out)

	in = rc.Params{
		"fs":           r.FremoteName,
		"remote":       "file1",
		"url":          ts.URL,
		"autoFilename": false,
		"noClobber":    true,
	}
	out, err = call.Fn(context.Background(), in)
	require.Error(t, err)
	assert.Equal(t, rc.Params(nil), out)

	urlFileName := "filename.txt"
	in = rc.Params{
		"fs":           r.FremoteName,
		"remote":       "",
		"url":          ts.URL + "/" + urlFileName,
		"autoFilename": true,
		"noClobber":    false,
	}
	out, err = call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params(nil), out)

	in = rc.Params{
		"fs":           r.FremoteName,
		"remote":       "",
		"url":          ts.URL,
		"autoFilename": true,
		"noClobber":    false,
	}
	out, err = call.Fn(context.Background(), in)
	require.Error(t, err)
	assert.Equal(t, rc.Params(nil), out)

	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1, fstest.NewItem(urlFileName, contents, t1)}, nil, fs.ModTimeNotSupported)
}

// operations/delete: Remove files in the path
func TestRcDelete(t *testing.T) {
	r, call := rcNewRun(t, "operations/delete")

	file1 := r.WriteObject(context.Background(), "small", "1234567890", t2)                                                                                           // 10 bytes
	file2 := r.WriteObject(context.Background(), "medium", "------------------------------------------------------------", t1)                                        // 60 bytes
	file3 := r.WriteObject(context.Background(), "large", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", t1) // 100 bytes
	r.CheckRemoteItems(t, file1, file2, file3)

	in := rc.Params{
		"fs": r.FremoteName,
	}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params(nil), out)

	r.CheckRemoteItems(t)
}

// operations/deletefile: Remove the single file pointed to
func TestRcDeletefile(t *testing.T) {
	r, call := rcNewRun(t, "operations/deletefile")

	file1 := r.WriteObject(context.Background(), "small", "1234567890", t2)                                                    // 10 bytes
	file2 := r.WriteObject(context.Background(), "medium", "------------------------------------------------------------", t1) // 60 bytes
	r.CheckRemoteItems(t, file1, file2)

	in := rc.Params{
		"fs":     r.FremoteName,
		"remote": "small",
	}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params(nil), out)

	r.CheckRemoteItems(t, file2)
}

// operations/list: List the given remote and path in JSON format.
func TestRcList(t *testing.T) {
	r, call := rcNewRun(t, "operations/list")

	file1 := r.WriteObject(context.Background(), "a", "a", t1)
	file2 := r.WriteObject(context.Background(), "subdir/b", "bb", t2)

	r.CheckRemoteItems(t, file1, file2)

	in := rc.Params{
		"fs":     r.FremoteName,
		"remote": "",
	}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)

	list := out["list"].([]*operations.ListJSONItem)
	assert.Equal(t, 2, len(list))

	checkFile1 := func(got *operations.ListJSONItem) {
		assert.WithinDuration(t, t1, got.ModTime.When, time.Second)
		assert.Equal(t, "a", got.Path)
		assert.Equal(t, "a", got.Name)
		assert.Equal(t, int64(1), got.Size)
		assert.Equal(t, "application/octet-stream", got.MimeType)
		assert.Equal(t, false, got.IsDir)
	}
	checkFile1(list[0])

	checkSubdir := func(got *operations.ListJSONItem) {
		assert.Equal(t, "subdir", got.Path)
		assert.Equal(t, "subdir", got.Name)
		// assert.Equal(t, int64(-1), got.Size) // size can vary for directories
		assert.Equal(t, "inode/directory", got.MimeType)
		assert.Equal(t, true, got.IsDir)
	}
	checkSubdir(list[1])

	in = rc.Params{
		"fs":     r.FremoteName,
		"remote": "",
		"opt": rc.Params{
			"recurse": true,
		},
	}
	out, err = call.Fn(context.Background(), in)
	require.NoError(t, err)

	list = out["list"].([]*operations.ListJSONItem)
	assert.Equal(t, 3, len(list))
	checkFile1(list[0])
	checkSubdir(list[1])

	checkFile2 := func(got *operations.ListJSONItem) {
		assert.WithinDuration(t, t2, got.ModTime.When, time.Second)
		assert.Equal(t, "subdir/b", got.Path)
		assert.Equal(t, "b", got.Name)
		assert.Equal(t, int64(2), got.Size)
		assert.Equal(t, "application/octet-stream", got.MimeType)
		assert.Equal(t, false, got.IsDir)
	}
	checkFile2(list[2])
}

// operations/stat: Stat the given remote and path in JSON format.
func TestRcStat(t *testing.T) {
	r, call := rcNewRun(t, "operations/stat")

	file1 := r.WriteObject(context.Background(), "subdir/a", "a", t1)

	r.CheckRemoteItems(t, file1)

	fetch := func(t *testing.T, remotePath string) *operations.ListJSONItem {
		in := rc.Params{
			"fs":     r.FremoteName,
			"remote": remotePath,
		}
		out, err := call.Fn(context.Background(), in)
		require.NoError(t, err)
		return out["item"].(*operations.ListJSONItem)
	}

	t.Run("Root", func(t *testing.T) {
		stat := fetch(t, "")
		assert.Equal(t, "", stat.Path)
		assert.Equal(t, "", stat.Name)
		assert.Equal(t, int64(-1), stat.Size)
		assert.Equal(t, "inode/directory", stat.MimeType)
		assert.Equal(t, true, stat.IsDir)
	})

	t.Run("File", func(t *testing.T) {
		stat := fetch(t, "subdir/a")
		assert.WithinDuration(t, t1, stat.ModTime.When, time.Second)
		assert.Equal(t, "subdir/a", stat.Path)
		assert.Equal(t, "a", stat.Name)
		assert.Equal(t, int64(1), stat.Size)
		assert.Equal(t, "application/octet-stream", stat.MimeType)
		assert.Equal(t, false, stat.IsDir)
	})

	t.Run("Subdir", func(t *testing.T) {
		stat := fetch(t, "subdir")
		assert.Equal(t, "subdir", stat.Path)
		assert.Equal(t, "subdir", stat.Name)
		// assert.Equal(t, int64(-1), stat.Size) // size can vary for directories
		assert.Equal(t, "inode/directory", stat.MimeType)
		assert.Equal(t, true, stat.IsDir)
	})

	t.Run("NotFound", func(t *testing.T) {
		stat := fetch(t, "notfound")
		assert.Nil(t, stat)
	})
}

// operations/settier: Set the storage tier of a fs
func TestRcSetTier(t *testing.T) {
	ctx := context.Background()
	r, call := rcNewRun(t, "operations/settier")
	if !r.Fremote.Features().SetTier {
		t.Skip("settier not supported")
	}
	file1 := r.WriteObject(context.Background(), "file1", "file1 contents", t1)
	r.CheckRemoteItems(t, file1)

	// Because we don't know what the current tier options here are, let's
	// just get the current tier, and reuse that
	o, err := r.Fremote.NewObject(ctx, file1.Path)
	require.NoError(t, err)
	trr, ok := o.(fs.GetTierer)
	require.True(t, ok)
	ctier := trr.GetTier()
	in := rc.Params{
		"fs":   r.FremoteName,
		"tier": ctier,
	}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params(nil), out)

}

// operations/settier: Set the storage tier of a file
func TestRcSetTierFile(t *testing.T) {
	ctx := context.Background()
	r, call := rcNewRun(t, "operations/settierfile")
	if !r.Fremote.Features().SetTier {
		t.Skip("settier not supported")
	}
	file1 := r.WriteObject(context.Background(), "file1", "file1 contents", t1)
	r.CheckRemoteItems(t, file1)

	// Because we don't know what the current tier options here are, let's
	// just get the current tier, and reuse that
	o, err := r.Fremote.NewObject(ctx, file1.Path)
	require.NoError(t, err)
	trr, ok := o.(fs.GetTierer)
	require.True(t, ok)
	ctier := trr.GetTier()
	in := rc.Params{
		"fs":     r.FremoteName,
		"remote": "file1",
		"tier":   ctier,
	}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params(nil), out)

}

// operations/mkdir: Make a destination directory or container
func TestRcMkdir(t *testing.T) {
	ctx := context.Background()
	r, call := rcNewRun(t, "operations/mkdir")
	r.Mkdir(context.Background(), r.Fremote)

	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{}, []string{}, fs.GetModifyWindow(ctx, r.Fremote))

	in := rc.Params{
		"fs":     r.FremoteName,
		"remote": "subdir",
	}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params(nil), out)

	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{}, []string{"subdir"}, fs.GetModifyWindow(ctx, r.Fremote))
}

// operations/movefile: Move a file from source remote to destination remote
func TestRcMovefile(t *testing.T) {
	r, call := rcNewRun(t, "operations/movefile")
	file1 := r.WriteFile("file1", "file1 contents", t1)
	r.Mkdir(context.Background(), r.Fremote)
	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t)

	in := rc.Params{
		"srcFs":     r.LocalName,
		"srcRemote": "file1",
		"dstFs":     r.FremoteName,
		"dstRemote": "file1-renamed",
	}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params(nil), out)

	r.CheckLocalItems(t)
	file1.Path = "file1-renamed"
	r.CheckRemoteItems(t, file1)
}

// operations/purge: Remove a directory or container and all of its contents
func TestRcPurge(t *testing.T) {
	ctx := context.Background()
	r, call := rcNewRun(t, "operations/purge")
	file1 := r.WriteObject(context.Background(), "subdir/file1", "subdir/file1 contents", t1)

	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1}, []string{"subdir"}, fs.GetModifyWindow(ctx, r.Fremote))

	in := rc.Params{
		"fs":     r.FremoteName,
		"remote": "subdir",
	}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params(nil), out)

	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{}, []string{}, fs.GetModifyWindow(ctx, r.Fremote))
}

// operations/rmdir: Remove an empty directory or container
func TestRcRmdir(t *testing.T) {
	ctx := context.Background()
	r, call := rcNewRun(t, "operations/rmdir")
	r.Mkdir(context.Background(), r.Fremote)
	assert.NoError(t, r.Fremote.Mkdir(context.Background(), "subdir"))

	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{}, []string{"subdir"}, fs.GetModifyWindow(ctx, r.Fremote))

	in := rc.Params{
		"fs":     r.FremoteName,
		"remote": "subdir",
	}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params(nil), out)

	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{}, []string{}, fs.GetModifyWindow(ctx, r.Fremote))
}

// operations/rmdirs: Remove all the empty directories in the path
func TestRcRmdirs(t *testing.T) {
	ctx := context.Background()
	r, call := rcNewRun(t, "operations/rmdirs")
	r.Mkdir(context.Background(), r.Fremote)
	assert.NoError(t, r.Fremote.Mkdir(context.Background(), "subdir"))
	assert.NoError(t, r.Fremote.Mkdir(context.Background(), "subdir/subsubdir"))

	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{}, []string{"subdir", "subdir/subsubdir"}, fs.GetModifyWindow(ctx, r.Fremote))

	in := rc.Params{
		"fs":     r.FremoteName,
		"remote": "subdir",
	}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params(nil), out)

	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{}, []string{}, fs.GetModifyWindow(ctx, r.Fremote))

	assert.NoError(t, r.Fremote.Mkdir(context.Background(), "subdir"))
	assert.NoError(t, r.Fremote.Mkdir(context.Background(), "subdir/subsubdir"))

	in = rc.Params{
		"fs":        r.FremoteName,
		"remote":    "subdir",
		"leaveRoot": true,
	}
	out, err = call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params(nil), out)

	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{}, []string{"subdir"}, fs.GetModifyWindow(ctx, r.Fremote))

}

// operations/size: Count the number of bytes and files in remote
func TestRcSize(t *testing.T) {
	r, call := rcNewRun(t, "operations/size")
	file1 := r.WriteObject(context.Background(), "small", "1234567890", t2)                                                           // 10 bytes
	file2 := r.WriteObject(context.Background(), "subdir/medium", "------------------------------------------------------------", t1) // 60 bytes
	file3 := r.WriteObject(context.Background(), "subdir/subsubdir/large", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", t1)  // 50 bytes
	r.CheckRemoteItems(t, file1, file2, file3)

	in := rc.Params{
		"fs": r.FremoteName,
	}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, rc.Params{
		"count":    int64(3),
		"bytes":    int64(120),
		"sizeless": int64(0),
	}, out)
}

// operations/publiclink: Create or retrieve a public link to the given file or folder.
func TestRcPublicLink(t *testing.T) {
	r, call := rcNewRun(t, "operations/publiclink")
	in := rc.Params{
		"fs":     r.FremoteName,
		"remote": "",
		"expire": "5m",
		"unlink": false,
	}
	_, err := call.Fn(context.Background(), in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "doesn't support public links")
}

// operations/fsinfo: Return information about the remote
func TestRcFsInfo(t *testing.T) {
	r, call := rcNewRun(t, "operations/fsinfo")
	in := rc.Params{
		"fs": r.FremoteName,
	}
	got, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	want := operations.GetFsInfo(r.Fremote)
	assert.Equal(t, want.Name, got["Name"])
	assert.Equal(t, want.Root, got["Root"])
	assert.Equal(t, want.String, got["String"])
	assert.Equal(t, float64(want.Precision), got["Precision"])
	var hashes []interface{}
	for _, hash := range want.Hashes {
		hashes = append(hashes, hash)
	}
	assert.Equal(t, hashes, got["Hashes"])
	var features = map[string]interface{}{}
	for k, v := range want.Features {
		features[k] = v
	}
	assert.Equal(t, features, got["Features"])

}

// operations/uploadfile : Tests if upload file succeeds
func TestUploadFile(t *testing.T) {
	r, call := rcNewRun(t, "operations/uploadfile")
	ctx := context.Background()

	testFileName := "uploadfile-test.txt"
	testFileContent := "Hello World"
	r.WriteFile(testFileName, testFileContent, t1)
	testItem1 := fstest.NewItem(testFileName, testFileContent, t1)
	testItem2 := fstest.NewItem(path.Join("subdir", testFileName), testFileContent, t1)

	currentFile, err := os.Open(path.Join(r.LocalName, testFileName))
	require.NoError(t, err)

	defer func() {
		assert.NoError(t, currentFile.Close())
	}()

	formReader, contentType, _, err := rest.MultipartUpload(ctx, currentFile, url.Values{}, "file", testFileName)
	require.NoError(t, err)

	httpReq := httptest.NewRequest("POST", "/", formReader)
	httpReq.Header.Add("Content-Type", contentType)

	in := rc.Params{
		"_request": httpReq,
		"fs":       r.FremoteName,
		"remote":   "",
	}

	_, err = call.Fn(context.Background(), in)
	require.NoError(t, err)

	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{testItem1}, nil, fs.ModTimeNotSupported)

	assert.NoError(t, r.Fremote.Mkdir(context.Background(), "subdir"))

	currentFile2, err := os.Open(path.Join(r.LocalName, testFileName))
	require.NoError(t, err)

	defer func() {
		assert.NoError(t, currentFile2.Close())
	}()

	formReader, contentType, _, err = rest.MultipartUpload(ctx, currentFile2, url.Values{}, "file", testFileName)
	require.NoError(t, err)

	httpReq = httptest.NewRequest("POST", "/", formReader)
	httpReq.Header.Add("Content-Type", contentType)

	in = rc.Params{
		"_request": httpReq,
		"fs":       r.FremoteName,
		"remote":   "subdir",
	}

	_, err = call.Fn(context.Background(), in)
	require.NoError(t, err)

	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{testItem1, testItem2}, nil, fs.ModTimeNotSupported)

}

// operations/command: Runs a backend command
func TestRcCommand(t *testing.T) {
	r, call := rcNewRun(t, "backend/command")
	in := rc.Params{
		"fs":      r.FremoteName,
		"command": "noop",
		"opt": map[string]string{
			"echo": "true",
			"blue": "",
		},
		"arg": []string{
			"path1",
			"path2",
		},
	}
	got, err := call.Fn(context.Background(), in)
	if err != nil {
		assert.False(t, r.Fremote.Features().IsLocal, "mustn't fail on local remote")
		assert.Contains(t, err.Error(), "command not found")
		return
	}
	want := rc.Params{"result": map[string]interface{}{
		"arg": []string{
			"path1",
			"path2",
		},
		"name": "noop",
		"opt": map[string]string{
			"blue": "",
			"echo": "true",
		},
	}}
	assert.Equal(t, want, got)
	errTxt := "explosion in the sausage factory"
	in["opt"].(map[string]string)["error"] = errTxt
	_, err = call.Fn(context.Background(), in)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), errTxt)
}

// operations/command: Runs a backend command
func TestRcDu(t *testing.T) {
	ctx := context.Background()
	_, call := rcNewRun(t, "core/du")
	in := rc.Params{}
	out, err := call.Fn(ctx, in)
	if err == diskusage.ErrUnsupported {
		t.Skip(err)
	}
	assert.NotEqual(t, "", out["dir"])
	info := out["info"].(diskusage.Info)
	assert.True(t, info.Total != 0)
	assert.True(t, info.Total > info.Free)
	assert.True(t, info.Total > info.Available)
	assert.True(t, info.Free >= info.Available)
}

// operations/check: check the source and destination are the same
func TestRcCheck(t *testing.T) {
	ctx := context.Background()
	r, call := rcNewRun(t, "operations/check")
	r.Mkdir(ctx, r.Fremote)

	MD5SUMS := `
0ef726ce9b1a7692357ff70dd321d595  file1
deadbeefcafe00000000000000000000  subdir/file2
0386a8b8fcf672c326845c00ba41b9e2  subdir/subsubdir/file4
`

	file1 := r.WriteBoth(ctx, "file1", "file1 contents", t1)
	file2 := r.WriteFile("subdir/file2", MD5SUMS, t2)
	file3 := r.WriteObject(ctx, "subdir/subsubdir/file3", "file3 contents", t3)
	file4a := r.WriteFile("subdir/subsubdir/file4", "file4 contents", t3)
	file4b := r.WriteObject(ctx, "subdir/subsubdir/file4", "file4 different contents", t3)
	// operations.HashLister(ctx, hash.MD5, false, false, r.Fremote, os.Stdout)

	r.CheckLocalItems(t, file1, file2, file4a)
	r.CheckRemoteItems(t, file1, file3, file4b)

	pstring := func(items ...fstest.Item) *[]string {
		xs := make([]string, len(items))
		for i, item := range items {
			xs[i] = item.Path
		}
		return &xs
	}

	for _, testName := range []string{"Normal", "Download"} {
		t.Run(testName, func(t *testing.T) {
			in := rc.Params{
				"srcFs":        r.LocalName,
				"dstFs":        r.FremoteName,
				"combined":     true,
				"missingOnSrc": true,
				"missingOnDst": true,
				"match":        true,
				"differ":       true,
				"error":        true,
			}
			if testName == "Download" {
				in["download"] = true
			}
			out, err := call.Fn(ctx, in)
			require.NoError(t, err)

			combined := []string{
				"= " + file1.Path,
				"+ " + file2.Path,
				"- " + file3.Path,
				"* " + file4a.Path,
			}
			sort.Strings(combined)
			sort.Strings(*out["combined"].(*[]string))
			want := rc.Params{
				"missingOnSrc": pstring(file3),
				"missingOnDst": pstring(file2),
				"differ":       pstring(file4a),
				"error":        pstring(),
				"match":        pstring(file1),
				"combined":     &combined,
				"status":       "3 differences found",
				"success":      false,
			}
			if testName == "Normal" {
				want["hashType"] = "md5"
			}

			assert.Equal(t, want, out)
		})
	}

	t.Run("CheckFile", func(t *testing.T) {
		// The checksum file is treated as the source and srcFs is not used
		in := rc.Params{
			"dstFs":           r.FremoteName,
			"combined":        true,
			"missingOnSrc":    true,
			"missingOnDst":    true,
			"match":           true,
			"differ":          true,
			"error":           true,
			"checkFileFs":     r.LocalName,
			"checkFileRemote": file2.Path,
			"checkFileHash":   "md5",
		}
		out, err := call.Fn(ctx, in)
		require.NoError(t, err)

		combined := []string{
			"= " + file1.Path,
			"+ " + file2.Path,
			"- " + file3.Path,
			"* " + file4a.Path,
		}
		sort.Strings(combined)
		sort.Strings(*out["combined"].(*[]string))
		if strings.HasPrefix(out["status"].(string), "file not in") {
			out["status"] = "file not in"
		}
		want := rc.Params{
			"missingOnSrc": pstring(file3),
			"missingOnDst": pstring(file2),
			"differ":       pstring(file4a),
			"error":        pstring(),
			"match":        pstring(file1),
			"combined":     &combined,
			"hashType":     "md5",
			"status":       "file not in",
			"success":      false,
		}

		assert.Equal(t, want, out)
	})

}

// operations/hashsum: hashsum a directory
func TestRcHashsum(t *testing.T) {
	ctx := context.Background()
	r, call := rcNewRun(t, "operations/hashsum")
	r.Mkdir(ctx, r.Fremote)

	file1Contents := "file1 contents"
	file1 := r.WriteBoth(ctx, "hashsum-file1", file1Contents, t1)
	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file1)

	hasher := hash.NewMultiHasher()
	_, err := hasher.Write([]byte(file1Contents))
	require.NoError(t, err)

	for _, test := range []struct {
		ht       hash.Type
		base64   bool
		download bool
	}{
		{
			ht: r.Fremote.Hashes().GetOne(),
		}, {
			ht:     r.Fremote.Hashes().GetOne(),
			base64: true,
		}, {
			ht:       hash.Whirlpool,
			base64:   false,
			download: true,
		}, {
			ht:       hash.Whirlpool,
			base64:   true,
			download: true,
		},
	} {
		t.Run(fmt.Sprintf("hash=%v,base64=%v,download=%v", test.ht, test.base64, test.download), func(t *testing.T) {
			file1Hash, err := hasher.SumString(test.ht, test.base64)
			require.NoError(t, err)

			in := rc.Params{
				"fs":       r.FremoteName,
				"hashType": test.ht.String(),
				"base64":   test.base64,
				"download": test.download,
			}

			out, err := call.Fn(ctx, in)
			require.NoError(t, err)
			assert.Equal(t, test.ht.String(), out["hashType"])
			want := []string{
				fmt.Sprintf("%s  hashsum-file1", file1Hash),
			}
			assert.Equal(t, want, out["hashsum"])
		})
	}
}

// operations/hashsum: hashsum a single file
func TestRcHashsumFile(t *testing.T) {
	ctx := context.Background()
	r, call := rcNewRun(t, "operations/hashsum")
	r.Mkdir(ctx, r.Fremote)

	file1Contents := "file1 contents"
	file1 := r.WriteBoth(ctx, "hashsum-file1", file1Contents, t1)
	file2Contents := "file2 contents"
	file2 := r.WriteBoth(ctx, "hashsum-file2", file2Contents, t1)
	r.CheckLocalItems(t, file1, file2)
	r.CheckRemoteItems(t, file1, file2)

	// Make an fs pointing to just the file
	fsString := path.Join(r.FremoteName, file1.Path)

	in := rc.Params{
		"fs":       fsString,
		"hashType": "MD5",
		"download": true,
	}

	out, err := call.Fn(ctx, in)
	require.NoError(t, err)
	assert.Equal(t, "md5", out["hashType"])
	assert.Equal(t, []string{"0ef726ce9b1a7692357ff70dd321d595  hashsum-file1"}, out["hashsum"])
}
