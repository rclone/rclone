package gomods

import (
	"github.com/gobuffalo/envy"
	"github.com/markbates/safe"
	"github.com/pkg/errors"
)

const ENV = "GO111MODULE"

var ErrModsOff = errors.New("go mods are turned off")

func Force(b bool) {
	if b {
		envy.MustSet(ENV, "on")
		return
	}
	envy.MustSet(ENV, "off")
}

func On() bool {
	return envy.Mods()
}

func Disable(fn func() error) error {
	oe := envy.Get(ENV, "off")
	envy.MustSet(ENV, "off")

	err := safe.RunE(fn)
	envy.MustSet(ENV, oe)
	return err
}
