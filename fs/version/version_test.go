package version

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	for _, test := range []struct {
		in      string
		want    Version
		wantErr bool
	}{
		{"v1.41", Version{1, 41}, false},
		{"rclone v1.41", Version{1, 41}, false},
		{"rclone v1.41.23", Version{1, 41, 23}, false},
		{"rclone v1.41.23-100", Version{1, 41, 23, 100}, false},
		{"rclone v1.41-100", Version{1, 41, 0, 100}, false},
		{"rclone v1.41.23-100-g12312a", Version{1, 41, 23, 100}, false},
		{"rclone v1.41-100-g12312a", Version{1, 41, 0, 100}, false},
		{"rclone v1.42-005-g56e1e820β", Version{1, 42, 0, 5}, false},
		{"rclone v1.42-005-g56e1e820-feature-branchβ", Version{1, 42, 0, 5}, false},

		{"v1.41s", nil, true},
		{"rclone v1-41", nil, true},
		{"rclone v1.41.2c3", nil, true},
		{"rclone v1.41.23-100 potato", nil, true},
		{"rclone 1.41-100", nil, true},
		{"rclone v1.41.23-100-12312a", nil, true},

		{"v1.41-DEV", Version{1, 41, 999, 999}, false},
	} {
		what := fmt.Sprintf("in=%q", test.in)
		got, err := New(test.in)
		if test.wantErr {
			assert.Error(t, err, what)
		} else {
			assert.NoError(t, err, what)
		}
		assert.Equal(t, test.want, got, what)
	}

}

func TestCmp(t *testing.T) {
	for _, test := range []struct {
		a, b Version
		want int
	}{
		{Version{1}, Version{1}, 0},
		{Version{1}, Version{2}, -1},
		{Version{2}, Version{1}, 1},
		{Version{2}, Version{2, 1}, -1},
		{Version{2, 1}, Version{2}, 1},
		{Version{2, 1}, Version{2, 1}, 0},
		{Version{2, 1}, Version{2, 2}, -1},
		{Version{2, 2}, Version{2, 1}, 1},
	} {
		got := test.a.Cmp(test.b)
		if got < 0 {
			got = -1
		} else if got > 0 {
			got = 1
		}
		assert.Equal(t, test.want, got, fmt.Sprintf("%v cmp %v", test.a, test.b))
		// test the reverse
		got = -test.b.Cmp(test.a)
		assert.Equal(t, test.want, got, fmt.Sprintf("%v cmp %v", test.b, test.a))
	}
}

func TestString(t *testing.T) {
	v, err := New("v1.44.1-2")
	assert.NoError(t, err)

	assert.Equal(t, "1.44.1.2", v.String())
}

func TestIsGit(t *testing.T) {
	v, err := New("v1.44")
	assert.NoError(t, err)
	assert.Equal(t, false, v.IsGit())

	v, err = New("v1.44-DEV")
	assert.NoError(t, err)
	assert.Equal(t, true, v.IsGit())
}
