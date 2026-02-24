package server

import (
	"context"
	"testing"

	"github.com/rclone/go-proton-api"
	"github.com/stretchr/testify/require"
)

func TestServer_Quark_CreateUser(t *testing.T) {
	withServer(t, func(ctx context.Context, _ *Server, m *proton.Manager) {
		// Create two users, one with keys and one without.
		require.NoError(t, m.Quark(ctx, "user:create", "--name", "user-no-keys", "--password", "test", "--create-address"))
		require.NoError(t, m.Quark(ctx, "user:create", "--name", "user-keys", "--password", "test", "--gen-keys", "rsa2048"))
		require.NoError(t, m.Quark(ctx, "user:create", "--name", "user-disabled", "--password", "test", "--gen-keys", "rsa2048", "--status", "1"))

		{
			// The address should be created but should have no keys.
			c, _, err := m.NewClientWithLogin(ctx, "user-no-keys", []byte("test"))
			require.NoError(t, err)
			defer c.Close()

			addr, err := c.GetAddresses(ctx)
			require.NoError(t, err)
			require.Len(t, addr, 1)
			require.Len(t, addr[0].Keys, 0)
		}

		{
			// The address should be created and should have keys.
			c, _, err := m.NewClientWithLogin(ctx, "user-keys", []byte("test"))
			require.NoError(t, err)
			defer c.Close()

			addr, err := c.GetAddresses(ctx)
			require.NoError(t, err)
			require.Len(t, addr, 1)
			require.Len(t, addr[0].Keys, 1)
		}

		{
			// The address should be created and should be disabled
			c, _, err := m.NewClientWithLogin(ctx, "user-disabled", []byte("test"))
			require.NoError(t, err)
			defer c.Close()

			addr, err := c.GetAddresses(ctx)
			require.NoError(t, err)
			require.Len(t, addr, 1)
			require.Len(t, addr[0].Keys, 1)
			require.Equal(t, addr[0].Status, proton.AddressStatusDisabled)
		}
	})
}

func TestServer_Quark_CreateAddress(t *testing.T) {
	withServer(t, func(ctx context.Context, _ *Server, m *proton.Manager) {
		// Create a user with one address.
		require.NoError(t, m.Quark(ctx, "user:create", "--name", "user", "--password", "test", "--gen-keys", "rsa2048"))

		// Login to the user.
		c, _, err := m.NewClientWithLogin(ctx, "user", []byte("test"))
		require.NoError(t, err)
		defer c.Close()

		// Get the user.
		user, err := c.GetUser(ctx)
		require.NoError(t, err)

		// Initially the user should have one address and it should have keys.
		addr, err := c.GetAddresses(ctx)
		require.NoError(t, err)
		require.Len(t, addr, 1)
		require.Len(t, addr[0].Keys, 1)

		// Create a new address.
		require.NoError(t, m.Quark(ctx, "user:create:address", "--gen-keys", "rsa2048", user.ID, "test", "alias@proton.local"))

		// Now the user should have two addresses, and they should both have keys.
		newAddr, err := c.GetAddresses(ctx)
		require.NoError(t, err)
		require.Len(t, newAddr, 2)
		require.Len(t, newAddr[0].Keys, 1)
		require.Len(t, newAddr[1].Keys, 1)
	})
}
