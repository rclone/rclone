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
	"encoding/json"
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

// Device constants matching the Python implementation
const (
	defaultAndroidID = "38106360b2a855e1"
	gmsVersion       = "254730032"
	sdkVersion       = "33"
	photosVersion    = "51079550"
	// Photos app client signature (for photos.native scope)
	photosClientSig = "24bb24c05e47e0aefa68a58a766179d9b613a600"
)

// GooglePhotosAuth handles Google Photos OAuth token generation with token binding
type GooglePhotosAuth struct {
	email       string
	masterToken string
	androidID   string
	privateKey  *ecdsa.PrivateKey
	issuer      string
	httpClient  *http.Client

	// Ephemeral key for token decryption (regenerated per request)
	ephemeralPrivateKey *ecdsa.PrivateKey
	ephemeralMu         sync.Mutex
}

// NewGooglePhotosAuth creates a new Google Photos auth client
func NewGooglePhotosAuth(email, masterToken, androidID, privateKeyHex string, httpClient *http.Client) (*GooglePhotosAuth, error) {
	auth := &GooglePhotosAuth{
		email:       email,
		masterToken: masterToken,
		androidID:   androidID,
		httpClient:  httpClient,
	}

	if androidID == "" {
		auth.androidID = defaultAndroidID
	}

	if privateKeyHex != "" {
		privateKey, err := createPrivateKey(privateKeyHex)
		if err != nil {
			return nil, fmt.Errorf("failed to create private key: %w", err)
		}
		auth.privateKey = privateKey
		auth.issuer = auth.computeIssuer()
	}

	return auth, nil
}

// createPrivateKey creates an ECDSA P-256 private key from hex scalar
func createPrivateKey(hexScalar string) (*ecdsa.PrivateKey, error) {
	// Parse hex string to big.Int
	s := new(big.Int)
	_, ok := s.SetString(hexScalar, 16)
	if !ok {
		return nil, fmt.Errorf("invalid hex scalar")
	}

	// Create P-256 private key
	curve := elliptic.P256()
	x, y := curve.ScalarBaseMult(s.Bytes())

	privateKey := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: curve,
			X:     x,
			Y:     y,
		},
		D: s,
	}

	return privateKey, nil
}

// computeIssuer computes the issuer (SHA256 hash of DER-encoded public key, base64url encoded)
func (auth *GooglePhotosAuth) computeIssuer() string {
	// Use x509 to properly marshal the public key to PKIX/SubjectPublicKeyInfo format
	// This matches Python's: public_key.public_bytes(encoding=DER, format=SubjectPublicKeyInfo)
	pubKeyDER, err := x509.MarshalPKIXPublicKey(&auth.privateKey.PublicKey)
	if err != nil {
		fs.Errorf(nil, "gphoto_auth: failed to marshal public key: %v", err)
		return ""
	}

	hash := sha256.Sum256(pubKeyDER)
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// base64URLEncode encodes bytes to URL-safe base64 without padding
func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

// base64URLDecode decodes URL-safe base64 (with or without padding)
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

// signJWT creates and signs a JWT with ES256
func (auth *GooglePhotosAuth) signJWT(payload map[string]interface{}) (string, error) {
	header := map[string]string{
		"alg": "ES256",
		"typ": "JWT",
	}

	headerJSON, _ := json.Marshal(header)
	payloadJSON, _ := json.Marshal(payload)

	headerB64 := base64URLEncode(headerJSON)
	payloadB64 := base64URLEncode(payloadJSON)

	message := headerB64 + "." + payloadB64

	// Sign with ECDSA
	hash := sha256.Sum256([]byte(message))
	r, s, err := ecdsa.Sign(rand.Reader, auth.privateKey, hash[:])
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	// Convert to fixed-size byte arrays (32 bytes each for P-256)
	sigBytes := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(sigBytes[32-len(rBytes):32], rBytes)
	copy(sigBytes[64-len(sBytes):], sBytes)

	sigB64 := base64URLEncode(sigBytes)

	return message + "." + sigB64, nil
}

// encodeTinkKeyset encodes an ECIES-AEAD-HKDF public key in Tink keyset format
func encodeTinkKeyset(keyID uint32, xBytes, yBytes []byte) []byte {
	// Type URLs
	eciesTypeURL := []byte("type.googleapis.com/google.crypto.tink.EciesAeadHkdfPublicKey")
	aesGcmTypeURL := []byte("type.googleapis.com/google.crypto.tink.AesGcmKey")

	enc := NewProtoEncoder()

	// Build KeyTemplate (for AES-128-GCM)
	keyTemplateValue := NewProtoEncoder()
	keyTemplateValue.EncodeInt32(2, 16) // key_size = 16

	keyTemplate := NewProtoEncoder()
	keyTemplate.EncodeBytes(1, aesGcmTypeURL)
	keyTemplate.EncodeMessage(2, keyTemplateValue.Bytes())
	keyTemplate.EncodeInt32(3, 1) // output_prefix_type = TINK

	// kem_params: {1: 2, 2: 3} (curve=P256, hash=SHA256)
	kemParams := NewProtoEncoder()
	kemParams.EncodeInt32(1, 2) // curve = P256
	kemParams.EncodeInt32(2, 3) // hash = SHA256

	// dem_params: {2: KeyTemplate}
	demParams := NewProtoEncoder()
	demParams.EncodeMessage(2, keyTemplate.Bytes())

	// params (NESTED): {1: kem_params, 2: dem_params, 3: 1}
	params := NewProtoEncoder()
	params.EncodeMessage(1, kemParams.Bytes())
	params.EncodeMessage(2, demParams.Bytes())
	params.EncodeInt32(3, 1) // ec_point_format = UNCOMPRESSED

	// Always prepend 0x00 to coordinates (Google's Tink impl always adds this prefix)
	xEncoded := append([]byte{0x00}, xBytes...)
	yEncoded := append([]byte{0x00}, yBytes...)

	// EciesAeadHkdfPublicKey value: {2: params, 3: x, 4: y}
	pubKeyValue := NewProtoEncoder()
	pubKeyValue.EncodeMessage(2, params.Bytes())
	pubKeyValue.EncodeBytes(3, xEncoded)
	pubKeyValue.EncodeBytes(4, yEncoded)

	// keyData: {1: type_url, 2: pub_key_value, 3: 3}
	keyData := NewProtoEncoder()
	keyData.EncodeBytes(1, eciesTypeURL)
	keyData.EncodeMessage(2, pubKeyValue.Bytes())
	keyData.EncodeInt32(3, 3) // key_material_type = ASYMMETRIC_PUBLIC

	// key message: {1: keyData, 2: 1, 3: key_id, 4: 1}
	keyMsg := NewProtoEncoder()
	keyMsg.EncodeMessage(1, keyData.Bytes())
	keyMsg.EncodeInt32(2, 1)          // status = ENABLED
	keyMsg.EncodeInt32(3, int32(keyID)) // key_id
	keyMsg.EncodeInt32(4, 1)          // output_prefix_type = TINK

	// keyset: {1: key_id, 2: key_msg}
	enc.EncodeInt32(1, int32(keyID))
	enc.EncodeMessage(2, keyMsg.Bytes())

	return enc.Bytes()
}

// generateEphemeralKey generates an ephemeral ECDSA key and returns Tink keyset info
func (auth *GooglePhotosAuth) generateEphemeralKey() (map[string]interface{}, error) {
	auth.ephemeralMu.Lock()
	defer auth.ephemeralMu.Unlock()

	// Generate new P-256 key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ephemeral key: %w", err)
	}
	auth.ephemeralPrivateKey = privateKey

	// Get public key coordinates (32 bytes each for P-256)
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
	keyID := uint32(keyIDBytes[0])<<24 | uint32(keyIDBytes[1])<<16 | uint32(keyIDBytes[2])<<8 | uint32(keyIDBytes[3])

	// Encode Tink keyset
	keysetBytes := encodeTinkKeyset(keyID, xPadded, yPadded)

	return map[string]interface{}{
		"kty":                    "type.googleapis.com/google.crypto.tink.EciesAeadHkdfPublicKey",
		"TinkKeysetPublicKeyInfo": base64URLEncode(keysetBytes),
	}, nil
}

// decryptToken decrypts an encrypted token using ECIES-AEAD-HKDF
func (auth *GooglePhotosAuth) decryptToken(encryptedToken, itMetadata string) (string, error) {
	auth.ephemeralMu.Lock()
	ephemeralKey := auth.ephemeralPrivateKey
	auth.ephemeralMu.Unlock()

	if ephemeralKey == nil {
		return encryptedToken, nil
	}

	ciphertext, err := base64URLDecode(encryptedToken)
	if err != nil {
		fs.Debugf(nil, "gphoto_auth: failed to decode encrypted token: %v", err)
		return encryptedToken, nil
	}

	// Tink ciphertext format:
	// - prefix (5 bytes): 1 byte version (0x01) + 4 bytes key_id
	// - encapsulated_key (65 bytes): 0x04 + 32-byte X + 32-byte Y
	// - encrypted_data: IV (12 bytes) + ciphertext + tag (16 bytes)

	if len(ciphertext) < 5+65+12+16 {
		fs.Debugf(nil, "gphoto_auth: ciphertext too short: %d bytes", len(ciphertext))
		return encryptedToken, nil
	}

	// Extract sender's public key (encapsulated key)
	senderPubBytes := ciphertext[5:70]
	aesCiphertext := ciphertext[70:]

	// Validate EC point format
	if senderPubBytes[0] != 0x04 {
		fs.Debugf(nil, "gphoto_auth: expected uncompressed EC point (0x04), got 0x%02x", senderPubBytes[0])
		return encryptedToken, nil
	}

	// Parse sender's public key
	senderX := new(big.Int).SetBytes(senderPubBytes[1:33])
	senderY := new(big.Int).SetBytes(senderPubBytes[33:65])

	senderPubKey := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     senderX,
		Y:     senderY,
	}

	// ECDH: compute shared secret (x-coordinate of shared point)
	sharedX, _ := elliptic.P256().ScalarMult(senderPubKey.X, senderPubKey.Y, ephemeralKey.D.Bytes())
	sharedSecret := sharedX.Bytes()
	// Pad to 32 bytes
	if len(sharedSecret) < 32 {
		padded := make([]byte, 32)
		copy(padded[32-len(sharedSecret):], sharedSecret)
		sharedSecret = padded
	}

	// HKDF key derivation per Tink spec
	// IKM = sender_pub_bytes + shared_secret
	hkdfIKM := append(senderPubBytes, sharedSecret...)

	// Derive AES-128-GCM key (16 bytes)
	hkdfReader := hkdf.New(sha256.New, hkdfIKM, nil, nil)
	aesKey := make([]byte, 16)
	if _, err := io.ReadFull(hkdfReader, aesKey); err != nil {
		fs.Debugf(nil, "gphoto_auth: HKDF failed: %v", err)
		return encryptedToken, nil
	}

	// AES-GCM decryption
	if len(aesCiphertext) < 12+16 {
		fs.Debugf(nil, "gphoto_auth: AES ciphertext too short: %d bytes", len(aesCiphertext))
		return encryptedToken, nil
	}

	nonce := aesCiphertext[0:12]
	ciphertextWithTag := aesCiphertext[12:]

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		fs.Debugf(nil, "gphoto_auth: failed to create AES cipher: %v", err)
		return encryptedToken, nil
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		fs.Debugf(nil, "gphoto_auth: failed to create GCM: %v", err)
		return encryptedToken, nil
	}

	plaintext, err := aesgcm.Open(nil, nonce, ciphertextWithTag, nil)
	if err != nil {
		fs.Debugf(nil, "gphoto_auth: AES-GCM decryption failed: %v", err)
		return encryptedToken, nil
	}

	tokenStr := string(plaintext)

	// Apply microg-style processing for ya29.m. tokens
	if strings.HasPrefix(tokenStr, "ya29.m.") && itMetadata != "" {
		processed := auth.processTokenMicrogStyle(tokenStr, itMetadata)
		if processed != "" {
			tokenStr = processed
		}
	}

	return tokenStr, nil
}

// parseProtoFields parses protobuf fields into a map
func parseProtoFields(data []byte) map[int]interface{} {
	fields := make(map[int]interface{})
	decoder := NewProtoDecoder(data)

	for {
		fieldNum, wireType, value, err := decoder.DecodeField()
		if err != nil {
			break
		}
		if wireType == 0 {
			fields[fieldNum] = value
		} else if wireType == 2 {
			fields[fieldNum] = value
		}
	}
	return fields
}

// processTokenMicrogStyle processes token using microg-style HMAC signature recalculation
func (auth *GooglePhotosAuth) processTokenMicrogStyle(tokenStr, itMetadata string) string {
	// Skip "ya29.m." prefix
	protoB64 := tokenStr[7:]
	protoBytes, err := base64URLDecode(protoB64)
	if err != nil {
		fs.Debugf(nil, "gphoto_auth: failed to decode token proto: %v", err)
		return tokenStr
	}

	// Parse fields from decrypted token
	fields := parseProtoFields(protoBytes)

	// Field 1 = auth bytes, Field 3 = HMAC key
	authBytes, ok := fields[1].([]byte)
	if !ok {
		fs.Debugf(nil, "gphoto_auth: field 1 not found or not bytes")
		return tokenStr
	}

	hmacKey, ok := fields[3].([]byte)
	if !ok {
		fs.Debugf(nil, "gphoto_auth: field 3 (HMAC key) not found or not bytes")
		return tokenStr
	}

	// Parse itMetadata to get effectiveDurationSeconds (field 4)
	metaBytes, err := base64URLDecode(itMetadata)
	if err != nil {
		fs.Debugf(nil, "gphoto_auth: failed to decode itMetadata: %v", err)
		return tokenStr
	}

	metaFields := parseProtoFields(metaBytes)
	effectiveDuration := int64(3660) // Default
	if duration, ok := metaFields[4].(uint64); ok {
		effectiveDuration = int64(duration)
	}

	// Build OAuthAuthorization: {2: effectiveDurationSeconds}
	oauthAuth := NewProtoEncoder()
	oauthAuth.EncodeInt64(2, effectiveDuration)

	// Build OAuthTokenData: {1: fieldType=1 (SCOPE), 2: authorization, 3: durationMillis=0}
	oauthTokenData := NewProtoEncoder()
	oauthTokenData.EncodeInt32(1, 1) // fieldType = 1 (SCOPE)
	oauthTokenData.EncodeMessage(2, oauthAuth.Bytes())
	oauthTokenData.EncodeInt64(3, 0) // durationMillis = 0

	// HMAC-SHA256 sign the OAuthTokenData using field 3 as key
	mac := hmac.New(sha256.New, hmacKey)
	mac.Write(oauthTokenData.Bytes())
	newSignature := mac.Sum(nil)

	// Build new ItAuthData: {1: auth, 2: OAuthTokenData, 3: new_signature}
	newProto := NewProtoEncoder()
	newProto.EncodeBytes(1, authBytes)
	newProto.EncodeMessage(2, oauthTokenData.Bytes())
	newProto.EncodeBytes(3, newSignature)

	// Append remaining fields (4, 5, 6, etc.) from original token
	for fieldNum := 4; fieldNum <= 10; fieldNum++ {
		if value, ok := fields[fieldNum]; ok {
			switch v := value.(type) {
			case uint64:
				newProto.EncodeInt64(fieldNum, int64(v))
			case []byte:
				newProto.EncodeBytes(fieldNum, v)
			}
		}
	}

	// Encode the new token
	newToken := "ya29.m." + base64URLEncode(newProto.Bytes())
	return newToken
}

// buildRequestData builds the form data for token request
func (auth *GooglePhotosAuth) buildRequestData(withJWT bool) url.Values {
	data := url.Values{}
	data.Set("androidId", auth.androidID)
	data.Set("lang", "en-US")
	data.Set("google_play_services_version", gmsVersion)
	data.Set("sdk_version", sdkVersion)
	data.Set("device_country", "us")
	data.Set("it_caveat_types", "2")
	data.Set("app", "com.google.android.apps.photos")
	data.Set("oauth2_foreground", "1")
	data.Set("Email", auth.email)
	data.Set("pkgVersionCode", photosVersion)
	data.Set("has_permission", "1")
	data.Set("token_request_options", "CAA4AVABYAA=")
	data.Set("client_sig", photosClientSig)
	data.Set("Token", auth.masterToken)
	data.Set("consumerVersionCode", photosVersion)
	data.Set("check_email", "1")
	data.Set("service", "oauth2:openid https://www.googleapis.com/auth/mobileapps.native https://www.googleapis.com/auth/photos.native")
	data.Set("callerPkg", "com.google.android.apps.photos")
	data.Set("check_tb_upgrade_eligible", "1")
	data.Set("callerSig", photosClientSig)

	if withJWT && auth.privateKey != nil {
		payload := map[string]interface{}{
			"namespace": "TokenBinding",
			"aud":       "https://accounts.google.com/accountmanager",
			"iss":       auth.issuer,
			"iat":       time.Now().Unix(),
		}

		// Generate ephemeral key
		ephemeralKey, err := auth.generateEphemeralKey()
		if err == nil {
			payload["ephemeral_key"] = ephemeralKey
		}

		jwt, err := auth.signJWT(payload)
		if err == nil {
			data.Set("assertion_jwt", jwt)
		}
	}

	return data
}

// parseResponse parses key=value response from Google auth
func parseResponse(responseText string) map[string]string {
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

// TokenResult contains the token response
type TokenResult struct {
	Token  string
	Expiry int64
	Scopes string
	Error  string
}

// GetToken fetches an OAuth token from Google
func (auth *GooglePhotosAuth) GetToken(ctx context.Context) (*TokenResult, error) {
	fs.Infof(nil, "gphoto_auth: requesting token for %s (has_private_key=%v)", auth.email, auth.privateKey != nil)

	headers := http.Header{}
	headers.Set("Content-Type", "application/x-www-form-urlencoded")
	headers.Set("User-Agent", "GoogleAuth/1.4 (generic_x86 PPR1.180610.011); gzip")
	headers.Set("Accept-Encoding", "gzip")
	headers.Set("Connection", "Keep-Alive")

	// Try with JWT if we have a private key
	data := auth.buildRequestData(auth.privateKey != nil)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://android.googleapis.com/auth", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header = headers

	resp, err := auth.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Handle gzip response
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
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Log raw response for debugging
	fs.Debugf(nil, "gphoto_auth: raw response (%d bytes): %s", len(body), string(body))

	result := parseResponse(string(body))

	// Log auth response for debugging
	fs.Infof(nil, "gphoto_auth: response status=%d, has_it=%v, has_error=%v", resp.StatusCode, result["it"] != "", result["Error"] != "")
	if result["Error"] != "" {
		fs.Errorf(nil, "gphoto_auth: token request error: %s", result["Error"])
	}

	// If error and we have private key but didn't try JWT, retry with JWT
	if result["Error"] != "" && auth.privateKey != nil && data.Get("assertion_jwt") == "" {
		fs.Infof(nil, "gphoto_auth: retrying with JWT assertion")
		data = auth.buildRequestData(true)

		req, err = http.NewRequestWithContext(ctx, "POST", "https://android.googleapis.com/auth", strings.NewReader(data.Encode()))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header = headers

		resp2, err := auth.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to send request: %w", err)
		}
		defer resp2.Body.Close()

		// Handle gzip response
		reader = resp2.Body
		if resp2.Header.Get("Content-Encoding") == "gzip" {
			gzReader, err := gzip.NewReader(resp2.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to create gzip reader: %w", err)
			}
			defer gzReader.Close()
			reader = gzReader
		}

		body, err = io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		fs.Debugf(nil, "gphoto_auth: retry raw response (%d bytes): %s", len(body), string(body))
		result = parseResponse(string(body))
		fs.Infof(nil, "gphoto_auth: retry response has_it=%v, has_error=%v", result["it"] != "", result["Error"] != "")
	}

	// Build response
	if token, ok := result["it"]; ok {
		fs.Infof(nil, "gphoto_auth: obtained token (encrypted=%s)", result["TokenEncrypted"])

		// Decrypt if token is encrypted
		if result["TokenEncrypted"] == "1" {
			decrypted, err := auth.decryptToken(token, result["itMetadata"])
			if err != nil {
				fs.Errorf(nil, "gphoto_auth: token decryption failed: %v", err)
			} else {
				token = decrypted
				fs.Debugf(nil, "gphoto_auth: token decrypted successfully")
			}
		}

		expiry := int64(0)
		if expiryStr, ok := result["Expiry"]; ok {
			fmt.Sscanf(expiryStr, "%d", &expiry)
			expiry *= 1000 // Convert to milliseconds
		}

		return &TokenResult{
			Token:  token,
			Expiry: expiry,
			Scopes: result["grantedScopes"],
		}, nil
	}

	// No token in response - this is a fatal error
	errMsg := result["Error"]
	if errMsg == "" {
		errMsg = "no token in response (empty response from Google)"
	}
	fs.Errorf(nil, "gphoto_auth: token request failed: %s", errMsg)
	return nil, fmt.Errorf("token request failed: %s", errMsg)
}

// AccountCredentials holds the credentials for a Google account
type AccountCredentials struct {
	MasterToken   string `json:"master_token"`
	PrivateKeyS   string `json:"private_key_s,omitempty"`
}

// TokenManager manages tokens for multiple accounts with caching
type TokenManager struct {
	accounts    map[string]*AccountCredentials
	cache       map[string]*cachedToken
	authClients map[string]*GooglePhotosAuth
	httpClient  *http.Client
	mu          sync.RWMutex
}

type cachedToken struct {
	token  string
	expiry int64
}

// NewTokenManager creates a new token manager
func NewTokenManager(accounts map[string]*AccountCredentials, httpClient *http.Client) (*TokenManager, error) {
	tm := &TokenManager{
		accounts:    accounts,
		cache:       make(map[string]*cachedToken),
		authClients: make(map[string]*GooglePhotosAuth),
		httpClient:  httpClient,
	}

	for key, creds := range accounts {
		// Normalize username
		username := key
		if idx := strings.Index(key, "@"); idx != -1 {
			username = key[:idx]
		}
		email := username + "@gmail.com"

		auth, err := NewGooglePhotosAuth(email, creds.MasterToken, "", creds.PrivateKeyS, httpClient)
		if err != nil {
			return nil, fmt.Errorf("failed to create auth for %s: %w", username, err)
		}
		tm.authClients[username] = auth
	}

	return tm, nil
}

// normalizeUser extracts username from email
func normalizeUser(user string) string {
	if idx := strings.Index(user, "@"); idx != -1 {
		return user[:idx]
	}
	return user
}

// isTokenValid checks if a cached token is still valid
func (tm *TokenManager) isTokenValid(username string, bufferMs int64) bool {
	tm.mu.RLock()
	cached, ok := tm.cache[username]
	tm.mu.RUnlock()

	if !ok {
		return false
	}

	nowMs := time.Now().UnixMilli()
	return nowMs < (cached.expiry - bufferMs)
}

// GetCachedToken returns a cached token if valid
func (tm *TokenManager) GetCachedToken(user string) string {
	username := normalizeUser(user)

	if tm.isTokenValid(username, 60000) {
		tm.mu.RLock()
		defer tm.mu.RUnlock()
		return tm.cache[username].token
	}
	return ""
}

// GetToken gets a token for a user, using cache if valid
func (tm *TokenManager) GetToken(ctx context.Context, user string, force bool) (string, error) {
	username := normalizeUser(user)

	tm.mu.RLock()
	auth, ok := tm.authClients[username]
	tm.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("unknown account: %s", username)
	}

	if !force && tm.isTokenValid(username, 60000) {
		tm.mu.RLock()
		token := tm.cache[username].token
		tm.mu.RUnlock()
		return token, nil
	}

	result, err := auth.GetToken(ctx)
	if err != nil {
		return "", err
	}

	if result.Error != "" {
		return "", fmt.Errorf("token fetch failed: %s", result.Error)
	}

	tm.mu.Lock()
	tm.cache[username] = &cachedToken{
		token:  result.Token,
		expiry: result.Expiry,
	}
	tm.mu.Unlock()

	return result.Token, nil
}
