// Sync files and directories to and from local and remote object stores
//
// Nick Craig-Wood <nick@craig-wood.com>
package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/spf13/pflag"

	"github.com/ncw/rclone/fs"
	_ "github.com/ncw/rclone/fs/all" // import all fs
)

// Globals
var (
	// Flags
	cpuProfile    = pflag.StringP("cpuprofile", "", "", "Write cpu profile to file")
	memProfile    = pflag.String("memprofile", "", "Write memory profile to file")
	statsInterval = pflag.DurationP("stats", "", time.Minute*1, "Interval to print stats (0 to disable)")
	version       = pflag.BoolP("version", "V", false, "Print the version number")
	logFile       = pflag.StringP("log-file", "", "", "Log everything to this file")
	retries       = pflag.IntP("retries", "", 3, "Retry operations this many times if they fail")
)

// Command holds info about the current running command
type Command struct {
	Name     string
	Help     string
	ArgsHelp string
	Run      func(fdst, fsrc fs.Fs) error
	MinArgs  int
	MaxArgs  int
	NoStats  bool
	Retry    bool
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

// Commands is a slice of possible Command~s
var Commands = []Command{
	{
		Name:     "copy",
		ArgsHelp: "source:path dest:path",
		Help: `
        Copy the source to the destination.  Doesn't transfer
        unchanged files, testing by size and modification time or
        MD5SUM.  Doesn't delete files from the destination.`,
		Run: func(fdst, fsrc fs.Fs) error {
			return fs.CopyDir(fdst, fsrc)
		},
		MinArgs: 2,
		MaxArgs: 2,
		Retry:   true,
	},
	{
		Name:     "sync",
		ArgsHelp: "source:path dest:path",
		Help: `
        Sync the source to the destination, changing the destination
        only.  Doesn't transfer unchanged files, testing by size and
        modification time or MD5SUM.  Destination is updated to match
        source, including deleting files if necessary.  Since this can
        cause data loss, test first with the --dry-run flag.`,
		Run: func(fdst, fsrc fs.Fs) error {
			return fs.Sync(fdst, fsrc)
		},
		MinArgs: 2,
		MaxArgs: 2,
		Retry:   true,
	},
	{
		Name:     "move",
		ArgsHelp: "source:path dest:path",
		Help: `
        Moves the source to the destination.  This is equivalent to a
        copy followed by a purge, but may use server side operations
        to speed it up. Since this can cause data loss, test first
        with the --dry-run flag.`,
		Run: func(fdst, fsrc fs.Fs) error {
			return fs.MoveDir(fdst, fsrc)
		},
		MinArgs: 2,
		MaxArgs: 2,
		Retry:   true,
	},
	{
		Name:     "ls",
		ArgsHelp: "remote:path",
		Help: `
        List all the objects in the the path with size and path.`,
		Run: func(fdst, fsrc fs.Fs) error {
			return fs.List(fdst, os.Stdout)
		},
		MinArgs: 1,
		MaxArgs: 1,
	},
	{
		Name:     "lsd",
		ArgsHelp: "remote:path",
		Help: `
        List all directories/containers/buckets in the the path.`,
		Run: func(fdst, fsrc fs.Fs) error {
			return fs.ListDir(fdst, os.Stdout)
		},
		MinArgs: 1,
		MaxArgs: 1,
	},
	{
		Name:     "lsl",
		ArgsHelp: "remote:path",
		Help: `
        List all the objects in the the path with modification time,
        size and path.`,
		Run: func(fdst, fsrc fs.Fs) error {
			return fs.ListLong(fdst, os.Stdout)
		},
		MinArgs: 1,
		MaxArgs: 1,
	},
	{
		Name:     "md5sum",
		ArgsHelp: "remote:path",
		Help: `
        Produces an md5sum file for all the objects in the path.  This
        is in the same format as the standard md5sum tool produces.`,
		Run: func(fdst, fsrc fs.Fs) error {
			return fs.Md5sum(fdst, os.Stdout)
		},
		MinArgs: 1,
		MaxArgs: 1,
	},
	{
		Name:     "sha1sum",
		ArgsHelp: "remote:path",
		Help: `
        Produces an sha1sum file for all the objects in the path.  This
        is in the same format as the standard sha1sum tool produces.`,
		Run: func(fdst, fsrc fs.Fs) error {
			return fs.Sha1sum(fdst, os.Stdout)
		},
		MinArgs: 1,
		MaxArgs: 1,
	},
	{
		Name:     "size",
		ArgsHelp: "remote:path",
		Help: `
        Returns the total size of objects in remote:path and the number
        of objects.`,
		Run: func(fdst, fsrc fs.Fs) error {
			objects, size, err := fs.Count(fdst)
			if err != nil {
				return err
			}
			fmt.Printf("Total objects: %d\n", objects)
			fmt.Printf("Total size: %v (%d bytes)\n", fs.SizeSuffix(size), size)
			return nil
		},
		MinArgs: 1,
		MaxArgs: 1,
	},
	{
		Name:     "mkdir",
		ArgsHelp: "remote:path",
		Help: `
        Make the path if it doesn't already exist`,
		Run: func(fdst, fsrc fs.Fs) error {
			return fs.Mkdir(fdst)
		},
		MinArgs: 1,
		MaxArgs: 1,
		Retry:   true,
	},
	{
		Name:     "rmdir",
		ArgsHelp: "remote:path",
		Help: `
        Remove the path.  Note that you can't remove a path with
        objects in it, use purge for that.`,
		Run: func(fdst, fsrc fs.Fs) error {
			return fs.Rmdir(fdst)
		},
		MinArgs: 1,
		MaxArgs: 1,
		Retry:   true,
	},
	{
		Name:     "purge",
		ArgsHelp: "remote:path",
		Help: `
        Remove the path and all of its contents.  Does not obey
        filters - use remove for that.`,
		Run: func(fdst, fsrc fs.Fs) error {
			return fs.Purge(fdst)
		},
		MinArgs: 1,
		MaxArgs: 1,
		Retry:   true,
	},
	{
		Name:     "delete",
		ArgsHelp: "remote:path",
		Help: `
        Remove the contents of path.  Obeys include/exclude filters.`,
		Run: func(fdst, fsrc fs.Fs) error {
			return fs.Delete(fdst)
		},
		MinArgs: 1,
		MaxArgs: 1,
		Retry:   true,
	},
	{
		Name:     "check",
		ArgsHelp: "source:path dest:path",
		Help: `
        Checks the files in the source and destination match.  It
        compares sizes and MD5SUMs and prints a report of files which
        don't match.  It doesn't alter the source or destination.`,
		Run: func(fdst, fsrc fs.Fs) error {
			return fs.Check(fdst, fsrc)
		},
		MinArgs: 2,
		MaxArgs: 2,
	},
	{
		Name:     "dedupe",
		ArgsHelp: "remote:path",
		Help: `
        Interactively find duplicate files and offer to delete all
        but one or rename them to be different. Only useful with
        Google Drive which can have duplicate file names.`,
		Run: func(fdst, fsrc fs.Fs) error {
			return fs.Deduplicate(fdst, fs.Config.DedupeMode)
		},
		MinArgs: 1,
		MaxArgs: 1,
	},
	{
		Name: "config",
		Help: `
        Enter an interactive configuration session.`,
		Run: func(fdst, fsrc fs.Fs) error {
			fs.EditConfig()
			return nil
		},
		NoStats: true,
	},
	{
		Name: "authorize",
		Help: `
        Remote authorization. Used to authorize a remote or headless
        rclone from a machine with a browser - use as instructed by
        rclone config.`,
		Run: func(fdst, fsrc fs.Fs) error {
			fs.Authorize(pflag.Args()[1:])
			return nil
		},
		NoStats: true,
		MinArgs: 1,
		MaxArgs: 3,
	},
	{
		Name:     "cleanup",
		ArgsHelp: "remote:path",
		Help: `
        Clean up the remote if possible.  Empty the trash or delete
        old file versions. Not supported by all remotes.`,
		Run: func(fdst, fsrc fs.Fs) error {
			return fs.CleanUp(fdst)
		},
		MinArgs: 1,
		MaxArgs: 1,
		Retry:   true,
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
It is only necessary to use a unique prefix of the subcommand, eg 'mo'
for 'move'.
`)
}

// Exit with the message
func fatal(message string, args ...interface{}) {
	syntaxError()
	fmt.Fprintf(os.Stderr, message, args...)
	os.Exit(1)
}

// ParseFlags parses the command line flags
func ParseFlags() {
	pflag.Usage = syntaxError
	pflag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())
}

// ParseCommand parses the command from the command line
func ParseCommand() (*Command, []string) {
	args := pflag.Args()
	if len(args) < 1 {
		fatal("No command supplied\n")
	}

	cmd := strings.ToLower(args[0])
	args = args[1:]

	// Find the command doing a prefix match
	var found = make([]*Command, 0, 1)
	var command *Command
	for i := range Commands {
		trialCommand := &Commands[i]
		// exact command name found - use that
		if trialCommand.Name == cmd {
			command = trialCommand
			break
		} else if strings.HasPrefix(trialCommand.Name, cmd) {
			found = append(found, trialCommand)
		}
	}
	if command == nil {
		switch len(found) {
		case 0:
			fs.Stats.Error()
			log.Fatalf("Unknown command %q", cmd)
		case 1:
			command = found[0]
		default:
			fs.Stats.Error()
			var names []string
			for _, cmd := range found {
				names = append(names, `"`+cmd.Name+`"`)
			}
			log.Fatalf("Not unique - matches multiple commands: %s", strings.Join(names, ", "))
		}
	}
	if command.Run == nil {
		syntaxError()
	}
	command.checkArgs(args)
	return command, args
}

// NewFsSrc creates a src Fs from a name
func NewFsSrc(remote string) fs.Fs {
	fsInfo, configName, fsPath, err := fs.ParseRemote(remote)
	if err != nil {
		fs.Stats.Error()
		log.Fatalf("Failed to create file system for %q: %v", remote, err)
	}
	f, err := fsInfo.NewFs(configName, fsPath)
	if err == fs.ErrorIsFile {
		if !fs.Config.Filter.InActive() {
			fs.Stats.Error()
			log.Fatalf("Can't limit to single files when using filters: %v", remote)
		}
		// Limit transfers to this file
		err = fs.Config.Filter.AddFile(path.Base(fsPath))
		// Set --no-traverse as only one file
		fs.Config.NoTraverse = true
	}
	if err != nil {
		fs.Stats.Error()
		log.Fatalf("Failed to create file system for %q: %v", remote, err)
	}
	return f
}

// NewFs creates a dst Fs from a name
func NewFs(remote string) fs.Fs {
	f, err := fs.NewFs(remote)
	if err != nil {
		fs.Stats.Error()
		log.Fatalf("Failed to create file system for %q: %v", remote, err)
	}
	return f
}

// StartStats prints the stats every statsInterval
func StartStats() {
	if *statsInterval <= 0 {
		return
	}
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

	// Log file output
	if *logFile != "" {
		f, err := os.OpenFile(*logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
		if err != nil {
			log.Fatalf("Failed to open log file: %v", err)
		}
		_, err = f.Seek(0, os.SEEK_END)
		if err != nil {
			fs.ErrorLog(nil, "Failed to seek log file to end: %v", err)
		}
		log.SetOutput(f)
		fs.DebugLogger.SetOutput(f)
		redirectStderr(f)
	}

	// Load the rest of the config now we have started the logger
	fs.LoadConfig()

	// Write the args for debug purposes
	fs.Debug("rclone", "Version %q starting with parameters %q", fs.Version, os.Args)

	// Setup CPU profiling if desired
	if *cpuProfile != "" {
		fs.Log(nil, "Creating CPU profile %q\n", *cpuProfile)
		f, err := os.Create(*cpuProfile)
		if err != nil {
			fs.Stats.Error()
			log.Fatal(err)
		}
		err = pprof.StartCPUProfile(f)
		if err != nil {
			fs.Stats.Error()
			log.Fatal(err)
		}
		defer pprof.StopCPUProfile()
	}

	// Setup memory profiling if desired
	if *memProfile != "" {
		defer func() {
			fs.Log(nil, "Saving Memory profile %q\n", *memProfile)
			f, err := os.Create(*memProfile)
			if err != nil {
				fs.Stats.Error()
				log.Fatal(err)
			}
			err = pprof.WriteHeapProfile(f)
			if err != nil {
				fs.Stats.Error()
				log.Fatal(err)
			}
			err = f.Close()
			if err != nil {
				fs.Stats.Error()
				log.Fatal(err)
			}
		}()
	}

	// Make source and destination fs
	var fdst, fsrc fs.Fs
	if len(args) >= 1 {
		fdst = NewFsSrc(args[0])
	}
	if len(args) >= 2 {
		fsrc = fdst
		fdst = NewFs(args[1])
	}

	fs.CalculateModifyWindow(fdst, fsrc)

	if !command.NoStats {
		StartStats()
	}

	// Exit if no command to run
	if command.Run == nil {
		return
	}

	// Run the actual command
	var err error
	for try := 1; try <= *retries; try++ {
		err = command.Run(fdst, fsrc)
		if !command.Retry || (err == nil && !fs.Stats.Errored()) {
			break
		}
		if err != nil {
			fs.Log(nil, "Attempt %d/%d failed with %d errors and: %v", try, *retries, fs.Stats.GetErrors(), err)
		} else {
			fs.Log(nil, "Attempt %d/%d failed with %d errors", try, *retries, fs.Stats.GetErrors())
		}
		if try < *retries {
			fs.Stats.ResetErrors()
		}
	}
	if err != nil {
		log.Fatalf("Failed to %s: %v", command.Name, err)
	}
	if !command.NoStats && (!fs.Config.Quiet || fs.Stats.Errored() || *statsInterval > 0) {
		fs.Log(nil, "%s", fs.Stats)
	}
	if fs.Config.Verbose {
		fs.Debug(nil, "Go routines at exit %d\n", runtime.NumGoroutine())
	}
	if fs.Stats.Errored() {
		os.Exit(1)
	}
}
