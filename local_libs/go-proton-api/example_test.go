package proton_test

import (
	"context"
	"fmt"
	"time"

	"github.com/rclone/go-proton-api"
)

func ExampleManager_NewClient() {
	// Create a new manager.
	m := proton.New()

	// If auth information is already known, it can be used to create a client straight away.
	c := m.NewClient("...uid...", "...acc...", "...ref...")
	defer c.Close()

	// All API operations must be run within a context.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Do something with the client.
	if _, err := c.GetUser(ctx); err != nil {
		panic(err)
	}
}

func ExampleManager_NewClientWithRefresh() {
	// Create a new manager.
	m := proton.New()

	// All API operations must be run within a context.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// If UID/RefreshToken is already known, it can be used to create a new client straight away.
	c, _, err := m.NewClientWithRefresh(ctx, "...uid...", "...ref...")
	if err != nil {
		panic(err)
	}
	defer c.Close()

	// Do something with the client.
	if _, err := c.GetUser(ctx); err != nil {
		panic(err)
	}
}

func ExampleManager_NewClientWithLogin() {
	// Create a new manager.
	m := proton.New()

	// All API operations must be run within a context.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Clients are created via username/password if auth information isn't already known.
	c, auth, err := m.NewClientWithLogin(ctx, "...user...", []byte("...pass..."))
	if err != nil {
		panic(err)
	}
	defer c.Close()

	// If 2FA is necessary, an additional request is required.
	if auth.TwoFA.Enabled&proton.HasTOTP != 0 {
		if err := c.Auth2FA(ctx, proton.Auth2FAReq{TwoFactorCode: "...TOTP..."}); err != nil {
			panic(err)
		}
	}

	// Do something with the client.
	if _, err := c.GetUser(ctx); err != nil {
		panic(err)
	}
}

func ExampleClient_AddAuthHandler() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a new manager.
	m := proton.New()

	// Create a new client.
	c := m.NewClient("...uid...", "...acc...", "...ref...")
	defer c.Close()

	// Register an auth handler with the client.
	// This could be used for example to save the auth to keychain.
	c.AddAuthHandler(func(auth proton.Auth) {
		// Do something with auth.
	})

	if _, err := c.GetUser(ctx); err != nil {
		panic(err)
	}
}

func ExampleClient_NewEventStream() {
	m := proton.New()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c, _, err := m.NewClientWithLogin(ctx, "...user...", []byte("...pass..."))
	if err != nil {
		panic(err)
	}
	defer c.Close()

	// Get the latest event ID.
	fromEventID, err := c.GetLatestEventID(context.Background())
	if err != nil {
		panic(err)
	}

	// Create a new event streamer.
	for event := range c.NewEventStream(ctx, 20*time.Second, 20*time.Second, fromEventID) {
		fmt.Println(event.EventID)
	}
}
