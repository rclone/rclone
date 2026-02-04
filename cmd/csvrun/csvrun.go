// Package csvrun provides the csvrun command.
package csvrun

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/errcount"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var (
	mode = "copy"
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.StringVarP(cmdFlags, &mode, "mode", "", mode, "Operation mode: copy or move", "")
}

var commandDefinition = &cobra.Command{
	Use:   "csvrun csv_file",
	Short: `Read a CSV file and perform file operations (copy/move).`,
	Long: strings.ReplaceAll(`Reads a CSV file and performs file operations (copy or move) for each row.
The CSV file must have a header row and the following columns:
SourceRemote, SourcePath, TargetRemote, TargetPath, [Mode]

- SourceRemote: The rclone remote name (e.g., "drive:") or empty for local.
- SourcePath: The path to the source file.
- TargetRemote: The rclone remote name (e.g., "backup:") or empty for local.
- TargetPath: The path to the destination file.
- Mode (Optional): The operation to perform: "copy" or "move". Overrides the --mode flag.

The first row of the CSV is expected to be a header and will be skipped.
If the "Mode" column is not present in the CSV, the --mode flag must be specified.
If both are present, the CSV column takes precedence for that row.

This command efficiently reuses remote connections (Fs objects) to optimize session
token usage and supports parallel execution using --transfers.

Example CSV:
|||csv
SourceRemote,SourcePath,TargetRemote,TargetPath,Mode
drive:,/docs/report.pdf,backup:,/2023/report.pdf,copy
s3:,bucket/image.jpg,drive:,/images/image.jpg,move
drive:,/data/log.txt,backup:,/logs/log.txt,
|||

Example usage:
|||sh
rclone csvrun transfer_list.csv --mode copy
rclone csvrun move_list.csv --mode move
|||
`, "|", "`"),
	Annotations: map[string]string{
		"versionIntroduced": "v1.70",
	},
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(1, 1, command, args)
		return run(args[0])
	},
}

func run(csvFile string) error {
	f, err := os.Open(csvFile)
	if err != nil {
		return fmt.Errorf("failed to open .csv file: %w", err)
	}
	defer fs.CheckClose(f, &err)

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1 // Allow variable number of fields to support optional 5th column


	// Read header
	_, err = reader.Read()
	if err != nil {
		if err == io.EOF {
			return fmt.Errorf("csv file is empty")
		}
		return fmt.Errorf("failed to read csv header: %w", err)
	}

	// Read all records
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed reading .csv file: %w", err)
	}

	ec := errcount.New()
	g, gCtx := errgroup.WithContext(context.Background())
	ci := fs.GetConfig(gCtx)
	g.SetLimit(ci.Transfers)

	for _, row := range records {
		if len(row) < 4 {
			continue
		}

		// Capture variables for the closure
		row := row
		srcRemote, srcPath := row[0], row[1]
		dstRemote, dstPath := row[2], row[3]
		
		rowMode := mode
		if len(row) >= 5 && row[4] != "" {
			rowMode = strings.ToLower(strings.TrimSpace(row[4]))
		}

		if rowMode == "" {
			err := fmt.Errorf("mode not specified in CSV or flag for row: %v", row)
			fs.Errorf(nil, "%v", err)
			ec.Add(err)
			continue
		}

		cp := true
		if rowMode == "move" {
			cp = false
		} else if rowMode != "copy" {
			err := fmt.Errorf("invalid mode %q: must be 'copy' or 'move'", rowMode)
			fs.Errorf(nil, "%v", err)
			ec.Add(err)
			continue
		}
		
		// Ensure remotes end with : if they are not empty and not local paths starting with / or .
		// Actually, fs.NewFs expects "remote:" or "remote:path". 
		// The requirement says "SourceRemoteName is the rclone conf name". 
		// Usually rclone remotes in config don't have colon in the name, but usage requires it.
		// Let's assume the user puts the name "drive" and we might need to append ":" if missing,
		// OR we rely on standard rclone syntax. 
		// The prompt says "SourceRemoteName is the rclone conf name". 
		// If I look at rclone internals, NewFs takes "remote:path". 
		// If the CSV splits them, we should join them as "RemoteName:".
		// Let's allow users to provide "drive:" or "drive". 
		
		// Helper to ensure valid fs string
		getFsString := func(remote string) string {
			if remote == "" {
				return "" // Local
			}
			if !strings.Contains(remote, ":") && !strings.HasPrefix(remote, "/") && !strings.HasPrefix(remote, ".") {
				return remote + ":"
			}
			return remote
		}

		srcFsString := getFsString(srcRemote)
		dstFsString := getFsString(dstRemote)

		g.Go(func() error {
			// Get Fs from cache or create new
			// We use cache.Get to reuse Fs objects
			
			// For Source
			// cache.Get takes a remote path string.
			// If we pass just "drive:", it returns the root Fs.
			srcFs, err := cache.Get(gCtx, srcFsString)
			if err != nil {
				err = fmt.Errorf("failed to make fs for %q: %w", srcFsString, err)
				fs.Errorf(nil, "%v", err)
				ec.Add(err)
				return nil
			}

			// For Target
			dstFs, err := cache.Get(gCtx, dstFsString)
			if err != nil {
				err = fmt.Errorf("failed to make fs for %q: %w", dstFsString, err)
				fs.Errorf(nil, "%v", err)
				ec.Add(err)
				return nil
			}

			// MoveOrCopyFile
			err = operations.MoveOrCopyFile(gCtx, dstFs, srcFs, dstPath, srcPath, cp, false)
			if err != nil {
				fs.Errorf(srcPath, "Failed to %s to %s: %v", rowMode, dstPath, err)
				ec.Add(err)
			}
			return nil
		})
	}

	ec.Add(g.Wait())
	return ec.Err(fmt.Sprintf("not all operations succeeded"))
}

