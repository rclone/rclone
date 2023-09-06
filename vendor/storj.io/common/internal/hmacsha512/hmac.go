// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package hmacsha512

// Partial is an in-progress HMAC calculation.
type Partial struct {
	outer digest
	inner digest
	isave digest
	osave digest
}

// New creates a new HMAC-SHA512 calculator.
// It only supports keys that are smaller than sha512.BlockSize.
func New(key []byte) Partial {
	p := Partial{}
	p.Init(key)
	return p
}

// Init initializes the state with the specified key.
func (hm *Partial) Init(key []byte) {
	if len(key) > BlockSize {
		hm.outer.Reset()
		hm.outer.Write(key)
		newKey := hm.outer.FinishAndSum()
		key = newKey[:]
	}

	hm.outer.Reset()
	hm.inner.Reset()

	var ipad [BlockSize]byte
	var opad [BlockSize]byte
	copy(ipad[:], key)
	copy(opad[:], key)

	for i := range ipad {
		ipad[i] ^= 0x36
	}
	for i := range opad {
		opad[i] ^= 0x5c
	}

	hm.inner.Write(ipad[:])
	hm.outer.Write(opad[:])

	hm.isave = hm.inner
	hm.osave = hm.outer
}

// Write appends message to the HMAC calculation.
func (hm *Partial) Write(p []byte) {
	hm.inner.Write(p)
}

// SumAndReset calculates the sum so far and resets the state.
func (hm *Partial) SumAndReset() [Size]byte {
	in := hm.inner.FinishAndSum()
	hm.inner = hm.isave
	hm.outer = hm.osave
	hm.outer.Write(in[:])
	return hm.outer.FinishAndSum()
}
