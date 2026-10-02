package febbox

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/febbox/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
)

func TestListAuthenticatesAndPaginates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if got := r.Form.Get("grant_type"); got != "client_credentials" {
				t.Errorf("grant_type = %q", got)
			}
			if !strings.Contains(r.UserAgent(), "Mozilla/5.0") {
				t.Errorf("user agent = %q", r.UserAgent())
			}
			_, _ = io.WriteString(w, `{"code":1,"msg":"success","data":{"access_token":"token","expires_in":3600,"token_type":"Bearer","refresh_token":"refresh"}}`)
		case "/oauth":
			if got := r.Header.Get("Authorization"); got != "Bearer token" {
				t.Errorf("authorization = %q", got)
			}
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatal(err)
			}
			page := r.FormValue("page")
			if page == "1" {
				_, _ = io.WriteString(w, `{"code":1,"msg":"success","data":{"file_list":[{"fid":10,"file_name":"dir","file_size":0,"parent_id":0,"update_time":1700000000,"is_dir":1},{"fid":11,"file_name":"first.txt","file_size":5,"parent_id":0,"update_time":1700000001,"is_dir":0,"ext":"txt"}]}}`)
				return
			}
			_, _ = io.WriteString(w, `{"code":1,"msg":"success","data":{"file_list":[{"fid":12,"file_name":"second.txt","file_size":6,"parent_id":0,"update_time":1700000002,"is_dir":0,"ext":"txt"}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	f, err := NewFs(context.Background(), "test", "", configmap.Simple{
		"client_id":     "client",
		"client_secret": "secret",
		"endpoint":      server.URL + "/oauth",
		"list_chunk":    "2",
	})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := f.List(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(entries), 3; got != want {
		t.Fatalf("entries = %d, want %d", got, want)
	}
	if got := entries[0].Remote(); got != "dir" {
		t.Errorf("first remote = %q", got)
	}
}

func TestNewFsAllowsMissingRoot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			_, _ = io.WriteString(w, `{"code":1,"msg":"success","data":{"access_token":"token","expires_in":3600,"token_type":"Bearer"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"code":1,"msg":"success","data":{"file_list":[]}}`)
	}))
	defer server.Close()

	f, err := NewFs(context.Background(), "test", "missing", configmap.Simple{
		"client_id":     "client",
		"client_secret": "secret",
		"endpoint":      server.URL + "/oauth",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := f.Root(); got != "missing" {
		t.Errorf("root = %q", got)
	}
}

func TestMutationsAndAddURLUseOfficialModules(t *testing.T) {
	var mu sync.Mutex
	var calls []url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			_, _ = io.WriteString(w, `{"code":1,"msg":"success","data":{"access_token":"token","expires_in":3600,"token_type":"Bearer"}}`)
			return
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		form := r.MultipartForm.Value
		module := r.FormValue("module")
		if module != "file_list" {
			mu.Lock()
			calls = append(calls, form)
			mu.Unlock()
		}
		switch module {
		case "file_list":
			_, _ = io.WriteString(w, `{"code":1,"msg":"success","data":{"file_list":[{"fid":9,"file_name":"uploads","parent_id":0,"update_time":1700000000,"is_dir":1}]}}`)
		case "create_dir":
			_, _ = io.WriteString(w, `{"code":1,"msg":"create dir success","data":{"fid":"20","pid":"0","file_name":"dir"}}`)
		case "file_delete":
			_, _ = io.WriteString(w, `{"code":1,"msg":"delete file success","data":null}`)
		case "file_add":
			_, _ = io.WriteString(w, `{"code":1,"msg":"success","data":{"task_id":"30"}}`)
		default:
			_, _ = io.WriteString(w, `{"code":-10003,"msg":"module not found","data":null}`)
		}
	}))
	defer server.Close()

	fsValue, err := NewFs(context.Background(), "test", "uploads", configmap.Simple{
		"client_id":     "client",
		"client_secret": "secret",
		"endpoint":      server.URL + "/oauth",
	})
	if err != nil {
		t.Fatal(err)
	}
	f := fsValue.(*Fs)
	id, err := f.CreateDir(context.Background(), "0", "dir")
	if err != nil {
		t.Fatal(err)
	}
	if id != "20" {
		t.Errorf("directory ID = %q", id)
	}
	o := &Object{fs: f, id: "11", remote: "file.txt"}
	if err := o.Remove(context.Background()); err != nil {
		t.Fatal(err)
	}
	result, err := f.Command(context.Background(), "addurl", []string{"https://example.com/file.txt"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.(*api.AddURL).TaskID; got != "30" {
		t.Errorf("task ID = %q", got)
	}
	if _, err := f.Command(context.Background(), "addurl", []string{"file:///tmp/file"}, nil); err == nil {
		t.Fatal("invalid URL was accepted")
	}

	mu.Lock()
	defer mu.Unlock()
	if got, want := len(calls), 3; got != want {
		t.Fatalf("API calls = %d, want %d", got, want)
	}
	if got := calls[0].Get("module"); got != "create_dir" {
		t.Errorf("create module = %q", got)
	}
	if got := calls[1].Get("module"); got != "file_delete" {
		t.Errorf("delete module = %q", got)
	}
	if got := calls[2].Get("path"); got != "/uploads" {
		t.Errorf("add path = %q", got)
	}
}

func TestRmdirDoesNotDeleteAccountRoot(t *testing.T) {
	deleteCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			_, _ = io.WriteString(w, `{"code":1,"msg":"success","data":{"access_token":"token","expires_in":3600,"token_type":"Bearer"}}`)
			return
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("module") == "file_delete" {
			deleteCalls++
		}
		_, _ = io.WriteString(w, `{"code":1,"msg":"success","data":{"file_list":[]}}`)
	}))
	defer server.Close()

	fsValue, err := NewFs(context.Background(), "test", "", configmap.Simple{
		"client_id":     "client",
		"client_secret": "secret",
		"endpoint":      server.URL + "/oauth",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = fsValue.Rmdir(context.Background(), ""); err == nil {
		t.Fatal("account root removal was accepted")
	}
	if deleteCalls != 0 {
		t.Errorf("delete calls = %d", deleteCalls)
	}
}

func TestCreateDirDoesNotRetry(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			_, _ = io.WriteString(w, `{"code":1,"msg":"success","data":{"access_token":"token","expires_in":3600,"token_type":"Bearer"}}`)
			return
		}
		calls++
		http.Error(w, "temporary failure", http.StatusInternalServerError)
	}))
	defer server.Close()

	fsValue, err := NewFs(context.Background(), "test", "", configmap.Simple{
		"client_id":     "client",
		"client_secret": "secret",
		"endpoint":      server.URL + "/oauth",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()
	_, _ = fsValue.(*Fs).CreateDir(ctx, "0", "dir")
	if calls != 1 {
		t.Errorf("create calls = %d, want 1", calls)
	}
}

func TestServerSideMoveAndCopy(t *testing.T) {
	var mu sync.Mutex
	var modules []string
	listCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			_, _ = io.WriteString(w, `{"code":1,"msg":"success","data":{"access_token":"token","expires_in":3600,"token_type":"Bearer"}}`)
			return
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		module := r.FormValue("module")
		mu.Lock()
		modules = append(modules, module)
		mu.Unlock()
		switch module {
		case "file_move":
			_, _ = io.WriteString(w, `{"code":1,"msg":"move success","data":null}`)
		case "file_copy":
			_, _ = io.WriteString(w, `{"code":1,"msg":"copy success","data":null}`)
		case "file_rename":
			_, _ = io.WriteString(w, `{"code":1,"msg":"file rename success","data":null}`)
		case "file_list":
			listCalls++
			if listCalls == 1 {
				_, _ = io.WriteString(w, `{"code":1,"msg":"success","data":{"file_list":[]}}`)
				return
			}
			_, _ = io.WriteString(w, `{"code":1,"msg":"success","data":{"file_list":[{"fid":31,"file_name":"source.txt","file_size":4,"parent_id":20,"update_time":1700000000,"is_dir":0,"ext":"txt"}]}}`)
		default:
			_, _ = io.WriteString(w, `{"code":-10003,"msg":"module not found","data":null}`)
		}
	}))
	defer server.Close()

	fsValue, err := NewFs(context.Background(), "test", "", configmap.Simple{
		"client_id":     "client",
		"client_secret": "secret",
		"endpoint":      server.URL + "/oauth",
	})
	if err != nil {
		t.Fatal(err)
	}
	f := fsValue.(*Fs)
	f.dirCache.Put("dst", "20")
	sourceFs := *f
	source := &Object{fs: &sourceFs, remote: "source.txt", id: "11", parentID: "0", size: 4}

	moved, err := f.Move(context.Background(), source, "dst/moved.txt")
	if err != nil {
		t.Fatal(err)
	}
	if moved.Remote() != "dst/moved.txt" || moved.(*Object).id != "11" {
		t.Errorf("moved object = %#v", moved)
	}
	copied, err := f.Copy(context.Background(), source, "dst/copied.txt")
	if err != nil {
		t.Fatal(err)
	}
	if copied.Remote() != "dst/copied.txt" || copied.(*Object).id != "31" {
		t.Errorf("copied object = %#v", copied)
	}

	mu.Lock()
	defer mu.Unlock()
	if got, want := strings.Join(modules, ","), "file_move,file_rename,file_list,file_copy,file_list,file_rename"; got != want {
		t.Errorf("modules = %q, want %q", got, want)
	}
}

func TestOpenMapsPremiumRestrictionAndForwardsRange(t *testing.T) {
	tests := []struct {
		name      string
		apiResult string
		wantErr   error
		wantBody  string
		wantRange string
	}{
		{
			name:      "free account",
			apiResult: `{"code":-403,"msg":"module disabled: file_get_download_url","data":null}`,
			wantErr:   fs.ErrorPermissionDenied,
		},
		{
			name:      "premium account",
			apiResult: `{"code":1,"msg":"success","data":[{"fid":11,"download_url":"DOWNLOAD_URL"}]}`,
			wantBody:  "data",
			wantRange: "bytes=1-2",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for name, values := range r.Header {
					for _, value := range values {
						if strings.Contains(value, "test-access-token") || strings.Contains(value, "test-client-secret") {
							t.Errorf("download leaked credentials in header %q", name)
						}
					}
				}
				if strings.Contains(r.URL.String(), "test-access-token") || strings.Contains(r.URL.String(), "test-client-secret") {
					t.Errorf("download leaked credentials in URL %q", r.URL)
				}
				if got := r.Header.Get("Range"); got != test.wantRange {
					t.Errorf("range = %q, want %q", got, test.wantRange)
				}
				w.WriteHeader(http.StatusPartialContent)
				_, _ = io.WriteString(w, "data")
			}))
			defer downloadServer.Close()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/oauth/token":
					_, _ = io.WriteString(w, `{"code":1,"msg":"success","data":{"access_token":"test-access-token","expires_in":3600,"token_type":"Bearer"}}`)
				case "/oauth":
					result := strings.ReplaceAll(test.apiResult, "DOWNLOAD_URL", downloadServer.URL+"/download")
					_, _ = io.WriteString(w, result)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			f, err := NewFs(context.Background(), "test", "", configmap.Simple{
				"client_id":     "client",
				"client_secret": "test-client-secret",
				"endpoint":      server.URL + "/oauth",
			})
			if err != nil {
				t.Fatal(err)
			}
			o := &Object{fs: f.(*Fs), remote: "file.txt", id: "11", size: 4}
			rc, err := o.Open(context.Background(), &fs.RangeOption{Start: 1, End: 2})
			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("error = %v, want %v", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := rc.Close(); err != nil {
					t.Errorf("close download: %v", err)
				}
			}()
			body, err := io.ReadAll(rc)
			if err != nil {
				t.Fatal(err)
			}
			if got := string(body); got != test.wantBody {
				t.Errorf("body = %q, want %q", got, test.wantBody)
			}
		})
	}
}
