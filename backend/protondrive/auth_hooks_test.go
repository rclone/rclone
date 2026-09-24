package protondrive

import (
	"testing"

	"github.com/rclone/go-proton-api"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func storedCredentials(uid, accessToken, refreshToken, saltedKeyPass string) configmap.Simple {
	m := configmap.Simple{}
	setConfigMap(m, uid, accessToken, refreshToken, saltedKeyPass)
	return m
}

func TestConfigAuthHooks_AuthHandlerPersistsRotatedTokens(t *testing.T) {
	m := storedCredentials("uid", "acc1", "ref1", "salt")
	authHandler, _, _ := configAuthHooks(m)

	authHandler(proton.Auth{UID: "uid", AccessToken: "acc2", RefreshToken: "ref2"})

	uid, acc, ref, salt, ok := getConfigMap(m)
	require.True(t, ok)
	assert.Equal(t, "uid", uid)
	assert.Equal(t, "acc2", acc)
	assert.Equal(t, "ref2", ref)
	assert.Equal(t, "salt", salt, "salted key pass must survive a token rotation")
}

func TestConfigAuthHooks_DeAuthHandlerClearsCredentials(t *testing.T) {
	m := storedCredentials("uid", "acc1", "ref1", "salt")
	_, deAuthHandler, _ := configAuthHooks(m)

	deAuthHandler()

	_, _, _, _, ok := getConfigMap(m)
	assert.False(t, ok)
}

func TestConfigAuthHooks_RefreshHookReturnsStoredCredentials(t *testing.T) {
	m := storedCredentials("uid", "acc1", "ref1", "salt")
	_, _, refreshHook := configAuthHooks(m)

	uid, acc, ref, ok := refreshHook()
	require.True(t, ok)
	assert.Equal(t, "uid", uid)
	assert.Equal(t, "acc1", acc)
	assert.Equal(t, "ref1", ref)
}

func TestConfigAuthHooks_RefreshHookRejectsIncompleteCredentials(t *testing.T) {
	m := storedCredentials("uid", "acc1", "", "salt")
	_, _, refreshHook := configAuthHooks(m)

	_, _, _, ok := refreshHook()
	assert.False(t, ok)
}

// Two Fs on the same remote each get their own hooks over the same config.
// When one persists a rotated token the other's refresh hook must see it.
func TestConfigAuthHooks_RefreshHookSeesRotationByAnotherFs(t *testing.T) {
	m := storedCredentials("uid", "acc1", "ref1", "salt")
	authHandlerA, _, _ := configAuthHooks(m)
	_, _, refreshHookB := configAuthHooks(m)

	authHandlerA(proton.Auth{UID: "uid", AccessToken: "acc2", RefreshToken: "ref2"})

	_, acc, ref, ok := refreshHookB()
	require.True(t, ok)
	assert.Equal(t, "acc2", acc)
	assert.Equal(t, "ref2", ref)
}
