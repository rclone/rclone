// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"database/sql/driver"

	"github.com/zeebo/errs"

	"storj.io/common/storj/location"
)

// PlacementConstraint is the ID of the placement/geofencing rule.
type PlacementConstraint uint16

const (

	// EveryCountry includes all countries.
	EveryCountry PlacementConstraint = 0

	// EU includes only the 27 members of European Union.
	EU PlacementConstraint = 1

	// EEA defines the European Economic Area (EU + 3 countries), the area where GDPR is valid.
	EEA PlacementConstraint = 2

	// US filters nodes only from the United States.
	US PlacementConstraint = 3

	// DE placement uses nodes only from Germany.
	DE PlacementConstraint = 4

	// InvalidPlacement is used when there is no information about the stored placement.
	InvalidPlacement PlacementConstraint = 5

	// NR placement uses nodes that are not in RU or other countries sanctioned because of the RU/UA War.
	NR PlacementConstraint = 6
)

// AllowedCountry checks if country is allowed by the placement policy.
func (p PlacementConstraint) AllowedCountry(isoCountryCode location.CountryCode) bool {
	if p == EveryCountry {
		return true
	}
	switch p {
	case EEA:
		for _, c := range location.EuCountries {
			if c == isoCountryCode {
				return true
			}
		}
		for _, c := range location.EeaNonEuCountries {
			if c == isoCountryCode {
				return true
			}
		}
	case EU:
		for _, c := range location.EuCountries {
			if c == isoCountryCode {
				return true
			}
		}
	case US:
		return isoCountryCode.Equal(location.UnitedStates)
	case DE:
		return isoCountryCode.Equal(location.Germany)
	case NR:
		return !isoCountryCode.Equal(location.Russia) && !isoCountryCode.Equal(location.Belarus)
	default:
		return false
	}
	return false
}

// Value implements the driver.Valuer interface.
func (p PlacementConstraint) Value() (driver.Value, error) {
	return int64(p), nil
}

// Scan implements the sql.Scanner interface.
func (p *PlacementConstraint) Scan(value interface{}) error {
	if value == nil {
		*p = EveryCountry
		return nil
	}

	if _, isInt64 := value.(int64); !isInt64 {
		return errs.New("unable to scan %T into PlacementConstraint", value)
	}

	code, err := driver.Int32.ConvertValue(value)
	if err != nil {
		return errs.Wrap(err)
	}
	*p = PlacementConstraint(uint16(code.(int64)))
	return nil

}
