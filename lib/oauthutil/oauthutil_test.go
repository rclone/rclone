package oauthutil

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuthServerHandleAuth checks that only a callback carrying the
// random state may complete the flow, since the callback is an
// unauthenticated request any web page in the user's browser can make.
func TestAuthServerHandleAuth(t *testing.T) {
	for _, test := range []struct {
		name       string
		query      string
		wantStatus int
		wantOK     bool
	}{
		{"CorrectState", "?code=CODE&state=correct-state", http.StatusOK, true},
		{"WrongState", "?code=CODE&state=wrong-state", http.StatusBadRequest, false},
		{"BlankState", "?code=CODE&state=", http.StatusBadRequest, false},
		{"MissingState", "?code=CODE", http.StatusBadRequest, false},
		{"MissingCode", "?state=correct-state", http.StatusBadRequest, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := newAuthServer(&Options{}, "", "correct-state", "")
			w := httptest.NewRecorder()
			s.handleAuth(w, httptest.NewRequest("GET", "/"+test.query, nil))
			assert.Equal(t, test.wantStatus, w.Code)
			result := <-s.result
			assert.Equal(t, test.wantOK, result.OK)
			if test.wantOK {
				assert.Equal(t, "CODE", result.Code)
				assert.Equal(t, "CODE", result.Form.Get("code"))
			} else {
				assert.Equal(t, "", result.Code)
			}
		})
	}
}

// TestConfigOauthPKCE verifies that the PKCE verifier generated with
// the authorization URL on the remote setup path (title bar redirect)
// is used when the pasted verification code is exchanged for a token
func TestConfigOauthPKCE(t *testing.T) {
	for _, usePKCE := range []bool{true, false} {
		t.Run(fmt.Sprintf("usePKCE=%v", usePKCE), func(t *testing.T) {
			// Fake token server which captures the code_verifier of the exchange
			var mu sync.Mutex
			var codeVerifier string
			tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.ParseForm()
				mu.Lock()
				codeVerifier = r.PostFormValue("code_verifier")
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, `{"access_token":"access_token","token_type":"Bearer"}`)
			}))
			defer tokenServer.Close()

			testConfig := &Config{
				ClientID:     "client_id",
				ClientSecret: "client_secret",
				AuthURL:      tokenServer.URL + "/auth",
				TokenURL:     tokenServer.URL + "/token",
				RedirectURL:  TitleBarRedirectURL,
				UsePKCE:      usePKCE,
			}
			ri := &fs.RegInfo{
				Name: "testpkce",
				Config: func(ctx context.Context, name string, m configmap.Mapper, in fs.ConfigIn) (*fs.ConfigOut, error) {
					return ConfigOut("", &Options{OAuth2Config: testConfig})
				},
			}
			m := configmap.Simple{}
			ctx := context.Background()

			// Start the oauth flow as if the backend had returned OAuth
			// options from a state "oauth" with an empty result
			in := fs.ConfigIn{State: fs.StatePush("", "*oauth", "", "oauth", "")}

			// *oauth: no token yet so goto *oauth-confirm
			out, err := ConfigOAuth(ctx, "remote", m, ri, in)
			require.NoError(t, err)
			require.True(t, strings.HasPrefix(out.State, "*oauth-confirm"), out.State)

			// *oauth-confirm: replace the token -> *oauth-islocal
			out, err = ConfigOAuth(ctx, "remote", m, ri, fs.ConfigIn{State: out.State, Result: "true"})
			require.NoError(t, err)
			require.True(t, strings.HasPrefix(out.State, "*oauth-islocal"), out.State)

			// *oauth-islocal: no browser on this machine -> *oauth-remote
			out, err = ConfigOAuth(ctx, "remote", m, ri, fs.ConfigIn{State: out.State, Result: "false"})
			require.NoError(t, err)
			require.True(t, strings.HasPrefix(out.State, "*oauth-remote"), out.State)

			// *oauth-remote: title bar redirect so no webserver is
			// needed: show the authorization URL and ask for the code
			out, err = ConfigOAuth(ctx, "remote", m, ri, fs.ConfigIn{State: out.State})
			require.NoError(t, err)
			require.True(t, strings.HasPrefix(out.State, "*oauth-do"), out.State)
			require.NotNil(t, out.Option)
			authURL := regexp.MustCompile(`https?://\S+`).FindString(out.Option.Help)
			require.NotEmpty(t, authURL)
			u, err := url.Parse(authURL)
			require.NoError(t, err)
			challenge := u.Query().Get("code_challenge")
			if usePKCE {
				require.Equal(t, "S256", u.Query().Get("code_challenge_method"))
				require.NotEmpty(t, challenge)
			}

			// *oauth-do: exchange the pasted code for a token
			out, err = ConfigOAuth(ctx, "remote", m, ri, fs.ConfigIn{State: out.State, Result: "verification-code"})
			require.NoError(t, err)
			require.True(t, strings.HasPrefix(out.State, "*oauth-done"), out.State)

			// The token must have been stored
			token, ok := m.Get("token")
			require.True(t, ok)
			require.Contains(t, token, "access_token")

			// The exchange must have been made with the verifier which
			// matches the challenge sent in the authorization URL
			mu.Lock()
			defer mu.Unlock()
			if usePKCE {
				sum := sha256.Sum256([]byte(codeVerifier))
				assert.Equal(t, challenge, base64.RawURLEncoding.EncodeToString(sum[:]))
			} else {
				assert.Empty(t, codeVerifier)
			}
		})
	}
}
