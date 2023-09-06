package crypto

import (
	"bytes"
	"io"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/pkg/errors"
)

type Reader interface {
	Read(b []byte) (n int, err error)
}

type Writer interface {
	Write(b []byte) (n int, err error)
}

type WriteCloser interface {
	Write(b []byte) (n int, err error)
	Close() (err error)
}

type PlainMessageMetadata struct {
	IsBinary bool
	Filename string
	ModTime  int64
}

func NewPlainMessageMetadata(isBinary bool, filename string, modTime int64) *PlainMessageMetadata {
	return &PlainMessageMetadata{IsBinary: isBinary, Filename: filename, ModTime: modTime}
}

// EncryptStream is used to encrypt data as a Writer.
// It takes a writer for the encrypted data and returns a WriteCloser for the plaintext data
// If signKeyRing is not nil, it is used to do an embedded signature.
func (keyRing *KeyRing) EncryptStream(
	pgpMessageWriter Writer,
	plainMessageMetadata *PlainMessageMetadata,
	signKeyRing *KeyRing,
) (plainMessageWriter WriteCloser, err error) {
	return encryptStream(
		keyRing,
		pgpMessageWriter,
		pgpMessageWriter,
		plainMessageMetadata,
		signKeyRing,
		false,
		nil,
	)
}

// EncryptStreamWithContext is used to encrypt data as a Writer.
// It takes a writer for the encrypted data and returns a WriteCloser for the plaintext data
// If signKeyRing is not nil, it is used to do an embedded signature.
// * signingContext : (optional) a context for the embedded signature.
func (keyRing *KeyRing) EncryptStreamWithContext(
	pgpMessageWriter Writer,
	plainMessageMetadata *PlainMessageMetadata,
	signKeyRing *KeyRing,
	signingContext *SigningContext,
) (plainMessageWriter WriteCloser, err error) {
	return encryptStream(
		keyRing,
		pgpMessageWriter,
		pgpMessageWriter,
		plainMessageMetadata,
		signKeyRing,
		false,
		signingContext,
	)
}

// EncryptStreamWithCompression is used to encrypt data as a Writer.
// The plaintext data is compressed before being encrypted.
// It takes a writer for the encrypted data and returns a WriteCloser for the plaintext data
// If signKeyRing is not nil, it is used to do an embedded signature.
func (keyRing *KeyRing) EncryptStreamWithCompression(
	pgpMessageWriter Writer,
	plainMessageMetadata *PlainMessageMetadata,
	signKeyRing *KeyRing,
) (plainMessageWriter WriteCloser, err error) {
	return encryptStream(
		keyRing,
		pgpMessageWriter,
		pgpMessageWriter,
		plainMessageMetadata,
		signKeyRing,
		true,
		nil,
	)
}

// EncryptStreamWithContextAndCompression is used to encrypt data as a Writer.
// The plaintext data is compressed before being encrypted.
// It takes a writer for the encrypted data and returns a WriteCloser for the plaintext data
// If signKeyRing is not nil, it is used to do an embedded signature.
// * signingContext : (optional) a context for the embedded signature.
func (keyRing *KeyRing) EncryptStreamWithContextAndCompression(
	pgpMessageWriter Writer,
	plainMessageMetadata *PlainMessageMetadata,
	signKeyRing *KeyRing,
	signingContext *SigningContext,
) (plainMessageWriter WriteCloser, err error) {
	return encryptStream(
		keyRing,
		pgpMessageWriter,
		pgpMessageWriter,
		plainMessageMetadata,
		signKeyRing,
		true,
		signingContext,
	)
}

func encryptStream(
	encryptionKeyRing *KeyRing,
	keyPacketWriter Writer,
	dataPacketWriter Writer,
	plainMessageMetadata *PlainMessageMetadata,
	signKeyRing *KeyRing,
	compress bool,
	signingContext *SigningContext,
) (plainMessageWriter WriteCloser, err error) {
	if plainMessageMetadata == nil {
		// Use sensible default metadata
		plainMessageMetadata = &PlainMessageMetadata{
			IsBinary: true,
			Filename: "",
			ModTime:  GetUnixTime(),
		}
	}

	hints := &openpgp.FileHints{
		FileName: plainMessageMetadata.Filename,
		IsBinary: plainMessageMetadata.IsBinary,
		ModTime:  time.Unix(plainMessageMetadata.ModTime, 0),
	}

	plainMessageWriter, err = asymmetricEncryptStream(hints, keyPacketWriter, dataPacketWriter, encryptionKeyRing, signKeyRing, compress, signingContext)
	if err != nil {
		return nil, err
	}
	return plainMessageWriter, nil
}

// EncryptSplitResult is used to wrap the encryption writecloser while storing the key packet.
type EncryptSplitResult struct {
	isClosed           bool
	keyPacketBuf       *bytes.Buffer
	keyPacket          []byte
	plainMessageWriter WriteCloser // The writer to writer plaintext data in.
}

func (res *EncryptSplitResult) Write(b []byte) (n int, err error) {
	return res.plainMessageWriter.Write(b)
}

func (res *EncryptSplitResult) Close() (err error) {
	err = res.plainMessageWriter.Close()
	if err != nil {
		return err
	}
	res.isClosed = true
	res.keyPacket = res.keyPacketBuf.Bytes()
	return nil
}

// GetKeyPacket returns the Public-Key Encrypted Session Key Packets (https://datatracker.ietf.org/doc/html/rfc4880#section-5.1).
// This can be retrieved only after the message has been fully written and the writer is closed.
func (res *EncryptSplitResult) GetKeyPacket() (keyPacket []byte, err error) {
	if !res.isClosed {
		return nil, errors.New("gopenpgp: can't access key packet until the message writer has been closed")
	}
	return res.keyPacket, nil
}

// EncryptSplitStream is used to encrypt data as a stream.
// It takes a writer for the Symmetrically Encrypted Data Packet
// (https://datatracker.ietf.org/doc/html/rfc4880#section-5.7)
// and returns a writer for the plaintext data and the key packet.
// If signKeyRing is not nil, it is used to do an embedded signature.
func (keyRing *KeyRing) EncryptSplitStream(
	dataPacketWriter Writer,
	plainMessageMetadata *PlainMessageMetadata,
	signKeyRing *KeyRing,
) (*EncryptSplitResult, error) {
	return encryptSplitStream(
		keyRing,
		dataPacketWriter,
		plainMessageMetadata,
		signKeyRing,
		false,
		nil,
	)
}

// EncryptSplitStreamWithContext is used to encrypt data as a stream.
// It takes a writer for the Symmetrically Encrypted Data Packet
// (https://datatracker.ietf.org/doc/html/rfc4880#section-5.7)
// and returns a writer for the plaintext data and the key packet.
// If signKeyRing is not nil, it is used to do an embedded signature.
// * signingContext : (optional) a context for the embedded signature.
func (keyRing *KeyRing) EncryptSplitStreamWithContext(
	dataPacketWriter Writer,
	plainMessageMetadata *PlainMessageMetadata,
	signKeyRing *KeyRing,
	signingContext *SigningContext,
) (*EncryptSplitResult, error) {
	return encryptSplitStream(
		keyRing,
		dataPacketWriter,
		plainMessageMetadata,
		signKeyRing,
		false,
		signingContext,
	)
}

// EncryptSplitStreamWithCompression is used to encrypt data as a stream.
// It takes a writer for the Symmetrically Encrypted Data Packet
// (https://datatracker.ietf.org/doc/html/rfc4880#section-5.7)
// and returns a writer for the plaintext data and the key packet.
// If signKeyRing is not nil, it is used to do an embedded signature.
func (keyRing *KeyRing) EncryptSplitStreamWithCompression(
	dataPacketWriter Writer,
	plainMessageMetadata *PlainMessageMetadata,
	signKeyRing *KeyRing,
) (*EncryptSplitResult, error) {
	return encryptSplitStream(
		keyRing,
		dataPacketWriter,
		plainMessageMetadata,
		signKeyRing,
		true,
		nil,
	)
}

// EncryptSplitStreamWithContextAndCompression is used to encrypt data as a stream.
// It takes a writer for the Symmetrically Encrypted Data Packet
// (https://datatracker.ietf.org/doc/html/rfc4880#section-5.7)
// and returns a writer for the plaintext data and the key packet.
// If signKeyRing is not nil, it is used to do an embedded signature.
// * signingContext : (optional) a context for the embedded signature.
func (keyRing *KeyRing) EncryptSplitStreamWithContextAndCompression(
	dataPacketWriter Writer,
	plainMessageMetadata *PlainMessageMetadata,
	signKeyRing *KeyRing,
	signingContext *SigningContext,
) (*EncryptSplitResult, error) {
	return encryptSplitStream(
		keyRing,
		dataPacketWriter,
		plainMessageMetadata,
		signKeyRing,
		true,
		signingContext,
	)
}

func encryptSplitStream(
	encryptionKeyRing *KeyRing,
	dataPacketWriter Writer,
	plainMessageMetadata *PlainMessageMetadata,
	signKeyRing *KeyRing,
	compress bool,
	signingContext *SigningContext,
) (*EncryptSplitResult, error) {
	var keyPacketBuf bytes.Buffer
	plainMessageWriter, err := encryptStream(
		encryptionKeyRing,
		&keyPacketBuf,
		dataPacketWriter,
		plainMessageMetadata,
		signKeyRing,
		compress,
		signingContext,
	)
	if err != nil {
		return nil, err
	}

	return &EncryptSplitResult{
		keyPacketBuf:       &keyPacketBuf,
		plainMessageWriter: plainMessageWriter,
	}, nil
}

// PlainMessageReader is used to wrap the data of the decrypted plain message.
// It can be used to read the decrypted data and verify the embedded signature.
type PlainMessageReader struct {
	details             *openpgp.MessageDetails
	verifyKeyRing       *KeyRing
	verifyTime          int64
	readAll             bool
	verificationContext *VerificationContext
}

// GetMetadata returns the metadata of the decrypted message.
func (msg *PlainMessageReader) GetMetadata() *PlainMessageMetadata {
	return &PlainMessageMetadata{
		Filename: msg.details.LiteralData.FileName,
		IsBinary: msg.details.LiteralData.IsBinary,
		ModTime:  int64(msg.details.LiteralData.Time),
	}
}

// Read is used to access the message decrypted data.
// Makes PlainMessageReader implement the Reader interface.
func (msg *PlainMessageReader) Read(b []byte) (n int, err error) {
	n, err = msg.details.UnverifiedBody.Read(b)
	if errors.Is(err, io.EOF) {
		msg.readAll = true
	}
	return
}

// VerifySignature is used to verify that the signature is valid.
// This method needs to be called once all the data has been read.
// It will return an error if the signature is invalid
// or if the message hasn't been read entirely.
func (msg *PlainMessageReader) VerifySignature() (err error) {
	if !msg.readAll {
		return errors.New("gopenpgp: can't verify the signature until the message reader has been read entirely")
	}
	if msg.verifyKeyRing != nil {
		processSignatureExpiration(msg.details, msg.verifyTime)
		err = verifyDetailsSignature(msg.details, msg.verifyKeyRing, msg.verificationContext)
	} else {
		err = errors.New("gopenpgp: no verify keyring was provided before decryption")
	}
	return
}

// DecryptStream is used to decrypt a pgp message as a Reader.
// It takes a reader for the message data
// and returns a PlainMessageReader for the plaintext data.
// If verifyKeyRing is not nil, PlainMessageReader.VerifySignature() will
// verify the embedded signature with the given key ring and verification time.
func (keyRing *KeyRing) DecryptStream(
	message Reader,
	verifyKeyRing *KeyRing,
	verifyTime int64,
) (plainMessage *PlainMessageReader, err error) {
	return decryptStream(
		keyRing,
		message,
		verifyKeyRing,
		verifyTime,
		nil,
	)
}

// DecryptStreamWithContext is used to decrypt a pgp message as a Reader.
// It takes a reader for the message data
// and returns a PlainMessageReader for the plaintext data.
// If verifyKeyRing is not nil, PlainMessageReader.VerifySignature() will
// verify the embedded signature with the given key ring and verification time.
// * verificationContext (optional): context for the signature verification.
func (keyRing *KeyRing) DecryptStreamWithContext(
	message Reader,
	verifyKeyRing *KeyRing,
	verifyTime int64,
	verificationContext *VerificationContext,
) (plainMessage *PlainMessageReader, err error) {
	return decryptStream(
		keyRing,
		message,
		verifyKeyRing,
		verifyTime,
		verificationContext,
	)
}

func decryptStream(
	decryptionKeyRing *KeyRing,
	message Reader,
	verifyKeyRing *KeyRing,
	verifyTime int64,
	verificationContext *VerificationContext,
) (plainMessage *PlainMessageReader, err error) {
	messageDetails, err := asymmetricDecryptStream(
		message,
		decryptionKeyRing,
		verifyKeyRing,
		verifyTime,
		verificationContext,
	)
	if err != nil {
		return nil, err
	}

	return &PlainMessageReader{
		messageDetails,
		verifyKeyRing,
		verifyTime,
		false,
		verificationContext,
	}, err
}

// DecryptSplitStream is used to decrypt a split pgp message as a Reader.
// It takes a key packet and a reader for the data packet
// and returns a PlainMessageReader for the plaintext data.
// If verifyKeyRing is not nil, PlainMessageReader.VerifySignature() will
// verify the embedded signature with the given key ring and verification time.
func (keyRing *KeyRing) DecryptSplitStream(
	keypacket []byte,
	dataPacketReader Reader,
	verifyKeyRing *KeyRing, verifyTime int64,
) (plainMessage *PlainMessageReader, err error) {
	messageReader := io.MultiReader(
		bytes.NewReader(keypacket),
		dataPacketReader,
	)
	return keyRing.DecryptStream(
		messageReader,
		verifyKeyRing,
		verifyTime,
	)
}

// DecryptSplitStreamWithContext is used to decrypt a split pgp message as a Reader.
// It takes a key packet and a reader for the data packet
// and returns a PlainMessageReader for the plaintext data.
// If verifyKeyRing is not nil, PlainMessageReader.VerifySignature() will
// verify the embedded signature with the given key ring and verification time.
// * verificationContext (optional): context for the signature verification.
func (keyRing *KeyRing) DecryptSplitStreamWithContext(
	keypacket []byte,
	dataPacketReader Reader,
	verifyKeyRing *KeyRing, verifyTime int64,
	verificationContext *VerificationContext,
) (plainMessage *PlainMessageReader, err error) {
	messageReader := io.MultiReader(
		bytes.NewReader(keypacket),
		dataPacketReader,
	)
	return keyRing.DecryptStreamWithContext(
		messageReader,
		verifyKeyRing,
		verifyTime,
		verificationContext,
	)
}

// SignDetachedStream generates and returns a PGPSignature for a given message Reader.
func (keyRing *KeyRing) SignDetachedStream(message Reader) (*PGPSignature, error) {
	return keyRing.SignDetachedStreamWithContext(message, nil)
}

// SignDetachedStreamWithContext generates and returns a PGPSignature for a given message Reader.
// If a context is provided, it is added to the signature as notation data
// with the name set in `constants.SignatureContextName`.
func (keyRing *KeyRing) SignDetachedStreamWithContext(message Reader, context *SigningContext) (*PGPSignature, error) {
	return signMessageDetached(
		keyRing,
		message,
		true,
		context,
	)
}

// VerifyDetachedStream verifies a message reader with a detached PGPSignature
// and returns a SignatureVerificationError if fails.
func (keyRing *KeyRing) VerifyDetachedStream(
	message Reader,
	signature *PGPSignature,
	verifyTime int64,
) error {
	_, err := verifySignature(
		keyRing.entities,
		message,
		signature.GetBinary(),
		verifyTime,
		nil,
	)
	return err
}

// VerifyDetachedStreamWithContext verifies a message reader with a detached PGPSignature
// and returns a SignatureVerificationError if fails.
// If a context is provided, it verifies that the signature is valid in the given context, using
// the signature notations.
func (keyRing *KeyRing) VerifyDetachedStreamWithContext(
	message Reader,
	signature *PGPSignature,
	verifyTime int64,
	verificationContext *VerificationContext,
) error {
	_, err := verifySignature(
		keyRing.entities,
		message,
		signature.GetBinary(),
		verifyTime,
		verificationContext,
	)
	return err
}

// SignDetachedEncryptedStream generates and returns a PGPMessage
// containing an encrypted detached signature for a given message Reader.
func (keyRing *KeyRing) SignDetachedEncryptedStream(
	message Reader,
	encryptionKeyRing *KeyRing,
) (encryptedSignature *PGPMessage, err error) {
	if encryptionKeyRing == nil {
		return nil, errors.New("gopenpgp: no encryption key ring provided")
	}
	signature, err := keyRing.SignDetachedStream(message)
	if err != nil {
		return nil, err
	}
	plainMessage := NewPlainMessage(signature.GetBinary())
	encryptedSignature, err = encryptionKeyRing.Encrypt(plainMessage, nil)
	return
}

// VerifyDetachedEncryptedStream verifies a PlainMessage
// with a PGPMessage containing an encrypted detached signature
// and returns a SignatureVerificationError if fails.
func (keyRing *KeyRing) VerifyDetachedEncryptedStream(
	message Reader,
	encryptedSignature *PGPMessage,
	decryptionKeyRing *KeyRing,
	verifyTime int64,
) error {
	if decryptionKeyRing == nil {
		return errors.New("gopenpgp: no decryption key ring provided")
	}
	plainMessage, err := decryptionKeyRing.Decrypt(encryptedSignature, nil, 0)
	if err != nil {
		return err
	}
	signature := NewPGPSignature(plainMessage.GetBinary())
	return keyRing.VerifyDetachedStream(message, signature, verifyTime)
}
