package fs

// CountSuffix is parsed by flag with k/M/G decimal suffixes
import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// CountSuffix is an int64 with a friendly way of printing setting
type CountSuffix int64

// Common multipliers for SizeSuffix
const (
	CountSuffixBase CountSuffix = 1
	Kilo                        = 1000 * CountSuffixBase
	Mega                        = 1000 * Kilo
	Giga                        = 1000 * Mega
	Tera                        = 1000 * Giga
	Peta                        = 1000 * Tera
	Exa                         = 1000 * Peta
)
const (
	// CountSuffixMax is the largest CountSuffix multiplier
	CountSuffixMax = Exa
	// CountSuffixMaxValue is the largest value that can be used to create CountSuffix
	CountSuffixMaxValue = math.MaxInt64
	// CountSuffixMinValue is the smallest value that can be used to create CountSuffix
	CountSuffixMinValue = math.MinInt64
)

// Turn CountSuffix into a string and a suffix
func (x CountSuffix) string() (string, string) {
	scaled := float64(0)
	suffix := ""
	switch {
	case x < 0:
		return "off", ""
	case x == 0:
		return "0", ""
	case x < Kilo:
		scaled = float64(x)
		suffix = ""
	case x < Mega:
		scaled = float64(x) / float64(Kilo)
		suffix = "k"
	case x < Giga:
		scaled = float64(x) / float64(Mega)
		suffix = "M"
	case x < Tera:
		scaled = float64(x) / float64(Giga)
		suffix = "G"
	case x < Peta:
		scaled = float64(x) / float64(Tera)
		suffix = "T"
	case x < Exa:
		scaled = float64(x) / float64(Peta)
		suffix = "P"
	default:
		scaled = float64(x) / float64(Exa)
		suffix = "E"
	}
	if math.Floor(scaled) == scaled {
		return fmt.Sprintf("%.0f", scaled), suffix
	}
	return fmt.Sprintf("%.3f", scaled), suffix
}

// String turns CountSuffix into a string
func (x CountSuffix) String() string {
	val, suffix := x.string()
	return val + suffix
}

// Unit turns CountSuffix into a string with a unit
func (x CountSuffix) Unit(unit string) string {
	val, suffix := x.string()
	if val == "off" {
		return val
	}
	return val + " " + suffix + unit
}

func (x *CountSuffix) multiplierFromSymbol(s byte) (found bool, multiplier float64) {
	switch s {
	case 'k', 'K':
		return true, float64(Kilo)
	case 'm', 'M':
		return true, float64(Mega)
	case 'g', 'G':
		return true, float64(Giga)
	case 't', 'T':
		return true, float64(Tera)
	case 'p', 'P':
		return true, float64(Peta)
	case 'e', 'E':
		return true, float64(Exa)
	default:
		return false, float64(CountSuffixBase)
	}
}

// Set a CountSuffix
func (x *CountSuffix) Set(s string) error {
	if len(s) == 0 {
		return errors.New("empty string")
	}
	if strings.ToLower(s) == "off" {
		*x = -1
		return nil
	}
	suffix := s[len(s)-1]
	suffixLen := 1
	multiplierFound := false
	var multiplier float64
	switch suffix {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '.':
		suffixLen = 0
		multiplier = float64(Kilo)
	case 'b', 'B':
		if len(s) > 1 {
			suffix = s[len(s)-2]
			if multiplierFound, multiplier = x.multiplierFromSymbol(suffix); multiplierFound {
				suffixLen = 2
			}
		} else {
			multiplier = float64(CountSuffixBase)
		}
	default:
		if multiplierFound, multiplier = x.multiplierFromSymbol(suffix); !multiplierFound {
			return fmt.Errorf("bad suffix %q", suffix)
		}
	}
	s = s[:len(s)-suffixLen]
	value, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	if value < 0 {
		return fmt.Errorf("size can't be negative %q", s)
	}
	value *= multiplier
	*x = CountSuffix(value)
	return nil
}

// Type of the value
func (x CountSuffix) Type() string {
	return "CountSuffix"
}

// Scan implements the fmt.Scanner interface
func (x *CountSuffix) Scan(s fmt.ScanState, ch rune) error {
	token, err := s.Token(true, nil)
	if err != nil {
		return err
	}
	return x.Set(string(token))
}

// CountSuffixList is a slice CountSuffix values
type CountSuffixList []CountSuffix

func (l CountSuffixList) Len() int           { return len(l) }
func (l CountSuffixList) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }
func (l CountSuffixList) Less(i, j int) bool { return l[i] < l[j] }

// Sort sorts the list
func (l CountSuffixList) Sort() {
	sort.Sort(l)
}

// UnmarshalJSON makes sure the value can be parsed as a string or integer in JSON
func (x *CountSuffix) UnmarshalJSON(in []byte) error {
	return UnmarshalJSONFlag(in, x, func(i int64) error {
		*x = CountSuffix(i)
		return nil
	})
}
