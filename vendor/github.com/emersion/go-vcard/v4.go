package vcard

import (
	"strings"
)

// See https://github.com/mangstadt/ez-vcard/wiki/Version-differences

// ToV4 converts a card to vCard version 4.
func ToV4(card Card) {
	version := card.Value(FieldVersion)
	if strings.HasPrefix(version, "4.") {
		return
	}

	card.SetValue(FieldVersion, "4.0")

	for k, fields := range card {
		if strings.EqualFold(k, FieldVersion) {
			continue
		}

		for _, f := range fields {
			if f.Params.HasType("pref") {
				delete(f.Params, "pref")
				f.Params.Set("pref", "1")
			}
		}
	}
}
