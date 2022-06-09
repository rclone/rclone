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
	"crypto/rand"
	"errors"
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
	"github.com/rclone/rclone/fstest/fstests"
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
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()

	err := operations.Mkdir(ctx, r.Fremote, "")
	require.NoError(t, err)
	fstest.CheckListing(t, r.Fremote, []fstest.Item{})

	err = operations.Mkdir(ctx, r.Fremote, "")
	require.NoError(t, err)
}

func TestLsd(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject(ctx, "sub dir/hello world", "hello world", t1)

	r.CheckRemoteItems(t, file1)

	var buf bytes.Buffer
	err := operations.ListDir(ctx, r.Fremote, &buf)
	require.NoError(t, err)
	res := buf.String()
	assert.Contains(t, res, "sub dir\n")
}

func TestLs(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth(ctx, "potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth(ctx, "empty space", "-", t2)

	r.CheckRemoteItems(t, file1, file2)

	var buf bytes.Buffer
	err := operations.List(ctx, r.Fremote, &buf)
	require.NoError(t, err)
	res := buf.String()
	assert.Contains(t, res, "        1 empty space\n")
	assert.Contains(t, res, "       60 potato2\n")
}

func TestLsWithFilesFrom(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth(ctx, "potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth(ctx, "empty space", "-", t2)

	r.CheckRemoteItems(t, file1, file2)

	// Set the --files-from equivalent
	f, err := filter.NewFilter(nil)
	require.NoError(t, err)
	require.NoError(t, f.AddFile("potato2"))
	require.NoError(t, f.AddFile("notfound"))

	// Change the active filter
	ctx = filter.ReplaceConfig(ctx, f)

	var buf bytes.Buffer
	err = operations.List(ctx, r.Fremote, &buf)
	require.NoError(t, err)
	assert.Equal(t, "       60 potato2\n", buf.String())

	// Now try with --no-traverse
	ci.NoTraverse = true

	buf.Reset()
	err = operations.List(ctx, r.Fremote, &buf)
	require.NoError(t, err)
	assert.Equal(t, "       60 potato2\n", buf.String())
}

func TestLsLong(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth(ctx, "potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth(ctx, "empty space", "-", t2)

	r.CheckRemoteItems(t, file1, file2)

	var buf bytes.Buffer
	err := operations.ListLong(ctx, r.Fremote, &buf)
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
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth(ctx, "potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth(ctx, "empty space", "-", t2)

	r.CheckRemoteItems(t, file1, file2)

	hashes := r.Fremote.Hashes()

	var quickXorHash hash.Type
	err := quickXorHash.Set("QuickXorHash")
	require.NoError(t, err)

	for _, test := range []struct {
		name     string
		download bool
		base64   bool
		ht       hash.Type
		want     []string
	}{
		{
			ht: hash.MD5,
			want: []string{
				"336d5ebc5436534e61d16e63ddfca327  empty space\n",
				"d6548b156ea68a4e003e786df99eee76  potato2\n",
			},
		},
		{
			ht:       hash.MD5,
			download: true,
			want: []string{
				"336d5ebc5436534e61d16e63ddfca327  empty space\n",
				"d6548b156ea68a4e003e786df99eee76  potato2\n",
			},
		},
		{
			ht: hash.SHA1,
			want: []string{
				"3bc15c8aae3e4124dd409035f32ea2fd6835efc9  empty space\n",
				"9dc7f7d3279715991a22853f5981df582b7f9f6d  potato2\n",
			},
		},
		{
			ht:       hash.SHA1,
			download: true,
			want: []string{
				"3bc15c8aae3e4124dd409035f32ea2fd6835efc9  empty space\n",
				"9dc7f7d3279715991a22853f5981df582b7f9f6d  potato2\n",
			},
		},
		{
			ht: quickXorHash,
			want: []string{
				"2d00000000000000000000000100000000000000  empty space\n",
				"4001dad296b6b4a52d6d694b67dad296b6b4a52d  potato2\n",
			},
		},
		{
			ht:       quickXorHash,
			download: true,
			want: []string{
				"2d00000000000000000000000100000000000000  empty space\n",
				"4001dad296b6b4a52d6d694b67dad296b6b4a52d  potato2\n",
			},
		},
		{
			ht:     quickXorHash,
			base64: true,
			want: []string{
				"LQAAAAAAAAAAAAAAAQAAAAAAAAA=  empty space\n",
				"QAHa0pa2tKUtbWlLZ9rSlra0pS0=  potato2\n",
			},
		},
		{
			ht:       quickXorHash,
			base64:   true,
			download: true,
			want: []string{
				"LQAAAAAAAAAAAAAAAQAAAAAAAAA=  empty space\n",
				"QAHa0pa2tKUtbWlLZ9rSlra0pS0=  potato2\n",
			},
		},
	} {
		if !hashes.Contains(test.ht) {
			continue
		}
		name := strings.Title(test.ht.String())
		if test.download {
			name += "Download"
		}
		if test.base64 {
			name += "Base64"
		}
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			err := operations.HashLister(ctx, test.ht, test.base64, test.download, r.Fremote, &buf)
			require.NoError(t, err)
			res := buf.String()
			for _, line := range test.want {
				assert.Contains(t, res, line)
			}
		})
	}
}

func TestHashSumsWithErrors(t *testing.T) {
	ctx := context.Background()
	memFs, err := fs.NewFs(ctx, ":memory:")
	require.NoError(t, err)

	// Make a test file
	content := "-"
	item1 := fstest.NewItem("file1", content, t1)
	_, _ = fstests.PutTestContents(ctx, t, memFs, &item1, content, true)

	// MemoryFS supports MD5
	buf := &bytes.Buffer{}
	err = operations.HashLister(ctx, hash.MD5, false, false, memFs, buf)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "336d5ebc5436534e61d16e63ddfca327  file1\n")

	// MemoryFS can't do SHA1, but UNSUPPORTED must not appear in the output
	buf.Reset()
	err = operations.HashLister(ctx, hash.SHA1, false, false, memFs, buf)
	require.NoError(t, err)
	assert.NotContains(t, buf.String(), " UNSUPPORTED ")

	// ERROR must not appear in the output either
	assert.NotContains(t, buf.String(), " ERROR ")
	// TODO mock an unreadable file
}

func TestHashStream(t *testing.T) {
	reader := strings.NewReader("")
	in := ioutil.NopCloser(reader)
	out := &bytes.Buffer{}
	for _, test := range []struct {
		input      string
		ht         hash.Type
		wantHex    string
		wantBase64 string
	}{
		{
			input:      "",
			ht:         hash.MD5,
			wantHex:    "d41d8cd98f00b204e9800998ecf8427e  -\n",
			wantBase64: "1B2M2Y8AsgTpgAmY7PhCfg==  -\n",
		},
		{
			input:      "",
			ht:         hash.SHA1,
			wantHex:    "da39a3ee5e6b4b0d3255bfef95601890afd80709  -\n",
			wantBase64: "2jmj7l5rSw0yVb_vlWAYkK_YBwk=  -\n",
		},
		{
			input:      "Hello world!",
			ht:         hash.MD5,
			wantHex:    "86fb269d190d2c85f6e0468ceca42a20  -\n",
			wantBase64: "hvsmnRkNLIX24EaM7KQqIA==  -\n",
		},
		{
			input:      "Hello world!",
			ht:         hash.SHA1,
			wantHex:    "d3486ae9136e7856bc42212385ea797094475802  -\n",
			wantBase64: "00hq6RNueFa8QiEjhep5cJRHWAI=  -\n",
		},
	} {
		reader.Reset(test.input)
		require.NoError(t, operations.HashSumStream(test.ht, false, in, out))
		assert.Equal(t, test.wantHex, out.String())
		_, _ = reader.Seek(0, io.SeekStart)
		out.Reset()
		require.NoError(t, operations.HashSumStream(test.ht, true, in, out))
		assert.Equal(t, test.wantBase64, out.String())
		out.Reset()
	}
}

func TestSuffixName(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
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
		ci.Suffix = test.suffix
		ci.SuffixKeepExtension = test.keepExt
		got := operations.SuffixName(ctx, test.remote)
		assert.Equal(t, test.want, got, fmt.Sprintf("%+v", test))
	}
}

func TestCount(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth(ctx, "potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth(ctx, "empty space", "-", t2)
	file3 := r.WriteBoth(ctx, "sub dir/potato3", "hello", t2)

	r.CheckRemoteItems(t, file1, file2, file3)

	// Check the MaxDepth too
	ci.MaxDepth = 1

	objects, size, sizeless, err := operations.Count(ctx, r.Fremote)
	require.NoError(t, err)
	assert.Equal(t, int64(2), objects)
	assert.Equal(t, int64(61), size)
	assert.Equal(t, int64(0), sizeless)
}

func TestDelete(t *testing.T) {
	ctx := context.Background()
	fi, err := filter.NewFilter(nil)
	require.NoError(t, err)
	fi.Opt.MaxSize = 60
	ctx = filter.ReplaceConfig(ctx, fi)
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject(ctx, "small", "1234567890", t2)                                                                                           // 10 bytes
	file2 := r.WriteObject(ctx, "medium", "------------------------------------------------------------", t1)                                        // 60 bytes
	file3 := r.WriteObject(ctx, "large", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", t1) // 100 bytes
	r.CheckRemoteItems(t, file1, file2, file3)

	err = operations.Delete(ctx, r.Fremote)
	require.NoError(t, err)
	r.CheckRemoteItems(t, file3)
}

func TestRetry(t *testing.T) {
	ctx := context.Background()

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
	assert.Equal(t, nil, operations.Retry(ctx, nil, 5, fn))
	assert.Equal(t, 0, i)

	i, err = 10, io.EOF
	assert.Equal(t, io.EOF, operations.Retry(ctx, nil, 5, fn))
	assert.Equal(t, 5, i)

	i, err = 10, fs.ErrorObjectNotFound
	assert.Equal(t, fs.ErrorObjectNotFound, operations.Retry(ctx, nil, 5, fn))
	assert.Equal(t, 9, i)

}

func TestCat(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth(ctx, "file1", "ABCDEFGHIJ", t1)
	file2 := r.WriteBoth(ctx, "file2", "012345678", t2)

	r.CheckRemoteItems(t, file1, file2)

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
		err := operations.Cat(ctx, r.Fremote, &buf, test.offset, test.count)
		require.NoError(t, err)
		res := buf.String()

		if res != test.a+test.b && res != test.b+test.a {
			t.Errorf("Incorrect output from Cat(%d,%d): %q", test.offset, test.count, res)
		}
	}
}

func TestPurge(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRunIndividual(t) // make new container (azureblob has delayed mkdir after rmdir)
	defer r.Finalise()
	r.Mkdir(ctx, r.Fremote)

	// Make some files and dirs
	r.ForceMkdir(ctx, r.Fremote)
	file1 := r.WriteObject(ctx, "A1/B1/C1/one", "aaa", t1)
	//..and dirs we expect to delete
	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "A2"))
	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "A1/B2"))
	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "A1/B2/C2"))
	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "A1/B1/C3"))
	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "A3"))
	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "A3/B3"))
	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "A3/B3/C4"))
	//..and one more file at the end
	file2 := r.WriteObject(ctx, "A1/two", "bbb", t2)

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
		fs.GetModifyWindow(ctx, r.Fremote),
	)

	require.NoError(t, operations.Purge(ctx, r.Fremote, "A1/B1"))

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
		fs.GetModifyWindow(ctx, r.Fremote),
	)

	require.NoError(t, operations.Purge(ctx, r.Fremote, ""))

	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		[]fstest.Item{},
		[]string{},
		fs.GetModifyWindow(ctx, r.Fremote),
	)

}

func TestRmdirsNoLeaveRoot(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	r.Mkdir(ctx, r.Fremote)

	// Make some files and dirs we expect to keep
	r.ForceMkdir(ctx, r.Fremote)
	file1 := r.WriteObject(ctx, "A1/B1/C1/one", "aaa", t1)
	//..and dirs we expect to delete
	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "A2"))
	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "A1/B2"))
	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "A1/B2/C2"))
	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "A1/B1/C3"))
	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "A3"))
	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "A3/B3"))
	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "A3/B3/C4"))
	//..and one more file at the end
	file2 := r.WriteObject(ctx, "A1/two", "bbb", t2)

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
		fs.GetModifyWindow(ctx, r.Fremote),
	)

	require.NoError(t, operations.Rmdirs(ctx, r.Fremote, "A3/B3/C4", false))

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
		fs.GetModifyWindow(ctx, r.Fremote),
	)

	require.NoError(t, operations.Rmdirs(ctx, r.Fremote, "", false))

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
		fs.GetModifyWindow(ctx, r.Fremote),
	)

}

func TestRmdirsLeaveRoot(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	r.Mkdir(ctx, r.Fremote)

	r.ForceMkdir(ctx, r.Fremote)

	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "A1"))
	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "A1/B1"))
	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "A1/B1/C1"))

	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		[]fstest.Item{},
		[]string{
			"A1",
			"A1/B1",
			"A1/B1/C1",
		},
		fs.GetModifyWindow(ctx, r.Fremote),
	)

	require.NoError(t, operations.Rmdirs(ctx, r.Fremote, "A1", true))

	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		[]fstest.Item{},
		[]string{
			"A1",
		},
		fs.GetModifyWindow(ctx, r.Fremote),
	)
}

func TestRmdirsWithFilter(t *testing.T) {
	ctx := context.Background()
	ctx, fi := filter.AddConfig(ctx)
	require.NoError(t, fi.AddRule("+ /A1/B1/**"))
	require.NoError(t, fi.AddRule("- *"))
	r := fstest.NewRun(t)
	defer r.Finalise()
	r.Mkdir(ctx, r.Fremote)

	r.ForceMkdir(ctx, r.Fremote)

	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "A1"))
	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "A1/B1"))
	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "A1/B1/C1"))

	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		[]fstest.Item{},
		[]string{
			"A1",
			"A1/B1",
			"A1/B1/C1",
		},
		fs.GetModifyWindow(ctx, r.Fremote),
	)

	require.NoError(t, operations.Rmdirs(ctx, r.Fremote, "", false))

	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		[]fstest.Item{},
		[]string{
			"A1",
		},
		fs.GetModifyWindow(ctx, r.Fremote),
	)
}

func TestCopyURL(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()

	contents := "file contents\n"
	file1 := r.WriteFile("file1", contents, t1)
	file2 := r.WriteFile("file2", contents, t1)
	r.Mkdir(ctx, r.Fremote)
	r.CheckRemoteItems(t)

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

	o, err := operations.CopyURL(ctx, r.Fremote, "file1", ts.URL, false, false)
	require.NoError(t, err)
	assert.Equal(t, int64(len(contents)), o.Size())

	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1}, nil, fs.ModTimeNotSupported)

	// Check file clobbering
	_, err = operations.CopyURL(ctx, r.Fremote, "file1", ts.URL, false, true)
	require.Error(t, err)

	// Check auto file naming
	status = 0
	urlFileName := "filename.txt"
	o, err = operations.CopyURL(ctx, r.Fremote, "", ts.URL+"/"+urlFileName, true, false)
	require.NoError(t, err)
	assert.Equal(t, int64(len(contents)), o.Size())
	assert.Equal(t, urlFileName, o.Remote())

	// Check auto file naming when url without file name
	_, err = operations.CopyURL(ctx, r.Fremote, "file1", ts.URL, true, false)
	require.Error(t, err)

	// Check an error is returned for a 404
	status = http.StatusNotFound
	o, err = operations.CopyURL(ctx, r.Fremote, "file1", ts.URL, false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Not Found")
	assert.Nil(t, o)
	status = 0

	// check when reading from unverified HTTPS server
	ci.InsecureSkipVerify = true
	fshttp.ResetTransport()
	defer fshttp.ResetTransport()
	tss := httptest.NewTLSServer(handler)
	defer tss.Close()

	o, err = operations.CopyURL(ctx, r.Fremote, "file2", tss.URL, false, false)
	require.NoError(t, err)
	assert.Equal(t, int64(len(contents)), o.Size())
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1, file2, fstest.NewItem(urlFileName, contents, t1)}, nil, fs.ModTimeNotSupported)
}

func TestCopyURLToWriter(t *testing.T) {
	ctx := context.Background()
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
	err := operations.CopyURLToWriter(ctx, ts.URL, &buf)
	require.NoError(t, err)
	assert.Equal(t, contents, buf.String())

	// test fetch with error
	status = http.StatusNotFound
	buf.Reset()
	err = operations.CopyURLToWriter(ctx, ts.URL, &buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Not Found")
	assert.Equal(t, 0, len(buf.String()))
}

func TestMoveFile(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()

	file1 := r.WriteFile("file1", "file1 contents", t1)
	r.CheckLocalItems(t, file1)

	file2 := file1
	file2.Path = "sub/file2"

	err := operations.MoveFile(ctx, r.Fremote, r.Flocal, file2.Path, file1.Path)
	require.NoError(t, err)
	r.CheckLocalItems(t)
	r.CheckRemoteItems(t, file2)

	r.WriteFile("file1", "file1 contents", t1)
	r.CheckLocalItems(t, file1)

	err = operations.MoveFile(ctx, r.Fremote, r.Flocal, file2.Path, file1.Path)
	require.NoError(t, err)
	r.CheckLocalItems(t)
	r.CheckRemoteItems(t, file2)

	err = operations.MoveFile(ctx, r.Fremote, r.Fremote, file2.Path, file2.Path)
	require.NoError(t, err)
	r.CheckLocalItems(t)
	r.CheckRemoteItems(t, file2)
}

func TestMoveFileWithIgnoreExisting(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteFile("file1", "file1 contents", t1)
	r.CheckLocalItems(t, file1)

	ci.IgnoreExisting = true

	err := operations.MoveFile(ctx, r.Fremote, r.Flocal, file1.Path, file1.Path)
	require.NoError(t, err)
	r.CheckLocalItems(t)
	r.CheckRemoteItems(t, file1)

	// Recreate file with updated content
	file1b := r.WriteFile("file1", "file1 modified", t2)
	r.CheckLocalItems(t, file1b)

	// Ensure modified file did not transfer and was not deleted
	err = operations.MoveFile(ctx, r.Fremote, r.Flocal, file1.Path, file1b.Path)
	require.NoError(t, err)
	r.CheckLocalItems(t, file1b)
	r.CheckRemoteItems(t, file1)
}

func TestCaseInsensitiveMoveFile(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()
	if !r.Fremote.Features().CaseInsensitive {
		return
	}

	file1 := r.WriteFile("file1", "file1 contents", t1)
	r.CheckLocalItems(t, file1)

	file2 := file1
	file2.Path = "sub/file2"

	err := operations.MoveFile(ctx, r.Fremote, r.Flocal, file2.Path, file1.Path)
	require.NoError(t, err)
	r.CheckLocalItems(t)
	r.CheckRemoteItems(t, file2)

	r.WriteFile("file1", "file1 contents", t1)
	r.CheckLocalItems(t, file1)

	err = operations.MoveFile(ctx, r.Fremote, r.Flocal, file2.Path, file1.Path)
	require.NoError(t, err)
	r.CheckLocalItems(t)
	r.CheckRemoteItems(t, file2)

	file2Capitalized := file2
	file2Capitalized.Path = "sub/File2"

	err = operations.MoveFile(ctx, r.Fremote, r.Fremote, file2Capitalized.Path, file2.Path)
	require.NoError(t, err)
	r.CheckLocalItems(t)
	r.CheckRemoteItems(t, file2Capitalized)
}

func TestMoveFileBackupDir(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()
	if !operations.CanServerSideMove(r.Fremote) {
		t.Skip("Skipping test as remote does not support server-side move or copy")
	}

	ci.BackupDir = r.FremoteName + "/backup"

	file1 := r.WriteFile("dst/file1", "file1 contents", t1)
	r.CheckLocalItems(t, file1)

	file1old := r.WriteObject(ctx, "dst/file1", "file1 contents old", t1)
	r.CheckRemoteItems(t, file1old)

	err := operations.MoveFile(ctx, r.Fremote, r.Flocal, file1.Path, file1.Path)
	require.NoError(t, err)
	r.CheckLocalItems(t)
	file1old.Path = "backup/dst/file1"
	r.CheckRemoteItems(t, file1old, file1)
}

func TestCopyFile(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()

	file1 := r.WriteFile("file1", "file1 contents", t1)
	r.CheckLocalItems(t, file1)

	file2 := file1
	file2.Path = "sub/file2"

	err := operations.CopyFile(ctx, r.Fremote, r.Flocal, file2.Path, file1.Path)
	require.NoError(t, err)
	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file2)

	err = operations.CopyFile(ctx, r.Fremote, r.Flocal, file2.Path, file1.Path)
	require.NoError(t, err)
	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file2)

	err = operations.CopyFile(ctx, r.Fremote, r.Fremote, file2.Path, file2.Path)
	require.NoError(t, err)
	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file2)
}

func TestCopyFileBackupDir(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()
	if !operations.CanServerSideMove(r.Fremote) {
		t.Skip("Skipping test as remote does not support server-side move or copy")
	}

	ci.BackupDir = r.FremoteName + "/backup"

	file1 := r.WriteFile("dst/file1", "file1 contents", t1)
	r.CheckLocalItems(t, file1)

	file1old := r.WriteObject(ctx, "dst/file1", "file1 contents old", t1)
	r.CheckRemoteItems(t, file1old)

	err := operations.CopyFile(ctx, r.Fremote, r.Flocal, file1.Path, file1.Path)
	require.NoError(t, err)
	r.CheckLocalItems(t, file1)
	file1old.Path = "backup/dst/file1"
	r.CheckRemoteItems(t, file1old, file1)
}

// Test with CompareDest set
func TestCopyFileCompareDest(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()

	ci.CompareDest = []string{r.FremoteName + "/CompareDest"}
	fdst, err := fs.NewFs(ctx, r.FremoteName+"/dst")
	require.NoError(t, err)

	// check empty dest, empty compare
	file1 := r.WriteFile("one", "one", t1)
	r.CheckLocalItems(t, file1)

	err = operations.CopyFile(ctx, fdst, r.Flocal, file1.Path, file1.Path)
	require.NoError(t, err)

	file1dst := file1
	file1dst.Path = "dst/one"

	r.CheckRemoteItems(t, file1dst)

	// check old dest, empty compare
	file1b := r.WriteFile("one", "onet2", t2)
	r.CheckRemoteItems(t, file1dst)
	r.CheckLocalItems(t, file1b)

	err = operations.CopyFile(ctx, fdst, r.Flocal, file1b.Path, file1b.Path)
	require.NoError(t, err)

	file1bdst := file1b
	file1bdst.Path = "dst/one"

	r.CheckRemoteItems(t, file1bdst)

	// check old dest, new compare
	file3 := r.WriteObject(ctx, "dst/one", "one", t1)
	file2 := r.WriteObject(ctx, "CompareDest/one", "onet2", t2)
	file1c := r.WriteFile("one", "onet2", t2)
	r.CheckRemoteItems(t, file2, file3)
	r.CheckLocalItems(t, file1c)

	err = operations.CopyFile(ctx, fdst, r.Flocal, file1c.Path, file1c.Path)
	require.NoError(t, err)

	r.CheckRemoteItems(t, file2, file3)

	// check empty dest, new compare
	file4 := r.WriteObject(ctx, "CompareDest/two", "two", t2)
	file5 := r.WriteFile("two", "two", t2)
	r.CheckRemoteItems(t, file2, file3, file4)
	r.CheckLocalItems(t, file1c, file5)

	err = operations.CopyFile(ctx, fdst, r.Flocal, file5.Path, file5.Path)
	require.NoError(t, err)

	r.CheckRemoteItems(t, file2, file3, file4)

	// check new dest, new compare
	err = operations.CopyFile(ctx, fdst, r.Flocal, file5.Path, file5.Path)
	require.NoError(t, err)

	r.CheckRemoteItems(t, file2, file3, file4)

	// check empty dest, old compare
	file5b := r.WriteFile("two", "twot3", t3)
	r.CheckRemoteItems(t, file2, file3, file4)
	r.CheckLocalItems(t, file1c, file5b)

	err = operations.CopyFile(ctx, fdst, r.Flocal, file5b.Path, file5b.Path)
	require.NoError(t, err)

	file5bdst := file5b
	file5bdst.Path = "dst/two"

	r.CheckRemoteItems(t, file2, file3, file4, file5bdst)
}

// Test with CopyDest set
func TestCopyFileCopyDest(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()

	if r.Fremote.Features().Copy == nil {
		t.Skip("Skipping test as remote does not support server-side copy")
	}

	ci.CopyDest = []string{r.FremoteName + "/CopyDest"}

	fdst, err := fs.NewFs(ctx, r.FremoteName+"/dst")
	require.NoError(t, err)

	// check empty dest, empty copy
	file1 := r.WriteFile("one", "one", t1)
	r.CheckLocalItems(t, file1)

	err = operations.CopyFile(ctx, fdst, r.Flocal, file1.Path, file1.Path)
	require.NoError(t, err)

	file1dst := file1
	file1dst.Path = "dst/one"

	r.CheckRemoteItems(t, file1dst)

	// check old dest, empty copy
	file1b := r.WriteFile("one", "onet2", t2)
	r.CheckRemoteItems(t, file1dst)
	r.CheckLocalItems(t, file1b)

	err = operations.CopyFile(ctx, fdst, r.Flocal, file1b.Path, file1b.Path)
	require.NoError(t, err)

	file1bdst := file1b
	file1bdst.Path = "dst/one"

	r.CheckRemoteItems(t, file1bdst)

	// check old dest, new copy, backup-dir

	ci.BackupDir = r.FremoteName + "/BackupDir"

	file3 := r.WriteObject(ctx, "dst/one", "one", t1)
	file2 := r.WriteObject(ctx, "CopyDest/one", "onet2", t2)
	file1c := r.WriteFile("one", "onet2", t2)
	r.CheckRemoteItems(t, file2, file3)
	r.CheckLocalItems(t, file1c)

	err = operations.CopyFile(ctx, fdst, r.Flocal, file1c.Path, file1c.Path)
	require.NoError(t, err)

	file2dst := file2
	file2dst.Path = "dst/one"
	file3.Path = "BackupDir/one"

	r.CheckRemoteItems(t, file2, file2dst, file3)
	ci.BackupDir = ""

	// check empty dest, new copy
	file4 := r.WriteObject(ctx, "CopyDest/two", "two", t2)
	file5 := r.WriteFile("two", "two", t2)
	r.CheckRemoteItems(t, file2, file2dst, file3, file4)
	r.CheckLocalItems(t, file1c, file5)

	err = operations.CopyFile(ctx, fdst, r.Flocal, file5.Path, file5.Path)
	require.NoError(t, err)

	file4dst := file4
	file4dst.Path = "dst/two"

	r.CheckRemoteItems(t, file2, file2dst, file3, file4, file4dst)

	// check new dest, new copy
	err = operations.CopyFile(ctx, fdst, r.Flocal, file5.Path, file5.Path)
	require.NoError(t, err)

	r.CheckRemoteItems(t, file2, file2dst, file3, file4, file4dst)

	// check empty dest, old copy
	file6 := r.WriteObject(ctx, "CopyDest/three", "three", t2)
	file7 := r.WriteFile("three", "threet3", t3)
	r.CheckRemoteItems(t, file2, file2dst, file3, file4, file4dst, file6)
	r.CheckLocalItems(t, file1c, file5, file7)

	err = operations.CopyFile(ctx, fdst, r.Flocal, file7.Path, file7.Path)
	require.NoError(t, err)

	file7dst := file7
	file7dst.Path = "dst/three"

	r.CheckRemoteItems(t, file2, file2dst, file3, file4, file4dst, file6, file7dst)
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

// testFs is for unit testing fs.Fs
type testFs struct {
	testFsInfo
}

func (i *testFs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	return nil, nil
}

func (i *testFs) NewObject(ctx context.Context, remote string) (fs.Object, error) { return nil, nil }

func (i *testFs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return nil, nil
}

func (i *testFs) Mkdir(ctx context.Context, dir string) error { return nil }

func (i *testFs) Rmdir(ctx context.Context, dir string) error { return nil }

// copied from TestOverlapping because the behavior of OverlappingFilterCheck should be identical to Overlapping
// when no filters are set
func TestOverlappingFilterCheckWithoutFilter(t *testing.T) {
	ctx := context.Background()
	src := &testFs{testFsInfo{name: "name", root: "root"}}
	slash := string(os.PathSeparator) // native path separator
	for _, test := range []struct {
		name     string
		root     string
		expected bool
	}{
		{"name", "root", true},
		{"name", "/root", true},
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
		dst := &testFs{testFsInfo{name: test.name, root: test.root}}
		what := fmt.Sprintf("(%q,%q) vs (%q,%q)", src.name, src.root, dst.name, dst.root)
		actual := operations.OverlappingFilterCheck(ctx, src, dst)
		assert.Equal(t, test.expected, actual, what)
		actual = operations.OverlappingFilterCheck(ctx, dst, src)
		assert.Equal(t, test.expected, actual, what)
	}
}

func TestOverlappingFilterCheckWithFilter(t *testing.T) {
	ctx := context.Background()
	fi, err := filter.NewFilter(nil)
	require.NoError(t, err)
	require.NoError(t, fi.Add(false, "*/exclude/"))
	fi.Opt.ExcludeFile = []string{".ignore"}
	ctx = filter.ReplaceConfig(ctx, fi)

	src := &testFs{testFsInfo{name: "name", root: "root"}}
	slash := string(os.PathSeparator) // native path separator
	for _, test := range []struct {
		name     string
		root     string
		expected bool
	}{
		{"name", "root", true},
		{"name", "/root", true},
		{"name", "root/", true},
		{"name", "root" + slash, true},
		{"name", "root/exclude", false},
		{"name", "root/exclude/", false},
		{"name", "/root/exclude/", false},
		{"name", "root" + slash + "exclude", false},
		{"name", "root" + slash + "exclude" + slash, false},
		{"name", "root/.ignore", false},
		{"name", "root" + slash + ".ignore", false},
		{"namey", "root/include", false},
		{"namey", "root/include/", false},
		{"namey", "root" + slash + "include", false},
		{"namey", "root" + slash + "include" + slash, false},
	} {
		dst := &testFs{testFsInfo{name: test.name, root: test.root}}
		what := fmt.Sprintf("(%q,%q) vs (%q,%q)", src.name, src.root, dst.name, dst.root)
		actual := operations.OverlappingFilterCheck(ctx, dst, src)
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
			"md5":      "0cc175b9c0f1b6a831c399e269772661",
			"sha1":     "86f7e437faa5a7fce15d1ddcb9eaeaea377667b8",
			"dropbox":  "bf5d3affb73efd2ec6c36ad3112dd933efed63c4e1cbffcfa88e2759c144f2d8",
			"quickxor": "6100000000000000000000000100000000000000"},
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
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()

	r.Mkdir(ctx, r.Fremote)

	// Make some files and dirs
	r.ForceMkdir(ctx, r.Fremote)
	files := []fstest.Item{
		r.WriteObject(ctx, "A1/one", "one", t1),
		r.WriteObject(ctx, "A1/two", "two", t2),
		r.WriteObject(ctx, "A1/B1/three", "three", t3),
		r.WriteObject(ctx, "A1/B1/C1/four", "four", t1),
		r.WriteObject(ctx, "A1/B1/C2/five", "five", t2),
	}
	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "A1/B2"))
	require.NoError(t, operations.Mkdir(ctx, r.Fremote, "A1/B1/C3"))

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
		fs.GetModifyWindow(ctx, r.Fremote),
	)

	require.NoError(t, operations.DirMove(ctx, r.Fremote, "A1", "A2"))

	for i := range files {
		files[i].Path = strings.ReplaceAll(files[i].Path, "A1/", "A2/")
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
		fs.GetModifyWindow(ctx, r.Fremote),
	)

	// Disable DirMove
	features := r.Fremote.Features()
	features.DirMove = nil

	require.NoError(t, operations.DirMove(ctx, r.Fremote, "A2", "A3"))

	for i := range files {
		files[i].Path = strings.ReplaceAll(files[i].Path, "A2/", "A3/")
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
		fs.GetModifyWindow(ctx, r.Fremote),
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
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	check := func(withChecksum, ignoreChecksum bool) {
		ci.CheckSum, ci.IgnoreChecksum = withChecksum, ignoreChecksum

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

		if *fstest.SizeLimit > 0 && int64(ci.StreamingUploadCutoff) > *fstest.SizeLimit {
			savedCutoff := ci.StreamingUploadCutoff
			ci.StreamingUploadCutoff = fs.SizeSuffix(*fstest.SizeLimit)
			t.Logf("Adjust StreamingUploadCutoff to size limit %s (was %s)", ci.StreamingUploadCutoff, savedCutoff)
		}

		fstest.CheckListing(t, r.Fremote, []fstest.Item{})

		data1 := "this is some really nice test data"
		path1 := prefix + "small_file_from_pipe"

		data2 := string(make([]byte, ci.StreamingUploadCutoff+1))
		path2 := prefix + "big_file_from_pipe"

		in := ioutil.NopCloser(strings.NewReader(data1))
		_, err := operations.Rcat(ctx, r.Fremote, path1, in, t1)
		require.NoError(t, err)

		in = ioutil.NopCloser(strings.NewReader(data2))
		_, err = operations.Rcat(ctx, r.Fremote, path2, in, t2)
		require.NoError(t, err)

		file1 := fstest.NewItem(path1, data1, t1)
		file2 := fstest.NewItem(path2, data2, t2)
		r.CheckRemoteItems(t, file1, file2)
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
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()

	const body = "------------------------------------------------------------"
	file1 := r.WriteFile("potato1", body, t1)
	file2 := r.WriteFile("potato2", body, t2)
	// Test with known length
	bodyReader := ioutil.NopCloser(strings.NewReader(body))
	obj, err := operations.RcatSize(ctx, r.Fremote, file1.Path, bodyReader, int64(len(body)), file1.ModTime)
	require.NoError(t, err)
	assert.Equal(t, int64(len(body)), obj.Size())
	assert.Equal(t, file1.Path, obj.Remote())

	// Test with unknown length
	bodyReader = ioutil.NopCloser(strings.NewReader(body)) // reset Reader
	ioutil.NopCloser(strings.NewReader(body))
	obj, err = operations.RcatSize(ctx, r.Fremote, file2.Path, bodyReader, -1, file2.ModTime)
	require.NoError(t, err)
	assert.Equal(t, int64(len(body)), obj.Size())
	assert.Equal(t, file2.Path, obj.Remote())

	// Check files exist
	r.CheckRemoteItems(t, file1, file2)
}

func TestCopyFileMaxTransfer(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	defer r.Finalise()
	defer accounting.Stats(ctx).ResetCounters()

	const sizeCutoff = 2048

	// Make random incompressible data
	randomData := make([]byte, sizeCutoff)
	_, err := rand.Read(randomData)
	require.NoError(t, err)
	randomString := string(randomData)

	file1 := r.WriteFile("TestCopyFileMaxTransfer/file1", "file1 contents", t1)
	file2 := r.WriteFile("TestCopyFileMaxTransfer/file2", "file2 contents"+randomString, t2)
	file3 := r.WriteFile("TestCopyFileMaxTransfer/file3", "file3 contents"+randomString, t2)
	file4 := r.WriteFile("TestCopyFileMaxTransfer/file4", "file4 contents"+randomString, t2)

	// Cutoff mode: Hard
	ci.MaxTransfer = sizeCutoff
	ci.CutoffMode = fs.CutoffModeHard

	// file1: Show a small file gets transferred OK
	accounting.Stats(ctx).ResetCounters()
	err = operations.CopyFile(ctx, r.Fremote, r.Flocal, file1.Path, file1.Path)
	require.NoError(t, err)
	r.CheckLocalItems(t, file1, file2, file3, file4)
	r.CheckRemoteItems(t, file1)

	// file2: show a large file does not get transferred
	accounting.Stats(ctx).ResetCounters()
	err = operations.CopyFile(ctx, r.Fremote, r.Flocal, file2.Path, file2.Path)
	require.NotNil(t, err, "Did not get expected max transfer limit error")
	assert.Contains(t, err.Error(), "Max transfer limit reached")
	assert.True(t, fserrors.IsFatalError(err), fmt.Sprintf("Not fatal error: %v: %#v:", err, err))
	r.CheckLocalItems(t, file1, file2, file3, file4)
	r.CheckRemoteItems(t, file1)

	// Cutoff mode: Cautious
	ci.CutoffMode = fs.CutoffModeCautious

	// file3: show a large file does not get transferred
	accounting.Stats(ctx).ResetCounters()
	err = operations.CopyFile(ctx, r.Fremote, r.Flocal, file3.Path, file3.Path)
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "Max transfer limit reached")
	assert.True(t, fserrors.IsNoRetryError(err))
	r.CheckLocalItems(t, file1, file2, file3, file4)
	r.CheckRemoteItems(t, file1)

	if strings.HasPrefix(r.Fremote.Name(), "TestChunker") {
		t.Log("skipping remainder of test for chunker as it involves multiple transfers")
		return
	}

	// Cutoff mode: Soft
	ci.CutoffMode = fs.CutoffModeSoft

	// file4: show a large file does get transferred this time
	accounting.Stats(ctx).ResetCounters()
	err = operations.CopyFile(ctx, r.Fremote, r.Flocal, file4.Path, file4.Path)
	require.NoError(t, err)
	r.CheckLocalItems(t, file1, file2, file3, file4)
	r.CheckRemoteItems(t, file1, file4)
}

func TestTouchDir(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()

	if r.Fremote.Precision() == fs.ModTimeNotSupported {
		t.Skip("Skipping test as remote does not support modtime")
	}

	file1 := r.WriteBoth(ctx, "potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth(ctx, "empty space", "-", t2)
	file3 := r.WriteBoth(ctx, "sub dir/potato3", "hello", t2)
	r.CheckRemoteItems(t, file1, file2, file3)

	timeValue := time.Date(2010, 9, 8, 7, 6, 5, 4, time.UTC)
	err := operations.TouchDir(ctx, r.Fremote, "", timeValue, true)
	require.NoError(t, err)
	if accounting.Stats(ctx).GetErrors() != 0 {
		err = accounting.Stats(ctx).GetLastError()
		require.True(t, errors.Is(err, fs.ErrorCantSetModTime) || errors.Is(err, fs.ErrorCantSetModTimeWithoutDelete))
	} else {
		file1.ModTime = timeValue
		file2.ModTime = timeValue
		file3.ModTime = timeValue
		r.CheckRemoteItems(t, file1, file2, file3)
	}
}
