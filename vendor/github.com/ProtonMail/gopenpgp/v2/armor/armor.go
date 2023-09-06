// Package armor contains a set of helper methods for armoring and unarmoring
// data.
package armor

import (
	"bytes"
	"io"
	"io/ioutil"

	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/gopenpgp/v2/constants"
	"github.com/ProtonMail/gopenpgp/v2/internal"
	"github.com/pkg/errors"
)

// ArmorKey armors input as a public key.
func ArmorKey(input []byte) (string, error) {
	return ArmorWithType(input, constants.PublicKeyHeader)
}

// ArmorWithTypeBuffered returns a io.WriteCloser which, when written to, writes
// armored data to w with the given armorType.
func ArmorWithTypeBuffered(w io.Writer, armorType string) (io.WriteCloser, error) {
	return armor.Encode(w, armorType, nil)
}

// ArmorWithType armors input with the given armorType.
func ArmorWithType(input []byte, armorType string) (string, error) {
	return armorWithTypeAndHeaders(input, armorType, internal.ArmorHeaders)
}

// ArmorWithTypeAndCustomHeaders armors input with the given armorType and
// headers.
func ArmorWithTypeAndCustomHeaders(input []byte, armorType, version, comment string) (string, error) {
	headers := make(map[string]string)
	if version != "" {
		headers["Version"] = version
	}
	if comment != "" {
		headers["Comment"] = comment
	}
	return armorWithTypeAndHeaders(input, armorType, headers)
}

// Unarmor unarmors an armored input into a byte array.
func Unarmor(input string) ([]byte, error) {
	b, err := internal.Unarmor(input)
	if err != nil {
		return nil, errors.Wrap(err, "gopengp: unable to unarmor")
	}
	return ioutil.ReadAll(b.Body)
}

func armorWithTypeAndHeaders(input []byte, armorType string, headers map[string]string) (string, error) {
	var b bytes.Buffer

	w, err := armor.Encode(&b, armorType, headers)

	if err != nil {
		return "", errors.Wrap(err, "gopengp: unable to encode armoring")
	}
	if _, err = w.Write(input); err != nil {
		return "", errors.Wrap(err, "gopengp: unable to write armored to buffer")
	}
	if err := w.Close(); err != nil {
		return "", errors.Wrap(err, "gopengp: unable to close armor buffer")
	}
	return b.String(), nil
}
