package funambol

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testUser = "user@example.com"
	testPass = "secret"
	testKey  = "vk-test"
)

const (
	pathGenerate = "/sapi/login/otp/generate"
	pathLogin    = "/sapi/login"
	pathOTP      = "/sapi/login/otp"
)

// newLoginServer serves the given handlers keyed by path.
func newLoginServer(t *testing.T, handlers map[string]http.HandlerFunc) configmap.Simple {
	mux := http.NewServeMux()
	for path, h := range handlers {
		mux.HandleFunc(path, h)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return configmap.Simple{
		"user":     testUser,
		"pass":     obscure.MustObscure(testPass),
		"endpoint": srv.URL,
	}
}

// requireCredentials fails the request with 401 unless the form carries them.
func requireCredentials(w http.ResponseWriter, r *http.Request) bool {
	if r.FormValue("login") == testUser && r.FormValue("password") == testPass {
		return true
	}
	w.WriteHeader(http.StatusUnauthorized)
	return false
}

// A dropped connection must not make the retry send an empty form.
func TestConfigRetryResendsCredentials(t *testing.T) {
	var calls atomic.Int32
	generate := func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			conn, _, err := w.(http.Hijacker).Hijack()
			require.NoError(t, err)
			_ = conn.Close()
			return
		}

		if requireCredentials(w, r) {
			_, _ = w.Write([]byte(`{}`))
		}
	}
	m := newLoginServer(t, map[string]http.HandlerFunc{pathGenerate: generate})

	out, err := Config(context.Background(), "test", m, fs.ConfigIn{})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, "otp", out.State)
	assert.Equal(t, int32(2), calls.Load())
}

// 204 from generate means no code is needed: log in with the same form.
func TestConfigNoContentLogsIn(t *testing.T) {
	generate := func(w http.ResponseWriter, r *http.Request) {
		if requireCredentials(w, r) {
			w.WriteHeader(http.StatusNoContent)
		}
	}
	login := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "login", r.URL.Query().Get("action"))
		if requireCredentials(w, r) {
			_, _ = w.Write([]byte(`{"data":{"validationkey":"` + testKey + `","roles":[{"name":"standard"}]}}`))
		}
	}
	m := newLoginServer(t, map[string]http.HandlerFunc{pathGenerate: generate, pathLogin: login})

	out, err := Config(context.Background(), "test", m, fs.ConfigIn{})
	require.NoError(t, err)
	assert.Nil(t, out)

	f, err := newSessionFs(context.Background(), "test", m)
	require.NoError(t, err)
	cookies, _ := m.Get("cookies")
	f.restoreCookies(cookies)
	assert.Equal(t, testKey, f.validationKeyFromJar())
}

// MFA-0001 from login means a code was sent after all.
func TestConfigLoginRequiresCode(t *testing.T) {
	generate := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	login := func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"error":{"code":"MFA-0001"},"data":{"validationkey":"` + testKey + `"}}`))
	}
	m := newLoginServer(t, map[string]http.HandlerFunc{pathGenerate: generate, pathLogin: login})

	out, err := Config(context.Background(), "test", m, fs.ConfigIn{})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, "otp", out.State)
}

// Non-JSON error bodies are quoted only up to maxErrorBody.
func TestSapiErrorHandlerTruncates(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusForbidden,
		Status:     "403 Forbidden",
		Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", 10*maxErrorBody))),
	}

	err := sapiErrorHandler(resp)
	require.Error(t, err)
	assert.Less(t, len(err.Error()), 2*maxErrorBody)
}

// A rejected login reports bad credentials, not the HTML error page.
func TestConfigUnauthorized(t *testing.T) {
	generate := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("<!DOCTYPE html><html>" + strings.Repeat("x", 1000) + "</html>"))
	}
	m := newLoginServer(t, map[string]http.HandlerFunc{pathGenerate: generate})

	_, err := Config(context.Background(), "test", m, fs.ConfigIn{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "username or password")
	assert.NotContains(t, err.Error(), "<html")
}

// The code step must mint its own key: MFA-0001's provisional key is not
// enough to finish the login.
func TestConfigCodeNeedsNewKey(t *testing.T) {
	generate := func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"error":{"code":"MFA-0001"},"data":{"validationkey":"` + testKey + `"}}`))
	}
	otp := func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}
	m := newLoginServer(t, map[string]http.HandlerFunc{pathGenerate: generate, pathOTP: otp})

	out, err := Config(context.Background(), "test", m, fs.ConfigIn{})
	require.NoError(t, err)
	require.Equal(t, "otp", out.State)

	out, err = Config(context.Background(), "test", m, fs.ConfigIn{State: "otp", Result: "123456"})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Contains(t, out.Error, "did not complete")
}

// A key set as a cookie completes the login without one in the body.
func TestConfigLoginCookieKey(t *testing.T) {
	generate := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	login := func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "validationKey", Value: testKey, Path: "/"})
		_, _ = w.Write([]byte(`{}`))
	}
	m := newLoginServer(t, map[string]http.HandlerFunc{pathGenerate: generate, pathLogin: login})

	out, err := Config(context.Background(), "test", m, fs.ConfigIn{})
	require.NoError(t, err)
	assert.Nil(t, out)
}

// A non-JSON success (e.g. a bot-protection page) is an error, not a code.
func TestConfigNonJSONReply(t *testing.T) {
	generate := func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>blocked</html>"))
	}
	m := newLoginServer(t, map[string]http.HandlerFunc{pathGenerate: generate})

	_, err := Config(context.Background(), "test", m, fs.ConfigIn{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "blocked")
}

// 403 points at a disabled account or a captcha.
func TestConfigForbidden(t *testing.T) {
	generate := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}
	m := newLoginServer(t, map[string]http.HandlerFunc{pathGenerate: generate})

	_, err := Config(context.Background(), "test", m, fs.ConfigIn{})
	require.ErrorIs(t, err, errCaptcha)
}
