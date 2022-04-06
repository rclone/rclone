package operations_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fstest"
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
	defer r.Finalise()
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
	defer r.Finalise()

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
	defer r.Finalise()
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
	defer r.Finalise()
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
	defer r.Finalise()

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
	defer r.Finalise()

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
	defer r.Finalise()

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
		assert.Equal(t, int64(-1), got.Size)
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
	defer r.Finalise()

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
		assert.Equal(t, int64(-1), stat.Size)
		assert.Equal(t, "inode/directory", stat.MimeType)
		assert.Equal(t, true, stat.IsDir)
	})

	t.Run("NotFound", func(t *testing.T) {
		stat := fetch(t, "notfound")
		assert.Nil(t, stat)
	})
}

// operations/mkdir: Make a destination directory or container
func TestRcMkdir(t *testing.T) {
	ctx := context.Background()
	r, call := rcNewRun(t, "operations/mkdir")
	defer r.Finalise()
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
	defer r.Finalise()
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
	defer r.Finalise()
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
	defer r.Finalise()
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
	defer r.Finalise()
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
	defer r.Finalise()
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
	defer r.Finalise()
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
	defer r.Finalise()
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

//operations/uploadfile : Tests if upload file succeeds
//
func TestUploadFile(t *testing.T) {
	r, call := rcNewRun(t, "operations/uploadfile")
	defer r.Finalise()
	ctx := context.Background()

	testFileName := "test.txt"
	testFileContent := "Hello World"
	r.WriteFile(testFileName, testFileContent, t1)
	testItem1 := fstest.NewItem(testFileName, testFileContent, t1)
	testItem2 := fstest.NewItem(path.Join("subdir", testFileName), testFileContent, t1)

	currentFile, err := os.Open(path.Join(r.LocalName, testFileName))
	require.NoError(t, err)

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

	currentFile, err = os.Open(path.Join(r.LocalName, testFileName))
	require.NoError(t, err)

	formReader, contentType, _, err = rest.MultipartUpload(ctx, currentFile, url.Values{}, "file", testFileName)
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
	defer r.Finalise()
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
