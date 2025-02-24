// Package convmv provides the convmv command.
package convmv

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/list"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/random"
	"github.com/spf13/cobra"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/unicode/norm"
)

// Globals
var (
	Opt   ConvOpt
	Cmaps = map[int]*charmap.Charmap{}
)

// ConvOpt sets the conversion options
type ConvOpt struct {
	ctx         context.Context
	f           fs.Fs
	ConvertAlgo Convert
	FindReplace []string
	Prefix      string
	Suffix      string
	Max         int
	Enc         encoder.MultiEncoder
	CmapFlag    fs.Enum[cmapChoices]
	Cmap        *charmap.Charmap
	List        bool
}

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.FVarP(cmdFlags, &Opt.ConvertAlgo, "conv", "t", "Conversion algorithm: "+Opt.ConvertAlgo.Help(), "")
	flags.StringVarP(cmdFlags, &Opt.Prefix, "prefix", "", "", "In 'prefix' or 'trimprefix' mode, append or trim this prefix", "")
	flags.StringVarP(cmdFlags, &Opt.Suffix, "suffix", "", "", "In 'suffix' or 'trimsuffix' mode, append or trim this suffix", "")
	flags.IntVarP(cmdFlags, &Opt.Max, "max", "m", -1, "In 'truncate' mode, truncate all path segments longer than this many characters", "")
	flags.StringArrayVarP(cmdFlags, &Opt.FindReplace, "replace", "r", nil, "In 'replace' mode, this is a pair of find,replace values (can repeat flag more than once)", "")
	flags.FVarP(cmdFlags, &Opt.Enc, "encoding", "", "Custom backend encoding: (use --list to see full list)", "")
	flags.FVarP(cmdFlags, &Opt.CmapFlag, "charmap", "", "Other character encoding (use --list to see full list) ", "")
	flags.BoolVarP(cmdFlags, &Opt.List, "list", "", false, "Print full list of options", "")
}

// Convert describes conversion setting
type Convert = fs.Enum[convertChoices]

// Supported conversion options
const (
	ConvNone Convert = iota
	ConvToNFC
	ConvToNFD
	ConvToNFKC
	ConvToNFKD
	ConvFindReplace
	ConvPrefix
	ConvSuffix
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

type convertChoices struct{}

func (convertChoices) Choices() []string {
	return []string{
		ConvNone:         "none",
		ConvToNFC:        "nfc",
		ConvToNFD:        "nfd",
		ConvToNFKC:       "nfkc",
		ConvToNFKD:       "nfkd",
		ConvFindReplace:  "replace",
		ConvPrefix:       "prefix",
		ConvSuffix:       "suffix",
		ConvTrimPrefix:   "trimprefix",
		ConvTrimSuffix:   "trimsuffix",
		ConvIndex:        "index",
		ConvDate:         "date",
		ConvTruncate:     "truncate",
		ConvBase64Encode: "base64encode",
		ConvBase64Decode: "base64decode",
		ConvEncoder:      "encoder",
		ConvDecoder:      "decoder",
		ConvISO8859_1:    "ISO-8859-1",
		ConvWindows1252:  "Windows-1252",
		ConvMacintosh:    "Macintosh",
		ConvCharmap:      "charmap",
		ConvLowercase:    "lowercase",
		ConvUppercase:    "uppercase",
		ConvTitlecase:    "titlecase",
		ConvASCII:        "ascii",
		ConvURL:          "url",
		ConvMapper:       "mapper",
	}
}

func (convertChoices) Type() string {
	return "string"
}

type cmapChoices struct{}

func (cmapChoices) Choices() []string {
	choices := make([]string, 1)
	i := 0
	for _, enc := range charmap.All {
		c, ok := enc.(*charmap.Charmap)
		if !ok {
			continue
		}
		name := strings.ReplaceAll(c.String(), " ", "-")
		if name == "" {
			name = fmt.Sprintf("unknown-%d", i)
		}
		Cmaps[i] = c
		choices = append(choices, name)
		i++
	}
	return choices
}

func (cmapChoices) Type() string {
	return "string"
}

func charmapByID(cm fs.Enum[cmapChoices]) *charmap.Charmap {
	c, ok := Cmaps[int(cm)]
	if ok {
		return c
	}
	return nil
}

var commandDefinition = &cobra.Command{
	Use:   "convmv source:path",
	Short: `Convert file and directory names`,
	// Warning! "|" will be replaced by backticks below
	Long: strings.ReplaceAll(`
This command renames files and directory names according a user supplied conversion.

It is useful for renaming a lot of files in an automated way.

`+sprintList()+`

`, "|", "`"),
	Annotations: map[string]string{
		"versionIntroduced": "v1.70",
		"groups":            "Filter,Listing,Important,Copy",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc, srcFileName := cmd.NewFsFile(args[0])
		cmd.Run(false, true, command, func() error { // retries switched off to prevent double-encoding
			return Convmv(context.Background(), fsrc, srcFileName)
		})
	},
}

// Convmv converts and renames files and directories
// pass srcFileName == "" to convmv every object in fsrc instead of a single object
func Convmv(ctx context.Context, f fs.Fs, srcFileName string) error {
	Opt.ctx = ctx
	Opt.f = f
	if Opt.List {
		printList()
		return nil
	}
	err := Opt.validate()
	if err != nil {
		return err
	}

	if srcFileName == "" {
		// it's a dir
		return walkConv(ctx, f, "")
	}
	// it's a file
	obj, err := f.NewObject(Opt.ctx, srcFileName)
	if err != nil {
		return err
	}
	oldName, newName, skip, err := parseEntry(obj)
	if err != nil {
		return err
	}
	if skip {
		return nil
	}
	return operations.MoveFile(Opt.ctx, Opt.f, Opt.f, newName, oldName)
}

func (opt *ConvOpt) validate() error {
	switch opt.ConvertAlgo {
	case ConvNone:
		return errors.New("must choose a conversion mode with -t flag")
	case ConvFindReplace:
		if len(opt.FindReplace) == 0 {
			return errors.New("must include --replace flag in replace mode")
		}
		for _, set := range opt.FindReplace {
			split := strings.Split(set, ",")
			if len(split) != 2 {
				return errors.New("--replace must include exactly two comma-separated values")
			}
			if split[0] == "" {
				return errors.New("'find' value cannot be blank ('replace' can be)")
			}
		}
	case ConvPrefix, ConvTrimPrefix:
		if opt.Prefix == "" {
			return errors.New("must include a --prefix")
		}
	case ConvSuffix, ConvTrimSuffix:
		if opt.Suffix == "" {
			return errors.New("must include a --suffix")
		}
	case ConvTruncate:
		if opt.Max < 1 {
			return errors.New("--max cannot be less than 1 in 'truncate' mode")
		}
	case ConvCharmap:
		if opt.CmapFlag == 0 {
			return errors.New("must specify a charmap with --charmap flag")
		}
		c := charmapByID(opt.CmapFlag)
		if c == nil {
			return errors.New("unknown charmap")
		}
		opt.Cmap = c
	}

	return nil
}

// keeps track of which dirs we've already renamed
func walkConv(ctx context.Context, f fs.Fs, dir string) error {
	entries, err := list.DirSorted(ctx, f, false, dir)
	if err != nil {
		return err
	}
	return walkFunc(dir, entries, nil)
}

func walkFunc(path string, entries fs.DirEntries, err error) error {
	fs.Debugf(path, "walking dir")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		switch x := entry.(type) {
		case fs.Object:
			oldName, newName, skip, err := parseEntry(x)
			if err != nil {
				return err
			}
			if skip {
				continue
			}
			fs.Debugf(x, "%v %v %v %v %v", Opt.ctx, Opt.f, Opt.f, newName, oldName)
			err = operations.MoveFile(Opt.ctx, Opt.f, Opt.f, newName, oldName)
			if err != nil {
				return err
			}
		case fs.Directory:
			oldName, newName, skip, err := parseEntry(x)
			if err != nil {
				return err
			}
			if !skip { // still want to recurse during dry-runs to get accurate logs
				err = DirMoveCaseInsensitive(Opt.ctx, Opt.f, oldName, newName)
				if err != nil {
					return err
				}
			} else {
				newName = oldName // otherwise dry-runs won't be able to find it
			}
			// recurse, calling it by its new name
			err = walkConv(Opt.ctx, Opt.f, newName)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// ConvertPath converts a path string according to the chosen ConvertAlgo.
// Each path segment is converted separately, to preserve path separators.
// If baseOnly is true, only the base will be converted (useful for renaming while walking a dir tree recursively.)
// for example, "some/nested/path" -> "some/nested/CONVERTEDPATH"
// otherwise, the entire is path is converted.
func ConvertPath(s string, ConvertAlgo Convert, baseOnly bool) (string, error) {
	if s == "" || s == "/" || s == "\\" || s == "." {
		return "", nil
	}

	if baseOnly {
		convertedBase, err := ConvertPathSegment(filepath.Base(s), ConvertAlgo)
		return filepath.Join(filepath.Dir(s), convertedBase), err
	}

	segments := strings.Split(s, string(os.PathSeparator))
	convertedSegments := make([]string, len(segments))
	for _, seg := range segments {
		convSeg, err := ConvertPathSegment(seg, ConvertAlgo)
		if err != nil {
			return "", err
		}
		convertedSegments = append(convertedSegments, convSeg)
	}
	return filepath.Join(convertedSegments...), nil
}

// ConvertPathSegment converts one path segment (or really any string) according to the chosen ConvertAlgo.
// It assumes path separators have already been trimmed.
func ConvertPathSegment(s string, ConvertAlgo Convert) (string, error) {
	fs.Debugf(s, "converting")
	switch ConvertAlgo {
	case ConvNone:
		return s, nil
	case ConvToNFC:
		return norm.NFC.String(s), nil
	case ConvToNFD:
		return norm.NFD.String(s), nil
	case ConvToNFKC:
		return norm.NFKC.String(s), nil
	case ConvToNFKD:
		return norm.NFKD.String(s), nil
	case ConvBase64Encode:
		return base64.URLEncoding.EncodeToString([]byte(s)), nil // URLEncoding to avoid slashes
	case ConvBase64Decode:
		if s == ".DS_Store" {
			return s, nil
		}
		b, err := base64.URLEncoding.DecodeString(s)
		return string(b), err
	case ConvFindReplace:
		oldNews := []string{}
		for _, pair := range Opt.FindReplace {
			split := strings.Split(pair, ",")
			oldNews = append(oldNews, split...)
		}
		replacer := strings.NewReplacer(oldNews...)
		return replacer.Replace(s), nil
	case ConvPrefix:
		return Opt.Prefix + s, nil
	case ConvSuffix:
		return s + Opt.Suffix, nil
	case ConvTrimPrefix:
		return strings.TrimPrefix(s, Opt.Prefix), nil
	case ConvTrimSuffix:
		return strings.TrimSuffix(s, Opt.Suffix), nil
	case ConvTruncate:
		if Opt.Max <= 0 {
			return s, nil
		}
		if utf8.RuneCountInString(s) <= Opt.Max {
			return s, nil
		}
		runes := []rune(s)
		return string(runes[:Opt.Max]), nil
	case ConvEncoder:
		return Opt.Enc.Encode(s), nil
	case ConvDecoder:
		return Opt.Enc.Decode(s), nil
	case ConvISO8859_1:
		return encodeWithReplacement(s, charmap.ISO8859_1), nil
	case ConvWindows1252:
		return encodeWithReplacement(s, charmap.Windows1252), nil
	case ConvMacintosh:
		return encodeWithReplacement(s, charmap.Macintosh), nil
	case ConvCharmap:
		return encodeWithReplacement(s, Opt.Cmap), nil
	case ConvLowercase:
		return strings.ToLower(s), nil
	case ConvUppercase:
		return strings.ToUpper(s), nil
	case ConvTitlecase:
		return strings.ToTitle(s), nil
	case ConvASCII:
		return toASCII(s), nil
	default:
		return "", errors.New("this option is not yet implemented")
	}
}

func parseEntry(e fs.DirEntry) (oldName, newName string, skip bool, err error) {
	oldName = e.Remote()
	newName, err = ConvertPath(oldName, Opt.ConvertAlgo, true)
	if err != nil {
		fs.Errorf(oldName, "error converting: %v", err)
		return oldName, newName, true, err
	}
	if oldName == newName {
		fs.Debugf(oldName, "name is already correct - skipping")
		return oldName, newName, true, nil
	}
	skip = operations.SkipDestructive(Opt.ctx, oldName, "rename to "+newName)
	return oldName, newName, skip, nil
}

// DirMoveCaseInsensitive does DirMove in two steps (to temp name, then real name)
// which is necessary for some case-insensitive backends
func DirMoveCaseInsensitive(ctx context.Context, f fs.Fs, srcRemote, dstRemote string) (err error) {
	tmpDstRemote := dstRemote + "-rclone-move-" + random.String(8)
	err = operations.DirMove(ctx, f, srcRemote, tmpDstRemote)
	if err != nil {
		return err
	}
	return operations.DirMove(ctx, f, tmpDstRemote, dstRemote)
}

func encodeWithReplacement(s string, cmap *charmap.Charmap) string {
	return strings.Map(func(r rune) rune {
		b, ok := cmap.EncodeRune(r)
		if !ok {
			return '_'
		}
		return cmap.DecodeByte(b)
	}, s)
}

func toASCII(s string) string {
	return strings.Map(func(r rune) rune {
		if r <= 127 {
			return r
		}
		return -1
	}, s)
}

func sprintList() string {
	var out strings.Builder

	_, _ = out.WriteString(`### Conversion modes

The conversion mode |-t| or |--conv| flag must be specified. This
defines what transformation the |convmv| command will make.

`)
	for _, v := range Opt.ConvertAlgo.Choices() {
		_, _ = fmt.Fprintf(&out, "- `%s`\n", v)
	}
	_, _ = out.WriteRune('\n')

	_, _ = out.WriteString(`### Char maps

These are the choices for the |--charmap| flag.

`)
	for _, v := range Opt.CmapFlag.Choices() {
		_, _ = fmt.Fprintf(&out, "- `%s`\n", v)
	}
	_, _ = out.WriteRune('\n')

	_, _ = out.WriteString(`### Encoding masks

These are the valid options for the --encoding flag.

`)
	for _, v := range strings.Split(encoder.ValidStrings(), ", ") {
		_, _ = fmt.Fprintf(&out, "- `%s`\n", v)
	}
	_, _ = out.WriteRune('\n')

	sprintExamples(&out)

	return out.String()
}

func printList() {
	fmt.Println(sprintList())
}
