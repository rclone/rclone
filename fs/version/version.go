package version

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
)

// Version represents a parsed rclone version number
type Version []int

var parseVersion = regexp.MustCompile(`^(?:rclone )?v(\d+)\.(\d+)(?:\.(\d+))?(?:-(\d+)(?:-(g[\wβ-]+))?)?$`)

// New parses a version number from a string
//
// This will be returned with up to 4 elements for major, minor,
// patch, subpatch release.
//
// If the version number represents a compiled from git version
// number, then it will be returned as major, minor, 999, 999
func New(in string) (v Version, err error) {
	isGit := strings.HasSuffix(in, "-DEV")
	if isGit {
		in = in[:len(in)-4]
	}
	r := parseVersion.FindStringSubmatch(in)
	if r == nil {
		return v, errors.Errorf("failed to match version string %q", in)
	}
	atoi := func(s string) int {
		i, err := strconv.Atoi(s)
		if err != nil {
			fs.Errorf(nil, "Failed to parse %q as int from %q: %v", s, in, err)
		}
		return i
	}
	v = Version{
		atoi(r[1]), // major
		atoi(r[2]), // minor
	}
	if r[3] != "" {
		v = append(v, atoi(r[3])) // patch
	} else if r[4] != "" {
		v = append(v, 0) // patch
	}
	if r[4] != "" {
		v = append(v, atoi(r[4])) // dev
	}
	if isGit {
		v = append(v, 999, 999)
	}
	return v, nil
}

// String converts v to a string
func (v Version) String() string {
	var out []string
	for _, vv := range v {
		out = append(out, fmt.Sprint(vv))
	}
	return strings.Join(out, ".")
}

// Cmp compares two versions returning >0, <0 or 0
func (v Version) Cmp(o Version) (d int) {
	n := len(v)
	if n > len(o) {
		n = len(o)
	}
	for i := 0; i < n; i++ {
		d = v[i] - o[i]
		if d != 0 {
			return d
		}
	}
	return len(v) - len(o)
}

// IsGit returns true if the current version was compiled from git
func (v Version) IsGit() bool {
	return len(v) >= 4 && v[2] == 999 && v[3] == 999
}
