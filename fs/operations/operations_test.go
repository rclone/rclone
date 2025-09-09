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
	"errors"
	"fmt"
	"io"
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
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
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

	err := operations.Mkdir(ctx, r.Fremote, "")
	require.NoError(t, err)
	fstest.CheckListing(t, r.Fremote, []fstest.Item{})

	err = operations.Mkdir(ctx, r.Fremote, "")
	require.NoError(t, err)
}

func TestLsd(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
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
		name := cases.Title(language.Und, cases.NoLower).String(test.ht.String())
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
	_ = fstests.PutTestContents(ctx, t, memFs, &item1, content, true)

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
	in := io.NopCloser(reader)
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
		{"test.txt.csv", "-suffix", true, "test-suffix.txt.csv"},
		{"test", "-suffix", false, "test-suffix"},
		{"test", "-suffix", true, "test-suffix"},
		{"test.html", "-suffix", true, "test-suffix.html"},
		{"test.html.txt", "-suffix", true, "test-suffix.html.txt"},
		{"test.csv.html.txt", "-suffix", true, "test-suffix.csv.html.txt"},
		{"test.badext.csv.html.txt", "-suffix", true, "test.badext-suffix.csv.html.txt"},
		{"test.badext", "-suffix", true, "test-suffix.badext"},
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
	file1 := r.WriteObject(ctx, "small", "1234567890", t2)                                                                                           // 10 bytes
	file2 := r.WriteObject(ctx, "medium", "------------------------------------------------------------", t1)                                        // 60 bytes
	file3 := r.WriteObject(ctx, "large", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", t1) // 100 bytes
	r.CheckRemoteItems(t, file1, file2, file3)

	err = operations.Delete(ctx, r.Fremote)
	require.NoError(t, err)
	r.CheckRemoteItems(t, file3)
}

func isChunker(f fs.Fs) bool {
	return strings.HasPrefix(f.Name(), "TestChunker")
}

func skipIfChunker(t *testing.T, f fs.Fs) {
	if isChunker(f) {
		t.Skip("Skipping test on chunker backend")
	}
}

func TestMaxDelete(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	accounting.GlobalStats().ResetCounters()
	ci.MaxDelete = 2
	defer r.Finalise()
	skipIfChunker(t, r.Fremote)                                                                                                                      // chunker does copy/delete on s3
	file1 := r.WriteObject(ctx, "small", "1234567890", t2)                                                                                           // 10 bytes
	file2 := r.WriteObject(ctx, "medium", "------------------------------------------------------------", t1)                                        // 60 bytes
	file3 := r.WriteObject(ctx, "large", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", t1) // 100 bytes
	r.CheckRemoteItems(t, file1, file2, file3)
	err := operations.Delete(ctx, r.Fremote)

	require.Error(t, err)
	objects, _, _, err := operations.Count(ctx, r.Fremote)
	require.NoError(t, err)
	assert.Equal(t, int64(1), objects)
}

// TestMaxDeleteSizeLargeFile one of the files is larger than allowed
func TestMaxDeleteSizeLargeFile(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	accounting.GlobalStats().ResetCounters()
	ci.MaxDeleteSize = 70
	defer r.Finalise()
	skipIfChunker(t, r.Fremote)                                                                                                                      // chunker does copy/delete on s3
	file1 := r.WriteObject(ctx, "small", "1234567890", t2)                                                                                           // 10 bytes
	file2 := r.WriteObject(ctx, "medium", "------------------------------------------------------------", t1)                                        // 60 bytes
	file3 := r.WriteObject(ctx, "large", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", t1) // 100 bytes
	r.CheckRemoteItems(t, file1, file2, file3)

	err := operations.Delete(ctx, r.Fremote)
	require.Error(t, err)
	r.CheckRemoteItems(t, file3)
}

func TestMaxDeleteSize(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	accounting.GlobalStats().ResetCounters()
	ci.MaxDeleteSize = 160
	defer r.Finalise()
	skipIfChunker(t, r.Fremote)                                                                                                                      // chunker does copy/delete on s3
	file1 := r.WriteObject(ctx, "small", "1234567890", t2)                                                                                           // 10 bytes
	file2 := r.WriteObject(ctx, "medium", "------------------------------------------------------------", t1)                                        // 60 bytes
	file3 := r.WriteObject(ctx, "large", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", t1) // 100 bytes
	r.CheckRemoteItems(t, file1, file2, file3)

	err := operations.Delete(ctx, r.Fremote)
	require.Error(t, err)
	objects, _, _, err := operations.Count(ctx, r.Fremote)
	require.NoError(t, err)
	assert.Equal(t, int64(1), objects) // 10 or 100 bytes
}

func TestReadFile(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()

	contents := "A file to read the contents."
	file := r.WriteObject(ctx, "ReadFile", contents, t1)
	r.CheckRemoteItems(t, file)

	o, err := r.Fremote.NewObject(ctx, file.Path)
	require.NoError(t, err)

	buf, err := operations.ReadFile(ctx, o)
	require.NoError(t, err)
	assert.Equal(t, contents, string(buf))
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

	i, err = 3, fmt.Errorf("Wrapped EOF is retriable: %w", io.EOF)
	assert.Equal(t, nil, operations.Retry(ctx, nil, 5, fn))
	assert.Equal(t, 0, i)

	i, err = 10, pacer.RetryAfterError(errors.New("BANG"), 10*time.Millisecond)
	assert.Equal(t, err, operations.Retry(ctx, nil, 5, fn))
	assert.Equal(t, 5, i)

	i, err = 10, fs.ErrorObjectNotFound
	assert.Equal(t, fs.ErrorObjectNotFound, operations.Retry(ctx, nil, 5, fn))
	assert.Equal(t, 9, i)

}

func TestCat(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	file1 := r.WriteBoth(ctx, "file1", "ABCDEFGHIJ", t1)
	file2 := r.WriteBoth(ctx, "file2", "012345678", t2)

	r.CheckRemoteItems(t, file1, file2)

	for _, test := range []struct {
		offset    int64
		count     int64
		separator string
		a         string
		b         string
	}{
		{0, -1, "", "ABCDEFGHIJ", "012345678"},
		{0, 5, "", "ABCDE", "01234"},
		{-3, -1, "", "HIJ", "678"},
		{1, 3, "", "BCD", "123"},
		{0, -1, "\n", "ABCDEFGHIJ", "012345678"},
	} {
		var buf bytes.Buffer
		err := operations.Cat(ctx, r.Fremote, &buf, test.offset, test.count, []byte(test.separator))
		require.NoError(t, err)
		res := buf.String()

		if res != test.a+test.separator+test.b+test.separator && res != test.b+test.separator+test.a+test.separator {
			t.Errorf("Incorrect output from Cat(%d,%d,%s): %q", test.offset, test.count, test.separator, res)
		}
	}
}

func TestPurge(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRunIndividual(t) // make new container (azureblob has delayed mkdir after rmdir)
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

	// Delete the files so we can remove everything including the root
	for _, file := range []fstest.Item{file1, file2} {
		o, err := r.Fremote.NewObject(ctx, file.Path)
		require.NoError(t, err)
		require.NoError(t, o.Remove(ctx))
	}

	require.NoError(t, operations.Rmdirs(ctx, r.Fremote, "", false))

	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		[]fstest.Item{},
		[]string{},
		fs.GetModifyWindow(ctx, r.Fremote),
	)
}

func TestRmdirsLeaveRoot(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
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

	contents := "file contents\n"
	file1 := r.WriteFile("file1", contents, t1)
	file2 := r.WriteFile("file2", contents, t1)
	r.Mkdir(ctx, r.Fremote)
	r.CheckRemoteItems(t)

	// check when reading from regular HTTP server
	status := 0
	nameHeader := false
	headerFilename := "headerfilename.txt"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != 0 {
			http.Error(w, "an error occurred", status)
		}
		if nameHeader {
			w.Header().Set("Content-Disposition", `attachment; filename="folder\`+headerFilename+`"`)
		}
		_, err := w.Write([]byte(contents))
		assert.NoError(t, err)
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	o, err := operations.CopyURL(ctx, r.Fremote, "file1", ts.URL, false, false, false)
	require.NoError(t, err)
	assert.Equal(t, int64(len(contents)), o.Size())

	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1}, nil, fs.ModTimeNotSupported)

	// Check file clobbering
	_, err = operations.CopyURL(ctx, r.Fremote, "file1", ts.URL, false, false, true)
	require.Error(t, err)

	// Check auto file naming
	status = 0
	urlFileName := "filename.txt"
	o, err = operations.CopyURL(ctx, r.Fremote, "", ts.URL+"/"+urlFileName, true, false, false)
	require.NoError(t, err)
	assert.Equal(t, int64(len(contents)), o.Size())
	assert.Equal(t, urlFileName, o.Remote())

	// Check header file naming
	nameHeader = true
	o, err = operations.CopyURL(ctx, r.Fremote, "", ts.URL, true, true, false)
	require.NoError(t, err)
	assert.Equal(t, int64(len(contents)), o.Size())
	assert.Equal(t, headerFilename, o.Remote())

	// Check auto file naming when url without file name
	_, err = operations.CopyURL(ctx, r.Fremote, "file1", ts.URL, true, false, false)
	require.Error(t, err)

	// Check header file naming without header set
	nameHeader = false
	_, err = operations.CopyURL(ctx, r.Fremote, "file1", ts.URL, true, true, false)
	require.Error(t, err)

	// Check an error is returned for a 404
	status = http.StatusNotFound
	o, err = operations.CopyURL(ctx, r.Fremote, "file1", ts.URL, false, false, false)
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

	o, err = operations.CopyURL(ctx, r.Fremote, "file2", tss.URL, false, false, false)
	require.NoError(t, err)
	assert.Equal(t, int64(len(contents)), o.Size())
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1, file2, fstest.NewItem(urlFileName, contents, t1), fstest.NewItem(headerFilename, contents, t1)}, nil, fs.ModTimeNotSupported)
}

func TestCopyURLToWriter(t *testing.T) {
	ctx := context.Background()
	contents := "file contents\n"

	// check when reading from regular HTTP server
	status := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != 0 {
			http.Error(w, "an error occurred", status)
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

func TestCaseInsensitiveMoveFileDryRun(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
	if !r.Fremote.Features().CaseInsensitive {
		return
	}

	file1 := r.WriteObject(ctx, "hello", "world", t1)
	r.CheckRemoteItems(t, file1)

	ci.DryRun = true
	err := operations.MoveFile(ctx, r.Fremote, r.Fremote, "HELLO", file1.Path)
	require.NoError(t, err)
	r.CheckRemoteItems(t, file1)
}

func TestMoveFileBackupDir(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
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
	require.NoError(t, fi.Add(false, "/exclude/"))
	require.NoError(t, fi.Add(false, "/Exclude2/"))
	require.NoError(t, fi.Add(true, "*"))
	ctx = filter.ReplaceConfig(ctx, fi)

	src := &testFs{testFsInfo{name: "name", root: "root"}}
	src.features.CaseInsensitive = true
	slash := string(os.PathSeparator) // native path separator
	for _, test := range []struct {
		name     string
		root     string
		expected bool
	}{
		{"name", "root", true},
		{"name", "ROOT", true}, // case insensitive is set
		{"name", "/root", true},
		{"name", "root/", true},
		{"name", "root" + slash, true},
		{"name", "root/exclude", false},
		{"name", "root/Exclude2", false},
		{"name", "root/include", true},
		{"name", "root/exclude/", false},
		{"name", "root/Exclude2/", false},
		{"name", "root/exclude/sub", false},
		{"name", "root/Exclude2/sub", false},
		{"name", "/root/exclude/", false},
		{"name", "root" + slash + "exclude", false},
		{"name", "root" + slash + "exclude" + slash, false},
		{"namey", "root/include", false},
		{"namey", "root/include/", false},
		{"namey", "root" + slash + "include", false},
		{"namey", "root" + slash + "include" + slash, false},
	} {
		dst := &testFs{testFsInfo{name: test.name, root: test.root}}
		dst.features.CaseInsensitive = true
		what := fmt.Sprintf("(%q,%q) vs (%q,%q)", src.name, src.root, dst.name, dst.root)
		actual := operations.OverlappingFilterCheck(ctx, dst, src)
		assert.Equal(t, test.expected, actual, what)
		actual = operations.OverlappingFilterCheck(ctx, src, dst)
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
	list.AddModTime("")
	assert.Equal(t, t1.Local().Format("2006-01-02 15:04:05"), list.Format(item0))

	list.SetOutput(nil)
	list.AddModTime("unix")
	assert.Equal(t, fmt.Sprint(t1.Local().Unix()), list.Format(item0))

	list.SetOutput(nil)
	list.AddModTime("unixnano")
	assert.Equal(t, fmt.Sprint(t1.Local().UnixNano()), list.Format(item0))

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
	list.AddMetadata()
	assert.Equal(t, "{}", list.Format(item0))
	assert.Equal(t, "{}", list.Format(item1))

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
	list.AddModTime("")
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
	list.AddModTime("")
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

	// Try with a DirMove method that exists but returns fs.ErrorCantDirMove (ex. combine moving across upstreams)
	// Should fall back to manual move (copy + delete)

	features.DirMove = func(ctx context.Context, src fs.Fs, srcRemote string, dstRemote string) error {
		return fs.ErrorCantDirMove
	}

	assert.NoError(t, operations.DirMove(ctx, r.Fremote, "A3", "A4"))

	for i := range files {
		files[i].Path = strings.ReplaceAll(files[i].Path, "A3/", "A4/")
	}

	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		files,
		[]string{
			"A4",
			"A4/B1",
			"A4/B2",
			"A4/B1/C1",
			"A4/B1/C2",
			"A4/B1/C3",
		},
		fs.GetModifyWindow(ctx, r.Fremote),
	)
}

func TestGetFsInfo(t *testing.T) {
	r := fstest.NewRun(t)

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
	check := func(t *testing.T, withChecksum, ignoreChecksum bool) {
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

		in := io.NopCloser(strings.NewReader(data1))
		_, err := operations.Rcat(ctx, r.Fremote, path1, in, t1, nil)
		require.NoError(t, err)

		in = io.NopCloser(strings.NewReader(data2))
		_, err = operations.Rcat(ctx, r.Fremote, path2, in, t2, nil)
		require.NoError(t, err)

		file1 := fstest.NewItem(path1, data1, t1)
		file2 := fstest.NewItem(path2, data2, t2)
		r.CheckRemoteItems(t, file1, file2)
	}

	for i := range 4 {
		withChecksum := (i & 1) != 0
		ignoreChecksum := (i & 2) != 0
		t.Run(fmt.Sprintf("withChecksum=%v,ignoreChecksum=%v", withChecksum, ignoreChecksum), func(t *testing.T) {
			check(t, withChecksum, ignoreChecksum)
		})
	}
}

func TestRcatMetadata(t *testing.T) {
	r := fstest.NewRun(t)

	if !r.Fremote.Features().UserMetadata {
		t.Skip("Skipping as destination doesn't support user metadata")
	}

	test := func(disableUploadCutoff bool) {
		ctx := context.Background()
		ctx, ci := fs.AddConfig(ctx)
		ci.Metadata = true
		data := "this is some really nice test data with metadata"
		path := "rcat_metadata"

		meta := fs.Metadata{
			"key":     "value",
			"sausage": "potato",
		}

		if disableUploadCutoff {
			ci.StreamingUploadCutoff = 0
			data += " uploadCutoff=0"
			path += "_uploadcutoff0"
		}

		fstest.CheckListing(t, r.Fremote, []fstest.Item{})

		in := io.NopCloser(strings.NewReader(data))
		_, err := operations.Rcat(ctx, r.Fremote, path, in, t1, meta)
		require.NoError(t, err)

		file := fstest.NewItem(path, data, t1)
		r.CheckRemoteItems(t, file)

		o, err := r.Fremote.NewObject(ctx, path)
		require.NoError(t, err)
		gotMeta, err := fs.GetMetadata(ctx, o)
		require.NoError(t, err)
		// Check the specific user data we set is set
		// Likely there will be other values
		assert.Equal(t, "value", gotMeta["key"])
		assert.Equal(t, "potato", gotMeta["sausage"])

		// Delete the test file
		require.NoError(t, o.Remove(ctx))
	}

	t.Run("Normal", func(t *testing.T) {
		test(false)
	})
	t.Run("ViaDisk", func(t *testing.T) {
		test(true)
	})
}

func TestRcatSize(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)

	const body = "------------------------------------------------------------"
	file1 := r.WriteFile("potato1", body, t1)
	file2 := r.WriteFile("potato2", body, t2)
	// Test with known length
	bodyReader := io.NopCloser(strings.NewReader(body))
	obj, err := operations.RcatSize(ctx, r.Fremote, file1.Path, bodyReader, int64(len(body)), file1.ModTime, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(len(body)), obj.Size())
	assert.Equal(t, file1.Path, obj.Remote())

	// Test with unknown length
	bodyReader = io.NopCloser(strings.NewReader(body)) // reset Reader
	io.NopCloser(strings.NewReader(body))
	obj, err = operations.RcatSize(ctx, r.Fremote, file2.Path, bodyReader, -1, file2.ModTime, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(len(body)), obj.Size())
	assert.Equal(t, file2.Path, obj.Remote())

	// Check files exist
	r.CheckRemoteItems(t, file1, file2)
}

func TestRcatSizeMetadata(t *testing.T) {
	r := fstest.NewRun(t)

	if !r.Fremote.Features().UserMetadata {
		t.Skip("Skipping as destination doesn't support user metadata")
	}

	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	ci.Metadata = true

	meta := fs.Metadata{
		"key":     "value",
		"sausage": "potato",
	}

	const body = "------------------------------------------------------------"
	file1 := r.WriteFile("potato1", body, t1)
	file2 := r.WriteFile("potato2", body, t2)

	// Test with known length
	bodyReader := io.NopCloser(strings.NewReader(body))
	obj, err := operations.RcatSize(ctx, r.Fremote, file1.Path, bodyReader, int64(len(body)), file1.ModTime, meta)
	require.NoError(t, err)
	assert.Equal(t, int64(len(body)), obj.Size())
	assert.Equal(t, file1.Path, obj.Remote())

	// Test with unknown length
	bodyReader = io.NopCloser(strings.NewReader(body)) // reset Reader
	io.NopCloser(strings.NewReader(body))
	obj, err = operations.RcatSize(ctx, r.Fremote, file2.Path, bodyReader, -1, file2.ModTime, meta)
	require.NoError(t, err)
	assert.Equal(t, int64(len(body)), obj.Size())
	assert.Equal(t, file2.Path, obj.Remote())

	// Check files exist
	r.CheckRemoteItems(t, file1, file2)

	// Check metadata OK
	for _, path := range []string{file1.Path, file2.Path} {
		o, err := r.Fremote.NewObject(ctx, path)
		require.NoError(t, err)
		gotMeta, err := fs.GetMetadata(ctx, o)
		require.NoError(t, err)
		// Check the specific user data we set is set
		// Likely there will be other values
		assert.Equal(t, "value", gotMeta["key"])
		assert.Equal(t, "potato", gotMeta["sausage"])
	}
}

func TestTouchDir(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)

	if r.Fremote.Precision() == fs.ModTimeNotSupported {
		t.Skip("Skipping test as remote does not support modtime")
	}

	file1 := r.WriteBoth(ctx, "potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth(ctx, "empty space", "-", t2)
	file3 := r.WriteBoth(ctx, "sub dir/potato3", "hello", t2)
	r.CheckRemoteItems(t, file1, file2, file3)

	accounting.GlobalStats().ResetCounters()
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

var testMetadata = fs.Metadata{
	// System metadata supported by all backends
	"mtime": t1.Format(time.RFC3339Nano),
	// User metadata
	"potato": "jersey",
}

func TestMkdirMetadata(t *testing.T) {
	const name = "dir with metadata"
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	ci.Metadata = true
	r := fstest.NewRun(t)
	features := r.Fremote.Features()
	if features.MkdirMetadata == nil {
		t.Skip("Skipping test as remote does not support MkdirMetadata")
	}

	newDst, err := operations.MkdirMetadata(ctx, r.Fremote, name, testMetadata)
	require.NoError(t, err)
	require.NotNil(t, newDst)

	require.True(t, features.ReadDirMetadata, "Expecting ReadDirMetadata to be supported if MkdirMetadata is supported")

	// Check the returned directory and one read from the listing
	fstest.CheckEntryMetadata(ctx, t, r.Fremote, newDst, testMetadata)
	fstest.CheckEntryMetadata(ctx, t, r.Fremote, fstest.NewDirectory(ctx, t, r.Fremote, name), testMetadata)
}

func TestMkdirModTime(t *testing.T) {
	const name = "directory with modtime"
	ctx := context.Background()
	r := fstest.NewRun(t)
	if r.Fremote.Features().DirSetModTime == nil && r.Fremote.Features().MkdirMetadata == nil {
		t.Skip("Skipping test as remote does not support DirSetModTime or MkdirMetadata")
	}
	newDst, err := operations.MkdirModTime(ctx, r.Fremote, name, t2)
	require.NoError(t, err)

	// Check the returned directory and one read from the listing
	// newDst may be nil here depending on how the modtime was set
	if newDst != nil {
		fstest.CheckDirModTime(ctx, t, r.Fremote, newDst, t2)
	}
	fstest.CheckDirModTime(ctx, t, r.Fremote, fstest.NewDirectory(ctx, t, r.Fremote, name), t2)
}

func TestCopyDirMetadata(t *testing.T) {
	const nameNonExistent = "non existent directory"
	const nameExistent = "existing directory"
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	ci.Metadata = true
	r := fstest.NewRun(t)
	if !r.Fremote.Features().WriteDirMetadata && r.Fremote.Features().MkdirMetadata == nil {
		t.Skip("Skipping test as remote does not support WriteDirMetadata or MkdirMetadata")
	}

	// Create a source local directory with metadata
	newSrc, err := operations.MkdirMetadata(ctx, r.Flocal, "dir with metadata to be copied", testMetadata)
	require.NoError(t, err)
	require.NotNil(t, newSrc)

	// First try with the directory not existing
	newDst, err := operations.CopyDirMetadata(ctx, r.Fremote, nil, nameNonExistent, newSrc)
	require.NoError(t, err)
	require.NotNil(t, newDst)

	// Check the returned directory and one read from the listing
	fstest.CheckEntryMetadata(ctx, t, r.Fremote, newDst, testMetadata)
	fstest.CheckEntryMetadata(ctx, t, r.Fremote, fstest.NewDirectory(ctx, t, r.Fremote, nameNonExistent), testMetadata)

	// Then try with the directory existing
	require.NoError(t, r.Fremote.Rmdir(ctx, nameNonExistent))
	require.NoError(t, r.Fremote.Mkdir(ctx, nameExistent))
	existingDir := fstest.NewDirectory(ctx, t, r.Fremote, nameExistent)

	newDst, err = operations.CopyDirMetadata(ctx, r.Fremote, existingDir, "SHOULD BE IGNORED", newSrc)
	require.NoError(t, err)
	require.NotNil(t, newDst)

	// Check the returned directory and one read from the listing
	fstest.CheckEntryMetadata(ctx, t, r.Fremote, newDst, testMetadata)
	fstest.CheckEntryMetadata(ctx, t, r.Fremote, fstest.NewDirectory(ctx, t, r.Fremote, nameExistent), testMetadata)
}

func TestSetDirModTime(t *testing.T) {
	const name = "set modtime on existing directory"
	ctx, ci := fs.AddConfig(context.Background())
	r := fstest.NewRun(t)
	if r.Fremote.Features().DirSetModTime == nil && !r.Fremote.Features().WriteDirSetModTime {
		t.Skip("Skipping test as remote does not support DirSetModTime or WriteDirSetModTime")
	}

	// Check that we obey --no-update-dir-modtime - this should return nil, nil
	ci.NoUpdateDirModTime = true
	newDst, err := operations.SetDirModTime(ctx, r.Fremote, nil, "set modtime on non existent directory", t2)
	require.NoError(t, err)
	require.Nil(t, newDst)
	ci.NoUpdateDirModTime = false

	// First try with the directory not existing - should return an error
	newDst, err = operations.SetDirModTime(ctx, r.Fremote, nil, "set modtime on non existent directory", t2)
	require.Error(t, err)
	require.Nil(t, newDst)

	// Then try with the directory existing
	require.NoError(t, r.Fremote.Mkdir(ctx, name))
	existingDir := fstest.NewDirectory(ctx, t, r.Fremote, name)

	newDst, err = operations.SetDirModTime(ctx, r.Fremote, existingDir, "SHOULD BE IGNORED", t2)
	require.NoError(t, err)
	require.NotNil(t, newDst)

	// Check the returned directory and one read from the listing
	// The modtime will only be correct on newDst if it had a SetModTime method
	if _, ok := newDst.(fs.SetModTimer); ok {
		fstest.CheckDirModTime(ctx, t, r.Fremote, newDst, t2)
	}
	fstest.CheckDirModTime(ctx, t, r.Fremote, fstest.NewDirectory(ctx, t, r.Fremote, name), t2)

	// Now wrap the directory to make the SetModTime method return fs.ErrorNotImplemented and check that it falls back correctly
	wrappedDir := fs.NewDirWrapper(existingDir.Remote(), fs.NewDir(existingDir.Remote(), existingDir.ModTime(ctx)))
	newDst, err = operations.SetDirModTime(ctx, r.Fremote, wrappedDir, "SHOULD BE IGNORED", t1)
	require.NoError(t, err)
	require.NotNil(t, newDst)
	fstest.CheckDirModTime(ctx, t, r.Fremote, fstest.NewDirectory(ctx, t, r.Fremote, name), t1)
}

func TestDirsEqual(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	ci.Metadata = true
	r := fstest.NewRun(t)
	if !r.Fremote.Features().WriteDirMetadata && r.Fremote.Features().MkdirMetadata == nil {
		t.Skip("Skipping test as remote does not support WriteDirMetadata or MkdirMetadata")
	}

	opt := operations.DirsEqualOpt{
		ModifyWindow:   fs.GetModifyWindow(ctx, r.Flocal, r.Fremote),
		SetDirModtime:  true,
		SetDirMetadata: true,
	}

	// Create a source local directory with metadata
	src, err := operations.MkdirMetadata(ctx, r.Flocal, "dir with metadata to be copied", testMetadata)
	require.NoError(t, err)
	require.NotNil(t, src)

	// try with nil dst -- should be false
	equal := operations.DirsEqual(ctx, src, nil, opt)
	assert.False(t, equal)

	// make a dest with an equal modtime
	dst, err := operations.MkdirModTime(ctx, r.Fremote, "dst", src.ModTime(ctx))
	require.NoError(t, err)

	// try with equal modtimes -- should be true
	equal = operations.DirsEqual(ctx, src, dst, opt)
	assert.True(t, equal)

	// try with unequal modtimes -- should be false
	dst, err = operations.SetDirModTime(ctx, r.Fremote, dst, "", t2)
	require.NoError(t, err)
	equal = operations.DirsEqual(ctx, src, dst, opt)
	assert.False(t, equal)

	// try with unequal modtimes that are within modify window -- should be true
	halfWindow := opt.ModifyWindow / 2
	dst, err = operations.SetDirModTime(ctx, r.Fremote, dst, "", src.ModTime(ctx).Add(halfWindow))
	require.NoError(t, err)
	equal = operations.DirsEqual(ctx, src, dst, opt)
	assert.True(t, equal)

	// test ignoretimes -- should be false
	ci.IgnoreTimes = true
	equal = operations.DirsEqual(ctx, src, dst, opt)
	assert.False(t, equal)

	// test immutable -- should be true
	ci.IgnoreTimes = false
	ci.Immutable = true
	dst, err = operations.SetDirModTime(ctx, r.Fremote, dst, "", t3)
	require.NoError(t, err)
	equal = operations.DirsEqual(ctx, src, dst, opt)
	assert.True(t, equal)

	// test dst newer than src with --update -- should be true
	ci.Immutable = false
	ci.UpdateOlder = true
	equal = operations.DirsEqual(ctx, src, dst, opt)
	assert.True(t, equal)

	// test no SetDirModtime or SetDirMetadata -- should be true
	ci.UpdateOlder = false
	opt.SetDirMetadata, opt.SetDirModtime = false, false
	equal = operations.DirsEqual(ctx, src, dst, opt)
	assert.True(t, equal)
}

func TestRemoveExisting(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	if r.Fremote.Features().Move == nil {
		t.Skip("Skipping as remote can't Move")
	}

	file1 := r.WriteObject(ctx, "sub dir/test remove existing", "hello world", t1)
	file2 := r.WriteObject(ctx, "sub dir/test remove existing with long name 123456789012345678901234567890123456789012345678901234567890123456789", "hello long name world", t1)

	r.CheckRemoteItems(t, file1, file2)

	var returnedError error

	// Check not found first
	cleanup, err := operations.RemoveExisting(ctx, r.Fremote, "not found", "TEST")
	assert.Equal(t, err, nil)
	r.CheckRemoteItems(t, file1, file2)
	cleanup(&returnedError)
	r.CheckRemoteItems(t, file1, file2)

	// Remove file1
	cleanup, err = operations.RemoveExisting(ctx, r.Fremote, file1.Path, "TEST")
	assert.Equal(t, err, nil)
	//r.CheckRemoteItems(t, file1, file2)

	// Check file1 with temporary name exists
	var buf bytes.Buffer
	err = operations.List(ctx, r.Fremote, &buf)
	require.NoError(t, err)
	res := buf.String()
	assert.NotContains(t, res, "       11 "+file1.Path+"\n")
	assert.Contains(t, res, "       11 "+file1.Path+".")
	assert.Contains(t, res, "       21 "+file2.Path+"\n")

	cleanup(&returnedError)
	r.CheckRemoteItems(t, file2)

	// Remove file2 with an error
	cleanup, err = operations.RemoveExisting(ctx, r.Fremote, file2.Path, "TEST")
	assert.Equal(t, err, nil)

	// Check file2 with truncated temporary name exists
	buf.Reset()
	err = operations.List(ctx, r.Fremote, &buf)
	require.NoError(t, err)
	res = buf.String()
	assert.NotContains(t, res, "       21 "+file2.Path+"\n")
	assert.NotContains(t, res, "       21 "+file2.Path+".")
	assert.Contains(t, res, "       21 "+file2.Path[:100])

	returnedError = errors.New("BOOM")
	cleanup(&returnedError)
	r.CheckRemoteItems(t, file2)

	// Remove file2
	cleanup, err = operations.RemoveExisting(ctx, r.Fremote, file2.Path, "TEST")
	assert.Equal(t, err, nil)

	// Check file2 with truncated temporary name exists
	buf.Reset()
	err = operations.List(ctx, r.Fremote, &buf)
	require.NoError(t, err)
	res = buf.String()
	assert.NotContains(t, res, "       21 "+file2.Path+"\n")
	assert.NotContains(t, res, "       21 "+file2.Path+".")
	assert.Contains(t, res, "       21 "+file2.Path[:100])

	returnedError = nil
	cleanup(&returnedError)
	r.CheckRemoteItems(t)
}
