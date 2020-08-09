// Package makefiles builds a directory structure with the required
// number of files in of the required size.
package makefiles

import (
	cryptrand "crypto/rand"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"

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

	// Globals
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
}

var commandDefinition = &cobra.Command{
	Use:   "makefiles <dir>",
	Short: `Make a random file hierarchy in <dir>`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		outputDirectory := args[0]
		directoriesToCreate = numberOfFiles / averageFilesPerDirectory
		averageSize := (minFileSize + maxFileSize) / 2
		log.Printf("Creating %d files of average size %v in %d directories in %q.", numberOfFiles, averageSize, directoriesToCreate, outputDirectory)
		root := &dir{name: outputDirectory, depth: 1}
		for totalDirectories < directoriesToCreate {
			root.createDirectories()
		}
		dirs := root.list("", []string{})
		for i := 0; i < numberOfFiles; i++ {
			dir := dirs[rand.Intn(len(dirs))]
			writeFile(dir, fileName())
		}
		log.Printf("Done.")
	},
}

// fileName creates a unique random file or directory name
func fileName() (name string) {
	for {
		length := rand.Intn(maxFileNameLength-minFileNameLength) + minFileNameLength
		name = random.String(length)
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
		switch rand.Intn(4) {
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
func writeFile(dir, name string) {
	err := os.MkdirAll(dir, 0777)
	if err != nil {
		log.Fatalf("Failed to make directory %q: %v", dir, err)
	}
	path := filepath.Join(dir, name)
	fd, err := os.Create(path)
	if err != nil {
		log.Fatalf("Failed to open file %q: %v", path, err)
	}
	size := rand.Int63n(int64(maxFileSize-minFileSize)) + int64(minFileSize)
	_, err = io.CopyN(fd, cryptrand.Reader, size)
	if err != nil {
		log.Fatalf("Failed to write %v bytes to file %q: %v", size, path, err)
	}
	err = fd.Close()
	if err != nil {
		log.Fatalf("Failed to close file %q: %v", path, err)
	}
}
