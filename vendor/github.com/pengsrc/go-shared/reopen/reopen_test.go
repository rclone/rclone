package reopen

import (
	"io/ioutil"
	"os"
	"testing"
)

// TestReopenAppend tests that we always append to an existing file
func TestReopenAppend(t *testing.T) {
	filename := "/tmp/reopen_test_foo"
	defer os.Remove(filename)

	// Create a sample file using normal means.
	orig, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Unable to create initial file %s: %s", filename, err)
	}
	_, err = orig.Write([]byte("line0\n"))
	if err != nil {
		t.Fatalf("Unable to write initial line %s: %s", filename, err)
	}
	err = orig.Close()
	if err != nil {
		t.Fatalf("Unable to close initial file: %s", err)
	}

	// Test that making a new File appends.
	f, err := NewFileWriter(filename)
	if err != nil {
		t.Fatalf("Unable to create %s", filename)
	}
	_, err = f.Write([]byte("line1\n"))
	if err != nil {
		t.Errorf("Got write error1: %s", err)
	}

	// Test that reopen always appends.
	err = f.Reopen()
	if err != nil {
		t.Errorf("Got reopen error %s: %s", filename, err)
	}
	_, err = f.Write([]byte("line2\n"))
	if err != nil {
		t.Errorf("Got write error2 on %s: %s", filename, err)
	}

	// Close file.
	err = f.Close()
	if err != nil {
		t.Errorf("Got closing error for %s: %s", filename, err)
	}

	// Read file, make sure it contains line0, line1, line2.
	out, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatalf("Unable read in final file %s: %s", filename, err)
	}
	outStr := string(out)
	if outStr != "line0\nline1\nline2\n" {
		t.Errorf("Result was %s", outStr)
	}
}

// TestChangeINode tests that reopen works when inode is swapped out.
func TestChangeINODE(t *testing.T) {
	filename := "/tmp/reopen_test_foo"
	moveFilename := "/tmp/reopen_test_foo.orig"
	defer os.Remove(filename)
	defer os.Remove(moveFilename)

	// Step 1 -- Create a sample file using normal means.
	orig, err := os.Create(filename)
	if err != nil {
		t.Fatalf("Unable to create initial file %s: %s", filename, err)
	}
	err = orig.Close()
	if err != nil {
		t.Fatalf("Unable to close initial file: %s", err)
	}

	// Step 2 -- Test that making a new File appends.
	f, err := NewFileWriter(filename)
	if err != nil {
		t.Fatalf("Unable to create %s", filename)
	}
	_, err = f.Write([]byte("line1\n"))
	if err != nil {
		t.Errorf("Got write error1: %s", err)
	}

	// Step 3 -- Now move file.
	err = os.Rename(filename, moveFilename)
	if err != nil {
		t.Errorf("Renaming error: %s", err)
	}
	f.Write([]byte("after1\n"))

	// Step Test that reopen always appends.
	err = f.Reopen()
	if err != nil {
		t.Errorf("Got reopen error %s: %s", filename, err)
	}
	_, err = f.Write([]byte("line2\n"))
	if err != nil {
		t.Errorf("Got write error2 on %s: %s", filename, err)
	}

	// Close file.
	err = f.Close()
	if err != nil {
		t.Errorf("Got closing error for %s: %s", filename, err)
	}

	// Read file, make sure it contains line0, line1, line2.
	out, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatalf("Unable read in final file %s: %s", filename, err)
	}
	outStr := string(out)
	if outStr != "line2\n" {
		t.Errorf("Result was %s", outStr)
	}
}
