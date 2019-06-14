package matchers

import (
	"bytes"
)

// Class matches an java class file.
func Class(in []byte) bool {
	return len(in) > 4 && bytes.Equal(in[:4], []byte{0xCA, 0xFE, 0xBA, 0xBE})
}

// Swf matches an Adobe Flash swf file.
func Swf(in []byte) bool {
	return len(in) > 3 &&
		bytes.Equal(in[:3], []byte("CWS")) ||
		bytes.Equal(in[:3], []byte("FWS")) ||
		bytes.Equal(in[:3], []byte("ZWS"))
}

// Wasm matches a web assembly File Format file.
func Wasm(in []byte) bool {
	return len(in) > 4 && bytes.Equal(in[:4], []byte{0x00, 0x61, 0x73, 0x6D})
}

// Dbf matches a dBase file.
// https://www.dbase.com/Knowledgebase/INT/db7_file_fmt.htm
func Dbf(in []byte) bool {
	// 3rd and 4th bytes contain the last update month and day of month
	if !(0 < in[2] && in[2] < 13 && 0 < in[3] && in[3] < 32) {
		return false
	}

	// dbf type is dictated by the first byte
	dbfTypes := []byte{
		0x02, 0x03, 0x04, 0x05, 0x30, 0x31, 0x32, 0x42, 0x62, 0x7B, 0x82,
		0x83, 0x87, 0x8A, 0x8B, 0x8E, 0xB3, 0xCB, 0xE5, 0xF5, 0xF4, 0xFB,
	}
	for _, b := range dbfTypes {
		if in[0] == b {
			return true
		}
	}

	return false
}

// Exe matches a Windows/DOS executable file.
func Exe(in []byte) bool {
	return bytes.HasPrefix(in, []byte{0x4D, 0x5A})
}

// Elf matches an Executable and Linkable Format file.
func Elf(in []byte) bool {
	return bytes.HasPrefix(in, []byte{0x7F, 0x45, 0x4C, 0x46})
}

// ElfObj matches an object file.
func ElfObj(in []byte) bool {
	return len(in) > 17 && ((in[16] == 0x01 && in[17] == 0x00) ||
		(in[16] == 0x00 && in[17] == 0x01))
}

// ElfExe matches an executable file.
func ElfExe(in []byte) bool {
	return len(in) > 17 && ((in[16] == 0x02 && in[17] == 0x00) ||
		(in[16] == 0x00 && in[17] == 0x02))
}

// ElfLib matches a shared library file.
func ElfLib(in []byte) bool {
	return len(in) > 17 && ((in[16] == 0x03 && in[17] == 0x00) ||
		(in[16] == 0x00 && in[17] == 0x03))
}

// ElfDump matches a core dump file.
func ElfDump(in []byte) bool {
	return len(in) > 17 && ((in[16] == 0x04 && in[17] == 0x00) ||
		(in[16] == 0x00 && in[17] == 0x04))
}

// Dcm matches a DICOM medical format file.
func Dcm(in []byte) bool {
	return len(in) > 131 &&
		bytes.Equal(in[128:132], []byte{0x44, 0x49, 0x43, 0x4D})
}
