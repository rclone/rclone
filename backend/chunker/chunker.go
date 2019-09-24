// Package chunker provides wrappers for Fs and Object which split large files in chunks
package chunker

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	gohash "hash"
	"io"
	"io/ioutil"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
)

const (
	// optimizeFirstChunk enables the following Put optimization:
	// if a single chunk is expected, put the first chunk using base target
	// name instead of temporary name, thus avoiding extra rename operation.
	// WARNING: this optimization is not transaction safe!
	optimizeFirstChunk = false

	// Normally metadata is a small (100-200 bytes) piece of JSON.
	// Valid metadata size should not exceed this limit.
	maxMetaDataSize = 199

	metaDataVersion = 1
)

// Formatting of temporary chunk names. Temporary suffix *follows* chunk
// suffix to prevent name collisions between chunks and ordinary files.
var (
	tempChunkFormat = `%s..tmp_%010d`
	tempChunkRegexp = regexp.MustCompile(`^(.+)\.\.tmp_([0-9]{10,19})$`)
)

// Note: metadata logic is tightly coupled with chunker code in many
// places of the code, eg. in checks whether a file can have meta object
// or is eligible for chunking.
// If more metadata formats (or versions of a format) are added in future,
// it may be advisable to factor it into a "metadata strategy" interface
// similar to chunkingReader or linearReader below.

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "chunker",
		Description: "Transparently chunk/split large files",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "remote",
			Required: true,
			Help: `Remote to chunk/unchunk.
Normally should contain a ':' and a path, eg "myremote:path/to/dir",
"myremote:bucket" or maybe "myremote:" (not recommended).`,
		}, {
			Name:     "chunk_size",
			Advanced: false,
			Default:  fs.SizeSuffix(2147483648), // 2GB
			Help:     `Files larger than chunk size will be split in chunks.`,
		}, {
			Name:     "name_format",
			Advanced: true,
			Default:  `*.rclone_chunk.###`,
			Help: `String format of chunk file names.
The two placeholders are: base file name (*) and chunk number (#...).
There must be one and only one asterisk and one or more consecutive hash characters.
If chunk number has less digits than the number of hashes, it is left-padded by zeros.
If there are more digits in the number, they are left as is.
Possible chunk files are ignored if their name does not match given format.`,
		}, {
			Name:     "start_from",
			Advanced: true,
			Default:  1,
			Help: `Minimum valid chunk number. Usually 0 or 1.
By default chunk numbers start from 1.`,
		}, {
			Name:     "meta_format",
			Advanced: true,
			Default:  "simplejson",
			Help: `Format of the metadata object or "none". By default "simplejson".
Metadata is a small JSON file named after the composite file.`,
			Examples: []fs.OptionExample{{
				Value: "none",
				Help:  `Do not use metadata files at all. Requires hash type "none".`,
			}, {
				Value: "simplejson",
				Help: `Simple JSON supports hash sums and chunk validation.
It has the following fields: size, nchunks, md5, sha1.`,
			}},
		}, {
			Name:     "hash_type",
			Advanced: false,
			Default:  "md5",
			Help:     `Choose how chunker handles hash sums.`,
			Examples: []fs.OptionExample{{
				Value: "none",
				Help: `Chunker can pass any hash supported by wrapped remote
for a single-chunk file but returns nothing otherwise.`,
			}, {
				Value: "md5",
				Help:  `MD5 for multi-chunk files. Requires "simplejson".`,
			}, {
				Value: "sha1",
				Help:  `SHA1 for multi-chunk files. Requires "simplejson".`,
			}, {
				Value: "md5quick",
				Help: `Copying a file to chunker will request MD5 from the source
falling back to SHA1 if unsupported. Requires "simplejson".`,
			}, {
				Value: "sha1quick",
				Help:  `Similar to "md5quick" but prefers SHA1 over MD5. Requires "simplejson".`,
			}},
		}, {
			Name:     "fail_on_bad_chunks",
			Advanced: true,
			Default:  false,
			Help: `The list command might encounter files with missinng or invalid chunks.
This boolean flag tells what rclone should do in such cases.`,
			Examples: []fs.OptionExample{
				{
					Value: "true",
					Help:  "Fail with error.",
				}, {
					Value: "false",
					Help:  "Silently ignore invalid object.",
				},
			},
		}},
	})
}

// NewFs constructs an Fs from the path, container:path
func NewFs(name, rpath string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	if opt.StartFrom < 0 {
		return nil, errors.New("start_from must be non-negative")
	}

	remote := opt.Remote
	if strings.HasPrefix(remote, name+":") {
		return nil, errors.New("can't point remote at itself - check the value of the remote setting")
	}

	baseInfo, baseName, basePath, baseConfig, err := fs.ConfigFs(remote)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse remote %q to wrap", remote)
	}
	// Look for a file first
	remotePath := fspath.JoinRootPath(basePath, rpath)
	baseFs, err := baseInfo.NewFs(baseName, remotePath, baseConfig)
	if err != fs.ErrorIsFile && err != nil {
		return nil, errors.Wrapf(err, "failed to make remote %s:%q to wrap", baseName, remotePath)
	}
	if !operations.CanServerSideMove(baseFs) {
		return nil, errors.New("can't use chunker on a backend which doesn't support server side move or copy")
	}

	f := &Fs{
		base: baseFs,
		name: name,
		root: rpath,
		opt:  *opt,
	}

	switch opt.MetaFormat {
	case "none":
		f.useMeta = false
	case "simplejson":
		f.useMeta = true
	default:
		return nil, fmt.Errorf("unsupported meta format '%s'", opt.MetaFormat)
	}

	requireMetaHash := true
	switch opt.HashType {
	case "none":
		requireMetaHash = false
	case "md5":
		f.useMD5 = true
	case "sha1":
		f.useSHA1 = true
	case "md5quick":
		f.useMD5 = true
		f.quickHash = true
	case "sha1quick":
		f.useSHA1 = true
		f.quickHash = true
	default:
		return nil, fmt.Errorf("unsupported hash type '%s'", opt.HashType)
	}
	if requireMetaHash && opt.MetaFormat != "simplejson" {
		return nil, fmt.Errorf("hash type '%s' requires meta format 'simplejson'", opt.HashType)
	}

	if err := f.parseNameFormat(opt.NameFormat); err != nil {
		return nil, fmt.Errorf("invalid name format '%s': %v", opt.NameFormat, err)
	}

	// Handle the tricky case detected by FsMkdir/FsPutFiles/FsIsFile
	// when `rpath` points to a composite multi-chunk file without metadata,
	// i.e. `rpath` does not exist in the wrapped remote, but chunker
	// detects a composite file because it finds the first chunk!
	// (yet can't satisfy fstest.CheckListing, will ignore)
	if err == nil && !f.useMeta && strings.Contains(rpath, "/") {
		firstChunkPath := f.makeChunkName(remotePath, 0, -1)
		_, testErr := baseInfo.NewFs(baseName, firstChunkPath, baseConfig)
		if testErr == fs.ErrorIsFile {
			err = testErr
		}
	}

	// Note 1: the features here are ones we could support, and they are
	// ANDed with the ones from wrappedFs.
	// Note 2: features.Fill() points features.PutStream to our PutStream,
	// but features.Mask() will nullify it if wrappedFs does not have it.
	f.features = (&fs.Features{
		CaseInsensitive:         true,
		DuplicateFiles:          true,
		ReadMimeType:            true,
		WriteMimeType:           true,
		BucketBased:             true,
		CanHaveEmptyDirectories: true,
		ServerSideAcrossConfigs: true,
	}).Fill(f).Mask(baseFs).WrapsFs(f, baseFs)

	return f, err
}

// Options defines the configuration for this backend
type Options struct {
	Remote          string        `config:"remote"`
	ChunkSize       fs.SizeSuffix `config:"chunk_size"`
	NameFormat      string        `config:"name_format"`
	StartFrom       int           `config:"start_from"`
	MetaFormat      string        `config:"meta_format"`
	HashType        string        `config:"hash_type"`
	FailOnBadChunks bool          `config:"fail_on_bad_chunks"`
}

// Fs represents a wrapped fs.Fs
type Fs struct {
	base       fs.Fs
	wrapper    fs.Fs
	name       string
	root       string
	useMeta    bool
	useMD5     bool // mutually exclusive with useSHA1
	useSHA1    bool // mutually exclusive with useMD5
	quickHash  bool
	nameFormat string
	nameRegexp *regexp.Regexp
	opt        Options
	features   *fs.Features // optional features
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// String returns a description of the FS
func (f *Fs) String() string {
	return fmt.Sprintf("Chunked '%s:%s'", f.name, f.root)
}

// parseNameFormat converts pattern-based name format into Printf format and Regexp
func (f *Fs) parseNameFormat(pattern string) error {
	if strings.Count(pattern, "*") != 1 {
		return errors.New("pattern must have exactly one asterisk (*)")
	}
	numDigits := strings.Count(pattern, "#")
	if numDigits < 1 {
		return errors.New("pattern must have a hash character (#)")
	}
	if strings.Index(pattern, "*") > strings.Index(pattern, "#") {
		return errors.New("asterisk (*) in pattern must come before hashes (#)")
	}
	if ok, _ := regexp.MatchString("^[^#]*[#]+[^#]*$", pattern); !ok {
		return errors.New("hashes (#) in pattern must be consecutive")
	}

	reHashes := regexp.MustCompile("[#]+")
	strRegex := regexp.QuoteMeta(pattern)
	reDigits := "([0-9]+)"
	if numDigits > 1 {
		reDigits = fmt.Sprintf("([0-9]{%d,})", numDigits)
	}
	strRegex = reHashes.ReplaceAllLiteralString(strRegex, reDigits)
	strRegex = strings.Replace(strRegex, "\\*", "(.+)", -1)
	f.nameRegexp = regexp.MustCompile("^" + strRegex + "$")

	strFmt := strings.Replace(pattern, "%", "%%", -1) // escape percent signs for name format
	fmtDigits := "%d"
	if numDigits > 1 {
		fmtDigits = fmt.Sprintf("%%0%dd", numDigits)
	}
	strFmt = reHashes.ReplaceAllLiteralString(strFmt, fmtDigits)
	f.nameFormat = strings.Replace(strFmt, "*", "%s", -1)
	return nil
}

// makeChunkName produces a chunk name for a given file name.
// chunkNo must be a zero based index.
// A negative tempNo (eg. -1) indicates normal chunk, else temporary.
func (f *Fs) makeChunkName(mainName string, chunkNo int, tempNo int64) string {
	name := mainName
	name = fmt.Sprintf(f.nameFormat, name, chunkNo+f.opt.StartFrom)
	if tempNo < 0 {
		return name
	}
	return fmt.Sprintf(tempChunkFormat, name, tempNo)
}

// parseChunkName validates and parses a given chunk name.
// Returned mainName is "" if it's not a chunk name.
// Returned chunkNo is zero based.
// Returned tempNo is -1 for a normal chunk
// or non-negative integer for a temporary chunk.
func (f *Fs) parseChunkName(name string) (mainName string, chunkNo int, tempNo int64) {
	var err error
	chunkMatch := f.nameRegexp.FindStringSubmatchIndex(name)
	chunkName := name
	tempNo = -1
	if chunkMatch == nil {
		tempMatch := tempChunkRegexp.FindStringSubmatchIndex(name)
		if tempMatch == nil {
			return "", -1, -1
		}
		chunkName = name[tempMatch[2]:tempMatch[3]]
		tempNo, err = strconv.ParseInt(name[tempMatch[4]:tempMatch[5]], 10, 64)
		if err != nil {
			return "", -1, -1
		}
		chunkMatch = f.nameRegexp.FindStringSubmatchIndex(chunkName)
		if chunkMatch == nil {
			return "", -1, -1
		}
	}
	mainName = chunkName[chunkMatch[2]:chunkMatch[3]]
	chunkNo, err = strconv.Atoi(chunkName[chunkMatch[4]:chunkMatch[5]])
	if err != nil {
		return "", -1, -1
	}
	chunkNo -= f.opt.StartFrom
	if chunkNo < 0 {
		fs.Infof(f, "invalid chunk number in name %q", name)
		return "", -1, -1
	}
	return mainName, chunkNo, tempNo
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
//
// Commands normally cleanup all temporary chunks in case of a failure.
// However, if rclone dies unexpectedly, it can leave behind a bunch of
// hidden temporary chunks. List and its underlying chunkEntries()
// silently skip all temporary chunks in the directory. It's okay if
// they belong to an unfinished command running in parallel.
//
// However, there is no way to discover dead temporary chunks a.t.m.
// As a workaround users can use `purge` to forcibly remove the whole
// directory together with dead chunks.
// In future a flag named like `--chunker-list-hidden` may be added to
// rclone that will tell List to reveal hidden chunks.
//
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	entries, err = f.base.List(ctx, dir)
	if err != nil {
		return nil, err
	}
	return f.chunkEntries(ctx, entries, f.opt.FailOnBadChunks)
}

// ListR lists the objects and directories of the Fs starting
// from dir recursively into out.
//
// dir should be "" to start from the root, and should not
// have trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
//
// It should call callback for each tranche of entries read.
// These need not be returned in any particular order.  If
// callback returns an error then the listing will stop
// immediately.
//
// Don't implement this unless you have a more efficient way
// of listing recursively that doing a directory traversal.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	do := f.base.Features().ListR
	return do(ctx, dir, func(entries fs.DirEntries) error {
		newEntries, err := f.chunkEntries(ctx, entries, f.opt.FailOnBadChunks)
		if err != nil {
			return err
		}
		return callback(newEntries)
	})
}

// chunkEntries is called by List(R). It merges chunk entries from
// wrapped remote into composite directory entries.
func (f *Fs) chunkEntries(ctx context.Context, origEntries fs.DirEntries, hardErrors bool) (chunkedEntries fs.DirEntries, err error) {
	// sort entries, so that meta objects (if any) appear before their chunks
	sortedEntries := make(fs.DirEntries, len(origEntries))
	copy(sortedEntries, origEntries)
	sort.Sort(sortedEntries)

	byRemote := make(map[string]*Object)
	badEntry := make(map[string]bool)
	isSubdir := make(map[string]bool)

	var tempEntries fs.DirEntries
	for _, dirOrObject := range sortedEntries {
		switch entry := dirOrObject.(type) {
		case fs.Object:
			remote := entry.Remote()
			if mainRemote, chunkNo, tempNo := f.parseChunkName(remote); mainRemote != "" {
				if tempNo != -1 {
					fs.Debugf(f, "skip temporary chunk %q", remote)
					break
				}
				mainObject := byRemote[mainRemote]
				if mainObject == nil && f.useMeta {
					fs.Debugf(f, "skip chunk %q without meta object", remote)
					break
				}
				if mainObject == nil {
					// useMeta is false - create chunked object without meta data
					mainObject = f.newObject(mainRemote, nil, nil)
					byRemote[mainRemote] = mainObject
					if !badEntry[mainRemote] {
						tempEntries = append(tempEntries, mainObject)
					}
				}
				if err := mainObject.addChunk(entry, chunkNo); err != nil {
					if hardErrors {
						return nil, err
					}
					badEntry[mainRemote] = true
				}
				break
			}
			object := f.newObject("", entry, nil)
			byRemote[remote] = object
			tempEntries = append(tempEntries, object)
		case fs.Directory:
			isSubdir[entry.Remote()] = true
			wrapDir := fs.NewDirCopy(ctx, entry)
			wrapDir.SetRemote(entry.Remote())
			tempEntries = append(tempEntries, wrapDir)
		default:
			if hardErrors {
				return nil, errors.Errorf("Unknown object type %T", entry)
			}
			fs.Debugf(f, "unknown object type %T", entry)
		}
	}

	for _, entry := range tempEntries {
		if object, ok := entry.(*Object); ok {
			remote := object.Remote()
			if isSubdir[remote] {
				if hardErrors {
					return nil, fmt.Errorf("%q is both meta object and directory", remote)
				}
				badEntry[remote] = true // fall thru
			}
			if badEntry[remote] {
				fs.Debugf(f, "invalid directory entry %q", remote)
				continue
			}
			if err := object.validate(); err != nil {
				if hardErrors {
					return nil, err
				}
				fs.Debugf(f, "invalid chunks in object %q", remote)
				continue
			}
		}
		chunkedEntries = append(chunkedEntries, entry)
	}

	return chunkedEntries, nil
}

// NewObject finds the Object at remote.
//
// Please note that every NewObject invocation will scan the whole directory.
// Using here something like fs.DirCache might improve performance (and make
// logic more complex though).
//
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	if mainRemote, _, _ := f.parseChunkName(remote); mainRemote != "" {
		return nil, fmt.Errorf("%q should be meta object, not a chunk", remote)
	}

	var (
		o       *Object
		baseObj fs.Object
		err     error
	)

	if f.useMeta {
		baseObj, err = f.base.NewObject(ctx, remote)
		if err != nil {
			return nil, err
		}
		remote = baseObj.Remote()

		// meta object cannot be large - assume single chunk
		o = f.newObject("", baseObj, nil)
		if o.size > maxMetaDataSize {
			return o, nil
		}
	} else {
		// will read single wrapped object later
		o = f.newObject(remote, nil, nil)
	}

	// the object is small, probably it contains meta data
	dir := path.Dir(strings.TrimRight(remote, "/"))
	if dir == "." {
		dir = ""
	}
	entries, err := f.base.List(ctx, dir)
	switch err {
	case nil:
		// OK, fall thru
	case fs.ErrorDirNotFound:
		entries = nil
	default:
		return nil, errors.Wrap(err, "can't detect chunked object")
	}

	for _, dirOrObject := range entries {
		entry, ok := dirOrObject.(fs.Object)
		if !ok {
			continue
		}
		entryRemote := entry.Remote()
		if !strings.Contains(entryRemote, remote) {
			continue // bypass regexp to save cpu
		}
		mainRemote, chunkNo, tempNo := f.parseChunkName(entryRemote)
		if mainRemote == "" || mainRemote != remote || tempNo != -1 {
			continue // skip non-matching or temporary chunks
		}
		//fs.Debugf(f, "%q belongs to %q as chunk %d", entryRemote, mainRemote, chunkNo)
		if err := o.addChunk(entry, chunkNo); err != nil {
			return nil, err
		}
	}

	if o.main == nil && (o.chunks == nil || len(o.chunks) == 0) {
		if f.useMeta {
			return nil, fs.ErrorObjectNotFound
		}
		// lazily read single wrapped object
		baseObj, err = f.base.NewObject(ctx, remote)
		if err == nil {
			err = o.addChunk(baseObj, 0)
		}
		if err != nil {
			return nil, err
		}
	}

	// calculate total file size
	if err := o.validate(); err != nil {
		return nil, err
	}
	// note: will call readMetaData lazily when needed
	return o, nil
}

func (o *Object) readMetaData(ctx context.Context) error {
	if o.isFull {
		return nil
	}
	if !o.isChunked() || !o.f.useMeta {
		o.isFull = true
		return nil
	}

	// validate meta data
	metaObject := o.main
	reader, err := metaObject.Open(ctx)
	if err != nil {
		return err
	}
	metaData, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}

	switch o.f.opt.MetaFormat {
	case "simplejson":
		metaInfo, err := unmarshalSimpleJSON(ctx, metaObject, metaData)
		if err != nil {
			// TODO: in a rare case we might mistake a small file for metadata
			return errors.Wrap(err, "invalid metadata")
		}
		if o.size != metaInfo.Size() || len(o.chunks) != metaInfo.nChunks {
			return errors.New("metadata doesn't match file size")
		}
		o.md5 = metaInfo.md5
		o.sha1 = metaInfo.sha1
	}

	o.isFull = true
	return nil
}

// put implements Put, PutStream, PutUnchecked, Update
func (f *Fs) put(ctx context.Context, in io.Reader, src fs.ObjectInfo, remote string, options []fs.OpenOption, basePut putFn) (obj fs.Object, err error) {
	c := f.newChunkingReader(src)
	wrapIn := c.wrapStream(ctx, in, src)

	var metaObject fs.Object
	defer func() {
		if err != nil {
			c.rollback(ctx, metaObject)
		}
	}()

	baseRemote := remote
	tempNo := time.Now().Unix()
	if tempNo < 0 {
		tempNo = -tempNo // unlikely but must be positive
	}

	// Transfer chunks data
	for chunkNo := 0; !c.done; chunkNo++ {
		tempRemote := f.makeChunkName(baseRemote, chunkNo, tempNo)
		size := c.sizeLeft
		if size > c.chunkSize {
			size = c.chunkSize
		}
		savedReadCount := c.readCount

		// If a single chunk is expected, avoid the extra rename operation
		chunkRemote := tempRemote
		if c.expectSingle && chunkNo == 0 && optimizeFirstChunk {
			chunkRemote = baseRemote
		}
		info := f.wrapInfo(src, chunkRemote, size)

		// TODO: handle range/limit options
		chunk, errChunk := basePut(ctx, wrapIn, info, options...)
		if errChunk != nil {
			return nil, errChunk
		}

		if size > 0 && c.readCount == savedReadCount && c.expectSingle {
			// basePut returned success but did not call chunking Read.
			// This is possible if wrapped remote has performed put by hash
			// (chunker bridges Hash from source for *single-chunk* files).
			// Account for actually read bytes here as a workaround.
			c.accountBytes(size)
		}
		if c.sizeLeft == 0 && !c.done {
			// The file has been apparently put by hash, force completion.
			c.done = true
		}

		// Expected a single chunk but more to come, so name it as usual.
		if !c.done && chunkRemote != tempRemote {
			fs.Infof(chunk, "Expected single chunk, got more")
			chunkMoved, errMove := f.baseMove(ctx, chunk, tempRemote, false)
			if errMove != nil {
				_ = chunk.Remove(ctx) // ignore error
				return nil, errMove
			}
			chunk = chunkMoved
		}

		// Wrapped remote may or may not have seen EOF from chunking reader,
		// eg. the box multi-uploader reads exactly the chunk size specified
		// and skips the "EOF" read - switch to next limit here.
		if !(c.chunkLimit == 0 || c.chunkLimit == c.chunkSize || c.sizeTotal == -1 || c.done) {
			_ = chunk.Remove(ctx) // ignore error
			return nil, fmt.Errorf("Destination ignored %d data bytes", c.chunkLimit)
		}
		c.chunkLimit = c.chunkSize

		c.chunks = append(c.chunks, chunk)
	}

	// Validate uploaded size
	if c.sizeTotal != -1 && c.readCount != c.sizeTotal {
		return nil, fmt.Errorf("Incorrect upload size %d != %d", c.readCount, c.sizeTotal)
	}

	// Finalize the single-chunk object
	if len(c.chunks) == 1 {
		// If old object was chunked, remove its chunks
		f.removeOldChunks(ctx, baseRemote)

		// Rename single chunk in place
		chunk := c.chunks[0]
		if chunk.Remote() != baseRemote {
			chunkMoved, errMove := f.baseMove(ctx, chunk, baseRemote, true)
			if errMove != nil {
				_ = chunk.Remove(ctx) // ignore error
				return nil, errMove
			}
			chunk = chunkMoved
		}

		return f.newObject("", chunk, nil), nil
	}

	// Validate total size of chunks
	var sizeTotal int64
	for _, chunk := range c.chunks {
		sizeTotal += chunk.Size()
	}
	if sizeTotal != c.readCount {
		return nil, fmt.Errorf("Incorrect chunks size %d != %d", sizeTotal, c.readCount)
	}

	// If old object was chunked, remove its chunks
	f.removeOldChunks(ctx, baseRemote)

	// Rename chunks from temporary to final names
	for chunkNo, chunk := range c.chunks {
		chunkRemote := f.makeChunkName(baseRemote, chunkNo, -1)
		chunkMoved, errMove := f.baseMove(ctx, chunk, chunkRemote, false)
		if errMove != nil {
			return nil, errMove
		}
		c.chunks[chunkNo] = chunkMoved
	}

	if !f.useMeta {
		// Remove stale metadata, if any
		oldMeta, errOldMeta := f.base.NewObject(ctx, baseRemote)
		if errOldMeta == nil {
			_ = oldMeta.Remove(ctx) // ignore error
		}

		o := f.newObject(baseRemote, nil, c.chunks)
		o.size = sizeTotal
		return o, nil
	}

	// Update meta object
	var metaData []byte
	switch f.opt.MetaFormat {
	case "simplejson":
		c.updateHashes()
		metaData, err = marshalSimpleJSON(ctx, sizeTotal, len(c.chunks), c.md5, c.sha1)
	}
	if err == nil {
		metaInfo := f.wrapInfo(src, baseRemote, int64(len(metaData)))
		metaObject, err = basePut(ctx, bytes.NewReader(metaData), metaInfo)
	}
	if err != nil {
		return nil, err
	}

	o := f.newObject("", metaObject, c.chunks)
	o.size = sizeTotal
	return o, nil
}

type putFn func(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error)

type chunkingReader struct {
	baseReader   io.Reader
	sizeTotal    int64
	sizeLeft     int64
	readCount    int64
	chunkSize    int64
	chunkLimit   int64
	err          error
	done         bool
	chunks       []fs.Object
	expectSingle bool
	fs           *Fs
	hasher       gohash.Hash
	md5          string
	sha1         string
}

func (f *Fs) newChunkingReader(src fs.ObjectInfo) *chunkingReader {
	c := &chunkingReader{
		fs:        f,
		readCount: 0,
		chunkSize: int64(f.opt.ChunkSize),
		sizeTotal: src.Size(),
	}
	c.chunkLimit = c.chunkSize
	c.sizeLeft = c.sizeTotal
	c.expectSingle = c.sizeTotal >= 0 && c.sizeTotal <= c.chunkSize
	return c
}

func (c *chunkingReader) wrapStream(ctx context.Context, in io.Reader, src fs.ObjectInfo) io.Reader {
	baseIn, wrapBack := accounting.UnWrap(in)

	switch {
	case c.fs.useMD5:
		if c.md5, _ = src.Hash(ctx, hash.MD5); c.md5 == "" {
			if c.fs.quickHash {
				c.sha1, _ = src.Hash(ctx, hash.SHA1)
			} else {
				c.hasher = md5.New()
			}
		}
	case c.fs.useSHA1:
		if c.sha1, _ = src.Hash(ctx, hash.SHA1); c.sha1 == "" {
			if c.fs.quickHash {
				c.md5, _ = src.Hash(ctx, hash.MD5)
			} else {
				c.hasher = sha1.New()
			}
		}
	}

	if c.hasher != nil {
		baseIn = io.TeeReader(baseIn, c.hasher)
	}
	c.baseReader = baseIn
	return wrapBack(c)
}

func (c *chunkingReader) updateHashes() {
	if c.hasher == nil {
		return
	}
	switch {
	case c.fs.useMD5:
		c.md5 = hex.EncodeToString(c.hasher.Sum(nil))
	case c.fs.useSHA1:
		c.sha1 = hex.EncodeToString(c.hasher.Sum(nil))
	}
}

// Note: Read is not called if wrapped remote performs put by hash.
func (c *chunkingReader) Read(buf []byte) (bytesRead int, err error) {
	if c.chunkLimit <= 0 {
		// Chunk complete - switch to next one.
		// We might not get here because some remotes (eg. box multi-uploader)
		// read the specified size exactly and skip the concluding EOF Read.
		// Then a check in the put loop will kick in.
		c.chunkLimit = c.chunkSize
		return 0, io.EOF
	}
	if int64(len(buf)) > c.chunkLimit {
		buf = buf[0:c.chunkLimit]
	}
	bytesRead, err = c.baseReader.Read(buf)
	if err != nil && err != io.EOF {
		c.err = err
		c.done = true
		return
	}
	c.accountBytes(int64(bytesRead))
	if bytesRead == 0 && c.sizeLeft == 0 {
		err = io.EOF // Force EOF when no data left.
	}
	if err == io.EOF {
		c.done = true
	}
	return
}

func (c *chunkingReader) accountBytes(bytesRead int64) {
	c.readCount += bytesRead
	c.chunkLimit -= bytesRead
	if c.sizeLeft != -1 {
		c.sizeLeft -= bytesRead
	}
}

// rollback removes uploaded temporary chunk
func (c *chunkingReader) rollback(ctx context.Context, metaObject fs.Object) {
	if metaObject != nil {
		c.chunks = append(c.chunks, metaObject)
	}
	for _, chunk := range c.chunks {
		if err := chunk.Remove(ctx); err != nil {
			fs.Errorf(chunk, "Failed to remove temporary chunk: %v", err)
		}
	}
}

func (f *Fs) removeOldChunks(ctx context.Context, remote string) {
	oldFsObject, err := f.NewObject(ctx, remote)
	if err == nil {
		oldObject := oldFsObject.(*Object)
		for _, chunk := range oldObject.chunks {
			if err := chunk.Remove(ctx); err != nil {
				fs.Errorf(chunk, "Failed to remove old chunk: %v", err)
			}
		}
	}
}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.put(ctx, in, src, src.Remote(), options, f.base.Put)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.put(ctx, in, src, src.Remote(), options, f.base.Features().PutStream)
}

// Update in to the object with the modTime given of the given size
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	if err := o.readMetaData(ctx); err != nil {
		return err
	}
	basePut := o.f.base.Put
	if src.Size() < 0 {
		basePut = o.f.base.Features().PutStream
		if basePut == nil {
			return errors.New("wrapped file system does not support streaming uploads")
		}
	}
	oNew, err := o.f.put(ctx, in, src, o.Remote(), options, basePut)
	if err == nil {
		*o = *oNew.(*Object)
	}
	return err
}

// PutUnchecked uploads the object
//
// This will create a duplicate if we upload a new file without
// checking to see if there is one already - use Put() for that.
// TODO: really split stream here
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	do := f.base.Features().PutUnchecked
	if do == nil {
		return nil, errors.New("can't PutUnchecked")
	}
	// TODO: handle options and chunking!
	o, err := do(ctx, in, f.wrapInfo(src, "", -1))
	if err != nil {
		return nil, err
	}
	return f.newObject("", o, nil), nil
}

// Precision returns the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return f.base.Precision()
}

// Hashes returns the supported hash sets.
// Chunker advertises a hash type if and only if it can be calculated
// for files of any size, multi-chunked or small.
func (f *Fs) Hashes() hash.Set {
	// composites && all of them && small files supported by wrapped remote
	if f.useMD5 && !f.quickHash && f.base.Hashes().Contains(hash.MD5) {
		return hash.NewHashSet(hash.MD5)
	}
	if f.useSHA1 && !f.quickHash && f.base.Hashes().Contains(hash.SHA1) {
		return hash.NewHashSet(hash.SHA1)
	}
	return hash.NewHashSet() // can't provide strong guarantees
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	return f.base.Mkdir(ctx, dir)
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.base.Rmdir(ctx, dir)
}

// Purge all files in the root and the root directory
//
// Implement this if you have a way of deleting all the files
// quicker than just running Remove() on the result of List()
//
// Return an error if it doesn't exist.
//
// This command will chain to `purge` from wrapped remote.
// As a result it removes not only chunker files with their
// active chunks but also all hidden chunks in the directory.
//
func (f *Fs) Purge(ctx context.Context) error {
	do := f.base.Features().Purge
	if do == nil {
		return fs.ErrorCantPurge
	}
	return do(ctx)
}

// Remove an object (chunks and metadata, if any)
//
// Remove deletes only active chunks of the object.
// It does not try to look for temporary chunks because they could belong
// to another command modifying this composite file in parallel.
//
// Commands normally cleanup all temporary chunks in case of a failure.
// However, if rclone dies unexpectedly, it can leave hidden temporary
// chunks, which cannot be discovered using the `list` command.
// Remove does not try to search for such chunks or delete them.
// Sometimes this can lead to strange results eg. when `list` shows that
// directory is empty but `rmdir` refuses to remove it because on the
// level of wrapped remote it's actually *not* empty.
// As a workaround users can use `purge` to forcibly remove it.
//
// In future, a flag `--chunker-delete-hidden` may be added which tells
// Remove to search directory for hidden chunks and remove them too
// (at the risk of breaking parallel commands).
//
func (o *Object) Remove(ctx context.Context) (err error) {
	if o.main != nil {
		err = o.main.Remove(ctx)
	}
	for _, chunk := range o.chunks {
		chunkErr := chunk.Remove(ctx)
		if err == nil {
			err = chunkErr
		}
	}
	return err
}

// copyOrMove implements copy or move
func (f *Fs) copyOrMove(ctx context.Context, o *Object, remote string, do copyMoveFn, md5, sha1, opName string) (fs.Object, error) {
	if !o.isChunked() {
		fs.Debugf(o, "%s non-chunked object...", opName)
		oResult, err := do(ctx, o.mainChunk(), remote) // chain operation to a single wrapped chunk
		if err != nil {
			return nil, err
		}
		return f.newObject("", oResult, nil), nil
	}

	fs.Debugf(o, "%s %d chunks...", opName, len(o.chunks))
	mainRemote := o.remote
	var newChunks []fs.Object
	var err error

	// Copy or move chunks
	for _, chunk := range o.chunks {
		chunkRemote := chunk.Remote()
		if !strings.HasPrefix(chunkRemote, mainRemote) {
			err = fmt.Errorf("invalid chunk %q", chunkRemote)
			break
		}
		chunkSuffix := chunkRemote[len(mainRemote):]
		chunkResult, err := do(ctx, chunk, remote+chunkSuffix)
		if err != nil {
			break
		}
		newChunks = append(newChunks, chunkResult)
	}

	// Copy or move old metadata
	var metaObject fs.Object
	if err == nil && o.main != nil {
		metaObject, err = do(ctx, o.main, remote)
	}
	if err != nil {
		for _, chunk := range newChunks {
			_ = chunk.Remove(ctx) // ignore error
		}
		return nil, err
	}

	// Create wrapping object, calculate and validate total size
	newObj := f.newObject(remote, metaObject, newChunks)
	err = newObj.validate()
	if err != nil {
		_ = newObj.Remove(ctx) // ignore error
		return nil, err
	}

	// Update metadata
	var metaData []byte
	switch f.opt.MetaFormat {
	case "simplejson":
		metaData, err = marshalSimpleJSON(ctx, newObj.size, len(newChunks), md5, sha1)
		if err == nil {
			metaInfo := f.wrapInfo(metaObject, "", int64(len(metaData)))
			err = newObj.main.Update(ctx, bytes.NewReader(metaData), metaInfo)
		}
	case "none":
		if newObj.main != nil {
			err = newObj.main.Remove(ctx)
		}
	}

	// Return wrapping object
	if err != nil {
		_ = newObj.Remove(ctx) // ignore error
		return nil, err
	}
	return newObj, nil
}

type copyMoveFn func(context.Context, fs.Object, string) (fs.Object, error)

func (f *Fs) okForServerSide(ctx context.Context, src fs.Object, opName string) (obj *Object, md5, sha1 string, ok bool) {
	var diff string
	obj, ok = src.(*Object)

	switch {
	case !ok:
		diff = "remote types"
	case !operations.SameConfig(f.base, obj.f.base):
		diff = "wrapped remotes"
	case f.opt.ChunkSize != obj.f.opt.ChunkSize:
		diff = "chunk sizes"
	case f.opt.NameFormat != obj.f.opt.NameFormat:
		diff = "chunk name formats"
	case f.opt.MetaFormat != obj.f.opt.MetaFormat:
		diff = "meta formats"
	}
	if diff != "" {
		fs.Debugf(src, "Can't %s - different %s", opName, diff)
		ok = false
		return
	}

	if f.opt.MetaFormat != "simplejson" || !obj.isChunked() {
		ok = true // hash is not required for meta data
		return
	}

	switch {
	case f.useMD5:
		md5, _ = obj.Hash(ctx, hash.MD5)
		ok = md5 != ""
		if !ok && f.quickHash {
			sha1, _ = obj.Hash(ctx, hash.SHA1)
			ok = sha1 != ""
		}
	case f.useSHA1:
		sha1, _ = obj.Hash(ctx, hash.SHA1)
		ok = sha1 != ""
		if !ok && f.quickHash {
			md5, _ = obj.Hash(ctx, hash.MD5)
			ok = md5 != ""
		}
	default:
		ok = false
	}
	if !ok {
		fs.Debugf(src, "Can't %s - required hash not found", opName)
	}
	return
}

// Copy src to this remote using server side copy operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	baseCopy := f.base.Features().Copy
	if baseCopy == nil {
		return nil, fs.ErrorCantCopy
	}
	obj, md5, sha1, ok := f.okForServerSide(ctx, src, "copy")
	if !ok {
		return nil, fs.ErrorCantCopy
	}
	return f.copyOrMove(ctx, obj, remote, baseCopy, md5, sha1, "copy")
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
	baseMove := func(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
		return f.baseMove(ctx, src, remote, false)
	}
	obj, md5, sha1, ok := f.okForServerSide(ctx, src, "move")
	if !ok {
		return nil, fs.ErrorCantMove
	}
	return f.copyOrMove(ctx, obj, remote, baseMove, md5, sha1, "move")
}

// baseMove chains to the wrapped Move or simulates it by Copy+Delete
func (f *Fs) baseMove(ctx context.Context, src fs.Object, remote string, deleteDest bool) (fs.Object, error) {
	var dest fs.Object
	if deleteDest {
		var err error
		dest, err = f.base.NewObject(ctx, remote)
		if err != nil {
			dest = nil
		}
	}
	return operations.Move(ctx, f.base, dest, remote, src)
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
	do := f.base.Features().DirMove
	if do == nil {
		return fs.ErrorCantDirMove
	}
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	return do(ctx, srcFs.base, srcRemote, dstRemote)
}

// CleanUp the trash in the Fs
//
// Implement this if you have a way of emptying the trash or
// otherwise cleaning up old versions of files.
func (f *Fs) CleanUp(ctx context.Context) error {
	do := f.base.Features().CleanUp
	if do == nil {
		return errors.New("can't CleanUp")
	}
	return do(ctx)
}

// About gets quota information from the Fs
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	do := f.base.Features().About
	if do == nil {
		return nil, errors.New("About not supported")
	}
	return do(ctx)
}

// UnWrap returns the Fs that this Fs is wrapping
func (f *Fs) UnWrap() fs.Fs {
	return f.base
}

// WrapFs returns the Fs that is wrapping this Fs
func (f *Fs) WrapFs() fs.Fs {
	return f.wrapper
}

// SetWrapper sets the Fs that is wrapping this Fs
func (f *Fs) SetWrapper(wrapper fs.Fs) {
	f.wrapper = wrapper
}

// ChangeNotify calls the passed function with a path
// that has had changes. If the implementation
// uses polling, it should adhere to the given interval.
func (f *Fs) ChangeNotify(ctx context.Context, notifyFunc func(string, fs.EntryType), pollIntervalChan <-chan time.Duration) {
	do := f.base.Features().ChangeNotify
	if do == nil {
		return
	}
	wrappedNotifyFunc := func(path string, entryType fs.EntryType) {
		//fs.Debugf(f, "ChangeNotify: path %q entryType %d", path, entryType)
		if entryType == fs.EntryObject {
			if mainPath, _, tempNo := f.parseChunkName(path); mainPath != "" && tempNo == -1 {
				path = mainPath
			}
		}
		notifyFunc(path, entryType)
	}
	do(ctx, wrappedNotifyFunc, pollIntervalChan)
}

// Object wraps one or more remote data chunks
type Object struct {
	remote string
	main   fs.Object // optional meta object if file is chunked, or single data chunk
	chunks []fs.Object
	size   int64 // cached total size of chunks in a chunked file
	isFull bool
	md5    string
	sha1   string
	f      *Fs
}

func (o *Object) addChunk(chunk fs.Object, chunkNo int) error {
	if chunkNo < 0 {
		return fmt.Errorf("invalid chunk number %d", chunkNo+o.f.opt.StartFrom)
	}
	if chunkNo == len(o.chunks) {
		o.chunks = append(o.chunks, chunk)
		return nil
	}
	if chunkNo > len(o.chunks) {
		newChunks := make([]fs.Object, (chunkNo + 1), (chunkNo+1)*2)
		copy(newChunks, o.chunks)
		o.chunks = newChunks
	}
	o.chunks[chunkNo] = chunk
	return nil
}

// validate verifies the object internals and updates total size
func (o *Object) validate() error {
	if !o.isChunked() {
		_ = o.mainChunk() // verify that single wrapped chunk exists
		return nil
	}

	metaObject := o.main // this file is chunked - o.main refers to optional meta object
	if metaObject != nil && metaObject.Size() > maxMetaDataSize {
		// metadata of a chunked file must be a tiny piece of json
		o.size = -1
		return fmt.Errorf("%q metadata is too large", o.remote)
	}

	var totalSize int64
	for _, chunk := range o.chunks {
		if chunk == nil {
			o.size = -1
			return fmt.Errorf("%q has missing chunks", o)
		}
		totalSize += chunk.Size()
	}
	o.size = totalSize // cache up total size
	return nil
}

func (f *Fs) newObject(remote string, main fs.Object, chunks []fs.Object) *Object {
	var size int64 = -1
	if main != nil {
		size = main.Size()
		if remote == "" {
			remote = main.Remote()
		}
	}
	return &Object{
		remote: remote,
		main:   main,
		size:   size,
		f:      f,
		chunks: chunks,
	}
}

// mainChunk returns:
// - a single wrapped chunk for non-chunked files
// - meta object for chunked files with metadata
// - first chunk for chunked files without metadata
func (o *Object) mainChunk() fs.Object {
	if o.main != nil {
		return o.main // meta object or single wrapped chunk
	}
	if o.chunks != nil {
		return o.chunks[0] // first chunk for chunked files
	}
	panic("invalid chunked object") // unlikely
}

func (o *Object) isChunked() bool {
	return o.chunks != nil
}

// Fs returns read only access to the Fs that this object is part of
func (o *Object) Fs() fs.Info {
	return o.f
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

// Size returns the size of the file
func (o *Object) Size() int64 {
	if o.isChunked() {
		return o.size // total size of chunks in a chunked file
	}
	return o.mainChunk().Size() // size of a single wrapped chunk
}

// Storable returns whether object is storable
func (o *Object) Storable() bool {
	return o.mainChunk().Storable()
}

// ModTime returns the modification time of the file
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.mainChunk().ModTime(ctx)
}

// SetModTime sets the modification time of the file
func (o *Object) SetModTime(ctx context.Context, mtime time.Time) error {
	if err := o.readMetaData(ctx); err != nil {
		return err
	}
	return o.mainChunk().SetModTime(ctx, mtime)
}

// Hash returns the selected checksum of the file.
// If no checksum is available it returns "".
//
// Hash prefers wrapped hashsum for a non-chunked file, then tries to
// read it from metadata. This in theory handles an unusual case when
// a small file is modified on the lower level by wrapped remote
// but chunker is not yet aware of changes.
//
// Currently metadata (if not configured as 'none') is kept only for
// multi-chunk files, but for small files chunker obtains hashsums from
// wrapped remote. If a particular hashsum type is not supported,
// chunker won't fail with `unsupported` error but return empty hash.
//
// In future metadata logic can be extended: if a normal (non-quick)
// hash type is configured, chunker will check whether wrapped remote
// supports it (see Fs.Hashes as an example). If not, it will add metadata
// to small files as well, thus providing hashsums for all files.
//
func (o *Object) Hash(ctx context.Context, hashType hash.Type) (string, error) {
	if !o.isChunked() {
		// First, chain to the single wrapped chunk, if possible.
		if value, err := o.mainChunk().Hash(ctx, hashType); err == nil && value != "" {
			return value, nil
		}
	}
	if err := o.readMetaData(ctx); err != nil {
		return "", err
	}
	// Try saved hash if the file is chunked or the wrapped remote fails.
	switch hashType {
	case hash.MD5:
		if o.md5 == "" {
			return "", nil
		}
		return o.md5, nil
	case hash.SHA1:
		if o.sha1 == "" {
			return "", nil
		}
		return o.sha1, nil
	default:
		return "", hash.ErrUnsupported
	}
}

// UnWrap returns the wrapped Object
func (o *Object) UnWrap() fs.Object {
	return o.mainChunk()
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (rc io.ReadCloser, err error) {
	if !o.isChunked() {
		return o.mainChunk().Open(ctx, options...) // chain to a single wrapped chunk
	}
	if err := o.readMetaData(ctx); err != nil {
		return nil, err
	}

	var openOptions []fs.OpenOption
	var offset, limit int64 = 0, -1

	for _, option := range options {
		switch opt := option.(type) {
		case *fs.SeekOption:
			offset = opt.Offset
		case *fs.RangeOption:
			offset, limit = opt.Decode(o.size)
		default:
			// pass on Options to wrapped open if appropriate
			openOptions = append(openOptions, option)
		}
	}

	if offset < 0 {
		return nil, errors.New("invalid offset")
	}
	if limit < 0 {
		limit = o.size - offset
	}

	return o.newLinearReader(ctx, offset, limit, openOptions)
}

// linearReader opens and reads file chunks sequentially, without read-ahead
type linearReader struct {
	ctx     context.Context
	chunks  []fs.Object
	options []fs.OpenOption
	limit   int64
	count   int64
	pos     int
	reader  io.ReadCloser
	err     error
}

func (o *Object) newLinearReader(ctx context.Context, offset, limit int64, options []fs.OpenOption) (io.ReadCloser, error) {
	r := &linearReader{
		ctx:     ctx,
		chunks:  o.chunks,
		options: options,
		limit:   limit,
	}

	// skip to chunk for given offset
	err := io.EOF
	for offset >= 0 && err != nil {
		offset, err = r.nextChunk(offset)
	}
	if err == nil || err == io.EOF {
		r.err = err
		return r, nil
	}
	return nil, err
}

func (r *linearReader) nextChunk(offset int64) (int64, error) {
	if r.err != nil {
		return -1, r.err
	}
	if r.pos >= len(r.chunks) || r.limit <= 0 || offset < 0 {
		return -1, io.EOF
	}

	chunk := r.chunks[r.pos]
	count := chunk.Size()
	r.pos++

	if offset >= count {
		return offset - count, io.EOF
	}
	count -= offset
	if r.limit < count {
		count = r.limit
	}
	options := append(r.options, &fs.RangeOption{Start: offset, End: offset + count - 1})

	if err := r.Close(); err != nil {
		return -1, err
	}

	reader, err := chunk.Open(r.ctx, options...)
	if err != nil {
		return -1, err
	}

	r.reader = reader
	r.count = count
	return offset, nil
}

func (r *linearReader) Read(p []byte) (n int, err error) {
	if r.err != nil {
		return 0, r.err
	}
	if r.limit <= 0 {
		r.err = io.EOF
		return 0, io.EOF
	}

	for r.count <= 0 {
		// current chunk has been read completely or its size is zero
		off, err := r.nextChunk(0)
		if off < 0 {
			r.err = err
			return 0, err
		}
	}

	n, err = r.reader.Read(p)
	if err == nil || err == io.EOF {
		r.count -= int64(n)
		r.limit -= int64(n)
		if r.limit > 0 {
			err = nil // more data to read
		}
	}
	r.err = err
	return
}

func (r *linearReader) Close() (err error) {
	if r.reader != nil {
		err = r.reader.Close()
		r.reader = nil
	}
	return
}

// ObjectInfo describes a wrapped fs.ObjectInfo for being the source
type ObjectInfo struct {
	src     fs.ObjectInfo
	fs      *Fs
	nChunks int
	size    int64  // overrides source size by the total size of chunks
	remote  string // overrides remote name
	md5     string // overrides MD5 checksum
	sha1    string // overrides SHA1 checksum
}

func (f *Fs) wrapInfo(src fs.ObjectInfo, newRemote string, totalSize int64) *ObjectInfo {
	return &ObjectInfo{
		src:    src,
		fs:     f,
		size:   totalSize,
		remote: newRemote,
	}
}

// Fs returns read only access to the Fs that this object is part of
func (oi *ObjectInfo) Fs() fs.Info {
	if oi.fs == nil {
		panic("stub ObjectInfo")
	}
	return oi.fs
}

// String returns string representation
func (oi *ObjectInfo) String() string {
	return oi.src.String()
}

// Storable returns whether object is storable
func (oi *ObjectInfo) Storable() bool {
	return oi.src.Storable()
}

// Remote returns the remote path
func (oi *ObjectInfo) Remote() string {
	if oi.remote != "" {
		return oi.remote
	}
	return oi.src.Remote()
}

// Size returns the size of the file
func (oi *ObjectInfo) Size() int64 {
	if oi.size != -1 {
		return oi.size
	}
	return oi.src.Size()
}

// ModTime returns the modification time
func (oi *ObjectInfo) ModTime(ctx context.Context) time.Time {
	return oi.src.ModTime(ctx)
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (oi *ObjectInfo) Hash(ctx context.Context, hashType hash.Type) (string, error) {
	var errUnsupported error
	switch hashType {
	case hash.MD5:
		if oi.md5 != "" {
			return oi.md5, nil
		}
	case hash.SHA1:
		if oi.sha1 != "" {
			return oi.sha1, nil
		}
	default:
		errUnsupported = hash.ErrUnsupported
	}
	if oi.Size() != oi.src.Size() {
		// fail if this info wraps a file part
		return "", errUnsupported
	}
	// chain to full source if possible
	value, err := oi.src.Hash(ctx, hashType)
	if err == hash.ErrUnsupported {
		return "", errUnsupported
	}
	return value, err
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	if doer, ok := o.mainChunk().(fs.IDer); ok {
		return doer.ID()
	}
	return ""
}

// Meta format `simplejson`
type metaSimpleJSON struct {
	Version int    `json:"ver"`
	Size    int64  `json:"size"`
	NChunks int    `json:"nchunks"`
	MD5     string `json:"md5"`
	SHA1    string `json:"sha1"`
}

func marshalSimpleJSON(ctx context.Context, size int64, nChunks int, md5, sha1 string) (data []byte, err error) {
	metaData := &metaSimpleJSON{
		Version: metaDataVersion,
		Size:    size,
		NChunks: nChunks,
		MD5:     md5,
		SHA1:    sha1,
	}
	return json.Marshal(&metaData)
}

// Note: only metadata format version 1 is supported a.t.m.
//
// Current implementation creates metadata only for files larger than
// configured chunk size. This approach has drawback: availability of
// configured hashsum type for small files depends on the wrapped remote.
// Future versions of chunker may change approach as described in comment
// to the Hash method. They can transparently migrate older metadata.
// New format will have a higher version number and cannot be correctly
// hanled by current implementation.
// The version check below will then explicitly ask user to upgrade rclone.
//
func unmarshalSimpleJSON(ctx context.Context, metaObject fs.Object, data []byte) (info *ObjectInfo, err error) {
	var metaData *metaSimpleJSON
	err = json.Unmarshal(data, &metaData)
	if err != nil {
		return nil, err
	}

	// Perform strict checks, avoid corruption of future metadata formats.
	if metaData.Size < 0 {
		return nil, errors.New("negative file size")
	}
	if metaData.NChunks <= 0 {
		return nil, errors.New("wrong number of chunks")
	}
	if metaData.MD5 != "" {
		_, err = hex.DecodeString(metaData.MD5)
		if len(metaData.MD5) != 32 || err != nil {
			return nil, errors.New("wrong md5 hash")
		}
	}
	if metaData.SHA1 != "" {
		_, err = hex.DecodeString(metaData.SHA1)
		if len(metaData.SHA1) != 40 || err != nil {
			return nil, errors.New("wrong sha1 hash")
		}
	}
	if metaData.Version <= 0 {
		return nil, errors.New("wrong version number")
	}
	if metaData.Version != metaDataVersion {
		return nil, errors.Errorf("version %d is not supported, please upgrade rclone", metaData.Version)
	}

	var nilFs *Fs // nil object triggers appropriate type method
	info = nilFs.wrapInfo(metaObject, "", metaData.Size)
	info.md5 = metaData.MD5
	info.sha1 = metaData.SHA1
	info.nChunks = metaData.NChunks
	return info, nil
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.PutUncheckeder  = (*Fs)(nil)
	_ fs.PutStreamer     = (*Fs)(nil)
	_ fs.CleanUpper      = (*Fs)(nil)
	_ fs.UnWrapper       = (*Fs)(nil)
	_ fs.ListRer         = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Wrapper         = (*Fs)(nil)
	_ fs.ChangeNotifier  = (*Fs)(nil)
	_ fs.ObjectInfo      = (*ObjectInfo)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.ObjectUnWrapper = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)
