// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the GO_LICENSE file.

package hmacsha512

import "golang.org/x/sys/cpu"

var useAsm = cpu.S390X.HasSHA512
