package upload

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

var partSize = 5 * 1024

//Test_newFileChunk is the test function for New
func Test_newFileChunk(t *testing.T) {
	setup()

	fd, _ := os.Open("test_file")
	defer fd.Close()
	fr := newChunk(fd, partSize)
	if fr.size != 512000 {
		t.Fatalf("expected 512000, got %d", fr.size)
	}

	tearDown()
}

// Test_nextPart is the test function for nextSeekablePart
func Test_nextPart(t *testing.T) {
	setup()

	fd, _ := os.Open("test_file")
	defer fd.Close()
	fr := newChunk(fd, partSize)
	partBody, err := fr.nextPart()
	if err != nil {
		fmt.Println(err)
	}
	temp := make([]byte, 6000)
	n, _ := partBody.Read(temp)
	if n != partSize {
		t.Fatalf("expected 5120, got %d", len(temp))
	}

	tearDown()
}

func setup() {
	exec.Command("dd", "if=/dev/zero", "of=test_file", "bs=1024", "count=500").Output()
}

func tearDown() {
	exec.Command("rm", "", "test_file").Output()
}
