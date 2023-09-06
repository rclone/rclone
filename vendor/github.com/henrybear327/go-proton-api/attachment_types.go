package proton

import (
	"github.com/ProtonMail/gluon/rfc822"
)

type Attachment struct {
	ID string

	Name        string
	Size        int64
	MIMEType    rfc822.MIMEType
	Disposition Disposition
	Headers     Headers

	KeyPackets string
	Signature  string
}

type Disposition string

const (
	InlineDisposition     Disposition = "inline"
	AttachmentDisposition Disposition = "attachment"
)

type CreateAttachmentReq struct {
	MessageID string

	Filename    string
	MIMEType    rfc822.MIMEType
	Disposition Disposition
	ContentID   string

	Body []byte
}
