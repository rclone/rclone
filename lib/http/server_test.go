package http

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func testEmptyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
}

func testEchoHandler(data []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(data)
	})
}

func testAuthUserHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := CtxGetUser(r.Context())
		if !ok {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		_, _ = w.Write([]byte(userID))
	})
}

func testExpectRespBody(t *testing.T, resp *http.Response, expected []byte) {
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, expected, body)
}

func testGetServerURL(t *testing.T, s *Server) string {
	urls := s.URLs()
	require.GreaterOrEqual(t, len(urls), 1, "server should return at least one url")
	return urls[0]
}

func testNewHTTPClientUnix(path string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", path)
			},
		},
	}
}

func testReadTestdataFile(t *testing.T, path string) []byte {
	data, err := os.ReadFile(filepath.Join("./testdata", path))
	require.NoError(t, err, "")
	return data
}

func TestNewServerUnix(t *testing.T) {
	ctx := context.Background()

	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "rclone.sock")

	cfg := DefaultCfg()
	cfg.ListenAddr = []string{path}

	auth := AuthConfig{
		BasicUser: "test",
		BasicPass: "test",
	}

	s, err := NewServer(ctx, WithConfig(cfg), WithAuth(auth))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, s.Shutdown())
		_, err := os.Stat(path)
		require.ErrorIs(t, err, os.ErrNotExist, "shutdown should remove socket")
	}()

	require.Empty(t, s.URLs(), "unix socket should not appear in URLs")

	expected := []byte("hello world")
	s.Router().Mount("/", testEchoHandler(expected))
	s.Serve()

	client := testNewHTTPClientUnix(path)
	req, err := http.NewRequest("GET", "http://unix", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)

	testExpectRespBody(t, resp, expected)

	require.Equal(t, http.StatusOK, resp.StatusCode, "unix sockets should ignore auth")

	for _, key := range _testCORSHeaderKeys {
		require.NotContains(t, resp.Header, key, "unix sockets should not be sent CORS headers")
	}
}

func TestNewServerHTTP(t *testing.T) {
	ctx := context.Background()

	cfg := DefaultCfg()
	cfg.ListenAddr = []string{"127.0.0.1:0"}

	auth := AuthConfig{
		BasicUser: "test",
		BasicPass: "test",
	}

	s, err := NewServer(ctx, WithConfig(cfg), WithAuth(auth))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, s.Shutdown())
	}()

	url := testGetServerURL(t, s)
	require.True(t, strings.HasPrefix(url, "http://"), "url should have http scheme")

	expected := []byte("hello world")
	s.Router().Mount("/", testEchoHandler(expected))
	s.Serve()

	t.Run("StatusUnauthorized", func(t *testing.T) {
		client := &http.Client{}
		req, err := http.NewRequest("GET", url, nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() {
			_ = resp.Body.Close()
		}()

		require.Equal(t, http.StatusUnauthorized, resp.StatusCode, "no basic auth creds should return unauthorized")
	})

	t.Run("StatusOK", func(t *testing.T) {
		client := &http.Client{}
		req, err := http.NewRequest("GET", url, nil)
		require.NoError(t, err)

		req.SetBasicAuth(auth.BasicUser, auth.BasicPass)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() {
			_ = resp.Body.Close()
		}()

		require.Equal(t, http.StatusOK, resp.StatusCode, "using basic auth creds should return ok")

		testExpectRespBody(t, resp, expected)
	})
}
func TestNewServerBaseURL(t *testing.T) {
	servers := []struct {
		name   string
		cfg    Config
		suffix string
	}{
		{
			name: "Empty",
			cfg: Config{
				ListenAddr: []string{"127.0.0.1:0"},
				BaseURL:    "",
			},
			suffix: "/",
		},
		{
			name: "Single/NoTrailingSlash",
			cfg: Config{
				ListenAddr: []string{"127.0.0.1:0"},
				BaseURL:    "/rclone",
			},
			suffix: "/rclone/",
		},
		{
			name: "Single/TrailingSlash",
			cfg: Config{
				ListenAddr: []string{"127.0.0.1:0"},
				BaseURL:    "/rclone/",
			},
			suffix: "/rclone/",
		},
		{
			name: "Multi/NoTrailingSlash",
			cfg: Config{
				ListenAddr: []string{"127.0.0.1:0"},
				BaseURL:    "/rclone/test/base/url",
			},
			suffix: "/rclone/test/base/url/",
		},
		{
			name: "Multi/TrailingSlash",
			cfg: Config{
				ListenAddr: []string{"127.0.0.1:0"},
				BaseURL:    "/rclone/test/base/url/",
			},
			suffix: "/rclone/test/base/url/",
		},
	}

	for _, ss := range servers {
		t.Run(ss.name, func(t *testing.T) {
			s, err := NewServer(context.Background(), WithConfig(ss.cfg))
			require.NoError(t, err)
			defer func() {
				require.NoError(t, s.Shutdown())
			}()

			expected := []byte("data")
			s.Router().Get("/", testEchoHandler(expected).ServeHTTP)
			s.Serve()

			url := testGetServerURL(t, s)
			require.True(t, strings.HasPrefix(url, "http://"), "url should have http scheme")
			require.True(t, strings.HasSuffix(url, ss.suffix), "url should have the expected suffix")

			client := &http.Client{}
			req, err := http.NewRequest("GET", url, nil)
			require.NoError(t, err)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer func() {
				_ = resp.Body.Close()
			}()

			t.Log(url, resp.Request.URL)

			require.Equal(t, http.StatusOK, resp.StatusCode, "should return ok")

			testExpectRespBody(t, resp, expected)
		})
	}
}

func TestNewServerTLS(t *testing.T) {
	serverCertBytes := testReadTestdataFile(t, "local.crt")
	serverKeyBytes := testReadTestdataFile(t, "local.key")
	clientCertBytes := testReadTestdataFile(t, "client.crt")
	clientKeyBytes := testReadTestdataFile(t, "client.key")
	clientCert, err := tls.X509KeyPair(clientCertBytes, clientKeyBytes)
	require.NoError(t, err)

	// TODO: generate a proper cert with SAN

	servers := []struct {
		name          string
		clientCerts   []tls.Certificate
		wantErr       bool
		wantClientErr bool
		err           error
		http          Config
	}{
		{
			name: "FromFile/Valid",
			http: Config{
				ListenAddr:    []string{"127.0.0.1:0"},
				TLSCert:       "./testdata/local.crt",
				TLSKey:        "./testdata/local.key",
				MinTLSVersion: "tls1.0",
			},
		},
		{
			name:    "FromFile/NoCert",
			wantErr: true,
			err:     ErrTLSFileMismatch,
			http: Config{
				ListenAddr:    []string{"127.0.0.1:0"},
				TLSCert:       "",
				TLSKey:        "./testdata/local.key",
				MinTLSVersion: "tls1.0",
			},
		},
		{
			name:    "FromFile/InvalidCert",
			wantErr: true,
			http: Config{
				ListenAddr:    []string{"127.0.0.1:0"},
				TLSCert:       "./testdata/local.crt.invalid",
				TLSKey:        "./testdata/local.key",
				MinTLSVersion: "tls1.0",
			},
		},
		{
			name:    "FromFile/NoKey",
			wantErr: true,
			err:     ErrTLSFileMismatch,
			http: Config{
				ListenAddr:    []string{"127.0.0.1:0"},
				TLSCert:       "./testdata/local.crt",
				TLSKey:        "",
				MinTLSVersion: "tls1.0",
			},
		},
		{
			name:    "FromFile/InvalidKey",
			wantErr: true,
			http: Config{
				ListenAddr:    []string{"127.0.0.1:0"},
				TLSCert:       "./testdata/local.crt",
				TLSKey:        "./testdata/local.key.invalid",
				MinTLSVersion: "tls1.0",
			},
		},
		{
			name: "FromBody/Valid",
			http: Config{
				ListenAddr:    []string{"127.0.0.1:0"},
				TLSCertBody:   serverCertBytes,
				TLSKeyBody:    serverKeyBytes,
				MinTLSVersion: "tls1.0",
			},
		},
		{
			name:    "FromBody/NoCert",
			wantErr: true,
			err:     ErrTLSBodyMismatch,
			http: Config{
				ListenAddr:    []string{"127.0.0.1:0"},
				TLSCertBody:   nil,
				TLSKeyBody:    serverKeyBytes,
				MinTLSVersion: "tls1.0",
			},
		},
		{
			name:    "FromBody/InvalidCert",
			wantErr: true,
			http: Config{
				ListenAddr:    []string{"127.0.0.1:0"},
				TLSCertBody:   []byte("JUNK DATA"),
				TLSKeyBody:    serverKeyBytes,
				MinTLSVersion: "tls1.0",
			},
		},
		{
			name:    "FromBody/NoKey",
			wantErr: true,
			err:     ErrTLSBodyMismatch,
			http: Config{
				ListenAddr:    []string{"127.0.0.1:0"},
				TLSCertBody:   serverCertBytes,
				TLSKeyBody:    nil,
				MinTLSVersion: "tls1.0",
			},
		},
		{
			name:    "FromBody/InvalidKey",
			wantErr: true,
			http: Config{
				ListenAddr:    []string{"127.0.0.1:0"},
				TLSCertBody:   serverCertBytes,
				TLSKeyBody:    []byte("JUNK DATA"),
				MinTLSVersion: "tls1.0",
			},
		},
		{
			name: "MinTLSVersion/Valid/1.1",
			http: Config{
				ListenAddr:    []string{"127.0.0.1:0"},
				TLSCertBody:   serverCertBytes,
				TLSKeyBody:    serverKeyBytes,
				MinTLSVersion: "tls1.1",
			},
		},
		{
			name: "MinTLSVersion/Valid/1.2",
			http: Config{
				ListenAddr:    []string{"127.0.0.1:0"},
				TLSCertBody:   serverCertBytes,
				TLSKeyBody:    serverKeyBytes,
				MinTLSVersion: "tls1.2",
			},
		},
		{
			name: "MinTLSVersion/Valid/1.3",
			http: Config{
				ListenAddr:    []string{"127.0.0.1:0"},
				TLSCertBody:   serverCertBytes,
				TLSKeyBody:    serverKeyBytes,
				MinTLSVersion: "tls1.3",
			},
		},
		{
			name:    "MinTLSVersion/Invalid",
			wantErr: true,
			err:     ErrInvalidMinTLSVersion,
			http: Config{
				ListenAddr:    []string{"127.0.0.1:0"},
				TLSCertBody:   serverCertBytes,
				TLSKeyBody:    serverKeyBytes,
				MinTLSVersion: "tls0.9",
			},
		},
		{
			name:        "MutualTLS/InvalidCA",
			clientCerts: []tls.Certificate{clientCert},
			wantErr:     true,
			http: Config{
				ListenAddr:    []string{"127.0.0.1:0"},
				TLSCertBody:   serverCertBytes,
				TLSKeyBody:    serverKeyBytes,
				MinTLSVersion: "tls1.0",
				ClientCA:      "./testdata/client-ca.crt.invalid",
			},
		},
		{
			name:          "MutualTLS/InvalidClient",
			clientCerts:   []tls.Certificate{},
			wantClientErr: true,
			http: Config{
				ListenAddr:    []string{"127.0.0.1:0"},
				TLSCertBody:   serverCertBytes,
				TLSKeyBody:    serverKeyBytes,
				MinTLSVersion: "tls1.0",
				ClientCA:      "./testdata/client-ca.crt",
			},
		},
		{
			name:        "MutualTLS/Valid",
			clientCerts: []tls.Certificate{clientCert},
			http: Config{
				ListenAddr:    []string{"127.0.0.1:0"},
				TLSCertBody:   serverCertBytes,
				TLSKeyBody:    serverKeyBytes,
				MinTLSVersion: "tls1.0",
				ClientCA:      "./testdata/client-ca.crt",
			},
		},
	}

	for _, ss := range servers {
		t.Run(ss.name, func(t *testing.T) {
			s, err := NewServer(context.Background(), WithConfig(ss.http))
			if ss.wantErr == true {
				if ss.err != nil {
					require.ErrorIs(t, err, ss.err, "new server should return the expected error")
				} else {
					require.Error(t, err, "new server should return error for invalid TLS config")
				}
				return
			}

			require.NoError(t, err)
			defer func() {
				require.NoError(t, s.Shutdown())
			}()

			expected := []byte("secret-page")
			s.Router().Mount("/", testEchoHandler(expected))
			s.Serve()

			url := testGetServerURL(t, s)
			require.True(t, strings.HasPrefix(url, "https://"), "url should have https scheme")

			client := &http.Client{
				Transport: &http.Transport{
					DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
						dest := strings.TrimPrefix(url, "https://")
						dest = strings.TrimSuffix(dest, "/")
						return net.Dial("tcp", dest)
					},
					TLSClientConfig: &tls.Config{
						Certificates:       ss.clientCerts,
						InsecureSkipVerify: true,
					},
				},
			}
			req, err := http.NewRequest("GET", "https://dev.rclone.org", nil)
			require.NoError(t, err)

			resp, err := client.Do(req)

			if ss.wantClientErr {
				require.Error(t, err, "new server client should return error")
				return
			}

			require.NoError(t, err)
			defer func() {
				_ = resp.Body.Close()
			}()

			require.Equal(t, http.StatusOK, resp.StatusCode, "should return ok")

			testExpectRespBody(t, resp, expected)
		})
	}
}

func TestHelpPrefixServer(t *testing.T) {
	// This test assumes template variables are placed correctly.
	const testPrefix = "server-help-test"
	helpMessage := Help(testPrefix)
	if !strings.Contains(helpMessage, testPrefix) {
		t.Fatal("flag prefix not found")
	}
}
