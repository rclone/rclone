package proton

import "github.com/ProtonMail/gluon/rfc822"

type MailSettings struct {
	DisplayName     string
	Signature       string
	DraftMIMEType   rfc822.MIMEType
	AttachPublicKey Bool
	Sign            SignExternalMessages
	PGPScheme       EncryptionScheme
}

type SignExternalMessages int

const (
	SignExternalMessagesDisabled SignExternalMessages = iota
	SignExternalMessagesEnabled
)

type SetDisplayNameReq struct {
	DisplayName string
}

type SetSignatureReq struct {
	Signature string
}

type SetDraftMIMETypeReq struct {
	MIMEType rfc822.MIMEType
}

type SetAttachPublicKeyReq struct {
	AttachPublicKey Bool
}

type SetSignExternalMessagesReq struct {
	Sign SignExternalMessages
}

type SetDefaultPGPSchemeReq struct {
	PGPScheme EncryptionScheme
}
