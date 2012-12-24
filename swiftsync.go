// Sync files and directories to and from swift
// 
// Nick Craig-Wood <nick@craig-wood.com>
package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"github.com/ncw/swift"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"time"
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

// A filesystem like object which can either be a remote object or a
// local file/directory or both
type FsObject struct {
	remote string      // The remote path
	path   string      // The local path
	info   os.FileInfo // Interface for file info
}

type FsObjectsChan chan *FsObject

type FsObjects []FsObject

// Write debuging output for this FsObject
func (fs *FsObject) Debugf(text string, args ...interface{}) {
	out := fmt.Sprintf(text, args...)
	log.Printf("%s: %s", fs.remote, out)
}

// md5sum calculates the md5sum of a file returning a lowercase hex string
func (fs *FsObject) md5sum() (string, error) {
	in, err := os.Open(fs.path)
	if err != nil {
		fs.Debugf("Failed to open: %s", err)
		return "", err
	}
	defer in.Close() // FIXME ignoring error
	hash := md5.New()
	_, err = io.Copy(hash, in)
	if err != nil {
		fs.Debugf("Failed to read: %s", err)
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// Sets the modification time of the local fs object
func (fs *FsObject) SetModTime(modTime time.Time) {
	err := Chtimes(fs.path, modTime, modTime)
	if err != nil {
		fs.Debugf("Failed to set mtime on file: %s", err)
	}
}

// Checks to see if the remote and local objects are equal by looking
// at size, mtime and MD5SUM
//
// If the remote and local size are different then it is considered to
// be not equal.
//
// If the size is the same and the mtime is the same then it is
// considered to be equal.  This is the heuristic rsync uses when
// not using --checksum.
//
// If the size is the same and and mtime is different or unreadable
// and the MD5SUM is the same then the file is considered to be
// equal.  In this case the mtime on the object is updated. If
// upload is set then the remote object is changed otherwise the local
// object.
//
// Otherwise the file is considered to be not equal including if there
// were errors reading info.
func (fs *FsObject) Equal(c *swift.Connection, container string, upload bool) bool {
	// FIXME could pass in an Object here if we have one which
	// will mean we could do the Size and Hash checks without a
	// remote call if we wanted
	obj, h, err := c.Object(container, fs.remote)
	if err != nil {
		fs.Debugf("Failed to read info: %s", err)
		return false
	}
	if obj.Bytes != fs.info.Size() {
		fs.Debugf("Sizes differ")
		return false
	}

	// Size the same so check the mtime
	m := h.ObjectMetadata()
	remoteModTime, err := m.GetModTime()
	if err != nil {
		fs.Debugf("Failed to read mtime: %s", err)
	} else if !remoteModTime.Equal(fs.info.ModTime()) {
		fs.Debugf("Modification times differ")
	} else {
		fs.Debugf("Size and modification time the same")
		return true
	}

	// mtime is unreadable or different but size is the same so
	// check the MD5SUM
	localMd5, err := fs.md5sum()
	if err != nil {
		fs.Debugf("Failed to calculate md5: %s", err)
		return false
	}
	// fs.Debugf("Local  MD5 %s", localMd5)
	// fs.Debugf("Remote MD5 %s", obj.Hash)
	if localMd5 != strings.ToLower(obj.Hash) {
		fs.Debugf("Md5sums differ")
		return false
	}

	// Size and MD5 the same but mtime different so update the
	// mtime of the local or remote object here
	if upload {
		m.SetModTime(fs.info.ModTime())
		err = c.ObjectUpdate(container, fs.remote, m.ObjectHeaders())
		if err != nil {
			fs.Debugf("Failed to update remote mtime: %s", err)
		}
		fs.Debugf("Updated mtime of remote object")
	} else {
		fmt.Printf("metadata %q, remoteModTime = %s\n", m, remoteModTime)
		fs.SetModTime(remoteModTime)
		fs.Debugf("Updated mtime of local object")
	}

	fs.Debugf("Size and MD5SUM of local and remote objects identical")
	return true
}

// Is this object storable
func (fs *FsObject) storable() bool {
	mode := fs.info.Mode()
	if mode&(os.ModeSymlink|os.ModeNamedPipe|os.ModeSocket|os.ModeDevice) != 0 {
		fs.Debugf("Can't transfer non file/directory")
		return false
	} else if mode&os.ModeDir != 0 {
		// Debug?
		fs.Debugf("FIXME Skipping directory")
		return false
	}
	return true
}

// Puts the FsObject into the container
func (fs *FsObject) put(c *swift.Connection, container string) {
	// FIXME content type
	in, err := os.Open(fs.path)
	if err != nil {
		fs.Debugf("Failed to open: %s", err)
		return
	}
	defer in.Close()
	m := swift.Metadata{}
	m.SetModTime(fs.info.ModTime())
	_, err = c.ObjectPut(container, fs.remote, in, true, "", "", m.ObjectHeaders())
	if err != nil {
		fs.Debugf("Failed to upload: %s", err)
		return
	}
	fs.Debugf("Uploaded")
}

// Stat a FsObject into info
func (fs *FsObject) lstat() error {
	info, err := os.Lstat(fs.path)
	fs.info = info
	return err
}

// Return an FsObject from a path
//
// May return nil if an error occurred
func NewFsObject(root, path string) *FsObject {
	remote, err := filepath.Rel(root, path)
	if err != nil {
		log.Printf("Failed to get relative path %s: %s", path, err)
		return nil
	}
	if remote == "." {
		remote = ""
	}
	fs := &FsObject{remote: remote, path: path}
	err = fs.lstat()
	if err != nil {
		log.Printf("Failed to stat %s: %s", path, err)
		return nil
	}
	return fs
}

// Walk the path returning a channel of FsObjects
//
// FIXME ignore symlinks?
// FIXME what about hardlinks / etc
func walk(root string) FsObjectsChan {
	out := make(FsObjectsChan, *checkers)
	go func() {
		err := filepath.Walk(root, func(path string, f os.FileInfo, err error) error {
			if err != nil {
				log.Printf("Failed to open directory: %s: %s", path, err)
			} else {
				if fs := NewFsObject(root, path); fs != nil {
					out <- fs
				}
			}
			return nil
		})
		if err != nil {
			log.Printf("Failed to open directory: %s: %s", root, err)
		}
		close(out)
	}()
	return out
}

// Read FsObjects on in and write them to out if they need uploading
//
// FIXME potentially doing lots of MD5SUMS at once
func checker(c *swift.Connection, container string, in, out FsObjectsChan, upload bool, wg *sync.WaitGroup) {
	defer wg.Done()
	for fs := range in {
		if !upload {
			_ = fs.lstat()
			if fs.info == nil {
				fs.Debugf("Couldn't find local file - download")
				out <- fs
				continue
			}
		}

		// Check to see if can store this
		if !fs.storable() {
			continue
		}
		// Check to see if changed or not
		if fs.Equal(c, container, upload) {
			fs.Debugf("Unchanged skipping")
			continue
		}
		out <- fs
	}
}

// Read FsObjects on in and upload them
func uploader(c *swift.Connection, container string, in FsObjectsChan, wg *sync.WaitGroup) {
	defer wg.Done()
	for fs := range in {
		fs.put(c, container)
	}
}

// Syncs a directory into a container
func upload(c *swift.Connection, args []string) {
	root, container := args[0], args[1]
	mkdir(c, []string{container})
	to_be_checked := walk(root)
	to_be_uploaded := make(FsObjectsChan, *transfers)

	var checkerWg sync.WaitGroup
	checkerWg.Add(*checkers)
	for i := 0; i < *checkers; i++ {
		go checker(c, container, to_be_checked, to_be_uploaded, true, &checkerWg)
	}

	var uploaderWg sync.WaitGroup
	uploaderWg.Add(*transfers)
	for i := 0; i < *transfers; i++ {
		go uploader(c, container, to_be_uploaded, &uploaderWg)
	}

	log.Printf("Waiting for checks to finish")
	checkerWg.Wait()
	close(to_be_uploaded)
	log.Printf("Waiting for uploads to finish")
	uploaderWg.Wait()
}

// Get an object to the filepath making directories if necessary
func (fs *FsObject) get(c *swift.Connection, container string) {
	log.Printf("Download %s to %s", fs.remote, fs.path)

	dir := path.Dir(fs.path)
	err := os.MkdirAll(dir, 0770)
	if err != nil {
		fs.Debugf("Couldn't make directory: %s", err)
		return
	}

	out, err := os.Create(fs.path)
	if err != nil {
		fs.Debugf("Failed to open: %s", err)
		return
	}

	h, getErr := c.ObjectGet(container, fs.remote, out, true, nil)
	if getErr != nil {
		fs.Debugf("Failed to download: %s", getErr)
	}

	closeErr := out.Close()
	if closeErr != nil {
		fs.Debugf("Error closing: %s", closeErr)
	}

	if getErr != nil || closeErr != nil {
		fs.Debugf("Removing failed download")
		err = os.Remove(fs.path)
		if err != nil {
			fs.Debugf("Failed to remove failed download: %s", err)
		}
		return
	}

	// Set the mtime
	modTime, err := h.ObjectMetadata().GetModTime()
	if err != nil {
		fs.Debugf("Failed to read mtime from object: %s", err)
	} else {
		fs.SetModTime(modTime)
	}
}

// Read FsObjects on in and download them
func downloader(c *swift.Connection, container string, in FsObjectsChan, wg *sync.WaitGroup) {
	defer wg.Done()
	for fs := range in {
		fs.get(c, container)
	}
}

// Syncs a container into a directory
//
// FIXME need optional stat in FsObject and to be able to make FsObjects from ObjectsAll
//
// FIXME should download and stat many at once
func download(c *swift.Connection, args []string) {
	container, root := args[0], args[1]
	// FIXME this would be nice running into a channel!
	objects, err := c.ObjectsAll(container, nil)
	if err != nil {
		log.Fatalf("Couldn't read container %q: %s", container, err)
	}

	err = os.MkdirAll(root, 0770)
	if err != nil {
		log.Fatalf("Couldn't make directory %q: %s", root, err)
	}

	to_be_checked := make(FsObjectsChan, *checkers)
	to_be_downloaded := make(FsObjectsChan, *transfers)

	var checkerWg sync.WaitGroup
	checkerWg.Add(*checkers)
	for i := 0; i < *checkers; i++ {
		go checker(c, container, to_be_checked, to_be_downloaded, false, &checkerWg)
	}

	var downloaderWg sync.WaitGroup
	downloaderWg.Add(*transfers)
	for i := 0; i < *transfers; i++ {
		go downloader(c, container, to_be_downloaded, &downloaderWg)
	}

	for i := range objects {
		object := &objects[i]
		filepath := path.Join(root, object.Name)
		to_be_checked <- &FsObject{remote: object.Name, path: filepath}
	}
	close(to_be_checked)

	log.Printf("Waiting for checks to finish")
	checkerWg.Wait()
	close(to_be_downloaded)
	log.Printf("Waiting for downloads to finish")
	downloaderWg.Wait()

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

// Lists files in a container
func list(c *swift.Connection, args []string) {
	if len(args) == 0 {
		listContainers(c)
		return
	}
	container := args[0]
	//objects, err := c.ObjectsAll(container, &swift.ObjectsOpts{Prefix: "", Delimiter: '/'})
	objects, err := c.ObjectsAll(container, nil)
	if err != nil {
		log.Fatalf("Couldn't read container %q: %s", container, err)
	}
	for _, object := range objects {
		if object.PseudoDirectory {
			fmt.Printf("%9s %19s %s\n", "Directory", "-", object.Name)
		} else {
			fmt.Printf("%9d %19s %s\n", object.Bytes, object.LastModified.Format("2006-01-02 15:04:05"), object.Name)
		}
	}
}

// Makes a container
func mkdir(c *swift.Connection, args []string) {
	container := args[0]
	err := c.ContainerCreate(container, nil)
	if err != nil {
		log.Fatalf("Couldn't create container %q: %s", container, err)
	}
}

// Removes a container
func rmdir(c *swift.Connection, args []string) {
	container := args[0]
	err := c.ContainerDelete(container)
	if err != nil {
		log.Fatalf("Couldn't delete container %q: %s", container, err)
	}
}

// Removes a container and all of its contents
//
// FIXME should make FsObjects and use debugging
func purge(c *swift.Connection, args []string) {
	container := args[0]
	objects, err := c.ObjectsAll(container, nil)
	if err != nil {
		log.Fatalf("Couldn't read container %q: %s", container, err)
	}

	to_be_deleted := make(chan *swift.Object, *transfers)

	var wg sync.WaitGroup
	wg.Add(*transfers)
	for i := 0; i < *transfers; i++ {
		go func() {
			defer wg.Done()
			for object := range to_be_deleted {
				err := c.ObjectDelete(container, object.Name)
				if err != nil {
					log.Printf("%s: Couldn't delete: %s\n", object.Name, err)
				} else {
					log.Printf("%s: Deleted\n", object.Name)
				}
			}
		}()
	}

	for i := range objects {
		to_be_deleted <- &objects[i]
	}
	close(to_be_deleted)

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
