package backend

import (
	"encoding/base64"

	"github.com/ProtonMail/gluon/rfc822"
	"github.com/google/uuid"
	"github.com/rclone/go-proton-api"
)

func (b *Backend) createAttData(dataPacket []byte) string {
	attDataID := uuid.NewString()

	b.attDataLock.Lock()
	defer b.attDataLock.Unlock()

	b.attData[attDataID] = dataPacket

	return attDataID
}

type attachment struct {
	attachID  string
	attDataID string

	filename    string
	mimeType    rfc822.MIMEType
	disposition proton.Disposition

	keyPackets []byte
	armSig     string
}

func newAttachment(
	filename string,
	mimeType rfc822.MIMEType,
	disposition proton.Disposition,
	keyPackets []byte,
	dataPacketID string,
	armSig string,
) *attachment {
	return &attachment{
		attachID:  uuid.NewString(),
		attDataID: dataPacketID,

		filename:    filename,
		mimeType:    mimeType,
		disposition: disposition,

		keyPackets: keyPackets,
		armSig:     armSig,
	}
}

func (att *attachment) toAttachment() proton.Attachment {
	return proton.Attachment{
		ID: att.attachID,

		Name:        att.filename,
		MIMEType:    att.mimeType,
		Disposition: att.disposition,

		KeyPackets: base64.StdEncoding.EncodeToString(att.keyPackets),
		Signature:  att.armSig,
	}
}
