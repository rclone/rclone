package azblob

import "sync/atomic"

// AtomicMorpherInt32 identifies a method passed to and invoked by the AtomicMorphInt32 function.
// The AtomicMorpher callback is passed a startValue and based on this value it returns
// what the new value should be and the result that AtomicMorph should return to its caller.
type atomicMorpherInt32 func(startVal int32) (val int32, morphResult interface{})

const targetAndMorpherMustNotBeNil = "target and morpher must not be nil"

// AtomicMorph atomically morphs target in to new value (and result) as indicated bythe AtomicMorpher callback function.
func atomicMorphInt32(target *int32, morpher atomicMorpherInt32) interface{} {
	for {
		currentVal := atomic.LoadInt32(target)
		desiredVal, morphResult := morpher(currentVal)
		if atomic.CompareAndSwapInt32(target, currentVal, desiredVal) {
			return morphResult
		}
	}
}

// AtomicMorpherUint32 identifies a method passed to and invoked by the AtomicMorph function.
// The AtomicMorpher callback is passed a startValue and based on this value it returns
// what the new value should be and the result that AtomicMorph should return to its caller.
type atomicMorpherUint32 func(startVal uint32) (val uint32, morphResult interface{})

// AtomicMorph atomically morphs target in to new value (and result) as indicated bythe AtomicMorpher callback function.
func atomicMorphUint32(target *uint32, morpher atomicMorpherUint32) interface{} {
	for {
		currentVal := atomic.LoadUint32(target)
		desiredVal, morphResult := morpher(currentVal)
		if atomic.CompareAndSwapUint32(target, currentVal, desiredVal) {
			return morphResult
		}
	}
}

// AtomicMorpherUint64 identifies a method passed to and invoked by the AtomicMorphUint64 function.
// The AtomicMorpher callback is passed a startValue and based on this value it returns
// what the new value should be and the result that AtomicMorph should return to its caller.
type atomicMorpherInt64 func(startVal int64) (val int64, morphResult interface{})

// AtomicMorph atomically morphs target in to new value (and result) as indicated bythe AtomicMorpher callback function.
func atomicMorphInt64(target *int64, morpher atomicMorpherInt64) interface{} {
	for {
		currentVal := atomic.LoadInt64(target)
		desiredVal, morphResult := morpher(currentVal)
		if atomic.CompareAndSwapInt64(target, currentVal, desiredVal) {
			return morphResult
		}
	}
}

// AtomicMorpherUint64 identifies a method passed to and invoked by the AtomicMorphUint64 function.
// The AtomicMorpher callback is passed a startValue and based on this value it returns
// what the new value should be and the result that AtomicMorph should return to its caller.
type atomicMorpherUint64 func(startVal uint64) (val uint64, morphResult interface{})

// AtomicMorph atomically morphs target in to new value (and result) as indicated bythe AtomicMorpher callback function.
func atomicMorphUint64(target *uint64, morpher atomicMorpherUint64) interface{} {
	for {
		currentVal := atomic.LoadUint64(target)
		desiredVal, morphResult := morpher(currentVal)
		if atomic.CompareAndSwapUint64(target, currentVal, desiredVal) {
			return morphResult
		}
	}
}
