// Copyright 2019 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"os/user"
	"strconv"
)

// HasAccess tests if a caller can access a file with permissions
// `perm` in mode `mask`
func HasAccess(callerUid, callerGid, fileUid, fileGid uint32, perm uint32, mask uint32) bool {
	if callerUid == 0 {
		// root can do anything.
		return true
	}
	mask = mask & 7
	if mask == 0 {
		return true
	}

	if callerUid == fileUid {
		if perm&(mask<<6) != 0 {
			return true
		}
	}
	if callerGid == fileGid {
		if perm&(mask<<3) != 0 {
			return true
		}
	}
	if perm&mask != 0 {
		return true
	}

	// Check other groups.
	if perm&(mask<<3) == 0 {
		// avoid expensive lookup if it's not allowed anyway
		return false
	}

	u, err := user.LookupId(strconv.Itoa(int(callerUid)))
	if err != nil {
		return false
	}
	gs, err := u.GroupIds()
	if err != nil {
		return false
	}

	fileGidStr := strconv.Itoa(int(fileGid))
	for _, gidStr := range gs {
		if gidStr == fileGidStr {
			return true
		}
	}
	return false
}
