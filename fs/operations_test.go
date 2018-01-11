// Integration tests - test rclone by doing real transactions to a
// storage provider to and from the local disk.
//
// By default it will use a local fs, however you can provide a
// -remote option to use a different remote.  The test_all.go script
// is a wrapper to call this for all the test remotes.
//
// FIXME not safe for concurrent running of tests until fs.Config is
// no longer a global
//
// NB When writing tests
//
// Make sure every series of writes to the remote has a
// fstest.CheckItems() before use.  This make sure the directory
// listing is now consistent and stops cascading errors.
//
// Call fs.Stats.ResetCounters() before every fs.Sync() as it uses the
// error count internally.

package fs_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"regexp"
	"strings"
	"testing"
	"time"

	_ "github.com/ncw/rclone/backend/all" // import all backends
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Some times used in the tests
var (
	t1 = fstest.Time("2001-02-03T04:05:06.499999999Z")
	t2 = fstest.Time("2011-12-25T12:59:59.123456789Z")
	t3 = fstest.Time("2011-12-30T12:59:59.000000000Z")
)

// TestMain drives the tests
func TestMain(m *testing.M) {
	fstest.TestMain(m)
}

func TestMkdir(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	fstest.TestMkdir(t, r.Fremote)
}

func TestLsd(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject("sub dir/hello world", "hello world", t1)

	fstest.CheckItems(t, r.Fremote, file1)

	var buf bytes.Buffer
	err := fs.ListDir(r.Fremote, &buf)
	require.NoError(t, err)
	res := buf.String()
	assert.Contains(t, res, "sub dir\n")
}

func TestLs(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth("potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth("empty space", "", t2)

	fstest.CheckItems(t, r.Fremote, file1, file2)

	var buf bytes.Buffer
	err := fs.List(r.Fremote, &buf)
	require.NoError(t, err)
	res := buf.String()
	assert.Contains(t, res, "        0 empty space\n")
	assert.Contains(t, res, "       60 potato2\n")
}

func TestLsLong(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth("potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth("empty space", "", t2)

	fstest.CheckItems(t, r.Fremote, file1, file2)

	var buf bytes.Buffer
	err := fs.ListLong(r.Fremote, &buf)
	require.NoError(t, err)
	res := buf.String()
	lines := strings.Split(strings.Trim(res, "\n"), "\n")
	assert.Equal(t, 2, len(lines))

	timeFormat := "2006-01-02 15:04:05.000000000"
	precision := r.Fremote.Precision()
	location := time.Now().Location()
	checkTime := func(m, filename string, expected time.Time) {
		modTime, err := time.ParseInLocation(timeFormat, m, location) // parse as localtime
		if err != nil {
			t.Errorf("Error parsing %q: %v", m, err)
		} else {
			dt, ok := fstest.CheckTimeEqualWithPrecision(expected, modTime, precision)
			if !ok {
				t.Errorf("%s: Modification time difference too big |%s| > %s (%s vs %s) (precision %s)", filename, dt, precision, modTime, expected, precision)
			}
		}
	}

	m1 := regexp.MustCompile(`(?m)^        0 (\d{4}-\d\d-\d\d \d\d:\d\d:\d\d\.\d{9}) empty space$`)
	if ms := m1.FindStringSubmatch(res); ms == nil {
		t.Errorf("empty space missing: %q", res)
	} else {
		checkTime(ms[1], "empty space", t2.Local())
	}

	m2 := regexp.MustCompile(`(?m)^       60 (\d{4}-\d\d-\d\d \d\d:\d\d:\d\d\.\d{9}) potato2$`)
	if ms := m2.FindStringSubmatch(res); ms == nil {
		t.Errorf("potato2 missing: %q", res)
	} else {
		checkTime(ms[1], "potato2", t1.Local())
	}
}

func TestHashSums(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth("potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth("empty space", "", t2)

	fstest.CheckItems(t, r.Fremote, file1, file2)

	// MD5 Sum

	var buf bytes.Buffer
	err := fs.Md5sum(r.Fremote, &buf)
	require.NoError(t, err)
	res := buf.String()
	if !strings.Contains(res, "d41d8cd98f00b204e9800998ecf8427e  empty space\n") &&
		!strings.Contains(res, "                     UNSUPPORTED  empty space\n") &&
		!strings.Contains(res, "                                  empty space\n") {
		t.Errorf("empty space missing: %q", res)
	}
	if !strings.Contains(res, "d6548b156ea68a4e003e786df99eee76  potato2\n") &&
		!strings.Contains(res, "                     UNSUPPORTED  potato2\n") &&
		!strings.Contains(res, "                                  potato2\n") {
		t.Errorf("potato2 missing: %q", res)
	}

	// SHA1 Sum

	buf.Reset()
	err = fs.Sha1sum(r.Fremote, &buf)
	require.NoError(t, err)
	res = buf.String()
	if !strings.Contains(res, "da39a3ee5e6b4b0d3255bfef95601890afd80709  empty space\n") &&
		!strings.Contains(res, "                             UNSUPPORTED  empty space\n") &&
		!strings.Contains(res, "                                          empty space\n") {
		t.Errorf("empty space missing: %q", res)
	}
	if !strings.Contains(res, "9dc7f7d3279715991a22853f5981df582b7f9f6d  potato2\n") &&
		!strings.Contains(res, "                             UNSUPPORTED  potato2\n") &&
		!strings.Contains(res, "                                          potato2\n") {
		t.Errorf("potato2 missing: %q", res)
	}

	// Dropbox Hash Sum

	buf.Reset()
	err = fs.DropboxHashSum(r.Fremote, &buf)
	require.NoError(t, err)
	res = buf.String()
	if !strings.Contains(res, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  empty space\n") &&
		!strings.Contains(res, "                                                     UNSUPPORTED  empty space\n") &&
		!strings.Contains(res, "                                                                  empty space\n") {
		t.Errorf("empty space missing: %q", res)
	}
	if !strings.Contains(res, "a979481df794fed9c3990a6a422e0b1044ac802c15fab13af9c687f8bdbee01a  potato2\n") &&
		!strings.Contains(res, "                                                     UNSUPPORTED  potato2\n") &&
		!strings.Contains(res, "                                                                  potato2\n") {
		t.Errorf("potato2 missing: %q", res)
	}
}

func TestCount(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth("potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth("empty space", "", t2)
	file3 := r.WriteBoth("sub dir/potato3", "hello", t2)

	fstest.CheckItems(t, r.Fremote, file1, file2, file3)

	// Check the MaxDepth too
	fs.Config.MaxDepth = 1
	defer func() { fs.Config.MaxDepth = -1 }()

	objects, size, err := fs.Count(r.Fremote)
	require.NoError(t, err)
	assert.Equal(t, int64(2), objects)
	assert.Equal(t, int64(60), size)
}

func TestDelete(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject("small", "1234567890", t2)                                                                                           // 10 bytes
	file2 := r.WriteObject("medium", "------------------------------------------------------------", t1)                                        // 60 bytes
	file3 := r.WriteObject("large", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", t1) // 100 bytes
	fstest.CheckItems(t, r.Fremote, file1, file2, file3)

	fs.Config.Filter.MaxSize = 60
	defer func() {
		fs.Config.Filter.MaxSize = -1
	}()

	err := fs.Delete(r.Fremote)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Fremote, file3)
}

func testCheck(t *testing.T, checkFunction func(fdst, fsrc fs.Fs) error) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	check := func(i int, wantErrors int64) {
		fs.Debugf(r.Fremote, "%d: Starting check test", i)
		oldErrors := fs.Stats.GetErrors()
		err := checkFunction(r.Flocal, r.Fremote)
		gotErrors := fs.Stats.GetErrors() - oldErrors
		if wantErrors == 0 && err != nil {
			t.Errorf("%d: Got error when not expecting one: %v", i, err)
		}
		if wantErrors != 0 && err == nil {
			t.Errorf("%d: No error when expecting one", i)
		}
		if wantErrors != gotErrors {
			t.Errorf("%d: Expecting %d errors but got %d", i, wantErrors, gotErrors)
		}
		fs.Debugf(r.Fremote, "%d: Ending check test", i)
	}

	file1 := r.WriteBoth("rutabaga", "is tasty", t3)
	fstest.CheckItems(t, r.Fremote, file1)
	fstest.CheckItems(t, r.Flocal, file1)
	check(1, 0)

	file2 := r.WriteFile("potato2", "------------------------------------------------------------", t1)
	fstest.CheckItems(t, r.Flocal, file1, file2)
	check(2, 1)

	file3 := r.WriteObject("empty space", "", t2)
	fstest.CheckItems(t, r.Fremote, file1, file3)
	check(3, 2)

	file2r := file2
	if fs.Config.SizeOnly {
		file2r = r.WriteObject("potato2", "--Some-Differences-But-Size-Only-Is-Enabled-----------------", t1)
	} else {
		r.WriteObject("potato2", "------------------------------------------------------------", t1)
	}
	fstest.CheckItems(t, r.Fremote, file1, file2r, file3)
	check(4, 1)

	r.WriteFile("empty space", "", t2)
	fstest.CheckItems(t, r.Flocal, file1, file2, file3)
	check(5, 0)
}

func TestCheck(t *testing.T) {
	testCheck(t, fs.Check)
}

func TestCheckDownload(t *testing.T) {
	testCheck(t, fs.CheckDownload)
}

func TestCheckSizeOnly(t *testing.T) {
	fs.Config.SizeOnly = true
	defer func() { fs.Config.SizeOnly = false }()
	TestCheck(t)
}

func skipIfCantDedupe(t *testing.T, f fs.Fs) {
	if f.Features().PutUnchecked == nil {
		t.Skip("Can't test deduplicate - no PutUnchecked")
	}
	if !f.Features().DuplicateFiles {
		t.Skip("Can't test deduplicate - no duplicate files possible")
	}
	if !f.Hashes().Contains(fs.HashMD5) {
		t.Skip("Can't test deduplicate - MD5 not supported")
	}
}

func TestDeduplicateInteractive(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	skipIfCantDedupe(t, r.Fremote)

	file1 := r.WriteUncheckedObject("one", "This is one", t1)
	file2 := r.WriteUncheckedObject("one", "This is one", t1)
	file3 := r.WriteUncheckedObject("one", "This is one", t1)
	r.CheckWithDuplicates(t, file1, file2, file3)

	err := fs.Deduplicate(r.Fremote, fs.DeduplicateInteractive)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Fremote, file1)
}

func TestDeduplicateSkip(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	skipIfCantDedupe(t, r.Fremote)

	file1 := r.WriteUncheckedObject("one", "This is one", t1)
	file2 := r.WriteUncheckedObject("one", "This is one", t1)
	file3 := r.WriteUncheckedObject("one", "This is another one", t1)
	r.CheckWithDuplicates(t, file1, file2, file3)

	err := fs.Deduplicate(r.Fremote, fs.DeduplicateSkip)
	require.NoError(t, err)

	r.CheckWithDuplicates(t, file1, file3)
}

func TestDeduplicateFirst(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	skipIfCantDedupe(t, r.Fremote)

	file1 := r.WriteUncheckedObject("one", "This is one", t1)
	file2 := r.WriteUncheckedObject("one", "This is one A", t1)
	file3 := r.WriteUncheckedObject("one", "This is one BB", t1)
	r.CheckWithDuplicates(t, file1, file2, file3)

	err := fs.Deduplicate(r.Fremote, fs.DeduplicateFirst)
	require.NoError(t, err)

	objects, size, err := fs.Count(r.Fremote)
	require.NoError(t, err)
	assert.Equal(t, int64(1), objects)
	if size != file1.Size && size != file2.Size && size != file3.Size {
		t.Errorf("Size not one of the object sizes %d", size)
	}
}

func TestDeduplicateNewest(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	skipIfCantDedupe(t, r.Fremote)

	file1 := r.WriteUncheckedObject("one", "This is one", t1)
	file2 := r.WriteUncheckedObject("one", "This is one too", t2)
	file3 := r.WriteUncheckedObject("one", "This is another one", t3)
	r.CheckWithDuplicates(t, file1, file2, file3)

	err := fs.Deduplicate(r.Fremote, fs.DeduplicateNewest)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Fremote, file3)
}

func TestDeduplicateOldest(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	skipIfCantDedupe(t, r.Fremote)

	file1 := r.WriteUncheckedObject("one", "This is one", t1)
	file2 := r.WriteUncheckedObject("one", "This is one too", t2)
	file3 := r.WriteUncheckedObject("one", "This is another one", t3)
	r.CheckWithDuplicates(t, file1, file2, file3)

	err := fs.Deduplicate(r.Fremote, fs.DeduplicateOldest)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Fremote, file1)
}

func TestDeduplicateRename(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	skipIfCantDedupe(t, r.Fremote)

	file1 := r.WriteUncheckedObject("one.txt", "This is one", t1)
	file2 := r.WriteUncheckedObject("one.txt", "This is one too", t2)
	file3 := r.WriteUncheckedObject("one.txt", "This is another one", t3)
	r.CheckWithDuplicates(t, file1, file2, file3)

	err := fs.Deduplicate(r.Fremote, fs.DeduplicateRename)
	require.NoError(t, err)

	require.NoError(t, fs.Walk(r.Fremote, "", true, -1, func(dirPath string, entries fs.DirEntries, err error) error {
		if err != nil {
			return err
		}
		entries.ForObject(func(o fs.Object) {
			remote := o.Remote()
			if remote != "one-1.txt" &&
				remote != "one-2.txt" &&
				remote != "one-3.txt" {
				t.Errorf("Bad file name after rename %q", remote)
			}
			size := o.Size()
			if size != file1.Size && size != file2.Size && size != file3.Size {
				t.Errorf("Size not one of the object sizes %d", size)
			}
		})
		return nil
	}))
}

// This should really be a unit test, but the test framework there
// doesn't have enough tools to make it easy
func TestMergeDirs(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	mergeDirs := r.Fremote.Features().MergeDirs
	if mergeDirs == nil {
		t.Skip("Can't merge directories")
	}

	file1 := r.WriteObject("dupe1/one.txt", "This is one", t1)
	file2 := r.WriteObject("dupe2/two.txt", "This is one too", t2)
	file3 := r.WriteObject("dupe3/three.txt", "This is another one", t3)

	objs, dirs, err := fs.WalkGetAll(r.Fremote, "", true, 1)
	require.NoError(t, err)
	assert.Equal(t, 3, len(dirs))
	assert.Equal(t, 0, len(objs))

	err = mergeDirs(dirs)
	require.NoError(t, err)

	file2.Path = "dupe1/two.txt"
	file3.Path = "dupe1/three.txt"
	fstest.CheckItems(t, r.Fremote, file1, file2, file3)

	objs, dirs, err = fs.WalkGetAll(r.Fremote, "", true, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, len(dirs))
	assert.Equal(t, 0, len(objs))
	assert.Equal(t, "dupe1", dirs[0].Remote())
}

func TestCat(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth("file1", "ABCDEFGHIJ", t1)
	file2 := r.WriteBoth("file2", "012345678", t2)

	fstest.CheckItems(t, r.Fremote, file1, file2)

	for _, test := range []struct {
		offset int64
		count  int64
		a      string
		b      string
	}{
		{0, -1, "ABCDEFGHIJ", "012345678"},
		{0, 5, "ABCDE", "01234"},
		{-3, -1, "HIJ", "678"},
		{1, 3, "BCD", "123"},
	} {
		var buf bytes.Buffer
		err := fs.Cat(r.Fremote, &buf, test.offset, test.count)
		require.NoError(t, err)
		res := buf.String()

		if res != test.a+test.b && res != test.b+test.a {
			t.Errorf("Incorrect output from Cat(%d,%d): %q", test.offset, test.count, res)
		}
	}
}

func TestRcat(t *testing.T) {
	checkSumBefore := fs.Config.CheckSum
	defer func() { fs.Config.CheckSum = checkSumBefore }()

	check := func(withChecksum bool) {
		fs.Config.CheckSum = withChecksum
		prefix := "no_checksum_"
		if withChecksum {
			prefix = "with_checksum_"
		}

		r := fstest.NewRun(t)
		defer r.Finalise()

		fstest.CheckListing(t, r.Fremote, []fstest.Item{})

		data1 := "this is some really nice test data"
		path1 := prefix + "small_file_from_pipe"

		data2 := string(make([]byte, fs.Config.StreamingUploadCutoff+1))
		path2 := prefix + "big_file_from_pipe"

		in := ioutil.NopCloser(strings.NewReader(data1))
		_, err := fs.Rcat(r.Fremote, path1, in, t1)
		require.NoError(t, err)

		in = ioutil.NopCloser(strings.NewReader(data2))
		_, err = fs.Rcat(r.Fremote, path2, in, t2)
		require.NoError(t, err)

		file1 := fstest.NewItem(path1, data1, t1)
		file2 := fstest.NewItem(path2, data2, t2)
		fstest.CheckItems(t, r.Fremote, file1, file2)
	}

	check(true)
	check(false)
}

func TestRmdirsNoLeaveRoot(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	r.Mkdir(r.Fremote)

	// Make some files and dirs we expect to keep
	r.ForceMkdir(r.Fremote)
	file1 := r.WriteObject("A1/B1/C1/one", "aaa", t1)
	//..and dirs we expect to delete
	require.NoError(t, fs.Mkdir(r.Fremote, "A2"))
	require.NoError(t, fs.Mkdir(r.Fremote, "A1/B2"))
	require.NoError(t, fs.Mkdir(r.Fremote, "A1/B2/C2"))
	require.NoError(t, fs.Mkdir(r.Fremote, "A1/B1/C3"))
	require.NoError(t, fs.Mkdir(r.Fremote, "A3"))
	require.NoError(t, fs.Mkdir(r.Fremote, "A3/B3"))
	require.NoError(t, fs.Mkdir(r.Fremote, "A3/B3/C4"))
	//..and one more file at the end
	file2 := r.WriteObject("A1/two", "bbb", t2)

	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		[]fstest.Item{
			file1, file2,
		},
		[]string{
			"A1",
			"A1/B1",
			"A1/B1/C1",
			"A2",
			"A1/B2",
			"A1/B2/C2",
			"A1/B1/C3",
			"A3",
			"A3/B3",
			"A3/B3/C4",
		},
		fs.Config.ModifyWindow,
	)

	require.NoError(t, fs.Rmdirs(r.Fremote, "", false))

	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		[]fstest.Item{
			file1, file2,
		},
		[]string{
			"A1",
			"A1/B1",
			"A1/B1/C1",
		},
		fs.Config.ModifyWindow,
	)

}

func TestRmdirsLeaveRoot(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	r.Mkdir(r.Fremote)

	r.ForceMkdir(r.Fremote)

	require.NoError(t, fs.Mkdir(r.Fremote, "A1"))
	require.NoError(t, fs.Mkdir(r.Fremote, "A1/B1"))
	require.NoError(t, fs.Mkdir(r.Fremote, "A1/B1/C1"))

	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		[]fstest.Item{},
		[]string{
			"A1",
			"A1/B1",
			"A1/B1/C1",
		},
		fs.Config.ModifyWindow,
	)

	require.NoError(t, fs.Rmdirs(r.Fremote, "A1", true))

	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		[]fstest.Item{},
		[]string{
			"A1",
		},
		fs.Config.ModifyWindow,
	)
}

func TestMoveFile(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	file1 := r.WriteFile("file1", "file1 contents", t1)
	fstest.CheckItems(t, r.Flocal, file1)

	file2 := file1
	file2.Path = "sub/file2"

	err := fs.MoveFile(r.Fremote, r.Flocal, file2.Path, file1.Path)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal)
	fstest.CheckItems(t, r.Fremote, file2)

	r.WriteFile("file1", "file1 contents", t1)
	fstest.CheckItems(t, r.Flocal, file1)

	err = fs.MoveFile(r.Fremote, r.Flocal, file2.Path, file1.Path)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal)
	fstest.CheckItems(t, r.Fremote, file2)

	err = fs.MoveFile(r.Fremote, r.Fremote, file2.Path, file2.Path)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal)
	fstest.CheckItems(t, r.Fremote, file2)
}

func TestCopyFile(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	file1 := r.WriteFile("file1", "file1 contents", t1)
	fstest.CheckItems(t, r.Flocal, file1)

	file2 := file1
	file2.Path = "sub/file2"

	err := fs.CopyFile(r.Fremote, r.Flocal, file2.Path, file1.Path)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote, file2)

	err = fs.CopyFile(r.Fremote, r.Flocal, file2.Path, file1.Path)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote, file2)

	err = fs.CopyFile(r.Fremote, r.Fremote, file2.Path, file2.Path)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote, file2)
}

// testFsInfo is for unit testing fs.Info
type testFsInfo struct {
	name      string
	root      string
	stringVal string
	precision time.Duration
	hashes    fs.HashSet
	features  fs.Features
}

// Name of the remote (as passed into NewFs)
func (i *testFsInfo) Name() string { return i.name }

// Root of the remote (as passed into NewFs)
func (i *testFsInfo) Root() string { return i.root }

// String returns a description of the FS
func (i *testFsInfo) String() string { return i.stringVal }

// Precision of the ModTimes in this Fs
func (i *testFsInfo) Precision() time.Duration { return i.precision }

// Returns the supported hash types of the filesystem
func (i *testFsInfo) Hashes() fs.HashSet { return i.hashes }

// Returns the supported hash types of the filesystem
func (i *testFsInfo) Features() *fs.Features { return &i.features }

func TestSameConfig(t *testing.T) {
	a := &testFsInfo{name: "name", root: "root"}
	for _, test := range []struct {
		name     string
		root     string
		expected bool
	}{
		{"name", "root", true},
		{"name", "rooty", true},
		{"namey", "root", false},
		{"namey", "roott", false},
	} {
		b := &testFsInfo{name: test.name, root: test.root}
		actual := fs.SameConfig(a, b)
		assert.Equal(t, test.expected, actual)
		actual = fs.SameConfig(b, a)
		assert.Equal(t, test.expected, actual)
	}
}

func TestSame(t *testing.T) {
	a := &testFsInfo{name: "name", root: "root"}
	for _, test := range []struct {
		name     string
		root     string
		expected bool
	}{
		{"name", "root", true},
		{"name", "rooty", false},
		{"namey", "root", false},
		{"namey", "roott", false},
	} {
		b := &testFsInfo{name: test.name, root: test.root}
		actual := fs.Same(a, b)
		assert.Equal(t, test.expected, actual)
		actual = fs.Same(b, a)
		assert.Equal(t, test.expected, actual)
	}
}

func TestOverlapping(t *testing.T) {
	a := &testFsInfo{name: "name", root: "root"}
	for _, test := range []struct {
		name     string
		root     string
		expected bool
	}{
		{"name", "root", true},
		{"namey", "root", false},
		{"name", "rooty", false},
		{"namey", "rooty", false},
		{"name", "roo", false},
		{"name", "root/toot", true},
		{"name", "root/toot/", true},
		{"name", "", true},
		{"name", "/", true},
	} {
		b := &testFsInfo{name: test.name, root: test.root}
		what := fmt.Sprintf("(%q,%q) vs (%q,%q)", a.name, a.root, b.name, b.root)
		actual := fs.Overlapping(a, b)
		assert.Equal(t, test.expected, actual, what)
		actual = fs.Overlapping(b, a)
		assert.Equal(t, test.expected, actual, what)
	}
}

func TestListDirSorted(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	fs.Config.Filter.MaxSize = 10
	defer func() {
		fs.Config.Filter.MaxSize = -1
	}()

	files := []fstest.Item{
		r.WriteObject("a.txt", "hello world", t1),
		r.WriteObject("zend.txt", "hello", t1),
		r.WriteObject("sub dir/hello world", "hello world", t1),
		r.WriteObject("sub dir/hello world2", "hello world", t1),
		r.WriteObject("sub dir/ignore dir/.ignore", "", t1),
		r.WriteObject("sub dir/ignore dir/should be ignored", "to ignore", t1),
		r.WriteObject("sub dir/sub sub dir/hello world3", "hello world", t1),
	}
	fstest.CheckItems(t, r.Fremote, files...)
	var items fs.DirEntries
	var err error

	// Turn the DirEntry into a name, ending with a / if it is a
	// dir
	str := func(i int) string {
		item := items[i]
		name := item.Remote()
		switch item.(type) {
		case fs.Object:
		case fs.Directory:
			name += "/"
		default:
			t.Fatalf("Unknown type %+v", item)
		}
		return name
	}

	items, err = fs.ListDirSorted(r.Fremote, true, "")
	require.NoError(t, err)
	require.Len(t, items, 3)
	assert.Equal(t, "a.txt", str(0))
	assert.Equal(t, "sub dir/", str(1))
	assert.Equal(t, "zend.txt", str(2))

	items, err = fs.ListDirSorted(r.Fremote, false, "")
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "sub dir/", str(0))
	assert.Equal(t, "zend.txt", str(1))

	items, err = fs.ListDirSorted(r.Fremote, true, "sub dir")
	require.NoError(t, err)
	require.Len(t, items, 4)
	assert.Equal(t, "sub dir/hello world", str(0))
	assert.Equal(t, "sub dir/hello world2", str(1))
	assert.Equal(t, "sub dir/ignore dir/", str(2))
	assert.Equal(t, "sub dir/sub sub dir/", str(3))

	items, err = fs.ListDirSorted(r.Fremote, false, "sub dir")
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "sub dir/ignore dir/", str(0))
	assert.Equal(t, "sub dir/sub sub dir/", str(1))

	// testing ignore file
	fs.Config.Filter.ExcludeFile = ".ignore"

	items, err = fs.ListDirSorted(r.Fremote, false, "sub dir")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "sub dir/sub sub dir/", str(0))

	items, err = fs.ListDirSorted(r.Fremote, false, "sub dir/ignore dir")
	require.NoError(t, err)
	require.Len(t, items, 0)

	items, err = fs.ListDirSorted(r.Fremote, true, "sub dir/ignore dir")
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "sub dir/ignore dir/.ignore", str(0))
	assert.Equal(t, "sub dir/ignore dir/should be ignored", str(1))

	fs.Config.Filter.ExcludeFile = ""
	items, err = fs.ListDirSorted(r.Fremote, false, "sub dir/ignore dir")
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "sub dir/ignore dir/.ignore", str(0))
	assert.Equal(t, "sub dir/ignore dir/should be ignored", str(1))
}

type byteReader struct {
	c byte
}

func (br *byteReader) Read(p []byte) (n int, err error) {
	if br.c == 0 {
		err = io.EOF
	} else if len(p) >= 1 {
		p[0] = br.c
		n = 1
		br.c--
	}
	return
}

func TestReadFill(t *testing.T) {
	buf := []byte{9, 9, 9, 9, 9}

	n, err := fs.ReadFill(&byteReader{0}, buf)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 0, n)
	assert.Equal(t, []byte{9, 9, 9, 9, 9}, buf)

	n, err = fs.ReadFill(&byteReader{3}, buf)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 3, n)
	assert.Equal(t, []byte{3, 2, 1, 9, 9}, buf)

	n, err = fs.ReadFill(&byteReader{8}, buf)
	assert.Equal(t, nil, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, []byte{8, 7, 6, 5, 4}, buf)
}

type errorReader struct {
	err error
}

func (er errorReader) Read(p []byte) (n int, err error) {
	return 0, er.err
}

func TestCheckEqualReaders(t *testing.T) {
	b65a := make([]byte, 65*1024)
	b65b := make([]byte, 65*1024)
	b65b[len(b65b)-1] = 1
	b66 := make([]byte, 66*1024)

	differ, err := fs.CheckEqualReaders(bytes.NewBuffer(b65a), bytes.NewBuffer(b65a))
	assert.NoError(t, err)
	assert.Equal(t, differ, false)

	differ, err = fs.CheckEqualReaders(bytes.NewBuffer(b65a), bytes.NewBuffer(b65b))
	assert.NoError(t, err)
	assert.Equal(t, differ, true)

	differ, err = fs.CheckEqualReaders(bytes.NewBuffer(b65a), bytes.NewBuffer(b66))
	assert.NoError(t, err)
	assert.Equal(t, differ, true)

	differ, err = fs.CheckEqualReaders(bytes.NewBuffer(b66), bytes.NewBuffer(b65a))
	assert.NoError(t, err)
	assert.Equal(t, differ, true)

	myErr := errors.New("sentinel")
	wrap := func(b []byte) io.Reader {
		r := bytes.NewBuffer(b)
		e := errorReader{myErr}
		return io.MultiReader(r, e)
	}

	differ, err = fs.CheckEqualReaders(wrap(b65a), bytes.NewBuffer(b65a))
	assert.Equal(t, myErr, err)
	assert.Equal(t, differ, true)

	differ, err = fs.CheckEqualReaders(wrap(b65a), bytes.NewBuffer(b65b))
	assert.Equal(t, myErr, err)
	assert.Equal(t, differ, true)

	differ, err = fs.CheckEqualReaders(wrap(b65a), bytes.NewBuffer(b66))
	assert.Equal(t, myErr, err)
	assert.Equal(t, differ, true)

	differ, err = fs.CheckEqualReaders(wrap(b66), bytes.NewBuffer(b65a))
	assert.Equal(t, myErr, err)
	assert.Equal(t, differ, true)

	differ, err = fs.CheckEqualReaders(bytes.NewBuffer(b65a), wrap(b65a))
	assert.Equal(t, myErr, err)
	assert.Equal(t, differ, true)

	differ, err = fs.CheckEqualReaders(bytes.NewBuffer(b65a), wrap(b65b))
	assert.Equal(t, myErr, err)
	assert.Equal(t, differ, true)

	differ, err = fs.CheckEqualReaders(bytes.NewBuffer(b65a), wrap(b66))
	assert.Equal(t, myErr, err)
	assert.Equal(t, differ, true)

	differ, err = fs.CheckEqualReaders(bytes.NewBuffer(b66), wrap(b65a))
	assert.Equal(t, myErr, err)
	assert.Equal(t, differ, true)
}

func TestListFormat(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject("a", "a", t1)
	file2 := r.WriteObject("subdir/b", "b", t1)

	fstest.CheckItems(t, r.Fremote, file1, file2)

	items, _ := fs.ListDirSorted(r.Fremote, true, "")
	var list fs.ListFormat
	list.AddPath()
	list.SetDirSlash(false)
	assert.Equal(t, "subdir", fs.ListFormatted(&items[1], &list))

	list.SetDirSlash(true)
	assert.Equal(t, "subdir/", fs.ListFormatted(&items[1], &list))

	list.SetOutput(nil)
	assert.Equal(t, "", fs.ListFormatted(&items[1], &list))

	list.AppendOutput(func() string { return "a" })
	list.AppendOutput(func() string { return "b" })
	assert.Equal(t, "ab", fs.ListFormatted(&items[1], &list))
	list.SetSeparator(":::")
	assert.Equal(t, "a:::b", fs.ListFormatted(&items[1], &list))

	list.SetOutput(nil)
	list.AddModTime()
	assert.Equal(t, items[0].ModTime().Format("2006-01-02 15:04:05"), fs.ListFormatted(&items[0], &list))

	list.SetOutput(nil)
	list.AddSize()
	assert.Equal(t, "1", fs.ListFormatted(&items[0], &list))

	list.AddPath()
	list.AddModTime()
	list.SetDirSlash(true)
	list.SetSeparator("__SEP__")
	assert.Equal(t, "1__SEP__a__SEP__"+items[0].ModTime().Format("2006-01-02 15:04:05"), fs.ListFormatted(&items[0], &list))
	assert.Equal(t, fmt.Sprintf("%d", items[1].Size())+"__SEP__subdir/__SEP__"+items[1].ModTime().Format("2006-01-02 15:04:05"), fs.ListFormatted(&items[1], &list))

	for _, test := range []struct {
		ht   fs.HashType
		want string
	}{
		{fs.HashMD5, "0cc175b9c0f1b6a831c399e269772661"},
		{fs.HashSHA1, "86f7e437faa5a7fce15d1ddcb9eaeaea377667b8"},
		{fs.HashDropbox, "bf5d3affb73efd2ec6c36ad3112dd933efed63c4e1cbffcfa88e2759c144f2d8"},
	} {
		list.SetOutput(nil)
		list.AddHash(test.ht)
		got := fs.ListFormatted(&items[0], &list)
		if got != "UNSUPPORTED" && got != "" {
			assert.Equal(t, test.want, got)
		}
	}
}
