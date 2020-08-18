package filename

import (
	"encoding/base64"
	"encoding/binary"

	"github.com/klauspost/compress/huff0"
)

// Encode will encode the string and return a base64 (url) compatible version of it.
// Calling Decode with the returned string should always succeed.
// It is not a requirement that the input string is valid utf-8.
func Encode(s string) string {
	initCoders()
	bestSize := len(s)
	bestTable := tableUncompressed
	org := []byte(s)
	bestOut := []byte(s)

	// Try all tables and choose the best
	for i, enc := range encTables[:] {
		if len(org) <= 1 || len(org) > maxLength {
			// Use the uncompressed
			break
		}
		if enc == nil {
			continue
		}
		// Try to encode using table.
		err := func() error {
			encTableLocks[i].Lock()
			defer encTableLocks[i].Unlock()
			out, _, err := huff0.Compress1X(org, enc)
			if err != nil {
				return err
			}
			if len(out) < bestSize {
				bestOut = bestOut[:len(out)]
				bestTable = i
				bestSize = len(out)
				copy(bestOut, out)
			}
			return nil
		}()
		// If input is a single byte repeated store as RLE or save uncompressed.
		if err == huff0.ErrUseRLE {
			if len(org) > 2 {
				// Encode as one byte repeated since it will be smaller than uncompressed.
				n := binary.PutUvarint(bestOut, uint64(len(org)))
				bestOut = bestOut[:n+1]
				bestOut[n] = org[0]
				bestSize = n + 1
				bestTable = tableRLE
			}
			break
		}
	}

	return string(encodeURL[bestTable]) + base64.URLEncoding.EncodeToString(bestOut)
}
