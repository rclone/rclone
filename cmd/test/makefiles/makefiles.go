// Package makefiles builds a directory structure with the required
// number of files in of the required size.
package makefiles

import (
	"io"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/test"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/lib/file"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/lib/readers"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	// Flags
	numberOfFiles            = 1000
	averageFilesPerDirectory = 10
	maxDepth                 = 10
	minFileSize              = fs.SizeSuffix(0)
	maxFileSize              = fs.SizeSuffix(100)
	minFileNameLength        = 4
	maxFileNameLength        = 12
	seed                     = int64(1)
	zero                     = false
	sparse                   = false
	ascii                    = false
	pattern                  = false
	chargen                  = false

	// Globals
	randSource          *rand.Rand
	source              io.Reader
	directoriesToCreate int
	totalDirectories    int
	fileNames           = map[string]struct{}{} // keep a note of which file name we've used already
)

func init() {
	test.Command.AddCommand(makefilesCmd)
	makefilesFlags := makefilesCmd.Flags()
	flags.IntVarP(makefilesFlags, &numberOfFiles, "files", "", numberOfFiles, "Number of files to create", "")
	flags.IntVarP(makefilesFlags, &averageFilesPerDirectory, "files-per-directory", "", averageFilesPerDirectory, "Average number of files per directory", "")
	flags.IntVarP(makefilesFlags, &maxDepth, "max-depth", "", maxDepth, "Maximum depth of directory hierarchy", "")
	flags.FVarP(makefilesFlags, &minFileSize, "min-file-size", "", "Minimum size of file to create", "")
	flags.FVarP(makefilesFlags, &maxFileSize, "max-file-size", "", "Maximum size of files to create", "")
	flags.IntVarP(makefilesFlags, &minFileNameLength, "min-name-length", "", minFileNameLength, "Minimum size of file names", "")
	flags.IntVarP(makefilesFlags, &maxFileNameLength, "max-name-length", "", maxFileNameLength, "Maximum size of file names", "")

	test.Command.AddCommand(makefileCmd)
	makefileFlags := makefileCmd.Flags()

	// Common flags to makefiles and makefile
	for _, f := range []*pflag.FlagSet{makefilesFlags, makefileFlags} {
		flags.Int64VarP(f, &seed, "seed", "", seed, "Seed for the random number generator (0 for random)", "")
		flags.BoolVarP(f, &zero, "zero", "", zero, "Fill files with ASCII 0x00", "")
		flags.BoolVarP(f, &sparse, "sparse", "", sparse, "Make the files sparse (appear to be filled with ASCII 0x00)", "")
		flags.BoolVarP(f, &ascii, "ascii", "", ascii, "Fill files with random ASCII printable bytes only", "")
		flags.BoolVarP(f, &pattern, "pattern", "", pattern, "Fill files with a periodic pattern", "")
		flags.BoolVarP(f, &chargen, "chargen", "", chargen, "Fill files with a ASCII chargen pattern", "")
	}
}

var makefilesCmd = &cobra.Command{
	Use:   "makefiles <dir>",
	Short: `Make a random file hierarchy in a directory`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.55",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		commonInit()
		outputDirectory := args[0]
		directoriesToCreate = numberOfFiles / averageFilesPerDirectory
		averageSize := (minFileSize + maxFileSize) / 2
		start := time.Now()
		fs.Logf(nil, "Creating %d files of average size %v in %d directories in %q.", numberOfFiles, averageSize, directoriesToCreate, outputDirectory)
		root := &dir{name: outputDirectory, depth: 1}
		for totalDirectories < directoriesToCreate {
			root.createDirectories()
		}
		dirs := root.list("", []string{})
		totalBytes := int64(0)
		for i := 0; i < numberOfFiles; i++ {
			dir := dirs[randSource.Intn(len(dirs))]
			size := int64(minFileSize)
			if maxFileSize > minFileSize {
				size += randSource.Int63n(int64(maxFileSize - minFileSize))
			}
			writeFile(dir, fileName(), size)
			totalBytes += size
		}
		dt := time.Since(start)
		fs.Logf(nil, "Written %vB in %v at %vB/s.", fs.SizeSuffix(totalBytes), dt.Round(time.Millisecond), fs.SizeSuffix((totalBytes*int64(time.Second))/int64(dt)))
	},
}

var makefileCmd = &cobra.Command{
	Use:   "makefile <size> [<file>]+ [flags]",
	Short: `Make files with random contents of the size given`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.59",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1e6, command, args)
		commonInit()
		var size fs.SizeSuffix
		err := size.Set(args[0])
		if err != nil {
			fs.Fatalf(nil, "Failed to parse size %q: %v", args[0], err)
		}
		start := time.Now()
		fs.Logf(nil, "Creating %d files of size %v.", len(args[1:]), size)
		totalBytes := int64(0)
		for _, filePath := range args[1:] {
			dir := filepath.Dir(filePath)
			name := filepath.Base(filePath)
			writeFile(dir, name, int64(size))
			totalBytes += int64(size)
		}
		dt := time.Since(start)
		fs.Logf(nil, "Written %vB in %v at %vB/s.", fs.SizeSuffix(totalBytes), dt.Round(time.Millisecond), fs.SizeSuffix((totalBytes*int64(time.Second))/int64(dt)))
	},
}

func bool2int(b bool) int {
	if b {
		return 1
	}
	return 0
}

// common initialisation for makefiles and makefile
func commonInit() {
	if seed == 0 {
		seed = time.Now().UnixNano()
		fs.Logf(nil, "Using random seed = %d", seed)
	}
	randSource = rand.New(rand.NewSource(seed))
	if bool2int(zero)+bool2int(sparse)+bool2int(ascii)+bool2int(pattern)+bool2int(chargen) > 1 {
		fs.Fatal(nil, "Can only supply one of --zero, --sparse, --ascii, --pattern or --chargen")
	}
	switch {
	case zero, sparse:
		source = zeroReader{}
	case ascii:
		source = asciiReader{}
	case pattern:
		source = readers.NewPatternReader(math.MaxInt64)
	case chargen:
		source = &chargenReader{}
	default:
		source = randSource
	}
	if minFileSize > maxFileSize {
		maxFileSize = minFileSize
	}
}

type zeroReader struct{}

// Read a chunk of zeroes
func (zeroReader) Read(p []byte) (n int, err error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

type asciiReader struct{}

// Read a chunk of printable ASCII characters
func (asciiReader) Read(p []byte) (n int, err error) {
	n, err = randSource.Read(p)
	for i := range p[:n] {
		p[i] = (p[i] % (0x7F - 0x20)) + 0x20
	}
	return n, err
}

type chargenReader struct {
	start   byte // offset from startChar to start line with
	written byte // chars in line so far
}

// Read a chunk of printable ASCII characters in chargen format
func (r *chargenReader) Read(p []byte) (n int, err error) {
	const (
		startChar    = 0x20 // ' '
		endChar      = 0x7E // '~' inclusive
		charsPerLine = 72
	)
	for i := range p {
		if r.written >= charsPerLine {
			r.start++
			if r.start > endChar-startChar {
				r.start = 0
			}
			p[i] = '\n'
			r.written = 0
		} else {
			c := r.start + r.written + startChar
			if c > endChar {
				c -= endChar - startChar + 1
			}
			p[i] = c
			r.written++
		}
	}
	return len(p), err
}

// fileName creates a unique random file or directory name
func fileName() (name string) {
	for {
		length := randSource.Intn(maxFileNameLength-minFileNameLength) + minFileNameLength
		name = random.StringFn(length, randSource)
		if _, found := fileNames[name]; !found {
			break
		}
	}
	fileNames[name] = struct{}{}
	return name
}

// dir is a directory in the directory hierarchy being built up
type dir struct {
	name     string
	depth    int
	children []*dir
	parent   *dir
}

// Create a random directory hierarchy under d
func (d *dir) createDirectories() {
	for totalDirectories < directoriesToCreate {
		newDir := &dir{
			name:   fileName(),
			depth:  d.depth + 1,
			parent: d,
		}
		d.children = append(d.children, newDir)
		totalDirectories++
		switch randSource.Intn(4) {
		case 0:
			if d.depth < maxDepth {
				newDir.createDirectories()
			}
		case 1:
			return
		}
	}
}

// list the directory hierarchy
func (d *dir) list(path string, output []string) []string {
	dirPath := filepath.Join(path, d.name)
	output = append(output, dirPath)
	for _, subDir := range d.children {
		output = subDir.list(dirPath, output)
	}
	return output
}

// writeFile writes a random file at dir/name
func writeFile(dir, name string, size int64) {
	err := file.MkdirAll(dir, 0777)
	if err != nil {
		fs.Fatalf(nil, "Failed to make directory %q: %v", dir, err)
	}
	path := filepath.Join(dir, name)
	fd, err := os.Create(path)
	if err != nil {
		fs.Fatalf(nil, "Failed to open file %q: %v", path, err)
	}
	if sparse {
		err = fd.Truncate(size)
	} else {
		_, err = io.CopyN(fd, source, size)
	}
	if err != nil {
		fs.Fatalf(nil, "Failed to write %v bytes to file %q: %v", size, path, err)
	}
	err = fd.Close()
	if err != nil {
		fs.Fatalf(nil, "Failed to close file %q: %v", path, err)
	}
	fs.Infof(path, "Written file size %v", fs.SizeSuffix(size))
}
