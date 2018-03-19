package check

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckDir(t *testing.T) {
	// Path not exist.
	assert.Error(t, Dir("/not-exist-dir"))

	// Path is not directory.
	f, err := ioutil.TempFile(os.TempDir(), "test-check-dir-")
	assert.NoError(t, err)
	f.Close()
	os.Remove(f.Name())

	// OK.
	assert.NoError(t, Dir(os.TempDir()))
}
