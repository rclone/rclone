package fshttp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// trackingBody records whether a request body was read to EOF and closed
type trackingBody struct {
	io.Reader
	eof    bool
	closed bool
}

func (b *trackingBody) Read(p []byte) (n int, err error) {
	n, err = b.Reader.Read(p)
	if err == io.EOF {
		b.eof = true
	}
	return n, err
}

func (b *trackingBody) Close() error {
	b.closed = true
	return nil
}

func TestFaultInjector(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	ctx := context.Background()
	client := NewClientCustom(ctx, nil)
	injectErr := errors.New("injected error")

	// Fail the first request with a status code and the second with an
	// error, then let everything through
	var calls atomic.Int32
	SetFaultInjector(func(req *http.Request) (int, error) {
		switch calls.Add(1) {
		case 1:
			return http.StatusInternalServerError, nil
		case 2:
			return 0, injectErr
		}
		return 0, nil
	})
	defer SetFaultInjector(nil)

	do := func() (*trackingBody, *http.Response, error) {
		body := &trackingBody{Reader: strings.NewReader("hello")}
		req, err := http.NewRequestWithContext(ctx, "PUT", server.URL, body)
		require.NoError(t, err)
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
		return body, resp, err
	}

	body, resp, err := do()
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Equal(t, "500 Internal Server Error", resp.Status)
	assert.True(t, body.eof, "body should be drained")
	assert.True(t, body.closed, "body should be closed")
	assert.Equal(t, int32(0), hits.Load(), "server should not see a faulted request")

	body, _, err = do()
	require.ErrorIs(t, err, injectErr)
	assert.True(t, body.eof, "body should be drained")
	assert.True(t, body.closed, "body should be closed")
	assert.Equal(t, int32(0), hits.Load(), "server should not see a faulted request")

	_, resp, err = do()
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, int32(1), hits.Load())

	// Removing the injector lets requests through
	SetFaultInjector(nil)
	_, resp, err = do()
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, int32(2), hits.Load())
	assert.Equal(t, int32(3), calls.Load())

}
