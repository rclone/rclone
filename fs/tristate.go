package fs

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Tristate is a boolean that can has the states, true, false and
// unset/invalid/nil
type Tristate struct {
	Value bool
	Valid bool
}

// String renders the tristate as true/false/unset
func (t Tristate) String() string {
	if !t.Valid {
		return "unset"
	}
	if t.Value {
		return "true"
	}
	return "false"
}

// Set the List entries
func (t *Tristate) Set(s string) error {
	s = strings.ToLower(s)
	if s == "" || s == "nil" || s == "null" || s == "unset" {
		t.Valid = false
		return nil
	}
	value, err := strconv.ParseBool(s)
	if err != nil {
		return fmt.Errorf("failed to parse Tristate %q: %w", s, err)
	}
	t.Value = value
	t.Valid = true
	return nil
}

// Type of the value
func (Tristate) Type() string {
	return "Tristate"
}

// Scan implements the fmt.Scanner interface
func (t *Tristate) Scan(s fmt.ScanState, ch rune) error {
	token, err := s.Token(true, nil)
	if err != nil {
		return err
	}
	return t.Set(string(token))
}

// UnmarshalJSON parses it as a bool or nil for unset
func (t *Tristate) UnmarshalJSON(in []byte) error {
	var b *bool
	err := json.Unmarshal(in, &b)
	if err != nil {
		return err
	}
	if b != nil {
		t.Valid = true
		t.Value = *b
	} else {
		t.Valid = false
	}
	return nil
}

// MarshalJSON encodes it as a bool or nil for unset
func (t *Tristate) MarshalJSON() ([]byte, error) {
	if !t.Valid {
		return json.Marshal(nil)
	}
	return json.Marshal(t.Value)
}
