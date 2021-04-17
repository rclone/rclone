// Package config reads, writes and edits the config file and deals with command line flags
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	mathrand "math/rand"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/lib/file"
	"github.com/rclone/rclone/lib/random"
)

const (
	configFileName       = "rclone.conf"
	hiddenConfigFileName = "." + configFileName
	noConfigFile         = "notfound"

	// ConfigToken is the key used to store the token under
	ConfigToken = "token"

	// ConfigClientID is the config key used to store the client id
	ConfigClientID = "client_id"

	// ConfigClientSecret is the config key used to store the client secret
	ConfigClientSecret = "client_secret"

	// ConfigAuthURL is the config key used to store the auth server endpoint
	ConfigAuthURL = "auth_url"

	// ConfigTokenURL is the config key used to store the token server endpoint
	ConfigTokenURL = "token_url"

	// ConfigEncoding is the config key to change the encoding for a backend
	ConfigEncoding = "encoding"

	// ConfigEncodingHelp is the help for ConfigEncoding
	ConfigEncodingHelp = "This sets the encoding for the backend.\n\nSee: the [encoding section in the overview](/overview/#encoding) for more info."

	// ConfigAuthorize indicates that we just want "rclone authorize"
	ConfigAuthorize = "config_authorize"

	// ConfigAuthNoBrowser indicates that we do not want to open browser
	ConfigAuthNoBrowser = "config_auth_no_browser"
)

// Storage defines an interface for loading and saving config to
// persistent storage. Rclone provides a default implementation to
// load and save to a config file when this is imported
//
// import "github.com/rclone/rclone/fs/config/configfile"
// configfile.LoadConfig(ctx)
type Storage interface {
	// GetSectionList returns a slice of strings with names for all the
	// sections
	GetSectionList() []string

	// HasSection returns true if section exists in the config file
	HasSection(section string) bool

	// DeleteSection removes the named section and all config from the
	// config file
	DeleteSection(section string)

	// GetKeyList returns the keys in this section
	GetKeyList(section string) []string

	// GetValue returns the key in section with a found flag
	GetValue(section string, key string) (value string, found bool)

	// SetValue sets the value under key in section
	SetValue(section string, key string, value string)

	// DeleteKey removes the key under section
	DeleteKey(section string, key string) bool

	// Load the config from permanent storage
	Load() error

	// Save the config to permanent storage
	Save() error

	// Serialize the config into a string
	Serialize() (string, error)
}

// Global
var (
	// Data is the global config data structure
	Data Storage = defaultStorage{}

	// CacheDir points to the cache directory.  Users of this
	// should make a subdirectory and use MkdirAll() to create it
	// and any parents.
	CacheDir = makeCacheDir()

	// Password can be used to configure the random password generator
	Password = random.Password
)

var configPath string

func init() {
	// Set the function pointers up in fs
	fs.ConfigFileGet = FileGetFlag
	fs.ConfigFileSet = SetValueAndSave
	configPath = makeConfigPath()
}

// Join directory with filename, and check if exists
func findFile(dir string, name string) string {
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

// Find current user's home directory
func findHomeDir() (string, error) {
	path, err := homedir.Dir()
	if err != nil {
		fs.Debugf(nil, "Home directory lookup failed and cannot be used as configuration location: %v", err)
	} else if path == "" {
		// On Unix homedir return success but empty string for user with empty home configured in passwd file
		fs.Debugf(nil, "Home directory not defined and cannot be used as configuration location")
	}
	return path, err
}

// Find rclone executable directory and look for existing rclone.conf there
// (<rclone_exe_dir>/rclone.conf)
func findLocalConfig() (configDir string, configFile string) {
	if exePath, err := os.Executable(); err == nil {
		configDir = filepath.Dir(exePath)
		configFile = findFile(configDir, configFileName)
	}
	return
}

// Get path to Windows AppData config subdirectory for rclone and look for existing rclone.conf there
// ($AppData/rclone/rclone.conf)
func findAppDataConfig() (configDir string, configFile string) {
	if appDataDir := os.Getenv("APPDATA"); appDataDir != "" {
		configDir = filepath.Join(appDataDir, "rclone")
		configFile = findFile(configDir, configFileName)
	} else {
		fs.Debugf(nil, "Environment variable APPDATA is not defined and cannot be used as configuration location")
	}
	return
}

// Get path to XDG config subdirectory for rclone and look for existing rclone.conf there
// (see XDG Base Directory specification: https://specifications.freedesktop.org/basedir-spec/latest/).
// ($XDG_CONFIG_HOME\rclone\rclone.conf)
func findXDGConfig() (configDir string, configFile string) {
	if xdgConfigDir := os.Getenv("XDG_CONFIG_HOME"); xdgConfigDir != "" {
		configDir = filepath.Join(xdgConfigDir, "rclone")
		configFile = findFile(configDir, configFileName)
	}
	return
}

// Get path to .config subdirectory for rclone and look for existing rclone.conf there
// (~/.config/rclone/rclone.conf)
func findDotConfigConfig(home string) (configDir string, configFile string) {
	if home != "" {
		configDir = filepath.Join(home, ".config", "rclone")
		configFile = findFile(configDir, configFileName)
	}
	return
}

// Look for existing .rclone.conf (legacy hidden filename) in root of user's home directory
// (~/.rclone.conf)
func findOldHomeConfig(home string) (configDir string, configFile string) {
	if home != "" {
		configDir = home
		configFile = findFile(home, hiddenConfigFileName)
	}
	return
}

// Return the path to the configuration file
func makeConfigPath() string {
	// Look for existing rclone.conf in prioritized list of known locations
	// Also get configuration directory to use for new config file when no existing is found.
	var (
		configFile        string
		configDir         string
		primaryConfigDir  string
		fallbackConfigDir string
	)
	// <rclone_exe_dir>/rclone.conf
	if _, configFile = findLocalConfig(); configFile != "" {
		return configFile
	}
	// Windows: $AppData/rclone/rclone.conf
	// This is also the default location for new config when no existing is found
	if runtime.GOOS == "windows" {
		if primaryConfigDir, configFile = findAppDataConfig(); configFile != "" {
			return configFile
		}
	}
	// $XDG_CONFIG_HOME/rclone/rclone.conf
	// Also looking for this on Windows, for backwards compatibility reasons.
	if configDir, configFile = findXDGConfig(); configFile != "" {
		return configFile
	}
	if runtime.GOOS != "windows" {
		// On Unix this is also the default location for new config when no existing is found
		primaryConfigDir = configDir
	}
	// ~/.config/rclone/rclone.conf
	// This is also the fallback location for new config
	// (when $AppData on Windows and $XDG_CONFIG_HOME on Unix is not defined)
	homeDir, homeDirErr := findHomeDir()
	if fallbackConfigDir, configFile = findDotConfigConfig(homeDir); configFile != "" {
		return configFile
	}
	// ~/.rclone.conf
	if _, configFile = findOldHomeConfig(homeDir); configFile != "" {
		return configFile
	}

	// No existing config file found, prepare proper default for a new one.
	// But first check if if user supplied a --config variable or environment
	// variable, since then we skip actually trying to create the default
	// and report any errors related to it (we can't use pflag for this because
	// it isn't initialised yet so we search the command line manually).
	_, configSupplied := os.LookupEnv("RCLONE_CONFIG")
	if !configSupplied {
		for _, item := range os.Args {
			if item == "--config" || strings.HasPrefix(item, "--config=") {
				configSupplied = true
				break
			}
		}
	}
	// If we found a configuration directory to be used for new config during search
	// above, then create it to be ready for rclone.conf file to be written into it
	// later, and also as a test of permissions to use fallback if not even able to
	// create the directory.
	if primaryConfigDir != "" {
		configDir = primaryConfigDir
	} else if fallbackConfigDir != "" {
		configDir = fallbackConfigDir
	} else {
		configDir = ""
	}
	if configDir != "" {
		configFile = filepath.Join(configDir, configFileName)
		if configSupplied {
			// User supplied custom config option, just return the default path
			// as is without creating any directories, since it will not be used
			// anyway and we don't want to unnecessarily create empty directory.
			return configFile
		}
		var mkdirErr error
		if mkdirErr = os.MkdirAll(configDir, os.ModePerm); mkdirErr == nil {
			return configFile
		}
		// Problem: Try a fallback location. If we did find a home directory then
		// just assume file .rclone.conf (legacy hidden filename) can be written in
		// its root (~/.rclone.conf).
		if homeDir != "" {
			fs.Debugf(nil, "Configuration directory could not be created and will not be used: %v", mkdirErr)
			return filepath.Join(homeDir, hiddenConfigFileName)
		}
		if !configSupplied {
			fs.Errorf(nil, "Couldn't find home directory nor create configuration directory: %v", mkdirErr)
		}
	} else if !configSupplied {
		if homeDirErr != nil {
			fs.Errorf(nil, "Couldn't find configuration directory nor home directory: %v", homeDirErr)
		} else {
			fs.Errorf(nil, "Couldn't find configuration directory nor home directory")
		}
	}
	// No known location that can be used: Did possibly find a configDir
	// (XDG_CONFIG_HOME or APPDATA) which couldn't be created, but in any case
	// did not find a home directory!
	// Report it as an error, and return as last resort the path relative to current
	// working directory, of .rclone.conf (legacy hidden filename).
	if !configSupplied {
		fs.Errorf(nil, "Defaulting to storing config in current directory.")
		fs.Errorf(nil, "Use --config flag to workaround.")
	}
	return hiddenConfigFileName
}

// GetConfigPath returns the current config file path
func GetConfigPath() string {
	return configPath
}

// SetConfigPath sets new config file path
//
// Checks for empty string, os null device, or special path, all of which indicates in-memory config.
func SetConfigPath(path string) (err error) {
	var cfgPath string
	if path == "" || path == os.DevNull {
		cfgPath = ""
	} else if filepath.Base(path) == noConfigFile {
		cfgPath = ""
	} else if err = file.IsReserved(path); err != nil {
		return err
	} else if cfgPath, err = filepath.Abs(path); err != nil {
		return err
	}
	configPath = cfgPath
	return nil
}

// LoadConfig loads the config file
func LoadConfig(ctx context.Context) {
	// Set RCLONE_CONFIG_DIR for backend config and subprocesses
	// If empty configPath (in-memory only) the value will be "."
	_ = os.Setenv("RCLONE_CONFIG_DIR", filepath.Dir(configPath))
	// Load configuration from file (or initialize sensible default if no file or error)
	if err := Data.Load(); err == ErrorConfigFileNotFound {
		if configPath == "" {
			fs.Debugf(nil, "Config is memory-only - using defaults")
		} else {
			fs.Logf(nil, "Config file %q not found - using defaults", configPath)
		}
	} else if err != nil {
		log.Fatalf("Failed to load config file %q: %v", configPath, err)
	} else {
		fs.Debugf(nil, "Using config file from %q", configPath)
	}
}

// ErrorConfigFileNotFound is returned when the config file is not found
var ErrorConfigFileNotFound = errors.New("config file not found")

// SaveConfig calling function which saves configuration file.
// if SaveConfig returns error trying again after sleep.
func SaveConfig() {
	if configPath == "" {
		fs.Debugf(nil, "Skipping save for memory-only config")
		return
	}
	ctx := context.Background()
	ci := fs.GetConfig(ctx)
	var err error
	for i := 0; i < ci.LowLevelRetries+1; i++ {
		if err = Data.Save(); err == nil {
			return
		}
		waitingTimeMs := mathrand.Intn(1000)
		time.Sleep(time.Duration(waitingTimeMs) * time.Millisecond)
	}
	fs.Errorf(nil, "Failed to save config after %d tries: %v", ci.LowLevelRetries, err)
}

// SetValueAndSave sets the key to the value and saves just that
// value in the config file.  It loads the old config file in from
// disk first and overwrites the given value only.
func SetValueAndSave(name, key, value string) error {
	// Set the value in config in case we fail to reload it
	Data.SetValue(name, key, value)
	// Save it again
	SaveConfig()
	return nil
}

// getWithDefault gets key out of section name returning defaultValue if not
// found.
func getWithDefault(name, key, defaultValue string) string {
	value, found := Data.GetValue(name, key)
	if !found {
		return defaultValue
	}
	return value
}

// UpdateRemote adds the keyValues passed in to the remote of name.
// keyValues should be key, value pairs.
func UpdateRemote(ctx context.Context, name string, keyValues rc.Params, doObscure, noObscure bool) error {
	if doObscure && noObscure {
		return errors.New("can't use --obscure and --no-obscure together")
	}
	err := fspath.CheckConfigName(name)
	if err != nil {
		return err
	}
	ctx = suppressConfirm(ctx)

	// Work out which options need to be obscured
	needsObscure := map[string]struct{}{}
	if !noObscure {
		if fsType := FileGet(name, "type"); fsType != "" {
			if ri, err := fs.Find(fsType); err != nil {
				fs.Debugf(nil, "Couldn't find fs for type %q", fsType)
			} else {
				for _, opt := range ri.Options {
					if opt.IsPassword {
						needsObscure[opt.Name] = struct{}{}
					}
				}
			}
		} else {
			fs.Debugf(nil, "UpdateRemote: Couldn't find fs type")
		}
	}

	// Set the config
	for k, v := range keyValues {
		vStr := fmt.Sprint(v)
		// Obscure parameter if necessary
		if _, ok := needsObscure[k]; ok {
			_, err := obscure.Reveal(vStr)
			if err != nil || doObscure {
				// If error => not already obscured, so obscure it
				// or we are forced to obscure
				vStr, err = obscure.Obscure(vStr)
				if err != nil {
					return errors.Wrap(err, "UpdateRemote: obscure failed")
				}
			}
		}
		Data.SetValue(name, k, vStr)
	}
	RemoteConfig(ctx, name)
	SaveConfig()
	cache.ClearConfig(name) // remove any remotes based on this config from the cache
	return nil
}

// CreateRemote creates a new remote with name, provider and a list of
// parameters which are key, value pairs.  If update is set then it
// adds the new keys rather than replacing all of them.
func CreateRemote(ctx context.Context, name string, provider string, keyValues rc.Params, doObscure, noObscure bool) error {
	err := fspath.CheckConfigName(name)
	if err != nil {
		return err
	}
	// Delete the old config if it exists
	Data.DeleteSection(name)
	// Set the type
	Data.SetValue(name, "type", provider)
	// Set the remaining values
	return UpdateRemote(ctx, name, keyValues, doObscure, noObscure)
}

// PasswordRemote adds the keyValues passed in to the remote of name.
// keyValues should be key, value pairs.
func PasswordRemote(ctx context.Context, name string, keyValues rc.Params) error {
	ctx = suppressConfirm(ctx)
	err := fspath.CheckConfigName(name)
	if err != nil {
		return err
	}
	for k, v := range keyValues {
		keyValues[k] = obscure.MustObscure(fmt.Sprint(v))
	}
	return UpdateRemote(ctx, name, keyValues, false, true)
}

// JSONListProviders prints all the providers and options in JSON format
func JSONListProviders() error {
	b, err := json.MarshalIndent(fs.Registry, "", "    ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal examples")
	}
	_, err = os.Stdout.Write(b)
	if err != nil {
		return errors.Wrap(err, "failed to write providers list")
	}
	return nil
}

// fsOption returns an Option describing the possible remotes
func fsOption() *fs.Option {
	o := &fs.Option{
		Name:    "Storage",
		Help:    "Type of storage to configure.",
		Default: "",
	}
	for _, item := range fs.Registry {
		example := fs.OptionExample{
			Value: item.Name,
			Help:  item.Description,
		}
		o.Examples = append(o.Examples, example)
	}
	o.Examples.Sort()
	return o
}

// FileGetFlag gets the config key under section returning the
// the value and true if found and or ("", false) otherwise
func FileGetFlag(section, key string) (string, bool) {
	return Data.GetValue(section, key)
}

// FileGet gets the config key under section returning the default if not set.
//
// It looks up defaults in the environment if they are present
func FileGet(section, key string) string {
	var defaultVal string
	envKey := fs.ConfigToEnv(section, key)
	newValue, found := os.LookupEnv(envKey)
	if found {
		defaultVal = newValue
	}
	return getWithDefault(section, key, defaultVal)
}

// FileSet sets the key in section to value.  It doesn't save
// the config file.
func FileSet(section, key, value string) {
	if value != "" {
		Data.SetValue(section, key, value)
	} else {
		FileDeleteKey(section, key)
	}
}

// FileDeleteKey deletes the config key in the config file.
// It returns true if the key was deleted,
// or returns false if the section or key didn't exist.
func FileDeleteKey(section, key string) bool {
	return Data.DeleteKey(section, key)
}

var matchEnv = regexp.MustCompile(`^RCLONE_CONFIG_(.*?)_TYPE=.*$`)

// FileSections returns the sections in the config file
// including any defined by environment variables.
func FileSections() []string {
	sections := Data.GetSectionList()
	for _, item := range os.Environ() {
		matches := matchEnv.FindStringSubmatch(item)
		if len(matches) == 2 {
			sections = append(sections, strings.ToLower(matches[1]))
		}
	}
	return sections
}

// DumpRcRemote dumps the config for a single remote
func DumpRcRemote(name string) (dump rc.Params) {
	params := rc.Params{}
	for _, key := range Data.GetKeyList(name) {
		params[key] = FileGet(name, key)
	}
	return params
}

// DumpRcBlob dumps all the config as an unstructured blob suitable
// for the rc
func DumpRcBlob() (dump rc.Params) {
	dump = rc.Params{}
	for _, name := range Data.GetSectionList() {
		dump[name] = DumpRcRemote(name)
	}
	return dump
}

// Dump dumps all the config as a JSON file
func Dump() error {
	dump := DumpRcBlob()
	b, err := json.MarshalIndent(dump, "", "    ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal config dump")
	}
	_, err = os.Stdout.Write(b)
	if err != nil {
		return errors.Wrap(err, "failed to write config dump")
	}
	return nil
}

// makeCacheDir returns a directory to use for caching.
//
// Code borrowed from go stdlib until it is made public
func makeCacheDir() (dir string) {
	// Compute default location.
	switch runtime.GOOS {
	case "windows":
		dir = os.Getenv("LocalAppData")

	case "darwin":
		dir = os.Getenv("HOME")
		if dir != "" {
			dir += "/Library/Caches"
		}

	case "plan9":
		dir = os.Getenv("home")
		if dir != "" {
			// Plan 9 has no established per-user cache directory,
			// but $home/lib/xyz is the usual equivalent of $HOME/.xyz on Unix.
			dir += "/lib/cache"
		}

	default: // Unix
		// https://standards.freedesktop.org/basedir-spec/basedir-spec-latest.html
		dir = os.Getenv("XDG_CACHE_HOME")
		if dir == "" {
			dir = os.Getenv("HOME")
			if dir != "" {
				dir += "/.cache"
			}
		}
	}

	// if no dir found then use TempDir - we will have a cachedir!
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "rclone")
}
