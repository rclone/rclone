package scsu

const (
	/** Single Byte mode command values */

	/** SQ<i>n</i> Quote from Window . <p>
	  If the following byte is less than 0x80, quote from
	  static window <i>n</i>, else quote from dynamic window <i>n</i>.
	*/

	SQ0 = 0x01 // Quote from window pair 0
	SQ1 = 0x02 // Quote from window pair 1
	SQ2 = 0x03 // Quote from window pair 2
	SQ3 = 0x04 // Quote from window pair 3
	SQ4 = 0x05 // Quote from window pair 4
	SQ5 = 0x06 // Quote from window pair 5
	SQ6 = 0x07 // Quote from window pair 6
	SQ7 = 0x08 // Quote from window pair 7

	SDX = 0x0B // Define a window as extended
	Srs = 0x0C // reserved

	SQU = 0x0E // Quote a single Unicode character
	SCU = 0x0F // Change to Unicode mode

	/** SC<i>n</i> Change to Window <i>n</i>. <p>
	  If the following bytes are less than 0x80, interpret them
	  as command bytes or pass them through, else add the offset
	  for dynamic window <i>n</i>. */
	SC0 = 0x10 // Select window 0
	SC1 = 0x11 // Select window 1
	SC2 = 0x12 // Select window 2
	SC3 = 0x13 // Select window 3
	SC4 = 0x14 // Select window 4
	SC5 = 0x15 // Select window 5
	SC6 = 0x16 // Select window 6
	SC7 = 0x17 // Select window 7
	SD0 = 0x18 // Define and select window 0
	SD1 = 0x19 // Define and select window 1
	SD2 = 0x1A // Define and select window 2
	SD3 = 0x1B // Define and select window 3
	SD4 = 0x1C // Define and select window 4
	SD5 = 0x1D // Define and select window 5
	SD6 = 0x1E // Define and select window 6
	SD7 = 0x1F // Define and select window 7

	UC0 = 0xE0 // Select window 0
	UC1 = 0xE1 // Select window 1
	UC2 = 0xE2 // Select window 2
	UC3 = 0xE3 // Select window 3
	UC4 = 0xE4 // Select window 4
	UC5 = 0xE5 // Select window 5
	UC6 = 0xE6 // Select window 6
	UC7 = 0xE7 // Select window 7
	UD0 = 0xE8 // Define and select window 0
	UD1 = 0xE9 // Define and select window 1
	UD2 = 0xEA // Define and select window 2
	UD3 = 0xEB // Define and select window 3
	UD4 = 0xEC // Define and select window 4
	UD5 = 0xED // Define and select window 5
	UD6 = 0xEE // Define and select window 6
	UD7 = 0xEF // Define and select window 7

	UQU = 0xF0 // Quote a single Unicode character
	UDX = 0xF1 // Define a Window as extended
	Urs = 0xF2 // reserved

)

var (
	/** constant offsets for the 8 static windows */
	staticOffset = [...]int32{
		0x0000, // ASCII for quoted tags
		0x0080, // Latin - 1 Supplement (for access to punctuation)
		0x0100, // Latin Extended-A
		0x0300, // Combining Diacritical Marks
		0x2000, // General Punctuation
		0x2080, // Currency Symbols
		0x2100, // Letterlike Symbols and Number Forms
		0x3000, // CJK Symbols and punctuation
	}

	/** initial offsets for the 8 dynamic (sliding) windows */
	initialDynamicOffset = [...]int32{
		0x0080, // Latin-1
		0x00C0, // Latin Extended A   //@005 fixed from 0x0100
		0x0400, // Cyrillic
		0x0600, // Arabic
		0x0900, // Devanagari
		0x3040, // Hiragana
		0x30A0, // Katakana
		0xFF00, // Fullwidth ASCII
	}
)

const (
	/**
	  These values are used in defineWindow
	**/

	/**
	 * Unicode code points from 3400 to E000 are not addressable by
	 * dynamic window, since in these areas no short run alphabets are
	 * found. Therefore add gapOffset to all values from gapThreshold */
	gapThreshold = 0x68
	gapOffset    = 0xAC00

	/* values between reservedStart and fixedThreshold are reserved */
	reservedStart = 0xA8

	/* use table of predefined fixed offsets for values from fixedThreshold */
	fixedThreshold = 0xF9
)

var (
	/** Table of fixed predefined Offsets, and byte values that index into  **/
	fixedOffset = [...]int32{
		/* 0xF9 */ 0x00C0, // Latin-1 Letters + half of Latin Extended A
		/* 0xFA */ 0x0250, // IPA extensions
		/* 0xFB */ 0x0370, // Greek
		/* 0xFC */ 0x0530, // Armenian
		/* 0xFD */ 0x3040, // Hiragana
		/* 0xFE */ 0x30A0, // Katakana
		/* 0xFF */ 0xFF60, // Halfwidth Katakana
	}
)

type scsu struct {
	window        int // current active window
	unicodeMode   bool
	dynamicOffset [8]int32
}

/** whether a character is compressible */
func isCompressible(ch rune) bool {
	return ch < 0x3400 || ch >= 0xE000 && ch <= 0x20000
}

func (scsu *scsu) init() {
	scsu.dynamicOffset = initialDynamicOffset
}

func (scsu *scsu) reset() {
	scsu.window = 0
	scsu.unicodeMode = false
}
