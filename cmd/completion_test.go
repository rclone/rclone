package cmd

import (
	"bytes"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnumFlagCompletion(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{"option", []string{"serve", "--cache-mode", ""}, "off\nminimal\nwrites\nfull\n:4\n"},
		{"prefix", []string{"serve", "--cache-mode", "f"}, "full\n:4\n"},
		{"equals", []string{"serve", "--cache-mode=f"}, "full\n:4\n"},
		{"invalid", []string{"serve", "--cache-mode", "invalid"}, ":4\n"},
		{"global", []string{"serve", "--log-level", "d"}, "DEBUG\n:4\n"},
		{"inherited", []string{"serve", "http", "--log-level=D"}, "DEBUG\n:4\n"},
		{"local", []string{"serve", "http", "--local-mode", ""}, "off\nminimal\nwrites\nfull\n:4\n"},
		{"custom-inherited", []string{"serve", "http", "--custom-mode", ""}, "custom\n:4\n"},
		{"custom", []string{"serve", "--custom-mode", ""}, "custom\n:4\n"},
		{"string", []string{"serve", "--name", ""}, ":0\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := &cobra.Command{Use: "rclone"}
			serve := &cobra.Command{Use: "serve"}
			http := &cobra.Command{Use: "http", Run: func(*cobra.Command, []string) {}}
			local := vfscommon.CacheModeOff
			http.Flags().Var(&local, "local-mode", "Local mode")
			serve.AddCommand(http)
			root.AddCommand(serve)
			level := fs.LogLevelNotice
			root.PersistentFlags().Var(&level, "log-level", "Log level")
			option := &fs.Option{Default: vfscommon.CacheModeOff}
			serve.PersistentFlags().Var(option, "cache-mode", "Cache mode")
			custom := vfscommon.CacheModeOff
			serve.PersistentFlags().Var(&custom, "custom-mode", "Custom mode")
			require.NoError(t, serve.RegisterFlagCompletionFunc("custom-mode", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
				return []string{"custom"}, cobra.ShellCompDirectiveNoFileComp
			}))
			serve.PersistentFlags().String("name", "", "Name")
			require.NoError(t, registerEnumFlagCompletions(root))
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&bytes.Buffer{})
			root.SetArgs(append([]string{"__complete"}, test.args...))
			require.NoError(t, root.Execute())
			assert.Equal(t, test.want, out.String())
		})
	}
}

func TestEnumGlobalFlagCompletion(t *testing.T) {
	previous := pflag.CommandLine
	pflag.CommandLine = pflag.NewFlagSet("global", pflag.ContinueOnError)
	t.Cleanup(func() { pflag.CommandLine = previous })
	level := fs.LogLevelNotice
	pflag.CommandLine.Var(&level, "log-level", "Log level")
	root := &cobra.Command{Use: "rclone"}
	root.AddCommand(&cobra.Command{Use: "copy", Run: func(*cobra.Command, []string) {}})
	require.NoError(t, registerEnumFlagCompletions(root))
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"__complete", "copy", "--log-level", "d"})
	require.NoError(t, root.Execute())
	assert.Equal(t, "DEBUG\n:4\n", out.String())
}

func TestEnumFlagCompletionHidden(t *testing.T) {
	root := &cobra.Command{Use: "rclone"}
	mode := vfscommon.CacheModeOff
	root.Flags().Var(&mode, "hidden-mode", "Hidden mode")
	require.NoError(t, root.Flags().MarkHidden("hidden-mode"))
	require.NoError(t, registerEnumFlagCompletions(root))
	_, exists := root.GetFlagCompletionFunc("hidden-mode")
	assert.False(t, exists)
}
