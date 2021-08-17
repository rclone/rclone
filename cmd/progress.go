// Show the dynamic progress bar

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/terminal"
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
func startProgress(printProgressFunc func(string)) func() {
	stopStats := make(chan struct{})
	oldLogPrint := fs.LogPrint
	oldSyncPrint := operations.SyncPrintf

	if !log.Redirected() {
		// Intercept the log calls if not logging to file or syslog
		fs.LogPrint = func(level fs.LogLevel, text string) {
			printProgressFunc(fmt.Sprintf("%s %-6s: %s", time.Now().Format(logTimeFormat), level, text))
		}
	}

	// Intercept output from functions such as HashLister to stdout
	operations.SyncPrintf = func(format string, a ...interface{}) {
		printProgressFunc(fmt.Sprintf(format, a...))
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
				printProgressFunc("")
			case <-stopStats:
				ticker.Stop()
				printProgressFunc("")
				fs.LogPrint = oldLogPrint
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
	nlines     = 0 // number of lines in the previous stats block
	progressMu sync.Mutex
)

// printProgress prints the progress with an optional log
func printProgress(logMessage string) {
	progressMu.Lock()
	defer progressMu.Unlock()

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
	for i := 0; i < nlines-1; i++ {
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

// printProgressJSON prints the progress as JSON with an optional log
func printProgressJSON(logMessage string) {
	progressMu.Lock()
	defer progressMu.Unlock()

	stats, err := accounting.GlobalStats().RemoteStats()
	if err != nil {
		return
	}
	stats["log"] = logMessage

	statsBytes, err := json.Marshal(stats)
	if err != nil {
		return
	}
	terminal.Write(statsBytes)
	terminal.WriteString("\n")
}
