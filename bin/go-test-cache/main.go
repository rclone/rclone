// This code was copied from:
// https://github.com/fastly/cli/blob/main/scripts/go-test-cache/main.go
// which in turn is based on the following script and was generated using AI.
// https://github.com/airplanedev/blog-examples/blob/main/go-test-caching/update_file_timestamps.py?ref=airplane.ghost.io
//
// REFERENCE ARTICLE:
// https://web.archive.org/web/20240308061717/https://www.airplane.dev/blog/caching-golang-tests-in-ci
//
// It updates the mtime of the files to a mtime dervived from the sha1 hash of their contents.
package main

import (
	"crypto/sha1"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	bufSize    = 65536
	baseDate   = 1684178360
	timeFormat = "2006-01-02 15:04:05"
)

func main() {
	repoRoot := "."
	allDirs := make([]string, 0)

	err := filepath.Walk(repoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			dirPath := filepath.Join(repoRoot, path)
			relPath, _ := filepath.Rel(repoRoot, dirPath)

			if strings.HasPrefix(relPath, ".") {
				return nil
			}

			allDirs = append(allDirs, dirPath)
		} else {
			filePath := filepath.Join(repoRoot, path)
			relPath, _ := filepath.Rel(repoRoot, filePath)

			if strings.HasPrefix(relPath, ".") {
				return nil
			}

			sha1Hash, err := getFileSHA1(filePath)
			if err != nil {
				return err
			}

			modTime := getModifiedTime(sha1Hash)

			log.Printf("Setting modified time of file %s to %s\n", relPath, modTime.Format(timeFormat))
			err = os.Chtimes(filePath, modTime, modTime)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		log.Fatal("Error:", err)
	}

	sort.Slice(allDirs, func(i, j int) bool {
		return len(allDirs[i]) > len(allDirs[j]) || (len(allDirs[i]) == len(allDirs[j]) && allDirs[i] < allDirs[j])
	})

	for _, dirPath := range allDirs {
		relPath, _ := filepath.Rel(repoRoot, dirPath)

		log.Printf("Setting modified time of directory %s to %s\n", relPath, time.Unix(baseDate, 0).Format(timeFormat))
		err := os.Chtimes(dirPath, time.Unix(baseDate, 0), time.Unix(baseDate, 0))
		if err != nil {
			log.Fatal("Error:", err)
		}
	}

	log.Println("Done")
}

func getFileSHA1(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// G401: Use of weak cryptographic primitive
	// Disabling as the hash is used not for security reasons.
	// The hash is used as a cache key to improve test run times.
	// #nosec
	// nosemgrep: go.lang.security.audit.crypto.use_of_weak_crypto.use-of-sha1
	hash := sha1.New()
	if _, err := io.CopyBuffer(hash, file, make([]byte, bufSize)); err != nil {
		return "", err
	}

	return string(hash.Sum(nil)), nil
}

func getModifiedTime(sha1Hash string) time.Time {
	hashBytes := []byte(sha1Hash)
	lastFiveBytes := hashBytes[:5]
	lastFiveValue := int64(0)

	for _, b := range lastFiveBytes {
		lastFiveValue = (lastFiveValue << 8) + int64(b)
	}

	modTime := baseDate - (lastFiveValue % 10000)
	return time.Unix(modTime, 0)
}
