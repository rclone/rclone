// Copyright 2017 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

//+build go1.7

package delay

import (
	stdctx "context"
	"reflect"

	netctx "golang.org/x/net/context"
)

var (
	stdContextType = reflect.TypeOf((*stdctx.Context)(nil)).Elem()
	netContextType = reflect.TypeOf((*netctx.Context)(nil)).Elem()
)

func isContext(t reflect.Type) bool {
	return t == stdContextType || t == netContextType
}
