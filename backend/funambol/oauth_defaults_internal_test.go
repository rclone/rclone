package funambol

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestO2OAuthDefaultCredentials(t *testing.T) {
	for _, endpoint := range []string{"", defaultEndpoint + "/"} {
		t.Run(endpoint, func(t *testing.T) {
			m := configmap.Simple{"user": smsPhone, "endpoint": endpoint}
			f, err := newSessionFs(context.Background(), "test", m)
			require.NoError(t, err)
			require.True(t, f.opt.usesOAuth(), "ordinary O2 setup must obtain renewable credentials")
			c, err := f.oauthConfig()
			require.NoError(t, err)
			assert.NotEmpty(t, c.ClientID)
			assert.True(t, c.ClientSecret != "", "the bundled application needs its matching secret")
			assert.Equal(t, "macos", f.opt.OAuthPlatform)
			assert.True(t, strings.HasPrefix(m["device_id"], "fol-"))
			assert.Empty(t, m["client_id"])
			assert.Empty(t, m["client_secret"])

			var pending smsPending
			authorize, err := f.oauthStart(&pending)
			require.NoError(t, err)
			u, err := url.Parse(authorize)
			require.NoError(t, err)
			assert.NotEmpty(t, u.Query().Get("client_id"))
			assert.Equal(t, "offline", u.Query().Get("access_type"))
			assert.Equal(t, "S256", u.Query().Get("code_challenge_method"))
			assert.Equal(t, defaultEndpoint+clientOAuthPath, u.Query().Get("redirect_uri"))

			m["token"] = `{"access_token":"access","refresh_token":"refresh"}`
			header, _ := f.oauthHeader()
			assert.Equal(t, "macos", oauthHeaderData(t, header)["platform"])
		})
	}
}

func TestO2OAuthDefaultCredentialOverrides(t *testing.T) {
	for _, secret := range []string{"", "custom-secret"} {
		t.Run("custom-client-secret="+secret, func(t *testing.T) {
			m := configmap.Simple{"client_id": testClient}
			if secret != "" {
				m["client_secret"] = obscure.MustObscure(secret)
			}
			f, err := newSessionFs(context.Background(), "test", m)
			require.NoError(t, err)
			c, err := f.oauthConfig()
			require.NoError(t, err)
			assert.Equal(t, testClient, c.ClientID)
			assert.True(t, c.ClientSecret == secret, "a custom client must not inherit the bundled secret")
			assert.Equal(t, "windows", f.opt.OAuthPlatform)
		})
	}
	f, err := newSessionFs(context.Background(), "test", configmap.Simple{"oauth_platform": "explicit-platform"})
	require.NoError(t, err)
	assert.Equal(t, "explicit-platform", f.opt.OAuthPlatform)
}

func TestO2OAuthRestoresTokenWithoutExplicitClient(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m := configmap.Simple{"token": `{"access_token":"saved-access","refresh_token":"saved-refresh"}`}
	_, err := NewFs(ctx, "test", "", m)
	// Cancellation prevents a live login; reaching it proves that runtime startup
	// uses the saved token instead of requiring another interactive authorization.
	require.ErrorIs(t, err, context.Canceled)
}

func TestO2OAuthDefaultsPreserveOtherLogins(t *testing.T) {
	for name, m := range map[string]configmap.Simple{
		"other tenant": {"endpoint": "https://cloud.example.com"},
		"password":     {"pass": obscure.MustObscure(testPass)},
		"secret only":  {"client_secret": obscure.MustObscure("custom-secret")},
	} {
		t.Run(name, func(t *testing.T) {
			f, err := newSessionFs(context.Background(), "test", m)
			require.NoError(t, err)
			assert.Empty(t, f.opt.ClientID)
			assert.False(t, f.opt.usesOAuth())
			if name == "other tenant" {
				assert.Empty(t, f.opt.ClientSecret)
			}
		})
	}
}

func TestO2OAuthDefaultsStayOnOfficialOAuthEndpoints(t *testing.T) {
	for _, endpoint := range []*string{&o2AuthURL, &o2TokenURL} {
		original := *endpoint
		*endpoint = "https://custom.example/oauth"
		f, err := newSessionFs(context.Background(), "test", configmap.Simple{})
		*endpoint = original
		require.NoError(t, err)
		assert.Empty(t, f.opt.ClientID)
		assert.Empty(t, f.opt.ClientSecret)
	}
}

func TestO2OAuthPlatformDefault(t *testing.T) {
	info, err := fs.Find("funambol")
	require.NoError(t, err)
	for _, option := range info.Options {
		if option.Name == "oauth_platform" {
			assert.Equal(t, "", option.Default, "choose the platform after choosing application credentials")
			return
		}
	}
	t.Fatal("missing oauth_platform option")
}
