package proton_test

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/bradenaw/juniper/parallel"
	"github.com/rclone/go-proton-api"
	"github.com/rclone/go-proton-api/server"
	"github.com/stretchr/testify/require"
)

func TestAuth(t *testing.T) {
	s := server.New()
	defer s.Close()

	_, _, err := s.CreateUser("user", []byte("pass"))
	require.NoError(t, err)

	m := proton.New(
		proton.WithHostURL(s.GetHostURL()),
		proton.WithTransport(proton.InsecureTransport()),
	)
	defer m.Close()

	// Create one session.
	c1, auth1, err := m.NewClientWithLogin(context.Background(), "user", []byte("pass"))
	require.NoError(t, err)

	// Revoke all other sessions.
	require.NoError(t, c1.AuthRevokeAll(context.Background()))

	// Create another session.
	c2, _, err := m.NewClientWithLogin(context.Background(), "user", []byte("pass"))
	require.NoError(t, err)

	// There should be two sessions.
	sessions, err := c1.AuthSessions(context.Background())
	require.NoError(t, err)
	require.Len(t, sessions, 2)

	// Revoke the first session.
	require.NoError(t, c2.AuthRevoke(context.Background(), auth1.UID))

	// The first session should no longer work.
	require.Error(t, c1.AuthDelete(context.Background()))

	// There should be one session remaining.
	remaining, err := c2.AuthSessions(context.Background())
	require.NoError(t, err)
	require.Len(t, remaining, 1)

	// Delete the last session.
	require.NoError(t, c2.AuthDelete(context.Background()))
}

func TestAuth_Refresh(t *testing.T) {
	s := server.New()
	defer s.Close()

	// Create a user on the server.
	userID, _, err := s.CreateUser("user", []byte("pass"))
	require.NoError(t, err)

	// The auth is valid for 4 seconds.
	s.SetAuthLife(4 * time.Second)

	m := proton.New(
		proton.WithHostURL(s.GetHostURL()),
		proton.WithTransport(proton.InsecureTransport()),
	)
	defer m.Close()

	// Create one session for the user.
	c, auth, err := m.NewClientWithLogin(context.Background(), "user", []byte("pass"))
	require.NoError(t, err)
	require.Equal(t, userID, auth.UserID)

	// Wait for 2 seconds.
	time.Sleep(2 * time.Second)

	// The client should still be authenticated.
	{
		user, err := c.GetUser(context.Background())
		require.NoError(t, err)
		require.Equal(t, "user", user.Name)
		require.Equal(t, userID, user.ID)
	}

	// Wait for 2 more seconds.
	time.Sleep(2 * time.Second)

	// The client's auth token should have expired, but will be refreshed on the next request.
	{
		user, err := c.GetUser(context.Background())
		require.NoError(t, err)
		require.Equal(t, "user", user.Name)
		require.Equal(t, userID, user.ID)
	}
}

func TestAuth_Refresh_Multi(t *testing.T) {
	s := server.New()
	defer s.Close()

	// Create a user on the server.
	userID, _, err := s.CreateUser("user", []byte("pass"))
	require.NoError(t, err)

	// The auth is valid for 4 seconds.
	s.SetAuthLife(4 * time.Second)

	m := proton.New(
		proton.WithHostURL(s.GetHostURL()),
		proton.WithTransport(proton.InsecureTransport()),
	)
	defer m.Close()

	c, auth, err := m.NewClientWithLogin(context.Background(), "user", []byte("pass"))
	require.NoError(t, err)
	require.Equal(t, userID, auth.UserID)

	time.Sleep(2 * time.Second)

	// The client should still be authenticated.
	parallel.Do(runtime.NumCPU(), 100, func(idx int) {
		user, err := c.GetUser(context.Background())
		require.NoError(t, err)
		require.Equal(t, "user", user.Name)
		require.Equal(t, userID, user.ID)
	})

	// Wait for the auth to expire.
	time.Sleep(2 * time.Second)

	// Client auth token should have expired, but will be refreshed on the next request.
	parallel.Do(runtime.NumCPU(), 100, func(idx int) {
		user, err := c.GetUser(context.Background())
		require.NoError(t, err)
		require.Equal(t, "user", user.Name)
		require.Equal(t, userID, user.ID)
	})
}

func TestAuth_Refresh_Deauth(t *testing.T) {
	s := server.New()
	defer s.Close()

	// Create a user on the server.
	userID, _, err := s.CreateUser("user", []byte("pass"))
	require.NoError(t, err)

	m := proton.New(
		proton.WithHostURL(s.GetHostURL()),
		proton.WithTransport(proton.InsecureTransport()),
	)
	defer m.Close()

	// Create one session for the user.
	c, auth, err := m.NewClientWithLogin(context.Background(), "user", []byte("pass"))
	require.NoError(t, err)
	require.Equal(t, userID, auth.UserID)

	deauth := false
	c.AddDeauthHandler(func() {
		deauth = true
	})

	// The client should still be authenticated.
	{
		user, err := c.GetUser(context.Background())
		require.NoError(t, err)
		require.Equal(t, "user", user.Name)
		require.Equal(t, userID, user.ID)
	}

	require.NoError(t, s.RevokeUser(userID))

	// The client's auth token should have expired, and should not be refreshed
	{
		_, err := c.GetUser(context.Background())
		require.Error(t, err)
	}

	// The client shuold call de-auth handlers.
	require.Eventually(t, func() bool { return deauth }, time.Second, 300*time.Millisecond)
}
