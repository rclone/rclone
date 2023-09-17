package proton_api_bridge

import (
	"context"
	"encoding/base64"
	"io/ioutil"
	"log"
	"net/mail"
	"path/filepath"

	"github.com/ProtonMail/gluon/rfc822"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/henrybear327/go-proton-api"
)

type MailSendingParameters struct {
	TemplateFile          string
	EmailSubject          string
	RecipientEmailAddress string
	EmailAttachments      []string
	EmailContentIDs       []string
}

func (protonDrive *ProtonDrive) SendEmail(ctx context.Context, i int, errChan chan error, config *MailSendingParameters) {
	log.Println("SendEmail in", i)
	defer log.Println("SendEmail out", i)

	createDraftResp, err := protonDrive.createDraft(ctx, config)
	if err != nil {
		errChan <- err
	}

	attachments, err := protonDrive.uploadAttachments(ctx, createDraftResp, config)
	if err != nil {
		errChan <- err
	}

	err = protonDrive.sendDraft(ctx, createDraftResp.ID, attachments, config)
	if err != nil {
		errChan <- err
	}

	errChan <- nil
}

func (protonDrive *ProtonDrive) getHTMLBody(config *MailSendingParameters) ([]byte, error) {
	htmlTemplate, err := ioutil.ReadFile(config.TemplateFile)
	if err != nil {
		return nil, err
	}

	return htmlTemplate, nil
}

func (protonDrive *ProtonDrive) createDraft(ctx context.Context, config *MailSendingParameters) (*proton.Message, error) {
	htmlTemplate, err := protonDrive.getHTMLBody(config)
	if err != nil {
		return nil, err
	}

	createDraftReq := proton.CreateDraftReq{
		Message: proton.DraftTemplate{
			Subject: config.EmailSubject,
			Sender: &mail.Address{
				Address: protonDrive.signatureAddress,
			},
			ToList: []*mail.Address{
				{
					Address: config.RecipientEmailAddress,
				},
			},
			CCList:   []*mail.Address{},
			BCCList:  []*mail.Address{},
			Body:     string(htmlTemplate), // NOTE: the body here is for yourself to view it! No sender encryption is done yet
			MIMEType: rfc822.TextHTML,
			// Unread:   false,

			// ExternalID: "", // FIXME: what's this
		},
	}

	createDraftResp, err := protonDrive.c.CreateDraft(ctx, protonDrive.DefaultAddrKR, createDraftReq)
	if err != nil {
		return nil, err
	}

	return &createDraftResp, nil
}

func (protonDrive *ProtonDrive) getAttachmentSessionKeyMap(attachments []*proton.Attachment) (map[string]*crypto.SessionKey, error) {
	ret := make(map[string]*crypto.SessionKey)

	for i := range attachments {
		keyPacket, err := base64.StdEncoding.DecodeString(attachments[i].KeyPackets)
		if err != nil {
			return nil, err
		}

		key, err := protonDrive.DefaultAddrKR.DecryptSessionKey(keyPacket)
		if err != nil {
			return nil, err
		}

		ret[attachments[i].ID] = key
	}

	return ret, nil
}

func (protonDrive *ProtonDrive) uploadAttachments(ctx context.Context, createDraftResp *proton.Message, config *MailSendingParameters) ([]*proton.Attachment, error) {
	attachments := make([]*proton.Attachment, 0)
	for i := range config.EmailAttachments {
		// read out attachment file
		fileByteArray, err := ioutil.ReadFile(config.EmailAttachments[i])
		if err != nil {
			return nil, err
		}

		req := proton.CreateAttachmentReq{
			MessageID: createDraftResp.ID,

			Filename:    filepath.Base(config.EmailAttachments[i]),
			MIMEType:    rfc822.MultipartMixed, // FIXME: what is this?
			Disposition: proton.InlineDisposition,
			ContentID:   config.EmailContentIDs[i],

			Body: fileByteArray,
		}

		uploadAttachmentResp, err := protonDrive.c.UploadAttachment(ctx, protonDrive.DefaultAddrKR, req)
		if err != nil {
			return nil, err
		}

		// log.Printf("uploadAttachmentResp %#v", uploadAttachmentResp)

		attachments = append(attachments, &uploadAttachmentResp)
	}

	return attachments, nil
}

func (protonDrive *ProtonDrive) sendDraft(ctx context.Context, messageID string, attachents []*proton.Attachment, config *MailSendingParameters) error {
	// FIXME: repect all sendPrefs
	// FIXME: respect PGPMIMEScheme, etc.

	recipientPublicKeys, recipientType, err := protonDrive.c.GetPublicKeys(ctx, config.RecipientEmailAddress)
	if err != nil {
		return err
	}
	if recipientType != proton.RecipientTypeInternal {
		log.Fatalln("Currently only support internal email sending")
	}
	recipientKR, err := recipientPublicKeys.GetKeyRing()
	if err != nil {
		return err
	}

	htmlTemplate, err := protonDrive.getHTMLBody(config)
	if err != nil {
		return err
	}

	atts, err := protonDrive.getAttachmentSessionKeyMap(attachents)
	if err != nil {
		return err
	}

	// send email
	sendReq := proton.SendDraftReq{
		// Packages: []*proton.MessagePackage{},
	}

	// for each of the recipient, we encrypt body for them
	if err = sendReq.AddTextPackage(protonDrive.DefaultAddrKR,
		string(htmlTemplate),
		rfc822.TextHTML,
		map[string]proton.SendPreferences{config.RecipientEmailAddress: {
			Encrypt:          true,
			PubKey:           recipientKR,
			SignatureType:    proton.DetachedSignature,
			EncryptionScheme: proton.InternalScheme,
			MIMEType:         rfc822.TextHTML, // FIXME
		}},
		atts,
	); err != nil {
		return err
	}

	/* msg */
	_, err = protonDrive.c.SendDraft(ctx, messageID, sendReq)
	if err != nil {
		return err
	}
	// log.Println(msg)

	return nil
}
