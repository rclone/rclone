//go:build ignore

// Read two files with lots of seeking to stress test the seek code
package main

import (
	"bytes"
	"flag"
	"io"
	"log"
	"math/rand"
	"os"
	"time"
)

var (
	// Flags
	iterations   = flag.Int("n", 1e6, "Iterations to try")
	maxBlockSize = flag.Int("b", 1024*1024, "Max block size to read")
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func randomSeekTest(size int64, in1, in2 *os.File, file1, file2 string) {
	start := rand.Int63n(size)
	blockSize := rand.Intn(*maxBlockSize)
	if int64(blockSize) > size-start {
		blockSize = int(size - start)
	}
	log.Printf("Reading %d from %d", blockSize, start)

	_, err := in1.Seek(start, io.SeekStart)
	if err != nil {
		log.Fatalf("Seek failed on %q: %v", file1, err)
	}
	_, err = in2.Seek(start, io.SeekStart)
	if err != nil {
		log.Fatalf("Seek failed on %q: %v", file2, err)
	}

	buf1 := make([]byte, blockSize)
	n1, err := io.ReadFull(in1, buf1)
	if err != nil {
		log.Fatalf("Read failed on %q: %v", file1, err)
	}

	buf2 := make([]byte, blockSize)
	n2, err := io.ReadFull(in2, buf2)
	if err != nil {
		log.Fatalf("Read failed on %q: %v", file2, err)
	}

	if n1 != n2 {
		log.Fatalf("Read different lengths %d (%q) != %d (%q)", n1, file1, n2, file2)
	}

	if !bytes.Equal(buf1, buf2) {
		log.Printf("Dumping different blocks")
		err = os.WriteFile("/tmp/z1", buf1, 0777)
		if err != nil {
			log.Fatalf("Failed to write /tmp/z1: %v", err)
		}
		err = os.WriteFile("/tmp/z2", buf2, 0777)
		if err != nil {
			log.Fatalf("Failed to write /tmp/z2: %v", err)
		}
		log.Fatalf("Read different contents - saved in /tmp/z1 and /tmp/z2")
	}
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 2 {
		log.Fatalf("Require 2 files as argument")
	}
	file1, file2 := args[0], args[1]
	in1, err := os.Open(file1)
	if err != nil {
		log.Fatalf("Couldn't open %q: %v", file1, err)
	}
	in2, err := os.Open(file2)
	if err != nil {
		log.Fatalf("Couldn't open %q: %v", file2, err)
	}

	fi1, err := in1.Stat()
	if err != nil {
		log.Fatalf("Couldn't stat %q: %v", file1, err)
	}
	fi2, err := in2.Stat()
	if err != nil {
		log.Fatalf("Couldn't stat %q: %v", file2, err)
	}

	if fi1.Size() != fi2.Size() {
		log.Fatalf("Files not the same size")
	}

	for i := 0; i < *iterations; i++ {
		randomSeekTest(fi1.Size(), in1, in2, file1, file2)
	}

	err = in1.Close()
	if err != nil {
		log.Fatalf("Error closing %q: %v", file1, err)
	}
	err = in2.Close()
	if err != nil {
		log.Fatalf("Error closing %q: %v", file2, err)
	}
}
