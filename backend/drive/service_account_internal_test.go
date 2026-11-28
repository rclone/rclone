package drive

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testServiceAccount = "sa-extract@example-project.iam.gserviceaccount.com"
	testImpersonate    = "test.user@example.org"
)

// fakeGoogle stands in for the metadata server (Application Default
// Credentials), the IAM Credentials API and the OAuth token endpoint, so the
// keyless flow can be run without reaching Google.
type fakeGoogle struct {
	mu         sync.Mutex
	iamStatus  int
	signAuth   []string
	signPath   []string
	claims     []map[string]any
	assertions []string
}

func (g *fakeGoogle) serve(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch {
	case r.Host == "iamcredentials.googleapis.com" && strings.HasSuffix(r.URL.Path, ":signJwt"):
		var req struct {
			Payload string `json:"payload"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var claims map[string]any
		_ = json.Unmarshal([]byte(req.Payload), &claims)
		g.signAuth = append(g.signAuth, r.Header.Get("Authorization"))
		g.signPath = append(g.signPath, r.URL.Path)
		g.claims = append(g.claims, claims)
		if g.iamStatus != http.StatusOK {
			http.Error(w, `{"error":{"message":"refused"}}`, g.iamStatus)
			return
		}
		_, _ = w.Write([]byte(`{"keyId":"k1","signedJwt":"signed.jwt.value"}`))
	case r.Host == "oauth2.googleapis.com" && r.URL.Path == "/token":
		_ = r.ParseForm()
		g.assertions = append(g.assertions, r.PostForm.Get("assertion"))
		_, _ = w.Write([]byte(`{"access_token":"delegated-token","token_type":"Bearer","expires_in":3600}`))
	default:
		http.NotFound(w, r)
	}
}

// newFakeGoogle redirects every outgoing HTTPS connection made through
// http.DefaultTransport to the fake and makes Application Default Credentials
// come from a fake metadata server. It restores both when the test ends.
func newFakeGoogle(t *testing.T) *fakeGoogle {
	t.Helper()
	g := &fakeGoogle{iamStatus: http.StatusOK}

	tlsServer := httptest.NewTLSServer(http.HandlerFunc(g.serve))
	t.Cleanup(tlsServer.Close)
	pool := x509.NewCertPool()
	pool.AddCert(tlsServer.Certificate())
	addr := tlsServer.Listener.Addr().String()

	orig := http.DefaultTransport
	tr := orig.(*http.Transport).Clone()
	tr.Proxy = nil
	tr.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}
	// The test server's certificate is valid for example.com, not for Google's hosts.
	tr.TLSClientConfig = &tls.Config{RootCAs: pool, ServerName: "example.com"}
	http.DefaultTransport = tr
	t.Cleanup(func() { http.DefaultTransport = orig })

	metadata := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Metadata-Flavor") != "Google" {
			http.Error(w, "missing Metadata-Flavor", http.StatusForbidden)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/token") {
			_, _ = w.Write([]byte(`{"access_token":"adc-token","token_type":"Bearer","expires_in":3600}`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(metadata.Close)
	t.Setenv("GCE_METADATA_HOST", strings.TrimPrefix(metadata.URL, "http://"))
	// Keep any real Application Default Credentials on this machine out of the test.
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	t.Setenv("CLOUDSDK_CONFIG", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	return g
}

// newTarget returns a server that records the Authorization header of the
// requests it gets, standing in for the Drive API.
func newTarget(t *testing.T) (url string, auths func() []string) {
	t.Helper()
	var mu sync.Mutex
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = append(got, r.Header.Get("Authorization"))
		mu.Unlock()
	}))
	t.Cleanup(srv.Close)
	return srv.URL, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), got...)
	}
}

func keylessDescriptor() []byte {
	return []byte(`{"type":"service_account","client_email":"` + testServiceAccount + `"}`)
}

func get(t *testing.T, client *http.Client, url string) error {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

// A descriptor with a client_email and no private_key signs through IAM as the
// service account, with the impersonated user as the subject and only the
// requested scope, using Application Default Credentials as the caller.
func TestKeylessServiceAccountDelegation(t *testing.T) {
	g := newFakeGoogle(t)
	target, auths := newTarget(t)

	client, err := getServiceAccountClient(context.Background(),
		&Options{Scope: "drive.readonly", Impersonate: testImpersonate}, keylessDescriptor())
	require.NoError(t, err)

	require.NoError(t, get(t, client, target))
	require.NoError(t, get(t, client, target))

	g.mu.Lock()
	defer g.mu.Unlock()
	require.Len(t, g.claims, 1, "the token is reused, not signed again for every request")
	assert.Equal(t, "/v1/projects/-/serviceAccounts/"+testServiceAccount+":signJwt", g.signPath[0])
	assert.Equal(t, "Bearer adc-token", g.signAuth[0], "Application Default Credentials call IAM")
	assert.Equal(t, testServiceAccount, g.claims[0]["iss"])
	assert.Equal(t, testImpersonate, g.claims[0]["sub"])
	assert.Equal(t, "https://www.googleapis.com/auth/drive.readonly", g.claims[0]["scope"])
	assert.Equal(t, "https://oauth2.googleapis.com/token", g.claims[0]["aud"])
	assert.Equal(t, []string{"signed.jwt.value"}, g.assertions, "the IAM-signed JWT is what gets exchanged")
	assert.Equal(t, []string{"Bearer delegated-token", "Bearer delegated-token"}, auths())
}

// Without a subject the token would be the service account's own, which has no
// access to a user's Drive, so the keyless path refuses to start.
func TestKeylessServiceAccountNeedsImpersonate(t *testing.T) {
	_, err := getServiceAccountClient(context.Background(), &Options{}, keylessDescriptor())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--drive-impersonate")
}

// If IAM refuses to sign (missing roles/iam.serviceAccountTokenCreator, say) the
// request must fail with that error and must not go out without credentials.
func TestKeylessServiceAccountIAMRefusalFailsRequest(t *testing.T) {
	g := newFakeGoogle(t)
	g.iamStatus = http.StatusForbidden
	target, auths := newTarget(t)

	client, err := getServiceAccountClient(context.Background(),
		&Options{Impersonate: testImpersonate}, keylessDescriptor())
	require.NoError(t, err)

	err = get(t, client, target)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status code 403")
	assert.Empty(t, auths(), "nothing is sent to Drive without a token")
}

// A descriptor that has a private_key keeps the existing key-based path: it
// needs no --drive-impersonate and makes no IAM call when the client is built.
func TestServiceAccountWithKeyIsUnchanged(t *testing.T) {
	g := newFakeGoogle(t)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	descriptor, err := json.Marshal(map[string]string{
		"type":         "service_account",
		"client_email": testServiceAccount,
		"private_key":  string(pemKey),
		"token_uri":    "https://oauth2.googleapis.com/token",
	})
	require.NoError(t, err)

	client, err := getServiceAccountClient(context.Background(), &Options{}, descriptor)
	require.NoError(t, err)
	assert.NotNil(t, client)

	g.mu.Lock()
	defer g.mu.Unlock()
	assert.Empty(t, g.claims)
}

// Data that is not a service account descriptor goes to the existing parser and
// gets its error, as before, even when it has a client_email.
func TestServiceAccountInvalidDescriptorKeepsExistingError(t *testing.T) {
	for _, tc := range []struct {
		name string
		data string
	}{
		{"not JSON", `not json`},
		{"empty object", `{}`},
		{"no type", `{"client_email":"` + testServiceAccount + `"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := getServiceAccountClient(context.Background(),
				&Options{Impersonate: testImpersonate}, []byte(tc.data))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "error processing credentials")
		})
	}
}

// Without a client_email there is no service account to sign as, so the
// descriptor is not keyless and is left to the existing parser.
func TestServiceAccountWithoutClientEmailIsNotKeyless(t *testing.T) {
	_, err := getServiceAccountClient(context.Background(), &Options{}, []byte(`{"type":"service_account"}`))
	assert.NoError(t, err, "the keyless branch would have asked for --drive-impersonate")
}
