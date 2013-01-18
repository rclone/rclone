// Sync files and directories to and from swift
// 
// Nick Craig-Wood <nick@craig-wood.com>
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"time"
)

// Globals
var (
	// Flags
	cpuprofile    = flag.String("cpuprofile", "", "Write cpu profile to file")
	verbose       = flag.Bool("verbose", false, "Print lots more stuff")
	quiet         = flag.Bool("quiet", false, "Print as little stuff as possible")
	dry_run       = flag.Bool("dry-run", false, "Do a trial run with no permanent changes")
	checkers      = flag.Int("checkers", 8, "Number of checkers to run in parallel.")
	transfers     = flag.Int("transfers", 4, "Number of file transfers to run in parallel.")
	statsInterval = flag.Duration("stats", time.Minute*1, "Interval to print stats")
	modifyWindow  = flag.Duration("modify-window", time.Nanosecond, "Max time diff to be considered the same")
)

// A pair of FsObjects
type PairFsObjects struct {
	src, dst FsObject
}

type PairFsObjectsChan chan PairFsObjects

// Check to see if src needs to be copied to dst and if so puts it in out
func checkOne(src, dst FsObject, out FsObjectsChan) {
	if dst == nil {
		FsDebug(src, "Couldn't find local file - download")
		out <- src
		return
	}
	// Check to see if can store this
	if !src.Storable() {
		return
	}
	// Check to see if changed or not
	if Equal(src, dst) {
		FsDebug(src, "Unchanged skipping")
		return
	}
	out <- src
}

// Read FsObjects~s on in send to out if they need uploading
//
// FIXME potentially doing lots of MD5SUMS at once
func PairChecker(in PairFsObjectsChan, out FsObjectsChan, wg *sync.WaitGroup) {
	defer wg.Done()
	for pair := range in {
		src := pair.src
		stats.Checking(src)
		checkOne(src, pair.dst, out)
		stats.DoneChecking(src)
	}
}

// Read FsObjects~s on in send to out if they need uploading
//
// FIXME potentially doing lots of MD5SUMS at once
func Checker(in, out FsObjectsChan, fdst Fs, wg *sync.WaitGroup) {
	defer wg.Done()
	for src := range in {
		stats.Checking(src)
		dst := fdst.NewFsObject(src.Remote())
		checkOne(src, dst, out)
		stats.DoneChecking(src)
	}
}

// Read FsObjects on in and copy them
func Copier(in FsObjectsChan, fdst Fs, wg *sync.WaitGroup) {
	defer wg.Done()
	for src := range in {
		stats.Transferring(src)
		Copy(fdst, src)
		stats.DoneTransferring(src)
	}
}

// Copies fsrc into fdst
func CopyFs(fdst, fsrc Fs) {
	err := fdst.Mkdir()
	if err != nil {
		stats.Error()
		log.Fatal("Failed to make destination")
	}

	to_be_checked := fsrc.List()
	to_be_uploaded := make(FsObjectsChan, *transfers)

	var checkerWg sync.WaitGroup
	checkerWg.Add(*checkers)
	for i := 0; i < *checkers; i++ {
		go Checker(to_be_checked, to_be_uploaded, fdst, &checkerWg)
	}

	var copierWg sync.WaitGroup
	copierWg.Add(*transfers)
	for i := 0; i < *transfers; i++ {
		go Copier(to_be_uploaded, fdst, &copierWg)
	}

	log.Printf("Waiting for checks to finish")
	checkerWg.Wait()
	close(to_be_uploaded)
	log.Printf("Waiting for transfers to finish")
	copierWg.Wait()
}

// Delete all the files passed in the channel
func DeleteFiles(to_be_deleted FsObjectsChan) {
	var wg sync.WaitGroup
	wg.Add(*transfers)
	for i := 0; i < *transfers; i++ {
		go func() {
			defer wg.Done()
			for dst := range to_be_deleted {
				if *dry_run {
					FsDebug(dst, "Not deleting as -dry-run")
				} else {
					stats.Checking(dst)
					err := dst.Remove()
					stats.DoneChecking(dst)
					if err != nil {
						stats.Error()
						FsLog(dst, "Couldn't delete: %s", err)
					} else {
						FsDebug(dst, "Deleted")
					}
				}
			}
		}()
	}

	log.Printf("Waiting for deletions to finish")
	wg.Wait()
}

// Syncs fsrc into fdst
func Sync(fdst, fsrc Fs) {
	err := fdst.Mkdir()
	if err != nil {
		stats.Error()
		log.Fatal("Failed to make destination")
	}

	log.Printf("Building file list")

	// Read the destination files first
	// FIXME could do this in parallel and make it use less memory
	delFiles := make(map[string]FsObject)
	for dst := range fdst.List() {
		delFiles[dst.Remote()] = dst
	}

	// Read source files checking them off against dest files
	to_be_checked := make(PairFsObjectsChan, *transfers)
	to_be_uploaded := make(FsObjectsChan, *transfers)

	var checkerWg sync.WaitGroup
	checkerWg.Add(*checkers)
	for i := 0; i < *checkers; i++ {
		go PairChecker(to_be_checked, to_be_uploaded, &checkerWg)
	}

	var copierWg sync.WaitGroup
	copierWg.Add(*transfers)
	for i := 0; i < *transfers; i++ {
		go Copier(to_be_uploaded, fdst, &copierWg)
	}

	go func() {
		for src := range fsrc.List() {
			remote := src.Remote()
			dst, found := delFiles[remote]
			if found {
				delete(delFiles, remote)
				to_be_checked <- PairFsObjects{src, dst}
			} else {
				// No need to check doesn't exist
				to_be_uploaded <- src
			}
		}
		close(to_be_checked)
	}()

	log.Printf("Waiting for checks to finish")
	checkerWg.Wait()
	close(to_be_uploaded)
	log.Printf("Waiting for transfers to finish")
	copierWg.Wait()

	if stats.errors != 0 {
		log.Printf("Not deleting files as there were IO errors")
		return
	}

	// Delete the spare files
	toDelete := make(FsObjectsChan, *transfers)
	go func() {
		for _, fs := range delFiles {
			toDelete <- fs
		}
		close(toDelete)
	}()
	DeleteFiles(toDelete)
}

// Checks the files in fsrc and fdst according to Size and MD5SUM
func Check(fdst, fsrc Fs) {
	log.Printf("Building file list")

	// Read the destination files first
	// FIXME could do this in parallel and make it use less memory
	dstFiles := make(map[string]FsObject)
	for dst := range fdst.List() {
		dstFiles[dst.Remote()] = dst
	}

	// Read the source files checking them against dstFiles
	// FIXME could do this in parallel and make it use less memory
	srcFiles := make(map[string]FsObject)
	commonFiles := make(map[string][]FsObject)
	for src := range fsrc.List() {
		remote := src.Remote()
		if dst, ok := dstFiles[remote]; ok {
			commonFiles[remote] = []FsObject{dst, src}
			delete(dstFiles, remote)
		} else {
			srcFiles[remote] = src
		}
	}

	log.Printf("Files in %s but not in %s", fdst, fsrc)
	for remote := range dstFiles {
		stats.Error()
		log.Printf(remote)
	}

	log.Printf("Files in %s but not in %s", fsrc, fdst)
	for remote := range srcFiles {
		stats.Error()
		log.Printf(remote)
	}

	checks := make(chan []FsObject, *transfers)
	go func() {
		for _, check := range commonFiles {
			checks <- check
		}
		close(checks)
	}()

	var checkerWg sync.WaitGroup
	checkerWg.Add(*checkers)
	for i := 0; i < *checkers; i++ {
		go func() {
			defer checkerWg.Done()
			for check := range checks {
				dst, src := check[0], check[1]
				stats.Checking(src)
				if src.Size() != dst.Size() {
					stats.DoneChecking(src)
					stats.Error()
					FsLog(src, "Sizes differ")
					continue
				}
				same, err := CheckMd5sums(src, dst)
				stats.DoneChecking(src)
				if err != nil {
					continue
				}
				if !same {
					stats.Error()
					FsLog(src, "Md5sums differ")
				}
				FsDebug(src, "OK")
			}
		}()
	}

	log.Printf("Waiting for checks to finish")
	checkerWg.Wait()
	log.Printf("%d differences found", stats.errors)
}

// List the Fs to stdout
//
// Lists in parallel which may get them out of order
func List(f Fs) {
	in := f.List()
	var wg sync.WaitGroup
	wg.Add(*checkers)
	for i := 0; i < *checkers; i++ {
		go func() {
			defer wg.Done()
			for fs := range in {
				stats.Checking(fs)
				modTime := fs.ModTime()
				stats.DoneChecking(fs)
				fmt.Printf("%9d %19s %s\n", fs.Size(), modTime.Format("2006-01-02 15:04:05.00000000"), fs.Remote())
			}
		}()
	}
	wg.Wait()
}

// Lists files in a container
func list(fdst, fsrc Fs) {
	if fdst == nil {
		// FIXMESwiftContainers()
		S3Buckets()
		return
	}
	List(fdst)
}

// Makes a destination directory or container
func mkdir(fdst, fsrc Fs) {
	err := fdst.Mkdir()
	if err != nil {
		stats.Error()
		log.Fatalf("Mkdir failed: %s", err)
	}
}

// Removes a container but not if not empty
func rmdir(fdst, fsrc Fs) {
	if *dry_run {
		log.Printf("Not deleting %s as -dry-run", fdst)
	} else {
		err := fdst.Rmdir()
		if err != nil {
			stats.Error()
			log.Fatalf("Rmdir failed: %s", err)
		}
	}
}

// Removes a container and all of its contents
//
// FIXME doesn't delete local directories
func purge(fdst, fsrc Fs) {
	if f, ok := fdst.(Purger); ok {
		err := f.Purge()
		if err != nil {
			stats.Error()
			log.Fatalf("Purge failed: %s", err)
		}
	} else {
		DeleteFiles(fdst.List())
		log.Printf("Deleting path")
		rmdir(fdst, fsrc)
	}
}

type Command struct {
	name             string
	help             string
	run              func(fdst, fsrc Fs)
	minArgs, maxArgs int
}

// checkArgs checks there are enough arguments and prints a message if not
func (cmd *Command) checkArgs(args []string) {
	if len(args) < cmd.minArgs {
		syntaxError()
		fmt.Fprintf(os.Stderr, "Command %s needs %d arguments mininum\n", cmd.name, cmd.minArgs)
		os.Exit(1)
	} else if len(args) > cmd.maxArgs {
		syntaxError()
		fmt.Fprintf(os.Stderr, "Command %s needs %d arguments maximum\n", cmd.name, cmd.maxArgs)
		os.Exit(1)
	}
}

var Commands = []Command{
	{
		"copy",
		`<source> <destination>

        Copy the source to the destination.  Doesn't transfer
        unchanged files, testing first by modification time then by
        MD5SUM.  Doesn't delete files from the destination.

`,
		CopyFs,
		2, 2,
	},
	{
		"sync",
		`<source> <destination>

        Sync the source to the destination.  Doesn't transfer
        unchanged files, testing first by modification time then by
        MD5SUM.  Deletes any files that exist in source that don't
        exist in destination. Since this can cause data loss, test
        first with the -dry-run flag.`,

		Sync,
		2, 2,
	},
	{
		"ls",
		`[<path>]

        List the path. If no parameter is supplied then it lists the
        available swift containers.`,

		list,
		0, 1,
	},
	{
		"mkdir",
		`<path>

        Make the path if it doesn't already exist`,

		mkdir,
		1, 1,
	},
	{
		"rmdir",
		`<path>

        Remove the path.  Note that you can't remove a path with
        objects in it, use purge for that.`,

		rmdir,
		1, 1,
	},
	{
		"purge",
		`<path>

        Remove the path and all of its contents.`,

		purge,
		1, 1,
	},
	{
		"check",
		`<source> <destination>

        Checks the files in the source and destination match.  It
        compares sizes and MD5SUMs and prints a report of files which
        don't match.  It doesn't alter the source or destination.`,

		Check,
		2, 2,
	},
	{
		"help",
		`

        This help.`,

		nil,
		0, 0,
	},
}

// syntaxError prints the syntax
func syntaxError() {
	fmt.Fprintf(os.Stderr, `Sync files and directories to and from swift

Syntax: [options] subcommand <parameters> <parameters...>

Subcommands:

`)
	for i := range Commands {
		cmd := &Commands[i]
		fmt.Fprintf(os.Stderr, "    %s: %s\n\n", cmd.name, cmd.help)
	}

	fmt.Fprintf(os.Stderr, "Options:\n")
	flag.PrintDefaults()
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

func main() {
	flag.Usage = syntaxError
	flag.Parse()
	args := flag.Args()
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Setup profiling if desired
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			stats.Error()
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if len(args) < 1 {
		fatal("No command supplied\n")
	}

	cmd := strings.ToLower(args[0])
	args = args[1:]

	// Find the command doing a prefix match
	var found *Command
	for i := range Commands {
		command := &Commands[i]
		// exact command name found - use that
		if command.name == cmd {
			found = command
			break
		} else if strings.HasPrefix(command.name, cmd) {
			if found != nil {
				stats.Error()
				log.Fatalf("Not unique - matches multiple commands %q", cmd)
			}
			found = command
		}
	}
	if found == nil {
		stats.Error()
		log.Fatalf("Unknown command %q", cmd)
	}
	found.checkArgs(args)

	// Make source and destination fs
	var fdst, fsrc Fs
	var err error
	if len(args) >= 1 {
		fdst, err = NewFs(args[0])
		if err != nil {
			stats.Error()
			log.Fatal("Failed to create file system: ", err)
		}
	}
	if len(args) >= 2 {
		fsrc, err = NewFs(args[1])
		if err != nil {
			stats.Error()
			log.Fatal("Failed to create destination file system: ", err)
		}
		fsrc, fdst = fdst, fsrc
	}

	// Work out modify window
	if fsrc != nil {
		precision := fsrc.Precision()
		log.Printf("Source precision %s\n", precision)
		if precision > *modifyWindow {
			*modifyWindow = precision
		}
	}
	if fdst != nil {
		precision := fdst.Precision()
		log.Printf("Destination precision %s\n", precision)
		if precision > *modifyWindow {
			*modifyWindow = precision
		}
	}
	log.Printf("Modify window is %s\n", *modifyWindow)

	// Print the stats every statsInterval
	go func() {
		ch := time.Tick(*statsInterval)
		for {
			<-ch
			stats.Log()
		}
	}()

	// Run the actual command
	if found.run != nil {
		found.run(fdst, fsrc)
		fmt.Println(stats)
		log.Printf("*** Go routines at exit %d\n", runtime.NumGoroutine())
		if stats.errors > 0 {
			os.Exit(1)
		}
		os.Exit(0)
	} else {
		syntaxError()
	}

}
