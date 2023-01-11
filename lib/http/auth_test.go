package http

import (
	"strings"
	"testing"
)

func TestHelpPrefixAuth(t *testing.T) {
	// This test assumes template variables are placed correctly.
	const testPrefix = "server-help-test"
	helpMessage := AuthHelp(testPrefix)
	if !strings.Contains(helpMessage, testPrefix) {
		t.Fatal("flag prefix not found")
	}
}
