//go:build !plan9

// Package extract implements 'rclone archive extract'
package extract

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/mholt/archives"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/archive"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

func init() {
	archive.Command.AddCommand(Command)
}

// Command - extract
var Command = &cobra.Command{
	Use:   "extract [flags] <source> <destination>",
	Short: `Extract archives from source to destination.`,
	Long: strings.ReplaceAll(`

Extract the archive contents to a destination directory auto detecting
the format. See [rclone archive create](/commands/rclone_archive_create/)
for the archive formats supported.

For example on this archive:

|||
$ rclone archive list --long remote:archive.zip
        6 2025-10-30 09:46:23.000000000 file.txt
        0 2025-10-30 09:46:57.000000000 dir/
        4 2025-10-30 09:46:57.000000000 dir/bye.txt
|||

You can run extract like this

|||
$ rclone archive extract remote:archive.zip remote:extracted
|||

Which gives this result

|||
$ rclone tree remote:extracted
/
├── dir
│   └── bye.txt
└── file.txt
|||

The source or destination or both can be local or remote.

Filters can be used to only extract certain files:

|||
$ rclone archive extract archive.zip partial --include "bye.*"
$ rclone tree partial
/
└── dir
    └── bye.txt
|||

The [archive backend](/archive/) can also be used to extract files. It
can be used to read only mount archives also but it supports a
different set of archive formats to the archive commands.
`, "|", "`"),
	Annotations: map[string]string{
		"versionIntroduced": "v1.72",
	},
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(2, 2, command, args)

		src, srcFile := cmd.NewFsFile(args[0])
		dst, dstFile := cmd.NewFsFile(args[1])

		cmd.Run(false, false, command, func() error {
			return ArchiveExtract(context.Background(), dst, dstFile, src, srcFile)
		})
		return nil
	},
}

// ArchiveExtract extracts files from (src, srcFile) to (dst, dstDir)
func ArchiveExtract(ctx context.Context, dst fs.Fs, dstDir string, src fs.Fs, srcFile string) error {
	var srcObj fs.Object
	var filesExtracted = 0
	var err error

	fi := filter.GetConfig(ctx)
	ci := fs.GetConfig(ctx)
	// get source object
	srcObj, err = src.NewObject(ctx, srcFile)
	fs.Debugf(nil, "srcFile: %q, src : %v", srcFile, src)
	if errors.Is(err, fs.ErrorIsDir) {
		return fmt.Errorf("source can't be a directory: %w", err)
	} else if errors.Is(err, fs.ErrorObjectNotFound) {
		return fmt.Errorf("source not found: %w", err)
	} else if err != nil {
		return fmt.Errorf("unable to access source: %w", err)
	}
	fs.Debugf(nil, "Source archive file: %s/%s", src.Root(), srcFile)
	// Create destination directory
	err = dst.Mkdir(ctx, dstDir)
	if err != nil {
		return fmt.Errorf("unable to access destination: %w", err)
	}

	fs.Debugf(dst, "Destination for extracted files: %q", dstDir)
	// start accounting
	tr := accounting.Stats(ctx).NewTransfer(srcObj, nil)
	defer tr.Done(ctx, err)
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
	acc := tr.Account(ctx, in0)
	in, err := acc.WithReadAtSeeker()
	if err != nil {
		return err
	}
	// identify format
	format, _, err := archives.Identify(ctx, "", in)
	if err != nil {
		return fmt.Errorf("failed to open check file type: %w", err)
	}
	fs.Debugf(nil, "Extract %s/%s, format %s to %s", src.Root(), srcFile, strings.TrimPrefix(format.Extension(), "."), dst.Root())

	// check if extract is supported by format
	ex, isExtract := format.(archives.Extraction)
	if !isExtract {
		return fmt.Errorf("extraction for %s not supported", strings.TrimPrefix(format.Extension(), "."))
	}
	// extract files
	err = ex.Extract(ctx, in, func(ctx context.Context, f archives.FileInfo) error {
		remote, err := destPath(f.NameInArchive, dstDir)
		if err != nil {
			return err
		}
		// Skip the archive root entry ("./") which has no name of its own.
		if remote == "" {
			return nil
		}
		// check if file should be extracted
		if !fi.Include(remote, f.Size(), f.ModTime(), fs.Metadata{}) {
			return nil
		}
		// process directory
		if f.IsDir() {
			// directory
			fs.Debugf(nil, "mkdir %s", remote)
			// leave if --dry-run set
			if ci.DryRun {
				return nil
			}
			// create the directory
			return operations.Mkdir(ctx, dst, remote)
		}
		// process file
		fs.Debugf(nil, "Extract %s", remote)
		// leave if --dry-run set
		if ci.DryRun {
			filesExtracted++
			return nil
		}
		// open file
		fin, err := f.Open()
		if err != nil {
			return err
		}
		// extract the file to destination
		_, err = operations.Rcat(ctx, dst, remote, fin, f.ModTime(), nil)
		if err == nil {
			filesExtracted++
		}
		return err
	})

	fs.Infof(nil, "Total files extracted %d", filesExtracted)

	return err
}

// destPath maps an archive entry name onto its destination remote within
// dstDir, returning an error if the name is unsafe.
//
// Archive entry names are attacker controlled. A leading "./" is stripped:
// tar archives created with relative paths (e.g. "tar -czf archive.tar.gz .")
// use "./" prefixed entries and, without stripping, rclone would encode the
// "." as a full-width dot character creating a spurious directory.
//
// Any entry with a ".." path component is then rejected to prevent a path
// traversal ("Zip Slip") attack: path.Join collapses "..", so an entry such
// as "../escaped.txt" joined onto "dir" would resolve to "escaped.txt" and be
// written outside the selected destination directory. Both "/" and "\" are
// treated as separators when looking for ".." segments: archive names should
// use "/", but a crafted archive may use "\", which the local backend treats
// as a path separator on Windows.
//
// The returned remote is empty for the archive root entry ("./"), which the
// caller should skip.
func destPath(nameInArchive, dstDir string) (string, error) {
	remote := strings.TrimPrefix(nameInArchive, "./")
	isSeparator := func(r rune) bool { return r == '/' || r == '\\' }
	for _, segment := range strings.FieldsFunc(remote, isSeparator) {
		if segment == ".." {
			return "", fmt.Errorf("refusing to extract archive entry %q with a %q path component", nameInArchive, "..")
		}
	}
	if remote == "" {
		return "", nil
	}
	if dstDir != "" {
		remote = path.Join(dstDir, remote)
	}
	return remote, nil
}
