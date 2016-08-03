// Sync files and directories to and from local and remote object stores
//
// Nick Craig-Wood <nick@craig-wood.com>
package main

// FIXME only attach the remote flags when using a remote???
// would probably mean bringing all the flags in to here? Or define some flagsets in fs...

import (
	"fmt"
	"log"
	"os"
	"path"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"

	"github.com/spf13/cobra"
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
	version       bool
	logFile       = pflag.StringP("log-file", "", "", "Log everything to this file")
	retries       = pflag.IntP("retries", "", 3, "Retry operations this many times if they fail")
	dedupeMode    = fs.DeduplicateInteractive
)

var rootCmd = &cobra.Command{
	Use:   "rclone",
	Short: "Sync files and directories to and from local and remote object stores - " + fs.Version,
	Long: `
Rclone is a command line program to sync files and directories to and
from various cloud storage systems, such as:

  * Google Drive
  * Amazon S3
  * Openstack Swift / Rackspace cloud files / Memset Memstore
  * Dropbox
  * Google Cloud Storage
  * Amazon Drive
  * Microsoft One Drive
  * Hubic
  * Backblaze B2
  * Yandex Disk
  * The local filesystem

Features

  * MD5/SHA1 hashes checked at all times for file integrity
  * Timestamps preserved on files
  * Partial syncs supported on a whole file basis
  * Copy mode to just copy new/changed files
  * Sync (one way) mode to make a directory identical
  * Check mode to check for file hash equality
  * Can sync to and from network, eg two different cloud accounts

See the home page for installation, usage, documentation, changelog
and configuration walkthroughs.

  * http://rclone.org/
`,
	Run: func(cmd *cobra.Command, args []string) {
		if version {
			showVersion()
			os.Exit(0)
		}
	},
}

func init() {
	rootCmd.Flags().BoolVarP(&version, "version", "V", false, "Print the version number")
	rootCmd.AddCommand(copyCmd, syncCmd, moveCmd, lsCmd, lsdCmd,
		lslCmd, md5sumCmd, sha1sumCmd, sizeCmd, mkdirCmd,
		rmdirCmd, purgeCmd, deleteCmd, checkCmd, dedupeCmd,
		genautocompleteCmd, configCmd, authorizeCmd,
		cleanupCmd, memtestCmd, versionCmd)
	dedupeCmd.Flags().VarP(&dedupeMode, "dedupe-mode", "", "Dedupe mode interactive|skip|first|newest|oldest|rename.")
	cobra.OnInitialize(initConfig)
}

func showVersion() {
	fmt.Printf("rclone %s\n", fs.Version)
}

// NewFsSrc creates a src Fs from a name
//
// This can point to a file
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

// NewFsDst creates a dst Fs from a name
//
// This must point to a directory
func NewFsDst(remote string) fs.Fs {
	f, err := fs.NewFs(remote)
	if err != nil {
		fs.Stats.Error()
		log.Fatalf("Failed to create file system for %q: %v", remote, err)
	}
	return f
}

// Create a new src and dst fs from the arguments
func newFsSrcDst(args []string) (fs.Fs, fs.Fs) {
	fsrc, fdst := NewFsSrc(args[0]), NewFsDst(args[1])
	fs.CalculateModifyWindow(fdst, fsrc)
	return fdst, fsrc
}

// Create a new src fs from the arguments
func newFsSrc(args []string) fs.Fs {
	fsrc := NewFsSrc(args[0])
	fs.CalculateModifyWindow(fsrc)
	return fsrc
}

// Create a new dst fs from the arguments
//
// Dst fs-es can't point to single files
func newFsDst(args []string) fs.Fs {
	fdst := NewFsDst(args[0])
	fs.CalculateModifyWindow(fdst)
	return fdst
}

// run the function with stats and retries if required
func run(Retry bool, cmd *cobra.Command, f func() error) {
	var err error
	stopStats := startStats()
	for try := 1; try <= *retries; try++ {
		err = f()
		if !Retry || (err == nil && !fs.Stats.Errored()) {
			break
		}
		if fs.IsFatalError(err) {
			fs.Log(nil, "Fatal error received - not attempting retries")
			break
		}
		if fs.IsNoRetryError(err) {
			fs.Log(nil, "Can't retry this error - not attempting retries")
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
	close(stopStats)
	if err != nil {
		log.Fatalf("Failed to %s: %v", cmd.Name(), err)
	}
	if !fs.Config.Quiet || fs.Stats.Errored() || *statsInterval > 0 {
		fs.Log(nil, "%s", fs.Stats)
	}
	if fs.Config.Verbose {
		fs.Debug(nil, "Go routines at exit %d\n", runtime.NumGoroutine())
	}
	if fs.Stats.Errored() {
		os.Exit(1)
	}
}

// checkArgs checks there are enough arguments and prints a message if not
func checkArgs(MinArgs, MaxArgs int, cmd *cobra.Command, args []string) {
	if len(args) < MinArgs {
		_ = cmd.Usage()
		fmt.Fprintf(os.Stderr, "Command %s needs %d arguments mininum\n", cmd.Name(), MinArgs)
		os.Exit(1)
	} else if len(args) > MaxArgs {
		_ = cmd.Usage()
		fmt.Fprintf(os.Stderr, "Command %s needs %d arguments maximum\n", cmd.Name(), MaxArgs)
		os.Exit(1)
	}
}

// startStats prints the stats every statsInterval
//
// It returns a channel which should be closed to stop the stats.
func startStats() chan struct{} {
	stopStats := make(chan struct{})
	if *statsInterval > 0 {
		go func() {
			ticker := time.NewTicker(*statsInterval)
			for {
				select {
				case <-ticker.C:
					fs.Stats.Log()
				case <-stopStats:
					ticker.Stop()
					return
				}
			}
		}()
	}
	return stopStats
}

// The commands
var copyCmd = &cobra.Command{
	Use:   "copy source:path dest:path",
	Short: `Copy files from source to dest, skipping already copied`,
	Long: `
Copy the source to the destination.  Doesn't transfer
unchanged files, testing by size and modification time or
MD5SUM.  Doesn't delete files from the destination.`,
	Run: func(cmd *cobra.Command, args []string) {
		checkArgs(2, 2, cmd, args)
		fsrc, fdst := newFsSrcDst(args)
		run(true, cmd, func() error {
			return fs.CopyDir(fdst, fsrc)
		})
	},
}

var syncCmd = &cobra.Command{
	Use:   "sync source:path dest:path",
	Short: `Make source and dest identical, modifying destination only.`,
	Long: `
Sync the source to the destination, changing the destination
only.  Doesn't transfer unchanged files, testing by size and
modification time or MD5SUM.  Destination is updated to match
source, including deleting files if necessary.  Since this can
cause data loss, test first with the --dry-run flag.`,
	Run: func(cmd *cobra.Command, args []string) {
		checkArgs(2, 2, cmd, args)
		fsrc, fdst := newFsSrcDst(args)
		run(true, cmd, func() error {
			return fs.Sync(fdst, fsrc)
		})
	},
}

var moveCmd = &cobra.Command{
	Use:   "move source:path dest:path",
	Short: `Move files from source to dest.`,
	Long: `
Moves the source to the destination.  This is equivalent to a
copy followed by a purge, but may use server side operations
to speed it up. Since this can cause data loss, test first
with the --dry-run flag.`,
	Run: func(cmd *cobra.Command, args []string) {
		checkArgs(2, 2, cmd, args)
		fsrc, fdst := newFsSrcDst(args)
		run(true, cmd, func() error {
			return fs.MoveDir(fdst, fsrc)
		})
	},
}

var lsCmd = &cobra.Command{
	Use:   "ls remote:path",
	Short: `List all the objects in the the path with size and path.`,
	Run: func(cmd *cobra.Command, args []string) {
		checkArgs(1, 1, cmd, args)
		fsrc := newFsSrc(args)
		run(false, cmd, func() error {
			return fs.List(fsrc, os.Stdout)
		})
	},
}

var lsdCmd = &cobra.Command{
	Use:   "lsd remote:path",
	Short: `List all directories/containers/buckets in the the path.`,
	Run: func(cmd *cobra.Command, args []string) {
		checkArgs(1, 1, cmd, args)
		fsrc := newFsSrc(args)
		run(false, cmd, func() error {
			return fs.ListDir(fsrc, os.Stdout)
		})
	},
}

var lslCmd = &cobra.Command{
	Use:   "lsl remote:path",
	Short: `List all the objects path with modification time, size and path.`,
	Run: func(cmd *cobra.Command, args []string) {
		checkArgs(1, 1, cmd, args)
		fsrc := newFsSrc(args)
		run(false, cmd, func() error {
			return fs.ListLong(fsrc, os.Stdout)
		})
	},
}

var md5sumCmd = &cobra.Command{
	Use:   "md5sum remote:path",
	Short: `Produces an md5sum file for all the objects in the path.`,
	Long: `
Produces an md5sum file for all the objects in the path.  This
is in the same format as the standard md5sum tool produces.`,
	Run: func(cmd *cobra.Command, args []string) {
		checkArgs(1, 1, cmd, args)
		fsrc := newFsSrc(args)
		run(false, cmd, func() error {
			return fs.Md5sum(fsrc, os.Stdout)
		})
	},
}

var sha1sumCmd = &cobra.Command{
	Use:   "sha1sum remote:path",
	Short: `Produces an sha1sum file for all the objects in the path.`,
	Long: `
Produces an sha1sum file for all the objects in the path.  This
is in the same format as the standard sha1sum tool produces.`,
	Run: func(cmd *cobra.Command, args []string) {
		checkArgs(1, 1, cmd, args)
		fsrc := newFsSrc(args)
		run(false, cmd, func() error {
			return fs.Sha1sum(fsrc, os.Stdout)
		})
	},
}

var sizeCmd = &cobra.Command{
	Use:   "size remote:path",
	Short: `Returns the total size and number of objects in remote:path.`,
	Run: func(cmd *cobra.Command, args []string) {
		checkArgs(1, 1, cmd, args)
		fsrc := newFsSrc(args)
		run(false, cmd, func() error {
			objects, size, err := fs.Count(fsrc)
			if err != nil {
				return err
			}
			fmt.Printf("Total objects: %d\n", objects)
			fmt.Printf("Total size: %s (%d Bytes)\n", fs.SizeSuffix(size).Unit("Bytes"), size)
			return nil
		})
	},
}

var mkdirCmd = &cobra.Command{
	Use:   "mkdir remote:path",
	Short: `Make the path if it doesn't already exist.`,
	Run: func(cmd *cobra.Command, args []string) {
		checkArgs(1, 1, cmd, args)
		fdst := newFsDst(args)
		run(true, cmd, func() error {
			return fs.Mkdir(fdst)
		})
	},
}

var rmdirCmd = &cobra.Command{
	Use:   "rmdir remote:path",
	Short: `Remove the path.`,
	Long: `
Remove the path.  Note that you can't remove a path with
objects in it, use purge for that.`,
	Run: func(cmd *cobra.Command, args []string) {
		checkArgs(1, 1, cmd, args)
		fdst := newFsDst(args)
		run(true, cmd, func() error {
			return fs.Rmdir(fdst)
		})
	},
}

var purgeCmd = &cobra.Command{
	Use:   "purge remote:path",
	Short: `Remove the path and all of its contents.`,
	Long: `
Remove the path and all of its contents.  Does not obey
filters - use remove for that.`,
	Run: func(cmd *cobra.Command, args []string) {
		checkArgs(1, 1, cmd, args)
		fdst := newFsDst(args)
		run(true, cmd, func() error {
			return fs.Purge(fdst)
		})
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete remote:path",
	Short: `Remove the contents of path.`,
	Long: `
Remove the contents of path.  Obeys include/exclude filters.`,
	Run: func(cmd *cobra.Command, args []string) {
		checkArgs(1, 1, cmd, args)
		fsrc := newFsSrc(args)
		run(true, cmd, func() error {
			return fs.Delete(fsrc)
		})
	},
}

var checkCmd = &cobra.Command{
	Use:   "check source:path dest:path",
	Short: `Checks the files in the source and destination match.`,
	Long: `
Checks the files in the source and destination match.  It
compares sizes and MD5SUMs and prints a report of files which
don't match.  It doesn't alter the source or destination.`,
	Run: func(cmd *cobra.Command, args []string) {
		checkArgs(2, 2, cmd, args)
		fsrc, fdst := newFsSrcDst(args)
		run(false, cmd, func() error {
			return fs.Check(fdst, fsrc)
		})
	},
}

var dedupeCmd = &cobra.Command{
	Use:   "dedupe [mode] remote:path",
	Short: `Interactively find duplicate files delete/rename them.`,
	Long: `
Interactively find duplicate files and offer to delete all
but one or rename them to be different. Only useful with
Google Drive which can have duplicate file names.

Pass in an extra parameter to make the dedupe noninteractive

  interactive - interactive as above.
  skip        - removes identical files then skips anything left.
  first       - removes identical files then keeps the first one.
  newest      - removes identical files then keeps the newest one.
  oldest      - removes identical files then keeps the oldest one.
  rename      - removes identical files then renames the rest to be different.
`,
	Run: func(cmd *cobra.Command, args []string) {
		checkArgs(1, 2, cmd, args)
		if len(args) > 1 {
			err := dedupeMode.Set(args[0])
			if err != nil {
				log.Fatal(err)
			}
			args = args[1:]
		}
		fdst := newFsSrc(args)
		run(false, cmd, func() error {
			return fs.Deduplicate(fdst, dedupeMode)
		})
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: `Enter an interactive configuration session.`,
	Run: func(cmd *cobra.Command, args []string) {
		checkArgs(0, 0, cmd, args)
		fs.EditConfig()
	},
}

var genautocompleteCmd = &cobra.Command{
	Use:   "genautocomplete [output_file]",
	Short: `Output bash completion script for rclone.`,
	Long: `
Generates a bash shell autocompletion script for rclone.

This writes to /etc/bash_completion.d/rclone by default so will
probably need to be run with sudo or as root, eg

    sudo rclone genautocomplete

Logout and login again to use the autocompletion scripts, or source
them directly

    . /etc/bash_completion

If you supply a command line argument the script will be written
there.
`,
	Run: func(cmd *cobra.Command, args []string) {
		checkArgs(0, 1, cmd, args)
		out := "/etc/bash_completion.d/rclone"
		if len(args) > 0 {
			out = args[0]
		}
		err := rootCmd.GenBashCompletionFile(out)
		if err != nil {
			log.Fatal(err)
		}
	},
}

var authorizeCmd = &cobra.Command{
	Use:   "authorize",
	Short: `Remote authorization.`,
	Long: `
Remote authorization. Used to authorize a remote or headless
rclone from a machine with a browser - use as instructed by
rclone config.`,
	Run: func(cmd *cobra.Command, args []string) {
		checkArgs(1, 3, cmd, args)
		fs.Authorize(args)
	},
}

var cleanupCmd = &cobra.Command{
	Use:   "cleanup remote:path",
	Short: `Clean up the remote if possible`,
	Long: `
Clean up the remote if possible.  Empty the trash or delete
old file versions. Not supported by all remotes.`,
	Run: func(cmd *cobra.Command, args []string) {
		checkArgs(1, 1, cmd, args)
		fsrc := newFsSrc(args)
		run(true, cmd, func() error {
			return fs.CleanUp(fsrc)
		})
	},
}

var memtestCmd = &cobra.Command{
	Use:    "memtest remote:path",
	Short:  `Load all the objects at remote:path and report memory stats.`,
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		checkArgs(1, 1, cmd, args)
		fsrc := newFsSrc(args)
		run(false, cmd, func() error {
			objects, _, err := fs.Count(fsrc)
			if err != nil {
				return err
			}
			objs := make([]fs.Object, 0, objects)
			var before, after runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&before)
			var mu sync.Mutex
			err = fs.ListFn(fsrc, func(o fs.Object) {
				mu.Lock()
				objs = append(objs, o)
				mu.Unlock()
			})
			if err != nil {
				return err
			}
			runtime.GC()
			runtime.ReadMemStats(&after)
			usedMemory := after.Alloc - before.Alloc
			fs.Log(nil, "%d objects took %d bytes, %.1f bytes/object", len(objs), usedMemory, float64(usedMemory)/float64(len(objs)))
			fs.Log(nil, "System memory changed from %d to %d bytes a change of %d bytes", before.Sys, after.Sys, after.Sys-before.Sys)
			return nil
		})
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: `Show the version number.`,
	Run: func(cmd *cobra.Command, args []string) {
		checkArgs(0, 0, cmd, args)
		showVersion()
	},
}

// initConfig is run by cobra after initialising the flags
func initConfig() {
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
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
	os.Exit(0)
}
