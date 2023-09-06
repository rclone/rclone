package crypto

import (
	"github.com/pkg/errors"
)

type signAndEncryptWriteCloser struct {
	signWriter    WriteCloser
	encryptWriter WriteCloser
}

func (w *signAndEncryptWriteCloser) Write(b []byte) (int, error) {
	return w.signWriter.Write(b)
}

func (w *signAndEncryptWriteCloser) Close() error {
	if err := w.signWriter.Close(); err != nil {
		return err
	}
	return w.encryptWriter.Close()
}

// EncryptStream is used to encrypt data as a Writer.
// It takes a writer for the encrypted data packet and returns a writer for the plaintext data.
// If signKeyRing is not nil, it is used to do an embedded signature.
func (sk *SessionKey) EncryptStream(
	dataPacketWriter Writer,
	plainMessageMetadata *PlainMessageMetadata,
	signKeyRing *KeyRing,
) (plainMessageWriter WriteCloser, err error) {
	return sk.encryptStream(
		dataPacketWriter,
		plainMessageMetadata,
		signKeyRing,
		false,
		nil,
	)
}

// EncryptStreamWithContext is used to encrypt data as a Writer.
// It takes a writer for the encrypted data packet and returns a writer for the plaintext data.
// If signKeyRing is not nil, it is used to do an embedded signature.
// * signingContext : (optional) the context for the signature.
func (sk *SessionKey) EncryptStreamWithContext(
	dataPacketWriter Writer,
	plainMessageMetadata *PlainMessageMetadata,
	signKeyRing *KeyRing,
	signingContext *SigningContext,
) (plainMessageWriter WriteCloser, err error) {
	return sk.encryptStream(
		dataPacketWriter,
		plainMessageMetadata,
		signKeyRing,
		false,
		signingContext,
	)
}

// EncryptStreamWithCompression is used to encrypt data as a Writer.
// The plaintext data is compressed before being encrypted.
// It takes a writer for the encrypted data packet and returns a writer for the plaintext data.
// If signKeyRing is not nil, it is used to do an embedded signature.
// * signingContext : (optional) the context for the signature.
func (sk *SessionKey) EncryptStreamWithCompression(
	dataPacketWriter Writer,
	plainMessageMetadata *PlainMessageMetadata,
	signKeyRing *KeyRing,
) (plainMessageWriter WriteCloser, err error) {
	return sk.encryptStream(
		dataPacketWriter,
		plainMessageMetadata,
		signKeyRing,
		true,
		nil,
	)
}

// EncryptStreamWithContextAndCompression is used to encrypt data as a Writer.
// The plaintext data is compressed before being encrypted.
// It takes a writer for the encrypted data packet and returns a writer for the plaintext data.
// If signKeyRing is not nil, it is used to do an embedded signature.
// * signingContext : (optional) the context for the signature.
func (sk *SessionKey) EncryptStreamWithContextAndCompression(
	dataPacketWriter Writer,
	plainMessageMetadata *PlainMessageMetadata,
	signKeyRing *KeyRing,
	signingContext *SigningContext,
) (plainMessageWriter WriteCloser, err error) {
	return sk.encryptStream(
		dataPacketWriter,
		plainMessageMetadata,
		signKeyRing,
		true,
		signingContext,
	)
}

func (sk *SessionKey) encryptStream(
	dataPacketWriter Writer,
	plainMessageMetadata *PlainMessageMetadata,
	signKeyRing *KeyRing,
	compress bool,
	signingContext *SigningContext,
) (plainMessageWriter WriteCloser, err error) {
	encryptWriter, signWriter, err := encryptStreamWithSessionKey(
		plainMessageMetadata,
		dataPacketWriter,
		sk,
		signKeyRing,
		compress,
		signingContext,
	)

	if err != nil {
		return nil, err
	}
	if signWriter != nil {
		plainMessageWriter = &signAndEncryptWriteCloser{signWriter, encryptWriter}
	} else {
		plainMessageWriter = encryptWriter
	}
	return plainMessageWriter, err
}

// DecryptStream is used to decrypt a data packet as a Reader.
// It takes a reader for the data packet
// and returns a PlainMessageReader for the plaintext data.
// If verifyKeyRing is not nil, PlainMessageReader.VerifySignature() will
// verify the embedded signature with the given key ring and verification time.
func (sk *SessionKey) DecryptStream(
	dataPacketReader Reader,
	verifyKeyRing *KeyRing,
	verifyTime int64,
) (plainMessage *PlainMessageReader, err error) {
	return decryptStreamWithSessionKeyAndContext(
		sk,
		dataPacketReader,
		verifyKeyRing,
		verifyTime,
		nil,
	)
}

// DecryptStreamWithContext is used to decrypt a data packet as a Reader.
// It takes a reader for the data packet
// and returns a PlainMessageReader for the plaintext data.
// If verifyKeyRing is not nil, PlainMessageReader.VerifySignature() will
// verify the embedded signature with the given key ring and verification time.
// * verificationContext (optional): context for the signature verification.
func (sk *SessionKey) DecryptStreamWithContext(
	dataPacketReader Reader,
	verifyKeyRing *KeyRing,
	verifyTime int64,
	verificationContext *VerificationContext,
) (plainMessage *PlainMessageReader, err error) {
	return decryptStreamWithSessionKeyAndContext(
		sk,
		dataPacketReader,
		verifyKeyRing,
		verifyTime,
		verificationContext,
	)
}

func decryptStreamWithSessionKeyAndContext(
	sessionKey *SessionKey,
	dataPacketReader Reader,
	verifyKeyRing *KeyRing,
	verifyTime int64,
	verificationContext *VerificationContext,
) (plainMessage *PlainMessageReader, err error) {
	messageDetails, err := decryptStreamWithSessionKey(
		sessionKey,
		dataPacketReader,
		verifyKeyRing,
		verificationContext,
	)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: error in reading message")
	}

	return &PlainMessageReader{
		messageDetails,
		verifyKeyRing,
		verifyTime,
		false,
		verificationContext,
	}, err
}
