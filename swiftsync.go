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
)

// Globals
var (
	// Flags
	cpuprofile = flag.String("cpuprofile", "", "Write cpu profile to file")
	snet       = flag.Bool("snet", false, "Use internal service network") // FIXME not implemented
	verbose    = flag.Bool("verbose", false, "Print lots more stuff")
	quiet      = flag.Bool("quiet", false, "Print as little stuff as possible")
	checkers   = flag.Int("checkers", 8, "Number of checkers to run in parallel.")
	transfers  = flag.Int("transfers", 4, "Number of file transfers to run in parallel.")
)

// Read FsObjects~s on in send to out if they need uploading
//
// FIXME potentially doing lots of MD5SUMS at once
func Checker(in, out FsObjectsChan, fdst Fs, wg *sync.WaitGroup) {
	defer wg.Done()
	for src := range in {
		dst := fdst.NewFsObject(src.Remote())
		if dst == nil {
			src.Debugf("Couldn't find local file - download")
			out <- src
			continue
		}

		// Check to see if can store this
		if !src.Storable() {
			continue
		}
		// Check to see if changed or not
		if Equal(src, dst) {
			src.Debugf("Unchanged skipping")
			continue
		}
		out <- src
	}
}

// Read FsObjects on in and copy them
func Copier(in FsObjectsChan, fdst Fs, wg *sync.WaitGroup) {
	defer wg.Done()
	for src := range in {
		fdst.Put(src)
	}
}

// Copies fsrc into fdst
func Copy(fdst, fsrc Fs) {
	err := fdst.Mkdir()
	if err != nil {
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

// Copy~s from source to dest
func copy_(fdst, fsrc Fs) {
	Copy(fdst, fsrc)
}

// List the Fs to stdout
func List(f Fs) {
	// FIXME error?
	in := f.List()
	for fs := range in {
		// FIXME
		//		if object.PseudoDirectory {
		//			fmt.Printf("%9s %19s %s\n", "Directory", "-", fs.Remote())
		//		} else {
		// FIXME ModTime is expensive?
		modTime, _ := fs.ModTime()
		fmt.Printf("%9d %19s %s\n", fs.Size(), modTime.Format("2006-01-02 15:04:05"), fs.Remote())
		//			fmt.Printf("%9d %19s %s\n", fs.Size(), object.LastModified.Format("2006-01-02 15:04:05"), fs.Remote())
		//		}
	}
}

// Lists files in a container
func list(fdst, fsrc Fs) {
	if fdst == nil {
		SwiftContainers()
		return
	}
	List(fdst)
}

// Makes a destination directory or container
func mkdir(fdst, fsrc Fs) {
	err := fdst.Mkdir()
	if err != nil {
		log.Fatalf("Mkdir failed: %s", err)
	}
}

// Removes a container but not if not empty
func rmdir(fdst, fsrc Fs) {
	err := fdst.Rmdir()
	if err != nil {
		log.Fatalf("Rmdir failed: %s", err)
	}
}

// Removes a container and all of its contents
//
// FIXME doesn't delete local directories
func purge(fdst, fsrc Fs) {
	to_be_deleted := fdst.List()

	var wg sync.WaitGroup
	wg.Add(*transfers)
	for i := 0; i < *transfers; i++ {
		go func() {
			defer wg.Done()
			for dst := range to_be_deleted {
				err := dst.Remove()
				if err != nil {
					log.Printf("%s: Couldn't delete: %s\n", dst.Remote(), err)
				} else {
					log.Printf("%s: Deleted\n", dst.Remote())
				}
			}
		}()
	}

	log.Printf("Waiting for deletions to finish")
	wg.Wait()

	log.Printf("Deleting path")
	rmdir(fdst, fsrc)
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
		copy_,
		2, 2,
	},
	{
		"ls",
		`[<path>]

        List the path. If no parameter is supplied then it lists the
        available swift containers.

`,
		list,
		0, 1,
	},
	{
		"mkdir",
		`<path>

        Make the path if it doesn't already exist

`,
		mkdir,
		1, 1,
	},
	{
		"rmdir",
		`<path>

        Remove the path.  Note that you can't remove a path with
	objects in it, use purge for that

`,
		rmdir,
		1, 1,
	},
	{
		"purge",
		`<path>

        Remove the path and all of its contents.

`,
		purge,
		1, 1,
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
		fmt.Fprintf(os.Stderr, "    %s: %s\n", cmd.name, cmd.help)
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
				log.Fatalf("Not unique - matches multiple commands %q", cmd)
			}
			found = command
		}
	}
	if found == nil {
		log.Fatalf("Unknown command %q", cmd)
	}
	found.checkArgs(args)

	// Make source and destination fs
	var fdst, fsrc Fs
	var err error
	if len(args) >= 1 {
		fdst, err = NewFs(args[0])
		if err != nil {
			log.Fatal("Failed to create file system: ", err)
		}
	}
	if len(args) >= 2 {
		fsrc, err = NewFs(args[1])
		if err != nil {
			log.Fatal("Failed to create destination file system: ", err)
		}
		fsrc, fdst = fdst, fsrc
	}

	// Run the actual command
	found.run(fdst, fsrc)
}
