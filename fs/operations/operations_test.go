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
// Call accounting.GlobalStats().ResetCounters() before every fs.Sync() as it
// uses the error count internally.

package operations_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/all" // import all backends
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/random"
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

	err := operations.Mkdir(context.Background(), r.Fremote, "")
	require.NoError(t, err)
	fstest.CheckListing(t, r.Fremote, []fstest.Item{})

	err = operations.Mkdir(context.Background(), r.Fremote, "")
	require.NoError(t, err)
}

func TestLsd(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject(context.Background(), "sub dir/hello world", "hello world", t1)

	fstest.CheckItems(t, r.Fremote, file1)

	var buf bytes.Buffer
	err := operations.ListDir(context.Background(), r.Fremote, &buf)
	require.NoError(t, err)
	res := buf.String()
	assert.Contains(t, res, "sub dir\n")
}

func TestLs(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth(context.Background(), "potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth(context.Background(), "empty space", "-", t2)

	fstest.CheckItems(t, r.Fremote, file1, file2)

	var buf bytes.Buffer
	err := operations.List(context.Background(), r.Fremote, &buf)
	require.NoError(t, err)
	res := buf.String()
	assert.Contains(t, res, "        1 empty space\n")
	assert.Contains(t, res, "       60 potato2\n")
}

func TestLsWithFilesFrom(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth(context.Background(), "potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth(context.Background(), "empty space", "-", t2)

	fstest.CheckItems(t, r.Fremote, file1, file2)

	// Set the --files-from equivalent
	f, err := filter.NewFilter(nil)
	require.NoError(t, err)
	require.NoError(t, f.AddFile("potato2"))
	require.NoError(t, f.AddFile("notfound"))

	// Monkey patch the active filter
	oldFilter := filter.Active
	filter.Active = f
	defer func() {
		filter.Active = oldFilter
	}()

	var buf bytes.Buffer
	err = operations.List(context.Background(), r.Fremote, &buf)
	require.NoError(t, err)
	assert.Equal(t, "       60 potato2\n", buf.String())

	// Now try with --no-traverse
	oldNoTraverse := fs.Config.NoTraverse
	fs.Config.NoTraverse = true
	defer func() {
		fs.Config.NoTraverse = oldNoTraverse
	}()

	buf.Reset()
	err = operations.List(context.Background(), r.Fremote, &buf)
	require.NoError(t, err)
	assert.Equal(t, "       60 potato2\n", buf.String())
}

func TestLsLong(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth(context.Background(), "potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth(context.Background(), "empty space", "-", t2)

	fstest.CheckItems(t, r.Fremote, file1, file2)

	var buf bytes.Buffer
	err := operations.ListLong(context.Background(), r.Fremote, &buf)
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
			fstest.AssertTimeEqualWithPrecision(t, filename, expected, modTime, precision)
		}
	}

	m1 := regexp.MustCompile(`(?m)^        1 (\d{4}-\d\d-\d\d \d\d:\d\d:\d\d\.\d{9}) empty space$`)
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
	file1 := r.WriteBoth(context.Background(), "potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth(context.Background(), "empty space", "-", t2)

	fstest.CheckItems(t, r.Fremote, file1, file2)

	// MD5 Sum

	var buf bytes.Buffer
	err := operations.Md5sum(context.Background(), r.Fremote, &buf)
	require.NoError(t, err)
	res := buf.String()
	if !strings.Contains(res, "336d5ebc5436534e61d16e63ddfca327  empty space\n") &&
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
	err = operations.Sha1sum(context.Background(), r.Fremote, &buf)
	require.NoError(t, err)
	res = buf.String()
	if !strings.Contains(res, "3bc15c8aae3e4124dd409035f32ea2fd6835efc9  empty space\n") &&
		!strings.Contains(res, "                             UNSUPPORTED  empty space\n") &&
		!strings.Contains(res, "                                          empty space\n") {
		t.Errorf("empty space missing: %q", res)
	}
	if !strings.Contains(res, "9dc7f7d3279715991a22853f5981df582b7f9f6d  potato2\n") &&
		!strings.Contains(res, "                             UNSUPPORTED  potato2\n") &&
		!strings.Contains(res, "                                          potato2\n") {
		t.Errorf("potato2 missing: %q", res)
	}

	// QuickXorHash Sum

	buf.Reset()
	var ht hash.Type
	err = ht.Set("QuickXorHash")
	require.NoError(t, err)
	err = operations.HashLister(context.Background(), ht, r.Fremote, &buf)
	require.NoError(t, err)
	res = buf.String()
	if !strings.Contains(res, "2d00000000000000000000000100000000000000  empty space\n") &&
		!strings.Contains(res, "                             UNSUPPORTED  empty space\n") &&
		!strings.Contains(res, "                                          empty space\n") {
		t.Errorf("empty space missing: %q", res)
	}
	if !strings.Contains(res, "4001dad296b6b4a52d6d694b67dad296b6b4a52d  potato2\n") &&
		!strings.Contains(res, "                             UNSUPPORTED  potato2\n") &&
		!strings.Contains(res, "                                          potato2\n") {
		t.Errorf("potato2 missing: %q", res)
	}

	// QuickXorHash Sum with Base64 Encoded

	buf.Reset()
	err = operations.HashListerBase64(context.Background(), ht, r.Fremote, &buf)
	require.NoError(t, err)
	res = buf.String()
	if !strings.Contains(res, "LQAAAAAAAAAAAAAAAQAAAAAAAAA=  empty space\n") &&
		!strings.Contains(res, "                 UNSUPPORTED  empty space\n") &&
		!strings.Contains(res, "                              empty space\n") {
		t.Errorf("empty space missing: %q", res)
	}
	if !strings.Contains(res, "QAHa0pa2tKUtbWlLZ9rSlra0pS0=  potato2\n") &&
		!strings.Contains(res, "                 UNSUPPORTED  potato2\n") &&
		!strings.Contains(res, "                              potato2\n") {
		t.Errorf("potato2 missing: %q", res)
	}
}

func TestSuffixName(t *testing.T) {
	origSuffix, origKeepExt := fs.Config.Suffix, fs.Config.SuffixKeepExtension
	defer func() {
		fs.Config.Suffix, fs.Config.SuffixKeepExtension = origSuffix, origKeepExt
	}()
	for _, test := range []struct {
		remote  string
		suffix  string
		keepExt bool
		want    string
	}{
		{"test.txt", "", false, "test.txt"},
		{"test.txt", "", true, "test.txt"},
		{"test.txt", "-suffix", false, "test.txt-suffix"},
		{"test.txt", "-suffix", true, "test-suffix.txt"},
		{"test.txt.csv", "-suffix", false, "test.txt.csv-suffix"},
		{"test.txt.csv", "-suffix", true, "test.txt-suffix.csv"},
		{"test", "-suffix", false, "test-suffix"},
		{"test", "-suffix", true, "test-suffix"},
	} {
		fs.Config.Suffix = test.suffix
		fs.Config.SuffixKeepExtension = test.keepExt
		got := operations.SuffixName(test.remote)
		assert.Equal(t, test.want, got, fmt.Sprintf("%+v", test))
	}
}

func TestCount(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth(context.Background(), "potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth(context.Background(), "empty space", "-", t2)
	file3 := r.WriteBoth(context.Background(), "sub dir/potato3", "hello", t2)

	fstest.CheckItems(t, r.Fremote, file1, file2, file3)

	// Check the MaxDepth too
	fs.Config.MaxDepth = 1
	defer func() { fs.Config.MaxDepth = -1 }()

	objects, size, err := operations.Count(context.Background(), r.Fremote)
	require.NoError(t, err)
	assert.Equal(t, int64(2), objects)
	assert.Equal(t, int64(61), size)
}

func TestDelete(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject(context.Background(), "small", "1234567890", t2)                                                                                           // 10 bytes
	file2 := r.WriteObject(context.Background(), "medium", "------------------------------------------------------------", t1)                                        // 60 bytes
	file3 := r.WriteObject(context.Background(), "large", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", t1) // 100 bytes
	fstest.CheckItems(t, r.Fremote, file1, file2, file3)

	filter.Active.Opt.MaxSize = 60
	defer func() {
		filter.Active.Opt.MaxSize = -1
	}()

	err := operations.Delete(context.Background(), r.Fremote)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Fremote, file3)
}

func TestRetry(t *testing.T) {
	var i int
	var err error
	fn := func() error {
		i--
		if i <= 0 {
			return nil
		}
		return err
	}

	i, err = 3, io.EOF
	assert.Equal(t, nil, operations.Retry(nil, 5, fn))
	assert.Equal(t, 0, i)

	i, err = 10, io.EOF
	assert.Equal(t, io.EOF, operations.Retry(nil, 5, fn))
	assert.Equal(t, 5, i)

	i, err = 10, fs.ErrorObjectNotFound
	assert.Equal(t, fs.ErrorObjectNotFound, operations.Retry(nil, 5, fn))
	assert.Equal(t, 9, i)

}

func TestCat(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth(context.Background(), "file1", "ABCDEFGHIJ", t1)
	file2 := r.WriteBoth(context.Background(), "file2", "012345678", t2)

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
		err := operations.Cat(context.Background(), r.Fremote, &buf, test.offset, test.count)
		require.NoError(t, err)
		res := buf.String()

		if res != test.a+test.b && res != test.b+test.a {
			t.Errorf("Incorrect output from Cat(%d,%d): %q", test.offset, test.count, res)
		}
	}
}

func TestPurge(t *testing.T) {
	r := fstest.NewRunIndividual(t) // make new container (azureblob has delayed mkdir after rmdir)
	defer r.Finalise()
	r.Mkdir(context.Background(), r.Fremote)

	// Make some files and dirs
	r.ForceMkdir(context.Background(), r.Fremote)
	file1 := r.WriteObject(context.Background(), "A1/B1/C1/one", "aaa", t1)
	//..and dirs we expect to delete
	require.NoError(t, operations.Mkdir(context.Background(), r.Fremote, "A2"))
	require.NoError(t, operations.Mkdir(context.Background(), r.Fremote, "A1/B2"))
	require.NoError(t, operations.Mkdir(context.Background(), r.Fremote, "A1/B2/C2"))
	require.NoError(t, operations.Mkdir(context.Background(), r.Fremote, "A1/B1/C3"))
	require.NoError(t, operations.Mkdir(context.Background(), r.Fremote, "A3"))
	require.NoError(t, operations.Mkdir(context.Background(), r.Fremote, "A3/B3"))
	require.NoError(t, operations.Mkdir(context.Background(), r.Fremote, "A3/B3/C4"))
	//..and one more file at the end
	file2 := r.WriteObject(context.Background(), "A1/two", "bbb", t2)

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
		fs.GetModifyWindow(r.Fremote),
	)

	require.NoError(t, operations.Purge(context.Background(), r.Fremote, "A1/B1"))

	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		[]fstest.Item{
			file2,
		},
		[]string{
			"A1",
			"A2",
			"A1/B2",
			"A1/B2/C2",
			"A3",
			"A3/B3",
			"A3/B3/C4",
		},
		fs.GetModifyWindow(r.Fremote),
	)

	require.NoError(t, operations.Purge(context.Background(), r.Fremote, ""))

	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		[]fstest.Item{},
		[]string{},
		fs.GetModifyWindow(r.Fremote),
	)

}

func TestRmdirsNoLeaveRoot(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	r.Mkdir(context.Background(), r.Fremote)

	// Make some files and dirs we expect to keep
	r.ForceMkdir(context.Background(), r.Fremote)
	file1 := r.WriteObject(context.Background(), "A1/B1/C1/one", "aaa", t1)
	//..and dirs we expect to delete
	require.NoError(t, operations.Mkdir(context.Background(), r.Fremote, "A2"))
	require.NoError(t, operations.Mkdir(context.Background(), r.Fremote, "A1/B2"))
	require.NoError(t, operations.Mkdir(context.Background(), r.Fremote, "A1/B2/C2"))
	require.NoError(t, operations.Mkdir(context.Background(), r.Fremote, "A1/B1/C3"))
	require.NoError(t, operations.Mkdir(context.Background(), r.Fremote, "A3"))
	require.NoError(t, operations.Mkdir(context.Background(), r.Fremote, "A3/B3"))
	require.NoError(t, operations.Mkdir(context.Background(), r.Fremote, "A3/B3/C4"))
	//..and one more file at the end
	file2 := r.WriteObject(context.Background(), "A1/two", "bbb", t2)

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
		fs.GetModifyWindow(r.Fremote),
	)

	require.NoError(t, operations.Rmdirs(context.Background(), r.Fremote, "A3/B3/C4", false))

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
		},
		fs.GetModifyWindow(r.Fremote),
	)

	require.NoError(t, operations.Rmdirs(context.Background(), r.Fremote, "", false))

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
		fs.GetModifyWindow(r.Fremote),
	)

}

func TestRmdirsLeaveRoot(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	r.Mkdir(context.Background(), r.Fremote)

	r.ForceMkdir(context.Background(), r.Fremote)

	require.NoError(t, operations.Mkdir(context.Background(), r.Fremote, "A1"))
	require.NoError(t, operations.Mkdir(context.Background(), r.Fremote, "A1/B1"))
	require.NoError(t, operations.Mkdir(context.Background(), r.Fremote, "A1/B1/C1"))

	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		[]fstest.Item{},
		[]string{
			"A1",
			"A1/B1",
			"A1/B1/C1",
		},
		fs.GetModifyWindow(r.Fremote),
	)

	require.NoError(t, operations.Rmdirs(context.Background(), r.Fremote, "A1", true))

	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		[]fstest.Item{},
		[]string{
			"A1",
		},
		fs.GetModifyWindow(r.Fremote),
	)
}

func TestCopyURL(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	contents := "file contents\n"
	file1 := r.WriteFile("file1", contents, t1)
	file2 := r.WriteFile("file2", contents, t1)
	r.Mkdir(context.Background(), r.Fremote)
	fstest.CheckItems(t, r.Fremote)

	// check when reading from regular HTTP server
	status := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != 0 {
			http.Error(w, "an error ocurred", status)
		}
		_, err := w.Write([]byte(contents))
		assert.NoError(t, err)
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	o, err := operations.CopyURL(context.Background(), r.Fremote, "file1", ts.URL, false, false)
	require.NoError(t, err)
	assert.Equal(t, int64(len(contents)), o.Size())

	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1}, nil, fs.ModTimeNotSupported)

	// Check file clobbering
	o, err = operations.CopyURL(context.Background(), r.Fremote, "file1", ts.URL, false, true)
	require.Error(t, err)

	// Check auto file naming
	status = 0
	urlFileName := "filename.txt"
	o, err = operations.CopyURL(context.Background(), r.Fremote, "", ts.URL+"/"+urlFileName, true, false)
	require.NoError(t, err)
	assert.Equal(t, int64(len(contents)), o.Size())
	assert.Equal(t, urlFileName, o.Remote())

	// Check auto file naming when url without file name
	o, err = operations.CopyURL(context.Background(), r.Fremote, "file1", ts.URL, true, false)
	require.Error(t, err)

	// Check an error is returned for a 404
	status = http.StatusNotFound
	o, err = operations.CopyURL(context.Background(), r.Fremote, "file1", ts.URL, false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Not Found")
	assert.Nil(t, o)
	status = 0

	// check when reading from unverified HTTPS server
	fs.Config.InsecureSkipVerify = true
	fshttp.ResetTransport()
	defer func() {
		fs.Config.InsecureSkipVerify = false
		fshttp.ResetTransport()
	}()
	tss := httptest.NewTLSServer(handler)
	defer tss.Close()

	o, err = operations.CopyURL(context.Background(), r.Fremote, "file2", tss.URL, false, false)
	require.NoError(t, err)
	assert.Equal(t, int64(len(contents)), o.Size())
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1, file2, fstest.NewItem(urlFileName, contents, t1)}, nil, fs.ModTimeNotSupported)
}

func TestCopyURLToWriter(t *testing.T) {
	contents := "file contents\n"

	// check when reading from regular HTTP server
	status := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != 0 {
			http.Error(w, "an error ocurred", status)
			return
		}
		_, err := w.Write([]byte(contents))
		assert.NoError(t, err)
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// test normal fetch
	var buf bytes.Buffer
	err := operations.CopyURLToWriter(context.Background(), ts.URL, &buf)
	require.NoError(t, err)
	assert.Equal(t, contents, buf.String())

	// test fetch with error
	status = http.StatusNotFound
	buf.Reset()
	err = operations.CopyURLToWriter(context.Background(), ts.URL, &buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Not Found")
	assert.Equal(t, 0, len(buf.String()))
}

func TestMoveFile(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	file1 := r.WriteFile("file1", "file1 contents", t1)
	fstest.CheckItems(t, r.Flocal, file1)

	file2 := file1
	file2.Path = "sub/file2"

	err := operations.MoveFile(context.Background(), r.Fremote, r.Flocal, file2.Path, file1.Path)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal)
	fstest.CheckItems(t, r.Fremote, file2)

	r.WriteFile("file1", "file1 contents", t1)
	fstest.CheckItems(t, r.Flocal, file1)

	err = operations.MoveFile(context.Background(), r.Fremote, r.Flocal, file2.Path, file1.Path)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal)
	fstest.CheckItems(t, r.Fremote, file2)

	err = operations.MoveFile(context.Background(), r.Fremote, r.Fremote, file2.Path, file2.Path)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal)
	fstest.CheckItems(t, r.Fremote, file2)
}

func TestCaseInsensitiveMoveFile(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	if !r.Fremote.Features().CaseInsensitive {
		return
	}

	file1 := r.WriteFile("file1", "file1 contents", t1)
	fstest.CheckItems(t, r.Flocal, file1)

	file2 := file1
	file2.Path = "sub/file2"

	err := operations.MoveFile(context.Background(), r.Fremote, r.Flocal, file2.Path, file1.Path)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal)
	fstest.CheckItems(t, r.Fremote, file2)

	r.WriteFile("file1", "file1 contents", t1)
	fstest.CheckItems(t, r.Flocal, file1)

	err = operations.MoveFile(context.Background(), r.Fremote, r.Flocal, file2.Path, file1.Path)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal)
	fstest.CheckItems(t, r.Fremote, file2)

	file2Capitalized := file2
	file2Capitalized.Path = "sub/File2"

	err = operations.MoveFile(context.Background(), r.Fremote, r.Fremote, file2Capitalized.Path, file2.Path)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal)
	fstest.CheckItems(t, r.Fremote, file2Capitalized)
}

func TestMoveFileBackupDir(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	if !operations.CanServerSideMove(r.Fremote) {
		t.Skip("Skipping test as remote does not support server side move or copy")
	}

	oldBackupDir := fs.Config.BackupDir
	fs.Config.BackupDir = r.FremoteName + "/backup"
	defer func() {
		fs.Config.BackupDir = oldBackupDir
	}()

	file1 := r.WriteFile("dst/file1", "file1 contents", t1)
	fstest.CheckItems(t, r.Flocal, file1)

	file1old := r.WriteObject(context.Background(), "dst/file1", "file1 contents old", t1)
	fstest.CheckItems(t, r.Fremote, file1old)

	err := operations.MoveFile(context.Background(), r.Fremote, r.Flocal, file1.Path, file1.Path)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal)
	file1old.Path = "backup/dst/file1"
	fstest.CheckItems(t, r.Fremote, file1old, file1)
}

func TestCopyFile(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	file1 := r.WriteFile("file1", "file1 contents", t1)
	fstest.CheckItems(t, r.Flocal, file1)

	file2 := file1
	file2.Path = "sub/file2"

	err := operations.CopyFile(context.Background(), r.Fremote, r.Flocal, file2.Path, file1.Path)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote, file2)

	err = operations.CopyFile(context.Background(), r.Fremote, r.Flocal, file2.Path, file1.Path)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote, file2)

	err = operations.CopyFile(context.Background(), r.Fremote, r.Fremote, file2.Path, file2.Path)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote, file2)
}

func TestCopyFileBackupDir(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	if !operations.CanServerSideMove(r.Fremote) {
		t.Skip("Skipping test as remote does not support server side move or copy")
	}

	oldBackupDir := fs.Config.BackupDir
	fs.Config.BackupDir = r.FremoteName + "/backup"
	defer func() {
		fs.Config.BackupDir = oldBackupDir
	}()

	file1 := r.WriteFile("dst/file1", "file1 contents", t1)
	fstest.CheckItems(t, r.Flocal, file1)

	file1old := r.WriteObject(context.Background(), "dst/file1", "file1 contents old", t1)
	fstest.CheckItems(t, r.Fremote, file1old)

	err := operations.CopyFile(context.Background(), r.Fremote, r.Flocal, file1.Path, file1.Path)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal, file1)
	file1old.Path = "backup/dst/file1"
	fstest.CheckItems(t, r.Fremote, file1old, file1)
}

// Test with CompareDest set
func TestCopyFileCompareDest(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	fs.Config.CompareDest = r.FremoteName + "/CompareDest"
	defer func() {
		fs.Config.CompareDest = ""
	}()
	fdst, err := fs.NewFs(r.FremoteName + "/dst")
	require.NoError(t, err)

	// check empty dest, empty compare
	file1 := r.WriteFile("one", "one", t1)
	fstest.CheckItems(t, r.Flocal, file1)

	err = operations.CopyFile(context.Background(), fdst, r.Flocal, file1.Path, file1.Path)
	require.NoError(t, err)

	file1dst := file1
	file1dst.Path = "dst/one"

	fstest.CheckItems(t, r.Fremote, file1dst)

	// check old dest, empty compare
	file1b := r.WriteFile("one", "onet2", t2)
	fstest.CheckItems(t, r.Fremote, file1dst)
	fstest.CheckItems(t, r.Flocal, file1b)

	err = operations.CopyFile(context.Background(), fdst, r.Flocal, file1b.Path, file1b.Path)
	require.NoError(t, err)

	file1bdst := file1b
	file1bdst.Path = "dst/one"

	fstest.CheckItems(t, r.Fremote, file1bdst)

	// check old dest, new compare
	file3 := r.WriteObject(context.Background(), "dst/one", "one", t1)
	file2 := r.WriteObject(context.Background(), "CompareDest/one", "onet2", t2)
	file1c := r.WriteFile("one", "onet2", t2)
	fstest.CheckItems(t, r.Fremote, file2, file3)
	fstest.CheckItems(t, r.Flocal, file1c)

	err = operations.CopyFile(context.Background(), fdst, r.Flocal, file1c.Path, file1c.Path)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Fremote, file2, file3)

	// check empty dest, new compare
	file4 := r.WriteObject(context.Background(), "CompareDest/two", "two", t2)
	file5 := r.WriteFile("two", "two", t2)
	fstest.CheckItems(t, r.Fremote, file2, file3, file4)
	fstest.CheckItems(t, r.Flocal, file1c, file5)

	err = operations.CopyFile(context.Background(), fdst, r.Flocal, file5.Path, file5.Path)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Fremote, file2, file3, file4)

	// check new dest, new compare
	err = operations.CopyFile(context.Background(), fdst, r.Flocal, file5.Path, file5.Path)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Fremote, file2, file3, file4)

	// check empty dest, old compare
	file5b := r.WriteFile("two", "twot3", t3)
	fstest.CheckItems(t, r.Fremote, file2, file3, file4)
	fstest.CheckItems(t, r.Flocal, file1c, file5b)

	err = operations.CopyFile(context.Background(), fdst, r.Flocal, file5b.Path, file5b.Path)
	require.NoError(t, err)

	file5bdst := file5b
	file5bdst.Path = "dst/two"

	fstest.CheckItems(t, r.Fremote, file2, file3, file4, file5bdst)
}

// Test with CopyDest set
func TestCopyFileCopyDest(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	if r.Fremote.Features().Copy == nil {
		t.Skip("Skipping test as remote does not support server side copy")
	}

	fs.Config.CopyDest = r.FremoteName + "/CopyDest"
	defer func() {
		fs.Config.CopyDest = ""
	}()

	fdst, err := fs.NewFs(r.FremoteName + "/dst")
	require.NoError(t, err)

	// check empty dest, empty copy
	file1 := r.WriteFile("one", "one", t1)
	fstest.CheckItems(t, r.Flocal, file1)

	err = operations.CopyFile(context.Background(), fdst, r.Flocal, file1.Path, file1.Path)
	require.NoError(t, err)

	file1dst := file1
	file1dst.Path = "dst/one"

	fstest.CheckItems(t, r.Fremote, file1dst)

	// check old dest, empty copy
	file1b := r.WriteFile("one", "onet2", t2)
	fstest.CheckItems(t, r.Fremote, file1dst)
	fstest.CheckItems(t, r.Flocal, file1b)

	err = operations.CopyFile(context.Background(), fdst, r.Flocal, file1b.Path, file1b.Path)
	require.NoError(t, err)

	file1bdst := file1b
	file1bdst.Path = "dst/one"

	fstest.CheckItems(t, r.Fremote, file1bdst)

	// check old dest, new copy, backup-dir

	fs.Config.BackupDir = r.FremoteName + "/BackupDir"

	file3 := r.WriteObject(context.Background(), "dst/one", "one", t1)
	file2 := r.WriteObject(context.Background(), "CopyDest/one", "onet2", t2)
	file1c := r.WriteFile("one", "onet2", t2)
	fstest.CheckItems(t, r.Fremote, file2, file3)
	fstest.CheckItems(t, r.Flocal, file1c)

	err = operations.CopyFile(context.Background(), fdst, r.Flocal, file1c.Path, file1c.Path)
	require.NoError(t, err)

	file2dst := file2
	file2dst.Path = "dst/one"
	file3.Path = "BackupDir/one"

	fstest.CheckItems(t, r.Fremote, file2, file2dst, file3)
	fs.Config.BackupDir = ""

	// check empty dest, new copy
	file4 := r.WriteObject(context.Background(), "CopyDest/two", "two", t2)
	file5 := r.WriteFile("two", "two", t2)
	fstest.CheckItems(t, r.Fremote, file2, file2dst, file3, file4)
	fstest.CheckItems(t, r.Flocal, file1c, file5)

	err = operations.CopyFile(context.Background(), fdst, r.Flocal, file5.Path, file5.Path)
	require.NoError(t, err)

	file4dst := file4
	file4dst.Path = "dst/two"

	fstest.CheckItems(t, r.Fremote, file2, file2dst, file3, file4, file4dst)

	// check new dest, new copy
	err = operations.CopyFile(context.Background(), fdst, r.Flocal, file5.Path, file5.Path)
	require.NoError(t, err)

	fstest.CheckItems(t, r.Fremote, file2, file2dst, file3, file4, file4dst)

	// check empty dest, old copy
	file6 := r.WriteObject(context.Background(), "CopyDest/three", "three", t2)
	file7 := r.WriteFile("three", "threet3", t3)
	fstest.CheckItems(t, r.Fremote, file2, file2dst, file3, file4, file4dst, file6)
	fstest.CheckItems(t, r.Flocal, file1c, file5, file7)

	err = operations.CopyFile(context.Background(), fdst, r.Flocal, file7.Path, file7.Path)
	require.NoError(t, err)

	file7dst := file7
	file7dst.Path = "dst/three"

	fstest.CheckItems(t, r.Fremote, file2, file2dst, file3, file4, file4dst, file6, file7dst)
}

// testFsInfo is for unit testing fs.Info
type testFsInfo struct {
	name      string
	root      string
	stringVal string
	precision time.Duration
	hashes    hash.Set
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
func (i *testFsInfo) Hashes() hash.Set { return i.hashes }

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
		actual := operations.SameConfig(a, b)
		assert.Equal(t, test.expected, actual)
		actual = operations.SameConfig(b, a)
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
		actual := operations.Same(a, b)
		assert.Equal(t, test.expected, actual)
		actual = operations.Same(b, a)
		assert.Equal(t, test.expected, actual)
	}
}

func TestOverlapping(t *testing.T) {
	a := &testFsInfo{name: "name", root: "root"}
	slash := string(os.PathSeparator) // native path separator
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
		{"name", "root" + slash + "toot", true},
		{"name", "root" + slash + "toot" + slash, true},
		{"name", "", true},
		{"name", "/", true},
	} {
		b := &testFsInfo{name: test.name, root: test.root}
		what := fmt.Sprintf("(%q,%q) vs (%q,%q)", a.name, a.root, b.name, b.root)
		actual := operations.Overlapping(a, b)
		assert.Equal(t, test.expected, actual, what)
		actual = operations.Overlapping(b, a)
		assert.Equal(t, test.expected, actual, what)
	}
}

func TestListFormat(t *testing.T) {
	item0 := &operations.ListJSONItem{
		Path:      "a",
		Name:      "a",
		Encrypted: "encryptedFileName",
		Size:      1,
		MimeType:  "application/octet-stream",
		ModTime: operations.Timestamp{
			When:   t1,
			Format: "2006-01-02T15:04:05.000000000Z07:00"},
		IsDir: false,
		Hashes: map[string]string{
			"MD5":          "0cc175b9c0f1b6a831c399e269772661",
			"SHA-1":        "86f7e437faa5a7fce15d1ddcb9eaeaea377667b8",
			"DropboxHash":  "bf5d3affb73efd2ec6c36ad3112dd933efed63c4e1cbffcfa88e2759c144f2d8",
			"QuickXorHash": "6100000000000000000000000100000000000000"},
		ID:     "fileID",
		OrigID: "fileOrigID",
	}

	item1 := &operations.ListJSONItem{
		Path:      "subdir",
		Name:      "subdir",
		Encrypted: "encryptedDirName",
		Size:      -1,
		MimeType:  "inode/directory",
		ModTime: operations.Timestamp{
			When:   t2,
			Format: "2006-01-02T15:04:05.000000000Z07:00"},
		IsDir:  true,
		Hashes: map[string]string(nil),
		ID:     "dirID",
		OrigID: "dirOrigID",
	}

	var list operations.ListFormat
	list.AddPath()
	list.SetDirSlash(false)
	assert.Equal(t, "subdir", list.Format(item1))

	list.SetDirSlash(true)
	assert.Equal(t, "subdir/", list.Format(item1))

	list.SetOutput(nil)
	assert.Equal(t, "", list.Format(item1))

	list.AppendOutput(func(item *operations.ListJSONItem) string { return "a" })
	list.AppendOutput(func(item *operations.ListJSONItem) string { return "b" })
	assert.Equal(t, "ab", list.Format(item1))
	list.SetSeparator(":::")
	assert.Equal(t, "a:::b", list.Format(item1))

	list.SetOutput(nil)
	list.AddModTime()
	assert.Equal(t, t1.Local().Format("2006-01-02 15:04:05"), list.Format(item0))

	list.SetOutput(nil)
	list.SetSeparator("|")
	list.AddID()
	list.AddOrigID()
	assert.Equal(t, "fileID|fileOrigID", list.Format(item0))
	assert.Equal(t, "dirID|dirOrigID", list.Format(item1))

	list.SetOutput(nil)
	list.AddMimeType()
	assert.Contains(t, list.Format(item0), "/")
	assert.Equal(t, "inode/directory", list.Format(item1))

	list.SetOutput(nil)
	list.AddPath()
	list.SetAbsolute(true)
	assert.Equal(t, "/a", list.Format(item0))
	list.SetAbsolute(false)
	assert.Equal(t, "a", list.Format(item0))

	list.SetOutput(nil)
	list.AddSize()
	assert.Equal(t, "1", list.Format(item0))

	list.AddPath()
	list.AddModTime()
	list.SetDirSlash(true)
	list.SetSeparator("__SEP__")
	assert.Equal(t, "1__SEP__a__SEP__"+t1.Local().Format("2006-01-02 15:04:05"), list.Format(item0))
	assert.Equal(t, "-1__SEP__subdir/__SEP__"+t2.Local().Format("2006-01-02 15:04:05"), list.Format(item1))

	for _, test := range []struct {
		ht   hash.Type
		want string
	}{
		{hash.MD5, "0cc175b9c0f1b6a831c399e269772661"},
		{hash.SHA1, "86f7e437faa5a7fce15d1ddcb9eaeaea377667b8"},
	} {
		list.SetOutput(nil)
		list.AddHash(test.ht)
		assert.Equal(t, test.want, list.Format(item0))
	}

	list.SetOutput(nil)
	list.SetSeparator("|")
	list.SetCSV(true)
	list.AddSize()
	list.AddPath()
	list.AddModTime()
	list.SetDirSlash(true)
	assert.Equal(t, "1|a|"+t1.Local().Format("2006-01-02 15:04:05"), list.Format(item0))
	assert.Equal(t, "-1|subdir/|"+t2.Local().Format("2006-01-02 15:04:05"), list.Format(item1))

	list.SetOutput(nil)
	list.SetSeparator("|")
	list.AddPath()
	list.AddEncrypted()
	assert.Equal(t, "a|encryptedFileName", list.Format(item0))
	assert.Equal(t, "subdir/|encryptedDirName/", list.Format(item1))

}

func TestDirMove(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	r.Mkdir(context.Background(), r.Fremote)

	// Make some files and dirs
	r.ForceMkdir(context.Background(), r.Fremote)
	files := []fstest.Item{
		r.WriteObject(context.Background(), "A1/one", "one", t1),
		r.WriteObject(context.Background(), "A1/two", "two", t2),
		r.WriteObject(context.Background(), "A1/B1/three", "three", t3),
		r.WriteObject(context.Background(), "A1/B1/C1/four", "four", t1),
		r.WriteObject(context.Background(), "A1/B1/C2/five", "five", t2),
	}
	require.NoError(t, operations.Mkdir(context.Background(), r.Fremote, "A1/B2"))
	require.NoError(t, operations.Mkdir(context.Background(), r.Fremote, "A1/B1/C3"))

	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		files,
		[]string{
			"A1",
			"A1/B1",
			"A1/B2",
			"A1/B1/C1",
			"A1/B1/C2",
			"A1/B1/C3",
		},
		fs.GetModifyWindow(r.Fremote),
	)

	require.NoError(t, operations.DirMove(context.Background(), r.Fremote, "A1", "A2"))

	for i := range files {
		files[i].Path = strings.Replace(files[i].Path, "A1/", "A2/", -1)
	}

	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		files,
		[]string{
			"A2",
			"A2/B1",
			"A2/B2",
			"A2/B1/C1",
			"A2/B1/C2",
			"A2/B1/C3",
		},
		fs.GetModifyWindow(r.Fremote),
	)

	// Disable DirMove
	features := r.Fremote.Features()
	oldDirMove := features.DirMove
	features.DirMove = nil
	defer func() {
		features.DirMove = oldDirMove
	}()

	require.NoError(t, operations.DirMove(context.Background(), r.Fremote, "A2", "A3"))

	for i := range files {
		files[i].Path = strings.Replace(files[i].Path, "A2/", "A3/", -1)
	}

	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		files,
		[]string{
			"A3",
			"A3/B1",
			"A3/B2",
			"A3/B1/C1",
			"A3/B1/C2",
			"A3/B1/C3",
		},
		fs.GetModifyWindow(r.Fremote),
	)

}

func TestGetFsInfo(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	f := r.Fremote
	info := operations.GetFsInfo(f)
	assert.Equal(t, f.Name(), info.Name)
	assert.Equal(t, f.Root(), info.Root)
	assert.Equal(t, f.String(), info.String)
	assert.Equal(t, f.Precision(), info.Precision)
	hashSet := hash.NewHashSet()
	for _, hashName := range info.Hashes {
		var ht hash.Type
		require.NoError(t, ht.Set(hashName))
		hashSet.Add(ht)
	}
	assert.Equal(t, f.Hashes(), hashSet)
	assert.Equal(t, f.Features().Enabled(), info.Features)
}

func TestRcat(t *testing.T) {
	check := func(withChecksum, ignoreChecksum bool) {
		checksumBefore, ignoreChecksumBefore := fs.Config.CheckSum, fs.Config.IgnoreChecksum
		fs.Config.CheckSum, fs.Config.IgnoreChecksum = withChecksum, ignoreChecksum
		defer func() {
			fs.Config.CheckSum, fs.Config.IgnoreChecksum = checksumBefore, ignoreChecksumBefore
		}()

		var prefix string
		if withChecksum {
			prefix = "with_checksum_"
		} else {
			prefix = "no_checksum_"
		}
		if ignoreChecksum {
			prefix = "ignore_checksum_"
		}

		r := fstest.NewRun(t)
		defer r.Finalise()

		if *fstest.SizeLimit > 0 && int64(fs.Config.StreamingUploadCutoff) > *fstest.SizeLimit {
			savedCutoff := fs.Config.StreamingUploadCutoff
			defer func() {
				fs.Config.StreamingUploadCutoff = savedCutoff
			}()
			fs.Config.StreamingUploadCutoff = fs.SizeSuffix(*fstest.SizeLimit)
			t.Logf("Adjust StreamingUploadCutoff to size limit %s (was %s)", fs.Config.StreamingUploadCutoff, savedCutoff)
		}

		fstest.CheckListing(t, r.Fremote, []fstest.Item{})

		data1 := "this is some really nice test data"
		path1 := prefix + "small_file_from_pipe"

		data2 := string(make([]byte, fs.Config.StreamingUploadCutoff+1))
		path2 := prefix + "big_file_from_pipe"

		in := ioutil.NopCloser(strings.NewReader(data1))
		_, err := operations.Rcat(context.Background(), r.Fremote, path1, in, t1)
		require.NoError(t, err)

		in = ioutil.NopCloser(strings.NewReader(data2))
		_, err = operations.Rcat(context.Background(), r.Fremote, path2, in, t2)
		require.NoError(t, err)

		file1 := fstest.NewItem(path1, data1, t1)
		file2 := fstest.NewItem(path2, data2, t2)
		fstest.CheckItems(t, r.Fremote, file1, file2)
	}

	for i := 0; i < 4; i++ {
		withChecksum := (i & 1) != 0
		ignoreChecksum := (i & 2) != 0
		t.Run(fmt.Sprintf("withChecksum=%v,ignoreChecksum=%v", withChecksum, ignoreChecksum), func(t *testing.T) {
			check(withChecksum, ignoreChecksum)
		})
	}
}

func TestRcatSize(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	const body = "------------------------------------------------------------"
	file1 := r.WriteFile("potato1", body, t1)
	file2 := r.WriteFile("potato2", body, t2)
	// Test with known length
	bodyReader := ioutil.NopCloser(strings.NewReader(body))
	obj, err := operations.RcatSize(context.Background(), r.Fremote, file1.Path, bodyReader, int64(len(body)), file1.ModTime)
	require.NoError(t, err)
	assert.Equal(t, int64(len(body)), obj.Size())
	assert.Equal(t, file1.Path, obj.Remote())

	// Test with unknown length
	bodyReader = ioutil.NopCloser(strings.NewReader(body)) // reset Reader
	ioutil.NopCloser(strings.NewReader(body))
	obj, err = operations.RcatSize(context.Background(), r.Fremote, file2.Path, bodyReader, -1, file2.ModTime)
	require.NoError(t, err)
	assert.Equal(t, int64(len(body)), obj.Size())
	assert.Equal(t, file2.Path, obj.Remote())

	// Check files exist
	fstest.CheckItems(t, r.Fremote, file1, file2)
}

func TestCopyFileMaxTransfer(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	old := fs.Config.MaxTransfer
	oldMode := fs.Config.CutoffMode

	defer func() {
		fs.Config.MaxTransfer = old
		fs.Config.CutoffMode = oldMode
		accounting.Stats(context.Background()).ResetCounters()
	}()

	ctx := context.Background()

	const sizeCutoff = 2048
	file1 := r.WriteFile("TestCopyFileMaxTransfer/file1", "file1 contents", t1)
	file2 := r.WriteFile("TestCopyFileMaxTransfer/file2", "file2 contents"+random.String(sizeCutoff), t2)
	file3 := r.WriteFile("TestCopyFileMaxTransfer/file3", "file3 contents"+random.String(sizeCutoff), t2)
	file4 := r.WriteFile("TestCopyFileMaxTransfer/file4", "file4 contents"+random.String(sizeCutoff), t2)

	// Cutoff mode: Hard
	fs.Config.MaxTransfer = sizeCutoff
	fs.Config.CutoffMode = fs.CutoffModeHard

	// file1: Show a small file gets transferred OK
	accounting.Stats(ctx).ResetCounters()
	err := operations.CopyFile(ctx, r.Fremote, r.Flocal, file1.Path, file1.Path)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal, file1, file2, file3, file4)
	fstest.CheckItems(t, r.Fremote, file1)

	// file2: show a large file does not get transferred
	accounting.Stats(ctx).ResetCounters()
	err = operations.CopyFile(ctx, r.Fremote, r.Flocal, file2.Path, file2.Path)
	require.NotNil(t, err, "Did not get expected max transfer limit error")
	assert.Contains(t, err.Error(), "Max transfer limit reached")
	assert.True(t, fserrors.IsFatalError(err))
	fstest.CheckItems(t, r.Flocal, file1, file2, file3, file4)
	fstest.CheckItems(t, r.Fremote, file1)

	// Cutoff mode: Cautious
	fs.Config.CutoffMode = fs.CutoffModeCautious

	// file3: show a large file does not get transferred
	accounting.Stats(ctx).ResetCounters()
	err = operations.CopyFile(ctx, r.Fremote, r.Flocal, file3.Path, file3.Path)
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "Max transfer limit reached")
	assert.True(t, fserrors.IsFatalError(err))
	fstest.CheckItems(t, r.Flocal, file1, file2, file3, file4)
	fstest.CheckItems(t, r.Fremote, file1)

	if strings.HasPrefix(r.Fremote.Name(), "TestChunker") {
		t.Log("skipping remainder of test for chunker as it involves multiple transfers")
		return
	}

	// Cutoff mode: Soft
	fs.Config.CutoffMode = fs.CutoffModeSoft

	// file4: show a large file does get transferred this time
	accounting.Stats(ctx).ResetCounters()
	err = operations.CopyFile(ctx, r.Fremote, r.Flocal, file4.Path, file4.Path)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal, file1, file2, file3, file4)
	fstest.CheckItems(t, r.Fremote, file1, file4)
}
