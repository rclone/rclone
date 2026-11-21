//go:build !plan9 && !js && !aix

package genfilters

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/vt"
	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRemote makes a local remote with some files in
func newRemote(t *testing.T) fs.Fs {
	dir := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt", "docs/one.md", "docs/sub/deep.md", "music/song.mp3", "tmp/junk.tmp"} {
		p := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o777))
		require.NoError(t, os.WriteFile(p, []byte(name), 0o666))
	}
	f, err := fs.NewFs(context.Background(), dir)
	require.NoError(t, err)
	return f
}

// session drives the user interface on a mock terminal
type session struct {
	t    *testing.T
	mt   vt.MockTerm
	s    tcell.Screen
	done chan error
}

const width, height = 100, 20

// start runs Run with opt on f
func start(t *testing.T, f fs.Fs, opt Options) *session {
	mt := vt.NewMockTerm(vt.MockOptSize{X: width, Y: height})
	s, err := tcell.NewTerminfoScreenFromTty(mt)
	require.NoError(t, err)
	tcell.ShimScreen(s)
	ss := &session{t: t, mt: mt, s: s, done: make(chan error, 1)}
	go func() { ss.done <- Run(context.Background(), f, opt) }()
	ss.wait("press ? for help")
	return ss
}

// screen returns the text on the screen once it stops changing, so a
// check doesn't see it halfway through a redraw
func (ss *session) screen() string {
	text := ss.read()
	for range 100 {
		time.Sleep(5 * time.Millisecond)
		next := ss.read()
		if next == text {
			break
		}
		text = next
	}
	return text
}

// read returns the text on the screen now
func (ss *session) read() string {
	var b strings.Builder
	for y := range height {
		for x := range width {
			c := ss.mt.GetCell(vt.Coord{X: vt.Col(x), Y: vt.Row(y)}).C
			if c == "" {
				c = " "
			}
			b.WriteString(c)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// wait waits until want is on the screen
func (ss *session) wait(want string) {
	ss.t.Helper()
	for range 200 {
		if strings.Contains(ss.screen(), want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	ss.t.Fatalf("never saw %q on the screen:\n%s", want, ss.screen())
}

// waitGone waits until text isn't on the screen
func (ss *session) waitGone(text string) {
	ss.t.Helper()
	for range 200 {
		if !strings.Contains(ss.screen(), text) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	ss.t.Fatalf("%q stayed on the screen:\n%s", text, ss.screen())
}

// line returns the line of the tree containing text, leaving out the
// header and the rules, which show the remote's path
func (ss *session) line(text string) string {
	lines := strings.Split(ss.screen(), "\n")
	for _, l := range lines[2 : height-1] {
		tree, _, _ := strings.Cut(l, "│")
		if strings.Contains(tree, text) {
			return strings.TrimRight(tree, " ")
		}
	}
	return ""
}

// keys sends key presses: a tcell.Key or a string of runes
func (ss *session) keys(keys ...any) {
	for _, k := range keys {
		switch k := k.(type) {
		case tcell.Key:
			ss.s.EventQ() <- tcell.NewEventKey(k, "", tcell.ModNone)
		case string:
			for _, r := range k {
				ss.s.EventQ() <- tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone)
			}
		}
	}
}

// finish waits for Run to return
func (ss *session) finish() error {
	select {
	case err := <-ss.done:
		return err
	case <-time.After(10 * time.Second):
		ss.t.Fatalf("Run didn't return:\n%s", ss.screen())
		return nil
	}
}

const header = "# Filter rules for %q made with rclone genfilters\n# See https://rclone.org/filtering/\n\n"

func TestRunToggle(t *testing.T) {
	f := newRemote(t)
	file := filepath.Join(t.TempDir(), "rules.txt")
	ss := start(t, f, Options{Edit: file})
	ss.wait("a.txt")
	// docs/ music/ tmp/ a.txt b.txt
	ss.keys(tcell.KeyDown, tcell.KeyDown, " ", "j", " ")
	ss.wait("- /a.txt")
	assert.Contains(t, ss.line("tmp/"), "- ▸ tmp/ (excluded)")
	assert.Contains(t, ss.line("b.txt"), "    b.txt")
	assert.NotContains(t, ss.line("b.txt"), "excluded")
	ss.keys("q")
	require.NoError(t, ss.finish())

	got, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf(header, fs.ConfigString(f))+"- /tmp/**\n- /a.txt\n+ **\n", string(got))
	assert.NoFileExists(t, file+".bak")
}

func TestRunExcludeOthersNested(t *testing.T) {
	f := newRemote(t)
	file := filepath.Join(t.TempDir(), "rules.txt")
	ss := start(t, f, Options{Edit: file, ExcludeOthers: true})
	ss.wait("everything without a rule is excluded")
	// open docs/ to show sub/ and one.md, then include one.md
	ss.keys(tcell.KeyRight)
	ss.wait("one.md")
	ss.keys("jj ")
	ss.wait("+ /docs/one.md")
	assert.Contains(t, ss.line("docs/"), "▾ docs/ (partly included)")
	assert.Contains(t, ss.line("one.md"), "+     one.md (included)")
	assert.Contains(t, ss.line("sub/"), "▸ sub/")
	assert.NotContains(t, ss.line("sub/"), "included")
	// close docs/ from inside it, then include music/
	ss.keys(tcell.KeyLeft)
	ss.wait("▸ docs/")
	assert.NotContains(t, ss.screen(), "one.md (included)")
	ss.keys("j ")
	ss.wait("+ /music/**")
	ss.keys("q")
	require.NoError(t, ss.finish())

	got, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf(header, fs.ConfigString(f))+"+ /music/**\n+ /docs/one.md\n- **\n", string(got))
}

func TestRunExcludeOthersKeepsRules(t *testing.T) {
	f := newRemote(t)
	file := filepath.Join(t.TempDir(), "rules.txt")
	const old = "# mine\n+ /docs/**\n+ /a.txt\n- **\n"
	require.NoError(t, os.WriteFile(file, []byte(old), 0o600))
	ss := start(t, f, Options{Edit: file, ExcludeOthers: true})
	ss.wait("a.txt (included)")
	ss.keys("q")
	require.NoError(t, ss.finish())
	got, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Equal(t, old, string(got))
	assert.NoFileExists(t, file+".bak")
}

func TestRunTrailingSpace(t *testing.T) {
	ctx := context.Background()
	f, err := fs.NewFs(ctx, t.TempDir())
	require.NoError(t, err)
	// through rclone, as Windows can only store "notes " in its encoded form
	for _, name := range []string{"b.txt", "notes ", "notes"} {
		_, err := operations.Rcat(ctx, f, name, io.NopCloser(strings.NewReader(name)), time.Now(), nil)
		require.NoError(t, err)
	}
	notExcluded := regexp.MustCompile(`(?m)^    notes +│`)

	// unchanged rules aren't rewritten
	file := filepath.Join(t.TempDir(), "rules.txt")
	const old = "# mine\n- /notes[ ]\n+ **\n"
	require.NoError(t, os.WriteFile(file, []byte(old), 0o600))
	ss := start(t, f, Options{Edit: file})
	ss.wait("-   notes  (excluded)")
	assert.Regexp(t, notExcluded, ss.screen())
	ss.keys("q")
	require.NoError(t, ss.finish())
	got, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Equal(t, old, string(got))
	assert.NoFileExists(t, file+".bak")

	// another spelling of the same rule is written the usual way
	require.NoError(t, os.WriteFile(file, []byte("# mine\n- /[n]otes[ ]\n+ **\n"), 0o600))
	ss = start(t, f, Options{Edit: file})
	ss.wait("-   notes  (excluded)")
	assert.Regexp(t, notExcluded, ss.screen())
	ss.keys(" ")
	ss.wait("- /b.txt")
	ss.keys("q")
	require.NoError(t, ss.finish())
	got, err = os.ReadFile(file)
	require.NoError(t, err)
	assert.Equal(t, "# mine\n- /notes[ ]\n- /b.txt\n+ **\n", string(got))
	includes := engine(t, splitLines(string(got)))
	assert.False(t, includes("notes "))
	assert.True(t, includes("notes"))
	assert.False(t, includes("b.txt"))
}

func TestRunEditExisting(t *testing.T) {
	f := newRemote(t)
	file := filepath.Join(t.TempDir(), "rules.txt")
	const old = "# mine\n- /a.txt\n+ **\n"
	require.NoError(t, os.WriteFile(file, []byte(old), 0o600))
	ss := start(t, f, Options{Edit: file})
	ss.wait("a.txt (excluded)")
	ss.keys("jjjj ")
	ss.wait("- /b.txt")
	ss.keys("q")
	require.NoError(t, ss.finish())

	got, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Equal(t, "# mine\n- /b.txt\n- /a.txt\n+ **\n", string(got))
	bak, err := os.ReadFile(file + ".bak")
	require.NoError(t, err)
	assert.Equal(t, old, string(bak))
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(file)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm())
	}
}

func TestRunUnchanged(t *testing.T) {
	f := newRemote(t)
	file := filepath.Join(t.TempDir(), "rules.txt")
	const old = "- /a.txt\n+ **\n"
	require.NoError(t, os.WriteFile(file, []byte(old), 0o600))
	ss := start(t, f, Options{Edit: file})
	ss.wait("a.txt (excluded)")
	ss.keys("q")
	require.NoError(t, ss.finish())
	got, err := os.ReadFile(file)
	require.NoError(t, err)
	// the default header is only added to new files
	assert.Equal(t, fmt.Sprintf(header, fs.ConfigString(f))+old, string(got))
	assert.FileExists(t, file+".bak")

	// rules which haven't changed aren't written
	require.NoError(t, os.Remove(file+".bak"))
	ss = start(t, f, Options{Edit: file})
	ss.wait("a.txt (excluded)")
	ss.keys("q")
	require.NoError(t, ss.finish())
	assert.NoFileExists(t, file+".bak")
}

func TestRunCancel(t *testing.T) {
	f := newRemote(t)
	file := filepath.Join(t.TempDir(), "rules.txt")
	const old = "- /a.txt\n+ **\n"
	require.NoError(t, os.WriteFile(file, []byte(old), 0o600))
	ss := start(t, f, Options{Edit: file})
	ss.wait("a.txt (excluded)")
	ss.keys("jjjj ")
	ss.wait("- /b.txt")
	ss.keys(tcell.KeyCtrlC)
	assert.ErrorIs(t, ss.finish(), ErrCancelled)
	got, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Equal(t, old, string(got))
	assert.NoFileExists(t, file+".bak")
}

func TestRunEditRules(t *testing.T) {
	f := newRemote(t)
	file := filepath.Join(t.TempDir(), "rules.txt")
	ss := start(t, f, Options{Edit: file})
	ss.wait("a.txt")
	// go to the start of the catch-all rule and type a rule before it
	ss.keys(tcell.KeyTab, tcell.KeyPgDn, tcell.KeyHome, "- /b.txt", tcell.KeyEnter)
	ss.wait("b.txt (excluded)")
	assert.Contains(t, ss.screen(), "Editing the rules")

	// a rule genfilters can't edit shows an error and keeps the tree
	ss.keys("- *.tmp", tcell.KeyEnter)
	ss.wait(`must start with "/"`)
	assert.Contains(t, ss.screen(), "b.txt (excluded)")
	ss.keys(tcell.KeyTab)
	ss.wait("<keep editing>")
	ss.keys(tcell.KeyEnter)
	ss.waitGone("<keep editing>")
	assert.Contains(t, ss.screen(), `must start with "/"`)

	// fix it
	ss.keys(tcell.KeyUp, tcell.KeyEnd)
	for range len("- *.tmp") {
		ss.keys(tcell.KeyBackspace2)
	}
	ss.keys("- /tmp/**")
	ss.wait("tmp/ (excluded)")
	ss.keys(tcell.KeyTab)
	ss.wait("3 rules")

	// and discard a bad edit
	ss.keys(tcell.KeyTab, tcell.KeyHome, "x")
	ss.wait("malformed rule")
	ss.keys(tcell.KeyTab)
	ss.wait("<discard edits>")
	ss.keys(tcell.KeyRight, tcell.KeyEnter)
	ss.wait("3 rules")
	ss.keys("q")
	require.NoError(t, ss.finish())

	got, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf(header, fs.ConfigString(f))+"- /tmp/**\n- /b.txt\n+ **\n", string(got))
}

func TestRunSetMode(t *testing.T) {
	f := newRemote(t)
	file := filepath.Join(t.TempDir(), "rules.txt")
	ss := start(t, f, Options{Edit: file})
	ss.wait("a.txt")
	ss.keys("jjj ")
	ss.wait("- /a.txt")
	ss.keys("x")
	ss.wait("removes all 1 rules")
	// cancel keeps the rules
	ss.keys(tcell.KeyEnter)
	ss.wait("- /a.txt")
	ss.keys("x", tcell.KeyRight, tcell.KeyEnter)
	ss.wait("everything without a rule is excluded")
	assert.NotContains(t, ss.screen(), "- /a.txt")
	// with no rules there is nothing to confirm
	ss.keys("i")
	ss.wait("everything without a rule is included")
	ss.keys("?")
	ss.wait("toggle help on and off")
	ss.keys("?", "q")
	require.NoError(t, ss.finish())
	got, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf(header, fs.ConfigString(f))+"+ **\n", string(got))
}

func TestRunErrors(t *testing.T) {
	f := newRemote(t)
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.txt")
	require.NoError(t, os.WriteFile(bad, []byte("- /a.txt\n- *.tmp\n"), 0o600))
	include := filepath.Join(dir, "include.txt")
	require.NoError(t, os.WriteFile(include, []byte("- /a.txt\n"), 0o600))
	ctx := context.Background()

	err := Run(ctx, f, Options{Edit: bad})
	assert.ErrorContains(t, err, `line 2: "- *.tmp"`)
	err = Run(ctx, f, Options{Edit: include, ExcludeOthers: true})
	assert.ErrorContains(t, err, "--exclude-others can't be used")
	err = Run(ctx, f, Options{Open: true})
	assert.ErrorContains(t, err, "--open needs --edit")

	ctx, fi := filter.AddConfig(ctx)
	require.NoError(t, fi.AddRule("- *.tmp"))
	err = Run(ctx, f, Options{Edit: include})
	assert.ErrorContains(t, err, "can't be used with filter flags")
}

func TestRunStdinStdout(t *testing.T) {
	f := newRemote(t)
	swap := func(f **os.File, to *os.File) {
		old := *f
		*f = to
		t.Cleanup(func() { *f = old })
	}
	inR, inW, err := os.Pipe()
	require.NoError(t, err)
	outR, outW, err := os.Pipe()
	require.NoError(t, err)
	swap(&os.Stdin, inR)
	swap(&os.Stdout, outW)
	_, err = inW.WriteString("- /a.txt\n- **\n")
	require.NoError(t, err)
	require.NoError(t, inW.Close())

	ss := start(t, f, Options{})
	ss.wait("everything without a rule is excluded")
	ss.keys("jjjj ")
	ss.wait("+ /b.txt")
	ss.keys("q")
	require.NoError(t, ss.finish())
	require.NoError(t, outW.Close())
	got, err := io.ReadAll(outR)
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf(header, fs.ConfigString(f))+"+ /b.txt\n- /a.txt\n- **\n", string(got))
}
