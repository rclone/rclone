package meta

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/gobuffalo/envy"
	"github.com/gobuffalo/flect/name"
	"github.com/pkg/errors"
)

// App represents meta data for a Buffalo application on disk
type App struct {
	Pwd         string     `json:"pwd" toml:"-"`
	Root        string     `json:"root" toml:"-"`
	GoPath      string     `json:"go_path" toml:"-"`
	PackagePkg  string     `json:"package_path" toml:"-"`
	ActionsPkg  string     `json:"actions_path" toml:"-"`
	ModelsPkg   string     `json:"models_path" toml:"-"`
	GriftsPkg   string     `json:"grifts_path" toml:"-"`
	WithModules bool       `json:"with_modules" toml:"-"`
	Name        name.Ident `json:"name" toml:"name"`
	Bin         string     `json:"bin" toml:"bin"`
	VCS         string     `json:"vcs" toml:"vcs"`
	WithPop     bool       `json:"with_pop" toml:"with_pop"`
	WithSQLite  bool       `json:"with_sqlite" toml:"with_sqlite"`
	WithDep     bool       `json:"with_dep" toml:"with_dep"`
	WithWebpack bool       `json:"with_webpack" toml:"with_webpack"`
	WithYarn    bool       `json:"with_yarn" toml:"with_yarn"`
	WithDocker  bool       `json:"with_docker" toml:"with_docker"`
	WithGrifts  bool       `json:"with_grifts" toml:"with_grifts"`
	AsWeb       bool       `json:"as_web" toml:"as_web"`
	AsAPI       bool       `json:"as_api" toml:"as_api"`
}

func (a App) IsZero() bool {
	return a.String() == App{}.String()
}

func resolvePackageName(name string, pwd string) string {
	result := envy.CurrentPackage()

	if filepath.Base(result) != name {
		result = path.Join(result, name)
	}

	if envy.Mods() {
		if !strings.HasPrefix(pwd, filepath.Join(envy.GoPath(), "src")) {
			result = name
		}

		//Extract package from go.mod
		if f, err := os.Open(filepath.Join(pwd, "go.mod")); err == nil {
			if s, err := ioutil.ReadAll(f); err == nil {
				re := regexp.MustCompile("module (.*)")
				res := re.FindAllStringSubmatch(string(s), 1)

				if len(res) == 1 && len(res[0]) == 2 {
					result = res[0][1]
				}
			}
		}
	}

	return result
}

// ResolveSymlinks takes a path and gets the pointed path
// if the original one is a symlink.
func ResolveSymlinks(p string) string {
	cd, err := os.Lstat(p)
	if err != nil {
		return p
	}
	if cd.Mode()&os.ModeSymlink != 0 {
		// This is a symlink
		r, err := filepath.EvalSymlinks(p)
		if err != nil {
			return p
		}
		return r
	}
	return p
}

func (a App) String() string {
	b, _ := json.Marshal(a)
	return string(b)
}

// Encode the list of plugins, in TOML format, to the reader
func (a App) Encode(w io.Writer) error {
	if err := toml.NewEncoder(w).Encode(a); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// Decode the list of plugins, in TOML format, from the reader
func (a *App) Decode(r io.Reader) error {
	xa := New(".")
	if _, err := toml.DecodeReader(r, &xa); err != nil {
		return errors.WithStack(err)
	}
	(*a) = xa
	return nil
}

// PackageRoot sets the root package of the application and
// recalculates package related values
func (a *App) PackageRoot(pp string) {
	a.PackagePkg = pp
	a.ActionsPkg = pp + "/actions"
	a.ModelsPkg = pp + "/models"
	a.GriftsPkg = pp + "/grifts"
}
