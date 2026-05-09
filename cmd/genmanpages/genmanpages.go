// Package genmanpages provides the genmanpages command.
package genmanpages

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/lib/file"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/spf13/pflag"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
}

var commandDefinition = &cobra.Command{
	Use:   "genmanpages output_directory",
	Short: `Output man pages for rclone to the directory supplied.`,
	Long: `This produces man page files for all rclone commands to the directory
supplied. The pages use the standard naming convention (rclone-copy.1,
rclone-sync.1, etc.) similar to git's man page layout.`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.73",
	},
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(1, 1, command, args)

		out := args[0]
		err := file.MkdirAll(out, 0777)
		if err != nil {
			return err
		}

		now := time.Now()
		header := &doc.GenManHeader{
			Title:   "RCLONE",
			Section: "1",
			Date:    &now,
			Source:  "Rclone",
			Manual:  "User Manual",
		}

		// Inject shared option groups into each command's Long description
		// before generating man pages, since cobra/doc only renders what's
		// in the command's Long + flags.
		injectSharedOptions(cmd.Root)

		// Hide all persistent flags on Root so they don't appear as
		// "OPTIONS INHERITED FROM PARENT COMMANDS" in every subcommand
		// man page. These are the massive backend flags that make man
		// pages unusable. We restore them after generation.
		var hiddenFlags []string
		cmd.Root.PersistentFlags().VisitAll(func(f *pflag.Flag) {
			if !f.Hidden {
				f.Hidden = true
				hiddenFlags = append(hiddenFlags, f.Name)
			}
		})
		pflag.CommandLine.VisitAll(func(f *pflag.Flag) {
			if !f.Hidden {
				f.Hidden = true
				hiddenFlags = append(hiddenFlags, f.Name)
			}
		})
		defer func() {
			for _, name := range hiddenFlags {
				if f := cmd.Root.PersistentFlags().Lookup(name); f != nil {
					f.Hidden = false
				}
				if f := pflag.CommandLine.Lookup(name); f != nil {
					f.Hidden = false
				}
			}
		}()

		// Generate man pages for all subcommands (not root).
		// We skip root because it has thousands of backend flags registered
		// on it via pflag.CommandLine which makes for an unusable man page.
		for _, c := range cmd.Root.Commands() {
			if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
				continue
			}
			err = doc.GenManTreeFromOpts(c, doc.GenManTreeOptions{
				Header:           header,
				Path:             out,
				CommandSeparator: "-",
			})
			if err != nil {
				return err
			}
		}

		// Generate a concise root man page using a temporary command
		// that has only the essential content.
		err = genRootManPage(out, header)
		if err != nil {
			return err
		}

		fmt.Printf("Generated %d man pages in %s\n", countFiles(out), out)
		return nil
	},
}

// genRootManPage creates a concise rclone.1 man page with shared root help
// text and SEE ALSO references to all subcommands.
func genRootManPage(dir string, header *doc.GenManHeader) error {
	// Build a temporary command tree that mirrors the real one
	// but without the massive flags list.
	rootCmd := &cobra.Command{
		Use:               cmd.Root.Use + " [flags]",
		Short:             rootManPageShort(cmd.Root),
		Long:              cmd.Root.Long,
		DisableAutoGenTag: true,
	}

	// Add subcommands as stubs for SEE ALSO generation
	for _, c := range cmd.Root.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}
		rootCmd.AddCommand(&cobra.Command{
			Use:                c.Use,
			Short:              c.Short,
			DisableAutoGenTag:  true,
			DisableFlagParsing: true,
			Run:                func(_ *cobra.Command, _ []string) {},
		})
	}

	var buf bytes.Buffer
	err := doc.GenMan(rootCmd, header, &buf)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, "rclone.1"), buf.Bytes(), 0644)
}

func rootManPageShort(root *cobra.Command) string {
	summary := strings.TrimSpace(root.Long)
	if summary != "" {
		summary = strings.SplitN(summary, "\n\n", 2)[0]
		summary = strings.Join(strings.Fields(summary), " ")
	}
	if summary == "" {
		summary = strings.TrimSpace(root.Short)
	}
	return summary
}

// countFiles counts .1 files in a directory.
func countFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".1") {
			count++
		}
	}
	return count
}

// injectSharedOptions appends shared option group documentation to each
// command's Long description. This replicates the behavior in gendocs.go
// where shared flags (Copy, Sync, Filter, etc.) are documented per command.
func injectSharedOptions(root *cobra.Command) {
	injectSharedOptionsRecursive(root)
}

func injectSharedOptionsRecursive(root *cobra.Command) {
	for _, c := range root.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}
		injectSharedOptionsRecursive(c)
	}

	groupsString := root.Annotations["groups"]
	if groupsString == "" {
		return
	}

	var out strings.Builder
	out.WriteString("\n\n")
	out.WriteString("Options shared with other commands are described next.\n")
	out.WriteString("See the global flags page for global options not listed here.\n\n")

	groups := flags.All.Include(groupsString)
	for _, group := range groups.Groups {
		if group.Flags.HasFlags() {
			fmt.Fprintf(&out, "## %s Options\n\n", group.Name)
			fmt.Fprintf(&out, "%s\n\n", group.Help)
			out.WriteString("```\n")
			out.WriteString(group.Flags.FlagUsages())
			out.WriteString("```\n\n")
		}
	}

	root.Long += out.String()
}
