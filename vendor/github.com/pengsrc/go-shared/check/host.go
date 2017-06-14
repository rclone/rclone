package check

import (
	"regexp"
)

// HostAndPort checks whether a string contains host and port.
// It returns true if matched.
func HostAndPort(hostAndPort string) bool {
	return regexp.MustCompile(`^[^:]+:[0-9]+$`).MatchString(hostAndPort)
}
