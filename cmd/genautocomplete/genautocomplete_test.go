package genautocomplete

import (
	"os"
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
