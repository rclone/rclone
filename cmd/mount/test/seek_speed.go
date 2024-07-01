//go:build ignore

// Read blocks out of a single file to time the seeking code
package main

import (
	"flag"
	"io"
	"log"
	"math/rand"
	"os"
	"time"
)

var (
	// Flags
	iterations   = flag.Int("n", 25, "Iterations to try")
	maxBlockSize = flag.Int("b", 1024*1024, "Max block size to read")
	randSeed     = flag.Int64("seed", 1, "Seed for the random number generator")
)

func randomSeekTest(size int64, in *os.File, name string) {
	startTime := time.Now()
	start := rand.Int63n(size)
	blockSize := rand.Intn(*maxBlockSize)
	if int64(blockSize) > size-start {
		blockSize = int(size - start)
	}

	_, err := in.Seek(start, io.SeekStart)
	if err != nil {
		log.Fatalf("Seek failed on %q: %v", name, err)
	}

	buf := make([]byte, blockSize)
	_, err = io.ReadFull(in, buf)
	if err != nil {
		log.Fatalf("Read failed on %q: %v", name, err)
	}

	log.Printf("Reading %d from %d took %v ", blockSize, start, time.Since(startTime))
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		log.Fatalf("Require 1 file as argument")
	}
	rand.Seed(*randSeed)

	name := args[0]
	openStart := time.Now()
	in, err := os.Open(name)
	if err != nil {
		log.Fatalf("Couldn't open %q: %v", name, err)
	}
	log.Printf("File Open took %v", time.Since(openStart))

	fi, err := in.Stat()
	if err != nil {
		log.Fatalf("Couldn't stat %q: %v", name, err)
	}

	start := time.Now()
	for i := 0; i < *iterations; i++ {
		randomSeekTest(fi.Size(), in, name)
	}
	dt := time.Since(start)
	log.Printf("That took %v for %d iterations, %v per iteration", dt, *iterations, dt/time.Duration(*iterations))

	err = in.Close()
	if err != nil {
		log.Fatalf("Error closing %q: %v", name, err)
	}
}
