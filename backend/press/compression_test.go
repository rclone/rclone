package press

import (
	"io"
	"io/ioutil"
	"os"
	"bufio"
	"bytes"
	"strings"
	"testing"
//	"time"
	"crypto/md5"
)

const TestString = "The quick brown fox jumps over the lazy dog."

// Tests compression and decompression for a preset
func testCompressDecompress(t *testing.T, preset string, testString string) {
	// Create compression instance
	comp, err := NewCompressionPreset(preset)
	if err != nil {
		t.Fatal(err)
	}

	// Open files and hashers
	testFile := strings.NewReader(testString)
	testFileHasher := md5.New()
	if err != nil {
		t.Fatal(err)
	}
	compressedFile, err := ioutil.TempFile(os.TempDir(), "rclone_compression_test")
	if err != nil {
		t.Fatal(err)
	}
	outHasher := md5.New()

	// Compress file and hash it (size doesn't matter here)
	testFileReader, testFileWriter := io.Pipe()
	go func() {
		io.Copy(io.MultiWriter(testFileHasher, testFileWriter), testFile)
		testFileWriter.Close()
	}()
	comp.CompressFile(testFileReader, 0, compressedFile)
	testFileHash := testFileHasher.Sum(nil)

	// Get the size, seek to the beginning of the compressed file
	size, err := compressedFile.Seek(0, io.SeekEnd)
	if err != nil {
		t.Fatal(err)
	}
	_, err = compressedFile.Seek(0, io.SeekStart)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Compressed size: %d\n", size)

	// Decompress file into a hasher
	FileHandle, decompressedSize, err := comp.DecompressFile(compressedFile, size)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Decompressed size: %d\n", decompressedSize)
	bufr := bufio.NewReaderSize(FileHandle, 12345678)
	_, err = io.Copy(outHasher, bufr)
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	decompressedFileHash := outHasher.Sum(nil)

	// Clean up
	compressedFile.Close()
	os.Remove(compressedFile.Name())

	// Compare hashes
	t.Logf("Hash of original file: %x\n", testFileHash)
	t.Logf("Hash of recovered file: %x\n", decompressedFileHash)
	if !bytes.Equal(testFileHash, decompressedFileHash) {
		t.Fatal("Hashes do not match!")
	}
}

// Tests LZ4
func TestLZ4(t *testing.T) {
	testCompressDecompress(t, "lz4", TestString)
}

// Tests Snappy
func TestSnappy(t *testing.T) {
	testCompressDecompress(t, "snappy", TestString)
}

// Tests GZIP
func TestGzip(t *testing.T) {
	testCompressDecompress(t, "gzip-min", TestString)
}

// Tests XZ
func TestXZ(t *testing.T) {
	testCompressDecompress(t, "xz-min", TestString)
}