package helper

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"io"

	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/pkg/errors"
)

const AES_BLOCK_SIZE = 16

func supported(cipher packet.CipherFunction) bool {
	switch cipher {
	case packet.CipherAES128, packet.CipherAES192, packet.CipherAES256:
		return true
	case packet.CipherCAST5, packet.Cipher3DES:
		return false
	}
	return false
}

func blockSize(cipher packet.CipherFunction) int {
	switch cipher {
	case packet.CipherAES128, packet.CipherAES192, packet.CipherAES256:
		return AES_BLOCK_SIZE
	case packet.CipherCAST5, packet.Cipher3DES:
		return 0
	}
	return 0
}

func blockCipher(cipher packet.CipherFunction, key []byte) (cipher.Block, error) {
	switch cipher {
	case packet.CipherAES128, packet.CipherAES192, packet.CipherAES256:
		return aes.NewCipher(key)
	case packet.CipherCAST5, packet.Cipher3DES:
		return nil, errors.New("gopenpgp: cipher not supported for quick check")
	}
	return nil, errors.New("gopenpgp: unknown cipher")
}

// QuickCheckDecryptReader checks with high probability if the provided session key
// can decrypt a data packet given its 24 byte long prefix.
// The method reads up to but not exactly 24 bytes from the prefixReader.
// NOTE: Only works for SEIPDv1 packets with AES.
func QuickCheckDecryptReader(sessionKey *crypto.SessionKey, prefixReader crypto.Reader) (bool, error) {
	algo, err := sessionKey.GetCipherFunc()
	if err != nil {
		return false, errors.New("gopenpgp: cipher algorithm not found")
	}
	if !supported(algo) {
		return false, errors.New("gopenpgp: cipher not supported for quick check")
	}
	packetParser := packet.NewReader(prefixReader)
	_, err = packetParser.Next()
	if err != nil {
		return false, errors.New("gopenpgp: failed to parse packet prefix")
	}

	blockSize := blockSize(algo)
	encryptedData := make([]byte, blockSize+2)
	_, err = io.ReadFull(prefixReader, encryptedData)
	if err != nil {
		return false, errors.New("gopenpgp: prefix is too short to check")
	}

	blockCipher, err := blockCipher(algo, sessionKey.Key)
	if err != nil {
		return false, errors.New("gopenpgp: failed to initialize the cipher")
	}
	_ = packet.NewOCFBDecrypter(blockCipher, encryptedData, packet.OCFBNoResync)
	return encryptedData[blockSize-2] == encryptedData[blockSize] &&
		encryptedData[blockSize-1] == encryptedData[blockSize+1], nil
}

// QuickCheckDecrypt checks with high probability if the provided session key
// can decrypt the encrypted data packet given its 24 byte long prefix.
// The method only considers the first 24 bytes of the prefix slice (prefix[:24]).
// NOTE: Only works for SEIPDv1 packets with AES.
func QuickCheckDecrypt(sessionKey *crypto.SessionKey, prefix []byte) (bool, error) {
	return QuickCheckDecryptReader(sessionKey, bytes.NewReader(prefix))
}
