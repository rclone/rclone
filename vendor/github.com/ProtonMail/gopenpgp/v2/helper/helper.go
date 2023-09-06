// Package helper contains several functions with a simple interface to extend usability and compatibility with gomobile
package helper

import (
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/pkg/errors"
)

// EncryptMessageWithPassword encrypts a string with a passphrase using AES256.
func EncryptMessageWithPassword(password []byte, plaintext string) (ciphertext string, err error) {
	var pgpMessage *crypto.PGPMessage

	var message = crypto.NewPlainMessageFromString(plaintext)

	if pgpMessage, err = crypto.EncryptMessageWithPassword(message, password); err != nil {
		return "", errors.Wrap(err, "gopenpgp: unable to encrypt message with password")
	}

	if ciphertext, err = pgpMessage.GetArmored(); err != nil {
		return "", errors.Wrap(err, "gopenpgp: unable to armor ciphertext")
	}

	return ciphertext, nil
}

// DecryptMessageWithPassword decrypts an armored message with a random token.
// The algorithm is derived from the armoring.
func DecryptMessageWithPassword(password []byte, ciphertext string) (plaintext string, err error) {
	var message *crypto.PlainMessage
	var pgpMessage *crypto.PGPMessage

	if pgpMessage, err = crypto.NewPGPMessageFromArmored(ciphertext); err != nil {
		return "", errors.Wrap(err, "gopenpgp: unable to unarmor ciphertext")
	}

	if message, err = crypto.DecryptMessageWithPassword(pgpMessage, password); err != nil {
		return "", errors.Wrap(err, "gopenpgp: unable to decrypt message with password")
	}

	return message.GetString(), nil
}

// EncryptMessageArmored generates an armored PGP message given a plaintext and
// an armored public key.
func EncryptMessageArmored(key, plaintext string) (string, error) {
	return encryptMessageArmored(key, crypto.NewPlainMessageFromString(plaintext))
}

// EncryptSignMessageArmored generates an armored signed PGP message given a
// plaintext and an armored public key a private key and its passphrase.
func EncryptSignMessageArmored(
	publicKey, privateKey string, passphrase []byte, plaintext string,
) (ciphertext string, err error) {
	var privateKeyObj, unlockedKeyObj *crypto.Key
	var publicKeyRing, privateKeyRing *crypto.KeyRing
	var pgpMessage *crypto.PGPMessage

	var message = crypto.NewPlainMessageFromString(plaintext)

	if publicKeyRing, err = createPublicKeyRing(publicKey); err != nil {
		return "", err
	}

	if privateKeyObj, err = crypto.NewKeyFromArmored(privateKey); err != nil {
		return "", errors.Wrap(err, "gopenpgp: unable to read key")
	}

	if unlockedKeyObj, err = privateKeyObj.Unlock(passphrase); err != nil {
		return "", errors.Wrap(err, "gopenpgp: unable to unlock key")
	}
	defer unlockedKeyObj.ClearPrivateParams()

	if privateKeyRing, err = crypto.NewKeyRing(unlockedKeyObj); err != nil {
		return "", errors.Wrap(err, "gopenpgp: unable to create new keyring")
	}

	if pgpMessage, err = publicKeyRing.Encrypt(message, privateKeyRing); err != nil {
		return "", errors.Wrap(err, "gopenpgp: unable to encrypt message")
	}

	if ciphertext, err = pgpMessage.GetArmored(); err != nil {
		return "", errors.Wrap(err, "gopenpgp: unable to armor ciphertext")
	}

	return ciphertext, nil
}

// DecryptMessageArmored decrypts an armored PGP message given a private key
// and its passphrase.
func DecryptMessageArmored(
	privateKey string, passphrase []byte, ciphertext string,
) (string, error) {
	message, err := decryptMessageArmored(privateKey, passphrase, ciphertext)
	if err != nil {
		return "", err
	}

	return message.GetString(), nil
}

// DecryptVerifyMessageArmored decrypts an armored PGP message given a private
// key and its passphrase and verifies the embedded signature. Returns the
// plain data or an error on signature verification failure.
func DecryptVerifyMessageArmored(
	publicKey, privateKey string, passphrase []byte, ciphertext string,
) (plaintext string, err error) {
	var privateKeyObj, unlockedKeyObj *crypto.Key
	var publicKeyRing, privateKeyRing *crypto.KeyRing
	var pgpMessage *crypto.PGPMessage
	var message *crypto.PlainMessage

	if publicKeyRing, err = createPublicKeyRing(publicKey); err != nil {
		return "", err
	}

	if privateKeyObj, err = crypto.NewKeyFromArmored(privateKey); err != nil {
		return "", errors.Wrap(err, "gopenpgp: unable to unarmor private key")
	}

	if unlockedKeyObj, err = privateKeyObj.Unlock(passphrase); err != nil {
		return "", errors.Wrap(err, "gopenpgp: unable to unlock private key")
	}
	defer unlockedKeyObj.ClearPrivateParams()

	if privateKeyRing, err = crypto.NewKeyRing(unlockedKeyObj); err != nil {
		return "", errors.Wrap(err, "gopenpgp: unable to create new keyring")
	}

	if pgpMessage, err = crypto.NewPGPMessageFromArmored(ciphertext); err != nil {
		return "", errors.Wrap(err, "gopenpgp: unable to unarmor ciphertext")
	}

	if message, err = privateKeyRing.Decrypt(pgpMessage, publicKeyRing, crypto.GetUnixTime()); err != nil {
		return "", errors.Wrap(err, "gopenpgp: unable to decrypt message")
	}

	return message.GetString(), nil
}

// DecryptVerifyAttachment decrypts and verifies an attachment split into the
// keyPacket, dataPacket and an armored (!) signature, given a publicKey, and a
// privateKey with its passphrase. Returns the plain data or an error on
// signature verification failure.
func DecryptVerifyAttachment(
	publicKey, privateKey string,
	passphrase, keyPacket, dataPacket []byte,
	armoredSignature string,
) (plainData []byte, err error) {
	// We decrypt the attachment
	message, err := decryptAttachment(privateKey, passphrase, keyPacket, dataPacket)
	if err != nil {
		return nil, err
	}

	// We verify the signature
	var check bool
	if check, err = verifyDetachedArmored(publicKey, message, armoredSignature); err != nil {
		return nil, err
	}
	if !check {
		return nil, errors.New("gopenpgp: unable to verify attachment")
	}

	return message.GetBinary(), nil
}

// EncryptBinaryMessageArmored generates an armored PGP message given a binary data and
// an armored public key.
func EncryptBinaryMessageArmored(key string, data []byte) (string, error) {
	return encryptMessageArmored(key, crypto.NewPlainMessage(data))
}

// DecryptBinaryMessageArmored decrypts an armored PGP message given a private key
// and its passphrase.
func DecryptBinaryMessageArmored(privateKey string, passphrase []byte, ciphertext string) ([]byte, error) {
	message, err := decryptMessageArmored(privateKey, passphrase, ciphertext)
	if err != nil {
		return nil, err
	}

	return message.GetBinary(), nil
}

// encryptSignArmoredDetached takes a public key for encryption,
// a private key and its passphrase for signature, and the plaintext data
// Returns an armored ciphertext and a detached armored encrypted signature.
func encryptSignArmoredDetached(
	publicKey, privateKey string,
	passphrase, plainData []byte,
) (ciphertextArmored, encryptedSignatureArmored string, err error) {
	var message = crypto.NewPlainMessage(plainData)

	// We encrypt and signcrypt
	ciphertext, encryptedSignatureArmored, err := encryptSignObjDetached(publicKey, privateKey, passphrase, message)
	if err != nil {
		return "", "", err
	}

	// We armor the ciphertext and signature
	ciphertextArmored, err = ciphertext.GetArmored()
	if err != nil {
		return "", "", errors.Wrap(err, "gopenpgp: unable to armor the ciphertext")
	}

	return ciphertextArmored, encryptedSignatureArmored, nil
}

// DecryptVerifyArmoredDetached decrypts an armored pgp message
// and verify a detached armored encrypted signature
// given a publicKey, and a privateKey with its passphrase.
// Returns the plain data or an error on
// signature verification failure.
func DecryptVerifyArmoredDetached(
	publicKey, privateKey string,
	passphrase []byte,
	ciphertextArmored string,
	encryptedSignatureArmored string,
) (plainData []byte, err error) {
	// Some type casting
	ciphertext, err := crypto.NewPGPMessageFromArmored(ciphertextArmored)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to unarmor ciphertext")
	}

	// We decrypt and verify the encrypted signature
	message, err := decryptVerifyObjDetached(publicKey, privateKey, passphrase, ciphertext, encryptedSignatureArmored)
	if err != nil {
		return nil, err
	}
	return message.GetBinary(), nil
}

// encryptSignBinaryDetached takes a public key for encryption,
// a private key and its passphrase for signature, and the plaintext data
// Returns encrypted binary data and a detached armored encrypted signature.
func encryptSignBinaryDetached(
	publicKey, privateKey string,
	passphrase, plainData []byte,
) (encryptedData []byte, encryptedSignatureArmored string, err error) {
	// Some type casting
	message := crypto.NewPlainMessage(plainData)

	// We encrypt and signcrypt
	ciphertext, encryptedSignatureArmored, err := encryptSignObjDetached(publicKey, privateKey, passphrase, message)
	if err != nil {
		return nil, "", err
	}

	// We get the encrypted data
	encryptedData = ciphertext.GetBinary()
	return encryptedData, encryptedSignatureArmored, nil
}

// DecryptVerifyBinaryDetached decrypts binary encrypted data
// and verify a detached armored encrypted signature
// given a publicKey, and a privateKey with its passphrase.
// Returns the plain data or an error on
// signature verification failure.
func DecryptVerifyBinaryDetached(
	publicKey, privateKey string,
	passphrase []byte,
	encryptedData []byte,
	encryptedSignatureArmored string,
) (plainData []byte, err error) {
	// Some type casting
	ciphertext := crypto.NewPGPMessage(encryptedData)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to parse ciphertext")
	}

	// We decrypt and verify the encrypted signature
	message, err := decryptVerifyObjDetached(publicKey, privateKey, passphrase, ciphertext, encryptedSignatureArmored)
	if err != nil {
		return nil, err
	}
	return message.GetBinary(), nil
}

// EncryptAttachmentWithKey encrypts a binary file
// Using a given armored public key.
func EncryptAttachmentWithKey(
	publicKey string,
	filename string,
	plainData []byte,
) (message *crypto.PGPSplitMessage, err error) {
	publicKeyRing, err := createPublicKeyRing(publicKey)
	if err != nil {
		return nil, err
	}
	return EncryptAttachment(plainData, filename, publicKeyRing)
}

// DecryptAttachmentWithKey decrypts a binary file
// Using a given armored private key and its passphrase.
func DecryptAttachmentWithKey(
	privateKey string,
	passphrase, keyPacket, dataPacket []byte,
) (attachment []byte, err error) {
	message, err := decryptAttachment(privateKey, passphrase, keyPacket, dataPacket)
	if err != nil {
		return nil, err
	}
	return message.GetBinary(), nil
}

// EncryptSessionKey encrypts a session key
// using a given armored public key.
func EncryptSessionKey(
	publicKey string,
	sessionKey *crypto.SessionKey,
) (encryptedSessionKey []byte, err error) {
	publicKeyRing, err := createPublicKeyRing(publicKey)
	if err != nil {
		return nil, err
	}
	encryptedSessionKey, err = publicKeyRing.EncryptSessionKey(sessionKey)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to encrypt sessionKey")
	}
	return encryptedSessionKey, nil
}

// DecryptSessionKey decrypts a session key
// using a given armored private key
// and its passphrase.
func DecryptSessionKey(
	privateKey string,
	passphrase, encryptedSessionKey []byte,
) (sessionKey *crypto.SessionKey, err error) {
	privateKeyObj, err := crypto.NewKeyFromArmored(privateKey)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to read armored key")
	}

	privateKeyUnlocked, err := privateKeyObj.Unlock(passphrase)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to unlock private key")
	}

	defer privateKeyUnlocked.ClearPrivateParams()

	privateKeyRing, err := crypto.NewKeyRing(privateKeyUnlocked)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to create new keyring")
	}

	sessionKey, err = privateKeyRing.DecryptSessionKey(encryptedSessionKey)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to decrypt session key")
	}

	return sessionKey, nil
}

func encryptMessageArmored(key string, message *crypto.PlainMessage) (string, error) {
	ciphertext, err := encryptMessage(key, message)
	if err != nil {
		return "", err
	}

	ciphertextArmored, err := ciphertext.GetArmored()
	if err != nil {
		return "", errors.Wrap(err, "gopenpgp: unable to armor ciphertext")
	}

	return ciphertextArmored, nil
}

func decryptMessageArmored(privateKey string, passphrase []byte, ciphertextArmored string) (*crypto.PlainMessage, error) {
	ciphertext, err := crypto.NewPGPMessageFromArmored(ciphertextArmored)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to parse ciphertext")
	}

	return decryptMessage(privateKey, passphrase, ciphertext)
}

func encryptMessage(key string, message *crypto.PlainMessage) (*crypto.PGPMessage, error) {
	publicKeyRing, err := createPublicKeyRing(key)
	if err != nil {
		return nil, err
	}

	ciphertext, err := publicKeyRing.Encrypt(message, nil)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to encrypt message")
	}

	return ciphertext, nil
}

func decryptMessage(privateKey string, passphrase []byte, ciphertext *crypto.PGPMessage) (*crypto.PlainMessage, error) {
	privateKeyObj, err := crypto.NewKeyFromArmored(privateKey)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to parse the private key")
	}

	privateKeyUnlocked, err := privateKeyObj.Unlock(passphrase)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to unlock key")
	}

	defer privateKeyUnlocked.ClearPrivateParams()

	privateKeyRing, err := crypto.NewKeyRing(privateKeyUnlocked)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to create the private key ring")
	}

	message, err := privateKeyRing.Decrypt(ciphertext, nil, 0)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to decrypt message")
	}

	return message, nil
}

func signDetached(privateKey string, passphrase []byte, message *crypto.PlainMessage) (detachedSignature *crypto.PGPSignature, err error) {
	privateKeyObj, err := crypto.NewKeyFromArmored(privateKey)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to parse private key")
	}

	privateKeyUnlocked, err := privateKeyObj.Unlock(passphrase)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to unlock key")
	}

	defer privateKeyUnlocked.ClearPrivateParams()

	privateKeyRing, err := crypto.NewKeyRing(privateKeyUnlocked)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to create new keyring")
	}

	detachedSignature, err = privateKeyRing.SignDetached(message)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to sign message")
	}

	return detachedSignature, nil
}

func verifyDetachedArmored(publicKey string, message *crypto.PlainMessage, armoredSignature string) (check bool, err error) {
	var detachedSignature *crypto.PGPSignature

	// We unarmor the signature
	if detachedSignature, err = crypto.NewPGPSignatureFromArmored(armoredSignature); err != nil {
		return false, errors.Wrap(err, "gopenpgp: unable to unarmor signature")
	}
	// we verify the signature
	return verifyDetached(publicKey, message, detachedSignature)
}

func verifyDetached(publicKey string, message *crypto.PlainMessage, detachedSignature *crypto.PGPSignature) (check bool, err error) {
	var publicKeyRing *crypto.KeyRing

	// We prepare the public key for signature verification
	publicKeyRing, err = createPublicKeyRing(publicKey)
	if err != nil {
		return false, err
	}

	// We verify the signature
	if publicKeyRing.VerifyDetached(message, detachedSignature, crypto.GetUnixTime()) != nil {
		return false, nil
	}
	return true, nil
}

func decryptAttachment(
	privateKey string,
	passphrase, keyPacket, dataPacket []byte,
) (message *crypto.PlainMessage, err error) {
	var privateKeyObj, unlockedKeyObj *crypto.Key
	var privateKeyRing *crypto.KeyRing

	packets := crypto.NewPGPSplitMessage(keyPacket, dataPacket)

	// prepare the private key for decryption
	if privateKeyObj, err = crypto.NewKeyFromArmored(privateKey); err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to parse private key")
	}
	if unlockedKeyObj, err = privateKeyObj.Unlock(passphrase); err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to unlock private key")
	}
	defer unlockedKeyObj.ClearPrivateParams()

	if privateKeyRing, err = crypto.NewKeyRing(unlockedKeyObj); err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to create new keyring")
	}

	if message, err = privateKeyRing.DecryptAttachment(packets); err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to decrypt attachment")
	}

	return message, nil
}

func createPublicKeyRing(publicKey string) (*crypto.KeyRing, error) {
	publicKeyObj, err := crypto.NewKeyFromArmored(publicKey)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to parse public key")
	}

	if publicKeyObj.IsPrivate() {
		publicKeyObj, err = publicKeyObj.ToPublic()
		if err != nil {
			return nil, errors.Wrap(err, "gopenpgp: unable to extract public key from private key")
		}
	}

	publicKeyRing, err := crypto.NewKeyRing(publicKeyObj)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to create new keyring")
	}

	return publicKeyRing, nil
}

func encryptSignObjDetached(
	publicKey, privateKey string,
	passphrase []byte,
	message *crypto.PlainMessage,
) (ciphertext *crypto.PGPSplitMessage, encryptedSignatureArmored string, err error) {
	// We generate the session key
	sessionKey, err := crypto.GenerateSessionKey()
	if err != nil {
		return nil, "", errors.Wrap(err, "gopenpgp: unable to create new session key")
	}

	// We encrypt the message with the session key
	messageDataPacket, err := sessionKey.Encrypt(message)
	if err != nil {
		return nil, "", errors.Wrap(err, "gopenpgp: unable to encrypt message")
	}

	// We sign the message
	detachedSignature, err := signDetached(privateKey, passphrase, message)
	if err != nil {
		return nil, "", errors.Wrap(err, "gopenpgp: unable to sign message")
	}

	// We encrypt the signature with the session key
	signaturePlaintext := crypto.NewPlainMessage(detachedSignature.GetBinary())
	signatureDataPacket, err := sessionKey.Encrypt(signaturePlaintext)
	if err != nil {
		return nil, "", errors.Wrap(err, "gopenpgp: unable to encrypt detached signature")
	}

	// We encrypt the session key
	keyPacket, err := EncryptSessionKey(publicKey, sessionKey)
	if err != nil {
		return nil, "", errors.Wrap(err, "gopenpgp: unable to encrypt the session key")
	}

	// We join the key packets and datapackets
	ciphertext = crypto.NewPGPSplitMessage(keyPacket, messageDataPacket)
	encryptedSignature := crypto.NewPGPSplitMessage(keyPacket, signatureDataPacket)
	encryptedSignatureArmored, err = encryptedSignature.GetArmored()
	if err != nil {
		return nil, "", errors.Wrap(err, "gopenpgp: unable to armor encrypted signature")
	}
	return ciphertext, encryptedSignatureArmored, nil
}

func decryptVerifyObjDetached(
	publicKey, privateKey string,
	passphrase []byte,
	ciphertext *crypto.PGPMessage,
	encryptedSignatureArmored string,
) (message *crypto.PlainMessage, err error) {
	// We decrypt the message
	if message, err = decryptMessage(privateKey, passphrase, ciphertext); err != nil {
		return nil, err
	}

	encryptedSignature, err := crypto.NewPGPMessageFromArmored(encryptedSignatureArmored)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to parse encrypted signature")
	}

	// We decrypt the signature
	signatureMessage, err := decryptMessage(privateKey, passphrase, encryptedSignature)
	if err != nil {
		return nil, err
	}
	detachedSignature := crypto.NewPGPSignature(signatureMessage.GetBinary())

	// We verify the signature
	var check bool
	if check, err = verifyDetached(publicKey, message, detachedSignature); err != nil {
		return nil, err
	}
	if !check {
		return nil, errors.New("gopenpgp: unable to verify message")
	}

	return message, nil
}
