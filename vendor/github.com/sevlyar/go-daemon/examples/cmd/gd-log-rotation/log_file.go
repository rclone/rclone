package main

import (
	"os"
	"sync"
	"time"
)

type LogFile struct {
	mu   sync.Mutex
	name string
	file *os.File
}

// NewLogFile creates a new LogFile. The file is optional - it will be created if needed.
func NewLogFile(name string, file *os.File) (*LogFile, error) {
	rw := &LogFile{
		file: file,
		name: name,
	}
	if file == nil {
		if err := rw.Rotate(); err != nil {
			return nil, err
		}
	}
	return rw, nil
}

func (l *LogFile) Write(b []byte) (n int, err error) {
	l.mu.Lock()
	n, err = l.file.Write(b)
	l.mu.Unlock()
	return
}

// Rotate renames old log file, creates new one, switches log and closes the old file.
func (l *LogFile) Rotate() error {
	// rename dest file if it already exists.
	if _, err := os.Stat(l.name); err == nil {
		name := l.name + "." + time.Now().Format(time.RFC3339)
		if err = os.Rename(l.name, name); err != nil {
			return err
		}
	}
	// create new file.
	file, err := os.Create(l.name)
	if err != nil {
		return err
	}
	// switch dest file safely.
	l.mu.Lock()
	file, l.file = l.file, file
	l.mu.Unlock()
	// close old file if open.
	if file != nil {
		if err := file.Close(); err != nil {
			return err
		}
	}
	return nil
}
