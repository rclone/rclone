//  The MIT License
//
//  Copyright (c) 2019 Proton Technologies AG
//
//  Permission is hereby granted, free of charge, to any person obtaining a copy
//  of this software and associated documentation files (the "Software"), to deal
//  in the Software without restriction, including without limitation the rights
//  to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
//  copies of the Software, and to permit persons to whom the Software is
//  furnished to do so, subject to the following conditions:
//
//  The above copyright notice and this permission notice shall be included in
//  all copies or substantial portions of the Software.
//
//  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
//  IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
//  FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
//  AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
//  LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
//  OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
//  THE SOFTWARE.

package srp

import (
	"bytes"
	"encoding/base64"
	"errors"
	"math/big"

	"crypto/rand"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	"github.com/cronokirby/saferith"
)

var (
	// ErrDataAfterModulus found extra data after decode the modulus
	ErrDataAfterModulus = errors.New("pm-srp: extra data after modulus")

	// ErrInvalidSignature invalid modulus signature
	ErrInvalidSignature = errors.New("pm-srp: invalid modulus signature")

	RandReader = rand.Reader
)

// Store random reader in a variable to be able to overwrite it in tests

// Proofs Srp Proofs object. Changed SrpProofs to Proofs because the name will be used as srp.SrpProofs by other packages and as SrpSrpProofs on mobile
// ClientProof []byte  client proof
// ClientEphemeral []byte  calculated from
// ExpectedServerProof []byte
type Proofs struct {
	ClientProof, ClientEphemeral, ExpectedServerProof, sharedSession []byte
}

// Auth stores byte data for the calculation of SRP proofs.
//  * Changed SrpAuto to Auth because the name will be used as srp.SrpAuto by other packages and as SrpSrpAuth on mobile
//  * Also the data from the API called Auth. it could be match the meaning and reduce the confusion
type Auth struct {
	Modulus, ServerEphemeral, HashedPassword []byte
	Version                                  int
}

// Amored pubkey for modulus verification
const modulusPubkey = "-----BEGIN PGP PUBLIC KEY BLOCK-----\r\n\r\nxjMEXAHLgxYJKwYBBAHaRw8BAQdAFurWXXwjTemqjD7CXjXVyKf0of7n9Ctm\r\nL8v9enkzggHNEnByb3RvbkBzcnAubW9kdWx1c8J3BBAWCgApBQJcAcuDBgsJ\r\nBwgDAgkQNQWFxOlRjyYEFQgKAgMWAgECGQECGwMCHgEAAPGRAP9sauJsW12U\r\nMnTQUZpsbJb53d0Wv55mZIIiJL2XulpWPQD/V6NglBd96lZKBmInSXX/kXat\r\nSv+y0io+LR8i2+jV+AbOOARcAcuDEgorBgEEAZdVAQUBAQdAeJHUz1c9+KfE\r\nkSIgcBRE3WuXC4oj5a2/U3oASExGDW4DAQgHwmEEGBYIABMFAlwBy4MJEDUF\r\nhcTpUY8mAhsMAAD/XQD8DxNI6E78meodQI+wLsrKLeHn32iLvUqJbVDhfWSU\r\nWO4BAMcm1u02t4VKw++ttECPt+HUgPUq5pqQWe5Q2cW4TMsE\r\n=Y4Mw\r\n-----END PGP PUBLIC KEY BLOCK-----"

// readClearSignedMessage reads the clear text from signed message and verifies
// signature. There must be no data appended after signed message in input string.
// The message must be sign by key corresponding to `modulusPubkey`.
func readClearSignedMessage(signedMessage string) (string, error) {
	modulusBlock, rest := clearsign.Decode([]byte(signedMessage))
	if len(rest) != 0 {
		return "", ErrDataAfterModulus
	}

	modulusKeyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader([]byte(modulusPubkey)))
	if err != nil {
		return "", errors.New("pm-srp: can not read modulus pubkey")
	}

	_, err = openpgp.CheckDetachedSignature(modulusKeyring, bytes.NewReader(modulusBlock.Bytes), modulusBlock.ArmoredSignature.Body, nil)
	if err != nil {
		return "", ErrInvalidSignature
	}

	return string(modulusBlock.Bytes), nil
}

func GetModulusKey() string {
	return modulusPubkey
}

// NewAuth Creates new Auth from strings input. Salt and server ephemeral are in
// base64 format. Modulus is base64 with signature attached. The signature is
// verified against server key. The version controls password hash algorithm.
//
// Parameters:
//	 - version int: The *x* component of the vector.
//	 - username string: The *y* component of the vector.
//	 - password []byte: The *z* component of the vector.
// 	 - b64salt string: The std-base64 formatted salt
// Returns:
//   - auth *Auth: the pre calculated auth information
//   - err error: throw error
// Usage:
//
// Warnings:
//	 - Be careful! Poos can hurt.
func NewAuth(version int, username string, password []byte, b64salt, signedModulus, serverEphemeral string) (auth *Auth, err error) {
	data := &Auth{}

	// Modulus
	var modulus string
	modulus, err = readClearSignedMessage(signedModulus)
	if err != nil {
		return
	}
	data.Modulus, err = base64.StdEncoding.DecodeString(modulus)
	if err != nil {
		return
	}

	// Password
	var decodedSalt []byte
	if version >= 3 {
		decodedSalt, err = base64.StdEncoding.DecodeString(b64salt)
		if err != nil {
			return
		}
	}
	data.HashedPassword, err = HashPassword(version, password, username, decodedSalt, data.Modulus)
	if err != nil {
		return
	}

	// Server ephermeral
	data.ServerEphemeral, err = base64.StdEncoding.DecodeString(serverEphemeral)
	if err != nil {
		return
	}

	// Authentication version
	data.Version = version

	auth = data
	return
}

// NewAuthForVerifier Creates new Auth from strings input. Salt and server ephemeral are in
// base64 format. Modulus is base64 with signature attached. The signature is
// verified against server key. The version controls password hash algorithm.
//
// Parameters:
//	 - version int: The *x* component of the vector.
//	 - username string: The *y* component of the vector.
//	 - password []byte: The *z* component of the vector.
// 	 - salt string:
// Returns:
//   - auth *Auth: the pre calculated auth information
//   - err error: throw error
// Usage:
//
// Warnings:
//	 - none.
func NewAuthForVerifier(password []byte, signedModulus string, rawSalt []byte) (auth *Auth, err error) {
	data := &Auth{}

	// Modulus
	var modulus string
	modulus, err = readClearSignedMessage(signedModulus)
	if err != nil {
		return
	}
	data.Modulus, err = base64.StdEncoding.DecodeString(modulus)
	if err != nil {
		return
	}

	// hash version is 4
	data.HashedPassword, err = hashPasswordVersion3(password, rawSalt, data.Modulus)
	if err != nil {
		return
	}
	// Authentication version hardcoded
	data.Version = 4
	auth = data
	return
}

func toInt(arr []byte) *big.Int {
	var reversed = make([]byte, len(arr))
	for i := 0; i < len(arr); i++ {
		reversed[len(arr)-i-1] = arr[i]
	}
	return big.NewInt(0).SetBytes(reversed)
}

func fromInt(bitLength int, num *big.Int) []byte {
	var arr = num.Bytes()
	var reversed = make([]byte, bitLength/8)
	for i := 0; i < len(arr); i++ {
		reversed[len(arr)-i-1] = arr[i]
	}
	return reversed
}

func toNat(arr []byte) *saferith.Nat {
	var reversed = make([]byte, len(arr))
	for i := 0; i < len(arr); i++ {
		reversed[len(arr)-i-1] = arr[i]
	}
	return new(saferith.Nat).SetBytes(reversed)
}

func newNat(val uint64) *saferith.Nat {
	return new(saferith.Nat).SetUint64(val)
}

func toModulus(arr []byte) *saferith.Modulus {
	var reversed = make([]byte, len(arr))
	for i := 0; i < len(arr); i++ {
		reversed[len(arr)-i-1] = arr[i]
	}
	return saferith.ModulusFromBytes(reversed)
}

func fromNat(bitLength int, nat *saferith.Nat) []byte {
	var arr = nat.Bytes()
	var reversed = make([]byte, bitLength/8)
	for i := 0; i < len(arr); i++ {
		reversed[len(arr)-i-1] = arr[i]
	}
	return reversed
}

func computeMultiplier(generator, modulus *big.Int, bitLength int) (*saferith.Nat, error) {
	modulusMinusOne := big.NewInt(0).Sub(modulus, big.NewInt(1))
	multiplier := toInt(expandHash(append(fromInt(bitLength, generator), fromInt(bitLength, modulus)...)))
	multiplier = multiplier.Mod(multiplier, modulus)

	if multiplier.Cmp(big.NewInt(1)) <= 0 || multiplier.Cmp(modulusMinusOne) >= 0 {
		return nil, errors.New("pm-srp: SRP multiplier is out of bounds")
	}

	return new(saferith.Nat).SetBig(multiplier, bitLength), nil
}

func checkParams(bitLength int, ephemeral, generator, modulus *big.Int) error {

	if !generator.IsInt64() || generator.Int64() != 2 {
		return errors.New("go-srp: SRP generator must always be 2")
	}

	if modulus.BitLen() != bitLength {
		return errors.New("go-srp: SRP modulus has incorrect size")
	}

	if modulus.Bit(0) != 1 || modulus.Bit(1) != 1 || modulus.Bit(2) != 0 {
		// By quadratic reciprocity, 2 is a square mod N if and only if
		// N is 1 or 7 mod 8. We want the generator, 2, to generate the
		// whole group, not just the prime-order subgroup, so it should
		// *not* be a square. In addition, since N should be prime, it
		// must not be even, and since (N-1)/2 should be prime, N must
		// not be 1 mod 4. This leaves 3 mod 8 as the only option.
		return errors.New("go-srp: SRP modulus is not 3 mod 8")
	}

	modulusMinusOne := big.NewInt(0).Sub(modulus, big.NewInt(1))
	if ephemeral.Cmp(big.NewInt(1)) <= 0 || ephemeral.Cmp(modulusMinusOne) >= 0 {
		return errors.New("go-srp: SRP server ephemeral is out of bounds")
	}

	// halfModulus is (N-1)/2. We've already checked that N is odd.
	halfModulus := big.NewInt(0).Rsh(modulus, 1)

	// Check safe primality
	if !halfModulus.ProbablyPrime(10) {
		return errors.New("pm-srp: SRP modulus is not a safe prime")
	}

	// Check primality using the Lucas primality test. This requires a
	// single exponentiation for complete confidence (assuming halfModulus
	// is prime), and so is much more efficient than relying on ProbablyPrime.
	// To prove primality with the Lucas test with base 2, it suffices to
	// show that 2^(N-1) = 1 (mod N) and 2^((N-1)/2) != 1 (mod N). The stricter
	// condition, that 2^((N-1)/2) = -1 (mod N), is a single exponentiation
	// and doubles as a test / guarantee that 2 is a generator of the whole group
	// (and not a square).
	if big.NewInt(0).Exp(generator, halfModulus, modulus).Cmp(modulusMinusOne) != 0 {
		return errors.New("pm-srp: SRP modulus is not prime")
	}

	return nil
}

func generateClientEphemeral(
	bitLength int,
	modulusInt, modulusMinusOneInt *big.Int,
	modulusMinusOneNat *saferith.Nat,
	modulus *saferith.Modulus,
) (secret *saferith.Nat, ephemeral []byte, err error) {
	var secretInt *big.Int
	var secretBytes []byte
	lowerBoundNat := newNat(uint64(bitLength * 2))
	for {
		secretInt, err = rand.Int(RandReader, modulusMinusOneInt)
		if err != nil {
			return nil, nil, err
		}
		secretBytes = fromInt(bitLength, secretInt)
		secret = toNat(secretBytes)

		// Prevent g^a from being smaller than the modulus
		// and a to be >= than N-1
		notTooSmall, _, _ := secret.Cmp(lowerBoundNat)
		_, _, notTooLarge := secret.Cmp(modulusMinusOneNat)
		if notTooSmall == 1 && notTooLarge == 1 {
			break
		}
	}
	ephemeralNat := new(saferith.Nat).Exp(newNat(2), secret, modulus)
	ephemeral = fromNat(bitLength, ephemeralNat)
	return secret, ephemeral, nil
}

func computeScrambleParam(clientEphemeralBytes, serverEphemeralBytes []byte) *saferith.Nat {
	return toNat(
		expandHash(
			append(
				clientEphemeralBytes,
				serverEphemeralBytes...,
			),
		),
	)
}

func computeBaseClientSide(hashedPassword, generator, serverEphemeral, multiplier *saferith.Nat, modulus *saferith.Modulus) *saferith.Nat {
	var receiver saferith.Nat
	return receiver.ModSub(
		serverEphemeral,
		receiver.ModMul(
			receiver.Exp(
				generator,
				hashedPassword,
				modulus,
			),
			multiplier,
			modulus,
		),
		modulus,
	)
}

func computeExponentClientSide(bitLength int, scramblingParam, hashedPassword, clientSecret *saferith.Nat, modulusMinusOne *saferith.Modulus) *saferith.Nat {
	var receiver saferith.Nat
	return receiver.ModAdd(
		receiver.ModMul(
			scramblingParam,
			hashedPassword,
			modulusMinusOne,
		),
		clientSecret,
		modulusMinusOne,
	)
}

func computeSharedSecretClientSide(
	bitLength int,
	hashedPassword, generator, serverEphemeral, multiplier, modulusMinusOneNat, clientSecret, scramblingParam *saferith.Nat,
	modulus *saferith.Modulus,
) []byte {
	base := computeBaseClientSide(
		hashedPassword,
		generator,
		serverEphemeral,
		multiplier,
		modulus,
	)
	modulusMinusOne := saferith.ModulusFromNat(modulusMinusOneNat)
	exponent := computeExponentClientSide(
		bitLength,
		scramblingParam,
		hashedPassword,
		clientSecret,
		modulusMinusOne,
	)
	sharedSession := new(saferith.Nat).Exp(
		base,
		exponent,
		modulus,
	)
	return fromNat(bitLength, sharedSession)
}

func computeClientProof(clientEphemeral, serverEphemeral, sharedSecret []byte) []byte {
	return expandHash(
		bytes.Join(
			[][]byte{
				clientEphemeral,
				serverEphemeral,
				sharedSecret,
			},
			[]byte{},
		),
	)
}

func computeServerProof(clientEphemeral, clientProof, sharedSecret []byte) []byte {
	return expandHash(
		bytes.Join(
			[][]byte{
				clientEphemeral,
				clientProof,
				sharedSecret,
			},
			[]byte{},
		),
	)
}

// GenerateProofs calculates SPR proofs.
func (s *Auth) GenerateProofs(bitLength int) (*Proofs, error) {
	serverEphemeralInt := toInt(s.ServerEphemeral)
	generatorInt := big.NewInt(2)
	modulusInt := toInt(s.Modulus)
	modulusMinusOneInt := big.NewInt(0).Sub(modulusInt, big.NewInt(1))
	err := checkParams(
		bitLength,
		serverEphemeralInt,
		generatorInt,
		modulusInt,
	)
	if err != nil {
		return nil, err
	}

	modulus := toModulus(s.Modulus)
	modulusMinusOneNat := new(saferith.Nat).SetBig(modulusMinusOneInt, bitLength)
	var clientSecret, scramblingParam *saferith.Nat
	var clientEphemeralBytes []byte
	for {
		clientSecret, clientEphemeralBytes, err = generateClientEphemeral(
			bitLength,
			modulusInt, modulusMinusOneInt,
			modulusMinusOneNat,
			modulus,
		)
		if err != nil {
			return nil, err
		}
		scramblingParam = computeScrambleParam(clientEphemeralBytes, s.ServerEphemeral)
		if _, equal, _ := scramblingParam.Cmp(newNat(0)); equal != 1 { // Very likely
			break
		}
	}

	multiplierNat, err := computeMultiplier(generatorInt, modulusInt, bitLength)
	if err != nil {
		return nil, err
	}

	hashedPasswordNat := toNat(s.HashedPassword)
	generatorNat := newNat(2)
	serverEphemeralNat := toNat(s.ServerEphemeral)

	sharedSecret := computeSharedSecretClientSide(
		bitLength,
		hashedPasswordNat,
		generatorNat,
		serverEphemeralNat,
		multiplierNat,
		modulusMinusOneNat,
		clientSecret,
		scramblingParam,
		modulus,
	)

	clientProof := computeClientProof(clientEphemeralBytes, s.ServerEphemeral, sharedSecret)

	serverProof := computeServerProof(clientEphemeralBytes, clientProof, sharedSecret)

	proofs := &Proofs{
		ClientEphemeral:     clientEphemeralBytes,
		ClientProof:         clientProof,
		ExpectedServerProof: serverProof,
		sharedSession:       sharedSecret,
	}
	return proofs, nil
}

// GenerateVerifier verifier for update pwds and create accounts
func (s *Auth) GenerateVerifier(bitLength int) ([]byte, error) {
	modulus := toModulus(s.Modulus)
	generator := newNat(2)
	hashedPassword := toNat(s.HashedPassword)
	calModPow := new(saferith.Nat).SetUint64(0).Exp(generator, hashedPassword, modulus)
	return fromNat(bitLength, calModPow), nil
}

func RandomBits(bits int) ([]byte, error) {
	return RandomBytes(bits / 8)
}

func RandomBytes(byes int) (raw []byte, err error) {
	raw = make([]byte, byes)
	_, err = rand.Read(raw)
	return
}
