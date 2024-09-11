package proxy

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestRun(t *testing.T) {
	opt := DefaultOpt
	cmd := "go run proxy_code.go"
	opt.AuthProxy = cmd
	p := New(context.Background(), &opt)

	t.Run("Normal", func(t *testing.T) {
		config, err := p.run(map[string]string{
			"type": "ftp",
			"user": "me",
			"pass": "pass",
			"host": "127.0.0.1",
		})
		require.NoError(t, err)
		assert.Equal(t, configmap.Simple{
			"type":  "ftp",
			"user":  "me-test",
			"pass":  "pass",
			"host":  "127.0.0.1",
			"_root": "",
		}, config)
	})

	t.Run("Error", func(t *testing.T) {
		config, err := p.run(map[string]string{
			"error": "potato",
		})
		assert.Nil(t, config)
		require.Error(t, err)
		require.Contains(t, err.Error(), "potato")
	})

	t.Run("Obscure", func(t *testing.T) {
		config, err := p.run(map[string]string{
			"type":     "ftp",
			"user":     "me",
			"pass":     "pass",
			"host":     "127.0.0.1",
			"_obscure": "pass,user",
		})
		require.NoError(t, err)
		config["user"] = obscure.MustReveal(config["user"])
		config["pass"] = obscure.MustReveal(config["pass"])
		assert.Equal(t, configmap.Simple{
			"type":     "ftp",
			"user":     "me-test",
			"pass":     "pass",
			"host":     "127.0.0.1",
			"_obscure": "pass,user",
			"_root":    "",
		}, config)
	})

	const testUser = "testUser"
	const testPass = "testPass"

	t.Run("call w/Password", func(t *testing.T) {
		// check cache empty
		assert.Equal(t, 0, p.vfsCache.Entries())
		defer p.vfsCache.Clear()

		passwordBytes := []byte(testPass)
		value, err := p.call(testUser, testPass, false)
		require.NoError(t, err)
		entry, ok := value.(cacheEntry)
		require.True(t, ok)

		// check hash is correct in entry
		assert.Equal(t, entry.pwHash, sha256.Sum256(passwordBytes))
		require.NotNil(t, entry.vfs)
		f := entry.vfs.Fs()
		require.NotNil(t, f)
		assert.Equal(t, "proxy-"+testUser, f.Name())
		assert.True(t, strings.HasPrefix(f.String(), "Local file system"))

		// check it is in the cache
		assert.Equal(t, 1, p.vfsCache.Entries())
		cacheValue, ok := p.vfsCache.GetMaybe(testUser)
		assert.True(t, ok)
		assert.Equal(t, value, cacheValue)
	})

	t.Run("Call w/Password", func(t *testing.T) {
		// check cache empty
		assert.Equal(t, 0, p.vfsCache.Entries())
		defer p.vfsCache.Clear()

		vfs, vfsKey, err := p.Call(testUser, testPass, false)
		require.NoError(t, err)
		require.NotNil(t, vfs)
		assert.Equal(t, "proxy-"+testUser, vfs.Fs().Name())
		assert.Equal(t, testUser, vfsKey)

		// check it is in the cache
		assert.Equal(t, 1, p.vfsCache.Entries())
		cacheValue, ok := p.vfsCache.GetMaybe(testUser)
		assert.True(t, ok)
		cacheEntry, ok := cacheValue.(cacheEntry)
		assert.True(t, ok)
		assert.Equal(t, vfs, cacheEntry.vfs)

		// Test Get works while we have something in the cache
		t.Run("Get", func(t *testing.T) {
			assert.Equal(t, vfs, p.Get(testUser))
			assert.Nil(t, p.Get("unknown"))
		})

		// now try again from the cache
		vfs, vfsKey, err = p.Call(testUser, testPass, false)
		require.NoError(t, err)
		require.NotNil(t, vfs)
		assert.Equal(t, "proxy-"+testUser, vfs.Fs().Name())
		assert.Equal(t, testUser, vfsKey)

		// check cache is at the same level
		assert.Equal(t, 1, p.vfsCache.Entries())

		// now try again from the cache but with wrong password
		vfs, vfsKey, err = p.Call(testUser, testPass+"wrong", false)
		require.Error(t, err)
		require.Contains(t, err.Error(), "incorrect password")
		require.Nil(t, vfs)
		require.Equal(t, "", vfsKey)

		// check cache is at the same level
		assert.Equal(t, 1, p.vfsCache.Entries())

	})

	privateKey, privateKeyErr := rsa.GenerateKey(rand.Reader, 2048)
	if privateKeyErr != nil {
		fs.Fatal(nil, "error generating test private key "+privateKeyErr.Error())
	}
	publicKey, publicKeyError := ssh.NewPublicKey(&privateKey.PublicKey)
	if privateKeyErr != nil {
		fs.Fatal(nil, "error generating test public key "+publicKeyError.Error())
	}

	publicKeyString := base64.StdEncoding.EncodeToString(publicKey.Marshal())

	t.Run("Call w/PublicKey", func(t *testing.T) {
		// check cache empty
		assert.Equal(t, 0, p.vfsCache.Entries())
		defer p.vfsCache.Clear()

		value, err := p.call(testUser, publicKeyString, true)
		require.NoError(t, err)
		entry, ok := value.(cacheEntry)
		require.True(t, ok)

		// check publicKey is correct in entry
		require.NoError(t, err)
		require.NotNil(t, entry.vfs)
		f := entry.vfs.Fs()
		require.NotNil(t, f)
		assert.Equal(t, "proxy-"+testUser, f.Name())
		assert.True(t, strings.HasPrefix(f.String(), "Local file system"))

		// check it is in the cache
		assert.Equal(t, 1, p.vfsCache.Entries())
		cacheValue, ok := p.vfsCache.GetMaybe(testUser)
		assert.True(t, ok)
		assert.Equal(t, value, cacheValue)
	})

	t.Run("call w/PublicKey", func(t *testing.T) {
		// check cache empty
		assert.Equal(t, 0, p.vfsCache.Entries())
		defer p.vfsCache.Clear()

		vfs, vfsKey, err := p.Call(
			testUser,
			publicKeyString,
			true,
		)
		require.NoError(t, err)
		require.NotNil(t, vfs)
		assert.Equal(t, "proxy-"+testUser, vfs.Fs().Name())
		assert.Equal(t, testUser, vfsKey)

		// check it is in the cache
		assert.Equal(t, 1, p.vfsCache.Entries())
		cacheValue, ok := p.vfsCache.GetMaybe(testUser)
		assert.True(t, ok)
		cacheEntry, ok := cacheValue.(cacheEntry)
		assert.True(t, ok)
		assert.Equal(t, vfs, cacheEntry.vfs)

		// Test Get works while we have something in the cache
		t.Run("Get", func(t *testing.T) {
			assert.Equal(t, vfs, p.Get(testUser))
			assert.Nil(t, p.Get("unknown"))
		})

		// now try again from the cache
		vfs, vfsKey, err = p.Call(testUser, publicKeyString, true)
		require.NoError(t, err)
		require.NotNil(t, vfs)
		assert.Equal(t, "proxy-"+testUser, vfs.Fs().Name())
		assert.Equal(t, testUser, vfsKey)

		// check cache is at the same level
		assert.Equal(t, 1, p.vfsCache.Entries())

		// now try again from the cache but with wrong public key
		vfs, vfsKey, err = p.Call(testUser, publicKeyString+"wrong", true)
		require.Error(t, err)
		require.Contains(t, err.Error(), "incorrect public key")
		require.Nil(t, vfs)
		require.Equal(t, "", vfsKey)

		// check cache is at the same level
		assert.Equal(t, 1, p.vfsCache.Entries())
	})
}
