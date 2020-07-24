package genautocomplete

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompletionBash(t *testing.T) {
	tempFile, err := ioutil.TempFile("", "completion_bash")
	assert.NoError(t, err)
	defer func() { _ = tempFile.Close() }()
	defer func() { _ = os.Remove(tempFile.Name()) }()

	bashCommandDefinition.Run(bashCommandDefinition, []string{tempFile.Name()})

	bs, err := ioutil.ReadFile(tempFile.Name())
	assert.NoError(t, err)
	assert.NotEmpty(t, string(bs))
}

func TestCompletionZsh(t *testing.T) {
	tempFile, err := ioutil.TempFile("", "completion_zsh")
	assert.NoError(t, err)
	defer func() { _ = tempFile.Close() }()
	defer func() { _ = os.Remove(tempFile.Name()) }()

	zshCommandDefinition.Run(zshCommandDefinition, []string{tempFile.Name()})

	bs, err := ioutil.ReadFile(tempFile.Name())
	assert.NoError(t, err)
	assert.NotEmpty(t, string(bs))
}

func TestCompletionFish(t *testing.T) {
	tempFile, err := ioutil.TempFile("", "completion_fish")
	assert.NoError(t, err)
	defer func() { _ = tempFile.Close() }()
	defer func() { _ = os.Remove(tempFile.Name()) }()

	fishCommandDefinition.Run(fishCommandDefinition, []string{tempFile.Name()})

	bs, err := ioutil.ReadFile(tempFile.Name())
	assert.NoError(t, err)
	assert.NotEmpty(t, string(bs))
}
