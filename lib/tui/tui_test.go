//go:build !plan9 && !js && !aix

package tui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/vt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockScreen(t *testing.T, w, h int) (vt.MockTerm, tcell.Screen) {
	mt := vt.NewMockTerm(vt.MockOptSize{X: vt.Col(w), Y: vt.Row(h)})
	s, err := tcell.NewTerminfoScreenFromTty(mt)
	require.NoError(t, err)
	require.NoError(t, s.Init())
	t.Cleanup(s.Fini)
	return mt, s
}

// row returns the text shown on row y of mt
func row(mt vt.MockTerm, w, y int) string {
	var b strings.Builder
	for x := range w {
		c := mt.GetCell(vt.Coord{X: vt.Col(x), Y: vt.Row(y)}).C
		if c == "" {
			c = " "
		}
		b.WriteString(c)
	}
	return b.String()
}

func TestLine(t *testing.T) {
	mt, s := newMockScreen(t, 20, 3)
	Line(s, 0, 0, 10, tcell.StyleDefault, '-', "abc")
	Line(s, 0, 1, 4, tcell.StyleDefault, '-', "abcdefgh")
	Linef(s, 2, 2, 12, tcell.StyleDefault, '.', "%d-%s", 7, "日本")
	s.Show()
	assert.Equal(t, "abc-------          ", row(mt, 20, 0))
	assert.Equal(t, "abcd                ", row(mt, 20, 1))
	// wide characters take two cells, the second showing as a space here
	assert.Equal(t, "  7-日 本 ....        ", row(mt, 20, 2))
}

func TestBoxDraw(t *testing.T) {
	const w, h = 30, 9
	mt, s := newMockScreen(t, w, h)
	b := Box{}
	b.Popup([]string{"Title", "some text"})
	b.Menu = []string{"no", "yes"}
	b.Move(1)
	b.Draw(s)
	s.Show()

	var screen []string
	for y := range h {
		screen = append(screen, strings.TrimRight(row(mt, w, y), " "))
	}
	assert.Equal(t, []string{
		"",
		"",
		"         ┌─────────────┐",
		"         │Title        │",
		"         │some text    │",
		"         │ <no>  <yes> │",
		"         └─────────────┘",
		"",
		"",
	}, screen)
}

func TestBoxMove(t *testing.T) {
	b := Box{}
	b.Move(1)
	assert.Equal(t, 0, b.Button, "no menu")
	b.Menu = []string{"a", "b", "c"}
	b.Move(1)
	b.Move(1)
	b.Move(1)
	assert.Equal(t, 2, b.Button)
	b.Move(-1)
	b.Move(-1)
	b.Move(-1)
	assert.Equal(t, 0, b.Button)
}

func TestBoxToggle(t *testing.T) {
	b := Box{}
	b.Toggle([]string{"help"})
	assert.True(t, b.Show)
	b.Toggle([]string{"other"})
	assert.True(t, b.Show)
	assert.Equal(t, []string{"other"}, b.Text)
	b.Toggle([]string{"other"})
	assert.False(t, b.Show)
}

func TestEventRune(t *testing.T) {
	assert.Equal(t, 'x', EventRune(tcell.NewEventKey(tcell.KeyRune, "x", tcell.ModNone)))
	assert.Equal(t, '日', EventRune(tcell.NewEventKey(tcell.KeyRune, "日", tcell.ModNone)))
	assert.Equal(t, Key(tcell.KeyEnter), EventRune(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)))
	assert.Negative(t, Key(tcell.KeyEnter))
}
