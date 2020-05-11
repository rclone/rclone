// Copyright (C) 2014 Space Monkey, Inc.
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

package errors

import (
	"sync/atomic"
)

var (
	lastId int32 = 0
)

// DataKey's job is to make sure that keys in each error instances namespace
// are lexically scoped, thus helping developers not step on each others' toes
// between large packages. You can only store data on an error using a DataKey,
// and you can only make DataKeys with GenSym().
type DataKey struct{ id int32 }

// GenSym generates a brand new, never-before-seen DataKey
func GenSym() DataKey { return DataKey{id: atomic.AddInt32(&lastId, 1)} }
