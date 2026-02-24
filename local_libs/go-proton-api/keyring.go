package proton

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/bradenaw/juniper/xslices"
	"github.com/sirupsen/logrus"
)

func ExtractSignatures(kr *crypto.KeyRing, arm string) ([]Signature, error) {
	entities := xslices.Map(kr.GetKeys(), func(key *crypto.Key) *openpgp.Entity {
		return key.GetEntity()
	})

	p, err := armor.Decode(strings.NewReader(arm))
	if err != nil {
		return nil, err
	}

	msg, err := openpgp.ReadMessage(p.Body, openpgp.EntityList(entities), nil, nil)
	if err != nil {
		return nil, err
	}

	if _, err := io.ReadAll(msg.UnverifiedBody); err != nil {
		return nil, err
	}

	if !msg.IsSigned {
		return nil, nil
	}

	var signatures []Signature

	for _, signature := range msg.UnverifiedSignatures {
		buf := new(bytes.Buffer)

		if err := signature.Serialize(buf); err != nil {
			return nil, err
		}

		signatures = append(signatures, Signature{
			Hash: signature.Hash.String(),
			Data: crypto.NewPGPSignature(buf.Bytes()),
		})
	}

	return signatures, nil
}

type Key struct {
	ID         string
	PrivateKey []byte
	Token      string
	Signature  string
	Primary    Bool
	Active     Bool
	Flags      KeyState
}

func (key *Key) UnmarshalJSON(data []byte) error {
	type Alias Key

	aux := &struct {
		PrivateKey string

		*Alias
	}{
		Alias: (*Alias)(key),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	privKey, err := crypto.NewKeyFromArmored(aux.PrivateKey)
	if err != nil {
		return err
	}

	raw, err := privKey.Serialize()
	if err != nil {
		return err
	}

	key.PrivateKey = raw

	return nil
}

func (key Key) MarshalJSON() ([]byte, error) {
	privKey, err := crypto.NewKey(key.PrivateKey)
	if err != nil {
		return nil, err
	}

	arm, err := privKey.Armor()
	if err != nil {
		return nil, err
	}

	type Alias Key

	aux := &struct {
		PrivateKey string

		*Alias
	}{
		PrivateKey: arm,
		Alias:      (*Alias)(&key),
	}

	return json.Marshal(aux)
}

type Keys []Key

func (keys Keys) Primary() Key {
	for _, key := range keys {
		if key.Primary {
			return key
		}
	}

	panic("no primary key available")
}

func (keys Keys) ByID(keyID string) Key {
	for _, key := range keys {
		if key.ID == keyID {
			return key
		}
	}

	panic("no primary key available")
}

func (keys Keys) Unlock(passphrase []byte, userKR *crypto.KeyRing) (*crypto.KeyRing, error) {
	kr, err := crypto.NewKeyRing(nil)
	if err != nil {
		return nil, err
	}

	for _, key := range xslices.Filter(keys, func(key Key) bool { return bool(key.Active) }) {
		unlocked, err := key.Unlock(passphrase, userKR)
		if err != nil {
			logrus.WithField("KeyID", key.ID).WithError(err).Warning("Cannot unlock key")
			continue
		}

		if err := kr.AddKey(unlocked); err != nil {
			return nil, err
		}
	}

	if kr.CountEntities() == 0 {
		return nil, errors.New("not able to unlock any key")
	}

	return kr, nil
}

func (keys Keys) TryUnlock(passphrase []byte, userKR *crypto.KeyRing) *crypto.KeyRing {
	kr, err := keys.Unlock(passphrase, userKR)
	if err != nil {
		return nil
	}

	return kr
}

func (key Key) Unlock(passphrase []byte, userKR *crypto.KeyRing) (*crypto.Key, error) {
	var secret []byte

	if key.Token == "" || key.Signature == "" {
		secret = passphrase
	} else {
		var err error

		if secret, err = key.getPassphraseFromToken(userKR); err != nil {
			return nil, err
		}
	}

	return key.unlock(secret)
}

func (key Key) getPassphraseFromToken(kr *crypto.KeyRing) ([]byte, error) {
	if kr == nil {
		return nil, errors.New("no user key was provided")
	}

	msg, err := crypto.NewPGPMessageFromArmored(key.Token)
	if err != nil {
		return nil, err
	}

	sig, err := crypto.NewPGPSignatureFromArmored(key.Signature)
	if err != nil {
		return nil, err
	}

	token, err := kr.Decrypt(msg, nil, 0)
	if err != nil {
		return nil, err
	}

	if err = kr.VerifyDetached(token, sig, 0); err != nil {
		return nil, err
	}

	return token.GetBinary(), nil
}

func (key Key) unlock(passphrase []byte) (*crypto.Key, error) {
	lk, err := crypto.NewKey(key.PrivateKey)
	if err != nil {
		return nil, err
	}
	defer lk.ClearPrivateParams()

	uk, err := lk.Unlock(passphrase)
	if err != nil {
		return nil, err
	}

	ok, err := uk.Check()
	if err != nil {
		return nil, err
	} else if !ok {
		return nil, errors.New("private and public keys do not match")
	}

	return uk, nil
}

func DecodeKeyPacket(packet string) []byte {
	if packet == "" {
		return nil
	}

	raw, err := base64.StdEncoding.DecodeString(packet)
	if err != nil {
		panic(err)
	}

	return raw
}
