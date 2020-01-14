//+build windows

package local

import "github.com/rclone/rclone/lib/encoder"

// This is the encoding used by the local backend for windows platforms
//
// List of replaced characters:
//   < (less than)     -> '＜' // FULLWIDTH LESS-THAN SIGN
//   > (greater than)  -> '＞' // FULLWIDTH GREATER-THAN SIGN
//   : (colon)         -> '：' // FULLWIDTH COLON
//   " (double quote)  -> '＂' // FULLWIDTH QUOTATION MARK
//   \ (backslash)     -> '＼' // FULLWIDTH REVERSE SOLIDUS
//   | (vertical line) -> '｜' // FULLWIDTH VERTICAL LINE
//   ? (question mark) -> '？' // FULLWIDTH QUESTION MARK
//   * (asterisk)      -> '＊' // FULLWIDTH ASTERISK
//
// Additionally names can't end with a period (.) or space ( ).
// List of replaced characters:
//   . (period)        -> '．' // FULLWIDTH FULL STOP
//     (space)         -> '␠' // SYMBOL FOR SPACE
//
// Also encode invalid UTF-8 bytes as Go can't convert them to UTF-16.
//
// https://docs.microsoft.com/de-de/windows/desktop/FileIO/naming-a-file#naming-conventions
const defaultEnc = (encoder.Base |
	encoder.EncodeWin |
	encoder.EncodeBackSlash |
	encoder.EncodeCtl |
	encoder.EncodeRightSpace |
	encoder.EncodeRightPeriod |
	encoder.EncodeInvalidUtf8)
