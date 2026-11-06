package tarkle

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testWS = "wskey001"

type seen struct{ path, workspace string }

// newTestServer answers just enough of the API; it records the
// X-Workspace-Key of every request, API and storage alike.
func newTestServer(t *testing.T, accountWide bool, workspaces string) (*httptest.Server, func() []seen) {
	var mu sync.Mutex
	var log []seen
	var srv *httptest.Server
	file := `{"id":"1","type":"file","name":"f.txt","size":3,"parent_id":null,"updated_at":"2020-01-02T03:04:05.000Z","modified_at":"2020-01-02T03:04:05.000Z"}`
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		log = append(log, seen{r.URL.Path, r.Header.Get("X-Workspace-Key")})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		p := strings.TrimPrefix(r.URL.Path, "/api/v1")
		switch {
		case strings.HasPrefix(r.URL.Path, "/store/"):
			_, _ = io.Copy(io.Discard, r.Body)
			_, _ = w.Write([]byte("abc"))
		case p == "/workspaces":
			_, _ = w.Write([]byte(`{"account_wide":` + map[bool]string{true: "true", false: "false"}[accountWide] + `,"workspaces":` + workspaces + `}`))
		case p == "/files/by-path":
			if strings.HasSuffix(r.URL.Query().Get("path"), "/f.txt") {
				_, _ = w.Write([]byte(file))
				return
			}
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"no"}}`))
		case p == "/files" && r.Method == "GET":
			_, _ = w.Write([]byte(`{"entries":[` + file + `],"next_cursor":null}`))
		case p == "/usage":
			_, _ = w.Write([]byte(`{"storage":{"used_bytes":3,"limit_bytes":100,"free_bytes":97}}`))
		case p == "/folders":
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{"id":"22222222-2222-4222-8222-222222222222","type":"folder","name":"sub"}`))
		case p == "/uploads":
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{"action":"upload","mode":"single","id":"33333333-3333-4333-8333-333333333333","part_size":3,"part_count":1,"uploads":[{"partNumber":1,"url":"` + srv.URL + `/store/put"}]}`))
		case strings.HasSuffix(p, "/complete"):
			_, _ = w.Write([]byte(strings.Replace(file, `"f.txt"`, `"new.txt"`, 1)))
		case strings.HasSuffix(p, "/download"):
			_, _ = w.Write([]byte(`{"download_url":"` + srv.URL + `/store/get","expires_in":900}`))
		case r.Method == "DELETE":
			_, _ = w.Write([]byte(`{"deleted":true,"deleted_transfers":[]}`))
		default:
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"no"}}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, func() []seen { mu.Lock(); defer mu.Unlock(); return append([]seen(nil), log...) }
}

func testConfig(url, workspace string) configmap.Simple {
	return configmap.Simple{"token": obscure.MustObscure("tok"), "url": url, "workspace": workspace, "list_chunk": "1000", "upload_concurrency": "4"}
}

func TestWorkspaceHeaderOnEveryAPICallNeverOnStorage(t *testing.T) {
	ctx := context.Background()
	srv, requests := newTestServer(t, true, `[]`)
	f, err := NewFs(ctx, "t", "", testConfig(srv.URL, testWS))
	require.NoError(t, err)
	tf := f.(*Fs)

	_, err = tf.About(ctx)
	require.NoError(t, err)
	require.NoError(t, tf.Mkdir(ctx, "sub"))
	_, err = tf.List(ctx, "")
	require.NoError(t, err)
	src := object.NewStaticObjectInfo("new.txt", time.Now(), 3, true, nil, nil)
	_, err = tf.Put(ctx, bytes.NewReader([]byte("abc")), src)
	require.NoError(t, err)
	o, err := tf.NewObject(ctx, "f.txt")
	require.NoError(t, err)
	in, err := o.Open(ctx)
	require.NoError(t, err)
	_ = in.Close()
	require.NoError(t, o.Remove(ctx))
	_ = o.SetModTime(ctx, time.Now()) // PATCH answers 404 in this server: only the header matters

	var api, storage int
	for _, r := range requests() {
		if strings.HasPrefix(r.path, "/store/") {
			storage++
			assert.Empty(t, r.workspace, "storage request %s must not carry the workspace header", r.path)
		} else {
			api++
			assert.Equal(t, testWS, r.workspace, "API request %s must carry the workspace header", r.path)
		}
	}
	assert.Greater(t, api, 6)
	assert.Equal(t, 2, storage, "one PUT and one GET went to the storage URLs")
}

func TestNewFsAccountWideTokenWithoutWorkspaceListsThem(t *testing.T) {
	for name, list := range map[string]string{
		"two":  `[{"workspace_key":"` + testWS + `","name":"Acme"},{"workspace_key":"wskey002","name":"Client Co"}]`,
		"one":  `[{"workspace_key":"` + testWS + `","name":"Acme"}]`,
		"none": `[]`,
	} {
		t.Run(name, func(t *testing.T) {
			srv, requests := newTestServer(t, true, list)
			_, err := NewFs(context.Background(), "t", "", testConfig(srv.URL, ""))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "workspace option is required")
			if name != "none" {
				assert.Contains(t, err.Error(), testWS+"  Acme")
			}
			if name == "two" {
				assert.Contains(t, err.Error(), "Client Co")
			}
			assert.Len(t, requests(), 1, "nothing but GET /workspaces: no workspace is picked for the user")
		})
	}
}

func TestWorkspaceErrorsArePermanentAndClear(t *testing.T) {
	for code, want := range map[string]string{
		"workspace_required":     "set the workspace option",
		"invalid_workspace":      "not a valid workspace key",
		"workspace_key_required": "not the workspace id",
		"no_access":              "cannot access the workspace",
		"legacy_token":           "retired workspace token",
	} {
		body, _ := json.Marshal(map[string]any{"error": map[string]string{"code": code, "message": "m"}})
		var calls int
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			if strings.HasSuffix(r.URL.Path, "/workspaces") {
				_, _ = w.Write([]byte(`{"workspaces":[]}`))
				return
			}
			switch code {
			case "workspace_required", "invalid_workspace", "workspace_key_required":
				w.WriteHeader(400)
			case "legacy_token":
				w.WriteHeader(401)
			default:
				w.WriteHeader(403)
			}
			_, _ = w.Write(body)
		}))
		f, err := NewFs(context.Background(), "t", "", testConfig(srv.URL, testWS))
		require.NoError(t, err)
		calls = 0
		_, err = f.(*Fs).About(context.Background())
		require.Error(t, err, code)
		assert.Contains(t, err.Error(), want, code)
		assert.Equal(t, 1, calls, "%s must not be retried", code)
		srv.Close()
	}
}
