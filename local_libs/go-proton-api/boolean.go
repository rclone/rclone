package proton

import "encoding/json"

// Bool is a convenience type for boolean values; it converts from APIBool to Go's builtin bool type.
type Bool bool

// APIBool is the boolean type used by the API (0 or 1).
type APIBool int

const (
	APIFalse APIBool = iota
	APITrue
)

func (b *Bool) UnmarshalJSON(data []byte) error {
	var v APIBool

	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	*b = Bool(v == APITrue)

	return nil
}

func (b Bool) MarshalJSON() ([]byte, error) {
	var v APIBool

	if b {
		v = APITrue
	} else {
		v = APIFalse
	}

	return json.Marshal(v)
}

func (b Bool) String() string {
	if b {
		return "true"
	}

	return "false"
}

func (b Bool) FormatURL() string {
	if b {
		return "1"
	}

	return "0"
}
