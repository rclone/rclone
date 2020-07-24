package operations_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/readers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCheck(t *testing.T, checkFunction func(ctx context.Context, opt *operations.CheckOpt) error) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	addBuffers := func(opt *operations.CheckOpt) {
		opt.Combined = new(bytes.Buffer)
		opt.MissingOnSrc = new(bytes.Buffer)
		opt.MissingOnDst = new(bytes.Buffer)
		opt.Match = new(bytes.Buffer)
		opt.Differ = new(bytes.Buffer)
		opt.Error = new(bytes.Buffer)
	}

	sortLines := func(in string) []string {
		if in == "" {
			return []string{}
		}
		lines := strings.Split(in, "\n")
		sort.Strings(lines)
		return lines
	}

	checkBuffer := func(name string, want map[string]string, out io.Writer) {
		expected := want[name]
		buf, ok := out.(*bytes.Buffer)
		require.True(t, ok)
		assert.Equal(t, sortLines(expected), sortLines(buf.String()), name)
	}

	checkBuffers := func(opt *operations.CheckOpt, want map[string]string) {
		checkBuffer("combined", want, opt.Combined)
		checkBuffer("missingonsrc", want, opt.MissingOnSrc)
		checkBuffer("missingondst", want, opt.MissingOnDst)
		checkBuffer("match", want, opt.Match)
		checkBuffer("differ", want, opt.Differ)
		checkBuffer("error", want, opt.Error)
	}

	check := func(i int, wantErrors int64, wantChecks int64, oneway bool, wantOutput map[string]string) {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			accounting.GlobalStats().ResetCounters()
			var buf bytes.Buffer
			log.SetOutput(&buf)
			defer func() {
				log.SetOutput(os.Stderr)
			}()
			opt := operations.CheckOpt{
				Fdst:   r.Fremote,
				Fsrc:   r.Flocal,
				OneWay: oneway,
			}
			addBuffers(&opt)
			err := checkFunction(context.Background(), &opt)
			gotErrors := accounting.GlobalStats().GetErrors()
			gotChecks := accounting.GlobalStats().GetChecks()
			if wantErrors == 0 && err != nil {
				t.Errorf("%d: Got error when not expecting one: %v", i, err)
			}
			if wantErrors != 0 && err == nil {
				t.Errorf("%d: No error when expecting one", i)
			}
			if wantErrors != gotErrors {
				t.Errorf("%d: Expecting %d errors but got %d", i, wantErrors, gotErrors)
			}
			if gotChecks > 0 && !strings.Contains(buf.String(), "matching files") {
				t.Errorf("%d: Total files matching line missing", i)
			}
			if wantChecks != gotChecks {
				t.Errorf("%d: Expecting %d total matching files but got %d", i, wantChecks, gotChecks)
			}
			checkBuffers(&opt, wantOutput)
		})
	}

	file1 := r.WriteBoth(context.Background(), "rutabaga", "is tasty", t3)
	fstest.CheckItems(t, r.Fremote, file1)
	fstest.CheckItems(t, r.Flocal, file1)
	check(1, 0, 1, false, map[string]string{
		"combined":     "= rutabaga\n",
		"missingonsrc": "",
		"missingondst": "",
		"match":        "rutabaga\n",
		"differ":       "",
		"error":        "",
	})

	file2 := r.WriteFile("potato2", "------------------------------------------------------------", t1)
	fstest.CheckItems(t, r.Flocal, file1, file2)
	check(2, 1, 1, false, map[string]string{
		"combined":     "+ potato2\n= rutabaga\n",
		"missingonsrc": "",
		"missingondst": "potato2\n",
		"match":        "rutabaga\n",
		"differ":       "",
		"error":        "",
	})

	file3 := r.WriteObject(context.Background(), "empty space", "-", t2)
	fstest.CheckItems(t, r.Fremote, file1, file3)
	check(3, 2, 1, false, map[string]string{
		"combined":     "- empty space\n+ potato2\n= rutabaga\n",
		"missingonsrc": "empty space\n",
		"missingondst": "potato2\n",
		"match":        "rutabaga\n",
		"differ":       "",
		"error":        "",
	})

	file2r := file2
	if fs.Config.SizeOnly {
		file2r = r.WriteObject(context.Background(), "potato2", "--Some-Differences-But-Size-Only-Is-Enabled-----------------", t1)
	} else {
		r.WriteObject(context.Background(), "potato2", "------------------------------------------------------------", t1)
	}
	fstest.CheckItems(t, r.Fremote, file1, file2r, file3)
	check(4, 1, 2, false, map[string]string{
		"combined":     "- empty space\n= potato2\n= rutabaga\n",
		"missingonsrc": "empty space\n",
		"missingondst": "",
		"match":        "rutabaga\npotato2\n",
		"differ":       "",
		"error":        "",
	})

	file3r := file3
	file3l := r.WriteFile("empty space", "DIFFER", t2)
	fstest.CheckItems(t, r.Flocal, file1, file2, file3l)
	check(5, 1, 3, false, map[string]string{
		"combined":     "* empty space\n= potato2\n= rutabaga\n",
		"missingonsrc": "",
		"missingondst": "",
		"match":        "potato2\nrutabaga\n",
		"differ":       "empty space\n",
		"error":        "",
	})

	file4 := r.WriteObject(context.Background(), "remotepotato", "------------------------------------------------------------", t1)
	fstest.CheckItems(t, r.Fremote, file1, file2r, file3r, file4)
	check(6, 2, 3, false, map[string]string{
		"combined":     "* empty space\n= potato2\n= rutabaga\n- remotepotato\n",
		"missingonsrc": "remotepotato\n",
		"missingondst": "",
		"match":        "potato2\nrutabaga\n",
		"differ":       "empty space\n",
		"error":        "",
	})
	check(7, 1, 3, true, map[string]string{
		"combined":     "* empty space\n= potato2\n= rutabaga\n",
		"missingonsrc": "",
		"missingondst": "",
		"match":        "potato2\nrutabaga\n",
		"differ":       "empty space\n",
		"error":        "",
	})
}

func TestCheck(t *testing.T) {
	testCheck(t, operations.Check)
}

func TestCheckFsError(t *testing.T) {
	dstFs, err := fs.NewFs("non-existent")
	if err != nil {
		t.Fatal(err)
	}
	srcFs, err := fs.NewFs("non-existent")
	if err != nil {
		t.Fatal(err)
	}
	opt := operations.CheckOpt{
		Fdst:   dstFs,
		Fsrc:   srcFs,
		OneWay: false,
	}
	err = operations.Check(context.Background(), &opt)
	require.Error(t, err)
}

func TestCheckDownload(t *testing.T) {
	testCheck(t, operations.CheckDownload)
}

func TestCheckSizeOnly(t *testing.T) {
	fs.Config.SizeOnly = true
	defer func() { fs.Config.SizeOnly = false }()
	TestCheck(t)
}

func TestCheckEqualReaders(t *testing.T) {
	b65a := make([]byte, 65*1024)
	b65b := make([]byte, 65*1024)
	b65b[len(b65b)-1] = 1
	b66 := make([]byte, 66*1024)

	differ, err := operations.CheckEqualReaders(bytes.NewBuffer(b65a), bytes.NewBuffer(b65a))
	assert.NoError(t, err)
	assert.Equal(t, differ, false)

	differ, err = operations.CheckEqualReaders(bytes.NewBuffer(b65a), bytes.NewBuffer(b65b))
	assert.NoError(t, err)
	assert.Equal(t, differ, true)

	differ, err = operations.CheckEqualReaders(bytes.NewBuffer(b65a), bytes.NewBuffer(b66))
	assert.NoError(t, err)
	assert.Equal(t, differ, true)

	differ, err = operations.CheckEqualReaders(bytes.NewBuffer(b66), bytes.NewBuffer(b65a))
	assert.NoError(t, err)
	assert.Equal(t, differ, true)

	myErr := errors.New("sentinel")
	wrap := func(b []byte) io.Reader {
		r := bytes.NewBuffer(b)
		e := readers.ErrorReader{Err: myErr}
		return io.MultiReader(r, e)
	}

	differ, err = operations.CheckEqualReaders(wrap(b65a), bytes.NewBuffer(b65a))
	assert.Equal(t, myErr, err)
	assert.Equal(t, differ, true)

	differ, err = operations.CheckEqualReaders(wrap(b65a), bytes.NewBuffer(b65b))
	assert.Equal(t, myErr, err)
	assert.Equal(t, differ, true)

	differ, err = operations.CheckEqualReaders(wrap(b65a), bytes.NewBuffer(b66))
	assert.Equal(t, myErr, err)
	assert.Equal(t, differ, true)

	differ, err = operations.CheckEqualReaders(wrap(b66), bytes.NewBuffer(b65a))
	assert.Equal(t, myErr, err)
	assert.Equal(t, differ, true)

	differ, err = operations.CheckEqualReaders(bytes.NewBuffer(b65a), wrap(b65a))
	assert.Equal(t, myErr, err)
	assert.Equal(t, differ, true)

	differ, err = operations.CheckEqualReaders(bytes.NewBuffer(b65a), wrap(b65b))
	assert.Equal(t, myErr, err)
	assert.Equal(t, differ, true)

	differ, err = operations.CheckEqualReaders(bytes.NewBuffer(b65a), wrap(b66))
	assert.Equal(t, myErr, err)
	assert.Equal(t, differ, true)

	differ, err = operations.CheckEqualReaders(bytes.NewBuffer(b66), wrap(b65a))
	assert.Equal(t, myErr, err)
	assert.Equal(t, differ, true)
}
