// Package pid provides structure and helper functions to create and remove
// PID file. A PID file is usually a file used to store the process ID of a
// running process.
package pid

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

// File is a file used to store the process ID of a running process.
type File struct {
	path string
}

func checkPIDFileAlreadyExists(path string) error {
	if pidByte, err := ioutil.ReadFile(path); err == nil {
		pidString := strings.TrimSpace(string(pidByte))
		if pid, err := strconv.Atoi(pidString); err == nil {
			if processExists(pid) {
				return fmt.Errorf("pid file found, ensure server is not running or delete %s", path)
			}
		}
	}
	return nil
}

// New creates a PID file using the specified path.
func New(path string) (*File, error) {
	if err := checkPIDFileAlreadyExists(path); err != nil {
		return nil, err
	}
	if err := ioutil.WriteFile(path, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
		return nil, err
	}

	return &File{path: path}, nil
}

// Remove removes the File.
func (file File) Remove() error {
	return os.Remove(file.path)
}
