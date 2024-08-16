package union

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	err1 = errors.New("Error 1")
	err2 = errors.New("Error 2")
	err3 = errors.New("Error 3")
)

func TestErrorsMap(t *testing.T) {
	es := Errors{
		nil,
		err1,
		err2,
	}
	want := Errors{
		err2,
	}
	got := es.Map(func(e error) error {
		if e == err1 {
			return nil
		}
		return e
	})
	assert.Equal(t, want, got)
}

func TestErrorsFilterNil(t *testing.T) {
	es := Errors{
		nil,
		err1,
		nil,
		err2,
		nil,
	}
	want := Errors{
		err1,
		err2,
	}
	got := es.FilterNil()
	assert.Equal(t, want, got)
}

func TestErrorsErr(t *testing.T) {
	// Check not all nil case
	es := Errors{
		nil,
		err1,
		nil,
		err2,
		nil,
	}
	want := Errors{
		err1,
		err2,
	}
	got := es.Err()

	// Check all nil case
	assert.Equal(t, want, got)
	es = Errors{
		nil,
		nil,
		nil,
	}
	assert.Nil(t, es.Err())
}

func TestErrorsError(t *testing.T) {
	assert.Equal(t, "no error", Errors{}.Error())
	assert.Equal(t, "1 error: Error 1", Errors{err1}.Error())
	assert.Equal(t, "1 error: nil error", Errors{nil}.Error())
	assert.Equal(t, "2 errors: Error 1; Error 2", Errors{err1, err2}.Error())
}

func TestErrorsUnwrap(t *testing.T) {
	es := Errors{
		err1,
		err2,
	}
	assert.Equal(t, []error{err1, err2}, es.Unwrap())
	assert.True(t, errors.Is(es, err1))
	assert.True(t, errors.Is(es, err2))
	assert.False(t, errors.Is(es, err3))
}
