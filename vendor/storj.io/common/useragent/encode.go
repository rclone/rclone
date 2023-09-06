// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package useragent

import (
	"fmt"
	"strings"
)

// EncodeEntries encodes all entries.
func EncodeEntries(entries []Entry) ([]byte, error) {
	parts := make([]string, len(entries))
	for i, entry := range entries {
		if entry.Product != "" {
			ok := isToken([]byte(entry.Product))
			if !ok {
				return nil, fmt.Errorf("product token is not valid: %s", entry.Product)
			}

			parts[i] = entry.Product
			if entry.Version != "" {
				ok := isToken([]byte(entry.Version))
				if !ok {
					return nil, fmt.Errorf("version token is not valid: %s", entry.Version)
				}

				parts[i] += "/" + entry.Version
			}
		}
		if entry.Comment != "" {
			if entry.Product != "" {
				parts[i] += " "
			}
			parts[i] += "(" + entry.Comment + ")"
		}
	}
	return []byte(strings.Join(parts, " ")), nil
}

func isToken(data []byte) bool {
	// token is a sequence of token characters.
	// See tchar for the allowed characters.
	for i := range data {
		if !istchar(data[i]) {
			return false
		}
	}
	return true
}
