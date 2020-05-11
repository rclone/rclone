// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pb

import (
	"database/sql/driver"

	proto "github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
)

var scanError = errs.Class("Protobuf Scanner")
var valueError = errs.Class("Protobuf Valuer")

//scan automatically converts database []byte to proto.Messages
func scan(msg proto.Message, value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return scanError.New("%t was %t, expected []bytes", msg, value)
	}
	return scanError.Wrap(Unmarshal(bytes, msg))
}

//value automatically converts proto.Messages to database []byte
func value(msg proto.Message) (driver.Value, error) {
	value, err := Marshal(msg)
	return value, valueError.Wrap(err)
}

// Scan implements the Scanner interface.
func (n *InjuredSegment) Scan(value interface{}) error {
	return scan(n, value)
}

// Value implements the driver Valuer interface.
func (n InjuredSegment) Value() (driver.Value, error) {
	return value(&n)
}
