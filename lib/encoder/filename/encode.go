package filename

import (
	"encoding/base64"
	"encoding/binary"

	"github.com/dop251/scsu"
	"github.com/klauspost/compress/huff0"
)

// Encode will encode the string and return a base64 (url) compatible version of it.
// Calling Decode with the returned string should always succeed.
// It is not a requirement that the input string is valid utf-8.
func Encode(s string) string {
	table, payload := EncodeBytes(s)
	return string(encodeURL[table]) + base64.URLEncoding.EncodeToString(payload)
}

// EncodeBytes will compress the given string and return a table identifier and a payload.
func EncodeBytes(s string) (table byte, payload []byte) {
	initCoders()
	bestSize := len(s)
	bestTable := byte(tableUncompressed)
	org := []byte(s)
	bestOut := []byte(s)
	// Try all tables and choose the best
	for i, enc := range encTables[:] {
		org := org
		if len(org) <= 1 || len(org) > maxLength {
			// Use the uncompressed
			break
		}

		if enc == nil {
			continue
		}

		if i == tableSCSU {
			var err error
			olen := len(org)
			org, err = scsu.EncodeStrict(s, make([]byte, 0, len(org)))
			if err != nil || olen <= len(org) {
				continue
			}
			if len(org) < bestSize {
				// This is already better, store so we can use if the table cannot.
				bestOut = bestOut[:len(org)]
				bestTable = tableSCSUPlain
				bestSize = len(org)
				copy(bestOut, org)
			}
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
				bestTable = byte(i)
				bestSize = len(out)
				copy(bestOut, out)
			}
			return nil
		}()
		// If input is a single byte repeated store as RLE or save uncompressed.
		if err == huff0.ErrUseRLE && i != tableSCSU {
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

	return bestTable, bestOut
}
