// Read and write the config file
package fs

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/user"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Unknwon/goconfig"
)

const (
	configFileName = ".rclone.conf"
)

// Global
var (
	// Config file
	ConfigFile *goconfig.ConfigFile
	// Config file path
	ConfigPath string
	// Global config
	Config = &ConfigInfo{}
	// Home directory
	HomeDir string
	// Flags
	verbose      = flag.Bool("verbose", false, "Print lots more stuff")
	quiet        = flag.Bool("quiet", false, "Print as little stuff as possible")
	modifyWindow = flag.Duration("modify-window", time.Nanosecond, "Max time diff to be considered the same")
	checkers     = flag.Int("checkers", 8, "Number of checkers to run in parallel.")
	transfers    = flag.Int("transfers", 4, "Number of file transfers to run in parallel.")
)

// Filesystem config options
type ConfigInfo struct {
	Verbose      bool
	Quiet        bool
	ModifyWindow time.Duration
	Checkers     int
	Transfers    int
}

// Loads the config file
func LoadConfig() {
	// Read some flags if set
	//
	// FIXME read these from the config file too
	Config.Verbose = *verbose
	Config.Quiet = *quiet
	Config.ModifyWindow = *modifyWindow
	Config.Checkers = *checkers
	Config.Transfers = *transfers

	// Find users home directory
	usr, err := user.Current()
	if err != nil {
		log.Printf("Couldn't find home directory: %v", err)
		return
	}
	HomeDir = usr.HomeDir
	ConfigPath = path.Join(HomeDir, configFileName)

	// Load configuration file.
	ConfigFile, err = goconfig.LoadConfigFile(ConfigPath)
	if err != nil {
		log.Printf("Failed to load config file %v - using defaults", ConfigPath)
	}
}

// Save configuration file.
func SaveConfig() {
	err := goconfig.SaveConfigFile(ConfigFile, ConfigPath)
	if err != nil {
		log.Fatalf("Failed to save config file: %v", err)
	}
}

// Show an overview of the config file
func ShowConfig() {
	remotes := ConfigFile.GetSectionList()
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

// Read some input
func ReadLine() string {
	buf := bufio.NewReader(os.Stdin)
	line, err := buf.ReadString('\n')
	if err != nil {
		log.Fatalf("Failed to read line: %v", err)
	}
	return strings.TrimSpace(line)
}

// Command - choose one
func Command(commands []string) int {
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
		i := strings.IndexByte(optString, result[0])
		if i >= 0 {
			return i
		}
	}
}

// Choose one of the defaults or type a new string if newOk is set
func Choose(what string, defaults, help []string, newOk bool) string {
	fmt.Printf("Choose a number from below")
	if newOk {
		fmt.Printf(", or type in your own value")
	}
	fmt.Println()
	for i, text := range defaults {
		if help != nil {
			parts := strings.Split(help[i], "\n")
			for _, part := range parts {
				fmt.Printf(" * %s\n", part)
			}
		}
		fmt.Printf("%2d) %s\n", i+1, text)
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

// Show the contents of the remote
func ShowRemote(name string) {
	fmt.Printf("--------------------\n")
	fmt.Printf("[%s]\n", name)
	for _, key := range ConfigFile.GetKeyList(name) {
		fmt.Printf("%s = %s\n", key, ConfigFile.MustValue(name, key))
	}
	fmt.Printf("--------------------\n")
}

// Print the contents of the remote and ask if it is OK
func OkRemote(name string) bool {
	ShowRemote(name)
	switch i := Command([]string{"yYes this is OK", "eEdit this remote", "dDelete this remote"}); i {
	case 0:
		return true
	case 1:
		return false
	case 2:
		ConfigFile.DeleteSection(name)
		return true
	default:
		log.Printf("Bad choice %d", i)
	}
	return false
}

// Make a new remote
func NewRemote(name string) {
	fmt.Printf("What type of source is it?\n")
	types := []string{}
	for _, item := range fsRegistry {
		types = append(types, item.Name)
	}
	newType := Choose("type", types, nil, false)
	ConfigFile.SetValue(name, "type", newType)
	fs, err := Find(newType)
	if err != nil {
		log.Fatalf("Failed to find fs: %v", err)
	}
	for _, option := range fs.Options {
		ConfigFile.SetValue(name, option.Name, option.Choose())
	}
	if OkRemote(name) {
		SaveConfig()
		return
	}
	EditRemote(name)
}

// Edit a remote
func EditRemote(name string) {
	ShowRemote(name)
	fmt.Printf("Edit remote\n")
	for {
		for _, key := range ConfigFile.GetKeyList(name) {
			value := ConfigFile.MustValue(name, key)
			fmt.Printf("Press enter to accept current value, or type in a new one\n")
			fmt.Printf("%s = %s>", key, value)
			newValue := ReadLine()
			if newValue != "" {
				ConfigFile.SetValue(name, key, newValue)
			}
		}
		if OkRemote(name) {
			break
		}
	}
	SaveConfig()
}

// Edit the config file interactively
func EditConfig() {
	for {
		fmt.Printf("Current remotes:\n\n")
		ShowConfig()
		fmt.Printf("\n")
		switch i := Command([]string{"eEdit existing remote", "nNew remote", "dDelete remote", "qQuit config"}); i {
		case 0:
			name := ChooseRemote()
			EditRemote(name)
		case 1:
			fmt.Printf("name> ")
			name := ReadLine()
			NewRemote(name)
		case 2:
			name := ChooseRemote()
			ConfigFile.DeleteSection(name)
		case 3:
			return
		}
	}
}
