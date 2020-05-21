package rc

import (
	"context"
	"errors"
	"fmt"
)

type PackageJSON struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Copyright   string `json:"copyright"`
	License     string `json:"license"`
	Private     bool   `json:"private"`
	Homepage    string `json:"homepage"`
	Repository  struct {
		Type string `json:"type"`
		URL  string `json:"url"`
	} `json:"repository"`
	Bugs struct {
		URL string `json:"url"`
	} `json:"bugs"`
}

var loadedTestPlugins map[string]string

func init() {
	loadedTestPlugins = make(map[string]string)
}

func init() {
	Add(Call{
		Path:         "pluginsctl/addTestPlugin",
		AuthRequired: true,
		Fn:           rcAddPlugin,
		Title:        "Show current mount points",
		Help: `This shows currently mounted points, which can be used for performing an unmount

This takes no parameters and returns

- mountPoints: list of current mount points

Eg

    rclone rc mount/listmounts
`,
	})
}

func rcAddPlugin(_ context.Context, in Params) (out Params, err error) {
	test, err := in.GetBool("test")
	if err != nil {
		return nil, err
	}
	name, err := in.GetString("name")
	if err != nil {
		return nil, err
	}

	url, err := in.GetString("loadUrl")
	if err != nil {
		return nil, err
	}
	if test {
		loadedTestPlugins[name] = url
	}
	return nil, nil
}

func init() {
	Add(Call{
		Path:         "pluginsctl/listTestPlugins",
		AuthRequired: true,
		Fn:           rcGetLoadedPlugins,
		Title:        "Show current mount points",
		Help: `This shows currently mounted points, which can be used for performing an unmount

This takes no parameters and returns

- mountPoints: list of current mount points

Eg

    rclone rc mount/listmounts
`,
	})
}

func rcGetLoadedPlugins(_ context.Context, in Params) (out Params, err error) {
	return Params{
		"loadedTestPlugins": loadedTestPlugins,
	}, nil
}

func init() {
	Add(Call{
		Path:         "pluginsctl/removeTestPlugin",
		AuthRequired: true,
		Fn:           rcRemovePlugin,
		Title:        "Show current mount points",
		Help: `This shows currently mounted points, which can be used for performing an unmount

This takes no parameters and returns

- mountPoints: list of current mount points

Eg

    rclone rc mount/listmounts
`,
	})
}

func rcRemovePlugin(_ context.Context, in Params) (out Params, err error) {
	name, err := in.GetString("name")
	if err != nil {
		return nil, err
	}
	_, ok := loadedTestPlugins[name]
	if ok {
		delete(loadedTestPlugins, name)
		return nil, nil
	}
	return nil, errors.New(fmt.Sprintf("plugin %s not loaded", name))
}
