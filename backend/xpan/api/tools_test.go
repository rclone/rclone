package api

import "testing"

func TestArrayValue(t *testing.T) {
	array := []string{"a", "b", "c"}
	prefix := "prefix"
	expected := "[\"prefixa\",\"prefixb\",\"prefixc\"]"
	actual := ArrayValue(array, prefix)
	if actual != expected {
		t.Errorf("ArrayValue(%q, %q) = %q, want %q", array, prefix, actual, expected)
	}
}
