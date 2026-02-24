package proton

import (
	"bytes"
	"encoding/base64"
	"io"
	"testing"

	"github.com/ProtonMail/gluon/rfc822"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptMessage_Simple(t *testing.T) {
	const message = `From: Nathaniel Borenstein <nsb@bellcore.com>
To:  Ned Freed <ned@innosoft.com>
Subject: Sample message (import 2)
MIME-Version: 1.0
Content-type: text/plain

This is explicitly typed plain ASCII text.
`
	key, err := crypto.GenerateKey("foobar", "foo@bar.com", "x25519", 0)
	require.NoError(t, err)

	kr, err := crypto.NewKeyRing(key)
	require.NoError(t, err)

	encryptedMessage, err := EncryptRFC822(kr, []byte(message))
	require.NoError(t, err)

	section := rfc822.Parse(encryptedMessage)

	// Check root header:
	header, err := section.ParseHeader()
	require.NoError(t, err)

	assert.Equal(t, header.Get("From"), "Nathaniel Borenstein <nsb@bellcore.com>")
	assert.Equal(t, header.Get("To"), "Ned Freed <ned@innosoft.com>")
	assert.Equal(t, header.Get("Subject"), "Sample message (import 2)")
	assert.Equal(t, header.Get("MIME-Version"), "1.0")

	// Read the body.
	body, err := section.DecodedBody()
	require.NoError(t, err)

	// Unarmor the PGP message.
	enc, err := crypto.NewPGPMessageFromArmored(string(body))
	require.NoError(t, err)

	// Decrypt the PGP message.
	dec, err := kr.Decrypt(enc, nil, crypto.GetUnixTime())
	require.NoError(t, err)
	require.Equal(t, "This is explicitly typed plain ASCII text.\n", dec.GetString())
}

func TestEncryptMessage_MultipleTextParts(t *testing.T) {
	const message = `From: Nathaniel Borenstein <nsb@bellcore.com>
To:  Ned Freed <ned@innosoft.com>
Subject: Sample message (import 2)
MIME-Version: 1.0
Content-type: multipart/mixed; boundary="simple boundary"
Received: from mail.protonmail.ch by mail.protonmail.ch; Tue, 25 Nov 2016

This is the preamble.  It is to be ignored, though it
is a handy place for mail composers to include an
explanatory note to non-MIME compliant readers.
--simple boundary

This is implicitly typed plain ASCII text.
It does NOT end with a linebreak.
--simple boundary
Content-type: text/plain; charset=us-ascii

This is explicitly typed plain ASCII text.
It DOES end with a linebreak.

--simple boundary--
This is the epilogue.  It is also to be ignored.
`
	key, err := crypto.GenerateKey("foobar", "foo@bar.com", "x25519", 0)
	require.NoError(t, err)

	kr, err := crypto.NewKeyRing(key)
	require.NoError(t, err)

	encryptedMessage, err := EncryptRFC822(kr, []byte(message))
	require.NoError(t, err)

	section := rfc822.Parse(encryptedMessage)

	{
		// Check root header:
		header, err := section.ParseHeader()
		require.NoError(t, err)

		assert.Equal(t, header.Get("From"), "Nathaniel Borenstein <nsb@bellcore.com>")
		assert.Equal(t, header.Get("To"), "Ned Freed <ned@innosoft.com>")
		assert.Equal(t, header.Get("Subject"), "Sample message (import 2)")
		assert.Equal(t, header.Get("MIME-Version"), "1.0")
		assert.Equal(t, header.Get("Received"), "from mail.protonmail.ch by mail.protonmail.ch; Tue, 25 Nov 2016")

		mediaType, params, err := rfc822.ParseMediaType(header.Get("Content-Type"))
		require.NoError(t, err)
		assert.Equal(t, "multipart/encrypted", mediaType)
		assert.Equal(t, "application/pgp-encrypted", params["protocol"])
		assert.NotEmpty(t, params["boundary"])
	}

	children, err := section.Children()
	require.NoError(t, err)
	require.Equal(t, 2, len(children))

	{
		// check first child.
		child := children[0]
		header, err := child.ParseHeader()
		require.NoError(t, err)

		assert.Equal(t, header.Get("Content-Description"), "PGP/MIME version identification")
		assert.Equal(t, header.Get("Content-Type"), "application/pgp-encrypted")

		assert.Equal(t, []byte("Version: 1"), child.Body())
	}

	{
		// check second child.
		child := children[1]
		header, err := child.ParseHeader()
		require.NoError(t, err)

		assert.Equal(t, header.Get("Content-Description"), "OpenPGP encrypted message")
		assert.Equal(t, header.Get("Content-Disposition"), "inline; filename=encrypted.asc")
		assert.Equal(t, header.Get("Content-type"), "application/octet-stream; name=encrypted.asc")

		body := child.Body()
		assert.True(t, bytes.HasPrefix(body, []byte("-----BEGIN PGP MESSAGE-----")))
		assert.True(t, bytes.HasSuffix(body, []byte("-----END PGP MESSAGE-----")))
	}
}

func TestEncryptMessage_Attachment(t *testing.T) {
	const message = `From: Nathaniel Borenstein <nsb@bellcore.com>
To:  Ned Freed <ned@innosoft.com>
Subject: Sample message (import 2)
MIME-Version: 1.0
Content-type: multipart/mixed; boundary="simple boundary"

--simple boundary
Content-type: text/plain; charset=us-ascii

Hello world

--simple boundary
Content-Type: application/pdf; name="test.pdf"
Content-Disposition: attachment; filename="test.pdf"
Content-Transfer-Encoding: base64

SGVsbG8gQXR0YWNobWVudA==

--simple boundary--
`
	key, err := crypto.GenerateKey("foobar", "foo@bar.com", "x25519", 0)
	require.NoError(t, err)

	kr, err := crypto.NewKeyRing(key)
	require.NoError(t, err)

	encryptedMessage, err := EncryptRFC822(kr, []byte(message))
	require.NoError(t, err)

	section := rfc822.Parse(encryptedMessage)

	{
		// Check root header:
		header, err := section.ParseHeader()
		require.NoError(t, err)

		assert.Equal(t, header.Get("From"), "Nathaniel Borenstein <nsb@bellcore.com>")
		assert.Equal(t, header.Get("To"), "Ned Freed <ned@innosoft.com>")
		assert.Equal(t, header.Get("Subject"), "Sample message (import 2)")
		assert.Equal(t, header.Get("MIME-Version"), "1.0")

		mediaType, params, err := rfc822.ParseMediaType(header.Get("Content-Type"))
		require.NoError(t, err)
		assert.Equal(t, "multipart/mixed", mediaType)
		assert.NotEmpty(t, params["boundary"])
	}

	children, err := section.Children()
	require.NoError(t, err)
	require.Equal(t, 2, len(children))

	{
		// check first child.
		child := children[0]
		header, err := child.ParseHeader()
		require.NoError(t, err)

		header.Entries(func(key, value string) {
			// Old header should be deleted.
			assert.NotEqual(t, key, "Content-type")
			assert.NotEqual(t, value, "text/plain; charset=us-ascii")
		})

		assert.Equal(t, header.Get("Content-Type"), "text/plain; charset=utf-8")
	}

	{
		// check second child.
		child := children[1]
		header, err := child.ParseHeader()
		require.NoError(t, err)

		assert.Equal(t, header.Get("Content-Transfer-Encoding"), "base64")
		assert.Equal(t, header.Get("Content-Disposition"), `attachment; filename="test.pdf"`)
		assert.Equal(t, header.Get("Content-type"), `application/pdf; name="test.pdf"`)

		body := child.Body()

		// Read the body.
		bodyDecoded, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, bytes.NewReader(body)))
		require.NoError(t, err)

		// Unarmor the PGP message.
		enc := crypto.NewPGPMessage(bodyDecoded)

		// Decrypt the PGP message.
		dec, err := kr.Decrypt(enc, nil, crypto.GetUnixTime())
		require.NoError(t, err)
		require.Equal(t, "Hello Attachment", dec.GetString())
	}
}
