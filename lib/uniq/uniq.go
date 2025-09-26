package uniq

import (
	"hash/fnv"
	"sync/atomic"
)

const (
	jsIntegerBits = 53
	prefixBits    = 28
	counterBits   = jsIntegerBits - prefixBits
	prefixMask    = (1 << prefixBits) - 1
	counterMask   = (1 << counterBits) - 1
	jsMaxInt      = (1 << jsIntegerBits) - 1
)

// Generator produces monotonically increasing identifiers that remain below
// JavaScript's maximum safe integer. The upper bits are derived from the
// provided seed to keep values unique across different processes on the same
// machine, while the lower bits encode a per-process counter.
type Generator struct {
	prefix  uint64
	counter atomic.Uint64
}

// New constructs a generator that derives a stable prefix from the supplied
// seed. Different seeds are extremely likely to result in different prefixes,
// which keeps the generated identifiers unique across processes using distinct
// seeds (for example, rclone's executeId). The seed may be empty; in that case
// the prefix is forced to a non-zero value so the resulting identifiers remain
// positive.
func New(seed string) *Generator {
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(seed))
	prefix := hasher.Sum64() & prefixMask
	if prefix == 0 {
		prefix = 1
	}
	return &Generator{prefix: prefix}
}

// Next returns the next identifier from the generator. The returned value is
// an int64 that is always non-negative and strictly less than 2^53, keeping it
// safe to transport through JavaScript numbers without precision loss. If the
// per-process counter exceeds the available range this method panics.
func (g *Generator) Next() int64 {
	seq := g.counter.Add(1) - 1
	if seq > counterMask {
		panic("uniq: exhausted unique identifiers for this seed")
	}
	id := (g.prefix << counterBits) | seq
	if id > jsMaxInt {
		// This should be impossible due to the masking above but keep the check
		// in place in case of future modifications.
		panic("uniq: generated identifier exceeds JavaScript safe integer range")
	}
	return int64(id)
}