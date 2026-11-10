package filen

import (
	"context"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	ctx := context.Background()

	t.Run("AsksFor2FAWithoutAPIKey", func(t *testing.T) {
		m := configmap.Simple{
			"email":    "user@example.com",
			"password": obscure.MustObscure("secret"),
		}
		out, err := Config(ctx, "test", m, fs.ConfigIn{})
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Equal(t, "login", out.State)
		require.NotNil(t, out.Option)
		assert.Equal(t, "config_2fa", out.Option.Name)
		assert.False(t, out.Option.Required)
	})

	t.Run("AsksFor2FAWithBlankedAPIKey", func(t *testing.T) {
		m := configmap.Simple{
			"email":    "user@example.com",
			"password": obscure.MustObscure("secret"),
			"api_key":  obscure.MustObscure(""),
		}
		out, err := Config(ctx, "test", m, fs.ConfigIn{})
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Equal(t, "login", out.State)
	})

	t.Run("DoneWithoutCredentials", func(t *testing.T) {
		for _, m := range []configmap.Simple{
			{},
			{"email": "user@example.com"},
			{"password": obscure.MustObscure("secret")},
		} {
			out, err := Config(ctx, "test", m, fs.ConfigIn{})
			require.NoError(t, err)
			assert.Nil(t, out)
		}
	})

	t.Run("DoneWithAPIKey", func(t *testing.T) {
		m := configmap.Simple{
			"email":    "user@example.com",
			"password": obscure.MustObscure("secret"),
			"api_key":  obscure.MustObscure("key"),
		}
		out, err := Config(ctx, "test", m, fs.ConfigIn{})
		require.NoError(t, err)
		assert.Nil(t, out)
	})

	t.Run("DoneWithInternalPassword", func(t *testing.T) {
		m := configmap.Simple{
			"email":    "user@example.com",
			"password": obscure.MustObscure("INTERNAL"),
		}
		out, err := Config(ctx, "test", m, fs.ConfigIn{})
		require.NoError(t, err)
		assert.Nil(t, out)
	})

	t.Run("UnknownState", func(t *testing.T) {
		_, err := Config(ctx, "test", configmap.Simple{}, fs.ConfigIn{State: "bogus"})
		assert.ErrorContains(t, err, `unknown state "bogus"`)
	})
}
