package http

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
				CustomAuthFn: func(r *http.Request, user, pass string) (value any, err error) {
					if user == "custom" && pass == "custom" && r.RemoteAddr != "" {
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

			// OPTIONS is not exempt from authentication: a handler
			// may reveal information about the path in its answer
			t.Run("NoCredsOPTIONS", func(t *testing.T) {
				client := &http.Client{}
				req, err := http.NewRequest("OPTIONS", url, nil)
				require.NoError(t, err)

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer func() {
					_ = resp.Body.Close()
				}()

				require.Equal(t, http.StatusUnauthorized, resp.StatusCode, "OPTIONS with no creds should return unauthorized")
			})

			// A CORS preflight is only exempt when CORS is enabled
			// which it isn't for these servers
			t.Run("NoCredsPreflight", func(t *testing.T) {
				client := &http.Client{}
				req, err := http.NewRequest("OPTIONS", url, nil)
				require.NoError(t, err)
				req.Header.Set("Origin", "http://test.rclone.org")
				req.Header.Set("Access-Control-Request-Method", "GET")

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer func() {
					_ = resp.Body.Close()
				}()

				require.Equal(t, http.StatusUnauthorized, resp.StatusCode, "preflight with no creds and no CORS should return unauthorized")
			})
		})
	}
}

// A request arriving over a plain HTTP listener has no TLS state so it can't
// carry a client certificate and must be rejected.
func TestMiddlewareAuthCertificateUserNoTLS(t *testing.T) {
	handler := MiddlewareAuthCertificateUser()(testEchoHandler([]byte("ok")))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "http://example.com/", nil))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
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
				CustomAuthFn: func(_ *http.Request, user, pass string) (value any, err error) {
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
				CustomAuthFn: func(_ *http.Request, user, pass string) (value any, err error) {
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
			name: "Basic",
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
		{
			name: "Custom",
			http: Config{
				ListenAddr:  []string{"127.0.0.1:0"},
				AllowOrigin: "http://test.rclone.org",
			},
			auth: AuthConfig{
				Realm: "test",
				CustomAuthFn: func(r *http.Request, user, pass string) (value any, err error) {
					if user == "test_user" && pass == "test_pass" {
						return true, nil
					}
					return nil, errors.New("invalid credentials")
				},
			},
		},
		{
			name: "UserFromHeader",
			http: Config{
				ListenAddr:  []string{"127.0.0.1:0"},
				AllowOrigin: "http://test.rclone.org",
			},
			auth: AuthConfig{
				Realm:          "test",
				UserFromHeader: "X-Remote-User",
			},
		},
	}

	for _, ss := range authServers {
		t.Run(ss.name, func(t *testing.T) {
			s, err := NewServer(context.Background(), WithConfig(ss.http), WithAuth(ss.auth))
			require.NoError(t, err)
			defer func() {
				require.NoError(t, s.Shutdown())
			}()

			s.Router().Mount("/", testEchoHandler([]byte("secret-page")))
			s.Serve()

			url := testGetServerURL(t, s)

			doOPTIONS := func(t *testing.T, setup func(req *http.Request)) *http.Response {
				req, err := http.NewRequest("OPTIONS", url, nil)
				require.NoError(t, err)
				if setup != nil {
					setup(req)
				}
				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				t.Cleanup(func() {
					_ = resp.Body.Close()
				})
				return resp
			}

			preflight := func(req *http.Request) {
				req.Header.Set("Origin", ss.http.AllowOrigin)
				req.Header.Set("Access-Control-Request-Method", "GET")
			}

			// checkPreflight checks resp was answered by the CORS
			// middleware and never reached the handler behind it
			checkPreflight := func(t *testing.T, resp *http.Response) {
				require.Equal(t, http.StatusOK, resp.StatusCode, "preflight OPTIONS should return ok")
				testExpectRespBody(t, resp, []byte{})
				for _, key := range _testCORSHeaderKeys {
					require.Contains(t, resp.Header, key, "CORS headers should be sent")
				}
				require.Equal(t, ss.http.AllowOrigin, resp.Header.Get("Access-Control-Allow-Origin"), "allow origin should match")
			}

			// A browser preflight can't carry credentials so it
			// must succeed without them
			t.Run("PreflightNoCreds", func(t *testing.T) {
				checkPreflight(t, doOPTIONS(t, preflight))
			})

			// A proxy in front of the browser may stamp an
			// Authorization header the server can't check onto the
			// preflight, which mustn't stop it succeeding
			t.Run("PreflightIgnoresAuthorization", func(t *testing.T) {
				checkPreflight(t, doOPTIONS(t, func(req *http.Request) {
					preflight(req)
					req.Header.Set("Authorization", "Bearer not-checked-here")
				}))
				checkPreflight(t, doOPTIONS(t, func(req *http.Request) {
					preflight(req)
					req.SetBasicAuth("test_user", "wrong")
				}))
				checkPreflight(t, doOPTIONS(t, func(req *http.Request) {
					preflight(req)
					req.SetBasicAuth("test_user", "test_pass")
				}))
			})

			// Any other OPTIONS is authenticated like any other
			// request as the handler's answer may reveal
			// information about the path
			t.Run("OPTIONSNoCreds", func(t *testing.T) {
				resp := doOPTIONS(t, nil)
				require.Equal(t, http.StatusUnauthorized, resp.StatusCode, "OPTIONS with no creds should return unauthorized")
			})

			t.Run("OPTIONSGoodCreds", func(t *testing.T) {
				resp := doOPTIONS(t, func(req *http.Request) {
					if ss.auth.UserFromHeader != "" {
						req.Header.Set(ss.auth.UserFromHeader, "test_user")
					} else {
						req.SetBasicAuth("test_user", "test_pass")
					}
				})
				require.Equal(t, http.StatusOK, resp.StatusCode, "OPTIONS with good creds should return ok")
				testExpectRespBody(t, resp, []byte("secret-page"))
			})
		})
	}
}

func TestMiddlewareResponseHeaders(t *testing.T) {
	servers := []struct {
		name   string
		http   Config
		header http.Header
	}{
		{
			name: "SingleHeader",
			http: Config{
				ListenAddr:      []string{"127.0.0.1:0"},
				ResponseHeaders: []string{"X-Test-Header: test-value"},
			},
			header: http.Header{
				"X-Test-Header": []string{"test-value"},
			},
		},
		{
			name: "MultipleHeaders",
			http: Config{
				ListenAddr:      []string{"127.0.0.1:0"},
				ResponseHeaders: []string{"X-Header-One: one", "X-Header-Two: two"},
			},
			header: http.Header{
				"X-Header-One": []string{"one"},
				"X-Header-Two": []string{"two"},
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

			expected := []byte("header-test")
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

			for key, vals := range ss.header {
				require.Contains(t, resp.Header, key, "response should contain custom header")
				require.Equal(t, vals, resp.Header.Values(key), "header value should match")
			}
		})
	}
}
