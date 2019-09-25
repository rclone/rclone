
# ansiterm
    import "github.com/juju/ansiterm"

Package ansiterm provides a Writer that writes out the ANSI escape
codes for color and styles.







## type Color
``` go
type Color int
```
Color represents one of the standard 16 ANSI colors.



``` go
const (
    Default Color
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
```








### func (Color) String
``` go
func (c Color) String() string
```
String returns the name of the color.



## type Context
``` go
type Context struct {
    Foreground Color
    Background Color
    Styles     []Style
}
```
Context provides a way to specify both foreground and background colors
along with other styles and write text to a Writer with those colors and
styles.









### func Background
``` go
func Background(color Color) *Context
```
Background is a convenience function that creates a Context with the
specified color as the background color.


### func Foreground
``` go
func Foreground(color Color) *Context
```
Foreground is a convenience function that creates a Context with the
specified color as the foreground color.


### func Styles
``` go
func Styles(styles ...Style) *Context
```
Styles is a convenience function that creates a Context with the
specified styles set.




### func (\*Context) Fprint
``` go
func (c *Context) Fprint(w sgrWriter, args ...interface{})
```
Fprint will set the sgr values of the writer to the specified foreground,
background and styles, then formats using the default formats for its
operands and writes to w. Spaces are added between operands when neither is
a string. It returns the number of bytes written and any write error
encountered.



### func (\*Context) Fprintf
``` go
func (c *Context) Fprintf(w sgrWriter, format string, args ...interface{})
```
Fprintf will set the sgr values of the writer to the specified
foreground, background and styles, then write the formatted string,
then reset the writer.



### func (\*Context) SetBackground
``` go
func (c *Context) SetBackground(color Color) *Context
```
SetBackground sets the background to the specified color.



### func (\*Context) SetForeground
``` go
func (c *Context) SetForeground(color Color) *Context
```
SetForeground sets the foreground to the specified color.



### func (\*Context) SetStyle
``` go
func (c *Context) SetStyle(styles ...Style) *Context
```
SetStyle replaces the styles with the new values.



## type Style
``` go
type Style int
```


``` go
const (
    Bold Style
    Faint
    Italic
    Underline
    Blink
    Reverse
    Strikethrough
    Conceal
)
```








### func (Style) String
``` go
func (s Style) String() string
```


## type TabWriter
``` go
type TabWriter struct {
    Writer
    // contains filtered or unexported fields
}
```
TabWriter is a filter that inserts padding around tab-delimited
columns in its input to align them in the output.

It also setting of colors and styles over and above the standard
tabwriter package.









### func NewTabWriter
``` go
func NewTabWriter(output io.Writer, minwidth, tabwidth, padding int, padchar byte, flags uint) *TabWriter
```
NewTabWriter returns a writer that is able to set colors and styels.
The ansi escape codes are stripped for width calculations.




### func (\*TabWriter) Flush
``` go
func (t *TabWriter) Flush() error
```
Flush should be called after the last call to Write to ensure
that any data buffered in the Writer is written to output. Any
incomplete escape sequence at the end is considered
complete for formatting purposes.



### func (\*TabWriter) Init
``` go
func (t *TabWriter) Init(output io.Writer, minwidth, tabwidth, padding int, padchar byte, flags uint) *TabWriter
```
A Writer must be initialized with a call to Init. The first parameter (output)
specifies the filter output. The remaining parameters control the formatting:


	minwidth	minimal cell width including any padding
	tabwidth	width of tab characters (equivalent number of spaces)
	padding		padding added to a cell before computing its width
	padchar		ASCII char used for padding
			if padchar == '\t', the Writer will assume that the
			width of a '\t' in the formatted output is tabwidth,
			and cells are left-aligned independent of align_left
			(for correct-looking results, tabwidth must correspond
			to the tab width in the viewer displaying the result)
	flags		formatting control



## type Writer
``` go
type Writer struct {
    io.Writer
    // contains filtered or unexported fields
}
```
Writer allows colors and styles to be specified. If the io.Writer
is not a terminal capable of color, all attempts to set colors or
styles are no-ops.









### func NewWriter
``` go
func NewWriter(w io.Writer) *Writer
```
NewWriter returns a Writer that allows the caller to specify colors and
styles. If the io.Writer is not a terminal capable of color, all attempts
to set colors or styles are no-ops.




### func (\*Writer) ClearStyle
``` go
func (w *Writer) ClearStyle(s Style)
```
ClearStyle clears the text style.



### func (\*Writer) Reset
``` go
func (w *Writer) Reset()
```
Reset returns the default foreground and background colors with no styles.



### func (\*Writer) SetBackground
``` go
func (w *Writer) SetBackground(c Color)
```
SetBackground sets the background color.



### func (\*Writer) SetForeground
``` go
func (w *Writer) SetForeground(c Color)
```
SetForeground sets the foreground color.



### func (\*Writer) SetStyle
``` go
func (w *Writer) SetStyle(s Style)
```
SetStyle sets the text style.









- - -
Generated by [godoc2md](http://godoc.org/github.com/davecheney/godoc2md)