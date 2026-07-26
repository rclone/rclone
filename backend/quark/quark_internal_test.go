package quark

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/quark/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderQRCode(t *testing.T) {
	got, err := renderQRCode("https://su.quark.cn/example?token=test")
	require.NoError(t, err)
	assert.Contains(t, got, "\x1b[40m")
	assert.Contains(t, got, "\x1b[107m")
	assert.Greater(t, strings.Count(got, "\n"), 10)
}

func TestConfigQRCodeLogin(t *testing.T) {
	var pollCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/cas/ajax/getTokenForQrcodeLogin":
			assert.Equal(t, "532", r.URL.Query().Get("client_id"))
			http.SetCookie(w, &http.Cookie{Name: "_UP_SESSION", Value: "pending", Path: "/"})
			_, _ = fmt.Fprint(w, `{"status":2000000,"message":"ok","data":{"members":{"token":"qr-token"}}}`)
		case "/cas/ajax/getServiceTicketByQrcodeToken":
			pollCount.Add(1)
			assert.Equal(t, "qr-token", r.URL.Query().Get("token"))
			cookie, err := r.Cookie("_UP_SESSION")
			require.NoError(t, err)
			assert.Equal(t, "pending", cookie.Value)
			_, _ = fmt.Fprint(w, `{"status":2000000,"message":"ok","data":{"members":{"service_ticket":"service-ticket"}}}`)
		case "/account/info":
			assert.Equal(t, "service-ticket", r.URL.Query().Get("st"))
			http.SetCookie(w, &http.Cookie{Name: "__pus", Value: "pus-value", Path: "/"})
			http.SetCookie(w, &http.Cookie{Name: "__puus", Value: "puus-value", Path: "/"})
			_, _ = fmt.Fprint(w, `{"success":true,"code":"OK","data":{"nickname":"rclone-test"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	oldEndpoints := currentEndpoints
	oldPollInterval := qrPollInterval
	oldPollTimeout := qrPollTimeout
	currentEndpoints = endpointSet{
		UOP:   server.URL,
		Pan:   server.URL,
		Drive: server.URL,
		Scan:  server.URL + "/scan?token=%s",
	}
	qrPollInterval = time.Millisecond
	qrPollTimeout = time.Second
	t.Cleanup(func() {
		currentEndpoints = oldEndpoints
		qrPollInterval = oldPollInterval
		qrPollTimeout = oldPollTimeout
	})

	m := configmap.Simple{}
	out, err := Config(context.Background(), "test", m, fs.ConfigIn{State: "qr_start"})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.Option)
	assert.Equal(t, "qr_poll", out.State)
	assert.Contains(t, out.Option.Help, "\x1b[40m")
	assert.Contains(t, out.Option.Help, server.URL+"/scan?token=qr-token")
	storedSession, ok := m.Get(configQRSession)
	require.True(t, ok)
	assert.NotContains(t, storedSession, "qr-token")
	_, err = obscure.Reveal(storedSession)
	require.NoError(t, err)

	out, err = Config(context.Background(), "test", m, fs.ConfigIn{State: "qr_poll", Result: "true"})
	require.NoError(t, err)
	assert.Nil(t, out)
	assert.EqualValues(t, 1, pollCount.Load())

	encodedCookie, ok := m.Get("cookie")
	require.True(t, ok)
	cookie, err := obscure.Reveal(encodedCookie)
	require.NoError(t, err)
	assert.Contains(t, cookie, "__pus=pus-value")
	assert.Contains(t, cookie, "__puus=puus-value")
}

func TestConfigQRCodeLoginPollsPendingStatus(t *testing.T) {
	var polls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/cas/ajax/getTokenForQrcodeLogin":
			http.SetCookie(w, &http.Cookie{Name: "_UP_SESSION", Value: "pending", Path: "/"})
			_, _ = fmt.Fprint(w, `{"status":2000000,"data":{"members":{"token":"qr-token"}}}`)
		case "/cas/ajax/getServiceTicketByQrcodeToken":
			if polls.Add(1) == 1 {
				_, _ = fmt.Fprint(w, `{"status":50004001,"message":"not scanned"}`)
				return
			}
			_, _ = fmt.Fprint(w, `{"status":2000000,"data":{"members":{"service_ticket":"service-ticket"}}}`)
		case "/account/info":
			http.SetCookie(w, &http.Cookie{Name: "__pus", Value: "one", Path: "/"})
			_, _ = fmt.Fprint(w, `{"success":true,"code":"OK","data":{}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	oldEndpoints := currentEndpoints
	oldInterval := qrPollInterval
	oldTimeout := qrPollTimeout
	currentEndpoints = endpointSet{UOP: server.URL, Pan: server.URL, Drive: server.URL, Scan: server.URL + "/scan?token=%s"}
	qrPollInterval = time.Millisecond
	qrPollTimeout = time.Second
	t.Cleanup(func() {
		currentEndpoints = oldEndpoints
		qrPollInterval = oldInterval
		qrPollTimeout = oldTimeout
	})

	m := configmap.Simple{}
	_, err := Config(context.Background(), "test", m, fs.ConfigIn{State: "qr_start"})
	require.NoError(t, err)
	out, err := Config(context.Background(), "test", m, fs.ConfigIn{State: "qr_poll", Result: "true"})
	require.NoError(t, err)
	assert.Nil(t, out)
	assert.EqualValues(t, 2, polls.Load())
}

func TestConfigQRCodeLoginRejectsExpiredCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/cas/ajax/getTokenForQrcodeLogin":
			_, _ = fmt.Fprint(w, `{"status":2000000,"data":{"members":{"token":"expired-token"}}}`)
		case "/cas/ajax/getServiceTicketByQrcodeToken":
			_, _ = fmt.Fprint(w, `{"status":50004003,"message":"expired"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	oldEndpoints := currentEndpoints
	currentEndpoints = endpointSet{UOP: server.URL, Pan: server.URL, Drive: server.URL, Scan: server.URL + "/scan?token=%s"}
	t.Cleanup(func() { currentEndpoints = oldEndpoints })
	m := configmap.Simple{}
	_, err := Config(context.Background(), "test", m, fs.ConfigIn{State: "qr_start"})
	require.NoError(t, err)
	_, err = Config(context.Background(), "test", m, fs.ConfigIn{State: "qr_poll", Result: "true"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestConfigKeepsExistingLogin(t *testing.T) {
	m := configmap.Simple{"cookie": obscure.MustObscure("__pus=one; __puus=two")}
	out, err := Config(context.Background(), "test", m, fs.ConfigIn{})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.Option)
	assert.Equal(t, "relogin", out.State)
	assert.False(t, out.Option.Default.(bool))

	out, err = Config(context.Background(), "test", m, fs.ConfigIn{State: "relogin", Result: "false"})
	require.NoError(t, err)
	assert.Nil(t, out)
}

func TestFileAPIRequests(t *testing.T) {
	var listCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		assert.Contains(t, r.Header.Get("Cookie"), "__pus=one")
		switch r.URL.Path {
		case "/1/clouddrive/file/sort":
			listCalls.Add(1)
			assert.Equal(t, "0", r.URL.Query().Get("pdir_fid"))
			assert.Equal(t, "1", r.URL.Query().Get("_page"))
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"list":[{"fid":"dir-id","pdir_fid":"0","file_name":"docs","dir":true,"size":0,"updated_at":1710000000000},{"fid":"file-id","pdir_fid":"0","file_name":"hello.txt","file":true,"size":5,"updated_at":1710000001000,"md5":"5d41402abc4b2a76b9719d911017c592","sha1":"aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d","format_type":"text/plain"}]}}`)
		case "/1/clouddrive/file":
			var body createDirRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "0", body.ParentID)
			assert.Equal(t, "new-dir", body.FileName)
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"fid":"new-dir-id"}}`)
		case "/1/clouddrive/file/move":
			var body moveRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, []string{"file-id"}, body.FileList)
			assert.Equal(t, "dir-id", body.ToParentID)
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"fid":"file-id"}}`)
		case "/1/clouddrive/file/rename":
			var body renameRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "file-id", body.ID)
			assert.Equal(t, "renamed.txt", body.FileName)
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"fid":"file-id"}}`)
		case "/1/clouddrive/file/delete":
			var body deleteRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, []string{"file-id"}, body.FileList)
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"fid":"file-id"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	f := newTestFs(t, server.URL)
	items, err := f.listAll(context.Background(), rootID)
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.True(t, items[0].IsDir())
	assert.Equal(t, "hello.txt", items[1].Name())
	assert.EqualValues(t, 1, listCalls.Load())

	id, err := f.createDir(context.Background(), rootID, "new-dir")
	require.NoError(t, err)
	assert.Equal(t, "new-dir-id", id)
	require.NoError(t, f.moveItem(context.Background(), "file-id", "dir-id"))
	require.NoError(t, f.renameItem(context.Background(), "file-id", "renamed.txt"))
	require.NoError(t, f.deleteItem(context.Background(), "file-id"))
}

func TestModTimeMilliseconds(t *testing.T) {
	item := api.Item{UpdatedAt: 1710000000123, LUpdatedAt: 1700000000000}
	assert.Equal(t, time.UnixMilli(1700000000000), item.ModTime())
	item.LUpdatedAt = 0
	assert.Equal(t, time.UnixMilli(1710000000123), item.ModTime())
}

func newTestFs(t *testing.T, driveURL string) *Fs {
	t.Helper()
	oldEndpoints := currentEndpoints
	currentEndpoints.Drive = driveURL
	t.Cleanup(func() { currentEndpoints = oldEndpoints })
	m := configmap.Simple{"cookie": obscure.MustObscure("__pus=one; __puus=two")}
	f, err := newFs(context.Background(), "test", "", m)
	require.NoError(t, err)
	require.NoError(t, f.dirCache.FindRoot(context.Background(), false))
	return f
}
