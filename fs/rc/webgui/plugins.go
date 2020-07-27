package webgui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/rc"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
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
	TestURL     string `json:"testUrl"`
	Repository  struct {
		Type string `json:"type"`
		URL  string `json:"url"`
	} `json:"repository"`
	Bugs struct {
		URL string `json:"url"`
	} `json:"bugs"`

	//RcloneHandlesType []string `json:"rcloneHandlesType"`
	Rclone RcloneConfig `json:"rclone"`
}

type RcloneConfig struct {
	HandlesType []string `json:"handlesType"`
	PluginType  string   `json:"pluginType"`
}

var (
	loadedTestPlugins *Plugins
	cachePath         string
	PluginsPath       string
	pluginsConfigPath string
	loadedPlugins     *Plugins
	pluginsProxy      = &httputil.ReverseProxy{}
)

func init() {
	cachePath = filepath.Join(config.CacheDir, "webgui")
	PluginsPath = filepath.Join(cachePath, "plugins")
	pluginsConfigPath = filepath.Join(PluginsPath, "config")

	loadedPlugins = newPlugins("availablePlugins.json")
	err := loadedPlugins.readFromFile()
	if err != nil {
		fs.Errorf(nil, "error reading available plugins", err)
	}
	loadedTestPlugins = newPlugins("testPlugins.json")
	err = loadedTestPlugins.readFromFile()

	if err != nil {
		fs.Errorf(nil, "error reading test plugins", err)
	}
}

type Plugins struct {
	mutex         sync.Mutex
	LoadedPlugins map[string]PackageJSON `json:"loadedPlugins"`
	fileName      string
}

func newPlugins(fileName string) *Plugins {
	p := Plugins{LoadedPlugins: map[string]PackageJSON{}}
	p.fileName = fileName
	p.mutex = sync.Mutex{}
	return &p
}

func (p *Plugins) readFromFile() (err error) {
	//p.mutex.Lock()
	//defer p.mutex.Unlock()
	err = CreatePathIfNotExist(pluginsConfigPath)
	if err != nil {
		return err
	}
	availablePluginsJson := filepath.Join(pluginsConfigPath, p.fileName)
	data, err := ioutil.ReadFile(availablePluginsJson)
	if err != nil {
		// create a file ?
	}
	err = json.Unmarshal(data, &p)
	if err != nil {
		fs.Logf(nil, "%s", err)
	}
	return nil
}

func (p *Plugins) addPlugin(pluginName string, packageJsonPath string) (err error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	data, err := ioutil.ReadFile(packageJsonPath)
	if err != nil {
		return err
	}
	var pkgJson = PackageJSON{}
	err = json.Unmarshal(data, &pkgJson)
	if err != nil {
		return err
	}
	p.LoadedPlugins[pluginName] = pkgJson

	err = p.writeToFile()
	if err != nil {
		return err
	}

	return nil
}

func (p *Plugins) addTestPlugin(pluginName string, testURL string, handlesType []string) (err error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	err = p.readFromFile()
	if err != nil {
		return err
	}

	var pkgJson = PackageJSON{
		Name:    pluginName,
		TestURL: testURL,
		Rclone: RcloneConfig{
			HandlesType: handlesType,
		},
	}

	p.LoadedPlugins[pluginName] = pkgJson

	err = p.writeToFile()
	if err != nil {
		return err
	}

	return nil
}

func (p *Plugins) writeToFile() (err error) {
	//p.mutex.Lock()
	//defer p.mutex.Unlock()
	availablePluginsJson := filepath.Join(pluginsConfigPath, p.fileName)

	file, err := json.MarshalIndent(p, "", " ")

	err = ioutil.WriteFile(availablePluginsJson, file, 0755)
	if err != nil {
		fs.Logf(nil, "%s", err)
	}
	return nil
}

func (p *Plugins) removePlugin(name string) (err error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	err = p.readFromFile()
	if err != nil {
		return err
	}

	_, ok := p.LoadedPlugins[name]
	if !ok {
		return errors.New(fmt.Sprintf("plugin %s not loaded", name))
	}
	delete(p.LoadedPlugins, name)

	err = p.writeToFile()
	if err != nil {
		return err
	}
	return nil
}

func (p *Plugins) GetPluginByName(name string) (out *PackageJSON, err error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	po, ok := p.LoadedPlugins[name]
	if !ok {
		return nil, errors.New(fmt.Sprintf("plugin %s not loaded", name))
	}
	return &po, nil

}

func init() {
	rc.Add(rc.Call{
		Path:         "pluginsctl/addTestPlugin",
		AuthRequired: true,
		Fn:           rcAddTestPlugin,
		Title:        "Show current mount points",
		Help: `This shows currently mounted points, which can be used for performing an unmount

This takes no parameters and returns

- mountPoints: list of current mount points

Eg

    rclone rc pluginsctl/addTestPlugin
`,
	})
}

func rcAddTestPlugin(_ context.Context, in rc.Params) (out rc.Params, err error) {
	name, err := in.GetString("name")
	if err != nil {
		return nil, err
	}

	loadUrl, err := in.GetString("loadUrl")
	if err != nil {
		return nil, err
	}
	var handlesTypes []string
	err = in.GetStructMissingOK("handlesTypes", &handlesTypes)
	if err != nil {
		return nil, err
	}

	err = loadedTestPlugins.addTestPlugin(name, loadUrl, handlesTypes)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "pluginsctl/listTestPlugins",
		AuthRequired: true,
		Fn:           rcGetLoadedPlugins,
		Title:        "Show current mount points",
		Help: `This shows currently mounted points, which can be used for performing an unmount

This takes no parameters and returns

- mountPoints: list of current mount points

Eg

    rclone rc pluginsctl/listTestPlugins
`,
	})
}

func rcGetLoadedPlugins(_ context.Context, in rc.Params) (out rc.Params, err error) {
	return rc.Params{
		"loadedTestPlugins": loadedTestPlugins.LoadedPlugins,
	}, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "pluginsctl/removeTestPlugin",
		AuthRequired: true,
		Fn:           rcRemoveTestPlugin,
		Title:        "Show current mount points",
		Help: `This shows currently mounted points, which can be used for performing an unmount

This takes no parameters and returns

- mountPoints: list of current mount points

Eg

    rclone rc pluginsctl/removeTestPlugin
`,
	})
}

func rcRemoveTestPlugin(_ context.Context, in rc.Params) (out rc.Params, err error) {
	name, err := in.GetString("name")
	if err != nil {
		return nil, err
	}
	err = loadedTestPlugins.removePlugin(name)
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
		Title:        "Show current mount points",
		Help: `This shows currently mounted points, which can be used for performing an unmount

This takes no parameters and returns

- mountPoints: list of current mount points

Eg

   rclone rc pluginsctl/addPlugin
`,
	})
}

func rcAddPlugin(_ context.Context, in rc.Params) (out rc.Params, err error) {
	pluginUrl, err := in.GetString("url")
	if err != nil {
		return nil, err
	}

	author, repoName, repoBranch, err := getAuthorRepoBranchGithub(pluginUrl)
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

	packageJsonUrl := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/package.json", author, repoName, branch)
	packageJsonFilePath := filepath.Join(currentPluginPath, "package.json")
	err = DownloadFile(packageJsonFilePath, packageJsonUrl)
	if err != nil {
		return nil, err
	}
	// register in plugins

	// download release and save in plugins/<author>/repo-name/app
	// https://api.github.com/repos/rclone/rclone-webui-react/releases/latest
	releaseUrl, tag, _, err := GetLatestReleaseURL(fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/%s", author, repoName, version))
	zipName := tag + ".zip"
	zipPath := filepath.Join(currentPluginPath, zipName)

	err = DownloadFile(zipPath, releaseUrl)
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

	err = loadedPlugins.addPlugin(pluginID, packageJsonFilePath)
	if err != nil {
		return nil, err
	}

	return nil, nil

}

// getAuthorRepoBranchGithub gives author, repoName and branch from a github.com url
//	url examples:
//	https://github.com/rclone/rclone-webui-react/
//	http://github.com/rclone/rclone-webui-react
//	https://github.com/rclone/rclone-webui-react/tree/caman-js
// 	github.com/rclone/rclone-webui-react
//
func getAuthorRepoBranchGithub(url string) (author string, repoName string, branch string, err error) {
	repoUrl := url
	repoUrl = strings.Replace(repoUrl, "https://", "", 1)
	repoUrl = strings.Replace(repoUrl, "http://", "", 1)

	urlSplits := strings.Split(repoUrl, "/")

	if len(urlSplits) < 3 || len(urlSplits) > 5 || urlSplits[0] != "github.com" {
		return "", "", "", errors.New(fmt.Sprintf("Invalid github url: %s", url))
	}

	// get branch name
	if len(urlSplits) == 5 && urlSplits[3] == "tree" {
		return urlSplits[1], urlSplits[2], urlSplits[4], nil
	}

	return urlSplits[1], urlSplits[2], "master", nil
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

func rcGetPlugins(_ context.Context, in rc.Params) (out rc.Params, err error) {
	err = loadedPlugins.readFromFile()
	if err != nil {
		return nil, err
	}
	err = loadedTestPlugins.readFromFile()
	if err != nil {
		return nil, err
	}

	return rc.Params{
		"loadedPlugins":     loadedPlugins.LoadedPlugins,
		"loadedTestPlugins": loadedTestPlugins.LoadedPlugins,
	}, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "pluginsctl/removePlugin",
		AuthRequired: true,
		Fn:           rcRemovePlugin,
		Title:        "Get the list of currently loaded plugins",
		Help: `This allows you to get the currently enabled plugins and their details.

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
		Title:        "Get the list of currently loaded plugins",
		Help: `This allows you to get the currently enabled plugins and their details.

This takes no parameters and returns

- loadedPlugins: list of current production plugins
- testPlugins: list of temporarily loaded development plugins, usually running on a different server.

Eg

   rclone rc pluginsctl/getPlugins
`,
	})
}

func filterPlugins(plugins *Plugins, compare func(packageJSON *PackageJSON) bool) map[string]PackageJSON {
	output := map[string]PackageJSON{}

	for key, val := range plugins.LoadedPlugins {
		if compare(&val) {
			output[key] = val
		}
	}

	return output
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
				if packageJSON.Rclone.HandlesType[i] == handlesType {
					return true
				}
			}
			return false
		})

		loadedTestPluginsResult = filterPlugins(loadedTestPlugins, func(packageJSON *PackageJSON) bool {
			for i := range packageJSON.Rclone.HandlesType {
				if packageJSON.Rclone.HandlesType[i] == handlesType {
					return true
				}
			}
			return false
		})
	} else {
		loadedPluginsResult = filterPlugins(loadedPlugins, func(packageJSON *PackageJSON) bool {
			return packageJSON.Rclone.PluginType == pluginType
		})

		loadedTestPluginsResult = filterPlugins(loadedTestPlugins, func(packageJSON *PackageJSON) bool {
			return packageJSON.Rclone.PluginType == pluginType
		})
	}

	return rc.Params{
		"loadedPlugins": loadedPluginsResult,
		"testPlugins":   loadedTestPluginsResult,
	}, nil

}

var PluginsMatch = regexp.MustCompile(`^plugins\/([^\/]*)\/([^\/\?]+)[\/]?(.*)$`)

func getDirectorForProxy(origin *url.URL) func(req *http.Request) {
	return func(req *http.Request) {
		req.Header.Add("X-Forwarded-Host", req.Host)
		req.Header.Add("X-Origin-Host", origin.Host)
		req.URL.Scheme = "http"
		req.URL.Host = origin.Host
		req.URL.Path = origin.Path
	}
}

func ServePluginOK(w http.ResponseWriter, r *http.Request, pluginsMatchResult []string) (ok bool) {
	testPlugin, err := loadedTestPlugins.GetPluginByName(fmt.Sprintf("%s/%s", pluginsMatchResult[1], pluginsMatchResult[2]))
	if err != nil {
		return false
	}

	origin, _ := url.Parse(fmt.Sprintf("%s/%s", testPlugin.TestURL, pluginsMatchResult[3]))

	director := getDirectorForProxy(origin)

	pluginsProxy.Director = director
	pluginsProxy.ServeHTTP(w, r)
	return true
}
