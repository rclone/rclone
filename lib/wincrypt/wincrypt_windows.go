//go:build windows
// +build windows

//go:generate stringer -type=rdnAttrType

// Package wincrypt implements loading certificate/key pairs from Windows certificate store for Mutual TLS authentication
package wincrypt

import (
	"crypto"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"maps"
	"math/big"
	"reflect"
	"slices"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/atexit"
	"golang.org/x/sys/windows"
)

//nolint:revive // Windows constants
var (
	NCRYPT_KEY_USAGE_PROPERTY       = [...]uint16{'K', 'e', 'y', ' ', 'U', 's', 'a', 'g', 'e', 0}
	NCRYPT_ALGORITHM_GROUP_PROPERTY = [...]uint16{'A', 'l', 'g', 'o', 'r', 'i', 't', 'h', 'm', ' ', 'G', 'r', 'o', 'u', 'p', 0}
	NCRYPT_ECDH_ALGORITHM_GROUP     = [...]uint16{'E', 'C', 'D', 'H', 0}
	NCRYPT_RSA_ALGORITHM_GROUP      = [...]uint16{'R', 'S', 'A', 0}
	NCRYPT_ECDSA_ALGORITHM_GROUP    = [...]uint16{'E', 'C', 'D', 'S', 'A', 0}
	BCRYPT_SHA1_ALGORITHM           = [...]uint16{'S', 'H', 'A', '1', 0}
	BCRYPT_SHA256_ALGORITHM         = [...]uint16{'S', 'H', 'A', '2', '5', '6', 0}
	BCRYPT_SHA384_ALGORITHM         = [...]uint16{'S', 'H', 'A', '3', '8', '4', 0}
	BCRYPT_SHA512_ALGORITHM         = [...]uint16{'S', 'H', 'A', '5', '1', '2', 0}
	USER_STORE_PERSONAL             = [...]uint16{'M', 'Y', 0}
)

var (
	ncrypt                                   = syscall.NewLazyDLL("ncrypt.dll")
	crypt32                                  = syscall.NewLazyDLL("crypt32.dll")
	cryptui                                  = syscall.NewLazyDLL("cryptui.dll")
	procCryptUIDlgSelectCertificateFromStore = cryptui.NewProc("CryptUIDlgSelectCertificateFromStore")
	procCryptAcquireCertificatePrivateKey    = crypt32.NewProc("CryptAcquireCertificatePrivateKey")
	procCertEnumCertificatesInStore          = crypt32.NewProc("CertEnumCertificatesInStore")
	procCertDuplicateCertificateContext      = crypt32.NewProc("CertDuplicateCertificateContext")
	procNCryptGetProperty                    = ncrypt.NewProc("NCryptGetProperty")
	procNCryptSignHash                       = ncrypt.NewProc("NCryptSignHash")
	procNCryptFreeObject                     = ncrypt.NewProc("NCryptFreeObject")

	attrAltNames = map[rdnAttrType]string{
		COMMONNAME:             "CN",
		COUNTRYNAME:            "C",
		LOCALITYNAME:           "L",
		STATEORPROVINCENAME:    "ST",
		STREETADDRESS:          "STREET",
		ORGANIZATIONNAME:       "O",
		ORGANIZATIONALUNITNAME: "OU",
	}
)

//nolint:revive // Windows constants
const (
	NCRYPT_ALLOW_ALL_USAGES         uint32 = 0x00FFFFFF
	NCRYPT_ALLOW_DECRYPT_FLAG       uint32 = 0x00000001
	NCRYPT_ALLOW_SIGNING_FLAG       uint32 = 0x00000002
	NCRYPT_ALLOW_KEY_AGREEMENT_FLAG uint32 = 0x00000004
	BCRYPT_PAD_PKCS1                uint32 = 0x00000002
	BCRYPT_PAD_PSS                  uint32 = 0x00000008
)

type keyType int
type rdnAttrType int

//nolint:revive // Constants in uppercase used for matching
const (
	COMMONNAME rdnAttrType = iota
	SERIALNUMBER
	COUNTRYNAME
	LOCALITYNAME
	STATEORPROVINCENAME
	STREETADDRESS
	ORGANIZATIONNAME
	ORGANIZATIONALUNITNAME
	POSTALCODE
	ATTRUNSPECIFIED
)

type subjectAttr struct {
	attrType rdnAttrType
	values   [][]string
}

const (
	keyTypeUnspecified keyType = iota
	keyTypeRSA
	keyTypeECDSA
)

//nolint:revive // Windows structures
type (
	BCRYPT_PKCS1_PADDING_INFO struct {
		pszAlgId *uint16
	}

	BCRYPT_PSS_PADDING_INFO struct {
		pszAlgId *uint16
		cbSalt   uint32
	}
)

// WINCRYPT is a structure encapsulates CNG key handle
type WINCRYPT struct {
	cert       tls.Certificate
	priv       syscall.Handle
	keyType    keyType
	shouldFree bool
}

func wrapError(prefix string, err error, args ...any) error {
	prefix = fmt.Sprintf(prefix, args...)
	if errno, ok := err.(syscall.Errno); ok {
		return fmt.Errorf("%s, errorCode=\"0x%08X\", errorMessage=\"%w\"", prefix, uint32(errno), err)
	}
	return fmt.Errorf("%s: %w", prefix, err)
}

func isKeySuitableForSigning(prov syscall.Handle, keyType keyType) (bool, error) {
	var keyUsage uint32
	var keyUsageSize uint32 = 4
	err := ncryptGetProperty(prov, NCRYPT_KEY_USAGE_PROPERTY[:], castTo(&keyUsage), keyUsageSize, &keyUsageSize, 0)
	if err != nil {
		return false, wrapError("failed to get Key Usage information", err)
	}
	switch {
	case keyUsage == NCRYPT_ALLOW_ALL_USAGES:
		fs.Debug(nil, "All usage flag set")
		return true, nil
	case keyUsage&NCRYPT_ALLOW_SIGNING_FLAG != 0:
		fs.Debug(nil, "Signing usage flag set")
		return true, nil
	case (keyUsage&NCRYPT_ALLOW_KEY_AGREEMENT_FLAG != 0) && (keyType == keyTypeECDSA):
		fs.Debug(nil, "Allow ECC key with key agreement usage flag set")
		return true, nil
	default:
		return false, errors.New("provided key is not suitable for signing purpose")
	}
}

func freeCertificates(certs []*syscall.CertContext) {
	if len(certs) > 0 {
		for _, cert := range certs {
			_ = syscall.CertFreeCertificateContext(cert)
		}
	}
}

func castFrom[T any](ptr uintptr) *T {
	// Add another indirect level to please go vet
	return (*T)(*(*unsafe.Pointer)(unsafe.Pointer(&ptr)))
}

func castTo[T any](ptr *T) uintptr {
	return uintptr(unsafe.Pointer(ptr))
}

func enumerateCertificates(store syscall.Handle) ([]*syscall.CertContext, error) {
	currentCtx, _, err := procCertEnumCertificatesInStore.Call(uintptr(store), 0)
	if err == syscall.Errno(windows.CRYPT_E_NOT_FOUND) {
		return nil, nil
	} else if err != windows.ERROR_SUCCESS {
		return nil, err
	}
	var certs []*syscall.CertContext
	for {
		copyCtx, _, err := procCertDuplicateCertificateContext.Call(currentCtx)
		if err != windows.ERROR_SUCCESS {
			certs = append(certs, castFrom[syscall.CertContext](currentCtx))
			freeCertificates(certs)
			return nil, err
		}
		certs = append(certs, castFrom[syscall.CertContext](copyCtx))
		currentCtx, _, err = procCertEnumCertificatesInStore.Call(uintptr(store), currentCtx)
		if err == syscall.Errno(windows.CRYPT_E_NOT_FOUND) {
			return certs, nil
		} else if err != windows.ERROR_SUCCESS {
			freeCertificates(certs)
			return nil, err
		}
	}
}

// Unprintable string, used as map key only
func calculateCertHash(hasher hash.Hash, cert *x509.Certificate) string {
	hasher.Reset()
	hasher.Write(cert.Raw)
	return string(hasher.Sum(nil))
}

func parseCertContext(ctx *syscall.CertContext) (*x509.Certificate, error) {
	encoded := unsafe.Slice(ctx.EncodedCert, ctx.Length)
	buf := make([]byte, ctx.Length)
	copy(buf, encoded)
	return x509.ParseCertificate(buf)
}

func getRDNAttrType(name string) rdnAttrType {
	str := strings.ToUpper(name)
	for attrType := rdnAttrType(0); attrType < ATTRUNSPECIFIED; attrType++ {
		if attrType.String() == str {
			return attrType
		} else if altName, ok := attrAltNames[attrType]; ok && altName == str {
			return attrType
		}
	}
	return ATTRUNSPECIFIED
}

func parseOpenSSLSubject(subjectStr string) ([]*subjectAttr, error) {
	subject := []rune(subjectStr)
	if len(subject) == 0 {
		return nil, errors.New("no subject string specified")
	}
	if subject[0] != '/' {
		return nil, fmt.Errorf("invalid first charater of subject string, expects '/', got '%c'", subject[0])
	}
	attrs := make(map[rdnAttrType]*subjectAttr, int(ATTRUNSPECIFIED))
	i := 1
	var str strings.Builder
	noTypeError := errors.New("no RDN type string specified")
	parseName := func() (string, error) {
		if i >= len(subject) {
			return "", noTypeError
		}
		str.Reset()
	err:
		for ; i < len(subject); i++ {
			switch subject[i] {
			case '/':
				if str.Len() == 0 {
					return "", noTypeError
				}
				break err
			case '=':
				if str.Len() == 0 {
					return "", noTypeError
				}
				i += 1
				return str.String(), nil
			case '\\':
				if i+1 >= len(subject) {
					return "", fmt.Errorf("unexpected EOF during parsing escape character in RDN type string at index %d", i)
				}
				i += 1
				fallthrough
			default:
				str.WriteRune(subject[i])
			}
		}
		return "", fmt.Errorf("missing '=' after RDN type string \"%s\" in subject name string at index %d", str.String(), i)
	}
	parseValue := func() ([]string, error) {
		var values []string
		// attributes without value are allowed
		if i >= len(subject) {
			return values, nil
		}
		str.Reset()
	out:
		for ; i < len(subject); i++ {
			switch subject[i] {
			case '=':
				return nil, fmt.Errorf("unexpected character '=' found at index %d. If it's intended, please escape it with '\\'", i)
			case '/':
				i += 1
				break out
			case '+':
				if str.Len() != 0 {
					values = append(values, str.String())
					str.Reset()
				}
			case '\\':
				if i+1 >= len(subject) {
					return nil, fmt.Errorf("unexpected EOF during parsing escape character in value at index %d", i)
				}
				i += 1
				fallthrough
			default:
				str.WriteRune(subject[i])
			}
		}
		if str.Len() != 0 {
			values = append(values, str.String())
			str.Reset()
		}
		return values, nil
	}
	for i < len(subject) {
		name, err := parseName()
		if err != nil {
			return nil, err
		}
		values, err := parseValue()
		if err != nil {
			return nil, err
		}
		attrType := getRDNAttrType(name)
		if attrType == ATTRUNSPECIFIED {
			fs.Errorf(nil, "Unknown RDN attribute type \"%s\", ignoring", name)
			continue
		}
		if attrType == COMMONNAME || attrType == SERIALNUMBER {
			if len(values) > 1 {
				return nil, fmt.Errorf("attribute type \"%s\" should be single value", attrType.String())
			}
		}
		if attr, ok := attrs[attrType]; ok {
			attr.values = append(attr.values, values)
		} else {
			attrs[attrType] = &subjectAttr{attrType: attrType, values: [][]string{values}}
		}
	}
	return slices.Collect(maps.Values(attrs)), nil
}

type certPair struct {
	cert *x509.Certificate
	ctx  *syscall.CertContext
}

func matchCertificatesByHash(availablePairs []*certPair, hashes string) ([]*certPair, error) {
	if len(availablePairs) == 0 {
		return availablePairs, nil
	}
	hashStrs := strings.Split(hashes, ",")

	hashTypes := make(map[crypto.Hash]bool)

	validPairs := make(map[*certPair]bool, len(availablePairs))

	for i := 0; i < len(hashStrs); i++ {
		hashStrs[i] = strings.TrimSpace(hashStrs[i])
		hashStr := hashStrs[i]
		if hashStr == "" {
			return nil, fmt.Errorf("invalid hash string format: \"%s\"", hashes)
		}
		switch len(hashStr) {
		case 32:
			hashTypes[crypto.MD5] = true
		case 40:
			hashTypes[crypto.SHA1] = true
		case 64:
			hashTypes[crypto.SHA256] = true
		case 96:
			hashTypes[crypto.SHA384] = true
		case 128:
			hashTypes[crypto.SHA512] = true
		default:
			return nil, fmt.Errorf("hash string \"%s\" is not a MD5/SHA-1/SHA-256/SHA-384/SHA-512 hex string", hashStr)
		}
	}
	certHashTable := make(map[string][]*certPair)
	var hashers []hash.Hash
	for k := range hashTypes {
		hashers = append(hashers, k.New())
	}
	for _, pair := range availablePairs {
		for _, hasher := range hashers {
			hashBytes := calculateCertHash(hasher, pair.cert)
			entries, ok := certHashTable[hashBytes]
			if !ok {
				certHashTable[hashBytes] = []*certPair{pair}
			} else {
				// Duplicate hash found
				entries = append(entries, pair)
				certHashTable[hashBytes] = entries
			}
		}
	}

	for _, hashStr := range hashStrs {

		hash, err := hex.DecodeString(hashStr)
		if err != nil {
			return nil, err
		}

		entries, ok := certHashTable[string(hash)]
		if !ok {
			fs.Errorf(nil, "No certificate has hash \"%s\"", hashStr)
			continue
		}

		for _, entry := range entries {
			if _, ok := validPairs[entry]; !ok {
				validPairs[entry] = true
			} else {
				fs.LogPrintf(fs.LogLevelWarning, nil, "Found duplicate certificate with hash \"%s\", ignoring", hashStr)
			}
		}
	}

	return slices.Collect(maps.Keys(validPairs)), nil
}

func isCertAttributesMatch(cert *x509.Certificate, attrs []*subjectAttr) bool {
	valid := true
	for _, attr := range attrs {

		// For attribute specified multiple times, as long as one of them match, the attribute matches
		attrMatch := false
		for _, attrValues := range attr.values {
			// empty attributes always match
			if len(attrValues) == 0 {
				attrMatch = true
				break
			}

			var targetStrs []string
			switch attr.attrType {
			case COMMONNAME:
				targetStrs = []string{cert.Subject.CommonName}
			case SERIALNUMBER:
				targetStrs = []string{cert.Subject.SerialNumber}
			case COUNTRYNAME:
				targetStrs = cert.Subject.Country
			case LOCALITYNAME:
				targetStrs = cert.Subject.Locality
			case STATEORPROVINCENAME:
				targetStrs = cert.Subject.Province
			case STREETADDRESS:
				targetStrs = cert.Subject.StreetAddress
			case ORGANIZATIONNAME:
				targetStrs = cert.Subject.Organization
			case ORGANIZATIONALUNITNAME:
				targetStrs = cert.Subject.OrganizationalUnit
			case POSTALCODE:
				targetStrs = cert.Subject.PostalCode
			}

			// always won't match
			if len(attrValues) > len(targetStrs) || len(targetStrs) == 0 {
				continue
			}

			attrValueMatch := true
			matched := make(map[int]bool, len(targetStrs))

			for _, value := range attrValues {
				match := false
				for i, target := range targetStrs {
					if _, ok := matched[i]; ok {
						continue
					}
					if strings.Contains(target, value) {
						matched[i] = true
						match = true
						break
					}
				}

				// if any attribute value does not match, the whole match fails
				if !match {
					attrValueMatch = false
					break
				}
			}

			if attrValueMatch {
				attrMatch = true
				break
			}
		}

		if !attrMatch {
			valid = false
			break
		}
	}
	return valid
}

func matchCertificatesBySubject(availablePairs []*certPair, subject string) ([]*certPair, error) {
	if len(availablePairs) == 0 {
		return availablePairs, nil
	}
	attrs, err := parseOpenSSLSubject(subject)
	if err != nil {
		return nil, err
	}

	var validPairs []*certPair
	for _, pair := range availablePairs {

		if isCertAttributesMatch(pair.cert, attrs) {
			validPairs = append(validPairs, pair)
		}
	}
	return validPairs, nil
}

func filterInvalidCerts(pairs []*certPair) (result []*certPair) {
	now := time.Now().UTC()
	for _, pair := range pairs {
		if pair.cert.NotAfter.UTC().After(now) && pair.cert.NotBefore.UTC().Before(now) {
			result = append(result, pair)
		}
	}
	return
}

/*
LoadWinCryptCerts loads certificate/key pairs from Windows certificate store matche specified criteria.
Please check documentation for the detailed format of "criteria" string.
*/
func LoadWinCryptCerts(criteria string) (crypts []*WINCRYPT, err error) {
	criteria = strings.TrimSpace(criteria)
	store, err := syscall.CertOpenSystemStore(syscall.Handle(0), &USER_STORE_PERSONAL[0])
	if err != nil {
		return nil, wrapError("failed to open user personal certificate store", err)
	}
	defer func() { _ = syscall.CertCloseStore(store, 0) }()

	var availableCerts []*certPair
	availableCertCtxs := []*syscall.CertContext{}
	defer func() { freeCertificates(availableCertCtxs) }()
	matchFunc := func(pairs []*certPair, _ string) ([]*certPair, error) {
		return pairs, nil
	}
	if strings.EqualFold(criteria, "select") {
		ctx, err := selectCertificateFromUserStore(store)
		if err != nil {
			return nil, wrapError("error occurred trying to get selected certificate handle", err)
		}
		// User canceled selection
		if ctx == nil {
			return nil, errors.New("no WinCrypt certificate was chosen")
		}
		availableCertCtxs = append(availableCertCtxs, ctx)
	} else {
		certCtxs, err := enumerateCertificates(store)
		if err != nil {
			return nil, wrapError("failed to enumerate certificate in user personal certificate store", err)
		}
		if len(certCtxs) == 0 {
			return nil, errors.New("no certificate found in user personal certificate store")
		}
		availableCertCtxs = append(availableCertCtxs, certCtxs...)

		if !strings.EqualFold(criteria, "all") {
			if len(criteria) >= 5 && strings.EqualFold(criteria[0:5], "hash:") {
				if len(criteria) < 6 {
					return nil, errors.New("no hash specified")
				}
				criteria = criteria[5:]
				matchFunc = matchCertificatesByHash
			} else {
				matchFunc = matchCertificatesBySubject
			}
		}
	}

	for _, ctx := range availableCertCtxs {
		cert, err := parseCertContext(ctx)
		if err != nil {
			fs.Errorf(nil, "failed to parse the certificate, skiping: %s", err)
			continue
		}
		availableCerts = append(availableCerts, &certPair{cert, ctx})
	}

	availableCerts, err = matchFunc(availableCerts, criteria)
	if err != nil {
		return nil, err
	}

	availableCerts = filterInvalidCerts(availableCerts)
	if len(availableCerts) == 0 {
		return nil, fmt.Errorf("no certificate found matches the specified criteria \"%s\"", criteria)
	}
	for _, cert := range availableCerts {
		crypt := new(WINCRYPT)
		defer func() {
			if crypt != nil {
				_ = crypt.Close()
			}
		}()
		fs.Debugf(nil, "Certificate Subject: %s, SerialNumber: %X", cert.cert.Subject.String(), cert.cert.SerialNumber)
		var keyFlags uint32
		err = cryptAcquireCertificatePrivateKey(cert.ctx, windows.CRYPT_ACQUIRE_COMPARE_KEY_FLAG|windows.CRYPT_ACQUIRE_ONLY_NCRYPT_KEY_FLAG, &(crypt.priv), &keyFlags, &crypt.shouldFree)
		if err != nil {
			if errors.Unwrap(err) != nil {
				err = errors.Unwrap(err)
			}
			if err == syscall.Errno(windows.CRYPT_E_NO_KEY_PROPERTY) {
				fs.LogPrintf(fs.LogLevelNotice, nil, "The certificate has no associated private key, skiping")
			} else if err == syscall.Errno(windows.NTE_BAD_PUBLIC_KEY) {
				fs.Errorf(nil, "The public key of this certificate does not match the private key, skiping")
			} else {
				fs.Errorf(nil, "%s", wrapError("Unknown error occurred during private key handle acquisition, skiping", err))
			}
			continue
		}
		if keyFlags != windows.CERT_NCRYPT_KEY_SPEC {
			fs.LogPrintf(fs.LogLevelNotice, nil, "The certificate has no associated NCrypt key handle, skiping")
			continue
		}
		crypt.keyType, err = ncryptGetPrivateKeyType(crypt.priv)
		if err != nil {
			fs.Errorf(nil, "Failed to determine private key type, skiping: %s", err)
			continue
		}
		v, err := isKeySuitableForSigning(crypt.priv, crypt.keyType)
		if err != nil {
			fs.Errorf(nil, "Failed to determine key pair capability, skiping: %s", err)
			continue
		} else if !v {
			fs.Error(nil, "The key pair is not suitable for signing purpose, skiping")
			continue
		}
		crypt.cert = tls.Certificate{
			PrivateKey:  crypt,
			Leaf:        cert.cert,
			Certificate: [][]byte{cert.cert.Raw},
		}
		if crypt.keyType == keyTypeECDSA {
			crypt.cert.SupportedSignatureAlgorithms = []tls.SignatureScheme{
				tls.ECDSAWithSHA1,
				tls.ECDSAWithP256AndSHA256,
				tls.ECDSAWithP384AndSHA384,
				tls.ECDSAWithP521AndSHA512,
			}
		} else {
			crypt.cert.SupportedSignatureAlgorithms = []tls.SignatureScheme{
				tls.PKCS1WithSHA1,
				tls.PKCS1WithSHA256,
				tls.PKCS1WithSHA384,
				tls.PKCS1WithSHA512,
				tls.PSSWithSHA256,
				tls.PSSWithSHA384,
				tls.PSSWithSHA512,
			}
		}
		crypts = append(crypts, crypt)
		crypt = nil
	}
	if len(crypts) == 0 {
		return nil, errors.New("no valid certificate/private key pair available")
	}
	atexit.Register(func() {
		for _, crypt := range crypts {
			if crypt != nil {
				_ = crypt.Close()
			}
		}
	})
	return crypts, nil
}

// Public returns public key of loaded certificate
func (w *WINCRYPT) Public() crypto.PublicKey {
	return w.cert.Leaf.PublicKey
}

// TLSCertificate returns client TLS certificate with CNG private key
func (w *WINCRYPT) TLSCertificate() tls.Certificate {
	return w.cert
}

func goHashToNCryptHash(h crypto.Hash) (alg *uint16, err error) {
	err = nil
	switch h {
	case crypto.SHA1:
		alg = &BCRYPT_SHA1_ALGORITHM[0]
	case crypto.SHA256:
		alg = &BCRYPT_SHA256_ALGORITHM[0]
	case crypto.SHA384:
		alg = &BCRYPT_SHA384_ALGORITHM[0]
	case crypto.SHA512:
		alg = &BCRYPT_SHA512_ALGORITHM[0]
	default:
		err = fmt.Errorf("no suitable hash algorithm identifier found for %v", h)
	}
	return alg, err
}

// Sign signs "digest" by CNG private key represented by CNG key handle
func (w *WINCRYPT) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	var paddingInfo uintptr
	var signFlags uint32
	signingType := "ECDSA"
	if w.keyType == keyTypeRSA {
		pss, ok := opts.(*rsa.PSSOptions)

		alg, err := goHashToNCryptHash(opts.HashFunc())
		if err != nil {
			return nil, err
		}
		if ok {
			signingType = "RSA PSS"
			pssPad := BCRYPT_PSS_PADDING_INFO{}
			pssPad.pszAlgId = alg
			var cbSalt uint32
			if pss.SaltLength == rsa.PSSSaltLengthEqualsHash || pss.SaltLength == rsa.PSSSaltLengthAuto {
				cbSalt = uint32(opts.HashFunc().Size())
			} else {
				cbSalt = uint32(pss.SaltLength)
			}
			pssPad.cbSalt = cbSalt
			signFlags = BCRYPT_PAD_PSS
			paddingInfo = castTo(&pssPad)
		} else {
			signingType = "RSA PKCS1"
			pkcs1Pad := BCRYPT_PKCS1_PADDING_INFO{}
			pkcs1Pad.pszAlgId = alg
			signFlags = BCRYPT_PAD_PKCS1
			paddingInfo = castTo(&pkcs1Pad)
		}
	} else if w.keyType != keyTypeECDSA {
		return nil, errors.New("unsupported private key type")
	}
	signature, err = ncryptSignHash(w.priv, paddingInfo, digest, uint32(len(digest)), signFlags)
	if err == syscall.Errno(windows.SCARD_W_CANCELLED_BY_USER) {
		return nil, errors.New("signing operation canceled by user")
	}
	if err != nil {
		return nil, wrapError(signingType+" signing failed", err)
	}
	if w.keyType == keyTypeECDSA {
		signature, err = ecdsaConvertIEEEP1363ToASN1(signature)
		if err != nil {
			return nil, err
		}
	}
	fs.Debugf(nil, "%s signed successfully", signingType)
	return
}

// Close frees CNG key handle
func (w *WINCRYPT) Close() error {
	if w.shouldFree && w.cert.PrivateKey != nil {
		w.cert.PrivateKey = nil
		_ = ncryptFreeObject(w.priv)
	}
	return nil
}

func selectCertificateFromUserStore(store syscall.Handle) (ctx *syscall.CertContext, err error) {
	pCtx, _, err := procCryptUIDlgSelectCertificateFromStore.Call((uintptr)(store), 0x0, 0x0, 0x0, 0x0, 0x0, 0x0)
	ctx = castFrom[syscall.CertContext](pCtx)
	if err == windows.ERROR_SUCCESS {
		err = nil
	}
	return
}

func ncryptGetProperty(prov syscall.Handle, prop []uint16, pbOut uintptr, cbOut uint32, pcbOut *uint32, dwFlags uint32) (err error) {
	errno, _, _ := procNCryptGetProperty.Call(uintptr(prov), castTo(&prop[0]), pbOut, uintptr(cbOut), castTo(pcbOut), uintptr(dwFlags))
	err = syscall.Errno(errno)
	if err == windows.ERROR_SUCCESS {
		err = nil
	}
	return err
}

func cryptAcquireCertificatePrivateKey(ctx *syscall.CertContext, dwFlags uint32, prov *syscall.Handle, pdwKeySpec *uint32, shouldFree *bool) (err error) {
	var callerFree uint32
	result, _, err := procCryptAcquireCertificatePrivateKey.Call(castTo(ctx), uintptr(dwFlags), 0x0, castTo(prov), castTo(pdwKeySpec), castTo(&callerFree))
	if result == 0 {
		return err
	}
	err = nil
	*shouldFree = (callerFree == 1)
	return
}

func ncryptFreeObject(obj syscall.Handle) (err error) {
	errno, _, _ := procNCryptFreeObject.Call(uintptr(obj))
	err = syscall.Errno(errno)
	if err == windows.ERROR_SUCCESS {
		err = nil
	}
	return
}

func ncryptSignHash(prov syscall.Handle, paddingInfo uintptr, hash []byte, cbHash uint32, dwFlags uint32) (signature []byte, err error) {
	var sigSize uint32
	errno, _, _ := procNCryptSignHash.Call(uintptr(prov), 0x0, castTo(&hash[0]), uintptr(cbHash), 0x0, 0x0, castTo(&sigSize), uintptr(dwFlags))
	err = syscall.Errno(errno)
	if err != windows.ERROR_SUCCESS {
		return nil, wrapError("failed to get signature size", err)
	}
	signature = make([]byte, sigSize)
	errno, _, _ = procNCryptSignHash.Call(uintptr(prov), paddingInfo, castTo(&hash[0]), uintptr(cbHash), castTo(&signature[0]), uintptr(sigSize), castTo(&sigSize), uintptr(dwFlags))
	err = syscall.Errno(errno)
	if err == windows.ERROR_SUCCESS {
		err = nil
	}
	return
}

func ecdsaConvertIEEEP1363ToASN1(src []byte) ([]byte, error) {
	// R and S
	var sigs = [2]*big.Int{new(big.Int), new(big.Int)}
	sigs[0].SetBytes(src[:len(src)/2])
	sigs[1].SetBytes(src[len(src)/2:])
	return asn1.Marshal(sigs[:])
}

func ncryptGetPrivateKeyType(prov syscall.Handle) (ktype keyType, err error) {
	var propertySize uint32
	ktype = keyTypeUnspecified
	err = ncryptGetProperty(prov, NCRYPT_ALGORITHM_GROUP_PROPERTY[:], 0x0, 0x0, &propertySize, 0)
	if err != nil {
		return ktype, wrapError("failed to query algorithm group size", err)
	}
	if propertySize == 0 || propertySize&0x1 == 0x1 {
		return ktype, fmt.Errorf("invalid property size: %d", propertySize)
	}
	var alg = make([]uint16, propertySize/2)
	err = ncryptGetProperty(prov, NCRYPT_ALGORITHM_GROUP_PROPERTY[:], castTo(&alg[0]), propertySize, &propertySize, 0)
	if err != nil {
		return ktype, wrapError("failed to query algorithm group", err)
	}
	str := windows.UTF16PtrToString(&alg[0])
	fs.Debugf(nil, "Algorithm Group: %s", str)
	if reflect.DeepEqual(alg, NCRYPT_ECDH_ALGORITHM_GROUP[:]) || reflect.DeepEqual(alg, NCRYPT_ECDSA_ALGORITHM_GROUP[:]) {
		ktype = keyTypeECDSA
	} else if reflect.DeepEqual(alg, NCRYPT_RSA_ALGORITHM_GROUP[:]) {
		ktype = keyTypeRSA
	} else {
		return ktype, fmt.Errorf("unsupported private key algorithm group: %v", str)
	}
	return ktype, nil
}
