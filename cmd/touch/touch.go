package touch

import (
	"bytes"
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
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
	flags.BoolVarP(cmdFlags, &notCreateNewFile, "no-create", "C", false, "Do not create the file if it does not exist. Implied with --recursive.")
	flags.StringVarP(cmdFlags, &timeAsArgument, "timestamp", "t", "", "Use specified time instead of the current time of day.")
	flags.BoolVarP(cmdFlags, &localTime, "localtime", "", false, "Use localtime for timestamp, not UTC.")
	flags.BoolVarP(cmdFlags, &recursive, "recursive", "R", false, "Recursively touch all files.")
}

var commandDefinition = &cobra.Command{
	Use:   "touch remote:path",
	Short: `Create new file or change file modification time.`,
	Long: `
Set the modification time on file(s) as specified by remote:path to
have the current time.

If remote:path does not exist then a zero sized file will be created,
unless ` + "`--no-create`" + ` or ` + "`--recursive`" + ` is provided.

If ` + "`--recursive`" + ` is used then recursively sets the modification
time on all existing files that is found under the path. Filters are supported,
and you can test with the ` + "`--dry-run`" + ` or the ` + "`--interactive`" + ` flag.

If ` + "`--timestamp`" + ` is used then sets the modification time to that
time instead of the current time. Times may be specified as one of:

- 'YYMMDD' - e.g. 17.10.30
- 'YYYY-MM-DDTHH:MM:SS' - e.g. 2006-01-02T15:04:05
- 'YYYY-MM-DDTHH:MM:SS.SSS' - e.g. 2006-01-02T15:04:05.123456789

Note that value of ` + "`--timestamp`" + ` is in UTC. If you want local time
then add the ` + "`--localtime`" + ` flag.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		f, fileName := cmd.NewFsFile(args[0])
		cmd.Run(true, false, command, func() error {
			return Touch(context.Background(), f, fileName)
		})
	},
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
			return t, errors.Wrap(err, "failed to parse timestamp argument")
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
func Touch(ctx context.Context, f fs.Fs, fileName string) error {
	t, err := timeOfTouch()
	if err != nil {
		return err
	}
	fs.Debugf(nil, "Touch time %v", t)
	file, err := f.NewObject(ctx, fileName)
	if err != nil {
		if errors.Cause(err) == fs.ErrorObjectNotFound {
			// Touch single non-existent file
			if notCreateNewFile {
				fs.Logf(f, "Not touching non-existent file due to --no-create")
				return nil
			}
			if recursive {
				fs.Logf(f, "Not touching non-existent file due to --recursive")
				return nil
			}
			if operations.SkipDestructive(ctx, f, "touch (create)") {
				return nil
			}
			fs.Debugf(f, "Touching (creating)")
			if err = createEmptyObject(ctx, fileName, t, f); err != nil {
				return errors.Wrap(err, "failed to touch (create)")
			}
		}
		if errors.Cause(err) == fs.ErrorNotAFile {
			if recursive {
				// Touch existing directory, recursive
				fs.Debugf(nil, "Touching files in directory recursively")
				return operations.TouchDir(ctx, f, t, true)
			}
			// Touch existing directory without recursing
			fs.Debugf(nil, "Touching files in directory non-recursively")
			return operations.TouchDir(ctx, f, t, false)
		}
		return err
	}
	// Touch single existing file
	if !operations.SkipDestructive(ctx, fileName, "touch") {
		fs.Debugf(f, "Touching %q", fileName)
		err = file.SetModTime(ctx, t)
		if err != nil {
			return errors.Wrap(err, "failed to touch")
		}
	}
	return nil
}
