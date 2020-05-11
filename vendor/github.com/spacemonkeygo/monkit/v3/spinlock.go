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

package monkit

import (
	"runtime"
	"sync/atomic"
)

type spinLock uint32

func (s *spinLock) Lock() {
	for {
		if atomic.CompareAndSwapUint32((*uint32)(s), 0, 1) {
			return
		}
		runtime.Gosched()
	}
}

func (s *spinLock) Unlock() {
	atomic.StoreUint32((*uint32)(s), 0)
}
