// Package config reads, writes and edits the config file and deals with command line flags
package config

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	mathrand "math/rand"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Unknwon/goconfig"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/driveletter"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/lib/random"
	"golang.org/x/crypto/nacl/secretbox"
	"golang.org/x/text/unicode/norm"
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

	// ConfigAuthorize indicates that we just want "rclone authorize"
	ConfigAuthorize = "config_authorize"
)

// Global
var (
	// configFile is the global config data structure. Don't read it directly, use getConfigData()
	configFile *goconfig.ConfigFile

	// ConfigPath points to the config file
	ConfigPath = makeConfigPath()

	// CacheDir points to the cache directory.  Users of this
	// should make a subdirectory and use MkdirAll() to create it
	// and any parents.
	CacheDir = makeCacheDir()

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
)

func init() {
	// Set the function pointers up in fs
	fs.ConfigFileGet = FileGetFlag
	fs.ConfigFileSet = SetValueAndSave
}

func getConfigData() *goconfig.ConfigFile {
	if configFile == nil {
		LoadConfig()
	}
	return configFile
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
func LoadConfig() {
	// Load configuration file.
	var err error
	configFile, err = loadConfigFile()
	if err == errorConfigFileNotFound {
		fs.Logf(nil, "Config file %q not found - using defaults", ConfigPath)
		configFile, _ = goconfig.LoadFromReader(&bytes.Buffer{})
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

var errorConfigFileNotFound = errors.New("config file not found")

// loadConfigFile will load a config file, and
// automatically decrypt it.
func loadConfigFile() (*goconfig.ConfigFile, error) {
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
			return nil, errorConfigFileNotFound
		}
		return nil, err
	}
	// Find first non-empty line
	r := bufio.NewReader(bytes.NewBuffer(b))
	for {
		line, _, err := r.ReadLine()
		if err != nil {
			if err == io.EOF {
				return goconfig.LoadFromReader(bytes.NewBuffer(b))
			}
			return nil, err
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
			return nil, errors.New("unsupported configuration encryption - update rclone for support")
		}
		return goconfig.LoadFromReader(bytes.NewBuffer(b))
	}

	// Encrypted content is base64 encoded.
	dec := base64.NewDecoder(base64.StdEncoding, r)
	box, err := ioutil.ReadAll(dec)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load base64 encoded data")
	}
	if len(box) < 24+secretbox.Overhead {
		return nil, errors.New("Configuration data too short")
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
					return nil, errors.New("unable to decrypt configuration and not allowed to ask for password - set RCLONE_CONFIG_PASS to your configuration password")
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
	return goconfig.LoadFromReader(bytes.NewBuffer(out))
}

// checkPassword normalises and validates the password
func checkPassword(password string) (string, error) {
	if !utf8.ValidString(password) {
		return "", errors.New("password contains invalid utf8 characters")
	}
	// Check for leading/trailing whitespace
	trimmedPassword := strings.TrimSpace(password)
	// Warn user if password has leading+trailing whitespace
	if len(password) != len(trimmedPassword) {
		_, _ = fmt.Fprintln(os.Stderr, "Your password contains leading/trailing whitespace - in previous versions of rclone this was stripped")
	}
	// Normalize to reduce weird variations.
	password = norm.NFKC.String(password)
	if len(password) == 0 || len(trimmedPassword) == 0 {
		return "", errors.New("no characters in password")
	}
	return password, nil
}

// GetPassword asks the user for a password with the prompt given.
func GetPassword(prompt string) string {
	_, _ = fmt.Fprintln(PasswordPromptOutput, prompt)
	for {
		_, _ = fmt.Fprint(PasswordPromptOutput, "password:")
		password := ReadPassword()
		password, err := checkPassword(password)
		if err == nil {
			return password
		}
		_, _ = fmt.Fprintf(os.Stderr, "Bad password: %v\n", err)
	}
}

// ChangePassword will query the user twice for the named password. If
// the same password is entered it is returned.
func ChangePassword(name string) string {
	for {
		a := GetPassword(fmt.Sprintf("Enter %s password:", name))
		b := GetPassword(fmt.Sprintf("Confirm %s password:", name))
		if a == b {
			return a
		}
		fmt.Println("Passwords do not match!")
	}
}

// getConfigPassword will query the user for a password the
// first time it is required.
func getConfigPassword(q string) {
	if len(configKey) != 0 {
		return
	}
	for {
		password := GetPassword(q)
		err := setConfigPassword(password)
		if err == nil {
			return
		}
		_, _ = fmt.Fprintln(os.Stderr, "Error:", err)
	}
}

// setConfigPassword will set the configKey to the hash of
// the password. If the length of the password is
// zero after trimming+normalization, an error is returned.
func setConfigPassword(password string) error {
	password, err := checkPassword(password)
	if err != nil {
		return err
	}
	// Create SHA256 has of the password
	sha := sha256.New()
	_, err = sha.Write([]byte("[" + password + "][rclone-config]"))
	if err != nil {
		return err
	}
	configKey = sha.Sum(nil)
	if PassConfigKeyForDaemonization {
		tempFile, err := ioutil.TempFile("", "rclone")
		if err != nil {
			log.Fatalf("cannot create temp file to store configKey: %v", err)
		}
		_, err = tempFile.WriteString(obscure.MustObscure(string(configKey)))
		if err != nil {
			errRemove := os.Remove(tempFile.Name())
			if errRemove != nil {
				log.Fatalf("error writing configKey to temp file and also error deleting it: %v", err)
			}
			log.Fatalf("error writing configKey to temp file: %v", err)
		}
		err = tempFile.Close()
		if err != nil {
			errRemove := os.Remove(tempFile.Name())
			if errRemove != nil {
				log.Fatalf("error closing temp file with configKey and also error deleting it: %v", err)
			}
			log.Fatalf("error closing temp file with configKey: %v", err)
		}
		fs.Debugf(nil, "saving configKey to temp file")
		err = os.Setenv("_RCLONE_CONFIG_KEY_FILE", tempFile.Name())
		if err != nil {
			errRemove := os.Remove(tempFile.Name())
			if errRemove != nil {
				log.Fatalf("unable to set environment variable _RCLONE_CONFIG_KEY_FILE and unable to delete the temp file: %v", err)
			}
			log.Fatalf("unable to set environment variable _RCLONE_CONFIG_KEY_FILE: %v", err)
		}
	}
	return nil
}

// changeConfigPassword will query the user twice
// for a password. If the same password is entered
// twice the key is updated.
func changeConfigPassword() {
	err := setConfigPassword(ChangePassword("NEW configuration"))
	if err != nil {
		fmt.Printf("Failed to set config password: %v\n", err)
		return
	}
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
	err = goconfig.SaveConfigData(getConfigData(), &buf)
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

// SaveConfig calling function which saves configuration file.
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

// SetValueAndSave sets the key to the value and saves just that
// value in the config file.  It loads the old config file in from
// disk first and overwrites the given value only.
func SetValueAndSave(name, key, value string) (err error) {
	// Set the value in config in case we fail to reload it
	getConfigData().SetValue(name, key, value)
	// Reload the config file
	reloadedConfigFile, err := loadConfigFile()
	if err == errorConfigFileNotFound {
		// Config file not written yet so ignore reload
		return nil
	} else if err != nil {
		return err
	}
	_, err = reloadedConfigFile.GetSection(name)
	if err != nil {
		// Section doesn't exist yet so ignore reload
		return err
	}
	// Update the config file with the reloaded version
	configFile = reloadedConfigFile
	// Set the value in the reloaded version
	reloadedConfigFile.SetValue(name, key, value)
	// Save it again
	SaveConfig()
	return nil
}

// FileGetFresh reads the config key under section return the value or
// an error if the config file was not found or that value couldn't be
// read.
func FileGetFresh(section, key string) (value string, err error) {
	reloadedConfigFile, err := loadConfigFile()
	if err != nil {
		return "", err
	}
	return reloadedConfigFile.GetValue(section, key)
}

// ShowRemotes shows an overview of the config file
func ShowRemotes() {
	remotes := getConfigData().GetSectionList()
	if len(remotes) == 0 {
		return
	}
	sort.Strings(remotes)
	fmt.Printf("%-20s %s\n", "Name", "Type")
	fmt.Printf("%-20s %s\n", "====", "====")
	for _, remote := range remotes {
		fmt.Printf("%-20s %s\n", remote, FileGet(remote, "type"))
	}
}

// ChooseRemote chooses a remote name
func ChooseRemote() string {
	remotes := getConfigData().GetSectionList()
	sort.Strings(remotes)
	return Choose("remote", remotes, nil, false)
}

// ReadLine reads some input
var ReadLine = func() string {
	buf := bufio.NewReader(os.Stdin)
	line, err := buf.ReadString('\n')
	if err != nil {
		log.Fatalf("Failed to read line: %v", err)
	}
	return strings.TrimSpace(line)
}

// Command - choose one
func Command(commands []string) byte {
	opts := []string{}
	for _, text := range commands {
		fmt.Printf("%c) %s\n", text[0], text[1:])
		opts = append(opts, text[:1])
	}
	optString := strings.Join(opts, "")
	optHelp := strings.Join(opts, "/")
	for {
		fmt.Printf("%s> ", optHelp)
		result := strings.ToLower(ReadLine())
		if len(result) != 1 {
			continue
		}
		i := strings.Index(optString, string(result[0]))
		if i >= 0 {
			return result[0]
		}
	}
}

// Confirm asks the user for Yes or No and returns true or false
//
// If AutoConfirm is set, it will return true
func Confirm() bool {
	return Command([]string{"yYes", "nNo"}) == 'y'
}

// ConfirmWithConfig asks the user for Yes or No and returns true or
// false.
//
// If AutoConfirm is set, it will look up the value in m and return
// that, but if it isn't set then it will return the Default value
// passed in
func ConfirmWithConfig(m configmap.Getter, configName string, Default bool) bool {
	if fs.Config.AutoConfirm {
		configString, ok := m.Get(configName)
		if ok {
			configValue, err := strconv.ParseBool(configString)
			if err != nil {
				fs.Errorf(nil, "Failed to parse config parameter %s=%q as boolean - using default %v: %v", configName, configString, Default, err)
			} else {
				Default = configValue
			}
		}
		answer := "No"
		if Default {
			answer = "Yes"
		}
		fmt.Printf("Auto confirm is set: answering %s, override by setting config parameter %s=%v\n", answer, configName, !Default)
		return Default
	}
	return Confirm()
}

// Choose one of the defaults or type a new string if newOk is set
func Choose(what string, defaults, help []string, newOk bool) string {
	valueDescription := "an existing"
	if newOk {
		valueDescription = "your own"
	}
	fmt.Printf("Choose a number from below, or type in %s value\n", valueDescription)
	for i, text := range defaults {
		var lines []string
		if help != nil {
			parts := strings.Split(help[i], "\n")
			lines = append(lines, parts...)
		}
		lines = append(lines, fmt.Sprintf("%q", text))
		pos := i + 1
		if len(lines) == 1 {
			fmt.Printf("%2d > %s\n", pos, text)
		} else {
			mid := (len(lines) - 1) / 2
			for i, line := range lines {
				var sep rune
				switch i {
				case 0:
					sep = '/'
				case len(lines) - 1:
					sep = '\\'
				default:
					sep = '|'
				}
				number := "  "
				if i == mid {
					number = fmt.Sprintf("%2d", pos)
				}
				fmt.Printf("%s %c %s\n", number, sep, line)
			}
		}
	}
	for {
		fmt.Printf("%s> ", what)
		result := ReadLine()
		i, err := strconv.Atoi(result)
		if err != nil {
			if newOk {
				return result
			}
			for _, v := range defaults {
				if result == v {
					return result
				}
			}
			continue
		}
		if i >= 1 && i <= len(defaults) {
			return defaults[i-1]
		}
	}
}

// ChooseNumber asks the user to enter a number between min and max
// inclusive prompting them with what.
func ChooseNumber(what string, min, max int) int {
	for {
		fmt.Printf("%s> ", what)
		result := ReadLine()
		i, err := strconv.Atoi(result)
		if err != nil {
			fmt.Printf("Bad number: %v\n", err)
			continue
		}
		if i < min || i > max {
			fmt.Printf("Out of range - %d to %d inclusive\n", min, max)
			continue
		}
		return i
	}
}

// ShowRemote shows the contents of the remote
func ShowRemote(name string) {
	fmt.Printf("--------------------\n")
	fmt.Printf("[%s]\n", name)
	fs := MustFindByName(name)
	for _, key := range getConfigData().GetKeyList(name) {
		isPassword := false
		for _, option := range fs.Options {
			if option.Name == key && option.IsPassword {
				isPassword = true
				break
			}
		}
		value := FileGet(name, key)
		if isPassword && value != "" {
			fmt.Printf("%s = *** ENCRYPTED ***\n", key)
		} else {
			fmt.Printf("%s = %s\n", key, value)
		}
	}
	fmt.Printf("--------------------\n")
}

// OkRemote prints the contents of the remote and ask if it is OK
func OkRemote(name string) bool {
	ShowRemote(name)
	switch i := Command([]string{"yYes this is OK", "eEdit this remote", "dDelete this remote"}); i {
	case 'y':
		return true
	case 'e':
		return false
	case 'd':
		getConfigData().DeleteSection(name)
		return true
	default:
		fs.Errorf(nil, "Bad choice %c", i)
	}
	return false
}

// MustFindByName finds the RegInfo for the remote name passed in or
// exits with a fatal error.
func MustFindByName(name string) *fs.RegInfo {
	fsType := FileGet(name, "type")
	if fsType == "" {
		log.Fatalf("Couldn't find type of fs for %q", name)
	}
	return fs.MustFind(fsType)
}

// RemoteConfig runs the config helper for the remote if needed
func RemoteConfig(name string) {
	fmt.Printf("Remote config\n")
	f := MustFindByName(name)
	if f.Config != nil {
		m := fs.ConfigMap(f, name)
		f.Config(name, m)
	}
}

// matchProvider returns true if provider matches the providerConfig string.
//
// The providerConfig string can either be a list of providers to
// match, or if it starts with "!" it will be a list of providers not
// to match.
//
// If either providerConfig or provider is blank then it will return true
func matchProvider(providerConfig, provider string) bool {
	if providerConfig == "" || provider == "" {
		return true
	}
	negate := false
	if strings.HasPrefix(providerConfig, "!") {
		providerConfig = providerConfig[1:]
		negate = true
	}
	providers := strings.Split(providerConfig, ",")
	matched := false
	for _, p := range providers {
		if p == provider {
			matched = true
			break
		}
	}
	if negate {
		return !matched
	}
	return matched
}

// ChooseOption asks the user to choose an option
func ChooseOption(o *fs.Option, name string) string {
	var subProvider = getConfigData().MustValue(name, fs.ConfigProvider, "")
	fmt.Println(o.Help)
	if o.IsPassword {
		actions := []string{"yYes type in my own password", "gGenerate random password"}
		if !o.Required {
			actions = append(actions, "nNo leave this optional password blank")
		}
		var password string
		switch i := Command(actions); i {
		case 'y':
			password = ChangePassword("the")
		case 'g':
			for {
				fmt.Printf("Password strength in bits.\n64 is just about memorable\n128 is secure\n1024 is the maximum\n")
				bits := ChooseNumber("Bits", 64, 1024)
				password, err := random.Password(bits)
				if err != nil {
					log.Fatalf("Failed to make password: %v", err)
				}
				fmt.Printf("Your password is: %s\n", password)
				fmt.Printf("Use this password? Please note that an obscured version of this \npassword (and not the " +
					"password itself) will be stored under your \nconfiguration file, so keep this generated password " +
					"in a safe place.\n")
				if Confirm() {
					break
				}
			}
		case 'n':
			return ""
		default:
			fs.Errorf(nil, "Bad choice %c", i)
		}
		return obscure.MustObscure(password)
	}
	what := fmt.Sprintf("%T value", o.Default)
	switch o.Default.(type) {
	case bool:
		what = "boolean value (true or false)"
	case fs.SizeSuffix:
		what = "size with suffix k,M,G,T"
	case fs.Duration:
		what = "duration s,m,h,d,w,M,y"
	case int, int8, int16, int32, int64:
		what = "signed integer"
	case uint, byte, uint16, uint32, uint64:
		what = "unsigned integer"
	}
	var in string
	for {
		fmt.Printf("Enter a %s. Press Enter for the default (%q).\n", what, fmt.Sprint(o.Default))
		if len(o.Examples) > 0 {
			var values []string
			var help []string
			for _, example := range o.Examples {
				if matchProvider(example.Provider, subProvider) {
					values = append(values, example.Value)
					help = append(help, example.Help)
				}
			}
			in = Choose(o.Name, values, help, true)
		} else {
			fmt.Printf("%s> ", o.Name)
			in = ReadLine()
		}
		if in == "" {
			if o.Required && fmt.Sprint(o.Default) == "" {
				fmt.Printf("This value is required and it has no default.\n")
				continue
			}
			break
		}
		newIn, err := configstruct.StringToInterface(o.Default, in)
		if err != nil {
			fmt.Printf("Failed to parse %q: %v\n", in, err)
			continue
		}
		in = fmt.Sprint(newIn) // canonicalise
		break
	}
	return in
}

// Suppress the confirm prompts and return a function to undo that
func suppressConfirm() func() {
	old := fs.Config.AutoConfirm
	fs.Config.AutoConfirm = true
	return func() {
		fs.Config.AutoConfirm = old
	}
}

// UpdateRemote adds the keyValues passed in to the remote of name.
// keyValues should be key, value pairs.
func UpdateRemote(name string, keyValues rc.Params) error {
	defer suppressConfirm()()

	// Work out which options need to be obscured
	needsObscure := map[string]struct{}{}
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

	// Set the config
	for k, v := range keyValues {
		vStr := fmt.Sprint(v)
		// Obscure parameter if necessary
		if _, ok := needsObscure[k]; ok {
			_, err := obscure.Reveal(vStr)
			if err != nil {
				// If error => not already obscured, so obscure it
				vStr, err = obscure.Obscure(vStr)
				if err != nil {
					return errors.Wrap(err, "UpdateRemote: obscure failed")
				}
			}
		}
		getConfigData().SetValue(name, k, vStr)
	}
	RemoteConfig(name)
	SaveConfig()
	return nil
}

// CreateRemote creates a new remote with name, provider and a list of
// parameters which are key, value pairs.  If update is set then it
// adds the new keys rather than replacing all of them.
func CreateRemote(name string, provider string, keyValues rc.Params) error {
	// Delete the old config if it exists
	getConfigData().DeleteSection(name)
	// Set the type
	getConfigData().SetValue(name, "type", provider)
	// Set the remaining values
	return UpdateRemote(name, keyValues)
}

// PasswordRemote adds the keyValues passed in to the remote of name.
// keyValues should be key, value pairs.
func PasswordRemote(name string, keyValues rc.Params) error {
	defer suppressConfirm()()
	for k, v := range keyValues {
		keyValues[k] = obscure.MustObscure(fmt.Sprint(v))
	}
	return UpdateRemote(name, keyValues)
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

// NewRemoteName asks the user for a name for a remote
func NewRemoteName() (name string) {
	for {
		fmt.Printf("name> ")
		name = ReadLine()
		parts := fspath.Matcher.FindStringSubmatch(name + ":")
		switch {
		case name == "":
			fmt.Printf("Can't use empty name.\n")
		case driveletter.IsDriveLetter(name):
			fmt.Printf("Can't use %q as it can be confused with a drive letter.\n", name)
		case parts == nil:
			fmt.Printf("Can't use %q as it has invalid characters in it.\n", name)
		default:
			return name
		}
	}
}

// editOptions edits the options.  If new is true then it just allows
// entry and doesn't show any old values.
func editOptions(ri *fs.RegInfo, name string, isNew bool) {
	fmt.Printf("** See help for %s backend at: https://rclone.org/%s/ **\n\n", ri.Name, ri.FileName())
	hasAdvanced := false
	for _, advanced := range []bool{false, true} {
		if advanced {
			if !hasAdvanced {
				break
			}
			fmt.Printf("Edit advanced config? (y/n)\n")
			if !Confirm() {
				break
			}
		}
		for _, option := range ri.Options {
			isVisible := option.Hide&fs.OptionHideConfigurator == 0
			hasAdvanced = hasAdvanced || (option.Advanced && isVisible)
			if option.Advanced != advanced {
				continue
			}
			subProvider := getConfigData().MustValue(name, fs.ConfigProvider, "")
			if matchProvider(option.Provider, subProvider) && isVisible {
				if !isNew {
					fmt.Printf("Value %q = %q\n", option.Name, FileGet(name, option.Name))
					fmt.Printf("Edit? (y/n)>\n")
					if !Confirm() {
						continue
					}
				}
				FileSet(name, option.Name, ChooseOption(&option, name))
			}
		}
	}
}

// NewRemote make a new remote from its name
func NewRemote(name string) {
	var (
		newType string
		ri      *fs.RegInfo
		err     error
	)

	// Set the type first
	for {
		newType = ChooseOption(fsOption(), name)
		ri, err = fs.Find(newType)
		if err != nil {
			fmt.Printf("Bad remote %q: %v\n", newType, err)
			continue
		}
		break
	}
	getConfigData().SetValue(name, "type", newType)

	editOptions(ri, name, true)
	RemoteConfig(name)
	if OkRemote(name) {
		SaveConfig()
		return
	}
	EditRemote(ri, name)
}

// EditRemote gets the user to edit a remote
func EditRemote(ri *fs.RegInfo, name string) {
	ShowRemote(name)
	fmt.Printf("Edit remote\n")
	for {
		editOptions(ri, name, false)
		if OkRemote(name) {
			break
		}
	}
	SaveConfig()
	RemoteConfig(name)
}

// DeleteRemote gets the user to delete a remote
func DeleteRemote(name string) {
	getConfigData().DeleteSection(name)
	SaveConfig()
}

// copyRemote asks the user for a new remote name and copies name into
// it. Returns the new name.
func copyRemote(name string) string {
	newName := NewRemoteName()
	// Copy the keys
	for _, key := range getConfigData().GetKeyList(name) {
		value := getConfigData().MustValue(name, key, "")
		getConfigData().SetValue(newName, key, value)
	}
	return newName
}

// RenameRemote renames a config section
func RenameRemote(name string) {
	fmt.Printf("Enter new name for %q remote.\n", name)
	newName := copyRemote(name)
	if name != newName {
		getConfigData().DeleteSection(name)
		SaveConfig()
	}
}

// CopyRemote copies a config section
func CopyRemote(name string) {
	fmt.Printf("Enter name for copy of %q remote.\n", name)
	copyRemote(name)
	SaveConfig()
}

// ShowConfigLocation prints the location of the config file in use
func ShowConfigLocation() {
	if _, err := os.Stat(ConfigPath); os.IsNotExist(err) {
		fmt.Println("Configuration file doesn't exist, but rclone will use this path:")
	} else {
		fmt.Println("Configuration file is stored at:")
	}
	fmt.Printf("%s\n", ConfigPath)
}

// ShowConfig prints the (unencrypted) config options
func ShowConfig() {
	var buf bytes.Buffer
	if err := goconfig.SaveConfigData(getConfigData(), &buf); err != nil {
		log.Fatalf("Failed to serialize config: %v", err)
	}
	str := buf.String()
	if str == "" {
		str = "; empty config\n"
	}
	fmt.Printf("%s", str)
}

// EditConfig edits the config file interactively
func EditConfig() {
	for {
		haveRemotes := len(getConfigData().GetSectionList()) != 0
		what := []string{"eEdit existing remote", "nNew remote", "dDelete remote", "rRename remote", "cCopy remote", "sSet configuration password", "qQuit config"}
		if haveRemotes {
			fmt.Printf("Current remotes:\n\n")
			ShowRemotes()
			fmt.Printf("\n")
		} else {
			fmt.Printf("No remotes found - make a new one\n")
			// take 2nd item and last 2 items of menu list
			what = append(what[1:2], what[len(what)-2:]...)
		}
		switch i := Command(what); i {
		case 'e':
			name := ChooseRemote()
			fs := MustFindByName(name)
			EditRemote(fs, name)
		case 'n':
			NewRemote(NewRemoteName())
		case 'd':
			name := ChooseRemote()
			DeleteRemote(name)
		case 'r':
			RenameRemote(ChooseRemote())
		case 'c':
			CopyRemote(ChooseRemote())
		case 's':
			SetPassword()
		case 'q':
			return

		}
	}
}

// SetPassword will allow the user to modify the current
// configuration encryption settings.
func SetPassword() {
	for {
		if len(configKey) > 0 {
			fmt.Println("Your configuration is encrypted.")
			what := []string{"cChange Password", "uUnencrypt configuration", "qQuit to main menu"}
			switch i := Command(what); i {
			case 'c':
				changeConfigPassword()
				SaveConfig()
				fmt.Println("Password changed")
				continue
			case 'u':
				configKey = nil
				SaveConfig()
				continue
			case 'q':
				return
			}

		} else {
			fmt.Println("Your configuration is not encrypted.")
			fmt.Println("If you add a password, you will protect your login information to cloud services.")
			what := []string{"aAdd Password", "qQuit to main menu"}
			switch i := Command(what); i {
			case 'a':
				changeConfigPassword()
				SaveConfig()
				fmt.Println("Password set")
				continue
			case 'q':
				return
			}
		}
	}
}

// Authorize is for remote authorization of headless machines.
//
// It expects 1 or 3 arguments
//
//   rclone authorize "fs name"
//   rclone authorize "fs name" "client id" "client secret"
func Authorize(args []string) {
	defer suppressConfirm()()
	switch len(args) {
	case 1, 3:
	default:
		log.Fatalf("Invalid number of arguments: %d", len(args))
	}
	newType := args[0]
	f := fs.MustFind(newType)
	if f.Config == nil {
		log.Fatalf("Can't authorize fs %q", newType)
	}
	// Name used for temporary fs
	name := "**temp-fs**"

	// Make sure we delete it
	defer DeleteRemote(name)

	// Indicate that we are running rclone authorize
	getConfigData().SetValue(name, ConfigAuthorize, "true")
	if len(args) == 3 {
		getConfigData().SetValue(name, ConfigClientID, args[1])
		getConfigData().SetValue(name, ConfigClientSecret, args[2])
	}
	m := fs.ConfigMap(f, name)
	f.Config(name, m)
}

// FileGetFlag gets the config key under section returning the
// the value and true if found and or ("", false) otherwise
func FileGetFlag(section, key string) (string, bool) {
	newValue, err := getConfigData().GetValue(section, key)
	return newValue, err == nil
}

// FileGet gets the config key under section returning the
// default or empty string if not set.
//
// It looks up defaults in the environment if they are present
func FileGet(section, key string, defaultVal ...string) string {
	envKey := fs.ConfigToEnv(section, key)
	newValue, found := os.LookupEnv(envKey)
	if found {
		defaultVal = []string{newValue}
	}
	return getConfigData().MustValue(section, key, defaultVal...)
}

// FileSet sets the key in section to value.  It doesn't save
// the config file.
func FileSet(section, key, value string) {
	if value != "" {
		getConfigData().SetValue(section, key, value)
	} else {
		FileDeleteKey(section, key)
	}
}

// FileDeleteKey deletes the config key in the config file.
// It returns true if the key was deleted,
// or returns false if the section or key didn't exist.
func FileDeleteKey(section, key string) bool {
	return getConfigData().DeleteKey(section, key)
}

var matchEnv = regexp.MustCompile(`^RCLONE_CONFIG_(.*?)_TYPE=.*$`)

// FileRefresh ensures the latest configFile is loaded from disk
func FileRefresh() error {
	reloadedConfigFile, err := loadConfigFile()
	if err != nil {
		return err
	}
	configFile = reloadedConfigFile
	return nil
}

// FileSections returns the sections in the config file
// including any defined by environment variables.
func FileSections() []string {
	sections := getConfigData().GetSectionList()
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
	for _, key := range getConfigData().GetKeyList(name) {
		params[key] = FileGet(name, key)
	}
	return params
}

// DumpRcBlob dumps all the config as an unstructured blob suitable
// for the rc
func DumpRcBlob() (dump rc.Params) {
	dump = rc.Params{}
	for _, name := range getConfigData().GetSectionList() {
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
