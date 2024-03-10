package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/spf13/cobra"
)

// Make a debug message while doing the completion.
//
// These end up in the file specified by BASH_COMP_DEBUG_FILE
func compLogf(format string, a ...any) {
	cobra.CompDebugln(fmt.Sprintf(format, a...), true)
}

// Add remotes to the completions being built up
func addRemotes(toComplete string, completions []string) []string {
	remotes := config.FileSections()
	for _, remote := range remotes {
		remote += ":"
		if strings.HasPrefix(remote, toComplete) {
			completions = append(completions, remote)
		}
	}
	return completions
}

// Add local files to the completions being built up
func addLocalFiles(toComplete string, result cobra.ShellCompDirective, completions []string) (cobra.ShellCompDirective, []string) {
	path := filepath.Clean(toComplete)
	dir, file := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	if len(dir) > 0 && dir[0] != filepath.Separator && dir[0] != '/' {
		dir = strings.TrimRight(dir, string(filepath.Separator))
		dir = strings.TrimRight(dir, "/")
	}
	fi, err := os.Stat(toComplete)
	if err == nil {
		if fi.IsDir() {
			dir = toComplete
			file = ""
		}
	}
	fis, err := os.ReadDir(dir)
	if err != nil {
		compLogf("Failed to read directory %q: %v", dir, err)
		return result, completions
	}
	for _, fi := range fis {
		name := fi.Name()
		if strings.HasPrefix(name, file) {
			path := filepath.Join(dir, name)
			if fi.IsDir() {
				path += string(filepath.Separator)
				result |= cobra.ShellCompDirectiveNoSpace
			}
			completions = append(completions, path)
		}
	}
	return result, completions
}

// Add remote files to the completions being built up
func addRemoteFiles(toComplete string, result cobra.ShellCompDirective, completions []string) (cobra.ShellCompDirective, []string) {
	ctx := context.Background()
	parent, _, err := fspath.Split(toComplete)
	if err != nil {
		compLogf("Failed to split path %q: %v", toComplete, err)
		return result, completions
	}
	f, err := cache.Get(ctx, parent)
	if err == fs.ErrorIsFile {
		completions = append(completions, toComplete)
		return result, completions
	} else if err != nil {
		compLogf("Failed to make Fs %q: %v", parent, err)
		return result, completions
	}
	fis, err := f.List(ctx, "")
	if err != nil {
		compLogf("Failed to list Fs %q: %v", parent, err)
		return result, completions
	}
	for _, fi := range fis {
		remote := fi.Remote()
		path := parent + remote
		if strings.HasPrefix(path, toComplete) {
			if _, ok := fi.(fs.Directory); ok {
				path += "/"
				result |= cobra.ShellCompDirectiveNoSpace
			}
			completions = append(completions, path)
		}
	}
	return result, completions
}

// Workaround doesn't seem to be needed for BashCompletionV2
const useColonWorkaround = false

// do command completion
//
// This is called by the command completion scripts using a hidden __complete or __completeNoDesc commands.
func validArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	compLogf("ValidArgsFunction called with args=%q toComplete=%q", args, toComplete)

	fixBug := -1
	if useColonWorkaround {
		// Work around what I think is a bug in cobra's bash
		// completion which seems to be splitting the arguments on :
		// Or there is something I don't understand - ncw
		args = append(args, toComplete)
		colonArg := -1
		for i, arg := range args {
			if arg == ":" {
				colonArg = i
			}
		}
		if colonArg > 0 {
			newToComplete := strings.Join(args[colonArg-1:], "")
			fixBug = len(newToComplete) - len(toComplete)
			toComplete = newToComplete
		}
		compLogf("...shuffled args=%q toComplete=%q", args, toComplete)
	}

	result := cobra.ShellCompDirectiveDefault
	completions := []string{}

	// See whether we have a valid remote yet
	_, err := fspath.Parse(toComplete)
	parseOK := err == nil
	hasColon := strings.ContainsRune(toComplete, ':')
	validRemote := parseOK && hasColon
	compLogf("valid remote = %v", validRemote)

	// Add remotes for completion
	if !validRemote {
		completions = addRemotes(toComplete, completions)
	}

	// Add local files for completion
	if !validRemote {
		result, completions = addLocalFiles(toComplete, result, completions)
	}

	// Add remote files for completion
	if validRemote {
		result, completions = addRemoteFiles(toComplete, result, completions)
	}

	// If using bug workaround, adjust completions to start with :
	if useColonWorkaround && fixBug >= 0 {
		for i := range completions {
			if len(completions[i]) >= fixBug {
				completions[i] = completions[i][fixBug:]
			}
		}
	}

	return completions, result
}
