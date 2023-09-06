package ber

import "encoding/asn1"

// Marshal wraps the asn1.Marshal function
func Marshal(val interface{}) ([]byte, error) {
	return asn1.Marshal(val)
}
