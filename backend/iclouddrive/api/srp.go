package api

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"math/big"
)

// RFC 5054 2048-bit group parameters for SRP-6a (g=2, SHA-256).
var (
	srpN = mustParseBigHex(
		"AC6BDB41324A9A9BF166DE5E1389582FAF72B6651987EE07FC3192943DB56050" +
			"A37329CBB4A099ED8193E0757767A13DD52312AB4B03310DCD7F48A9DA04FD50" +
			"E8083969EDB767B0CF6095179A163AB3661A05FBD5FAAAE82918A9962F0B93B8" +
			"55F97993EC975EEAA80D740ADBF4FF747359D041D5C33EA71D281E446B14773B" +
			"CA97B43A23FB801676BD207A436C6481F1D2B9078717461A5B9D32E688F87748" +
			"544523B524B0D57D5EA77A2775D2ECFA032CFBDBF52FB37861602790" +
			"04E57AE6AF874E7303CE53299CCC041C7BC308D82A5698F3A8D0C38271AE35F8" +
			"E9DBFBB694B5C803D89F7AE435DE236D525F54759B65E372FCD68EF20FA7111F" +
			"9E4AFF73")
	srpG         = big.NewInt(2)
	srpNLenBytes = 2048 / 8
	srpHashFunc  = sha256.New
)

func mustParseBigHex(s string) *big.Int {
	b, ok := new(big.Int).SetString(s, 16)
	if !ok {
		panic("srp: bad hex constant for N")
	}
	return b
}

// srpClient implements the client side of SRP-6a for Apple's iCloud auth.
type srpClient struct {
	a  *big.Int // client secret
	A  *big.Int // client public value
	k  *big.Int // multiplier
	M1 []byte
	M2 []byte
	K  []byte
}

// newSRPClient generates a random 32-byte secret and computes the public value A.
func newSRPClient() *srpClient {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		panic(fmt.Sprintf("srp: rand.Read failed: %v", err))
	}

	a := new(big.Int).SetBytes(secret)
	A := new(big.Int).Exp(srpG, a, srpN)
	k := getMultiplier()

	return &srpClient{
		a: a,
		A: A,
		k: k,
	}
}

// getABytes returns the padded public value A.
func (c *srpClient) getABytes() []byte {
	return padToN(c.A)
}

// processChallenge computes the session key and proof values from the server challenge.
// username is the Apple ID, derivedKey is the output of derivePassword, salt and B come
// from the server's init response (raw bytes, already base64-decoded).
func (c *srpClient) processChallenge(username, derivedKey, salt, serverB []byte) {
	B := new(big.Int).SetBytes(serverB)

	// Validate B
	if B.Cmp(big.NewInt(0)) <= 0 || B.Cmp(srpN) >= 0 {
		panic("srp: invalid server-supplied B, must be 1..N-1")
	}

	x := calculateX(salt, derivedKey)
	u := calculateU(c.A, B)
	S := calculateS(c.k, x, c.a, B, u)
	c.K = calculateK(S)

	aBytes := padToN(c.A)
	bBytes := padToN(B)
	c.M1 = calculateM1(username, salt, aBytes, bBytes, c.K)
	c.M2 = calculateM2(aBytes, c.M1, c.K)
}

// derivePassword performs Apple's password key derivation.
// For "s2k": PBKDF2(SHA256(password), salt, iterations, 32, SHA256)
// For "s2k_fo": PBKDF2(hex(SHA256(password)), salt, iterations, 32, SHA256)
func derivePassword(password string, salt []byte, iterations int, protocol string) ([]byte, error) {
	passHash := sha256.Sum256([]byte(password))

	var passInput string
	switch protocol {
	case "s2k_fo":
		passInput = hex.EncodeToString(passHash[:])
	default: // "s2k"
		passInput = string(passHash[:])
	}
	return pbkdf2.Key(sha256.New, passInput, salt, iterations, 32)
}

// padToN pads a big.Int to the SRP group size (256 bytes for 2048-bit).
func padToN(n *big.Int) []byte {
	b := n.Bytes()
	if len(b) >= srpNLenBytes {
		return b
	}
	padded := make([]byte, srpNLenBytes)
	copy(padded[srpNLenBytes-len(b):], b)
	return padded
}

func srpHash(data []byte) []byte {
	h := srpHashFunc()
	h.Write(data)
	return h.Sum(nil)
}

func hashToInt(h hash.Hash) *big.Int {
	return new(big.Int).SetBytes(h.Sum(nil))
}

// getMultiplier computes k = H(N | pad(g))
func getMultiplier() *big.Int {
	h := srpHashFunc()
	nBytes := srpN.Bytes()
	gBytes := srpG.Bytes()
	// pad g to same length as N
	for len(gBytes) < len(nBytes) {
		gBytes = append([]byte{0}, gBytes...)
	}
	h.Write(nBytes)
	h.Write(gBytes)
	return hashToInt(h)
}

// calculateX computes x = H(salt | H(":" | password))
// Apple's variant: NoUserNameInX — username is omitted from the inner hash.
// The "password" here is actually the derived key from derivePassword.
func calculateX(salt, derivedKey []byte) *big.Int {
	h := srpHashFunc()
	// Apple omits username (NoUserNameInX), only writes ":" + derivedKey
	h.Write([]byte(":"))
	h.Write(derivedKey)
	digest := h.Sum(nil)

	h2 := srpHashFunc()
	h2.Write(salt)
	h2.Write(digest)
	return new(big.Int).SetBytes(h2.Sum(nil))
}

// calculateU computes u = H(pad(A) | pad(B))
func calculateU(A, B *big.Int) *big.Int {
	h := srpHashFunc()
	h.Write(padToN(A))
	h.Write(padToN(B))
	return hashToInt(h)
}

// calculateS computes the client-side shared secret:
// S = (B - k * g^x) ^ (a + u*x) mod N
func calculateS(k, x, a, B, u *big.Int) []byte {
	// g^x mod N
	gx := new(big.Int).Exp(srpG, x, srpN)

	// k * g^x
	kgx := new(big.Int).Mul(k, gx)

	// B - k*g^x
	diff := new(big.Int).Sub(B, kgx)

	// u*x
	ux := new(big.Int).Mul(u, x)

	// a + u*x
	exp := new(big.Int).Add(a, ux)

	// (B - k*g^x) ^ (a + u*x) mod N
	S := new(big.Int).Exp(diff, exp, srpN)

	// Ensure positive
	S.Mod(S, srpN)

	return padToN(S)
}

// calculateK computes K = H(S)
func calculateK(S []byte) []byte {
	return srpHash(S)
}

// calculateM1 computes the client proof (Apple/iCloud variant):
// M1 = H(H(g) XOR H(N) | H(username) | salt | A | B | K)
func calculateM1(username, salt, A, B, K []byte) []byte {
	hg := srpHash(padToN(srpG))
	hn := srpHash(srpN.Bytes())
	hi := srpHash(username)

	hxor := make([]byte, len(hg))
	for i := range hg {
		hxor[i] = hg[i] ^ hn[i]
	}

	h := srpHashFunc()
	h.Write(hxor)
	h.Write(hi)
	h.Write(salt)
	h.Write(A)
	h.Write(B)
	h.Write(K)
	return h.Sum(nil)
}

// calculateM2 computes the server proof: M2 = H(A | M1 | K)
func calculateM2(A, M1, K []byte) []byte {
	h := srpHashFunc()
	h.Write(A)
	h.Write(M1)
	h.Write(K)
	return h.Sum(nil)
}
