package press

import (
	"io"
	"io/ioutil"
	"os"
	"bufio"
	"bytes"
	"testing"
//	"time"
	"crypto/md5"
)

const testFileName = "test.vdi"
const outFileName = "/tmp/compression_compressed.gzip"
const outFileName2 = "/dev/null"

const Preset = "lz4"

func TestCompressDecompress(t *testing.T) {
	// Create compression instance
	comp, err := NewCompressionPreset(Preset)
	if err != nil {
		t.Fatal(err)
	}

	// Open files and hashers
	testFile, err := os.Open(testFileName)
	testFileHasher := md5.New()
	if err != nil {
		t.Fatal(err)
	}
	compressedFile, err := ioutil.TempFile(os.TempDir(), testFileName)
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
	testFile.Close()
	compressedFile.Close()

	// Compare hashes
	t.Logf("Hash of original file: %x\n", testFileHash)
	t.Logf("Hash of recovered file: %x\n", decompressedFileHash)
	if !bytes.Equal(testFileHash, decompressedFileHash) {
		t.Fatal("Hashes do not match!")
	}
}
/*
func TestSeek(t *testing.T) {
	comp, err := NewCompressionPreset(Preset)
	if err != nil {
		t.Fatal(err)
	}
	inFileInfo, err := os.Stat(testFileName+comp.GetFileExtension())
	if err != nil {
		t.Fatal(err)
	}
	inFile, err := os.Open(testFileName+comp.GetFileExtension())
	if err != nil {
		t.Fatal(err)
	}
	outFil, err := os.Create(outFileName2)
	if err != nil {
		t.Fatal(err)
	}
	FileHandle, decompressedSize, err := comp.DecompressFile(inFile, inFileInfo.Size())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Decompressed size: %d\n", decompressedSize)
	for {
		FileHandle.Seek(12345678, io.SeekCurrent) // 93323248
		_, err := io.CopyN(outFil, FileHandle, 16)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	inFile.Close()
	outFil.Close()
}

func TestFileCompressionInfo(t *testing.T) {
	comp, err := NewCompressionPreset(Preset)
	if err != nil {
		t.Fatal(err)
	}
	inFile, err := os.Open(testFileName)
	if err != nil {
		t.Fatal(err)
	}
	inFile2, err := os.Open(testFileName+comp.GetFileExtension())
	if err != nil {
		t.Fatal(err)
	}
	_, extension, err := comp.GetFileCompressionInfo(inFile)
	t.Logf("Extension for uncompressed: %s\n", extension)
	_, extension, err = comp.GetFileCompressionInfo(inFile2)
	t.Logf("Extension for compressed: %s\n", extension)
	inFile.Close()
}*/
