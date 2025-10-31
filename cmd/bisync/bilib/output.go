// Package bilib provides common stuff for bisync and bisync_test
package bilib

import (
	"bytes"
	"log/slog"
	"sync"

	"github.com/rclone/rclone/fs/log"
)

// CaptureOutput runs a function capturing its output at log level INFO.
func CaptureOutput(fun func()) []byte {
	var mu sync.Mutex
	buf := &bytes.Buffer{}
	oldLevel := log.Handler.SetLevel(slog.LevelInfo)
	log.Handler.SetOutput(func(level slog.Level, text string) {
		mu.Lock()
		defer mu.Unlock()
		buf.WriteString(text)
	})
	defer func() {
		log.Handler.ResetOutput()
		log.Handler.SetLevel(oldLevel)
	}()
	fun()
	mu.Lock()
	defer mu.Unlock()
	return buf.Bytes()
}
