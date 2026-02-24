package proton

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/go-resty/resty/v2"
)

func (c *Client) GetAttachment(ctx context.Context, attachmentID string) ([]byte, error) {
	var buffer bytes.Buffer
	if err := c.getAttachment(ctx, attachmentID, &buffer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func (c *Client) GetAttachmentInto(ctx context.Context, attachmentID string, reader io.ReaderFrom) error {
	return c.getAttachment(ctx, attachmentID, reader)
}

func (c *Client) UploadAttachment(ctx context.Context, addrKR *crypto.KeyRing, req CreateAttachmentReq) (Attachment, error) {
	var res struct {
		Attachment Attachment
	}

	kr, err := addrKR.FirstKey()
	if err != nil {
		return res.Attachment, fmt.Errorf("failed to get first key: %w", err)
	}

	sig, err := kr.SignDetached(crypto.NewPlainMessage(req.Body))
	if err != nil {
		return Attachment{}, fmt.Errorf("failed to sign attachment: %w", err)
	}

	enc, err := kr.EncryptAttachment(crypto.NewPlainMessage(req.Body), req.Filename)
	if err != nil {
		return Attachment{}, fmt.Errorf("failed to encrypt attachment: %w", err)
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).
			SetMultipartFormData(map[string]string{
				"MessageID":   req.MessageID,
				"Filename":    req.Filename,
				"MIMEType":    string(req.MIMEType),
				"Disposition": string(req.Disposition),
				"ContentID":   req.ContentID,
			}).
			SetMultipartFields(
				&resty.MultipartField{
					Param:       "KeyPackets",
					FileName:    "blob",
					ContentType: "application/octet-stream",
					Reader:      bytes.NewReader(enc.KeyPacket),
				},
				&resty.MultipartField{
					Param:       "DataPacket",
					FileName:    "blob",
					ContentType: "application/octet-stream",
					Reader:      bytes.NewReader(enc.DataPacket),
				},
				&resty.MultipartField{
					Param:       "Signature",
					FileName:    "blob",
					ContentType: "application/octet-stream",
					Reader:      bytes.NewReader(sig.GetBinary()),
				},
			).
			Post("/mail/v4/attachments")
	}); err != nil {
		return Attachment{}, err
	}

	return res.Attachment, nil
}

func (c *Client) getAttachment(ctx context.Context, attachmentID string, reader io.ReaderFrom) error {
	res, err := c.doRes(ctx, func(req *resty.Request) (*resty.Response, error) {
		res, err := req.SetDoNotParseResponse(true).Get("/mail/v4/attachments/" + attachmentID)
		return parseResponse(res, err)
	})
	if err != nil {
		return fmt.Errorf("failed to request attachment: %w", err)
	}
	defer res.RawBody().Close()

	if _, err = reader.ReadFrom(res.RawBody()); err != nil {
		return err
	}

	return nil
}
