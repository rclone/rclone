package lshelp

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutputStdout(t *testing.T) {
	OutputFile = ""
	out, closeOut, err := Output()
	require.NoError(t, err)
	assert.Equal(t, os.Stdout, out)
	closeOut()
}

func TestOutputFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "listing.txt")
	OutputFile = path
	defer func() { OutputFile = "" }()

	out, closeOut, err := Output()
	require.NoError(t, err)
	assert.NotEqual(t, os.Stdout, out)
	_, err = fmt.Fprintln(out, "hello")
	require.NoError(t, err)
	closeOut()

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "hello\n", string(got))
}

func TestOutputFileError(t *testing.T) {
	OutputFile = filepath.Join(t.TempDir(), "missing", "listing.txt")
	defer func() { OutputFile = "" }()

	out, closeOut, err := Output()
	assert.Error(t, err)
	assert.Nil(t, out)
	assert.Nil(t, closeOut)
}
