package transform

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"

	"github.com/rclone/rclone/fs"
)

type transform struct {
	key   Algo   // for example, "prefix"
	value string // for example, "some_prefix_"
	tag   tag    // file, dir, or all
}

// tag controls which part of the file path is affected (file, dir, all)
type tag int

// tag modes
const (
	file tag = iota // Only transform the leaf name of files (default)
	dir             // Only transform name of directories - these may appear anywhere in the path
	all             // Transform the entire path for files and directories
)

// Transforming returns true when transforms are in use
func Transforming(ctx context.Context) bool {
	ci := fs.GetConfig(ctx)
	return len(ci.NameTransform) > 0
}

// SetOptions sets the options in ctx from flags passed in.
// Any existing flags will be overwritten.
// s should be in the same format as cmd line flags, i.e. "all,prefix=XXX"
func SetOptions(ctx context.Context, s ...string) (err error) {
	ci := fs.GetConfig(ctx)
	ci.NameTransform = s
	_, err = getOptions(ctx)
	return err
}

// cache to minimize re-parsing
var (
	cachedNameTransform []string
	cachedOpt           []transform
	cacheLock           sync.Mutex
)

// getOptions sets the options from flags passed in.
func getOptions(ctx context.Context) (opt []transform, err error) {
	if !Transforming(ctx) {
		return opt, nil
	}

	ci := fs.GetConfig(ctx)

	// return cached opt if available
	if cachedNameTransform != nil && slices.Equal(ci.NameTransform, cachedNameTransform) {
		return cachedOpt, nil
	}

	for _, transform := range ci.NameTransform {
		t, err := parse(transform)
		if err != nil {
			return opt, err
		}
		opt = append(opt, t)
	}
	updateCache(ci.NameTransform, opt)
	return opt, nil
}

func updateCache(nt []string, o []transform) {
	cacheLock.Lock()
	cachedNameTransform = slices.Clone(nt)
	cachedOpt = o
	cacheLock.Unlock()
}

// parse a single instance of --name-transform
func parse(s string) (t transform, err error) {
	if s == "" {
		return t, nil
	}
	s = t.parseTag(s)
	err = t.parseKeyVal(s)
	return t, err
}

// parse the tag (file/dir/all), set the option accordingly, and return the trimmed string
//
// we don't worry about errors here because it will error anyway as an invalid key
func (t *transform) parseTag(s string) string {
	if strings.HasPrefix(s, "file,") {
		t.tag = file
		return strings.TrimPrefix(s, "file,")
	}
	if strings.HasPrefix(s, "dir,") {
		t.tag = dir
		return strings.TrimPrefix(s, "dir,")
	}
	if strings.HasPrefix(s, "all,") {
		t.tag = all
		return strings.TrimPrefix(s, "all,")
	}
	return s
}

// parse key and value (if any) by splitting on '=' sign
// (file/dir/all tag has already been trimmed)
func (t *transform) parseKeyVal(s string) (err error) {
	if !strings.ContainsRune(s, '=') {
		err = t.key.Set(s)
		if err != nil {
			return err
		}
		if t.requiresValue() {
			fs.Debugf(nil, "received %v", s)
			return errors.New("value is required for " + t.key.String())
		}
		return nil
	}
	split := strings.Split(s, "=")
	if len(split) != 2 {
		return errors.New("too many values")
	}
	if split[0] == "" {
		return errors.New("key cannot be blank")
	}
	err = t.key.Set(split[0])
	if err != nil {
		return err
	}
	t.value = split[1]
	return nil
}

// returns true if this particular algorithm requires a value
func (t *transform) requiresValue() bool {
	switch t.key {
	case ConvFindReplace:
		return true
	case ConvPrefix:
		return true
	case ConvSuffix:
		return true
	case ConvSuffixKeepExtension:
		return true
	case ConvTrimPrefix:
		return true
	case ConvTrimSuffix:
		return true
	case ConvIndex:
		return true
	case ConvDate:
		return true
	case ConvTruncate:
		return true
	case ConvTruncateKeepExtension:
		return true
	case ConvTruncateBytes:
		return true
	case ConvTruncateBytesKeepExtension:
		return true
	case ConvEncoder:
		return true
	case ConvDecoder:
		return true
	case ConvRegex:
		return true
	case ConvCommand:
		return true
	}
	return false
}

// Algo describes conversion setting
type Algo = fs.Enum[transformChoices]

// Supported transform options
const (
	ConvNone Algo = iota
	ConvToNFC
	ConvToNFD
	ConvToNFKC
	ConvToNFKD
	ConvFindReplace
	ConvPrefix
	ConvSuffix
	ConvSuffixKeepExtension
	ConvTrimPrefix
	ConvTrimSuffix
	ConvIndex
	ConvDate
	ConvTruncate
	ConvTruncateKeepExtension
	ConvTruncateBytes
	ConvTruncateBytesKeepExtension
	ConvBase64Encode
	ConvBase64Decode
	ConvEncoder
	ConvDecoder
	ConvISO8859_1
	ConvWindows1252
	ConvMacintosh
	ConvCharmap
	ConvLowercase
	ConvUppercase
	ConvTitlecase
	ConvASCII
	ConvURL
	ConvRegex
	ConvCommand
)

type transformChoices struct{}

func (transformChoices) Choices() []string {
	return []string{
		ConvNone:                       "none",
		ConvToNFC:                      "nfc",
		ConvToNFD:                      "nfd",
		ConvToNFKC:                     "nfkc",
		ConvToNFKD:                     "nfkd",
		ConvFindReplace:                "replace",
		ConvPrefix:                     "prefix",
		ConvSuffix:                     "suffix",
		ConvSuffixKeepExtension:        "suffix_keep_extension",
		ConvTrimPrefix:                 "trimprefix",
		ConvTrimSuffix:                 "trimsuffix",
		ConvIndex:                      "index",
		ConvDate:                       "date",
		ConvTruncate:                   "truncate",
		ConvTruncateKeepExtension:      "truncate_keep_extension",
		ConvTruncateBytes:              "truncate_bytes",
		ConvTruncateBytesKeepExtension: "truncate_bytes_keep_extension",
		ConvBase64Encode:               "base64encode",
		ConvBase64Decode:               "base64decode",
		ConvEncoder:                    "encoder",
		ConvDecoder:                    "decoder",
		ConvISO8859_1:                  "ISO-8859-1",
		ConvWindows1252:                "Windows-1252",
		ConvMacintosh:                  "Macintosh",
		ConvCharmap:                    "charmap",
		ConvLowercase:                  "lowercase",
		ConvUppercase:                  "uppercase",
		ConvTitlecase:                  "titlecase",
		ConvASCII:                      "ascii",
		ConvURL:                        "url",
		ConvRegex:                      "regex",
		ConvCommand:                    "command",
	}
}

func (transformChoices) Type() string {
	return "string"
}
