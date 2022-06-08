// Package version provides machinery for versioning file names
// with a timestamp-based version string
package version

import (
	"path"
	"regexp"
	"strings"
	"time"
)

const versionFormat = "-v2006-01-02-150405.000"

var versionRegexp = regexp.MustCompile(`-v\d{4}-\d{2}-\d{2}-\d{6}-\d{3}`)

// Split fileName into base and extension so that base + ext == fileName
func splitExt(fileName string) (base, ext string) {
	ext = path.Ext(fileName)
	base = fileName[:len(fileName)-len(ext)]
	// .file splits to base == "", ext == ".file"
	// so swap ext and base in this case
	if ext != "" && base == "" {
		base, ext = ext, base
	}
	return base, ext
}

// Add returns fileName modified to include t as the version
func Add(fileName string, t time.Time) string {
	base, ext := splitExt(fileName)
	s := t.Format(versionFormat)
	// Replace the '.' with a '-'
	s = strings.ReplaceAll(s, ".", "-")
	return base + s + ext
}

// Remove returns a modified fileName without the version string and the time it represented
// If the fileName did not have a version then time.Time{} is returned along with an unmodified fileName
func Remove(fileName string) (t time.Time, fileNameWithoutVersion string) {
	fileNameWithoutVersion = fileName
	base, ext := splitExt(fileName)
	if len(base) < len(versionFormat) {
		return
	}
	versionStart := len(base) - len(versionFormat)
	// Check it ends in -xxx
	if base[len(base)-4] != '-' {
		return
	}
	// Replace with .xxx for parsing
	base = base[:len(base)-4] + "." + base[len(base)-3:]
	newT, err := time.Parse(versionFormat, base[versionStart:])
	if err != nil {
		return
	}
	return newT, base[:versionStart] + ext
}

// Match returns true if the fileName has a version string
func Match(fileName string) bool {
	return versionRegexp.MatchString(fileName)
}
