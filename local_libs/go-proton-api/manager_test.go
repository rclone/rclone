package proton_test

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/rclone/go-proton-api"
	"github.com/rclone/go-proton-api/server"
	"github.com/stretchr/testify/require"
)

func TestConnectionReuse(t *testing.T) {
	s := server.New()
	defer s.Close()

	ctl := proton.NewNetCtl()

	var dialed int

	ctl.OnDial(func(net.Conn) {
		dialed++
	})

	m := proton.New(
		proton.WithHostURL(s.GetHostURL()),
		proton.WithTransport(ctl.NewRoundTripper(&tls.Config{InsecureSkipVerify: true})),
	)

	// This should succeed; the resulting connection should be reused.
	require.NoError(t, m.Ping(context.Background()))

	// We should have dialed once.
	require.Equal(t, 1, dialed)

	// This should succeed; we should not re-dial.
	require.NoError(t, m.Ping(context.Background()))

	// We should not have re-dialed.
	require.Equal(t, 1, dialed)
}

func TestAuthRefresh(t *testing.T) {
	s := server.New()
	defer s.Close()

	_, _, err := s.CreateUser("user", []byte("pass"))
	require.NoError(t, err)

	m := proton.New(
		proton.WithHostURL(s.GetHostURL()),
		proton.WithTransport(proton.InsecureTransport()),
	)

	c1, auth, err := m.NewClientWithLogin(context.Background(), "user", []byte("pass"))
	require.NoError(t, err)
	defer c1.Close()

	c2, auth, err := m.NewClientWithRefresh(context.Background(), auth.UID, auth.RefreshToken)
	require.NoError(t, err)
	defer c2.Close()
}

func TestHandleTooManyRequests(t *testing.T) {
	// Create a server with a rate limit of 1 request per second.
	s := server.New(server.WithRateLimit(1, time.Second))
	defer s.Close()

	var calls []server.Call

	// Watch the calls made.
	s.AddCallWatcher(func(call server.Call) {
		calls = append(calls, call)
	})

	m := proton.New(
		proton.WithHostURL(s.GetHostURL()),
		proton.WithTransport(proton.InsecureTransport()),
	)
	defer m.Close()

	// Make five calls; they should all succeed, but will be rate limited.
	for i := 0; i < 5; i++ {
		require.NoError(t, m.Ping(context.Background()))
	}

	// After each 429 response, we should wait at least the requested duration before making the next request.
	for idx, call := range calls {
		if call.Status == http.StatusTooManyRequests {
			after, err := strconv.Atoi(call.ResponseHeader.Get("Retry-After"))
			require.NoError(t, err)

			// The next call should be made after the requested duration.
			require.True(t, calls[idx+1].Time.After(call.Time.Add(time.Duration(after)*time.Second)))
		}
	}
}

func TestHandleTooManyRequests503(t *testing.T) {
	// Create a server with a rate limit of 1 request per second.
	s := server.New(server.WithRateLimitAndCustomStatusCode(1, time.Second, http.StatusServiceUnavailable))
	defer s.Close()

	var calls []server.Call

	// Watch the calls made.
	s.AddCallWatcher(func(call server.Call) {
		calls = append(calls, call)
	})

	m := proton.New(
		proton.WithHostURL(s.GetHostURL()),
		proton.WithTransport(proton.InsecureTransport()),
	)
	defer m.Close()

	// Make five calls; they should all succeed, but will be rate limited.
	for i := 0; i < 5; i++ {
		require.NoError(t, m.Ping(context.Background()))
	}

	// After each 503 response, we should wait at least the requested duration before making the next request.
	for idx, call := range calls {
		if call.Status == http.StatusServiceUnavailable {
			after, err := strconv.Atoi(call.ResponseHeader.Get("Retry-After"))
			require.NoError(t, err)

			// The next call should be made after the requested duration.
			require.True(t, calls[idx+1].Time.After(call.Time.Add(time.Duration(after)*time.Second)))
		}
	}
}

func TestHandleTooManyRequests_Malformed(t *testing.T) {
	var calls []time.Time

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if len(calls) == 0 {
			w.Header().Set("Retry-After", "malformed")
			w.WriteHeader(http.StatusTooManyRequests)
		}

		calls = append(calls, time.Now())
	}))
	defer ts.Close()

	m := proton.New(proton.WithHostURL(ts.URL))
	defer m.Close()

	require.NoError(t, m.Ping(context.Background()))

	// The first call should fail because the Retry-After header is invalid.
	// The second call should succeed.
	require.Len(t, calls, 2)

	// The second call should be made at least 10 seconds after the first call.
	require.True(t, calls[1].After(calls[0].Add(10*time.Second)))
}

func TestHandleUnprocessableEntity(t *testing.T) {
	var numCalls int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		numCalls++
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer ts.Close()

	m := proton.New(
		proton.WithHostURL(ts.URL),
		proton.WithRetryCount(5),
	)

	// The call should fail because the first call should fail (422s are not retried).
	c := m.NewClient("", "", "")
	defer c.Close()

	if _, err := c.GetAddresses(context.Background()); err == nil {
		t.Fatal("expected error, instead got", err)
	}

	// The server should be called 1 time.
	// The first call should return 422.
	if numCalls != 1 {
		t.Fatal("expected numCalls to be 1, instead got", numCalls)
	}
}

func TestHandleDialFailure(t *testing.T) {
	var numCalls int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		numCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	m := proton.New(
		proton.WithHostURL(ts.URL),
		proton.WithRetryCount(5),
		proton.WithTransport(newFailingRoundTripper(5)),
	)

	// The call should succeed because the last retry should succeed (dial errors are retried).
	c := m.NewClient("", "", "")
	defer c.Close()

	if _, err := c.GetAddresses(context.Background()); err != nil {
		t.Fatal("got unexpected error", err)
	}

	// The server should be called 1 time.
	// The first 4 attempts don't reach the server.
	if numCalls != 1 {
		t.Fatal("expected numCalls to be 1, instead got", numCalls)
	}
}

func TestHandleTooManyDialFailures(t *testing.T) {
	var numCalls int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		numCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// The failingRoundTripper will fail the first 10 times it is used.
	// This is more than the number of retries we permit.
	// Thus, dials will fail.
	m := proton.New(
		proton.WithHostURL(ts.URL),
		proton.WithRetryCount(5),
		proton.WithTransport(newFailingRoundTripper(10)),
	)

	// The call should fail because every dial will fail and we'll run out of retries.
	c := m.NewClient("", "", "")
	defer c.Close()

	if _, err := c.GetAddresses(context.Background()); err == nil {
		t.Fatal("expected error, instead got", err)
	}

	// The server should never be called.
	if numCalls != 0 {
		t.Fatal("expected numCalls to be 0, instead got", numCalls)
	}
}

func TestRetriesWithContextTimeout(t *testing.T) {
	var numCalls int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		numCalls++

		if numCalls < 5 {
			w.WriteHeader(http.StatusTooManyRequests)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		time.Sleep(time.Second)
	}))
	defer ts.Close()

	m := proton.New(
		proton.WithHostURL(ts.URL),
		proton.WithRetryCount(5),
	)

	// Timeout after 1s.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Theoretically, this should succeed; on the fifth retry, we'll get StatusOK.
	// However, that will take at least >5s, and we only allow 1s in the context.
	// Thus, it will fail.
	c := m.NewClient("", "", "")
	defer c.Close()

	if _, err := c.GetAddresses(ctx); err == nil {
		t.Fatal("expected error, instead got", err)
	}
}

func TestReturnErrNoConnection(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// We will fail more times than we retry, so requests should fail with ErrNoConnection.
	m := proton.New(
		proton.WithHostURL(ts.URL),
		proton.WithRetryCount(5),
		proton.WithTransport(newFailingRoundTripper(10)),
	)

	// The call should fail because every dial will fail and we'll run out of retries.
	c := m.NewClient("", "", "")
	defer c.Close()

	if _, err := c.GetAddresses(context.Background()); err == nil {
		t.Fatal("expected error, instead got", err)
	}
}

func TestStatusCallbacks(t *testing.T) {
	s := server.New()
	defer s.Close()

	ctl := proton.NewNetCtl()

	m := proton.New(
		proton.WithHostURL(s.GetHostURL()),
		proton.WithTransport(ctl.NewRoundTripper(&tls.Config{InsecureSkipVerify: true})),
	)

	statusCh := make(chan proton.Status, 1)

	m.AddStatusObserver(func(status proton.Status) {
		statusCh <- status
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctl.Disable()

	require.Error(t, m.Ping(ctx))
	require.Equal(t, proton.StatusDown, <-statusCh)

	ctl.Enable()

	require.NoError(t, m.Ping(ctx))
	require.Equal(t, proton.StatusUp, <-statusCh)

	ctl.SetReadLimit(1)

	require.Error(t, m.Ping(ctx))
	require.Equal(t, proton.StatusDown, <-statusCh)

	ctl.SetReadLimit(0)

	require.NoError(t, m.Ping(ctx))
	require.Equal(t, proton.StatusUp, <-statusCh)
}

func Test503IsReportedAsAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	m := proton.New(
		proton.WithHostURL(ts.URL),
		proton.WithRetryCount(5),
	)

	c := m.NewClient("", "", "")
	defer c.Close()

	_, err := c.GetAddresses(context.Background())
	require.Error(t, err)

	var protonErr *proton.APIError
	require.True(t, errors.As(err, &protonErr))
	require.Equal(t, 503, protonErr.Status)
}

type failingRoundTripper struct {
	http.RoundTripper

	fails, calls int
}

func newFailingRoundTripper(fails int) http.RoundTripper {
	return &failingRoundTripper{
		RoundTripper: http.DefaultTransport,
		fails:        fails,
	}
}

func (rt *failingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.calls++

	if rt.calls < rt.fails {
		return nil, errors.New("simulating network error")
	}

	return rt.RoundTripper.RoundTrip(req)
}
