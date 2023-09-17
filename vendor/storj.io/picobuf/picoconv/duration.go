// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package picoconv

import (
	"math"
	"time"

	"storj.io/picobuf"
)

// Duration implements protobuf duration conversion to standard time.Duration.
type Duration time.Duration

// PicoEncode implements custom encoding function.
func (d *Duration) PicoEncode(c *picobuf.Encoder, field picobuf.FieldNumber) bool {
	if d == nil {
		return false
	}
	z := time.Duration(*d)

	n := z.Nanoseconds()
	seconds := n / 1e9
	nanos := int32(n - seconds*1e9)
	c.Message(field, func(c *picobuf.Encoder) bool {
		c.Int64(1, &seconds)
		c.Int32(2, &nanos)
		return true
	})

	return true
}

// PicoDecode implements custom decoding function.
func (d *Duration) PicoDecode(c *picobuf.Decoder, field picobuf.FieldNumber) {
	if c.PendingField() != field {
		return
	}

	var seconds int64
	var nanos int32
	c.Message(field, func(c *picobuf.Decoder) {
		c.Int64(1, &seconds)
		c.Int32(2, &nanos)
	})

	z := time.Duration(seconds) * time.Second
	overflow := z/time.Second != time.Duration(seconds)
	z += time.Duration(nanos) * time.Nanosecond
	overflow = overflow || (seconds < 0 && nanos < 0 && z > 0)
	overflow = overflow || (seconds > 0 && nanos > 0 && z < 0)
	if overflow {
		switch {
		case seconds < 0:
			*d = Duration(time.Duration(math.MinInt64))
			return
		case seconds > 0:
			*d = Duration(time.Duration(math.MaxInt64))
			return
		}
	}

	*d = Duration(z)
}
