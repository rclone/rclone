// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package location

import (
	"database/sql/driver"
	"strings"

	"github.com/zeebo/errs"
)

// CountryCode stores upper case ISO code of countries.
type CountryCode uint16

// ToCountryCode convert string to CountryCode.
// encoding is based on the ASCII representation of the country code.
func ToCountryCode(s string) CountryCode {
	if len(s) != 2 {
		return CountryCode(0)
	}
	upper := strings.ToUpper(s)
	return CountryCode(uint16(upper[0])*uint16(256) + uint16(upper[1]))
}

// Equal compares two country code.
func (c CountryCode) Equal(o CountryCode) bool {
	return c == o
}

// String returns with the upper-case (two letter) ISO code of the country.
func (c CountryCode) String() string {
	if c == 0 {
		return ""
	}
	return string([]byte{byte(c / 256), byte(c % 256)})
}

// Value implements the driver.Valuer interface.
func (c CountryCode) Value() (driver.Value, error) {
	return c.String(), nil
}

// Scan implements the sql.Scanner interface.
func (c *CountryCode) Scan(value interface{}) error {
	if value == nil {
		*c = None
		return nil
	}

	if _, isString := value.(string); !isString {
		return errs.New("unable to scan %T into CountryCode", value)
	}

	rawValue, err := driver.String.ConvertValue(value)
	if err != nil {
		return errs.Wrap(err)
	}
	*c = ToCountryCode(rawValue.(string))
	return nil

}
