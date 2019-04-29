// +build noencode

package encoder

// Fake encodings used for testing
const (
	EncodeStandard = EncodeZero | EncodeSlash | EncodeDot
	Standard       = MultiEncoder(EncodeStandard)
)
