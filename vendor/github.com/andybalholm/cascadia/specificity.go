package cascadia

// Specificity is the CSS specificity as defined in
// https://www.w3.org/TR/selectors/#specificity-rules
// with the convention Specificity = [A,B,C].
type Specificity [3]int

// returns `true` if s < other (strictly), false otherwise
func (s Specificity) Less(other Specificity) bool {
	for i := range s {
		if s[i] < other[i] {
			return true
		}
		if s[i] > other[i] {
			return false
		}
	}
	return false
}

func (s Specificity) Add(other Specificity) Specificity {
	for i, sp := range other {
		s[i] += sp
	}
	return s
}
