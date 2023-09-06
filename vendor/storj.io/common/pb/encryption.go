// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package pb

import "encoding/json"

// MarshalJSON implements the json.Marshaler interface.
func (cs CipherSuite) MarshalJSON() ([]byte, error) {
	return json.Marshal(cs.String())
}
