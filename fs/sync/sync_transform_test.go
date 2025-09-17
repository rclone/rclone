// Test transform

package sync

import (
	"cmp"
	"context"
	"fmt"
	"path"
	"slices"
	"strings"
	"testing"

	_ "github.com/rclone/rclone/backend/all"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/transform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/unicode/norm"
)

var debug = ``

func TestTransform(t *testing.T) {
	type args struct {
		TransformOpt     []string
		TransformBackOpt []string
		Lossless         bool // whether the TransformBackAlgo is always losslessly invertible
	}
	tests := []struct {
		name string
		args args
	}{
		{name: "NFC", args: args{
			TransformOpt:     []string{"nfc"},
			TransformBackOpt: []string{"nfd"},
			Lossless:         false,
		}},
		{name: "NFD", args: args{
			TransformOpt:     []string{"nfd"},
			TransformBackOpt: []string{"nfc"},
			Lossless:         false,
		}},
		{name: "base64", args: args{
			TransformOpt:     []string{"base64encode"},
			TransformBackOpt: []string{"base64encode"},
			Lossless:         false,
		}},
		{name: "prefix", args: args{
			TransformOpt:     []string{"prefix=PREFIX"},
			TransformBackOpt: []string{"trimprefix=PREFIX"},
			Lossless:         true,
		}},
		{name: "suffix", args: args{
			TransformOpt:     []string{"suffix=SUFFIX"},
			TransformBackOpt: []string{"trimsuffix=SUFFIX"},
			Lossless:         true,
		}},
		{name: "truncate", args: args{
			TransformOpt:     []string{"truncate=10"},
			TransformBackOpt: []string{"truncate=10"},
			Lossless:         false,
		}},
		{name: "encoder", args: args{
			TransformOpt:     []string{"encoder=Colon,SquareBracket"},
			TransformBackOpt: []string{"decoder=Colon,SquareBracket"},
			Lossless:         true,
		}},
		{name: "ISO-8859-1", args: args{
			TransformOpt:     []string{"ISO-8859-1"},
			TransformBackOpt: []string{"ISO-8859-1"},
			Lossless:         false,
		}},
		{name: "charmap", args: args{
			TransformOpt:     []string{"all,charmap=ISO-8859-7"},
			TransformBackOpt: []string{"all,charmap=ISO-8859-7"},
			Lossless:         false,
		}},
		{name: "lowercase", args: args{
			TransformOpt:     []string{"all,lowercase"},
			TransformBackOpt: []string{"all,lowercase"},
			Lossless:         false,
		}},
		{name: "ascii", args: args{
			TransformOpt:     []string{"all,ascii"},
			TransformBackOpt: []string{"all,ascii"},
			Lossless:         false,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := fstest.NewRun(t)
			defer r.Finalise()

			ctx := context.Background()
			r.Mkdir(ctx, r.Flocal)
			r.Mkdir(ctx, r.Fremote)
			items := makeTestFiles(t, r, "dir1")
			deleteDSStore(t, r)
			r.CheckRemoteListing(t, items, nil)
			r.CheckLocalListing(t, items, nil)

			err := transform.SetOptions(ctx, tt.args.TransformOpt...)
			require.NoError(t, err)

			err = Sync(ctx, r.Fremote, r.Flocal, true)
			assert.NoError(t, err)
			compareNames(ctx, t, r, items)

			err = transform.SetOptions(ctx, tt.args.TransformBackOpt...)
			require.NoError(t, err)
			err = Sync(ctx, r.Fremote, r.Flocal, true)
			assert.NoError(t, err)
			compareNames(ctx, t, r, items)

			if tt.args.Lossless {
				deleteDSStore(t, r)
				r.CheckRemoteItems(t, items...)
			}
		})
	}
}

const alphabet = "abcdefg123456789"

var extras = []string{"apple", "banana", "appleappleapplebanana", "splitbananasplit"}

func makeTestFiles(t *testing.T, r *fstest.Run, dir string) []fstest.Item {
	t.Helper()
	n := 0
	// Create test files
	items := []fstest.Item{}
	for _, c := range alphabet {
		var out strings.Builder
		for i := range rune(7) {
			out.WriteRune(c + i)
		}
		fileName := path.Join(dir, fmt.Sprintf("%04d-%s.txt", n, out.String()))
		fileName = strings.ToValidUTF8(fileName, "")
		fileName = strings.NewReplacer(":", "", "<", "", ">", "", "?", "").Replace(fileName) // remove characters illegal on windows

		if debug != "" {
			fileName = debug
		}

		item := r.WriteObject(context.Background(), fileName, fileName, t1)
		r.WriteFile(fileName, fileName, t1)
		items = append(items, item)
		n++

		if debug != "" {
			break
		}
	}

	for _, extra := range extras {
		item := r.WriteObject(context.Background(), extra, extra, t1)
		r.WriteFile(extra, extra, t1)
		items = append(items, item)
	}

	return items
}

func deleteDSStore(t *testing.T, r *fstest.Run) {
	ctxDSStore, fi := filter.AddConfig(context.Background())
	err := fi.AddRule(`+ *.DS_Store`)
	assert.NoError(t, err)
	err = fi.AddRule(`- **`)
	assert.NoError(t, err)
	err = operations.Delete(ctxDSStore, r.Fremote)
	assert.NoError(t, err)
}

func compareNames(ctx context.Context, t *testing.T, r *fstest.Run, items []fstest.Item) {
	var entries fs.DirEntries

	deleteDSStore(t, r)
	err := walk.ListR(context.Background(), r.Fremote, "", true, -1, walk.ListObjects, func(e fs.DirEntries) error {
		entries = append(entries, e...)
		return nil
	})
	assert.NoError(t, err)
	entries = slices.DeleteFunc(entries, func(E fs.DirEntry) bool { // remove those pesky .DS_Store files
		if strings.Contains(E.Remote(), ".DS_Store") {
			err := operations.DeleteFile(context.Background(), E.(fs.Object))
			assert.NoError(t, err)
			return true
		}
		return false
	})
	require.Equal(t, len(items), entries.Len())

	// sort by CONVERTED name
	slices.SortStableFunc(items, func(a, b fstest.Item) int {
		aConv := transform.Path(ctx, a.Path, false)
		bConv := transform.Path(ctx, b.Path, false)
		return cmp.Compare(aConv, bConv)
	})
	slices.SortStableFunc(entries, func(a, b fs.DirEntry) int {
		return cmp.Compare(a.Remote(), b.Remote())
	})

	for i, e := range entries {
		expect := transform.Path(ctx, items[i].Path, false)
		msg := fmt.Sprintf("expected %v, got %v", detectEncoding(expect), detectEncoding(e.Remote()))
		assert.Equal(t, expect, e.Remote(), msg)
	}
}

func detectEncoding(s string) string {
	if norm.NFC.IsNormalString(s) && norm.NFD.IsNormalString(s) {
		return "BOTH"
	}
	if !norm.NFC.IsNormalString(s) && norm.NFD.IsNormalString(s) {
		return "NFD"
	}
	if norm.NFC.IsNormalString(s) && !norm.NFD.IsNormalString(s) {
		return "NFC"
	}
	return "OTHER"
}

func TestTransformCopy(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	err := transform.SetOptions(ctx, "all,suffix_keep_extension=_somesuffix")
	require.NoError(t, err)
	file1 := r.WriteFile("sub dir/hello world.txt", "hello world", t1)

	r.Mkdir(ctx, r.Fremote)
	ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, r.Fremote, r.Flocal, true)
	testLoggerVsLsf(ctx, r.Fremote, r.Flocal, operations.GetLoggerOpt(ctx).JSON, t)
	require.NoError(t, err)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, fstest.NewItem("sub dir_somesuffix/hello world_somesuffix.txt", "hello world", t1))
}

func TestDoubleTransform(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	err := transform.SetOptions(ctx, "all,prefix=tac", "all,prefix=tic")
	require.NoError(t, err)
	file1 := r.WriteFile("toe/toe", "hello world", t1)

	r.Mkdir(ctx, r.Fremote)
	ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, r.Fremote, r.Flocal, true)
	testLoggerVsLsf(ctx, r.Fremote, r.Flocal, operations.GetLoggerOpt(ctx).JSON, t)
	require.NoError(t, err)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, fstest.NewItem("tictactoe/tictactoe", "hello world", t1))
}

func TestFileTag(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	err := transform.SetOptions(ctx, "file,prefix=tac", "file,prefix=tic")
	require.NoError(t, err)
	file1 := r.WriteFile("toe/toe/toe", "hello world", t1)

	r.Mkdir(ctx, r.Fremote)
	ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, r.Fremote, r.Flocal, true)
	testLoggerVsLsf(ctx, r.Fremote, r.Flocal, operations.GetLoggerOpt(ctx).JSON, t)
	require.NoError(t, err)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, fstest.NewItem("toe/toe/tictactoe", "hello world", t1))
}

func TestNoTag(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	err := transform.SetOptions(ctx, "prefix=tac", "prefix=tic")
	require.NoError(t, err)
	file1 := r.WriteFile("toe/toe/toe", "hello world", t1)

	r.Mkdir(ctx, r.Fremote)
	ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, r.Fremote, r.Flocal, true)
	testLoggerVsLsf(ctx, r.Fremote, r.Flocal, operations.GetLoggerOpt(ctx).JSON, t)
	require.NoError(t, err)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, fstest.NewItem("toe/toe/tictactoe", "hello world", t1))
}

func TestDirTag(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	err := transform.SetOptions(ctx, "dir,prefix=tac", "dir,prefix=tic")
	require.NoError(t, err)
	r.WriteFile("toe/toe/toe.txt", "hello world", t1)
	_, err = operations.MkdirModTime(ctx, r.Flocal, "empty_dir", t1)
	require.NoError(t, err)

	r.Mkdir(ctx, r.Fremote)
	ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, r.Fremote, r.Flocal, true)
	testLoggerVsLsf(ctx, r.Fremote, r.Flocal, operations.GetLoggerOpt(ctx).JSON, t)
	require.NoError(t, err)

	r.CheckLocalListing(t, []fstest.Item{fstest.NewItem("toe/toe/toe.txt", "hello world", t1)}, []string{"empty_dir", "toe", "toe/toe"})
	r.CheckRemoteListing(t, []fstest.Item{fstest.NewItem("tictactoe/tictactoe/toe.txt", "hello world", t1)}, []string{"tictacempty_dir", "tictactoe", "tictactoe/tictactoe"})
}

func TestAllTag(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	err := transform.SetOptions(ctx, "all,prefix=tac", "all,prefix=tic")
	require.NoError(t, err)
	r.WriteFile("toe/toe/toe.txt", "hello world", t1)
	_, err = operations.MkdirModTime(ctx, r.Flocal, "empty_dir", t1)
	require.NoError(t, err)

	r.Mkdir(ctx, r.Fremote)
	ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, r.Fremote, r.Flocal, true)
	testLoggerVsLsf(ctx, r.Fremote, r.Flocal, operations.GetLoggerOpt(ctx).JSON, t)
	require.NoError(t, err)

	r.CheckLocalListing(t, []fstest.Item{fstest.NewItem("toe/toe/toe.txt", "hello world", t1)}, []string{"empty_dir", "toe", "toe/toe"})
	r.CheckRemoteListing(t, []fstest.Item{fstest.NewItem("tictactoe/tictactoe/tictactoe.txt", "hello world", t1)}, []string{"tictacempty_dir", "tictactoe", "tictactoe/tictactoe"})
	err = operations.Check(ctx, &operations.CheckOpt{Fsrc: r.Flocal, Fdst: r.Fremote}) // should not error even though dst has transformed names
	assert.NoError(t, err)
}

func TestRunTwice(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	err := transform.SetOptions(ctx, "dir,prefix=tac", "dir,prefix=tic")
	require.NoError(t, err)
	file1 := r.WriteFile("toe/toe/toe.txt", "hello world", t1)

	r.Mkdir(ctx, r.Fremote)
	ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, r.Fremote, r.Flocal, true)
	testLoggerVsLsf(ctx, r.Fremote, r.Flocal, operations.GetLoggerOpt(ctx).JSON, t)
	require.NoError(t, err)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, fstest.NewItem("tictactoe/tictactoe/toe.txt", "hello world", t1))

	// result should not change second time, since src is unchanged
	ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, r.Fremote, r.Flocal, true)
	testLoggerVsLsf(ctx, r.Fremote, r.Flocal, operations.GetLoggerOpt(ctx).JSON, t)
	require.NoError(t, err)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, fstest.NewItem("tictactoe/tictactoe/toe.txt", "hello world", t1))
}

func TestSyntax(t *testing.T) {
	ctx := context.Background()
	err := transform.SetOptions(ctx, "prefix")
	assert.Error(t, err) // should error as required value is missing

	err = transform.SetOptions(ctx, "banana")
	assert.Error(t, err) // should error as unrecognized option

	err = transform.SetOptions(ctx, "=123")
	assert.Error(t, err) // should error as required key is missing

	err = transform.SetOptions(ctx, "prefix=123")
	assert.NoError(t, err) // should not error
}

func TestConflicting(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	err := transform.SetOptions(ctx, "prefix=tac", "trimprefix=tac")
	require.NoError(t, err)
	file1 := r.WriteFile("toe/toe/toe", "hello world", t1)

	r.Mkdir(ctx, r.Fremote)
	ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, r.Fremote, r.Flocal, true)
	testLoggerVsLsf(ctx, r.Fremote, r.Flocal, operations.GetLoggerOpt(ctx).JSON, t)
	require.NoError(t, err)

	// should result in no change as prefix and trimprefix cancel out
	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, fstest.NewItem("toe/toe/toe", "hello world", t1))
}

func TestMove(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	err := transform.SetOptions(ctx, "all,prefix=tac", "all,prefix=tic")
	require.NoError(t, err)
	r.WriteFile("toe/toe/toe.txt", "hello world", t1)
	_, err = operations.MkdirModTime(ctx, r.Flocal, "empty_dir", t1)
	require.NoError(t, err)

	r.Mkdir(ctx, r.Fremote)
	ctx = predictDstFromLogger(ctx)
	err = MoveDir(ctx, r.Fremote, r.Flocal, true, true)
	testLoggerVsLsf(ctx, r.Fremote, r.Flocal, operations.GetLoggerOpt(ctx).JSON, t)
	require.NoError(t, err)

	r.CheckLocalListing(t, []fstest.Item{}, []string{})
	r.CheckRemoteListing(t, []fstest.Item{fstest.NewItem("tictactoe/tictactoe/tictactoe.txt", "hello world", t1)}, []string{"tictacempty_dir", "tictactoe", "tictactoe/tictactoe"})
}

func TestTransformFile(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	err := transform.SetOptions(ctx, "all,prefix=tac", "all,prefix=tic")
	require.NoError(t, err)
	r.WriteFile("toe/toe/toe.txt", "hello world", t1)
	_, err = operations.MkdirModTime(ctx, r.Flocal, "empty_dir", t1)
	require.NoError(t, err)

	r.Mkdir(ctx, r.Fremote)
	ctx = predictDstFromLogger(ctx)
	err = MoveDir(ctx, r.Fremote, r.Flocal, true, true)
	testLoggerVsLsf(ctx, r.Fremote, r.Flocal, operations.GetLoggerOpt(ctx).JSON, t)
	require.NoError(t, err)

	r.CheckLocalListing(t, []fstest.Item{}, []string{})
	r.CheckRemoteListing(t, []fstest.Item{fstest.NewItem("tictactoe/tictactoe/tictactoe.txt", "hello world", t1)}, []string{"tictacempty_dir", "tictactoe", "tictactoe/tictactoe"})

	err = transform.SetOptions(ctx, "all,trimprefix=tic", "all,trimprefix=tac")
	require.NoError(t, err)
	err = operations.TransformFile(ctx, r.Fremote, "tictactoe/tictactoe/tictactoe.txt")
	require.NoError(t, err)
	r.CheckLocalListing(t, []fstest.Item{}, []string{})
	r.CheckRemoteListing(t, []fstest.Item{fstest.NewItem("toe/toe/toe.txt", "hello world", t1)}, []string{"tictacempty_dir", "tictactoe", "tictactoe/tictactoe", "toe", "toe/toe"})
}

func TestManualTransformFile(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)

	r.Flocal.Features().DisableList([]string{"Copy", "Move"})
	r.Fremote.Features().DisableList([]string{"Copy", "Move"})

	err := transform.SetOptions(ctx, "all,prefix=tac", "all,prefix=tic")
	require.NoError(t, err)
	r.WriteFile("toe/toe/toe.txt", "hello world", t1)
	_, err = operations.MkdirModTime(ctx, r.Flocal, "empty_dir", t1)
	require.NoError(t, err)

	r.Mkdir(ctx, r.Fremote)
	ctx = predictDstFromLogger(ctx)
	err = MoveDir(ctx, r.Fremote, r.Flocal, true, true)
	testLoggerVsLsf(ctx, r.Fremote, r.Flocal, operations.GetLoggerOpt(ctx).JSON, t)
	require.NoError(t, err)

	r.CheckLocalListing(t, []fstest.Item{}, []string{})
	r.CheckRemoteListing(t, []fstest.Item{fstest.NewItem("tictactoe/tictactoe/tictactoe.txt", "hello world", t1)}, []string{"tictacempty_dir", "tictactoe", "tictactoe/tictactoe"})

	err = transform.SetOptions(ctx, "all,trimprefix=tic", "all,trimprefix=tac")
	require.NoError(t, err)
	err = operations.TransformFile(ctx, r.Fremote, "tictactoe/tictactoe/tictactoe.txt")
	require.NoError(t, err)
	r.CheckLocalListing(t, []fstest.Item{}, []string{})
	r.CheckRemoteListing(t, []fstest.Item{fstest.NewItem("toe/toe/toe.txt", "hello world", t1)}, []string{"tictacempty_dir", "tictactoe", "tictactoe/tictactoe", "toe", "toe/toe"})
}

func TestBase64(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	err := transform.SetOptions(ctx, "all,base64encode")
	require.NoError(t, err)
	file1 := r.WriteFile("toe/toe/toe.txt", "hello world", t1)

	r.Mkdir(ctx, r.Fremote)
	ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, r.Fremote, r.Flocal, true)
	testLoggerVsLsf(ctx, r.Fremote, r.Flocal, operations.GetLoggerOpt(ctx).JSON, t)
	require.NoError(t, err)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, fstest.NewItem("dG9l/dG9l/dG9lLnR4dA==", "hello world", t1))

	// round trip
	err = transform.SetOptions(ctx, "all,base64decode")
	require.NoError(t, err)
	ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, r.Flocal, r.Fremote, true)
	testLoggerVsLsf(ctx, r.Flocal, r.Fremote, operations.GetLoggerOpt(ctx).JSON, t)
	require.NoError(t, err)

	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, fstest.NewItem("dG9l/dG9l/dG9lLnR4dA==", "hello world", t1))
}

func TestError(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	err := transform.SetOptions(ctx, "all,prefix=ta/c") // has illegal character
	require.NoError(t, err)
	file1 := r.WriteFile("toe/toe/toe", "hello world", t1)

	r.Mkdir(ctx, r.Fremote)
	// ctx = predictDstFromLogger(ctx)
	err = Sync(ctx, r.Fremote, r.Flocal, true)
	// testLoggerVsLsf(ctx, r.Fremote, r.Flocal, operations.GetLoggerOpt(ctx).JSON, t)
	assert.Error(t, err)

	r.CheckLocalListing(t, []fstest.Item{file1}, []string{"toe", "toe/toe"})
	r.CheckRemoteListing(t, []fstest.Item{file1}, []string{"toe", "toe/toe"})
}
