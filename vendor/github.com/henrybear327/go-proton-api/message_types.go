package proton

import (
	"bytes"
	"io"
	"net/mail"

	"github.com/ProtonMail/gluon/rfc822"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"golang.org/x/exp/slices"
)

type MessageMetadata struct {
	ID         string
	AddressID  string
	LabelIDs   []string
	ExternalID string

	Subject  string
	Sender   *mail.Address
	ToList   []*mail.Address
	CCList   []*mail.Address
	BCCList  []*mail.Address
	ReplyTos []*mail.Address

	Flags        MessageFlag
	Time         int64
	Size         int
	Unread       Bool
	IsReplied    Bool
	IsRepliedAll Bool
	IsForwarded  Bool

	NumAttachments int
}

func (meta MessageMetadata) Seen() bool {
	return !bool(meta.Unread)
}

func (meta MessageMetadata) Starred() bool {
	return slices.Contains(meta.LabelIDs, StarredLabel)
}

func (meta MessageMetadata) IsDraft() bool {
	return meta.Flags&(MessageFlagReceived|MessageFlagSent) == 0
}

type MessageFilter struct {
	ID []string `json:",omitempty"`

	Subject    string `json:",omitempty"`
	AddressID  string `json:",omitempty"`
	ExternalID string `json:",omitempty"`
	LabelID    string `json:",omitempty"`
	EndID      string `json:",omitempty"`
	Desc       Bool
}

type Message struct {
	MessageMetadata

	Header        string
	ParsedHeaders Headers
	Body          string
	MIMEType      rfc822.MIMEType
	Attachments   []Attachment
}

type MessageFlag int64

const (
	MessageFlagReceived    MessageFlag = 1 << 0
	MessageFlagSent        MessageFlag = 1 << 1
	MessageFlagInternal    MessageFlag = 1 << 2
	MessageFlagE2E         MessageFlag = 1 << 3
	MessageFlagAuto        MessageFlag = 1 << 4
	MessageFlagReplied     MessageFlag = 1 << 5
	MessageFlagRepliedAll  MessageFlag = 1 << 6
	MessageFlagForwarded   MessageFlag = 1 << 7
	MessageFlagAutoReplied MessageFlag = 1 << 8
	MessageFlagImported    MessageFlag = 1 << 9
	MessageFlagOpened      MessageFlag = 1 << 10
	MessageFlagReceiptSent MessageFlag = 1 << 11
	MessageFlagNotified    MessageFlag = 1 << 12
	MessageFlagTouched     MessageFlag = 1 << 13
	MessageFlagReceipt     MessageFlag = 1 << 14

	MessageFlagReceiptRequest MessageFlag = 1 << 16
	MessageFlagPublicKey      MessageFlag = 1 << 17
	MessageFlagSign           MessageFlag = 1 << 18
	MessageFlagUnsubscribed   MessageFlag = 1 << 19
	MessageFlagScheduledSend  MessageFlag = 1 << 20
	MessageFlagAlias          MessageFlag = 1 << 21

	MessageFlagDMARCPass      MessageFlag = 1 << 23
	MessageFlagSPFFail        MessageFlag = 1 << 24
	MessageFlagDKIMFail       MessageFlag = 1 << 25
	MessageFlagDMARCFail      MessageFlag = 1 << 26
	MessageFlagHamManual      MessageFlag = 1 << 27
	MessageFlagSpamAuto       MessageFlag = 1 << 28
	MessageFlagSpamManual     MessageFlag = 1 << 29
	MessageFlagPhishingAuto   MessageFlag = 1 << 30
	MessageFlagPhishingManual MessageFlag = 1 << 31
)

func (f MessageFlag) Has(flag MessageFlag) bool {
	return f&flag != 0
}

func (f MessageFlag) Matches(flag MessageFlag) bool {
	return f&flag == flag
}

func (f MessageFlag) HasAny(flags ...MessageFlag) bool {
	for _, flag := range flags {
		if f.Has(flag) {
			return true
		}
	}

	return false
}

func (f MessageFlag) HasAll(flags ...MessageFlag) bool {
	for _, flag := range flags {
		if !f.Has(flag) {
			return false
		}
	}

	return true
}

func (f MessageFlag) Add(flag MessageFlag) MessageFlag {
	return f | flag
}

func (f MessageFlag) Remove(flag MessageFlag) MessageFlag {
	return f &^ flag
}

func (f MessageFlag) Toggle(flag MessageFlag) MessageFlag {
	if f.Has(flag) {
		return f.Remove(flag)
	}

	return f.Add(flag)
}

func (m Message) Decrypt(kr *crypto.KeyRing) ([]byte, error) {
	enc, err := crypto.NewPGPMessageFromArmored(m.Body)
	if err != nil {
		return nil, err
	}

	dec, err := kr.Decrypt(enc, nil, crypto.GetUnixTime())
	if err != nil {
		return nil, err
	}

	return dec.GetBinary(), nil
}

func (m Message) DecryptInto(kr *crypto.KeyRing, buffer io.ReaderFrom) error {
	armored, err := armor.Decode(bytes.NewReader([]byte(m.Body)))
	if err != nil {
		return err
	}

	stream, err := kr.DecryptStream(armored.Body, nil, crypto.GetUnixTime())
	if err != nil {
		return err
	}

	if _, err := buffer.ReadFrom(stream); err != nil {
		return err
	}

	return nil
}

type FullMessage struct {
	Message

	AttData [][]byte
}

type Signature struct {
	Hash string
	Data *crypto.PGPSignature
}

type MessageActionReq struct {
	IDs []string
}

type LabelMessagesReq struct {
	LabelID string
	IDs     []string
}

type LabelMessagesRes struct {
	Responses []LabelMessageRes
	UndoToken UndoToken
}

func (res LabelMessagesRes) ok() (bool, string) {
	for _, resp := range res.Responses {
		if resp.Response.Code != SuccessCode {
			return false, resp.Response.Error()
		}
	}

	return true, ""
}

type LabelMessageRes struct {
	ID       string
	Response APIError
}

type MessageGroupCount struct {
	LabelID string
	Total   int
	Unread  int
}
