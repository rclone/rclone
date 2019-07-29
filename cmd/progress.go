// Show the dynamic progress bar

package cmd

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/log"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	// interval between progress prints
	defaultProgressInterval = 500 * time.Millisecond
	// time format for logging
	logTimeFormat = "2006-01-02 15:04:05"
)

// startProgress starts the progress bar printing
//
// It returns a func which should be called to stop the stats.
func startProgress() func() {
	err := initTerminal()
	if err != nil {
		fs.Errorf(nil, "Failed to start progress: %v", err)
		return func() {}
	}
	stopStats := make(chan struct{})
	oldLogPrint := fs.LogPrint
	if !log.Redirected() {
		// Intercept the log calls if not logging to file or syslog
		fs.LogPrint = func(level fs.LogLevel, text string) {
			printProgress(fmt.Sprintf("%s %-6s: %s", time.Now().Format(logTimeFormat), level, text))

		}
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		progressInterval := defaultProgressInterval
		if ShowStats() && *statsInterval > 0 {
			progressInterval = *statsInterval
		}
		ticker := time.NewTicker(progressInterval)
		for {
			select {
			case <-ticker.C:
				printProgress("")
			case <-stopStats:
				ticker.Stop()
				printProgress("")
				fs.LogPrint = oldLogPrint
				fmt.Println("")
				return
			}
		}
	}()
	return func() {
		close(stopStats)
		wg.Wait()
	}
}

// VT100 codes
const (
	eraseLine         = "\x1b[2K"
	moveToStartOfLine = "\x1b[0G"
	moveUp            = "\x1b[A"
)

// state for the progress printing
var (
	nlines     = 0 // number of lines in the previous stats block
	progressMu sync.Mutex
)

// printProgress prints the progress with an optional log
func printProgress(logMessage string) {
	progressMu.Lock()
	defer progressMu.Unlock()

	var buf bytes.Buffer
	w, h, err := terminal.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		w, h = 80, 25
	}
	_ = h
	stats := strings.TrimSpace(accounting.GlobalStats().String())
	logMessage = strings.TrimSpace(logMessage)

	out := func(s string) {
		buf.WriteString(s)
	}

	if logMessage != "" {
		out("\n")
		out(moveUp)
	}
	// Move to the start of the block we wrote erasing all the previous lines
	for i := 0; i < nlines-1; i++ {
		out(eraseLine)
		out(moveUp)
	}
	out(eraseLine)
	out(moveToStartOfLine)
	if logMessage != "" {
		out(eraseLine)
		out(logMessage + "\n")
	}
	fixedLines := strings.Split(stats, "\n")
	nlines = len(fixedLines)
	for i, line := range fixedLines {
		if len(line) > w {
			line = line[:w]
		}
		out(line)
		if i != nlines-1 {
			out("\n")
		}
	}
	writeToTerminal(buf.Bytes())
}
