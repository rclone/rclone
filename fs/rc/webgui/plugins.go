// Package webgui provides plugin functionality to the Web GUI.
package webgui

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/rc"
)

// PackageJSON is the structure of package.json of a plugin
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
	Rclone RcloneConfig `json:"rclone"`
}

// RcloneConfig represents the rclone specific config
type RcloneConfig struct {
	HandlesType      []string `json:"handlesType"`
	PluginType       string   `json:"pluginType"`
	RedirectReferrer bool     `json:"redirectReferrer"`
	Test             bool     `json:"-"`
}

func (r *PackageJSON) isTesting() bool {
	return r.Rclone.Test
}

var (
	//loadedTestPlugins *Plugins
	cachePath string

	loadedPlugins *Plugins
	pluginsProxy  = &httputil.ReverseProxy{}
	// PluginsMatch is used for matching author and plugin name in the url path
	PluginsMatch = regexp.MustCompile(`^plugins\/([^\/]*)\/([^\/\?]+)[\/]?(.*)$`)
	// PluginsPath is the base path where webgui plugins are stored
	PluginsPath              string
	pluginsConfigPath        string
	availablePluginsJSONPath = "availablePlugins.json"
	initSuccess              = false
	initMutex                = &sync.Mutex{}
)

// Plugins represents the structure how plugins are saved onto disk
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

func initPluginsOrError() error {
	if !rc.Opt.WebUI {
		return errors.New("WebUI needs to be enabled for plugins to work")
	}
	initMutex.Lock()
	defer initMutex.Unlock()
	if !initSuccess {
		cachePath = filepath.Join(config.GetCacheDir(), "webgui")
		PluginsPath = filepath.Join(cachePath, "plugins")
		pluginsConfigPath = filepath.Join(PluginsPath, "config")
		loadedPlugins = newPlugins(availablePluginsJSONPath)
		err := loadedPlugins.readFromFile()
		if err != nil {
			fs.Errorf(nil, "error reading available plugins: %v", err)
		}
		initSuccess = true
	}

	return nil
}

func (p *Plugins) readFromFile() (err error) {
	err = CreatePathIfNotExist(pluginsConfigPath)
	if err != nil {
		return err
	}
	availablePluginsJSON := filepath.Join(pluginsConfigPath, p.fileName)
	_, err = os.Stat(availablePluginsJSON)
	if err == nil {
		data, err := os.ReadFile(availablePluginsJSON)
		if err != nil {
			return err
		}
		err = json.Unmarshal(data, &p)
		if err != nil {
			fs.Logf(nil, "%s", err)
		}
		return nil
	} else if os.IsNotExist(err) {
		// path does not exist
		err = p.writeToFile()
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Plugins) addPlugin(pluginName string, packageJSONPath string) (err error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	data, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return err
	}
	var pkgJSON = PackageJSON{}
	err = json.Unmarshal(data, &pkgJSON)
	if err != nil {
		return err
	}
	p.LoadedPlugins[pluginName] = pkgJSON

	err = p.writeToFile()
	if err != nil {
		return err
	}

	return nil
}

func (p *Plugins) writeToFile() (err error) {
	availablePluginsJSON := filepath.Join(pluginsConfigPath, p.fileName)

	file, err := json.MarshalIndent(p, "", " ")
	if err != nil {
		fs.Logf(nil, "%s", err)
	}
	err = os.WriteFile(availablePluginsJSON, file, 0755)
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
		return fmt.Errorf("plugin %s not loaded", name)
	}
	delete(p.LoadedPlugins, name)

	err = p.writeToFile()
	if err != nil {
		return err
	}
	return nil
}

// GetPluginByName returns the plugin object for the key (author/plugin-name)
func (p *Plugins) GetPluginByName(name string) (out *PackageJSON, err error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	po, ok := p.LoadedPlugins[name]
	if !ok {
		return nil, fmt.Errorf("plugin %s not loaded", name)
	}
	return &po, nil

}

// getAuthorRepoBranchGitHub gives author, repoName and branch from a github.com url
//
//	url examples:
//	https://github.com/rclone/rclone-webui-react/
//	http://github.com/rclone/rclone-webui-react
//	https://github.com/rclone/rclone-webui-react/tree/caman-js
//	github.com/rclone/rclone-webui-react
func getAuthorRepoBranchGitHub(url string) (author string, repoName string, branch string, err error) {
	repoURL := url
	repoURL = strings.Replace(repoURL, "https://", "", 1)
	repoURL = strings.Replace(repoURL, "http://", "", 1)

	urlSplits := strings.Split(repoURL, "/")

	if len(urlSplits) < 3 || len(urlSplits) > 5 || urlSplits[0] != "github.com" {
		return "", "", "", fmt.Errorf("invalid github url: %s", url)
	}

	// get branch name
	if len(urlSplits) == 5 && urlSplits[3] == "tree" {
		return urlSplits[1], urlSplits[2], urlSplits[4], nil
	}

	return urlSplits[1], urlSplits[2], "master", nil
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

// getDirectorForProxy is a helper function for reverse proxy of test plugins
func getDirectorForProxy(origin *url.URL) func(req *http.Request) {
	return func(req *http.Request) {
		req.Header.Add("X-Forwarded-Host", req.Host)
		req.Header.Add("X-Origin-Host", origin.Host)
		req.URL.Scheme = "http"
		req.URL.Host = origin.Host
		req.URL.Path = origin.Path
	}
}

// ServePluginOK checks the plugin url and uses reverse proxy to allow redirection for content not being served by rclone
func ServePluginOK(w http.ResponseWriter, r *http.Request, pluginsMatchResult []string) (ok bool) {
	testPlugin, err := loadedPlugins.GetPluginByName(fmt.Sprintf("%s/%s", pluginsMatchResult[1], pluginsMatchResult[2]))
	if err != nil {
		return false
	}
	if !testPlugin.Rclone.Test {
		return false
	}
	origin, _ := url.Parse(fmt.Sprintf("%s/%s", testPlugin.TestURL, pluginsMatchResult[3]))

	director := getDirectorForProxy(origin)

	pluginsProxy.Director = director
	pluginsProxy.ServeHTTP(w, r)
	return true
}

var referrerPathReg = regexp.MustCompile(`^(https?):\/\/(.+):([0-9]+)?\/(.*)\/?\?(.*)$`)

// ServePluginWithReferrerOK check if redirectReferrer is set for the referred a plugin, if yes,
// sends a redirect to actual url. This function is useful for plugins to refer to absolute paths when
// the referrer in http.Request is set
func ServePluginWithReferrerOK(w http.ResponseWriter, r *http.Request, path string) (ok bool) {
	err := initPluginsOrError()
	if err != nil {
		return false
	}
	referrer := r.Referer()
	referrerPathMatch := referrerPathReg.FindStringSubmatch(referrer)

	if len(referrerPathMatch) > 3 {
		referrerPluginMatch := PluginsMatch.FindStringSubmatch(referrerPathMatch[4])
		if len(referrerPluginMatch) > 2 {
			pluginKey := fmt.Sprintf("%s/%s", referrerPluginMatch[1], referrerPluginMatch[2])
			currentPlugin, err := loadedPlugins.GetPluginByName(pluginKey)
			if err != nil {
				return false
			}
			if currentPlugin.Rclone.RedirectReferrer {
				path = fmt.Sprintf("/plugins/%s/%s/%s", referrerPluginMatch[1], referrerPluginMatch[2], path)

				http.Redirect(w, r, path, http.StatusMovedPermanently)
				return true
			}
		}
	}
	return false
}
