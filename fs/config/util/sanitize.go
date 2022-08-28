package util

// SanitizeSensitiveValue replaces a logged value with a non-sensitive value if the value is deemed sensitive for
// logging purposes
func SanitizeSensitiveValue(value string, isPassword bool) string {
	if isPassword {
		return "<sensitive value>"
	}

	return value
}
