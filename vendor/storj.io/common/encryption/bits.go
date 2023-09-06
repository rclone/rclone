// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption

// incrementBytes takes a byte slice buf and treats it like a little-endian
// encoded unsigned integer. it adds amount to it (which must be nonnegative)
// in place. if rollover happens (the most significant bytes don't fit
// anymore), truncated is true.
func incrementBytes(buf []byte, amount int64) (truncated bool, err error) {
	if amount < 0 {
		return false, Error.New("amount was negative")
	}

	idx := 0
	for amount > 0 && idx < len(buf) {
		var inc, prev byte
		inc, amount = byte(amount), amount>>8

		prev = buf[idx]
		buf[idx] += inc
		if buf[idx] < prev {
			amount++
		}

		idx++
	}

	return amount != 0, nil
}
