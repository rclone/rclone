package azblob

import "sync/atomic"

// AtomicMorpherInt32 identifies a method passed to and invoked by the AtomicMorphInt32 function.
// The AtomicMorpher callback is passed a startValue and based on this value it returns
// what the new value should be and the result that AtomicMorph should return to its caller.
type AtomicMorpherInt32 func(startVal int32) (val int32, morphResult interface{})

// AtomicMorph atomically morphs target in to new value (and result) as indicated bythe AtomicMorpher callback function.
func AtomicMorphInt32(target *int32, morpher AtomicMorpherInt32) interface{} {
	if target == nil || morpher == nil {
		panic("target and morpher mut not be nil")
	}
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
type AtomicMorpherUint32 func(startVal uint32) (val uint32, morphResult interface{})

// AtomicMorph atomically morphs target in to new value (and result) as indicated bythe AtomicMorpher callback function.
func AtomicMorphUint32(target *uint32, morpher AtomicMorpherUint32) interface{} {
	if target == nil || morpher == nil {
		panic("target and morpher mut not be nil")
	}
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
type AtomicMorpherInt64 func(startVal int64) (val int64, morphResult interface{})

// AtomicMorph atomically morphs target in to new value (and result) as indicated bythe AtomicMorpher callback function.
func AtomicMorphInt64(target *int64, morpher AtomicMorpherInt64) interface{} {
	if target == nil || morpher == nil {
		panic("target and morpher mut not be nil")
	}
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
type AtomicMorpherUint64 func(startVal uint64) (val uint64, morphResult interface{})

// AtomicMorph atomically morphs target in to new value (and result) as indicated bythe AtomicMorpher callback function.
func AtomicMorphUint64(target *uint64, morpher AtomicMorpherUint64) interface{} {
	if target == nil || morpher == nil {
		panic("target and morpher mut not be nil")
	}
	for {
		currentVal := atomic.LoadUint64(target)
		desiredVal, morphResult := morpher(currentVal)
		if atomic.CompareAndSwapUint64(target, currentVal, desiredVal) {
			return morphResult
		}
	}
}
