// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package ansiterm

import (
	"fmt"
	"io"
)

// Context provides a way to specify both foreground and background colors
// along with other styles and write text to a Writer with those colors and
// styles.
type Context struct {
	Foreground Color
	Background Color
	Styles     []Style
}

// Foreground is a convenience function that creates a Context with the
// specified color as the foreground color.
func Foreground(color Color) *Context {
	return &Context{Foreground: color}
}

// Background is a convenience function that creates a Context with the
// specified color as the background color.
func Background(color Color) *Context {
	return &Context{Background: color}
}

// Styles is a convenience function that creates a Context with the
// specified styles set.
func Styles(styles ...Style) *Context {
	return &Context{Styles: styles}
}

// SetForeground sets the foreground to the specified color.
func (c *Context) SetForeground(color Color) *Context {
	c.Foreground = color
	return c
}

// SetBackground sets the background to the specified color.
func (c *Context) SetBackground(color Color) *Context {
	c.Background = color
	return c
}

// SetStyle replaces the styles with the new values.
func (c *Context) SetStyle(styles ...Style) *Context {
	c.Styles = styles
	return c
}

type sgrWriter interface {
	io.Writer
	writeSGR(value sgr)
}

// Fprintf will set the sgr values of the writer to the specified
// foreground, background and styles, then write the formatted string,
// then reset the writer.
func (c *Context) Fprintf(w sgrWriter, format string, args ...interface{}) {
	w.writeSGR(c)
	fmt.Fprintf(w, format, args...)
	w.writeSGR(reset)
}

// Fprint will set the sgr values of the writer to the specified foreground,
// background and styles, then formats using the default formats for its
// operands and writes to w. Spaces are added between operands when neither is
// a string. It returns the number of bytes written and any write error
// encountered.
func (c *Context) Fprint(w sgrWriter, args ...interface{}) {
	w.writeSGR(c)
	fmt.Fprint(w, args...)
	w.writeSGR(reset)
}

func (c *Context) sgr() string {
	var values attributes
	if foreground := c.Foreground.foreground(); foreground != unknownAttribute {
		values = append(values, foreground)
	}
	if background := c.Background.background(); background != unknownAttribute {
		values = append(values, background)
	}
	for _, style := range c.Styles {
		if value := style.enable(); value != unknownAttribute {
			values = append(values, value)
		}
	}
	return values.sgr()
}
