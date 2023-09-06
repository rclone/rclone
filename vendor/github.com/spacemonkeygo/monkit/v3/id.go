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
	crand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"sync/atomic"

	"github.com/spacemonkeygo/monkit/v3/monotime"
)

var (
	idCounter uint64
	inc       uint64
)

func init() {
	var buf [16]byte
	if _, err := crand.Read(buf[:]); err == nil {
		idCounter = binary.BigEndian.Uint64(buf[0:8]) >> 1
		inc = binary.BigEndian.Uint64(buf[0:8])>>1 | 3
	} else {
		rng := rand.New(rand.NewSource(monotime.Now().UnixNano()))
		idCounter = uint64(rng.Int63())
		inc = uint64(rng.Int63() | 3)
	}
}

// NewId returns a random integer intended for use when constructing new
// traces. See NewTrace.
func NewId() int64 {
	id := atomic.AddUint64(&idCounter, inc)
	return int64(id >> 1)
}
