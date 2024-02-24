// Test the VFS to exhaustion, specifically looking for deadlocks
//
// Run on a mounted filesystem
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"os"
	"path"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rclone/rclone/lib/file"
	"github.com/rclone/rclone/lib/random"
)

var (
	nameLength = flag.Int("name-length", 10, "Length of names to create")
	verbose    = flag.Bool("v", false, "Set to show more info")
	number     = flag.Int("n", 4, "Number of tests to run simultaneously")
	iterations = flag.Int("i", 100, "Iterations of the test")
	timeout    = flag.Duration("timeout", 10*time.Second, "Inactivity time to detect a deadlock")
	testNumber atomic.Int32
)

// Test contains stats about the running test which work for files or
// directories
type Test struct {
	dir     string
	name    string
	created bool
	handle  *os.File
	tests   []func()
	isDir   bool
	number  int32
	prefix  string
	timer   *time.Timer
}

// NewTest creates a new test and fills in the Tests
func NewTest(Dir string) *Test {
	t := &Test{
		dir:    Dir,
		name:   random.String(*nameLength),
		isDir:  rand.Intn(2) == 0,
		number: testNumber.Add(1),
		timer:  time.NewTimer(*timeout),
	}
	width := int(math.Floor(math.Log10(float64(*number)))) + 1
	t.prefix = fmt.Sprintf("%*d: %s: ", width, t.number, t.path())
	if t.isDir {
		t.tests = []func(){
			t.list,
			t.rename,
			t.mkdir,
			t.rmdir,
		}
	} else {
		t.tests = []func(){
			t.list,
			t.rename,
			t.open,
			t.close,
			t.remove,
			t.read,
			t.write,
		}
	}
	return t
}

// kick the deadlock timeout
func (t *Test) kick() {
	if !t.timer.Stop() {
		<-t.timer.C
	}
	t.timer.Reset(*timeout)
}

// randomTest runs a random test
func (t *Test) randomTest() {
	t.kick()
	i := rand.Intn(len(t.tests))
	t.tests[i]()
}

// logf logs things - not shown unless -v
func (t *Test) logf(format string, a ...interface{}) {
	if *verbose {
		log.Printf(t.prefix+format, a...)
	}
}

// errorf logs errors
func (t *Test) errorf(format string, a ...interface{}) {
	log.Printf(t.prefix+"ERROR: "+format, a...)
}

// list test
func (t *Test) list() {
	t.logf("list")
	fis, err := os.ReadDir(t.dir)
	if err != nil {
		t.errorf("%s: failed to read directory: %v", t.dir, err)
		return
	}
	if t.created && len(fis) == 0 {
		t.errorf("%s: expecting entries in directory, got none", t.dir)
		return
	}
	found := false
	for _, fi := range fis {
		if fi.Name() == t.name {
			found = true
		}
	}
	if t.created {
		if !found {
			t.errorf("%s: expecting to find %q in directory, got none", t.dir, t.name)
			return
		}
	} else {
		if found {
			t.errorf("%s: not expecting to find %q in directory, got none", t.dir, t.name)
			return
		}
	}
}

// path returns the current path to the item
func (t *Test) path() string {
	return path.Join(t.dir, t.name)
}

// rename test
func (t *Test) rename() {
	if !t.created {
		return
	}
	t.logf("rename")
	NewName := random.String(*nameLength)
	newPath := path.Join(t.dir, NewName)
	err := os.Rename(t.path(), newPath)
	if err != nil {
		t.errorf("failed to rename to %q: %v", newPath, err)
		return
	}
	t.name = NewName
}

// close test
func (t *Test) close() {
	if t.handle == nil {
		return
	}
	t.logf("close")
	err := t.handle.Close()
	t.handle = nil
	if err != nil {
		t.errorf("failed to close: %v", err)
		return
	}
}

// open test
func (t *Test) open() {
	t.close()
	t.logf("open")
	handle, err := file.OpenFile(t.path(), os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		t.errorf("failed to open: %v", err)
		return
	}
	t.handle = handle
	t.created = true
}

// read test
func (t *Test) read() {
	if t.handle == nil {
		return
	}
	t.logf("read")
	bytes := make([]byte, 10)
	_, err := t.handle.Read(bytes)
	if err != nil && err != io.EOF {
		t.errorf("failed to read: %v", err)
		return
	}
}

// write test
func (t *Test) write() {
	if t.handle == nil {
		return
	}
	t.logf("write")
	bytes := make([]byte, 10)
	_, err := t.handle.Write(bytes)
	if err != nil {
		t.errorf("failed to write: %v", err)
		return
	}
}

// remove test
func (t *Test) remove() {
	if !t.created {
		return
	}
	t.logf("remove")
	err := os.Remove(t.path())
	if err != nil {
		t.errorf("failed to remove: %v", err)
		return
	}
	t.created = false
}

// mkdir test
func (t *Test) mkdir() {
	if t.created {
		return
	}
	t.logf("mkdir")
	err := os.Mkdir(t.path(), 0777)
	if err != nil {
		t.errorf("failed to mkdir %q", t.path())
		return
	}
	t.created = true
}

// rmdir test
func (t *Test) rmdir() {
	if !t.created {
		return
	}
	t.logf("rmdir")
	err := os.Remove(t.path())
	if err != nil {
		t.errorf("failed to rmdir %q", t.path())
		return
	}
	t.created = false
}

// Tidy removes any stray files and stops the deadlock timer
func (t *Test) Tidy() {
	t.timer.Stop()
	if !t.isDir {
		t.close()
		t.remove()
	} else {
		t.rmdir()
	}
	t.logf("finished")
}

// RandomTests runs random tests with deadlock detection
func (t *Test) RandomTests(iterations int, quit chan struct{}) {
	var finished = make(chan struct{})
	go func() {
		for i := 0; i < iterations; i++ {
			t.randomTest()
		}
		close(finished)
	}()
	select {
	case <-finished:
	case <-quit:
		quit <- struct{}{}
	case <-t.timer.C:
		t.errorf("deadlock detected")
		quit <- struct{}{}
	}
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		log.Fatalf("%s: Syntax [opts] <directory>", os.Args[0])
	}
	dir := args[0]
	_ = file.MkdirAll(dir, 0777)

	var (
		wg   sync.WaitGroup
		quit = make(chan struct{}, *iterations)
	)
	for i := 0; i < *number; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t := NewTest(dir)
			defer t.Tidy()
			t.RandomTests(*iterations, quit)
		}()
	}
	wg.Wait()
}
