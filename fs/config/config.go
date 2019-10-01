// Package config reads, writes and edits the config file and deals with command line flags
package config

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	mathrand "math/rand"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/lib/random"
	"golang.org/x/crypto/nacl/secretbox"
)

const (
	configFileFolder     = "rclone"
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

	// ConfigAuthorize indicates that we just want "rclone authorize"
	ConfigAuthorize = "config_authorize"
)

// Global
var (
	provider Provider

	// ConfigPath points to the config file
	ConfigPath 	= makeConfigPath(configFileName, configFileFolder, hiddenConfigFileName)

	// CacheDir points to the cache directory.  Users of this
	// should make a subdirectory and use MkdirAll() to create it
	// and any parents.
	CacheDir = makeCacheDir(configFileFolder)

	// Key to use for password en/decryption.
	// When nil, no encryption will be used for saving.
	configKey []byte

	// output of prompt for password
	PasswordPromptOutput = os.Stderr

	// If set to true, the configKey is obscured with obscure.Obscure and saved to a temp file when it is
	// calculated from the password. The path of that temp file is then written to the environment variable
	// `_RCLONE_CONFIG_KEY_FILE`. If `_RCLONE_CONFIG_KEY_FILE` is present, password prompt is skipped and `RCLONE_CONFIG_PASS` ignored.
	// For security reasons, the temp file is deleted once the configKey is successfully loaded.
	// This can be used to pass the configKey to a child process.
	PassConfigKeyForDaemonization = false

	// Password can be used to configure the random password generator
	Password = random.Password

	errorConfigFileNotFound = errors.New("config file not found")

	configProviders []*ProviderDefinition
)

func RegisterConfigProvider(pd *ProviderDefinition) {
	configProviders = append(configProviders, pd)
}

type ProviderDefinition struct {
	NewFunc func()Provider
	FileTypes []string
}

type Provider interface {
	String() string
	Load(io.Reader) error
	Save(io.Writer) error
	GetRemoteConfig() RemoteConfig
}

type GlobalProvider interface {
	GetString(key string) string
	SetString(key string, value string)
}

type RemoteConfig interface {
	GetRemotes() []string
	HasRemote(remote string) bool
	GetRemote(remote string) Section
	CreateRemote(remote string) Section
	DeleteRemote(name string)
	RenameRemote(oldName string, newName string)
	CopyRemote(source string, destination string)
}

type Section interface {
	GetKeys() []string
	GetConfig() map[string]interface{}
	Remove(name string)

	Get(name string) interface{}
	GetString(name string) string
	GetStringDefault(name string, default_ string) string

	SetString(name string, value string)
	SetInt(name string, value int)
	Set(name string, value interface{})
}

func GetProvider() Provider {
	return provider
}

func GetRemoteConfig() RemoteConfig {
	return provider.GetRemoteConfig()
}

func init() {
	fs.ConfigFileGet = func(remote, key string) (string, bool) {
		if GetRemoteConfig().HasRemote(remote) {
			return GetRemoteConfig().GetRemote(remote).GetString(key), true
		}
		return "", false
	}

	fs.ConfigFileSet = func(remote, key, value string) (err error) {
		GetRemoteConfig().GetRemote(remote).SetString(key, value)
		return nil
	}
}

// Save calling function which saves configuration file.
// if saveConfig returns error trying again after sleep.
func SaveConfig() {
	var err error
	for i := 0; i < fs.Config.LowLevelRetries+1; i++ {
		if err = saveConfig(); err == nil {
			return
		}
		waitingTimeMs := mathrand.Intn(1000)
		time.Sleep(time.Duration(waitingTimeMs) * time.Millisecond)
	}
	log.Fatalf("Failed to save config after %d tries: %v", fs.Config.LowLevelRetries, err)

	return
}

// LoadConfig loads the config file
func LoadConfig() {
	for _, cp := range configProviders {
		for _, ft := range cp.FileTypes {
			if ft == path.Ext(ConfigPath)[1:] {
				provider = cp.NewFunc()
				break
			}
		}
	}

	// Load configuration file.
	var err error

	err = loadConfigFile()
	if err == errorConfigFileNotFound {
		fs.Logf(nil, "Config file %q not found - using defaults", ConfigPath)
	} else if err != nil {
		log.Fatalf("Failed to load config file %q: %v", ConfigPath, err)
	} else {
		fs.Debugf(nil, "Using config file from %q", ConfigPath)
	}

	// Start the token bucket limiter
	accounting.StartTokenBucket()

	// Start the bandwidth update ticker
	accounting.StartTokenTicker()

	// Start the transactions per second limiter
	fshttp.StartHTTPTokenBucket()
}

// Return the path to the configuration file
func makeConfigPath(configFileName, configFileFolder, hiddenConfigFileName string) string {
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
		cfgdir = filepath.Join(xdgdir, configFileFolder)
	} else if homeDir != "" {
		// User's configuration directory for rclone is $HOME/.config/rclone
		cfgdir = filepath.Join(homeDir, ".config", configFileFolder)
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

// findConfigFile will load a config file, and
// automatically decrypt it.
func loadConfigFile() (error) {
	envpw := os.Getenv("RCLONE_CONFIG_PASS")
	if len(configKey) == 0 && envpw != "" {
		err := setConfigPassword(envpw)
		if err != nil {
			fs.Errorf(nil, "Using RCLONE_CONFIG_PASS returned: %v", err)
		} else {
			fs.Debugf(nil, "Using RCLONE_CONFIG_PASS password.")
		}
	}

	b, err := ioutil.ReadFile(ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return errorConfigFileNotFound
		}
		return err
	}

	// Find first non-empty line
	r := bufio.NewReader(bytes.NewBuffer(b))
	for {
		line, _, err := r.ReadLine()
		if err != nil {
			if err == io.EOF {
				return provider.Load(bytes.NewReader(b))
			}
			return err
		}
		l := strings.TrimSpace(string(line))
		if len(l) == 0 || strings.HasPrefix(l, ";") || strings.HasPrefix(l, "#") {
			continue
		}
		// First non-empty or non-comment must be ENCRYPT_V0
		if l == "RCLONE_ENCRYPT_V0:" {
			break
		}
		if strings.HasPrefix(l, "RCLONE_ENCRYPT_V") {
			return errors.New("unsupported configuration encryption - update rclone for support")
		}
		return provider.Load(bytes.NewReader(b))
	}

	// Encrypted content is base64 encoded.
	dec := base64.NewDecoder(base64.StdEncoding, r)
	box, err := ioutil.ReadAll(dec)
	if err != nil {
		return errors.Wrap(err, "failed to load base64 encoded data")
	}
	if len(box) < 24+secretbox.Overhead {
		return errors.New("Configuration data too short")
	}

	var out []byte
	for {
		if envKeyFile := os.Getenv("_RCLONE_CONFIG_KEY_FILE"); len(envKeyFile) > 0 {
			fs.Debugf(nil, "attempting to obtain configKey from temp file %s", envKeyFile)
			obscuredKey, err := ioutil.ReadFile(envKeyFile)
			if err != nil {
				errRemove := os.Remove(envKeyFile)
				if errRemove != nil {
					log.Fatalf("unable to read obscured config key and unable to delete the temp file: %v", err)
				}
				log.Fatalf("unable to read obscured config key: %v", err)
			}
			errRemove := os.Remove(envKeyFile)
			if errRemove != nil {
				log.Fatalf("unable to delete temp file with configKey: %v", err)
			}
			configKey = []byte(obscure.MustReveal(string(obscuredKey)))
			fs.Debugf(nil, "using _RCLONE_CONFIG_KEY_FILE for configKey")
		} else {
			if len(configKey) == 0 {
				if !fs.Config.AskPassword {
					return errors.New("unable to decrypt configuration and not allowed to ask for password - set RCLONE_CONFIG_PASS to your configuration password")
				}
				getConfigPassword("Enter configuration password:")
			}
		}

		// Nonce is first 24 bytes of the ciphertext
		var nonce [24]byte
		copy(nonce[:], box[:24])
		var key [32]byte
		copy(key[:], configKey[:32])

		// Attempt to decrypt
		var ok bool
		out, ok = secretbox.Open(nil, box[24:], &nonce, &key)
		if ok {
			break
		}

		// Retry
		fs.Errorf(nil, "Couldn't decrypt configuration, most likely wrong password.")
		configKey = nil
	}
	return provider.Load(bytes.NewReader(out))
}

// saveConfig saves configuration file.
// if configKey has been set, the file will be encrypted.
func saveConfig() error {
	dir, name := filepath.Split(ConfigPath)
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "failed to create config directory")
	}
	f, err := ioutil.TempFile(dir, name)
	if err != nil {
		return errors.Errorf("Failed to create temp file for new config: %v", err)
	}
	defer func() {
		if err := os.Remove(f.Name()); err != nil && !os.IsNotExist(err) {
			fs.Errorf(nil, "Failed to remove temp config file: %v", err)
		}
	}()

	var buf bytes.Buffer

	err = provider.Save(&buf)
	if err != nil {
		return errors.Errorf("Failed to save config file: %v", err)
	}

	if len(configKey) == 0 {
		if _, err := buf.WriteTo(f); err != nil {
			return errors.Errorf("Failed to write temp config file: %v", err)
		}
	} else {
		_, _ = fmt.Fprintln(f, "# Encrypted rclone configuration File")
		_, _ = fmt.Fprintln(f, "")
		_, _ = fmt.Fprintln(f, "RCLONE_ENCRYPT_V0:")

		// Generate new nonce and write it to the start of the ciphertext
		var nonce [24]byte
		n, _ := rand.Read(nonce[:])
		if n != 24 {
			return errors.Errorf("nonce short read: %d", n)
		}
		enc := base64.NewEncoder(base64.StdEncoding, f)
		_, err = enc.Write(nonce[:])
		if err != nil {
			return errors.Errorf("Failed to write temp config file: %v", err)
		}

		var key [32]byte
		copy(key[:], configKey[:32])

		b := secretbox.Seal(nil, buf.Bytes(), &nonce, &key)
		_, err = enc.Write(b)
		if err != nil {
			return errors.Errorf("Failed to write temp config file: %v", err)
		}
		_ = enc.Close()
	}

	err = f.Close()
	if err != nil {
		return errors.Errorf("Failed to close config file: %v", err)
	}

	var fileMode os.FileMode = 0600
	info, err := os.Stat(ConfigPath)
	if err != nil {
		fs.Debugf(nil, "Using default permissions for config file: %v", fileMode)
	} else if info.Mode() != fileMode {
		fs.Debugf(nil, "Keeping previous permissions for config file: %v", info.Mode())
		fileMode = info.Mode()
	}

	attemptCopyGroup(ConfigPath, f.Name())

	err = os.Chmod(f.Name(), fileMode)
	if err != nil {
		fs.Errorf(nil, "Failed to set permissions on config file: %v", err)
	}

	if err = os.Rename(ConfigPath, ConfigPath+".old"); err != nil && !os.IsNotExist(err) {
		return errors.Errorf("Failed to move previous config to backup location: %v", err)
	}
	if err = os.Rename(f.Name(), ConfigPath); err != nil {
		return errors.Errorf("Failed to move newly written config from %s to final location: %v", f.Name(), err)
	}
	if err := os.Remove(ConfigPath + ".old"); err != nil && !os.IsNotExist(err) {
		fs.Errorf(nil, "Failed to remove backup config file: %v", err)
	}
	return nil
}
