// Package env contains functions for dealing with environment variables
package env

import (
	"os"

	homedir "github.com/mitchellh/go-homedir"
)

// ShellExpandHelp describes what ShellExpand does for inclusion into help
const ShellExpandHelp = "\n\nLeading `~` will be expanded in the file name as will environment variables such as `${RCLONE_CONFIG_DIR}`.\n"

// ShellExpand replaces a leading "~" with the home directory" and
// expands all environment variables afterwards.
func ShellExpand(s string) string {
	if s != "" {
		if s[0] == '~' {
			newS, err := homedir.Expand(s)
			if err == nil {
				s = newS
			}
		}
		s = os.ExpandEnv(s)
	}
	return s
}
