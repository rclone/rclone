//go:build !plan9

// Package list implements 'rclone archive list'
package list

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mholt/archives"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/archive"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

var (
	longList  = false
	plainList = false
	filesOnly = false
	dirsOnly  = false
)

func init() {
	flagSet := Command.Flags()
	flags.BoolVarP(flagSet, &longList, "long", "", longList, "List extra attributtes", "")
	flags.BoolVarP(flagSet, &plainList, "plain", "", plainList, "Only list file names", "")
	flags.BoolVarP(flagSet, &filesOnly, "files-only", "", false, "Only list files", "")
	flags.BoolVarP(flagSet, &dirsOnly, "dirs-only", "", false, "Only list directories", "")
	archive.Command.AddCommand(Command)
}

// Command - list
var Command = &cobra.Command{
	Use:   "list [flags] <source>",
	Short: `List archive contents from source.`,
	Long: strings.ReplaceAll(`
List the contents of an archive to the console, auto detecting the
format. See [rclone archive create](/commands/rclone_archive_create/)
for the archive formats supported.

For example:

|||
$ rclone archive list remote:archive.zip
        6 file.txt
        0 dir/
        4 dir/bye.txt
|||

Or with |--long| flag for more info:

|||
$ rclone archive list --long remote:archive.zip
        6 2025-10-30 09:46:23.000000000 file.txt
        0 2025-10-30 09:46:57.000000000 dir/
        4 2025-10-30 09:46:57.000000000 dir/bye.txt
|||

Or with |--plain| flag which is useful for scripting:

|||
$ rclone archive list --plain /path/to/archive.zip
file.txt
dir/
dir/bye.txt
|||

Or with |--dirs-only|:

|||
$ rclone archive list --plain --dirs-only /path/to/archive.zip
dir/
|||

Or with |--files-only|:

|||
$ rclone archive list --plain --files-only /path/to/archive.zip
file.txt
dir/bye.txt
|||

Filters may also be used:

|||
$ rclone archive list --long archive.zip --include "bye.*"
        4 2025-10-30 09:46:57.000000000 dir/bye.txt
|||

The [archive backend](/archive/) can also be used to list files. It
can be used to read only mount archives also but it supports a
different set of archive formats to the archive commands.
`, "|", "`"),
	Annotations: map[string]string{
		"versionIntroduced": "v1.72",
	},
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(1, 1, command, args)
		src, srcFile := cmd.NewFsFile(args[0])
		cmd.Run(false, false, command, func() error {
			return ArchiveList(context.Background(), src, srcFile, listFile)
		})
		return nil
	},
}

func listFile(ctx context.Context, f archives.FileInfo) error {
	ci := fs.GetConfig(ctx)
	fi := filter.GetConfig(ctx)

	// check if excluded
	if !fi.Include(f.NameInArchive, f.Size(), f.ModTime(), fs.Metadata{}) {
		return nil
	}
	if filesOnly && f.IsDir() {
		return nil
	}
	if dirsOnly && !f.IsDir() {
		return nil
	}
	// get entry name
	name := f.NameInArchive
	if f.IsDir() && !strings.HasSuffix(name, "/") {
		name += "/"
	}
	// print info
	if longList {
		operations.SyncFprintf(os.Stdout, "%s %s %s\n", operations.SizeStringField(f.Size(), ci.HumanReadable, 9), f.ModTime().Format("2006-01-02 15:04:05.000000000"), name)
	} else if plainList {
		operations.SyncFprintf(os.Stdout, "%s\n", name)
	} else {
		operations.SyncFprintf(os.Stdout, "%s %s\n", operations.SizeStringField(f.Size(), ci.HumanReadable, 9), name)
	}
	return nil
}

// ArchiveList -- print a list of the files in the archive
func ArchiveList(ctx context.Context, src fs.Fs, srcFile string, listFn archives.FileHandler) error {
	var srcObj fs.Object
	var err error
	// get object
	srcObj, err = src.NewObject(ctx, srcFile)
	if err != nil {
		return fmt.Errorf("source is not a file, %w", err)
	}
	fs.Debugf(nil, "Source archive file: %s/%s", src.Root(), srcFile)
	// start accounting
	tr := accounting.Stats(ctx).NewTransfer(srcObj, nil)
	defer func() {
		tr.Done(ctx, err)
	}()
	// open source
	var options []fs.OpenOption
	for _, option := range fs.GetConfig(ctx).DownloadHeaders {
		options = append(options, option)
	}
	in0, err := operations.Open(ctx, srcObj, options...)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", srcFile, err)
	}
	// account and buffer the transfer
	// in = tr.Account(ctx, in).WithBuffer()
	in := tr.Account(ctx, in0)
	// identify format
	format, _, err := archives.Identify(ctx, "", in)

	if err != nil {
		return fmt.Errorf("failed to open check file type, %w", err)
	}
	fs.Debugf(nil, "Listing %s/%s, format %s", src.Root(), srcFile, strings.TrimPrefix(format.Extension(), "."))
	// check if extract is supported by format
	ex, isExtract := format.(archives.Extraction)
	if !isExtract {
		return fmt.Errorf("extraction for %s not supported", strings.TrimPrefix(format.Extension(), "."))
	}
	// list files
	return ex.Extract(ctx, in, listFn)
}
