package proton

import (
	"bytes"
	"errors"
	"strings"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/bradenaw/juniper/xslices"
	"github.com/emersion/go-vcard"
)

const (
	FieldPMScheme   = "X-PM-SCHEME"
	FieldPMSign     = "X-PM-SIGN"
	FieldPMEncrypt  = "X-PM-ENCRYPT"
	FieldPMMIMEType = "X-PM-MIMETYPE"
)

type Cards []*Card

func (c *Cards) Merge(kr *crypto.KeyRing) (vcard.Card, error) {
	merged := newVCard()

	for _, card := range *c {
		dec, err := card.decode(kr)
		if err != nil {
			return nil, err
		}

		for k, fields := range dec {
			for _, f := range fields {
				merged.Add(k, f)
			}
		}
	}

	return merged, nil
}

func (c *Cards) Get(cardType CardType) (*Card, bool) {
	for _, card := range *c {
		if card.Type == cardType {
			return card, true
		}
	}

	return nil, false
}

type Card struct {
	Type      CardType
	Data      string
	Signature string
}

type CardType int

const (
	CardTypeClear CardType = iota
	CardTypeEncrypted
	CardTypeSigned
)

func NewCard(kr *crypto.KeyRing, cardType CardType) (*Card, error) {
	card := &Card{Type: cardType}

	if err := card.encode(kr, newVCard()); err != nil {
		return nil, err
	}

	return card, nil
}

func newVCard() vcard.Card {
	card := make(vcard.Card)

	card.AddValue(vcard.FieldVersion, "4.0")

	return card
}

func (c Card) Get(kr *crypto.KeyRing, key string) ([]*vcard.Field, error) {
	dec, err := c.decode(kr)
	if err != nil {
		return nil, err
	}

	return dec[key], nil
}

func (c *Card) Set(kr *crypto.KeyRing, key, value string) error {
	dec, err := c.decode(kr)
	if err != nil {
		return err
	}

	if field := dec.Get(key); field != nil {
		field.Value = value

		return c.encode(kr, dec)
	}

	dec.AddValue(key, value)

	return c.encode(kr, dec)
}

func (c *Card) ChangeType(kr *crypto.KeyRing, cardType CardType) error {
	dec, err := c.decode(kr)
	if err != nil {
		return err
	}

	c.Type = cardType

	return c.encode(kr, dec)
}

// GetGroup returns a type to manipulate the group defined by the given key/value pair.
func (c Card) GetGroup(kr *crypto.KeyRing, groupKey, groupValue string) (CardGroup, error) {
	group, err := c.getGroup(kr, groupKey, groupValue)
	if err != nil {
		return CardGroup{}, err
	}

	return CardGroup{Card: c, kr: kr, group: group}, nil
}

// DeleteGroup removes all values in the group defined by the given key/value pair.
func (c *Card) DeleteGroup(kr *crypto.KeyRing, groupKey, groupValue string) error {
	group, err := c.getGroup(kr, groupKey, groupValue)
	if err != nil {
		return err
	}

	return c.deleteGroup(kr, group)
}

type CardGroup struct {
	Card

	kr    *crypto.KeyRing
	group string
}

// Get returns the values in the group with the given key.
func (g CardGroup) Get(key string) ([]string, error) {
	dec, err := g.decode(g.kr)
	if err != nil {
		return nil, err
	}

	var fields []*vcard.Field

	for _, field := range dec[key] {
		if field.Group != g.group {
			continue
		}

		fields = append(fields, field)
	}

	return xslices.Map(fields, func(field *vcard.Field) string {
		return field.Value
	}), nil
}

// Set sets the value in the group.
func (g *CardGroup) Set(key, value string, params vcard.Params) error {
	dec, err := g.decode(g.kr)
	if err != nil {
		return err
	}

	for _, field := range dec[key] {
		if field.Group != g.group {
			continue
		}

		field.Value = value

		return g.encode(g.kr, dec)
	}

	dec.Add(key, &vcard.Field{
		Value:  value,
		Group:  g.group,
		Params: params,
	})

	return g.encode(g.kr, dec)
}

// Add adds a value to the group.
func (g *CardGroup) Add(key, value string, params vcard.Params) error {
	dec, err := g.decode(g.kr)
	if err != nil {
		return err
	}

	dec.Add(key, &vcard.Field{
		Value:  value,
		Group:  g.group,
		Params: params,
	})

	return g.encode(g.kr, dec)
}

// Remove removes the value in the group with the given key/value.
func (g *CardGroup) Remove(key, value string) error {
	dec, err := g.decode(g.kr)
	if err != nil {
		return err
	}

	fields, ok := dec[key]
	if !ok {
		return errors.New("no such key")
	}

	var rest []*vcard.Field

	for _, field := range fields {
		if field.Group != g.group {
			rest = append(rest, field)
		} else if field.Value != value {
			rest = append(rest, field)
		}
	}

	if len(rest) > 0 {
		dec[key] = rest
	} else {
		delete(dec, key)
	}

	return g.encode(g.kr, dec)
}

// RemoveAll removes all values in the group with the given key.
func (g *CardGroup) RemoveAll(key string) error {
	dec, err := g.decode(g.kr)
	if err != nil {
		return err
	}

	fields, ok := dec[key]
	if !ok {
		return errors.New("no such key")
	}

	var rest []*vcard.Field

	for _, field := range fields {
		if field.Group != g.group {
			rest = append(rest, field)
		}
	}

	if len(rest) > 0 {
		dec[key] = rest
	} else {
		delete(dec, key)
	}

	return g.encode(g.kr, dec)
}

func (c Card) getGroup(kr *crypto.KeyRing, groupKey, groupValue string) (string, error) {
	fields, err := c.Get(kr, groupKey)
	if err != nil {
		return "", err
	}

	for _, field := range fields {
		if field.Value != groupValue {
			continue
		}

		return field.Group, nil
	}

	return "", errors.New("no such field")
}

func (c *Card) deleteGroup(kr *crypto.KeyRing, group string) error {
	dec, err := c.decode(kr)
	if err != nil {
		return err
	}

	for key, fields := range dec {
		var rest []*vcard.Field

		for _, field := range fields {
			if field.Group != group {
				rest = append(rest, field)
			}
		}

		if len(rest) > 0 {
			dec[key] = rest
		} else {
			delete(dec, key)
		}
	}

	return c.encode(kr, dec)
}

func (c Card) decode(kr *crypto.KeyRing) (vcard.Card, error) {
	if c.Type&CardTypeEncrypted != 0 {
		enc, err := crypto.NewPGPMessageFromArmored(c.Data)
		if err != nil {
			return nil, err
		}

		dec, err := kr.Decrypt(enc, nil, crypto.GetUnixTime())
		if err != nil {
			return nil, err
		}

		c.Data = dec.GetString()
	}

	if c.Type&CardTypeSigned != 0 {
		sig, err := crypto.NewPGPSignatureFromArmored(c.Signature)
		if err != nil {
			return nil, err
		}

		if err := kr.VerifyDetached(crypto.NewPlainMessageFromString(c.Data), sig, crypto.GetUnixTime()); err != nil {
			return nil, err
		}
	}

	return vcard.NewDecoder(strings.NewReader(c.Data)).Decode()
}

func (c *Card) encode(kr *crypto.KeyRing, card vcard.Card) error {
	buf := new(bytes.Buffer)

	if err := vcard.NewEncoder(buf).Encode(card); err != nil {
		return err
	}

	if c.Type&CardTypeSigned != 0 {
		sig, err := kr.SignDetached(crypto.NewPlainMessageFromString(buf.String()))
		if err != nil {
			return err
		}

		if c.Signature, err = sig.GetArmored(); err != nil {
			return err
		}
	}

	if c.Type&CardTypeEncrypted != 0 {
		enc, err := kr.Encrypt(crypto.NewPlainMessageFromString(buf.String()), nil)
		if err != nil {
			return err
		}

		if c.Data, err = enc.GetArmored(); err != nil {
			return err
		}
	} else {
		c.Data = buf.String()
	}

	return nil
}
