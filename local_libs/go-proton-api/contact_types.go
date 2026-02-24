package proton

import (
	"encoding/base64"
	"strconv"
	"strings"

	"github.com/ProtonMail/gluon/rfc822"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/emersion/go-vcard"
)

type RecipientType int

const (
	RecipientTypeInternal RecipientType = iota + 1
	RecipientTypeExternal
)

type ContactSettings struct {
	MIMEType *rfc822.MIMEType
	Scheme   *EncryptionScheme
	Sign     *bool
	Encrypt  *bool
	Keys     []*crypto.Key
}

type Contact struct {
	ContactMetadata
	ContactCards
}

func (c *Contact) GetSettings(kr *crypto.KeyRing, email string) (ContactSettings, error) {
	signedCard, ok := c.Cards.Get(CardTypeSigned)
	if !ok {
		return ContactSettings{}, nil
	}

	group, err := signedCard.GetGroup(kr, vcard.FieldEmail, email)
	if err != nil {
		return ContactSettings{}, nil
	}

	var settings ContactSettings

	scheme, err := group.Get(FieldPMScheme)
	if err != nil {
		return ContactSettings{}, err
	}

	if len(scheme) > 0 {
		switch scheme[0] {
		case "pgp-inline":
			settings.Scheme = newPtr(PGPInlineScheme)

		case "pgp-mime":
			settings.Scheme = newPtr(PGPMIMEScheme)
		}
	}

	mimeType, err := group.Get(FieldPMMIMEType)
	if err != nil {
		return ContactSettings{}, err
	}

	if len(mimeType) > 0 {
		settings.MIMEType = newPtr(rfc822.MIMEType(mimeType[0]))
	}

	sign, err := group.Get(FieldPMSign)
	if err != nil {
		return ContactSettings{}, err
	}

	if len(sign) > 0 {
		sign, err := strconv.ParseBool(sign[0])
		if err != nil {
			return ContactSettings{}, err
		}

		settings.Sign = newPtr(sign)
	}

	encrypt, err := group.Get(FieldPMEncrypt)
	if err != nil {
		return ContactSettings{}, err
	}

	if len(encrypt) > 0 {
		encrypt, err := strconv.ParseBool(encrypt[0])
		if err != nil {
			return ContactSettings{}, err
		}

		settings.Encrypt = newPtr(encrypt)
	}

	keys, err := group.Get(vcard.FieldKey)
	if err != nil {
		return ContactSettings{}, err
	}

	if len(keys) > 0 {
		for _, key := range keys {
			dec, err := base64.StdEncoding.DecodeString(strings.SplitN(key, ",", 2)[1])
			if err != nil {
				return ContactSettings{}, err
			}

			pubKey, err := crypto.NewKey(dec)
			if err != nil {
				return ContactSettings{}, err
			}

			settings.Keys = append(settings.Keys, pubKey)
		}
	}

	return settings, nil
}

type ContactMetadata struct {
	ID            string
	Name          string
	UID           string
	Size          int64
	CreateTime    int64
	ModifyTime    int64
	ContactEmails []ContactEmail
	LabelIDs      []string
}

type ContactCards struct {
	Cards Cards
}

type ContactEmail struct {
	ID        string
	Name      string
	Email     string
	Type      []string
	ContactID string
	LabelIDs  []string
}

type CreateContactsReq struct {
	Contacts  []ContactCards
	Overwrite int
	Labels    int
}

type CreateContactsRes struct {
	Index int

	Response struct {
		APIError
		Contact Contact
	}
}

type UpdateContactReq struct {
	Cards Cards
}

type DeleteContactsReq struct {
	IDs []string
}

func newPtr[T any](v T) *T {
	return &v
}
