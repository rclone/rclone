// +build go1.8

package daemon

import (
	"os"
)

func osExecutable() (string, error) {
	return os.Executable()
}
