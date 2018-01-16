package convert

import (
	"strings"
	"testing"
)

func TestStringSliceWithConverter(t *testing.T) {
	s := StringSliceWithConverter([]string{"A", "b", "C"}, strings.ToLower)
	e := []string{"a", "b", "c"}
	if s[0] != e[0] || s[1] != e[1] || s[2] != e[2] {
		t.Errorf("%v != %v", s, e)
	}
}
