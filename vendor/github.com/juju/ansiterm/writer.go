// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package ansiterm

import (
	"fmt"
	"io"
)

// Writer allows colors and styles to be specified. If the io.Writer
// is not a terminal capable of color, all attempts to set colors or
// styles are no-ops.
type Writer struct {
	io.Writer

	noColor bool
}

// NewWriter returns a Writer that allows the caller to specify colors and
// styles. If the io.Writer is not a terminal capable of color, all attempts
// to set colors or styles are no-ops.
func NewWriter(w io.Writer) *Writer {
	writer, colorCapable := colorEnabledWriter(w)
	return &Writer{
		Writer:  writer,
		noColor: !colorCapable,
	}
}

// SetColorCapable forces the writer to either write the ANSI escape color
// if capable is true, or to not write them if capable is false.
func (w *Writer) SetColorCapable(capable bool) {
	w.noColor = !capable
}

// SetForeground sets the foreground color.
func (w *Writer) SetForeground(c Color) {
	w.writeSGR(c.foreground())
}

// SetBackground sets the background color.
func (w *Writer) SetBackground(c Color) {
	w.writeSGR(c.background())
}

// SetStyle sets the text style.
func (w *Writer) SetStyle(s Style) {
	w.writeSGR(s.enable())
}

// ClearStyle clears the text style.
func (w *Writer) ClearStyle(s Style) {
	w.writeSGR(s.disable())
}

// Reset returns the default foreground and background colors with no styles.
func (w *Writer) Reset() {
	w.writeSGR(reset)
}

type sgr interface {
	// sgr returns the combined escape sequence for the Select Graphic Rendition.
	sgr() string
}

// writeSGR takes the appropriate integer SGR parameters
// and writes out the ANIS escape code.
func (w *Writer) writeSGR(value sgr) {
	if w.noColor {
		return
	}
	fmt.Fprint(w, value.sgr())
}
