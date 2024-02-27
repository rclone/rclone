package fs

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Bits is an option which can be any combination of the Choices.
//
// Suggested implementation is something like this:
//
//	type bits = Bits[bitsChoices]
//
//	const (
//		bitA bits = 1 << iota
//		bitB
//		bitC
//	)
//
//	type bitsChoices struct{}
//
//	func (bitsChoices) Choices() []BitsChoicesInfo {
//		return []BitsChoicesInfo{
//			{Bit: uint64(0), Name: "OFF"}, // Optional Off value - "" if not defined
//			{Bit: uint64(bitA), Name: "A"},
//			{Bit: uint64(bitB), Name: "B"},
//			{Bit: uint64(bitC), Name: "C"},
//		}
//	}
type Bits[C BitsChoices] uint64

// BitsChoicesInfo should be returned from the Choices method
type BitsChoicesInfo struct {
	Bit  uint64
	Name string
}

// BitsChoices returns the valid choices for this type.
//
// It must work on the zero value.
//
// Note that when using this in an Option the ExampleBitsChoices will be
// filled in automatically.
type BitsChoices interface {
	// Choices returns the valid choices for each bit of this type
	Choices() []BitsChoicesInfo
}

// String turns a Bits into a string
func (b Bits[C]) String() string {
	var out []string
	choices := b.Choices()
	// Return an off value if set
	if b == 0 {
		for _, info := range choices {
			if info.Bit == 0 {
				return info.Name
			}
		}
	}
	for _, info := range choices {
		if info.Bit == 0 {
			continue
		}
		if b&Bits[C](info.Bit) != 0 {
			out = append(out, info.Name)
			b &^= Bits[C](info.Bit)
		}
	}
	if b != 0 {
		out = append(out, fmt.Sprintf("Unknown-0x%X", int(b)))
	}
	return strings.Join(out, ",")
}

// Help returns a comma separated list of all possible bits.
func (b Bits[C]) Help() string {
	var out []string
	for _, info := range b.Choices() {
		out = append(out, info.Name)
	}
	return strings.Join(out, ", ")
}

// Choices returns the possible values of the Bits.
func (b Bits[C]) Choices() []BitsChoicesInfo {
	var c C
	return c.Choices()
}

// Set a Bits as a comma separated list of flags
func (b *Bits[C]) Set(s string) error {
	var flags Bits[C]
	parts := strings.Split(s, ",")
	choices := b.Choices()
	for _, part := range parts {
		found := false
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		for _, info := range choices {
			if strings.EqualFold(info.Name, part) {
				found = true
				flags |= Bits[C](info.Bit)
			}
		}
		if !found {
			return fmt.Errorf("invalid choice %q from: %s", part, b.Help())
		}
	}
	*b = flags
	return nil
}

// IsSet returns true all the bits in mask are set in b.
func (b Bits[C]) IsSet(mask Bits[C]) bool {
	return (b & mask) == mask
}

// Type of the value.
//
// If C has a Type() string method then it will be used instead.
func (b Bits[C]) Type() string {
	var c C
	if do, ok := any(c).(typer); ok {
		return do.Type()
	}
	return "Bits"
}

// Scan implements the fmt.Scanner interface
func (b *Bits[C]) Scan(s fmt.ScanState, ch rune) error {
	token, err := s.Token(true, nil)
	if err != nil {
		return err
	}
	return b.Set(string(token))
}

// UnmarshalJSON makes sure the value can be parsed as a string or integer in JSON
func (b *Bits[C]) UnmarshalJSON(in []byte) error {
	return UnmarshalJSONFlag(in, b, func(i int64) error {
		*b = (Bits[C])(i)
		return nil
	})
}

// MarshalJSON encodes it as string
func (b *Bits[C]) MarshalJSON() ([]byte, error) {
	return json.Marshal(b.String())
}
