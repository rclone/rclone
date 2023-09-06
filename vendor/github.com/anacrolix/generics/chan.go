package generics

import "golang.org/x/exp/constraints"

// I can't seem to make a common function for things the make function works with. "no core type"
func MakeChanWithLen[T chan U, U any, L constraints.Integer](m *T, l L) {
	*m = make(T, l)
}
