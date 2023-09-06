package async

import "sync/atomic"

// atomicBool is an atomic boolean value.
// The zero value is false.
type atomicBool struct {
	v uint32
}

// Load atomically loads and returns the value stored in x.
func (x *atomicBool) load() bool { return atomic.LoadUint32(&x.v) != 0 }

// Store atomically stores val into x.
func (x *atomicBool) store(val bool) { atomic.StoreUint32(&x.v, b32(val)) }

// b32 returns a uint32 0 or 1 representing b.
func b32(b bool) uint32 {
	if b {
		return 1
	}

	return 0
}
