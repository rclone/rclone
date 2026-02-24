package backend

import (
	"strconv"
	"sync/atomic"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/bradenaw/juniper/xslices"
	"github.com/emersion/go-vcard"
	"github.com/rclone/go-proton-api"
)

var globalContactID int32

func ContactCardToContact(card *proton.Card, contactID string, kr *crypto.KeyRing) (proton.Contact, error) {
	emails, err := card.Get(kr, vcard.FieldEmail)
	if err != nil {
		return proton.Contact{}, err
	}
	names, err := card.Get(kr, vcard.FieldFormattedName)
	if err != nil {
		return proton.Contact{}, err
	}
	return proton.Contact{
		ContactMetadata: proton.ContactMetadata{
			ID:   contactID,
			Name: names[0].Value,
			ContactEmails: xslices.Map(emails, func(email *vcard.Field) proton.ContactEmail {
				id := atomic.AddInt32(&globalContactID, 1)
				return proton.ContactEmail{
					ID:        strconv.Itoa(int(id)),
					Name:      names[0].Value,
					Email:     email.Value,
					ContactID: contactID,
				}
			}),
		},
		ContactCards: proton.ContactCards{Cards: proton.Cards{card}},
	}, nil
}
