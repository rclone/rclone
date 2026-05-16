// Package imap implements a provider for imap servers.
package imap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/env"

	"github.com/emersion/go-imap"
)

var (
	currentUser          = env.CurrentUser()
	imapHost             = "imap.rclone.org"
	securityNone         = 0
	securityStartTLS     = 1
	securityTLS          = 2
	messageNameRegEx     = regexp.MustCompile(`(\d+)\.H([^\.]+)\.([^,]+),S\=(\d+),W\=(\d+)-2,(.*)`)
	remoteRegex          = regexp.MustCompile(`(?:(?P<parent>.*)\/)?(?P<file>\d+\.H[^\.]+\.[^,]+,S\=\d+,W\=\d+-2,.*)|(?P<dir>.*)`)
	errorInvalidFileName = errors.New("invalid file name")
	errorInvalidMessage  = errors.New("invalid IMAP message")
	errorReadingMessage  = errors.New("failed to read message")
)

// Options defines the configuration for this backend
type Options struct {
	Host               string `config:"host"`
	User               string `config:"user"`
	Pass               string `config:"pass"`
	Port               string `config:"port"`
	AskPassword        bool   `config:"ask_password"`
	Security           string `config:"security"`
	InsecureSkipVerify bool   `config:"no_check_certificate"`
}

type countReader struct {
	io.Reader
	total int64
}

func (w *countReader) Close() error {
	closer, isCloser := w.Reader.(io.Closer)
	if isCloser {
		return closer.Close()
	}
	return nil
}

func (w *countReader) Read(p []byte) (int, error) {
	n, err := w.Reader.Read(p)
	w.total += int64(n)
	return n, err
}

// mail client interface

type mailclient interface {
	Logout()
	ListMailboxes(dir string) ([]string, error)
	HasMailbox(dir string) bool
	RenameMailbox(from string, to string) error
	CreateMailbox(name string) error
	DeleteMailbox(name string) error
	ExpungeMailbox(mailbox string) error
	SetFlags(mailbox string, ids []uint32, flags ...string) error
	Save(mailbox string, date time.Time, size int64, reader io.Reader, flags []string) (err error)
	Search(mailbox string, since, before time.Time, larger, smaller uint32) (seqNums []uint32, err error)
	ForEach(mailbox string, action func(uint32, time.Time, uint32, []string, io.Reader)) error
	ForEachID(mailbox string, ids []uint32, action func(uint32, time.Time, uint32, []string, io.Reader)) error
}

// Register with Fs
func init() {
	fsi := &fs.RegInfo{
		Name:        "imap",
		Description: "IMAP",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:      "host",
			Help:      "IMAP host to connect to.\n\nE.g. \"imap.example.com\".",
			Required:  true,
			Sensitive: true,
		}, {
			Name:      "user",
			Help:      "IMAP username.",
			Default:   currentUser,
			Sensitive: true,
		}, {
			Name:    "port",
			Help:    "IMAP port number.",
			Default: 143,
		}, {
			Name:       "pass",
			Help:       "IMAP password.",
			IsPassword: true,
		}, {
			Name:    "security",
			Help:    "IMAP Connection type: none,starttls,tls",
			Default: "starttls",
		}, {
			Name:    "no_check_certificate",
			Help:    "Skip server certificate verification",
			Default: true,
		}},
	}
	fs.Register(fsi)
}

// ------------------------------------------------------------
// Utility functions
// ------------------------------------------------------------

func createHash(reader io.ReadCloser, t ...hash.Type) (map[hash.Type]string, int64, error) {
	if len(t) == 0 {
		// default hash XXH3 because is fast
		t = []hash.Type{hash.XXH3}
	}
	// create hasher
	set := hash.NewHashSet(t...)
	hasher, err := hash.NewMultiHasherTypes(set)
	if err != nil {
		return nil, 0, err
	}
	// create counting reader
	newReader := &countReader{Reader: reader}
	// do hash
	if _, err := io.Copy(hasher, newReader); err != nil {
		return nil, 0, err
	}
	// close reader
	err = newReader.Close()
	if err != nil {
		return nil, newReader.total, err
	}
	// save hashes to map
	hashes := map[hash.Type]string{}
	for _, curr := range t {
		// get the hash
		checksum, err := hasher.SumString(hash.XXH3, false)
		if err == nil {
			hashes[curr] = checksum
		}
	}
	//
	return hashes, newReader.total, nil
}

func removeRoot(root, element string) string {
	// remove leading/trailing slashes from element and root
	root = strings.Trim(root, "/")
	element = strings.Trim(element, "/")
	// check if element matches
	if root == "" {
		return element
	} else if root == element {
		return ""
	} else if strings.HasPrefix(element, root+"/") {
		return strings.TrimPrefix(element, root+"/")
	}
	return element
}

func regexToMap(r *regexp.Regexp, str string) map[string]string {
	results := map[string]string{}
	matches := r.FindStringSubmatch(str)
	if matches != nil {
		for i, name := range r.SubexpNames() {
			if i != 0 {
				results[name] = matches[i]
			}
		}
	}
	return results
}

// ------------------------------------------------------------
// Maildir name functions
// Example: 1771568830.H71c34c671bcb1da4.imap.rclone.org,S=3444,W=3444-2,SATD
//
//	1771568830 - date in unix format
//	H71c34c671bcb1da4 - XXH3 hash for the message (H+hash value)
//	imap.rclone.org - host, not really needed.
//	S=3444 - Message size
//	W=3444 - Message size reported by mailserver
//	SATD - Maildir flags
//
// ------------------------------------------------------------
func createMaildirName(date time.Time, checksum string, size int64, rf822Size int64, flags []string) string {
	name := fmt.Sprintf("%d.H%s.%s,S=%d,W=%d-2,", date.UTC().Unix(), checksum, imapHost, size, rf822Size)
	if slices.Contains(flags, imap.SeenFlag) {
		name += "S"
	}
	if slices.Contains(flags, imap.AnsweredFlag) {
		name += "A"
	}
	if slices.Contains(flags, imap.DeletedFlag) {
		name += "T"
	}
	if slices.Contains(flags, imap.DraftFlag) {
		name += "D"
	}
	if slices.Contains(flags, imap.FlaggedFlag) {
		name += "F"
	}
	return name
}

func parseMaildirName(name string) (date time.Time, checksum string, size int64, rf822Size int64, flags []string, err error) {
	matches := messageNameRegEx.FindStringSubmatch(path.Base(name))
	if matches == nil {
		err = errorInvalidFileName
		return
	}
	// get date
	i, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		err = errorInvalidFileName
		return
	}
	date = time.Unix(i, 0).UTC()
	// get hash
	checksum = matches[2]
	// get size
	size, err = strconv.ParseInt(matches[4], 10, 32)
	if err != nil {
		err = errorInvalidFileName
		return
	}
	// get rf822Size
	rf822Size, err = strconv.ParseInt(matches[5], 10, 32)
	if err != nil {
		err = errorInvalidFileName
		return
	}
	// get flags
	if strings.Contains(matches[6], "S") {
		flags = append(flags, imap.SeenFlag)
	}
	if strings.Contains(matches[6], "A") {
		flags = append(flags, imap.AnsweredFlag)
	}
	if strings.Contains(matches[6], "T") {
		flags = append(flags, imap.DeletedFlag)
	}
	if strings.Contains(matches[6], "D") {
		flags = append(flags, imap.DraftFlag)
	}
	if strings.Contains(matches[6], "F") {
		flags = append(flags, imap.FlaggedFlag)
	}
	return
}

// ------------------------------------------------------------
// Descriptor
// ------------------------------------------------------------

type descriptor struct {
	parent    string
	name      string
	date      time.Time
	xxh3sum   string
	size      int64
	rf822Size int64
	flags     []string
}

func newDescriptor(parent string, name string, date time.Time, checksum string, size int64, rf822Size int64, flags []string) *descriptor {
	// create Maildir name for message
	if name == "" {
		name = createMaildirName(date, checksum, size, rf822Size, flags)
	}
	return &descriptor{
		parent:    parent,
		name:      name,
		date:      date.UTC(),
		xxh3sum:   checksum,
		size:      size,
		rf822Size: rf822Size,
		flags:     flags,
	}
}

func readerToDescriptor(parent string, name string, date time.Time, reader io.Reader, rf822Size int64, flags []string) (*descriptor, error) {
	seeker, isSeeker := reader.(io.ReadSeeker)
	if !isSeeker {
		fs.Debugf(nil, "readerToMessage - not a seeker!!")
		return nil, errorReadingMessage
	}
	// check if message
	_, err := mail.ReadMessage(seeker)
	if err != nil {
		return nil, errorInvalidMessage
	}
	// rewind to calculate checksum
	_, err = seeker.Seek(0, io.SeekStart)
	if err != nil {
		return nil, errorReadingMessage
	}
	// calc XXH3 checksum
	hashes, size, err := createHash(io.NopCloser(reader))
	if err != nil {
		return nil, errorReadingMessage
	}
	// get the hash
	checksum, found := hashes[hash.XXH3]
	if !found {
		return nil, errorReadingMessage
	}
	// rewind
	_, err = seeker.Seek(0, io.SeekStart)
	if err != nil {
		return nil, errorReadingMessage
	}
	//
	return newDescriptor(parent, name, date, checksum, size, rf822Size, flags), nil
}

func objectToDescriptor(ctx context.Context, parent string, o fs.Object) (info *descriptor, err error) {
	// try and check if name is valid maildir name
	info, err = nameToDescriptor(parent, o.Remote())
	if err == nil {
		return info, nil
	}
	// not a valid name, check if a valid message
	reader, err := o.Open(ctx)
	if err != nil {
		return nil, errorReadingMessage
	}
	// check if message
	_, err = mail.ReadMessage(reader)
	if err != nil {
		return nil, errorInvalidMessage
	}
	// valid message, get checksum
	reader, err = o.Open(ctx)
	if err != nil {
		return nil, errorReadingMessage
	}
	hashes, size, err := createHash(reader)
	if err != nil {
		return nil, errorReadingMessage
	}
	checksum, found := hashes[hash.XXH3]
	if !found {
		return nil, errorReadingMessage
	}
	return newDescriptor(parent, o.Remote(), o.ModTime(ctx).UTC(), checksum, size, size, []string{}), nil
}

func nameToDescriptor(parent, name string) (*descriptor, error) {
	date, checksum, size, rf822Size, flags, err := parseMaildirName(name)
	if err != nil {
		return nil, errorInvalidFileName
	}
	return newDescriptor(parent, name, date, checksum, size, rf822Size, flags), nil
}

func (i *descriptor) Equal(o *descriptor) bool {
	return i.date == o.date && i.xxh3sum == o.xxh3sum && i.size == o.size
}

func (i *descriptor) Name() string {
	return i.name
}

func (i *descriptor) MaildirName(flags bool) string {
	return createMaildirName(i.date, i.xxh3sum, i.size, i.rf822Size, i.flags)
}

func (i *descriptor) Matches(name string) bool {
	matches := messageNameRegEx.FindStringSubmatch(path.Base(name))
	if matches == nil {
		return false
	}
	// get date
	idate, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return false
	}
	date := time.Unix(idate, 0).UTC()
	// get hash
	checksum := matches[2]
	// get size
	size, err := strconv.ParseInt(matches[4], 10, 64)
	if err != nil {
		return false
	}
	return i.date == date && i.xxh3sum == checksum && i.size == size
}

func (i *descriptor) ModTime() time.Time {
	return i.date
}

func (i *descriptor) Checksum() string {
	return i.xxh3sum
}

func (i *descriptor) Host() string {
	return imapHost
}

func (i *descriptor) Size() int64 {
	return i.size
}

func (i *descriptor) IsFlagSet(value string) bool {
	return slices.Contains(i.flags, value)
}

// ------------------------------------------------------------
// Fs
// ------------------------------------------------------------

// Fs represents a remote IMAP server
type Fs struct {
	name     string       // name of this remote
	root     string       // the path we are working on if any
	opt      Options      // parsed config options
	features *fs.Features // optional features
	//
	host       string
	port       int
	user       string
	pass       string
	security   int
	skipVerify bool
}

// Name of this fs
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String returns a description of the FS
func (f *Fs) String() string {
	if f.security == securityTLS {
		return fmt.Sprintf("imaps://%s:%d", f.host, f.port)
	}
	return fmt.Sprintf("imap://%s:%d", f.host, f.port)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

func (f *Fs) createMailClient() (mailclient, error) {
	return newMailClientV2(f)
}

func (f *Fs) createDescriptor(parent string, name string, date time.Time, rf822Size uint32, flags []string, reader io.Reader) (*descriptor, error) {
	// calc checksum
	hashes, size, err := createHash(io.NopCloser(reader))
	if err != nil {
		fs.Debugf(nil, "Error creating entry: %s", err.Error())
		return nil, err
	}
	// get the hash
	checksum := hashes[hash.XXH3]
	return newDescriptor(parent, name, date, checksum, size, int64(rf822Size), flags), nil
}

func (f *Fs) createObject(seqNum uint32, info *descriptor) *Object {
	return &Object{fs: f, seqNum: seqNum, info: info, hashes: map[hash.Type]string{hash.XXH3: info.Checksum()}}
}

// List returns the entries in the directory
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	var parent string
	var file string
	var mailboxes []string
	var info *descriptor
	// connect to imap server
	client, err := f.createMailClient()
	if err != nil {
		return nil, err
	}
	// connected, logout on exit
	defer client.Logout()
	// parse
	groups := regexToMap(remoteRegex, path.Join(f.root, dir))
	file = groups["file"]
	if file == "" {
		parent = groups["dir"]
	} else {
		parent = groups["parent"]
	}
	// check parent and file
	if parent == "" {
		// find message in root, impossible
		if file != "" {
			fs.Debugf(nil, "List %s: matching %s (there are no messages in root)", f.root, file)
			return entries, nil
		}
		// list mailboxes in root
		fs.Debugf(nil, "List root")
	} else if !client.HasMailbox(parent) {
		// mailbox does not exist, leave
		fs.Debugf(nil, "List %s:%s, not found", f.name, parent)
		return nil, fs.ErrorDirNotFound
	} else if file == "" {
		// list mailbox contents
		fs.Debugf(nil, "List %s:%s", f.name, parent)
	} else {
		fs.Debugf(nil, "List %s:%s matching %s", f.name, parent, file)
		// list a message in mailbox,file must be in rclone maildir format
		// extract info from file name
		info, err = nameToDescriptor(parent, file)
		if err == errorInvalidFileName {
			// searching for filename not in maildir format, will never be found
			return entries, nil
		} else if err != nil {
			// unknown error
			return nil, err
		}
	}
	// get mailboxes
	mailboxes, err = client.ListMailboxes(parent)
	for _, name := range mailboxes {
		if file == "" {
			d := fs.NewDir(path.Join(dir, name), time.Unix(0, 0))
			entries = append(entries, d)
		}
	}
	// get messages
	if parent == "" && file == "" {
		// root has no messages
		return entries, nil
	} else if info != nil {
		// searching for a matching message
		ids, err := client.Search(parent, info.ModTime().AddDate(0, -1, 0), info.ModTime().AddDate(0, 1, 0), uint32(info.Size()-50), uint32(info.Size()+50))
		// error searching
		if err != nil {
			return nil, err
		}
		// no matches
		if len(ids) == 0 {
			return entries, nil
		}
		// ok fetch messages with ids
		fs.Debugf(nil, "Fetch from %s - IDS=%s", strings.Trim(parent, "/"), strings.Join(strings.Fields(fmt.Sprint(ids)), ", "))
		err = client.ForEachID(parent, ids, func(seqNum uint32, date time.Time, size uint32, flags []string, reader io.Reader) {
			currInfo, err := f.createDescriptor(parent, file, date, size, flags, reader)
			if err != nil {
				fs.Debugf(nil, "Error creating descriptor for ID(%d), %s", seqNum, err.Error())
			} else if currInfo.Equal(info) {
				entries = append(entries, f.createObject(seqNum, currInfo))
			}
		})
		if err != nil {
			return nil, fmt.Errorf("failed to fetch message: %w", err)
		}
	} else {
		// fetch all messages in mailbox
		fs.Debugf(nil, "Fetch all from %s", strings.Trim(parent, "/"))
		err = client.ForEach(parent, func(seqNum uint32, date time.Time, size uint32, flags []string, reader io.Reader) {
			currInfo, err := f.createDescriptor(parent, file, date, size, flags, reader)
			if err != nil {
				fs.Debugf(nil, "Error creating descriptor for ID(%d), %s", seqNum, err.Error())
			}
			entries = append(entries, f.createObject(seqNum, currInfo))
		})
		if err != nil {
			return nil, fmt.Errorf("failed to fetch message list: %w", err)
		}
	}
	// return entries
	return entries, nil
}

// Precision of the object storage system
func (f *Fs) Precision() time.Duration {
	return time.Second
}

func (f *Fs) findObject(ctx context.Context, info *descriptor) (*Object, error) {
	var o *Object
	// connect to imap server
	client, err := f.createMailClient()
	if err != nil {
		return nil, fmt.Errorf("failed append: %w", err)
	}
	// connected, logout on exit
	defer client.Logout()
	// search messages
	ids, err := client.Search(info.parent, info.ModTime().AddDate(0, -1, 0), info.ModTime().AddDate(0, 1, 0), uint32(info.Size()-50), uint32(info.Size()+50))
	if err != nil {
		return nil, fs.ErrorObjectNotFound
	} else if len(ids) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	// messages found, check matching one
	err = client.ForEachID(info.parent, ids, func(seqNum uint32, date time.Time, size uint32, flags []string, reader io.Reader) {
		// leave if we found it
		if o != nil {
			return
		}
		//
		currInfo, err := f.createDescriptor(info.parent, "", date, size, flags, reader)
		if err != nil {
			return
		} else if info.Equal(currInfo) {
			o = f.createObject(seqNum, currInfo)
		}
	})
	//
	if err != nil {
		return nil, err
	} else if o == nil {
		return nil, fs.ErrorObjectNotFound
	}
	//
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	var info *descriptor
	var err error
	//
	srcObj, hasSource := ctx.Value(operations.SourceObjectKey).(fs.Object)
	if hasSource {
		// srcObj is valid, for message using date,checksum,size instead of name
		info, err = objectToDescriptor(ctx, f.root, srcObj)
	} else {
		// try and get descriptor from name
		info, err = nameToDescriptor(f.root, remote)
	}
	// leave if no descriptor
	if err != nil {
		return nil, fserrors.NoRetryError(err)
	} else if hasSource {
		fs.Debugf(nil, "Using source to find destination object")
	} else {
		fs.Debugf(nil, "Using name to find destination object")
	}
	return f.findObject(ctx, info)
}

// Put the object
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	var info *descriptor
	var err error
	// check if srcObject available
	srcObj, hasSource := ctx.Value(operations.SourceObjectKey).(fs.Object)
	if hasSource {
		fs.Debugf(nil, "found SourceObjectKey in context, convert to descriptor")
		info, err = objectToDescriptor(ctx, f.root, srcObj)
	} else {
		err = errorReadingMessage
	}
	// check if name is valid maildir name
	if err != nil {
		fs.Debugf(nil, "failed getting descriptor from context,use name")
		info, err = nameToDescriptor(f.root, src.Remote())
	}
	// check if we can create info from reader
	if err != nil {
		fs.Debugf(nil, "failed getting descriptor from context and name,use reader")
		info, err = readerToDescriptor(f.root, src.Remote(), src.ModTime(ctx), in, src.Size(), []string{})
	}
	// leave if unable to create info or if it is a dry run
	ci := fs.GetConfig(ctx)
	if err != nil {
		return nil, fserrors.NoRetryError(err)
	} else if ci.DryRun {
		return f.createObject(1, info), nil
	}
	// connect to imap server
	client, err := f.createMailClient()
	if err != nil {
		return nil, fmt.Errorf("failed append: %w", err)
	}
	// connected, logout on exit
	defer client.Logout()
	// mkdir if not found
	mailbox := path.Join(info.parent, path.Dir(src.Remote()))
	if !client.HasMailbox(mailbox) {
		fs.Debugf(nil, "Create mailbox %s", mailbox)
		err = client.CreateMailbox(mailbox)
		if err != nil {
			return nil, fserrors.NoRetryError(fmt.Errorf("failed append: %w", err))
		}
	}
	// upload message
	err = client.Save(mailbox, info.ModTime(), info.Size(), in, info.flags)
	if err != nil {
		return nil, fmt.Errorf("failed append: %w", err)
	}
	return f.findObject(ctx, info)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// Mkdir creates the directory if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) (err error) {
	// connect to imap server
	client, err := f.createMailClient()
	if err != nil {
		return err
	}
	// connected, logout on exit
	defer client.Logout()
	// check if mailbox exists
	root := path.Join(f.root, dir)
	if client.HasMailbox(root) {
		return nil
	}
	// do nothing if dry run
	ci := fs.GetConfig(ctx)
	if ci.DryRun {
		return nil
	}
	// create the mailbox
	return client.CreateMailbox(root)
}

// Rmdir deletes a directory
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) (err error) {
	// connect to imap server
	client, err := f.createMailClient()
	if err != nil {
		return err
	}
	// connected, logout on exit
	defer client.Logout()
	// check if mailbox exists
	root := path.Join(f.root, dir)
	if !client.HasMailbox(root) {
		return nil
	}
	// do nothing if dry run
	ci := fs.GetConfig(ctx)
	if ci.DryRun {
		return nil
	}
	// create the mailbox
	err = client.DeleteMailbox(root)
	if err != nil {
		return fserrors.NoRetryError(err)
	}
	return nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server-side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	//
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	srcPath := path.Join(srcFs.root, srcRemote)
	dstPath := path.Join(f.root, dstRemote)
	// connect to imap server
	client, err := f.createMailClient()
	if err != nil {
		return err
	}
	// connected, logout on exit
	defer client.Logout()
	// check if mailbox exists
	if client.HasMailbox(dstPath) {
		return fs.ErrorDirExists
	}
	// leave if dry dryrun
	ci := fs.GetConfig(ctx)
	if ci.DryRun {
		return nil
	}
	// do rename
	return client.RenameMailbox(srcPath, dstPath)
}

// Hashes - valid MD5,SHA1,XXH3
func (f *Fs) Hashes() hash.Set {
	hashSet := hash.NewHashSet()
	hashSet.Add(hash.SHA1)
	hashSet.Add(hash.MD5)
	hashSet.Add(hash.XXH3)
	return hashSet
}

// ------------------------------------------------------------
// Object
// ------------------------------------------------------------

// Object describes an IMAP message
type Object struct {
	fs     *Fs
	seqNum uint32
	info   *descriptor
	hashes map[hash.Type]string
}

// Equal compare objects
func (o *Object) Equal(v *Object) bool {
	return o.info.Equal(v.info)
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// String version of o
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.Remote()
}

// Remote returns the remote path
func (o *Object) Remote() string {
	var name = path.Join(o.info.parent, o.info.Name())
	//
	if name == o.fs.root {
		return path.Base(o.info.name)
	}
	return removeRoot(o.fs.root, name)
}

// Hash returns the hash of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	// find hash in map
	checksum, found := o.hashes[t]
	// hash found in cache
	if found {
		return checksum, nil
	}
	// get reader
	reader, err := o.Open(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to calculate %v: %w", t, err)
	}
	// create hash
	hashes, _, err := createHash(reader, t)
	if err != nil {
		return "", hash.ErrUnsupported
	}
	// get the hash
	checksum, found = hashes[t]
	if !found {
		return "", hash.ErrUnsupported
	}
	o.hashes[t] = checksum
	// return hash
	return checksum, err
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.info.Size()
}

// ModTime returns the modification time of the object
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.info.ModTime()
}

// SetModTime sets the modification time of the object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	fs.Debugf(o.fs, "SetModTime is not supported")
	return nil
}

// Storable returns a boolean as to whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	var result io.Reader
	//
	fs.Debugf(nil, "Open remote=%s, root=%s", o.Remote(), o.fs.root)
	// connect to imap server
	client, err := o.fs.createMailClient()
	if err != nil {
		return nil, err
	}
	// connected, logout on exit
	defer client.Logout()
	// fetch message
	mailbox := path.Join(o.info.parent, path.Dir(o.info.name))
	// get message
	err = client.ForEachID(mailbox, []uint32{o.seqNum}, func(_ uint32, _ time.Time, _ uint32, _ []string, reader io.Reader) {
		// leave if we found it
		if result != nil {
			return
		}
		result = reader
	})
	if err != nil {
		return nil, err
	} else if result == nil {
		return nil, fmt.Errorf("failed to open %s, invalid reader", o.Remote())
	}
	// this should be seekable
	return io.NopCloser(result), nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	// leave if dry run
	ci := fs.GetConfig(ctx)
	if ci.DryRun {
		return nil
	}
	// connect to imap server
	client, err := o.fs.createMailClient()
	if err != nil {
		return err
	}
	// connected, logout on exit
	defer client.Logout()
	// set flags to deleted
	mailbox := path.Join(o.info.parent, path.Dir(o.info.name))
	err = client.SetFlags(mailbox, []uint32{o.seqNum}, imap.DeletedFlag)
	if err != nil {
		return err
	}
	// expunge mailbox
	return client.ExpungeMailbox(mailbox)
}

// MimeType returns the mime type of the file
// In this case all messages are message/rfc822
func (o *Object) MimeType(ctx context.Context) string {
	return "message/rfc822"
}

// Update an object
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	return fmt.Errorf("Update not supported")
}

// Matches compares object to a messageInfo
func (o *Object) Matches(name string) bool {
	return o.info.Matches(name)
}

// NewFs constructs an Fs from the path.
//
// The returned Fs is the actual Fs, referenced by remote in the config
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	var security int
	var port int
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	// user
	user := opt.User
	if user == "" {
		user = currentUser
	}
	// password
	pass := ""
	if opt.AskPassword && opt.Pass == "" {
		pass = config.GetPassword("IMAP server password")
	} else {
		pass, err = obscure.Reveal(opt.Pass)
		if err != nil {
			return nil, fmt.Errorf("NewFS decrypt password: %w", err)
		}
	}
	// security
	opt.Security = strings.TrimSpace(strings.ToLower(opt.Security))
	if opt.Security == "" {
		security = securityStartTLS
	} else if opt.Security == "starttls" {
		security = securityStartTLS
	} else if opt.Security == "tls" {
		security = securityTLS
	} else if opt.Security == "none" {
		security = securityNone
	} else {
		return nil, fmt.Errorf("invalid security: %s", opt.Security)
	}
	// port, check later for SSL
	if opt.Port == "" {
		if security == securityTLS {
			port = 993
		} else {
			port = 143
		}
	} else {
		port, err = strconv.Atoi(opt.Port)
		if err != nil {
			return nil, fmt.Errorf("invalid port value: %s", opt.Port)
		}
	}
	// host
	if opt.Host == "" {
		return nil, errors.New("host is required for IMAP")
	}
	// create filesystem
	f := &Fs{
		name:       name,
		root:       root,
		opt:        *opt,
		host:       opt.Host,
		port:       port,
		user:       user,
		pass:       pass,
		security:   security,
		skipVerify: opt.InsecureSkipVerify,
	}

	f.features = (&fs.Features{}).Fill(ctx, f)
	//
	return f, nil
}

// Check the interfaces are satisfied
var (
	_ fs.Fs          = &Fs{}
	_ fs.DirMover    = &Fs{}
	_ fs.PutStreamer = &Fs{}
	_ fs.Object      = &Object{}
	_ fs.MimeTyper   = &Object{}
)
