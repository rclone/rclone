// Package cmd implemnts the rclone command
//
// It is in a sub package so it's internals can be re-used elsewhere
package cmd

// FIXME only attach the remote flags when using a remote???
// would probably mean bringing all the flags in to here? Or define some flagsets in fs...

import (
	"fmt"
	"log"
	"os"
	"path"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/ncw/rclone/fs"
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
)

// Root is the main rclone command
var Root = &cobra.Command{
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
}

// runRoot implements the main rclone command with no subcommands
func runRoot(cmd *cobra.Command, args []string) {
	if version {
		ShowVersion()
		os.Exit(0)
	} else {
		_ = Root.Usage()
		fmt.Fprintf(os.Stderr, "Command not found.\n")
		os.Exit(1)
	}
}

func init() {
	Root.Run = runRoot
	Root.Flags().BoolVarP(&version, "version", "V", false, "Print the version number")
	cobra.OnInitialize(initConfig)
}

// ShowVersion prints the version to stdout
func ShowVersion() {
	fmt.Printf("rclone %s\n", fs.Version)
}

// newFsSrc creates a src Fs from a name
//
// This can point to a file
func newFsSrc(remote string) fs.Fs {
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

// newFsDst creates a dst Fs from a name
//
// This must point to a directory
func newFsDst(remote string) fs.Fs {
	f, err := fs.NewFs(remote)
	if err != nil {
		fs.Stats.Error()
		log.Fatalf("Failed to create file system for %q: %v", remote, err)
	}
	return f
}

// NewFsSrcDst creates a new src and dst fs from the arguments
func NewFsSrcDst(args []string) (fs.Fs, fs.Fs) {
	fsrc, fdst := newFsSrc(args[0]), newFsDst(args[1])
	fs.CalculateModifyWindow(fdst, fsrc)
	return fdst, fsrc
}

// NewFsSrc creates a new src fs from the arguments
func NewFsSrc(args []string) fs.Fs {
	fsrc := newFsSrc(args[0])
	fs.CalculateModifyWindow(fsrc)
	return fsrc
}

// NewFsDst creates a new dst fs from the arguments
//
// Dst fs-es can't point to single files
func NewFsDst(args []string) fs.Fs {
	fdst := newFsDst(args[0])
	fs.CalculateModifyWindow(fdst)
	return fdst
}

// Run the function with stats and retries if required
func Run(Retry bool, cmd *cobra.Command, f func() error) {
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

// CheckArgs checks there are enough arguments and prints a message if not
func CheckArgs(MinArgs, MaxArgs int, cmd *cobra.Command, args []string) {
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
