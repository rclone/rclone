package matchers

import (
	"bytes"
	"encoding/csv"
	"io"
)

// Csv matches a comma-separated values file.
func Csv(in []byte) bool {
	return sv(in, ',')
}

// Tsv matches a tab-separated values file.
func Tsv(in []byte) bool {
	return sv(in, '\t')
}

func sv(in []byte, comma rune) bool {
	r := csv.NewReader(butLastLineReader(in, ReadLimit))
	r.Comma = comma
	r.TrimLeadingSpace = true
	r.LazyQuotes = true
	r.Comment = '#'

	lines, err := r.ReadAll()
	return err == nil && r.FieldsPerRecord > 1 && len(lines) > 1
}

// butLastLineReader returns a reader to the provided byte slice.
// the reader is guaranteed to reach EOF before it reads `cutAt` bytes.
// bytes after the last newline are dropped from the input.
func butLastLineReader(in []byte, cutAt int) io.Reader {
	if len(in) >= cutAt {
		for i := cutAt - 1; i > 0; i-- {
			if in[i] == '\n' {
				return bytes.NewReader(in[:i])
			}
		}

		// no newline was found between the 0 index and cutAt
		return bytes.NewReader(in[:cutAt])
	}

	return bytes.NewReader(in)
}
