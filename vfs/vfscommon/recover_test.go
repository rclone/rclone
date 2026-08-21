package vfscommon

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecoverPanic(t *testing.T) {
	// Panic recovered into err
	err := func() (err error) {
		defer RecoverPanic("test", &err)
		panic("boom")
	}()
	require.Error(t, err)
	assert.ErrorContains(t, err, "boom")

	// No panic leaves err alone
	err = func() (err error) {
		defer RecoverPanic("test", nil)
		return nil
	}()
	assert.NoError(t, err)

	// A nil error target is allowed, for goroutines with nothing to report to
	assert.NotPanics(t, func() {
		defer RecoverPanic("test", nil)
		panic("boom")
	})
}

func TestRecoverCall(t *testing.T) {
	assert.ErrorContains(t, RecoverCall("test", func() error { panic("boom") }), "boom")

	// Ordinary errors and results pass through untouched
	errFoo := errors.New("foo")
	assert.Equal(t, errFoo, RecoverCall("test", func() error { return errFoo }))
	assert.NoError(t, RecoverCall("test", func() error { return nil }))
}
