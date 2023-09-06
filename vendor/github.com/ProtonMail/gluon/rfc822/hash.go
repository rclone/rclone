package rfc822

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"strings"

	"github.com/sirupsen/logrus"
)

// GetMessageHash returns the hash of the given message.
// This takes into account:
// - the Subject header,
// - the From/To/Cc headers,
// - the Content-Type header of each (leaf) part,
// - the Content-Disposition header of each (leaf) part,
// - the (decoded) body of each part.
func GetMessageHash(b []byte) (string, error) {
	section := Parse(b)

	header, err := section.ParseHeader()
	if err != nil {
		return "", err
	}

	h := sha256.New()

	if _, err := h.Write([]byte(header.Get("Subject"))); err != nil {
		return "", err
	}

	if _, err := h.Write([]byte(header.Get("From"))); err != nil {
		return "", err
	}

	if _, err := h.Write([]byte(header.Get("To"))); err != nil {
		return "", err
	}

	if _, err := h.Write([]byte(header.Get("Cc"))); err != nil {
		return "", err
	}

	if _, err := h.Write([]byte(header.Get("Reply-To"))); err != nil {
		return "", err
	}

	if _, err := h.Write([]byte(header.Get("In-Reply-To"))); err != nil {
		return "", err
	}

	if err := section.Walk(func(section *Section) error {
		children, err := section.Children()
		if err != nil {
			return err
		} else if len(children) > 0 {
			return nil
		}

		header, err := section.ParseHeader()
		if err != nil {
			return err
		}

		contentType := header.Get("Content-Type")
		mimeType, values, err := ParseMIMEType(contentType)
		if err != nil {
			logrus.Warnf("Message contains invalid mime type: %v", contentType)
		} else {
			if _, err := h.Write([]byte(mimeType)); err != nil {
				return err
			}

			for k, v := range values {
				if strings.EqualFold(k, "boundary") {
					continue
				}

				if _, err := h.Write([]byte(k)); err != nil {
					return err
				}

				if _, err := h.Write([]byte(v)); err != nil {
					return err
				}
			}
		}

		if _, err := h.Write([]byte(header.Get("Content-Disposition"))); err != nil {
			return err
		}

		body := section.Body()
		body = bytes.ReplaceAll(body, []byte{'\r'}, nil)
		body = bytes.TrimSpace(body)
		if _, err := h.Write(body); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}
