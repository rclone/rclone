//go:build !plan9 && !js && !aix

package genfilters

import (
	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
	runewidth "github.com/mattn/go-runewidth"
	"github.com/rclone/rclone/lib/tui"
)

// editor is a text editor for the rules
type editor struct {
	lines   [][]rune
	row     int // cursor line
	col     int // cursor position in the line, in runes
	top     int // first line shown
	left    int // first cell shown
	errLine int // line with an error, counting from 1, or 0 for none
}

// setText replaces the text, keeping the cursor in range
func (e *editor) setText(lines []string) {
	e.lines = make([][]rune, 0, len(lines)+1)
	for _, line := range lines {
		e.lines = append(e.lines, []rune(line))
	}
	if len(e.lines) == 0 {
		e.lines = append(e.lines, nil)
	}
	e.errLine = 0
	e.clamp()
}

// text returns the text as lines
func (e *editor) text() []string {
	lines := make([]string, len(e.lines))
	for i, line := range e.lines {
		lines[i] = string(line)
	}
	return lines
}

// clamp keeps the cursor inside the text
func (e *editor) clamp() {
	e.row = max(0, min(e.row, len(e.lines)-1))
	e.col = max(0, min(e.col, len(e.lines[e.row])))
}

// insert types r at the cursor
func (e *editor) insert(r rune) {
	line := e.lines[e.row]
	line = append(line[:e.col], append([]rune{r}, line[e.col:]...)...)
	e.lines[e.row] = line
	e.col++
}

// newline splits the line at the cursor
func (e *editor) newline() {
	line := e.lines[e.row]
	rest := append([]rune(nil), line[e.col:]...)
	e.lines[e.row] = line[:e.col]
	e.lines = append(e.lines[:e.row+1], append([][]rune{rest}, e.lines[e.row+1:]...)...)
	e.row++
	e.col = 0
}

// backspace deletes before the cursor, joining lines at the start of one
func (e *editor) backspace() {
	if e.col > 0 {
		line := e.lines[e.row]
		e.lines[e.row] = append(line[:e.col-1], line[e.col:]...)
		e.col--
		return
	}
	if e.row == 0 {
		return
	}
	prev := e.lines[e.row-1]
	e.col = len(prev)
	e.lines[e.row-1] = append(prev, e.lines[e.row]...)
	e.lines = append(e.lines[:e.row], e.lines[e.row+1:]...)
	e.row--
}

// del deletes at the cursor, joining lines at the end of one
func (e *editor) del() {
	line := e.lines[e.row]
	if e.col < len(line) {
		e.lines[e.row] = append(line[:e.col], line[e.col+1:]...)
		return
	}
	if e.row == len(e.lines)-1 {
		return
	}
	e.lines[e.row] = append(line, e.lines[e.row+1]...)
	e.lines = append(e.lines[:e.row+1], e.lines[e.row+2:]...)
}

// key handles a key press, returning whether the text changed
func (e *editor) key(c rune, height int) (changed bool) {
	switch c {
	case tui.Key(tcell.KeyUp):
		e.row--
	case tui.Key(tcell.KeyDown):
		e.row++
	case tui.Key(tcell.KeyLeft):
		if e.col == 0 && e.row > 0 {
			e.row--
			e.col = len(e.lines[e.row])
		} else {
			e.col--
		}
	case tui.Key(tcell.KeyRight):
		if e.col == len(e.lines[e.row]) && e.row < len(e.lines)-1 {
			e.row++
			e.col = 0
		} else {
			e.col++
		}
	case tui.Key(tcell.KeyHome), tui.Key(tcell.KeyCtrlA):
		e.col = 0
	case tui.Key(tcell.KeyEnd), tui.Key(tcell.KeyCtrlE):
		e.col = len(e.lines[e.row])
	case tui.Key(tcell.KeyPgUp):
		e.row -= height
	case tui.Key(tcell.KeyPgDn):
		e.row += height
	case tui.Key(tcell.KeyEnter):
		e.newline()
		changed = true
	case tui.Key(tcell.KeyBackspace), tui.Key(tcell.KeyBackspace2):
		e.backspace()
		changed = true
	case tui.Key(tcell.KeyDelete):
		e.del()
		changed = true
	default:
		if c < ' ' {
			return false
		}
		e.insert(c)
		changed = true
	}
	e.clamp()
	return changed
}

// draw shows the text in the box at x, y of size w, h, with the
// cursor if focused
func (e *editor) draw(s tcell.Screen, x, y, w, h int, focused bool) {
	// scroll so the cursor is visible
	if e.row < e.top {
		e.top = e.row
	} else if e.row >= e.top+h {
		e.top = e.row - h + 1
	}
	cursorX := runewidth.StringWidth(string(e.lines[e.row][:e.col]))
	if cursorX < e.left {
		e.left = cursorX
	} else if cursorX >= e.left+w {
		e.left = cursorX - w + 1
	}

	for i := range h {
		n := e.top + i
		if n >= len(e.lines) {
			tui.Line(s, x, y+i, x+w, tcell.StyleDefault, ' ', "")
			continue
		}
		style := tcell.StyleDefault
		if n+1 == e.errLine {
			style = style.Foreground(color.Red).Reverse(true)
		}
		// drop the cells scrolled off the left
		line, skip := e.lines[n], 0
		for len(line) > 0 && skip+runewidth.RuneWidth(line[0]) <= e.left {
			skip += runewidth.RuneWidth(line[0])
			line = line[1:]
		}
		tui.Line(s, x, y+i, x+w, style, ' ', string(line))
	}
	if focused {
		s.ShowCursor(x+cursorX-e.left, y+e.row-e.top)
	}
}
