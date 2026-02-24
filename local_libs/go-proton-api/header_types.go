package proton

import (
	"encoding/json"
	"errors"
)

var ErrBadHeader = errors.New("bad header")

type Headers map[string][]string

func (h *Headers) UnmarshalJSON(b []byte) error {
	type rawHeaders map[string]any

	raw := make(rawHeaders)

	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	header := make(Headers)

	for key, val := range raw {
		switch val := val.(type) {
		case string:
			header[key] = []string{val}

		case []any:
			for _, val := range val {
				switch val := val.(type) {
				case string:
					header[key] = append(header[key], val)

				default:
					return ErrBadHeader
				}
			}

		default:
			return ErrBadHeader
		}
	}

	*h = header

	return nil
}
