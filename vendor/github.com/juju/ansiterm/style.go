// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package ansiterm

const (
	_ Style = iota
	Bold
	Faint
	Italic
	Underline
	Blink
	Reverse
	Strikethrough
	Conceal
)

type Style int

func (s Style) String() string {
	switch s {
	case Bold:
		return "bold"
	case Faint:
		return "faint"
	case Italic:
		return "italic"
	case Underline:
		return "underline"
	case Blink:
		return "blink"
	case Reverse:
		return "reverse"
	case Strikethrough:
		return "strikethrough"
	case Conceal:
		return "conceal"
	default:
		return ""
	}
}

func (s Style) enable() attribute {
	switch s {
	case Bold:
		return 1
	case Faint:
		return 2
	case Italic:
		return 3
	case Underline:
		return 4
	case Blink:
		return 5
	case Reverse:
		return 7
	case Conceal:
		return 8
	case Strikethrough:
		return 9
	default:
		return unknownAttribute
	}
}

func (s Style) disable() attribute {
	value := s.enable()
	if value != unknownAttribute {
		return value + 20
	}
	return value
}
