package smb2

import (
	"encoding/binary"
	"unicode/utf16"
)

var (
	le = binary.LittleEndian
)

func Roundup(x, align int) int {
	return (x + (align - 1)) &^ (align - 1)
}

func UTF16FromString(s string) []uint16 {
	return utf16.Encode([]rune(s))
}

func UTF16ToString(s []uint16) string {
	return string(utf16.Decode(s))
}
