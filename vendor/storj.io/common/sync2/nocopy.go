// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

// noCopy is used to ensure that we don't copy things that shouldn't
// be copied.
//
// See https://golang.org/issues/8005#issuecomment-190753527.
//
// Currently users of noCopy must use "// nolint: structcheck",
// because golint-ci does not handle this correctly.
type noCopy struct{}

func (noCopy) Lock() {}
