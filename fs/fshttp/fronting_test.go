package fshttp

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testFrontingConfig(t *testing.T) context.Context {
	ctx, ci := fs.AddConfig(context.Background())
	ci.FrontingEnable = true
	ci.FrontingTarget = "front.example"
	ci.FrontingDomains = []string{"*.googleapis.com,example.com"}
	return ctx
}

func TestParseFrontingRules(t *testing.T) {
	rules, err := parseFrontingRules([]string{"*.googleapis.com,example.com"})
	require.NoError(t, err)
	require.Len(t, rules, 2)
	assert.Equal(t, "*.googleapis.com", rules[0].raw)
	assert.Equal(t, "example.com", rules[1].raw)

	fc := &frontingConfig{rules: rules}
	assert.Equal(t, "*.googleapis.com", fc.matchRule("www.googleapis.com"))
	assert.Equal(t, "example.com", fc.matchRule("example.com"))
	assert.Empty(t, fc.matchRule("googleapis.com"))
	assert.Empty(t, fc.matchRule("not-example.com"))
}

func TestParseFrontingRulesInvalid(t *testing.T) {
	_, err := parseFrontingRules([]string{"*foo.example.com"})
	require.Error(t, err)
}

func TestFrontingDisabledPassthrough(t *testing.T) {
	var gotHost string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, ci := fs.AddConfig(context.Background())
	ci.FrontingEnable = false
	client := NewClient(ctx)

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/ping", nil)
	require.NoError(t, err)
	_, err = client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, req.URL.Host, gotHost)
}

func TestFrontingEnabledMatchedHostRewritesDialTarget(t *testing.T) {
	var gotHost, filterURLHost string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	ctx, ci := fs.AddConfig(context.Background())
	ci.FrontingEnable = true
	frontHost, frontPort, err := net.SplitHostPort(target.Listener.Addr().String())
	require.NoError(t, err)
	ci.FrontingTarget = frontHost
	ci.FrontingDomains = []string{"*.googleapis.com"}

	client := NewClient(ctx)
	client.Transport.(*Transport).SetRequestFilter(func(req *http.Request) {
		filterURLHost = req.URL.Host
	})
	req, err := http.NewRequest(http.MethodGet, "http://storage.googleapis.com:"+frontPort+"/upload", nil)
	require.NoError(t, err)
	_, err = client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, "storage.googleapis.com:"+frontPort, gotHost)
	assert.Equal(t, "storage.googleapis.com:"+frontPort, filterURLHost)
}

func TestFrontingEnabledUnmatchedHostNoRewrite(t *testing.T) {
	var originalHits, frontHits int
	original := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originalHits++
		w.WriteHeader(http.StatusOK)
	}))
	defer original.Close()
	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		frontHits++
		w.WriteHeader(http.StatusOK)
	}))
	defer front.Close()

	ctx, ci := fs.AddConfig(context.Background())
	ci.FrontingEnable = true
	frontHost, _, splitErr := net.SplitHostPort(front.Listener.Addr().String())
	require.NoError(t, splitErr)
	ci.FrontingTarget = frontHost
	ci.FrontingDomains = []string{"*.googleapis.com"}

	client := NewClient(ctx)
	req, err := http.NewRequest(http.MethodGet, original.URL+"/noop", nil)
	require.NoError(t, err)
	_, err = client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, 1, originalHits)
	assert.Equal(t, 0, frontHits)
}

func TestFrontingSNIOverride(t *testing.T) {
	var (
		mu     sync.Mutex
		gotSNI string
	)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.TLS = &tls.Config{
		GetConfigForClient: func(chi *tls.ClientHelloInfo) (*tls.Config, error) {
			mu.Lock()
			gotSNI = chi.ServerName
			mu.Unlock()
			return &tls.Config{Certificates: srv.TLS.Certificates}, nil
		},
	}
	srv.StartTLS()
	defer srv.Close()

	ctx, ci := fs.AddConfig(context.Background())
	ci.InsecureSkipVerify = true
	ci.FrontingEnable = true
	frontHost, _, splitErr := net.SplitHostPort(srv.Listener.Addr().String())
	require.NoError(t, splitErr)
	ci.FrontingTarget = frontHost
	ci.FrontingDomains = []string{"*.googleapis.com"}
	ci.FrontingSNI = "allowed-front.example"

	client := NewClient(ctx)
	_, tlsPort, splitErr := net.SplitHostPort(srv.Listener.Addr().String())
	require.NoError(t, splitErr)
	req, err := http.NewRequest(http.MethodGet, "https://drive.googleapis.com:"+tlsPort+"/v3/files", nil)
	require.NoError(t, err)
	_, err = client.Do(req)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, "allowed-front.example", gotSNI)
}

func TestFrontingSNIOverrideNotAppliedToUnmatchedRequest(t *testing.T) {
	var (
		mu     sync.Mutex
		gotSNI string
	)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.TLS = &tls.Config{
		GetConfigForClient: func(chi *tls.ClientHelloInfo) (*tls.Config, error) {
			mu.Lock()
			gotSNI = chi.ServerName
			mu.Unlock()
			return &tls.Config{Certificates: srv.TLS.Certificates}, nil
		},
	}
	srv.StartTLS()
	defer srv.Close()

	ctx, ci := fs.AddConfig(context.Background())
	ci.InsecureSkipVerify = true
	ci.FrontingEnable = true
	ci.FrontingTarget = "localhost"
	ci.FrontingDomains = []string{"*.googleapis.com"}
	ci.FrontingSNI = "allowed-front.example"

	client := NewClient(ctx)
	_, tlsPort, splitErr := net.SplitHostPort(srv.Listener.Addr().String())
	require.NoError(t, splitErr)
	req, err := http.NewRequest(http.MethodGet, "https://localhost:"+tlsPort+"/v3/files", nil)
	require.NoError(t, err)
	_, err = client.Do(req)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, "localhost", gotSNI)
}

func TestParseFrontingConfigValidation(t *testing.T) {
	ctx := testFrontingConfig(t)
	ci := fs.GetConfig(ctx)
	ci.FrontingTarget = ""
	_, err := parseFrontingConfig(ci)
	require.Error(t, err)

	ci.FrontingTarget = "front.example:443"
	_, err = parseFrontingConfig(ci)
	require.Error(t, err)

	ci.FrontingTarget = "front.example"
	ci.FrontingSNI = "front.example:443"
	_, err = parseFrontingConfig(ci)
	require.Error(t, err)
}
