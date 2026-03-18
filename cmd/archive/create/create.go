//go:build !plan9

// Package create implements 'rclone archive create'.
package create

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/mholt/archives"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/archive"
	"github.com/rclone/rclone/cmd/archive/files"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/walk"
	"github.com/spf13/cobra"
)

var (
	fullPath = false
	prefix   = ""
	format   = ""
)

func init() {
	flagSet := Command.Flags()
	flags.BoolVarP(flagSet, &fullPath, "full-path", "", fullPath, "Set prefix for files in archive to source path", "")
	flags.StringVarP(flagSet, &prefix, "prefix", "", prefix, "Set prefix for files in archive to entered value or source path", "")
	flags.StringVarP(flagSet, &format, "format", "", format, "Create the archive with format or guess from extension.", "")
	archive.Command.AddCommand(Command)
}

// Command - create
var Command = &cobra.Command{
	Use:   "create [flags] <source> [<destination>]",
	Short: `Archive source file(s) to destination.`,
	// Warning! "!" will be replaced by backticks below
	Long: strings.ReplaceAll(`
Creates an archive from the files in source:path and saves the archive to
dest:path. If dest:path is missing, it will write to the console.

The valid formats for the !--format! flag are listed below. If
!--format! is not set rclone will guess it from the extension of dest:path.

| Format | Extensions |
|:-------|:-----------|
| zip    | .zip       |
| tar    | .tar       |
| tar.gz | .tar.gz, .tgz, .taz |
| tar.bz2| .tar.bz2, .tb2, .tbz, .tbz2, .tz2 |
| tar.lz | .tar.lz    |
| tar.lz4| .tar.lz4   |
| tar.xz | .tar.xz, .txz |
| tar.zst| .tar.zst, .tzst |
| tar.br | .tar.br    |
| tar.sz | .tar.sz    |
| tar.mz | .tar.mz    |

The !--prefix! and !--full-path! flags control the prefix for the files
in the archive.

If the flag !--full-path! is set then the files will have the full source
path as the prefix.

If the flag !--prefix=<value>! is set then the files will have
!<value>! as prefix. It's possible to create invalid file names with
!--prefix=<value>! so use with caution. Flag !--prefix! has
priority over !--full-path!.

Given a directory !/sourcedir! with the following:

    file1.txt
    dir1/file2.txt

Running the command !rclone archive create /sourcedir /dest.tar.gz!
will make an archive with the contents:

    file1.txt
    dir1/
    dir1/file2.txt

Running the command !rclone archive create --full-path /sourcedir /dest.tar.gz!
will make an archive with the contents:

    sourcedir/file1.txt
    sourcedir/dir1/
    sourcedir/dir1/file2.txt

Running the command !rclone archive create --prefix=my_new_path /sourcedir /dest.tar.gz!
will make an archive with the contents:

    my_new_path/file1.txt
    my_new_path/dir1/
    my_new_path/dir1/file2.txt
`, "!", "`"),
	Annotations: map[string]string{
		"versionIntroduced": "v1.72",
	},
	RunE: func(command *cobra.Command, args []string) error {
		var src, dst fs.Fs
		var dstFile string
		if len(args) == 1 { // source only, archive to stdout
			src = cmd.NewFsSrc(args)
		} else if len(args) == 2 {
			src = cmd.NewFsSrc(args)
			dst, dstFile = cmd.NewFsDstFile(args[1:2])
		} else {
			cmd.CheckArgs(1, 2, command, args)
		}
		cmd.Run(false, false, command, func() error {
			fmt.Printf("dst=%v, dstFile=%q, src=%v, format=%q, prefix=%q\n", dst, dstFile, src, format, prefix)
			if prefix != "" {
				return ArchiveCreate(context.Background(), dst, dstFile, src, format, prefix)
			} else if fullPath {
				return ArchiveCreate(context.Background(), dst, dstFile, src, format, src.Root())
			}
			return ArchiveCreate(context.Background(), dst, dstFile, src, format, "")
		})
		return nil
	},
}

// Globals
var (
	archiveFormats = map[string]archives.CompressedArchive{
		"zip": archives.CompressedArchive{
			Archival: archives.Zip{ContinueOnError: true},
		},
		"tar": archives.CompressedArchive{
			Archival: archives.Tar{ContinueOnError: true},
		},
		"tar.gz": archives.CompressedArchive{
			Compression: archives.Gz{},
			Archival:    archives.Tar{ContinueOnError: true},
		},
		"tar.bz2": archives.CompressedArchive{
			Compression: archives.Bz2{},
			Archival:    archives.Tar{ContinueOnError: true},
		},
		"tar.lz": archives.CompressedArchive{
			Compression: archives.Lzip{},
			Archival:    archives.Tar{ContinueOnError: true},
		},
		"tar.lz4": archives.CompressedArchive{
			Compression: archives.Lz4{},
			Archival:    archives.Tar{ContinueOnError: true},
		},
		"tar.xz": archives.CompressedArchive{
			Compression: archives.Xz{},
			Archival:    archives.Tar{ContinueOnError: true},
		},
		"tar.zst": archives.CompressedArchive{
			Compression: archives.Zstd{},
			Archival:    archives.Tar{ContinueOnError: true},
		},
		"tar.br": archives.CompressedArchive{
			Compression: archives.Brotli{},
			Archival:    archives.Tar{ContinueOnError: true},
		},
		"tar.sz": archives.CompressedArchive{
			Compression: archives.Sz{},
			Archival:    archives.Tar{ContinueOnError: true},
		},
		"tar.mz": archives.CompressedArchive{
			Compression: archives.MinLZ{},
			Archival:    archives.Tar{ContinueOnError: true},
		},
	}
	archiveExtensions = map[string]string{
		// zip
		"*.zip": "zip",
		// tar
		"*.tar": "tar",
		// tar.gz
		"*.tar.gz": "tar.gz",
		"*.tgz":    "tar.gz",
		"*.taz":    "tar.gz",
		// tar.bz2
		"*.tar.bz2": "tar.bz2",
		"*.tb2":     "tar.bz2",
		"*.tbz":     "tar.bz2",
		"*.tbz2":    "tar.bz2",
		"*.tz2":     "tar.bz2",
		// tar.lz
		"*.tar.lz": "tar.lz",
		// tar.lz4
		"*.tar.lz4": "tar.lz4",
		// tar.xz
		"*.tar.xz": "tar.xz",
		"*.txz":    "tar.xz",
		// tar.zst
		"*.tar.zst": "tar.zst",
		"*.tzst":    "tar.zst",
		// tar.br
		"*.tar.br": "tar.br",
		// tar.sz
		"*.tar.sz": "tar.sz",
		// tar.mz
		"*.tar.mz": "tar.mz",
	}
)

// sorted FileInfo list

type archivesFileInfoList []archives.FileInfo

func (a archivesFileInfoList) Len() int {
	return len(a)
}

func (a archivesFileInfoList) Less(i, j int) bool {
	if a[i].FileInfo.IsDir() == a[j].FileInfo.IsDir() {
		// both are same type, order by name
		return strings.Compare(a[i].NameInArchive, a[j].NameInArchive) < 0
	} else if a[i].FileInfo.IsDir() {
		return strings.Compare(strings.TrimSuffix(a[i].NameInArchive, "/"), path.Dir(a[j].NameInArchive)) < 0
	}
	return strings.Compare(path.Dir(a[i].NameInArchive), strings.TrimSuffix(a[j].NameInArchive, "/")) < 0
}

func (a archivesFileInfoList) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func getCompressor(format string, filename string) (archives.CompressedArchive, error) {
	var compressor archives.CompressedArchive
	var found bool
	// make filename lowercase for checks
	filename = strings.ToLower(filename)

	if format == "" {
		// format flag not set, get format from the file extension
		for pattern, formatName := range archiveExtensions {
			ok, err := path.Match(pattern, filename)
			if err != nil {
				// error in pattern
				return archives.CompressedArchive{}, fmt.Errorf("invalid extension pattern '%s'", pattern)
			} else if ok {
				// pattern matches filename, get compressor
				compressor, found = archiveFormats[formatName]
				break
			}
		}
	} else {
		// format flag set, look for it
		compressor, found = archiveFormats[format]
	}

	if found {
		return compressor, nil
	} else if format == "" {
		return archives.CompressedArchive{}, fmt.Errorf("format not set and can't be guessed from extension")
	}
	return archives.CompressedArchive{}, fmt.Errorf("invalid format '%s'", format)
}

// CheckValidDestination - takes (dst, dstFile) and checks it is valid
func CheckValidDestination(ctx context.Context, dst fs.Fs, dstFile string) error {
	var err error

	// check if dst + dstFile is a file
	_, err = dst.NewObject(ctx, dstFile)
	if err == nil {
		// (dst, dstFile) is a valid file we can overwrite
		return nil
	} else if errors.Is(err, fs.ErrorIsDir) {
		// dst is a directory
		return fmt.Errorf("destination must not be a directory: %w", err)
	} else if !errors.Is(err, fs.ErrorObjectNotFound) {
		// dst is a directory (we need a filename) or some other error happened
		// not good, leave
		return fmt.Errorf("error reading destination: %w", err)
	}

	// if we are here dst points to a non existent path
	return nil
}

func loadMetadata(ctx context.Context, o fs.DirEntry) fs.Metadata {
	meta, err := fs.GetMetadata(ctx, o)
	if err != nil {
		meta = make(fs.Metadata, 0)
	}
	return meta
}

// ArchiveCreate - compresses/archive source to destination
func ArchiveCreate(ctx context.Context, dst fs.Fs, dstFile string, src fs.Fs, format string, prefix string) error {
	var err error
	var list archivesFileInfoList
	var compArchive archives.CompressedArchive
	var totalLength int64

	// check id dst is valid
	err = CheckValidDestination(ctx, dst, dstFile)
	if err != nil {
		return err
	}

	ci := fs.GetConfig(ctx)
	fi := filter.GetConfig(ctx)
	// get archive format
	compArchive, err = getCompressor(format, dstFile)
	if err != nil {
		return err
	}
	// get source files
	err = walk.ListR(ctx, src, "", false, ci.MaxDepth, walk.ListAll, func(entries fs.DirEntries) error {
		// get directories
		entries.ForDir(func(o fs.Directory) {
			var metadata fs.Metadata
			if ci.Metadata {
				metadata = loadMetadata(ctx, o)
			}
			if fi.Include(o.Remote(), o.Size(), o.ModTime(ctx), metadata) {
				info := files.NewArchiveFileInfo(ctx, o, prefix, metadata)
				list = append(list, info)
			}
		})
		// get files
		entries.ForObject(func(o fs.Object) {
			var metadata fs.Metadata
			if ci.Metadata {
				metadata = loadMetadata(ctx, o)
			}
			if fi.Include(o.Remote(), o.Size(), o.ModTime(ctx), metadata) {
				info := files.NewArchiveFileInfo(ctx, o, prefix, metadata)
				list = append(list, info)
				totalLength += o.Size()
			}
		})
		return nil
	})
	if err != nil {
		return err
	} else if list.Len() == 0 {
		return fmt.Errorf("no files found in source")
	}
	sort.Stable(list)
	// create archive
	if ci.DryRun {
		// write nowhere
		counter := files.NewCountWriter(nil)
		err = compArchive.Archive(ctx, counter, list)
		// log totals
		fs.Infof(nil, "Total files added %d", list.Len())
		fs.Infof(nil, "Total bytes read %d", totalLength)
		fs.Infof(nil, "Compressed file size %d", counter.Count())

		return err
	} else if dst == nil {
		// write to stdout
		counter := files.NewCountWriter(os.Stdout)
		err = compArchive.Archive(ctx, counter, list)
		// log totals
		fs.Infof(nil, "Total files added %d", list.Len())
		fs.Infof(nil, "Total bytes read %d", totalLength)
		fs.Infof(nil, "Compressed file size %d", counter.Count())

		return err
	}
	// write to remote
	pipeReader, pipeWriter := io.Pipe()
	// write to pipewriter in background
	counter := files.NewCountWriter(pipeWriter)
	go func() {
		err := compArchive.Archive(ctx, counter, list)
		pipeWriter.CloseWithError(err)
	}()
	// rcat to remote from pipereader
	_, err = operations.Rcat(ctx, dst, dstFile, pipeReader, time.Now(), nil)
	// log totals
	fs.Infof(nil, "Total files added %d", list.Len())
	fs.Infof(nil, "Total bytes read %d", totalLength)
	fs.Infof(nil, "Compressed file size %d", counter.Count())

	return err
}
