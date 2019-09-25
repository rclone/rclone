// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package ansiterm

const (
	_ Color = iota
	Default
	Black
	Red
	Green
	Yellow
	Blue
	Magenta
	Cyan
	Gray
	DarkGray
	BrightRed
	BrightGreen
	BrightYellow
	BrightBlue
	BrightMagenta
	BrightCyan
	White
)

// Color represents one of the standard 16 ANSI colors.
type Color int

// String returns the name of the color.
func (c Color) String() string {
	switch c {
	case Default:
		return "default"
	case Black:
		return "black"
	case Red:
		return "red"
	case Green:
		return "green"
	case Yellow:
		return "yellow"
	case Blue:
		return "blue"
	case Magenta:
		return "magenta"
	case Cyan:
		return "cyan"
	case Gray:
		return "gray"
	case DarkGray:
		return "darkgray"
	case BrightRed:
		return "brightred"
	case BrightGreen:
		return "brightgreen"
	case BrightYellow:
		return "brightyellow"
	case BrightBlue:
		return "brightblue"
	case BrightMagenta:
		return "brightmagenta"
	case BrightCyan:
		return "brightcyan"
	case White:
		return "white"
	default:
		return ""
	}
}

func (c Color) foreground() attribute {
	switch c {
	case Default:
		return 39
	case Black:
		return 30
	case Red:
		return 31
	case Green:
		return 32
	case Yellow:
		return 33
	case Blue:
		return 34
	case Magenta:
		return 35
	case Cyan:
		return 36
	case Gray:
		return 37
	case DarkGray:
		return 90
	case BrightRed:
		return 91
	case BrightGreen:
		return 92
	case BrightYellow:
		return 93
	case BrightBlue:
		return 94
	case BrightMagenta:
		return 95
	case BrightCyan:
		return 96
	case White:
		return 97
	default:
		return unknownAttribute
	}
}

func (c Color) background() attribute {
	value := c.foreground()
	if value != unknownAttribute {
		return value + 10
	}
	return value
}
