package fshttp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanAuth(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
	}{
		{"", ""},
		{"floo", "floo"},
		{"Authorization: ", "Authorization: "},
		{"Authorization: \n", "Authorization: \n"},
		{"Authorization: A", "Authorization: X"},
		{"Authorization: A\n", "Authorization: X\n"},
		{"Authorization: AAAA", "Authorization: XXXX"},
		{"Authorization: AAAA\n", "Authorization: XXXX\n"},
		{"Authorization: AAAAA", "Authorization: XXXX"},
		{"Authorization: AAAAA\n", "Authorization: XXXX\n"},
		{"Authorization: AAAA\n", "Authorization: XXXX\n"},
		{"Authorization: AAAAAAAAA\nPotato: Help\n", "Authorization: XXXX\nPotato: Help\n"},
		{"Sausage: 1\nAuthorization: AAAAAAAAA\nPotato: Help\n", "Sausage: 1\nAuthorization: XXXX\nPotato: Help\n"},
	} {
		got := string(cleanAuth([]byte(test.in), authBufs[0]))
		assert.Equal(t, test.want, got, test.in)
	}
}

func TestCleanAuths(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
	}{
		{"", ""},
		{"floo", "floo"},
		{"Authorization: AAAAAAAAA\nPotato: Help\n", "Authorization: XXXX\nPotato: Help\n"},
		{"X-Auth-Token: AAAAAAAAA\nPotato: Help\n", "X-Auth-Token: XXXX\nPotato: Help\n"},
		{"X-Auth-Token: AAAAAAAAA\nAuthorization: AAAAAAAAA\nPotato: Help\n", "X-Auth-Token: XXXX\nAuthorization: XXXX\nPotato: Help\n"},
	} {
		got := string(cleanAuths([]byte(test.in)))
		assert.Equal(t, test.want, got, test.in)
	}
}
