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
	"github.com/rclone/rclone/lib/random"
	"github.com/spf13/cobra"
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
	test.Command.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.IntVarP(cmdFlags, &numberOfFiles, "files", "", numberOfFiles, "Number of files to create")
	flags.IntVarP(cmdFlags, &averageFilesPerDirectory, "files-per-directory", "", averageFilesPerDirectory, "Average number of files per directory")
	flags.IntVarP(cmdFlags, &maxDepth, "max-depth", "", maxDepth, "Maximum depth of directory hierarchy")
	flags.FVarP(cmdFlags, &minFileSize, "min-file-size", "", "Minimum size of file to create")
	flags.FVarP(cmdFlags, &maxFileSize, "max-file-size", "", "Maximum size of files to create")
	flags.IntVarP(cmdFlags, &minFileNameLength, "min-name-length", "", minFileNameLength, "Minimum size of file names")
	flags.IntVarP(cmdFlags, &maxFileNameLength, "max-name-length", "", maxFileNameLength, "Maximum size of file names")
	flags.Int64VarP(cmdFlags, &seed, "seed", "", seed, "Seed for the random number generator (0 for random)")
}

var commandDefinition = &cobra.Command{
	Use:   "makefiles <dir>",
	Short: `Make a random file hierarchy in <dir>`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		if seed == 0 {
			seed = time.Now().UnixNano()
			fs.Logf(nil, "Using random seed = %d", seed)
		}
		randSource = rand.New(rand.NewSource(seed))
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
			totalBytes += writeFile(dir, fileName())
		}
		dt := time.Since(start)
		fs.Logf(nil, "Written %viB in %v at %viB/s.", fs.SizeSuffix(totalBytes), dt.Round(time.Millisecond), fs.SizeSuffix((totalBytes*int64(time.Second))/int64(dt)))
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
func writeFile(dir, name string) int64 {
	err := os.MkdirAll(dir, 0777)
	if err != nil {
		log.Fatalf("Failed to make directory %q: %v", dir, err)
	}
	path := filepath.Join(dir, name)
	fd, err := os.Create(path)
	if err != nil {
		log.Fatalf("Failed to open file %q: %v", path, err)
	}
	size := randSource.Int63n(int64(maxFileSize-minFileSize)) + int64(minFileSize)
	_, err = io.CopyN(fd, randSource, size)
	if err != nil {
		log.Fatalf("Failed to write %v bytes to file %q: %v", size, path, err)
	}
	err = fd.Close()
	if err != nil {
		log.Fatalf("Failed to close file %q: %v", path, err)
	}
	fs.Infof(path, "Written file size %v", fs.SizeSuffix(size))
	return size
}
