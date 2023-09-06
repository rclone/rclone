package generics

func MakeMapIfNilAndSet[K comparable, V any](pm *map[K]V, k K, v V) {
	m := *pm
	if m == nil {
		m = make(map[K]V)
		*pm = m
	}
	m[k] = v
}

// Does this exist in the maps package?
func MakeMap[K comparable, V any](pm *map[K]V) {
	*pm = make(map[K]V)
}

func MakeMapIfNil[K comparable, V any, M ~map[K]V](pm *M) {
	if *pm == nil {
		*pm = make(map[K]V)
	}
}
