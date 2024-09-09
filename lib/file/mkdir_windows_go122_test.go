//go:build windows && go1.22

package file

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func checkMkdirAll(t *testing.T, path string, valid bool, errormsgs ...string) {
	if valid {
		assert.NoError(t, os.MkdirAll(path, 0777))
	} else {
		err := os.MkdirAll(path, 0777)
		assert.Error(t, err)
		ok := false
		for _, msg := range errormsgs {
			if err.Error() == msg {
				ok = true
			}
		}
		assert.True(t, ok, fmt.Sprintf("Error message '%v' didn't match any of %v", err, errormsgs))
	}
}

func checkMkdirAllSubdirs(t *testing.T, path string, valid bool, errormsgs ...string) {
	checkMkdirAll(t, path, valid, errormsgs...)
	checkMkdirAll(t, path+`\`, valid, errormsgs...)
	checkMkdirAll(t, path+`\parent`, valid, errormsgs...)
	checkMkdirAll(t, path+`\parent\`, valid, errormsgs...)
	checkMkdirAll(t, path+`\parent\child`, valid, errormsgs...)
	checkMkdirAll(t, path+`\parent\child\`, valid, errormsgs...)
}

// Testing paths on existing drive
func TestMkdirAllOnDrive(t *testing.T) {
	path := t.TempDir()

	dir, err := os.Stat(path)
	require.NoError(t, err)
	require.True(t, dir.IsDir())

	drive := filepath.VolumeName(path)

	checkMkdirAll(t, drive, true, "")
	checkMkdirAll(t, drive+`\`, true, "")
	checkMkdirAll(t, `\\?\`+drive, false, fmt.Sprintf(`mkdir \\?\%s: Access is denied.`, drive)) // This isn't actually a valid Windows path, it worked under go1.21.3 but fails under go1.21.4 and newer
	checkMkdirAll(t, `\\?\`+drive+`\`, true, "")
	checkMkdirAllSubdirs(t, path, true, "")
	checkMkdirAllSubdirs(t, `\\?\`+path, true, "")
}

// Testing paths on unused drive
// This covers the cases that we wanted to improve with our own custom version
// of golang's os.MkdirAll, introduced in PR #5401. Before go1.22 the original
// os.MkdirAll would recurse extended-length paths down to the "\\?" prefix and
// return the noninformative error:
// "mkdir \\?: The filename, directory name, or volume label syntax is incorrect."
// Our version stopped the recursion at drive's root directory, and reported,
// before go1.21.4:
// "mkdir \\?\A:\: The system cannot find the path specified."
// or, starting with go1.21.4:
// "mkdir \\?\A:: The system cannot find the path specified."
// See https://github.com/rclone/rclone/pull/5401.
// Starting with go1.22 golang's os.MkdirAll have similar improvements that made
// our custom version no longer necessary.
func TestMkdirAllOnUnusedDrive(t *testing.T) {
	letter := FindUnusedDriveLetter()
	require.NotEqual(t, letter, 0)
	drive := string(letter) + ":"
	checkMkdirAll(t, drive, false, fmt.Sprintf(`mkdir %s: The system cannot find the path specified.`, drive))
	checkMkdirAll(t, drive+`\`, false, fmt.Sprintf(`mkdir %s\: The system cannot find the path specified.`, drive))
	checkMkdirAll(t, drive+`\parent`, false, fmt.Sprintf(`mkdir %s\parent: The system cannot find the path specified.`, drive))
	checkMkdirAll(t, drive+`\parent\`, false, fmt.Sprintf(`mkdir %s\parent\: The system cannot find the path specified.`, drive))
	checkMkdirAll(t, drive+`\parent\child`, false, fmt.Sprintf(`mkdir %s\parent: The system cannot find the path specified.`, drive))
	checkMkdirAll(t, drive+`\parent\child\`, false, fmt.Sprintf(`mkdir %s\parent: The system cannot find the path specified.`, drive))

	drive = `\\?\` + drive
	checkMkdirAll(t, drive, false, fmt.Sprintf(`mkdir %s: The system cannot find the file specified.`, drive))
	checkMkdirAll(t, drive+`\`, false, fmt.Sprintf(`mkdir %s\: The system cannot find the path specified.`, drive))
	checkMkdirAll(t, drive+`\parent`, false, fmt.Sprintf(`mkdir %s\parent: The system cannot find the path specified.`, drive))
	checkMkdirAll(t, drive+`\parent\`, false, fmt.Sprintf(`mkdir %s\parent\: The system cannot find the path specified.`, drive))
	checkMkdirAll(t, drive+`\parent\child`, false, fmt.Sprintf(`mkdir %s\parent: The system cannot find the path specified.`, drive))
	checkMkdirAll(t, drive+`\parent\child\`, false, fmt.Sprintf(`mkdir %s\parent: The system cannot find the path specified.`, drive))
}

// Testing paths on unknown network host
// This covers more cases that we wanted to improve in our custom version of
// golang's os.MkdirAll, extending that explained in TestMkdirAllOnUnusedDrive.
// With our first fix, stopping it from recursing extended-length paths down to
// the "\\?" prefix, it would now stop at `\\?\UNC`, because that is what
// filepath.VolumeName returns (which is wrong, that is not a volume name!),
// and still return a noninformative error:
// "mkdir \\?\UNC\\: The filename, directory name, or volume label syntax is incorrect."
// Our version stopped the recursion at level before this, and reports:
// "mkdir \\?\UNC\0.0.0.0: The specified path is invalid."
// See https://github.com/rclone/rclone/pull/6420.
// Starting with go1.22 golang's os.MkdirAll have similar improvements that made
// our custom version no longer necessary.
func TestMkdirAllOnUnusedNetworkHost(t *testing.T) {
	sharePath := `\\0.0.0.0\share`
	checkMkdirAll(t, sharePath, false, fmt.Sprintf(`mkdir %s: The format of the specified network name is invalid.`, sharePath))
	checkMkdirAll(t, sharePath+`\`, false, fmt.Sprintf(`mkdir %s\: The format of the specified network name is invalid.`, sharePath))
	checkMkdirAll(t, sharePath+`\parent`, false, fmt.Sprintf(`mkdir %s\parent: The format of the specified network name is invalid.`, sharePath))
	checkMkdirAll(t, sharePath+`\parent\`, false, fmt.Sprintf(`mkdir %s\parent\: The format of the specified network name is invalid.`, sharePath))
	checkMkdirAll(t, sharePath+`\parent\child`, false, fmt.Sprintf(`mkdir %s\parent: The format of the specified network name is invalid.`, sharePath))
	checkMkdirAll(t, sharePath+`\parent\child\`, false, fmt.Sprintf(`mkdir %s\parent: The format of the specified network name is invalid.`, sharePath))

	serverPath := `\\?\UNC\0.0.0.0`
	sharePath = serverPath + `\share`
	checkMkdirAll(t, sharePath, false, fmt.Sprintf(`mkdir %s: The specified path is invalid.`, serverPath))
	checkMkdirAll(t, sharePath+`\`, false, fmt.Sprintf(`mkdir %s: The specified path is invalid.`, serverPath))
	checkMkdirAll(t, sharePath+`\parent`, false, fmt.Sprintf(`mkdir %s: The specified path is invalid.`, serverPath))
	checkMkdirAll(t, sharePath+`\parent\`, false, fmt.Sprintf(`mkdir %s: The specified path is invalid.`, serverPath))
	checkMkdirAll(t, sharePath+`\parent\child`, false, fmt.Sprintf(`mkdir %s: The specified path is invalid.`, serverPath))
	checkMkdirAll(t, sharePath+`\parent\child\`, false, fmt.Sprintf(`mkdir %s: The specified path is invalid.`, serverPath))
}
