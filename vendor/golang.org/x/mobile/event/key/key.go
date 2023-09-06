// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate stringer -type=Code

// Package key defines an event for physical keyboard keys.
//
// On-screen software keyboards do not send key events.
//
// See the golang.org/x/mobile/app package for details on the event model.
package key

import (
	"fmt"
	"strings"
)

// Event is a key event.
type Event struct {
	// Rune is the meaning of the key event as determined by the
	// operating system. The mapping is determined by system-dependent
	// current layout, modifiers, lock-states, etc.
	//
	// If non-negative, it is a Unicode codepoint: pressing the 'a' key
	// generates different Runes 'a' or 'A' (but the same Code) depending on
	// the state of the shift key.
	//
	// If -1, the key does not generate a Unicode codepoint. To distinguish
	// them, look at Code.
	Rune rune

	// Code is the identity of the physical key relative to a notional
	// "standard" keyboard, independent of current layout, modifiers,
	// lock-states, etc
	//
	// For standard key codes, its value matches USB HID key codes.
	// Compare its value to uint32-typed constants in this package, such
	// as CodeLeftShift and CodeEscape.
	//
	// Pressing the regular '2' key and number-pad '2' key (with Num-Lock)
	// generate different Codes (but the same Rune).
	Code Code

	// Modifiers is a bitmask representing a set of modifier keys: ModShift,
	// ModAlt, etc.
	Modifiers Modifiers

	// Direction is the direction of the key event: DirPress, DirRelease,
	// or DirNone (for key repeats).
	Direction Direction

	// TODO: add a Device ID, for multiple input devices?
	// TODO: add a time.Time?
}

func (e Event) String() string {
	if e.Rune >= 0 {
		return fmt.Sprintf("key.Event{%q (%v), %v, %v}", e.Rune, e.Code, e.Modifiers, e.Direction)
	}
	return fmt.Sprintf("key.Event{(%v), %v, %v}", e.Code, e.Modifiers, e.Direction)
}

// Direction is the direction of the key event.
type Direction uint8

const (
	DirNone    Direction = 0
	DirPress   Direction = 1
	DirRelease Direction = 2
)

// Modifiers is a bitmask representing a set of modifier keys.
type Modifiers uint32

const (
	ModShift   Modifiers = 1 << 0
	ModControl Modifiers = 1 << 1
	ModAlt     Modifiers = 1 << 2
	ModMeta    Modifiers = 1 << 3 // called "Command" on OS X
)

// Code is the identity of a key relative to a notional "standard" keyboard.
type Code uint32

// Physical key codes.
//
// For standard key codes, its value matches USB HID key codes.
// TODO: add missing codes.
const (
	CodeUnknown Code = 0

	CodeA Code = 4
	CodeB Code = 5
	CodeC Code = 6
	CodeD Code = 7
	CodeE Code = 8
	CodeF Code = 9
	CodeG Code = 10
	CodeH Code = 11
	CodeI Code = 12
	CodeJ Code = 13
	CodeK Code = 14
	CodeL Code = 15
	CodeM Code = 16
	CodeN Code = 17
	CodeO Code = 18
	CodeP Code = 19
	CodeQ Code = 20
	CodeR Code = 21
	CodeS Code = 22
	CodeT Code = 23
	CodeU Code = 24
	CodeV Code = 25
	CodeW Code = 26
	CodeX Code = 27
	CodeY Code = 28
	CodeZ Code = 29

	Code1 Code = 30
	Code2 Code = 31
	Code3 Code = 32
	Code4 Code = 33
	Code5 Code = 34
	Code6 Code = 35
	Code7 Code = 36
	Code8 Code = 37
	Code9 Code = 38
	Code0 Code = 39

	CodeReturnEnter        Code = 40
	CodeEscape             Code = 41
	CodeDeleteBackspace    Code = 42
	CodeTab                Code = 43
	CodeSpacebar           Code = 44
	CodeHyphenMinus        Code = 45 // -
	CodeEqualSign          Code = 46 // =
	CodeLeftSquareBracket  Code = 47 // [
	CodeRightSquareBracket Code = 48 // ]
	CodeBackslash          Code = 49 // \
	CodeSemicolon          Code = 51 // ;
	CodeApostrophe         Code = 52 // '
	CodeGraveAccent        Code = 53 // `
	CodeComma              Code = 54 // ,
	CodeFullStop           Code = 55 // .
	CodeSlash              Code = 56 // /
	CodeCapsLock           Code = 57

	CodeF1  Code = 58
	CodeF2  Code = 59
	CodeF3  Code = 60
	CodeF4  Code = 61
	CodeF5  Code = 62
	CodeF6  Code = 63
	CodeF7  Code = 64
	CodeF8  Code = 65
	CodeF9  Code = 66
	CodeF10 Code = 67
	CodeF11 Code = 68
	CodeF12 Code = 69

	CodePause         Code = 72
	CodeInsert        Code = 73
	CodeHome          Code = 74
	CodePageUp        Code = 75
	CodeDeleteForward Code = 76
	CodeEnd           Code = 77
	CodePageDown      Code = 78

	CodeRightArrow Code = 79
	CodeLeftArrow  Code = 80
	CodeDownArrow  Code = 81
	CodeUpArrow    Code = 82

	CodeKeypadNumLock     Code = 83
	CodeKeypadSlash       Code = 84 // /
	CodeKeypadAsterisk    Code = 85 // *
	CodeKeypadHyphenMinus Code = 86 // -
	CodeKeypadPlusSign    Code = 87 // +
	CodeKeypadEnter       Code = 88
	CodeKeypad1           Code = 89
	CodeKeypad2           Code = 90
	CodeKeypad3           Code = 91
	CodeKeypad4           Code = 92
	CodeKeypad5           Code = 93
	CodeKeypad6           Code = 94
	CodeKeypad7           Code = 95
	CodeKeypad8           Code = 96
	CodeKeypad9           Code = 97
	CodeKeypad0           Code = 98
	CodeKeypadFullStop    Code = 99  // .
	CodeKeypadEqualSign   Code = 103 // =

	CodeF13 Code = 104
	CodeF14 Code = 105
	CodeF15 Code = 106
	CodeF16 Code = 107
	CodeF17 Code = 108
	CodeF18 Code = 109
	CodeF19 Code = 110
	CodeF20 Code = 111
	CodeF21 Code = 112
	CodeF22 Code = 113
	CodeF23 Code = 114
	CodeF24 Code = 115

	CodeHelp Code = 117

	CodeMute       Code = 127
	CodeVolumeUp   Code = 128
	CodeVolumeDown Code = 129

	CodeLeftControl  Code = 224
	CodeLeftShift    Code = 225
	CodeLeftAlt      Code = 226
	CodeLeftGUI      Code = 227
	CodeRightControl Code = 228
	CodeRightShift   Code = 229
	CodeRightAlt     Code = 230
	CodeRightGUI     Code = 231

	// The following codes are not part of the standard USB HID Usage IDs for
	// keyboards. See http://www.usb.org/developers/hidpage/Hut1_12v2.pdf
	//
	// Usage IDs are uint16s, so these non-standard values start at 0x10000.

	// CodeCompose is the Code for a compose key, sometimes called a multi key,
	// used to input non-ASCII characters such as ñ being composed of n and ~.
	//
	// See https://en.wikipedia.org/wiki/Compose_key
	CodeCompose Code = 0x10000
)

// TODO: Given we use runes outside the unicode space, should we provide a
// printing function? Related: it's a little unfortunate that printing a
// key.Event with %v gives not very readable output like:
//	{100 7 key.Modifiers() Press}

var mods = [...]struct {
	m Modifiers
	s string
}{
	{ModShift, "Shift"},
	{ModControl, "Control"},
	{ModAlt, "Alt"},
	{ModMeta, "Meta"},
}

func (m Modifiers) String() string {
	var match []string
	for _, mod := range mods {
		if mod.m&m != 0 {
			match = append(match, mod.s)
		}
	}
	return "key.Modifiers(" + strings.Join(match, "|") + ")"
}

func (d Direction) String() string {
	switch d {
	case DirNone:
		return "None"
	case DirPress:
		return "Press"
	case DirRelease:
		return "Release"
	default:
		return fmt.Sprintf("key.Direction(%d)", d)
	}
}
