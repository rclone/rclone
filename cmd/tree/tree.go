// Package tree provides the tree command.
package tree

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/a8m/tree"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/dirtree"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/terminal"
	"github.com/spf13/cobra"
)

var (
	opts        tree.Options
	outFileName string
	noReport    bool
	sort        string
	enc         = encoder.OS
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	// List
	flags.BoolVarP(cmdFlags, &opts.All, "all", "a", false, "All files are listed (list . files too)", "")
	flags.BoolVarP(cmdFlags, &opts.DirsOnly, "dirs-only", "d", false, "List directories only", "")
	flags.BoolVarP(cmdFlags, &opts.FullPath, "full-path", "", false, "Print the full path prefix for each file", "")
	//flags.BoolVarP(cmdFlags, &opts.IgnoreCase, "ignore-case", "", false, "Ignore case when pattern matching", "")
	flags.BoolVarP(cmdFlags, &noReport, "noreport", "", false, "Turn off file/directory count at end of tree listing", "")
	// flags.BoolVarP(cmdFlags, &opts.FollowLink, "follow", "l", false, "Follow symbolic links like directories","")
	flags.IntVarP(cmdFlags, &opts.DeepLevel, "level", "", 0, "Descend only level directories deep", "")
	// flags.StringVarP(cmdFlags, &opts.Pattern, "pattern", "P", "", "List only those files that match the pattern given")
	// flags.StringVarP(cmdFlags, &opts.IPattern, "exclude", "", "", "Do not list files that match the given pattern")
	flags.StringVarP(cmdFlags, &outFileName, "output", "o", "", "Output to file instead of stdout", "")
	// Files
	flags.BoolVarP(cmdFlags, &opts.ByteSize, "size", "s", false, "Print the size in bytes of each file.", "")
	flags.BoolVarP(cmdFlags, &opts.FileMode, "protections", "p", false, "Print the protections for each file.", "")
	// flags.BoolVarP(cmdFlags, &opts.ShowUid, "uid", "", false, "Displays file owner or UID number.")
	// flags.BoolVarP(cmdFlags, &opts.ShowGid, "gid", "", false, "Displays file group owner or GID number.")
	flags.BoolVarP(cmdFlags, &opts.Quotes, "quote", "Q", false, "Quote filenames with double quotes.", "")
	flags.BoolVarP(cmdFlags, &opts.LastMod, "modtime", "D", false, "Print the date of last modification.", "")
	// flags.BoolVarP(cmdFlags, &opts.Inodes, "inodes", "", false, "Print inode number of each file.")
	// flags.BoolVarP(cmdFlags, &opts.Device, "device", "", false, "Print device ID number to which each file belongs.")
	// Sort
	flags.BoolVarP(cmdFlags, &opts.NoSort, "unsorted", "U", false, "Leave files unsorted", "")
	flags.BoolVarP(cmdFlags, &opts.VerSort, "version", "", false, "Sort files alphanumerically by version", "")
	flags.BoolVarP(cmdFlags, &opts.ModSort, "sort-modtime", "t", false, "Sort files by last modification time", "")
	flags.BoolVarP(cmdFlags, &opts.CTimeSort, "sort-ctime", "", false, "Sort files by last status change time", "")
	flags.BoolVarP(cmdFlags, &opts.ReverSort, "sort-reverse", "r", false, "Reverse the order of the sort", "")
	flags.BoolVarP(cmdFlags, &opts.DirSort, "dirsfirst", "", false, "List directories before files (-U disables)", "")
	flags.StringVarP(cmdFlags, &sort, "sort", "", "", "Select sort: name,version,size,mtime,ctime", "")
	// Graphics
	flags.BoolVarP(cmdFlags, &opts.NoIndent, "noindent", "", false, "Don't print indentation lines", "")
}

var commandDefinition = &cobra.Command{
	Use:   "tree remote:path",
	Short: `List the contents of the remote in a tree like fashion.`,
	Long: `Lists the contents of a remote in a similar way to the unix tree command.

For example

    $ rclone tree remote:path
    /
    ├── file1
    ├── file2
    ├── file3
    └── subdir
        ├── file4
        └── file5

    1 directories, 5 files

You can use any of the filtering options with the tree command (e.g.
` + "`--include` and `--exclude`" + `.  You can also use ` + "`--fast-list`" + `.

The tree command has many options for controlling the listing which
are compatible with the tree command, for example you can include file
sizes with ` + "`--size`" + `.  Note that not all of them have
short options as they conflict with rclone's short options.

For a more interactive navigation of the remote see the
[ncdu](/commands/rclone_ncdu/) command.
`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.38",
		"groups":            "Filter,Listing",
	},
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		ci := fs.GetConfig(context.Background())
		var outFile io.Writer
		if outFileName != "" {
			var err error
			outFile, err = os.Create(outFileName)
			if err != nil {
				return fmt.Errorf("failed to create output file: %w", err)
			}
			opts.Colorize = false
		} else {
			terminal.Start()
			outFile = terminal.Out
			opts.Colorize = true
		}
		opts.VerSort = opts.VerSort || sort == "version"
		opts.ModSort = opts.ModSort || sort == "mtime"
		opts.CTimeSort = opts.CTimeSort || sort == "ctime"
		opts.NameSort = sort == "name"
		opts.SizeSort = sort == "size"
		opts.UnitSize = ci.HumanReadable
		if opts.DeepLevel == 0 {
			opts.DeepLevel = ci.MaxDepth
		}
		cmd.Run(false, false, command, func() error {
			return Tree(fsrc, outFile, &opts)
		})
		return nil
	},
}

// Tree lists fsrc to outFile using the Options passed in
func Tree(fsrc fs.Fs, outFile io.Writer, opts *tree.Options) error {
	dirs, err := walk.NewDirTree(context.Background(), fsrc, "", false, opts.DeepLevel)
	if err != nil {
		return err
	}
	opts.Fs = NewFs(dirs)
	opts.OutFile = outFile
	inf := tree.New("/")
	var nd, nf int
	if d, f := inf.Visit(opts); f != 0 {
		nd, nf = nd+d, nf+f
	}
	inf.Print(opts)
	// Print footer report
	if !noReport {
		footer := fmt.Sprintf("\n%d directories", nd)
		if !opts.DirsOnly {
			footer += fmt.Sprintf(", %d files", nf)
		}
		_, _ = fmt.Fprintln(outFile, footer)
	}
	return nil
}

// FileInfo maps an fs.DirEntry into an os.FileInfo
type FileInfo struct {
	entry fs.DirEntry
}

// Name is base name of the file
func (to *FileInfo) Name() string {
	return enc.FromStandardName(path.Base(to.entry.Remote()))
}

// Size in bytes for regular files; system-dependent for others
func (to *FileInfo) Size() int64 {
	return to.entry.Size()
}

// Mode is file mode bits
func (to *FileInfo) Mode() os.FileMode {
	if to.IsDir() {
		return os.FileMode(0777)
	}
	return os.FileMode(0666)
}

// ModTime is modification time
func (to *FileInfo) ModTime() time.Time {
	return to.entry.ModTime(context.Background())
}

// IsDir is abbreviation for Mode().IsDir()
func (to *FileInfo) IsDir() bool {
	_, ok := to.entry.(fs.Directory)
	return ok
}

// Sys is underlying data source (can return nil)
func (to *FileInfo) Sys() interface{} {
	return nil
}

// String returns the full path
func (to *FileInfo) String() string {
	return filepath.FromSlash(enc.FromStandardPath(to.entry.Remote()))
}

// Fs maps an fs.Fs into a tree.Fs
type Fs dirtree.DirTree

// NewFs creates a new tree
func NewFs(dirs dirtree.DirTree) Fs {
	return Fs(dirs)
}

// Stat returns info about the file
func (dirs Fs) Stat(filePath string) (fi os.FileInfo, err error) {
	defer log.Trace(nil, "filePath=%q", filePath)("fi=%+v, err=%v", &fi, &err)
	filePath = filepath.ToSlash(filePath)
	filePath = enc.ToStandardPath(filePath)
	filePath = strings.TrimLeft(filePath, "/")
	if filePath == "" {
		return &FileInfo{fs.NewDir("", time.Now())}, nil
	}
	_, entry := dirtree.DirTree(dirs).Find(filePath)
	if entry == nil {
		return nil, fmt.Errorf("couldn't find %q in directory cache", filePath)
	}
	return &FileInfo{entry}, nil
}

// ReadDir returns info about the directory and fills up the directory cache
func (dirs Fs) ReadDir(dir string) (names []string, err error) {
	defer log.Trace(nil, "dir=%s", dir)("names=%+v, err=%v", &names, &err)
	dir = filepath.ToSlash(dir)
	dir = enc.ToStandardPath(dir)
	dir = strings.TrimLeft(dir, "/")
	entries, ok := dirs[dir]
	if !ok {
		return nil, fmt.Errorf("couldn't find directory %q", dir)
	}
	for _, entry := range entries {
		names = append(names, enc.FromStandardName(path.Base(entry.Remote())))
	}
	return
}

// check interfaces
var (
	_ tree.Fs     = (*Fs)(nil)
	_ os.FileInfo = (*FileInfo)(nil)
)
