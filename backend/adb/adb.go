package adb

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config/configmap"
	"github.com/ncw/rclone/fs/config/configstruct"
	"github.com/ncw/rclone/fs/hash"
	"github.com/ncw/rclone/lib/readers"
	"github.com/pkg/errors"
	adb "github.com/thinkhy/go-adb"
	"github.com/thinkhy/go-adb/wire"
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "adb",
		Description: "Android Debug Bridge",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "serial",
			Help:     "The device serial to use. Leave empty for auto selection.",
			Advanced: true,
		}, {
			Name:     "host",
			Default:  "localhost",
			Help:     "The ADB server host.",
			Advanced: true,
		}, {
			Name:     "port",
			Default:  5037,
			Help:     "The ADB server port.",
			Advanced: true,
		}, {
			Name:     "executable",
			Help:     "The ADB executable path.",
			Advanced: true,
		}, {
			Name:     "copy_links",
			Help:     "Follow symlinks and copy the pointed to item.",
			Default:  false,
			Advanced: true,
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Serial         string
	Host           string
	Port           uint16
	Executable     string
	FollowSymlinks bool `config:"copy_links"`
}

// Fs represents a adb device
type Fs struct {
	name        string       // name of this remote
	root        string       // the path we are working on
	opt         Options      // parsed options
	features    *fs.Features // optional features
	client      *adb.Adb
	device      *execDevice
	statFunc    statFunc
	statFuncMu  sync.Mutex
	touchFunc   touchFunc
	touchFuncMu sync.Mutex
}

// Object describes a adb file
type Object struct {
	fs      *Fs    // what this object is part of
	remote  string // The remote path
	size    int64
	mode    os.FileMode
	modTime time.Time
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
	return fmt.Sprintf("ADB root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// NewFs constructs an Fs from the path, container:path
func NewFs(name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	if root == "" {
		root = "/"
	}

	f := &Fs{
		name:      name,
		root:      root,
		opt:       *opt,
		statFunc:  (*Object).statTry,
		touchFunc: (*Object).touchTry,
	}
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
	}).Fill(f)

	f.client, err = adb.NewWithConfig(adb.ServerConfig{
		Host:      opt.Host,
		Port:      int(opt.Port),
		PathToAdb: opt.Executable,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "Could not configure ADB server")
	}
	err = f.client.StartServer()
	if err != nil {
		return nil, errors.Wrapf(err, "Could not start ADB server")
	}

	serverVersion, err := f.client.ServerVersion()
	if err != nil {
		return nil, errors.Wrapf(err, "Could not get ADB server version")
	}
	fs.Debugf(f, "ADB server version: 0x%X", serverVersion)

	serials, err := f.client.ListDeviceSerials()
	if err != nil {
		return nil, errors.Wrapf(err, "Could not get ADB devices")
	}
	descriptor := adb.AnyDevice()
	if opt.Serial != "" {
		descriptor = adb.DeviceWithSerial(opt.Serial)
	}
	if len(serials) > 1 && opt.Serial == "" {
		return nil, errors.New("Multiple ADB devices found. Use the serial config to select a specific device")
	}
	f.device = &execDevice{f.client.Device(descriptor)}

	// follow symlinks for root pathes
	entry, err := f.newEntryFollowSymlinks("")
	switch err {
	case nil:
	case fs.ErrorObjectNotFound:
	default:
		return nil, err
	}
	switch entry.(type) {
	case fs.Object:
		f.root = path.Dir(f.root)
		return f, fs.ErrorIsFile
	case nil:
		return f, nil
	case fs.Directory:
		return f, nil
	default:
		return nil, errors.Errorf("Invalid root entry type %t", entry)
	}
}

// Precision of the object storage system
func (f *Fs) Precision() time.Duration {
	return 1 * time.Second
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
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
func (f *Fs) List(dir string) (entries fs.DirEntries, err error) {
	p := path.Join(f.root, dir)
	dirEntries, err := f.device.ListDirEntries(p)
	if err != nil {
		return nil, errors.Wrap(err, "ListDirEntries")
	}

	defer fs.CheckClose(dirEntries, &err)

	found := false
	for dirEntries.Next() {
		found = true
		dirEntry := dirEntries.Entry()
		switch dirEntry.Name {
		case ".", "..":
			continue
		}
		fsEntry, err := f.entryForDirEntry(path.Join(dir, dirEntry.Name), dirEntry, f.opt.FollowSymlinks)
		if err != nil {
			fs.Errorf(p, "Listing error: %q: %v", dirEntry.Name, err)
			return nil, err
		} else if fsEntry != nil {
			entries = append(entries, fsEntry)
		} else {
			fs.Debugf(f, "Skipping DirEntry %#v", dirEntry)
		}
	}
	err = dirEntries.Err()
	if err != nil {
		return nil, errors.Wrap(err, "ListDirEntries")
	}
	if !found {
		return nil, fs.ErrorDirNotFound
	}
	return
}

func (f *Fs) entryForDirEntry(remote string, e *adb.DirEntry, followSymlinks bool) (fs.DirEntry, error) {
	o := f.newObjectWithInfo(remote, e)
	// Follow symlinks if required
	if followSymlinks && (e.Mode&os.ModeSymlink) != 0 {
		err := f.statFunc(&o)
		if err != nil {
			return nil, err
		}
	}
	if o.mode.IsDir() {
		return fs.NewDir(remote, o.modTime), nil
	}
	return &o, nil
}

func (f *Fs) newEntry(remote string) (fs.DirEntry, error) {
	return f.newEntryWithFollow(remote, f.opt.FollowSymlinks)
}
func (f *Fs) newEntryFollowSymlinks(remote string) (fs.DirEntry, error) {
	return f.newEntryWithFollow(remote, true)
}
func (f *Fs) newEntryWithFollow(remote string, followSymlinks bool) (fs.DirEntry, error) {
	entry, err := f.device.Stat(path.Join(f.root, remote))
	if err != nil {
		if adb.HasErrCode(err, adb.FileNoExistError) {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, errors.Wrapf(err, "Stat failed")
	}
	return f.entryForDirEntry(remote, entry, followSymlinks)
}

func (f *Fs) newObjectWithInfo(remote string, e *adb.DirEntry) Object {
	return Object{
		fs:      f,
		remote:  remote,
		size:    int64(e.Size),
		mode:    e.Mode,
		modTime: e.ModifiedAt,
	}
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error ErrorObjectNotFound.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	entry, err := f.newEntry(remote)
	if err != nil {
		return nil, err
	}
	obj, ok := entry.(fs.Object)
	if !ok {
		return nil, fs.ErrorObjectNotFound
	}
	return obj, nil
}

// Put in to the remote path with the modTime given of the given size
//
// When called from outside a Fs by rclone, src.Size() will always be >= 0.
// But for unknown-sized objects (indicated by src.Size() == -1), Put should either
// return an error or upload it properly (rather than e.g. calling panic).
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	// Temporary Object under construction - info filled in by Update()
	o := f.newObject(remote)
	err := o.Update(in, src, options...)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// newObject makes a half completed Object
func (f *Fs) newObject(remote string) *Object {
	return &Object{
		fs:     f,
		remote: remote,
	}
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(dir string) error {
	p := path.Join(f.root, dir)
	output, code, err := f.device.execCommandWithExitCode("mkdir -p", p)
	switch err := err.(type) {
	case nil:
		return nil
	case adb.ShellExitError:
		entry, _ := f.newEntry(p)
		if _, ok := entry.(fs.Directory); ok {
			return nil
		}
		return errors.Errorf("mkdir %q failed with %d: %q", dir, code, output)
	default:
		return errors.Wrap(err, "mkdir")
	}
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(dir string) error {
	p := path.Join(f.root, dir)
	output, code, err := f.device.execCommandWithExitCode("rmdir", p)
	switch err := err.(type) {
	case nil:
		return nil
	case adb.ShellExitError:
		return errors.Errorf("rmdir %q failed with %d: %q", dir, code, output)
	default:
		return errors.Wrap(err, "rmdir")
	}
}

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

// ModTime returns the modification date of the file
// It should return a best guess if one isn't available
func (o *Object) ModTime() time.Time {
	return o.modTime
}

// Size returns the size of the file
func (o *Object) Size() int64 {
	return o.size
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Storable says whether this object can be stored
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets the metadata on the object to set the modification date
func (o *Object) SetModTime(t time.Time) error {
	return o.fs.touchFunc(o, t)
}

func (o *Object) stat() error {
	return o.statStatArg(statArgC, path.Join(o.fs.root, o.remote))
}

func (o *Object) setMetadata(entry *adb.DirEntry) {
	// Don't overwrite the values if we don't need to
	// this avoids upsetting the race detector
	if o.size != int64(entry.Size) {
		o.size = int64(entry.Size)
	}
	if !o.modTime.Equal(entry.ModifiedAt) {
		o.modTime = entry.ModifiedAt
	}
	if o.mode != entry.Mode {
		o.mode = decodeEntryMode(uint32(entry.Mode))
	}
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
func (o *Object) Open(options ...fs.OpenOption) (io.ReadCloser, error) {
	const blockSize = 1 << 12

	var offset, count int64 = 0, -1
	for _, option := range options {
		switch x := option.(type) {
		case *fs.RangeOption:
			offset, count = x.Decode(o.size)
		case *fs.SeekOption:
			offset = x.Offset
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}
	if offset > o.size {
		offset = o.size
	}
	if count < 0 {
		count = o.size - offset
	} else if count+offset > o.size {
		count = o.size - offset
	}
	fs.Debugf(o, "Open: remote: %q offset: %d count: %d", o.remote, offset, count)

	if count == 0 {
		return ioutil.NopCloser(bytes.NewReader(nil)), nil
	}
	offsetBlocks, offsetRest := offset/blockSize, offset%blockSize
	countBlocks := (count-1)/blockSize + 1

	conn, err := o.fs.device.execCommand(fmt.Sprintf("sh -c 'dd \"if=$0\" bs=%d skip=%d count=%d 2>/dev/null'", blockSize, offsetBlocks, countBlocks), path.Join(o.fs.root, o.remote))
	if err != nil {
		return nil, err
	}

	return &adbReader{
		ReadCloser: readers.NewLimitedReadCloser(conn, count+offsetRest),
		skip:       offsetRest,
		expected:   count,
	}, nil
}

// Update in to the object with the modTime given of the given size
//
// When called from outside a Fs by rclone, src.Size() will always be >= 0.
// But for unknown-sized objects (indicated by src.Size() == -1), Upload should either
// return an error or update the object properly (rather than e.g. calling panic).
func (o *Object) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	for _, option := range options {
		if option.Mandatory() {
			fs.Logf(option, "Unsupported mandatory option: %v", option)
		}
	}
	_, err := o.writeToFile(path.Join(o.fs.root, o.remote), in, 0666, src.ModTime())
	if err != nil {
		if removeErr := o.Remove(); removeErr != nil {
			fs.Errorf(o, "Failed to remove partially written file: %v", removeErr)
		}
		return err
	}
	return o.stat()
}

// Remove this object
func (o *Object) Remove() error {
	p := path.Join(o.fs.root, o.remote)
	output, code, err := o.fs.device.execCommandWithExitCode("rm", p)
	switch err := err.(type) {
	case nil:
		return nil
	case adb.ShellExitError:
		return errors.Errorf("rm %q failed with %d: %q", o.remote, code, output)
	default:
		return errors.Wrap(err, "rm")
	}
}

func (o *Object) writeToFile(path string, rd io.Reader, perms os.FileMode, modeTime time.Time) (written int64, err error) {
	dst, err := o.fs.device.OpenWrite(path, perms, modeTime)
	if err != nil {
		return
	}
	defer fs.CheckClose(dst, &err)
	return io.Copy(dst, rd)
}

type statFunc func(*Object) error

func (o *Object) statTry() error {
	o.fs.statFuncMu.Lock()
	defer o.fs.statFuncMu.Unlock()

	for _, f := range []statFunc{
		(*Object).statStatL, (*Object).statRealPath, (*Object).statReadLink,
	} {
		err := f(o)
		if err != nil {
			fs.Debugf(o, "%s", err)
		} else {
			o.fs.statFunc = f
			return nil
		}
	}

	return errors.Errorf("unable to resolve link target")
}

const (
	statArgLc = "-Lc"
	statArgC  = "-c"
)

func (o *Object) statStatL() error {
	return o.statStatArg(statArgLc, path.Join(o.fs.root, o.remote))
}

func (o *Object) statStatArg(arg, path string) error {
	output, code, err := o.fs.device.execCommandWithExitCode(fmt.Sprintf("stat %s %s", arg, "%f,%s,%Y"), path)
	output = strings.TrimSpace(output)
	switch err := err.(type) {
	case nil:
	case adb.ShellExitError:
		return errors.Errorf("stat %q failed with %d: %q", o.remote, code, output)
	default:
		return errors.Wrap(err, "stat")
	}

	parts := strings.Split(output, ",")
	if len(parts) != 3 {
		return errors.Errorf("stat %q invalid output %q", o.remote, output)
	}

	mode, err := strconv.ParseUint(parts[0], 16, 32)
	if err != nil {
		return errors.Errorf("stat %q invalid output %q", o.remote, output)
	}
	size, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return errors.Errorf("stat %q invalid output %q", o.remote, output)
	}
	modTime, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return errors.Errorf("stat %q invalid output %q", o.remote, output)
	}

	o.size = int64(size)
	o.modTime = time.Unix(modTime, 0)
	o.mode = decodeEntryMode(uint32(mode))
	return nil
}

func (o *Object) statReadLink() error {
	p := path.Join(o.fs.root, o.remote)
	output, code, err := o.fs.device.execCommandWithExitCode("readlink -f", p)
	output = strings.TrimSuffix(output, "\n")
	switch err := err.(type) {
	case nil:
	case adb.ShellExitError:
		return errors.Errorf("readlink %q failed with %d: %q", o.remote, code, output)
	default:
		return errors.Wrap(err, "readlink")
	}
	return o.statStatArg(statArgC, output)
}
func (o *Object) statRealPath() error {
	p := path.Join(o.fs.root, o.remote)
	output, code, err := o.fs.device.execCommandWithExitCode("realpath", p)
	output = strings.TrimSuffix(output, "\n")
	switch err := err.(type) {
	case nil:
	case adb.ShellExitError:
		return errors.Errorf("realpath %q failed with %d: %q", o.remote, code, output)
	default:
		return errors.Wrap(err, "realpath")
	}
	return o.statStatArg(statArgC, output)
}

type touchFunc func(*Object, time.Time) error

func (o *Object) touchTry(t time.Time) error {
	o.fs.touchFuncMu.Lock()
	defer o.fs.touchFuncMu.Unlock()

	for _, f := range []touchFunc{
		(*Object).touchCmd, (*Object).touchCd,
	} {
		err := f(o, t)
		if err != nil {
			fs.Debugf(o, "%s", err)
		} else {
			o.fs.touchFunc = f
			return nil
		}
	}

	return errors.Errorf("unable to resolve link target")
}

const (
	touchArgCmd = "-cmd"
	touchArgCd  = "-cd"
)

func (o *Object) touchCmd(t time.Time) error {
	return o.touchStatArg(touchArgCmd, path.Join(o.fs.root, o.remote), t)
}
func (o *Object) touchCd(t time.Time) error {
	return o.touchStatArg(touchArgCd, path.Join(o.fs.root, o.remote), t)
}

func (o *Object) touchStatArg(arg, path string, t time.Time) error {
	output, code, err := o.fs.device.execCommandWithExitCode(fmt.Sprintf("touch %s %s", arg, t.Format(time.RFC3339Nano)), path)
	output = strings.TrimSpace(output)
	switch err := err.(type) {
	case nil:
	case adb.ShellExitError:
		return errors.Errorf("touch %q failed with %d: %q", o.remote, code, output)
	default:
		return errors.Wrap(err, "touch")
	}

	err = o.stat()
	if err != nil {
		return err
	}
	if diff, ok := checkTimeEqualWithPrecision(t, o.modTime, o.fs.Precision()); !ok {
		return errors.Errorf("touch %q to %s was ineffective: %d", o.remote, t, diff)
	}

	return nil
}

func checkTimeEqualWithPrecision(t0, t1 time.Time, precision time.Duration) (time.Duration, bool) {
	dt := t0.Sub(t1)
	if dt >= precision || dt <= -precision {
		return dt, false
	}
	return dt, true
}

func decodeEntryMode(entryMode uint32) os.FileMode {
	const (
		unixIFBLK  = 0x6000
		unixIFMT   = 0xf000
		unixIFCHR  = 0x2000
		unixIFDIR  = 0x4000
		unixIFIFO  = 0x1000
		unixIFLNK  = 0xa000
		unixIFREG  = 0x8000
		unixIFSOCK = 0xc000
		unixISGID  = 0x400
		unixISUID  = 0x800
		unixISVTX  = 0x200
	)

	mode := os.FileMode(entryMode & 0777)
	switch entryMode & unixIFMT {
	case unixIFBLK:
		mode |= os.ModeDevice
	case unixIFCHR:
		mode |= os.ModeDevice | os.ModeCharDevice
	case unixIFDIR:
		mode |= os.ModeDir
	case unixIFIFO:
		mode |= os.ModeNamedPipe
	case unixIFLNK:
		mode |= os.ModeSymlink
	case unixIFREG:
		// nothing to do
	case unixIFSOCK:
		mode |= os.ModeSocket
	}
	if entryMode&unixISGID != 0 {
		mode |= os.ModeSetgid
	}
	if entryMode&unixISUID != 0 {
		mode |= os.ModeSetuid
	}
	if entryMode&unixISVTX != 0 {
		mode |= os.ModeSticky
	}
	return mode
}

type execDevice struct {
	*adb.Device
}

func (d *execDevice) execCommandWithExitCode(cmd string, arg string) (string, int, error) {
	cmdLine := fmt.Sprintf("sh -c '%s \"$0\"; echo :$?' '%s'", cmd, strings.Replace(arg, "'", "'\\''", -1))
	fs.Debugf("adb", "exec: %s", cmdLine)
	conn, err := d.execCommand(cmdLine)
	if err != nil {
		return "", -1, err
	}

	resp, err := conn.ReadUntilEof()
	if err != nil {
		return "", -1, errors.Wrap(err, "ExecCommand")
	}

	outStr := string(resp)
	idx := strings.LastIndexByte(outStr, ':')
	if idx == -1 {
		return outStr, -1, fmt.Errorf("adb shell aborted, can not parse exit code")
	}
	exitCode, _ := strconv.Atoi(strings.TrimSpace(outStr[idx+1:]))
	if exitCode != 0 {
		err = adb.ShellExitError{Command: cmdLine, ExitCode: exitCode}
	}
	return outStr[:idx], exitCode, err
}

func (d *execDevice) execCommand(cmd string, args ...string) (*wire.Conn, error) {
	cmd = prepareCommandLineEscaped(cmd, args...)
	conn, err := d.Dial()
	if err != nil {
		return nil, errors.Wrap(err, "ExecCommand")
	}
	defer func() {
		if err != nil && conn != nil {
			_ = conn.Close()
		}
	}()

	req := fmt.Sprintf("exec:%s", cmd)

	if err = conn.SendMessage([]byte(req)); err != nil {
		return nil, errors.Wrap(err, "ExecCommand")
	}
	if _, err = conn.ReadStatus(req); err != nil {
		return nil, errors.Wrap(err, "ExecCommand")
	}
	return conn, nil
}

func prepareCommandLineEscaped(cmd string, args ...string) string {
	for i, arg := range args {
		args[i] = fmt.Sprintf("'%s'", strings.Replace(arg, "'", "'\\''", -1))
	}

	// Prepend the command to the args array.
	if len(args) > 0 {
		cmd = fmt.Sprintf("%s %s", cmd, strings.Join(args, " "))
	}

	return cmd
}

type adbReader struct {
	io.ReadCloser
	skip     int64
	read     int64
	expected int64
}

func (r *adbReader) Read(b []byte) (n int, err error) {
	n, err = r.ReadCloser.Read(b)
	if s := r.skip; n > 0 && s > 0 {
		_n := int64(n)
		if _n <= s {
			r.skip -= _n
			return r.Read(b)
		}
		r.skip = 0
		copy(b, b[s:n])
		n -= int(s)
	}
	r.read += int64(n)
	if err == io.EOF && r.read < r.expected {
		fs.Debugf("adb", "Read: read: %d expected: %d n: %d", r.read, r.expected, n)
		return n, io.ErrUnexpectedEOF
	}
	return n, err
}
