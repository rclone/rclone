// Copyright 2019 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import "syscall"

// ENOATTR indicates that an extended attribute was not present.
var ENOATTR = syscall.ENODATA
