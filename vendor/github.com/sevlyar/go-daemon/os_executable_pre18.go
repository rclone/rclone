// +build !go1.8

package daemon

import (
	"github.com/kardianos/osext"
)

func osExecutable() (string, error) {
	return osext.Executable()
}
