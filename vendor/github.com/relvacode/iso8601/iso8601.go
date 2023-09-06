// Package iso8601 is a utility for parsing ISO8601 datetime strings into native Go times.
// The standard library's RFC3339 reference layout can be too strict for working with 3rd party APIs,
// especially ones written in other languages.
//
// Use the provided `Time` structure instead of the default `time.Time` to provide ISO8601 support for JSON responses.
package iso8601

import (
	"time"
)

const (
	year uint = iota
	month
	day
	hour
	minute
	second
	millisecond
)

const (
	// charStart is the binary position of the character `0`
	charStart uint = '0'
)

// ParseISOZone parses the 5 character zone information in an ISO8061 date string.
// This function expects input that matches:
//
//	-0100
//	+0100
//	+01:00
//	-01:00
//	+01
//	+01:45
//	+0145
func ParseISOZone(inp []byte) (*time.Location, error) {
	if len(inp) < 3 || len(inp) > 6 {
		return nil, ErrZoneCharacters
	}
	var neg bool
	switch inp[0] {
	case '+':
	case '-':
		neg = true
	default:
		return nil, newUnexpectedCharacterError(inp[0])
	}

	var offset int

	var z uint
	var multiplier = uint(3600) // start with initial multiplier of hours
	for i := 1; i < len(inp); i++ {
		if i == 3 { // next multiplier
			offset = int(z * multiplier)
			multiplier = 60 // multiplier for minutes
			z = 0
		} else { // next digit
			z = z * 10
		}

		switch inp[i] {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			z += uint(inp[i]) - charStart
		case ':':
			if i != 3 {
				return nil, newUnexpectedCharacterError(inp[i])
			}
		default:
			return nil, newUnexpectedCharacterError(inp[i])
		}

	}

	offset += int(z * multiplier)

	if neg {
		offset = -offset
	}
	if neg && offset == 0 {
		return nil, ErrInvalidZone
	}
	return time.FixedZone("", offset), nil
}

// Parse parses an ISO8601 compliant date-time byte slice into a time.Time object.
// If any component of an input date-time is not within the expected range then an *iso8601.RangeError is returned.
func Parse(inp []byte) (time.Time, error) {
	var (
		Y         uint
		M         uint
		d         uint
		h         uint
		m         uint
		s         uint
		fraction  int
		nfraction = 1 //counts amount of precision for the second fraction
	)

	// Always assume UTC by default
	var loc = time.UTC

	var c uint
	var p = year

	var i int

parse:
	for ; i < len(inp); i++ {
		switch inp[i] {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			c = c * 10
			c += uint(inp[i]) - charStart

			if p == millisecond {
				nfraction++
			}
		case '-':
			if p < hour {
				switch p {
				case year:
					Y = c
				case month:
					M = c
				default:
					return time.Time{}, newUnexpectedCharacterError(inp[i])
				}
				p++
				c = 0
				continue
			}
			fallthrough
		case '+':
			if i == 0 {
				// The ISO8601 technically allows signed year components.
				// Go does not allow negative years, but let's allow a positive sign to be more compatible with the spec.
				// It must be the very first character of the input (#11).
				continue
			}

			switch p {
			case hour:
				h = c
			case minute:
				m = c
			case second:
				s = c
			case millisecond:
				fraction = int(c)
			default:
				return time.Time{}, newUnexpectedCharacterError(inp[i])
			}
			c = 0
			var err error
			loc, err = ParseISOZone(inp[i:])
			if err != nil {
				return time.Time{}, err
			}
			break parse
		case 'T':
			if p != day {
				return time.Time{}, newUnexpectedCharacterError(inp[i])
			}
			d = c
			c = 0
			p++
		case ':':
			switch p {
			case hour:
				h = c
			case minute:
				m = c
			case second:
				m = c
			default:
				return time.Time{}, newUnexpectedCharacterError(inp[i])
			}
			c = 0
			p++
		case '.':
			if p != second {
				return time.Time{}, newUnexpectedCharacterError(inp[i])
			}
			s = c
			c = 0
			p++
		case 'Z':
			switch p {
			case hour:
				h = c
			case minute:
				m = c
			case second:
				s = c
			case millisecond:
				fraction = int(c)
			default:
				return time.Time{}, newUnexpectedCharacterError(inp[i])
			}
			c = 0
			if len(inp) != i+1 {
				return time.Time{}, ErrRemainingData
			}
		default:
			return time.Time{}, newUnexpectedCharacterError(inp[i])
		}
	}

	// Capture remaining data
	// Sometimes a date can end without a non-integer character
	if c > 0 {
		switch p {
		case day:
			d = c
		case hour:
			h = c
		case minute:
			m = c
		case second:
			s = c
		case millisecond:
			fraction = int(c)
		}
	}

	// Get the seconds fraction as nanoseconds
	if fraction < 0 || 1e9 <= fraction {
		return time.Time{}, ErrPrecision
	}
	scale := 10 - nfraction
	for i := 0; i < scale; i++ {
		fraction *= 10
	}

	switch {
	case M < 1 || M > 12: // Month 1-12
		return time.Time{}, &RangeError{
			Value:   string(inp),
			Element: "month",
			Given:   int(M),
			Min:     1,
			Max:     12,
		}
	case d < 1 || int(d) > daysIn(time.Month(M), int(Y)): // Day 1-daysIn(month, year)
		return time.Time{}, &RangeError{
			Value:   string(inp),
			Element: "day",
			Given:   int(d),
			Min:     1,
			Max:     daysIn(time.Month(M), int(Y)),
		}
	case h > 23: // Hour 0-23
		return time.Time{}, &RangeError{
			Value:   string(inp),
			Element: "hour",
			Given:   int(h),
			Min:     0,
			Max:     23,
		}
	case m > 59: // Minute 0-59
		return time.Time{}, &RangeError{
			Value:   string(inp),
			Element: "minute",
			Given:   int(m),
			Min:     0,
			Max:     59,
		}
	case s > 59: // Second 0-59
		return time.Time{}, &RangeError{
			Value:   string(inp),
			Element: "second",
			Given:   int(s),
			Min:     0,
			Max:     59,
		}
	}

	return time.Date(int(Y), time.Month(M), int(d), int(h), int(m), int(s), fraction, loc), nil
}

// ParseString parses an ISO8601 compliant date-time string into a time.Time object.
func ParseString(inp string) (time.Time, error) {
	return Parse([]byte(inp))
}
