package fs

// SizeSuffix is parsed by flag with K/M/G binary suffixes
import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// SizeSuffix is an int64 with a friendly way of printing setting
type SizeSuffix int64

// Common multipliers for SizeSuffix
const (
	SizeSuffixBase SizeSuffix = 1 << (iota * 10)
	Kibi
	Mebi
	Gibi
	Tebi
	Pebi
	Exbi
)
const (
	// SizeSuffixMax is the largest SizeSuffix multiplier
	SizeSuffixMax = Exbi
	// SizeSuffixMaxValue is the largest value that can be used to create SizeSuffix
	SizeSuffixMaxValue = math.MaxInt64
	// SizeSuffixMinValue is the smallest value that can be used to create SizeSuffix
	SizeSuffixMinValue = math.MinInt64
)

// Turn SizeSuffix into a string and a suffix
func (x SizeSuffix) string() (string, string) {
	scaled := float64(0)
	suffix := ""
	switch {
	case x < 0:
		return "off", ""
	case x == 0:
		return "0", ""
	case x < Kibi:
		scaled = float64(x)
		suffix = ""
	case x < Mebi:
		scaled = float64(x) / float64(Kibi)
		suffix = "Ki"
	case x < Gibi:
		scaled = float64(x) / float64(Mebi)
		suffix = "Mi"
	case x < Tebi:
		scaled = float64(x) / float64(Gibi)
		suffix = "Gi"
	case x < Pebi:
		scaled = float64(x) / float64(Tebi)
		suffix = "Ti"
	case x < Exbi:
		scaled = float64(x) / float64(Pebi)
		suffix = "Pi"
	default:
		scaled = float64(x) / float64(Exbi)
		suffix = "Ei"
	}
	if math.Floor(scaled) == scaled {
		return fmt.Sprintf("%.0f", scaled), suffix
	}
	return fmt.Sprintf("%.3f", scaled), suffix
}

// String turns SizeSuffix into a string
func (x SizeSuffix) String() string {
	val, suffix := x.string()
	return val + suffix
}

// Unit turns SizeSuffix into a string with a unit
func (x SizeSuffix) unit(unit string) string {
	val, suffix := x.string()
	if val == "off" {
		return val
	}
	var suffixUnit string
	if suffix != "" && unit != "" {
		suffixUnit = suffix + unit
	} else {
		suffixUnit = suffix + unit
	}
	return val + " " + suffixUnit
}

// BitUnit turns SizeSuffix into a string with bit unit
func (x SizeSuffix) BitUnit() string {
	return x.unit("bit")
}

// BitRateUnit turns SizeSuffix into a string with bit rate unit
func (x SizeSuffix) BitRateUnit() string {
	return x.unit("bit/s")
}

// ByteUnit turns SizeSuffix into a string with byte unit
func (x SizeSuffix) ByteUnit() string {
	return x.unit("Byte")
}

// ByteRateUnit turns SizeSuffix into a string with byte rate unit
func (x SizeSuffix) ByteRateUnit() string {
	return x.unit("Byte/s")
}

// ByteShortUnit turns SizeSuffix into a string with byte unit short form
func (x SizeSuffix) ByteShortUnit() string {
	return x.unit("B")
}

// ByteRateShortUnit turns SizeSuffix into a string with byte rate unit short form
func (x SizeSuffix) ByteRateShortUnit() string {
	return x.unit("B/s")
}

func (x *SizeSuffix) multiplierFromSymbol(s byte) (found bool, multiplier float64) {
	switch s {
	case 'k', 'K':
		return true, float64(Kibi)
	case 'm', 'M':
		return true, float64(Mebi)
	case 'g', 'G':
		return true, float64(Gibi)
	case 't', 'T':
		return true, float64(Tebi)
	case 'p', 'P':
		return true, float64(Pebi)
	case 'e', 'E':
		return true, float64(Exbi)
	default:
		return false, float64(SizeSuffixBase)
	}
}

// Set a SizeSuffix
func (x *SizeSuffix) Set(s string) error {
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
		multiplier = float64(Kibi)
	case 'b', 'B':
		if len(s) > 2 && s[len(s)-2] == 'i' {
			suffix = s[len(s)-3]
			suffixLen = 3
			if multiplierFound, multiplier = x.multiplierFromSymbol(suffix); !multiplierFound {
				return errors.Errorf("bad suffix %q", suffix)
			}
			// Could also support SI form MB, and treat it equivalent to MiB, but perhaps better to reserve it for CountSuffix?
			//} else if len(s) > 1 {
			//	suffix = s[len(s)-2]
			//	if multiplierFound, multiplier = x.multiplierFromSymbol(suffix); multiplierFound {
			//		suffixLen = 2
			//	}
			//}
		} else {
			multiplier = float64(SizeSuffixBase)
		}
	case 'i', 'I':
		if len(s) > 1 {
			suffix = s[len(s)-2]
			suffixLen = 2
			multiplierFound, multiplier = x.multiplierFromSymbol(suffix)
		}
		if !multiplierFound {
			return errors.Errorf("bad suffix %q", suffix)
		}
	default:
		if multiplierFound, multiplier = x.multiplierFromSymbol(suffix); !multiplierFound {
			return errors.Errorf("bad suffix %q", suffix)
		}
	}
	s = s[:len(s)-suffixLen]
	value, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	if value < 0 {
		return errors.Errorf("size can't be negative %q", s)
	}
	value *= multiplier
	*x = SizeSuffix(value)
	return nil
}

// Type of the value
func (x *SizeSuffix) Type() string {
	return "SizeSuffix"
}

// Scan implements the fmt.Scanner interface
func (x *SizeSuffix) Scan(s fmt.ScanState, ch rune) error {
	token, err := s.Token(true, nil)
	if err != nil {
		return err
	}
	return x.Set(string(token))
}

// SizeSuffixList is a slice SizeSuffix values
type SizeSuffixList []SizeSuffix

func (l SizeSuffixList) Len() int           { return len(l) }
func (l SizeSuffixList) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }
func (l SizeSuffixList) Less(i, j int) bool { return l[i] < l[j] }

// Sort sorts the list
func (l SizeSuffixList) Sort() {
	sort.Sort(l)
}

// UnmarshalJSONFlag unmarshals a JSON input for a flag. If the input
// is a string then it calls the Set method on the flag otherwise it
// calls the setInt function with a parsed int64.
func UnmarshalJSONFlag(in []byte, x interface{ Set(string) error }, setInt func(int64) error) error {
	// Try to parse as string first
	var s string
	err := json.Unmarshal(in, &s)
	if err == nil {
		return x.Set(s)
	}
	// If that fails parse as integer
	var i int64
	err = json.Unmarshal(in, &i)
	if err != nil {
		return err
	}
	return setInt(i)
}

// UnmarshalJSON makes sure the value can be parsed as a string or integer in JSON
func (x *SizeSuffix) UnmarshalJSON(in []byte) error {
	return UnmarshalJSONFlag(in, x, func(i int64) error {
		*x = SizeSuffix(i)
		return nil
	})
}
