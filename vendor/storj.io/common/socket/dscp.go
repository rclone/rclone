// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package socket

// A DSCP is a Differentiated Services Code Point.
//
//nolint:unused
type dscp byte

// See https://tools.ietf.org/html/rfc4594#section-2.3 for the definitions
// of the below Differentiated Services Code Points.
//
//nolint:deadcode,varcheck,unused
const (
	dscpDF   dscp = 0
	dscpCS6  dscp = 0b110000
	dscpEF   dscp = 0b101110
	dscpCS5  dscp = 0b101000
	dscpAF41 dscp = 0b100010
	dscpAF42 dscp = 0b100100
	dscpAF43 dscp = 0b100110
	dscpCS4  dscp = 0b100000
	dscpAF31 dscp = 0b011010
	dscpAF32 dscp = 0b011100
	dscpAF33 dscp = 0b011110
	dscpCS3  dscp = 0b011000
	dscpAF21 dscp = 0b010010
	dscpAF22 dscp = 0b010100
	dscpAF23 dscp = 0b010110
	dscpCS2  dscp = 0b010000
	dscpAF11 dscp = 0b001010
	dscpAF12 dscp = 0b001100
	dscpAF13 dscp = 0b001110
	dscpCS1  dscp = 0b001000
	dscpLE   dscp = 0b000001
)
