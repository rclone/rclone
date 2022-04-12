// Package makefiles builds a directory structure with the required
// number of files in of the required size.
package makefiles

import (
	"io"
	"log"
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

	// Globals
	randSource          *rand.Rand
	directoriesToCreate int
	totalDirectories    int
	fileNames           = map[string]struct{}{} // keep a note of which file name we've used already
)

func init() {
	test.Command.AddCommand(makefilesCmd)
	makefilesFlags := makefilesCmd.Flags()
	flags.IntVarP(makefilesFlags, &numberOfFiles, "files", "", numberOfFiles, "Number of files to create")
	flags.IntVarP(makefilesFlags, &averageFilesPerDirectory, "files-per-directory", "", averageFilesPerDirectory, "Average number of files per directory")
	flags.IntVarP(makefilesFlags, &maxDepth, "max-depth", "", maxDepth, "Maximum depth of directory hierarchy")
	flags.FVarP(makefilesFlags, &minFileSize, "min-file-size", "", "Minimum size of file to create")
	flags.FVarP(makefilesFlags, &maxFileSize, "max-file-size", "", "Maximum size of files to create")
	flags.IntVarP(makefilesFlags, &minFileNameLength, "min-name-length", "", minFileNameLength, "Minimum size of file names")
	flags.IntVarP(makefilesFlags, &maxFileNameLength, "max-name-length", "", maxFileNameLength, "Maximum size of file names")

	test.Command.AddCommand(makefileCmd)
	makefileFlags := makefileCmd.Flags()

	// Common flags to makefiles and makefile
	for _, f := range []*pflag.FlagSet{makefilesFlags, makefileFlags} {
		flags.Int64VarP(f, &seed, "seed", "", seed, "Seed for the random number generator (0 for random)")
	}
}

// common initialisation for makefiles and makefile
func commonInit() {
	if seed == 0 {
		seed = time.Now().UnixNano()
		fs.Logf(nil, "Using random seed = %d", seed)
	}
	randSource = rand.New(rand.NewSource(seed))
}

var makefilesCmd = &cobra.Command{
	Use:   "makefiles <dir>",
	Short: `Make a random file hierarchy in a directory`,
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
			size := randSource.Int63n(int64(maxFileSize-minFileSize)) + int64(minFileSize)
			writeFile(dir, fileName(), size)
			totalBytes += size
		}
		dt := time.Since(start)
		fs.Logf(nil, "Written %vB in %v at %vB/s.", fs.SizeSuffix(totalBytes), dt.Round(time.Millisecond), fs.SizeSuffix((totalBytes*int64(time.Second))/int64(dt)))
	},
}

var makefileCmd = &cobra.Command{
	Use:   "makefile <size> [<file>]+",
	Short: `Make files with random contents of the size given`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1e6, command, args)
		commonInit()
		var size fs.SizeSuffix
		err := size.Set(args[0])
		if err != nil {
			log.Fatalf("Failed to parse size %q: %v", args[0], err)
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

// fileName creates a unique random file or directory name
func fileName() (name string) {
	for {
		length := randSource.Intn(maxFileNameLength-minFileNameLength) + minFileNameLength
		name = random.StringFn(length, randSource.Intn)
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
	return
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
		log.Fatalf("Failed to make directory %q: %v", dir, err)
	}
	path := filepath.Join(dir, name)
	fd, err := os.Create(path)
	if err != nil {
		log.Fatalf("Failed to open file %q: %v", path, err)
	}
	_, err = io.CopyN(fd, randSource, size)
	if err != nil {
		log.Fatalf("Failed to write %v bytes to file %q: %v", size, path, err)
	}
	err = fd.Close()
	if err != nil {
		log.Fatalf("Failed to close file %q: %v", path, err)
	}
	fs.Infof(path, "Written file size %v", fs.SizeSuffix(size))
}
