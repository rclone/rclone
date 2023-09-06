package scsu

import (
	"errors"
	"fmt"
	"io"
	"unicode/utf16"
	"unicode/utf8"
)

// A RuneSource represents a sequence of runes with look-behind support.
//
// RuneAt returns a rune at a given position.
// The position starts at zero and is not guaranteed to be sequential, therefore
// the only valid arguments are 0 or one of the previously returned as nextPos.
// Supplying anything else results in an unspecified behaviour.
// Returns io.EOF when there are no more runes left.
// If a rune was read err must be nil (i.e. (rune, EOF) combination is not possible)
type RuneSource interface {
	RuneAt(pos int) (r rune, nextPos int, err error)
}

// StrictStringRuneSource does not tolerate invalid UTF-8 sequences.
type StrictStringRuneSource string

// StringRuneSource represents an UTF-8 string. Invalid sequences are replaced with
// utf8.RuneError.
type StringRuneSource string

// SingleRuneSource that contains a single rune.
type SingleRuneSource rune

// RuneSlice is a RuneSource backed by []rune.
type RuneSlice []rune

type encoder struct {
	scsu
	wr      io.Writer // nil when encoding into a slice
	out     []byte    // a buffer so that we can go back and replace SCU to SQU. In streaming mode does not need more than 5 bytes
	written int

	src     RuneSource
	curRune rune
	curErr  error
	nextPos int
	scuPos  int

	nextWindow int
}

// Encoder can be used to encode a string into []byte.
// Zero value is ready to use.
type Encoder struct {
	encoder
}

type Writer struct {
	encoder
}

var (
	ErrInvalidUTF8 = errors.New("invalid UTF-8")
)

func (s StrictStringRuneSource) RuneAt(pos int) (rune, int, error) {
	if pos < len(s) {
		r, size := utf8.DecodeRuneInString(string(s)[pos:])
		if r == utf8.RuneError && size == 1 {
			return 0, 0, ErrInvalidUTF8
		}
		return r, pos + size, nil
	}
	return 0, 0, io.EOF
}

func (s StringRuneSource) RuneAt(pos int) (rune, int, error) {
	if pos < len(s) {
		r, size := utf8.DecodeRuneInString(string(s)[pos:])
		return r, pos + size, nil
	}
	return 0, 0, io.EOF
}

func (s RuneSlice) RuneAt(pos int) (rune, int, error) {
	if pos < len(s) {
		return s[pos], pos + 1, nil
	}
	return 0, 0, io.EOF
}

func (r SingleRuneSource) RuneAt(pos int) (rune, int, error) {
	if pos == 0 {
		return rune(r), 1, nil
	}
	return 0, 0, io.EOF
}

func NewWriter(wr io.Writer) *Writer {
	e := new(Writer)
	e.wr = wr
	e.init()
	return e
}

func (e *encoder) init() {
	e.scsu.init()
	e.nextWindow = 3
	e.scuPos = -1
}

func (e *encoder) nextRune() {
	e.curRune, e.nextPos, e.curErr = e.src.RuneAt(e.nextPos)
}

/** locate a window for a character given a table of offsets
  @param ch - character
  @param offsetTable - table of window offsets
  @return true if the character fits a window from the table of windows */
func (e *encoder) locateWindow(ch rune, offsetTable []int32) bool {
	// always try the current window first
	// if the character fits the current window
	// just use the current window
	if win := e.window; win != -1 {
		if offset := offsetTable[win]; ch >= offset && ch < offset+0x80 {
			return true
		}
	}

	// try all windows in order
	for win, offset := range offsetTable {
		if ch >= offset && ch < offset+0x80 {
			e.window = win
			return true
		}
	}
	// none found
	return false
}

/** returns true if the character is ASCII, but not a control other than CR, LF and TAB */
func isAsciiCrLfOrTab(ch rune) bool {
	return (ch >= 0x20 && ch <= 0x7F) || // ASCII
		ch == 0x09 || ch == 0x0A || ch == 0x0D // CR/LF or TAB
}

/** output a run of characters in single byte mode
    In single byte mode pass through characters in the ASCII range, but
    quote characters overlapping with compression command codes. Runs
    of characters fitting the current window are output as runs of bytes
    in the range 0x80-0xFF.
**/
func (e *encoder) outputSingleByteRun() error {
	win := e.window
	for e.curErr == nil {
		ch := e.curRune
		// ASCII Letter, NUL, CR, LF and TAB are always passed through
		if isAsciiCrLfOrTab(ch) || ch == 0 {
			// pass through directly
			e.out = append(e.out, byte(ch&0x7F))
		} else if ch < 0x20 {
			// All other control codes must be quoted
			e.out = append(e.out, SQ0, byte(ch))
		} else if dOffset := e.dynamicOffset[win]; ch >= dOffset && ch < dOffset+0x80 {
			// Letters that fit the current dynamic window
			ch -= dOffset
			e.out = append(e.out, byte(ch|0x80))
		} else {
			// need to use some other compression mode for this
			// character so we terminate this loop
			break
		}
		err := e.flush()
		if err != nil {
			return err
		}
		e.nextRune()
	}

	return nil
}

/** quote a single character in single byte mode
  Quoting a character (aka 'non-locking shift') gives efficient access
  to characters that occur in isolation--usually punctuation characters.
  When quoting a character from a dynamic window use 0x80 - 0xFF, when
  quoting a character from a static window use 0x00-0x7f.
  **/
func (e *encoder) quoteSingleByte(ch rune) error {
	// Output command byte followed by...
	e.out = append(e.out, byte(SQ0+e.window))
	if offset := e.dynamicOffset[e.window]; ch >= offset && ch < offset+0x80 {
		// ... letter that fits the current dynamic window
		ch -= offset
		e.out = append(e.out, byte(ch|0x80))
	} else if offset := staticOffset[e.window]; ch >= offset && ch < offset+0x80 {
		// ... letter that fits the current static window
		ch -= offset
		e.out = append(e.out, byte(ch))
	} else {
		return fmt.Errorf("ch = %d not valid in quoteSingleByte. Internal Compressor Error", ch)
	}

	err := e.flush()
	if err != nil {
		return err
	}
	return nil
}

/** output a run of characters in Unicode mode
  A run of Unicode mode consists of characters which are all in the
  range of non-compressible characters or isolated occurrence
  of any other characters. Characters in the range 0xE00-0xF2FF must
  be quoted to avoid overlap with the Unicode mode compression command codes.
  **/
func (e *encoder) outputUnicodeRun() (err error) {
	for e.curErr == nil {
		r, n := e.curRune, e.nextPos
		var r1 rune
		var n1 int
		if isCompressible(r) {
			r1, n1, err = e.src.RuneAt(n)
			if err != nil && err != io.EOF {
				return
			}
			if err == nil && isCompressible(r1) {
				// at least 2 characters are compressible
				// break the run
				break
			}
			if err != nil && e.scuPos != -1 && (r == 0 || isAsciiCrLfOrTab(r)) {
				// The current character is the last one, it is a pass-though
				// character (i.e. can be encoded with one byte without
				// changing a window) and we have only produced one unicode
				// character so far.
				// The result will be an SQU followed by a unicode character,
				// followed by a single byte.
				// If we didn't break here it would be one byte longer
				// (SCU followed by 2 unicode characters).
				err = nil
				break
			}
		}

		// If we get here, the current character is only character
		// left in the input or it is followed by a non-compressible
		// character. In neither case do we gain by breaking the
		// run, so we proceed to output the character.
		if r < 0x10000 {
			if r >= 0xE000 && r <= 0xF2FF {
				// Characters in this range need to be escaped
				e.out = append(e.out, UQU)
			}
			e.out = append(e.out, byte(r>>8), byte(r))
		} else {
			r1, r2 := utf16.EncodeRune(r)
			e.out = append(e.out, byte(r1>>8), byte(r1), byte(r2>>8), byte(r2))
		}
		if n1 != 0 {
			e.curRune, e.nextPos = r1, n1
		} else {
			e.nextRune()
		}

		if len(e.out)-e.scuPos > 3 {
			err = e.flush()
			e.scuPos = -1
			if err != nil {
				return
			}
		}
	}

	return
}

// redefine a window so it surrounds a given character value
func (e *encoder) positionWindow(ch rune, fUnicodeMode bool) bool {
	iWin := e.nextWindow % 8 // simple LRU
	var iPosition uint16

	// iPosition 0 is a reserved value
	if ch < 0x80 {
		panic("ch < 0x80")
	}

	// Check the fixed offsets
	for i := 0; i < len(fixedOffset); i++ {
		if offset := fixedOffset[i]; ch >= offset && ch < offset+0x80 {
			iPosition = uint16(i)
			break
		}
	}

	extended := false
	if iPosition != 0 {
		e.dynamicOffset[iWin] = fixedOffset[iPosition]
		iPosition += fixedThreshold
	} else if ch < 0x3400 {
		// calculate a window position command and set the offset
		iPosition = uint16(ch >> 7)
		e.dynamicOffset[iWin] = ch & 0xFF80
	} else if ch < 0xE000 {
		// attempt to place a window where none can go
		return false
	} else if ch <= 0xFFFF {
		// calculate a window position command, accounting
		// for the gap in position values, and set the offset
		iPosition = uint16((ch - gapOffset) >> 7)
		e.dynamicOffset[iWin] = ch & 0xFF80
	} else {
		// if we get here, the character is in the extended range.

		iPosition = uint16((ch - 0x10000) >> 7)

		iPosition |= uint16(iWin << 13)
		e.dynamicOffset[iWin] = ch & 0x1FFF80
		extended = true
	}

	if !extended {
		// Outputting window definition command for the general cases
		var b byte
		if fUnicodeMode {
			b = UD0
		} else {
			b = SD0
		}
		e.out = append(e.out, b+byte(iWin), byte(iPosition&0xFF))
	} else {
		// Output an extended window definition command
		var b byte
		if fUnicodeMode {
			b = UDX
		} else {
			b = SDX
		}
		e.out = append(e.out, b, byte(iPosition>>8), byte(iPosition))
	}
	e.window = iWin
	e.nextWindow++
	return true
}

// Note, e.curRune must be compressible
func (e *encoder) chooseWindow() error {
	curCh, nextPos := e.curRune, e.nextPos
	var err error

	// Find a nearest compressible non-ASCII character which will be used
	// to select a window.
	// If we encounter a non-quotable unicode sequence (i.e. a single
	// extended incompressible character or two BMP incompressible characters
	// we stop and use the next character as it doesn't matter which window to
	// select: all the characters will be ASCII and then we'll have to switch to
	// unicode mode.
	windowDecider := curCh
	prevIncompressible := false
	for c, p := curCh, nextPos; ; {
		if c >= 0x80 {
			if !isCompressible(c) {
				if c >= 0x10000 || prevIncompressible {
					break
				}
				prevIncompressible = true
			} else {
				windowDecider = c
				break
			}
		} else {
			prevIncompressible = false
		}
		c, p, err = e.src.RuneAt(p)
		if err != nil {
			if err == io.EOF {
				err = nil
				break
			}
			return err
		}
	}
	prevWindow := e.window

	// try to locate a dynamic window
	if windowDecider < 0x80 || e.locateWindow(windowDecider, e.dynamicOffset[:]) {
		// lookahead to use SQn instead of SCn for single
		// character interruptions of runs in current window
		if !e.unicodeMode {
			ch2, n2, err := e.src.RuneAt(nextPos)
			if err != nil && err != io.EOF {
				return err
			}
			if err == nil && ch2 >= e.dynamicOffset[prevWindow] &&
				ch2 < e.dynamicOffset[prevWindow]+0x80 {
				err = e.quoteSingleByte(curCh)
				if err != nil {
					return err
				}
				e.curRune, e.nextPos = ch2, n2
				e.window = prevWindow
				return nil
			}
		}
		var b byte
		if e.unicodeMode {
			b = UC0
		} else {
			b = SC0
		}
		e.out = append(e.out, b+byte(e.window))
		e.unicodeMode = false
		return nil
	} else
	// try to locate a static window
	if !e.unicodeMode && e.locateWindow(windowDecider, staticOffset[:]) {
		// static windows are not accessible from Unicode mode
		err = e.quoteSingleByte(curCh)
		if err != nil {
			return err
		}
		e.nextRune()
		e.window = prevWindow // restore current Window settings
		return nil
	} else // try to define a window around windowDecider
	if e.positionWindow(windowDecider, e.unicodeMode) {
		e.unicodeMode = false
		return nil
	}
	return errors.New("could not select window. Internal Compressor Error")
}

func (e *encoder) encode(src RuneSource) error {
	var err error
	e.src, e.written, e.nextPos = src, 0, 0
	e.nextRune()

	for {
		if e.unicodeMode {
			err = e.outputUnicodeRun()
			if err != nil {
				break
			}
			if e.scuPos != -1 && len(e.out)-e.scuPos == 3 {
				// for single character Unicode runs use quote
				// go back and fix up the SCU to an SQU instead
				e.out[e.scuPos] = SQU
				e.scuPos = -1
				err = e.flush()
				if err != nil {
					break
				}
				e.unicodeMode = false
				continue
			} else {
				e.scuPos = -1
				err = e.flush()
				if err != nil {
					break
				}
			}
		} else {
			err = e.outputSingleByteRun()
			if err != nil {
				break
			}
		}
		if e.curErr != nil {
			err = e.curErr
			break
		}
		// note, if we were in unicode mode the character must be compressible
		if isCompressible(e.curRune) {
			err = e.chooseWindow()
			if err != nil {
				break
			}
		} else {
			// switching to unicode
			e.scuPos = len(e.out)
			e.out = append(e.out, SCU)
			e.unicodeMode = true
		}
	}

	if errors.Is(err, io.EOF) {
		err = e.flush()
	}

	e.src = nil // do not hold the reference

	return err
}

// WriteString encodes the given string and writes the binary representation
// into the writer. Invalid UTF-8 sequences are replaced with utf8.RuneError.
// Returns the number of bytes written and an error (if any).
func (w *Writer) WriteString(in string) (int, error) {
	return w.WriteRunes(StringRuneSource(in))
}

// WriteRune encodes the given rune and writes the binary representation
// into the writer.
// Returns the number of bytes written and an error (if any).
func (w *Writer) WriteRune(r rune) (int, error) {
	return w.WriteRunes(SingleRuneSource(r))
}

func (w *Writer) WriteRunes(src RuneSource) (int, error) {
	err := w.encode(src)
	return w.written, err
}

func (e *encoder) flush() error {
	if e.wr != nil && len(e.out) > 0 {
		n, err := e.wr.Write(e.out)
		e.written += n
		e.out = e.out[:0]
		return err
	}

	return nil
}

// Reset discards the encoder's state and makes it equivalent to the result of NewEncoder
// called with w allowing to re-use the instance.
func (w *Writer) Reset(out io.Writer) {
	w.wr = out
	w.out = w.out[:0]
	w.reset()
	w.init()
}

// Encode the given RuneSource and append to dst. If dst does not have enough capacity
// it will be re-allocated. It can be nil.
// Not goroutine-safe. The instance can be re-used after.
func (e *Encoder) Encode(src RuneSource, dst []byte) ([]byte, error) {
	e.reset()
	e.init()
	e.out = dst
	err := e.encode(src)
	out := e.out
	e.out = nil
	return out, err
}

// Encode src and append to dst. If dst does not have enough capacity
// it will be re-allocated. It can be nil.
func Encode(src string, dst []byte) ([]byte, error) {
	var e Encoder
	return e.Encode(StringRuneSource(src), dst)
}

// EncodeStrict is the same as Encode, however it stops and returns ErrInvalidUTF8
// if an invalid UTF-8 sequence is encountered rather than replacing it with
// utf8.RuneError.
func EncodeStrict(src string, dst []byte) ([]byte, error) {
	var e Encoder
	return e.Encode(StrictStringRuneSource(src), dst)
}

// FindFirstEncodable returns the position of the first byte that is not pass-through.
// Returns -1 if the entire string is pass-through (i.e. encoding it would return the string unchanged).
func FindFirstEncodable(src string) int {
	for i := 0; i < len(src); i++ {
		if !isAsciiCrLfOrTab(rune(src[i])) {
			return i
		}
	}
	return -1
}
