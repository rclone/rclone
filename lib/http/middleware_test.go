package http

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	goauth "github.com/abbot/go-http-auth"
	"github.com/stretchr/testify/require"
)

func TestMiddlewareAuth(t *testing.T) {
	servers := []struct {
		name         string
		expectedUser string
		remoteUser   string
		http         Config
		auth         AuthConfig
		user         string
		pass         string
	}{
		{
			name: "Basic",
			http: Config{
				ListenAddr: []string{"127.0.0.1:0"},
			},
			auth: AuthConfig{
				Realm:     "test",
				BasicUser: "test",
				BasicPass: "test",
			},
			user: "test",
			pass: "test",
		},
		{
			name: "Htpasswd/MD5",
			http: Config{
				ListenAddr: []string{"127.0.0.1:0"},
			},
			auth: AuthConfig{
				Realm:    "test",
				HtPasswd: "./testdata/.htpasswd",
			},
			user: "md5",
			pass: "md5",
		},
		{
			name: "Htpasswd/SHA",
			http: Config{
				ListenAddr: []string{"127.0.0.1:0"},
			},
			auth: AuthConfig{
				Realm:    "test",
				HtPasswd: "./testdata/.htpasswd",
			},
			user: "sha",
			pass: "sha",
		},
		{
			name: "Htpasswd/Bcrypt",
			http: Config{
				ListenAddr: []string{"127.0.0.1:0"},
			},
			auth: AuthConfig{
				Realm:    "test",
				HtPasswd: "./testdata/.htpasswd",
			},
			user: "bcrypt",
			pass: "bcrypt",
		},
		{
			name: "Custom",
			http: Config{
				ListenAddr: []string{"127.0.0.1:0"},
			},
			auth: AuthConfig{
				Realm: "test",
				CustomAuthFn: func(user, pass string) (value any, err error) {
					if user == "custom" && pass == "custom" {
						return true, nil
					}
					return nil, errors.New("invalid credentials")
				},
			},
			user: "custom",
			pass: "custom",
		}, {
			name:         "UserFromHeader",
			remoteUser:   "remoteUser",
			expectedUser: "remoteUser",
			http: Config{
				ListenAddr: []string{"127.0.0.1:0"},
			},
			auth: AuthConfig{
				UserFromHeader: "X-Remote-User",
			},
		}, {
			name:         "UserFromHeader/MixedWithHtPasswd",
			remoteUser:   "remoteUser",
			expectedUser: "md5",
			http: Config{
				ListenAddr: []string{"127.0.0.1:0"},
			},
			auth: AuthConfig{
				UserFromHeader: "X-Remote-User",
				Realm:          "test",
				HtPasswd:       "./testdata/.htpasswd",
			},
			user: "md5",
			pass: "md5",
		},
	}
	for _, ss := range servers {
		t.Run(ss.name, func(t *testing.T) {
			s, err := NewServer(context.Background(), WithConfig(ss.http), WithAuth(ss.auth))
			require.NoError(t, err)
			defer func() {
				require.NoError(t, s.Shutdown())
			}()

			expected := []byte("secret-page")
			if ss.expectedUser != "" {
				s.Router().Mount("/", testAuthUserHandler())
			} else {
				s.Router().Mount("/", testEchoHandler(expected))
			}

			s.Serve()

			url := testGetServerURL(t, s)

			t.Run("NoCreds", func(t *testing.T) {
				client := &http.Client{}
				req, err := http.NewRequest("GET", url, nil)
				require.NoError(t, err)

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer func() {
					_ = resp.Body.Close()
				}()

				require.Equal(t, http.StatusUnauthorized, resp.StatusCode, "using no creds should return unauthorized")
				if ss.auth.UserFromHeader == "" {
					wwwAuthHeader := resp.Header.Get("WWW-Authenticate")
					require.NotEmpty(t, wwwAuthHeader, "resp should contain WWW-Authtentication header")
					require.Contains(t, wwwAuthHeader, fmt.Sprintf("realm=%q", ss.auth.Realm), "WWW-Authtentication header should contain relam")
				}
			})
			t.Run("BadCreds", func(t *testing.T) {
				client := &http.Client{}
				req, err := http.NewRequest("GET", url, nil)
				require.NoError(t, err)

				if ss.user != "" {
					req.SetBasicAuth(ss.user+"BAD", ss.pass+"BAD")
				}

				if ss.auth.UserFromHeader != "" {
					req.Header.Set(ss.auth.UserFromHeader, "/test:")
				}

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer func() {
					_ = resp.Body.Close()
				}()

				require.Equal(t, http.StatusUnauthorized, resp.StatusCode, "using bad creds should return unauthorized")
				if ss.auth.UserFromHeader == "" {
					wwwAuthHeader := resp.Header.Get("WWW-Authenticate")
					require.NotEmpty(t, wwwAuthHeader, "resp should contain WWW-Authtentication header")
					require.Contains(t, wwwAuthHeader, fmt.Sprintf("realm=%q", ss.auth.Realm), "WWW-Authtentication header should contain relam")
				}
			})

			t.Run("GoodCreds", func(t *testing.T) {
				client := &http.Client{}
				req, err := http.NewRequest("GET", url, nil)
				require.NoError(t, err)

				if ss.user != "" {
					req.SetBasicAuth(ss.user, ss.pass)
				}

				if ss.auth.UserFromHeader != "" {
					req.Header.Set(ss.auth.UserFromHeader, ss.remoteUser)
				}

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer func() {
					_ = resp.Body.Close()
				}()

				require.Equal(t, http.StatusOK, resp.StatusCode, "using good creds should return ok")

				if ss.expectedUser != "" {
					testExpectRespBody(t, resp, []byte(ss.expectedUser))
				} else {
					testExpectRespBody(t, resp, expected)
				}
			})
		})
	}
}

func TestMiddlewareAuthCertificateUser(t *testing.T) {
	serverCertBytes := testReadTestdataFile(t, "local.crt")
	serverKeyBytes := testReadTestdataFile(t, "local.key")
	clientCertBytes := testReadTestdataFile(t, "client.crt")
	clientKeyBytes := testReadTestdataFile(t, "client.key")
	clientCert, err := tls.X509KeyPair(clientCertBytes, clientKeyBytes)
	require.NoError(t, err)
	emptyCertBytes := testReadTestdataFile(t, "emptyclient.crt")
	emptyKeyBytes := testReadTestdataFile(t, "emptyclient.key")
	emptyCert, err := tls.X509KeyPair(emptyCertBytes, emptyKeyBytes)
	require.NoError(t, err)
	invalidCert, err := tls.X509KeyPair(serverCertBytes, serverKeyBytes)
	require.NoError(t, err)

	servers := []struct {
		name        string
		wantErr     bool
		status      int
		result      string
		http        Config
		auth        AuthConfig
		clientCerts []tls.Certificate
	}{
		{
			name:    "Missing",
			wantErr: true,
			http: Config{
				ListenAddr:    []string{"127.0.0.1:0"},
				TLSCertBody:   serverCertBytes,
				TLSKeyBody:    serverKeyBytes,
				MinTLSVersion: "tls1.0",
				ClientCA:      "./testdata/client-ca.crt",
			},
		},
		{
			name:        "Invalid",
			wantErr:     true,
			clientCerts: []tls.Certificate{invalidCert},
			http: Config{
				ListenAddr:    []string{"127.0.0.1:0"},
				TLSCertBody:   serverCertBytes,
				TLSKeyBody:    serverKeyBytes,
				MinTLSVersion: "tls1.0",
				ClientCA:      "./testdata/client-ca.crt",
			},
		},
		{
			name:        "EmptyCommonName",
			status:      http.StatusUnauthorized,
			result:      fmt.Sprintf("%s\n", http.StatusText(http.StatusUnauthorized)),
			clientCerts: []tls.Certificate{emptyCert},
			http: Config{
				ListenAddr:    []string{"127.0.0.1:0"},
				TLSCertBody:   serverCertBytes,
				TLSKeyBody:    serverKeyBytes,
				MinTLSVersion: "tls1.0",
				ClientCA:      "./testdata/client-ca.crt",
			},
		},
		{
			name:        "Valid",
			status:      http.StatusOK,
			result:      "rclone-dev-client",
			clientCerts: []tls.Certificate{clientCert},
			http: Config{
				ListenAddr:    []string{"127.0.0.1:0"},
				TLSCertBody:   serverCertBytes,
				TLSKeyBody:    serverKeyBytes,
				MinTLSVersion: "tls1.0",
				ClientCA:      "./testdata/client-ca.crt",
			},
		},
		{
			name:        "CustomAuth/Invalid",
			status:      http.StatusUnauthorized,
			result:      fmt.Sprintf("%d %s\n", http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized)),
			clientCerts: []tls.Certificate{clientCert},
			http: Config{
				ListenAddr:    []string{"127.0.0.1:0"},
				TLSCertBody:   serverCertBytes,
				TLSKeyBody:    serverKeyBytes,
				MinTLSVersion: "tls1.0",
				ClientCA:      "./testdata/client-ca.crt",
			},
			auth: AuthConfig{
				Realm: "test",
				CustomAuthFn: func(user, pass string) (value any, err error) {
					if user == "custom" && pass == "custom" {
						return true, nil
					}
					return nil, errors.New("invalid credentials")
				},
			},
		},
		{
			name:        "CustomAuth/Valid",
			status:      http.StatusOK,
			result:      "rclone-dev-client",
			clientCerts: []tls.Certificate{clientCert},
			http: Config{
				ListenAddr:    []string{"127.0.0.1:0"},
				TLSCertBody:   serverCertBytes,
				TLSKeyBody:    serverKeyBytes,
				MinTLSVersion: "tls1.0",
				ClientCA:      "./testdata/client-ca.crt",
			},
			auth: AuthConfig{
				Realm: "test",
				CustomAuthFn: func(user, pass string) (value any, err error) {
					fmt.Println("CUSTOMAUTH", user, pass)
					if user == "rclone-dev-client" && pass == "" {
						return true, nil
					}
					return nil, errors.New("invalid credentials")
				},
			},
		},
	}

	for _, ss := range servers {
		t.Run(ss.name, func(t *testing.T) {
			s, err := NewServer(context.Background(), WithConfig(ss.http), WithAuth(ss.auth))
			require.NoError(t, err)
			defer func() {
				require.NoError(t, s.Shutdown())
			}()

			s.Router().Mount("/", testAuthUserHandler())
			s.Serve()

			url := testGetServerURL(t, s)
			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						Certificates:       ss.clientCerts,
						InsecureSkipVerify: true,
					},
				},
			}
			req, err := http.NewRequest("GET", url, nil)
			require.NoError(t, err)

			resp, err := client.Do(req)
			if ss.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			defer func() {
				_ = resp.Body.Close()
			}()

			require.Equal(t, ss.status, resp.StatusCode, fmt.Sprintf("should return status %d", ss.status))

			testExpectRespBody(t, resp, []byte(ss.result))
		})
	}

}

var _testCORSHeaderKeys = []string{
	"Access-Control-Allow-Origin",
	"Access-Control-Allow-Headers",
	"Access-Control-Allow-Methods",
}

func TestMiddlewareCORS(t *testing.T) {
	servers := []struct {
		name    string
		http    Config
		tryRoot bool
		method  string
		status  int
	}{
		{
			name: "CustomOrigin",
			http: Config{
				ListenAddr:  []string{"127.0.0.1:0"},
				AllowOrigin: "http://test.rclone.org",
			},
			method: "GET",
			status: http.StatusOK,
		},
		{
			name: "WithBaseURL",
			http: Config{
				ListenAddr:  []string{"127.0.0.1:0"},
				AllowOrigin: "http://test.rclone.org",
				BaseURL:     "/baseurl/",
			},
			method: "GET",
			status: http.StatusOK,
		},
		{
			name: "WithBaseURLTryRootGET",
			http: Config{
				ListenAddr:  []string{"127.0.0.1:0"},
				AllowOrigin: "http://test.rclone.org",
				BaseURL:     "/baseurl/",
			},
			method:  "GET",
			status:  http.StatusNotFound,
			tryRoot: true,
		},
		{
			name: "WithBaseURLTryRootOPTIONS",
			http: Config{
				ListenAddr:  []string{"127.0.0.1:0"},
				AllowOrigin: "http://test.rclone.org",
				BaseURL:     "/baseurl/",
			},
			method:  "OPTIONS",
			status:  http.StatusOK,
			tryRoot: true,
		},
	}

	for _, ss := range servers {
		t.Run(ss.name, func(t *testing.T) {
			s, err := NewServer(context.Background(), WithConfig(ss.http))
			require.NoError(t, err)
			defer func() {
				require.NoError(t, s.Shutdown())
			}()

			expected := []byte("data")
			s.Router().Mount("/", testEchoHandler(expected))
			s.Serve()

			url := testGetServerURL(t, s)
			// Try the query on the root, ignoring the baseURL
			if ss.tryRoot {
				slash := strings.LastIndex(url[:len(url)-1], "/")
				url = url[:slash+1]
			}

			client := &http.Client{}
			req, err := http.NewRequest(ss.method, url, nil)
			require.NoError(t, err)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer func() {
				_ = resp.Body.Close()
			}()

			require.Equal(t, ss.status, resp.StatusCode, "should return expected error code")

			if ss.status == http.StatusNotFound {
				return
			}
			testExpectRespBody(t, resp, expected)

			for _, key := range _testCORSHeaderKeys {
				require.Contains(t, resp.Header, key, "CORS headers should be sent")
			}

			expectedOrigin := url
			if ss.http.AllowOrigin != "" {
				expectedOrigin = ss.http.AllowOrigin
			}
			require.Equal(t, expectedOrigin, resp.Header.Get("Access-Control-Allow-Origin"), "allow origin should match")
		})
	}
}

func TestMiddlewareCORSEmptyOrigin(t *testing.T) {
	servers := []struct {
		name string
		http Config
	}{
		{
			name: "EmptyOrigin",
			http: Config{
				ListenAddr:  []string{"127.0.0.1:0"},
				AllowOrigin: "",
			},
		},
	}

	for _, ss := range servers {
		t.Run(ss.name, func(t *testing.T) {
			s, err := NewServer(context.Background(), WithConfig(ss.http))
			require.NoError(t, err)
			defer func() {
				require.NoError(t, s.Shutdown())
			}()

			expected := []byte("data")
			s.Router().Mount("/", testEchoHandler(expected))
			s.Serve()

			url := testGetServerURL(t, s)

			client := &http.Client{}
			req, err := http.NewRequest("GET", url, nil)
			require.NoError(t, err)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer func() {
				_ = resp.Body.Close()
			}()

			require.Equal(t, http.StatusOK, resp.StatusCode, "should return ok")

			testExpectRespBody(t, resp, expected)

			for _, key := range _testCORSHeaderKeys {
				require.NotContains(t, resp.Header, key, "CORS headers should not be sent")
			}
		})
	}
}

func TestMiddlewareCORSWithAuth(t *testing.T) {
	authServers := []struct {
		name string
		http Config
		auth AuthConfig
	}{
		{
			name: "ServerWithAuth",
			http: Config{
				ListenAddr:  []string{"127.0.0.1:0"},
				AllowOrigin: "http://test.rclone.org",
			},
			auth: AuthConfig{
				Realm:     "test",
				BasicUser: "test_user",
				BasicPass: "test_pass",
			},
		},
	}

	for _, ss := range authServers {
		t.Run(ss.name, func(t *testing.T) {
			s, err := NewServer(context.Background(), WithConfig(ss.http))
			require.NoError(t, err)
			defer func() {
				require.NoError(t, s.Shutdown())
			}()

			s.Router().Mount("/", testEmptyHandler())
			s.Serve()

			url := testGetServerURL(t, s)

			client := &http.Client{}
			req, err := http.NewRequest("OPTIONS", url, nil)
			require.NoError(t, err)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer func() {
				_ = resp.Body.Close()
			}()

			require.Equal(t, http.StatusOK, resp.StatusCode, "OPTIONS should return ok even if not authenticated")

			testExpectRespBody(t, resp, []byte{})

			for _, key := range _testCORSHeaderKeys {
				require.Contains(t, resp.Header, key, "CORS headers should be sent even if not authenticated")
			}

			expectedOrigin := url
			if ss.http.AllowOrigin != "" {
				expectedOrigin = ss.http.AllowOrigin
			}
			require.Equal(t, expectedOrigin, resp.Header.Get("Access-Control-Allow-Origin"), "allow origin should match")
		})
	}
}

func TestMiddlewareAuthBcryptCache(t *testing.T) {
	s, err := NewServer(context.Background(),
		WithConfig(Config{ListenAddr: []string{"127.0.0.1:0"}}),
		WithAuth(AuthConfig{Realm: "test", HtPasswd: "./testdata/.htpasswd"}),
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, s.Shutdown()) }()
	s.Router().Mount("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	s.Serve()
	url := testGetServerURL(t, s)
	client := &http.Client{}

	sendReq := func(user, pass string) int {
		req, err := http.NewRequest("GET", url, nil)
		require.NoError(t, err)
		if user != "" {
			req.SetBasicAuth(user, pass)
		}
		resp, err := client.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		return resp.StatusCode
	}

	t.Run("Bcrypt/CacheMiss", func(t *testing.T) {
		require.Equal(t, http.StatusOK, sendReq("bcrypt", "bcrypt"))
	})
	t.Run("Bcrypt/CacheHit", func(t *testing.T) {
		require.Equal(t, http.StatusOK, sendReq("bcrypt", "bcrypt"))
	})
	t.Run("Bcrypt/WrongPasswordFallsThrough", func(t *testing.T) {
		require.Equal(t, http.StatusUnauthorized, sendReq("bcrypt", "wrongpassword"))
	})
	t.Run("Bcrypt/NoCreds", func(t *testing.T) {
		require.Equal(t, http.StatusUnauthorized, sendReq("", ""))
	})
	t.Run("MD5/CachedLikeBcrypt", func(t *testing.T) {
		require.Equal(t, http.StatusOK, sendReq("md5", "md5"))
		require.Equal(t, http.StatusOK, sendReq("md5", "md5"))
		require.Equal(t, http.StatusUnauthorized, sendReq("md5", "wrongpassword"))
	})
}

func TestMiddlewareAuthBcryptCachePasswordChange(t *testing.T) {
	// bcrypt hash of "newpassword" at cost 4 (low cost for test speed)
	const newHash = "$2a$04$xWq7e2r0F01c5vAQd/pAf.E/28KCJTp4vNZzsPc/eDhBNwUi4FOjO"
	// original bcrypt hash of "bcrypt" from testdata/.htpasswd
	const origHash = "$2y$10$K/b3mVXUA6X857TOTYIL9.Lbaeg9oBjMQwUX5NefpVUCcYP0Z5KY2"

	tmpDir, err := os.MkdirTemp("", "htpasswd-test-*")
	require.NoError(t, err)
	htpasswd := filepath.Join(tmpDir, ".htpasswd")
	writeHtpasswd := func(hash string) {
		require.NoError(t, os.WriteFile(htpasswd, []byte("user:"+hash+"\n"), 0600))
	}

	writeHtpasswd(origHash)
	s, err := NewServer(context.Background(),
		WithConfig(Config{ListenAddr: []string{"127.0.0.1:0"}}),
		WithAuth(AuthConfig{Realm: "test", HtPasswd: htpasswd}),
	)
	require.NoError(t, err)
	// Shutdown before TempDir cleanup so goauth releases the file handle on Windows.
	t.Cleanup(func() {
		require.NoError(t, s.Shutdown())
		require.NoError(t, os.RemoveAll(tmpDir))
	})
	s.Router().Mount("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	s.Serve()
	url := testGetServerURL(t, s)
	client := &http.Client{}

	sendReq := func(pass string) int {
		req, err := http.NewRequest("GET", url, nil)
		require.NoError(t, err)
		req.SetBasicAuth("user", pass)
		resp, err := client.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		return resp.StatusCode
	}

	require.Equal(t, http.StatusOK, sendReq("bcrypt"), "original password should work")
	require.Equal(t, http.StatusOK, sendReq("bcrypt"), "cache hit should work")

	writeHtpasswd(newHash)

	require.Equal(t, http.StatusUnauthorized, sendReq("bcrypt"), "old password must be rejected after file change")
	require.Equal(t, http.StatusOK, sendReq("newpassword"), "new password must work after file change")
}

func newHandler() http.Handler {
	secretProvider := goauth.HtpasswdFileProvider("./testdata/.htpasswd")
	authenticator := NewLoggedBasicAuthenticator("test", secretProvider)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return basicAuth(authenticator)(next)
}

func BenchmarkBcryptAuthCacheMiss(b *testing.B) {
	h := newHandler()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		r.SetBasicAuth("bcrypt", fmt.Sprintf("wrongpassword%d", i))
		h.ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized {
			b.Fatalf("expected 401, got %d", w.Code)
		}
	}
}

func BenchmarkBcryptAuthCacheHit(b *testing.B) {
	h := newHandler()
	warmup := httptest.NewRequest("GET", "/", nil)
	warmup.SetBasicAuth("bcrypt", "bcrypt")
	h.ServeHTTP(httptest.NewRecorder(), warmup)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		r.SetBasicAuth("bcrypt", "bcrypt")
		h.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			b.Fatalf("expected 200, got %d", w.Code)
		}
	}
}

func BenchmarkMD5AuthCacheHit(b *testing.B) {
	h := newHandler()
	warmup := httptest.NewRequest("GET", "/", nil)
	warmup.SetBasicAuth("md5", "md5")
	h.ServeHTTP(httptest.NewRecorder(), warmup)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		r.SetBasicAuth("md5", "md5")
		h.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			b.Fatalf("expected 200, got %d", w.Code)
		}
	}
}
