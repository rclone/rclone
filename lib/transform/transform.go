// Package transform holds functions for path name transformations
package transform

import (
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"os"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/encoder"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/unicode/norm"
)

// Path transforms a path s according to the --name-transform options in use
//
// If no transforms are in use, s is returned unchanged
func Path(s string, isDir bool) string {
	if !Transforming() {
		return s
	}

	var err error
	old := s
	for _, t := range Opt.transforms {
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
			fs.Error(s, err.Error()) // TODO: return err instead of logging it?
		}
	}
	if old != s {
		fs.Debugf(old, "transformed to: %v", s)
	}
	return s
}

// Transforming returns true when transforms are in use
func Transforming() bool {
	return len(Opt.transforms) > 0
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

	segments := strings.Split(s, string(os.PathSeparator))
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
		if max <= 0 {
			return s, nil
		}
		if utf8.RuneCountInString(s) <= max {
			return s, nil
		}
		runes := []rune(s)
		return string(runes[:max]), nil
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
		var cmapType fs.Enum[cmapChoices]
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
	default:
		return "", errors.New("this option is not yet implemented")
	}
}

// SuffixKeepExtension adds a suffix while keeping extension
//
// i.e. file.txt becomes file_somesuffix.txt not file.txt_somesuffix
func SuffixKeepExtension(remote string, suffix string) string {
	var (
		base  = remote
		exts  = ""
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
	return base + suffix + exts
}

// forbid transformations that add/remove path separators
func validateSegment(s string) error {
	if s == "" {
		return errors.New("transform cannot render path segments empty")
	}
	if strings.ContainsRune(s, '/') {
		return fmt.Errorf("transform cannot add path separators: %v", s)
	}
	return nil
}
