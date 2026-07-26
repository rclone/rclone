package genautocomplete

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompletionBash(t *testing.T) {
	tempFile, err := os.CreateTemp("", "completion_bash")
	assert.NoError(t, err)
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}()

	bashCommandDefinition.Run(bashCommandDefinition, []string{tempFile.Name()})

	bs, err := os.ReadFile(tempFile.Name())
	assert.NoError(t, err)
	assert.NotEmpty(t, string(bs))
}

func TestCompletionBashStdout(t *testing.T) {
	originalStdout := os.Stdout
	tempFile, err := os.CreateTemp("", "completion_zsh")
	assert.NoError(t, err)
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}()

	os.Stdout = tempFile
	defer func() { os.Stdout = originalStdout }()

	bashCommandDefinition.Run(bashCommandDefinition, []string{"-"})

	output, err := os.ReadFile(tempFile.Name())
	assert.NoError(t, err)
	assert.NotEmpty(t, string(output))
}

func TestCompletionZsh(t *testing.T) {
	tempFile, err := os.CreateTemp("", "completion_zsh")
	assert.NoError(t, err)
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}()

	zshCommandDefinition.Run(zshCommandDefinition, []string{tempFile.Name()})

	bs, err := os.ReadFile(tempFile.Name())
	assert.NoError(t, err)
	assert.NotEmpty(t, string(bs))
}

func TestCompletionZshStdout(t *testing.T) {
	originalStdout := os.Stdout
	tempFile, err := os.CreateTemp("", "completion_zsh")
	assert.NoError(t, err)
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}()

	os.Stdout = tempFile
	defer func() { os.Stdout = originalStdout }()

	zshCommandDefinition.Run(zshCommandDefinition, []string{"-"})
	output, err := os.ReadFile(tempFile.Name())
	assert.NoError(t, err)
	assert.NotEmpty(t, string(output))
}

func TestCompletionFish(t *testing.T) {
	tempFile, err := os.CreateTemp("", "completion_fish")
	assert.NoError(t, err)
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}()

	fishCommandDefinition.Run(fishCommandDefinition, []string{tempFile.Name()})

	bs, err := os.ReadFile(tempFile.Name())
	assert.NoError(t, err)
	assert.NotEmpty(t, string(bs))
}

func TestCompletionFishStdout(t *testing.T) {
	originalStdout := os.Stdout
	tempFile, err := os.CreateTemp("", "completion_zsh")
	assert.NoError(t, err)
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}()

	os.Stdout = tempFile
	defer func() { os.Stdout = originalStdout }()

	fishCommandDefinition.Run(fishCommandDefinition, []string{"-"})

	output, err := os.ReadFile(tempFile.Name())
	assert.NoError(t, err)
	assert.NotEmpty(t, string(output))
}

func TestCompletionPowershell(t *testing.T) {
	tempFile, err := os.CreateTemp("", "completion_powershell")
	assert.NoError(t, err)
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}()

	powershellCommandDefinition.Run(powershellCommandDefinition, []string{tempFile.Name()})

	bs, err := os.ReadFile(tempFile.Name())
	assert.NoError(t, err)
	assert.NotEmpty(t, string(bs))
	// The generated script must force UTF-8 output decoding so that non-ASCII
	// remote names are not corrupted on non-UTF-8 PowerShell hosts.
	assert.Contains(t, string(bs), powerShellUTF8Fix)
}

func TestCompletionPowershellStdout(t *testing.T) {
	originalStdout := os.Stdout
	tempFile, err := os.CreateTemp("", "completion_powershell")
	assert.NoError(t, err)
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}()

	os.Stdout = tempFile
	defer func() { os.Stdout = originalStdout }()

	powershellCommandDefinition.Run(powershellCommandDefinition, []string{"-"})

	output, err := os.ReadFile(tempFile.Name())
	assert.NoError(t, err)
	assert.NotEmpty(t, string(output))
	assert.Contains(t, string(output), powerShellUTF8Fix)
}

func TestPatchPowerShellCompletion(t *testing.T) {
	t.Run("injects the encoding fix before the invoke line", func(t *testing.T) {
		script := "before\n    " + powerShellInvokeLine + "\nafter\n"
		got := patchPowerShellCompletion(script)
		// The fix is inserted on its own line, sharing the indentation of the
		// invoke line, immediately before it.
		want := "before\n    " + powerShellUTF8Fix + "\n    " + powerShellInvokeLine + "\nafter\n"
		assert.Equal(t, want, got)
		assert.Less(t, strings.Index(got, powerShellUTF8Fix), strings.Index(got, powerShellInvokeLine))
	})

	t.Run("leaves the script unchanged when the invoke line is absent", func(t *testing.T) {
		script := "some other script\nwithout the expected line\n"
		assert.Equal(t, script, patchPowerShellCompletion(script))
	})
}
