package fshttp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanAuth(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
	}{
		{"", ""},
		{"floo", "floo"},
		{"Authorization: ", "Authorization: "},
		{"Authorization: \n", "Authorization: \n"},
		{"Authorization: A", "Authorization: X"},
		{"Authorization: A\n", "Authorization: X\n"},
		{"Authorization: AAAA", "Authorization: XXXX"},
		{"Authorization: AAAA\n", "Authorization: XXXX\n"},
		{"Authorization: AAAAA", "Authorization: XXXX"},
		{"Authorization: AAAAA\n", "Authorization: XXXX\n"},
		{"Authorization: AAAA\n", "Authorization: XXXX\n"},
		{"Authorization: AAAAAAAAA\nPotato: Help\n", "Authorization: XXXX\nPotato: Help\n"},
		{"Sausage: 1\nAuthorization: AAAAAAAAA\nPotato: Help\n", "Sausage: 1\nAuthorization: XXXX\nPotato: Help\n"},
	} {
		got := string(cleanAuth([]byte(test.in), authBufs[0]))
		assert.Equal(t, test.want, got, test.in)
	}
}

func TestCleanAuths(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
	}{
		{"", ""},
		{"floo", "floo"},
		{"Authorization: AAAAAAAAA\nPotato: Help\n", "Authorization: XXXX\nPotato: Help\n"},
		{"X-Auth-Token: AAAAAAAAA\nPotato: Help\n", "X-Auth-Token: XXXX\nPotato: Help\n"},
		{"X-Auth-Token: AAAAAAAAA\nAuthorization: AAAAAAAAA\nPotato: Help\n", "X-Auth-Token: XXXX\nAuthorization: XXXX\nPotato: Help\n"},
	} {
		got := string(cleanAuths([]byte(test.in)))
		assert.Equal(t, test.want, got, test.in)
	}
}

var certSerial = int64(0)

// Create a test certificate and key pair that is valid for a specific
// duration
func createTestCert(validity time.Duration) (keyPEM []byte, certPEM []byte, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return
	}
	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	// PEM encoding of private key
	keyPEM = pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: keyBytes,
		},
	)

	// Now create the certificate
	notBefore := time.Now()
	notAfter := notBefore.Add(validity).Add(expireWindow)

	certSerial += 1
	template := x509.Certificate{
		SerialNumber:          big.NewInt(certSerial),
		Subject:               pkix.Name{CommonName: "localhost"},
		SignatureAlgorithm:    x509.SHA256WithRSA,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement | x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return
	}

	certPEM = pem.EncodeToMemory(
		&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: derBytes,
		},
	)
	return
}

func writeTestCert(t *testing.T, ci *fs.ConfigInfo, validity time.Duration) {
	keyPEM, certPEM, err := createTestCert(1 * time.Second)
	assert.NoError(t, err, "Cannot create test cert")
	err = os.WriteFile(ci.ClientCert, certPEM, 0666)
	assert.NoError(t, err, "Failed to write cert")
	err = os.WriteFile(ci.ClientKey, keyPEM, 0666)
	assert.NoError(t, err, "Failed to write key")
}

func TestCertificates(t *testing.T) {
	startTime := time.Now()
	// Starting a TLS server
	expectedSerial := int64(0)
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cert := r.TLS.PeerCertificates
		require.Greater(t, len(cert), 0, "No certificates received")
		expectedSerial += 1
		assert.Equal(t, expectedSerial, cert[0].SerialNumber.Int64(), "Did not get the correct serial number in certificate")
		// Check that the certificate hasn't expired. We cannot use cert validation
		// functions because those check for signature as well and our certificates
		// are not properly signed
		if time.Now().After(cert[0].NotAfter) {
			assert.Fail(t, "Certificate expired", "Certificate expires at %s, current time is %s", cert[0].NotAfter.Sub(startTime), time.Since(startTime))
		}

		// Write some test data to fullfil the request
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprintln(w, "test data")
	}))
	defer ts.Close()
	// Modify servers config to request a client certificate
	// we cannot validate the certificate since we are not properly signing it
	ts.TLS.ClientAuth = tls.RequestClientCert

	// Set --client-cert and --client-key in config to
	// a pair of temp files
	// create a test cert/key pair and write it to the files
	ctx := context.TODO()
	ci := fs.GetConfig(ctx)
	// Create a test certificate and write it to a temp file
	ci.ClientCert = t.TempDir() + "client.cert"
	ci.ClientKey = t.TempDir() + "client.key"
	validity := 1 * time.Second
	writeTestCert(t, ci, validity)

	// Now create the client with the above settings
	// we need to disable TLS verification since we don't
	// care about server certificate
	client := NewClient(ctx)
	tt := client.Transport.(*Transport)
	tt.TLSClientConfig.InsecureSkipVerify = true

	// Now make requests, the first request should be within
	// the valid window
	_, err := client.Get(ts.URL)
	assert.NoError(t, err)

	// Wait for the 2* valid duration of the certificate so that has definitely expired
	time.Sleep(2 * validity)

	// Create a new cert and write it to files
	writeTestCert(t, ci, validity)

	// The new cert should be auto-loaded before we make this request
	_, err = client.Get(ts.URL)
	assert.NoError(t, err)
}
