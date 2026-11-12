//go:build !plan9 && !js && !aix

// Package genfilters implements a text based user interface for making filter rules
package genfilters

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/lib/terminal"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"
)

// Options configures Run
type Options struct {
	Edit          string // filter file to edit; if empty read stdin and write stdout
	ExcludeOthers bool   // start excluding everything without a rule
	Open          bool   // open the edited file when done
	Regex         bool   // also write the rules as regular expressions to stderr
	Cmd           bool   // also write the rules as --filter flags to stderr
}

// ErrCancelled is returned by Run when the user quits without writing the rules
var ErrCancelled = errors.New("cancelled: the filter rules were not written")

var opt Options

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.StringVarP(cmdFlags, &opt.Edit, "edit", "", opt.Edit, "Edit this filter file, creating it if needed (the old version is kept as FILE.bak)", "")
	flags.BoolVarP(cmdFlags, &opt.ExcludeOthers, "exclude-others", "", opt.ExcludeOthers, "Start with everything excluded (- **) instead of included", "")
	flags.BoolVarP(cmdFlags, &opt.Open, "open", "", opt.Open, "Open the filter file when done (needs --edit)", "")
	flags.BoolVarP(cmdFlags, &opt.Regex, "regex", "", opt.Regex, "Also write the rules as regular expressions to stderr", "")
	flags.BoolVarP(cmdFlags, &opt.Cmd, "cmd", "", opt.Cmd, "Also write the rules as --filter flags to stderr", "")
}

var commandDefinition = &cobra.Command{
	Use:   "genfilters remote:path",
	Short: `Make filter rules with a text based user interface.`,
	Long: `This shows the files and directories of a remote in a text based
user interface, where you choose what to include and exclude. It makes
the [filter rules](/filtering/) for your choices, which you can use
with [` + "`--filter-from`" + `](/filtering/#filter-from-read-filtering-patterns-from-a-file)
or bisync's [` + "`--filters-file`" + `](/bisync/#filtering).

{{< asciinema ynXSjMcBUc8Gfg9V >}}

The rules are checked with rclone's own filters as you go, so what is
shown is what rclone will include. Directories are listed when you
open them, so it works well with big remotes.

With ` + "`--edit FILE`" + ` the rules are read from FILE, if it exists, and
written back to it when you finish. The old version is kept as
FILE.bak. For example:

` + "```console" + `
rclone genfilters remote:path --edit filters.txt
` + "```" + `

Otherwise the rules are read from standard input, if it isn't a
terminal, and written to standard output. To make a new filter file:

` + "```console" + `
rclone genfilters remote:path > filters.txt
` + "```" + `

To start from an existing one and write the result somewhere else:

` + "```console" + `
rclone genfilters remote:path < old.txt > new.txt
` + "```" + `

` + "`--regex`" + ` and ` + "`--cmd`" + ` also write the rules to standard error, as
the regular expressions rclone uses and as ` + "`--filter`" + ` flags, so they
stay out of the rules on standard output:

` + "```console" + `
rclone genfilters remote:path --regex --cmd > filters.txt 2> extra.txt
` + "```" + `

Everything without its own rule is included (` + "`+ **`" + `) unless you use
` + "`--exclude-others`" + ` or press ` + "`x`" + `, or the rules read in end with
` + "`- **`" + `.

Press TAB to edit the rules as text. The tree changes as you type. A
line with an error is shown in red, and the tree keeps the last rules
without errors. genfilters can only read rules it could have made: a
file (` + "`+ /path/file`" + `) or a directory and everything in it
(` + "`- /path/dir/**`" + `), each before any rule for a directory containing
it, then ` + "`+ **`" + ` or ` + "`- **`" + `. Comments are kept if they come before
the first rule.

You can interact with the user interface using key presses,
press '?' to toggle the help on and off. The supported keys are:

` + "```text" + `
` + strings.Join(helpText()[1:], "\n") + `
` + "```",
	Annotations: map[string]string{
		"versionIntroduced": "v1.76",
		"groups":            "Listing",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			return Run(context.Background(), fsrc, opt)
		})
	},
}

// Run shows the user interface for making filter rules for f
func Run(ctx context.Context, f fs.Fs, opt Options) error {
	if fi := filter.GetConfig(ctx); !fi.InActive() {
		return fmt.Errorf("genfilters can't be used with filter flags, as they would change what is shown:\n%s", fi.DumpFilters())
	}
	if opt.Open && opt.Edit == "" {
		return errors.New("--open needs --edit")
	}

	// Read the rules
	var (
		in     []byte
		source = "standard input"
		err    error
	)
	if opt.Edit != "" {
		source = opt.Edit
		in, err = os.ReadFile(opt.Edit)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	} else if !terminal.IsTerminal(int(os.Stdin.Fd())) {
		if in, err = io.ReadAll(os.Stdin); err != nil {
			return err
		}
	}
	mo, hasRules, err := parse(splitLines(string(in)))
	if err != nil {
		return fmt.Errorf("can't read the rules in %s: %w", source, err)
	}
	if opt.ExcludeOthers {
		if hasRules && mo.mode != excludeOthers {
			return fmt.Errorf("--exclude-others can't be used as the rules in %s include everything without a rule", source)
		}
		if !hasRules {
			if err := mo.setMode(excludeOthers); err != nil {
				return err
			}
		}
	}
	if len(mo.header) == 0 {
		mo.header = []string{
			fmt.Sprintf("# Filter rules for %s made with rclone genfilters", strconv.Quote(fs.ConfigString(f))),
			"# See https://rclone.org/filtering/",
			"",
		}
	}

	u := newUI(ctx, f, mo)
	if err := u.run(); err != nil {
		return err
	}
	if u.cancelled {
		return ErrCancelled
	}
	out := strings.Join(u.mo.lines(), "\n") + "\n"

	// Write the rules
	if opt.Edit != "" {
		if err := writeFile(opt.Edit, in, out); err != nil {
			return err
		}
		if opt.Open {
			if err := open.Start(opt.Edit); err != nil {
				return fmt.Errorf("failed to open %q: %w", opt.Edit, err)
			}
		}
	} else if _, err := os.Stdout.WriteString(out); err != nil {
		return err
	}
	if opt.Regex {
		_, _ = fmt.Fprintln(os.Stderr, u.mo.filt.DumpFilters())
	}
	if opt.Cmd {
		for _, rule := range u.mo.rules() {
			_, _ = fmt.Fprintf(os.Stderr, "--filter %s\n", strconv.Quote(rule))
		}
	}
	return nil
}

// splitLines splits text into lines without line endings
func splitLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// writeFile replaces the contents of name, which were old, with out,
// keeping old in name.bak
func writeFile(name string, old []byte, out string) error {
	if string(old) == out {
		fs.Logf(nil, "The rules in %q are unchanged", name)
		return nil
	}
	perm := os.FileMode(0o644)
	if fi, err := os.Stat(name); err == nil {
		perm = fi.Mode().Perm()
		if err := os.WriteFile(name+".bak", old, perm); err != nil {
			return fmt.Errorf("failed to keep the old rules: %w", err)
		}
	}
	// Write a temporary file first so name is never left half written
	tmp, err := os.CreateTemp(filepath.Dir(name), filepath.Base(name)+".*.tmp")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.WriteString(out); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), name); err != nil {
		return err
	}
	fs.Logf(nil, "Wrote the rules to %q", name)
	return nil
}
