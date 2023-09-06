// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import (
	"runtime/debug"

	"github.com/zeebo/errs"
)

// Error is common error for version package.
var Error = errs.Class("version")

// FromBuild returns version string for a module.
//
// This does not work inside tests.
func FromBuild(modname string) (string, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", Error.New("unable to read build info")
	}

	findmodule := func(modname string) *debug.Module {
		if info.Main.Path == modname {
			return &info.Main
		}
		for _, mod := range info.Deps {
			if mod.Path == modname {
				return mod
			}
		}
		return nil
	}

	mod := findmodule(modname)
	if mod == nil {
		return "", Error.New("unable to find module %q", modname)
	}

	return mod.Version, nil
}
