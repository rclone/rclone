package bucket

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplit(t *testing.T) {
	for _, test := range []struct {
		in         string
		wantBucket string
		wantPath   string
	}{
		{in: "", wantBucket: "", wantPath: ""},
		{in: "bucket", wantBucket: "bucket", wantPath: ""},
		{in: "bucket/path", wantBucket: "bucket", wantPath: "path"},
		{in: "bucket/path/subdir", wantBucket: "bucket", wantPath: "path/subdir"},
	} {
		gotBucket, gotPath := Split(test.in)
		assert.Equal(t, test.wantBucket, gotBucket, test.in)
		assert.Equal(t, test.wantPath, gotPath, test.in)
	}
}

func TestCache(t *testing.T) {
	c := NewCache()
	errBoom := errors.New("boom")

	assert.Equal(t, 0, len(c.status))

	// IsDeleted before creation
	assert.False(t, c.IsDeleted("bucket"))

	// MarkOK

	c.MarkOK("")
	assert.Equal(t, 0, len(c.status))

	// MarkOK again

	c.MarkOK("bucket")
	assert.Equal(t, map[string]bool{"bucket": true}, c.status)

	// MarkDeleted

	c.MarkDeleted("bucket")
	assert.Equal(t, map[string]bool{"bucket": false}, c.status)

	// MarkOK again

	c.MarkOK("bucket")
	assert.Equal(t, map[string]bool{"bucket": true}, c.status)

	// IsDeleted after creation
	assert.False(t, c.IsDeleted("bucket"))

	// Create from root

	err := c.Create("", nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, map[string]bool{"bucket": true}, c.status)

	// Create bucket that is already OK

	err = c.Create("bucket", nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, map[string]bool{"bucket": true}, c.status)

	// Create new bucket

	err = c.Create("bucket2", func() error {
		return nil
	}, func() (bool, error) {
		return true, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, map[string]bool{"bucket": true, "bucket2": true}, c.status)

	// Create bucket that has been deleted with error

	c.status["bucket2"] = false // mark bucket deleted
	err = c.Create("bucket2", nil, func() (bool, error) {
		return false, errBoom
	})
	assert.Equal(t, errBoom, err)
	assert.Equal(t, map[string]bool{"bucket": true, "bucket2": false}, c.status)

	// Create bucket that has been deleted with no error

	err = c.Create("bucket2", nil, func() (bool, error) {
		return true, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, map[string]bool{"bucket": true, "bucket2": true}, c.status)

	// Create a new bucket with no exists function

	err = c.Create("bucket3", func() error {
		return nil
	}, nil)
	assert.NoError(t, err)
	assert.Equal(t, map[string]bool{"bucket": true, "bucket2": true, "bucket3": true}, c.status)

	// Create a new bucket with no exists function with an error

	err = c.Create("bucket4", func() error {
		return errBoom
	}, nil)
	assert.Equal(t, errBoom, err)
	assert.Equal(t, map[string]bool{"bucket": true, "bucket2": true, "bucket3": true}, c.status)

	// Remove root

	err = c.Remove("", func() error {
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, map[string]bool{"bucket": true, "bucket2": true, "bucket3": true}, c.status)

	// Remove existing bucket

	err = c.Remove("bucket3", func() error {
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, map[string]bool{"bucket": true, "bucket2": true, "bucket3": false}, c.status)

	// IsDeleted after removal
	assert.True(t, c.IsDeleted("bucket3"))

	// Remove it again

	err = c.Remove("bucket3", func() error {
		return errBoom
	})
	assert.Equal(t, ErrAlreadyDeleted, err)
	assert.Equal(t, map[string]bool{"bucket": true, "bucket2": true, "bucket3": false}, c.status)

	// Remove bucket with error

	err = c.Remove("bucket2", func() error {
		return errBoom
	})
	assert.Equal(t, errBoom, err)
	assert.Equal(t, map[string]bool{"bucket": true, "bucket2": true, "bucket3": false}, c.status)
}
