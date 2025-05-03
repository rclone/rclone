//go:build windows
// +build windows

package wincrypt_test

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"math/big"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/wincrypt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

//nolint:revive // Windows constants
const (
	CERT_NCRYPT_KEY_HANDLE_PROP_ID  uint32 = 78
	CERT_KEY_PROV_INFO_PROP_ID      uint32 = 2
	CERT_CLOSE_STORE_FORCE_FLAG     uint32 = 1
	NCRYPT_OVERWRITE_KEY_FLAG       uint32 = 0x00000080
	BCRYPT_RSAPRIVATE_MAGIC         uint32 = 0x32415352
	BCRYPT_ECDSA_PRIVATE_P256_MAGIC uint32 = 0x32534345
	BCRYPT_ECDSA_PRIVATE_P384_MAGIC uint32 = 0x34534345
	BCRYPT_ECDSA_PRIVATE_P521_MAGIC uint32 = 0x36534345
	NCRYPTBUFFER_PKCS_KEY_NAME      uint32 = 45
	BCRYPTBUFFER_VERSION            uint32 = 0
	NCRYPT_SILENT_FLAG              uint32 = windows.CRYPT_SILENT
)

//nolint:revive // Windows constants
var (
	MS_KEY_STORAGE_PROVIDER = [...]uint16{'M', 'i', 'c', 'r', 'o', 's', 'o', 'f', 't', ' ', 'S', 'o', 'f', 't', 'w', 'a', 'r', 'e', ' ', 'K', 'e', 'y', ' ', 'S', 't', 'o', 'r', 'a', 'g', 'e', ' ', 'P', 'r', 'o', 'v', 'i', 'd', 'e', 'r', 0}
	BCRYPT_RSAPRIVATE_BLOB  = [...]uint16{'R', 'S', 'A', 'P', 'R', 'I', 'V', 'A', 'T', 'E', 'B', 'L', 'O', 'B', 0}
	BCRYPT_ECCPRIVATE_BLOB  = [...]uint16{'E', 'C', 'C', 'P', 'R', 'I', 'V', 'A', 'T', 'E', 'B', 'L', 'O', 'B', 0}
)

var (
	procCertAddEncodedCertificateToStore  = wincrypt.Crypt32.NewProc("CertAddEncodedCertificateToStore")
	procCertDeleteCertificateFromStore    = wincrypt.Crypt32.NewProc("CertDeleteCertificateFromStore")
	procCertSetCertificateContextProperty = wincrypt.Crypt32.NewProc("CertSetCertificateContextProperty")
	procNCryptOpenStorageProvider         = wincrypt.NCrypt.NewProc("NCryptOpenStorageProvider")
	procNCryptImportKey                   = wincrypt.NCrypt.NewProc("NCryptImportKey")
	procNCryptDeleteKey                   = wincrypt.NCrypt.NewProc("NCryptDeleteKey")

	RNG = rand.New(rand.NewSource(time.Now().Unix()))

	TestHashers = []crypto.Hash{
		crypto.MD5,
		crypto.SHA1,
		crypto.SHA256,
		crypto.SHA384,
		crypto.SHA512,
	}
)

//nolint:revive // Windows structures
type (
	NCRYPT_KEY_HANDLE  uintptr
	NCRYPT_PROV_HANDLE uintptr

	BCryptBuffer struct {
		cbBuffer   uint32
		bufferType uint32
		pvBuffer   uintptr
	}

	BCryptBufferDesc struct {
		ulVersion uint32
		cBuffers  uint32
		pBuffers  uintptr
	}

	CRYPT_KEY_PROV_INFO struct {
		pwszContainerName *uint16
		pwszProvName      *uint16
		dwProvType        uint32
		dwFlags           uint32
		cProvParam        uint32
		rgProvParam       uintptr
		dwKeySpec         uint32
	}
)

type keyPair struct {
	cert       *x509.Certificate
	key        crypto.Signer
	certCtx    *syscall.CertContext
	privHandle NCRYPT_KEY_HANDLE
}

type testHash struct {
	algo crypto.Hash
	hash string
}

type testData struct {
	name       string
	isECDSA    bool
	rsaKeySize uint32         // if RSA
	curve      elliptic.Curve // if ECDSA
	hashes     []testHash
}

type testCert struct {
	data *testData
	pair *keyPair
}

func castTo[T any](ptr *T) uintptr {
	return uintptr(unsafe.Pointer(ptr))
}

func ncryptOpenStorageProvider(handle *NCRYPT_PROV_HANDLE, provider []uint16, flags uint32) (err error) {
	errno, _, _ := procNCryptOpenStorageProvider.Call(castTo(handle), castTo(&provider[0]), uintptr(flags))
	err = syscall.Errno(errno)
	if err == windows.ERROR_SUCCESS {
		err = nil
	}
	return err
}

func ncryptImportKey(prov NCRYPT_PROV_HANDLE, importKey NCRYPT_KEY_HANDLE, blobType []uint16, params uintptr, handle *NCRYPT_KEY_HANDLE, data []byte, flags uint32) (err error) {
	errno, _, _ := procNCryptImportKey.Call(uintptr(prov), uintptr(importKey), castTo(&blobType[0]), params, castTo(handle), castTo(&data[0]), uintptr(len(data)), uintptr(flags))
	err = syscall.Errno(errno)
	if err == windows.ERROR_SUCCESS {
		err = nil
	}
	return err
}

func ncryptDeleteKey(key NCRYPT_KEY_HANDLE, flags uint32) (err error) {
	errno, _, _ := procNCryptDeleteKey.Call(uintptr(key), uintptr(flags))
	err = syscall.Errno(errno)
	if err == windows.ERROR_SUCCESS {
		err = nil
	}
	return err
}

func getBCryptBlobType(priv crypto.Signer) ([]uint16, error) {

	if _, ok := priv.(*rsa.PrivateKey); ok {
		return BCRYPT_RSAPRIVATE_BLOB[:], nil
	}
	if eckey, ok := priv.(*ecdsa.PrivateKey); ok {
		switch eckey.Curve {
		case elliptic.P256():
			fallthrough
		case elliptic.P384():
			fallthrough
		case elliptic.P521():
			return BCRYPT_ECCPRIVATE_BLOB[:], nil
		}
	}
	return nil, errors.New("unsupported private key type")
}

func certAddEncodedCertificateToStore(store syscall.Handle, encodingType uint32, encoded []byte, addDisposition uint32) (ctx *syscall.CertContext, err error) {
	result, _, err := procCertAddEncodedCertificateToStore.Call(uintptr(store), uintptr(encodingType), castTo(&encoded[0]), uintptr(len(encoded)), uintptr(addDisposition), castTo(&ctx))
	if result != 0 {
		err = nil
	}
	return
}

func certDeleteCertificateFromStore(ctx *syscall.CertContext) (err error) {
	result, _, err := procCertDeleteCertificateFromStore.Call(castTo(ctx))
	if result != 0 {
		err = nil
	}
	return
}

func certSetCertificateContextProperty(ctx *syscall.CertContext, propID uint32, flags uint32, data uintptr) (err error) {
	result, _, err := procCertSetCertificateContextProperty.Call(castTo(ctx), uintptr(propID), uintptr(flags), data)
	if result != 0 {
		err = nil
	}
	return
}

func getX509CertHash(hasher hash.Hash, cert *x509.Certificate) string {
	hasher.Reset()
	hasher.Write(cert.Raw)
	return hex.EncodeToString(hasher.Sum(nil))
}

func makeCert(caCert *x509.Certificate, caPriv crypto.Signer, priv crypto.Signer, tmpl *x509.Certificate, validity time.Duration) (*x509.Certificate, error) {
	notBefore := time.Now().UTC()
	notAfter := notBefore.UTC().Add(validity)
	clientTmpl := x509.Certificate{
		SerialNumber:          tmpl.SerialNumber,
		Subject:               tmpl.Subject,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement | x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IPAddresses:           tmpl.IPAddresses,
		DNSNames:              tmpl.DNSNames,
	}
	derBytes, err := x509.CreateCertificate(RNG, &clientTmpl, caCert, priv.Public(), caPriv)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(derBytes)
}

func encodeBNBE(bn *big.Int, size int) []byte {
	data := make([]byte, size)
	return bn.FillBytes(data)
}

func toBCryptRSABlob(key *rsa.PrivateKey) ([]byte, error) {
	bnPubExp := big.NewInt(int64(key.PublicKey.E))
	pubExp := encodeBNBE(bnPubExp, len(bnPubExp.Bytes()))
	cbN := (key.N.BitLen() + 7) / 8
	N := encodeBNBE(key.N, cbN)
	Prime1 := encodeBNBE(key.Primes[0], len(key.Primes[0].Bytes()))
	Prime2 := encodeBNBE(key.Primes[1], len(key.Primes[1].Bytes()))
	// See BCRYPT_RSAKEY_BLOB structure and remarks in MS documentation
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.NativeEndian, BCRYPT_RSAPRIVATE_MAGIC)
	if err != nil {
		return nil, err
	}
	err = binary.Write(buf, binary.NativeEndian, uint32(key.N.BitLen()))
	if err != nil {
		return nil, err
	}
	err = binary.Write(buf, binary.NativeEndian, uint32(len(pubExp)))
	if err != nil {
		return nil, err
	}
	err = binary.Write(buf, binary.NativeEndian, uint32(cbN))
	if err != nil {
		return nil, err
	}
	err = binary.Write(buf, binary.NativeEndian, uint32(len(Prime1)))
	if err != nil {
		return nil, err
	}
	err = binary.Write(buf, binary.NativeEndian, uint32(len(Prime2)))
	if err != nil {
		return nil, err
	}
	// Endianness does not apply to []byte
	err = binary.Write(buf, binary.NativeEndian, pubExp)
	if err != nil {
		return nil, err
	}
	err = binary.Write(buf, binary.NativeEndian, N)
	if err != nil {
		return nil, err
	}
	err = binary.Write(buf, binary.NativeEndian, Prime1)
	if err != nil {
		return nil, err
	}
	err = binary.Write(buf, binary.NativeEndian, Prime2)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func toBCryptECCBlob(key *ecdsa.PrivateKey) ([]byte, error) {
	// See BCRYPT_ECCKEY_BLOB structure and remarks in MS documentation
	buf := new(bytes.Buffer)
	var err error
	switch key.Curve {
	case elliptic.P256():
		err = binary.Write(buf, binary.NativeEndian, BCRYPT_ECDSA_PRIVATE_P256_MAGIC)
	case elliptic.P384():
		err = binary.Write(buf, binary.NativeEndian, BCRYPT_ECDSA_PRIVATE_P384_MAGIC)
	case elliptic.P521():
		err = binary.Write(buf, binary.NativeEndian, BCRYPT_ECDSA_PRIVATE_P521_MAGIC)
	default:
		err = fmt.Errorf("unsupported ECDSA curve \"%s\"", key.Params().Name)
	}

	if err != nil {
		return nil, err
	}

	cbKey := (key.Params().BitSize + 7) / 8

	err = binary.Write(buf, binary.NativeEndian, uint32(cbKey))
	if err != nil {
		return nil, err
	}

	// Ditto
	err = binary.Write(buf, binary.NativeEndian, encodeBNBE(key.X, cbKey))
	if err != nil {
		return nil, err
	}
	err = binary.Write(buf, binary.NativeEndian, encodeBNBE(key.Y, cbKey))
	if err != nil {
		return nil, err
	}
	err = binary.Write(buf, binary.NativeEndian, encodeBNBE(key.D, cbKey))
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeBCryptPrivateKey(key crypto.Signer) ([]byte, error) {
	if k, ok := key.(*rsa.PrivateKey); ok {
		return toBCryptRSABlob(k)
	}
	if k, ok := key.(*ecdsa.PrivateKey); ok {
		return toBCryptECCBlob(k)
	}
	return nil, errors.New("unsupported key type")
}

var certSerial int64

func makeCACert(commonName string, serialNumber int64) (*x509.Certificate, crypto.Signer, error) {
	caPriv, err := rsa.GenerateKey(RNG, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate CA pricate key: %v", err)
	}
	caTmp := x509.Certificate{
		SerialNumber: big.NewInt(serialNumber),
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	caBytes, err := x509.CreateCertificate(RNG, &caTmp, &caTmp, &caPriv.PublicKey, caPriv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create and sign CA certificate: %v", err)
	}
	caCert, err := x509.ParseCertificate(caBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse created CA certificate: %v", err)
	}
	return caCert, caPriv, nil
}

func makeTestCerts(t *testing.T, data []*testCert, prov NCRYPT_PROV_HANDLE, store syscall.Handle) (*x509.Certificate, crypto.Signer) {
	caCert, caPriv, err := makeCACert("WinCryptTestClientCA", 0xCA1)
	require.NoError(t, err)
	for _, item := range data {
		certSerial += 1
		var privKey crypto.Signer
		if !item.data.isECDSA {
			privKey, err = rsa.GenerateKey(RNG, int(item.data.rsaKeySize))
		} else {
			privKey, err = ecdsa.GenerateKey(item.data.curve, RNG)
		}
		require.NoError(t, err)
		blobType, err := getBCryptBlobType(privKey)
		require.NoError(t, err)
		serial := big.NewInt(certSerial)
		clientCertTmpl := x509.Certificate{
			Subject: pkix.Name{
				CommonName:   item.data.name + "Cert",
				SerialNumber: fmt.Sprintf("%X", serial),
			},
			SerialNumber: serial,
		}
		clientCert, err := makeCert(caCert, caPriv, privKey, &clientCertTmpl, 60*time.Second)
		require.NoError(t, err)
		cngPrivBytes, err := encodeBCryptPrivateKey(privKey)
		require.NoError(t, err)

		// Prepare key container name
		hasher := crypto.SHA1.New()
		hasher.Reset()
		hasher.Write(cngPrivBytes)
		name, err := windows.UTF16FromString(strings.ToUpper(hex.EncodeToString(hasher.Sum(nil))))
		require.NoError(t, err)

		var keyHandle NCRYPT_KEY_HANDLE
		buffer := &BCryptBuffer{
			cbBuffer:   uint32(len(name) * 2),
			bufferType: NCRYPTBUFFER_PKCS_KEY_NAME,
			pvBuffer:   castTo(&name[0]),
		}
		bufferDesc := &BCryptBufferDesc{
			ulVersion: BCRYPTBUFFER_VERSION,
			cBuffers:  1,
			pBuffers:  castTo(buffer),
		}
		err = ncryptImportKey(prov, NCRYPT_KEY_HANDLE(0), blobType, castTo(bufferDesc), &keyHandle, cngPrivBytes, NCRYPT_SILENT_FLAG|NCRYPT_OVERWRITE_KEY_FLAG)
		require.NoErrorf(t, err, "%w", wincrypt.WrapError("Failed to import private key", err))
		ctx, err := certAddEncodedCertificateToStore(store, windows.X509_ASN_ENCODING|windows.PKCS_7_ASN_ENCODING, clientCert.Raw, windows.CERT_STORE_ADD_REPLACE_EXISTING)
		require.NoErrorf(t, err, "%w", wincrypt.WrapError("Failed to add certificate to store", err))
		keyInfo := CRYPT_KEY_PROV_INFO{
			pwszContainerName: &name[0],
			pwszProvName:      &MS_KEY_STORAGE_PROVIDER[0],
			dwProvType:        0,
			dwFlags:           NCRYPT_SILENT_FLAG,
			cProvParam:        0,
			rgProvParam:       0,
			dwKeySpec:         0,
		}
		err = certSetCertificateContextProperty(ctx, CERT_KEY_PROV_INFO_PROP_ID, 0, castTo(&keyInfo))
		require.NoErrorf(t, err, "%w", wincrypt.WrapError("Failed to associate certficate with private key info", err))
		err = certSetCertificateContextProperty(ctx, CERT_NCRYPT_KEY_HANDLE_PROP_ID, 0, castTo(&keyHandle))
		require.NoErrorf(t, err, "%w", wincrypt.WrapError("Failed to associate certficate with private key", err))
		item.pair = &keyPair{
			cert:       clientCert,
			key:        privKey,
			certCtx:    ctx,
			privHandle: keyHandle,
		}
		for _, hasher := range TestHashers {
			item.data.hashes = append(item.data.hashes, testHash{algo: hasher, hash: getX509CertHash(hasher.New(), clientCert)})
		}
	}

	return caCert, caPriv
}

func TestWinCryptCertificate(t *testing.T) {
	store, err := syscall.CertOpenSystemStore(syscall.Handle(0), &wincrypt.USER_STORE_PERSONAL[0])
	require.NoError(t, err)
	var prov NCRYPT_PROV_HANDLE
	err = ncryptOpenStorageProvider(&prov, MS_KEY_STORAGE_PROVIDER[:], 0)
	require.NoErrorf(t, err, "%w", wincrypt.WrapError("Failed to open microsoft software key storage provider", err))
	serverCA, serverCAPriv, err := makeCACert("WinCryptTestServerCA", 0xCA2)
	require.NoError(t, err)
	serverPriv, err := rsa.GenerateKey(RNG, 2048)
	require.NoError(t, err)
	tmpl := x509.Certificate{
		Subject: pkix.Name{
			CommonName:   "localhost",
			SerialNumber: "ABC",
		},
		SerialNumber: big.NewInt(0xABC),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
	}
	serverCert, err := makeCert(serverCA, serverCAPriv, serverPriv, &tmpl, time.Hour*24*365)
	require.NoError(t, err)
	var ts *httptest.Server
	certs := []*testCert{
		{
			data: &testData{
				name:       "RSA2048TEST",
				isECDSA:    false,
				rsaKeySize: 2048,
			},
		}, {
			data: &testData{
				name:       "RSA4096TEST",
				isECDSA:    false,
				rsaKeySize: 4096,
			},
		}, {
			data: &testData{
				name:    "P256TEST",
				isECDSA: true,
				curve:   elliptic.P256(),
			},
		}, {
			data: &testData{
				name:    "P384TEST",
				isECDSA: true,
				curve:   elliptic.P384(),
			},
		}, {
			data: &testData{
				name:    "P521TEST",
				isECDSA: true,
				curve:   elliptic.P521(),
			},
		},
	}
	t.Cleanup(func() {
		// Cleanup
		if ts != nil {
			ts.Close()
		}
		atexit.Run()
		for _, cert := range certs {
			if cert.pair != nil {
				if cert.pair.privHandle != 0 {
					err := ncryptDeleteKey(cert.pair.privHandle, NCRYPT_SILENT_FLAG)
					if err != nil {
						_ = wincrypt.NCryptFreeObject(syscall.Handle(cert.pair.privHandle))
					}
				}
				if cert.pair.certCtx != nil {
					_ = certDeleteCertificateFromStore(cert.pair.certCtx)
				}
			}
		}
		if prov != 0 {
			_ = wincrypt.NCryptFreeObject(syscall.Handle(prov))
		}
		if store != 0 {
			_ = syscall.CertCloseStore(store, CERT_CLOSE_STORE_FORCE_FLAG)
		}
	})
	caCert, _ := makeTestCerts(t, certs, prov, store)

	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(caCert)
	ts = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peerCerts := r.TLS.PeerCertificates
		require.Greater(t, len(peerCerts), 0, "No certificate received")
		t.Logf("Received %d certificates", len(peerCerts))
		for _, receivedCert := range peerCerts {
			var testcert *testCert
			for _, cert := range certs {
				if receivedCert.SerialNumber.Cmp(cert.pair.cert.SerialNumber) == 0 {
					testcert = cert
					for _, hash := range testcert.data.hashes {
						assert.Equalf(t, hash.hash, getX509CertHash(hash.algo.New(), receivedCert), "%s hash of test certificate \"%s\" does not match expected one", hash.algo, cert.data.name)
					}
				}
			}
			assert.NotNilf(t, testcert, "Recevied certificate with subject \"%s\" serialNumber \"%X\" does not belong to test certificates", receivedCert.Subject, receivedCert.SerialNumber)
			if testcert != nil {
				t.Logf("Received certificate subject \"%s\", serialNumber \"%X\"", receivedCert.Subject, receivedCert.SerialNumber)
			}
		}

		chains, err := peerCerts[0].Verify(x509.VerifyOptions{Roots: clientCAs})
		assert.NoErrorf(t, err, "Failed to verify certificate: %v", err)
		if err == nil {
			t.Logf("Verified, constructed %d chains", len(chains))
			for i, chain := range chains {
				t.Logf("Chain %d:", i)
				for j, cert := range chain {
					t.Logf("Certificate %d: Subject: \"%s\", SerialNumber: \"%X\"", j, cert.Subject, cert.SerialNumber)
				}
			}
		}

		// Write some test data to fulfill the request
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprintln(w, "test data")
	}))
	serverRoot := x509.NewCertPool()
	serverRoot.AddCert(serverCA)
	ts.TLS = &tls.Config{
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  clientCAs,
		RootCAs:    serverRoot,
		MaxVersion: tls.VersionTLS12, // TLS V1.3 forces RSA PSS signing
		MinVersion: tls.VersionTLS12,
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{serverCert.Raw},
			PrivateKey:  serverPriv,
			SupportedSignatureAlgorithms: []tls.SignatureScheme{
				tls.PKCS1WithSHA1,
				tls.PKCS1WithSHA256,
				tls.PKCS1WithSHA384,
				tls.PKCS1WithSHA512,
				tls.PSSWithSHA256,
				tls.PSSWithSHA384,
				tls.PSSWithSHA512,
			},
		}},
	}
	ts.StartTLS()

	serverCAs := x509.NewCertPool()
	serverCAs.AddCert(serverCA)

	ctx := context.TODO()
	ci := fs.GetConfig(ctx)
	ci.WinCrypt = "all"
	client := fshttp.NewClient(ctx)
	tt := client.Transport.(*fshttp.Transport)
	tt.TLSClientConfig.RootCAs = serverCAs
	_, err = client.Get(ts.URL)
	assert.NoError(t, err)

	var builder strings.Builder
	for i, cert := range certs {
		ctx := context.TODO()
		ci := fs.GetConfig(ctx)
		ci.WinCrypt = fmt.Sprintf("/CN=%s/serialNumber=%d", cert.data.name+"Cert", i+1)
		client := fshttp.NewClient(ctx)
		tt := client.Transport.(*fshttp.Transport)
		tt.TLSClientConfig.RootCAs = serverCAs
		_, err = client.Get(ts.URL)
		assert.NoError(t, err)

		var loadedCerts1 []*x509.Certificate
		for _, c := range tt.TLSClientConfig.Certificates {
			loadedCerts1 = append(loadedCerts1, c.Leaf)
		}

		builder.Reset()
		builder.WriteString("hash: ")
		for i, hash := range cert.data.hashes {
			builder.WriteString(hash.hash)
			if i != len(cert.data.hashes)-1 {
				builder.WriteString(", ")
			}
		}
		hashQuery := builder.String()

		// Test RSA PSS and ECDSA signing
		ci.WinCrypt = hashQuery
		client = fshttp.NewClient(ctx)
		tt = client.Transport.(*fshttp.Transport)
		tt.TLSClientConfig.RootCAs = serverCAs
		_, err = client.Get(ts.URL)
		assert.NoError(t, err)

		var loadedCerts2 []*x509.Certificate
		for _, c := range tt.TLSClientConfig.Certificates {
			loadedCerts2 = append(loadedCerts2, c.Leaf)
		}
		assert.Equal(t, loadedCerts1, loadedCerts2)

		if !cert.data.isECDSA {
			ci.WinCrypt = ""
			client = fshttp.NewClient(ctx)
			tt = client.Transport.(*fshttp.Transport)
			tt.TLSClientConfig.RootCAs = serverCAs

			crypts, err := wincrypt.LoadWinCryptCerts(hashQuery)
			assert.NoError(t, err)
			var loadedCerts []*x509.Certificate
			for _, crypt := range crypts {
				tlsCert := crypt.TLSCertificate()

				// Test RSA PKCS v1.5 signing
				tlsCert.SupportedSignatureAlgorithms = []tls.SignatureScheme{
					tls.PKCS1WithSHA1,
					tls.PKCS1WithSHA256,
					tls.PKCS1WithSHA384,
					tls.PKCS1WithSHA512,
				}

				loadedCerts = append(loadedCerts, tlsCert.Leaf)
				tt.TLSClientConfig.Certificates = append(tt.TLSClientConfig.Certificates, tlsCert)
			}
			assert.ElementsMatch(t, loadedCerts1, loadedCerts)
			_, err = client.Get(ts.URL)
			assert.NoError(t, err)
		}
	}
}
