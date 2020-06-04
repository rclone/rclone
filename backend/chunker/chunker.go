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
	"math/rand"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
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

//
// Chunker's composite files have one or more chunks
// and optional metadata object. If it's present,
// meta object is named after the original file.
//
// The only supported metadata format is simplejson atm.
// It supports only per-file meta objects that are rudimentary,
// used mostly for consistency checks (lazily for performance reasons).
// Other formats can be developed that use an external meta store
// free of these limitations, but this needs some support from
// rclone core (eg. metadata store interfaces).
//
// The following types of chunks are supported:
// data and control, active and temporary.
// Chunk type is identified by matching chunk file name
// based on the chunk name format configured by user.
//
// Both data and control chunks can be either temporary (aka hidden)
// or active (non-temporary aka normal aka permanent).
// An operation creates temporary chunks while it runs.
// By completion it removes temporary and leaves active chunks.
//
// Temporary chunks have a special hardcoded suffix in addition
// to the configured name pattern.
// Temporary suffix includes so called transaction identifier
// (abbreviated as `xactID` below), a generic non-negative base-36 "number"
// used by parallel operations to share a composite object.
// Chunker also accepts the longer decimal temporary suffix (obsolete),
// which is transparently converted to the new format. In its maximum
// length of 13 decimals it makes a 7-digit base-36 number.
//
// Chunker can tell data chunks from control chunks by the characters
// located in the "hash placeholder" position of configured format.
// Data chunks have decimal digits there.
// Control chunks have in that position a short lowercase alphanumeric
// string (starting with a letter) prepended by underscore.
//
// Metadata format v1 does not define any control chunk types,
// they are currently ignored aka reserved.
// In future they can be used to implement resumable uploads etc.
//
const (
	ctrlTypeRegStr   = `[a-z][a-z0-9]{2,6}`
	tempSuffixFormat = `_%04s`
	tempSuffixRegStr = `_([0-9a-z]{4,9})`
	tempSuffixRegOld = `\.\.tmp_([0-9]{10,13})`
)

var (
	// regular expressions to validate control type and temporary suffix
	ctrlTypeRegexp   = regexp.MustCompile(`^` + ctrlTypeRegStr + `$`)
	tempSuffixRegexp = regexp.MustCompile(`^` + tempSuffixRegStr + `$`)
)

// Normally metadata is a small piece of JSON (about 100-300 bytes).
// The size of valid metadata must never exceed this limit.
// Current maximum provides a reasonable room for future extensions.
//
// Please refrain from increasing it, this can cause old rclone versions
// to fail, or worse, treat meta object as a normal file (see NewObject).
// If more room is needed please bump metadata version forcing previous
// releases to ask for upgrade, and offload extra info to a control chunk.
//
// And still chunker's primary function is to chunk large files
// rather than serve as a generic metadata container.
const maxMetadataSize = 255

// Current/highest supported metadata format.
const metadataVersion = 1

// optimizeFirstChunk enables the following optimization in the Put:
// If a single chunk is expected, put the first chunk using the
// base target name instead of a temporary name, thus avoiding
// extra rename operation.
// Warning: this optimization is not transaction safe.
const optimizeFirstChunk = false

// revealHidden is a stub until chunker lands the `reveal hidden` option.
const revealHidden = false

// Prevent memory overflow due to specially crafted chunk name
const maxSafeChunkNumber = 10000000

// Number of attempts to find unique transaction identifier
const maxTransactionProbes = 100

// standard chunker errors
var (
	ErrChunkOverflow = errors.New("chunk number overflow")
)

// variants of baseMove's parameter delMode
const (
	delNever  = 0 // don't delete, just move
	delAlways = 1 // delete destination before moving
	delFailed = 2 // move, then delete and try again if failed
)

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
It has the following fields: ver, size, nchunks, md5, sha1.`,
			}},
		}, {
			Name:     "hash_type",
			Advanced: false,
			Default:  "md5",
			Help:     `Choose how chunker handles hash sums. All modes but "none" require metadata.`,
			Examples: []fs.OptionExample{{
				Value: "none",
				Help:  `Pass any hash supported by wrapped remote for non-chunked files, return nothing otherwise`,
			}, {
				Value: "md5",
				Help:  `MD5 for composite files`,
			}, {
				Value: "sha1",
				Help:  `SHA1 for composite files`,
			}, {
				Value: "md5all",
				Help:  `MD5 for all files`,
			}, {
				Value: "sha1all",
				Help:  `SHA1 for all files`,
			}, {
				Value: "md5quick",
				Help:  `Copying a file to chunker will request MD5 from the source falling back to SHA1 if unsupported`,
			}, {
				Value: "sha1quick",
				Help:  `Similar to "md5quick" but prefers SHA1 over MD5`,
			}},
		}, {
			Name:     "fail_hard",
			Advanced: true,
			Default:  false,
			Help:     `Choose how chunker should handle files with missing or invalid chunks.`,
			Examples: []fs.OptionExample{
				{
					Value: "true",
					Help:  "Report errors and abort current command.",
				}, {
					Value: "false",
					Help:  "Warn user, skip incomplete file and proceed.",
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
	f.dirSort = true // processEntries requires that meta Objects prerun data chunks atm.

	if err := f.configure(opt.NameFormat, opt.MetaFormat, opt.HashType); err != nil {
		return nil, err
	}

	// Handle the tricky case detected by FsMkdir/FsPutFiles/FsIsFile
	// when `rpath` points to a composite multi-chunk file without metadata,
	// i.e. `rpath` does not exist in the wrapped remote, but chunker
	// detects a composite file because it finds the first chunk!
	// (yet can't satisfy fstest.CheckListing, will ignore)
	if err == nil && !f.useMeta && strings.Contains(rpath, "/") {
		firstChunkPath := f.makeChunkName(remotePath, 0, "", "")
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
	Remote     string        `config:"remote"`
	ChunkSize  fs.SizeSuffix `config:"chunk_size"`
	NameFormat string        `config:"name_format"`
	StartFrom  int           `config:"start_from"`
	MetaFormat string        `config:"meta_format"`
	HashType   string        `config:"hash_type"`
	FailHard   bool          `config:"fail_hard"`
}

// Fs represents a wrapped fs.Fs
type Fs struct {
	name         string
	root         string
	base         fs.Fs          // remote wrapped by chunker overlay
	wrapper      fs.Fs          // wrapper is used by SetWrapper
	useMeta      bool           // false if metadata format is 'none'
	useMD5       bool           // mutually exclusive with useSHA1
	useSHA1      bool           // mutually exclusive with useMD5
	hashFallback bool           // allows fallback from MD5 to SHA1 and vice versa
	hashAll      bool           // hash all files, mutually exclusive with hashFallback
	dataNameFmt  string         // name format of data chunks
	ctrlNameFmt  string         // name format of control chunks
	nameRegexp   *regexp.Regexp // regular expression to match chunk names
	xactIDRand   *rand.Rand     // generator of random transaction identifiers
	xactIDMutex  sync.Mutex     // mutex for the source of randomness
	opt          Options        // copy of Options
	features     *fs.Features   // optional features
	dirSort      bool           // reserved for future, ignored
}

// configure sets up chunker for given name format, meta format and hash type.
// It also seeds the source of random transaction identifiers.
// configure must be called only from NewFs or by unit tests.
func (f *Fs) configure(nameFormat, metaFormat, hashType string) error {
	if err := f.setChunkNameFormat(nameFormat); err != nil {
		return errors.Wrapf(err, "invalid name format '%s'", nameFormat)
	}
	if err := f.setMetaFormat(metaFormat); err != nil {
		return err
	}
	if err := f.setHashType(hashType); err != nil {
		return err
	}

	randomSeed := time.Now().UnixNano()
	f.xactIDRand = rand.New(rand.NewSource(randomSeed))

	return nil
}

func (f *Fs) setMetaFormat(metaFormat string) error {
	switch metaFormat {
	case "none":
		f.useMeta = false
	case "simplejson":
		f.useMeta = true
	default:
		return fmt.Errorf("unsupported meta format '%s'", metaFormat)
	}
	return nil
}

// setHashType
// must be called *after* setMetaFormat.
//
// In the "All" mode chunker will force metadata on all files
// if the wrapped remote can't provide given hashsum.
func (f *Fs) setHashType(hashType string) error {
	f.useMD5 = false
	f.useSHA1 = false
	f.hashFallback = false
	f.hashAll = false
	requireMetaHash := true

	switch hashType {
	case "none":
		requireMetaHash = false
	case "md5":
		f.useMD5 = true
	case "sha1":
		f.useSHA1 = true
	case "md5quick":
		f.useMD5 = true
		f.hashFallback = true
	case "sha1quick":
		f.useSHA1 = true
		f.hashFallback = true
	case "md5all":
		f.useMD5 = true
		f.hashAll = !f.base.Hashes().Contains(hash.MD5)
	case "sha1all":
		f.useSHA1 = true
		f.hashAll = !f.base.Hashes().Contains(hash.SHA1)
	default:
		return fmt.Errorf("unsupported hash type '%s'", hashType)
	}
	if requireMetaHash && !f.useMeta {
		return fmt.Errorf("hash type '%s' requires compatible meta format", hashType)
	}
	return nil
}

// setChunkNameFormat converts pattern based chunk name format
// into Printf format and Regular expressions for data and
// control chunks.
func (f *Fs) setChunkNameFormat(pattern string) error {
	// validate pattern
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
	if dir, _ := path.Split(pattern); dir != "" {
		return errors.New("directory separator prohibited")
	}
	if pattern[0] != '*' {
		return errors.New("pattern must start with asterisk") // to be lifted later
	}

	// craft a unified regular expression for all types of chunks
	reHashes := regexp.MustCompile("[#]+")
	reDigits := "[0-9]+"
	if numDigits > 1 {
		reDigits = fmt.Sprintf("[0-9]{%d,}", numDigits)
	}
	reDataOrCtrl := fmt.Sprintf("(?:(%s)|_(%s))", reDigits, ctrlTypeRegStr)

	// this must be non-greedy or else it could eat up temporary suffix
	const mainNameRegStr = "(.+?)"

	strRegex := regexp.QuoteMeta(pattern)
	strRegex = reHashes.ReplaceAllLiteralString(strRegex, reDataOrCtrl)
	strRegex = strings.Replace(strRegex, "\\*", mainNameRegStr, -1)
	strRegex = fmt.Sprintf("^%s(?:%s|%s)?$", strRegex, tempSuffixRegStr, tempSuffixRegOld)
	f.nameRegexp = regexp.MustCompile(strRegex)

	// craft printf formats for active data/control chunks
	fmtDigits := "%d"
	if numDigits > 1 {
		fmtDigits = fmt.Sprintf("%%0%dd", numDigits)
	}
	strFmt := strings.Replace(pattern, "%", "%%", -1)
	strFmt = strings.Replace(strFmt, "*", "%s", 1)
	f.dataNameFmt = reHashes.ReplaceAllLiteralString(strFmt, fmtDigits)
	f.ctrlNameFmt = reHashes.ReplaceAllLiteralString(strFmt, "_%s")
	return nil
}

// makeChunkName produces chunk name (or path) for a given file.
//
// filePath can be name, relative or absolute path of main file.
//
// chunkNo must be a zero based index of data chunk.
// Negative chunkNo eg. -1 indicates a control chunk.
// ctrlType is type of control chunk (must be valid).
// ctrlType must be "" for data chunks.
//
// xactID is a transaction identifier. Empty xactID denotes active chunk,
// otherwise temporary chunk name is produced.
//
func (f *Fs) makeChunkName(filePath string, chunkNo int, ctrlType, xactID string) string {
	dir, parentName := path.Split(filePath)
	var name, tempSuffix string
	switch {
	case chunkNo >= 0 && ctrlType == "":
		name = fmt.Sprintf(f.dataNameFmt, parentName, chunkNo+f.opt.StartFrom)
	case chunkNo < 0 && ctrlTypeRegexp.MatchString(ctrlType):
		name = fmt.Sprintf(f.ctrlNameFmt, parentName, ctrlType)
	default:
		panic("makeChunkName: invalid argument") // must not produce something we can't consume
	}
	if xactID != "" {
		tempSuffix = fmt.Sprintf(tempSuffixFormat, xactID)
		if !tempSuffixRegexp.MatchString(tempSuffix) {
			panic("makeChunkName: invalid argument")
		}
	}
	return dir + name + tempSuffix
}

// parseChunkName checks whether given file path belongs to
// a chunk and extracts chunk name parts.
//
// filePath can be name, relative or absolute path of a file.
//
// Returned parentPath is path of the composite file owning the chunk.
// It's a non-empty string if valid chunk name is detected
// or "" if it's not a chunk.
// Other returned values depend on detected chunk type:
// data or control, active or temporary:
//
// data chunk - the returned chunkNo is non-negative and ctrlType is ""
// control chunk - the chunkNo is -1 and ctrlType is a non-empty string
// active chunk - the returned xactID is ""
// temporary chunk - the xactID is a non-empty string
func (f *Fs) parseChunkName(filePath string) (parentPath string, chunkNo int, ctrlType, xactID string) {
	dir, name := path.Split(filePath)
	match := f.nameRegexp.FindStringSubmatch(name)
	if match == nil || match[1] == "" {
		return "", -1, "", ""
	}
	var err error

	chunkNo = -1
	if match[2] != "" {
		if chunkNo, err = strconv.Atoi(match[2]); err != nil {
			chunkNo = -1
		}
		if chunkNo -= f.opt.StartFrom; chunkNo < 0 {
			fs.Infof(f, "invalid data chunk number in file %q", name)
			return "", -1, "", ""
		}
	}

	if match[4] != "" {
		xactID = match[4]
	}
	if match[5] != "" {
		// old-style temporary suffix
		number, err := strconv.ParseInt(match[5], 10, 64)
		if err != nil || number < 0 {
			fs.Infof(f, "invalid old-style transaction number in file %q", name)
			return "", -1, "", ""
		}
		// convert old-style transaction number to base-36 transaction ID
		xactID = fmt.Sprintf(tempSuffixFormat, strconv.FormatInt(number, 36))
		xactID = xactID[1:] // strip leading underscore
	}

	parentPath = dir + match[1]
	ctrlType = match[3]
	return
}

// forbidChunk prints error message or raises error if file is chunk.
// First argument sets log prefix, use `false` to suppress message.
func (f *Fs) forbidChunk(o interface{}, filePath string) error {
	if parentPath, _, _, _ := f.parseChunkName(filePath); parentPath != "" {
		if f.opt.FailHard {
			return fmt.Errorf("chunk overlap with %q", parentPath)
		}
		if boolVal, isBool := o.(bool); !isBool || boolVal {
			fs.Errorf(o, "chunk overlap with %q", parentPath)
		}
	}
	return nil
}

// newXactID produces a sufficiently random transaction identifier.
//
// The temporary suffix mask allows identifiers consisting of 4-9
// base-36 digits (ie. digits 0-9 or lowercase letters a-z).
// The identifiers must be unique between transactions running on
// the single file in parallel.
//
// Currently the function produces 6-character identifiers.
// Together with underscore this makes a 7-character temporary suffix.
//
// The first 4 characters isolate groups of transactions by time intervals.
// The maximum length of interval is base-36 "zzzz" ie. 1,679,615 seconds.
// The function rather takes a maximum prime closest to this number
// (see https://primes.utm.edu) as the interval length to better safeguard
// against repeating pseudo-random sequences in cases when rclone is
// invoked from a periodic scheduler like unix cron.
// Thus, the interval is slightly more than 19 days 10 hours 33 minutes.
//
// The remaining 2 base-36 digits (in the range from 0 to 1295 inclusive)
// are taken from the local random source.
// This provides about 0.1% collision probability for two parallel
// operations started at the same second and working on the same file.
//
// Non-empty filePath argument enables probing for existing temporary chunk
// to further eliminate collisions.
func (f *Fs) newXactID(ctx context.Context, filePath string) (xactID string, err error) {
	const closestPrimeZzzzSeconds = 1679609
	const maxTwoBase36Digits = 1295

	unixSec := time.Now().Unix()
	if unixSec < 0 {
		unixSec = -unixSec // unlikely but the number must be positive
	}
	circleSec := unixSec % closestPrimeZzzzSeconds
	first4chars := strconv.FormatInt(circleSec, 36)

	for tries := 0; tries < maxTransactionProbes; tries++ {
		f.xactIDMutex.Lock()
		randomness := f.xactIDRand.Int63n(maxTwoBase36Digits + 1)
		f.xactIDMutex.Unlock()

		last2chars := strconv.FormatInt(randomness, 36)
		xactID = fmt.Sprintf("%04s%02s", first4chars, last2chars)

		if filePath == "" {
			return
		}
		probeChunk := f.makeChunkName(filePath, 0, "", xactID)
		_, probeErr := f.base.NewObject(ctx, probeChunk)
		if probeErr != nil {
			return
		}
	}

	return "", fmt.Errorf("can't setup transaction for %s", filePath)
}

// List the objects and directories in dir into entries.
// The entries can be returned in any order but should be
// for a complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't found.
//
// Commands normally cleanup all temporary chunks in case of a failure.
// However, if rclone dies unexpectedly, it can leave behind a bunch of
// hidden temporary chunks. List and its underlying chunkEntries()
// silently skip all temporary chunks in the directory. It's okay if
// they belong to an unfinished command running in parallel.
//
// However, there is no way to discover dead temporary chunks atm.
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
	return f.processEntries(ctx, entries, dir)
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
// of listing recursively than doing a directory traversal.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	do := f.base.Features().ListR
	return do(ctx, dir, func(entries fs.DirEntries) error {
		newEntries, err := f.processEntries(ctx, entries, dir)
		if err != nil {
			return err
		}
		return callback(newEntries)
	})
}

// processEntries assembles chunk entries into composite entries
func (f *Fs) processEntries(ctx context.Context, origEntries fs.DirEntries, dirPath string) (newEntries fs.DirEntries, err error) {
	var sortedEntries fs.DirEntries
	if f.dirSort {
		// sort entries so that meta objects go before their chunks
		sortedEntries = make(fs.DirEntries, len(origEntries))
		copy(sortedEntries, origEntries)
		sort.Sort(sortedEntries)
	} else {
		sortedEntries = origEntries
	}

	byRemote := make(map[string]*Object)
	badEntry := make(map[string]bool)
	isSubdir := make(map[string]bool)

	var tempEntries fs.DirEntries
	for _, dirOrObject := range sortedEntries {
		switch entry := dirOrObject.(type) {
		case fs.Object:
			remote := entry.Remote()
			if mainRemote, chunkNo, ctrlType, xactID := f.parseChunkName(remote); mainRemote != "" {
				if xactID != "" {
					if revealHidden {
						fs.Infof(f, "ignore temporary chunk %q", remote)
					}
					break
				}
				if ctrlType != "" {
					if revealHidden {
						fs.Infof(f, "ignore control chunk %q", remote)
					}
					break
				}
				mainObject := byRemote[mainRemote]
				if mainObject == nil && f.useMeta {
					fs.Debugf(f, "skip chunk %q without meta object", remote)
					break
				}
				if mainObject == nil {
					// useMeta is false - create chunked object without metadata
					mainObject = f.newObject(mainRemote, nil, nil)
					byRemote[mainRemote] = mainObject
					if !badEntry[mainRemote] {
						tempEntries = append(tempEntries, mainObject)
					}
				}
				if err := mainObject.addChunk(entry, chunkNo); err != nil {
					if f.opt.FailHard {
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
			if f.opt.FailHard {
				return nil, fmt.Errorf("Unknown object type %T", entry)
			}
			fs.Debugf(f, "unknown object type %T", entry)
		}
	}

	for _, entry := range tempEntries {
		if object, ok := entry.(*Object); ok {
			remote := object.Remote()
			if isSubdir[remote] {
				if f.opt.FailHard {
					return nil, fmt.Errorf("%q is both meta object and directory", remote)
				}
				badEntry[remote] = true // fall thru
			}
			if badEntry[remote] {
				fs.Debugf(f, "invalid directory entry %q", remote)
				continue
			}
			if err := object.validate(); err != nil {
				if f.opt.FailHard {
					return nil, err
				}
				fs.Debugf(f, "invalid chunks in object %q", remote)
				continue
			}
		}
		newEntries = append(newEntries, entry)
	}

	if f.dirSort {
		sort.Sort(newEntries)
	}
	return newEntries, nil
}

// NewObject finds the Object at remote.
//
// Please note that every NewObject invocation will scan the whole directory.
// Using here something like fs.DirCache might improve performance
// (yet making the logic more complex).
//
// Note that chunker prefers analyzing file names rather than reading
// the content of meta object assuming that directory scans are fast
// but opening even a small file can be slow on some backends.
//
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	if err := f.forbidChunk(false, remote); err != nil {
		return nil, errors.Wrap(err, "can't access")
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

		// Chunker's meta object cannot be large and maxMetadataSize acts
		// as a hard limit. Anything larger than that is treated as a
		// non-chunked file without even checking its contents, so it's
		// paramount to prevent metadata from exceeding the maximum size.
		o = f.newObject("", baseObj, nil)
		if o.size > maxMetadataSize {
			return o, nil
		}
	} else {
		// Metadata is disabled, hence this is either a multi-chunk
		// composite file without meta object or a non-chunked file.
		// Create an empty wrapper here, scan directory to determine
		// which case it is and postpone reading if it's the latter one.
		o = f.newObject(remote, nil, nil)
	}

	// If the object is small, it's probably a meta object.
	// However, composite file must have data chunks besides it.
	// Scan directory for possible data chunks now and decide later on.
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
		return nil, errors.Wrap(err, "can't detect composite file")
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
		mainRemote, chunkNo, ctrlType, xactID := f.parseChunkName(entryRemote)
		if mainRemote == "" || mainRemote != remote || ctrlType != "" || xactID != "" {
			continue // skip non-conforming, temporary and control chunks
		}
		//fs.Debugf(f, "%q belongs to %q as chunk %d", entryRemote, mainRemote, chunkNo)
		if err := o.addChunk(entry, chunkNo); err != nil {
			return nil, err
		}
	}

	if o.main == nil && (o.chunks == nil || len(o.chunks) == 0) {
		// Scanning hasn't found data chunks with conforming names.
		if f.useMeta {
			// Metadata is required but absent and there are no chunks.
			return nil, fs.ErrorObjectNotFound
		}

		// Data chunks are not found and metadata is disabled.
		// Thus, we are in the "latter case" from above.
		// Let's try the postponed reading of a non-chunked file and add it
		// as a single chunk to the empty composite wrapper created above
		// with nil metadata.
		baseObj, err = f.base.NewObject(ctx, remote)
		if err == nil {
			err = o.addChunk(baseObj, 0)
		}
		if err != nil {
			return nil, err
		}
	}

	// This is either a composite object with metadata or a non-chunked
	// file without metadata. Validate it and update the total data size.
	// As an optimization, skip metadata reading here - we will call
	// readMetadata lazily when needed (reading can be expensive).
	if err := o.validate(); err != nil {
		return nil, err
	}
	return o, nil
}

func (o *Object) readMetadata(ctx context.Context) error {
	if o.isFull {
		return nil
	}
	if !o.isComposite() || !o.f.useMeta {
		o.isFull = true
		return nil
	}

	// validate metadata
	metaObject := o.main
	reader, err := metaObject.Open(ctx)
	if err != nil {
		return err
	}
	metadata, err := ioutil.ReadAll(reader)
	_ = reader.Close() // ensure file handle is freed on windows
	if err != nil {
		return err
	}

	switch o.f.opt.MetaFormat {
	case "simplejson":
		metaInfo, err := unmarshalSimpleJSON(ctx, metaObject, metadata, true)
		if err != nil {
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
	xactID, errXact := f.newXactID(ctx, baseRemote)
	if errXact != nil {
		return nil, errXact
	}

	// Transfer chunks data
	for c.chunkNo = 0; !c.done; c.chunkNo++ {
		if c.chunkNo > maxSafeChunkNumber {
			return nil, ErrChunkOverflow
		}

		tempRemote := f.makeChunkName(baseRemote, c.chunkNo, "", xactID)
		size := c.sizeLeft
		if size > c.chunkSize {
			size = c.chunkSize
		}
		savedReadCount := c.readCount

		// If a single chunk is expected, avoid the extra rename operation
		chunkRemote := tempRemote
		if c.expectSingle && c.chunkNo == 0 && optimizeFirstChunk {
			chunkRemote = baseRemote
		}
		info := f.wrapInfo(src, chunkRemote, size)

		// TODO: handle range/limit options
		chunk, errChunk := basePut(ctx, wrapIn, info, options...)
		if errChunk != nil {
			return nil, errChunk
		}

		if size > 0 && c.readCount == savedReadCount && c.expectSingle {
			// basePut returned success but didn't call chunkingReader's Read.
			// This is possible if wrapped remote has performed the put by hash
			// because chunker bridges Hash from source for non-chunked files.
			// Hence, force Read here to update accounting and hashsums.
			if err := c.dummyRead(wrapIn, size); err != nil {
				return nil, err
			}
		}
		if c.sizeLeft == 0 && !c.done {
			// The file has been apparently put by hash, force completion.
			c.done = true
		}

		// Expected a single chunk but more to come, so name it as usual.
		if !c.done && chunkRemote != tempRemote {
			fs.Infof(chunk, "Expected single chunk, got more")
			chunkMoved, errMove := f.baseMove(ctx, chunk, tempRemote, delFailed)
			if errMove != nil {
				silentlyRemove(ctx, chunk)
				return nil, errMove
			}
			chunk = chunkMoved
		}

		// Wrapped remote may or may not have seen EOF from chunking reader,
		// eg. the box multi-uploader reads exactly the chunk size specified
		// and skips the "EOF" read. Hence, switch to next limit here.
		if !(c.chunkLimit == 0 || c.chunkLimit == c.chunkSize || c.sizeTotal == -1 || c.done) {
			silentlyRemove(ctx, chunk)
			return nil, fmt.Errorf("Destination ignored %d data bytes", c.chunkLimit)
		}
		c.chunkLimit = c.chunkSize

		c.chunks = append(c.chunks, chunk)
	}

	// Validate uploaded size
	if c.sizeTotal != -1 && c.readCount != c.sizeTotal {
		return nil, fmt.Errorf("Incorrect upload size %d != %d", c.readCount, c.sizeTotal)
	}

	// Check for input that looks like valid metadata
	needMeta := len(c.chunks) > 1
	if c.readCount <= maxMetadataSize && len(c.chunks) == 1 {
		_, err := unmarshalSimpleJSON(ctx, c.chunks[0], c.smallHead, false)
		needMeta = err == nil
	}

	// Finalize small object as non-chunked.
	// This can be bypassed, and single chunk with metadata will be
	// created if forced by consistent hashing or due to unsafe input.
	if !needMeta && !f.hashAll && f.useMeta {
		// If previous object was chunked, remove its chunks
		f.removeOldChunks(ctx, baseRemote)

		// Rename single data chunk in place
		chunk := c.chunks[0]
		if chunk.Remote() != baseRemote {
			chunkMoved, errMove := f.baseMove(ctx, chunk, baseRemote, delAlways)
			if errMove != nil {
				silentlyRemove(ctx, chunk)
				return nil, errMove
			}
			chunk = chunkMoved
		}

		return f.newObject("", chunk, nil), nil
	}

	// Validate total size of data chunks
	var sizeTotal int64
	for _, chunk := range c.chunks {
		sizeTotal += chunk.Size()
	}
	if sizeTotal != c.readCount {
		return nil, fmt.Errorf("Incorrect chunks size %d != %d", sizeTotal, c.readCount)
	}

	// If previous object was chunked, remove its chunks
	f.removeOldChunks(ctx, baseRemote)

	// Rename data chunks from temporary to final names
	for chunkNo, chunk := range c.chunks {
		chunkRemote := f.makeChunkName(baseRemote, chunkNo, "", "")
		chunkMoved, errMove := f.baseMove(ctx, chunk, chunkRemote, delFailed)
		if errMove != nil {
			return nil, errMove
		}
		c.chunks[chunkNo] = chunkMoved
	}

	if !f.useMeta {
		// Remove stale metadata, if any
		oldMeta, errOldMeta := f.base.NewObject(ctx, baseRemote)
		if errOldMeta == nil {
			silentlyRemove(ctx, oldMeta)
		}

		o := f.newObject(baseRemote, nil, c.chunks)
		o.size = sizeTotal
		return o, nil
	}

	// Update meta object
	var metadata []byte
	switch f.opt.MetaFormat {
	case "simplejson":
		c.updateHashes()
		metadata, err = marshalSimpleJSON(ctx, sizeTotal, len(c.chunks), c.md5, c.sha1)
	}
	if err == nil {
		metaInfo := f.wrapInfo(src, baseRemote, int64(len(metadata)))
		metaObject, err = basePut(ctx, bytes.NewReader(metadata), metaInfo)
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
	chunkNo      int
	err          error
	done         bool
	chunks       []fs.Object
	expectSingle bool
	smallHead    []byte
	fs           *Fs
	hasher       gohash.Hash
	md5          string
	sha1         string
}

func (f *Fs) newChunkingReader(src fs.ObjectInfo) *chunkingReader {
	c := &chunkingReader{
		fs:        f,
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
			if c.fs.hashFallback {
				c.sha1, _ = src.Hash(ctx, hash.SHA1)
			} else {
				c.hasher = md5.New()
			}
		}
	case c.fs.useSHA1:
		if c.sha1, _ = src.Hash(ctx, hash.SHA1); c.sha1 == "" {
			if c.fs.hashFallback {
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
	if c.chunkNo == 0 && c.expectSingle && bytesRead > 0 && c.readCount <= maxMetadataSize {
		c.smallHead = append(c.smallHead, buf[:bytesRead]...)
	}
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

// dummyRead updates accounting, hashsums etc by simulating reads
func (c *chunkingReader) dummyRead(in io.Reader, size int64) error {
	if c.hasher == nil && c.readCount+size > maxMetadataSize {
		c.accountBytes(size)
		return nil
	}
	const bufLen = 1048576 // 1MB
	buf := make([]byte, bufLen)
	for size > 0 {
		n := size
		if n > bufLen {
			n = bufLen
		}
		if _, err := io.ReadFull(in, buf[0:n]); err != nil {
			return err
		}
		size -= n
	}
	return nil
}

// rollback removes uploaded temporary chunks
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

// Put into the remote path with the given modTime and size.
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	if err := f.forbidChunk(src, src.Remote()); err != nil {
		return nil, errors.Wrap(err, "refusing to put")
	}
	return f.put(ctx, in, src, src.Remote(), options, f.base.Put)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	if err := f.forbidChunk(src, src.Remote()); err != nil {
		return nil, errors.Wrap(err, "refusing to upload")
	}
	return f.put(ctx, in, src, src.Remote(), options, f.base.Features().PutStream)
}

// Update in to the object with the modTime given of the given size
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	if err := o.f.forbidChunk(o, o.Remote()); err != nil {
		return errors.Wrap(err, "update refused")
	}
	if err := o.readMetadata(ctx); err != nil {
		// refuse to update a file of unsupported format
		return errors.Wrap(err, "refusing to update")
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
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	do := f.base.Features().PutUnchecked
	if do == nil {
		return nil, errors.New("can't PutUnchecked")
	}
	// TODO: handle range/limit options and really chunk stream here!
	o, err := do(ctx, in, f.wrapInfo(src, "", -1))
	if err != nil {
		return nil, err
	}
	return f.newObject("", o, nil), nil
}

// Hashes returns the supported hash sets.
// Chunker advertises a hash type if and only if it can be calculated
// for files of any size, non-chunked or composite.
func (f *Fs) Hashes() hash.Set {
	// composites AND no fallback AND (chunker OR wrapped Fs will hash all non-chunked's)
	if f.useMD5 && !f.hashFallback && (f.hashAll || f.base.Hashes().Contains(hash.MD5)) {
		return hash.NewHashSet(hash.MD5)
	}
	if f.useSHA1 && !f.hashFallback && (f.hashAll || f.base.Hashes().Contains(hash.SHA1)) {
		return hash.NewHashSet(hash.SHA1)
	}
	return hash.NewHashSet() // can't provide strong guarantees
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	if err := f.forbidChunk(dir, dir); err != nil {
		return errors.Wrap(err, "can't mkdir")
	}
	return f.base.Mkdir(ctx, dir)
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.base.Rmdir(ctx, dir)
}

// Purge all files in the directory
//
// Implement this if you have a way of deleting all the files
// quicker than just running Remove() on the result of List()
//
// Return an error if it doesn't exist.
//
// This command will chain to `purge` from wrapped remote.
// As a result it removes not only composite chunker files with their
// active chunks but also all hidden temporary chunks in the directory.
//
func (f *Fs) Purge(ctx context.Context, dir string) error {
	do := f.base.Features().Purge
	if do == nil {
		return fs.ErrorCantPurge
	}
	return do(ctx, dir)
}

// Remove an object (chunks and metadata, if any)
//
// Remove deletes only active chunks of the composite object.
// It does not try to look for temporary chunks because they could belong
// to another command modifying this composite file in parallel.
//
// Commands normally cleanup all temporary chunks in case of a failure.
// However, if rclone dies unexpectedly, it can leave hidden temporary
// chunks, which cannot be discovered using the `list` command.
// Remove does not try to search for such chunks or to delete them.
// Sometimes this can lead to strange results eg. when `list` shows that
// directory is empty but `rmdir` refuses to remove it because on the
// level of wrapped remote it's actually *not* empty.
// As a workaround users can use `purge` to forcibly remove it.
//
// In future, a flag `--chunker-delete-hidden` may be added which tells
// Remove to search directory for hidden chunks and remove them too
// (at the risk of breaking parallel commands).
//
// Remove is the only operation allowed on the composite files with
// invalid or future metadata format.
// We don't let user copy/move/update unsupported composite files.
// Let's at least let her get rid of them, just complain loudly.
//
// This can litter directory with orphan chunks of unsupported types,
// but as long as we remove meta object, even future releases will
// treat the composite file as removed and refuse to act upon it.
//
// Disclaimer: corruption can still happen if unsupported file is removed
// and then recreated with the same name.
// Unsupported control chunks will get re-picked by a more recent
// rclone version with unexpected results. This can be helped by
// the `delete hidden` flag above or at least the user has been warned.
//
func (o *Object) Remove(ctx context.Context) (err error) {
	if err := o.f.forbidChunk(o, o.Remote()); err != nil {
		// operations.Move can still call Remove if chunker's Move refuses
		// to corrupt file in hard mode. Hence, refuse to Remove, too.
		return errors.Wrap(err, "refuse to corrupt")
	}
	if err := o.readMetadata(ctx); err != nil {
		// Proceed but warn user that unexpected things can happen.
		fs.Errorf(o, "Removing a file with unsupported metadata: %v", err)
	}

	// Remove non-chunked file or meta object of a composite file.
	if o.main != nil {
		err = o.main.Remove(ctx)
	}

	// Remove only active data chunks, ignore any temporary chunks that
	// might probably be created in parallel by other transactions.
	for _, chunk := range o.chunks {
		chunkErr := chunk.Remove(ctx)
		if err == nil {
			err = chunkErr
		}
	}

	// There are no known control chunks to remove atm.
	return err
}

// copyOrMove implements copy or move
func (f *Fs) copyOrMove(ctx context.Context, o *Object, remote string, do copyMoveFn, md5, sha1, opName string) (fs.Object, error) {
	if err := f.forbidChunk(o, remote); err != nil {
		return nil, errors.Wrapf(err, "can't %s", opName)
	}
	if !o.isComposite() {
		fs.Debugf(o, "%s non-chunked object...", opName)
		oResult, err := do(ctx, o.mainChunk(), remote) // chain operation to a single wrapped chunk
		if err != nil {
			return nil, err
		}
		return f.newObject("", oResult, nil), nil
	}
	if err := o.readMetadata(ctx); err != nil {
		// Refuse to copy/move composite files with invalid or future
		// metadata format which might involve unsupported chunk types.
		return nil, errors.Wrapf(err, "can't %s this file", opName)
	}

	fs.Debugf(o, "%s %d data chunks...", opName, len(o.chunks))
	mainRemote := o.remote
	var newChunks []fs.Object
	var err error

	// Copy/move active data chunks.
	// Ignore possible temporary chunks being created by parallel operations.
	for _, chunk := range o.chunks {
		chunkRemote := chunk.Remote()
		if !strings.HasPrefix(chunkRemote, mainRemote) {
			err = fmt.Errorf("invalid chunk name %q", chunkRemote)
			break
		}
		chunkSuffix := chunkRemote[len(mainRemote):]
		chunkResult, err := do(ctx, chunk, remote+chunkSuffix)
		if err != nil {
			break
		}
		newChunks = append(newChunks, chunkResult)
	}

	// Copy or move old metadata.
	// There are no known control chunks to move/copy atm.
	var metaObject fs.Object
	if err == nil && o.main != nil {
		metaObject, err = do(ctx, o.main, remote)
	}
	if err != nil {
		for _, chunk := range newChunks {
			silentlyRemove(ctx, chunk)
		}
		return nil, err
	}

	// Create wrapping object, calculate and validate total size
	newObj := f.newObject(remote, metaObject, newChunks)
	err = newObj.validate()
	if err != nil {
		silentlyRemove(ctx, newObj)
		return nil, err
	}

	// Update metadata
	var metadata []byte
	switch f.opt.MetaFormat {
	case "simplejson":
		metadata, err = marshalSimpleJSON(ctx, newObj.size, len(newChunks), md5, sha1)
		if err == nil {
			metaInfo := f.wrapInfo(metaObject, "", int64(len(metadata)))
			err = newObj.main.Update(ctx, bytes.NewReader(metadata), metaInfo)
		}
	case "none":
		if newObj.main != nil {
			err = newObj.main.Remove(ctx)
		}
	}

	// Return the composite object
	if err != nil {
		silentlyRemove(ctx, newObj)
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

	requireMetaHash := obj.isComposite() && f.opt.MetaFormat == "simplejson"
	if !requireMetaHash && !f.hashAll {
		ok = true // hash is not required for metadata
		return
	}

	switch {
	case f.useMD5:
		md5, _ = obj.Hash(ctx, hash.MD5)
		ok = md5 != ""
		if !ok && f.hashFallback {
			sha1, _ = obj.Hash(ctx, hash.SHA1)
			ok = sha1 != ""
		}
	case f.useSHA1:
		sha1, _ = obj.Hash(ctx, hash.SHA1)
		ok = sha1 != ""
		if !ok && f.hashFallback {
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
		return f.baseMove(ctx, src, remote, delNever)
	}
	obj, md5, sha1, ok := f.okForServerSide(ctx, src, "move")
	if !ok {
		return nil, fs.ErrorCantMove
	}
	return f.copyOrMove(ctx, obj, remote, baseMove, md5, sha1, "move")
}

// baseMove chains to the wrapped Move or simulates it by Copy+Delete
func (f *Fs) baseMove(ctx context.Context, src fs.Object, remote string, delMode int) (fs.Object, error) {
	var (
		dest fs.Object
		err  error
	)
	switch delMode {
	case delAlways:
		dest, err = f.base.NewObject(ctx, remote)
	case delFailed:
		dest, err = operations.Move(ctx, f.base, nil, remote, src)
		if err == nil {
			return dest, err
		}
		dest, err = f.base.NewObject(ctx, remote)
	case delNever:
		// fall thru, the default
	}
	if err != nil {
		dest = nil
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
//
// Replace data chunk names by the name of composite file.
// Ignore temporary and control chunks.
func (f *Fs) ChangeNotify(ctx context.Context, notifyFunc func(string, fs.EntryType), pollIntervalChan <-chan time.Duration) {
	do := f.base.Features().ChangeNotify
	if do == nil {
		return
	}
	wrappedNotifyFunc := func(path string, entryType fs.EntryType) {
		//fs.Debugf(f, "ChangeNotify: path %q entryType %d", path, entryType)
		if entryType == fs.EntryObject {
			mainPath, _, _, xactID := f.parseChunkName(path)
			if mainPath != "" && xactID == "" {
				path = mainPath
			}
		}
		notifyFunc(path, entryType)
	}
	do(ctx, wrappedNotifyFunc, pollIntervalChan)
}

// Object represents a composite file wrapping one or more data chunks
type Object struct {
	remote string
	main   fs.Object   // meta object if file is composite, or wrapped non-chunked file, nil if meta format is 'none'
	chunks []fs.Object // active data chunks if file is composite, or wrapped file as a single chunk if meta format is 'none'
	size   int64       // cached total size of chunks in a composite file or -1 for non-chunked files
	isFull bool        // true if metadata has been read
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
	if chunkNo > maxSafeChunkNumber {
		return ErrChunkOverflow
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
	if !o.isComposite() {
		_ = o.mainChunk() // verify that single wrapped chunk exists
		return nil
	}

	metaObject := o.main // this file is composite - o.main refers to meta object (or nil if meta format is 'none')
	if metaObject != nil && metaObject.Size() > maxMetadataSize {
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
	o.size = totalSize // cache up the total data size
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
// - a wrapped object for non-chunked files
// - meta object for chunked files with metadata
// - first chunk for chunked files without metadata
// Never returns nil.
func (o *Object) mainChunk() fs.Object {
	if o.main != nil {
		return o.main // meta object or non-chunked wrapped file
	}
	if o.chunks != nil {
		return o.chunks[0] // first chunk of a chunked composite file
	}
	panic("invalid chunked object") // very unlikely
}

func (o *Object) isComposite() bool {
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
	if o.isComposite() {
		return o.size // total size of data chunks in a composite file
	}
	return o.mainChunk().Size() // size of wrapped non-chunked file
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
	if err := o.readMetadata(ctx); err != nil {
		return err // refuse to act on unsupported format
	}
	return o.mainChunk().SetModTime(ctx, mtime)
}

// Hash returns the selected checksum of the file.
// If no checksum is available it returns "".
//
// Hash won't fail with `unsupported` error but return empty
// hash string if a particular hashsum type is not supported
//
// Hash takes hashsum from metadata if available or requests it
// from wrapped remote for non-chunked files.
// Metadata (if meta format is not 'none') is by default kept
// only for composite files. In the "All" hashing mode chunker
// will force metadata on all files if particular hashsum type
// is not supported by wrapped remote.
//
// Note that Hash prefers the wrapped hashsum for non-chunked
// file, then tries to read it from metadata. This in theory
// handles the unusual case when a small file has been tampered
// on the level of wrapped remote but chunker is unaware of that.
//
func (o *Object) Hash(ctx context.Context, hashType hash.Type) (string, error) {
	if !o.isComposite() {
		// First, chain to the wrapped non-chunked file if possible.
		if value, err := o.mainChunk().Hash(ctx, hashType); err == nil && value != "" {
			return value, nil
		}
	}
	if err := o.readMetadata(ctx); err != nil {
		return "", err // valid metadata is required to get hash, abort
	}
	// Try hash from metadata if the file is composite or if wrapped remote fails.
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
	if !o.isComposite() {
		return o.mainChunk().Open(ctx, options...) // chain to wrapped non-chunked file
	}
	if err := o.readMetadata(ctx); err != nil {
		// refuse to open unsupported format
		return nil, errors.Wrap(err, "can't open")
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
			// pass Options on to the wrapped open, if appropriate
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
	nChunks int    // number of data chunks
	size    int64  // overrides source size by the total size of data chunks
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

// Hash returns the selected checksum of the wrapped file
// It returns "" if no checksum is available or if this
// info doesn't wrap the complete file.
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
		// fail if this info wraps only a part of the file
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
	// required core fields
	Version  *int   `json:"ver"`
	Size     *int64 `json:"size"`    // total size of data chunks
	ChunkNum *int   `json:"nchunks"` // number of data chunks
	// optional extra fields
	MD5  string `json:"md5,omitempty"`
	SHA1 string `json:"sha1,omitempty"`
}

// marshalSimpleJSON
//
// Current implementation creates metadata in three cases:
// - for files larger than chunk size
// - if file contents can be mistaken as meta object
// - if consistent hashing is On but wrapped remote can't provide given hash
//
func marshalSimpleJSON(ctx context.Context, size int64, nChunks int, md5, sha1 string) ([]byte, error) {
	version := metadataVersion
	metadata := metaSimpleJSON{
		// required core fields
		Version:  &version,
		Size:     &size,
		ChunkNum: &nChunks,
		// optional extra fields
		MD5:  md5,
		SHA1: sha1,
	}
	data, err := json.Marshal(&metadata)
	if err == nil && data != nil && len(data) >= maxMetadataSize {
		// be a nitpicker, never produce something you can't consume
		return nil, errors.New("metadata can't be this big, please report to rclone developers")
	}
	return data, err
}

// unmarshalSimpleJSON
//
// Only metadata format version 1 is supported atm.
// Future releases will transparently migrate older metadata objects.
// New format will have a higher version number and cannot be correctly
// handled by current implementation.
// The version check below will then explicitly ask user to upgrade rclone.
//
func unmarshalSimpleJSON(ctx context.Context, metaObject fs.Object, data []byte, strictChecks bool) (info *ObjectInfo, err error) {
	// Be strict about JSON format
	// to reduce possibility that a random small file resembles metadata.
	if data != nil && len(data) > maxMetadataSize {
		return nil, errors.New("too big")
	}
	if data == nil || len(data) < 2 || data[0] != '{' || data[len(data)-1] != '}' {
		return nil, errors.New("invalid json")
	}
	var metadata metaSimpleJSON
	err = json.Unmarshal(data, &metadata)
	if err != nil {
		return nil, err
	}
	// Basic fields are strictly required
	// to reduce possibility that a random small file resembles metadata.
	if metadata.Version == nil || metadata.Size == nil || metadata.ChunkNum == nil {
		return nil, errors.New("missing required field")
	}
	// Perform strict checks, avoid corruption of future metadata formats.
	if *metadata.Version < 1 {
		return nil, errors.New("wrong version")
	}
	if *metadata.Size < 0 {
		return nil, errors.New("negative file size")
	}
	if *metadata.ChunkNum < 0 {
		return nil, errors.New("negative number of chunks")
	}
	if *metadata.ChunkNum > maxSafeChunkNumber {
		return nil, ErrChunkOverflow
	}
	if metadata.MD5 != "" {
		_, err = hex.DecodeString(metadata.MD5)
		if len(metadata.MD5) != 32 || err != nil {
			return nil, errors.New("wrong md5 hash")
		}
	}
	if metadata.SHA1 != "" {
		_, err = hex.DecodeString(metadata.SHA1)
		if len(metadata.SHA1) != 40 || err != nil {
			return nil, errors.New("wrong sha1 hash")
		}
	}
	// ChunkNum is allowed to be 0 in future versions
	if *metadata.ChunkNum < 1 && *metadata.Version <= metadataVersion {
		return nil, errors.New("wrong number of chunks")
	}
	// Non-strict mode also accepts future metadata versions
	if *metadata.Version > metadataVersion && strictChecks {
		return nil, fmt.Errorf("version %d is not supported, please upgrade rclone", metadata.Version)
	}

	var nilFs *Fs // nil object triggers appropriate type method
	info = nilFs.wrapInfo(metaObject, "", *metadata.Size)
	info.nChunks = *metadata.ChunkNum
	info.md5 = metadata.MD5
	info.sha1 = metadata.SHA1
	return info, nil
}

func silentlyRemove(ctx context.Context, o fs.Object) {
	_ = o.Remove(ctx) // ignore error
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

// Precision returns the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return f.base.Precision()
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
