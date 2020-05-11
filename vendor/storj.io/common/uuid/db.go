// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package uuid

import (
	"database/sql/driver"
)

// Value implements sql/driver.Valuer interface.
func (uuid UUID) Value() (driver.Value, error) {
	return uuid[:], nil
}

// Scan implements sql.Scanner interface.
func (uuid *UUID) Scan(value interface{}) error {
	switch value := value.(type) {
	case []byte:
		x, err := FromBytes(value)
		if err != nil {
			return Error.Wrap(err)
		}
		*uuid = x
		return nil
	case string:
		x, err := FromString(value)
		if err != nil {
			return Error.Wrap(err)
		}
		*uuid = x
		return nil
	default:
		return Error.New("unable to scan %T into UUID", value)
	}
}

// NullUUID represents a UUID that may be null.
// NullUUID implements the Scanner interface so it can be used
// as a scan destination, similar to sql.NullString.
type NullUUID struct {
	UUID  UUID
	Valid bool // Valid is true if UUID is not NULL
}

// Value implements sql/driver.Valuer interface.
func (n NullUUID) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.UUID.Value()
}

// Scan implements sql.Scanner interface.
func (n *NullUUID) Scan(value interface{}) error {
	if value == nil {
		n.UUID, n.Valid = UUID{}, false
		return nil
	}

	n.Valid = true
	return n.UUID.Scan(value)
}
