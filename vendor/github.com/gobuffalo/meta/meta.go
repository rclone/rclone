package meta

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/BurntSushi/toml"
	"github.com/gobuffalo/envy"

	fname "github.com/gobuffalo/flect/name"
)

func Named(name string, root string) App {
	pwd, _ := os.Getwd()
	if root == "." {
		root = pwd
	}

	// Handle symlinks
	var oldPwd = pwd
	pwd = ResolveSymlinks(pwd)
	os.Chdir(pwd)
	if runtime.GOOS != "windows" {
		// On Non-Windows OS, os.Getwd() uses PWD env var as a preferred
		// way to get the working dir.
		os.Setenv("PWD", pwd)
	}
	defer func() {
		// Restore PWD
		os.Chdir(oldPwd)
		if runtime.GOOS != "windows" {
			os.Setenv("PWD", oldPwd)
		}
	}()

	// Gather meta data
	if len(name) == 0 {
		name = filepath.Base(root)
	}
	pp := resolvePackageName(name, pwd)

	app := App{
		Pwd:         pwd,
		Root:        root,
		GoPath:      envy.GoPath(),
		Name:        fname.New(name),
		WithModules: envy.Mods(),
		AsAPI:       false,
		AsWeb:       true,
	}
	app.PackageRoot(pp)

	app.Bin = filepath.Join("bin", app.Name.String())

	if runtime.GOOS == "windows" {
		app.Bin += ".exe"
	}

	cf, err := os.Open(filepath.Join(app.Root, "config", "buffalo-app.toml"))
	if err != nil {
		return oldSchool(app)
	}
	defer cf.Close()

	if _, err := toml.DecodeReader(cf, &app); err != nil {
		fmt.Println(err)
	}

	return app
}

// New App based on the details found at the provided root path
func New(root string) App {
	return Named("", root)
}

func oldSchool(app App) App {
	root := app.Root
	db := filepath.Join(root, "database.yml")
	if _, err := os.Stat(db); err == nil {
		app.WithPop = true
		if b, err := ioutil.ReadFile(db); err == nil {
			app.WithSQLite = bytes.Contains(bytes.ToLower(b), []byte("sqlite"))
		}
	}
	if _, err := os.Stat(filepath.Join(root, "Gopkg.toml")); err == nil {
		app.WithDep = true
	}
	if _, err := os.Stat(filepath.Join(root, "webpack.config.js")); err == nil {
		app.WithWebpack = true
	}
	if _, err := os.Stat(filepath.Join(root, "yarn.lock")); err == nil {
		app.WithYarn = true
	}
	if _, err := os.Stat(filepath.Join(root, "Dockerfile")); err == nil {
		app.WithDocker = true
	}
	if _, err := os.Stat(filepath.Join(root, "grifts")); err == nil {
		app.WithGrifts = true
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		app.VCS = "git"
	} else if _, err := os.Stat(filepath.Join(root, ".bzr")); err == nil {
		app.VCS = "bzr"
	}
	return app
}
