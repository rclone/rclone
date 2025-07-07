// Show the dynamic progress bar

package cmd

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/terminal"
)

const (
	// interval between progress prints
	defaultProgressInterval = 500 * time.Millisecond
)

// startProgress starts the progress bar printing
//
// It returns a func which should be called to stop the stats.
func startProgress() func() {
	stopStats := make(chan struct{})
	oldSyncPrint := operations.SyncPrintf

	if !log.Redirected() {
		// Intercept the log calls if not logging to file or syslog
		log.Handler.SetOutput(func(level slog.Level, text string) {
			printProgress(text)
		})
	}

	// Intercept output from functions such as HashLister to stdout
	operations.SyncPrintf = func(format string, a ...any) {
		printProgress(fmt.Sprintf(format, a...))
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
				if !log.Redirected() {
					// Reset intercept of the log calls
					log.Handler.ResetOutput()
				}
				operations.SyncPrintf = oldSyncPrint
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

// state for the progress printing
var (
	nlines = 0 // number of lines in the previous stats block
)

// printProgress prints the progress with an optional log
func printProgress(logMessage string) {
	operations.StdoutMutex.Lock()
	defer operations.StdoutMutex.Unlock()

	var buf bytes.Buffer
	w, _ := terminal.GetSize()
	stats := strings.TrimSpace(accounting.GlobalStats().String())
	logMessage = strings.TrimSpace(logMessage)

	out := func(s string) {
		buf.WriteString(s)
	}

	if logMessage != "" {
		out("\n")
		out(terminal.MoveUp)
	}
	// Move to the start of the block we wrote erasing all the previous lines
	for range nlines - 1 {
		out(terminal.EraseLine)
		out(terminal.MoveUp)
	}
	out(terminal.EraseLine)
	out(terminal.MoveToStartOfLine)
	if logMessage != "" {
		out(terminal.EraseLine)
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
	terminal.Write(buf.Bytes())
}
