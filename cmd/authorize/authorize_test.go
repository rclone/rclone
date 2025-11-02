package authorize

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestAuthorizeCommand(t *testing.T) {
	// Test that the Use string is correctly formatted
	if commandDefinition.Use != "authorize <backendname> [base64_json_blob | client_id client_secret]" {
		t.Errorf("Command Use string doesn't match expected format: %s", commandDefinition.Use)
	}

	// Test that help output contains the argument information
	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.AddCommand(commandDefinition)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"authorize", "--help"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Failed to execute help command: %v", err)
	}

	helpOutput := buf.String()
	if !strings.Contains(helpOutput, "authorize <backendname>") {
		t.Errorf("Help output doesn't contain correct usage information")
	}
}
