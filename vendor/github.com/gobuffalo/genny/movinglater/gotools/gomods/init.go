package gomods

import (
	"go/build"
	"os"
	"os/exec"
	"strings"

	"github.com/gobuffalo/genny"
	"github.com/pkg/errors"
)

func New(name string, path string) (*genny.Group, error) {
	g := &genny.Group{}

	init, err := Init(name, path)
	if err != nil {
		return g, errors.WithStack(err)
	}
	g.Add(init)

	tidy, err := Tidy(path, false)
	if err != nil {
		return g, errors.WithStack(err)
	}
	g.Add(tidy)
	return g, nil
}

func Init(name string, path string) (*genny.Generator, error) {
	if len(name) == 0 && len(path) == 0 {
		pwd, err := os.Getwd()
		if err != nil {
			return nil, errors.WithStack(err)
		}
		path = pwd
	}

	if len(name) == 0 && path != "." {
		name = path
		c := build.Default
		for _, s := range c.SrcDirs() {
			name = strings.TrimPrefix(name, s)
		}
	}

	name = strings.Replace(name, "\\", "/", -1)
	name = strings.TrimPrefix(name, "/")

	g := genny.New()
	g.RunFn(func(r *genny.Runner) error {
		if !On() {
			return nil
		}
		return r.Chdir(path, func() error {
			args := []string{"mod", "init"}
			if len(name) > 0 {
				args = append(args, name)
			}
			cmd := exec.Command(genny.GoBin(), args...)
			return r.Exec(cmd)
		})
	})
	return g, nil
}
