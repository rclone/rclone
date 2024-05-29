package errcount

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrCount(t *testing.T) {
	ec := New()
	assert.Equal(t, nil, ec.Err("none"))

	e1 := errors.New("one")
	ec.Add(e1)

	err := ec.Err("stuff")
	assert.True(t, errors.Is(err, e1), err)
	assert.Equal(t, "stuff: one", err.Error())

	e2 := errors.New("two")
	ec.Add(e2)

	err = ec.Err("stuff")
	assert.True(t, errors.Is(err, e2), err)
	assert.Equal(t, "stuff: 2 errors: last error: two", err.Error())
}
