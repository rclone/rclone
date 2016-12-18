// Read, write and edit the config file

package fs

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"os/user"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Unknwon/goconfig"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"golang.org/x/crypto/nacl/secretbox"
	"golang.org/x/text/unicode/norm"
)

const (
	configFileName = ".rclone.conf"

	// ConfigToken is the key used to store the token under
	ConfigToken = "token"

	// ConfigClientID is the config key used to store the client id
	ConfigClientID = "client_id"

	// ConfigClientSecret is the config key used to store the client secret
	ConfigClientSecret = "client_secret"

	// ConfigAutomatic indicates that we want non-interactive configuration
	ConfigAutomatic = "config_automatic"
)

// SizeSuffix is parsed by flag with k/M/G suffixes
type SizeSuffix int64

// Global
var (
	// ConfigFile is the config file data structure
	ConfigFile *goconfig.ConfigFile
	// HomeDir is the home directory of the user
	HomeDir = configHome()
	// ConfigPath points to the config file
	ConfigPath = path.Join(HomeDir, configFileName)
	// Config is the global config
	Config = &ConfigInfo{}
	// Flags
	verbose         = pflag.BoolP("verbose", "v", false, "Print lots more stuff")
	quiet           = pflag.BoolP("quiet", "q", false, "Print as little stuff as possible")
	modifyWindow    = pflag.DurationP("modify-window", "", time.Nanosecond, "Max time diff to be considered the same")
	checkers        = pflag.IntP("checkers", "", 8, "Number of checkers to run in parallel.")
	transfers       = pflag.IntP("transfers", "", 4, "Number of file transfers to run in parallel.")
	configFile      = pflag.StringP("config", "", ConfigPath, "Config file.")
	checkSum        = pflag.BoolP("checksum", "c", false, "Skip based on checksum & size, not mod-time & size")
	sizeOnly        = pflag.BoolP("size-only", "", false, "Skip based on size only, not mod-time or checksum")
	ignoreTimes     = pflag.BoolP("ignore-times", "I", false, "Don't skip files that match size and time - transfer all files")
	ignoreExisting  = pflag.BoolP("ignore-existing", "", false, "Skip all files that exist on destination")
	dryRun          = pflag.BoolP("dry-run", "n", false, "Do a trial run with no permanent changes")
	connectTimeout  = pflag.DurationP("contimeout", "", 60*time.Second, "Connect timeout")
	timeout         = pflag.DurationP("timeout", "", 5*60*time.Second, "IO idle timeout")
	dumpHeaders     = pflag.BoolP("dump-headers", "", false, "Dump HTTP headers - may contain sensitive info")
	dumpBodies      = pflag.BoolP("dump-bodies", "", false, "Dump HTTP headers and bodies - may contain sensitive info")
	dumpAuth        = pflag.BoolP("dump-auth", "", false, "Dump HTTP headers with auth info")
	skipVerify      = pflag.BoolP("no-check-certificate", "", false, "Do not verify the server SSL certificate. Insecure.")
	AskPassword     = pflag.BoolP("ask-password", "", true, "Allow prompt for password for encrypted configuration.")
	deleteBefore    = pflag.BoolP("delete-before", "", false, "When synchronizing, delete files on destination before transfering")
	deleteDuring    = pflag.BoolP("delete-during", "", false, "When synchronizing, delete files during transfer (default)")
	deleteAfter     = pflag.BoolP("delete-after", "", false, "When synchronizing, delete files on destination after transfering")
	trackRenames    = pflag.BoolP("track-renames", "", false, "When synchronizing, track file renames and do a server side move if possible")
	lowLevelRetries = pflag.IntP("low-level-retries", "", 10, "Number of low level retries to do.")
	updateOlder     = pflag.BoolP("update", "u", false, "Skip files that are newer on the destination.")
	noGzip          = pflag.BoolP("no-gzip-encoding", "", false, "Don't set Accept-Encoding: gzip.")
	maxDepth        = pflag.IntP("max-depth", "", -1, "If set limits the recursion depth to this.")
	ignoreSize      = pflag.BoolP("ignore-size", "", false, "Ignore size when skipping use mod-time or checksum.")
	noTraverse      = pflag.BoolP("no-traverse", "", false, "Don't traverse destination file system on copy.")
	noUpdateModTime = pflag.BoolP("no-update-modtime", "", false, "Don't update destination mod-time if files identical.")
	bwLimit         SizeSuffix

	// Key to use for password en/decryption.
	// When nil, no encryption will be used for saving.
	configKey []byte
)

func init() {
	pflag.VarP(&bwLimit, "bwlimit", "", "Bandwidth limit in kBytes/s, or use suffix b|k|M|G")
}

// Turn SizeSuffix into a string and a suffix
func (x SizeSuffix) string() (string, string) {
	scaled := float64(0)
	suffix := ""
	switch {
	case x < 0:
		return "off", ""
	case x == 0:
		return "0", ""
	case x < 1024:
		scaled = float64(x)
		suffix = ""
	case x < 1024*1024:
		scaled = float64(x) / 1024
		suffix = "k"
	case x < 1024*1024*1024:
		scaled = float64(x) / 1024 / 1024
		suffix = "M"
	default:
		scaled = float64(x) / 1024 / 1024 / 1024
		suffix = "G"
	}
	if math.Floor(scaled) == scaled {
		return fmt.Sprintf("%.0f", scaled), suffix
	}
	return fmt.Sprintf("%.3f", scaled), suffix
}

// String turns SizeSuffix into a string
func (x SizeSuffix) String() string {
	val, suffix := x.string()
	return val + suffix
}

// Unit turns SizeSuffix into a string with a unit
func (x SizeSuffix) Unit(unit string) string {
	val, suffix := x.string()
	if val == "off" {
		return val
	}
	return val + " " + suffix + unit
}

// Set a SizeSuffix
func (x *SizeSuffix) Set(s string) error {
	if len(s) == 0 {
		return errors.New("empty string")
	}
	if strings.ToLower(s) == "off" {
		*x = -1
		return nil
	}
	suffix := s[len(s)-1]
	suffixLen := 1
	var multiplier float64
	switch suffix {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '.':
		suffixLen = 0
		multiplier = 1 << 10
	case 'b', 'B':
		multiplier = 1
	case 'k', 'K':
		multiplier = 1 << 10
	case 'm', 'M':
		multiplier = 1 << 20
	case 'g', 'G':
		multiplier = 1 << 30
	default:
		return errors.Errorf("bad suffix %q", suffix)
	}
	s = s[:len(s)-suffixLen]
	value, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	if value < 0 {
		return errors.Errorf("size can't be negative %q", s)
	}
	value *= multiplier
	*x = SizeSuffix(value)
	return nil
}

// Type of the value
func (x *SizeSuffix) Type() string {
	return "int64"
}

// Check it satisfies the interface
var _ pflag.Value = (*SizeSuffix)(nil)

// crypt internals
var (
	cryptKey = []byte{
		0x9c, 0x93, 0x5b, 0x48, 0x73, 0x0a, 0x55, 0x4d,
		0x6b, 0xfd, 0x7c, 0x63, 0xc8, 0x86, 0xa9, 0x2b,
		0xd3, 0x90, 0x19, 0x8e, 0xb8, 0x12, 0x8a, 0xfb,
		0xf4, 0xde, 0x16, 0x2b, 0x8b, 0x95, 0xf6, 0x38,
	}
	cryptBlock cipher.Block
	cryptRand  = rand.Reader
)

// crypt transforms in to out using iv under AES-CTR.
//
// in and out may be the same buffer.
//
// Note encryption and decryption are the same operation
func crypt(out, in, iv []byte) error {
	if cryptBlock == nil {
		var err error
		cryptBlock, err = aes.NewCipher(cryptKey)
		if err != nil {
			return err
		}
	}
	stream := cipher.NewCTR(cryptBlock, iv)
	stream.XORKeyStream(out, in)
	return nil
}

// Obscure a value
//
// This is done by encrypting with AES-CTR
func Obscure(x string) (string, error) {
	plaintext := []byte(x)
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(cryptRand, iv); err != nil {
		return "", errors.Wrap(err, "failed to read iv")
	}
	if err := crypt(ciphertext[aes.BlockSize:], plaintext, iv); err != nil {
		return "", errors.Wrap(err, "encrypt failed")
	}
	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

// MustObscure obscures a value, exiting with a fatal error if it failed
func MustObscure(x string) string {
	out, err := Obscure(x)
	if err != nil {
		log.Fatalf("Obscure failed: %v", err)
	}
	return out
}

// Reveal an obscured value
func Reveal(x string) (string, error) {
	ciphertext, err := base64.RawURLEncoding.DecodeString(x)
	if err != nil {
		return "", errors.Wrap(err, "base64 decode failed")
	}
	if len(ciphertext) < aes.BlockSize {
		return "", errors.New("input too short")
	}
	buf := ciphertext[aes.BlockSize:]
	iv := ciphertext[:aes.BlockSize]
	if err := crypt(buf, buf, iv); err != nil {
		return "", errors.Wrap(err, "decrypt failed")
	}
	return string(buf), nil
}

// MustReveal reveals an obscured value, exiting with a fatal error if it failed
func MustReveal(x string) string {
	out, err := Reveal(x)
	if err != nil {
		log.Fatalf("Reveal failed: %v", err)
	}
	return out
}

// ConfigInfo is filesystem config options
type ConfigInfo struct {
	Verbose            bool
	Quiet              bool
	DryRun             bool
	CheckSum           bool
	SizeOnly           bool
	IgnoreTimes        bool
	IgnoreExisting     bool
	ModifyWindow       time.Duration
	Checkers           int
	Transfers          int
	ConnectTimeout     time.Duration // Connect timeout
	Timeout            time.Duration // Data channel timeout
	DumpHeaders        bool
	DumpBodies         bool
	DumpAuth           bool
	Filter             *Filter
	InsecureSkipVerify bool // Skip server certificate verification
	DeleteBefore       bool // Delete before checking
	DeleteDuring       bool // Delete during checking/transfer
	DeleteAfter        bool // Delete after successful transfer.
	TrackRenames       bool // Track file renames.
	LowLevelRetries    int
	UpdateOlder        bool // Skip files that are newer on the destination
	NoGzip             bool // Disable compression
	MaxDepth           int
	IgnoreSize         bool
	NoTraverse         bool
	NoUpdateModTime    bool
	DataRateUnit       string
}

// Find the config directory
func configHome() string {
	// Find users home directory
	usr, err := user.Current()
	if err == nil {
		return usr.HomeDir
	}
	// Fall back to reading $HOME - work around user.Current() not
	// working for cross compiled binaries on OSX.
	// https://github.com/golang/go/issues/6376
	home := os.Getenv("HOME")
	if home != "" {
		return home
	}
	ErrorLog(nil, "Couldn't find home directory or read HOME environment variable.")
	ErrorLog(nil, "Defaulting to storing config in current directory.")
	ErrorLog(nil, "Use -config flag to workaround.")
	ErrorLog(nil, "Error was: %v", err)
	return ""
}

// LoadConfig loads the config file
func LoadConfig() {
	// Read some flags if set
	//
	// FIXME read these from the config file too
	Config.Verbose = *verbose
	Config.Quiet = *quiet
	Config.ModifyWindow = *modifyWindow
	Config.Checkers = *checkers
	Config.Transfers = *transfers
	Config.DryRun = *dryRun
	Config.Timeout = *timeout
	Config.ConnectTimeout = *connectTimeout
	Config.CheckSum = *checkSum
	Config.SizeOnly = *sizeOnly
	Config.IgnoreTimes = *ignoreTimes
	Config.IgnoreExisting = *ignoreExisting
	Config.DumpHeaders = *dumpHeaders
	Config.DumpBodies = *dumpBodies
	Config.DumpAuth = *dumpAuth
	Config.InsecureSkipVerify = *skipVerify
	Config.LowLevelRetries = *lowLevelRetries
	Config.UpdateOlder = *updateOlder
	Config.NoGzip = *noGzip
	Config.MaxDepth = *maxDepth
	Config.IgnoreSize = *ignoreSize
	Config.NoTraverse = *noTraverse
	Config.NoUpdateModTime = *noUpdateModTime

	ConfigPath = *configFile

	Config.DeleteBefore = *deleteBefore
	Config.DeleteDuring = *deleteDuring
	Config.DeleteAfter = *deleteAfter

	Config.TrackRenames = *trackRenames

	switch {
	case *deleteBefore && (*deleteDuring || *deleteAfter),
		*deleteDuring && *deleteAfter:
		log.Fatalf(`Only one of --delete-before, --delete-during or --delete-after can be used.`)

	// If none are specified, use "during".
	case !*deleteBefore && !*deleteDuring && !*deleteAfter:
		Config.DeleteDuring = true
	}

	if Config.IgnoreSize && Config.SizeOnly {
		log.Fatalf(`Can't use --size-only and --ignore-size together.`)
	}

	// Load configuration file.
	var err error
	ConfigFile, err = loadConfigFile()
	if err == errorConfigFileNotFound {
		Log(nil, "Config file %q not found - using defaults", ConfigPath)
		ConfigFile, _ = goconfig.LoadFromReader(&bytes.Buffer{})
	} else if err != nil {
		log.Fatalf("Failed to load config file %q: %v", ConfigPath, err)
	}

	// Load filters
	Config.Filter, err = NewFilter()
	if err != nil {
		log.Fatalf("Failed to load filters: %v", err)
	}

	// Start the token bucket limiter
	startTokenBucket()
}

var errorConfigFileNotFound = errors.New("config file not found")

// loadConfigFile will load a config file, and
// automatically decrypt it.
func loadConfigFile() (*goconfig.ConfigFile, error) {
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
	envpw := os.Getenv("RCLONE_CONFIG_PASS")

	var out []byte
	for {
		if len(configKey) == 0 && envpw != "" {
			err := setConfigPassword(envpw)
			if err != nil {
				fmt.Println("Using RCLONE_CONFIG_PASS returned:", err)
				envpw = ""
			} else {
				Debug(nil, "Using RCLONE_CONFIG_PASS password.")
			}
		}
		if len(configKey) == 0 {
			if !*AskPassword {
				return nil, errors.New("unable to decrypt configuration and not allowed to ask for password - set RCLONE_CONFIG_PASS to your configuration password")
			}
			getConfigPassword("Enter configuration password:")
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
		ErrorLog(nil, "Couldn't decrypt configuration, most likely wrong password.")
		configKey = nil
		envpw = ""
	}
	return goconfig.LoadFromReader(bytes.NewBuffer(out))
}

// checkPassword normalises and validates the password
func checkPassword(password string) (string, error) {
	if !utf8.ValidString(password) {
		return "", errors.New("password contains invalid utf8 characters")
	}
	// Remove leading+trailing whitespace
	password = strings.TrimSpace(password)
	// Normalize to reduce weird variations.
	password = norm.NFKC.String(password)
	if len(password) == 0 {
		return "", errors.New("no characters in password")
	}
	return password, nil
}

// GetPassword asks the user for a password with the prompt given.
func GetPassword(prompt string) string {
	fmt.Println(prompt)
	for {
		fmt.Print("password:")
		password := ReadPassword()
		password, err := checkPassword(password)
		if err == nil {
			return password
		}
		fmt.Printf("Bad password: %v\n", err)
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
		fmt.Println("Error:", err)
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

// SaveConfig saves configuration file.
// if configKey has been set, the file will be encrypted.
func SaveConfig() {
	if len(configKey) == 0 {
		err := goconfig.SaveConfigFile(ConfigFile, ConfigPath)
		if err != nil {
			log.Fatalf("Failed to save config file: %v", err)
		}
		err = os.Chmod(ConfigPath, 0600)
		if err != nil {
			ErrorLog(nil, "Failed to set permissions on config file: %v", err)
		}
		return
	}
	var buf bytes.Buffer
	err := goconfig.SaveConfigData(ConfigFile, &buf)
	if err != nil {
		log.Fatalf("Failed to save config file: %v", err)
	}

	f, err := os.Create(ConfigPath)
	if err != nil {
		log.Fatalf("Failed to save config file: %v", err)
	}

	fmt.Fprintln(f, "# Encrypted rclone configuration File")
	fmt.Fprintln(f, "")
	fmt.Fprintln(f, "RCLONE_ENCRYPT_V0:")

	// Generate new nonce and write it to the start of the ciphertext
	var nonce [24]byte
	n, _ := rand.Read(nonce[:])
	if n != 24 {
		log.Fatalf("nonce short read: %d", n)
	}
	enc := base64.NewEncoder(base64.StdEncoding, f)
	_, err = enc.Write(nonce[:])
	if err != nil {
		log.Fatalf("Failed to write config file: %v", err)
	}

	var key [32]byte
	copy(key[:], configKey[:32])

	b := secretbox.Seal(nil, buf.Bytes(), &nonce, &key)
	_, err = enc.Write(b)
	if err != nil {
		log.Fatalf("Failed to write config file: %v", err)
	}
	_ = enc.Close()
	err = f.Close()
	if err != nil {
		log.Fatalf("Failed to close config file: %v", err)
	}

	err = os.Chmod(ConfigPath, 0600)
	if err != nil {
		ErrorLog(nil, "Failed to set permissions on config file: %v", err)
	}
}

// ConfigSetValueAndSave sets the key to the value and saves just that
// value in the config file.  It loads the old config file in from
// disk first and overwrites the given value only.
func ConfigSetValueAndSave(name, key, value string) (err error) {
	// Set the value in config in case we fail to reload it
	ConfigFile.SetValue(name, key, value)
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
	ConfigFile = reloadedConfigFile
	// Set the value in the reloaded version
	reloadedConfigFile.SetValue(name, key, value)
	// Save it again
	SaveConfig()
	return nil
}

// ShowRemotes shows an overview of the config file
func ShowRemotes() {
	remotes := ConfigFile.GetSectionList()
	if len(remotes) == 0 {
		return
	}
	sort.Strings(remotes)
	fmt.Printf("%-20s %s\n", "Name", "Type")
	fmt.Printf("%-20s %s\n", "====", "====")
	for _, remote := range remotes {
		fmt.Printf("%-20s %s\n", remote, ConfigFile.MustValue(remote, "type"))
	}
}

// ChooseRemote chooses a remote name
func ChooseRemote() string {
	remotes := ConfigFile.GetSectionList()
	sort.Strings(remotes)
	return Choose("remote", remotes, nil, false)
}

// ReadLine reads some input
func ReadLine() string {
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
func Confirm() bool {
	return Command([]string{"yYes", "nNo"}) == 'y'
}

// Choose one of the defaults or type a new string if newOk is set
func Choose(what string, defaults, help []string, newOk bool) string {
	fmt.Printf("Choose a number from below")
	if newOk {
		fmt.Printf(", or type in your own value")
	}
	fmt.Println()
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
	for _, key := range ConfigFile.GetKeyList(name) {
		isPassword := false
		for _, option := range fs.Options {
			if option.Name == key && option.IsPassword {
				isPassword = true
				break
			}
		}
		value := ConfigFile.MustValue(name, key)
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
		ConfigFile.DeleteSection(name)
		return true
	default:
		ErrorLog(nil, "Bad choice %c", i)
	}
	return false
}

// MustFindByName finds the RegInfo for the remote name passed in or
// exits with a fatal error.
func MustFindByName(name string) *RegInfo {
	fsType := ConfigFile.MustValue(name, "type")
	if fsType == "" {
		log.Fatalf("Couldn't find type of fs for %q", name)
	}
	return MustFind(fsType)
}

// RemoteConfig runs the config helper for the remote if needed
func RemoteConfig(name string) {
	fmt.Printf("Remote config\n")
	f := MustFindByName(name)
	if f.Config != nil {
		f.Config(name)
	}
}

// ChooseOption asks the user to choose an option
func ChooseOption(o *Option) string {
	fmt.Println(o.Help)
	if o.IsPassword {
		actions := []string{"yYes type in my own password", "gGenerate random password"}
		if o.Optional {
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
				bytes := bits / 8
				if bits%8 != 0 {
					bytes++
				}
				var pw = make([]byte, bytes)
				n, _ := rand.Read(pw)
				if n != bytes {
					log.Fatalf("password short read: %d", n)
				}
				password = base64.RawURLEncoding.EncodeToString(pw)
				fmt.Printf("Your password is: %s\n", password)
				fmt.Printf("Use this password?\n")
				if Confirm() {
					break
				}
			}
		case 'n':
			return ""
		default:
			ErrorLog(nil, "Bad choice %c", i)
		}
		return MustObscure(password)
	}
	if len(o.Examples) > 0 {
		var values []string
		var help []string
		for _, example := range o.Examples {
			values = append(values, example.Value)
			help = append(help, example.Help)
		}
		return Choose(o.Name, values, help, true)
	}
	fmt.Printf("%s> ", o.Name)
	return ReadLine()
}

// fsOption returns an Option describing the possible remotes
func fsOption() *Option {
	o := &Option{
		Name: "Storage",
		Help: "Type of storage to configure.",
	}
	for _, item := range fsRegistry {
		example := OptionExample{
			Value: item.Name,
			Help:  item.Description,
		}
		o.Examples = append(o.Examples, example)
	}
	o.Examples.Sort()
	return o
}

// NewRemote make a new remote from its name
func NewRemote(name string) {
	newType := ChooseOption(fsOption())
	ConfigFile.SetValue(name, "type", newType)
	fs := MustFind(newType)
	for _, option := range fs.Options {
		ConfigFile.SetValue(name, option.Name, ChooseOption(&option))
	}
	RemoteConfig(name)
	if OkRemote(name) {
		SaveConfig()
		return
	}
	EditRemote(fs, name)
}

// EditRemote gets the user to edit a remote
func EditRemote(fs *RegInfo, name string) {
	ShowRemote(name)
	fmt.Printf("Edit remote\n")
	for {
		for _, option := range fs.Options {
			key := option.Name
			value := ConfigFile.MustValue(name, key)
			fmt.Printf("Value %q = %q\n", key, value)
			fmt.Printf("Edit? (y/n)>\n")
			if Confirm() {
				newValue := ChooseOption(&option)
				ConfigFile.SetValue(name, key, newValue)
			}
		}
		RemoteConfig(name)
		if OkRemote(name) {
			break
		}
	}
	SaveConfig()
}

// DeleteRemote gets the user to delete a remote
func DeleteRemote(name string) {
	ConfigFile.DeleteSection(name)
	SaveConfig()
}

// EditConfig edits the config file interactively
func EditConfig() {
	for {
		haveRemotes := len(ConfigFile.GetSectionList()) != 0
		what := []string{"eEdit existing remote", "nNew remote", "dDelete remote", "sSet configuration password", "qQuit config"}
		if haveRemotes {
			fmt.Printf("Current remotes:\n\n")
			ShowRemotes()
			fmt.Printf("\n")
		} else {
			fmt.Printf("No remotes found - make a new one\n")
			what = append(what[1:2], what[3:]...)
		}
		switch i := Command(what); i {
		case 'e':
			name := ChooseRemote()
			fs := MustFindByName(name)
			EditRemote(fs, name)
		case 'n':
		nameLoop:
			for {
				fmt.Printf("name> ")
				name := ReadLine()
				parts := matcher.FindStringSubmatch(name + ":")
				switch {
				case name == "":
					fmt.Printf("Can't use empty name\n")
				case isDriveLetter(name):
					fmt.Printf("Can't use %q as it can be confused a drive letter\n", name)
				case len(parts) != 3 || parts[2] != "":
					fmt.Printf("Can't use %q as it has invalid characters in it %v\n", name, parts)
				default:
					NewRemote(name)
					break nameLoop
				}
			}
		case 'd':
			name := ChooseRemote()
			DeleteRemote(name)
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
	switch len(args) {
	case 1, 3:
	default:
		log.Fatalf("Invalid number of arguments: %d", len(args))
	}
	newType := args[0]
	fs := MustFind(newType)
	if fs.Config == nil {
		log.Fatalf("Can't authorize fs %q", newType)
	}
	// Name used for temporary fs
	name := "**temp-fs**"

	// Make sure we delete it
	defer DeleteRemote(name)

	// Indicate that we want fully automatic configuration.
	ConfigFile.SetValue(name, ConfigAutomatic, "yes")
	if len(args) == 3 {
		ConfigFile.SetValue(name, ConfigClientID, args[1])
		ConfigFile.SetValue(name, ConfigClientSecret, args[2])
	}
	fs.Config(name)
}
