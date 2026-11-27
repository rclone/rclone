//go:build !plan9 && !js && !aix

package genfilters

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/lib/tui"
)

// helpText returns help text for genfilters
func helpText() (tr []string) {
	tr = []string{
		"rclone genfilters",
		" ↑,↓ or k,j to move",
		" →,l to open a directory",
		" ←,h to close a directory or go to its parent",
		" SPACE or RETURN to include/exclude",
		" TAB to edit the rules, TAB again to go back",
		" x exclude everything without a rule (- **)",
		" i include everything without a rule (+ **)",
		" c clear all the rules",
	}
	if !clipboard.Unsupported {
		tr = append(tr, " y copy the path as used in rules to the clipboard")
	}
	tr = append(tr, []string{
		" Y display the full path",
		" ^L refresh screen (fix screen corruption)",
		" ? to toggle help on and off",
		" ESC to close the menu box",
		" q to finish and write the rules",
		" ^C to quit without writing the rules",
	}...)
	return tr
}

// node is a file or directory in the tree
type node struct {
	key      string // remote with a trailing "/" for directories, "" for the root
	name     string
	depth    int
	parent   *node
	children []*node
	listed   bool
	expanded bool
}

func (n *node) isDir() bool {
	return n.key == "" || strings.HasSuffix(n.key, "/")
}

// ui contains the state of the user interface
type ui struct {
	ctx        context.Context
	s          tcell.Screen
	f          fs.Fs
	fsName     string
	mo         *model
	root       *node
	rows       []*node // nodes shown, in order
	cursor     int     // index in rows
	top        int     // first row shown
	height     int     // rows in the body, updated in draw
	box        tui.Box
	boxHandler func(option int) // called with the menu option chosen
	editing    bool             // whether the rules have the focus
	ed         editor
	editErr    error // error in the edited rules
	done       bool
	cancelled  bool
}

// newUI makes a user interface to edit mo on f
func newUI(ctx context.Context, f fs.Fs, mo *model) *ui {
	u := &ui{
		ctx:    ctx,
		f:      f,
		fsName: fs.ConfigString(f),
		mo:     mo,
		root:   &node{listed: false, expanded: true, depth: -1},
		height: 20,
	}
	u.ed.setText(mo.lines())
	return u
}

// list reads the entries of directory n into the tree
func (u *ui) list(n *node) error {
	objs, dirs, err := walk.GetAll(u.ctx, u.f, strings.TrimSuffix(n.key, "/"), true, 1)
	if err != nil {
		return err
	}
	less := func(a, b string) bool { return strings.ToLower(a) < strings.ToLower(b) }
	sort.Slice(dirs, func(i, j int) bool { return less(dirs[i].Remote(), dirs[j].Remote()) })
	sort.Slice(objs, func(i, j int) bool { return less(objs[i].Remote(), objs[j].Remote()) })
	n.children = n.children[:0]
	for _, d := range dirs {
		n.children = append(n.children, &node{key: d.Remote() + "/", name: path.Base(d.Remote()), depth: n.depth + 1, parent: n})
	}
	for _, o := range objs {
		n.children = append(n.children, &node{key: o.Remote(), name: path.Base(o.Remote()), depth: n.depth + 1, parent: n})
	}
	n.listed = true
	return nil
}

// flatten works out which nodes are shown
func (u *ui) flatten() {
	var current *node
	if u.cursor < len(u.rows) {
		current = u.rows[u.cursor]
	}
	u.rows = u.rows[:0]
	var walkNodes func(n *node)
	walkNodes = func(n *node) {
		for _, c := range n.children {
			u.rows = append(u.rows, c)
			if c.expanded {
				walkNodes(c)
			}
		}
	}
	walkNodes(u.root)
	for i, n := range u.rows {
		if n == current {
			u.cursor = i
			return
		}
	}
	u.cursor = max(0, min(u.cursor, len(u.rows)-1))
}

// current returns the node at the cursor or nil
func (u *ui) current() *node {
	if u.cursor < len(u.rows) {
		return u.rows[u.cursor]
	}
	return nil
}

// errorBox shows err in a box
func (u *ui) errorBox(err error) {
	u.box.Menu = nil
	u.box.Popup([]string{"error:", err.Error()})
}

// confirm asks a question, calling fn if the second option is chosen
func (u *ui) confirm(text []string, options []string, fn func()) {
	u.box.Menu = options
	u.box.Button = 0
	u.boxHandler = func(option int) {
		if option == 1 {
			fn()
		}
	}
	u.box.Popup(text)
}

// changed updates the rules shown after the model changed
func (u *ui) changed() {
	u.ed.setText(u.mo.lines())
}

// toggle includes or excludes the node at the cursor
func (u *ui) toggle() {
	n := u.current()
	if n == nil {
		return
	}
	if err := u.mo.set(n.key, !u.mo.included(n.key)); err != nil {
		u.errorBox(err)
	}
	u.changed()
}

// setMode changes the mode, asking first if that loses rules
func (u *ui) setMode(m mode) {
	what := map[mode]string{includeOthers: "including", excludeOthers: "excluding"}[m]
	apply := func() {
		if err := u.mo.setMode(m); err != nil {
			u.errorBox(err)
		}
		u.changed()
	}
	if len(u.mo.selected) == 0 {
		apply()
		return
	}
	u.confirm([]string{
		fmt.Sprintf("Start again, %s everything without a rule?", what),
		fmt.Sprintf("This removes all %d rules.", len(u.mo.selected)),
	}, []string{"cancel", "start again"}, apply)
}

// open opens the directory at the cursor, or moves into it if open
func (u *ui) open() {
	n := u.current()
	if n == nil || !n.isDir() {
		return
	}
	if n.expanded {
		if len(n.children) > 0 {
			u.cursor++
		}
		return
	}
	if !n.listed {
		if err := u.list(n); err != nil {
			u.errorBox(err)
			return
		}
	}
	n.expanded = true
	u.flatten()
}

// close closes the directory at the cursor, or goes to its parent
func (u *ui) close() {
	n := u.current()
	if n == nil {
		return
	}
	if n.expanded {
		n.expanded = false
		u.flatten()
		return
	}
	if n.parent != u.root {
		n.parent.expanded = false
		u.cursor = indexOf(u.rows, n.parent)
		u.flatten()
	}
}

func indexOf(rows []*node, n *node) int {
	for i, r := range rows {
		if r == n {
			return i
		}
	}
	return 0
}

// leaveRules gives the focus back to the tree, asking first if the
// rules have an error
func (u *ui) leaveRules() {
	leave := func() {
		u.editErr = nil
		u.editing = false
		u.changed()
	}
	if u.editErr == nil {
		leave()
		return
	}
	u.confirm([]string{"The rules have an error:", u.editErr.Error()},
		[]string{"keep editing", "discard edits"}, leave)
}

// editRules applies the rules being edited if they are valid
func (u *ui) editRules() {
	mo, _, err := parse(u.ed.text())
	u.editErr = err
	u.ed.errLine = 0
	if err != nil {
		if lineErr, ok := errors.AsType[*lineError](err); ok {
			u.ed.errLine = lineErr.line
		}
		return
	}
	u.mo = mo
}

// handleKey handles the key c
func (u *ui) handleKey(c rune) {
	if c == tui.Key(tcell.KeyCtrlC) {
		u.done, u.cancelled = true, true
		return
	}
	if c == tui.Key(tcell.KeyCtrlL) {
		u.s.Sync()
		return
	}
	if u.box.Show {
		switch c {
		case tui.Key(tcell.KeyLeft), 'h':
			u.box.Move(-1)
		case tui.Key(tcell.KeyRight), 'l':
			u.box.Move(1)
		case tui.Key(tcell.KeyEnter):
			u.box.Show = false
			if len(u.box.Menu) > 0 && u.boxHandler != nil {
				handler, option := u.boxHandler, u.box.Button
				u.box.Menu, u.boxHandler = nil, nil
				handler(option)
			}
		case tui.Key(tcell.KeyEsc), 'q', '?':
			u.box.Show = false
			u.box.Menu, u.boxHandler = nil, nil
		}
		return
	}
	if u.editing {
		switch c {
		case tui.Key(tcell.KeyTab), tui.Key(tcell.KeyEsc):
			u.leaveRules()
		default:
			if u.ed.key(c, u.height) {
				u.editRules()
			}
		}
		return
	}
	switch c {
	case tui.Key(tcell.KeyEsc):
	case 'q':
		u.done = true
	case tui.Key(tcell.KeyDown), 'j':
		u.cursor = min(u.cursor+1, len(u.rows)-1)
	case tui.Key(tcell.KeyUp), 'k':
		u.cursor = max(u.cursor-1, 0)
	case tui.Key(tcell.KeyPgDn):
		u.cursor = min(u.cursor+u.height, len(u.rows)-1)
	case tui.Key(tcell.KeyPgUp):
		u.cursor = max(u.cursor-u.height, 0)
	case tui.Key(tcell.KeyRight), 'l':
		u.open()
	case tui.Key(tcell.KeyLeft), 'h':
		u.close()
	case ' ', tui.Key(tcell.KeyEnter):
		u.toggle()
	case tui.Key(tcell.KeyTab):
		u.editing = true
	case 'x':
		u.setMode(excludeOthers)
	case 'i':
		u.setMode(includeOthers)
	case 'c':
		u.setMode(u.mo.mode)
	case 'y':
		if n := u.current(); n != nil && !clipboard.Unsupported {
			_ = clipboard.WriteAll(glob(n.key))
		}
	case 'Y':
		if n := u.current(); n != nil {
			u.box.Menu = nil
			u.box.Toggle([]string{"Full path", fspath.JoinRootPath(u.fsName, n.key)})
		}
	case '?':
		u.box.Menu = nil
		u.box.Toggle(helpText())
	}
	if u.cursor < 0 {
		u.cursor = 0
	}
}

// draw draws the screen
func (u *ui) draw() {
	s := u.s
	w, h := s.Size()
	s.Clear()
	u.height = max(1, h-3)
	treeWidth := max(20, w*3/5)
	reverse := tcell.StyleDefault.Reverse(true)

	tui.Linef(s, 0, 0, w, reverse, ' ', "rclone genfilters %s - press ? for help, TAB to edit the rules, q to finish", fs.Version)
	others := map[mode]string{includeOthers: "included", excludeOthers: "excluded"}[u.mo.mode]
	tui.Linef(s, 0, 1, w, tcell.StyleDefault, '-', "-- everything without a rule is %s -- %s ", others, u.fsName)

	// tree
	if u.cursor < u.top {
		u.top = u.cursor
	} else if u.cursor >= u.top+u.height {
		u.top = u.cursor - u.height + 1
	}
	if len(u.rows) == 0 {
		tui.Line(s, 0, 2, treeWidth, tcell.StyleDefault, ' ', "(empty)")
	}
	for i := range u.height {
		n := u.top + i
		if n >= len(u.rows) {
			break
		}
		u.drawNode(0, 2+i, treeWidth, u.rows[n], n == u.cursor && !u.editing)
	}

	// rules
	for y := 2; y < h-1; y++ {
		s.SetContent(treeWidth, y, tcell.RuneVLine, nil, tcell.StyleDefault)
	}
	u.ed.draw(s, treeWidth+2, 2, max(1, w-treeWidth-2), u.height, u.editing && !u.box.Show)
	if !u.editing || u.box.Show {
		s.HideCursor()
	}

	// footer
	footer := fmt.Sprintf("%d rules", len(u.mo.rules()))
	style := reverse
	if u.editErr != nil {
		footer = u.editErr.Error()
		style = tcell.StyleDefault.Background(color.Red).Reverse(true)
	} else if u.editing {
		footer = "Editing the rules - TAB to go back to the tree"
	}
	tui.Line(s, 0, h-1, w, style, ' ', footer)

	if u.box.Show {
		u.box.Draw(s)
	}
}

// drawNode draws n on line y up to xmax
func (u *ui) drawNode(x, y, xmax int, n *node, selected bool) {
	marker := "  "
	name := n.name
	if n.isDir() {
		marker = "▸ "
		if n.expanded {
			marker = "▾ "
		}
		name += "/"
	}
	rule := " "
	if include, ok := u.mo.selected[n.key]; ok {
		rule = sign(include)
	}
	style := tcell.StyleDefault.Foreground(color.Green)
	state := ""
	switch {
	case u.mo.partial(n.key):
		style = tcell.StyleDefault.Foreground(color.Yellow)
		state = " (partly included)"
	case !u.mo.included(n.key):
		style = tcell.StyleDefault.Foreground(color.Red)
		if u.mo.mode == includeOthers {
			state = " (excluded)"
		}
	case u.mo.mode == excludeOthers:
		state = " (included)"
	}
	if selected {
		style = style.Reverse(true)
	}
	text := rule + " " + strings.Repeat("  ", n.depth) + marker + name + state
	tui.Line(u.s, x, y, xmax, style, ' ', text)
}

// run shows the user interface until it is done
func (u *ui) run() (err error) {
	u.s, err = tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("screen new: %w", err)
	}
	if err = u.s.Init(); err != nil {
		return fmt.Errorf("screen init: %w", err)
	}
	// Hijack log output so that it doesn't corrupt the screen.
	defer tui.RedirectLogs()()
	defer u.s.Fini()

	if err = u.list(u.root); err != nil {
		return err
	}
	u.flatten()
	events := u.s.EventQ()
	for !u.done {
		u.draw()
		u.s.Show()
		switch ev := (<-events).(type) {
		case *tcell.EventResize:
			u.s.Sync()
		case *tcell.EventKey:
			u.handleKey(tui.EventRune(ev))
		}
	}
	return nil
}
