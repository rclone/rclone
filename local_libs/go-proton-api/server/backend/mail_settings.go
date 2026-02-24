package backend

import (
	"github.com/ProtonMail/gluon/rfc822"
	"github.com/rclone/go-proton-api"
)

type mailSettings struct {
	displayName   string
	draftMIMEType rfc822.MIMEType
	attachPubKey  bool
}

func newMailSettings(displayName string) *mailSettings {
	return &mailSettings{
		displayName:  displayName,
		attachPubKey: false,
	}
}

func (settings *mailSettings) toMailSettings() proton.MailSettings {
	return proton.MailSettings{
		DisplayName:     settings.displayName,
		DraftMIMEType:   settings.draftMIMEType,
		AttachPublicKey: proton.Bool(settings.attachPubKey),
	}
}
