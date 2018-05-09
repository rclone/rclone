// +build ignore

// Build a directory structure with the required number of files in
//
// Run with go run make_test_files.go [flag] <directory>
package main

import (
	cryptrand "crypto/rand"
	"flag"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
)

var (
	// Flags
	numberOfFiles            = flag.Int("n", 1000, "Number of files to create")
	averageFilesPerDirectory = flag.Int("files-per-directory", 10, "Average number of files per directory")
	maxDepth                 = flag.Int("max-depth", 10, "Maximum depth of directory heirachy")
	minFileSize              = flag.Int64("min-size", 0, "Minimum size of file to create")
	maxFileSize              = flag.Int64("max-size", 100, "Maximum size of files to create")
	minFileNameLength        = flag.Int("min-name-length", 4, "Minimum size of file to create")
	maxFileNameLength        = flag.Int("max-name-length", 12, "Maximum size of files to create")

	directoriesToCreate int
	totalDirectories    int
	fileNames           = map[string]struct{}{} // keep a note of which file name we've used already
)

// randomString create a random string for test purposes
func randomString(n int) string {
	const (
		vowel     = "aeiou"
		consonant = "bcdfghjklmnpqrstvwxyz"
		digit     = "0123456789"
	)
	pattern := []string{consonant, vowel, consonant, vowel, consonant, vowel, consonant, digit}
	out := make([]byte, n)
	p := 0
	for i := range out {
		source := pattern[p]
		p = (p + 1) % len(pattern)
		out[i] = source[rand.Intn(len(source))]
	}
	return string(out)
}

// fileName creates a unique random file or directory name
func fileName() (name string) {
	for {
		length := rand.Intn(*maxFileNameLength-*minFileNameLength) + *minFileNameLength
		name = randomString(length)
		if _, found := fileNames[name]; !found {
			break
		}
	}
	fileNames[name] = struct{}{}
	return name
}

// dir is a directory in the directory heirachy being built up
type dir struct {
	name     string
	depth    int
	children []*dir
	parent   *dir
}

// Create a random directory heirachy under d
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
			if d.depth < *maxDepth {
				newDir.createDirectories()
			}
		case 1:
			return
		}
	}
	return
}

// list the directory heirachy
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
	size := rand.Int63n(*maxFileSize-*minFileSize) + *minFileSize
	_, err = io.CopyN(fd, cryptrand.Reader, size)
	if err != nil {
		log.Fatalf("Failed to write %v bytes to file %q: %v", size, path, err)
	}
	err = fd.Close()
	if err != nil {
		log.Fatalf("Failed to close file %q: %v", path, err)
	}
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		log.Fatalf("Require 1 directory argument")
	}
	outputDirectory := args[0]
	log.Printf("Output dir %q", outputDirectory)

	directoriesToCreate = *numberOfFiles / *averageFilesPerDirectory
	log.Printf("directoriesToCreate %v", directoriesToCreate)
	root := &dir{name: outputDirectory, depth: 1}
	for totalDirectories < directoriesToCreate {
		root.createDirectories()
	}
	dirs := root.list("", []string{})
	for i := 0; i < *numberOfFiles; i++ {
		dir := dirs[rand.Intn(len(dirs))]
		writeFile(dir, fileName())
	}
}
