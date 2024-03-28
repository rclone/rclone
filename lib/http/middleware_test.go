package http

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMiddlewareAuth(t *testing.T) {
	servers := []struct {
		name string
		http Config
		auth AuthConfig
		user string
		pass string
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
				CustomAuthFn: func(user, pass string) (value interface{}, err error) {
					if user == "custom" && pass == "custom" {
						return true, nil
					}
					return nil, errors.New("invalid credentials")
				},
			},
			user: "custom",
			pass: "custom",
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
			s.Router().Mount("/", testEchoHandler(expected))
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

				wwwAuthHeader := resp.Header.Get("WWW-Authenticate")
				require.NotEmpty(t, wwwAuthHeader, "resp should contain WWW-Authtentication header")
				require.Contains(t, wwwAuthHeader, fmt.Sprintf("realm=%q", ss.auth.Realm), "WWW-Authtentication header should contain relam")
			})

			t.Run("BadCreds", func(t *testing.T) {
				client := &http.Client{}
				req, err := http.NewRequest("GET", url, nil)
				require.NoError(t, err)

				req.SetBasicAuth(ss.user+"BAD", ss.pass+"BAD")

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer func() {
					_ = resp.Body.Close()
				}()

				require.Equal(t, http.StatusUnauthorized, resp.StatusCode, "using bad creds should return unauthorized")

				wwwAuthHeader := resp.Header.Get("WWW-Authenticate")
				require.NotEmpty(t, wwwAuthHeader, "resp should contain WWW-Authtentication header")
				require.Contains(t, wwwAuthHeader, fmt.Sprintf("realm=%q", ss.auth.Realm), "WWW-Authtentication header should contain relam")
			})

			t.Run("GoodCreds", func(t *testing.T) {
				client := &http.Client{}
				req, err := http.NewRequest("GET", url, nil)
				require.NoError(t, err)

				req.SetBasicAuth(ss.user, ss.pass)

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer func() {
					_ = resp.Body.Close()
				}()

				require.Equal(t, http.StatusOK, resp.StatusCode, "using good creds should return ok")

				testExpectRespBody(t, resp, expected)
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
				CustomAuthFn: func(user, pass string) (value interface{}, err error) {
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
				CustomAuthFn: func(user, pass string) (value interface{}, err error) {
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
