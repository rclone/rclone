// Package touch provides the touch command.
package touch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

var (
	notCreateNewFile bool
	timeAsArgument   string
	localTime        bool
	recursive        bool
)

const (
	defaultLayout          string = "060102"
	layoutDateWithTime     string = "2006-01-02T15:04:05"
	layoutDateWithTimeNano string = "2006-01-02T15:04:05.999999999"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.BoolVarP(cmdFlags, &notCreateNewFile, "no-create", "C", false, "Do not create the file if it does not exist (implied with --recursive)", "")
	flags.StringVarP(cmdFlags, &timeAsArgument, "timestamp", "t", "", "Use specified time instead of the current time of day", "")
	flags.BoolVarP(cmdFlags, &localTime, "localtime", "", false, "Use localtime for timestamp, not UTC", "")
	flags.BoolVarP(cmdFlags, &recursive, "recursive", "R", false, "Recursively touch all files", "")
}

var commandDefinition = &cobra.Command{
	Use:   "touch remote:path",
	Short: `Create new file or change file modification time.`,
	Long: `Set the modification time on file(s) as specified by remote:path to
have the current time.

If remote:path does not exist then a zero sized file will be created,
unless ` + "`--no-create`" + ` or ` + "`--recursive`" + ` is provided.

If ` + "`--recursive`" + ` is used then recursively sets the modification
time on all existing files that is found under the path. Filters are supported,
and you can test with the ` + "`--dry-run`" + ` or the ` + "`--interactive`/`-i`" + ` flag.

If ` + "`--timestamp`" + ` is used then sets the modification time to that
time instead of the current time. Times may be specified as one of:

- 'YYMMDD' - e.g. 17.10.30
- 'YYYY-MM-DDTHH:MM:SS' - e.g. 2006-01-02T15:04:05
- 'YYYY-MM-DDTHH:MM:SS.SSS' - e.g. 2006-01-02T15:04:05.123456789

Note that value of ` + "`--timestamp`" + ` is in UTC. If you want local time
then add the ` + "`--localtime`" + ` flag.
`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.39",
		"groups":            "Filter,Listing,Important",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		f, remote := newFsDst(args)
		cmd.Run(true, false, command, func() error {
			return Touch(context.Background(), f, remote)
		})
	},
}

// newFsDst creates a new dst fs from the arguments.
//
// The returned fs will never point to a file. It will point to the
// parent directory of specified path, and is returned together with
// the basename of file or directory, except if argument is only a
// remote name. Similar to cmd.NewFsDstFile, but without raising fatal
// when name of file or directory is empty (e.g. "remote:" or "remote:path/").
func newFsDst(args []string) (f fs.Fs, remote string) {
	root, remote, err := fspath.Split(args[0])
	if err != nil {
		fs.Fatalf(nil, "Parsing %q failed: %v", args[0], err)
	}
	if root == "" {
		root = "."
	}
	f = cmd.NewFsDir([]string{root})
	return f, remote
}

// parseTimeArgument parses a timestamp string according to specific layouts
func parseTimeArgument(timeString string) (time.Time, error) {
	layout := defaultLayout
	if len(timeString) == len(layoutDateWithTime) {
		layout = layoutDateWithTime
	} else if len(timeString) > len(layoutDateWithTime) {
		layout = layoutDateWithTimeNano
	}
	if localTime {
		return time.ParseInLocation(layout, timeString, time.Local)
	}
	return time.Parse(layout, timeString)
}

// timeOfTouch returns the time value set on files
func timeOfTouch() (time.Time, error) {
	var t time.Time
	if timeAsArgument != "" {
		var err error
		if t, err = parseTimeArgument(timeAsArgument); err != nil {
			return t, fmt.Errorf("failed to parse timestamp argument: %w", err)
		}
	} else {
		t = time.Now()
	}
	return t, nil
}

// createEmptyObject creates an empty object (file) with specified timestamp
func createEmptyObject(ctx context.Context, remote string, modTime time.Time, f fs.Fs) error {
	var buffer []byte
	src := object.NewStaticObjectInfo(remote, modTime, int64(len(buffer)), true, nil, f)
	_, err := f.Put(ctx, bytes.NewBuffer(buffer), src)
	return err
}

// Touch create new file or change file modification time.
func Touch(ctx context.Context, f fs.Fs, remote string) error {
	t, err := timeOfTouch()
	if err != nil {
		return err
	}
	fs.Debugf(nil, "Touch time %v", t)
	var file fs.Object
	if remote == "" {
		err = fs.ErrorIsDir
	} else {
		file, err = f.NewObject(ctx, remote)
	}
	if err != nil {
		if errors.Is(err, fs.ErrorObjectNotFound) {
			// Touching non-existent path, possibly creating it as new file
			if remote == "" {
				fs.Logf(f, "Not touching empty directory")
				return nil
			}
			if notCreateNewFile {
				fs.Logf(f, "Not touching nonexistent file due to --no-create")
				return nil
			}
			if recursive {
				// For consistency, --recursive never creates new files.
				fs.Logf(f, "Not touching nonexistent file due to --recursive")
				return nil
			}
			if operations.SkipDestructive(ctx, f, "touch (create)") {
				return nil
			}
			fs.Debugf(f, "Touching (creating) %q", remote)
			if err = createEmptyObject(ctx, remote, t, f); err != nil {
				return fmt.Errorf("failed to touch (create): %w", err)
			}
		}
		if errors.Is(err, fs.ErrorIsDir) {
			// Touching existing directory
			if recursive {
				fs.Debugf(f, "Touching recursively files in directory %q", remote)
				return operations.TouchDir(ctx, f, remote, t, true)
			}
			fs.Debugf(f, "Touching non-recursively files in directory %q", remote)
			return operations.TouchDir(ctx, f, remote, t, false)
		}
		return err
	}
	// Touch single existing file
	if !operations.SkipDestructive(ctx, remote, "touch") {
		fs.Debugf(f, "Touching %q", remote)
		err = file.SetModTime(ctx, t)
		if err != nil {
			return fmt.Errorf("failed to touch: %w", err)
		}
	}
	return nil
}
