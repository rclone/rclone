// Sync files and directories to and from swift
// 
// Nick Craig-Wood <nick@craig-wood.com>
package main

import (
	"flag"
	"fmt"
	"github.com/ncw/swift"
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
	// FIXME make these part of swift so we get a standard set of flags?
	authUrl   = flag.String("auth", os.Getenv("ST_AUTH"), "Auth URL for server. Defaults to environment var ST_AUTH.")
	userName  = flag.String("user", os.Getenv("ST_USER"), "User name. Defaults to environment var ST_USER.")
	apiKey    = flag.String("key", os.Getenv("ST_KEY"), "API key (password). Defaults to environment var ST_KEY.")
	checkers  = flag.Int("checkers", 8, "Number of checkers to run in parallel.")
	transfers = flag.Int("transfers", 4, "Number of file transfers to run in parallel.")
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
func Copy(fsrc, fdst Fs) {
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

// Syncs a directory into a container
func upload(c *swift.Connection, args []string) {
	root, container := args[0], args[1]
	// FIXME
	fsrc := &FsLocal{root: root}
	fdst := &FsSwift{c: *c, container: container}

	Copy(fsrc, fdst)
}

// Syncs a container into a directory
//
// FIXME need optional stat in FsObject and to be able to make FsObjects from ObjectsAll
//
// FIXME should download and stat many at once
func download(c *swift.Connection, args []string) {
	container, root := args[0], args[1]

	// FIXME
	fsrc := &FsSwift{c: *c, container: container}
	fdst := &FsLocal{root: root}

	Copy(fsrc, fdst)
}

// Lists the containers
func listContainers(c *swift.Connection) {
	containers, err := c.ContainersAll(nil)
	if err != nil {
		log.Fatalf("Couldn't list containers: %s", err)
	}
	for _, container := range containers {
		fmt.Printf("%9d %12d %s\n", container.Count, container.Bytes, container.Name)
	}
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
func list(c *swift.Connection, args []string) {
	if len(args) == 0 {
		listContainers(c)
		return
	}
	container := args[0]
	// FIXME
	f := &FsSwift{c: *c, container: container}
	List(f)
}

// Local lists files
func llist(c *swift.Connection, args []string) {
	root := args[0]
	// FIXME
	f := &FsLocal{root: root}
	List(f)
}

// Makes a container
func mkdir(c *swift.Connection, args []string) {
	container := args[0]
	// FIXME
	fdst := &FsSwift{c: *c, container: container}

	err := fdst.Mkdir()
	if err != nil {
		log.Fatalf("Couldn't create container %q: %s", container, err)
	}
}

// Removes a container
func rmdir(c *swift.Connection, args []string) {
	container := args[0]
	// FIXME
	fdst := &FsSwift{c: *c, container: container}

	err := fdst.Rmdir()
	if err != nil {
		log.Fatalf("Couldn't delete container %q: %s", container, err)
	}
}

// Removes a container and all of its contents
//
// FIXME should make FsObjects and use debugging
func purge(c *swift.Connection, args []string) {
	container := args[0]
	// FIXME
	fdst := &FsSwift{c: *c, container: container}

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

	log.Printf("Deleting container")
	rmdir(c, args)
}

type Command struct {
	name             string
	help             string
	run              func(*swift.Connection, []string)
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
		"upload",
		`<directory> <container>
        Upload the local directory to the remote container.  Doesn't
        upload unchanged files, testing first by modification time
        then by MD5SUM
`,
		upload,
		2, 2,
	},
	{
		"download",
		`<container> <directory> 
        Download the container to the local directory.  Doesn't
        download unchanged files
`,
		download,
		2, 2,
	},
	{
		"ls",
		`[<container>]
        List the containers if no parameter supplied or the contents
        of <container>
`,
		list,
		0, 1,
	},
	{
		"lls",
		`[<directory>]
        List the directory
`,
		llist,
		1, 1,
	},
	{
		"mkdir",
		`<container>
        Make the container if it doesn't already exist
`,
		mkdir,
		1, 1,
	},
	{
		"rmdir",
		`<container>
        Remove the container.  Note that you can't remove a container with
        objects in - use rm for that
`,
		rmdir,
		1, 1,
	},
	{
		"purge",
		`<container>
        Remove the container and all of the contents.
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

	fmt.Println(args)
	if len(args) < 1 {
		fatal("No command supplied\n")
	}

	if *userName == "" {
		log.Fatal("Need -user or environmental variable ST_USER")
	}
	if *apiKey == "" {
		log.Fatal("Need -key or environmental variable ST_KEY")
	}
	if *authUrl == "" {
		log.Fatal("Need -auth or environmental variable ST_AUTH")
	}
	c := &swift.Connection{
		UserName: *userName,
		ApiKey:   *apiKey,
		AuthUrl:  *authUrl,
	}
	err := c.Authenticate()
	if err != nil {
		log.Fatal("Failed to authenticate", err)
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
	found.run(c, args)
}
