package plugdeps

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gobuffalo/meta"
	"github.com/karrick/godirwalk"
	"github.com/pkg/errors"
)

// ErrMissingConfig is if config/buffalo-plugins.toml file is not found. Use plugdeps#On(app) to test if plugdeps are being used
var ErrMissingConfig = errors.Errorf("could not find a buffalo-plugins config file at %s", ConfigPath(meta.New(".")))

// List all of the plugins the application depeneds on. Will return ErrMissingConfig
// if the app is not using config/buffalo-plugins.toml to manage their plugins.
// Use plugdeps#On(app) to test if plugdeps are being used.
func List(app meta.App) (*Plugins, error) {
	plugs := New()
	if app.WithPop {
		plugs.Add(pop)
	}

	lp, err := listLocal(app)
	if err != nil {
		return plugs, errors.WithStack(err)
	}
	plugs.Add(lp.List()...)

	if !On(app) {
		return plugs, ErrMissingConfig
	}

	p := ConfigPath(app)
	tf, err := os.Open(p)
	if err != nil {
		return plugs, errors.WithStack(err)
	}
	if err := plugs.Decode(tf); err != nil {
		return plugs, errors.WithStack(err)
	}

	return plugs, nil
}

func listLocal(app meta.App) (*Plugins, error) {
	plugs := New()
	proot := filepath.Join(app.Root, "plugins")
	if _, err := os.Stat(proot); err != nil {
		return plugs, nil
	}
	err := godirwalk.Walk(proot, &godirwalk.Options{
		FollowSymbolicLinks: true,
		Callback: func(path string, info *godirwalk.Dirent) error {
			if info.IsDir() {
				return nil
			}
			base := filepath.Base(path)
			if strings.HasPrefix(base, "buffalo-") {
				plugs.Add(Plugin{
					Binary: base,
					Local:  "." + strings.TrimPrefix(path, app.Root),
				})
			}
			return nil
		},
	})
	if err != nil {
		return plugs, errors.WithStack(err)
	}

	return plugs, nil
}

// ConfigPath returns the path to the config/buffalo-plugins.toml file
// relative to the app
func ConfigPath(app meta.App) string {
	return filepath.Join(app.Root, "config", "buffalo-plugins.toml")
}

// On checks for the existence of config/buffalo-plugins.toml if this
// file exists its contents will be used to list plugins. If the file is not
// found, then the BUFFALO_PLUGIN_PATH and ./plugins folders are consulted.
func On(app meta.App) bool {
	_, err := os.Stat(ConfigPath(app))
	return err == nil
}
