// Sync files and directories to and from local and remote object stores
//
// Nick Craig-Wood <nick@craig-wood.com>
package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/ogier/pflag"

	"github.com/ncw/rclone/fs"
	// Active file systems
	_ "github.com/ncw/rclone/drive"
	_ "github.com/ncw/rclone/googlecloudstorage"
	_ "github.com/ncw/rclone/local"
	_ "github.com/ncw/rclone/s3"
	_ "github.com/ncw/rclone/swift"
)

// Globals
var (
	// Flags
	cpuprofile    = pflag.StringP("cpuprofile", "", "", "Write cpu profile to file")
	statsInterval = pflag.DurationP("stats", "", time.Minute*1, "Interval to print stats")
	version       = pflag.BoolP("version", "V", false, "Print the version number")
)

type Command struct {
	Name     string
	Help     string
	ArgsHelp string
	Run      func(fdst, fsrc fs.Fs)
	MinArgs  int
	MaxArgs  int
	NoStats  bool
}

// checkArgs checks there are enough arguments and prints a message if not
func (cmd *Command) checkArgs(args []string) {
	if len(args) < cmd.MinArgs {
		syntaxError()
		fmt.Fprintf(os.Stderr, "Command %s needs %d arguments mininum\n", cmd.Name, cmd.MinArgs)
		os.Exit(1)
	} else if len(args) > cmd.MaxArgs {
		syntaxError()
		fmt.Fprintf(os.Stderr, "Command %s needs %d arguments maximum\n", cmd.Name, cmd.MaxArgs)
		os.Exit(1)
	}
}

var Commands = []Command{
	{
		Name:     "copy",
		ArgsHelp: "source://path dest://path",
		Help: `
        Copy the source to the destination.  Doesn't transfer
        unchanged files, testing first by modification time then by
        MD5SUM.  Doesn't delete files from the destination.`,
		Run: func(fdst, fsrc fs.Fs) {
			err := fs.Sync(fdst, fsrc, false)
			if err != nil {
				log.Fatalf("Failed to copy: %v", err)
			}
		},
		MinArgs: 2,
		MaxArgs: 2,
	},
	{
		Name:     "sync",
		ArgsHelp: "source://path dest://path",
		Help: `
        Sync the source to the destination.  Doesn't transfer
        unchanged files, testing first by modification time then by
        MD5SUM.  Deletes any files that exist in source that don't
        exist in destination. Since this can cause data loss, test
        first with the --dry-run flag.`,
		Run: func(fdst, fsrc fs.Fs) {
			err := fs.Sync(fdst, fsrc, true)
			if err != nil {
				log.Fatalf("Failed to sync: %v", err)
			}
		},
		MinArgs: 2,
		MaxArgs: 2,
	},
	{
		Name:     "ls",
		ArgsHelp: "[remote://path]",
		Help: `
        List all the objects in the the path with size and path.`,
		Run: func(fdst, fsrc fs.Fs) {
			err := fs.List(fdst)
			if err != nil {
				log.Fatalf("Failed to list: %v", err)
			}
		},
		MinArgs: 1,
		MaxArgs: 1,
	},
	{
		Name:     "lsd",
		ArgsHelp: "[remote://path]",
		Help: `
        List all directories/containers/buckets in the the path.`,
		Run: func(fdst, fsrc fs.Fs) {
			err := fs.ListDir(fdst)
			if err != nil {
				log.Fatalf("Failed to listdir: %v", err)
			}
		},
		MinArgs: 1,
		MaxArgs: 1,
	},
	{
		Name:     "lsl",
		ArgsHelp: "[remote://path]",
		Help: `
        List all the objects in the the path with modification time, size and path.`,
		Run: func(fdst, fsrc fs.Fs) {
			err := fs.ListLong(fdst)
			if err != nil {
				log.Fatalf("Failed to list long: %v", err)
			}
		},
		MinArgs: 1,
		MaxArgs: 1,
	},
	{
		Name:     "md5sum",
		ArgsHelp: "[remote://path]",
		Help: `
        Produces an md5sum file for all the objects in the path.`,
		Run: func(fdst, fsrc fs.Fs) {
			err := fs.Md5sum(fdst)
			if err != nil {
				log.Fatalf("Failed to list: %v", err)
			}
		},
		MinArgs: 1,
		MaxArgs: 1,
	},
	{
		Name:     "mkdir",
		ArgsHelp: "remote://path",
		Help: `
        Make the path if it doesn't already exist`,
		Run: func(fdst, fsrc fs.Fs) {
			err := fs.Mkdir(fdst)
			if err != nil {
				log.Fatalf("Failed to mkdir: %v", err)
			}
		},
		MinArgs: 1,
		MaxArgs: 1,
	},
	{
		Name:     "rmdir",
		ArgsHelp: "remote://path",
		Help: `
        Remove the path.  Note that you can't remove a path with
        objects in it, use purge for that.`,
		Run: func(fdst, fsrc fs.Fs) {
			err := fs.Rmdir(fdst)
			if err != nil {
				log.Fatalf("Failed to rmdir: %v", err)
			}
		},
		MinArgs: 1,
		MaxArgs: 1,
	},
	{
		Name:     "purge",
		ArgsHelp: "remote://path",
		Help: `
        Remove the path and all of its contents.`,
		Run: func(fdst, fsrc fs.Fs) {
			err := fs.Purge(fdst)
			if err != nil {
				log.Fatalf("Failed to purge: %v", err)
			}
		},
		MinArgs: 1,
		MaxArgs: 1,
	},
	{
		Name:     "check",
		ArgsHelp: "source://path dest://path",
		Help: `
        Checks the files in the source and destination match.  It
        compares sizes and MD5SUMs and prints a report of files which
        don't match.  It doesn't alter the source or destination.`,
		Run: func(fdst, fsrc fs.Fs) {
			err := fs.Check(fdst, fsrc)
			if err != nil {
				log.Fatalf("Failed to check: %v", err)
			}
		},
		MinArgs: 2,
		MaxArgs: 2,
	},
	{
		Name: "config",
		Help: `
        Enter an interactive configuration session.`,
		Run: func(fdst, fsrc fs.Fs) {
			fs.EditConfig()
		},
		NoStats: true,
	},
	{
		Name: "help",
		Help: `
        This help.`,
		NoStats: true,
	},
}

// syntaxError prints the syntax
func syntaxError() {
	fmt.Fprintf(os.Stderr, `Sync files and directories to and from local and remote object stores - %s.

Syntax: [options] subcommand <parameters> <parameters...>

Subcommands:

`, fs.Version)
	for i := range Commands {
		cmd := &Commands[i]
		fmt.Fprintf(os.Stderr, "    %s %s\n", cmd.Name, cmd.ArgsHelp)
		fmt.Fprintf(os.Stderr, "%s\n\n", cmd.Help)
	}

	fmt.Fprintf(os.Stderr, "Options:\n")
	pflag.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
It is only necessary to use a unique prefix of the subcommand, eg 'up' for 'upload'.
`)
}

// Exit with the message
func fatal(message string, args ...interface{}) {
	syntaxError()
	fmt.Fprintf(os.Stderr, message, args...)
	os.Exit(1)
}

// Parse the command line flags
func ParseFlags() {
	pflag.Usage = syntaxError
	pflag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())
	fs.LoadConfig()

	// Setup profiling if desired
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			fs.Stats.Error()
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
}

// Parse the command from the command line
func ParseCommand() (*Command, []string) {
	args := pflag.Args()
	if len(args) < 1 {
		fatal("No command supplied\n")
	}

	cmd := strings.ToLower(args[0])
	args = args[1:]

	// Find the command doing a prefix match
	var command *Command
	for i := range Commands {
		trialCommand := &Commands[i]
		// exact command name found - use that
		if trialCommand.Name == cmd {
			command = trialCommand
			break
		} else if strings.HasPrefix(trialCommand.Name, cmd) {
			if command != nil {
				fs.Stats.Error()
				log.Fatalf("Not unique - matches multiple commands %q", cmd)
			}
			command = trialCommand
		}
	}
	if command == nil {
		fs.Stats.Error()
		log.Fatalf("Unknown command %q", cmd)
	}
	if command.Run == nil {
		syntaxError()
	}
	command.checkArgs(args)
	return command, args
}

// Create a Fs from a name
func NewFs(remote string) fs.Fs {
	f, err := fs.NewFs(remote)
	if err != nil {
		fs.Stats.Error()
		log.Fatalf("Failed to create file system for %q: %v", remote, err)
	}
	return f
}

// Print the stats every statsInterval
func StartStats() {
	go func() {
		ch := time.Tick(*statsInterval)
		for {
			<-ch
			fs.Stats.Log()
		}
	}()
}

func main() {
	ParseFlags()
	if *version {
		fmt.Printf("rclone %s\n", fs.Version)
		os.Exit(0)
	}
	command, args := ParseCommand()

	// Make source and destination fs
	var fdst, fsrc fs.Fs
	if len(args) >= 1 {
		fdst = NewFs(args[0])
	}
	if len(args) >= 2 {
		fsrc = fdst
		fdst = NewFs(args[1])
	}

	fs.CalculateModifyWindow(fdst, fsrc)

	if !command.NoStats {
		StartStats()
	}

	// Run the actual command
	if command.Run != nil {
		command.Run(fdst, fsrc)
		if !command.NoStats {
			fmt.Println(fs.Stats)
		}
		if fs.Config.Verbose {
			log.Printf("*** Go routines at exit %d\n", runtime.NumGoroutine())
		}
		if fs.Stats.Errored() {
			os.Exit(1)
		}
		os.Exit(0)
	} else {
	}

}
