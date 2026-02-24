package proton_test

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"

	"github.com/rclone/go-proton-api"
	"github.com/rclone/go-proton-api/server"
	"github.com/stretchr/testify/require"
)

func TestAttachment_429Response(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := server.New()
	defer s.Close()

	m := proton.New(
		proton.WithHostURL(s.GetHostURL()),
		proton.WithTransport(proton.InsecureTransport()),
	)

	_, _, err := s.CreateUser("user", []byte("pass"))
	require.NoError(t, err)

	c, _, err := m.NewClientWithLogin(ctx, "user", []byte("pass"))
	require.NoError(t, err)

	s.AddStatusHook(func(r *http.Request) (int, bool) {
		return http.StatusTooManyRequests, true
	})

	_, err = c.GetAttachment(ctx, "someID")
	require.Error(t, err)

	apiErr := new(proton.APIError)
	require.True(t, errors.As(err, &apiErr), "expected to be API error")
	require.Equal(t, 429, apiErr.Status)
	require.Equal(t, proton.InvalidValue, apiErr.Code)
	require.Equal(t, "Request failed with status 429", apiErr.Message)
}

func TestAttachment_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	s := server.New()
	defer s.Close()

	m := proton.New(
		proton.WithHostURL(s.GetHostURL()),
		proton.WithTransport(proton.InsecureTransport()),
	)

	_, _, err := s.CreateUser("user", []byte("pass"))
	require.NoError(t, err)

	c, _, err := m.NewClientWithLogin(ctx, "user", []byte("pass"))
	require.NoError(t, err)

	wg := sync.WaitGroup{}
	wg.Add(1)

	s.AddStatusHook(func(r *http.Request) (int, bool) {
		wg.Wait()
		return http.StatusTooManyRequests, true
	})

	go func() {
		_, err = c.GetAttachment(ctx, "someID")
		wg.Done()
	}()

	cancel()

	wg.Wait()
	require.Error(t, err)
	require.True(t, errors.Is(err, context.Canceled))
}
