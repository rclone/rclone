package convert

// StringSliceWithConverter converts a list of string using the passed converter function
func StringSliceWithConverter(s []string, c func(string) string) []string {
	out := []string{}
	for _, i := range s {
		out = append(out, c(i))
	}
	return out
}
