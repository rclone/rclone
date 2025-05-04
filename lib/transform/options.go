package transform

import (
	"context"
	"errors"
	"strings"

	"github.com/rclone/rclone/fs"
)

func init() {
	fs.RegisterGlobalOptions(fs.OptionsInfo{Name: "name_transform", Opt: &Opt.Flags, Options: OptionsInfo, Reload: Reload})
}

type transform struct {
	key   transformAlgo // for example, "prefix"
	value string        // for example, "some_prefix_"
	tag   tag           // file, dir, or all
}

// Options stores the parsed and unparsed transform options.
// their order must never be changed or sorted.
type Options struct {
	Flags      Flags       // unparsed flag value like "file,prefix=ABC"
	transforms []transform // parsed from NameTransform
}

// Flags is a slice of unparsed values set from command line flags or env vars
type Flags struct {
	NameTransform []string `config:"name_transform"`
}

// Opt is the default options modified by the environment variables and command line flags
var Opt Options

// tag controls which part of the file path is affected (file, dir, all)
type tag int

// tag modes
const (
	file tag = iota // Only transform the leaf name of files (default)
	dir             // Only transform name of directories - these may appear anywhere in the path
	all             // Transform the entire path for files and directories
)

// OptionsInfo describes the Options in use
var OptionsInfo = fs.Options{{
	Name:    "name_transform",
	Default: []string{},
	Help:    "TODO",
	Groups:  "Filter",
}}

// Reload the transform options from the flags
func Reload(ctx context.Context) (err error) {
	return newOpt(Opt)
}

// SetOptions sets the options from flags passed in.
// Any existing flags will be overwritten.
// s should be in the same format as cmd line flags, i.e. "all,prefix=XXX"
func SetOptions(ctx context.Context, s ...string) (err error) {
	Opt = Options{Flags: Flags{NameTransform: s}}
	return Reload(ctx)
}

// overwite Opt.transforms with values from Opt.Flags
func newOpt(opt Options) (err error) {
	Opt.transforms = []transform{}

	for _, transform := range opt.Flags.NameTransform {
		t, err := parse(transform)
		if err != nil {
			return err
		}
		Opt.transforms = append(Opt.transforms, t)
	}
	return nil
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
	case ConvEncoder:
		return true
	case ConvDecoder:
		return true
	}
	return false
}

// transformAlgo describes conversion setting
type transformAlgo = fs.Enum[transformChoices]

// Supported transform options
const (
	ConvNone transformAlgo = iota
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
	ConvMapper
)

type transformChoices struct{}

func (transformChoices) Choices() []string {
	return []string{
		ConvNone:                "none",
		ConvToNFC:               "nfc",
		ConvToNFD:               "nfd",
		ConvToNFKC:              "nfkc",
		ConvToNFKD:              "nfkd",
		ConvFindReplace:         "replace",
		ConvPrefix:              "prefix",
		ConvSuffix:              "suffix",
		ConvSuffixKeepExtension: "suffix_keep_extension",
		ConvTrimPrefix:          "trimprefix",
		ConvTrimSuffix:          "trimsuffix",
		ConvIndex:               "index",
		ConvDate:                "date",
		ConvTruncate:            "truncate",
		ConvBase64Encode:        "base64encode",
		ConvBase64Decode:        "base64decode",
		ConvEncoder:             "encoder",
		ConvDecoder:             "decoder",
		ConvISO8859_1:           "ISO-8859-1",
		ConvWindows1252:         "Windows-1252",
		ConvMacintosh:           "Macintosh",
		ConvCharmap:             "charmap",
		ConvLowercase:           "lowercase",
		ConvUppercase:           "uppercase",
		ConvTitlecase:           "titlecase",
		ConvASCII:               "ascii",
		ConvURL:                 "url",
		ConvMapper:              "mapper",
	}
}

func (transformChoices) Type() string {
	return "string"
}
