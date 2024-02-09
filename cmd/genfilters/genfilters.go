//go:build !plan9 && !js
// +build !plan9,!js

// Package genfilters implements a text based user interface for generating rclone filters in --filter-from format
package genfilters

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"
)

var (
	inputfile      = ""
	outputfile     = ""
	fsPath         = ""
	noOpen         bool
	rootnode       *tview.TreeNode
	selected       = map[string]bool{} // [remoteWithSlash]included
	filt           *filter.Filter
	rules          []string
	rulesRegex     []string
	rulesCmdLine   []string
	rulesmap       map[string]string // [remoteWithSlash]resultingRule
	defaultInclude bool
	quit           func()
	startover      = false
	mode           = includeOthers
	showRules      = true
	showRegex      = false
	showCmdLine    = false
	debug          = false
	importOnce     sync.Once
)

type filtermode uint8

const (
	excludeOthers filtermode = 0
	includeOthers filtermode = 1 << iota
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.StringVarP(cmdFlags, &outputfile, "output-file", "o", "", "Write results to a file at this path, for use as a --filter-from file. (default: {currentdirectory}/rclone_genfilters.txt)", "")
	flags.StringVarP(cmdFlags, &inputfile, "input-file", "", "", "Load filters from this existing file on startup.", "")
	flags.BoolVarP(cmdFlags, &noOpen, "no-open", "", noOpen, "Do not automatically open the file when completed.", "")
	flags.BoolVarP(cmdFlags, &showRegex, "regex", "", showRegex, "Also show output as regex, in --dump filters format", "")
	flags.BoolVarP(cmdFlags, &showCmdLine, "cmd", "", showCmdLine, "Also show filters in command line syntax, to add the filters directly via the --filter flag instead of with a --filter-from file", "")
}

var commandDefinition = &cobra.Command{
	Use:   "genfilters remote:path",
	Short: `Generate filters automatically with a terminal-based user interface.`,
	Long: `
Genfilters is an interactive, automatic filter generator for rclone. It works
with any rclone remote, and at the end it spits out a file that can be used as
a [` + "`--filter-from`" + `](https://rclone.org/filtering/#filter-from-read-filtering-patterns-from-a-file) file
(or bisync [` + "`--filters-file`" + `](https://rclone.org/bisync/#filtering)).

{{< img width="40%" src="/img/genfilters_demo.gif" alt="genfilters demo" style="padding: 5px;" >}}

Supports both ` + "`- **`" + ` and ` + "`+ **`" + ` modes, and can also dump
filters as regex, and it is careful about the rule order and avoiding
redundancy. It all runs through rclone's actual filter module in realtime, so
you can see exactly what you're getting.

Directories are traversed only when they are 'expanded', so it is easy to use
even with large remotes.

It can also show the filters in command line syntax, to add the filters
directly via the [` + "`--filter`" + `](https://rclone.org/filtering/#filter-add-a-file-filtering-rule)
flag instead of with a ` + "`--filter-from`" + ` file.

You can interact with the user interface using key presses. The supported keys are:

    ` + strings.Join(helpText()[1:], "\n    ") + `

See flags for more options.

Example usage of output file:

	rclone tree remote:path --filter-from "/path/to/your/rclone_genfilters.txt"
`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.66",
		"groups":            "Listing",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsPath = args[0]
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			return GenFilters(context.Background(), fsrc, inputfile, outputfile)
		})
	},
}

// helpText returns help text for genfilters
func helpText() (tr []string) {
	tr = []string{
		"rclone genfilters",
		" ↑,↓ or k,j to Move up and down the tree",
		" ←,→ to include/exclude the current node",
		" return to expand/collapse the currently selected directory",
		" x enable 'exclude others' mode (- **)",
		" i enable 'include others' mode (+ **) (enabled by default)",
		" d toggle debug mode (shows more info)",
		" r toggle showing the rules to the right of the tree",
		" c clear selections and start over",
		" y copy current relative path to clipboard (if supported)",
		" Y copy current absolute path to clipboard (if supported)",
		" q/ESC/^c to quit and output final results",
	}
	return
}

// sets default settings on startup
func setDefaults() {
	if mode == includeOthers {
		defaultInclude = true // selected = excluded
	} else {
		defaultInclude = false // selected = included
	}
	if outputfile == "" {
		outputfile = filepath.Join(".", "rclone_genfilters.txt")
	}
}

// returns true if the node's include/exclude status is the global default.
// ex. a node that is included in "includeOthers" mode, or excluded in "excludeOthers" mode.
func isDefault(node *tview.TreeNode) bool {
	return filt.IncludeRemote(nodeString(node)) == defaultInclude
}

// handles processing for nodes that should be "selected"
// (ex. a node that is excluded in "includeOthers" mode)
// it does so by adding a rule to the "selected" map,
// then triggering a refresh of the filters, other nodes, and display
func selectIt(node *tview.TreeNode) {
	delete(selected, nodeString(node))
	refresh()
	// check if rule is redundant before adding
	if filt.IncludeRemote(nodeString(node)) == defaultInclude {
		selected[nodeString(node)] = !defaultInclude
	}
	refresh()
	includeExclude(node)
}

// handles processing for nodes that should be "unselected"
// (ex. a node that is included in "includeOthers" mode)
// note that sometimes we will still add a rule to the "selected" map for these,
// if necessary to override another rule. (ex. an included file in an otherwise excluded directory)
func unselectIt(node *tview.TreeNode) {
	delete(selected, nodeString(node))
	refresh()
	// check if we need an explicit rule
	if filt.IncludeRemote(nodeString(node)) != defaultInclude {
		selected[nodeString(node)] = defaultInclude
		refresh()
	}
	includeExclude(node)
}

// updates the color and display text for included nodes
func includeIt(node *tview.TreeNode) {
	node.SetColor(tcell.ColorGreen)
	node.SetText(displayText(node, true))
}

// updates the color and display text for excluded nodes
func excludeIt(node *tview.TreeNode) {
	node.SetColor(tcell.ColorRed)
	node.SetText(displayText(node, false))
}

// sets the text displayed for each node
func displayText(node *tview.TreeNode, include bool) string {
	message := ""
	if mode == excludeOthers && include {
		message = " (included)"
	}
	if mode == includeOthers && !include {
		message = " (excluded)"
	}
	name := nodeString(node)
	if nodeIsDir(node) {
		name = "🖿 " + name
	}
	if showRules {
		rule := getRule(nodeString(node))
		if rule != "" {
			spaces := 50 - len(name+message)
			if spaces < 1 {
				spaces = 1
			}
			message += strings.Repeat(" ", spaces) + rule
		}
	}
	if debug {
		message += fmt.Sprintf(" (included: %v, isDefault: %v, children: %v)", filt.IncludeRemote(nodeString(node)), isDefault(node), len(node.GetChildren()))
	}
	return name + message
}

// determines whether a node is included or excluded under the current filter rules
func includeExclude(node *tview.TreeNode) {
	if filt.IncludeRemote(nodeString(node)) {
		includeIt(node)
	} else {
		excludeIt(node)
	}
}

// true if node is a directory
func nodeIsDir(node *tview.TreeNode) bool {
	_, isDir := node.GetReference().(fs.Directory)
	return isDir
}

// appends a trailing slash if it's a directory, otherwise returns the remote unchanged
func withSlash(o fs.DirEntry) string {
	s, isDir := o.(fs.Directory)
	if isDir {
		return s.Remote() + "/"
	}
	return o.Remote()
}

// generates the remote string for the node, appending a trailing slash if necessary
func nodeString(node *tview.TreeNode) string {
	s, ok := node.GetReference().(fs.DirEntry)
	if ok {
		return withSlash(s)
	}
	return ""
}

// returns the rule for this node, if one is found in the rulesmap,
// otherwise returns ""
func getRule(s string) string {
	rule, ok := rulesmap[s]
	if ok {
		return rule
	}
	return ""
}

// GenFilters is the main entry point. It shows a navigable tree view of the current directory and generates filters.
func GenFilters(ctx context.Context, f fs.Fs, infile, outfile string) error {
	// checks and warnings
	origFilters := filter.GetConfig(ctx)
	if !origFilters.InActive() {
		return fmt.Errorf("filters detected! You must run this command without filters in order for the results to be accurate. \n%s", origFilters.DumpFilters())
	}

	filt, _ = filter.NewFilter(nil)
	outputfile = outfile
	inputfile = infile
	setDefaults()
	rootDir := "."
	root := tview.NewTreeNode(rootDir).SetColor(tcell.ColorGreen)
	rootnode = root
	tree := tview.NewTreeView().SetRoot(root).SetCurrentNode(root)

	processNew := func(node *tview.TreeNode) {
		if filt.IncludeRemote(nodeString(node)) {
			if mode == includeOthers {
				unselectIt(node)
			} else {
				selectIt(node) // it can come in fresh as "selected" if it's a child of something that is selected
			}
		} else {
			if mode == includeOthers {
				selectIt(node)
			} else {
				unselectIt(node)
			}
		}
		includeExclude(node)
	}

	// A helper function which adds the files and directories of the given path to the given target node.
	add := func(target *tview.TreeNode, path string) {
		if path == "." {
			path = ""
		}
		path = strings.TrimSuffix(path, "/") // trim on the way in, add back on the way out
		files, dirs, err := walk.GetAll(ctx, f, path, false, 1)
		if err != nil {
			fs.Logf(nil, "%v", err)
		}
		refresh()
		sort.Slice(files, func(i, j int) bool {
			return strings.ToLower(files[i].Remote()) < strings.ToLower(files[j].Remote())
		})
		sort.Slice(dirs, func(i, j int) bool {
			return strings.ToLower(dirs[i].Remote()) < strings.ToLower(dirs[j].Remote())
		})
		for _, dir := range dirs {
			node := tview.NewTreeNode("🖿 " + withSlash(dir)).SetReference(dir).SetSelectable(true)
			processNew(node)
			target.AddChild(node)
			refresh()
		}
		for _, file := range files {
			node := tview.NewTreeNode(file.Remote()).SetReference(file).SetSelectable(true)
			processNew(node)
			target.AddChild(node)
			refresh()
		}
	}

	// Add the current directory to the root node.
	add(root, rootDir)

	// if an input file was supplied, parse and import it
	if inputfile != "" {
		var err error
		importOnce.Do(func() {
			err = forEachLine(inputfile, false, importRule)
			refresh()
		})
		if err != nil {
			return fmt.Errorf("error parsing input file: %v", err)
		}
	}

	// If a directory was selected, open it.
	// note that tview is not using "selected" the way we are. tview means "expanded".
	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		reference := withSlash(node.GetReference().(fs.DirEntry))
		if reference == "" {
			return // Selecting the root node does nothing.
		}
		_, ok := node.GetReference().(fs.Directory)
		if ok {
			children := node.GetChildren()
			if len(children) == 0 { // note that this is only counting children we "know about" (if it was never expanded, we never traversed it)
				// Load and show files in this directory.
				path := reference
				add(node, path)
			} else {
				// Collapse if visible, expand if collapsed.
				node.SetExpanded(!node.IsExpanded())
			}
		}
	})

	// resets everything and starts over
	startOver := func(tree *tview.TreeView) {
		startover = true
		for k := range selected {
			delete(selected, k)
		}
		setDefaults()
		filt.Clear()
		tree.GetRoot().ClearChildren()
		refresh()
		root = tview.NewTreeNode(rootDir).SetColor(tcell.ColorGreen)
		tree.SetRoot(root).SetCurrentNode(root)
		add(root, rootDir)
		root = tree.GetRoot()
		unselectIt(root)
		includeExclude(root)
		overrideChildren(root, false)
		refresh()
		quit()
	}

	// listens for input keys pressed by the user, and decides what to do accordingly
	tree.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyLeft, tcell.KeyRight:
			node := tree.GetCurrentNode()
			if !isDefault(node) {
				unselectIt(node)
				includeExclude(node)
				overrideChildren(node, false)
			} else {
				selectIt(node)
				includeExclude(node)
				overrideChildren(node, true)
			}
			return nil // to override the default arrow key behavior
		case tcell.KeyEsc, tcell.KeyCtrlC:
			quit()
			// return tcell.NewEventKey(tcell.KeyCtrlC, key(tcell.KeyCtrlC), tcell.ModNone)
		case tcell.KeyRune:
			switch event.Rune() {
			case 'q':
				quit()
			case 'd':
				debug = !debug
				refresh()
			case 'r':
				showRules = !showRules
				refresh()
			case 'c':
				startOver(tree)
			case 'x':
				mode = excludeOthers
				startOver(tree)
			case 'i':
				mode = includeOthers
				startOver(tree)
			case 'y':
				if !clipboard.Unsupported {
					_ = clipboard.WriteAll(nodeString(tree.GetCurrentNode()))
				}
			case 'Y':
				if !clipboard.Unsupported {
					_ = clipboard.WriteAll(fspath.JoinRootPath(fsPath, nodeString(tree.GetCurrentNode())))
				}
			}
		}
		return event
	})

	// handles the box on the left which shows the resulting rules in realtime
	tree.Box.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		half := width / 2

		// draw text
		style := tcell.StyleDefault.Reverse(false)
		toPrint := rules
		if showRegex {
			toPrint = append(toPrint, rulesRegex...)
		}
		if showCmdLine {
			toPrint = append(toPrint, rulesCmdLine...)
		}
		for i, s := range toPrint {
			line(x, y+i, half-5, style, ' ', s, screen)
			style = tcell.StyleDefault.Reverse(false)
		}
		return x + half, y, width - (half + 5), height
	})

	// Lock the StdoutMutex - must not call fs.Log anything
	// otherwise it will deadlock with --interactive --progress
	operations.StdoutMutex.Lock()

	// create the app and run it
	app := tview.NewApplication()
	quit = app.Stop
	if err := app.SetRoot(tree, true).EnableMouse(true).Run(); err != nil {
		operations.StdoutMutex.Unlock()
		panic(err)
	}
	operations.StdoutMutex.Unlock()

	// if user triggers a startover, start over, otherwise print the resulting rules and generate the output file
	if !startover {
		rulesString := strings.Join(rules, "\n")
		fs.Logf(nil, "Your filter rules are: \n%s", rulesString)
		fs.Logf(nil, "Example usage: \nrclone tree %s --filter-from %s", strconv.Quote(fsPath), strconv.Quote(outputfile))
		outputFile(outputfile, rulesString)

		if showRegex {
			fs.Logf(nil, "Regex Filters: \n%s", strings.Join(rulesRegex, "\n"))
		}
		if showCmdLine {
			fs.Logf(nil, "Command Line Flag Filters: \n%s", strings.Join(rulesCmdLine, "\n"))
		}
	} else {
		startover = false
		_ = GenFilters(ctx, f, inputfile, outputfile)
	}
	return nil
}

// parses and sorts the selected map and returns a slice of rules and rulesmap
func parse() ([]string, map[string]string) {
	keys := make([]string, 0, len(selected))

	for k := range selected {
		keys = append(keys, k)
	}
	// reverse sort
	sort.SliceStable(keys, func(i, j int) bool {
		return strings.ToLower(keys[i]) > strings.ToLower(keys[j])
	})

	root := "+ /*"
	others := "+ **"
	if mode == excludeOthers {
		root = "- /*"
		others = "- **"
	}

	// loop
	// these are new vars, not our globals
	rules := []string{}
	rulesmap := map[string]string{} // [remoteWithSlash]resultingRule
	for _, s := range keys {
		prefix := ""
		if selected[s] {
			prefix = "+ /"
		} else {
			prefix = "- /"
		}
		rule := prefix + s
		if strings.HasSuffix(rule, "/") {
			rule = rule + "**"
		}
		rules = append(rules, rule)
		rulesmap[s] = rule
	}
	rules = append(rules, root)
	rulesmap["."] = root
	rules = append(rules, others)
	rulesmap["**"] = others

	return rules, rulesmap
}

// refreshes the actual rclone filter based on the current selected map
// at the end it redraws all the nodes in case their display should change
func refresh() {
	setDefaults()
	// overwrite the globals
	rules, rulesmap = parse()
	rulesRegex = []string{}
	rulesCmdLine = []string{}

	filt.Clear()
	filt, _ = filter.NewFilter(nil)

	for _, rule := range rules {
		_ = filt.AddRule(rule)
	}

	if showRegex {
		dump := "\n\n" + filt.DumpFilters()
		rulesRegex = append(rulesRegex, strings.Split(dump, "\n")...)
	}
	if showCmdLine {
		rulesCmdLine = append(rulesCmdLine, "\n", "\n")
		for _, rule := range rules {
			rulesCmdLine = append(rulesCmdLine, fmt.Sprintf(" --filter %s", strconv.Quote(rule)))
		}
	}

	redraw(rootnode)
}

// refresh the display for all nodes in the tree, recursively
func redraw(root *tview.TreeNode) {
	children := root.GetChildren()
	for _, child := range children {
		includeExclude(child)

		// recurse
		if len(child.GetChildren()) > 0 {
			redraw(child)
		}
	}
}

// when a directory's include/exclude status changes, apply that same change to all its children, grandchildren, etc.
// for example, if a user changes a directory from included to excluded, every file in that directory should also become excluded.
func overrideChildren(node *tview.TreeNode, selected bool) {
	children := node.GetChildren()
	for _, child := range children {
		if selected {
			selectIt(child)
		} else {
			unselectIt(child)
		}
		includeExclude(child)
		refresh()

		// recurse
		if len(child.GetChildren()) > 0 {
			overrideChildren(child, selected)
		}
	}
}

// handles writing an a text file for use as --filter-from
func outputFile(filename, rulesString string) {
	if filename == "" || rulesString == "" {
		fs.Debugf(nil, "skipping outputFile as something is missing. filename: %s, rulesString: %s", filename, rulesString)
		return
	}
	file, err := os.Create(filename)
	if err != nil {
		fs.Errorf(filename, "error writing file: %v", err)
	}
	defer func() { _ = file.Close() }()

	header := fmt.Sprintf("# Filters for %s generated by rclone genfilters %s\n\n", fsPath, time.Now().Format(time.DateTime))
	header += fmt.Sprintf("# Example usage: \n# rclone tree %s --filter-from %s\n", strconv.Quote(fsPath), strconv.Quote(outputfile))
	header += "# see https://rclone.org/filtering/#filter-from-read-filtering-patterns-from-a-file\n\n"
	header += fmt.Sprintf("# rclone bisync remote1:path1 remote2:path2 --create-empty-src-dirs --compare size,modtime,checksum --slow-hash-sync-only --resilient -MvP --drive-skip-gdocs --fix-case --resync --dry-run --filters-file %s\n", strconv.Quote(outputfile))
	header += "# see https://rclone.org/bisync/#filtering\n"
	header += fmt.Sprintf("\n# To generate:\n# rclone genfilters %s -o %s\n", strconv.Quote(fsPath), strconv.Quote(outputfile))
	header += "\n# REMINDERS:\n# Do not leave any trailing whitespace after paths! (leading whitespace is fine)\n# order matters!\n"

	_, err = fmt.Fprintf(file, "%s\n%s", header, rulesString)
	if err != nil {
		fs.Errorf(filename, "error writing file: %v", err)
	} else {
		if !noOpen {
			_ = open.Start(filename)
		}
	}
}

// forEachLine calls fn on every line in the file pointed to by path
//
// It ignores empty lines and lines starting with '#' or ';' if raw is false
func forEachLine(path string, raw bool, fn func(string) error) (err error) {
	var scanner *bufio.Scanner
	if path == "-" {
		scanner = bufio.NewScanner(os.Stdin)
	} else {
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		scanner = bufio.NewScanner(in)
		defer fs.CheckClose(in, &err)
	}
	for scanner.Scan() {
		line := scanner.Text()
		if !raw {
			line = strings.TrimSpace(line)
			if len(line) == 0 || line[0] == '#' || line[0] == ';' {
				continue
			}
		}
		err := fn(line)
		if err != nil {
			return err
		}
	}
	return scanner.Err()
}

// AddRule adds a filter rule with include/exclude indicated by the prefix
//
// These are
//
//	# Comment
//	+ glob
//	- glob
//	!
//
// '+' includes the glob, '-' excludes it and '!' resets the filter list
//
// Line comments may be introduced with '#' or ';'
func importRule(rule string) error {
	switch {
	case rule == "!":
		for k := range selected {
			delete(selected, k)
		}
		refresh()
		return nil
	case strings.HasPrefix(rule, "- "):
		if mode == includeOthers {
			importSelected(clean(rule))
		} else {
			importUnselected(clean(rule))
		}
	case strings.HasPrefix(rule, "+ "):
		if mode == includeOthers {
			importUnselected(clean(rule))
		} else {
			importSelected(clean(rule))
		}
	default:
		return fmt.Errorf("malformed rule %q", rule)
	}
	return nil
}

func clean(name string) string {
	name = strings.TrimPrefix(name, "- ")
	name = strings.TrimPrefix(name, "+ ")
	name = strings.TrimPrefix(name, "/")
	name = strings.TrimSuffix(name, "**")
	name = strings.TrimSuffix(name, "*")
	return name
}

// note that we deal in strings only here because nodes may not actually exist yet
// (if the input file includes files/folders more than 1 level deep)
func importSelected(name string) {
	// name is assumed to already have trailing slash if necessary
	delete(selected, name)
	refresh()
	// check if rule is redundant before adding
	if filt.IncludeRemote(name) == defaultInclude {
		selected[name] = !defaultInclude
	}
	refresh()
}

func importUnselected(name string) {
	// name is assumed to already have trailing slash if necessary
	delete(selected, name)
	refresh()
	// check if we need an explicit rule
	if filt.IncludeRemote(name) != defaultInclude {
		selected[name] = defaultInclude
		refresh()
	}
}

/*
results box stuff, mostly borrowed from ncdu
*/

func graphemeWidth(rs []rune) (wd int) {
	// copied/adapted from [runewidth.StringWidth]
	for _, r := range rs {
		wd = runewidth.RuneWidth(r)
		if wd > 0 {
			break
		}
	}
	return
}

// Line prints a string to given xmax, with given space
func line(x, y, xmax int, style tcell.Style, spacer rune, msg string, s tcell.Screen) {
	g := uniseg.NewGraphemes(msg)
	for g.Next() {
		rs := g.Runes()
		s.SetContent(x, y, rs[0], rs[1:], style)
		x += graphemeWidth(rs)
		if x >= xmax {
			return
		}
	}
	for ; x < xmax; x++ {
		s.SetContent(x, y, spacer, nil, style)
	}
}
