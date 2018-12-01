package gotools

import (
	"os/exec"

	"github.com/gobuffalo/genny"
	"github.com/gobuffalo/genny/movinglater/gotools/gomods"
)

func Get(pkg string, args ...string) genny.RunFn {
	return func(r *genny.Runner) error {
		args = append([]string{"get"}, args...)
		args = append(args, pkg)
		cmd := exec.Command(genny.GoBin(), args...)
		return r.Exec(cmd)
	}
}

func Install(pkg string, args ...string) genny.RunFn {
	return func(r *genny.Runner) error {
		return gomods.Disable(func() error {
			return Get(pkg, args...)(r)
		})
	}
}
