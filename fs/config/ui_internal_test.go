package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderHelpForTerminal(t *testing.T) {
	for _, test := range []struct {
		name string
		help string
		want string
	}{
		{
			name: "no link",
			help: "The encoding for the backend.",
			want: "The encoding for the backend.",
		},
		{
			name: "root relative link",
			help: "See the [encoding section in the overview](/overview/#encoding) for more info.",
			want: "See the encoding section in the overview (https://rclone.org/overview/#encoding) for more info.",
		},
		{
			name: "root relative link without anchor",
			help: "See [rclone serve sftp](/commands/rclone_serve_sftp) for details.",
			want: "See rclone serve sftp (https://rclone.org/commands/rclone_serve_sftp) for details.",
		},
		{
			name: "multiple links",
			help: "[the time option docs](/docs/#time-options) and [authentication docs](/azureblob#authentication).",
			want: "the time option docs (https://rclone.org/docs/#time-options) and authentication docs (https://rclone.org/azureblob#authentication).",
		},
		{
			name: "absolute url left untouched",
			help: "See [rclone forum](https://forum.rclone.org/) for help.",
			want: "See [rclone forum](https://forum.rclone.org/) for help.",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, renderHelpForTerminal(test.help))
		})
	}
}

func TestCheckPassword(t *testing.T) {
	// Empty password should give an error
	_, err := checkPassword("")
	require.Error(t, err)

	// Whitespace only should give an error
	_, err = checkPassword("  \t  ")
	require.Error(t, err)

	// Invalid utf8 should give an error
	_, err = checkPassword(string([]byte{0xff, 0xfe, 0xfd}) + "abc")
	require.Error(t, err)

	// Passwords must be returned verbatim, in particular without
	// Unicode normalization, so that obscured backend passwords reveal
	// to exactly what the user typed - see #9507. NFKC normalization
	// would turn ª (U+00AA) into a.
	pw := "ËQÖ4];giª±;<H'õPVzú6j¢ÔBí}×_-D%}"
	got, err := checkPassword(pw)
	require.NoError(t, err)
	assert.Equal(t, pw, got)
}
