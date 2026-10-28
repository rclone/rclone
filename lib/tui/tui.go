//go:build !plan9 && !js && !aix

// Package tui provides drawing helpers for rclone's text based user interfaces
package tui

import (
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"unicode/utf8"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
	runewidth "github.com/mattn/go-runewidth"
	"github.com/rclone/rclone/fs/log"
	"github.com/rivo/uniseg"
)

// graphemeWidth returns the number of cells in rs.
//
// The original [runewidth.StringWidth] iterates through graphemes
// and uses this same logic. To avoid iterating through graphemes
// repeatedly, we separate that out into its own function.
func graphemeWidth(rs []rune) (wd int) {
	// copied/adapted from [runewidth.StringWidth]
	for _, r := range rs {
		wd = runewidth.RuneWidth(r)
		if wd > 0 {
			break
		}
	}
	return wd
}

// Print a string
func Print(s tcell.Screen, x, y int, style tcell.Style, msg string) {
	g := uniseg.NewGraphemes(msg)
	for g.Next() {
		rs := g.Runes()
		s.SetContent(x, y, rs[0], rs[1:], style)
		x += graphemeWidth(rs)
	}
}

// Printf a string
func Printf(s tcell.Screen, x, y int, style tcell.Style, format string, args ...any) {
	Print(s, x, y, style, fmt.Sprintf(format, args...))
}

// Line prints a string to given xmax, with given space
func Line(s tcell.Screen, x, y, xmax int, style tcell.Style, spacer rune, msg string) {
	g := uniseg.NewGraphemes(msg)
	for g.Next() {
		rs := g.Runes()
		s.SetContent(x, y, rs[0], rs[1:], style)
		x += graphemeWidth(rs)
		if x >= xmax {
			return
		}
	}
	for ; x < xmax; x++ {
		s.SetContent(x, y, spacer, nil, style)
	}
}

// Linef a string
func Linef(s tcell.Screen, x, y, xmax int, style tcell.Style, spacer rune, format string, args ...any) {
	Line(s, x, y, xmax, style, spacer, fmt.Sprintf(format, args...))
}

// LineOptions Print line of selectable options
func LineOptions(s tcell.Screen, x, y, xmax int, style tcell.Style, options []string, selected int) {
	for x := x; x < xmax; x++ {
		s.SetContent(x, y, ' ', nil, style) // fill
	}
	x += ((xmax - x) - lineOptionLength(options)) / 2 // center

	for i, o := range options {
		s.SetContent(x, y, ' ', nil, style)
		x++

		ostyle := style
		if i == selected {
			ostyle = tcell.StyleDefault
		}

		s.SetContent(x, y, '<', nil, ostyle)
		x++

		g := uniseg.NewGraphemes(o)
		for g.Next() {
			rs := g.Runes()
			s.SetContent(x, y, rs[0], rs[1:], ostyle)
			x += graphemeWidth(rs)
		}

		s.SetContent(x, y, '>', nil, ostyle)
		x++

		s.SetContent(x, y, ' ', nil, style)
		x++
	}
}

func lineOptionLength(o []string) int {
	count := 0
	for _, i := range o {
		count += len(i)
	}
	return count + 4*len(o) // spacer and arrows <entry>
}

// Box is a popup box of text with an optional menu of options
type Box struct {
	Show   bool     // whether to show the box
	Text   []string // text to show in the box
	Menu   []string // menu options
	Button int      // selected menu option
}

// Draw the box centered on the screen
func (b *Box) Draw(s tcell.Screen) {
	w, h := s.Size()

	// Find dimensions of text
	boxWidth := 10
	for _, t := range b.Text {
		if len(t) > boxWidth && len(t) < w-4 {
			boxWidth = len(t)
		}
	}
	boxHeight := len(b.Text)

	// position
	x := (w - boxWidth) / 2
	y := (h - boxHeight) / 2
	xmax := x + boxWidth
	if len(b.Menu) != 0 {
		count := lineOptionLength(b.Menu)
		xmax = max(x+boxWidth, x+count)
	}
	ymax := y + len(b.Text)

	// draw text
	style := tcell.StyleDefault.Background(color.Red).Reverse(true)
	for i, t := range b.Text {
		Line(s, x, y+i, xmax, style, ' ', t)
		style = tcell.StyleDefault.Reverse(true)
	}

	if len(b.Menu) != 0 {
		LineOptions(s, x, ymax, xmax, style, b.Menu, b.Button)
		ymax++
	}

	// draw top border
	for i := y; i < ymax; i++ {
		s.SetContent(x-1, i, tcell.RuneVLine, nil, style)
		s.SetContent(xmax, i, tcell.RuneVLine, nil, style)
	}
	for j := x; j < xmax; j++ {
		s.SetContent(j, y-1, tcell.RuneHLine, nil, style)
		s.SetContent(j, ymax, tcell.RuneHLine, nil, style)
	}

	s.SetContent(x-1, y-1, tcell.RuneULCorner, nil, style)
	s.SetContent(xmax, y-1, tcell.RuneURCorner, nil, style)
	s.SetContent(x-1, ymax, tcell.RuneLLCorner, nil, style)
	s.SetContent(xmax, ymax, tcell.RuneLRCorner, nil, style)
}

// Move the selected menu option left (to < 0) or right (to > 0)
func (b *Box) Move(to int) {
	if len(b.Menu) == 0 {
		return
	}

	if to > 0 { // move right
		b.Button++
	} else { // move left
		b.Button--
	}

	if b.Button >= len(b.Menu) {
		b.Button = len(b.Menu) - 1
	} else if b.Button < 0 {
		b.Button = 0
	}
}

// Popup shows the box with the text in
func (b *Box) Popup(text []string) {
	b.Text = text
	b.Show = true
}

// Toggle shows the box with the text in, or hides it if it is already
// showing that text
func (b *Box) Toggle(text []string) {
	if b.Show && reflect.DeepEqual(b.Text, text) {
		b.Show = false
	} else {
		b.Popup(text)
	}
}

// Key returns a rune representing the key k. It is a negative value, to not collide with Unicode code-points.
func Key(k tcell.Key) rune {
	return rune(-k)
}

// EventRune returns the rune typed for ev, or [Key] of its key if it
// isn't a rune
func EventRune(ev *tcell.EventKey) rune {
	if k := ev.Key(); k != tcell.KeyRune {
		return Key(k)
	}
	c, _ := utf8.DecodeRuneInString(ev.Str())
	return c
}

// RedirectLogs holds back log output so that it doesn't corrupt the
// screen. The returned function writes the last 100 lines held back
// to stderr and must be called after the screen is finished.
func RedirectLogs() (restore func()) {
	if log.Redirected() {
		return func() {}
	}
	var logs []string
	log.Handler.SetOutput(func(level slog.Level, text string) {
		if len(logs) > 100 {
			logs = logs[len(logs)-100:]
		}
		logs = append(logs, text)
	})
	return func() {
		log.Handler.ResetOutput()
		for _, text := range logs {
			_, _ = os.Stderr.WriteString(text)
		}
	}
}
