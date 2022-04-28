//go:build windows
// +build windows

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
	defer f.Close()

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

func checkMkdirAll(t *testing.T, path string, valid bool, errormsg string) {
	if valid {
		assert.NoError(t, MkdirAll(path, 0777))
	} else {
		err := MkdirAll(path, 0777)
		assert.Error(t, err)
		assert.Equal(t, errormsg, err.Error())
	}
}

func checkMkdirAllSubdirs(t *testing.T, path string, valid bool, errormsg string) {
	checkMkdirAll(t, path, valid, errormsg)
	checkMkdirAll(t, path+`\`, valid, errormsg)
	checkMkdirAll(t, path+`\parent`, valid, errormsg)
	checkMkdirAll(t, path+`\parent\`, valid, errormsg)
	checkMkdirAll(t, path+`\parent\child`, valid, errormsg)
	checkMkdirAll(t, path+`\parent\child\`, valid, errormsg)
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
	checkMkdirAll(t, `\\?\`+drive, true, "")
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
	errormsg := fmt.Sprintf("mkdir %s\\: The system cannot find the path specified.", path)
	checkMkdirAllSubdirs(t, path, false, errormsg)
	errormsg = fmt.Sprintf("mkdir \\\\?\\%s\\: The system cannot find the path specified.", path)
	checkMkdirAllSubdirs(t, `\\?\`+path, false, errormsg)
}
