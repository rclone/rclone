//go:build !windows

package oauthutil

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBindErrorHintOther(t *testing.T) {
	// Non-Windows builds never add a hint.
	assert.Empty(t, bindErrorHint(nil))
	assert.Empty(t, bindErrorHint(os.ErrPermission))
	assert.Empty(t, bindErrorHint(errors.New("listen tcp 127.0.0.1:53682: bind: permission denied")))
}
