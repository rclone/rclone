// Package transform holds functions for path name transformations
//
//go:generate go run gen_help.go transform.md
package transform

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"net/url"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/encoder"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/unicode/norm"
)

//go:embed transform.md
var help string

// Help returns the help string cleaned up to simplify appending
func Help() string {
	// Chop off auto generated message
	nl := strings.IndexRune(help, '\n')
	return strings.TrimSpace(help[nl:]) + "\n\n"
}

// Path transforms a path s according to the --name-transform options in use
//
// If no transforms are in use, s is returned unchanged
func Path(ctx context.Context, s string, isDir bool) string {
	if !Transforming(ctx) {
		return s
	}

	old := s
	opt, err := getOptions(ctx)
	if err != nil {
		err = fs.CountError(ctx, err)
		fs.Errorf(s, "Failed to parse transform flags: %v", err)
	}
	for _, t := range opt {
		if isDir && t.tag == file {
			continue
		}
		baseOnly := !isDir && t.tag == file
		if t.tag == dir && !isDir {
			s, err = transformDir(s, t)
		} else {
			s, err = transformPath(s, t, baseOnly)
		}
		if err != nil {
			err = fs.CountError(ctx, fserrors.NoRetryError(err))
			fs.Errorf(s, "Failed to transform: %v", err)
		}
	}
	if old != s {
		fs.Debugf(old, "transformed to: %v", s)
	}
	if strings.Count(old, "/") != strings.Count(s, "/") {
		err = fs.CountError(ctx, fserrors.NoRetryError(fmt.Errorf("number of path segments must match: %v (%v), %v (%v)", old, strings.Count(old, "/"), s, strings.Count(s, "/"))))
		fs.Errorf(old, "%v", err)
		return old
	}
	return s
}

// transformPath transforms a path string according to the chosen TransformAlgo.
// Each path segment is transformed separately, to preserve path separators.
// If baseOnly is true, only the base will be transformed (useful for renaming while walking a dir tree recursively.)
// for example, "some/nested/path" -> "some/nested/CONVERTEDPATH"
// otherwise, the entire is path is transformed.
func transformPath(s string, t transform, baseOnly bool) (string, error) {
	if s == "" || s == "/" || s == "\\" || s == "." {
		return "", nil
	}

	if baseOnly {
		transformedBase, err := transformPathSegment(path.Base(s), t)
		if err := validateSegment(transformedBase); err != nil {
			return "", err
		}
		return path.Join(path.Dir(s), transformedBase), err
	}

	segments := strings.Split(s, "/")
	transformedSegments := make([]string, len(segments))
	for _, seg := range segments {
		convSeg, err := transformPathSegment(seg, t)
		if err != nil {
			return "", err
		}
		if err := validateSegment(convSeg); err != nil {
			return "", err
		}
		transformedSegments = append(transformedSegments, convSeg)
	}
	return path.Join(transformedSegments...), nil
}

// transform all but the last path segment
func transformDir(s string, t transform) (string, error) {
	dirPath, err := transformPath(path.Dir(s), t, false)
	if err != nil {
		return "", err
	}
	return path.Join(dirPath, path.Base(s)), nil
}

// transformPathSegment transforms one path segment (or really any string) according to the chosen TransformAlgo.
// It assumes path separators have already been trimmed.
func transformPathSegment(s string, t transform) (string, error) {
	switch t.key {
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
		if err != nil {
			fs.Errorf(s, "base64 error")
		}
		return string(b), err
	case ConvFindReplace:
		split := strings.Split(t.value, ":")
		if len(split) != 2 {
			return s, fmt.Errorf("wrong number of values: %v", t.value)
		}
		return strings.ReplaceAll(s, split[0], split[1]), nil
	case ConvPrefix:
		return t.value + s, nil
	case ConvSuffix:
		return s + t.value, nil
	case ConvSuffixKeepExtension:
		return SuffixKeepExtension(s, t.value), nil
	case ConvTrimPrefix:
		return strings.TrimPrefix(s, t.value), nil
	case ConvTrimSuffix:
		return strings.TrimSuffix(s, t.value), nil
	case ConvTruncate:
		max, err := strconv.Atoi(t.value)
		if err != nil {
			return s, err
		}
		return truncateChars(s, max, false), nil
	case ConvTruncateKeepExtension:
		max, err := strconv.Atoi(t.value)
		if err != nil {
			return s, err
		}
		return truncateChars(s, max, true), nil
	case ConvTruncateBytes:
		max, err := strconv.Atoi(t.value)
		if err != nil {
			return s, err
		}
		return truncateBytes(s, max, false)
	case ConvTruncateBytesKeepExtension:
		max, err := strconv.Atoi(t.value)
		if err != nil {
			return s, err
		}
		return truncateBytes(s, max, true)
	case ConvEncoder:
		var enc encoder.MultiEncoder
		err := enc.Set(t.value)
		if err != nil {
			return s, err
		}
		return enc.Encode(s), nil
	case ConvDecoder:
		var enc encoder.MultiEncoder
		err := enc.Set(t.value)
		if err != nil {
			return s, err
		}
		return enc.Decode(s), nil
	case ConvISO8859_1:
		return encodeWithReplacement(s, charmap.ISO8859_1), nil
	case ConvWindows1252:
		return encodeWithReplacement(s, charmap.Windows1252), nil
	case ConvMacintosh:
		return encodeWithReplacement(s, charmap.Macintosh), nil
	case ConvCharmap:
		var cmapType CharmapChoices
		err := cmapType.Set(t.value)
		if err != nil {
			return s, err
		}
		c := charmapByID(cmapType)
		return encodeWithReplacement(s, c), nil
	case ConvLowercase:
		return strings.ToLower(s), nil
	case ConvUppercase:
		return strings.ToUpper(s), nil
	case ConvTitlecase:
		return strings.ToTitle(s), nil
	case ConvASCII:
		return toASCII(s), nil
	case ConvURL:
		return url.QueryEscape(s), nil
	case ConvDate:
		return s + AppyTimeGlobs(t.value, time.Now()), nil
	case ConvRegex:
		split := strings.Split(t.value, "/")
		if len(split) != 2 {
			return s, fmt.Errorf("regex syntax error: %v", t.value)
		}
		re := regexp.MustCompile(split[0])
		return re.ReplaceAllString(s, split[1]), nil
	case ConvCommand:
		return mapper(s, t.value)
	default:
		return "", errors.New("this option is not yet implemented")
	}
}

// SuffixKeepExtension adds a suffix while keeping extension
//
// i.e. file.txt becomes file_somesuffix.txt not file.txt_somesuffix
func SuffixKeepExtension(remote string, suffix string) string {
	base, exts := splitExtension(remote)
	return base + suffix + exts
}

func splitExtension(remote string) (base, exts string) {
	base = remote
	var (
		first = true
		ext   = path.Ext(remote)
	)
	for ext != "" {
		// Look second and subsequent extensions in mime types.
		// If they aren't found then don't keep it as an extension.
		if !first && mime.TypeByExtension(ext) == "" {
			break
		}
		base = base[:len(base)-len(ext)]
		exts = ext + exts
		first = false
		ext = path.Ext(base)
	}
	return base, exts
}

func truncateChars(s string, max int, keepExtension bool) string {
	if max <= 0 {
		return s
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	exts := ""
	if keepExtension {
		s, exts = splitExtension(s)
	}
	runes := []rune(s)
	return string(runes[:max-utf8.RuneCountInString(exts)]) + exts
}

// truncateBytes is like truncateChars but counts the number of bytes, not UTF-8 characters
func truncateBytes(s string, max int, keepExtension bool) (string, error) {
	if max <= 0 {
		return s, nil
	}
	if len(s) <= max {
		return s, nil
	}
	exts := ""
	if keepExtension {
		s, exts = splitExtension(s)
	}

	// ensure we don't split a multi-byte UTF-8 character
	for i := max - len(exts); i > 0; i-- {
		b := append([]byte(s)[:i], exts...)
		if len(b) <= max && utf8.Valid(b) {
			return string(b), nil
		}
	}
	return "", errors.New("could not truncate to valid UTF-8")
}

// forbid transformations that add/remove path separators
func validateSegment(s string) error {
	if strings.TrimSpace(s) == "" {
		return errors.New("transform cannot render path segments empty")
	}
	if strings.ContainsRune(s, '/') {
		return fmt.Errorf("transform cannot add path separators: %v", s)
	}
	return nil
}

// ParseGlobs determines whether a string contains {brackets}
// and returns the substring (including both brackets) for replacing
// substring is first opening bracket to last closing bracket --
// good for {{this}} but not {this}{this}
func ParseGlobs(s string) (hasGlobs bool, substring string) {
	open := strings.Index(s, "{")
	close := strings.LastIndex(s, "}")
	if open >= 0 && close > open {
		return true, s[open : close+1]
	}
	return false, ""
}

// TrimBrackets converts {{this}} to this
func TrimBrackets(s string) string {
	return strings.Trim(s, "{}")
}

// TimeFormat converts a user-supplied string to a Go time constant, if possible
func TimeFormat(timeFormat string) string {
	switch timeFormat {
	case "Layout":
		timeFormat = time.Layout
	case "ANSIC":
		timeFormat = time.ANSIC
	case "UnixDate":
		timeFormat = time.UnixDate
	case "RubyDate":
		timeFormat = time.RubyDate
	case "RFC822":
		timeFormat = time.RFC822
	case "RFC822Z":
		timeFormat = time.RFC822Z
	case "RFC850":
		timeFormat = time.RFC850
	case "RFC1123":
		timeFormat = time.RFC1123
	case "RFC1123Z":
		timeFormat = time.RFC1123Z
	case "RFC3339":
		timeFormat = time.RFC3339
	case "RFC3339Nano":
		timeFormat = time.RFC3339Nano
	case "Kitchen":
		timeFormat = time.Kitchen
	case "Stamp":
		timeFormat = time.Stamp
	case "StampMilli":
		timeFormat = time.StampMilli
	case "StampMicro":
		timeFormat = time.StampMicro
	case "StampNano":
		timeFormat = time.StampNano
	case "DateTime":
		timeFormat = time.DateTime
	case "DateOnly":
		timeFormat = time.DateOnly
	case "TimeOnly":
		timeFormat = time.TimeOnly
	case "MacFriendlyTime", "macfriendlytime", "mac":
		timeFormat = "2006-01-02 0304PM" // not actually a Go constant -- but useful as macOS filenames can't have colons
	case "YYYYMMDD":
		timeFormat = "20060102"
	}
	return timeFormat
}

// AppyTimeGlobs converts "myfile-{DateOnly}.txt" to "myfile-2006-01-02.txt"
func AppyTimeGlobs(s string, t time.Time) string {
	hasGlobs, substring := ParseGlobs(s)
	if !hasGlobs {
		return s
	}
	timeString := t.Local().Format(TimeFormat(TrimBrackets(substring)))
	return strings.ReplaceAll(s, substring, timeString)
}

func mapper(s string, command string) (string, error) {
	out, err := exec.Command(command, s).CombinedOutput()
	if err != nil {
		out = bytes.TrimSpace(out)
		return s, fmt.Errorf("%s: error running command %q: %v", out, command+" "+s, err)
	}
	return string(bytes.TrimSpace(out)), nil
}
