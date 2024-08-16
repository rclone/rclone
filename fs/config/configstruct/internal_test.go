package configstruct

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCamelToSnake(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
	}{
		{"", ""},
		{"Type", "type"},
		{"AuthVersion", "auth_version"},
		{"AccessKeyID", "access_key_id"},
	} {
		got := camelToSnake(test.in)
		assert.Equal(t, test.want, got, test.in)
	}
}
