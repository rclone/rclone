package config

import (
	"fmt"
	"testing"

	"github.com/rclone/rclone/fs/rc"
	"github.com/stretchr/testify/assert"
)

func TestArgsToMap(t *testing.T) {
	for _, test := range []struct {
		args    []string
		want    rc.Params
		wantErr bool
	}{
		{
			args: []string{},
			want: rc.Params{},
		},
		{
			args: []string{"hello", "42"},
			want: rc.Params{"hello": "42"},
		},
		{
			args: []string{"hello", "42", "bye", "43"},
			want: rc.Params{"hello": "42", "bye": "43"},
		},
		{
			args: []string{"hello=42", "bye", "43"},
			want: rc.Params{"hello": "42", "bye": "43"},
		},
		{
			args: []string{"hello", "42", "bye=43"},
			want: rc.Params{"hello": "42", "bye": "43"},
		},
		{
			args: []string{"hello=42", "bye=43"},
			want: rc.Params{"hello": "42", "bye": "43"},
		},
		{
			args:    []string{"hello", "42", "bye", "43", "unused"},
			wantErr: true,
		},
		{
			args:    []string{"hello=42", "bye=43", "unused"},
			wantErr: true,
		},
	} {
		what := fmt.Sprintf("args = %#v", test.args)
		got, err := argsToMap(test.args)
		if test.wantErr {
			assert.Error(t, err, what)
		} else {
			assert.NoError(t, err, what)
			assert.Equal(t, test.want, got, what)
		}
	}
}
