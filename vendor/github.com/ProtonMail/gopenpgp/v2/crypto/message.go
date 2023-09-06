package crypto

import (
	"bytes"
	"encoding/base64"
	goerrors "errors"
	"io"
	"io/ioutil"
	"regexp"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/ProtonMail/gopenpgp/v2/armor"
	"github.com/ProtonMail/gopenpgp/v2/constants"
	"github.com/ProtonMail/gopenpgp/v2/internal"
	"github.com/pkg/errors"
)

// ---- MODELS -----

// PlainMessage stores a plain text / unencrypted message.
type PlainMessage struct {
	// The content of the message
	Data []byte
	// If the content is text or binary
	TextType bool
	// The file's latest modification time
	Time uint32
	// The encrypted message's filename
	Filename string
}

// PGPMessage stores a PGP-encrypted message.
type PGPMessage struct {
	// The content of the message
	Data []byte
}

// PGPSignature stores a PGP-encoded detached signature.
type PGPSignature struct {
	// The content of the signature
	Data []byte
}

// PGPSplitMessage contains a separate session key packet and symmetrically
// encrypted data packet.
type PGPSplitMessage struct {
	DataPacket []byte
	KeyPacket  []byte
}

// A ClearTextMessage is a signed but not encrypted PGP message,
// i.e. the ones beginning with -----BEGIN PGP SIGNED MESSAGE-----.
type ClearTextMessage struct {
	Data      []byte
	Signature []byte
}

// ---- GENERATORS -----

// NewPlainMessage generates a new binary PlainMessage ready for encryption,
// signature, or verification from the unencrypted binary data.
// This will encrypt the message with the binary flag and preserve the file as is.
func NewPlainMessage(data []byte) *PlainMessage {
	return &PlainMessage{
		Data:     clone(data),
		TextType: false,
		Filename: "",
		Time:     uint32(GetUnixTime()),
	}
}

// NewPlainMessageFromFile generates a new binary PlainMessage ready for encryption,
// signature, or verification from the unencrypted binary data.
// This will encrypt the message with the binary flag and preserve the file as is.
// It assigns a filename and a modification time.
func NewPlainMessageFromFile(data []byte, filename string, time uint32) *PlainMessage {
	return &PlainMessage{
		Data:     clone(data),
		TextType: false,
		Filename: filename,
		Time:     time,
	}
}

// NewPlainMessageFromString generates a new text PlainMessage,
// ready for encryption, signature, or verification from an unencrypted string.
// This will encrypt the message with the text flag, canonicalize the line endings
// (i.e. set all of them to \r\n) and strip the trailing spaces for each line.
// This allows seamless conversion to clear text signed messages (see RFC 4880 5.2.1 and 7.1).
func NewPlainMessageFromString(text string) *PlainMessage {
	return &PlainMessage{
		Data:     []byte(internal.Canonicalize(text)),
		TextType: true,
		Filename: "",
		Time:     uint32(GetUnixTime()),
	}
}

// NewPGPMessage generates a new PGPMessage from the unarmored binary data.
func NewPGPMessage(data []byte) *PGPMessage {
	return &PGPMessage{
		Data: clone(data),
	}
}

// NewPGPMessageFromArmored generates a new PGPMessage from an armored string ready for decryption.
func NewPGPMessageFromArmored(armored string) (*PGPMessage, error) {
	encryptedIO, err := internal.Unarmor(armored)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: error in unarmoring message")
	}

	message, err := ioutil.ReadAll(encryptedIO.Body)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: error in reading armored message")
	}

	return &PGPMessage{
		Data: message,
	}, nil
}

// NewPGPSplitMessage generates a new PGPSplitMessage from the binary unarmored keypacket,
// datapacket, and encryption algorithm.
func NewPGPSplitMessage(keyPacket []byte, dataPacket []byte) *PGPSplitMessage {
	return &PGPSplitMessage{
		KeyPacket:  clone(keyPacket),
		DataPacket: clone(dataPacket),
	}
}

// NewPGPSplitMessageFromArmored generates a new PGPSplitMessage by splitting an armored message into its
// session key packet and symmetrically encrypted data packet.
func NewPGPSplitMessageFromArmored(encrypted string) (*PGPSplitMessage, error) {
	message, err := NewPGPMessageFromArmored(encrypted)
	if err != nil {
		return nil, err
	}

	return message.SplitMessage()
}

// NewPGPSignature generates a new PGPSignature from the unarmored binary data.
func NewPGPSignature(data []byte) *PGPSignature {
	return &PGPSignature{
		Data: clone(data),
	}
}

// NewPGPSignatureFromArmored generates a new PGPSignature from the armored
// string ready for verification.
func NewPGPSignatureFromArmored(armored string) (*PGPSignature, error) {
	encryptedIO, err := internal.Unarmor(armored)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: error in unarmoring signature")
	}

	signature, err := ioutil.ReadAll(encryptedIO.Body)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: error in reading armored signature")
	}

	return &PGPSignature{
		Data: signature,
	}, nil
}

// NewClearTextMessage generates a new ClearTextMessage from data and
// signature.
func NewClearTextMessage(data []byte, signature []byte) *ClearTextMessage {
	return &ClearTextMessage{
		Data:      clone(data),
		Signature: clone(signature),
	}
}

// NewClearTextMessageFromArmored returns the message body and unarmored
// signature from a clearsigned message.
func NewClearTextMessageFromArmored(signedMessage string) (*ClearTextMessage, error) {
	modulusBlock, rest := clearsign.Decode([]byte(signedMessage))
	if len(rest) != 0 {
		return nil, errors.New("gopenpgp: extra data after modulus")
	}

	signature, err := ioutil.ReadAll(modulusBlock.ArmoredSignature.Body)
	if err != nil {
		return nil, errors.Wrap(err, "gopenpgp: error in reading cleartext message")
	}

	return NewClearTextMessage(modulusBlock.Bytes, signature), nil
}

// ---- MODEL METHODS -----

// GetBinary returns the binary content of the message as a []byte.
func (msg *PlainMessage) GetBinary() []byte {
	return msg.Data
}

// GetString returns the content of the message as a string.
func (msg *PlainMessage) GetString() string {
	return sanitizeString(strings.ReplaceAll(string(msg.Data), "\r\n", "\n"))
}

// GetBase64 returns the base-64 encoded binary content of the message as a
// string.
func (msg *PlainMessage) GetBase64() string {
	return base64.StdEncoding.EncodeToString(msg.Data)
}

// NewReader returns a New io.Reader for the binary data of the message.
func (msg *PlainMessage) NewReader() io.Reader {
	return bytes.NewReader(msg.GetBinary())
}

// IsText returns whether the message is a text message.
func (msg *PlainMessage) IsText() bool {
	return msg.TextType
}

// IsBinary returns whether the message is a binary message.
func (msg *PlainMessage) IsBinary() bool {
	return !msg.TextType
}

// getFormattedTime returns the message (latest modification) Time as time.Time.
func (msg *PlainMessage) getFormattedTime() time.Time {
	return time.Unix(int64(msg.Time), 0)
}

// GetBinary returns the unarmored binary content of the message as a []byte.
func (msg *PGPMessage) GetBinary() []byte {
	return msg.Data
}

// NewReader returns a New io.Reader for the unarmored binary data of the
// message.
func (msg *PGPMessage) NewReader() io.Reader {
	return bytes.NewReader(msg.GetBinary())
}

// GetArmored returns the armored message as a string.
func (msg *PGPMessage) GetArmored() (string, error) {
	return armor.ArmorWithType(msg.Data, constants.PGPMessageHeader)
}

// GetArmoredWithCustomHeaders returns the armored message as a string, with
// the given headers. Empty parameters are omitted from the headers.
func (msg *PGPMessage) GetArmoredWithCustomHeaders(comment, version string) (string, error) {
	return armor.ArmorWithTypeAndCustomHeaders(msg.Data, constants.PGPMessageHeader, version, comment)
}

// GetEncryptionKeyIDs Returns the key IDs of the keys to which the session key is encrypted.
func (msg *PGPMessage) GetEncryptionKeyIDs() ([]uint64, bool) {
	packets := packet.NewReader(bytes.NewReader(msg.Data))
	var err error
	var ids []uint64
	var encryptedKey *packet.EncryptedKey
Loop:
	for {
		var p packet.Packet
		if p, err = packets.Next(); goerrors.Is(err, io.EOF) {
			break
		}
		switch p := p.(type) {
		case *packet.EncryptedKey:
			encryptedKey = p
			ids = append(ids, encryptedKey.KeyId)
		case *packet.SymmetricallyEncrypted,
			*packet.AEADEncrypted,
			*packet.Compressed,
			*packet.LiteralData:
			break Loop
		}
	}
	if len(ids) > 0 {
		return ids, true
	}
	return ids, false
}

// GetHexEncryptionKeyIDs Returns the key IDs of the keys to which the session key is encrypted.
func (msg *PGPMessage) GetHexEncryptionKeyIDs() ([]string, bool) {
	return getHexKeyIDs(msg.GetEncryptionKeyIDs())
}

// GetSignatureKeyIDs Returns the key IDs of the keys to which the (readable) signature packets are encrypted to.
func (msg *PGPMessage) GetSignatureKeyIDs() ([]uint64, bool) {
	return getSignatureKeyIDs(msg.Data)
}

// GetHexSignatureKeyIDs Returns the key IDs of the keys to which the session key is encrypted.
func (msg *PGPMessage) GetHexSignatureKeyIDs() ([]string, bool) {
	return getHexKeyIDs(msg.GetSignatureKeyIDs())
}

// GetBinaryDataPacket returns the unarmored binary datapacket as a []byte.
func (msg *PGPSplitMessage) GetBinaryDataPacket() []byte {
	return msg.DataPacket
}

// GetBinaryKeyPacket returns the unarmored binary keypacket as a []byte.
func (msg *PGPSplitMessage) GetBinaryKeyPacket() []byte {
	return msg.KeyPacket
}

// GetBinary returns the unarmored binary joined packets as a []byte.
func (msg *PGPSplitMessage) GetBinary() []byte {
	return append(msg.KeyPacket, msg.DataPacket...)
}

// GetArmored returns the armored message as a string, with joined data and key
// packets.
func (msg *PGPSplitMessage) GetArmored() (string, error) {
	return armor.ArmorWithType(msg.GetBinary(), constants.PGPMessageHeader)
}

// GetPGPMessage joins asymmetric session key packet with the symmetric data
// packet to obtain a PGP message.
func (msg *PGPSplitMessage) GetPGPMessage() *PGPMessage {
	return NewPGPMessage(append(msg.KeyPacket, msg.DataPacket...))
}

// SplitMessage splits the message into key and data packet(s).
// Parameters are for backwards compatibility and are unused.
func (msg *PGPMessage) SplitMessage() (*PGPSplitMessage, error) {
	bytesReader := bytes.NewReader(msg.Data)
	packets := packet.NewReader(bytesReader)
	splitPoint := int64(0)
Loop:
	for {
		p, err := packets.Next()
		if goerrors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch p.(type) {
		case *packet.SymmetricKeyEncrypted, *packet.EncryptedKey:
			splitPoint = bytesReader.Size() - int64(bytesReader.Len())
		case *packet.SymmetricallyEncrypted, *packet.AEADEncrypted:
			break Loop
		}
	}
	return &PGPSplitMessage{
		KeyPacket:  clone(msg.Data[:splitPoint]),
		DataPacket: clone(msg.Data[splitPoint:]),
	}, nil
}

// SeparateKeyAndData splits the message into key and data packet(s).
// Parameters are for backwards compatibility and are unused.
// Deprecated: use SplitMessage().
func (msg *PGPMessage) SeparateKeyAndData(_ int, _ int) (*PGPSplitMessage, error) {
	return msg.SplitMessage()
}

// GetBinary returns the unarmored binary content of the signature as a []byte.
func (sig *PGPSignature) GetBinary() []byte {
	return sig.Data
}

// GetArmored returns the armored signature as a string.
func (sig *PGPSignature) GetArmored() (string, error) {
	return armor.ArmorWithType(sig.Data, constants.PGPSignatureHeader)
}

// GetSignatureKeyIDs Returns the key IDs of the keys to which the (readable) signature packets are encrypted to.
func (sig *PGPSignature) GetSignatureKeyIDs() ([]uint64, bool) {
	return getSignatureKeyIDs(sig.Data)
}

// GetHexSignatureKeyIDs Returns the key IDs of the keys to which the session key is encrypted.
func (sig *PGPSignature) GetHexSignatureKeyIDs() ([]string, bool) {
	return getHexKeyIDs(sig.GetSignatureKeyIDs())
}

// GetBinary returns the unarmored signed data as a []byte.
func (msg *ClearTextMessage) GetBinary() []byte {
	return msg.Data
}

// GetString returns the unarmored signed data as a string.
func (msg *ClearTextMessage) GetString() string {
	return string(msg.Data)
}

// GetBinarySignature returns the unarmored binary signature as a []byte.
func (msg *ClearTextMessage) GetBinarySignature() []byte {
	return msg.Signature
}

// GetArmored armors plaintext and signature with the PGP SIGNED MESSAGE
// armoring.
func (msg *ClearTextMessage) GetArmored() (string, error) {
	armSignature, err := armor.ArmorWithType(msg.GetBinarySignature(), constants.PGPSignatureHeader)
	if err != nil {
		return "", errors.Wrap(err, "gopenpgp: error in armoring cleartext message")
	}

	str := "-----BEGIN PGP SIGNED MESSAGE-----\r\nHash: SHA512\r\n\r\n"
	str += msg.GetString()
	str += "\r\n"
	str += armSignature

	return str, nil
}

// ---- UTILS -----

// IsPGPMessage checks if data if has armored PGP message format.
func IsPGPMessage(data string) bool {
	re := regexp.MustCompile("^-----BEGIN " + constants.PGPMessageHeader + "-----(?s:.+)-----END " +
		constants.PGPMessageHeader + "-----")
	return re.MatchString(data)
}

func getSignatureKeyIDs(data []byte) ([]uint64, bool) {
	packets := packet.NewReader(bytes.NewReader(data))
	var err error
	var ids []uint64
	var onePassSignaturePacket *packet.OnePassSignature
	var signaturePacket *packet.Signature

Loop:
	for {
		var p packet.Packet
		if p, err = packets.Next(); goerrors.Is(err, io.EOF) {
			break
		}
		switch p := p.(type) {
		case *packet.OnePassSignature:
			onePassSignaturePacket = p
			ids = append(ids, onePassSignaturePacket.KeyId)
		case *packet.Signature:
			signaturePacket = p
			if signaturePacket.IssuerKeyId != nil {
				ids = append(ids, *signaturePacket.IssuerKeyId)
			}
		case *packet.SymmetricallyEncrypted,
			*packet.AEADEncrypted,
			*packet.Compressed,
			*packet.LiteralData:
			break Loop
		}
	}
	if len(ids) > 0 {
		return ids, true
	}
	return ids, false
}

func getHexKeyIDs(keyIDs []uint64, ok bool) ([]string, bool) {
	hexIDs := make([]string, len(keyIDs))

	for i, id := range keyIDs {
		hexIDs[i] = keyIDToHex(id)
	}

	return hexIDs, ok
}
