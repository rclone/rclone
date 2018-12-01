package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gobuffalo/buffalo-plugins/plugins/plugdeps"
	"github.com/gobuffalo/envy"
	"github.com/gobuffalo/meta"
	"github.com/karrick/godirwalk"
	"github.com/markbates/oncer"
	"github.com/sirupsen/logrus"
)

const timeoutEnv = "BUFFALO_PLUGIN_TIMEOUT"

func timeout() time.Duration {
	t := time.Second
	oncer.Do("plugins.timeout", func() {
		rawTimeout, err := envy.MustGet(timeoutEnv)
		if err == nil {
			if parsed, err := time.ParseDuration(rawTimeout); err == nil {
				t = parsed
			} else {
				logrus.Errorf("%q value is malformed assuming default %q: %v", timeoutEnv, t, err)
			}
		} else {
			logrus.Debugf("%q not set, assuming default of %v", timeoutEnv, t)
		}
	})
	return t
}

// List maps a Buffalo command to a slice of Command
type List map[string]Commands

var _list List

// Available plugins for the `buffalo` command.
// It will look in $GOPATH/bin and the `./plugins` directory.
// This can be changed by setting the $BUFFALO_PLUGIN_PATH
// environment variable.
//
// Requirements:
// * file/command must be executable
// * file/command must start with `buffalo-`
// * file/command must respond to `available` and return JSON of
//	 plugins.Commands{}
//
// Limit full path scan with direct plugin path
//
// If a file/command doesn't respond to being invoked with `available`
// within one second, buffalo will assume that it is unable to load. This
// can be changed by setting the $BUFFALO_PLUGIN_TIMEOUT environment
// variable. It must be set to a duration that `time.ParseDuration` can
// process.
func Available() (List, error) {
	var err error
	oncer.Do("plugins.Available", func() {

		app := meta.New(".")

		if plugdeps.On(app) {
			_list, err = listPlugDeps(app)
			return
		}

		paths := []string{"plugins"}

		from, err := envy.MustGet("BUFFALO_PLUGIN_PATH")
		if err != nil {
			from, err = envy.MustGet("GOPATH")
			if err != nil {
				return
			}
			from = filepath.Join(from, "bin")
		}

		paths = append(paths, strings.Split(from, string(os.PathListSeparator))...)

		list := List{}
		for _, p := range paths {
			if ignorePath(p) {
				continue
			}
			if _, err := os.Stat(p); err != nil {
				continue
			}

			err := godirwalk.Walk(p, &godirwalk.Options{
				FollowSymbolicLinks: true,
				Callback: func(path string, info *godirwalk.Dirent) error {
					if err != nil {
						// May indicate a permissions problem with the path, skip it
						return nil
					}
					if info.IsDir() {
						return nil
					}
					base := filepath.Base(path)
					if strings.HasPrefix(base, "buffalo-") {
						ctx, cancel := context.WithTimeout(context.Background(), timeout())
						commands := askBin(ctx, path)
						cancel()
						for _, c := range commands {
							bc := c.BuffaloCommand
							if _, ok := list[bc]; !ok {
								list[bc] = Commands{}
							}
							c.Binary = path
							list[bc] = append(list[bc], c)
						}
					}
					return nil
				},
			})

			if err != nil {
				return
			}
		}
		_list = list
	})
	return _list, err
}

func askBin(ctx context.Context, path string) Commands {
	commands := Commands{}

	cmd := exec.CommandContext(ctx, path, "available")
	bb := &bytes.Buffer{}
	cmd.Stdout = bb
	err := cmd.Run()
	if err != nil {
		return commands
	}
	msg := bb.String()
	for len(msg) > 0 {
		err = json.NewDecoder(strings.NewReader(msg)).Decode(&commands)
		if err == nil {
			return commands
		}
		msg = msg[1:]
	}
	logrus.Errorf("[PLUGIN] error decoding plugin %s: %s\n%s\n", path, err, msg)
	return commands
}

func ignorePath(p string) bool {
	p = strings.ToLower(p)
	for _, x := range []string{`c:\windows`, `c:\program`} {
		if strings.HasPrefix(p, x) {
			return true
		}
	}
	return false
}

func listPlugDeps(app meta.App) (List, error) {
	list := List{}
	plugs, err := plugdeps.List(app)
	if err != nil {
		return list, err
	}
	for _, p := range plugs.List() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout())
		bin := p.Binary
		if len(p.Local) != 0 {
			bin = p.Local
		}
		commands := askBin(ctx, bin)
		cancel()
		for _, c := range commands {
			bc := c.BuffaloCommand
			if _, ok := list[bc]; !ok {
				list[bc] = Commands{}
			}
			c.Binary = p.Binary
			for _, pc := range p.Commands {
				if c.Name == pc.Name {
					c.Flags = pc.Flags
					break
				}
			}
			list[bc] = append(list[bc], c)
		}
	}
	return list, nil
}
