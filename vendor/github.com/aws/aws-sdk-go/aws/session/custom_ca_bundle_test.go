package session

import (
	"bytes"
	"crypto/tls"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/stretchr/testify/assert"
)

func createTLSServer(cert, key []byte, done <-chan struct{}) (*httptest.Server, error) {
	c, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}

	s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	s.TLS = &tls.Config{
		Certificates: []tls.Certificate{c},
	}
	s.TLS.BuildNameToCertificate()
	s.StartTLS()

	go func() {
		<-done
		s.Close()
	}()

	return s, nil
}

func setupTestCAFile(b []byte) (string, error) {
	bundleFile, err := ioutil.TempFile(os.TempDir(), "aws-sdk-go-session-test")
	if err != nil {
		return "", err
	}

	_, err = bundleFile.Write(b)
	if err != nil {
		return "", err
	}

	defer bundleFile.Close()
	return bundleFile.Name(), nil
}

func TestNewSession_WithCustomCABundle_Env(t *testing.T) {
	oldEnv := initSessionTestEnv()
	defer popEnv(oldEnv)

	done := make(chan struct{})
	server, err := createTLSServer(testTLSBundleCert, testTLSBundleKey, done)
	assert.NoError(t, err)

	// Write bundle to file
	caFilename, err := setupTestCAFile(testTLSBundleCA)
	defer func() {
		os.Remove(caFilename)
	}()
	assert.NoError(t, err)

	os.Setenv("AWS_CA_BUNDLE", caFilename)

	s, err := NewSession(&aws.Config{
		HTTPClient:  &http.Client{},
		Endpoint:    aws.String(server.URL),
		Region:      aws.String("mock-region"),
		Credentials: credentials.AnonymousCredentials,
	})
	assert.NoError(t, err)
	assert.NotNil(t, s)

	req, _ := http.NewRequest("GET", *s.Config.Endpoint, nil)
	resp, err := s.Config.HTTPClient.Do(req)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestNewSession_WithCustomCABundle_EnvNotExists(t *testing.T) {
	oldEnv := initSessionTestEnv()
	defer popEnv(oldEnv)

	os.Setenv("AWS_CA_BUNDLE", "file-not-exists")

	s, err := NewSession()
	assert.Error(t, err)
	assert.Equal(t, "LoadCustomCABundleError", err.(awserr.Error).Code())
	assert.Nil(t, s)
}

func TestNewSession_WithCustomCABundle_Option(t *testing.T) {
	oldEnv := initSessionTestEnv()
	defer popEnv(oldEnv)

	done := make(chan struct{})
	server, err := createTLSServer(testTLSBundleCert, testTLSBundleKey, done)
	assert.NoError(t, err)

	s, err := NewSessionWithOptions(Options{
		Config: aws.Config{
			HTTPClient:  &http.Client{},
			Endpoint:    aws.String(server.URL),
			Region:      aws.String("mock-region"),
			Credentials: credentials.AnonymousCredentials,
		},
		CustomCABundle: bytes.NewReader(testTLSBundleCA),
	})
	assert.NoError(t, err)
	assert.NotNil(t, s)

	req, _ := http.NewRequest("GET", *s.Config.Endpoint, nil)
	resp, err := s.Config.HTTPClient.Do(req)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestNewSession_WithCustomCABundle_OptionPriority(t *testing.T) {
	oldEnv := initSessionTestEnv()
	defer popEnv(oldEnv)

	done := make(chan struct{})
	server, err := createTLSServer(testTLSBundleCert, testTLSBundleKey, done)
	assert.NoError(t, err)

	os.Setenv("AWS_CA_BUNDLE", "file-not-exists")

	s, err := NewSessionWithOptions(Options{
		Config: aws.Config{
			HTTPClient:  &http.Client{},
			Endpoint:    aws.String(server.URL),
			Region:      aws.String("mock-region"),
			Credentials: credentials.AnonymousCredentials,
		},
		CustomCABundle: bytes.NewReader(testTLSBundleCA),
	})
	assert.NoError(t, err)
	assert.NotNil(t, s)

	req, _ := http.NewRequest("GET", *s.Config.Endpoint, nil)
	resp, err := s.Config.HTTPClient.Do(req)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

type mockRoundTripper struct{}

func (m *mockRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return nil, nil
}

func TestNewSession_WithCustomCABundle_UnsupportedTransport(t *testing.T) {
	oldEnv := initSessionTestEnv()
	defer popEnv(oldEnv)

	s, err := NewSessionWithOptions(Options{
		Config: aws.Config{
			HTTPClient: &http.Client{
				Transport: &mockRoundTripper{},
			},
		},
		CustomCABundle: bytes.NewReader(testTLSBundleCA),
	})
	assert.Error(t, err)
	assert.Equal(t, "LoadCustomCABundleError", err.(awserr.Error).Code())
	assert.Contains(t, err.(awserr.Error).Message(), "transport unsupported type")
	assert.Nil(t, s)
}

func TestNewSession_WithCustomCABundle_TransportSet(t *testing.T) {
	oldEnv := initSessionTestEnv()
	defer popEnv(oldEnv)

	done := make(chan struct{})
	server, err := createTLSServer(testTLSBundleCert, testTLSBundleKey, done)
	assert.NoError(t, err)

	s, err := NewSessionWithOptions(Options{
		Config: aws.Config{
			Endpoint:    aws.String(server.URL),
			Region:      aws.String("mock-region"),
			Credentials: credentials.AnonymousCredentials,
			HTTPClient: &http.Client{
				Transport: &http.Transport{
					Proxy: http.ProxyFromEnvironment,
					Dial: (&net.Dialer{
						Timeout:   30 * time.Second,
						KeepAlive: 30 * time.Second,
						DualStack: true,
					}).Dial,
					TLSHandshakeTimeout: 2 * time.Second,
				},
			},
		},
		CustomCABundle: bytes.NewReader(testTLSBundleCA),
	})
	assert.NoError(t, err)
	assert.NotNil(t, s)

	req, _ := http.NewRequest("GET", *s.Config.Endpoint, nil)
	resp, err := s.Config.HTTPClient.Do(req)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

/* Cert generation steps
# Create the CA key
openssl genrsa -des3 -out ca.key 1024

# Create the CA Cert
openssl req -new -sha256 -x509 -days 3650 \
    -subj "/C=GO/ST=Gopher/O=Testing ROOT CA" \
    -key ca.key -out ca.crt

# Create config
cat > csr_details.txt <<-EOF

[req]
default_bits = 1024
prompt = no
default_md = sha256
req_extensions = SAN
distinguished_name = dn

[ dn ]
C=GO
ST=Gopher
O=Testing Certificate
OU=Testing IP

[SAN]
subjectAltName = IP:127.0.0.1
EOF

# Create certificate signing request
openssl req -new -sha256 -nodes -newkey rsa:1024 \
    -config <( cat csr_details.txt ) \
    -keyout ia.key -out ia.csr

# Create a signed certificate
openssl x509 -req -days 3650 \
    -CAcreateserial \
    -extfile <( cat csr_details.txt ) \
    -extensions SAN \
    -CA ca.crt -CAkey ca.key -in ia.csr -out ia.crt

# Verify
openssl req -noout -text -in ia.csr
openssl x509 -noout -text -in ia.crt
*/
var (
	// ca.crt
	testTLSBundleCA = []byte(`-----BEGIN CERTIFICATE-----
MIICiTCCAfKgAwIBAgIJAJ5X1olt05XjMA0GCSqGSIb3DQEBCwUAMDgxCzAJBgNV
BAYTAkdPMQ8wDQYDVQQIEwZHb3BoZXIxGDAWBgNVBAoTD1Rlc3RpbmcgUk9PVCBD
QTAeFw0xNzAzMDkwMDAyMDZaFw0yNzAzMDcwMDAyMDZaMDgxCzAJBgNVBAYTAkdP
MQ8wDQYDVQQIEwZHb3BoZXIxGDAWBgNVBAoTD1Rlc3RpbmcgUk9PVCBDQTCBnzAN
BgkqhkiG9w0BAQEFAAOBjQAwgYkCgYEAw/8DN+t9XQR60jx42rsQ2WE2Dx85rb3n
GQxnKZZLNddsT8rDyxJNP18aFalbRbFlyln5fxWxZIblu9Xkm/HRhOpbSimSqo1y
uDx21NVZ1YsOvXpHby71jx3gPrrhSc/t/zikhi++6D/C6m1CiIGuiJ0GBiJxtrub
UBMXT0QtI2ECAwEAAaOBmjCBlzAdBgNVHQ4EFgQU8XG3X/YHBA6T04kdEkq6+4GV
YykwaAYDVR0jBGEwX4AU8XG3X/YHBA6T04kdEkq6+4GVYymhPKQ6MDgxCzAJBgNV
BAYTAkdPMQ8wDQYDVQQIEwZHb3BoZXIxGDAWBgNVBAoTD1Rlc3RpbmcgUk9PVCBD
QYIJAJ5X1olt05XjMAwGA1UdEwQFMAMBAf8wDQYJKoZIhvcNAQELBQADgYEAeILv
z49+uxmPcfOZzonuOloRcpdvyjiXblYxbzz6ch8GsE7Q886FTZbvwbgLhzdwSVgG
G8WHkodDUsymVepdqAamS3f8PdCUk8xIk9mop8LgaB9Ns0/TssxDvMr3sOD2Grb3
xyWymTWMcj6uCiEBKtnUp4rPiefcvCRYZ17/hLE=
-----END CERTIFICATE-----
`)

	// ai.crt
	testTLSBundleCert = []byte(`-----BEGIN CERTIFICATE-----
MIICGjCCAYOgAwIBAgIJAIIu+NOoxxM0MA0GCSqGSIb3DQEBBQUAMDgxCzAJBgNV
BAYTAkdPMQ8wDQYDVQQIEwZHb3BoZXIxGDAWBgNVBAoTD1Rlc3RpbmcgUk9PVCBD
QTAeFw0xNzAzMDkwMDAzMTRaFw0yNzAzMDcwMDAzMTRaMFExCzAJBgNVBAYTAkdP
MQ8wDQYDVQQIDAZHb3BoZXIxHDAaBgNVBAoME1Rlc3RpbmcgQ2VydGlmaWNhdGUx
EzARBgNVBAsMClRlc3RpbmcgSVAwgZ8wDQYJKoZIhvcNAQEBBQADgY0AMIGJAoGB
AN1hWHeioo/nASvbrjwCQzXCiWiEzGkw353NxsAB54/NqDL3LXNATtiSJu8kJBrm
Ah12IFLtWLGXjGjjYlHbQWnOR6awveeXnQZukJyRWh7m/Qlt9Ho0CgZE1U+832ac
5GWVldNxW1Lz4I+W9/ehzqe8I80RS6eLEKfUFXGiW+9RAgMBAAGjEzARMA8GA1Ud
EQQIMAaHBH8AAAEwDQYJKoZIhvcNAQEFBQADgYEAdF4WQHfVdPCbgv9sxgJjcR1H
Hgw9rZ47gO1IiIhzglnLXQ6QuemRiHeYFg4kjcYBk1DJguxzDTGnUwhUXOibAB+S
zssmrkdYYvn9aUhjc3XK3tjAoDpsPpeBeTBamuUKDHoH/dNRXxerZ8vu6uPR3Pgs
5v/KCV6IAEcvNyOXMPo=
-----END CERTIFICATE-----
`)

	// ai.key
	testTLSBundleKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIICXAIBAAKBgQDdYVh3oqKP5wEr2648AkM1wolohMxpMN+dzcbAAeePzagy9y1z
QE7YkibvJCQa5gIddiBS7Vixl4xo42JR20FpzkemsL3nl50GbpCckVoe5v0JbfR6
NAoGRNVPvN9mnORllZXTcVtS8+CPlvf3oc6nvCPNEUunixCn1BVxolvvUQIDAQAB
AoGBAMISrcirddGrlLZLLrKC1ULS2T0cdkqdQtwHYn4+7S5+/z42vMx1iumHLsSk
rVY7X41OWkX4trFxhvEIrc/O48bo2zw78P7flTxHy14uxXnllU8cLThE29SlUU7j
AVBNxJZMsXMlS/DowwD4CjFe+x4Pu9wZcReF2Z9ntzMpySABAkEA+iWoJCPE2JpS
y78q3HYYgpNY3gF3JqQ0SI/zTNkb3YyEIUffEYq0Y9pK13HjKtdsSuX4osTIhQkS
+UgRp6tCAQJBAOKPYTfQ2FX8ijgUpHZRuEAVaxASAS0UATiLgzXxLvOh/VC2at5x
wjOX6sD65pPz/0D8Qj52Cq6Q1TQ+377SDVECQAIy0od+yPweXxvrUjUd1JlRMjbB
TIrKZqs8mKbUQapw0bh5KTy+O1elU4MRPS3jNtBxtP25PQnuSnxmZcFTgAECQFzg
DiiFcsn9FuRagfkHExMiNJuH5feGxeFaP9WzI144v9GAllrOI6Bm3JNzx2ZLlg4b
20Qju8lIEj6yr6JYFaECQHM1VSojGRKpOl9Ox/R4yYSA9RV5Gyn00/aJNxVYyPD5
i3acL2joQm2kLD/LO8paJ4+iQdRXCOMMIpjxSNjGQjQ=
-----END RSA PRIVATE KEY-----
`)
)
