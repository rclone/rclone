//go:build !plan9 && !js && !aix

// Package ncdu implements a text based user interface for exploring a remote
package ncdu

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/ncdu/scan"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/tui"
	"github.com/spf13/cobra"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
}

var commandDefinition = &cobra.Command{
	Use:   "ncdu remote:path",
	Short: `Explore a remote with a text based user interface.`,
	Long: `This displays a text based user interface allowing the navigation of a
remote. It is most useful for answering the question - "What is using
all my disk space?".

{{< asciinema 157793 >}}

To make the user interface it first scans the entire remote given and
builds an in memory representation.  rclone ncdu can be used during
this scanning phase and you will see it building up the directory
structure as it goes along.

You can interact with the user interface using key presses,
press '?' to toggle the help on and off. The supported keys are:

` + "```text" + `
` + strings.Join(helpText()[1:], "\n") + `
` + "```" + `

Listed files/directories may be prefixed by a one-character flag,
some of them combined with a description in brackets at end of line.
These flags have the following meaning:

` + "```text" + `
e means this is an empty directory, i.e. contains no files (but
  may contain empty subdirectories)
~ means this is a directory where some of the files (possibly in
  subdirectories) have unknown size, and therefore the directory
  size may be underestimated (and average size inaccurate, as it
  is average of the files with known sizes).
. means an error occurred while reading a subdirectory, and
  therefore the directory size may be underestimated (and average
  size inaccurate)
! means an error occurred while reading this directory
` + "```" + `

This an homage to the [ncdu tool](https://dev.yorhel.nl/ncdu) but for
rclone remotes.  It is missing lots of features at the moment
but is useful as it stands. Unlike ncdu it does not show excluded files.

Note that it might take some time to delete big files/directories. The
UI won't respond in the meantime since the deletion is done synchronously.

For a non-interactive listing of the remote, see the
[tree](/commands/rclone_tree/) command. To just get the total size of
the remote you can also use the [size](/commands/rclone_size/) command.`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.37",
		"groups":            "Filter,Listing",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			return NewUI(fsrc).Run()
		})
	},
}

// helpText returns help text for ncdu
func helpText() (tr []string) {
	tr = []string{
		"rclone ncdu",
		" ↑,↓ or k,j to Move",
		" →,l to enter",
		" ←,h to return",
		" g toggle graph",
		" c toggle counts",
		" a toggle average size in directory",
		" m toggle modified time",
		" u toggle human-readable format",
		" n,s,C,A,M sort by name,size,count,asize,mtime",
		" d delete file/directory",
		" v select file/directory",
		" V enter visual select mode",
		" D delete selected files/directories",
	}
	if !clipboard.Unsupported {
		tr = append(tr, " y copy current path to clipboard")
	}
	tr = append(tr, []string{
		" Y display current path",
		" ^L refresh screen (fix screen corruption)",
		" r recalculate file sizes",
		" ? to toggle help on and off",
		" ESC to close the menu box",
		" q/^c to quit",
	}...)
	return
}

// UI contains the state of the user interface
type UI struct {
	s                  tcell.Screen
	f                  fs.Fs     // fs being displayed
	cancel             func()    // cancel the current scanning process
	fsName             string    // human name of Fs
	root               *scan.Dir // root directory
	d                  *scan.Dir // current directory being displayed
	path               string    // path of current directory
	box                tui.Box   // popup box
	boxMenuHandler     func(fs fs.Fs, path string, option int) (string, error)
	entries            fs.DirEntries // entries of current directory
	sortPerm           []int         // order to display entries in after sorting
	invSortPerm        []int         // inverse order
	dirListHeight      int           // height of listing
	listing            bool          // whether listing is in progress
	showGraph          bool          // toggle showing graph
	showCounts         bool          // toggle showing counts
	showDirAverageSize bool          // toggle average size
	showModTime        bool          // toggle showing timestamps
	humanReadable      bool          // toggle human-readable format
	visualSelectMode   bool          // toggle visual selection mode
	sortByName         int8          // +1 for normal (lexical), 0 for off, -1 for reverse
	sortBySize         int8          // +1 for normal (largest first), 0 for off, -1 for reverse (smallest first)
	sortByCount        int8
	sortByAverageSize  int8
	sortByModTime      int8              // +1 for normal (newest first), 0 for off, -1 for reverse (oldest first)
	dirPosMap          map[string]dirPos // store for directory positions
	selectedEntries    map[string]dirPos // selected entries of current directory
}

// Where we have got to in the directory listing
type dirPos struct {
	entry  int
	offset int
}

// find the biggest entry in the current listing
func (u *UI) biggestEntry() (biggest int64) {
	if u.d == nil {
		return
	}
	for i := range u.entries {
		attrs, _ := u.d.AttrI(u.sortPerm[i])
		if attrs.Size > biggest {
			biggest = attrs.Size
		}
	}
	return
}

// hasEmptyDir returns true if there is empty folder in current listing
func (u *UI) hasEmptyDir() bool {
	if u.d == nil {
		return false
	}
	for i := range u.entries {
		attrs, _ := u.d.AttrI(u.sortPerm[i])
		if attrs.IsDir && attrs.Count == 0 {
			return true
		}
	}
	return false
}

// Draw the current screen
func (u *UI) Draw() {
	ctx := context.Background()
	w, h := u.s.Size()
	u.dirListHeight = h - 3

	// Plot
	u.s.Clear()

	// Header line
	tui.Linef(u.s, 0, 0, w, tcell.StyleDefault.Reverse(true), ' ', "rclone ncdu %s - use the arrow keys to navigate, press ? for help", fs.Version)

	// Directory line
	tui.Linef(u.s, 0, 1, w, tcell.StyleDefault, '-', "-- %s ", u.path)

	// graphs
	const (
		graphBars = 10
		graph     = "##########          "
	)

	// Directory listing
	if u.d != nil {
		y := 2
		perBar := u.biggestEntry() / graphBars
		if perBar == 0 {
			perBar = 1
		}
		showEmptyDir := u.hasEmptyDir()
		dirPos := u.dirPosMap[u.path]
		// Check to see if a rescan has invalidated the position
		if dirPos.offset >= len(u.sortPerm) {
			delete(u.dirPosMap, u.path)
			dirPos.offset = 0
			dirPos.entry = 0
		}
		for i, j := range u.sortPerm[dirPos.offset:] {
			entry := u.entries[j]
			n := i + dirPos.offset
			if y >= h-1 {
				break
			}
			var attrs scan.Attrs
			var err error
			if u.showModTime {
				attrs, err = u.d.AttrWithModTimeI(ctx, u.sortPerm[n])
			} else {
				attrs, err = u.d.AttrI(u.sortPerm[n])
			}
			_, isSelected := u.selectedEntries[entry.String()]
			style := tcell.StyleDefault
			if attrs.EntriesHaveErrors {
				style = style.Foreground(color.Yellow)
			}
			if err != nil {
				style = style.Foreground(color.Red)
			}
			if isSelected {
				style = style.Foreground(color.LightYellow)
			}
			if n == dirPos.entry {
				style = style.Reverse(true)
			}
			mark := ' '
			if attrs.IsDir {
				mark = '/'
			}
			fileFlag := ' '
			message := ""
			if !attrs.Readable {
				message = " [not read yet]"
			}
			if attrs.CountUnknownSize > 0 {
				message = fmt.Sprintf(" [%d of %d files have unknown size, size may be underestimated]", attrs.CountUnknownSize, attrs.Count)
				fileFlag = '~'
			}
			if attrs.EntriesHaveErrors {
				message = " [some subdirectories could not be read, size may be underestimated]"
				fileFlag = '.'
			}
			if err != nil {
				message = fmt.Sprintf(" [%s]", err)
				fileFlag = '!'
			}
			extras := ""
			if u.showCounts {
				ss := operations.CountStringField(attrs.Count, u.humanReadable, 9) + " "
				if attrs.Count > 0 {
					extras += ss
				} else {
					extras += strings.Repeat(" ", len(ss))
				}
			}
			if u.showDirAverageSize {
				avg := attrs.AverageSize()
				ss := operations.SizeStringField(int64(avg), u.humanReadable, 9) + " "
				if avg > 0 {
					extras += ss
				} else {
					extras += strings.Repeat(" ", len(ss))
				}
			}
			if u.showModTime {
				extras += attrs.ModTime.Local().Format("2006-01-02 15:04:05") + " "
			}
			if showEmptyDir {
				if attrs.IsDir && attrs.Count == 0 && fileFlag == ' ' {
					fileFlag = 'e'
				}
			}
			if u.showGraph {
				bars := (attrs.Size + perBar/2 - 1) / perBar
				// clip if necessary - only happens during startup
				if bars > 10 {
					bars = 10
				} else if bars < 0 {
					bars = 0
				}
				extras += "[" + graph[graphBars-bars:2*graphBars-bars] + "] "
			}
			tui.Linef(u.s, 0, y, w, style, ' ', "%c %s %s%c%s%s",
				fileFlag, operations.SizeStringField(attrs.Size, u.humanReadable, 12), extras, mark, path.Base(entry.Remote()), message)
			y++
		}
	}

	// Footer
	if u.d == nil {
		tui.Line(u.s, 0, h-1, w, tcell.StyleDefault.Reverse(true), ' ', "Waiting for root directory...")
	} else {
		message := ""
		if u.listing {
			message = " [listing in progress]"
		}
		size, count := u.d.Attr()
		tui.Linef(u.s, 0, h-1, w, tcell.StyleDefault.Reverse(true), ' ', "Total usage: %s, Objects: %s%s",
			operations.SizeString(size, u.humanReadable), operations.CountString(count, u.humanReadable), message)
	}

	// Show the box on top if required
	if u.box.Show {
		u.box.Draw(u.s)
	}
}

// Move the cursor this many spaces adjusting the viewport as necessary
func (u *UI) move(d int) {
	if u.d == nil {
		return
	}

	absD := d
	if d < 0 {
		absD = -d
	}

	entries := len(u.entries)

	// Fetch current dirPos
	dirPos := u.dirPosMap[u.path]

	dirPos.entry += d

	// check entry in range
	if dirPos.entry < 0 {
		dirPos.entry = 0
	} else if dirPos.entry >= entries {
		dirPos.entry = entries - 1
	}

	// check cursor still on screen
	p := dirPos.entry - dirPos.offset // where dirPos.entry appears on the screen
	if p < 0 {
		dirPos.offset -= absD
	} else if p >= u.dirListHeight {
		dirPos.offset += absD
	}

	// check dirPos.offset in bounds
	if entries == 0 || dirPos.offset < 0 {
		dirPos.offset = 0
	} else if dirPos.offset >= entries {
		dirPos.offset = entries - 1
	}

	// toggle the current file for selection in selection mode
	if u.visualSelectMode {
		u.toggleSelectForCursor()
	}

	// write dirPos back for later
	u.dirPosMap[u.path] = dirPos
}

func (u *UI) removeEntry(pos int) {
	u.d.Remove(pos)
	u.setCurrentDir(u.d)
}

func (u *UI) delete() {
	if u.d == nil || len(u.entries) == 0 {
		return
	}
	if len(u.selectedEntries) > 0 {
		u.deleteSelected()
	} else {
		u.deleteSingle()
	}
}

// delete the entry at the current position
func (u *UI) deleteSingle() {
	ctx := context.Background()
	cursorPos := u.dirPosMap[u.path]
	dirPos := u.sortPerm[cursorPos.entry]
	dirEntry := u.entries[dirPos]
	u.box.Menu = []string{"cancel", "confirm"}
	if obj, isFile := dirEntry.(fs.Object); isFile {
		u.boxMenuHandler = func(f fs.Fs, p string, o int) (string, error) {
			if o != 1 {
				return "Aborted!", nil
			}
			err := operations.DeleteFile(ctx, obj)
			if err != nil {
				return "", err
			}
			u.removeEntry(dirPos)
			if cursorPos.entry >= len(u.entries) {
				u.move(-1) // move back onto a valid entry
			}
			return "Successfully deleted file!", nil
		}
		u.box.Popup([]string{
			"Delete this file?",
			fspath.JoinRootPath(u.fsName, dirEntry.String())})
	} else {
		u.boxMenuHandler = func(f fs.Fs, p string, o int) (string, error) {
			if o != 1 {
				return "Aborted!", nil
			}
			err := operations.Purge(ctx, f, dirEntry.String())
			if err != nil {
				return "", err
			}
			u.removeEntry(dirPos)
			if cursorPos.entry >= len(u.entries) {
				u.move(-1) // move back onto a valid entry
			}
			return "Successfully purged folder!", nil
		}
		u.box.Popup([]string{
			"Purge this directory?",
			"ALL files in it will be deleted",
			fspath.JoinRootPath(u.fsName, dirEntry.String())})
	}
}

func (u *UI) deleteSelected() {
	ctx := context.Background()

	u.box.Menu = []string{"cancel", "confirm"}

	u.boxMenuHandler = func(f fs.Fs, p string, o int) (string, error) {
		if o != 1 {
			return "Aborted!", nil
		}

		positionsToDelete := make([]int, len(u.selectedEntries))
		i := 0

		// the menu has been handled, so don't draw it over the progress
		u.box.Menu = []string{}

		for key, cursorPos := range u.selectedEntries {

			u.box.Popup([]string{
				"Deleting selected items...",
				fmt.Sprintf("%d of %d", i+1, len(positionsToDelete))})
			u.Draw()
			u.s.Show()

			dirPos := u.sortPerm[cursorPos.entry]
			dirEntry := u.entries[dirPos]
			var err error

			if obj, isFile := dirEntry.(fs.Object); isFile {
				err = operations.DeleteFile(ctx, obj)
			} else {
				err = operations.Purge(ctx, f, dirEntry.String())
			}

			if err != nil {
				return "", err
			}

			delete(u.selectedEntries, key)
			positionsToDelete[i] = dirPos
			i++
		}

		// deleting all entries at once, as doing it during the deletions
		// could cause issues.
		sort.Slice(positionsToDelete, func(i, j int) bool {
			return positionsToDelete[i] > positionsToDelete[j]
		})
		for _, dirPos := range positionsToDelete {
			u.removeEntry(dirPos)
		}

		// move cursor at end if needed
		cursorPos := u.dirPosMap[u.path]
		if cursorPos.entry >= len(u.entries) {
			u.move(-1)
		}

		return fmt.Sprintf("Successfully deleted %d items!", i), nil
	}
	u.box.Popup([]string{
		"Delete selected items?",
		fmt.Sprintf("ALL %d items will be deleted", len(u.selectedEntries))})
}

func (u *UI) displayPath() {
	u.box.Toggle([]string{
		"Current Path",
		u.path,
	})
}

func (u *UI) copyPath() {
	if !clipboard.Unsupported {
		_ = clipboard.WriteAll(u.path)
	}
}

// Sort by the configured sort method
type ncduSort struct {
	sortPerm []int
	entries  fs.DirEntries
	d        *scan.Dir
	u        *UI
}

// Less is part of sort.Interface.
func (ds *ncduSort) Less(i, j int) bool {
	var iAvgSize, jAvgSize float64
	var iattrs, jattrs scan.Attrs
	if ds.u.sortByModTime != 0 {
		ctx := context.Background()
		iattrs, _ = ds.d.AttrWithModTimeI(ctx, ds.sortPerm[i])
		jattrs, _ = ds.d.AttrWithModTimeI(ctx, ds.sortPerm[j])
	} else {
		iattrs, _ = ds.d.AttrI(ds.sortPerm[i])
		jattrs, _ = ds.d.AttrI(ds.sortPerm[j])
	}
	iname, jname := ds.entries[ds.sortPerm[i]].Remote(), ds.entries[ds.sortPerm[j]].Remote()
	if iattrs.Count > 0 {
		iAvgSize = iattrs.AverageSize()
	}
	if jattrs.Count > 0 {
		jAvgSize = jattrs.AverageSize()
	}

	switch {
	case ds.u.sortByName < 0:
		return iname > jname
	case ds.u.sortByName > 0:
		break
	case ds.u.sortBySize < 0:
		if iattrs.Size != jattrs.Size {
			return iattrs.Size < jattrs.Size
		}
	case ds.u.sortBySize > 0:
		if iattrs.Size != jattrs.Size {
			return iattrs.Size > jattrs.Size
		}
	case ds.u.sortByModTime < 0:
		if iattrs.ModTime != jattrs.ModTime {
			return iattrs.ModTime.Before(jattrs.ModTime)
		}
	case ds.u.sortByModTime > 0:
		if iattrs.ModTime != jattrs.ModTime {
			return iattrs.ModTime.After(jattrs.ModTime)
		}
	case ds.u.sortByCount < 0:
		if iattrs.Count != jattrs.Count {
			return iattrs.Count < jattrs.Count
		}
	case ds.u.sortByCount > 0:
		if iattrs.Count != jattrs.Count {
			return iattrs.Count > jattrs.Count
		}
	case ds.u.sortByAverageSize < 0:
		if iAvgSize != jAvgSize {
			return iAvgSize < jAvgSize
		}
		// if avgSize is equal, sort by size
		if iattrs.Size != jattrs.Size {
			return iattrs.Size < jattrs.Size
		}
	case ds.u.sortByAverageSize > 0:
		if iAvgSize != jAvgSize {
			return iAvgSize > jAvgSize
		}
		// if avgSize is equal, sort by size
		if iattrs.Size != jattrs.Size {
			return iattrs.Size > jattrs.Size
		}
	}
	// if everything equal, sort by name
	return iname < jname
}

// Swap is part of sort.Interface.
func (ds *ncduSort) Swap(i, j int) {
	ds.sortPerm[i], ds.sortPerm[j] = ds.sortPerm[j], ds.sortPerm[i]
}

// Len is part of sort.Interface.
func (ds *ncduSort) Len() int {
	return len(ds.sortPerm)
}

// sort the permutation map of the current directory
func (u *UI) sortCurrentDir() {
	u.sortPerm = u.sortPerm[:0]
	for i := range u.entries {
		u.sortPerm = append(u.sortPerm, i)
	}
	data := ncduSort{
		sortPerm: u.sortPerm,
		entries:  u.entries,
		d:        u.d,
		u:        u,
	}
	sort.Sort(&data)
	if len(u.invSortPerm) < len(u.sortPerm) {
		u.invSortPerm = make([]int, len(u.sortPerm))
	}
	for i, j := range u.sortPerm {
		u.invSortPerm[j] = i
	}
}

// setCurrentDir sets the current directory
func (u *UI) setCurrentDir(d *scan.Dir) {
	u.d = d
	u.entries = d.Entries()
	u.path = fspath.JoinRootPath(u.fsName, d.Path())
	u.selectedEntries = make(map[string]dirPos)
	u.visualSelectMode = false
	u.sortCurrentDir()
}

// enters the current entry
func (u *UI) enter() {
	if u.d == nil || len(u.entries) == 0 {
		return
	}
	dirPos := u.dirPosMap[u.path]
	d, _ := u.d.GetDir(u.sortPerm[dirPos.entry])
	if d == nil {
		return
	}
	u.setCurrentDir(d)
}

// handles a box option that was selected
func (u *UI) handleBoxOption() {
	msg, err := u.boxMenuHandler(u.f, u.path, u.box.Button)
	// reset
	u.box.Button = 0
	u.box.Menu = []string{}
	u.boxMenuHandler = nil
	if err != nil {
		u.box.Popup([]string{
			"error:",
			err.Error(),
		})
		return
	}

	u.box.Popup([]string{"Finished:", msg})

}

// up goes up to the parent directory
func (u *UI) up() {
	if u.d == nil {
		return
	}
	parent := u.d.Parent()
	if parent != nil {
		u.setCurrentDir(parent)
	}
}

// toggle the sorting for the flag passed in
func (u *UI) toggleSort(sortType *int8) {
	old := *sortType
	u.sortBySize = 0
	u.sortByCount = 0
	u.sortByName = 0
	u.sortByAverageSize = 0
	if old == 0 {
		*sortType = 1
	} else {
		*sortType = -old
	}
	u.sortCurrentDir()
}

func (u *UI) toggleSelectForCursor() {
	cursorPos := u.dirPosMap[u.path]
	dirPos := u.sortPerm[cursorPos.entry]
	dirEntry := u.entries[dirPos]

	_, present := u.selectedEntries[dirEntry.String()]

	if present {
		delete(u.selectedEntries, dirEntry.String())
	} else {
		u.selectedEntries[dirEntry.String()] = cursorPos
	}
}

// NewUI creates a new user interface for ncdu on f
func NewUI(f fs.Fs) *UI {
	return &UI{
		f:                  f,
		path:               "Waiting for root...",
		dirListHeight:      20, // updated in Draw
		fsName:             fs.ConfigString(f),
		showGraph:          true,
		showCounts:         false,
		showDirAverageSize: false,
		humanReadable:      true,
		sortByName:         0,
		sortBySize:         1, // Sort by largest first
		sortByModTime:      0,
		sortByCount:        0,
		dirPosMap:          make(map[string]dirPos),
		selectedEntries:    make(map[string]dirPos),
	}
}

func (u *UI) scan() (chan *scan.Dir, chan error, chan struct{}) {
	if cancel := u.cancel; cancel != nil {
		cancel()
	}
	u.listing = true
	ctx := context.Background()
	ctx, u.cancel = context.WithCancel(ctx)
	return scan.Scan(ctx, u.f)
}

// Run shows the user interface
func (u *UI) Run() error {
	var err error
	u.s, err = tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("screen new: %w", err)
	}
	err = u.s.Init()
	if err != nil {
		return fmt.Errorf("screen init: %w", err)
	}

	// Hijack log output so that it doesn't corrupt the screen.
	defer tui.RedirectLogs()()

	defer u.s.Fini()

	// scan the disk in the background
	rootChan, errChan, updated := u.scan()

	events := u.s.EventQ()

	// Main loop, waiting for events and channels
outer:
	for {
		select {
		case root := <-rootChan:
			u.root = root
			u.setCurrentDir(root)
		case err := <-errChan:
			if err != nil {
				return fmt.Errorf("ncdu directory listing: %w", err)
			}
			u.listing = false
		case <-updated:
			// TODO: might want to limit updates per second
			u.sortCurrentDir()
		case ev := <-events:
			switch ev := ev.(type) {
			case *tcell.EventResize:
				u.Draw()
				u.s.Sync()
				continue // don't draw again
			case *tcell.EventKey:
				c := tui.EventRune(ev)
				switch c {
				case tui.Key(tcell.KeyEsc), tui.Key(tcell.KeyCtrlC), 'q':
					if u.box.Show || c == tui.Key(tcell.KeyEsc) {
						u.box.Show = false
					} else {
						break outer
					}
				case tui.Key(tcell.KeyDown), 'j':
					u.move(1)
				case tui.Key(tcell.KeyUp), 'k':
					u.move(-1)
				case tui.Key(tcell.KeyPgDn), '-', '_':
					u.move(u.dirListHeight)
				case tui.Key(tcell.KeyPgUp), '=', '+':
					u.move(-u.dirListHeight)
				case tui.Key(tcell.KeyLeft), 'h':
					if u.box.Show {
						u.box.Move(-1)
						break
					}
					u.up()
				case tui.Key(tcell.KeyEnter):
					if len(u.box.Menu) > 0 {
						u.handleBoxOption()
						break
					}
					u.enter()
				case tui.Key(tcell.KeyRight), 'l':
					if u.box.Show {
						u.box.Move(1)
						break
					}
					u.enter()
				case 'c':
					u.showCounts = !u.showCounts
				case 'm':
					u.showModTime = !u.showModTime
				case 'g':
					u.showGraph = !u.showGraph
				case 'a':
					u.showDirAverageSize = !u.showDirAverageSize
				case 'n':
					u.toggleSort(&u.sortByName)
				case 's':
					u.toggleSort(&u.sortBySize)
				case 'M':
					u.toggleSort(&u.sortByModTime)
				case 'v':
					u.toggleSelectForCursor()
				case 'V':
					u.visualSelectMode = !u.visualSelectMode
				case 'C':
					u.toggleSort(&u.sortByCount)
				case 'A':
					u.toggleSort(&u.sortByAverageSize)
				case 'y':
					u.copyPath()
				case 'Y':
					u.displayPath()
				case 'd':
					u.delete()
				case 'u':
					u.humanReadable = !u.humanReadable
				case 'D':
					u.deleteSelected()
				case '?':
					u.box.Toggle(helpText())
				case 'r':
					// restart scan
					rootChan, errChan, updated = u.scan()

				// Refresh the screen. Not obvious what key to map
				// this onto, but ^L is a common choice.
				case tui.Key(tcell.KeyCtrlL):
					u.Draw()
					u.s.Sync()
					continue // don't draw again
				}
			}
		}

		u.Draw()
		u.s.Show()
	}
	return nil
}
