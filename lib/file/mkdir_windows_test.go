//go:build windows && !go1.22

package file

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Basic test from golang's os/path_test.go
func TestMkdirAll(t *testing.T) {
	tmpDir := t.TempDir()

	path := tmpDir + "/dir/./dir2"
	err := MkdirAll(path, 0777)
	if err != nil {
		t.Fatalf("MkdirAll %q: %s", path, err)
	}

	// Already exists, should succeed.
	err = MkdirAll(path, 0777)
	if err != nil {
		t.Fatalf("MkdirAll %q (second time): %s", path, err)
	}

	// Make file.
	fpath := path + "/file"
	f, err := Create(fpath)
	if err != nil {
		t.Fatalf("create %q: %s", fpath, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Fatalf("Close %q: %s", fpath, err)
		}
	}()

	// Can't make directory named after file.
	err = MkdirAll(fpath, 0777)
	if err == nil {
		t.Fatalf("MkdirAll %q: no error", fpath)
	}
	perr, ok := err.(*os.PathError)
	if !ok {
		t.Fatalf("MkdirAll %q returned %T, not *PathError", fpath, err)
	}
	if filepath.Clean(perr.Path) != filepath.Clean(fpath) {
		t.Fatalf("MkdirAll %q returned wrong error path: %q not %q", fpath, filepath.Clean(perr.Path), filepath.Clean(fpath))
	}

	// Can't make subdirectory of file.
	ffpath := fpath + "/subdir"
	err = MkdirAll(ffpath, 0777)
	if err == nil {
		t.Fatalf("MkdirAll %q: no error", ffpath)
	}
	perr, ok = err.(*os.PathError)
	if !ok {
		t.Fatalf("MkdirAll %q returned %T, not *PathError", ffpath, err)
	}
	if filepath.Clean(perr.Path) != filepath.Clean(fpath) {
		t.Fatalf("MkdirAll %q returned wrong error path: %q not %q", ffpath, filepath.Clean(perr.Path), filepath.Clean(fpath))
	}

	path = tmpDir + `\dir\.\dir2\`
	err = MkdirAll(path, 0777)
	if err != nil {
		t.Fatalf("MkdirAll %q: %s", path, err)
	}
}

func unusedDrive(t *testing.T) string {
	letter := FindUnusedDriveLetter()
	require.NotEqual(t, letter, 0)
	return string(letter) + ":"
}

func checkMkdirAll(t *testing.T, path string, valid bool, errormsgs ...string) {
	if valid {
		assert.NoError(t, MkdirAll(path, 0777))
	} else {
		err := MkdirAll(path, 0777)
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
	// checkMkdirAll(t, `\\?\`+drive, true, "") - this isn't actually a Valid Windows path - this test used to work under go1.21.3 but fails under go1.21.4
	checkMkdirAll(t, `\\?\`+drive+`\`, true, "")
	checkMkdirAllSubdirs(t, path, true, "")
	checkMkdirAllSubdirs(t, `\\?\`+path, true, "")
}

// Testing paths on unused drive
// This is where there is a difference from golang's os.MkdirAll. It would
// recurse extended-length paths down to the "\\?" prefix and return the
// noninformative error:
// "mkdir \\?: The filename, directory name, or volume label syntax is incorrect."
// Our version stops the recursion at drive's root directory, and reports:
// "mkdir \\?\A:\: The system cannot find the path specified."
func TestMkdirAllOnUnusedDrive(t *testing.T) {
	path := unusedDrive(t)
	errormsg := fmt.Sprintf(`mkdir %s\: The system cannot find the path specified.`, path)
	checkMkdirAllSubdirs(t, path, false, errormsg)
	errormsg1 := fmt.Sprintf(`mkdir \\?\%s\: The system cannot find the path specified.`, path) // pre go1.21.4
	errormsg2 := fmt.Sprintf(`mkdir \\?\%s: The system cannot find the file specified.`, path)  // go1.21.4 and after
	checkMkdirAllSubdirs(t, `\\?\`+path, false, errormsg1, errormsg2)
}

// Testing paths on unknown network host
// This is an additional difference from golang's os.MkdirAll. With our
// first fix, stopping it from recursing extended-length paths down to
// the "\\?" prefix, it would now stop at `\\?\UNC`, because that is what
// filepath.VolumeName returns (which is wrong, that is not a volume name!),
// and still return a nonifnromative error:
// "mkdir \\?\UNC\\: The filename, directory name, or volume label syntax is incorrect."
// Our version stops the recursion at level before this, and reports:
// "mkdir \\?\UNC\0.0.0.0: The specified path is invalid."
func TestMkdirAllOnUnusedNetworkHost(t *testing.T) {
	path := `\\0.0.0.0\share`
	errormsg := fmt.Sprintf("mkdir %s\\: The format of the specified network name is invalid.", path)
	checkMkdirAllSubdirs(t, path, false, errormsg)
	path = `\\?\UNC\0.0.0.0\share`
	checkMkdirAllSubdirs(t, path, false,
		`mkdir \\?\UNC\0.0.0.0: The specified path is invalid.`, // pre go1.20
		`mkdir \\?\UNC\0.0.0.0\share\: The format of the specified network name is invalid.`,
	)

}
