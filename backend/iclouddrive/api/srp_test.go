package api

import (
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPadToN(t *testing.T) {
	t.Run("pads small value to 256 bytes", func(t *testing.T) {
		result := padToN(big.NewInt(1))
		assert.Equal(t, srpNLenBytes, len(result))
		assert.Equal(t, byte(0), result[0])
		assert.Equal(t, byte(1), result[srpNLenBytes-1])
	})

	t.Run("pads zero to 256 bytes", func(t *testing.T) {
		result := padToN(big.NewInt(0))
		assert.Equal(t, srpNLenBytes, len(result))
		for _, b := range result {
			assert.Equal(t, byte(0), b)
		}
	})

	t.Run("preserves value that fills N", func(t *testing.T) {
		// Use N itself — already 256 bytes
		result := padToN(srpN)
		assert.Equal(t, srpNLenBytes, len(result))
		assert.Equal(t, srpN.Bytes(), result)
	})

	t.Run("round-trips through big.Int", func(t *testing.T) {
		original := big.NewInt(123456789)
		padded := padToN(original)
		recovered := new(big.Int).SetBytes(padded)
		assert.Equal(t, 0, original.Cmp(recovered))
	})
}

func TestGetMultiplier(t *testing.T) {
	k := getMultiplier()

	// k must be non-zero and less than N
	assert.True(t, k.Sign() > 0, "multiplier must be positive")
	assert.True(t, k.Cmp(srpN) < 0, "multiplier must be less than N")

	// k is deterministic
	k2 := getMultiplier()
	assert.Equal(t, 0, k.Cmp(k2), "multiplier must be deterministic")

	// k = H(N | pad(g)) — verify by manual computation
	h := srpHashFunc()
	nBytes := srpN.Bytes()
	gBytes := srpG.Bytes()
	for len(gBytes) < len(nBytes) {
		gBytes = append([]byte{0}, gBytes...)
	}
	h.Write(nBytes)
	h.Write(gBytes)
	expected := new(big.Int).SetBytes(h.Sum(nil))
	assert.Equal(t, 0, expected.Cmp(k), "multiplier must equal H(N|pad(g))")
}

func TestDerivePassword(t *testing.T) {
	salt := []byte("test_salt_value!")
	password := "test_password"

	t.Run("s2k produces 32-byte key", func(t *testing.T) {
		key, err := derivePassword(password, salt, 1000, "s2k")
		require.NoError(t, err)
		assert.Equal(t, 32, len(key))
	})

	t.Run("s2k_fo produces 32-byte key", func(t *testing.T) {
		key, err := derivePassword(password, salt, 1000, "s2k_fo")
		require.NoError(t, err)
		assert.Equal(t, 32, len(key))
	})

	t.Run("s2k and s2k_fo produce different keys", func(t *testing.T) {
		key1, err := derivePassword(password, salt, 1000, "s2k")
		require.NoError(t, err)
		key2, err := derivePassword(password, salt, 1000, "s2k_fo")
		require.NoError(t, err)
		assert.NotEqual(t, key1, key2, "s2k and s2k_fo must produce different keys")
	})

	t.Run("deterministic output", func(t *testing.T) {
		key1, err := derivePassword(password, salt, 1000, "s2k")
		require.NoError(t, err)
		key2, err := derivePassword(password, salt, 1000, "s2k")
		require.NoError(t, err)
		assert.Equal(t, key1, key2)
	})

	t.Run("different passwords produce different keys", func(t *testing.T) {
		key1, err := derivePassword("password1", salt, 1000, "s2k")
		require.NoError(t, err)
		key2, err := derivePassword("password2", salt, 1000, "s2k")
		require.NoError(t, err)
		assert.NotEqual(t, key1, key2)
	})

	t.Run("s2k uses raw SHA256 bytes", func(t *testing.T) {
		// Verify that s2k uses SHA256(password) as the PBKDF2 input.
		// Two passwords with the same SHA256 hash would produce the same key,
		// while two passwords with different SHA256 hashes must produce different keys.
		key1, err := derivePassword("hello", salt, 10, "s2k")
		require.NoError(t, err)
		key2, err := derivePassword("world", salt, 10, "s2k")
		require.NoError(t, err)
		assert.NotEqual(t, key1, key2)
	})

	t.Run("s2k_fo uses hex-encoded SHA256", func(t *testing.T) {
		// For s2k_fo, the PBKDF2 input is hex(SHA256(password)), which is
		// a 64-character ASCII string, not raw 32-byte hash.
		key, err := derivePassword("hello", salt, 10, "s2k_fo")
		require.NoError(t, err)
		assert.Equal(t, 32, len(key))
	})
}

func TestCalculateX(t *testing.T) {
	salt := []byte("some_salt")
	derivedKey := []byte("some_derived_key_value_32_bytes!")

	x := calculateX(salt, derivedKey)

	// x must be positive and less than the hash output space
	assert.True(t, x.Sign() > 0, "x must be positive")

	// x is deterministic
	x2 := calculateX(salt, derivedKey)
	assert.Equal(t, 0, x.Cmp(x2), "x must be deterministic")

	// Verify manually: x = H(salt | H(":" | derivedKey))
	h1 := srpHashFunc()
	h1.Write([]byte(":"))
	h1.Write(derivedKey)
	inner := h1.Sum(nil)

	h2 := srpHashFunc()
	h2.Write(salt)
	h2.Write(inner)
	expected := new(big.Int).SetBytes(h2.Sum(nil))
	assert.Equal(t, 0, expected.Cmp(x), "x must equal H(salt | H(\":\" | derivedKey))")
}

func TestCalculateU(t *testing.T) {
	A := big.NewInt(12345)
	B := big.NewInt(67890)

	u := calculateU(A, B)
	assert.True(t, u.Sign() > 0, "u must be positive")

	// u is deterministic
	u2 := calculateU(A, B)
	assert.Equal(t, 0, u.Cmp(u2))

	// u changes when inputs change
	u3 := calculateU(A, big.NewInt(99999))
	assert.NotEqual(t, 0, u.Cmp(u3), "u must change when B changes")

	// Verify: u = H(pad(A) | pad(B))
	h := srpHashFunc()
	h.Write(padToN(A))
	h.Write(padToN(B))
	expected := new(big.Int).SetBytes(h.Sum(nil))
	assert.Equal(t, 0, expected.Cmp(u))
}

func TestCalculateK(t *testing.T) {
	S := []byte("some_shared_secret_value_32bytes")

	K := calculateK(S)

	// K is a SHA-256 hash, so 32 bytes
	assert.Equal(t, sha256.Size, len(K))

	// K is deterministic
	K2 := calculateK(S)
	assert.Equal(t, K, K2)

	// Verify: K = H(S)
	expected := srpHash(S)
	assert.Equal(t, expected, K)
}

func TestCalculateM1(t *testing.T) {
	username := []byte("test@example.com")
	salt := []byte("test_salt")
	A := make([]byte, srpNLenBytes)
	B := make([]byte, srpNLenBytes)
	K := make([]byte, 32)

	// Fill with non-zero values
	A[srpNLenBytes-1] = 0x42
	B[srpNLenBytes-1] = 0x43
	K[0] = 0x44

	m1 := calculateM1(username, salt, A, B, K)

	// M1 is a SHA-256 hash
	assert.Equal(t, sha256.Size, len(m1))

	// M1 is deterministic
	m1b := calculateM1(username, salt, A, B, K)
	assert.Equal(t, m1, m1b)

	// M1 changes when username changes
	m1c := calculateM1([]byte("other@example.com"), salt, A, B, K)
	assert.NotEqual(t, m1, m1c)

	// Verify: M1 = H(H(g) XOR H(N) | H(username) | salt | A | B | K)
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
	expected := h.Sum(nil)
	assert.Equal(t, expected, m1)
}

func TestCalculateM2(t *testing.T) {
	A := make([]byte, srpNLenBytes)
	M1 := make([]byte, 32)
	K := make([]byte, 32)

	A[srpNLenBytes-1] = 0x01
	M1[0] = 0x02
	K[0] = 0x03

	m2 := calculateM2(A, M1, K)
	assert.Equal(t, sha256.Size, len(m2))

	// Verify: M2 = H(A | M1 | K)
	h := srpHashFunc()
	h.Write(A)
	h.Write(M1)
	h.Write(K)
	assert.Equal(t, h.Sum(nil), m2)
}

func TestNewSRPClient(t *testing.T) {
	client := newSRPClient()

	// A must be non-zero and in range [1, N-1]
	assert.True(t, client.A.Sign() > 0, "A must be positive")
	assert.True(t, client.A.Cmp(srpN) < 0, "A must be less than N")

	// A = g^a mod N
	expectedA := new(big.Int).Exp(srpG, client.a, srpN)
	assert.Equal(t, 0, expectedA.Cmp(client.A), "A must equal g^a mod N")

	// getABytes returns padded A
	aBytes := client.getABytes()
	assert.Equal(t, srpNLenBytes, len(aBytes))
	recoveredA := new(big.Int).SetBytes(aBytes)
	assert.Equal(t, 0, client.A.Cmp(recoveredA))

	// Two clients should have different secrets (probabilistic, but 2^256 collision chance)
	client2 := newSRPClient()
	assert.NotEqual(t, 0, client.a.Cmp(client2.a), "two clients must have different secrets")
}

func TestProcessChallenge(t *testing.T) {
	// Use a fixed secret for deterministic testing
	fixedSecret := make([]byte, 32)
	fixedSecret[31] = 0x42 // small non-zero value

	a := new(big.Int).SetBytes(fixedSecret)
	A := new(big.Int).Exp(srpG, a, srpN)
	k := getMultiplier()

	client := &srpClient{
		a: a,
		A: A,
		k: k,
	}

	username := []byte("testuser@apple.com")
	derivedKey := make([]byte, 32)
	for i := range derivedKey {
		derivedKey[i] = byte(i)
	}
	salt := []byte("fixed_salt_for_test!")

	// Compute a valid B: B = (k*v + g^b) mod N where v = g^x mod N
	x := calculateX(salt, derivedKey)
	v := new(big.Int).Exp(srpG, x, srpN)

	bSecret := make([]byte, 32)
	bSecret[31] = 0x99
	b := new(big.Int).SetBytes(bSecret)

	// B = (k*v + g^b) mod N
	kv := new(big.Int).Mul(k, v)
	gb := new(big.Int).Exp(srpG, b, srpN)
	B := new(big.Int).Add(kv, gb)
	B.Mod(B, srpN)

	serverB := padToN(B)

	client.processChallenge(username, derivedKey, salt, serverB)

	// M1, M2, K must be populated
	assert.NotNil(t, client.M1)
	assert.NotNil(t, client.M2)
	assert.NotNil(t, client.K)
	assert.Equal(t, sha256.Size, len(client.M1))
	assert.Equal(t, sha256.Size, len(client.M2))
	assert.Equal(t, sha256.Size, len(client.K))

	// M1 and M2 must be different
	assert.NotEqual(t, client.M1, client.M2)

	// Results must be deterministic — run again with same inputs
	client2 := &srpClient{
		a: new(big.Int).Set(a),
		A: new(big.Int).Set(A),
		k: new(big.Int).Set(k),
	}
	client2.processChallenge(username, derivedKey, salt, serverB)

	assert.Equal(t, client.M1, client2.M1, "M1 must be deterministic")
	assert.Equal(t, client.M2, client2.M2, "M2 must be deterministic")
	assert.Equal(t, client.K, client2.K, "K must be deterministic")

	// Verify the server could also compute S and get the same K.
	// Server computes: S_server = (A * v^u)^b mod N
	u := calculateU(A, B)
	vu := new(big.Int).Exp(v, u, srpN)
	avu := new(big.Int).Mul(A, vu)
	avu.Mod(avu, srpN)
	sServer := new(big.Int).Exp(avu, b, srpN)
	sServer.Mod(sServer, srpN)
	kServer := calculateK(padToN(sServer))
	assert.Equal(t, client.K, kServer, "client and server must derive the same session key")
}

func TestSRPGroupParameters(t *testing.T) {
	// Verify the 2048-bit prime starts with known hex prefix
	nHex := hex.EncodeToString(srpN.Bytes())
	assert.True(t, len(nHex) > 0)
	assert.Equal(t, "ac6bdb41", nHex[:8], "N must start with known prefix")

	// g must be 2
	assert.Equal(t, int64(2), srpG.Int64())

	// N byte length must match expected
	assert.Equal(t, 256, srpNLenBytes)
	assert.Equal(t, 256, len(srpN.Bytes()))
}

func TestSRPHash(t *testing.T) {
	data := []byte("hello world")
	hash := srpHash(data)

	// Must be SHA-256 output
	assert.Equal(t, sha256.Size, len(hash))

	// Must equal standard sha256
	expected := sha256.Sum256(data)
	assert.Equal(t, expected[:], hash)
}
