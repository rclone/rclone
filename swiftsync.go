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
)

// Globals
var (
	// Flags
	//fileSize      = flag.Int64("s", 1E9, "Size of the check files")
	cpuprofile = flag.String("cpuprofile", "", "Write cpu profile to file")
	//duration      = flag.Duration("duration", time.Hour*24, "Duration to run test")
	//statsInterval = flag.Duration("stats", time.Minute*1, "Interval to print stats")
	//logfile       = flag.String("logfile", "stressdisk.log", "File to write log to set to empty to ignore")

	snet    = flag.Bool("snet", false, "Use internal service network") // FIXME not implemented
	verbose = flag.Bool("verbose", false, "Print lots more stuff")
	quiet   = flag.Bool("quiet", false, "Print as little stuff as possible")
	// FIXME make these part of swift so we get a standard set of flags?
	authUrl   = flag.String("auth", os.Getenv("ST_AUTH"), "Auth URL for server. Defaults to environment var ST_AUTH.")
	userName  = flag.String("user", os.Getenv("ST_USER"), "User name. Defaults to environment var ST_USER.")
	apiKey    = flag.String("key", os.Getenv("ST_KEY"), "API key (password). Defaults to environment var ST_KEY.")
	checkers  = flag.Int("checkers", 8, "Number of checkers to run in parallel.")
	uploaders = flag.Int("uploaders", 4, "Number of uploaders to run in parallel.")
)

// Make this an interface?
//
// Have an implementation for SwiftObjects and FsObjects?
//
// This represents an object in the filesystem
type FsObject struct {
	rel  string
	path string
	info os.FileInfo
}

type FsObjectsChan chan *FsObject

type FsObjects []FsObject

// Write debuging output for this FsObject
func (fs *FsObject) Debugf(text string, args ...interface{}) {
	out := fmt.Sprintf(text, args...)
	log.Printf("%s: %s", fs.rel, out)
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

// Checks to see if an object has changed or not by looking at its size, mtime and MD5SUM
//
// If the remote object size is different then it is considered to be
// changed.
//
// If the size is the same and the mtime is the same then it is
// considered to be unchanged.  This is the heuristic rsync uses when
// not using --checksum.
//
// If the size is the same and and mtime is different or unreadable
// and the MD5SUM is the same then the file is considered to be
// unchanged.  In this case the mtime on the object is updated.
//
// Otherwise the file is considered to be changed.
func (fs *FsObject) changed(c *swift.Connection, container string) bool {
	obj, h, err := c.Object(container, fs.rel)
	if err != nil {
		fs.Debugf("Failed to read info: %s", err)
		return true
	}
	if obj.Bytes != fs.info.Size() {
		fs.Debugf("Sizes differ")
		return true
	}

	// Size the same so check the mtime
	m := h.ObjectMetadata()
	t, err := m.GetModTime()
	if err != nil {
		fs.Debugf("Failed to read mtime: %s", err)
	} else if !t.Equal(fs.info.ModTime()) {
		fs.Debugf("Modification times differ")
	} else {
		fs.Debugf("Size and modification time the same - skipping")
		return false
	}

	// mtime is unreadable or different but size is the same so
	// check the MD5SUM
	localMd5, err := fs.md5sum()
	if err != nil {
		fs.Debugf("Failed to calculate md5: %s", err)
		return true
	}
	// fs.Debugf("Local  MD5 %s", localMd5)
	// fs.Debugf("Remote MD5 %s", obj.Hash)
	if localMd5 != strings.ToLower(obj.Hash) {
		fs.Debugf("Md5sums differ")
		return true
	}

	// Update the mtime of the remote object here
	m.SetModTime(fs.info.ModTime())
	err = c.ObjectUpdate(container, fs.rel, m.ObjectHeaders())
	if err != nil {
		fs.Debugf("Failed to update mtime: %s", err)
	}
	fs.Debugf("Updated mtime")

	fs.Debugf("Size and MD5SUM identical - skipping")
	return false
}

// Is this object storable
func (fs *FsObject) storable(c *swift.Connection, container string) bool {
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
	_, err = c.ObjectPut(container, fs.rel, in, true, "", "", m.ObjectHeaders())
	if err != nil {
		fs.Debugf("Failed to upload: %s", err)
		return
	}
	fs.Debugf("Uploaded")
}

// Return an FsObject from a path
//
// May return nil if an error occurred
func NewFsObject(root, path string) *FsObject {
	info, err := os.Lstat(path)
	if err != nil {
		log.Printf("Failed to stat %s: %s", path, err)
		return nil
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		log.Printf("Failed to get relative path %s: %s", path, err)
		return nil
	}
	if rel == "." {
		rel = ""
	}
	return &FsObject{rel: rel, path: path, info: info}
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

// syntaxError prints the syntax
func syntaxError() {
	fmt.Fprintf(os.Stderr, `Sync files and directores to and from swift

FIXME

Full options:
`)
	flag.PrintDefaults()
}

// Exit with the message
func fatal(message string, args ...interface{}) {
	syntaxError()
	fmt.Fprintf(os.Stderr, message, args...)
	os.Exit(1)
}

// checkArgs checks there are enough arguments and prints a message if not
func checkArgs(args []string, n int, message string) {
	if len(args) != n {
		syntaxError()
		fmt.Fprintf(os.Stderr, "%d arguments required: %s\n", n, message)
		os.Exit(1)
	}
}

// Read FsObjects on in and write them to out if they need uploading
//
// FIXME potentially doing lots of MD5SUMS at once
func checker(c *swift.Connection, container string, in, out FsObjectsChan, wg *sync.WaitGroup) {
	defer wg.Done()
	for fs := range in {
		// Check to see if can store this
		if !fs.storable(c, container) {
			continue
		}
		// Check to see if changed or not
		if !fs.changed(c, container) {
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
func upload(c *swift.Connection, root, container string) {
	to_be_checked := walk(root)
	to_be_uploaded := make(FsObjectsChan, *uploaders)

	var checkerWg sync.WaitGroup
	checkerWg.Add(*checkers)
	for i := 0; i < *checkers; i++ {
		go checker(c, container, to_be_checked, to_be_uploaded, &checkerWg)
	}

	var uploaderWg sync.WaitGroup
	uploaderWg.Add(*uploaders)
	for i := 0; i < *uploaders; i++ {
		go uploader(c, container, to_be_uploaded, &uploaderWg)
	}

	log.Printf("Waiting for checks to finish")
	checkerWg.Wait()
	close(to_be_uploaded)
	log.Printf("Waiting for uploads to finish")
	uploaderWg.Wait()
}

// FIXME should be an FsOject method
//
// Get an object to the filepath making directories if necessary
func get(c *swift.Connection, container, name, filepath string) {
	log.Printf("Download %s to %s", name, filepath)

	dir := path.Dir(filepath)
	err := os.MkdirAll(dir, 0770)
	if err != nil {
		log.Printf("Couldn't make directory %q: %s", dir, err)
		return
	}

	out, err := os.Create(filepath)
	if err != nil {
		log.Printf("Failed to open %q: %s", filepath, err)
		return
	}

	h, getErr := c.ObjectGet(container, name, out, true, nil)
	if getErr != nil {
		log.Printf("Failed to download %q: %s", name, getErr)
	}

	closeErr := out.Close()
	if closeErr != nil {
		log.Printf("Error closing %q: %s", filepath, closeErr)
	}

	if getErr != nil || closeErr != nil {
		log.Printf("Removing failed download %q", filepath)
		err = os.Remove(filepath)
		if err != nil {
			log.Printf("Failed to remove %q: %s", filepath, err)
		}
		return
	}

	// Set the mtime
	// FIXME should be a FsObject method?
	modTime, err := h.ObjectMetadata().GetModTime()
	if err != nil {
		log.Printf("Failed to read mtime from object: %s", err)
	} else {
		err = os.Chtimes(filepath, modTime, modTime)
		if err != nil {
			log.Printf("Failed to set mtime on file: %s", err)
		}
	}
}

// Syncs a container into a directory
//
// FIXME don't want to update the modification times on the
// remote server if they are different - want to modify the local
// file!
//
// FIXME need optional stat in FsObject and to be able to make FsObjects from ObjectsAll
func download(c *swift.Connection, container, root string) {
	// FIXME this would be nice running into a channel!
	objects, err := c.ObjectsAll(container, nil)
	if err != nil {
		log.Fatalf("Couldn't read container %q: %s", container, err)
	}

	err = os.MkdirAll(root, 0770)
	if err != nil {
		log.Fatalf("Couldn't make directory %q: %s", root, err)
	}

	for i := range objects {
		object := &objects[i]
		filepath := path.Join(root, object.Name)
		// fs := NewFsObject(root, filepath)
		// if fs == nil {
		// 	log.Printf("%s: Download: not found", object.Name)
		// } else if !fs.storable(c, container) {
		// 	fs.Debugf("Skip: not storable")
		// 	continue
		// } else if !fs.changed(c, container) {
		// 	fs.Debugf("Skip: not changed")
		// 	continue
		// }
		get(c, container, object.Name, filepath)
	}
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
func list(c *swift.Connection, container string) {
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
func mkdir(c *swift.Connection, container string) {
	err := c.ContainerCreate(container, nil)
	if err != nil {
		log.Fatalf("Couldn't create container %q: %s", container, err)
	}
}

// Removes a container
func rmdir(c *swift.Connection, container string) {
	err := c.ContainerDelete(container)
	if err != nil {
		log.Fatalf("Couldn't delete container %q: %s", container, err)
	}
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

	command := args[0]
	args = args[1:]

	switch command {
	case "up", "upload":
		checkArgs(args, 2, "Need directory to read from and container to write to")
		upload(c, args[0], args[1])
	case "down", "download":
		checkArgs(args, 2, "Need container to read from and directory to write to")
		download(c, args[0], args[1])
	case "list", "ls":
		if len(args) == 0 {
			listContainers(c)
		} else {
			checkArgs(args, 1, "Need container to list")
			list(c, args[0])
		}
	case "mkdir":
		checkArgs(args, 1, "Need container to make")
		mkdir(c, args[0])
	case "rmdir":
		checkArgs(args, 1, "Need container to delte")
		rmdir(c, args[0])
	default:
		log.Fatalf("Unknown command %q", command)
	}

}
