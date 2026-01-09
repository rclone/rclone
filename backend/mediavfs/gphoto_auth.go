package mediavfs

import (
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"golang.org/x/crypto/hkdf"
)

// Device constants
const (
	defaultAndroidID = "38106360b2a855e1"
	gmsVersion       = "254730032"
	sdkVersion       = "33"
	photosVersion    = "51079550"
	// Photos app client signature (for photos.native scope)
	photosClientSig = "24bb24c05e47e0aefa68a58a766179d9b613a600"
)

// Type URLs for Tink
const (
	eciesTypeURL  = "type.googleapis.com/google.crypto.tink.EciesAeadHkdfPublicKey"
	aesGCMTypeURL = "type.googleapis.com/google.crypto.tink.AesGcmKey"
)

// encodeVarint encodes an integer as a protobuf varint.
func authEncodeVarint(value uint64) []byte {
	var result []byte
	for value > 127 {
		result = append(result, byte((value&0x7F)|0x80))
		value >>= 7
	}
	result = append(result, byte(value))
	return result
}

// encodeField encodes a protobuf field with tag and data.
func authEncodeField(fieldNum int, wireType int, data []byte) []byte {
	tag := uint64((fieldNum << 3) | wireType)
	return append(authEncodeVarint(tag), data...)
}

// encodeBytesField encodes a length-delimited bytes field.
func authEncodeBytesField(fieldNum int, data []byte) []byte {
	lengthPrefix := authEncodeVarint(uint64(len(data)))
	return authEncodeField(fieldNum, 2, append(lengthPrefix, data...))
}

// encodeVarintField encodes a varint field.
func authEncodeVarintField(fieldNum int, value uint64) []byte {
	return authEncodeField(fieldNum, 0, authEncodeVarint(value))
}

// encodeTinkKeyset encodes a Tink ECIES-AEAD-HKDF keyset for Google Photos token binding.
func encodeTinkKeyset(keyID uint32, xBytes, yBytes []byte) []byte {
	// Build KeyTemplate (for AES-128-GCM)
	// Structure: {1: type_url, 2: {2: 16}, 3: 1}
	keyTemplateValue := authEncodeVarintField(2, 16) // field 2 = key_size = 16
	keyTemplate := authEncodeBytesField(1, []byte(aesGCMTypeURL))
	keyTemplate = append(keyTemplate, authEncodeBytesField(2, keyTemplateValue)...)
	keyTemplate = append(keyTemplate, authEncodeVarintField(3, 1)...) // output_prefix_type = TINK

	// kem_params: {1: 2, 2: 3} (curve=P256, hash=SHA256)
	kemParams := authEncodeVarintField(1, 2)
	kemParams = append(kemParams, authEncodeVarintField(2, 3)...)

	// dem_params: {2: KeyTemplate}
	demParams := authEncodeBytesField(2, keyTemplate)

	// params (NESTED): {1: kem_params, 2: dem_params, 3: 1}
	params := authEncodeBytesField(1, kemParams)
	params = append(params, authEncodeBytesField(2, demParams)...)
	params = append(params, authEncodeVarintField(3, 1)...) // ec_point_format = UNCOMPRESSED

	// Always prepend 0x00 to coordinates (Google's Tink impl always adds this prefix)
	xEncoded := append([]byte{0x00}, xBytes...)
	yEncoded := append([]byte{0x00}, yBytes...)

	// EciesAeadHkdfPublicKey value: {2: params, 3: x, 4: y}
	// NO version field (field 1)
	pubKeyValue := authEncodeBytesField(2, params)
	pubKeyValue = append(pubKeyValue, authEncodeBytesField(3, xEncoded)...)
	pubKeyValue = append(pubKeyValue, authEncodeBytesField(4, yEncoded)...)

	// keyData: {1: type_url, 2: pub_key_value, 3: 3}
	keyData := authEncodeBytesField(1, []byte(eciesTypeURL))
	keyData = append(keyData, authEncodeBytesField(2, pubKeyValue)...)
	keyData = append(keyData, authEncodeVarintField(3, 3)...) // key_material_type = ASYMMETRIC_PUBLIC

	// key message: {1: keyData, 2: 1, 3: key_id, 4: 1}
	keyMsg := authEncodeBytesField(1, keyData)
	keyMsg = append(keyMsg, authEncodeVarintField(2, 1)...)           // status = ENABLED
	keyMsg = append(keyMsg, authEncodeVarintField(3, uint64(keyID))...)
	keyMsg = append(keyMsg, authEncodeVarintField(4, 1)...)           // output_prefix_type = TINK

	// keyset: {1: key_id, 2: key_msg}
	keyset := authEncodeVarintField(1, uint64(keyID))
	keyset = append(keyset, authEncodeBytesField(2, keyMsg)...)

	return keyset
}

// base64URLEncode encodes data to base64url without padding.
func base64URLEncode(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

// base64URLDecode decodes base64url data with or without padding.
func base64URLDecode(data string) ([]byte, error) {
	// Add padding if needed
	switch len(data) % 4 {
	case 2:
		data += "=="
	case 3:
		data += "="
	}
	return base64.URLEncoding.DecodeString(data)
}

// protoField represents a parsed protobuf field.
type protoField struct {
	wireType int
	data     []byte
	value    uint64
}

// parseProtoFields parses protobuf fields into a map.
func authParseProtoFields(data []byte) map[int]protoField {
	fields := make(map[int]protoField)
	pos := 0

	for pos < len(data) {
		if pos >= len(data) {
			break
		}
		tag := data[pos]
		fieldNum := int(tag >> 3)
		wireType := int(tag & 0x7)
		pos++

		switch wireType {
		case 0: // varint
			var value uint64
			var shift uint
			for pos < len(data) {
				b := data[pos]
				pos++
				value |= uint64(b&0x7F) << shift
				if b&0x80 == 0 {
					break
				}
				shift += 7
			}
			fields[fieldNum] = protoField{wireType: wireType, value: value}
		case 2: // length-delimited
			var length uint64
			var shift uint
			for pos < len(data) {
				b := data[pos]
				pos++
				length |= uint64(b&0x7F) << shift
				if b&0x80 == 0 {
					break
				}
				shift += 7
			}
			if pos+int(length) > len(data) {
				break
			}
			fieldData := data[pos : pos+int(length)]
			pos += int(length)
			fields[fieldNum] = protoField{wireType: wireType, data: fieldData}
		default:
			// Unknown wire type, stop parsing
			return fields
		}
	}
	return fields
}

// TokenResult represents the result of a token request.
type TokenResult struct {
	Token  string
	Expiry int64 // milliseconds
	Scopes string
	Error  string
	Source string // "cache" or "fresh"
}

// GooglePhotosAuth handles Google Photos OAuth token generation with optional token binding.
type GooglePhotosAuth struct {
	email       string
	masterToken string
	androidID   string

	privateKey          *ecdsa.PrivateKey
	issuer              string
	ephemeralPrivateKey *ecdsa.PrivateKey
	httpClient          *http.Client
	mu                  sync.Mutex
	authFailCount       int // Track consecutive auth failures
}

// NewGooglePhotosAuth creates a new GooglePhotosAuth instance.
func NewGooglePhotosAuth(email, masterToken, androidID string, privateKeyHex string, httpClient *http.Client) (*GooglePhotosAuth, error) {
	if androidID == "" {
		androidID = defaultAndroidID
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	auth := &GooglePhotosAuth{
		email:       email,
		masterToken: masterToken,
		androidID:   androidID,
		httpClient:  httpClient,
	}

	if privateKeyHex != "" {
		privateKey, err := auth.createPrivateKey(privateKeyHex)
		if err != nil {
			return nil, fmt.Errorf("failed to create private key: %w", err)
		}
		auth.privateKey = privateKey
		auth.issuer = auth.getIssuer()

		// Log public key coordinates for debugging (compare with Python output)
		fs.Infof(nil, "gphoto_auth: public key X (first 8 bytes): %x", privateKey.X.Bytes()[:8])
		fs.Infof(nil, "gphoto_auth: public key Y (first 8 bytes): %x", privateKey.Y.Bytes()[:8])
		fs.Infof(nil, "gphoto_auth: computed issuer=%s", auth.issuer)
	}

	return auth, nil
}

// createPrivateKey creates an ECDSA private key from a hex string.
func (a *GooglePhotosAuth) createPrivateKey(hexStr string) (*ecdsa.PrivateKey, error) {
	s := new(big.Int)
	s.SetString(hexStr, 16)

	curve := elliptic.P256()
	x, y := curve.ScalarBaseMult(s.Bytes())

	return &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: curve,
			X:     x,
			Y:     y,
		},
		D: s,
	}, nil
}

// getIssuer computes the issuer string from the public key.
// Uses standard x509.MarshalPKIXPublicKey for proper DER-encoded SubjectPublicKeyInfo.
func (a *GooglePhotosAuth) getIssuer() string {
	// Marshal public key to DER-encoded SubjectPublicKeyInfo (SPKI) format
	// This is equivalent to Python's:
	// public_key.public_bytes(encoding=DER, format=SubjectPublicKeyInfo)
	spki, err := x509.MarshalPKIXPublicKey(&a.privateKey.PublicKey)
	if err != nil {
		fs.Errorf(nil, "gphoto_auth: failed to marshal public key: %v", err)
		return ""
	}

	// Log SPKI hex for debugging (compare with Python's output)
	fs.Infof(nil, "gphoto_auth: SPKI length=%d, first 32 bytes: %x", len(spki), spki[:32])
	fs.Infof(nil, "gphoto_auth: SPKI last 32 bytes: %x", spki[len(spki)-32:])

	hash := sha256.Sum256(spki)
	fs.Infof(nil, "gphoto_auth: SPKI SHA256: %x", hash[:])
	return base64URLEncode(hash[:])
}

// signJWT signs a JWT with ES256.
// The payload must be serialized in the exact order Python uses (insertion order).
func (a *GooglePhotosAuth) signJWT(payload map[string]interface{}) (string, error) {
	// Use fixed header JSON to match Python's output exactly
	headerJSON := []byte(`{"alg":"ES256","typ":"JWT"}`)

	// Build payload JSON manually to match Python's key order exactly
	var payloadJSON []byte
	if ephKey, ok := payload["ephemeral_key"].(map[string]interface{}); ok {
		payloadJSON = []byte(fmt.Sprintf(
			`{"namespace":%q,"aud":%q,"iss":%q,"iat":%d,"ephemeral_key":{"kty":%q,"TinkKeysetPublicKeyInfo":%q}}`,
			payload["namespace"],
			payload["aud"],
			payload["iss"],
			int64(payload["iat"].(int64)),
			ephKey["kty"],
			ephKey["TinkKeysetPublicKeyInfo"],
		))
	} else {
		payloadJSON = []byte(fmt.Sprintf(
			`{"namespace":%q,"aud":%q,"iss":%q,"iat":%d}`,
			payload["namespace"],
			payload["aud"],
			payload["iss"],
			int64(payload["iat"].(int64)),
		))
	}

	headerB64 := base64URLEncode(headerJSON)
	payloadB64 := base64URLEncode(payloadJSON)
	message := []byte(headerB64 + "." + payloadB64)
	hash := sha256.Sum256(message)

	r, s, err := ecdsa.Sign(rand.Reader, a.privateKey, hash[:])
	if err != nil {
		return "", err
	}

	rBytes := r.Bytes()
	sBytes := s.Bytes()
	sigBytes := make([]byte, 64)
	copy(sigBytes[32-len(rBytes):32], rBytes)
	copy(sigBytes[64-len(sBytes):64], sBytes)
	sigB64 := base64URLEncode(sigBytes)

	// Self-verify the signature to ensure it's valid
	valid := ecdsa.Verify(&a.privateKey.PublicKey, hash[:], r, s)
	fs.Infof(nil, "gphoto_auth: signature self-verify: %v", valid)

	jwt := headerB64 + "." + payloadB64 + "." + sigB64

	// Print ALL debug info at once
	fs.Infof(nil, "=== JWT DEBUG START ===")
	fs.Infof(nil, "header_json: %s", string(headerJSON))
	fs.Infof(nil, "payload_json: %s", string(payloadJSON))
	fs.Infof(nil, "header_b64: %s", headerB64)
	fs.Infof(nil, "payload_b64: %s", payloadB64)
	fs.Infof(nil, "message_hash: %x", hash[:])
	fs.Infof(nil, "sig_r: %x", sigBytes[:32])
	fs.Infof(nil, "sig_s: %x", sigBytes[32:])
	fs.Infof(nil, "sig_b64: %s", sigB64)
	fs.Infof(nil, "full_jwt: %s", jwt)
	fs.Infof(nil, "=== JWT DEBUG END ===")

	return jwt, nil
}

// generateEphemeralKey generates an ephemeral key and stores the private key for later decryption.
func (a *GooglePhotosAuth) generateEphemeralKey() (map[string]interface{}, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	a.ephemeralPrivateKey = privateKey

	xBytes := privateKey.X.Bytes()
	yBytes := privateKey.Y.Bytes()

	// Pad to 32 bytes
	xPadded := make([]byte, 32)
	yPadded := make([]byte, 32)
	copy(xPadded[32-len(xBytes):], xBytes)
	copy(yPadded[32-len(yBytes):], yBytes)

	// Generate random key ID
	keyIDBytes := make([]byte, 4)
	rand.Read(keyIDBytes)
	keyID := binary.BigEndian.Uint32(keyIDBytes)

	keysetBytes := encodeTinkKeyset(keyID, xPadded, yPadded)

	// Log ephemeral key details for debugging
	fs.Infof(nil, "=== EPHEMERAL KEY DEBUG ===")
	fs.Infof(nil, "ephemeral private D: %x", privateKey.D.Bytes())
	fs.Infof(nil, "ephemeral public X: %x", xPadded)
	fs.Infof(nil, "ephemeral public Y: %x", yPadded)
	fs.Infof(nil, "keyID: %08x", keyID)
	fs.Infof(nil, "keyset bytes: %x", keysetBytes)
	fs.Infof(nil, "keyset b64: %s", base64URLEncode(keysetBytes))
	fs.Infof(nil, "=== END EPHEMERAL KEY DEBUG ===")

	return map[string]interface{}{
		"kty":                     "type.googleapis.com/google.crypto.tink.EciesAeadHkdfPublicKey",
		"TinkKeysetPublicKeyInfo": base64URLEncode(keysetBytes),
	}, nil
}

// decryptToken decrypts an encrypted token using Tink ECIES-AEAD-HKDF.
func (a *GooglePhotosAuth) decryptToken(encryptedToken, itMetadata string) (string, error) {
	if a.ephemeralPrivateKey == nil {
		fs.Errorf(nil, "gphoto_auth: decrypt failed - no ephemeral private key")
		return "", errors.New("no ephemeral private key available")
	}

	ciphertext, err := base64URLDecode(encryptedToken)
	if err != nil {
		fs.Errorf(nil, "gphoto_auth: decrypt failed - base64 decode error: %v", err)
		return encryptedToken, nil
	}

	fs.Infof(nil, "gphoto_auth: decrypt - ciphertext length: %d", len(ciphertext))

	// Minimum size: 5 (prefix) + 65 (EC point) + 12 (IV) + 16 (tag)
	if len(ciphertext) < 98 {
		fs.Errorf(nil, "gphoto_auth: decrypt failed - ciphertext too short: %d < 98", len(ciphertext))
		return encryptedToken, nil
	}

	// Skip Tink prefix (5 bytes: 1 version + 4 key_id)
	// Encapsulated key: uncompressed EC point (65 bytes: 0x04 + 32x + 32y)
	senderPubBytes := ciphertext[5:70]
	aesCiphertext := ciphertext[70:]

	fs.Infof(nil, "gphoto_auth: decrypt - prefix: %x, EC point first byte: %02x", ciphertext[:5], senderPubBytes[0])

	if senderPubBytes[0] != 0x04 {
		fs.Errorf(nil, "gphoto_auth: decrypt failed - expected EC point 0x04, got 0x%02x", senderPubBytes[0])
		return encryptedToken, nil
	}

	// Extract X and Y coordinates
	senderX := new(big.Int).SetBytes(senderPubBytes[1:33])
	senderY := new(big.Int).SetBytes(senderPubBytes[33:65])

	// Reconstruct sender's public key
	senderPubKey := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     senderX,
		Y:     senderY,
	}

	// ECDH to get shared secret
	sharedX, _ := elliptic.P256().ScalarMult(senderPubKey.X, senderPubKey.Y, a.ephemeralPrivateKey.D.Bytes())
	sharedSecret := sharedX.Bytes()

	// Pad to 32 bytes
	if len(sharedSecret) < 32 {
		padded := make([]byte, 32)
		copy(padded[32-len(sharedSecret):], sharedSecret)
		sharedSecret = padded
	}

	// Log all intermediate values for comparison with Python
	fs.Infof(nil, "=== DECRYPT DEBUG START ===")
	fs.Infof(nil, "senderPubBytes (65 bytes): %x", senderPubBytes)
	fs.Infof(nil, "senderX: %x", senderX.Bytes())
	fs.Infof(nil, "senderY: %x", senderY.Bytes())
	fs.Infof(nil, "ephemeralPrivateKey.D: %x", a.ephemeralPrivateKey.D.Bytes())
	fs.Infof(nil, "sharedSecret (32 bytes): %x", sharedSecret)

	// HKDF key derivation (verified to match Python):
	// IKM = sender_pub_bytes || shared_secret
	// Salt = empty (RFC 5869: empty salt treated as hash-length zeros)
	// Info = empty
	// IMPORTANT: Must copy senderPubBytes to avoid mutating ciphertext slice!
	hkdfIKM := make([]byte, len(senderPubBytes)+len(sharedSecret))
	copy(hkdfIKM, senderPubBytes)
	copy(hkdfIKM[len(senderPubBytes):], sharedSecret)
	fs.Infof(nil, "HKDF: IKM=senderPub||sharedSecret (%d bytes)", len(hkdfIKM))

	hkdfReader := hkdf.New(sha256.New, hkdfIKM, nil, nil)
	aesKey := make([]byte, 16) // AES-128
	if _, err := io.ReadFull(hkdfReader, aesKey); err != nil {
		fs.Errorf(nil, "gphoto_auth: decrypt failed - HKDF error: %v", err)
		return encryptedToken, nil
	}

	fs.Infof(nil, "aesKey (16 bytes): %x", aesKey)
	fs.Infof(nil, "aesCiphertext length: %d", len(aesCiphertext))
	fs.Infof(nil, "aesCiphertext first 20 bytes: %x", aesCiphertext[:min(20, len(aesCiphertext))])
	fs.Infof(nil, "=== DECRYPT DEBUG END ===")

	// AES-GCM decryption
	// Tink's inner AEAD (AES-GCM) also has a 5-byte prefix (version + key_id) before IV
	// So structure is: [5-byte prefix][12-byte IV][ciphertext][16-byte tag]
	// Total minimum: 5 + 12 + 16 = 33 bytes
	if len(aesCiphertext) < 33 {
		// Try without inner prefix (some implementations don't have it)
		if len(aesCiphertext) < 28 { // 12 (IV) + 16 (tag minimum)
			fs.Errorf(nil, "gphoto_auth: decrypt failed - aesCiphertext too short: %d", len(aesCiphertext))
			return encryptedToken, nil
		}
	}

	// Try both with and without inner 5-byte prefix
	var nonce, ciphertextWithTag []byte
	var decryptError error
	var tokenBytes []byte

	// First try: skip inner 5-byte prefix (Tink full format)
	if len(aesCiphertext) >= 33 {
		fs.Infof(nil, "gphoto_auth: trying with 5-byte inner prefix skip")
		innerPrefix := aesCiphertext[0:5]
		fs.Infof(nil, "gphoto_auth: inner prefix: %x", innerPrefix)
		nonce = aesCiphertext[5:17]
		ciphertextWithTag = aesCiphertext[17:]

		block, err := aes.NewCipher(aesKey)
		if err == nil {
			aesGCM, err := cipher.NewGCM(block)
			if err == nil {
				tokenBytes, decryptError = aesGCM.Open(nil, nonce, ciphertextWithTag, nil)
				if decryptError == nil {
					goto success
				}
				fs.Infof(nil, "gphoto_auth: with inner prefix failed: %v, trying without", decryptError)
			}
		}
	}

	// Second try: no inner prefix
	fs.Infof(nil, "gphoto_auth: trying without inner prefix")
	nonce = aesCiphertext[0:12]
	ciphertextWithTag = aesCiphertext[12:]

	{
		block, err := aes.NewCipher(aesKey)
		if err != nil {
			fs.Errorf(nil, "gphoto_auth: decrypt failed - AES cipher error: %v", err)
			return encryptedToken, nil
		}

		aesGCM, err := cipher.NewGCM(block)
		if err != nil {
			fs.Errorf(nil, "gphoto_auth: decrypt failed - GCM error: %v", err)
			return encryptedToken, nil
		}

		tokenBytes, decryptError = aesGCM.Open(nil, nonce, ciphertextWithTag, nil)
		if decryptError != nil {
			fs.Errorf(nil, "gphoto_auth: decrypt failed - GCM decrypt error: %v", decryptError)
			return encryptedToken, nil
		}
	}

success:
	tokenStr := string(tokenBytes)
	fs.Infof(nil, "gphoto_auth: decrypt SUCCESS - token prefix: %.20s...", tokenStr)

	// Apply microg-style processing for ya29.m. tokens
	if strings.HasPrefix(tokenStr, "ya29.m.") && itMetadata != "" {
		fs.Infof(nil, "gphoto_auth: applying microg-style processing")
		processed, err := a.processTokenMicrogStyle(tokenStr, itMetadata)
		if err != nil {
			fs.Errorf(nil, "gphoto_auth: microg processing failed: %v", err)
		} else {
			tokenStr = processed
			fs.Infof(nil, "gphoto_auth: microg processing SUCCESS")
		}
	}

	return tokenStr, nil
}

// processTokenMicrogStyle processes token using microg-style HMAC signature recalculation.
func (a *GooglePhotosAuth) processTokenMicrogStyle(tokenStr, itMetadata string) (string, error) {
	// Decode the proto part of the token
	protoB64 := tokenStr[7:] // Skip "ya29.m."
	protoBytes, err := base64URLDecode(protoB64)
	if err != nil {
		return tokenStr, err
	}

	// Parse all fields from the decrypted token
	fields := authParseProtoFields(protoBytes)

	// Extract field 1 (auth) and field 3 (HMAC key)
	field1, ok1 := fields[1]
	if !ok1 || field1.wireType != 2 {
		return tokenStr, errors.New("field 1 not found")
	}
	authBytes := field1.data

	field3, ok3 := fields[3]
	if !ok3 || field3.wireType != 2 {
		return tokenStr, errors.New("field 3 not found")
	}
	hmacKey := field3.data

	// Parse itMetadata to get effectiveDurationSeconds (field 4)
	metaBytes, err := base64URLDecode(itMetadata)
	if err != nil {
		return tokenStr, err
	}
	metaFields := authParseProtoFields(metaBytes)

	effectiveDuration := uint64(3660) // Default
	if field4, ok := metaFields[4]; ok && field4.wireType == 0 {
		effectiveDuration = field4.value
	}

	// Build OAuthAuthorization: {2: effectiveDurationSeconds}
	oauthAuth := authEncodeVarintField(2, effectiveDuration)

	// Build OAuthTokenData: {1: fieldType=1 (SCOPE), 2: authorization, 3: durationMillis=0}
	oauthTokenData := authEncodeVarintField(1, 1) // fieldType = 1 (SCOPE)
	oauthTokenData = append(oauthTokenData, authEncodeBytesField(2, oauthAuth)...)
	oauthTokenData = append(oauthTokenData, authEncodeVarintField(3, 0)...) // durationMillis = 0

	// HMAC-SHA256 sign the OAuthTokenData using field 3 as key
	mac := hmac.New(sha256.New, hmacKey)
	mac.Write(oauthTokenData)
	newSignature := mac.Sum(nil)

	// Build new ItAuthData: {1: auth, 2: OAuthTokenData, 3: new_signature}
	newProto := authEncodeBytesField(1, authBytes)
	newProto = append(newProto, authEncodeBytesField(2, oauthTokenData)...)
	newProto = append(newProto, authEncodeBytesField(3, newSignature)...)

	// Append remaining fields (4, 5, 6, etc.) from original token
	for fieldNum := 4; fieldNum <= 10; fieldNum++ {
		if field, ok := fields[fieldNum]; ok {
			if field.wireType == 0 {
				newProto = append(newProto, authEncodeVarintField(fieldNum, field.value)...)
			} else if field.wireType == 2 {
				newProto = append(newProto, authEncodeBytesField(fieldNum, field.data)...)
			}
		}
	}

	// Encode the new token
	newToken := "ya29.m." + base64URLEncode(newProto)
	return newToken, nil
}

// buildRequestData builds the request data for the auth endpoint.
func (a *GooglePhotosAuth) buildRequestData(withJWT bool) (url.Values, error) {
	data := url.Values{
		"androidId":                    {a.androidID},
		"lang":                         {"en-US"},
		"google_play_services_version": {gmsVersion},
		"sdk_version":                  {sdkVersion},
		"device_country":               {"us"},
		"it_caveat_types":              {"2"},
		"app":                          {"com.google.android.apps.photos"},
		"oauth2_foreground":            {"1"},
		"Email":                        {a.email},
		"pkgVersionCode":               {photosVersion},
		"has_permission":               {"1"},
		"token_request_options":        {"CAA4AVABYAA="},
		"client_sig":                   {photosClientSig},
		"Token":                        {a.masterToken},
		"consumerVersionCode":          {photosVersion},
		"check_email":                  {"1"},
		"service":                      {"oauth2:openid https://www.googleapis.com/auth/mobileapps.native https://www.googleapis.com/auth/photos.native"},
		"callerPkg":                    {"com.google.android.apps.photos"},
		"check_tb_upgrade_eligible":    {"1"},
		"callerSig":                    {photosClientSig},
	}

	if withJWT && a.privateKey != nil {
		payload := map[string]interface{}{
			"namespace": "TokenBinding",
			"aud":       "https://accounts.google.com/accountmanager",
			"iss":       a.issuer,
			"iat":       time.Now().Unix(),
		}

		ephemeralKey, err := a.generateEphemeralKey()
		if err != nil {
			return nil, err
		}
		payload["ephemeral_key"] = ephemeralKey

		jwt, err := a.signJWT(payload)
		if err != nil {
			return nil, err
		}
		data.Set("assertion_jwt", jwt)

		// Log the full JWT for debugging
		fs.Infof(nil, "gphoto_auth: assertion_jwt (first 100 chars): %.100s...", jwt)
	}

	return data, nil
}

// parseAuthResponse parses the key=value response from Google's auth endpoint.
func parseAuthResponse(responseText string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(responseText), "\n") {
		if idx := strings.Index(line, "="); idx != -1 {
			key := line[:idx]
			value := line[idx+1:]
			result[key] = value
		}
	}
	return result
}

// GetToken gets an OAuth token. Tries without JWT first, falls back to JWT if needed.
func (a *GooglePhotosAuth) GetToken(ctx context.Context) (*TokenResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Check if we've had too many consecutive auth failures
	if a.authFailCount >= 3 {
		fs.Errorf(nil, "gphoto_auth: FATAL - %d consecutive auth failures, exiting to prevent flooding Google servers", a.authFailCount)
		fs.Errorf(nil, "gphoto_auth: Please check your master_token and private_key_s configuration")
		panic("gphoto_auth: too many consecutive auth failures - check credentials")
	}

	fs.Infof(nil, "gphoto_auth: requesting token for %s (has_private_key=%v)", a.email, a.privateKey != nil)

	headers := map[string]string{
		"Content-Type":    "application/x-www-form-urlencoded",
		"User-Agent":      "GoogleAuth/1.4 (generic_x86 PPR1.180610.011); gzip",
		"Accept-Encoding": "gzip",
		"Connection":      "Keep-Alive",
	}

	var result map[string]string
	var data url.Values
	var err error

	if a.privateKey != nil {
		data, err = a.buildRequestData(true)
		if err != nil {
			return nil, err
		}
	} else {
		data, err = a.buildRequestData(false)
		if err != nil {
			return nil, err
		}
	}

	result, err = a.doAuthRequest(ctx, data, headers)
	if err != nil {
		return nil, err
	}

	// Retry with JWT if initial request failed and we have a private key
	if _, hasError := result["Error"]; hasError && a.privateKey != nil {
		if _, hasJWT := data["assertion_jwt"]; !hasJWT {
			fs.Infof(nil, "gphoto_auth: retrying with JWT assertion")
			data, err = a.buildRequestData(true)
			if err != nil {
				return nil, err
			}
			result, err = a.doAuthRequest(ctx, data, headers)
			if err != nil {
				return nil, err
			}
		}
	}

	// Build response
	if token, ok := result["it"]; ok {
		fs.Infof(nil, "gphoto_auth: obtained token (encrypted=%s)", result["TokenEncrypted"])
		// Log the raw encrypted token for Python debugging
		fs.Infof(nil, "=== RAW IT VALUE FOR PYTHON DEBUG ===")
		fs.Infof(nil, "it=%s", token)
		fs.Infof(nil, "=== END RAW IT VALUE ===")

		// Reset failure counter on success
		a.authFailCount = 0

		// Decrypt if token is encrypted
		if result["TokenEncrypted"] == "1" && a.ephemeralPrivateKey != nil {
			decrypted, _ := a.decryptToken(token, result["itMetadata"])
			token = decrypted
		}

		expiry := int64(0)
		if expiryStr, ok := result["Expiry"]; ok {
			fmt.Sscanf(expiryStr, "%d", &expiry)
			expiry *= 1000 // Convert to ms
		}

		return &TokenResult{
			Token:  token,
			Expiry: expiry,
			Scopes: result["grantedScopes"],
			Error:  "",
		}, nil
	}

	// Auth failed - increment failure counter
	a.authFailCount++

	errorMsg := result["Error"]
	if errorMsg == "" {
		// No Error key found - check what keys we did get
		if len(result) == 0 {
			errorMsg = "Empty or unparseable response (raw body logged above)"
		} else {
			// List all keys we found for debugging
			var keys []string
			for k := range result {
				keys = append(keys, k)
			}
			errorMsg = fmt.Sprintf("No Error key in response; keys found: %v", keys)
		}
	}
	fs.Errorf(nil, "gphoto_auth: token request failed (attempt %d/3): %s", a.authFailCount, errorMsg)
	return nil, fmt.Errorf("token request failed: %s", errorMsg)
}

// doAuthRequest performs the HTTP request to Google's auth endpoint.
func (a *GooglePhotosAuth) doAuthRequest(ctx context.Context, data url.Values, headers map[string]string) (map[string]string, error) {
	encodedData := data.Encode()

	// Log request details for debugging
	fs.Infof(nil, "=== REQUEST DEBUG ===")
	fs.Infof(nil, "Token (first 50): %.50s...", data.Get("Token"))
	fs.Infof(nil, "Email: %s", data.Get("Email"))
	fs.Infof(nil, "assertion_jwt present: %v", data.Get("assertion_jwt") != "")
	fs.Infof(nil, "Request body length: %d", len(encodedData))
	fs.Infof(nil, "=== END REQUEST DEBUG ===")

	req, err := http.NewRequestWithContext(ctx, "POST", "https://android.googleapis.com/auth", strings.NewReader(encodedData))
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Handle gzip decompression if response is compressed
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	fs.Infof(nil, "gphoto_auth: response status=%d, body_len=%d", resp.StatusCode, len(body))

	// For non-200 status codes, log the raw response body to help debugging
	if resp.StatusCode != http.StatusOK {
		// Log the raw body (truncate if too long)
		bodyStr := string(body)
		if len(bodyStr) > 500 {
			bodyStr = bodyStr[:500] + "..."
		}
		fs.Errorf(nil, "gphoto_auth: error response (status=%d): %s", resp.StatusCode, bodyStr)
	}

	return parseAuthResponse(string(body)), nil
}

// HasTokenBinding returns whether token binding is configured.
func (a *GooglePhotosAuth) HasTokenBinding() bool {
	return a.privateKey != nil
}

// AccountCredentials holds the credentials for an account.
type AccountCredentials struct {
	MasterToken string `json:"master_token"`
	PrivateKeyS string `json:"private_key_s,omitempty"` // hex-encoded private key scalar (optional)
}

// TokenManager manages tokens for multiple accounts with caching.
type TokenManager struct {
	accounts    map[string]*AccountCredentials
	cache       map[string]*cachedToken
	authClients map[string]*GooglePhotosAuth
	httpClient  *http.Client
	mu          sync.RWMutex
}

// cachedToken holds a cached token with its expiry.
type cachedToken struct {
	token  string
	expiry int64 // milliseconds
}

// NewTokenManager creates a new TokenManager instance.
func NewTokenManager(accounts map[string]*AccountCredentials, httpClient *http.Client) (*TokenManager, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	tm := &TokenManager{
		accounts:    accounts,
		cache:       make(map[string]*cachedToken),
		authClients: make(map[string]*GooglePhotosAuth),
		httpClient:  httpClient,
	}

	for key, creds := range accounts {
		username := key
		if idx := strings.Index(key, "@"); idx != -1 {
			username = key[:idx]
		}
		email := username + "@gmail.com"

		auth, err := NewGooglePhotosAuth(email, creds.MasterToken, "", creds.PrivateKeyS, httpClient)
		if err != nil {
			return nil, fmt.Errorf("failed to create auth client for %s: %w", username, err)
		}
		tm.authClients[username] = auth
	}

	return tm, nil
}

// normalizeUser extracts the username from an email or returns the username as-is.
func normalizeUser(user string) string {
	if idx := strings.Index(user, "@"); idx != -1 {
		return user[:idx]
	}
	return user
}

// isTokenValid checks if a cached token is still valid.
func (tm *TokenManager) isTokenValid(username string, bufferMs int64) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	cached, ok := tm.cache[username]
	if !ok {
		return false
	}
	nowMs := time.Now().UnixMilli()
	return nowMs < (cached.expiry - bufferMs)
}

// GetCachedToken returns a cached token if valid, otherwise returns empty string.
func (tm *TokenManager) GetCachedToken(user string) string {
	username := normalizeUser(user)
	if tm.isTokenValid(username, 60000) {
		tm.mu.RLock()
		defer tm.mu.RUnlock()
		if cached, ok := tm.cache[username]; ok {
			return cached.token
		}
	}
	return ""
}

// GetToken gets a token for a user, using cache if valid.
func (tm *TokenManager) GetToken(ctx context.Context, user string, force bool) (*TokenResult, error) {
	username := normalizeUser(user)

	tm.mu.RLock()
	auth, ok := tm.authClients[username]
	tm.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown account: %s", username)
	}

	if !force && tm.isTokenValid(username, 60000) {
		tm.mu.RLock()
		cached := tm.cache[username]
		tm.mu.RUnlock()
		return &TokenResult{
			Token:  cached.token,
			Expiry: cached.expiry,
			Source: "cache",
		}, nil
	}

	result, err := auth.GetToken(ctx)
	if err != nil {
		return nil, err
	}

	if result.Error != "" {
		return nil, fmt.Errorf("token fetch failed: %s", result.Error)
	}

	tm.mu.Lock()
	tm.cache[username] = &cachedToken{
		token:  result.Token,
		expiry: result.Expiry,
	}
	tm.mu.Unlock()

	result.Source = "fresh"
	return result, nil
}
