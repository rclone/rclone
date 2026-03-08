package obscure

import (
	"bytes"
	"testing"
	"context"

	"github.com/stretchr/testify/assert"

	"github.com/rclone/rclone/fs"
)

func TestObscure(t *testing.T) {
	ci := fs.GetConfig(context.Background())
	for _, test := range []struct {
		in   string
		want string
		iv   string
		plaintext bool
	}{
		{"", "YWFhYWFhYWFhYWFhYWFhYQ", "aaaaaaaaaaaaaaaa", false},
		{"potato", "YWFhYWFhYWFhYWFhYWFhYXMaGgIlEQ", "aaaaaaaaaaaaaaaa", false},
		{"potato", "YmJiYmJiYmJiYmJiYmJiYp3gcEWbAw", "bbbbbbbbbbbbbbbb", false},
		{"", "", "", true},
		{"potato", "potato", "", true},
	} {
		ci.PlaintextPasswords = test.plaintext
		cryptRand = bytes.NewBufferString(test.iv)
		got, err := Obscure(test.in)
		assert.NoError(t, err)
		assert.Equal(t, test.want, got)
		recoveredIn, err := Reveal(got)
		assert.NoError(t, err)
		assert.Equal(t, test.in, recoveredIn, "not bidirectional")
		// Now the Must variants
		cryptRand = bytes.NewBufferString(test.iv)
		got = MustObscure(test.in)
		assert.Equal(t, test.want, got)
		recoveredIn = MustReveal(got)
		assert.Equal(t, test.in, recoveredIn, "not bidirectional")

	}
}

func TestReveal(t *testing.T) {
	ci := fs.GetConfig(context.Background())
	for _, test := range []struct {
		in   string
		want string
		plaintext bool
	}{
		{"YWFhYWFhYWFhYWFhYWFhYQ", "", false},
		{"YWFhYWFhYWFhYWFhYWFhYXMaGgIlEQ", "potato", false},
		{"YmJiYmJiYmJiYmJiYmJiYp3gcEWbAw", "potato", false},
		{"", "", true},
		{"potato", "potato", true},
	} {
		ci.PlaintextPasswords = test.plaintext
		got, err := Reveal(test.in)
		assert.NoError(t, err)
		assert.Equal(t, test.want, got)
		// Now the Must variants
		got = MustReveal(test.in)
		assert.Equal(t, test.want, got)

	}
}

// Test some error cases
func TestRevealErrors(t *testing.T) {
	fs.GetConfig(context.Background()).PlaintextPasswords = false
	for _, test := range []struct {
		in      string
		wantErr string
	}{
		{"YmJiYmJiYmJiYmJiYmJiYp*gcEWbAw", "base64 decode failed when revealing password - is it obscured?: illegal base64 data at input byte 22"},
		{"aGVsbG8", "input too short when revealing password - is it obscured?"},
		{"", "input too short when revealing password - is it obscured?"},
	} {
		gotString, gotErr := Reveal(test.in)
		assert.Equal(t, "", gotString)
		assert.Equal(t, test.wantErr, gotErr.Error())
	}
}
