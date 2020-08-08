package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Presence describes the presence of a filename in file listing
type Presence int

// Possible Presence states
const (
	Absent Presence = iota
	Present
	Renamed
	Multiple
)

// Position is the placement of the test character in the filename
type Position int

// Predefined positions
const (
	PositionMiddle Position = 1 << iota
	PositionLeft
	PositionRight
	PositionNone Position = 0
	PositionAll  Position = PositionRight<<1 - 1
)

// PositionList contains all valid positions
var PositionList = []Position{PositionMiddle, PositionLeft, PositionRight}

// ControlResult contains the result of a single character test
type ControlResult struct {
	Text       string `json:"-"`
	WriteError map[Position]string
	GetError   map[Position]string
	InList     map[Position]Presence
}

// InfoReport is the structure of the JSON output
type InfoReport struct {
	Remote               string
	ControlCharacters    *map[string]ControlResult
	MaxFileLength        *int
	CanStream            *bool
	CanWriteUnnormalized *bool
	CanReadUnnormalized  *bool
	CanReadRenormalized  *bool
}

func (e Position) String() string {
	switch e {
	case PositionNone:
		return "none"
	case PositionAll:
		return "all"
	}
	var buf bytes.Buffer
	if e&PositionMiddle != 0 {
		buf.WriteString("middle")
		e &= ^PositionMiddle
	}
	if e&PositionLeft != 0 {
		if buf.Len() != 0 {
			buf.WriteRune(',')
		}
		buf.WriteString("left")
		e &= ^PositionLeft
	}
	if e&PositionRight != 0 {
		if buf.Len() != 0 {
			buf.WriteRune(',')
		}
		buf.WriteString("right")
		e &= ^PositionRight
	}
	if e != PositionNone {
		panic("invalid position")
	}
	return buf.String()
}

// MarshalText encodes the position when used as a map key
func (e Position) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

// UnmarshalText decodes a position when used as a map key
func (e *Position) UnmarshalText(text []byte) error {
	switch s := strings.ToLower(string(text)); s {
	default:
		*e = PositionNone
		for _, p := range strings.Split(s, ",") {
			switch p {
			case "left":
				*e |= PositionLeft
			case "middle":
				*e |= PositionMiddle
			case "right":
				*e |= PositionRight
			default:
				return fmt.Errorf("unknown position: %s", e)
			}
		}
	case "none":
		*e = PositionNone
	case "all":
		*e = PositionAll
	}
	return nil
}

func (e Presence) String() string {
	switch e {
	case Absent:
		return "absent"
	case Present:
		return "present"
	case Renamed:
		return "renamed"
	case Multiple:
		return "multiple"
	default:
		panic("invalid presence")
	}
}

// MarshalJSON encodes the presence when used as a JSON value
func (e Presence) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

// UnmarshalJSON decodes a presence when used as a JSON value
func (e *Presence) UnmarshalJSON(text []byte) error {
	var s string
	if err := json.Unmarshal(text, &s); err != nil {
		return err
	}
	switch s := strings.ToLower(s); s {
	case "absent":
		*e = Absent
	case "present":
		*e = Present
	case "renamed":
		*e = Renamed
	case "multiple":
		*e = Multiple
	default:
		return fmt.Errorf("unknown presence: %s", e)
	}
	return nil
}
