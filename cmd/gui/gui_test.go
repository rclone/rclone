package gui

import (
	"archive/zip"
	"compress/gzip"
	"io"
	iofs "io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testIndexHTML = `<!doctype html><html><body><div id="root"></div></body></html>`
	testIconSVG   = `<svg xmlns="http://www.w3.org/2000/svg"></svg>`
)

func TestBuildLoginURL(t *testing.T) {
	tests := []struct {
		name   string
		guiURL string
		rcURL  string
		user   string
		pass   string
		noAuth bool
		want   string
	}{
		{
			name:   "with credentials",
			guiURL: "http://localhost:5580/",
			rcURL:  "http://localhost:5572/",
			user:   "gui",
			pass:   "secret",
			noAuth: false,
			want:   "http://localhost:5580/login?pass=secret&url=http%3A%2F%2Flocalhost%3A5572%2F&user=gui",
		},
		{
			name:   "no auth",
			guiURL: "http://localhost:5580/",
			rcURL:  "http://localhost:5572/",
			user:   "",
			pass:   "",
			noAuth: true,
			want:   "http://localhost:5580/",
		},
		{
			name:   "no auth ignores credentials",
			guiURL: "http://localhost:5580/",
			rcURL:  "http://localhost:5572/",
			user:   "gui",
			pass:   "secret",
			noAuth: true,
			want:   "http://localhost:5580/",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildLoginURL(tt.guiURL, tt.rcURL, tt.user, tt.pass, tt.noAuth)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOriginFromURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "with trailing slash",
			url:  "http://localhost:5580/",
			want: "http://localhost:5580",
		},
		{
			name: "with path",
			url:  "http://localhost:5580/some/path",
			want: "http://localhost:5580",
		},
		{
			name: "no trailing slash",
			url:  "http://localhost:5580",
			want: "http://localhost:5580",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := originFromURL(tt.url)
			assert.Equal(t, tt.want, got)
		})
	}
}

// newTestHandler returns a guiHandler backed by the embedded GUI
// bundle, or skips the test if it is not present (i.e. `make fetch-gui`
// has not been run).
func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	srcFS, cleanup, err := guiSourceFS("")
	if err != nil {
		t.Skipf("skipping: GUI dist not embedded (run `make fetch-gui`): %v", err)
	}
	t.Cleanup(func() { _ = cleanup() })
	h, err := guiHandler(srcFS)
	require.NoError(t, err)
	return h
}

// writeTestDir creates a temp directory containing a fake GUI bundle
// (index.html + icon.svg) and returns its path.
func writeTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.html"), []byte(testIndexHTML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "icon.svg"), []byte(testIconSVG), 0644))
	return dir
}

// writeTestZip creates a temp .zip file containing a fake GUI bundle
// and returns its path.
func writeTestZip(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "dist.zip")
	f, err := os.Create(path)
	require.NoError(t, err)
	zw := zip.NewWriter(f)
	for name, content := range map[string]string{
		"index.html": testIndexHTML,
		"icon.svg":   testIconSVG,
	} {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	require.NoError(t, f.Close())
	return path
}

func TestGuiSourceFS(t *testing.T) {
	t.Run("empty path returns embedded", func(t *testing.T) {
		srcFS, cleanup, err := guiSourceFS("")
		if err != nil {
			t.Skipf("skipping: GUI dist not embedded (run `make fetch-gui`): %v", err)
		}
		defer func() { _ = cleanup() }()
		_, err = iofs.Stat(srcFS, "index.html")
		assert.NoError(t, err)
	})

	t.Run("directory path", func(t *testing.T) {
		dir := writeTestDir(t)
		srcFS, cleanup, err := guiSourceFS(dir)
		require.NoError(t, err)
		defer func() { _ = cleanup() }()
		data, err := iofs.ReadFile(srcFS, "index.html")
		require.NoError(t, err)
		assert.Equal(t, testIndexHTML, string(data))
	})

	t.Run("zip path", func(t *testing.T) {
		path := writeTestZip(t)
		srcFS, cleanup, err := guiSourceFS(path)
		require.NoError(t, err)
		defer func() { _ = cleanup() }()
		data, err := iofs.ReadFile(srcFS, "index.html")
		require.NoError(t, err)
		assert.Equal(t, testIndexHTML, string(data))
	})

	t.Run("nonexistent path", func(t *testing.T) {
		_, _, err := guiSourceFS(filepath.Join(t.TempDir(), "nope"))
		assert.Error(t, err)
	})

	t.Run("regular file without zip suffix", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "notazip.txt")
		require.NoError(t, os.WriteFile(path, []byte("hi"), 0644))
		_, _, err := guiSourceFS(path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "directory or a .zip file")
	})
}

// handlerForSource builds a handler from a temp directory or zip
// source. Used by parameterised tests that need to verify the handler
// works against all FS implementations, not just the embedded one.
func handlerForSource(t *testing.T, srcPath string) http.Handler {
	t.Helper()
	srcFS, cleanup, err := guiSourceFS(srcPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cleanup() })
	h, err := guiHandler(srcFS)
	require.NoError(t, err)
	return h
}

func TestHandlerAllSources(t *testing.T) {
	dir := writeTestDir(t)
	zipPath := writeTestZip(t)
	sources := []struct {
		name string
		path string
	}{
		{"directory", dir},
		{"zip", zipPath},
	}
	for _, src := range sources {
		t.Run(src.name, func(t *testing.T) {
			h := handlerForSource(t, src.path)

			// index.html
			req := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Result().StatusCode)
			body, _ := io.ReadAll(w.Result().Body)
			assert.Contains(t, string(body), `<div id="root"></div>`)

			// static asset
			req = httptest.NewRequest("GET", "/icon.svg", nil)
			w = httptest.NewRecorder()
			h.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Result().StatusCode)
			body, _ = io.ReadAll(w.Result().Body)
			assert.Contains(t, string(body), "<svg")

			// SPA fallback
			req = httptest.NewRequest("GET", "/login", nil)
			w = httptest.NewRecorder()
			h.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Result().StatusCode)
			body, _ = io.ReadAll(w.Result().Body)
			assert.Contains(t, string(body), `<div id="root"></div>`,
				"SPA fallback should serve index.html for unknown routes")
		})
	}
}

func TestHandlerServesIndexHTML(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	resp := w.Result()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "<div id=\"root\"></div>")
}

func TestHandlerServesStaticAssets(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest("GET", "/icon.svg", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(body), "<svg"), "expected SVG content")
}

func TestHandlerSPAFallback(t *testing.T) {
	h := newTestHandler(t)

	// /login is not a real file — it should fall back to index.html
	req := httptest.NewRequest("GET", "/login", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	resp := w.Result()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "<div id=\"root\"></div>",
		"SPA fallback should serve index.html for unknown routes")
}

func TestHandlerSPAFallbackDeepPath(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest("GET", "/some/deep/route", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	resp := w.Result()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "<div id=\"root\"></div>")
}

func TestHandlerServesGzip(t *testing.T) {
	dir := writeTestDir(t)
	srcFS, cleanup, err := guiSourceFS(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cleanup() })

	h, err := guiHandler(srcFS)
	require.NoError(t, err)

	// Build a chi router with Compress middleware, mirroring the
	// production setup in the gui command.
	r := chi.NewRouter()
	r.Use(middleware.Compress(5))
	r.Get("/*", h.ServeHTTP)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "gzip", resp.Header.Get("Content-Encoding"),
		"response should be gzip-encoded when client accepts it")

	// Decompress and verify the content is correct.
	gr, err := gzip.NewReader(resp.Body)
	require.NoError(t, err)
	body, err := io.ReadAll(gr)
	require.NoError(t, err)
	require.NoError(t, gr.Close())
	assert.Contains(t, string(body), `<div id="root"></div>`)
}
