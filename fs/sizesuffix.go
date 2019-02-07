package fs

// SizeSuffix is parsed by flag with k/M/G suffixes
import (
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
	Byte SizeSuffix = 1 << (iota * 10)
	KibiByte
	MebiByte
	GibiByte
	TebiByte
	PebiByte
	ExbiByte
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
	case x < 1<<10:
		scaled = float64(x)
		suffix = ""
	case x < 1<<20:
		scaled = float64(x) / (1 << 10)
		suffix = "k"
	case x < 1<<30:
		scaled = float64(x) / (1 << 20)
		suffix = "M"
	case x < 1<<40:
		scaled = float64(x) / (1 << 30)
		suffix = "G"
	case x < 1<<50:
		scaled = float64(x) / (1 << 40)
		suffix = "T"
	default:
		scaled = float64(x) / (1 << 50)
		suffix = "P"
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
func (x SizeSuffix) Unit(unit string) string {
	val, suffix := x.string()
	if val == "off" {
		return val
	}
	return val + " " + suffix + unit
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
	var multiplier float64
	switch suffix {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '.':
		suffixLen = 0
		multiplier = 1 << 10
	case 'b', 'B':
		multiplier = 1
	case 'k', 'K':
		multiplier = 1 << 10
	case 'm', 'M':
		multiplier = 1 << 20
	case 'g', 'G':
		multiplier = 1 << 30
	case 't', 'T':
		multiplier = 1 << 40
	case 'p', 'P':
		multiplier = 1 << 50
	default:
		return errors.Errorf("bad suffix %q", suffix)
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

// SizeSuffixList is a sclice SizeSuffix values
type SizeSuffixList []SizeSuffix

func (l SizeSuffixList) Len() int           { return len(l) }
func (l SizeSuffixList) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }
func (l SizeSuffixList) Less(i, j int) bool { return l[i] < l[j] }

// Sort sorts the list
func (l SizeSuffixList) Sort() {
	sort.Sort(l)
}
