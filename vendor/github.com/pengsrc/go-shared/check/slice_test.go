package check

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringSliceContains(t *testing.T) {
	assert.True(t, StringSliceContains([]string{"1", "2", "3"}, "2"))
	assert.False(t, StringSliceContains([]string{"1", "2", "3"}, "4"))
}

func TestIntSliceContains(t *testing.T) {
	assert.True(t, IntSliceContains([]int{1, 2, 3, 4, 5, 6}, 4))
	assert.False(t, IntSliceContains([]int{1, 2, 3, 4, 5, 6}, 7))
}

func TestInt32SliceContains(t *testing.T) {
	assert.True(t, Int32SliceContains([]int32{1, 2, 3, 4, 5, 6}, 4))
	assert.False(t, Int32SliceContains([]int32{1, 2, 3, 4, 5, 6}, 7))
}

func TestInt64SliceContains(t *testing.T) {
	assert.True(t, Int64SliceContains([]int64{1, 2, 3, 4, 5, 6}, 4))
	assert.False(t, Int64SliceContains([]int64{1, 2, 3, 4, 5, 6}, 7))
}
