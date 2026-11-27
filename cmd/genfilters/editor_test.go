//go:build !plan9 && !js && !aix

package genfilters

import (
	"testing"

	"github.com/gdamore/tcell/v3"
	"github.com/rclone/rclone/lib/tui"
	"github.com/stretchr/testify/assert"
)

func TestEditor(t *testing.T) {
	var e editor
	e.setText(nil)
	assert.Equal(t, []string{""}, e.text())

	key := func(keys ...any) {
		for _, k := range keys {
			switch k := k.(type) {
			case tcell.Key:
				e.key(tui.Key(k), 10)
			case string:
				for _, r := range k {
					e.key(r, 10)
				}
			}
		}
	}
	key("ab", tcell.KeyEnter, "日d")
	assert.Equal(t, []string{"ab", "日d"}, e.text())

	// backspace at the start of a line joins it to the line before
	key(tcell.KeyHome, tcell.KeyBackspace2)
	assert.Equal(t, []string{"ab日d"}, e.text())
	assert.Equal(t, 2, e.col)

	// delete at the end of a line joins the next line to it
	key(tcell.KeyEnd, tcell.KeyEnter, "x", tcell.KeyUp, tcell.KeyEnd, tcell.KeyDelete)
	assert.Equal(t, []string{"ab日dx"}, e.text())

	// left and right wrap between lines
	key(tcell.KeyDelete, tcell.KeyEnter, tcell.KeyLeft)
	assert.Equal(t, []string{"ab日d", ""}, e.text())
	assert.Equal(t, 0, e.row)
	assert.Equal(t, 4, e.col)
	key(tcell.KeyRight)
	assert.Equal(t, 1, e.row)

	// nothing happens past the ends or for control characters
	key(tcell.KeyCtrlB)
	assert.Equal(t, []string{"ab日d", ""}, e.text())
	e.setText([]string{"a"})
	key(tcell.KeyHome, tcell.KeyBackspace2, tcell.KeyEnd, tcell.KeyDelete, tcell.KeyPgDn, tcell.KeyPgUp)
	assert.Equal(t, []string{"a"}, e.text())

	// the cursor stays in the text when it is replaced
	e.setText([]string{"one", "two", "three"})
	key(tcell.KeyPgDn, tcell.KeyEnd)
	e.setText([]string{"x"})
	assert.Equal(t, 0, e.row)
	assert.Equal(t, 1, e.col)
}
