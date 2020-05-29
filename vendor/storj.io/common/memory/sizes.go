// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package memory

import "strings"

// Sizes implements flag.Value for collecting memory size.
type Sizes struct {
	Default []Size
	Custom  []Size
}

// Sizes returns the loaded values.
func (sizes Sizes) Sizes() []Size {
	if len(sizes.Custom) > 0 {
		return sizes.Custom
	}
	return sizes.Default
}

// String converts values to a string.
func (sizes Sizes) String() string {
	sz := sizes.Sizes()
	xs := make([]string, len(sz))
	for i, size := range sz {
		xs[i] = size.String()
	}
	return strings.Join(xs, " ")
}

// Set adds values from byte values.
func (sizes *Sizes) Set(s string) error {
	for _, x := range strings.Fields(s) {
		var size Size
		if err := size.Set(x); err != nil {
			return err
		}
		sizes.Custom = append(sizes.Custom, size)
	}
	return nil
}
