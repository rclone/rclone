package combine

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdjustmentDo(t *testing.T) {
	for _, test := range []struct {
		root       string
		mountpoint string
		in         string
		want       string
		wantErr    error
	}{
		{
			root:       "",
			mountpoint: "mountpoint",
			in:         "path/to/file.txt",
			want:       "mountpoint/path/to/file.txt",
		},
		{
			root:       "mountpoint",
			mountpoint: "mountpoint",
			in:         "path/to/file.txt",
			want:       "path/to/file.txt",
		},
		{
			root:       "mountpoint/path",
			mountpoint: "mountpoint",
			in:         "path/to/file.txt",
			want:       "to/file.txt",
		},
		{
			root:       "mountpoint/path",
			mountpoint: "mountpoint",
			in:         "wrongpath/to/file.txt",
			want:       "",
			wantErr:    errNotUnderRoot,
		},
	} {
		what := fmt.Sprintf("%+v", test)
		a := newAdjustment(test.root, test.mountpoint)
		got, gotErr := a.do(test.in)
		assert.Equal(t, test.wantErr, gotErr)
		assert.Equal(t, test.want, got, what)
	}

}

func TestAdjustmentUndo(t *testing.T) {
	for _, test := range []struct {
		root       string
		mountpoint string
		in         string
		want       string
		wantErr    error
	}{
		{
			root:       "",
			mountpoint: "mountpoint",
			in:         "mountpoint/path/to/file.txt",
			want:       "path/to/file.txt",
		},
		{
			root:       "mountpoint",
			mountpoint: "mountpoint",
			in:         "path/to/file.txt",
			want:       "path/to/file.txt",
		},
		{
			root:       "mountpoint/path",
			mountpoint: "mountpoint",
			in:         "to/file.txt",
			want:       "path/to/file.txt",
		},
		{
			root:       "wrongmountpoint/path",
			mountpoint: "mountpoint",
			in:         "to/file.txt",
			want:       "",
			wantErr:    errNotUnderRoot,
		},
	} {
		what := fmt.Sprintf("%+v", test)
		a := newAdjustment(test.root, test.mountpoint)
		got, gotErr := a.undo(test.in)
		assert.Equal(t, test.wantErr, gotErr)
		assert.Equal(t, test.want, got, what)
	}

}
