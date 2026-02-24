package proton

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ProtonMail/gluon/rfc822"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/emersion/go-message"
	"github.com/emersion/go-message/textproto"
	"github.com/google/uuid"
)

func BuildRFC822(kr *crypto.KeyRing, msg Message, attData map[string][]byte) ([]byte, error) {
	if msg.MIMEType == rfc822.MultipartMixed {
		return buildPGPRFC822(kr, msg)
	}

	header, err := getMixedMessageHeader(msg)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)

	w, err := message.CreateWriter(buf, header)
	if err != nil {
		return nil, err
	}

	var (
		inlineAtts []Attachment
		inlineData [][]byte
		attachAtts []Attachment
		attachData [][]byte
	)

	for _, att := range msg.Attachments {
		if att.Disposition == InlineDisposition {
			inlineAtts = append(inlineAtts, att)
			inlineData = append(inlineData, attData[att.ID])
		} else {
			attachAtts = append(attachAtts, att)
			attachData = append(attachData, attData[att.ID])
		}
	}

	if len(inlineAtts) > 0 {
		if err := writeRelatedParts(w, kr, msg, inlineAtts, inlineData); err != nil {
			return nil, err
		}
	} else if err := writeTextPart(w, kr, msg); err != nil {
		return nil, err
	}

	for i, att := range attachAtts {
		if err := writeAttachmentPart(w, kr, att, attachData[i]); err != nil {
			return nil, err
		}
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func writeTextPart(w *message.Writer, kr *crypto.KeyRing, msg Message) error {
	dec, err := msg.Decrypt(kr)
	if err != nil {
		return err
	}

	part, err := w.CreatePart(getTextPartHeader(dec, msg.MIMEType))
	if err != nil {
		return err
	}

	if _, err := part.Write(dec); err != nil {
		return err
	}

	return part.Close()
}

func writeAttachmentPart(w *message.Writer, kr *crypto.KeyRing, att Attachment, attData []byte) error {
	kps, err := base64.StdEncoding.DecodeString(att.KeyPackets)
	if err != nil {
		return err
	}

	msg := crypto.NewPGPSplitMessage(kps, attData).GetPGPMessage()

	dec, err := kr.Decrypt(msg, nil, crypto.GetUnixTime())
	if err != nil {
		return err
	}

	part, err := w.CreatePart(getAttachmentPartHeader(att))
	if err != nil {
		return err
	}

	if _, err := part.Write(dec.GetBinary()); err != nil {
		return err
	}

	return part.Close()
}

func writeRelatedParts(w *message.Writer, kr *crypto.KeyRing, msg Message, atts []Attachment, attData [][]byte) error {
	var header message.Header

	header.SetContentType(string(rfc822.MultipartRelated), nil)

	rel, err := w.CreatePart(header)
	if err != nil {
		return err
	}

	if err := writeTextPart(rel, kr, msg); err != nil {
		return err
	}

	for i, att := range atts {
		if err := writeAttachmentPart(rel, kr, att, attData[i]); err != nil {
			return err
		}
	}

	return rel.Close()
}

func buildPGPRFC822(kr *crypto.KeyRing, msg Message) ([]byte, error) {
	raw, err := textproto.ReadHeader(bufio.NewReader(strings.NewReader(msg.Header)))
	if err != nil {
		return nil, err
	}

	dec, err := msg.Decrypt(kr)
	if err != nil {
		return nil, err
	}

	sigs, err := ExtractSignatures(kr, msg.Body)
	if err != nil {
		return nil, err
	}

	if len(sigs) > 0 {
		return buildMultipartSignedRFC822(message.Header{Header: raw}, dec, sigs[0])
	}

	return buildMultipartEncryptedRFC822(message.Header{Header: raw}, dec)
}

func buildMultipartSignedRFC822(header message.Header, body []byte, sig Signature) ([]byte, error) {
	buf := new(bytes.Buffer)

	boundary := uuid.New().String()

	header.SetContentType("multipart/signed", map[string]string{
		"micalg":   sig.Hash,
		"protocol": "application/pgp-signature",
		"boundary": boundary,
	})

	if err := textproto.WriteHeader(buf, header.Header); err != nil {
		return nil, err
	}

	w := rfc822.NewMultipartWriter(buf, boundary)

	bodyHeader, bodyData := rfc822.Split(body)

	if err := w.AddPart(func(w io.Writer) error {
		if _, err := w.Write(bodyHeader); err != nil {
			return err
		}

		if _, err := w.Write(bodyData); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	var sigHeader message.Header

	sigHeader.SetContentType("application/pgp-signature", map[string]string{"name": "OpenPGP_signature.asc"})
	sigHeader.SetContentDisposition("attachment", map[string]string{"filename": "OpenPGP_signature"})
	sigHeader.Set("Content-Description", "OpenPGP digital signature")

	sigData, err := sig.Data.GetArmored()
	if err != nil {
		return nil, err
	}

	if err := w.AddPart(func(w io.Writer) error {
		if err := textproto.WriteHeader(w, sigHeader.Header); err != nil {
			return err
		}

		if _, err := w.Write([]byte(sigData)); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	if err := w.Done(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func buildMultipartEncryptedRFC822(header message.Header, body []byte) ([]byte, error) {
	buf := new(bytes.Buffer)

	bodyHeader, bodyData := rfc822.Split(body)

	parsedHeader, err := rfc822.NewHeader(bodyHeader)
	if err != nil {
		return nil, err
	}

	parsedHeader.Entries(func(key, val string) {
		header.Set(key, val)
	})

	if err := textproto.WriteHeader(buf, header.Header); err != nil {
		return nil, err
	}

	if _, err := buf.Write(bodyData); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func getMixedMessageHeader(msg Message) (message.Header, error) {
	raw, err := textproto.ReadHeader(bufio.NewReader(strings.NewReader(msg.Header)))
	if err != nil {
		return message.Header{}, err
	}

	header := message.Header{Header: raw}

	header.SetContentType(string(rfc822.MultipartMixed), nil)

	if date, err := mail.ParseDate(header.Get("Date")); err != nil || date.Before(time.Unix(0, 0)) {
		if msgTime := time.Unix(msg.Time, 0); msgTime.After(time.Unix(0, 0)) {
			header.Set("Date", msgTime.In(time.UTC).Format(time.RFC1123Z))
		} else {
			header.Del("Date")
		}

		header.Set("X-Original-Date", date.In(time.UTC).Format(time.RFC1123Z))
	}

	return header, nil
}

func getTextPartHeader(body []byte, mimeType rfc822.MIMEType) message.Header {
	var header message.Header

	params := make(map[string]string)

	if utf8.Valid(body) {
		params["charset"] = "utf-8"
	}

	header.SetContentType(string(mimeType), params)

	// Use quoted-printable for all text/... parts
	header.Set("Content-Transfer-Encoding", "quoted-printable")

	return header
}

func getAttachmentPartHeader(att Attachment) message.Header {
	var header message.Header

	for key, val := range att.Headers {
		for _, val := range val {
			header.Add(key, val)
		}
	}

	// All attachments have a content type.
	header.SetContentType(string(att.MIMEType), map[string]string{"name": mime.QEncoding.Encode("utf-8", att.Name)})

	// All attachments have a content disposition.
	header.SetContentDisposition(string(att.Disposition), map[string]string{"filename": mime.QEncoding.Encode("utf-8", att.Name)})

	// Use base64 for all attachments except embedded RFC822 messages.
	if att.MIMEType != rfc822.MessageRFC822 {
		header.Set("Content-Transfer-Encoding", "base64")
	} else {
		header.Del("Content-Transfer-Encoding")
	}

	return header
}
