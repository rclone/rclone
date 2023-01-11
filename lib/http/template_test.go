package http

import (
	"strings"
	"testing"
)

func TestHelpPrefixTemplate(t *testing.T) {
	// This test assumes template variables are placed correctly.
	const testPrefix = "template-help-test"
	helpMessage := TemplateHelp(testPrefix)
	if !strings.Contains(helpMessage, testPrefix) {
		t.Fatal("flag prefix not found")
	}
}
