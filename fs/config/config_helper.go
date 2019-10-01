package config

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/driveletter"
	"github.com/rclone/rclone/fs/fspath"
	"golang.org/x/text/unicode/norm"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

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
			subProvider := GetRemoteConfig().GetRemote(name).GetString(fs.ConfigProvider)
			if matchProvider(option.Provider, subProvider) && isVisible {
				if !isNew {
					fmt.Printf("Value %q = %q\n", option.Name, GetRemoteConfig().GetRemote(name).GetString(option.Name))
					fmt.Printf("Edit? (y/n)>\n")
					if !Confirm() {
						continue
					}
				}
				result := ChooseOption(&option, name)
				if option.Default != result {
					GetRemoteConfig().GetRemote(name).SetString(option.Name, result)
				}
			}
		}
	}
}

// NewRemoteName asks the user for a name for a remote
func NewRemoteName() (name string) {
	for {
		fmt.Printf("name> ")
		name = ReadLine()
		err := fspath.CheckConfigName(name)
		switch {
		case name == "":
			fmt.Printf("Can't use empty name.\n")
		case driveletter.IsDriveLetter(name):
			fmt.Printf("Can't use %q as it can be confused with a drive letter.\n", name)
		case err != nil:
			fmt.Printf("Can't use %q as %v.\n", name, err)
		default:
			return name
		}
	}
}

// RenameRemote renames a config section
func RenameRemote(name string) {
	fmt.Printf("Enter new name for %q remote.\n", name)
	newName := NewRemoteName()
	if name != newName {
		GetRemoteConfig().RenameRemote(name, newName)
	}
}

// CopyRemote copies a config section
func CopyRemote(name string) {
	fmt.Printf("Enter name for copy of %q remote.\n", name)
	GetRemoteConfig().CopyRemote(name, NewRemoteName())
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
	str := GetProvider().String()
	if str == "" {
		str = "; empty config\n"
	}
	fmt.Printf("%s", str)
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
	RunRemoteConfig(name)
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
	if GetRemoteConfig().HasRemote(name) {
		GetRemoteConfig().GetRemote(name).SetString("type", newType)
	} else {
		GetRemoteConfig().CreateRemote(name).SetString("type", newType)
	}

	editOptions(ri, name, true)
	RunRemoteConfig(name)
	if OkRemote(name) {
		return
	}
	EditRemote(ri, name)
}

// EditConfig edits the config file interactively
func EditConfig() {
	for {
		haveRemotes := len(GetRemoteConfig().GetRemotes()) != 0
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
			fsEntry := MustFindByName(name)
			EditRemote(fsEntry, name)
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
				fmt.Println("Password changed")
				continue
			case 'u':
				configKey = nil
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
				fmt.Println("Password set")
				continue
			case 'q':
				return
			}
		}
	}
}

// ReadNonEmptyLine prints prompt and calls Readline until non empty
func ReadNonEmptyLine(prompt string) string {
	result := ""
	for result == "" {
		fmt.Print(prompt)
		result = strings.TrimSpace(ReadLine())
	}
	return result
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
	GetRemoteConfig().GetRemote(name).SetString(ConfigAuthorize, "true")
	if len(args) == 3 {
		GetRemoteConfig().GetRemote(name).SetString(ConfigClientID, args[1])
		GetRemoteConfig().GetRemote(name).SetString(ConfigClientSecret, args[2])
	}
	m := fs.ConfigMap(f, name)
	f.Config(name, m)
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

// ShowRemotes shows an overview of the config file
func ShowRemotes() {
	remotes := GetRemoteConfig().GetRemotes()
	if len(remotes) == 0 {
		return
	}
	sort.Strings(remotes)
	fmt.Printf("%-20s %s\n", "Name", "Type")
	fmt.Printf("%-20s %s\n", "====", "====")
	for _, remote := range remotes {
		fmt.Printf("%-20s %s\n", remote, GetRemoteConfig().GetRemote(remote).GetString("type"))
	}
}

// ChooseRemote chooses a remote name
func ChooseRemote() string {
	remotes := GetRemoteConfig().GetRemotes()
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
	fsEntry := MustFindByName(name)
	for key, value := range GetRemoteConfig().GetRemote(name).GetConfig() {
		isPassword := false
		for _, option := range fsEntry.Options {
			if option.Name == key && option.IsPassword {
				isPassword = true
				break
			}
		}
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
		GetRemoteConfig().DeleteRemote(name)
		return true
	default:
		fs.Errorf(nil, "Bad choice %c", i)
	}
	return false
}

// MustFindByName finds the RegInfo for the remote name passed in or
// exits with a fatal error.
func MustFindByName(name string) *fs.RegInfo {
	fsType := GetRemoteConfig().GetRemote(name).GetString("type")
	if fsType == "" {
		log.Fatalf("Couldn't find type of fs for %q", name)
	}
	return fs.MustFind(fsType)
}

// RunRemoteConfig runs the config helper for the remote if needed
func RunRemoteConfig(name string) {
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
	var subProvider string
	if GetRemoteConfig().HasRemote(name) {
		subProvider = GetRemoteConfig().GetRemote(name).GetString(fs.ConfigProvider)
	}
	fmt.Println(o.Help)
	if o.IsPassword {
		actions := []string{"yYes type in my own password", "gGenerate random password"}
		if !o.Required {
			actions = append(actions, "nNo leave this optional password blank")
		}
		var password string
		var err error
		switch i := Command(actions); i {
		case 'y':
			password = ChangePassword("the")
		case 'g':
			for {
				fmt.Printf("Password strength in bits.\n64 is just about memorable\n128 is secure\n1024 is the maximum\n")
				bits := ChooseNumber("Bits", 64, 1024)
				password, err = Password(bits)
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


// makeCacheDir returns a directory to use for caching.
//
// Code borrowed from go stdlib until it is made public
func makeCacheDir(configFileFolder string) (dir string) {
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
	return filepath.Join(dir, configFileFolder)
}