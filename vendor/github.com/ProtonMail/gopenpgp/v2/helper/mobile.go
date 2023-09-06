package helper

import (
	"encoding/json"
	goerrors "errors"
	"runtime/debug"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/pkg/errors"
)

type ExplicitVerifyMessage struct {
	Message                    *crypto.PlainMessage
	SignatureVerificationError *crypto.SignatureVerificationError
}

// DecryptExplicitVerify decrypts a PGP message given a private keyring
// and a public keyring to verify the embedded signature. Returns the plain
// data and an error on signature verification failure.
func DecryptExplicitVerify(
	pgpMessage *crypto.PGPMessage,
	privateKeyRing, publicKeyRing *crypto.KeyRing,
	verifyTime int64,
) (*ExplicitVerifyMessage, error) {
	message, err := privateKeyRing.Decrypt(pgpMessage, publicKeyRing, verifyTime)
	return newExplicitVerifyMessage(message, err)
}

// DecryptExplicitVerifyWithContext decrypts a PGP message given a private keyring
// and a public keyring to verify the embedded signature. Returns the plain
// data and an error on signature verification failure.
// The caller can provide a context that will be used to verify the signature.
func DecryptExplicitVerifyWithContext(
	pgpMessage *crypto.PGPMessage,
	privateKeyRing, publicKeyRing *crypto.KeyRing,
	verifyTime int64,
	verificationContext *crypto.VerificationContext,
) (*ExplicitVerifyMessage, error) {
	message, err := privateKeyRing.DecryptWithContext(pgpMessage, publicKeyRing, verifyTime, verificationContext)
	return newExplicitVerifyMessage(message, err)
}

// DecryptSessionKeyExplicitVerify decrypts a PGP data packet given a session key
// and a public keyring to verify the embedded signature. Returns the plain data and
// an error on signature verification failure.
func DecryptSessionKeyExplicitVerify(
	dataPacket []byte,
	sessionKey *crypto.SessionKey,
	publicKeyRing *crypto.KeyRing,
	verifyTime int64,
) (*ExplicitVerifyMessage, error) {
	message, err := sessionKey.DecryptAndVerify(dataPacket, publicKeyRing, verifyTime)
	return newExplicitVerifyMessage(message, err)
}

// DecryptSessionKeyExplicitVerifyWithContext decrypts a PGP data packet given a session key
// and a public keyring to verify the embedded signature. Returns the plain data and
// an error on signature verification failure.
// The caller can provide a context that will be used to verify the signature.
func DecryptSessionKeyExplicitVerifyWithContext(
	dataPacket []byte,
	sessionKey *crypto.SessionKey,
	publicKeyRing *crypto.KeyRing,
	verifyTime int64,
	verificationContext *crypto.VerificationContext,
) (*ExplicitVerifyMessage, error) {
	message, err := sessionKey.DecryptAndVerifyWithContext(dataPacket, publicKeyRing, verifyTime, verificationContext)
	return newExplicitVerifyMessage(message, err)
}

func newExplicitVerifyMessage(message *crypto.PlainMessage, err error) (*ExplicitVerifyMessage, error) {
	var explicitVerify *ExplicitVerifyMessage
	if err != nil {
		castedErr := &crypto.SignatureVerificationError{}
		isType := goerrors.As(err, castedErr)
		if !isType {
			return nil, errors.Wrap(err, "gopenpgp: unable to decrypt message")
		}

		explicitVerify = &ExplicitVerifyMessage{
			Message:                    message,
			SignatureVerificationError: castedErr,
		}
	} else {
		explicitVerify = &ExplicitVerifyMessage{
			Message:                    message,
			SignatureVerificationError: nil,
		}
	}

	return explicitVerify, nil
}

// DecryptAttachment takes a keypacket and datpacket
// and returns a decrypted PlainMessage
// Specifically designed for attachments rather than text messages.
func DecryptAttachment(keyPacket []byte, dataPacket []byte, keyRing *crypto.KeyRing) (*crypto.PlainMessage, error) {
	splitMessage := crypto.NewPGPSplitMessage(keyPacket, dataPacket)

	decrypted, err := keyRing.DecryptAttachment(splitMessage)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to decrypt attachment")
	}
	return decrypted, nil
}

// EncryptAttachment encrypts a file given a plainData and a fileName.
// Returns a PGPSplitMessage containing a session key packet and symmetrically
// encrypted data. Specifically designed for attachments rather than text
// messages.
func EncryptAttachment(plainData []byte, filename string, keyRing *crypto.KeyRing) (*crypto.PGPSplitMessage, error) {
	plainMessage := crypto.NewPlainMessageFromFile(plainData, filename, uint32(crypto.GetUnixTime()))
	decrypted, err := keyRing.EncryptAttachment(plainMessage, "")
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to encrypt attachment")
	}
	return decrypted, nil
}

// GetJsonSHA256Fingerprints returns the SHA256 fingeprints of key and subkeys,
// encoded in JSON, since gomobile can not handle arrays.
func GetJsonSHA256Fingerprints(publicKey string) ([]byte, error) {
	key, err := crypto.NewKeyFromArmored(publicKey)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to parse key")
	}

	return json.Marshal(key.GetSHA256Fingerprints())
}

type EncryptSignArmoredDetachedMobileResult struct {
	CiphertextArmored, EncryptedSignatureArmored string
}

// EncryptSignArmoredDetachedMobile wraps the encryptSignArmoredDetached method
// to have only one return argument for mobile.
func EncryptSignArmoredDetachedMobile(
	publicKey, privateKey string,
	passphrase, plainData []byte,
) (wrappedTuple *EncryptSignArmoredDetachedMobileResult, err error) {
	ciphertext, encryptedSignature, err := encryptSignArmoredDetached(publicKey, privateKey, passphrase, plainData)
	if err != nil {
		return nil, err
	}

	return &EncryptSignArmoredDetachedMobileResult{
		CiphertextArmored:         ciphertext,
		EncryptedSignatureArmored: encryptedSignature,
	}, nil
}

type EncryptSignBinaryDetachedMobileResult struct {
	EncryptedData             []byte
	EncryptedSignatureArmored string
}

// EncryptSignBinaryDetachedMobile wraps the encryptSignBinaryDetached method
// to have only one return argument for mobile.
func EncryptSignBinaryDetachedMobile(
	publicKey, privateKey string,
	passphrase, plainData []byte,
) (wrappedTuple *EncryptSignBinaryDetachedMobileResult, err error) {
	ciphertext, encryptedSignature, err := encryptSignBinaryDetached(publicKey, privateKey, passphrase, plainData)
	if err != nil {
		return nil, err
	}
	return &EncryptSignBinaryDetachedMobileResult{
		EncryptedData:             ciphertext,
		EncryptedSignatureArmored: encryptedSignature,
	}, nil
}

// FreeOSMemory can be used to explicitly
// call the garbage collector and
// return the unused memory to the OS.
func FreeOSMemory() {
	debug.FreeOSMemory()
}
