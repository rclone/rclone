package backend

import (
	"github.com/ProtonMail/go-srp"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/ProtonMail/gopenpgp/v2/helper"
)

var GenerateKey = helper.GenerateKey

func hashPassword(password, salt []byte) ([]byte, error) {
	passphrase, err := srp.MailboxPassword(password, salt)
	if err != nil {
		return nil, err
	}

	return passphrase[len(passphrase)-31:], nil
}

func encryptWithSignature(kr *crypto.KeyRing, b []byte) (string, string, error) {
	enc, err := kr.Encrypt(crypto.NewPlainMessage(b), nil)
	if err != nil {
		return "", "", err
	}

	encArm, err := enc.GetArmored()
	if err != nil {
		return "", "", err
	}

	sig, err := kr.SignDetached(crypto.NewPlainMessage(b))
	if err != nil {
		return "", "", err
	}

	sigArm, err := sig.GetArmored()
	if err != nil {
		return "", "", err
	}

	return encArm, sigArm, nil
}
