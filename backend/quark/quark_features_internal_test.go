package quark

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeatureParitySurface(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()
	f := newTestFs(t, server.URL)
	features := f.Features()
	assert.True(t, features.CanHaveEmptyDirectories)
	assert.True(t, features.ReadMimeType)
	assert.NotNil(t, features.ListR)
	assert.NotNil(t, features.PutStream)
	assert.NotNil(t, features.Copy)
	assert.NotNil(t, features.Move)
	assert.NotNil(t, features.DirMove)
	assert.NotNil(t, features.Purge)
	assert.NotNil(t, features.PublicLink)
	assert.NotNil(t, features.About)
	assert.NotNil(t, features.UserInfo)
}

func TestNewObjectNotFoundReturnsNil(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "/1/clouddrive/file/sort", r.URL.Path)
		_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"list":[]}}`)
	}))
	defer server.Close()

	f := newTestFs(t, server.URL)
	o, err := f.NewObject(context.Background(), "missing.txt")
	require.ErrorIs(t, err, fs.ErrorObjectNotFound)
	assert.Nil(t, o)
}

func TestWaitForItemVisibilityAndRemoval(t *testing.T) {
	var calls atomic.Int32
	var removing atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "/1/clouddrive/file/sort", r.URL.Path)
		call := calls.Add(1)
		visible := call >= 2
		if removing.Load() {
			visible = call == 1
		}
		if visible {
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"list":[{"fid":"item-id","pdir_fid":"0","file_name":"item.txt","file":true}]}}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"list":[]}}`)
	}))
	defer server.Close()

	oldInterval := taskPollInterval
	oldTimeout := taskPollTimeout
	taskPollInterval = time.Millisecond
	taskPollTimeout = time.Second
	t.Cleanup(func() {
		taskPollInterval = oldInterval
		taskPollTimeout = oldTimeout
	})

	f := newTestFs(t, server.URL)
	item, err := f.waitForItem(context.Background(), rootID, "item-id", "item.txt", true)
	require.NoError(t, err)
	require.NotNil(t, item)
	assert.GreaterOrEqual(t, calls.Load(), int32(2))

	calls.Store(0)
	removing.Store(true)
	item, err = f.waitForItem(context.Background(), rootID, "item-id", "", false)
	require.NoError(t, err)
	assert.Nil(t, item)
	assert.GreaterOrEqual(t, calls.Load(), int32(2))
}

func TestListAllPaginationAndNamePreservation(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		page := r.URL.Query().Get("_page")
		calls.Add(1)
		switch page {
		case "1":
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"list":[{"fid":"1","file_name":"one%20file.txt","file":true},{"fid":"2","file_name":"a&amp;amp;b","dir":true}]}}`)
		case "2":
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"list":[{"fid":"3","file_name":"three","file":true}]}}`)
		default:
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"list":[]}}`)
		}
	}))
	defer server.Close()

	f := newTestFs(t, server.URL)
	f.opt.ListPageSize = 2
	items, err := f.listAll(context.Background(), rootID)
	require.NoError(t, err)
	require.Len(t, items, 3)
	assert.Equal(t, "one%20file.txt", items[0].Name())
	assert.Equal(t, "a&amp;b", items[1].Name())
	assert.EqualValues(t, 2, calls.Load())
}

func TestListRRecursesDirectories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("pdir_fid") {
		case rootID:
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"list":[{"fid":"dir-id","pdir_fid":"0","file_name":"dir","dir":true},{"fid":"root-file","pdir_fid":"0","file_name":"root.txt","file":true,"size":1}]}}`)
		case "dir-id":
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"list":[{"fid":"child-file","pdir_fid":"dir-id","file_name":"child.txt","file":true,"size":2}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	f := newTestFs(t, server.URL)
	var names []string
	err := f.ListR(context.Background(), "", func(entries fs.DirEntries) error {
		for _, entry := range entries {
			names = append(names, entry.Remote())
		}
		return nil
	})
	require.NoError(t, err)
	sort.Strings(names)
	assert.Equal(t, []string{"dir", "dir/child.txt", "root.txt"}, names)
}

func TestCopyRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "/1/clouddrive/file/copy", r.URL.Path)
		var body copyRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, []string{"source-id"}, body.FileList)
		assert.Equal(t, "dest-id", body.ToParentID)
		_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"fid":"copied-id"}}`)
	}))
	defer server.Close()

	f := newTestFs(t, server.URL)
	id, err := f.copyItem(context.Background(), "source-id", "dest-id")
	require.NoError(t, err)
	assert.Equal(t, "copied-id", id)
}

func TestCopyHandlesAutoRenamedIntermediate(t *testing.T) {
	var renamed atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/1/clouddrive/file/sort":
			name := "source(1).txt"
			if renamed.Load() {
				name = "destination.txt"
			}
			_, _ = fmt.Fprintf(w, `{"status":200,"code":0,"data":{"list":[{"fid":"source-id","pdir_fid":"0","file_name":"source.txt","file":true,"size":10},{"fid":"copied-id","pdir_fid":"0","file_name":%q,"file":true,"size":10}]}}`, name)
		case "/1/clouddrive/file/copy":
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"fid":"copied-id"}}`)
		case "/1/clouddrive/file/rename":
			var body renameRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "copied-id", body.ID)
			assert.Equal(t, "destination.txt", body.FileName)
			renamed.Store(true)
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"fid":"copied-id"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	f := newTestFs(t, server.URL)
	source := &Object{fs: f, remote: "source.txt", id: "source-id", parentID: rootID, size: 10}
	destination, err := f.Copy(context.Background(), source, "destination.txt")
	require.NoError(t, err)
	require.IsType(t, (*Object)(nil), destination)
	assert.Equal(t, "copied-id", destination.(*Object).id)
	assert.Equal(t, "destination.txt", destination.Remote())
	assert.True(t, renamed.Load())
}

func TestMoveHandlesAutoRenamedIntermediate(t *testing.T) {
	var moved atomic.Bool
	var renamed atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/1/clouddrive/file/sort":
			items := `[{"fid":"existing-id","pdir_fid":"0","file_name":"other.txt","file":true,"size":10}]`
			if moved.Load() {
				name := "other(1).txt"
				if renamed.Load() {
					name = "file name.txt"
				}
				items = fmt.Sprintf(`[{"fid":"existing-id","pdir_fid":"0","file_name":"other.txt","file":true,"size":10},{"fid":"source-id","pdir_fid":"0","file_name":%q,"file":true,"size":10}]`, name)
			}
			_, _ = fmt.Fprintf(w, `{"status":200,"code":0,"data":{"list":%s}}`, items)
		case "/1/clouddrive/file/move":
			moved.Store(true)
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"fid":"source-id"}}`)
		case "/1/clouddrive/file/rename":
			var body renameRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "source-id", body.ID)
			assert.Equal(t, "file name.txt", body.FileName)
			renamed.Store(true)
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"fid":"source-id"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	oldInterval := taskPollInterval
	oldTimeout := taskPollTimeout
	taskPollInterval = time.Millisecond
	taskPollTimeout = 2 * time.Second
	t.Cleanup(func() {
		taskPollInterval = oldInterval
		taskPollTimeout = oldTimeout
	})

	f := newTestFs(t, server.URL)
	source := &Object{fs: f, remote: "moveTest/other.txt", id: "source-id", parentID: "source-parent", size: 10}
	destination, err := f.Move(context.Background(), source, "file name.txt")
	require.NoError(t, err)
	require.IsType(t, (*Object)(nil), destination)
	assert.Equal(t, "source-id", destination.(*Object).id)
	assert.Equal(t, "file name.txt", destination.Remote())
	assert.True(t, moved.Load())
	assert.True(t, renamed.Load())
}

func TestAbout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "/1/clouddrive/member", r.URL.Path)
		_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"use_capacity":40,"total_capacity":100}}`)
	}))
	defer server.Close()

	f := newTestFs(t, server.URL)
	usage, err := f.About(context.Background())
	require.NoError(t, err)
	require.NotNil(t, usage.Total)
	require.NotNil(t, usage.Used)
	require.NotNil(t, usage.Free)
	assert.EqualValues(t, 100, *usage.Total)
	assert.EqualValues(t, 40, *usage.Used)
	assert.EqualValues(t, 60, *usage.Free)
}

func TestUserInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "/account/info", r.URL.Path)
		_, _ = fmt.Fprint(w, `{"success":true,"code":"OK","data":{"nickname":"tester","member_type":"SUPER_VIP"}}`)
	}))
	defer server.Close()

	oldEndpoints := currentEndpoints
	currentEndpoints.Pan = server.URL
	t.Cleanup(func() { currentEndpoints = oldEndpoints })
	f := newTestFs(t, server.URL)
	info, err := f.UserInfo(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "tester", info["Nickname"])
	assert.Equal(t, "SUPER_VIP", info["MemberType"])
}

func TestCreatePublicLink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/1/clouddrive/share":
			var body shareRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, []string{"file-id"}, body.FileIDs)
			assert.Equal(t, 2, body.ExpiredType)
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"task_id":"share-task","task_sync":false}}`)
		case "/1/clouddrive/task":
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"status":2,"share_id":"share-id"}}`)
		case "/1/clouddrive/share/password":
			var body sharePasswordRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "share-id", body.ShareID)
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"share_url":"https://pan.quark.cn/s/example"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	oldInterval := taskPollInterval
	oldTimeout := taskPollTimeout
	taskPollInterval = time.Millisecond
	taskPollTimeout = time.Second
	t.Cleanup(func() {
		taskPollInterval = oldInterval
		taskPollTimeout = oldTimeout
	})

	f := newTestFs(t, server.URL)
	link, err := f.createShare(context.Background(), "file.txt", []string{"file-id"}, 24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, "https://pan.quark.cn/s/example", link)
}

func TestUnlinkPublicLinks(t *testing.T) {
	var deleted []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/1/clouddrive/file/sort":
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"list":[{"fid":"file-id","pdir_fid":"0","file_name":"file.txt","file":true,"size":4}]}}`)
		case "/1/clouddrive/share/mypage/detail":
			assert.Equal(t, "1", r.URL.Query().Get("_page"))
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"list":[{"share_id":"matching-1","first_fid":"file-id"},{"share_id":"other","first_fid":"other-id"},{"share_id":"matching-2","first_fid":"file-id"}]}}`)
		case "/1/clouddrive/share/delete":
			var body struct {
				ShareIDs []string `json:"share_ids"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			deleted = body.ShareIDs
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	f := newTestFs(t, server.URL)
	link, err := f.PublicLink(context.Background(), "file.txt", fs.DurationOff, true)
	require.NoError(t, err)
	assert.Empty(t, link)
	assert.Equal(t, []string{"matching-1", "matching-2"}, deleted)
}

func TestRmdirRejectsNonEmptyDirectory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"list":[{"fid":"child","file_name":"child","file":true}]}}`)
	}))
	defer server.Close()

	f := newTestFs(t, server.URL)
	f.dirCache.Put("nonempty", "dir-id")
	err := f.Rmdir(context.Background(), "nonempty")
	assert.ErrorIs(t, err, fs.ErrorDirectoryNotEmpty)
}
