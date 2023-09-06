package crypto

import (
	"bytes"
	"io"

	"github.com/ProtonMail/go-crypto/openpgp"
	pgpErrors "github.com/ProtonMail/go-crypto/openpgp/errors"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/pkg/errors"
)

// EncryptMessageWithPassword encrypts a PlainMessage to PGPMessage with a
// SymmetricKey.
// * message : The plain data as a PlainMessage.
// * password: A password that will be derived into an encryption key.
// * output  : The encrypted data as PGPMessage.
func EncryptMessageWithPassword(message *PlainMessage, password []byte) (*PGPMessage, error) {
	encrypted, err := passwordEncrypt(message, password)
	if err != nil {
		return nil, err
	}

	return NewPGPMessage(encrypted), nil
}

// DecryptMessageWithPassword decrypts password protected pgp binary messages.
// * encrypted: The encrypted data as PGPMessage.
// * password: A password that will be derived into an encryption key.
// * output: The decrypted data as PlainMessage.
func DecryptMessageWithPassword(message *PGPMessage, password []byte) (*PlainMessage, error) {
	return passwordDecrypt(message.NewReader(), password)
}

// DecryptSessionKeyWithPassword decrypts the binary symmetrically encrypted
// session key packet and returns the session key.
func DecryptSessionKeyWithPassword(keyPacket, password []byte) (*SessionKey, error) {
	keyReader := bytes.NewReader(keyPacket)
	packets := packet.NewReader(keyReader)

	var symKeys []*packet.SymmetricKeyEncrypted
	for {
		var p packet.Packet
		var err error
		if p, err = packets.Next(); err != nil {
			break
		}

		if p, ok := p.(*packet.SymmetricKeyEncrypted); ok {
			symKeys = append(symKeys, p)
		}
	}

	// Try the symmetric passphrase first
	if len(symKeys) != 0 && password != nil {
		for _, s := range symKeys {
			key, cipherFunc, err := s.Decrypt(password)
			if err == nil {
				sk := &SessionKey{
					Key:  key,
					Algo: getAlgo(cipherFunc),
				}

				if err = sk.checkSize(); err != nil {
					return nil, errors.Wrap(err, "gopenpgp: unable to decrypt session key with password")
				}

				return sk, nil
			}
		}
	}

	return nil, errors.New("gopenpgp: unable to decrypt any packet")
}

// EncryptSessionKeyWithPassword encrypts the session key with the password and
// returns a binary symmetrically encrypted session key packet.
func EncryptSessionKeyWithPassword(sk *SessionKey, password []byte) ([]byte, error) {
	outbuf := &bytes.Buffer{}

	cf, err := sk.GetCipherFunc()
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to encrypt session key with password")
	}

	if len(password) == 0 {
		return nil, errors.New("gopenpgp: password can't be empty")
	}

	if err = sk.checkSize(); err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to encrypt session key with password")
	}

	config := &packet.Config{
		DefaultCipher: cf,
	}

	err = packet.SerializeSymmetricKeyEncryptedReuseKey(outbuf, sk.Key, password, config)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: unable to encrypt session key with password")
	}
	return outbuf.Bytes(), nil
}

// ----- INTERNAL FUNCTIONS ------

func passwordEncrypt(message *PlainMessage, password []byte) ([]byte, error) {
	var outBuf bytes.Buffer

	config := &packet.Config{
		DefaultCipher: packet.CipherAES256,
		Time:          getTimeGenerator(),
	}

	hints := &openpgp.FileHints{
		IsBinary: message.IsBinary(),
		FileName: message.Filename,
		ModTime:  message.getFormattedTime(),
	}

	encryptWriter, err := openpgp.SymmetricallyEncrypt(&outBuf, password, hints, config)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: error in encrypting message symmetrically")
	}
	_, err = encryptWriter.Write(message.GetBinary())
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: error in writing data to message")
	}

	err = encryptWriter.Close()
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: error in closing writer")
	}

	return outBuf.Bytes(), nil
}

func passwordDecrypt(encryptedIO io.Reader, password []byte) (*PlainMessage, error) {
	firstTimeCalled := true
	var prompt = func(keys []openpgp.Key, symmetric bool) ([]byte, error) {
		if firstTimeCalled {
			firstTimeCalled = false
			return password, nil
		}
		// Re-prompt still occurs if SKESK pasrsing fails (i.e. when decrypted cipher algo is invalid).
		// For most (but not all) cases, inputting a wrong passwords is expected to trigger this error.
		return nil, errors.New("gopenpgp: wrong password in symmetric decryption")
	}

	config := &packet.Config{
		Time: getTimeGenerator(),
	}

	var emptyKeyRing openpgp.EntityList
	md, err := openpgp.ReadMessage(encryptedIO, emptyKeyRing, prompt, config)
	if err != nil {
		// Parsing errors when reading the message are most likely caused by incorrect password, but we cannot know for sure
		return nil, errors.New("gopenpgp: error in reading password protected message: wrong password or malformed message")
	}

	messageBuf := bytes.NewBuffer(nil)
	_, err = io.Copy(messageBuf, md.UnverifiedBody)
	if errors.Is(err, pgpErrors.ErrMDCHashMismatch) {
		// This MDC error may also be triggered if the password is correct, but the encrypted data was corrupted.
		// To avoid confusion, we do not inform the user about the second possibility.
		return nil, errors.New("gopenpgp: wrong password in symmetric decryption")
	}
	if err != nil {
		// Parsing errors after decryption, triggered before parsing the MDC packet, are also usually the result of wrong password
		return nil, errors.New("gopenpgp: error in reading password protected message: wrong password or malformed message")
	}

	return &PlainMessage{
		Data:     messageBuf.Bytes(),
		TextType: !md.LiteralData.IsBinary,
		Filename: md.LiteralData.FileName,
		Time:     md.LiteralData.Time,
	}, nil
}
