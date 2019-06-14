// Package matchers holds the matching functions used to find mime types.
package matchers

// ReadLimit is the maximum number of bytes read
// from the input when detecting a reader.
const ReadLimit = 2048

// True is a dummy matching function used to match any input.
func True([]byte) bool {
	return true
}

// False is a dummy matching function used to never match input.
func False([]byte) bool {
	return false
}

// trimLWS trims whitespace from beginning of the input.
func trimLWS(in []byte) []byte {
	firstNonWS := 0
	for ; firstNonWS < len(in) && isWS(in[firstNonWS]); firstNonWS++ {
	}

	return in[firstNonWS:]
}

// trimRWS trims whitespace from the end of the input.
func trimRWS(in []byte) []byte {
	lastNonWS := len(in) - 1
	for ; lastNonWS > 0 && isWS(in[lastNonWS]); lastNonWS-- {
	}

	return in[:lastNonWS+1]
}

func firstLine(in []byte) []byte {
	lineEnd := 0
	for ; lineEnd < len(in) && in[lineEnd] != '\n'; lineEnd++ {
	}

	return in[:lineEnd]
}

func isWS(b byte) bool {
	return b == '\t' || b == '\n' || b == '\x0c' || b == '\r' || b == ' '
}
