package crypto

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/ProtonMail/gopenpgp/v2/constants"
	"github.com/pkg/errors"

	"github.com/ProtonMail/go-crypto/openpgp"
	pgpErrors "github.com/ProtonMail/go-crypto/openpgp/errors"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// SessionKey stores a decrypted session key.
type SessionKey struct {
	// The decrypted binary session key.
	Key []byte
	// The symmetric encryption algorithm used with this key.
	Algo string
}

var symKeyAlgos = map[string]packet.CipherFunction{
	constants.ThreeDES:  packet.Cipher3DES,
	constants.TripleDES: packet.Cipher3DES,
	constants.CAST5:     packet.CipherCAST5,
	constants.AES128:    packet.CipherAES128,
	constants.AES192:    packet.CipherAES192,
	constants.AES256:    packet.CipherAES256,
}

type checkReader struct {
	decrypted io.ReadCloser
	body      io.Reader
}

func (cr checkReader) Read(buf []byte) (int, error) {
	n, sensitiveParsingError := cr.body.Read(buf)
	if sensitiveParsingError == io.EOF {
		mdcErr := cr.decrypted.Close()
		if mdcErr != nil {
			return n, mdcErr
		}
		return n, io.EOF
	}

	if sensitiveParsingError != nil {
		return n, pgpErrors.StructuralError("parsing error")
	}

	return n, nil
}

// GetCipherFunc returns the cipher function corresponding to the algorithm used
// with this SessionKey.
func (sk *SessionKey) GetCipherFunc() (packet.CipherFunction, error) {
	cf, ok := symKeyAlgos[sk.Algo]
	if !ok {
		return cf, errors.New("gopenpgp: unsupported cipher function: " + sk.Algo)
	}
	return cf, nil
}

// GetBase64Key returns the session key as base64 encoded string.
func (sk *SessionKey) GetBase64Key() string {
	return base64.StdEncoding.EncodeToString(sk.Key)
}

// RandomToken generates a random token with the specified key size.
func RandomToken(size int) ([]byte, error) {
	config := &packet.Config{DefaultCipher: packet.CipherAES256}
	symKey := make([]byte, size)
	if _, err := io.ReadFull(config.Random(), symKey); err != nil {
		return nil, errors.Wrap(err, "gopenpgp: error in generating random token")
	}
	return symKey, nil
}

// GenerateSessionKeyAlgo generates a random key of the correct length for the
// specified algorithm.
func GenerateSessionKeyAlgo(algo string) (sk *SessionKey, err error) {
	cf, ok := symKeyAlgos[algo]
	if !ok {
		return nil, errors.New("gopenpgp: unknown symmetric key generation algorithm")
	}
	r, err := RandomToken(cf.KeySize())
	if err != nil {
		return nil, err
	}

	sk = &SessionKey{
		Key:  r,
		Algo: algo,
	}
	return sk, nil
}

// GenerateSessionKey generates a random key for the default cipher.
func GenerateSessionKey() (*SessionKey, error) {
	return GenerateSessionKeyAlgo(constants.AES256)
}

func NewSessionKeyFromToken(token []byte, algo string) *SessionKey {
	return &SessionKey{
		Key:  clone(token),
		Algo: algo,
	}
}

func newSessionKeyFromEncrypted(ek *packet.EncryptedKey) (*SessionKey, error) {
	var algo string
	for k, v := range symKeyAlgos {
		if v == ek.CipherFunc {
			algo = k
			break
		}
	}
	if algo == "" {
		return nil, fmt.Errorf("gopenpgp: unsupported cipher function: %v", ek.CipherFunc)
	}

	sk := &SessionKey{
		Key:  ek.Key,
		Algo: algo,
	}

	if err := sk.checkSize(); err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to decrypt session key")
	}

	return sk, nil
}

// Encrypt encrypts a PlainMessage to PGPMessage with a SessionKey.
// * message : The plain data as a PlainMessage.
// * output  : The encrypted data as PGPMessage.
func (sk *SessionKey) Encrypt(message *PlainMessage) ([]byte, error) {
	return encryptWithSessionKey(message, sk, nil, false, nil)
}

// EncryptAndSign encrypts a PlainMessage to PGPMessage with a SessionKey and signs it with a Private key.
// * message : The plain data as a PlainMessage.
// * signKeyRing: The KeyRing to sign the message
// * output  : The encrypted data as PGPMessage.
func (sk *SessionKey) EncryptAndSign(message *PlainMessage, signKeyRing *KeyRing) ([]byte, error) {
	return encryptWithSessionKey(message, sk, signKeyRing, false, nil)
}

// EncryptAndSignWithContext encrypts a PlainMessage to PGPMessage with a SessionKey and signs it with a Private key.
// * message : The plain data as a PlainMessage.
// * signKeyRing: The KeyRing to sign the message
// * output  : The encrypted data as PGPMessage.
// * signingContext : (optional) the context for the signature.
func (sk *SessionKey) EncryptAndSignWithContext(message *PlainMessage, signKeyRing *KeyRing, signingContext *SigningContext) ([]byte, error) {
	return encryptWithSessionKey(message, sk, signKeyRing, false, signingContext)
}

// EncryptWithCompression encrypts with compression support a PlainMessage to PGPMessage with a SessionKey.
// * message : The plain data as a PlainMessage.
// * output  : The encrypted data as PGPMessage.
func (sk *SessionKey) EncryptWithCompression(message *PlainMessage) ([]byte, error) {
	return encryptWithSessionKey(message, sk, nil, true, nil)
}

func encryptWithSessionKey(
	message *PlainMessage,
	sk *SessionKey,
	signKeyRing *KeyRing,
	compress bool,
	signingContext *SigningContext,
) ([]byte, error) {
	var encBuf = new(bytes.Buffer)

	encryptWriter, signWriter, err := encryptStreamWithSessionKey(
		NewPlainMessageMetadata(
			message.IsBinary(),
			message.Filename,
			int64(message.Time),
		),
		encBuf,
		sk,
		signKeyRing,
		compress,
		signingContext,
	)
	if err != nil {
		return nil, err
	}
	if signKeyRing != nil {
		_, err = signWriter.Write(message.GetBinary())
		if err != nil {
			return nil, errors.Wrap(err, "gopenpgp: error in writing signed message")
		}
		err = signWriter.Close()
		if err != nil {
			return nil, errors.Wrap(err, "gopenpgp: error in closing signing writer")
		}
	} else {
		_, err = encryptWriter.Write(message.GetBinary())
	}
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: error in writing message")
	}
	err = encryptWriter.Close()
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: error in closing encryption writer")
	}
	return encBuf.Bytes(), nil
}

func encryptStreamWithSessionKey(
	plainMessageMetadata *PlainMessageMetadata,
	dataPacketWriter io.Writer,
	sk *SessionKey,
	signKeyRing *KeyRing,
	compress bool,
	signingContext *SigningContext,
) (encryptWriter, signWriter io.WriteCloser, err error) {
	dc, err := sk.GetCipherFunc()
	if err != nil {
		return nil, nil, errors.Wrap(err, "gopenpgp: unable to encrypt with session key")
	}

	config := &packet.Config{
		Time:          getTimeGenerator(),
		DefaultCipher: dc,
	}

	var signEntity *openpgp.Entity
	if signKeyRing != nil {
		signEntity, err = signKeyRing.getSigningEntity()
		if err != nil {
			return nil, nil, errors.Wrap(err, "gopenpgp: unable to sign")
		}
	}

	if compress {
		config.DefaultCompressionAlgo = constants.DefaultCompression
		config.CompressionConfig = &packet.CompressionConfig{Level: constants.DefaultCompressionLevel}
	}

	if signingContext != nil {
		config.SignatureNotations = append(config.SignatureNotations, signingContext.getNotation())
	}

	if plainMessageMetadata == nil {
		// Use sensible default metadata
		plainMessageMetadata = &PlainMessageMetadata{
			IsBinary: true,
			Filename: "",
			ModTime:  GetUnixTime(),
		}
	}

	return encryptStreamWithSessionKeyAndConfig(
		plainMessageMetadata.IsBinary,
		plainMessageMetadata.Filename,
		uint32(plainMessageMetadata.ModTime),
		dataPacketWriter,
		sk,
		signEntity,
		config,
	)
}

func encryptStreamWithSessionKeyAndConfig(
	isBinary bool,
	filename string,
	modTime uint32,
	dataPacketWriter io.Writer,
	sk *SessionKey,
	signEntity *openpgp.Entity,
	config *packet.Config,
) (encryptWriter, signWriter io.WriteCloser, err error) {
	encryptWriter, err = packet.SerializeSymmetricallyEncrypted(
		dataPacketWriter,
		config.Cipher(),
		config.AEAD() != nil,
		packet.CipherSuite{Cipher: config.Cipher(), Mode: config.AEAD().Mode()},
		sk.Key,
		config,
	)

	if err != nil {
		return nil, nil, errors.Wrap(err, "gopenpgp: unable to encrypt")
	}

	if algo := config.Compression(); algo != packet.CompressionNone {
		encryptWriter, err = packet.SerializeCompressed(encryptWriter, algo, config.CompressionConfig)
		if err != nil {
			return nil, nil, errors.Wrap(err, "gopenpgp: error in compression")
		}
	}

	if signEntity != nil {
		hints := &openpgp.FileHints{
			IsBinary: isBinary,
			FileName: filename,
			ModTime:  time.Unix(int64(modTime), 0),
		}

		signWriter, err = openpgp.Sign(encryptWriter, signEntity, hints, config)
		if err != nil {
			return nil, nil, errors.Wrap(err, "gopenpgp: unable to sign")
		}
	} else {
		encryptWriter, err = packet.SerializeLiteral(
			encryptWriter,
			isBinary,
			filename,
			modTime,
		)
		if err != nil {
			return nil, nil, errors.Wrap(err, "gopenpgp: unable to serialize")
		}
	}
	return encryptWriter, signWriter, nil
}

// Decrypt decrypts pgp data packets using directly a session key.
// * encrypted: PGPMessage.
// * output: PlainMessage.
func (sk *SessionKey) Decrypt(dataPacket []byte) (*PlainMessage, error) {
	return sk.DecryptAndVerify(dataPacket, nil, 0)
}

// DecryptAndVerify decrypts pgp data packets using directly a session key and verifies embedded signatures.
// * encrypted: PGPMessage.
// * verifyKeyRing: KeyRing with verification public keys
// * verifyTime: when should the signature be valid, as timestamp. If 0 time verification is disabled.
// * output: PlainMessage.
func (sk *SessionKey) DecryptAndVerify(dataPacket []byte, verifyKeyRing *KeyRing, verifyTime int64) (*PlainMessage, error) {
	return decryptWithSessionKeyAndContext(
		sk,
		dataPacket,
		verifyKeyRing,
		verifyTime,
		nil,
	)
}

// DecryptAndVerifyWithContext decrypts pgp data packets using directly a session key and verifies embedded signatures.
// * encrypted: PGPMessage.
// * verifyKeyRing: KeyRing with verification public keys
// * verifyTime: when should the signature be valid, as timestamp. If 0 time verification is disabled.
// * output: PlainMessage.
// * verificationContext (optional): context for the signature verification.
func (sk *SessionKey) DecryptAndVerifyWithContext(dataPacket []byte, verifyKeyRing *KeyRing, verifyTime int64, verificationContext *VerificationContext) (*PlainMessage, error) {
	return decryptWithSessionKeyAndContext(
		sk,
		dataPacket,
		verifyKeyRing,
		verifyTime,
		verificationContext,
	)
}

func decryptWithSessionKeyAndContext(
	sk *SessionKey,
	dataPacket []byte,
	verifyKeyRing *KeyRing,
	verifyTime int64,
	verificationContext *VerificationContext,
) (*PlainMessage, error) {
	var messageReader = bytes.NewReader(dataPacket)

	md, err := decryptStreamWithSessionKey(sk, messageReader, verifyKeyRing, verificationContext)
	if err != nil {
		return nil, err
	}
	messageBuf := new(bytes.Buffer)
	_, err = messageBuf.ReadFrom(md.UnverifiedBody)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: error in reading message body")
	}

	if verifyKeyRing != nil {
		processSignatureExpiration(md, verifyTime)
		err = verifyDetailsSignature(md, verifyKeyRing, verificationContext)
	}

	return &PlainMessage{
		Data:     messageBuf.Bytes(),
		TextType: !md.LiteralData.IsBinary,
		Filename: md.LiteralData.FileName,
		Time:     md.LiteralData.Time,
	}, err
}

func decryptStreamWithSessionKey(
	sk *SessionKey,
	messageReader io.Reader,
	verifyKeyRing *KeyRing,
	verificationContext *VerificationContext,
) (*openpgp.MessageDetails, error) {
	var decrypted io.ReadCloser
	var keyring openpgp.EntityList

	// Read symmetrically encrypted data packet
	packets := packet.NewReader(messageReader)
	p, err := packets.Next()
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to read symmetric packet")
	}

	// Decrypt data packet
	switch p := p.(type) {
	case *packet.SymmetricallyEncrypted, *packet.AEADEncrypted:
		dc, err := sk.GetCipherFunc()
		if err != nil {
			return nil, errors.Wrap(err, "gopenpgp: unable to decrypt with session key")
		}
		encryptedDataPacket, isDataPacket := p.(packet.EncryptedDataPacket)
		if !isDataPacket {
			return nil, errors.Wrap(err, "gopenpgp: unknown data packet")
		}
		decrypted, err = encryptedDataPacket.Decrypt(dc, sk.Key)
		if err != nil {
			return nil, errors.Wrap(err, "gopenpgp: unable to decrypt symmetric packet")
		}
	default:
		return nil, errors.New("gopenpgp: invalid packet type")
	}

	config := &packet.Config{
		Time: getTimeGenerator(),
	}

	if verificationContext != nil {
		config.KnownNotations = map[string]bool{constants.SignatureContextName: true}
	}

	// Push decrypted packet as literal packet and use openpgp's reader
	if verifyKeyRing != nil {
		keyring = verifyKeyRing.entities
	} else {
		keyring = openpgp.EntityList{}
	}

	md, err := openpgp.ReadMessage(decrypted, keyring, nil, config)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to decode symmetric packet")
	}

	md.UnverifiedBody = checkReader{decrypted, md.UnverifiedBody}
	return md, nil
}

func (sk *SessionKey) checkSize() error {
	cf, ok := symKeyAlgos[sk.Algo]
	if !ok {
		return errors.New("unknown symmetric key algorithm")
	}

	if cf.KeySize() != len(sk.Key) {
		return errors.New("wrong session key size")
	}

	return nil
}

func getAlgo(cipher packet.CipherFunction) string {
	algo := constants.AES256
	for k, v := range symKeyAlgos {
		if v == cipher {
			algo = k
			break
		}
	}

	return algo
}
