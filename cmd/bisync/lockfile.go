package bisync

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/terminal"
)

const basicallyforever = 200 * 365 * 24 * time.Hour

var stopRenewal func()

var data = struct {
	Session     string
	PID         string
	TimeRenewed time.Time
	TimeExpires time.Time
}{}

func (b *bisyncRun) setLockFile() error {
	b.lockFile = ""
	b.setLockFileExpiration()
	if !b.opt.DryRun {
		b.lockFile = b.basePath + ".lck"
		if bilib.FileExists(b.lockFile) {
			if !b.lockFileIsExpired() {
				errTip := Color(terminal.MagentaFg, "Tip: this indicates that another bisync run (of these same paths) either is still running or was interrupted before completion. \n")
				errTip += Color(terminal.MagentaFg, "If you're SURE you want to override this safety feature, you can delete the lock file with the following command, then run bisync again: \n")
				errTip += fmt.Sprintf(Color(terminal.HiRedFg, "rclone deletefile \"%s\""), b.lockFile)
				return fmt.Errorf(Color(terminal.RedFg, "prior lock file found: %s \n")+errTip, Color(terminal.HiYellowFg, b.lockFile))
			}
		}

		pidStr := []byte(strconv.Itoa(os.Getpid()))
		if err = os.WriteFile(b.lockFile, pidStr, bilib.PermSecure); err != nil {
			return fmt.Errorf(Color(terminal.RedFg, "cannot create lock file: %s: %w"), b.lockFile, err)
		}
		fs.Debugf(nil, "Lock file created: %s", b.lockFile)
		b.renewLockFile()
		stopRenewal = b.startLockRenewal()
	}
	return nil
}

func (b *bisyncRun) removeLockFile() {
	if b.lockFile != "" {
		stopRenewal()
		errUnlock := os.Remove(b.lockFile)
		if errUnlock == nil {
			fs.Debugf(nil, "Lock file removed: %s", b.lockFile)
		} else if err == nil {
			err = errUnlock
		} else {
			fs.Errorf(nil, "cannot remove lockfile %s: %v", b.lockFile, errUnlock)
		}
		b.lockFile = "" // block removing it again
	}
}

func (b *bisyncRun) setLockFileExpiration() {
	if b.opt.MaxLock > 0 && b.opt.MaxLock < 2*time.Minute {
		fs.Logf(nil, Color(terminal.YellowFg, "--max-lock cannot be shorter than 2 minutes (unless 0.) Changing --max-lock from %v to %v"), b.opt.MaxLock, 2*time.Minute)
		b.opt.MaxLock = 2 * time.Minute
	} else if b.opt.MaxLock <= 0 {
		b.opt.MaxLock = basicallyforever
	}
}

func (b *bisyncRun) renewLockFile() {
	if b.lockFile != "" && bilib.FileExists(b.lockFile) {

		data.Session = b.basePath
		data.PID = strconv.Itoa(os.Getpid())
		data.TimeRenewed = time.Now()
		data.TimeExpires = time.Now().Add(b.opt.MaxLock)

		// save data file
		df, err := os.Create(b.lockFile)
		b.handleErr(b.lockFile, "error renewing lock file", err, true, true)
		b.handleErr(b.lockFile, "error encoding JSON to lock file", json.NewEncoder(df).Encode(data), true, true)
		b.handleErr(b.lockFile, "error closing lock file", df.Close(), true, true)
		if b.opt.MaxLock < basicallyforever {
			fs.Infof(nil, Color(terminal.HiBlueFg, "lock file renewed for %v. New expiration: %v"), b.opt.MaxLock, data.TimeExpires)
		}
	}
}

func (b *bisyncRun) lockFileIsExpired() bool {
	if b.lockFile != "" && bilib.FileExists(b.lockFile) {
		rdf, err := os.Open(b.lockFile)
		b.handleErr(b.lockFile, "error reading lock file", err, true, true)
		dec := json.NewDecoder(rdf)
		for {
			if err := dec.Decode(&data); err != nil {
				if err != io.EOF {
					fs.Errorf(b.lockFile, "err: %v", err)
				}
				break
			}
		}
		b.handleErr(b.lockFile, "error closing file", rdf.Close(), true, true)
		if !data.TimeExpires.IsZero() && data.TimeExpires.Before(time.Now()) {
			fs.Infof(b.lockFile, Color(terminal.GreenFg, "Lock file found, but it expired at %v. Will delete it and proceed."), data.TimeExpires)
			markFailed(b.listing1) // listing is untrusted so force revert to prior (if --recover) or create new ones (if --resync)
			markFailed(b.listing2)
			return true
		}
		fs.Infof(b.lockFile, Color(terminal.RedFg, "Valid lock file found. Expires at %v. (%v from now)"), data.TimeExpires, time.Since(data.TimeExpires).Abs().Round(time.Second))
		prettyprint(data, "Lockfile info", fs.LogLevelInfo)
	}
	return false
}

// StartLockRenewal renews the lockfile every --max-lock minus one minute.
//
// It returns a func which should be called to stop the renewal.
func (b *bisyncRun) startLockRenewal() func() {
	if b.opt.MaxLock <= 0 || b.opt.MaxLock >= basicallyforever || b.lockFile == "" {
		return func() {}
	}
	stopLockRenewal := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(b.opt.MaxLock - time.Minute)
		for {
			select {
			case <-ticker.C:
				b.renewLockFile()
			case <-stopLockRenewal:
				ticker.Stop()
				return
			}
		}
	}()
	return func() {
		close(stopLockRenewal)
		wg.Wait()
	}
}

func markFailed(file string) {
	failFile := file + "-err"
	if bilib.FileExists(file) {
		_ = os.Remove(failFile)
		_ = os.Rename(file, failFile)
	}
}
