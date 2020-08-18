// Package mega provides an interface to the Mega
// object storage system.
package mega

/*
Open questions
* Does mega support a content hash - what exactly are the mega hashes?
* Can mega support setting modification times?

Improvements:
* Uploads could be done in parallel
* Downloads would be more efficient done in one go
* Uploads would be more efficient with bigger chunks
* Looks like mega can support server side copy, but it isn't implemented in go-mega
* Upload can set modtime... - set as int64_t - can set ctime and mtime?
*/

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/readers"
	mega "github.com/t3rm1n4l/go-mega"
)

const (
	minSleep      = 10 * time.Millisecond
	maxSleep      = 2 * time.Second
	eventWaitTime = 500 * time.Millisecond
	decayConstant = 2 // bigger for slower decay, exponential
)

var (
	megaCacheMu sync.Mutex                // mutex for the below
	megaCache   = map[string]*mega.Mega{} // cache logged in Mega's by user
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "mega",
		Description: "Mega",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "user",
			Help:     "User name",
			Required: true,
		}, {
			Name:       "pass",
			Help:       "Password.",
			Required:   true,
			IsPassword: true,
		}, {
			Name: "debug",
			Help: `Output more debug from Mega.

If this flag is set (along with -vv) it will print further debugging
information from the mega backend.`,
			Default:  false,
			Advanced: true,
		}, {
			Name: "hard_delete",
			Help: `Delete files permanently rather than putting them into the trash.

Normally the mega backend will put all deletions into the trash rather
than permanently deleting them.  If you specify this then rclone will
permanently delete objects instead.`,
			Default:  false,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// Encode invalid UTF-8 bytes as json doesn't handle them properly.
			Default: (encoder.Base |
				encoder.EncodeInvalidUtf8),
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	User       string               `config:"user"`
	Pass       string               `config:"pass"`
	Debug      bool                 `config:"debug"`
	HardDelete bool                 `config:"hard_delete"`
	Enc        encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote mega
type Fs struct {
	name       string       // name of this remote
	root       string       // the path we are working on
	opt        Options      // parsed config options
	features   *fs.Features // optional features
	srv        *mega.Mega   // the connection to the server
	pacer      *fs.Pacer    // pacer for API calls
	rootNodeMu sync.Mutex   // mutex for _rootNode
	_rootNode  *mega.Node   // root node - call findRoot to use this
	mkdirMu    sync.Mutex   // used to serialize calls to mkdir / rmdir
}

// Object describes a mega object
//
// Will definitely have info but maybe not meta
//
// Normally rclone would just store an ID here but go-mega and mega.nz
// expect you to build an entire tree of all the objects in memory.
// In this case we just store a pointer to the object.
type Object struct {
	fs     *Fs        // what this object is part of
	remote string     // The remote path
	info   *mega.Node // pointer to the mega node
}

// ------------------------------------------------------------

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("mega root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// parsePath parses a mega 'url'
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
}

// shouldRetry returns a boolean as to whether this err deserves to be
// retried.  It returns the err as a convenience
func shouldRetry(err error) (bool, error) {
	// Let the mega library handle the low level retries
	return false, err
	/*
		switch errors.Cause(err) {
		case mega.EAGAIN, mega.ERATELIMIT, mega.ETEMPUNAVAIL:
			return true, err
		}
		return fserrors.ShouldRetry(err), err
	*/
}

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(remote string) (info *mega.Node, err error) {
	rootNode, err := f.findRoot(false)
	if err != nil {
		return nil, err
	}
	return f.findObject(rootNode, remote)
}

// NewFs constructs an Fs from the path, container:path
func NewFs(name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	if opt.Pass != "" {
		var err error
		opt.Pass, err = obscure.Reveal(opt.Pass)
		if err != nil {
			return nil, errors.Wrap(err, "couldn't decrypt password")
		}
	}

	// cache *mega.Mega on username so we can re-use and share
	// them between remotes.  They are expensive to make as they
	// contain all the objects and sharing the objects makes the
	// move code easier as we don't have to worry about mixing
	// them up between different remotes.
	megaCacheMu.Lock()
	defer megaCacheMu.Unlock()
	srv := megaCache[opt.User]
	if srv == nil {
		srv = mega.New().SetClient(fshttp.NewClient(fs.Config))
		srv.SetRetries(fs.Config.LowLevelRetries) // let mega do the low level retries
		srv.SetLogger(func(format string, v ...interface{}) {
			fs.Infof("*go-mega*", format, v...)
		})
		if opt.Debug {
			srv.SetDebugger(func(format string, v ...interface{}) {
				fs.Debugf("*go-mega*", format, v...)
			})
		}

		err := srv.Login(opt.User, opt.Pass)
		if err != nil {
			return nil, errors.Wrap(err, "couldn't login")
		}
		megaCache[opt.User] = srv
	}

	root = parsePath(root)
	f := &Fs{
		name:  name,
		root:  root,
		opt:   *opt,
		srv:   srv,
		pacer: fs.NewPacer(pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}
	f.features = (&fs.Features{
		DuplicateFiles:          true,
		CanHaveEmptyDirectories: true,
	}).Fill(f)

	// Find the root node and check if it is a file or not
	_, err = f.findRoot(false)
	switch err {
	case nil:
		// root node found and is a directory
	case fs.ErrorDirNotFound:
		// root node not found, so can't be a file
	case fs.ErrorIsFile:
		// root node is a file so point to parent directory
		root = path.Dir(root)
		if root == "." {
			root = ""
		}
		f.root = root
		return f, err
	}
	return f, nil
}

// splitNodePath splits nodePath into / separated parts, returning nil if it
// should refer to the root.
// It also encodes the parts into backend specific encoding
func (f *Fs) splitNodePath(nodePath string) (parts []string) {
	nodePath = path.Clean(nodePath)
	if nodePath == "." || nodePath == "/" {
		return nil
	}
	nodePath = f.opt.Enc.FromStandardPath(nodePath)
	return strings.Split(nodePath, "/")
}

// findNode looks up the node for the path of the name given from the root given
//
// It returns mega.ENOENT if it wasn't found
func (f *Fs) findNode(rootNode *mega.Node, nodePath string) (*mega.Node, error) {
	parts := f.splitNodePath(nodePath)
	if parts == nil {
		return rootNode, nil
	}
	nodes, err := f.srv.FS.PathLookup(rootNode, parts)
	if err != nil {
		return nil, err
	}
	return nodes[len(nodes)-1], nil
}

// findDir finds the directory rooted from the node passed in
func (f *Fs) findDir(rootNode *mega.Node, dir string) (node *mega.Node, err error) {
	node, err = f.findNode(rootNode, dir)
	if err == mega.ENOENT {
		return nil, fs.ErrorDirNotFound
	} else if err == nil && node.GetType() == mega.FILE {
		return nil, fs.ErrorIsFile
	}
	return node, err
}

// findObject looks up the node for the object of the name given
func (f *Fs) findObject(rootNode *mega.Node, file string) (node *mega.Node, err error) {
	node, err = f.findNode(rootNode, file)
	if err == mega.ENOENT {
		return nil, fs.ErrorObjectNotFound
	} else if err == nil && node.GetType() != mega.FILE {
		return nil, fs.ErrorNotAFile
	}
	return node, err
}

// lookupDir looks up the node for the directory of the name given
//
// if create is true it tries to create the root directory if not found
func (f *Fs) lookupDir(dir string) (*mega.Node, error) {
	rootNode, err := f.findRoot(false)
	if err != nil {
		return nil, err
	}
	return f.findDir(rootNode, dir)
}

// lookupParentDir finds the parent node for the remote passed in
func (f *Fs) lookupParentDir(remote string) (dirNode *mega.Node, leaf string, err error) {
	parent, leaf := path.Split(remote)
	dirNode, err = f.lookupDir(parent)
	return dirNode, leaf, err
}

// mkdir makes the directory and any parent directories for the
// directory of the name given
func (f *Fs) mkdir(rootNode *mega.Node, dir string) (node *mega.Node, err error) {
	f.mkdirMu.Lock()
	defer f.mkdirMu.Unlock()

	parts := f.splitNodePath(dir)
	if parts == nil {
		return rootNode, nil
	}
	var i int
	// look up until we find a directory which exists
	for i = 0; i <= len(parts); i++ {
		var nodes []*mega.Node
		nodes, err = f.srv.FS.PathLookup(rootNode, parts[:len(parts)-i])
		if err == nil {
			if len(nodes) == 0 {
				node = rootNode
			} else {
				node = nodes[len(nodes)-1]
			}
			break
		}
		if err != mega.ENOENT {
			return nil, errors.Wrap(err, "mkdir lookup failed")
		}
	}
	if err != nil {
		return nil, errors.Wrap(err, "internal error: mkdir called with non existent root node")
	}
	// i is number of directories to create (may be 0)
	// node is directory to create them from
	for _, name := range parts[len(parts)-i:] {
		// create directory called name in node
		err = f.pacer.Call(func() (bool, error) {
			node, err = f.srv.CreateDir(name, node)
			return shouldRetry(err)
		})
		if err != nil {
			return nil, errors.Wrap(err, "mkdir create node failed")
		}
	}
	return node, nil
}

// mkdirParent creates the parent directory of remote
func (f *Fs) mkdirParent(remote string) (dirNode *mega.Node, leaf string, err error) {
	rootNode, err := f.findRoot(true)
	if err != nil {
		return nil, "", err
	}
	parent, leaf := path.Split(remote)
	dirNode, err = f.mkdir(rootNode, parent)
	return dirNode, leaf, err
}

// findRoot looks up the root directory node and returns it.
//
// if create is true it tries to create the root directory if not found
func (f *Fs) findRoot(create bool) (*mega.Node, error) {
	f.rootNodeMu.Lock()
	defer f.rootNodeMu.Unlock()

	// Check if we haven't found it already
	if f._rootNode != nil {
		return f._rootNode, nil
	}

	// Check for pre-existing root
	absRoot := f.srv.FS.GetRoot()
	node, err := f.findDir(absRoot, f.root)
	//log.Printf("findRoot findDir %p %v", node, err)
	if err == nil {
		f._rootNode = node
		return node, nil
	}
	if !create || err != fs.ErrorDirNotFound {
		return nil, err
	}

	//..not found so create the root directory
	f._rootNode, err = f.mkdir(absRoot, f.root)
	return f._rootNode, err
}

// clearRoot unsets the root directory
func (f *Fs) clearRoot() {
	f.rootNodeMu.Lock()
	f._rootNode = nil
	f.rootNodeMu.Unlock()
	//log.Printf("cleared root directory")
}

// CleanUp deletes all files currently in trash
func (f *Fs) CleanUp(ctx context.Context) (err error) {
	trash := f.srv.FS.GetTrash()
	items := []*mega.Node{}
	_, err = f.list(ctx, trash, func(item *mega.Node) bool {
		items = append(items, item)
		return false
	})
	if err != nil {
		return errors.Wrap(err, "CleanUp failed to list items in trash")
	}
	fs.Infof(f, "Deleting %d items from the trash", len(items))
	errors := 0
	// similar to f.deleteNode(trash) but with HardDelete as true
	for _, item := range items {
		fs.Debugf(f, "Deleting trash %q", f.opt.Enc.ToStandardName(item.GetName()))
		deleteErr := f.pacer.Call(func() (bool, error) {
			err := f.srv.Delete(item, true)
			return shouldRetry(err)
		})
		if deleteErr != nil {
			err = deleteErr
			errors++
		}
	}
	fs.Infof(f, "Deleted %d items from the trash with %d errors", len(items), errors)
	return err
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(remote string, info *mega.Node) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	var err error
	if info != nil {
		// Set info
		err = o.setMetaData(info)
	} else {
		err = o.readMetaData() // reads info and meta, returning an error
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(remote, nil)
}

// list the objects into the function supplied
//
// If directories is set it only sends directories
// User function to process a File item from listAll
//
// Should return true to finish processing
type listFn func(*mega.Node) bool

// Lists the directory required calling the user function on each item found
//
// If the user fn ever returns true then it early exits with found = true
func (f *Fs) list(ctx context.Context, dir *mega.Node, fn listFn) (found bool, err error) {
	nodes, err := f.srv.FS.GetChildren(dir)
	if err != nil {
		return false, errors.Wrapf(err, "list failed")
	}
	for _, item := range nodes {
		if fn(item) {
			found = true
			break
		}
	}
	return
}

// List the objects and directories in dir into entries.  The
// entries can be returned in any order but should be for a
// complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	dirNode, err := f.lookupDir(dir)
	if err != nil {
		return nil, err
	}
	var iErr error
	_, err = f.list(ctx, dirNode, func(info *mega.Node) bool {
		remote := path.Join(dir, f.opt.Enc.ToStandardName(info.GetName()))
		switch info.GetType() {
		case mega.FOLDER, mega.ROOT, mega.INBOX, mega.TRASH:
			d := fs.NewDir(remote, info.GetTimeStamp()).SetID(info.GetHash())
			entries = append(entries, d)
		case mega.FILE:
			o, err := f.newObjectWithInfo(remote, info)
			if err != nil {
				iErr = err
				return true
			}
			entries = append(entries, o)
		}
		return false
	})
	if err != nil {
		return nil, err
	}
	if iErr != nil {
		return nil, iErr
	}
	return entries, nil
}

// Creates from the parameters passed in a half finished Object which
// must have setMetaData called on it
//
// Returns the dirNode, object, leaf and error
//
// Used to create new objects
func (f *Fs) createObject(remote string, modTime time.Time, size int64) (o *Object, dirNode *mega.Node, leaf string, err error) {
	dirNode, leaf, err = f.mkdirParent(remote)
	if err != nil {
		return nil, nil, leaf, err
	}
	// Temporary Object under construction
	o = &Object{
		fs:     f,
		remote: remote,
	}
	return o, dirNode, leaf, nil
}

// Put the object
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
// PutUnchecked uploads the object
//
// This will create a duplicate if we upload a new file without
// checking to see if there is one already - use Put() for that.
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	existingObj, err := f.newObjectWithInfo(src.Remote(), nil)
	switch err {
	case nil:
		return existingObj, existingObj.Update(ctx, in, src, options...)
	case fs.ErrorObjectNotFound:
		// Not found so create it
		return f.PutUnchecked(ctx, in, src)
	default:
		return nil, err
	}
}

// PutUnchecked the object
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
// PutUnchecked uploads the object
//
// This will create a duplicate if we upload a new file without
// checking to see if there is one already - use Put() for that.
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()
	modTime := src.ModTime(ctx)

	o, _, _, err := f.createObject(remote, modTime, size)
	if err != nil {
		return nil, err
	}
	return o, o.Update(ctx, in, src, options...)
}

// Mkdir creates the directory if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	rootNode, err := f.findRoot(true)
	if err != nil {
		return err
	}
	_, err = f.mkdir(rootNode, dir)
	return errors.Wrap(err, "Mkdir failed")
}

// deleteNode removes a file or directory, observing useTrash
func (f *Fs) deleteNode(node *mega.Node) (err error) {
	err = f.pacer.Call(func() (bool, error) {
		err = f.srv.Delete(node, f.opt.HardDelete)
		return shouldRetry(err)
	})
	return err
}

// purgeCheck removes the directory dir, if check is set then it
// refuses to do so if it has anything in
func (f *Fs) purgeCheck(dir string, check bool) error {
	f.mkdirMu.Lock()
	defer f.mkdirMu.Unlock()

	rootNode, err := f.findRoot(false)
	if err != nil {
		return err
	}
	dirNode, err := f.findDir(rootNode, dir)
	if err != nil {
		return err
	}

	if check {
		children, err := f.srv.FS.GetChildren(dirNode)
		if err != nil {
			return errors.Wrap(err, "purgeCheck GetChildren failed")
		}
		if len(children) > 0 {
			return fs.ErrorDirectoryNotEmpty
		}
	}

	waitEvent := f.srv.WaitEventsStart()

	err = f.deleteNode(dirNode)
	if err != nil {
		return errors.Wrap(err, "delete directory node failed")
	}

	// Remove the root node if we just deleted it
	if dirNode == rootNode {
		f.clearRoot()
	}

	f.srv.WaitEvents(waitEvent, eventWaitTime)
	return nil
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.purgeCheck(dir, true)
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// Purge deletes all the files in the directory
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(dir, false)
}

// move a file or folder (srcFs, srcRemote, info) to (f, dstRemote)
//
// info will be updates
func (f *Fs) move(dstRemote string, srcFs *Fs, srcRemote string, info *mega.Node) (err error) {
	var (
		dstFs                  = f
		srcDirNode, dstDirNode *mega.Node
		srcParent, dstParent   string
		srcLeaf, dstLeaf       string
	)

	if dstRemote != "" {
		// lookup or create the destination parent directory
		dstDirNode, dstLeaf, err = dstFs.mkdirParent(dstRemote)
	} else {
		// find or create the parent of the root directory
		absRoot := dstFs.srv.FS.GetRoot()
		dstParent, dstLeaf = path.Split(dstFs.root)
		dstDirNode, err = dstFs.mkdir(absRoot, dstParent)
	}
	if err != nil {
		return errors.Wrap(err, "server side move failed to make dst parent dir")
	}

	if srcRemote != "" {
		// lookup the existing parent directory
		srcDirNode, srcLeaf, err = srcFs.lookupParentDir(srcRemote)
	} else {
		// lookup the existing root parent
		absRoot := srcFs.srv.FS.GetRoot()
		srcParent, srcLeaf = path.Split(srcFs.root)
		srcDirNode, err = f.findDir(absRoot, srcParent)
	}
	if err != nil {
		return errors.Wrap(err, "server side move failed to lookup src parent dir")
	}

	// move the object into its new directory if required
	if srcDirNode != dstDirNode && srcDirNode.GetHash() != dstDirNode.GetHash() {
		//log.Printf("move src %p %q dst %p %q", srcDirNode, srcDirNode.GetName(), dstDirNode, dstDirNode.GetName())
		err = f.pacer.Call(func() (bool, error) {
			err = f.srv.Move(info, dstDirNode)
			return shouldRetry(err)
		})
		if err != nil {
			return errors.Wrap(err, "server side move failed")
		}
	}

	waitEvent := f.srv.WaitEventsStart()

	// rename the object if required
	if srcLeaf != dstLeaf {
		//log.Printf("rename %q to %q", srcLeaf, dstLeaf)
		err = f.pacer.Call(func() (bool, error) {
			err = f.srv.Rename(info, f.opt.Enc.FromStandardName(dstLeaf))
			return shouldRetry(err)
		})
		if err != nil {
			return errors.Wrap(err, "server side rename failed")
		}
	}

	f.srv.WaitEvents(waitEvent, eventWaitTime)

	return nil
}

// Move src to this remote using server side move operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantMove
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	dstFs := f

	//log.Printf("Move %q -> %q", src.Remote(), remote)
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	// Do the move
	err := f.move(remote, srcObj.fs, srcObj.remote, srcObj.info)
	if err != nil {
		return nil, err
	}

	// Create a destination object
	dstObj := &Object{
		fs:     dstFs,
		remote: remote,
		info:   srcObj.info,
	}
	return dstObj, nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	dstFs := f
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}

	// find the source
	info, err := srcFs.lookupDir(srcRemote)
	if err != nil {
		return err
	}

	// check the destination doesn't exist
	_, err = dstFs.lookupDir(dstRemote)
	if err == nil {
		return fs.ErrorDirExists
	} else if err != fs.ErrorDirNotFound {
		return errors.Wrap(err, "DirMove error while checking dest directory")
	}

	// Do the move
	err = f.move(dstRemote, srcFs, srcRemote, info)
	if err != nil {
		return err
	}

	// Clear src if it was the root
	if srcRemote == "" {
		srcFs.clearRoot()
	}

	return nil
}

// DirCacheFlush an optional interface to flush internal directory cache
func (f *Fs) DirCacheFlush() {
	// f.dirCache.ResetRoot()
	// FIXME Flush the mega somehow?
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// PublicLink generates a public link to the remote path (usually readable by anyone)
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (link string, err error) {
	root, err := f.findRoot(false)
	if err != nil {
		return "", errors.Wrap(err, "PublicLink failed to find root node")
	}
	node, err := f.findNode(root, remote)
	if err != nil {
		return "", errors.Wrap(err, "PublicLink failed to find path")
	}
	link, err = f.srv.Link(node, true)
	if err != nil {
		return "", errors.Wrap(err, "PublicLink failed to create link")
	}
	return link, nil
}

// MergeDirs merges the contents of all the directories passed
// in into the first one and rmdirs the other directories.
func (f *Fs) MergeDirs(ctx context.Context, dirs []fs.Directory) error {
	if len(dirs) < 2 {
		return nil
	}
	// find dst directory
	dstDir := dirs[0]
	dstDirNode := f.srv.FS.HashLookup(dstDir.ID())
	if dstDirNode == nil {
		return errors.Errorf("MergeDirs failed to find node for: %v", dstDir)
	}
	for _, srcDir := range dirs[1:] {
		// find src directory
		srcDirNode := f.srv.FS.HashLookup(srcDir.ID())
		if srcDirNode == nil {
			return errors.Errorf("MergeDirs failed to find node for: %v", srcDir)
		}

		// list the objects
		infos := []*mega.Node{}
		_, err := f.list(ctx, srcDirNode, func(info *mega.Node) bool {
			infos = append(infos, info)
			return false
		})
		if err != nil {
			return errors.Wrapf(err, "MergeDirs list failed on %v", srcDir)
		}
		// move them into place
		for _, info := range infos {
			fs.Infof(srcDir, "merging %q", f.opt.Enc.ToStandardName(info.GetName()))
			err = f.pacer.Call(func() (bool, error) {
				err = f.srv.Move(info, dstDirNode)
				return shouldRetry(err)
			})
			if err != nil {
				return errors.Wrapf(err, "MergeDirs move failed on %q in %v", f.opt.Enc.ToStandardName(info.GetName()), srcDir)
			}
		}
		// rmdir (into trash) the now empty source directory
		fs.Infof(srcDir, "removing empty directory")
		err = f.deleteNode(srcDirNode)
		if err != nil {
			return errors.Wrapf(err, "MergeDirs move failed to rmdir %q", srcDir)
		}
	}
	return nil
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	var q mega.QuotaResp
	var err error
	err = f.pacer.Call(func() (bool, error) {
		q, err = f.srv.GetQuota()
		return shouldRetry(err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Mega Quota")
	}
	usage := &fs.Usage{
		Total: fs.NewUsageValue(int64(q.Mstrg)),           // quota of bytes that can be used
		Used:  fs.NewUsageValue(int64(q.Cstrg)),           // bytes in use
		Free:  fs.NewUsageValue(int64(q.Mstrg - q.Cstrg)), // bytes which can be uploaded before reaching the quota
	}
	return usage, nil
}

// ------------------------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// Hash returns the hashes of an object
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.info.GetSize()
}

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *mega.Node) (err error) {
	if info.GetType() != mega.FILE {
		return fs.ErrorNotAFile
	}
	o.info = info
	return nil
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData() (err error) {
	if o.info != nil {
		return nil
	}
	info, err := o.fs.readMetaDataForPath(o.remote)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			err = fs.ErrorObjectNotFound
		}
		return err
	}
	return o.setMetaData(info)
}

// ModTime returns the modification time of the object
//
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.info.GetTimeStamp()
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// openObject represents a download in progress
type openObject struct {
	mu     sync.Mutex
	o      *Object
	d      *mega.Download
	id     int
	skip   int64
	chunk  []byte
	closed bool
}

// get the next chunk
func (oo *openObject) getChunk() (err error) {
	if oo.id >= oo.d.Chunks() {
		return io.EOF
	}
	var chunk []byte
	err = oo.o.fs.pacer.Call(func() (bool, error) {
		chunk, err = oo.d.DownloadChunk(oo.id)
		return shouldRetry(err)
	})
	if err != nil {
		return err
	}
	oo.id++
	oo.chunk = chunk
	return nil
}

// Read reads up to len(p) bytes into p.
func (oo *openObject) Read(p []byte) (n int, err error) {
	oo.mu.Lock()
	defer oo.mu.Unlock()
	if oo.closed {
		return 0, errors.New("read on closed file")
	}
	// Skip data at the start if requested
	for oo.skip > 0 {
		_, size, err := oo.d.ChunkLocation(oo.id)
		if err != nil {
			return 0, err
		}
		if oo.skip < int64(size) {
			break
		}
		oo.id++
		oo.skip -= int64(size)
	}
	if len(oo.chunk) == 0 {
		err = oo.getChunk()
		if err != nil {
			return 0, err
		}
		if oo.skip > 0 {
			oo.chunk = oo.chunk[oo.skip:]
			oo.skip = 0
		}
	}
	n = copy(p, oo.chunk)
	oo.chunk = oo.chunk[n:]
	return n, nil
}

// Close closed the file - MAC errors are reported here
func (oo *openObject) Close() (err error) {
	oo.mu.Lock()
	defer oo.mu.Unlock()
	if oo.closed {
		return nil
	}
	err = oo.o.fs.pacer.Call(func() (bool, error) {
		err = oo.d.Finish()
		return shouldRetry(err)
	})
	if err != nil {
		return errors.Wrap(err, "failed to finish download")
	}
	oo.closed = true
	return nil
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	var offset, limit int64 = 0, -1
	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			offset = x.Offset
		case *fs.RangeOption:
			offset, limit = x.Decode(o.Size())
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}

	var d *mega.Download
	err = o.fs.pacer.Call(func() (bool, error) {
		d, err = o.fs.srv.NewDownload(o.info)
		return shouldRetry(err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "open download file failed")
	}

	oo := &openObject{
		o:    o,
		d:    d,
		skip: offset,
	}

	return readers.NewLimitedReadCloser(oo, limit), nil
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	size := src.Size()
	if size < 0 {
		return errors.New("mega backend can't upload a file of unknown length")
	}
	//modTime := src.ModTime(ctx)
	remote := o.Remote()

	// Create the parent directory
	dirNode, leaf, err := o.fs.mkdirParent(remote)
	if err != nil {
		return errors.Wrap(err, "update make parent dir failed")
	}

	var u *mega.Upload
	err = o.fs.pacer.Call(func() (bool, error) {
		u, err = o.fs.srv.NewUpload(dirNode, o.fs.opt.Enc.FromStandardName(leaf), size)
		return shouldRetry(err)
	})
	if err != nil {
		return errors.Wrap(err, "upload file failed to create session")
	}

	// Upload the chunks
	// FIXME do this in parallel
	for id := 0; id < u.Chunks(); id++ {
		_, chunkSize, err := u.ChunkLocation(id)
		if err != nil {
			return errors.Wrap(err, "upload failed to read chunk location")
		}
		chunk := make([]byte, chunkSize)
		_, err = io.ReadFull(in, chunk)
		if err != nil {
			return errors.Wrap(err, "upload failed to read data")
		}

		err = o.fs.pacer.Call(func() (bool, error) {
			err = u.UploadChunk(id, chunk)
			return shouldRetry(err)
		})
		if err != nil {
			return errors.Wrap(err, "upload file failed to upload chunk")
		}
	}

	// Finish the upload
	var info *mega.Node
	err = o.fs.pacer.Call(func() (bool, error) {
		info, err = u.Finish()
		return shouldRetry(err)
	})
	if err != nil {
		return errors.Wrap(err, "failed to finish upload")
	}

	// If the upload succeeded and the original object existed, then delete it
	if o.info != nil {
		err = o.fs.deleteNode(o.info)
		if err != nil {
			return errors.Wrap(err, "upload failed to remove old version")
		}
		o.info = nil
	}

	return o.setMetaData(info)
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	err := o.fs.deleteNode(o.info)
	if err != nil {
		return errors.Wrap(err, "Remove object failed")
	}
	return nil
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.info.GetHash()
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.PutUncheckeder  = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.PublicLinker    = (*Fs)(nil)
	_ fs.MergeDirser     = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)
