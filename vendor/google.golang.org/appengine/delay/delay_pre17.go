// Copyright 2017 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

//+build !go1.7

package delay

import (
	"reflect"

	"golang.org/x/net/context"
)

var contextType = reflect.TypeOf((*context.Context)(nil)).Elem()

func isContext(t reflect.Type) bool {
	return t == contextType
}
