//go:build ignore

// Read lots files with lots of simultaneous seeking to stress test the seek code
package main

import (
	"flag"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

var (
	// Flags
	iterations   = flag.Int("n", 1e6, "Iterations to try")
	maxBlockSize = flag.Int("b", 1024*1024, "Max block size to read")
	simultaneous = flag.Int("transfers", 16, "Number of simultaneous files to open")
	seeksPerFile = flag.Int("seeks", 8, "Seeks per file")
	mask         = flag.Int64("mask", 0, "mask for seek, e.g. 0x7fff")
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func seekTest(n int, file string) {
	in, err := os.Open(file)
	if err != nil {
		log.Fatalf("Couldn't open %q: %v", file, err)
	}
	fi, err := in.Stat()
	if err != nil {
		log.Fatalf("Couldn't stat %q: %v", file, err)
	}
	size := fi.Size()

	// FIXME make sure we try start and end

	maxBlockSize := *maxBlockSize
	if int64(maxBlockSize) > size {
		maxBlockSize = int(size)
	}
	for i := 0; i < n; i++ {
		start := rand.Int63n(size)
		if *mask != 0 {
			start &^= *mask
		}
		blockSize := rand.Intn(maxBlockSize)
		beyondEnd := false
		switch rand.Intn(10) {
		case 0:
			start = 0
		case 1:
			start = size - int64(blockSize)
		case 2:
			// seek beyond the end
			start = size + int64(blockSize)
			beyondEnd = true
		default:
		}
		if !beyondEnd && int64(blockSize) > size-start {
			blockSize = int(size - start)
		}
		log.Printf("%s: Reading %d from %d", file, blockSize, start)

		_, err = in.Seek(start, io.SeekStart)
		if err != nil {
			log.Fatalf("Seek failed on %q: %v", file, err)
		}

		buf := make([]byte, blockSize)
		n, err := io.ReadFull(in, buf)
		if beyondEnd && err == io.EOF {
			// OK
		} else if err != nil {
			log.Fatalf("Read failed on %q: %v (%d)", file, err, n)
		}
	}

	err = in.Close()
	if err != nil {
		log.Fatalf("Error closing %q: %v", file, err)
	}
}

// Find all the files in dir
func findFiles(dir string) (files []string) {
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.Mode().IsRegular() && info.Size() > 0 {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		log.Fatalf("Require a directory as argument")
	}
	dir := args[0]
	files := findFiles(dir)
	jobs := make(chan string, *simultaneous)
	var wg sync.WaitGroup
	wg.Add(*simultaneous)
	for i := 0; i < *simultaneous; i++ {
		go func() {
			defer wg.Done()
			for file := range jobs {
				seekTest(*seeksPerFile, file)
			}
		}()
	}
	for i := 0; i < *iterations; i++ {
		i := rand.Intn(len(files))
		jobs <- files[i]
		//jobs <- files[i]
	}
	close(jobs)
	wg.Wait()
}
