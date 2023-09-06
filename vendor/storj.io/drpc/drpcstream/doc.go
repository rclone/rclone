// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package drpcstream sends protobufs using the dprc wire protocol.
//
// ![Stream state machine diagram](./state.png)
package drpcstream

// This go:generate directive creates the state.png from the state.dot file. Because the
// generation outputs different binary data each time, it is protected by an if statement
// to ensure that it only creates the png if the dot file has a newer modification time
// somewhat like a Makefile.
//go:generate bash -c "if [ state.dot -nt state.png ]; then dot -Tpng -o state.png state.dot; fi"
