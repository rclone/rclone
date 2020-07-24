package encoder

// Standard defines the encoding that is used for paths in- and output by rclone.
//
// List of replaced characters:
//     (0x00)  -> '␀' // SYMBOL FOR NULL
//   / (slash) -> '／' // FULLWIDTH SOLIDUS
const Standard = (EncodeZero |
	EncodeSlash |
	EncodeCtl |
	EncodeDel |
	EncodeDot)

// Base only encodes the zero byte and slash
const Base = (EncodeZero |
	EncodeSlash |
	EncodeDot)

// Display is the internal encoding for logging and output
const Display = Standard
