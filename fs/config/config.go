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
	"github.com/rclone/rclone/lib/random"
)

const (
	configFileName       = "rclone.conf"
	hiddenConfigFileName = "." + configFileName

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

	// ConfigPath points to the config file
	ConfigPath = makeConfigPath()

	// Password can be used to configure the random password generator
	Password = random.Password
)

func init() {
	// Set the function pointers up in fs
	fs.ConfigFileGet = FileGetFlag
	fs.ConfigFileSet = SetValueAndSave
}

// Return the path to the configuration file
func makeConfigPath() string {
	// Use rclone.conf from rclone executable directory if already existing
	exe, err := os.Executable()
	if err == nil {
		exedir := filepath.Dir(exe)
		cfgpath := filepath.Join(exedir, configFileName)
		_, err := os.Stat(cfgpath)
		if err == nil {
			return cfgpath
		}
	}

	// Find user's home directory
	homeDir, err := homedir.Dir()

	// Find user's configuration directory.
	// Prefer XDG config path, with fallback to $HOME/.config.
	// See XDG Base Directory specification
	// https://specifications.freedesktop.org/basedir-spec/latest/),
	xdgdir := os.Getenv("XDG_CONFIG_HOME")
	var cfgdir string
	if xdgdir != "" {
		// User's configuration directory for rclone is $XDG_CONFIG_HOME/rclone
		cfgdir = filepath.Join(xdgdir, "rclone")
	} else if homeDir != "" {
		// User's configuration directory for rclone is $HOME/.config/rclone
		cfgdir = filepath.Join(homeDir, ".config", "rclone")
	}

	// Use rclone.conf from user's configuration directory if already existing
	var cfgpath string
	if cfgdir != "" {
		cfgpath = filepath.Join(cfgdir, configFileName)
		_, err := os.Stat(cfgpath)
		if err == nil {
			return cfgpath
		}
	}

	// Use .rclone.conf from user's home directory if already existing
	var homeconf string
	if homeDir != "" {
		homeconf = filepath.Join(homeDir, hiddenConfigFileName)
		_, err := os.Stat(homeconf)
		if err == nil {
			return homeconf
		}
	}

	// Check to see if user supplied a --config variable or environment
	// variable.  We can't use pflag for this because it isn't initialised
	// yet so we search the command line manually.
	_, configSupplied := os.LookupEnv("RCLONE_CONFIG")
	if !configSupplied {
		for _, item := range os.Args {
			if item == "--config" || strings.HasPrefix(item, "--config=") {
				configSupplied = true
				break
			}
		}
	}

	// If user's configuration directory was found, then try to create it
	// and assume rclone.conf can be written there. If user supplied config
	// then skip creating the directory since it will not be used.
	if cfgpath != "" {
		// cfgpath != "" implies cfgdir != ""
		if configSupplied {
			return cfgpath
		}
		err := os.MkdirAll(cfgdir, os.ModePerm)
		if err == nil {
			return cfgpath
		}
	}

	// Assume .rclone.conf can be written to user's home directory.
	if homeconf != "" {
		return homeconf
	}

	// Default to ./.rclone.conf (current working directory) if everything else fails.
	if !configSupplied {
		fs.Errorf(nil, "Couldn't find home directory or read HOME or XDG_CONFIG_HOME environment variables.")
		fs.Errorf(nil, "Defaulting to storing config in current directory.")
		fs.Errorf(nil, "Use --config flag to workaround.")
		fs.Errorf(nil, "Error was: %v", err)
	}
	return hiddenConfigFileName
}

// LoadConfig loads the config file
func LoadConfig(ctx context.Context) {
	// Set RCLONE_CONFIG_DIR for backend config and subprocesses
	_ = os.Setenv("RCLONE_CONFIG_DIR", filepath.Dir(ConfigPath))

	// Load configuration file.
	if err := Data.Load(); err == ErrorConfigFileNotFound {
		fs.Logf(nil, "Config file %q not found - using defaults", ConfigPath)
	} else if err != nil {
		log.Fatalf("Failed to load config file %q: %v", ConfigPath, err)
	} else {
		fs.Debugf(nil, "Using config file from %q", ConfigPath)
	}
}

// ErrorConfigFileNotFound is returned when the config file is not found
var ErrorConfigFileNotFound = errors.New("config file not found")

// SaveConfig calling function which saves configuration file.
// if SaveConfig returns error trying again after sleep.
func SaveConfig() {
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
	return
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
