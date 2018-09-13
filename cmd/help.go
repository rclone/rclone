package cmd

import (
	"fmt"
	"os"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config/configflags"
	"github.com/ncw/rclone/fs/filter/filterflags"
	"github.com/ncw/rclone/fs/rc/rcflags"
	"github.com/ncw/rclone/lib/atexit"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Root is the main rclone command
var Root = &cobra.Command{
	Use:   "rclone",
	Short: "Sync files and directories to and from local and remote object stores - " + fs.Version,
	Long: `
Rclone syncs files to and from cloud storage providers as well as
mounting them, listing them in lots of different ways.

See the home page (https://rclone.org/) for installation, usage,
documentation, changelog and configuration walkthroughs.

`,
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		fs.Debugf("rclone", "Version %q finishing with parameters %q", fs.Version, os.Args)
		atexit.Run()
	},
}

// root help command
var helpCommand = &cobra.Command{
	Use:   "help",
	Short: Root.Short,
	Long:  Root.Long,
}

// Show the flags
var helpFlags = &cobra.Command{
	Use:   "flags",
	Short: "Show the global flags for rclone",
	Run: func(command *cobra.Command, args []string) {
		_ = command.Usage()
	},
}

// runRoot implements the main rclone command with no subcommands
func runRoot(cmd *cobra.Command, args []string) {
	if version {
		ShowVersion()
		resolveExitCode(nil)
	} else {
		_ = cmd.Usage()
		if len(args) > 0 {
			_, _ = fmt.Fprintf(os.Stderr, "Command not found.\n")
		}
		resolveExitCode(errorCommandNotFound)
	}
}

// setupRootCommand sets default usage, help, and error handling for
// the root command.
//
// Helpful example: http://rtfcode.com/xref/moby-17.03.2-ce/cli/cobra.go
func setupRootCommand(rootCmd *cobra.Command) {
	// Add global flags
	configflags.AddFlags(pflag.CommandLine)
	filterflags.AddFlags(pflag.CommandLine)
	rcflags.AddFlags(pflag.CommandLine)

	Root.Run = runRoot
	Root.Flags().BoolVarP(&version, "version", "V", false, "Print the version number")

	cobra.AddTemplateFunc("showGlobalFlags", func(cmd *cobra.Command) bool {
		return cmd.CalledAs() == "flags"
	})
	cobra.AddTemplateFunc("showCommands", func(cmd *cobra.Command) bool {
		return cmd.CalledAs() != "flags"
	})
	cobra.AddTemplateFunc("showLocalFlags", func(cmd *cobra.Command) bool {
		return cmd.CalledAs() != "rclone"
	})
	rootCmd.SetUsageTemplate(usageTemplate)
	// rootCmd.SetHelpTemplate(helpTemplate)
	// rootCmd.SetFlagErrorFunc(FlagErrorFunc)
	rootCmd.SetHelpCommand(helpCommand)
	// rootCmd.PersistentFlags().BoolP("help", "h", false, "Print usage")
	// rootCmd.PersistentFlags().MarkShorthandDeprecated("help", "please use --help")

	rootCmd.AddCommand(helpCommand)
	helpCommand.AddCommand(helpFlags)
	// rootCmd.AddCommand(helpBackend)
	// rootCmd.AddCommand(helpBackends)

	cobra.OnInitialize(initConfig)

}

var usageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if and (showCommands .) .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if and (showLocalFlags .) .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if and (showGlobalFlags .) .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}

Use "{{.CommandPath}} [command] --help" for more information about a command.
Use "rclone help flags" for more information about global flags.
Use "rclone help backends" for a list of supported services.
`
