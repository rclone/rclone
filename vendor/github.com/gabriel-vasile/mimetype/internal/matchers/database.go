package matchers

import "bytes"

// Sqlite matches an SQLite database file.
func Sqlite(in []byte) bool {
	return bytes.HasPrefix(in, []byte{
		0x53, 0x51, 0x4c, 0x69, 0x74, 0x65, 0x20, 0x66,
		0x6f, 0x72, 0x6d, 0x61, 0x74, 0x20, 0x33, 0x00,
	})
}

// MsAccessAce matches Microsoft Access dababase file.
func MsAccessAce(in []byte) bool {
	return msAccess(in, []byte("Standard ACE DB"))
}

// MsAccessMdb matches legacy Microsoft Access database file (JET, 2003 and earlier).
func MsAccessMdb(in []byte) bool {
	return msAccess(in, []byte("Standard Jet DB"))
}

func msAccess(in []byte, magic []byte) bool {
	return len(in) > 19 && bytes.Equal(in[4:19], magic)
}
