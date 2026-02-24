package proton

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"strings"

	"github.com/ProtonMail/gluon/rfc822"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/google/uuid"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/encoding/ianaindex"
)

// CharsetReader returns a charset decoder for the given charset.
// If set, it will be used to decode non-utf8 encoded messages.
var CharsetReader func(charset string, input io.Reader) (io.Reader, error)

// EncryptRFC822 encrypts the given message literal as a PGP attachment.
func EncryptRFC822(kr *crypto.KeyRing, literal []byte) ([]byte, error) {
	var buf bytes.Buffer

	if err := tryEncrypt(&buf, kr, rfc822.Parse(literal)); err != nil {
		return encryptFull(kr, literal)
	}

	return buf.Bytes(), nil
}

// tryEncrypt tries to encrypt the given message section.
// It first checks if the message is encrypted/signed or has multiple text parts.
// If so, it returns an error -- we need to encrypt the whole message as a PGP attachment.
func tryEncrypt(w io.Writer, kr *crypto.KeyRing, s *rfc822.Section) error {
	var textCount int

	if err := s.Walk(func(s *rfc822.Section) error {
		// Ensure we can read the content type.
		contentType, _, err := s.ContentType()
		if err != nil {
			return fmt.Errorf("cannot read content type: %w", err)
		}

		// Ensure we can read the content disposition.
		if header, err := s.ParseHeader(); err != nil {
			return fmt.Errorf("cannot read header: %w", err)
		} else if header.Has("Content-Disposition") {
			if _, _, err := rfc822.ParseMediaType(header.Get("Content-Disposition")); err != nil {
				return fmt.Errorf("cannot read content disposition: %w", err)
			}
		}

		// Check if the message is already encrypted or signed.
		if contentType.SubType() == "encrypted" {
			return fmt.Errorf("already encrypted")
		} else if contentType.SubType() == "signed" {
			return fmt.Errorf("already signed")
		}

		if contentType.Type() != "text" {
			return nil
		}

		if textCount++; textCount > 1 {
			return fmt.Errorf("multiple text parts")
		}

		return nil
	}); err != nil {
		return err
	}

	return encrypt(w, kr, s)
}

// encrypt encrypts the given message section with the given keyring and writes the result to w.
func encrypt(w io.Writer, kr *crypto.KeyRing, s *rfc822.Section) error {
	contentType, contentParams, err := s.ContentType()
	if err != nil {
		return err
	}

	if contentType.IsMultiPart() {
		return encryptMultipart(w, kr, s, contentParams["boundary"])
	}

	if contentType.Type() == "text" || contentType.Type() == "message" {
		return encryptText(w, kr, s)
	}

	return encryptAtt(w, kr, s)
}

// encryptMultipart encrypts the given multipart message section with the given keyring and writes the result to w.
func encryptMultipart(w io.Writer, kr *crypto.KeyRing, s *rfc822.Section, boundary string) error {
	// Write the header.
	if _, err := w.Write(s.Header()); err != nil {
		return err
	}

	// Create a new multipart writer with the boundary from the header.
	ww := rfc822.NewMultipartWriter(w, boundary)

	children, err := s.Children()
	if err != nil {
		return err
	}

	// Encrypt each child part.
	for _, child := range children {
		if err := ww.AddPart(func(w io.Writer) error {
			return encrypt(w, kr, child)
		}); err != nil {
			return err
		}
	}

	return ww.Done()
}

// encryptText encrypts the given text message section with the given keyring and writes the result to w.
func encryptText(w io.Writer, kr *crypto.KeyRing, s *rfc822.Section) error {
	contentType, contentParams, err := s.ContentType()
	if err != nil {
		return err
	}

	header, err := s.ParseHeader()
	if err != nil {
		return err
	}

	body, err := s.DecodedBody()
	if err != nil {
		return err
	}

	// Remove the Content-Transfer-Encoding header  as we decode the body.
	header.Del("Content-Transfer-Encoding")

	// If the text part has a charset, decode it to UTF-8.
	if charset, ok := contentParams["charset"]; ok {
		decoder, err := getCharsetDecoder(bytes.NewReader(body), charset)
		if err != nil {
			return err
		}

		if body, err = io.ReadAll(decoder); err != nil {
			return err
		}

		// Remove old content type.
		header.Del("Content-Type")

		header.Set("Content-Type", mime.FormatMediaType(
			string(contentType),
			replace(contentParams, "charset", "utf-8")),
		)
	}

	// Encrypt the body.
	enc, err := kr.Encrypt(crypto.NewPlainMessage(body), nil)
	if err != nil {
		return err
	}

	// Armor the encrypted body.
	arm, err := enc.GetArmored()
	if err != nil {
		return err
	}

	// Write the header.
	if _, err := w.Write(header.Raw()); err != nil {
		return err
	}

	// Write the armored body.
	if _, err := w.Write([]byte(arm)); err != nil {
		return err
	}

	return nil
}

// encryptAtt encrypts the given attachment section with the given keyring and writes the result to w.
func encryptAtt(w io.Writer, kr *crypto.KeyRing, s *rfc822.Section) error {
	header, err := s.ParseHeader()
	if err != nil {
		return err
	}

	body, err := s.DecodedBody()
	if err != nil {
		return err
	}

	// Set the Content-Transfer-Encoding header to base64.
	header.Set("Content-Transfer-Encoding", "base64")

	// Encrypt the body.
	enc, err := kr.Encrypt(crypto.NewPlainMessage(body), nil)
	if err != nil {
		return err
	}

	// Write the header.
	if _, err := w.Write(header.Raw()); err != nil {
		return err
	}

	// Write the base64 body.
	if err := encodeBase64(w, enc.GetBinary()); err != nil {
		return err
	}

	return nil
}

// encryptFull builds a PGP/MIME encrypted message from the given literal.
func encryptFull(kr *crypto.KeyRing, literal []byte) ([]byte, error) {
	enc, err := kr.Encrypt(crypto.NewPlainMessage(literal), kr)
	if err != nil {
		return nil, err
	}

	arm, err := enc.GetArmored()
	if err != nil {
		return nil, err
	}

	header, err := rfc822.Parse(literal).ParseHeader()
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	boundary := strings.ReplaceAll(uuid.NewString(), "-", "")
	multipartWriter := rfc822.NewMultipartWriter(buf, boundary)

	{
		newHeader := rfc822.NewEmptyHeader()

		if value, ok := header.GetChecked("Message-Id"); ok {
			newHeader.Set("Message-Id", value)
		}

		contentType := mime.FormatMediaType("multipart/encrypted", map[string]string{
			"boundary": boundary,
			"protocol": "application/pgp-encrypted",
		})
		newHeader.Set("Mime-version", "1.0")
		newHeader.Set("Content-Type", contentType)

		if value, ok := header.GetChecked("From"); ok {
			newHeader.Set("From", value)
		}

		if value, ok := header.GetChecked("To"); ok {
			newHeader.Set("To", value)
		}

		if value, ok := header.GetChecked("Subject"); ok {
			newHeader.Set("Subject", value)
		}

		if value, ok := header.GetChecked("Date"); ok {
			newHeader.Set("Date", value)
		}

		if value, ok := header.GetChecked("Received"); ok {
			newHeader.Set("Received", value)
		}

		buf.Write(newHeader.Raw())
	}

	// Write PGP control data
	{
		pgpControlHeader := rfc822.NewEmptyHeader()
		pgpControlHeader.Set("Content-Description", "PGP/MIME version identification")
		pgpControlHeader.Set("Content-Type", "application/pgp-encrypted")
		if err := multipartWriter.AddPart(func(writer io.Writer) error {
			if _, err := writer.Write(pgpControlHeader.Raw()); err != nil {
				return err
			}

			_, err := writer.Write([]byte("Version: 1"))

			return err
		}); err != nil {
			return nil, err
		}
	}

	// write PGP attachment
	{
		pgpAttachmentHeader := rfc822.NewEmptyHeader()
		contentType := mime.FormatMediaType("application/octet-stream", map[string]string{
			"name": "encrypted.asc",
		})
		pgpAttachmentHeader.Set("Content-Description", "OpenPGP encrypted message")
		pgpAttachmentHeader.Set("Content-Disposition", "inline; filename=encrypted.asc")
		pgpAttachmentHeader.Set("Content-Type", contentType)

		if err := multipartWriter.AddPart(func(writer io.Writer) error {
			if _, err := writer.Write(pgpAttachmentHeader.Raw()); err != nil {
				return err
			}

			_, err := writer.Write([]byte(arm))
			return err
		}); err != nil {
			return nil, err
		}
	}

	// finish messsage
	if err := multipartWriter.Done(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func encodeBase64(writer io.Writer, b []byte) error {
	encoder := base64.NewEncoder(base64.StdEncoding, writer)
	defer encoder.Close()

	if _, err := encoder.Write(b); err != nil {
		return err
	}

	return nil
}

func getCharsetDecoder(r io.Reader, charset string) (io.Reader, error) {
	if CharsetReader != nil {
		if enc, err := CharsetReader(charset, r); err == nil {
			return enc, nil
		}
	}

	if enc, err := ianaindex.MIME.Encoding(strings.ToLower(charset)); err == nil {
		return enc.NewDecoder().Reader(r), nil
	}

	if enc, err := ianaindex.MIME.Encoding("cs" + strings.ToLower(charset)); err == nil {
		return enc.NewDecoder().Reader(r), nil
	}

	if enc, err := htmlindex.Get(strings.ToLower(charset)); err == nil {
		return enc.NewDecoder().Reader(r), nil
	}

	return nil, fmt.Errorf("unknown charset: %s", charset)
}

func replace[Key comparable, Value any](m map[Key]Value, key Key, value Value) map[Key]Value {
	m[key] = value
	return m
}
