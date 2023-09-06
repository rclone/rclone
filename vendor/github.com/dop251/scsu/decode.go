package scsu

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf16"
)

type Reader struct {
	scsu
	brd       io.ByteReader
	bytesRead int
}

var (
	ErrIllegalInput = errors.New("illegal input")
)

func NewReader(r io.ByteReader) *Reader {
	d := &Reader{
		brd: r,
	}
	d.init()
	return d
}

func (r *Reader) readByte() (byte, error) {
	b, err := r.brd.ReadByte()
	if err == nil {
		r.bytesRead++
	}
	return b, err
}

/** (re-)define (and select) a dynamic window
  A sliding window position cannot start at any Unicode value,
  so rather than providing an absolute offset, this function takes
  an index value which selects among the possible starting values.

  Most scripts in Unicode start on or near a half-block boundary
  so the default behaviour is to multiply the index by 0x80. Han,
  Hangul, Surrogates and other scripts between 0x3400 and 0xDFFF
  show very poor locality--therefore no sliding window can be set
  there. A jumpOffset is added to the index value to skip that region,
  and only 167 index values total are required to select all eligible
  half-blocks.

  Finally, a few scripts straddle half block boundaries. For them, a
  table of fixed offsets is used, and the index values from 0xF9 to
  0xFF are used to select these special offsets.

  After (re-)defining a windows location it is selected so it is ready
  for use.

  Recall that all Windows are of the same length (128 code positions).
*/
func (r *Reader) defineWindow(iWindow int, offset byte) error {
	// 0 is a reserved value
	if offset == 0 {
		return ErrIllegalInput
	}
	if offset < gapThreshold {
		r.dynamicOffset[iWindow] = int32(offset) << 7
	} else if offset < reservedStart {
		r.dynamicOffset[iWindow] = (int32(offset) << 7) + gapOffset
	} else if offset < fixedThreshold {
		return fmt.Errorf("offset = %d", offset)
	} else {
		r.dynamicOffset[iWindow] = fixedOffset[offset-fixedThreshold]
	}

	// make the redefined window the active one
	r.window = iWindow
	return nil
}

/** (re-)define (and select) a window as an extended dynamic window
  The surrogate area in Unicode allows access to 2**20 codes beyond the
  first 64K codes by combining one of 1024 characters from the High
  Surrogate Area with one of 1024 characters from the Low Surrogate
  Area (see Unicode 2.0 for the details).

  The tags SDX and UDX set the window such that each subsequent byte in
  the range 80 to FF represents a surrogate pair. The following diagram
  shows how the bits in the two bytes following the SDX or UDX, and a
  subsequent data byte, map onto the bits in the resulting surrogate pair.

   hbyte         lbyte          data
  nnnwwwww      zzzzzyyy      1xxxxxxx

   high-surrogate     low-surrogate
  110110wwwwwzzzzz   110111yyyxxxxxxx

  @param chOffset - Since the three top bits of chOffset are not needed to
  set the location of the extended Window, they are used instead
  to select the window, thereby reducing the number of needed command codes.
  The bottom 13 bits of chOffset are used to calculate the offset relative to
  a 7 bit input data byte to yield the 20 bits expressed by each surrogate pair.
  **/
func (r *Reader) defineExtendedWindow(chOffset uint16) {
	// The top 3 bits of iOffsetHi are the window index
	window := chOffset >> 13

	// Calculate the new offset
	r.dynamicOffset[window] = ((int32(chOffset) & 0x1FFF) << 7) + (1 << 16)

	// make the redefined window the active one
	r.window = int(window)
}

// convert an io.EOF into io.ErrUnexpectedEOF
func unexpectedEOF(e error) error {
	if errors.Is(e, io.EOF) {
		return io.ErrUnexpectedEOF
	}

	return e
}

func (r *Reader) expandUnicode() (rune, error) {
	for {
		b, err := r.readByte()
		if err != nil {
			return 0, err
		}
		if b >= UC0 && b <= UC7 {
			r.window = int(b) - UC0
			r.unicodeMode = false
			return -1, nil
		}
		if b >= UD0 && b <= UD7 {
			b1, err := r.readByte()
			if err != nil {
				return 0, unexpectedEOF(err)
			}
			r.unicodeMode = false
			return -1, r.defineWindow(int(b)-UD0, b1)
		}
		if b == UDX {
			c, err := r.readUint16()
			if err != nil {
				return 0, unexpectedEOF(err)
			}
			r.defineExtendedWindow(c)
			r.unicodeMode = false
			return -1, nil
		}
		if b == UQU {
			r, err := r.readUint16()
			if err != nil {
				return 0, err
			}
			return rune(r), nil
		} else {
			b1, err := r.readByte()
			if err != nil {
				return 0, unexpectedEOF(err)
			}
			ch := rune(uint16FromTwoBytes(b, b1))
			if utf16.IsSurrogate(ch) {
				ch1, err := r.readUint16()
				if err != nil {
					return 0, unexpectedEOF(err)
				}
				surrLo := rune(ch1)
				if !utf16.IsSurrogate(surrLo) {
					return 0, ErrIllegalInput
				}
				return utf16.DecodeRune(ch, surrLo), nil
			}
			return ch, nil
		}
	}
}

func (r *Reader) readUint16() (uint16, error) {
	b1, err := r.readByte()
	if err != nil {
		return 0, unexpectedEOF(err)
	}
	b2, err := r.readByte()
	if err != nil {
		return 0, unexpectedEOF(err)
	}
	return uint16FromTwoBytes(b1, b2), nil
}

func uint16FromTwoBytes(hi, lo byte) uint16 {
	return uint16(hi)<<8 | uint16(lo)
}

/** expand portion of the input that is in single byte mode **/
func (r *Reader) expandSingleByte() (rune, error) {
	for {
		b, err := r.readByte()
		if err != nil {
			return 0, err
		}
		staticWindow := 0
		dynamicWindow := r.window

		switch b {
		case SQ0, SQ1, SQ2, SQ3, SQ4, SQ5, SQ6, SQ7:
			// Select window pair to quote from
			dynamicWindow = int(b) - SQ0
			staticWindow = dynamicWindow
			b, err = r.readByte()
			if err != nil {
				return 0, unexpectedEOF(err)
			}
			fallthrough
		default:
			// output as character
			if b < 0x80 {
				// use static window
				return int32(b) + staticOffset[staticWindow], nil
			} else {
				ch := int32(b) - 0x80
				ch += r.dynamicOffset[dynamicWindow]
				return ch, nil
			}
		case SDX:
			// define a dynamic window as extended
			ch, err := r.readUint16()
			if err != nil {
				return 0, unexpectedEOF(err)
			}
			r.defineExtendedWindow(ch)
		case SD0, SD1, SD2, SD3, SD4, SD5, SD6, SD7:
			// Position a dynamic Window
			b1, err := r.readByte()
			if err != nil {
				return 0, unexpectedEOF(err)
			}
			err = r.defineWindow(int(b)-SD0, b1)
			if err != nil {
				return 0, err
			}
		case SC0, SC1, SC2, SC3, SC4, SC5, SC6, SC7:
			// Select a new dynamic Window
			r.window = int(b) - SC0
		case SCU:
			// switch to Unicode mode and continue parsing
			r.unicodeMode = true
			return -1, nil
		case SQU:
			// directly extract one Unicode character
			ch, err := r.readUint16()
			if err != nil {
				return 0, err
			}
			return rune(ch), nil
		case Srs:
			return 0, ErrIllegalInput
		}
	}
}

func (r *Reader) readRune() (rune, error) {
	for {
		var c rune
		var err error
		if r.unicodeMode {
			c, err = r.expandUnicode()
		} else {
			c, err = r.expandSingleByte()
		}
		if err != nil {
			return 0, err
		}
		if c == -1 {
			continue
		}
		return c, nil
	}
}

// ReadRune reads a single SCSU encoded Unicode character
// and returns the rune and the amount of bytes consumed. If no character is
// available, err will be set.
func (r *Reader) ReadRune() (rune, int, error) {
	pr := r.bytesRead
	c, err := r.readRune()
	return c, r.bytesRead - pr, err
}

// ReadStringSizeHint is like ReadString, but takes a hint about the expected string size.
// Note this is the size of the UTF-8 encoded string in bytes.
func (r *Reader) ReadStringSizeHint(sizeHint int) (string, error) {
	var sb strings.Builder
	if sizeHint > 0 {
		sb.Grow(sizeHint)
	}
	for {
		r, err := r.readRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", err
		}
		sb.WriteRune(r)
	}
	return sb.String(), nil
}

// ReadString reads all available input as a string.
// It keeps reading the source reader until it returns io.EOF or an error occurs.
// In case of io.EOF the error returned by ReadString will be nil.
func (r *Reader) ReadString() (string, error) {
	return r.ReadStringSizeHint(0)
}

func (r *Reader) Reset(rd io.ByteReader) {
	r.brd, r.bytesRead = rd, 0
	r.reset()
	r.init()
}

// Decode a byte array as a string.
func Decode(b []byte) (string, error) {
	return NewReader(bytes.NewBuffer(b)).ReadStringSizeHint(len(b))
}
