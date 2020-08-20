package webgui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
)

func init() {
	rc.Add(rc.Call{
		Path:         "pluginsctl/listTestPlugins",
		AuthRequired: true,
		Fn:           rcListTestPlugins,
		Title:        "Show currently loaded test plugins",
		Help: `allows listing of test plugins with the rclone.test set to true in package.json of the plugin

This takes no parameters and returns

- loadedTestPlugins: list of currently available test plugins

Eg

    rclone rc pluginsctl/listTestPlugins
`,
	})
}

func rcListTestPlugins(_ context.Context, _ rc.Params) (out rc.Params, err error) {
	return rc.Params{
		"loadedTestPlugins": filterPlugins(loadedPlugins, func(json *PackageJSON) bool { return json.isTesting() }),
	}, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "pluginsctl/removeTestPlugin",
		AuthRequired: true,
		Fn:           rcRemoveTestPlugin,
		Title:        "Remove  a test plugin",
		Help: `This allows you to remove a plugin using it's name

This takes the following parameters

- name: name of the plugin in the format <author>/<plugin_name>

Eg

    rclone rc pluginsctl/removeTestPlugin name=rclone/rclone-webui-react
`,
	})
}
func rcRemoveTestPlugin(_ context.Context, in rc.Params) (out rc.Params, err error) {
	name, err := in.GetString("name")
	if err != nil {
		return nil, err
	}
	err = loadedPlugins.removePlugin(name)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "pluginsctl/addPlugin",
		AuthRequired: true,
		Fn:           rcAddPlugin,
		Title:        "Add a plugin using url",
		Help: `used for adding a plugin to the webgui

This takes the following parameters

- url: http url of the github repo where the plugin is hosted (http://github.com/rclone/rclone-webui-react)

Eg

   rclone rc pluginsctl/addPlugin
`,
	})
}

func rcAddPlugin(_ context.Context, in rc.Params) (out rc.Params, err error) {
	pluginURL, err := in.GetString("url")
	if err != nil {
		return nil, err
	}

	author, repoName, repoBranch, err := getAuthorRepoBranchGithub(pluginURL)
	if err != nil {
		return nil, err
	}

	branch, err := in.GetString("branch")
	if err != nil || branch == "" {
		branch = repoBranch
	}

	version, err := in.GetString("version")
	if err != nil || version == "" {
		version = "latest"
	}

	err = CreatePathIfNotExist(PluginsPath)
	if err != nil {
		return nil, err
	}

	// fetch and package.json
	// https://raw.githubusercontent.com/rclone/rclone-webui-react/master/package.json

	pluginID := fmt.Sprintf("%s/%s", author, repoName)

	currentPluginPath := filepath.Join(PluginsPath, pluginID)

	err = CreatePathIfNotExist(currentPluginPath)
	if err != nil {
		return nil, err
	}

	packageJSONUrl := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/package.json", author, repoName, branch)
	packageJSONFilePath := filepath.Join(currentPluginPath, "package.json")
	err = DownloadFile(packageJSONFilePath, packageJSONUrl)
	if err != nil {
		return nil, err
	}
	// register in plugins

	// download release and save in plugins/<author>/repo-name/app
	// https://api.github.com/repos/rclone/rclone-webui-react/releases/latest
	releaseURL, tag, _, err := GetLatestReleaseURL(fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/%s", author, repoName, version))
	if err != nil {
		return nil, err
	}
	zipName := tag + ".zip"
	zipPath := filepath.Join(currentPluginPath, zipName)

	err = DownloadFile(zipPath, releaseURL)
	if err != nil {
		return nil, err
	}

	extractPath := filepath.Join(currentPluginPath, "app")

	err = CreatePathIfNotExist(extractPath)
	if err != nil {
		return nil, err
	}
	err = os.RemoveAll(extractPath)
	if err != nil {
		fs.Logf(nil, "No previous downloads to remove")
	}

	fs.Logf(nil, "Unzipping plugin binary")

	err = Unzip(zipPath, extractPath)
	if err != nil {
		return nil, err
	}

	err = loadedPlugins.addPlugin(pluginID, packageJSONFilePath)
	if err != nil {
		return nil, err
	}

	return nil, nil

}

func init() {
	rc.Add(rc.Call{
		Path:         "pluginsctl/listPlugins",
		AuthRequired: true,
		Fn:           rcGetPlugins,
		Title:        "Get the list of currently loaded plugins",
		Help: `This allows you to get the currently enabled plugins and their details.

This takes no parameters and returns

- loadedPlugins: list of current production plugins
- testPlugins: list of temporarily loaded development plugins, usually running on a different server.

Eg

   rclone rc pluginsctl/listPlugins
`,
	})
}

func rcGetPlugins(_ context.Context, _ rc.Params) (out rc.Params, err error) {
	err = loadedPlugins.readFromFile()
	if err != nil {
		return nil, err
	}
	return rc.Params{
		"loadedPlugins":     filterPlugins(loadedPlugins, func(packageJSON *PackageJSON) bool { return !packageJSON.isTesting() }),
		"loadedTestPlugins": filterPlugins(loadedPlugins, func(packageJSON *PackageJSON) bool { return packageJSON.isTesting() }),
	}, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "pluginsctl/removePlugin",
		AuthRequired: true,
		Fn:           rcRemovePlugin,
		Title:        "Remove a loaded plugin",
		Help: `This allows you to remove a plugin using it's name

This takes parameters

- name: name of the plugin in the format <author>/<plugin_name>

Eg

   rclone rc pluginsctl/removePlugin name=rclone/video-plugin
`,
	})
}

func rcRemovePlugin(_ context.Context, in rc.Params) (out rc.Params, err error) {
	name, err := in.GetString("name")
	if err != nil {
		return nil, err
	}

	err = loadedPlugins.removePlugin(name)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "pluginsctl/getPluginsForType",
		AuthRequired: true,
		Fn:           rcGetPluginsForType,
		Title:        "Get plugins with type criteria",
		Help: `This shows all possible plugins by a mime type

This takes the following parameters

- type: supported mime type by a loaded plugin eg (video/mp4, audio/mp3)
- pluginType: filter plugins based on their type eg (DASHBOARD, FILE_HANDLER, TERMINAL) 

and returns

- loadedPlugins: list of current production plugins
- testPlugins: list of temporarily loaded development plugins, usually running on a different server.

Eg

   rclone rc pluginsctl/getPluginsForType type=video/mp4
`,
	})
}

func rcGetPluginsForType(_ context.Context, in rc.Params) (out rc.Params, err error) {
	handlesType, err := in.GetString("type")
	if err != nil {
		handlesType = ""
	}

	pluginType, err := in.GetString("pluginType")
	if err != nil {
		pluginType = ""
	}
	var loadedPluginsResult map[string]PackageJSON

	var loadedTestPluginsResult map[string]PackageJSON

	if pluginType == "" || pluginType == "FileHandler" {

		loadedPluginsResult = filterPlugins(loadedPlugins, func(packageJSON *PackageJSON) bool {
			for i := range packageJSON.Rclone.HandlesType {
				if packageJSON.Rclone.HandlesType[i] == handlesType && !packageJSON.Rclone.Test {
					return true
				}
			}
			return false
		})

		loadedTestPluginsResult = filterPlugins(loadedPlugins, func(packageJSON *PackageJSON) bool {
			for i := range packageJSON.Rclone.HandlesType {
				if packageJSON.Rclone.HandlesType[i] == handlesType && packageJSON.Rclone.Test {
					return true
				}
			}
			return false
		})
	} else {
		loadedPluginsResult = filterPlugins(loadedPlugins, func(packageJSON *PackageJSON) bool {
			return packageJSON.Rclone.PluginType == pluginType && !packageJSON.isTesting()
		})

		loadedTestPluginsResult = filterPlugins(loadedPlugins, func(packageJSON *PackageJSON) bool {
			return packageJSON.Rclone.PluginType == pluginType && packageJSON.isTesting()
		})
	}

	return rc.Params{
		"loadedPlugins":     loadedPluginsResult,
		"loadedTestPlugins": loadedTestPluginsResult,
	}, nil

}
