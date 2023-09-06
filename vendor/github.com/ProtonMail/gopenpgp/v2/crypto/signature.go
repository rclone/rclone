package crypto

import (
	"bytes"
	"crypto"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	pgpErrors "github.com/ProtonMail/go-crypto/openpgp/errors"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/pkg/errors"

	"github.com/ProtonMail/gopenpgp/v2/constants"
	"github.com/ProtonMail/gopenpgp/v2/internal"
)

var allowedHashes = []crypto.Hash{
	crypto.SHA224,
	crypto.SHA256,
	crypto.SHA384,
	crypto.SHA512,
}

// SignatureVerificationError is returned from Decrypt and VerifyDetached
// functions when signature verification fails.
type SignatureVerificationError struct {
	Status  int
	Message string
	Cause   error
}

// Error is the base method for all errors.
func (e SignatureVerificationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("Signature Verification Error: %v caused by %v", e.Message, e.Cause)
	}
	return fmt.Sprintf("Signature Verification Error: %v", e.Message)
}

// Unwrap returns the cause of failure.
func (e SignatureVerificationError) Unwrap() error {
	return e.Cause
}

// ------------------
// Internal functions
// ------------------

// newSignatureFailed creates a new SignatureVerificationError, type
// SignatureFailed.
func newSignatureBadContext(cause error) SignatureVerificationError {
	return SignatureVerificationError{
		Status:  constants.SIGNATURE_BAD_CONTEXT,
		Message: "Invalid signature context",
		Cause:   cause,
	}
}

func newSignatureFailed(cause error) SignatureVerificationError {
	return SignatureVerificationError{
		Status:  constants.SIGNATURE_FAILED,
		Message: "Invalid signature",
		Cause:   cause,
	}
}

// newSignatureInsecure creates a new SignatureVerificationError, type
// SignatureFailed, with a message describing the signature as insecure.
func newSignatureInsecure() SignatureVerificationError {
	return SignatureVerificationError{
		Status:  constants.SIGNATURE_FAILED,
		Message: "Insecure signature",
	}
}

// newSignatureNotSigned creates a new SignatureVerificationError, type
// SignatureNotSigned.
func newSignatureNotSigned() SignatureVerificationError {
	return SignatureVerificationError{
		Status:  constants.SIGNATURE_NOT_SIGNED,
		Message: "Missing signature",
	}
}

// newSignatureNoVerifier creates a new SignatureVerificationError, type
// SignatureNoVerifier.
func newSignatureNoVerifier() SignatureVerificationError {
	return SignatureVerificationError{
		Status:  constants.SIGNATURE_NO_VERIFIER,
		Message: "No matching signature",
	}
}

// processSignatureExpiration handles signature time verification manually, so
// we can add a margin to the creationTime check.
func processSignatureExpiration(md *openpgp.MessageDetails, verifyTime int64) {
	if !errors.Is(md.SignatureError, pgpErrors.ErrSignatureExpired) {
		return
	}
	if verifyTime == 0 {
		// verifyTime = 0: time check disabled, everything is okay
		md.SignatureError = nil
		return
	}
	created := md.Signature.CreationTime.Unix()
	expires := int64(math.MaxInt64)
	if md.Signature.SigLifetimeSecs != nil {
		expires = int64(*md.Signature.SigLifetimeSecs) + created
	}
	if created-internal.CreationTimeOffset <= verifyTime && verifyTime <= expires {
		md.SignatureError = nil
	}
}

// verifyDetailsSignature verifies signature from message details.
func verifyDetailsSignature(md *openpgp.MessageDetails, verifierKey *KeyRing, verificationContext *VerificationContext) error {
	if !md.IsSigned {
		return newSignatureNotSigned()
	}
	if md.SignedBy == nil ||
		len(verifierKey.entities) == 0 ||
		len(verifierKey.entities.KeysById(md.SignedByKeyId)) == 0 {
		return newSignatureNoVerifier()
	}
	if md.SignatureError != nil {
		return newSignatureFailed(md.SignatureError)
	}
	if md.Signature == nil ||
		md.Signature.Hash < allowedHashes[0] ||
		md.Signature.Hash > allowedHashes[len(allowedHashes)-1] {
		return newSignatureInsecure()
	}
	if verificationContext != nil {
		err := verificationContext.verifyContext(md.Signature)
		if err != nil {
			return newSignatureBadContext(err)
		}
	}

	return nil
}

// SigningContext gives the context that will be
// included in the signature's notation data.
type SigningContext struct {
	Value      string
	IsCritical bool
}

// NewSigningContext creates a new signing context.
// The value is set to the notation data.
// isCritical controls whether the notation is flagged as a critical packet.
func NewSigningContext(value string, isCritical bool) *SigningContext {
	return &SigningContext{Value: value, IsCritical: isCritical}
}

func (context *SigningContext) getNotation() *packet.Notation {
	return &packet.Notation{
		Name:            constants.SignatureContextName,
		Value:           []byte(context.Value),
		IsCritical:      context.IsCritical,
		IsHumanReadable: true,
	}
}

// VerificationContext gives the context that will be
// used to verify the signature.
type VerificationContext struct {
	Value         string
	IsRequired    bool
	RequiredAfter int64
}

// NewVerificationContext creates a new verification context.
// The value is checked against the signature's notation data.
// If isRequired is false, the signature is allowed to have no context set.
// If requiredAfter is != 0, the signature is allowed to have no context set if it
// was created before the unix time set in requiredAfter.
func NewVerificationContext(value string, isRequired bool, requiredAfter int64) *VerificationContext {
	return &VerificationContext{
		Value:         value,
		IsRequired:    isRequired,
		RequiredAfter: requiredAfter,
	}
}

func (context *VerificationContext) isRequiredAtTime(signatureTime time.Time) bool {
	return context.IsRequired &&
		(context.RequiredAfter == 0 || signatureTime.After(time.Unix(context.RequiredAfter, 0)))
}

func findContext(notations []*packet.Notation) (string, error) {
	context := ""
	for _, notation := range notations {
		if notation.Name == constants.SignatureContextName {
			if context != "" {
				return "", errors.New("gopenpgp: signature has multiple context notations")
			}
			if !notation.IsHumanReadable {
				return "", errors.New("gopenpgp: context notation was not set as human-readable")
			}
			context = string(notation.Value)
		}
	}
	return context, nil
}

func (context *VerificationContext) verifyContext(sig *packet.Signature) error {
	signatureContext, err := findContext(sig.Notations)
	if err != nil {
		return err
	}
	if signatureContext != context.Value {
		contextRequired := context.isRequiredAtTime(sig.CreationTime)
		if contextRequired {
			return errors.New("gopenpgp: signature did not have the required context")
		} else if signatureContext != "" {
			return errors.New("gopenpgp: signature had a wrong context")
		}
	}

	return nil
}

// verifySignature verifies if a signature is valid with the entity list.
func verifySignature(
	pubKeyEntries openpgp.EntityList,
	origText io.Reader,
	signature []byte,
	verifyTime int64,
	verificationContext *VerificationContext,
) (*packet.Signature, error) {
	config := &packet.Config{}
	if verifyTime == 0 {
		config.Time = func() time.Time {
			return time.Unix(0, 0)
		}
	} else {
		config.Time = func() time.Time {
			return time.Unix(verifyTime+internal.CreationTimeOffset, 0)
		}
	}

	if verificationContext != nil {
		config.KnownNotations = map[string]bool{constants.SignatureContextName: true}
	}
	signatureReader := bytes.NewReader(signature)

	sig, signer, err := openpgp.VerifyDetachedSignatureAndHash(pubKeyEntries, origText, signatureReader, allowedHashes, config)

	if sig != nil && signer != nil && (errors.Is(err, pgpErrors.ErrSignatureExpired) || errors.Is(err, pgpErrors.ErrKeyExpired)) { //nolint:nestif
		if verifyTime == 0 { // Expiration check disabled
			err = nil
		} else {
			// Maybe the creation time offset pushed it over the edge
			// Retry with the actual verification time
			config.Time = func() time.Time {
				return time.Unix(verifyTime, 0)
			}

			seeker, ok := origText.(io.ReadSeeker)
			if !ok {
				return nil, errors.Wrap(err, "gopenpgp: message reader do not support seeking, cannot retry signature verification")
			}

			_, err = seeker.Seek(0, io.SeekStart)
			if err != nil {
				return nil, newSignatureFailed(errors.Wrap(err, "gopenpgp: could not rewind the data reader."))
			}

			_, err = signatureReader.Seek(0, io.SeekStart)
			if err != nil {
				return nil, newSignatureFailed(err)
			}

			sig, signer, err = openpgp.VerifyDetachedSignatureAndHash(pubKeyEntries, seeker, signatureReader, allowedHashes, config)
		}
	}

	if err != nil {
		return nil, newSignatureFailed(err)
	}

	if sig == nil || signer == nil {
		return nil, newSignatureFailed(errors.New("gopenpgp: no signer or valid signature"))
	}

	if verificationContext != nil {
		err := verificationContext.verifyContext(sig)
		if err != nil {
			return nil, newSignatureBadContext(err)
		}
	}

	return sig, nil
}

func signMessageDetached(
	signKeyRing *KeyRing,
	messageReader io.Reader,
	isBinary bool,
	context *SigningContext,
) (*PGPSignature, error) {
	config := &packet.Config{
		DefaultHash: crypto.SHA512,
		Time:        getTimeGenerator(),
	}

	signEntity, err := signKeyRing.getSigningEntity()
	if err != nil {
		return nil, err
	}

	if context != nil {
		config.SignatureNotations = append(config.SignatureNotations, context.getNotation())
	}

	var outBuf bytes.Buffer
	if isBinary {
		err = openpgp.DetachSign(&outBuf, signEntity, messageReader, config)
	} else {
		err = openpgp.DetachSignText(&outBuf, signEntity, messageReader, config)
	}
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: error in signing")
	}

	return NewPGPSignature(outBuf.Bytes()), nil
}
