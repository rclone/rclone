// +build !noencode

package encoder

// Standard defines the encoding that is used for paths in- and output by rclone.
//
// List of replaced characters:
//     (0x00)  -> '␀' // SYMBOL FOR NULL
//   / (slash) -> '／' // FULLWIDTH SOLIDUS
const Standard = MultiEncoder(
	EncodeZero |
		EncodeSlash |
		EncodeCtl |
		EncodeDel |
		EncodeDot)
