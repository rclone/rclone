// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package ansiterm

import (
	"io"

	"github.com/juju/ansiterm/tabwriter"
)

// NewTabWriter returns a writer that is able to set colors and styels.
// The ansi escape codes are stripped for width calculations.
func NewTabWriter(output io.Writer, minwidth, tabwidth, padding int, padchar byte, flags uint) *TabWriter {
	return new(TabWriter).Init(output, minwidth, tabwidth, padding, padchar, flags)
}

// TabWriter is a filter that inserts padding around tab-delimited
// columns in its input to align them in the output.
//
// It also setting of colors and styles over and above the standard
// tabwriter package.
type TabWriter struct {
	Writer
	tw tabwriter.Writer
}

// Flush should be called after the last call to Write to ensure
// that any data buffered in the Writer is written to output. Any
// incomplete escape sequence at the end is considered
// complete for formatting purposes.
//
func (t *TabWriter) Flush() error {
	return t.tw.Flush()
}

// SetColumnAlignRight will mark a particular column as align right.
// This is reset on the next flush.
func (t *TabWriter) SetColumnAlignRight(column int) {
	t.tw.SetColumnAlignRight(column)
}

// A Writer must be initialized with a call to Init. The first parameter (output)
// specifies the filter output. The remaining parameters control the formatting:
//
//	minwidth	minimal cell width including any padding
//	tabwidth	width of tab characters (equivalent number of spaces)
//	padding		padding added to a cell before computing its width
//	padchar		ASCII char used for padding
//			if padchar == '\t', the Writer will assume that the
//			width of a '\t' in the formatted output is tabwidth,
//			and cells are left-aligned independent of align_left
//			(for correct-looking results, tabwidth must correspond
//			to the tab width in the viewer displaying the result)
//	flags		formatting control
//
func (t *TabWriter) Init(output io.Writer, minwidth, tabwidth, padding int, padchar byte, flags uint) *TabWriter {
	writer, colorCapable := colorEnabledWriter(output)
	t.Writer = Writer{
		Writer:  t.tw.Init(writer, minwidth, tabwidth, padding, padchar, flags),
		noColor: !colorCapable,
	}
	return t
}
