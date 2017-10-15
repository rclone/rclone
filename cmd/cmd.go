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
	"os/exec"
	"path"
	"regexp"
	"runtime"
	"runtime/pprof"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/accounting"
	"github.com/ncw/rclone/fs/config/configflags"
	"github.com/ncw/rclone/fs/config/flags"
	"github.com/ncw/rclone/fs/filter"
	"github.com/ncw/rclone/fs/filter/filterflags"
	"github.com/ncw/rclone/fs/fserrors"
	"github.com/ncw/rclone/fs/fspath"
	fslog "github.com/ncw/rclone/fs/log"
	"github.com/ncw/rclone/fs/rc"
	"github.com/ncw/rclone/fs/rc/rcflags"
	"github.com/ncw/rclone/lib/atexit"
)

// Globals
var (
	// Flags
	cpuProfile    = flags.StringP("cpuprofile", "", "", "Write cpu profile to file")
	memProfile    = flags.StringP("memprofile", "", "", "Write memory profile to file")
	statsInterval = flags.DurationP("stats", "", time.Minute*1, "Interval between printing stats, e.g 500ms, 60s, 5m. (0 to disable)")
	dataRateUnit  = flags.StringP("stats-unit", "", "bytes", "Show data rate in stats as either 'bits' or 'bytes'/s")
	version       bool
	retries       = flags.IntP("retries", "", 3, "Retry operations this many times if they fail")
	// Errors
	errorCommandNotFound    = errors.New("command not found")
	errorUncategorized      = errors.New("uncategorized error")
	errorNotEnoughArguments = errors.New("not enough arguments")
	errorTooManyArguents    = errors.New("too many arguments")
	errorUsageError         = errors.New("usage error")
)

const (
	exitCodeSuccess = iota
	exitCodeUsageError
	exitCodeUncategorizedError
	exitCodeDirNotFound
	exitCodeFileNotFound
	exitCodeRetryError
	exitCodeNoRetryError
	exitCodeFatalError
)

// Root is the main rclone command
var Root = &cobra.Command{
	Use:   "rclone",
	Short: "Sync files and directories to and from local and remote object stores - " + fs.Version,
	Long: `
Rclone is a command line program to sync files and directories to and
from various cloud storage systems and using file transfer services, such as:

  * Amazon Drive
  * Amazon S3
  * Backblaze B2
  * Box
  * Dropbox
  * FTP
  * Google Cloud Storage
  * Google Drive
  * HTTP
  * Hubic
  * Mega
  * Microsoft Azure Blob Storage
  * Microsoft OneDrive
  * Openstack Swift / Rackspace cloud files / Memset Memstore
  * pCloud
  * QingStor
  * SFTP
  * Webdav / Owncloud / Nextcloud
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

  * https://rclone.org/
`,
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		fs.Debugf("rclone", "Version %q finishing with parameters %q", fs.Version, os.Args)
		atexit.Run()
	},
}

// runRoot implements the main rclone command with no subcommands
func runRoot(cmd *cobra.Command, args []string) {
	if version {
		ShowVersion()
		resolveExitCode(nil)
	} else {
		_ = Root.Usage()
		fmt.Fprintf(os.Stderr, "Command not found.\n")
		resolveExitCode(errorCommandNotFound)
	}
}

func init() {
	// Add global flags
	configflags.AddFlags(pflag.CommandLine)
	filterflags.AddFlags(pflag.CommandLine)
	rcflags.AddFlags(pflag.CommandLine)

	Root.Run = runRoot
	Root.Flags().BoolVarP(&version, "version", "V", false, "Print the version number")
	cobra.OnInitialize(initConfig)
}

// ShowVersion prints the version to stdout
func ShowVersion() {
	fmt.Printf("rclone %s\n", fs.Version)
	fmt.Printf("- os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("- go version: %s\n", runtime.Version())
}

// NewFsFile creates a dst Fs from a name but may point to a file.
//
// It returns a string with the file name if points to a file
func NewFsFile(remote string) (fs.Fs, string) {
	fsInfo, configName, fsPath, err := fs.ParseRemote(remote)
	if err != nil {
		fs.CountError(err)
		log.Fatalf("Failed to create file system for %q: %v", remote, err)
	}
	f, err := fsInfo.NewFs(configName, fsPath)
	switch err {
	case fs.ErrorIsFile:
		return f, path.Base(fsPath)
	case nil:
		return f, ""
	default:
		fs.CountError(err)
		log.Fatalf("Failed to create file system for %q: %v", remote, err)
	}
	return nil, ""
}

// newFsSrc creates a src Fs from a name
//
// It returns a string with the file name if limiting to one file
//
// This can point to a file
func newFsSrc(remote string) (fs.Fs, string) {
	f, fileName := NewFsFile(remote)
	if fileName != "" {
		if !filter.Active.InActive() {
			err := errors.Errorf("Can't limit to single files when using filters: %v", remote)
			fs.CountError(err)
			log.Fatalf(err.Error())
		}
		// Limit transfers to this file
		err := filter.Active.AddFile(fileName)
		if err != nil {
			fs.CountError(err)
			log.Fatalf("Failed to limit to single file %q: %v", remote, err)
		}
	}
	return f, fileName
}

// newFsDst creates a dst Fs from a name
//
// This must point to a directory
func newFsDst(remote string) fs.Fs {
	f, err := fs.NewFs(remote)
	if err != nil {
		fs.CountError(err)
		log.Fatalf("Failed to create file system for %q: %v", remote, err)
	}
	return f
}

// NewFsSrcDst creates a new src and dst fs from the arguments
func NewFsSrcDst(args []string) (fs.Fs, fs.Fs) {
	fsrc, _ := newFsSrc(args[0])
	fdst := newFsDst(args[1])
	fs.CalculateModifyWindow(fdst, fsrc)
	return fsrc, fdst
}

// NewFsSrcDstFiles creates a new src and dst fs from the arguments
// If src is a file then srcFileName and dstFileName will be non-empty
func NewFsSrcDstFiles(args []string) (fsrc fs.Fs, srcFileName string, fdst fs.Fs, dstFileName string) {
	fsrc, srcFileName = newFsSrc(args[0])
	// If copying a file...
	dstRemote := args[1]
	// If file exists then srcFileName != "", however if the file
	// doesn't exist then we assume it is a directory...
	if srcFileName != "" {
		dstRemote, dstFileName = fspath.RemoteSplit(dstRemote)
		if dstRemote == "" {
			dstRemote = "."
		}
		if dstFileName == "" {
			log.Fatalf("%q is a directory", args[1])
		}
	}
	fdst, err := fs.NewFs(dstRemote)
	switch err {
	case fs.ErrorIsFile:
		fs.CountError(err)
		log.Fatalf("Source doesn't exist or is a directory and destination is a file")
	case nil:
	default:
		fs.CountError(err)
		log.Fatalf("Failed to create file system for destination %q: %v", dstRemote, err)
	}
	fs.CalculateModifyWindow(fdst, fsrc)
	return
}

// NewFsSrc creates a new src fs from the arguments
func NewFsSrc(args []string) fs.Fs {
	fsrc, _ := newFsSrc(args[0])
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

// NewFsDstFile creates a new dst fs with a destination file name from the arguments
func NewFsDstFile(args []string) (fdst fs.Fs, dstFileName string) {
	dstRemote, dstFileName := fspath.RemoteSplit(args[0])
	if dstRemote == "" {
		dstRemote = "."
	}
	if dstFileName == "" {
		log.Fatalf("%q is a directory", args[0])
	}
	fdst = newFsDst(dstRemote)
	fs.CalculateModifyWindow(fdst)
	return
}

// ShowStats returns true if the user added a `--stats` flag to the command line.
//
// This is called by Run to override the default value of the
// showStats passed in.
func ShowStats() bool {
	statsIntervalFlag := pflag.Lookup("stats")
	return statsIntervalFlag != nil && statsIntervalFlag.Changed
}

// Run the function with stats and retries if required
func Run(Retry bool, showStats bool, cmd *cobra.Command, f func() error) {
	var err error
	var stopStats chan struct{}
	if !showStats && ShowStats() {
		showStats = true
	}
	if showStats {
		stopStats = StartStats()
	}
	for try := 1; try <= *retries; try++ {
		err = f()
		if !Retry || (err == nil && !accounting.Stats.Errored()) {
			if try > 1 {
				fs.Errorf(nil, "Attempt %d/%d succeeded", try, *retries)
			}
			break
		}
		if fserrors.IsFatalError(err) {
			fs.Errorf(nil, "Fatal error received - not attempting retries")
			break
		}
		if fserrors.IsNoRetryError(err) {
			fs.Errorf(nil, "Can't retry this error - not attempting retries")
			break
		}
		if err != nil {
			fs.Errorf(nil, "Attempt %d/%d failed with %d errors and: %v", try, *retries, accounting.Stats.GetErrors(), err)
		} else {
			fs.Errorf(nil, "Attempt %d/%d failed with %d errors", try, *retries, accounting.Stats.GetErrors())
		}
		if try < *retries {
			accounting.Stats.ResetErrors()
		}
	}
	if showStats {
		close(stopStats)
	}
	if err != nil {
		log.Printf("Failed to %s: %v", cmd.Name(), err)
		resolveExitCode(err)
	}
	if showStats && (accounting.Stats.Errored() || *statsInterval > 0) {
		accounting.Stats.Log()
	}
	fs.Debugf(nil, "%d go routines active\n", runtime.NumGoroutine())

	// dump all running go-routines
	if fs.Config.Dump&fs.DumpGoRoutines != 0 {
		err = pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
		if err != nil {
			fs.Errorf(nil, "Failed to dump goroutines: %v", err)
		}
	}

	// dump open files
	if fs.Config.Dump&fs.DumpOpenFiles != 0 {
		c := exec.Command("lsof", "-p", strconv.Itoa(os.Getpid()))
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		err = c.Run()
		if err != nil {
			fs.Errorf(nil, "Failed to list open files: %v", err)
		}
	}

	if accounting.Stats.Errored() {
		resolveExitCode(accounting.Stats.GetLastError())
	}
}

// CheckArgs checks there are enough arguments and prints a message if not
func CheckArgs(MinArgs, MaxArgs int, cmd *cobra.Command, args []string) {
	if len(args) < MinArgs {
		_ = cmd.Usage()
		fmt.Fprintf(os.Stderr, "Command %s needs %d arguments mininum\n", cmd.Name(), MinArgs)
		// os.Exit(1)
		resolveExitCode(errorNotEnoughArguments)
	} else if len(args) > MaxArgs {
		_ = cmd.Usage()
		fmt.Fprintf(os.Stderr, "Command %s needs %d arguments maximum\n", cmd.Name(), MaxArgs)
		// os.Exit(1)
		resolveExitCode(errorTooManyArguents)
	}
}

// StartStats prints the stats every statsInterval
//
// It returns a channel which should be closed to stop the stats.
func StartStats() chan struct{} {
	stopStats := make(chan struct{})
	if *statsInterval > 0 {
		go func() {
			ticker := time.NewTicker(*statsInterval)
			for {
				select {
				case <-ticker.C:
					accounting.Stats.Log()
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
	// Start the logger
	fslog.InitLogging()

	// Finish parsing any command line flags
	configflags.SetFlags()

	// Load filters
	var err error
	filter.Active, err = filter.NewFilter(&filterflags.Opt)
	if err != nil {
		log.Fatalf("Failed to load filters: %v", err)
	}

	// Write the args for debug purposes
	fs.Debugf("rclone", "Version %q starting with parameters %q", fs.Version, os.Args)

	// Start the remote control if configured
	rc.Start(&rcflags.Opt)

	// Setup CPU profiling if desired
	if *cpuProfile != "" {
		fs.Infof(nil, "Creating CPU profile %q\n", *cpuProfile)
		f, err := os.Create(*cpuProfile)
		if err != nil {
			fs.CountError(err)
			log.Fatal(err)
		}
		err = pprof.StartCPUProfile(f)
		if err != nil {
			fs.CountError(err)
			log.Fatal(err)
		}
		atexit.Register(func() {
			pprof.StopCPUProfile()
		})
	}

	// Setup memory profiling if desired
	if *memProfile != "" {
		atexit.Register(func() {
			fs.Infof(nil, "Saving Memory profile %q\n", *memProfile)
			f, err := os.Create(*memProfile)
			if err != nil {
				fs.CountError(err)
				log.Fatal(err)
			}
			err = pprof.WriteHeapProfile(f)
			if err != nil {
				fs.CountError(err)
				log.Fatal(err)
			}
			err = f.Close()
			if err != nil {
				fs.CountError(err)
				log.Fatal(err)
			}
		})
	}

	if m, _ := regexp.MatchString("^(bits|bytes)$", *dataRateUnit); m == false {
		fs.Errorf(nil, "Invalid unit passed to --stats-unit. Defaulting to bytes.")
		fs.Config.DataRateUnit = "bytes"
	} else {
		fs.Config.DataRateUnit = *dataRateUnit
	}
}

func resolveExitCode(err error) {
	if err == nil {
		os.Exit(exitCodeSuccess)
	}

	err = errors.Cause(err)

	switch {
	case err == fs.ErrorDirNotFound:
		os.Exit(exitCodeDirNotFound)
	case err == fs.ErrorObjectNotFound:
		os.Exit(exitCodeFileNotFound)
	case err == errorUncategorized:
		os.Exit(exitCodeUncategorizedError)
	case fserrors.ShouldRetry(err):
		os.Exit(exitCodeRetryError)
	case fserrors.IsNoRetryError(err):
		os.Exit(exitCodeNoRetryError)
	case fserrors.IsFatalError(err):
		os.Exit(exitCodeFatalError)
	default:
		os.Exit(exitCodeUsageError)
	}
}
