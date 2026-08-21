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
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestRun(t *testing.T) {
	opt := Opt
	cmd := "go run proxy_code.go"
	opt.AuthProxy = cmd
	p := New(context.Background(), &opt, &vfscommon.Opt)

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

	t.Run("ClientIP", func(t *testing.T) {
		config, err := p.run(map[string]string{
			"type":      "ftp",
			"user":      "me",
			"pass":      "pass",
			"host":      "127.0.0.1",
			"client_ip": "192.0.2.1",
		})
		require.NoError(t, err)
		assert.Equal(t, configmap.Simple{
			"type":      "ftp",
			"user":      "me-test",
			"pass":      "pass",
			"host":      "127.0.0.1",
			"client_ip": "192.0.2.1",
			"_root":     "",
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
	const testIP = "192.0.2.1"
	const testAddr = testIP + ":1024"
	const otherAddr = "198.51.100.1:1024"

	t.Run("CacheKey", func(t *testing.T) {
		// The source port differs on every connection so it must not
		// affect the cache key, otherwise the proxy would be run for
		// every connection rather than once per client.
		assert.Equal(t,
			generateCacheKey(testUser, testPass, ipFromAddr(testIP+":1024")),
			generateCacheKey(testUser, testPass, ipFromAddr(testIP+":2048")))

		// A different client IP must produce a different key so the
		// proxy is consulted again
		assert.NotEqual(t,
			generateCacheKey(testUser, testPass, ipFromAddr(testAddr)),
			generateCacheKey(testUser, testPass, ipFromAddr(otherAddr)))
	})

	t.Run("call w/Password", func(t *testing.T) {
		// check cache empty
		assert.Equal(t, 0, p.vfsCache.Entries())
		defer p.vfsCache.Clear()

		passwordBytes := []byte(testPass)
		value, err := p.call(testUser, testPass, false, testIP)
		require.NoError(t, err)
		entry, ok := value.(cacheEntry)
		require.True(t, ok)

		// check hash is correct in entry
		assert.Equal(t, entry.pwHash, sha256.Sum256(passwordBytes))
		require.NotNil(t, entry.vfs)
		f := entry.vfs.Fs()
		require.NotNil(t, f)
		cacheKey := generateCacheKey(testUser, testPass, testIP)
		assert.Equal(t, "proxy-"+cacheKey, f.Name())
		assert.True(t, strings.HasPrefix(f.String(), "Local file system"))

		// check it is in the cache
		assert.Equal(t, 1, p.vfsCache.Entries())
		cacheValue, ok := p.vfsCache.GetMaybe(cacheKey)
		assert.True(t, ok)
		assert.Equal(t, value, cacheValue)
	})

	t.Run("Call w/Password", func(t *testing.T) {
		// check cache empty
		assert.Equal(t, 0, p.vfsCache.Entries())
		defer p.vfsCache.Clear()

		cacheKey := generateCacheKey(testUser, testPass, testIP)
		vfs, vfsKey, err := p.Call(testUser, testPass, false, testAddr)
		require.NoError(t, err)
		require.NotNil(t, vfs)
		assert.Equal(t, "proxy-"+cacheKey, vfs.Fs().Name())
		assert.Equal(t, cacheKey, vfsKey)

		// check it is in the cache
		assert.Equal(t, 1, p.vfsCache.Entries())
		cacheValue, ok := p.vfsCache.GetMaybe(cacheKey)
		assert.True(t, ok)
		cached, ok := cacheValue.(cacheEntry)
		assert.True(t, ok)
		assert.Equal(t, vfs, cached.vfs)

		// Test Get works while we have something in the cache
		t.Run("Get", func(t *testing.T) {
			assert.Equal(t, vfs, p.Get(cacheKey))
			assert.Nil(t, p.Get("unknown"))
		})

		// now try again from the cache
		vfs, vfsKey, err = p.Call(testUser, testPass, false, testAddr)
		require.NoError(t, err)
		require.NotNil(t, vfs)
		assert.Equal(t, "proxy-"+cacheKey, vfs.Fs().Name())
		assert.Equal(t, cacheKey, vfsKey)

		// check cache is at the same level
		assert.Equal(t, 1, p.vfsCache.Entries())

		// A different password produces a different cache key, so it
		// creates a fresh cache entry rather than hitting the existing
		// one. Authentication itself is the proxy script's job.
		vfs2, vfsKey2, err := p.Call(testUser, testPass+"different", false, testAddr)
		require.NoError(t, err)
		require.NotNil(t, vfs2)
		assert.NotEqual(t, cacheKey, vfsKey2)
		assert.Equal(t, 2, p.vfsCache.Entries())

		// The underlying fs.Fs must also be a fresh instance from fs/cache
		if vfs.Fs() == vfs2.Fs() {
			t.Error("fs/cache returned the stale backend after auth change")
		}

		// A different client IP also produces a different cache key, so
		// the proxy is consulted again rather than the cached backend
		// being reused - the proxy may be filtering on the IP.
		vfs3, vfsKey3, err := p.Call(testUser, testPass, false, otherAddr)
		require.NoError(t, err)
		require.NotNil(t, vfs3)
		assert.NotEqual(t, cacheKey, vfsKey3)
		assert.Equal(t, 3, p.vfsCache.Entries())

		// If a cached entry's pwHash somehow doesn't match the supplied
		// auth (eg a hash collision on the cache key), Call must reject
		// it. Simulate by corrupting the cached pwHash.
		entry := cacheEntry{vfs: vfs, pwHash: sha256.Sum256([]byte("tampered"))}
		p.vfsCache.Put(cacheKey, entry)
		vfs, vfsKey, err = p.Call(testUser, testPass, false, testAddr)
		require.Error(t, err)
		require.Contains(t, err.Error(), "incorrect password")
		require.Nil(t, vfs)
		require.Equal(t, "", vfsKey)
	})

	t.Run("Call w/o Address", func(t *testing.T) {
		// A client with no address, eg on a unix socket, must still
		// authenticate
		assert.Equal(t, 0, p.vfsCache.Entries())
		defer p.vfsCache.Clear()

		vfs, vfsKey, err := p.Call(testUser, testPass, false, "")
		require.NoError(t, err)
		require.NotNil(t, vfs)
		assert.Equal(t, generateCacheKey(testUser, testPass, ""), vfsKey)
		assert.Equal(t, 1, p.vfsCache.Entries())
	})

	privateKey, privateKeyErr := rsa.GenerateKey(rand.Reader, 2048)
	if privateKeyErr != nil {
		fs.Fatal(nil, "error generating test private key "+privateKeyErr.Error())
	}
	publicKey, publicKeyError := ssh.NewPublicKey(&privateKey.PublicKey)
	if publicKeyError != nil {
		fs.Fatal(nil, "error generating test public key "+publicKeyError.Error())
	}

	publicKeyString := base64.StdEncoding.EncodeToString(publicKey.Marshal())

	t.Run("Call w/PublicKey", func(t *testing.T) {
		// check cache empty
		assert.Equal(t, 0, p.vfsCache.Entries())
		defer p.vfsCache.Clear()

		value, err := p.call(testUser, publicKeyString, true, testIP)
		require.NoError(t, err)
		entry, ok := value.(cacheEntry)
		require.True(t, ok)

		// check publicKey is correct in entry
		require.NoError(t, err)
		require.NotNil(t, entry.vfs)
		f := entry.vfs.Fs()
		require.NotNil(t, f)
		cacheKey := generateCacheKey(testUser, publicKeyString, testIP)
		assert.Equal(t, "proxy-"+cacheKey, f.Name())
		assert.True(t, strings.HasPrefix(f.String(), "Local file system"))

		// check it is in the cache
		assert.Equal(t, 1, p.vfsCache.Entries())
		cacheValue, ok := p.vfsCache.GetMaybe(cacheKey)
		assert.True(t, ok)
		assert.Equal(t, value, cacheValue)
	})

	t.Run("call w/PublicKey", func(t *testing.T) {
		// check cache empty
		assert.Equal(t, 0, p.vfsCache.Entries())
		defer p.vfsCache.Clear()

		cacheKey := generateCacheKey(testUser, publicKeyString, testIP)
		vfs, vfsKey, err := p.Call(
			testUser,
			publicKeyString,
			true,
			testAddr,
		)
		require.NoError(t, err)
		require.NotNil(t, vfs)
		assert.Equal(t, "proxy-"+cacheKey, vfs.Fs().Name())
		assert.Equal(t, cacheKey, vfsKey)

		// check it is in the cache
		assert.Equal(t, 1, p.vfsCache.Entries())
		cacheValue, ok := p.vfsCache.GetMaybe(cacheKey)
		assert.True(t, ok)
		cached, ok := cacheValue.(cacheEntry)
		assert.True(t, ok)
		assert.Equal(t, vfs, cached.vfs)

		// Test Get works while we have something in the cache
		t.Run("Get", func(t *testing.T) {
			assert.Equal(t, vfs, p.Get(cacheKey))
			assert.Nil(t, p.Get("unknown"))
		})

		// now try again from the cache
		vfs, vfsKey, err = p.Call(testUser, publicKeyString, true, testAddr)
		require.NoError(t, err)
		require.NotNil(t, vfs)
		assert.Equal(t, "proxy-"+cacheKey, vfs.Fs().Name())
		assert.Equal(t, cacheKey, vfsKey)

		// check cache is at the same level
		assert.Equal(t, 1, p.vfsCache.Entries())

		// A different public key produces a different cache key, so it
		// creates a fresh cache entry rather than hitting the existing
		// one. Authentication itself is the proxy script's job.
		vfs2, vfsKey2, err := p.Call(testUser, publicKeyString+"different", true, testAddr)
		require.NoError(t, err)
		require.NotNil(t, vfs2)
		assert.NotEqual(t, cacheKey, vfsKey2)
		assert.Equal(t, 2, p.vfsCache.Entries())

		// The underlying fs.Fs must be a fresh instance from fs/cache
		if vfs.Fs() == vfs2.Fs() {
			t.Error("fs/cache returned the stale backend after public key change")
		}

		// If a cached entry's pwHash somehow doesn't match the supplied
		// auth (eg a hash collision on the cache key), Call must reject
		// it. Simulate by corrupting the cached pwHash.
		entry := cacheEntry{vfs: vfs, pwHash: sha256.Sum256([]byte("tampered"))}
		p.vfsCache.Put(cacheKey, entry)
		vfs, vfsKey, err = p.Call(testUser, publicKeyString, true, testAddr)
		require.Error(t, err)
		require.Contains(t, err.Error(), "incorrect public key")
		require.Nil(t, vfs)
		require.Equal(t, "", vfsKey)
	})
}

func TestIPFromAddr(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
	}{
		{"192.0.2.1:1024", "192.0.2.1"},
		{"[2001:db8::1]:1024", "2001:db8::1"},
		{"[::ffff:192.0.2.1]:1024", "192.0.2.1"},
		{"[fe80::1%eth0]:1024", "fe80::1%eth0"},
		{"/tmp/rclone.sock", ""},
		{"/tmp/foo:bar.sock", ""},
		{`C:\Users\me\rclone.sock`, ""},
		{"@", ""},
		{"<nil>", ""},
		{"", ""},
	} {
		assert.Equal(t, test.want, ipFromAddr(test.in), test.in)
	}
}
