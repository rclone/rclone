// Package adb implements an ADB (Android Debug Bridge) backend for rclone.
// It speaks the ADB wire protocol via a vendored subset of
// electricbubble/gadb (see backend/adb/internal/gadb) and does not shell
// out to the adb binary per operation.
//
// Architecture: the vendored gadb covers SYNC (push/pull) and shell: commands.
// The exec: service for range-read performance is provided by a custom
// transport in exec_transport.go in this same package.
package adb

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/backend/adb/internal/gadb"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/readers"
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
			Default:  uint16(5037),
			Help:     "The ADB server port.",
			Advanced: true,
		}, {
			Name:     "copy_links",
			Help:     "Follow symlinks and copy the pointed to item.",
			Default:  false,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.Standard |
				encoder.EncodeWin |
				encoder.EncodeBackSlash |
				encoder.EncodeLeftSpace |
				encoder.EncodeLeftPeriod |
				encoder.EncodeLeftTilde |
				encoder.EncodeLeftCrLfHtVt |
				encoder.EncodeRightSpace |
				encoder.EncodeRightPeriod |
				encoder.EncodeRightCrLfHtVt |
				encoder.EncodePercent |
				encoder.EncodeInvalidUtf8),
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Serial         string
	Host           string
	Port           uint16
	FollowSymlinks bool                 `config:"copy_links"`
	Enc            encoder.MultiEncoder `config:"encoding"`
}

// Fs represents an ADB device
type Fs struct {
	name        string       // name of this remote
	root        string       // the path we are working on
	opt         Options      // parsed options
	features    *fs.Features // optional features
	client      gadb.Client  // gadb client (value, not pointer)
	device      gadb.Device  // selected device (value, not pointer)
	statFunc    statFunc
	statFuncMu  sync.Mutex
	touchFunc   touchFunc
	touchFuncMu sync.Mutex
}

// Object describes an ADB file
type Object struct {
	fs      *Fs    // what this object is part of
	remote  string // The remote path
	size    int64
	mode    os.FileMode
	modTime time.Time
}

// devicePath returns the on-device path for a remote in rclone's standard
// encoding. All device-boundary operations (shell calls, gadb.Pull/Push,
// exec service) must construct paths through this helper so the encoder
// translation happens exactly once at the boundary. Object.remote and the
// dir/remote arguments to fs.Fs methods are kept in standard encoding to
// match rclone's API contract (e.g., backend/local does the same via
// localPath at backend/local/local.go:758).
func (f *Fs) devicePath(remote string) string {
	return path.Join(f.root, f.opt.Enc.FromStandardPath(remote))
}

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

// NewFs constructs an Fs for the given remote name and root path. The
// configured `host`/`port` ADB server must be reachable; the configured
// `serial` (or auto-selected single device) must be visible to it. If
// `root` resolves to a regular file, NewFs returns the parent directory's
// Fs along with fs.ErrorIsFile so rclone can treat the call as a single-
// file remote.
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
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
		statFunc:  (*Object).statProbe,
		touchFunc: (*Object).touchProbe,
	}
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)

	client, err := gadb.NewClientWith(opt.Host, int(opt.Port))
	if err != nil {
		return nil, fmt.Errorf("could not connect to ADB server at %s:%d: %w", opt.Host, opt.Port, err)
	}
	f.client = client

	serverVersion, err := f.client.ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("could not get ADB server version: %w", err)
	}
	fs.Debugf(f, "ADB server version: 0x%X", serverVersion)

	devices, err := f.client.DeviceList()
	if err != nil {
		return nil, fmt.Errorf("could not enumerate ADB devices: %w", err)
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("no ADB devices connected")
	}
	if len(devices) > 1 && opt.Serial == "" {
		return nil, fmt.Errorf("multiple ADB devices found; use the serial config to select a specific device (run: adb devices)")
	}

	var selected *gadb.Device
	if opt.Serial != "" {
		for i := range devices {
			if devices[i].Serial() == opt.Serial {
				selected = &devices[i]
				break
			}
		}
		if selected == nil {
			return nil, fmt.Errorf("ADB device with serial %q not found", opt.Serial)
		}
	} else {
		selected = &devices[0]
	}
	f.device = *selected

	// Follow symlinks for root paths
	entry, err := f.newEntryFollowSymlinks(ctx, "")
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
		return nil, fmt.Errorf("invalid root entry type %T", entry)
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

// List the objects and directories in dir into entries. The entries can be
// returned in any order but should be for a complete directory.
//
// dir should be "" to list the root, and should not have trailing slashes.
//
// Returns ErrDirNotFound if the directory is not found.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	p := f.devicePath(dir)
	dirEntries, err := f.device.List(p)
	if err != nil {
		return nil, fmt.Errorf("List: %w", err)
	}

	found := false
	for _, dirEntry := range dirEntries {
		found = true
		switch dirEntry.Name {
		case ".", "..":
			continue
		}
		decodedName := f.opt.Enc.ToStandardPath(dirEntry.Name)
		fsEntry, err := f.entryForDirEntry(ctx, path.Join(dir, decodedName), dirEntry, f.opt.FollowSymlinks)
		if err != nil {
			fs.Errorf(p, "Listing error: %q: %v", dirEntry.Name, err)
			return nil, err
		} else if fsEntry != nil {
			entries = append(entries, fsEntry)
		} else {
			fs.Debugf(f, "Skipping DirEntry %#v", dirEntry)
		}
	}
	if !found {
		// gadb.List returned zero entries. Two possible causes:
		//   - the directory does not exist
		//   - the directory exists but is empty AND the device's adb sync
		//     server omitted "." and ".." (rare; observed only on a few
		//     highly customized Android implementations)
		// Disambiguate via a stat. If the path is a directory, return an
		// empty listing; otherwise the directory really is missing.
		info, exists, statErr := f.statViaShell(ctx, dir)
		if statErr != nil {
			return nil, fmt.Errorf("list disambiguate: %w", statErr)
		}
		if exists && info.Mode.IsDir() {
			return nil, nil
		}
		return nil, fs.ErrorDirNotFound
	}
	return
}

func (f *Fs) entryForDirEntry(ctx context.Context, remote string, e gadb.DeviceFileInfo, followSymlinks bool) (fs.DirEntry, error) {
	o := f.newObjectWithInfo(remote, e)
	if followSymlinks && (e.Mode&os.ModeSymlink) != 0 {
		err := f.currentStatFunc()(&o, ctx)
		if err != nil {
			return nil, err
		}
	}
	if o.mode.IsDir() {
		return fs.NewDir(remote, o.modTime), nil
	}
	return &o, nil
}

// statViaShell stats a single path using the shell stat command.
// gadb has no sync-protocol Stat; this is the replacement.
// `remote` is in rclone standard encoding; converted to the on-device
// path via f.devicePath at the device boundary.
func (f *Fs) statViaShell(ctx context.Context, remote string) (gadb.DeviceFileInfo, bool, error) {
	p := f.devicePath(remote)
	output, code, err := execCommandWithExitCode(ctx, f.device, "stat -c %f,%s,%Y", p)
	if err != nil {
		if code > 0 {
			// Shell exited non-zero. The most common cause is "file does not
			// exist"; the alternatives (permission denied, malformed path) all
			// resolve to "not found at this path" from rclone's perspective.
			// Treat as not found.
			return gadb.DeviceFileInfo{}, false, nil
		}
		// code == -1: transport failure or context cancellation. Surface the
		// real error so callers (NewObject, newEntry) propagate context.Canceled
		// instead of fs.ErrorObjectNotFound, and so a dead daemon doesn't look
		// like a 404 to rclone's transfer engine.
		return gadb.DeviceFileInfo{}, false, fmt.Errorf("stat shell error: %w", err)
	}
	output = strings.TrimSpace(output)
	parts := strings.Split(output, ",")
	if len(parts) != 3 {
		return gadb.DeviceFileInfo{}, false, fmt.Errorf("stat %q invalid output %q", remote, output)
	}
	modeRaw, err := strconv.ParseUint(parts[0], 16, 32)
	if err != nil {
		return gadb.DeviceFileInfo{}, false, fmt.Errorf("stat %q invalid mode %q", remote, parts[0])
	}
	size, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return gadb.DeviceFileInfo{}, false, fmt.Errorf("stat %q invalid size %q", remote, parts[1])
	}
	modTimeEpoch, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return gadb.DeviceFileInfo{}, false, fmt.Errorf("stat %q invalid mtime %q", remote, parts[2])
	}
	info := gadb.DeviceFileInfo{
		Name:         path.Base(remote),
		Mode:         decodeEntryMode(uint32(modeRaw)),
		Size:         int64(size),
		LastModified: time.Unix(modTimeEpoch, 0),
	}
	return info, true, nil
}

func (f *Fs) newEntry(ctx context.Context, remote string) (fs.DirEntry, error) {
	return f.newEntryWithFollow(ctx, remote, f.opt.FollowSymlinks)
}

func (f *Fs) newEntryFollowSymlinks(ctx context.Context, remote string) (fs.DirEntry, error) {
	return f.newEntryWithFollow(ctx, remote, true)
}

func (f *Fs) newEntryWithFollow(ctx context.Context, remote string, followSymlinks bool) (fs.DirEntry, error) {
	info, found, err := f.statViaShell(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("stat failed: %w", err)
	}
	if !found {
		return nil, fs.ErrorObjectNotFound
	}
	return f.entryForDirEntry(ctx, remote, info, followSymlinks)
}

func (f *Fs) newObjectWithInfo(remote string, e gadb.DeviceFileInfo) Object {
	return Object{
		fs:      f,
		remote:  remote,
		size:    e.Size,
		mode:    e.Mode,
		modTime: e.LastModified,
	}
}

// NewObject finds the Object at remote.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	entry, err := f.newEntry(ctx, remote)
	if err != nil {
		return nil, err
	}
	obj, ok := entry.(fs.Object)
	if !ok {
		return nil, fs.ErrorObjectNotFound
	}
	return obj, nil
}

// Put uploads content to the remote path with the given modTime.
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o := f.newObject(src.Remote())
	err := o.Update(ctx, in, src, options...)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (f *Fs) newObject(remote string) *Object {
	return &Object{
		fs:     f,
		remote: remote,
	}
}

// Mkdir makes the directory (container, bucket). Does not fail if it already
// exists, because `mkdir -p` is itself idempotent — it returns 0 when the
// target is already a directory. If mkdir -p exits non-zero, the cause is a
// real failure (permission denied, parent missing, name conflicts with a
// non-directory) and we surface it.
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	p := f.devicePath(dir)
	output, code, err := execCommandWithExitCode(ctx, f.device, "mkdir -p", p)
	if err == nil {
		return nil
	}
	if code != 0 {
		return fmt.Errorf("mkdir %q failed with %d: %q", dir, code, output)
	}
	return fmt.Errorf("mkdir: %w", err)
}

// Rmdir removes the directory (container, bucket) if empty.
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	p := f.devicePath(dir)
	output, code, err := execCommandWithExitCode(ctx, f.device, "rmdir", p)
	if err == nil {
		return nil
	}
	if code != 0 {
		return fmt.Errorf("rmdir %q failed with %d: %q", dir, code, output)
	}
	return fmt.Errorf("rmdir: %w", err)
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// String returns a string version
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
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// Size returns the size of the file
func (o *Object) Size() int64 {
	return o.size
}

// Hash returns the selected checksum of the file
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Storable says whether this object can be stored
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets the modification time on the object
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return o.fs.currentTouchFunc()(o, ctx, t)
}

func (o *Object) stat(ctx context.Context) error {
	return o.fs.currentStatFunc()(o, ctx)
}

// Open opens the file for read. Call Close() on the returned io.ReadCloser.
//
// For full-file reads (offset=0, count=whole file), uses gadb.Pull which
// leverages the ADB SYNC protocol. For range reads (non-zero offset or partial
// count), dials the exec: service directly and runs dd with the computed block
// offset. The exec: service returns binary-clean stdout (no PTY processing),
// which is materially faster than the shell: service for binary reads —
// upstream gadb's author measured roughly an order-of-magnitude difference
// in 2019. We have not re-measured against current Android versions; the
// performance numbers in docs/content/adb.md cover end-to-end SYNC throughput.
//
// dd arithmetic (blockSize = 4096):
//
//	offsetBlocks = offset / blockSize     (block-aligned offset for dd skip=)
//	offsetRest   = offset % blockSize     (bytes to discard from first block)
//	countBlocks  = (offsetRest+count-1)/blockSize+1  (blocks needed to cover offsetRest+count bytes)
//
// adbReader handles discarding offsetRest leading bytes and upgrading io.EOF
// to io.ErrUnexpectedEOF when fewer bytes than expected are returned.
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	const blockSize = 1 << 12 // 4096

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
		return io.NopCloser(bytes.NewReader(nil)), nil
	}

	// Full-file read: use gadb.Pull (ADB SYNC protocol, no extra overhead).
	if offset == 0 && count == o.size {
		pipeReader, pipeWriter := io.Pipe()
		pullDone := make(chan struct{})
		go func() {
			defer close(pullDone)
			defer func() { _ = pipeWriter.Close() }()
			err := o.fs.device.Pull(o.fs.devicePath(o.remote), pipeWriter)
			if err != nil {
				pipeWriter.CloseWithError(err)
			}
		}()
		go func() {
			select {
			case <-ctx.Done():
				pipeWriter.CloseWithError(ctx.Err())
			case <-pullDone:
			}
		}()
		return pipeReader, nil
	}

	// Range read: dial exec: service and run dd with block-aligned offset.
	offsetBlocks := offset / blockSize
	offsetRest := offset % blockSize
	countBlocks := (offsetRest+count-1)/blockSize + 1

	filePath := o.fs.devicePath(o.remote)
	escapedPath := strings.ReplaceAll(filePath, "'", `'\''`)
	ddCmd := fmt.Sprintf("sh -c 'dd \"if=$0\" bs=%d skip=%d count=%d 2>/dev/null' '%s'",
		blockSize, offsetBlocks, countBlocks, escapedPath)

	t := newExecTransport(o.fs.opt.Host, int(o.fs.opt.Port), o.fs.opt.Serial)
	conn, err := t.Dial(ctx, ddCmd)
	if err != nil {
		return nil, fmt.Errorf("adb: exec dial for range read: %w", err)
	}

	return &adbReader{
		ReadCloser: readers.NewLimitedReadCloser(conn, count+offsetRest),
		skip:       offsetRest,
		expected:   count,
	}, nil
}

// Update uploads in to the remote path with the modTime given of the given size.
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	for _, option := range options {
		if option.Mandatory() {
			fs.Logf(option, "Unsupported mandatory option: %v", option)
		}
	}
	written, err := o.writeToFile(o.fs.devicePath(o.remote), in, 0666, src.ModTime(ctx))
	if err != nil {
		if removeErr := o.Remove(ctx); removeErr != nil {
			fs.Errorf(o, "Failed to remove partially written file: %v", removeErr)
		}
		return err
	}
	expected := src.Size()
	if expected == -1 {
		expected = written
	}
	// Post-write size verification poll: backoff schedule in milliseconds.
	// ADB SYNC writes return before the device's filesystem stat reflects
	// the new size on some devices (FUSE-backed /sdcard especially). The
	// stat-poll backoff covers the common case in <1s and the slow case
	// (large file, contended FUSE) up to ~19s total.
	postWriteStatBackoffMs := []int64{100, 250, 500, 1000, 2500, 5000, 10000}
	for _, t := range postWriteStatBackoffMs {
		err = o.stat(ctx)
		if err != nil {
			return err
		}
		if o.size == expected {
			return nil
		}
		fs.Debugf(o, "Invalid size after update, expected: %d got: %d", expected, o.size)
		select {
		case <-time.After(time.Duration(t) * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return o.stat(ctx)
}

// Remove this object
func (o *Object) Remove(ctx context.Context) error {
	p := o.fs.devicePath(o.remote)
	output, code, err := execCommandWithExitCode(ctx, o.fs.device, "rm", p)
	if err == nil {
		return nil
	}
	if code != 0 {
		return fmt.Errorf("rm %q failed with %d: %q", o.remote, code, output)
	}
	return fmt.Errorf("rm: %w", err)
}

func (o *Object) writeToFile(remotePath string, rd io.Reader, perms os.FileMode, modeTime time.Time) (written int64, err error) {
	// Count bytes with a Reader wrapper so the byte count is captured
	// synchronously by the caller without a goroutine or io.Pipe.
	cr := &countingReader{r: rd}
	err = o.fs.device.Push(cr, remotePath, modeTime, perms)
	if err != nil {
		return 0, err
	}
	return cr.n, nil
}

// countingReader wraps an io.Reader and tracks total bytes read.
type countingReader struct {
	r io.Reader
	n int64
}

// Read forwards to the wrapped reader and accumulates the byte count.
func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// statProbe runs once per Fs and picks the first stat strategy that
// succeeds: statStatL ("stat -Lc"), statRealPath ("realpath" then stat),
// or statReadLink ("readlink -f" then stat). On every tested toybox
// version (API 26-37), statStatL wins; the others are defensive fallbacks.
type statFunc func(*Object, context.Context) error

// currentStatFunc returns the current stat strategy under f.statFuncMu so
// reads pair with the probe-side write under the same mutex (Go memory
// model: writes under a lock are only guaranteed visible to readers that
// also acquire the lock). On the first call the returned function is
// statProbe itself, which acquires the same mutex when invoked; the probe
// runs once, writes the chosen strategy, and subsequent calls return that.
func (f *Fs) currentStatFunc() statFunc {
	f.statFuncMu.Lock()
	defer f.statFuncMu.Unlock()
	return f.statFunc
}

func (o *Object) statProbe(ctx context.Context) error {
	o.fs.statFuncMu.Lock()
	defer o.fs.statFuncMu.Unlock()

	for _, f := range []statFunc{
		(*Object).statStatL, (*Object).statRealPath, (*Object).statReadLink,
	} {
		err := f(o, ctx)
		if err != nil {
			fs.Debugf(o, "%s", err)
		} else {
			o.fs.statFunc = f
			return nil
		}
	}

	return fmt.Errorf("unable to resolve link target")
}

const (
	statArgLc = "-Lc"
	statArgC  = "-c"
)

func (o *Object) statStatL(ctx context.Context) error {
	return o.statStatArg(ctx, statArgLc, o.fs.devicePath(o.remote))
}

func (o *Object) statStatArg(ctx context.Context, arg, p string) error {
	output, code, err := execCommandWithExitCode(ctx, o.fs.device, fmt.Sprintf("stat %s %s", arg, "%f,%s,%Y"), p)
	output = strings.TrimSpace(output)
	if err != nil {
		if code != 0 {
			return fmt.Errorf("stat %q failed with %d: %q", o.remote, code, output)
		}
		return fmt.Errorf("stat: %w", err)
	}

	parts := strings.Split(output, ",")
	if len(parts) != 3 {
		return fmt.Errorf("stat %q invalid output %q", o.remote, output)
	}

	mode, err := strconv.ParseUint(parts[0], 16, 32)
	if err != nil {
		return fmt.Errorf("stat %q invalid output %q", o.remote, output)
	}
	size, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return fmt.Errorf("stat %q invalid output %q", o.remote, output)
	}
	modTime, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return fmt.Errorf("stat %q invalid output %q", o.remote, output)
	}

	o.size = int64(size)
	o.modTime = time.Unix(modTime, 0)
	o.mode = decodeEntryMode(uint32(mode))
	return nil
}

func (o *Object) statReadLink(ctx context.Context) error {
	p := o.fs.devicePath(o.remote)
	output, code, err := execCommandWithExitCode(ctx, o.fs.device, "readlink -f", p)
	output = strings.TrimRight(output, "\r\n")
	if err != nil {
		if code != 0 {
			return fmt.Errorf("readlink %q failed with %d: %q", o.remote, code, output)
		}
		return fmt.Errorf("readlink: %w", err)
	}
	return o.statStatArg(ctx, statArgC, output)
}

func (o *Object) statRealPath(ctx context.Context) error {
	p := o.fs.devicePath(o.remote)
	output, code, err := execCommandWithExitCode(ctx, o.fs.device, "realpath", p)
	output = strings.TrimRight(output, "\r\n")
	if err != nil {
		if code != 0 {
			return fmt.Errorf("realpath %q failed with %d: %q", o.remote, code, output)
		}
		return fmt.Errorf("realpath: %w", err)
	}
	return o.statStatArg(ctx, statArgC, output)
}

// touchProbe is a version-probed touch selector. It runs once per Fs at first
// SetModTime call, picks the first strategy that succeeds, and locks that
// choice in via f.touchFunc for all subsequent touch calls. The probe is
// mutex-guarded so concurrent first-use does not race.
//
// Strategies tried in order:
//  1. touchCmd - "touch -cmd <RFC3339>" (-c no-create, -m mtime-only, -d date)
//  2. touchCd  - "touch -cd <RFC3339>"  (-c no-create, -d date for atime+mtime)
//
// Toybox versions probed across API 26 through API 37 (toybox 0.7.3 through
// 0.8.13). All tested devices accept both -cmd and -cd uniformly, so touchCmd
// succeeds on the first call and touchCd never fires on tested toybox.
// touchCd is the defensive fallback for shells
// that reject the combined -m flag.
type touchFunc func(*Object, context.Context, time.Time) error

// currentTouchFunc returns the current touch strategy under f.touchFuncMu.
// Same rationale as currentStatFunc.
func (f *Fs) currentTouchFunc() touchFunc {
	f.touchFuncMu.Lock()
	defer f.touchFuncMu.Unlock()
	return f.touchFunc
}

func (o *Object) touchProbe(ctx context.Context, t time.Time) error {
	o.fs.touchFuncMu.Lock()
	defer o.fs.touchFuncMu.Unlock()

	for _, f := range []touchFunc{
		(*Object).touchCmd, (*Object).touchCd,
	} {
		err := f(o, ctx, t)
		if err != nil {
			fs.Debugf(o, "%s", err)
		} else {
			o.fs.touchFunc = f
			return nil
		}
	}

	return fmt.Errorf("unable to set modification time")
}

const (
	touchArgCmd = "-cmd"
	touchArgCd  = "-cd"
)

func (o *Object) touchCmd(ctx context.Context, t time.Time) error {
	return o.touchStatArg(ctx, touchArgCmd, o.fs.devicePath(o.remote), t)
}

func (o *Object) touchCd(ctx context.Context, t time.Time) error {
	return o.touchStatArg(ctx, touchArgCd, o.fs.devicePath(o.remote), t)
}

func (o *Object) touchStatArg(ctx context.Context, arg, p string, t time.Time) error {
	output, code, err := execCommandWithExitCode(ctx, o.fs.device, fmt.Sprintf("touch %s %s", arg, t.Format(time.RFC3339Nano)), p)
	output = strings.TrimSpace(output)
	if err != nil {
		if code != 0 {
			return fmt.Errorf("touch %q failed with %d: %q", o.remote, code, output)
		}
		return fmt.Errorf("touch: %w", err)
	}

	err = o.stat(ctx)
	if err != nil {
		return err
	}
	if diff, ok := checkTimeEqualWithPrecision(t, o.modTime, o.fs.Precision()); !ok {
		return fmt.Errorf("touch %q to %s was ineffective: %d", o.remote, t, diff)
	}

	return nil
}

// Move src to this remote using server-side mv.
//
// Returns fs.ErrorCantMove if src is not from this Fs.
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantMove
	}
	srcPath := srcObj.fs.devicePath(srcObj.remote)
	dstPath := f.devicePath(remote)
	// Toybox mv does not auto-create the destination's parent directory; if
	// the file is being placed under a subdirectory that does not yet exist
	// on the device, mv fails. Ensure the parent directory exists first.
	if err := f.Mkdir(ctx, path.Dir(remote)); err != nil {
		return nil, fmt.Errorf("adb mv parent mkdir: %w", err)
	}
	output, code, err := execTwoPathCmd(ctx, f.device, "mv", srcPath, dstPath)
	if err != nil || code != 0 {
		return nil, fmt.Errorf("adb mv failed (exit %d): %s: %w", code, output, err)
	}
	dst := &Object{
		fs:      f,
		remote:  remote,
		size:    srcObj.size,
		mode:    srcObj.mode,
		modTime: srcObj.modTime,
	}
	if statErr := dst.stat(ctx); statErr != nil {
		return nil, fmt.Errorf("adb mv stat verify: %w", statErr)
	}
	return dst, nil
}

// Copy src to this remote using server-side cp -p (preserves mtime).
//
// Returns fs.ErrorCantCopy if src is not from this Fs.
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantCopy
	}
	srcPath := srcObj.fs.devicePath(srcObj.remote)
	dstPath := f.devicePath(remote)
	// Toybox cp does not auto-create the destination's parent directory; if
	// the file is being placed under a subdirectory that does not yet exist
	// on the device, cp fails. Ensure the parent directory exists first.
	if err := f.Mkdir(ctx, path.Dir(remote)); err != nil {
		return nil, fmt.Errorf("adb cp parent mkdir: %w", err)
	}
	output, code, err := execTwoPathCmd(ctx, f.device, "cp -p", srcPath, dstPath)
	if err != nil || code != 0 {
		return nil, fmt.Errorf("adb cp -p failed (exit %d): %s: %w", code, output, err)
	}
	dst := &Object{fs: f, remote: remote}
	if statErr := dst.stat(ctx); statErr != nil {
		return nil, fmt.Errorf("adb cp stat verify: %w", statErr)
	}
	return dst, nil
}

// Purge deletes all the files and directories under dir.
//
// Returns nil if dir does not exist (consistent with rm -rf).
func (f *Fs) Purge(ctx context.Context, dir string) error {
	p := f.devicePath(dir)
	output, code, err := execCommandWithExitCode(ctx, f.device, "rm -rf", p)
	if err != nil && code != 0 {
		return fmt.Errorf("adb rm -rf failed (exit %d): %s: %w", code, output, err)
	}
	return nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server-side mv.
//
// Will only be called if src.Fs().Name() == f.Name().
// If the destination exists then return fs.ErrorDirExists.
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		return fs.ErrorCantDirMove
	}
	// Same ADB device? If not, fall back to file-by-file.
	if srcFs.device.Serial() != f.device.Serial() {
		return fs.ErrorCantDirMove
	}
	srcPath := srcFs.devicePath(srcRemote)
	dstPath := f.devicePath(dstRemote)
	// Per fs.DirMover contract: if dst exists, return ErrorDirExists. The
	// shell mv srcDir dstDir puts srcDir INSIDE dstDir when dstDir exists,
	// which violates the rclone semantic. Pre-check via stat.
	_, exit, _ := execCommandWithExitCode(ctx, f.device, "test -e", dstPath)
	if exit == 0 {
		return fs.ErrorDirExists
	}
	_, exit, err := execTwoPathCmd(ctx, f.device, "mv", srcPath, dstPath)
	if err != nil {
		return fmt.Errorf("adb dir mv failed: %w", err)
	}
	if exit != 0 {
		return fmt.Errorf("adb dir mv failed (exit %d)", exit)
	}
	return nil
}

// ParseDfOutput parses the two-line output of "df -k <path>" as run on an
// Android device shell. It handles both the /data/media layout (API 26-29)
// and the /dev/fuse layout (API 30+); both produce the same column shape:
//
//	Filesystem     1K-blocks    Used Available Use% Mounted on
//	/data/media     24837032 7480516  17356516  31% /storage/emulated
//
// Exported so that adb_test (an external test package) can unit-test it
// without requiring a live device.
func ParseDfOutput(out string) (total, used, free int64, err error) {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return 0, 0, 0, fmt.Errorf("adb df -k returned no data line: %q", out)
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 4 {
		return 0, 0, 0, fmt.Errorf("adb df -k unexpected format: %q", lines[1])
	}
	blocks, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("adb df -k blocks parse %q: %w", fields[1], err)
	}
	usedBlocks, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("adb df -k used parse %q: %w", fields[2], err)
	}
	avail, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("adb df -k avail parse %q: %w", fields[3], err)
	}
	return blocks * 1024, usedBlocks * 1024, avail * 1024, nil
}

// About reports filesystem usage from df -k on the configured root.
//
// Handles both /data/media (API < 30) and /dev/fuse (API 30+) formats.
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	target := f.root
	if target == "" || target == "/" {
		target = "/sdcard"
	}
	output, code, err := execCommandWithExitCode(ctx, f.device, "df -k", target)
	if err != nil || code != 0 {
		return nil, fmt.Errorf("adb df -k failed (exit %d): %s: %w", code, output, err)
	}
	total, usedBytes, free, err := ParseDfOutput(output)
	if err != nil {
		return nil, err
	}
	return &fs.Usage{
		Total: &total,
		Used:  &usedBytes,
		Free:  &free,
	}, nil
}

// ---- utility functions ----

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

// runShellWithTrailer runs cmdLine on the device via gadb shell, races it
// against ctx.Done, and parses the trailing ":EXITCODE" suffix produced by
// the standard `... ; echo :$?` wrapper used by exec wrappers below. Returns
// (output, exitCode, error). exitCode is -1 when the ADB transport fails or
// the trailer cannot be parsed. When exitCode > 0 the error is non-nil.
//
// Cancellation semantics: the gadb shell call runs on a goroutine and is raced
// against ctx.Done so the caller returns immediately on cancel. The producer
// goroutine continues until RunShellCommand returns (bounded by the gadb
// transport's SetReadDeadline, 60 seconds default), then writes to the
// buffered result channel and exits. ctx-cancel does NOT abort the in-flight
// device-side shell command; the device continues whatever work it was doing.
// This limitation is documented for users in docs/content/adb.md Limitations.
func runShellWithTrailer(ctx context.Context, d gadb.Device, cmdLine string) (string, int, error) {
	fs.Debugf("adb", "exec: %s", cmdLine)

	type result struct {
		output string
		err    error
	}
	resultCh := make(chan result, 1)
	go func() {
		out, err := d.RunShellCommand(cmdLine)
		resultCh <- result{out, err}
	}()

	var output string
	var execErr error
	select {
	case <-ctx.Done():
		return "", -1, fmt.Errorf("shell command cancelled: %w", ctx.Err())
	case r := <-resultCh:
		output = r.output
		execErr = r.err
	}

	if execErr != nil {
		return "", -1, fmt.Errorf("shell command failed: %w", execErr)
	}
	return ParseExitCodeTrailer(output)
}

// ParseExitCodeTrailer splits the output of a shell command run with the
// "...; echo :$?" wrapper into (stdout, exitCode, error). stdout is
// everything before the ":" trailer; exitCode is the parsed integer
// after. Returns a non-nil error for non-zero exit codes, parse failures,
// or a missing trailer (which usually means the shell aborted before
// echo could run).
//
// Exported so that adb_test (an external test package) can unit-test it
// without requiring a live device.
func ParseExitCodeTrailer(output string) (string, int, error) {
	idx := strings.LastIndexByte(output, ':')
	if idx == -1 {
		return output, -1, fmt.Errorf("adb shell aborted, cannot parse exit code")
	}
	codeStr := strings.TrimSpace(output[idx+1:])
	exitCode, parseErr := strconv.Atoi(codeStr)
	if parseErr != nil {
		// Regression for malformed exit code trailer: a non-numeric "$?"
		// must surface as an error, not silently mask as success.
		return output, -1, fmt.Errorf("adb shell trailer parse %q: %w", codeStr, parseErr)
	}
	if exitCode != 0 {
		return output[:idx], exitCode, fmt.Errorf("exit code %d", exitCode)
	}
	return output[:idx], 0, nil
}

// execTwoPathCmd runs a shell op with two path arguments, both single-quote
// escaped. Used by Move and Copy where srcPath and dstPath both come from
// rclone and may contain spaces or shell metacharacters. Wraps the command
// as `sh -c 'OP "$0" "$1"; echo :$?' 'SRC' 'DST'`. See runShellWithTrailer
// for return-value and cancellation semantics.
func execTwoPathCmd(ctx context.Context, d gadb.Device, op, src, dst string) (string, int, error) {
	esc := func(s string) string { return strings.ReplaceAll(s, "'", `'\''`) }
	cmdLine := fmt.Sprintf("sh -c '%s \"$0\" \"$1\"; echo :$?' '%s' '%s'", op, esc(src), esc(dst))
	return runShellWithTrailer(ctx, d, cmdLine)
}

// execCommandWithExitCode runs a shell command on the ADB device and captures
// both output and exit code. Wraps the command as `sh -c 'CMD "$0"; echo :$?' 'ARG'`.
// Only `arg` is single-quote escaped; `cmd` is interpolated raw, so it must
// not contain user-controlled data. Callers that need to escape both a command
// verb and a path argument should use execTwoPathCmd. See runShellWithTrailer
// for return-value and cancellation semantics.
func execCommandWithExitCode(ctx context.Context, d gadb.Device, cmd string, arg string) (string, int, error) {
	cmdLine := fmt.Sprintf("sh -c '%s \"$0\"; echo :$?' '%s'", cmd, strings.ReplaceAll(arg, "'", `'\''`))
	return runShellWithTrailer(ctx, d, cmdLine)
}

// Compile-time interface satisfaction asserts.
// These ensure that a future signature drift in fs.Fs or fs.Object surfaces at
// build time rather than at runtime when rclone tries to use the interface.
var (
	_ fs.Fs       = (*Fs)(nil)
	_ fs.Mover    = (*Fs)(nil)
	_ fs.Copier   = (*Fs)(nil)
	_ fs.Purger   = (*Fs)(nil)
	_ fs.Abouter  = (*Fs)(nil)
	_ fs.DirMover = (*Fs)(nil)
	_ fs.Object   = (*Object)(nil)
)
