package vcard

import (
	"errors"
	"io"
	"sort"
	"strings"
)

// An Encoder formats cards.
type Encoder struct {
	w io.Writer
}

// NewEncoder creates a new Encoder that writes cards to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w}
}

// Encode formats a card. The card must have a FieldVersion field.
func (enc *Encoder) Encode(c Card) error {
	begin := "BEGIN:VCARD\r\n"
	if _, err := io.WriteString(enc.w, begin); err != nil {
		return err
	}

	version := c.Get(FieldVersion)
	if version == nil {
		return errors.New("vcard: VERSION field missing")
	}
	if _, err := io.WriteString(enc.w, formatLine(FieldVersion, version)+"\r\n"); err != nil {
		return err
	}

	var keys []string
	for k := range c {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fields := c[k]
		if strings.EqualFold(k, FieldVersion) {
			continue
		}
		for _, f := range fields {
			if _, err := io.WriteString(enc.w, formatLine(k, f)+"\r\n"); err != nil {
				return err
			}
		}
	}

	end := "END:VCARD\r\n"
	_, err := io.WriteString(enc.w, end)
	return err
}

func formatLine(key string, field *Field) string {
	var s string

	if field.Group != "" {
		s += field.Group + "."
	}
	s += key

	for pk, pvs := range field.Params {
		for _, pv := range pvs {
			s += ";" + formatParam(pk, pv)
		}
	}

	s += ":" + formatValue(field.Value)
	return s
}

func formatParam(k, v string) string {
	return k + "=" + formatValue(v)
}

var valueFormatter = strings.NewReplacer("\\", "\\\\", "\n", "\\n", ",", "\\,")

func formatValue(v string) string {
	return valueFormatter.Replace(v)
}
