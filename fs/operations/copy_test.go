package operations_test

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTruncateString(t *testing.T) {
	for _, test := range []struct {
		in   string
		n    int
		want string
	}{
		{
			in:   "",
			n:    0,
			want: "",
		}, {
			in:   "Hello World",
			n:    5,
			want: "Hello",
		}, {
			in:   "ááááá",
			n:    5,
			want: "áá",
		}, {
			in:   "ááááá\xFF\xFF",
			n:    5,
			want: "áá\xc3",
		}, {
			in:   "世世世世世",
			n:    7,
			want: "世世",
		}, {
			in:   "🙂🙂🙂🙂🙂",
			n:    16,
			want: "🙂🙂🙂🙂",
		}, {
			in:   "🙂🙂🙂🙂🙂",
			n:    15,
			want: "🙂🙂🙂",
		}, {
			in:   "🙂🙂🙂🙂🙂",
			n:    14,
			want: "🙂🙂🙂",
		}, {
			in:   "🙂🙂🙂🙂🙂",
			n:    13,
			want: "🙂🙂🙂",
		}, {
			in:   "🙂🙂🙂🙂🙂",
			n:    12,
			want: "🙂🙂🙂",
		}, {
			in:   "🙂🙂🙂🙂🙂",
			n:    11,
			want: "🙂🙂",
		}, {
			in:   "𝓝𝓸𝓫𝓸𝓭𝔂 𝓲𝓼 𝓱𝓸𝓶𝓮 ᴬ ⱽⁱˢⁱᵗ ᶠʳᵒᵐ ᵗʰᵉ ⱽⁱˢⁱᵒⁿᵃʳʸ",
			n:    100,
			want: "𝓝𝓸𝓫𝓸𝓭𝔂 𝓲𝓼 𝓱𝓸𝓶𝓮 ᴬ ⱽⁱˢⁱᵗ ᶠʳᵒᵐ ᵗʰᵉ ⱽⁱˢ",
		}, {
			in:   "a𝓝𝓸𝓫𝓸𝓭𝔂 𝓲𝓼 𝓱𝓸𝓶𝓮 ᴬ ⱽⁱˢⁱᵗ ᶠʳᵒᵐ ᵗʰᵉ ⱽⁱˢⁱᵒⁿᵃʳʸ",
			n:    100,
			want: "a𝓝𝓸𝓫𝓸𝓭𝔂 𝓲𝓼 𝓱𝓸𝓶𝓮 ᴬ ⱽⁱˢⁱᵗ ᶠʳᵒᵐ ᵗʰᵉ ⱽⁱˢ",
		}, {
			in:   "aa𝓝𝓸𝓫𝓸𝓭𝔂 𝓲𝓼 𝓱𝓸𝓶𝓮 ᴬ ⱽⁱˢⁱᵗ ᶠʳᵒᵐ ᵗʰᵉ ⱽⁱˢⁱᵒⁿᵃʳʸ",
			n:    100,
			want: "aa𝓝𝓸𝓫𝓸𝓭𝔂 𝓲𝓼 𝓱𝓸𝓶𝓮 ᴬ ⱽⁱˢⁱᵗ ᶠʳᵒᵐ ᵗʰᵉ ⱽⁱ",
		}, {
			in:   "aaa𝓝𝓸𝓫𝓸𝓭𝔂 𝓲𝓼 𝓱𝓸𝓶𝓮 ᴬ ⱽⁱˢⁱᵗ ᶠʳᵒᵐ ᵗʰᵉ ⱽⁱˢⁱᵒⁿᵃʳʸ",
			n:    100,
			want: "aaa𝓝𝓸𝓫𝓸𝓭𝔂 𝓲𝓼 𝓱𝓸𝓶𝓮 ᴬ ⱽⁱˢⁱᵗ ᶠʳᵒᵐ ᵗʰᵉ ⱽⁱ",
		}, {
			in:   "aaaa𝓝𝓸𝓫𝓸𝓭𝔂 𝓲𝓼 𝓱𝓸𝓶𝓮 ᴬ ⱽⁱˢⁱᵗ ᶠʳᵒᵐ ᵗʰᵉ ⱽⁱˢⁱᵒⁿᵃʳʸ",
			n:    100,
			want: "aaaa𝓝𝓸𝓫𝓸𝓭𝔂 𝓲𝓼 𝓱𝓸𝓶𝓮 ᴬ ⱽⁱˢⁱᵗ ᶠʳᵒᵐ ᵗʰᵉ ⱽ",
		},
	} {
		got := operations.TruncateString(test.in, test.n)
		assert.Equal(t, test.want, got, fmt.Sprintf("In %q", test.in))
		assert.LessOrEqual(t, len(got), test.n)
		assert.GreaterOrEqual(t, len(got), test.n-3)
	}
}

func TestCopyFile(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)

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

// Find the longest file name for writing to local
func maxLengthFileName(t *testing.T, r *fstest.Run) string {
	require.NoError(t, r.Flocal.Mkdir(context.Background(), "")) // create the root
	const maxLen = 16 * 1024
	name := strings.Repeat("A", maxLen)
	i := sort.Search(len(name), func(i int) (fail bool) {
		filePath := path.Join(r.LocalName, name[:i])
		err := os.WriteFile(filePath, []byte{0}, 0777)
		if err != nil {
			return true
		}
		err = os.Remove(filePath)
		if err != nil {
			t.Logf("Failed to remove test file: %v", err)
		}
		return false
	})
	return name[:i-1]
}

// Check we can copy a file of maximum name length
func TestCopyLongFile(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	if !r.Fremote.Features().IsLocal {
		t.Skip("Test only runs on local")
	}

	// Find the maximum length of file we can write
	name := maxLengthFileName(t, r)
	t.Logf("Max length of file name is %d", len(name))
	file1 := r.WriteFile(name, "file1 contents", t1)
	r.CheckLocalItems(t, file1)

	err := operations.CopyFile(ctx, r.Fremote, r.Flocal, file1.Path, file1.Path)
	require.NoError(t, err)
	r.CheckLocalItems(t, file1)
	r.CheckRemoteItems(t, file1)
}

func TestCopyFileBackupDir(t *testing.T) {
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

func TestCopyInplace(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)

	if !r.Fremote.Features().PartialUploads {
		t.Skip("Partial uploads not supported")
	}

	ci.Inplace = true

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

func TestCopyLongFileName(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)

	if !r.Fremote.Features().PartialUploads {
		t.Skip("Partial uploads not supported")
	}

	ci.Inplace = false // the default

	file1 := r.WriteFile("file1", "file1 contents", t1)
	r.CheckLocalItems(t, file1)

	file2 := file1
	file2.Path = "sub/" + strings.Repeat("file2", 30)

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

func TestCopyFileMaxTransfer(t *testing.T) {
	ctx := context.Background()
	ctx, ci := fs.AddConfig(ctx)
	r := fstest.NewRun(t)
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

	if runtime.GOOS == "darwin" {
		// disable server-side copies as they don't count towards transfer size stats
		r.Flocal.Features().Disable("Copy")
		if r.Fremote.Features().IsLocal {
			r.Fremote.Features().Disable("Copy")
		}
	}

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
	if !errors.Is(err, accounting.ErrorMaxTransferLimitReachedFatal) {
		t.Log("Expecting error to contain accounting.ErrorMaxTransferLimitReachedFatal")
		// Sometimes the backends or their SDKs don't pass the
		// error through properly, so check that it at least
		// has the text we expect in.
		assert.Contains(t, err.Error(), "max transfer limit reached")
	}
	r.CheckLocalItems(t, file1, file2, file3, file4)
	r.CheckRemoteItems(t, file1)

	// Cutoff mode: Cautious
	ci.CutoffMode = fs.CutoffModeCautious

	// file3: show a large file does not get transferred
	accounting.Stats(ctx).ResetCounters()
	err = operations.CopyFile(ctx, r.Fremote, r.Flocal, file3.Path, file3.Path)
	require.NotNil(t, err)
	assert.True(t, errors.Is(err, accounting.ErrorMaxTransferLimitReachedGraceful))
	r.CheckLocalItems(t, file1, file2, file3, file4)
	r.CheckRemoteItems(t, file1)

	if isChunker(r.Fremote) {
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
