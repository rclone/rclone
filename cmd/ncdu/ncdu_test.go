//go:build !plan9 && !js && !aix

package ncdu

import (
	"context"
	"path"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	_ "github.com/rclone/rclone/backend/local"
	_ "github.com/rclone/rclone/backend/memory"
	"github.com/rclone/rclone/cmd/ncdu/scan"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var t1 = fstest.Time("2020-01-02T03:04:05.000000000Z")

// TestMain drives the tests
func TestMain(m *testing.M) {
	fstest.TestMain(m)
}

// newTestUI creates a UI backed by an 80×24 tcell SimulationScreen.
func newTestUI(t *testing.T, f fs.Fs) (*UI, tcell.SimulationScreen) {
	t.Helper()
	ss := tcell.NewSimulationScreen("UTF-8")
	require.NoError(t, ss.Init())
	ss.SetSize(80, 24)
	u := NewUI(f)
	u.s = ss
	return u, ss
}

// setupDir scans f, marks the scan as complete, and sets it as the UI's
// current directory. It returns the scanned root.
func setupDir(t *testing.T, u *UI, f fs.Fs) *scan.Dir {
	t.Helper()
	rootChan, errChan, updated := scan.Scan(context.Background(), f)
	var root *scan.Dir
	for {
		select {
		case r := <-rootChan:
			root = r
		case err := <-errChan:
			require.NoError(t, err)
			// rootChan and errChan may both be ready simultaneously;
			// Go's select picks randomly, so drain rootChan if we
			// haven't received it yet.
			if root == nil {
				root = <-rootChan
			}
			u.root = root
			u.listing = false
			u.setCurrentDir(root)
			return root
		case <-updated:
		}
	}
}

// cursorName returns the filename of the entry currently under the cursor.
func cursorName(u *UI) string {
	pos := u.dirPosMap[u.path]
	return path.Base(u.entries[u.sortPerm[pos.entry]].Remote())
}

// assertHeader checks row 0 (the top bar containing "rclone ncdu").
func assertHeader(t *testing.T, rows []string) {
	t.Helper()
	assert.Contains(t, rows[0], "rclone ncdu")
}

// assertPathBar checks row 1 (the dashed path line).
func assertPathBar(t *testing.T, rows []string, want string) {
	t.Helper()
	assert.Contains(t, rows[1], want)
}

// assertFooter checks the last row (the status bar).
func assertFooter(t *testing.T, rows []string, want string) {
	t.Helper()
	assert.Contains(t, rows[len(rows)-1], want)
}

// draw flushes the UI to the simulation screen and returns the screen contents
// as a slice of rows (trailing spaces stripped).
func draw(t *testing.T, u *UI, ss tcell.SimulationScreen) []string {
	t.Helper()
	u.Draw()
	ss.Show()
	cells, w, h := ss.GetContents()
	rows := make([]string, h)
	for y := range h {
		var sb strings.Builder
		for x := range w {
			cell := cells[y*w+x]
			r := ' '
			if len(cell.Runes) > 0 {
				r = cell.Runes[0]
			}
			sb.WriteRune(r)
		}
		rows[y] = strings.TrimRight(sb.String(), " ")
	}
	require.GreaterOrEqual(t, len(rows), 2)
	return rows
}

// TestNewUI checks that NewUI initialises sensible defaults.
func TestNewUI(t *testing.T) {
	r := fstest.NewRun(t)
	u := NewUI(r.Fremote)

	assert.Equal(t, r.Fremote, u.f)
	assert.True(t, u.showGraph)
	assert.True(t, u.humanReadable)
	assert.Equal(t, int8(1), u.sortBySize, "should sort by size (largest first) by default")
	assert.NotNil(t, u.dirPosMap)
	assert.NotNil(t, u.selectedEntries)
}

// TestDrawWaiting checks the initial "waiting" state before any directory has been scanned.
func TestDrawWaiting(t *testing.T) {
	r := fstest.NewRun(t)
	u, ss := newTestUI(t, r.Fremote)
	defer ss.Fini()

	rows := draw(t, u, ss)
	assertHeader(t, rows)
	assertPathBar(t, rows, "Waiting for root...")
	assertFooter(t, rows, "Waiting for root directory...")
}

// TestDrawWithDirectory checks the screen once a directory has been scanned.
func TestDrawWithDirectory(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	r.WriteObject(ctx, "bigfile.bin", strings.Repeat("x", 1024), t1)
	r.WriteObject(ctx, "small.txt", "hi", t1)
	r.WriteObject(ctx, "subdir/nested.txt", "nested", t1)

	u, ss := newTestUI(t, r.Fremote)
	defer ss.Fini()

	setupDir(t, u, r.Fremote)
	rows := draw(t, u, ss)

	assertHeader(t, rows)
	assertPathBar(t, rows, fs.ConfigString(r.Fremote))

	joined := strings.Join(rows, "\n")
	assert.Contains(t, joined, "bigfile.bin")
	assert.Contains(t, joined, "small.txt")
	assert.Contains(t, joined, "subdir")

	assertFooter(t, rows, "Total usage: 1.008Ki, Objects: 3")
}

// TestDrawListingInProgress checks that the "listing in progress" marker
// appears in the footer while scanning is still running.
func TestDrawListingInProgress(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	r.WriteObject(ctx, "a.txt", "aaa", t1)

	u, ss := newTestUI(t, r.Fremote)
	defer ss.Fini()

	setupDir(t, u, r.Fremote)
	u.listing = true // simulate still-scanning state

	rows := draw(t, u, ss)
	assertHeader(t, rows)
	assertPathBar(t, rows, fs.ConfigString(r.Fremote))
	assertFooter(t, rows, "listing in progress")
}

// TestMove exercises cursor navigation.
func TestMove(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	r.WriteObject(ctx, "file1.txt", "aaa", t1)
	r.WriteObject(ctx, "file2.txt", "bbbbb", t1)
	r.WriteObject(ctx, "subdir/child.txt", "cc", t1)

	u, ss := newTestUI(t, r.Fremote)
	defer ss.Fini()

	setupDir(t, u, r.Fremote)
	initialPath := u.path

	u.move(1)
	assert.Equal(t, "file1.txt", cursorName(u))

	u.move(-1)
	assert.Equal(t, "file2.txt", cursorName(u))

	assert.Equal(t, initialPath, u.path, "path should not change when navigating within a directory")
}

// TestUp checks that up() navigates to the parent directory.
func TestUp(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	r.WriteObject(ctx, "subdir/file.txt", "hello", t1)

	u, ss := newTestUI(t, r.Fremote)
	defer ss.Fini()

	root := setupDir(t, u, r.Fremote)

	// Navigate into the first subdirectory.
	var subDir *scan.Dir
	for i := range root.Entries() {
		if d, _ := root.GetDir(i); d != nil {
			subDir = d
			break
		}
	}
	require.NotNil(t, subDir)
	u.setCurrentDir(subDir)

	u.up()
	assert.Equal(t, root, u.d, "up() should return to the root directory")
}

// TestToggleSort checks that toggleSort cycles +1 → -1 → +1 and clears
// other sort fields when a new one is activated.
func TestToggleSort(t *testing.T) {
	r := fstest.NewRun(t)
	u := NewUI(r.Fremote)

	assert.Equal(t, int8(1), u.sortBySize)

	u.toggleSort(&u.sortByName)
	assert.Equal(t, int8(1), u.sortByName)
	assert.Equal(t, int8(0), u.sortBySize, "activating name sort should clear size sort")

	u.toggleSort(&u.sortByName)
	assert.Equal(t, int8(-1), u.sortByName)

	u.toggleSort(&u.sortByName)
	assert.Equal(t, int8(1), u.sortByName)
}

// TestPopupBox checks that popupBox and togglePopupBox manage showBox correctly.
func TestPopupBox(t *testing.T) {
	r := fstest.NewRun(t)
	u := NewUI(r.Fremote)

	assert.False(t, u.showBox)

	u.popupBox([]string{"line one", "line two"})
	assert.True(t, u.showBox)
	assert.Equal(t, []string{"line one", "line two"}, u.boxText)

	u.togglePopupBox([]string{"line one", "line two"}) // same text → dismiss
	assert.False(t, u.showBox)

	u.togglePopupBox([]string{"different"}) // different text → show
	assert.True(t, u.showBox)
	assert.Equal(t, []string{"different"}, u.boxText)
}

// TestBoxRendered checks that box text appears on screen when showBox is true.
func TestBoxRendered(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	r.WriteObject(ctx, "a.txt", "a", t1)

	u, ss := newTestUI(t, r.Fremote)
	defer ss.Fini()

	setupDir(t, u, r.Fremote)
	u.popupBox([]string{"Hello Box", "second line"})

	joined := strings.Join(draw(t, u, ss), "\n")
	assert.Contains(t, joined, "Hello Box")
	assert.Contains(t, joined, "second line")
}

// TestHelpText ensures helpText returns a non-empty slice with the title and key bindings.
func TestHelpText(t *testing.T) {
	h := helpText()
	require.NotEmpty(t, h)
	assert.Equal(t, "rclone ncdu", h[0])
	joined := strings.Join(h, "\n")
	assert.Contains(t, joined, "↑")
	assert.Contains(t, joined, "↓")
	assert.Contains(t, joined, "delete")
}

// TestSortCurrentDir checks that sortCurrentDir produces a valid permutation.
func TestSortCurrentDir(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	r.WriteObject(ctx, "c.txt", "ccc", t1)
	r.WriteObject(ctx, "a.txt", "a", t1)
	r.WriteObject(ctx, "b.txt", "bb", t1)

	u, ss := newTestUI(t, r.Fremote)
	defer ss.Fini()

	setupDir(t, u, r.Fremote)

	n := len(u.entries)
	require.Equal(t, n, len(u.sortPerm))

	seen := make([]bool, n)
	for _, p := range u.sortPerm {
		require.Less(t, p, n)
		require.False(t, seen[p], "duplicate index %d in sortPerm", p)
		seen[p] = true
	}
}

// TestVisualSelectMode checks that moving in visual select mode auto-selects entries.
func TestVisualSelectMode(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	r.WriteObject(ctx, "x.txt", "x", t1)
	r.WriteObject(ctx, "y.txt", "yy", t1)
	r.WriteObject(ctx, "z.txt", "zzz", t1)

	u, ss := newTestUI(t, r.Fremote)
	defer ss.Fini()

	setupDir(t, u, r.Fremote)
	u.visualSelectMode = true
	u.move(1)
	assert.Equal(t, 1, len(u.selectedEntries))
}
