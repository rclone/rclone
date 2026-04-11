package gui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestFreePort(t *testing.T) {
	port, err := freePort()
	assert.NoError(t, err)
	assert.Greater(t, port, 0)
	assert.Less(t, port, 65536)
}

// newTestHandler returns a guiHandler, or skips the test if the embedded
// GUI bundle is not present (i.e. `make fetch-gui` has not been run).
func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	h, err := guiHandler()
	if err != nil {
		t.Skipf("skipping: GUI dist not embedded (run `make fetch-gui`): %v", err)
	}
	return h
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
