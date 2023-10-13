package fs

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Enum is an option which can only be one of the Choices.
//
// Suggested implementation is something like this:
//
//	type choice = Enum[choices]
//
//	const (
//		choiceA choice = iota
//		choiceB
//		choiceC
//	)
//
//	type choices struct{}
//
//	func (choices) Choices() []string {
//		return []string{
//			choiceA: "A",
//			choiceB: "B",
//			choiceC: "C",
//		}
//	}
type Enum[C Choices] byte

// Choices returns the valid choices for this type.
//
// It must work on the zero value.
//
// Note that when using this in an Option the ExampleChoices will be
// filled in automatically.
type Choices interface {
	// Choices returns the valid choices for this type
	Choices() []string
}

// String renders the Enum as a string
func (e Enum[C]) String() string {
	choices := e.Choices()
	if int(e) >= len(choices) {
		return fmt.Sprintf("Unknown(%d)", e)
	}
	return choices[e]
}

// Choices returns the possible values of the Enum.
func (e Enum[C]) Choices() []string {
	var c C
	return c.Choices()
}

// Help returns a comma separated list of all possible states.
func (e Enum[C]) Help() string {
	return strings.Join(e.Choices(), ", ")
}

// Set the Enum entries
func (e *Enum[C]) Set(s string) error {
	for i, choice := range e.Choices() {
		if strings.EqualFold(s, choice) {
			*e = Enum[C](i)
			return nil
		}
	}
	return fmt.Errorf("invalid choice %q from: %s", s, e.Help())
}

// Type of the value.
//
// If C has a Type() string method then it will be used instead.
func (e Enum[C]) Type() string {
	var c C
	if do, ok := any(c).(typer); ok {
		return do.Type()
	}
	return strings.Join(e.Choices(), "|")
}

// Scan implements the fmt.Scanner interface
func (e *Enum[C]) Scan(s fmt.ScanState, ch rune) error {
	token, err := s.Token(true, nil)
	if err != nil {
		return err
	}
	return e.Set(string(token))
}

// UnmarshalJSON parses it as a string or an integer
func (e *Enum[C]) UnmarshalJSON(in []byte) error {
	choices := e.Choices()
	return UnmarshalJSONFlag(in, e, func(i int64) error {
		if i < 0 || i >= int64(len(choices)) {
			return fmt.Errorf("%d is out of range: must be 0..%d", i, len(choices))
		}
		*e = Enum[C](i)
		return nil
	})

}

// MarshalJSON encodes it as string
func (e *Enum[C]) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}
