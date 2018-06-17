package utils

import (
	"os/user"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetHome(t *testing.T) {
	usr, err := user.Current()
	assert.NoError(t, err)

	assert.Equal(t, usr.HomeDir, GetHome())
}
