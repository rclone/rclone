// Copyright (C) 2015 Space Monkey, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build !appengine

package monkit

import (
	"sync/atomic"
	"unsafe"
)

//
// *Func atomic functions
//

func loadFunc(addr **Func) (val *Func) {
	return (*Func)(atomic.LoadPointer(
		(*unsafe.Pointer)(unsafe.Pointer(addr))))
}

func compareAndSwapFunc(addr **Func, old, new *Func) bool {
	return atomic.CompareAndSwapPointer(
		(*unsafe.Pointer)(unsafe.Pointer(addr)),
		unsafe.Pointer(old),
		unsafe.Pointer(new))
}

//
// *traceWatcherRef atomic functions
//

func loadTraceWatcherRef(addr **traceWatcherRef) (val *traceWatcherRef) {
	return (*traceWatcherRef)(atomic.LoadPointer(
		(*unsafe.Pointer)(unsafe.Pointer(addr))))
}

func storeTraceWatcherRef(addr **traceWatcherRef, val *traceWatcherRef) {
	atomic.StorePointer(
		(*unsafe.Pointer)(unsafe.Pointer(addr)),
		unsafe.Pointer(val))
}

//
// *spanObserverTuple atomic functons
//

func compareAndSwapSpanObserverTuple(addr **spanObserverTuple,
	old, new *spanObserverTuple) bool {
	return atomic.CompareAndSwapPointer(
		(*unsafe.Pointer)(unsafe.Pointer(addr)),
		unsafe.Pointer(old),
		unsafe.Pointer(new))
}

func loadSpanObserverTuple(addr **spanObserverTuple) (val *spanObserverTuple) {
	return (*spanObserverTuple)(atomic.LoadPointer(
		(*unsafe.Pointer)(unsafe.Pointer(addr))))
}
