package srp

import (
	"bytes"
	"crypto/subtle"
	"encoding/base64"
	"math/big"

	"github.com/pkg/errors"

	"crypto/rand"

	"github.com/cronokirby/saferith"
)

// Server stores the internal state for the validation of SRP proofs.
type Server struct {
	generator, verifier, serverSecret, serverEphemeral, multiplier, modulus *saferith.Nat
	sharedSession                                                           []byte
	bitLength                                                               int
}

// NewServer creates a new server instance from the raw binary data.
func NewServer(modulusBytes, verifier []byte, bitLength int) (*Server, error) {
	modulusInt := toInt(modulusBytes)
	modulusMinusOneInt := big.NewInt(0).Sub(modulusInt, big.NewInt(1))
	modulusMinusOneNat := new(saferith.Nat).SetBig(modulusMinusOneInt, bitLength)
	var err error
	var secret *saferith.Nat
	var secretInt *big.Int
	var secretBytes []byte
	lowerBoundNat := newNat(uint64(bitLength * 2))
	for {
		secretInt, err = rand.Int(RandReader, modulusMinusOneInt)
		if err != nil {
			return nil, errors.Wrap(err, "Couldn't generate the secret")
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
	multiplier, err := computeMultiplier(big.NewInt(2), toInt(modulusBytes), bitLength)
	if err != nil {
		return nil, err
	}
	return &Server{
		generator:       newNat(2),
		modulus:         toNat(modulusBytes),
		serverSecret:    secret,
		verifier:        toNat(verifier),
		bitLength:       bitLength,
		serverEphemeral: nil,
		sharedSession:   nil,
		multiplier:      multiplier,
	}, nil
}

// NewServerWithSecret creates a new server instance without generating a random secret from the raw binary data.
// Use with caution as the secret should not be reused.
func NewServerWithSecret(modulusBytes, verifier, secretBytes []byte, bitLength int) (*Server, error) {
	secret := toNat(secretBytes)
	if greaterThan, _, _ := secret.Cmp(newNat(uint64(bitLength * 2))); greaterThan != 1 {
		return nil, errors.New("go-srp: invalid secret")
	}
	multiplier, err := computeMultiplier(big.NewInt(2), toInt(modulusBytes), bitLength)
	if err != nil {
		return nil, err
	}
	return &Server{
		generator:       newNat(2),
		modulus:         toNat(modulusBytes),
		serverSecret:    secret,
		verifier:        toNat(verifier),
		bitLength:       bitLength,
		serverEphemeral: nil,
		sharedSession:   nil,
		multiplier:      multiplier,
	}, nil
}

// NewServerFromSigned creates a new server instance from the signed modulus and the binary verifier.
func NewServerFromSigned(signedModulus string, verifier []byte, bitLength int) (*Server, error) {
	modulus, err := readClearSignedMessage(signedModulus)
	if err != nil {
		return nil, err
	}
	modulusData, err := base64.StdEncoding.DecodeString(modulus)
	if err != nil {
		return nil, err
	}

	return NewServer(modulusData, verifier, bitLength)
}

// GenerateChallenge is the first step for SRP exchange, and generates a valid challenge for the provided verifier.
func (s *Server) GenerateChallenge() (serverEphemeral []byte, err error) {
	mod := saferith.ModulusFromNat(s.modulus)
	s.serverEphemeral = new(saferith.Nat).ModAdd(
		new(saferith.Nat).ModMul(s.multiplier, s.verifier, mod),
		new(saferith.Nat).Exp(s.generator, s.serverSecret, mod),
		mod,
	)

	return fromNat(s.bitLength, s.serverEphemeral), nil
}

func computeBaseServerSide(clientEphemeral, verifier, scramblingParam *saferith.Nat, modulus *saferith.Modulus) *saferith.Nat {
	var receiver saferith.Nat
	return receiver.ModMul(
		clientEphemeral,
		receiver.Exp(
			verifier,
			scramblingParam,
			modulus,
		),
		modulus,
	)
}

func computeSharedSecretServerSide(
	bitLength int,
	clientEphemeral, verifier, scramblingParam, serverSecret *saferith.Nat,
	modulus *saferith.Modulus,
) []byte {
	base := computeBaseServerSide(
		clientEphemeral,
		verifier,
		scramblingParam,
		modulus,
	)
	sharedSession := new(saferith.Nat).Exp(
		base,
		serverSecret,
		modulus,
	)
	return fromNat(bitLength, sharedSession)
}

// VerifyProofs Verifies the client proof and - if valid - generates the shared secret and returnd the server proof.
// It concludes the exchange in valid state if successful.
func (s *Server) VerifyProofs(clientEphemeralBytes, clientProofBytes []byte) (serverProof []byte, err error) {
	if s.serverEphemeral == nil {
		return nil, errors.New("pm-srp: SRP server ephemeral is not generated")
	}

	modulusMinusOne := new(saferith.Nat).Sub(s.modulus, newNat(1), s.bitLength)
	clientEphemeral := toNat(clientEphemeralBytes)
	greaterThanOne, _, _ := clientEphemeral.Cmp(newNat(1))
	_, _, lessThanModulusMinusOne := clientEphemeral.Cmp(modulusMinusOne)
	if greaterThanOne != 1 || lessThanModulusMinusOne != 1 {
		return nil, errors.New("pm-srp: SRP client ephemeral is out of bounds")
	}

	scramblingParam := computeScrambleParam(clientEphemeralBytes, fromNat(s.bitLength, s.serverEphemeral))
	if _, isZero, _ := scramblingParam.Cmp(newNat(0)); isZero == 1 {
		return nil, errors.New("pm-srp: SRP client ephemeral is invalid")
	}

	modulus := saferith.ModulusFromNat(s.modulus)
	s.sharedSession = computeSharedSecretServerSide(
		s.bitLength,
		clientEphemeral,
		s.verifier,
		scramblingParam,
		s.serverSecret,
		modulus,
	)

	expectedClientProof := expandHash(bytes.Join([][]byte{
		clientEphemeralBytes,
		fromNat(s.bitLength, s.serverEphemeral),
		s.sharedSession,
	}, []byte{}))

	if subtle.ConstantTimeCompare(expectedClientProof, clientProofBytes) == 0 {
		s.sharedSession = nil
		return nil, errors.New("pm-srp: invalid SRP client proof")
	}

	return expandHash(bytes.Join([][]byte{
		clientEphemeralBytes,
		clientProofBytes,
		s.sharedSession,
	}, []byte{})), nil
}

// IsCompleted returns true if the exchange has been concluded in valid state.
func (s *Server) IsCompleted() bool {
	return s.sharedSession != nil
}

// GetSharedSession returns the shared secret as byte if the session has concluded in valid state.
func (s *Server) GetSharedSession() ([]byte, error) {
	if !s.IsCompleted() {
		return nil, errors.New("pm-srp: SRP is not completed")
	}
	return s.sharedSession, nil
}
