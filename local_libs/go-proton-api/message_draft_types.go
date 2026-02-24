package proton

import (
	"net/mail"

	"github.com/ProtonMail/gluon/rfc822"
)

type DraftTemplate struct {
	Subject  string
	Sender   *mail.Address
	ToList   []*mail.Address
	CCList   []*mail.Address
	BCCList  []*mail.Address
	Body     string
	MIMEType rfc822.MIMEType
	Unread   Bool

	ExternalID string `json:",omitempty"`
}

type CreateDraftAction int

const (
	ReplyAction CreateDraftAction = iota
	ReplyAllAction
	ForwardAction
	AutoResponseAction
	ReadReceiptAction
)

type CreateDraftReq struct {
	Message              DraftTemplate
	AttachmentKeyPackets []string

	ParentID string `json:",omitempty"`
	Action   CreateDraftAction
}

type UpdateDraftReq struct {
	Message              DraftTemplate
	AttachmentKeyPackets []string
}
