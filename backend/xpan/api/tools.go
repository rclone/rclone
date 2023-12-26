package api

import "strings"

// ArrayValue convert array to api query parameter value
func ArrayValue(array []string, prefix string) string {
	return "[\"" + prefix + strings.Join(array, "\",\""+prefix) + "\"]"
}
