package bisync

import (
	"context"
	"errors"
	"fmt"
	"strings"
	mutex "sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/terminal"
)

// CompareOpt describes the Compare options in force
type CompareOpt = struct {
	Modtime          bool
	Size             bool
	Checksum         bool
	HashType1        hash.Type
	HashType2        hash.Type
	NoSlowHash       bool
	SlowHashSyncOnly bool
	SlowHashDetected bool
	DownloadHash     bool
}

func (b *bisyncRun) setCompareDefaults(ctx context.Context) (err error) {
	ci := fs.GetConfig(ctx)

	// defaults
	b.opt.Compare.Size = true
	b.opt.Compare.Modtime = true
	b.opt.Compare.Checksum = false

	if ci.SizeOnly {
		b.opt.Compare.Size = true
		b.opt.Compare.Modtime = false
		b.opt.Compare.Checksum = false
	} else if ci.CheckSum && !b.opt.IgnoreListingChecksum {
		b.opt.Compare.Size = true
		b.opt.Compare.Modtime = false
		b.opt.Compare.Checksum = true
	}

	if ci.IgnoreSize {
		b.opt.Compare.Size = false
	}

	err = b.setFromCompareFlag(ctx)
	if err != nil {
		return err
	}

	if b.fs1.Features().SlowHash || b.fs2.Features().SlowHash {
		b.opt.Compare.SlowHashDetected = true
	}
	if b.opt.Compare.Checksum && !b.opt.IgnoreListingChecksum {
		b.setHashType(ci)
	}

	if b.opt.Compare.SlowHashSyncOnly && b.opt.Compare.SlowHashDetected && b.opt.Resync {
		fs.Log(nil, Color(terminal.Dim, "Ignoring checksums during --resync as --slow-hash-sync-only is set."))
		ci.CheckSum = false
		// note not setting b.opt.Compare.Checksum = false as we still want to build listings on the non-slow side, if any
	} else if b.opt.Compare.Checksum && !ci.CheckSum {
		fs.Log(nil, Color(terminal.YellowFg, "WARNING: Checksums will be compared for deltas but not during sync as --checksum is not set."))
	}
	if b.opt.Compare.Modtime && (b.fs1.Precision() == fs.ModTimeNotSupported || b.fs2.Precision() == fs.ModTimeNotSupported) {
		fs.Log(nil, Color(terminal.YellowFg, "WARNING: Modtime compare was requested but at least one remote does not support it. It is recommended to use --checksum or --size-only instead."))
	}
	if (ci.CheckSum || b.opt.Compare.Checksum) && b.opt.IgnoreListingChecksum {
		if (b.opt.Compare.HashType1 == hash.None || b.opt.Compare.HashType2 == hash.None) && !b.opt.Compare.DownloadHash {
			fs.Logf(nil, Color(terminal.YellowFg, `WARNING: Checksum compare was requested but at least one remote does not support checksums (or checksums are being ignored) and --ignore-listing-checksum is set.
			 Ignoring Checksums globally and falling back to --compare modtime,size for sync. (Use --compare size or --size-only to ignore modtime). Path1 (%s): %s, Path2 (%s): %s`),
				b.fs1.String(), b.opt.Compare.HashType1.String(), b.fs2.String(), b.opt.Compare.HashType2.String())
			b.opt.Compare.Modtime = true
			b.opt.Compare.Size = true
			ci.CheckSum = false
			b.opt.Compare.Checksum = false
		} else {
			fs.Log(nil, Color(terminal.YellowFg, "WARNING: Ignoring checksum for deltas as --ignore-listing-checksum is set"))
			// note: --checksum will still affect the internal sync calls
		}
	}
	if !ci.CheckSum && !b.opt.Compare.Checksum && !b.opt.IgnoreListingChecksum {
		fs.Infoc(nil, Color(terminal.Dim, "Setting --ignore-listing-checksum as neither --checksum nor --compare checksum are set."))
		b.opt.IgnoreListingChecksum = true
	}
	if !b.opt.Compare.Size && !b.opt.Compare.Modtime && !b.opt.Compare.Checksum {
		return errors.New(Color(terminal.RedFg, "must set a Compare method. (size, modtime, and checksum can't all be false.)"))
	}

	notSupported := func(label string, value bool, opt *bool) {
		if value {
			fs.Logf(nil, Color(terminal.YellowFg, "WARNING: %s is set but bisync does not support it. It will be ignored."), label)
			*opt = false
		}
	}
	notSupported("--update", ci.UpdateOlder, &ci.UpdateOlder)
	notSupported("--no-check-dest", ci.NoCheckDest, &ci.NoCheckDest)
	notSupported("--no-traverse", ci.NoTraverse, &ci.NoTraverse)
	// TODO: thorough search for other flags that should be on this list...

	prettyprint(b.opt.Compare, "Bisyncing with Comparison Settings", fs.LogLevelInfo)
	return nil
}

// returns true if the sizes are definitely different.
// returns false if equal, or if either is unknown.
func sizeDiffers(a, b int64) bool {
	if a < 0 || b < 0 {
		return false
	}
	return a != b
}

// returns true if the hashes are definitely different.
// returns false if equal, or if either is unknown.
func (b *bisyncRun) hashDiffers(stringA, stringB string, ht1, ht2 hash.Type, size1, size2 int64) bool {
	if stringA == "" || stringB == "" {
		if ht1 != hash.None && ht2 != hash.None && !(size1 <= 0 || size2 <= 0) {
			fs.Logf(nil, Color(terminal.YellowFg, "WARNING: hash unexpectedly blank despite Fs support (%s, %s) (you may need to --resync!)"), stringA, stringB)
		}
		return false
	}
	if ht1 != ht2 {
		if !(b.downloadHashOpt.downloadHash && ((ht1 == hash.MD5 && ht2 == hash.None) || (ht1 == hash.None && ht2 == hash.MD5))) {
			fs.Infof(nil, Color(terminal.YellowFg, "WARNING: Can't compare hashes of different types (%s, %s)"), ht1.String(), ht2.String())
			return false
		}
	}
	return stringA != stringB
}

// chooses hash type, giving priority to types both sides have in common
func (b *bisyncRun) setHashType(ci *fs.ConfigInfo) {
	b.downloadHashOpt.downloadHash = b.opt.Compare.DownloadHash
	if b.opt.Compare.NoSlowHash && b.opt.Compare.SlowHashDetected {
		fs.Infof(nil, "Not checking for common hash as at least one slow hash detected.")
	} else {
		common := b.fs1.Hashes().Overlap(b.fs2.Hashes())
		if common.Count() > 0 && common.GetOne() != hash.None {
			ht := common.GetOne()
			b.opt.Compare.HashType1 = ht
			b.opt.Compare.HashType2 = ht
			if !b.opt.Compare.SlowHashSyncOnly || !b.opt.Compare.SlowHashDetected {
				return
			}
		} else if b.opt.Compare.SlowHashSyncOnly && b.opt.Compare.SlowHashDetected {
			fs.Log(b.fs2, Color(terminal.YellowFg, "Ignoring --slow-hash-sync-only and falling back to --no-slow-hash as Path1 and Path2 have no hashes in common."))
			b.opt.Compare.SlowHashSyncOnly = false
			b.opt.Compare.NoSlowHash = true
			ci.CheckSum = false
		}
	}

	if !b.opt.Compare.DownloadHash && !b.opt.Compare.SlowHashSyncOnly {
		fs.Log(b.fs2, Color(terminal.YellowFg, "--checksum is in use but Path1 and Path2 have no hashes in common; falling back to --compare modtime,size for sync. (Use --compare size or --size-only to ignore modtime)"))
		fs.Infof("Path1 hashes", "%v", b.fs1.Hashes().String())
		fs.Infof("Path2 hashes", "%v", b.fs2.Hashes().String())
		b.opt.Compare.Modtime = true
		b.opt.Compare.Size = true
		ci.CheckSum = false
	}
	if (b.opt.Compare.NoSlowHash || b.opt.Compare.SlowHashSyncOnly) && b.fs1.Features().SlowHash {
		fs.Infoc(nil, Color(terminal.YellowFg, "Slow hash detected on Path1. Will ignore checksum due to slow-hash settings"))
		b.opt.Compare.HashType1 = hash.None
	} else {
		b.opt.Compare.HashType1 = b.fs1.Hashes().GetOne()
		if b.opt.Compare.HashType1 != hash.None {
			fs.Logf(b.fs1, Color(terminal.YellowFg, "will use %s for same-side diffs on Path1 only"), b.opt.Compare.HashType1)
		}
	}
	if (b.opt.Compare.NoSlowHash || b.opt.Compare.SlowHashSyncOnly) && b.fs2.Features().SlowHash {
		fs.Infoc(nil, Color(terminal.YellowFg, "Slow hash detected on Path2. Will ignore checksum due to slow-hash settings"))
		b.opt.Compare.HashType2 = hash.None
	} else {
		b.opt.Compare.HashType2 = b.fs2.Hashes().GetOne()
		if b.opt.Compare.HashType2 != hash.None {
			fs.Logf(b.fs2, Color(terminal.YellowFg, "will use %s for same-side diffs on Path2 only"), b.opt.Compare.HashType2)
		}
	}
	if b.opt.Compare.HashType1 == hash.None && b.opt.Compare.HashType2 == hash.None && !b.opt.Compare.DownloadHash {
		fs.Log(nil, Color(terminal.YellowFg, "WARNING: Ignoring checksums globally as hashes are ignored or unavailable on both sides."))
		b.opt.Compare.Checksum = false
		ci.CheckSum = false
		b.opt.IgnoreListingChecksum = true
	}
}

// returns true if the times are definitely different (by more than the modify window).
// returns false if equal, within modify window, or if either is unknown.
// considers precision per-Fs.
func timeDiffers(ctx context.Context, a, b time.Time, fsA, fsB fs.Info) bool {
	modifyWindow := fs.GetModifyWindow(ctx, fsA, fsB)
	if modifyWindow == fs.ModTimeNotSupported {
		return false
	}
	if a.IsZero() || b.IsZero() {
		fs.Logf(fsA, "Fs supports modtime, but modtime is missing")
		return false
	}
	dt := b.Sub(a)
	if dt < modifyWindow && dt > -modifyWindow {
		fs.Debugf(a, "modification time the same (differ by %s, within tolerance %s)", dt, modifyWindow)
		return false
	}

	fs.Debugf(a, "Modification times differ by %s: %v, %v", dt, a, b)
	return true
}

func (b *bisyncRun) setFromCompareFlag(ctx context.Context) error {
	if b.opt.CompareFlag == "" {
		return nil
	}
	var CompareFlag CompareOpt // for exclusions
	opts := strings.Split(b.opt.CompareFlag, ",")
	for _, opt := range opts {
		switch strings.ToLower(strings.TrimSpace(opt)) {
		case "size":
			b.opt.Compare.Size = true
			CompareFlag.Size = true
		case "modtime":
			b.opt.Compare.Modtime = true
			CompareFlag.Modtime = true
		case "checksum":
			b.opt.Compare.Checksum = true
			CompareFlag.Checksum = true
		default:
			return fmt.Errorf(Color(terminal.RedFg, "unknown compare option: %s (must be size, modtime, or checksum)"), opt)
		}
	}

	// exclusions (override defaults, only if --compare != "")
	if !CompareFlag.Size {
		b.opt.Compare.Size = false
	}
	if !CompareFlag.Modtime {
		b.opt.Compare.Modtime = false
	}
	if !CompareFlag.Checksum {
		b.opt.Compare.Checksum = false
	}

	// override sync flags to match
	ci := fs.GetConfig(ctx)
	if b.opt.Compare.Checksum {
		ci.CheckSum = true
	}
	if b.opt.Compare.Modtime && !b.opt.Compare.Checksum {
		ci.CheckSum = false
	}
	if !b.opt.Compare.Size {
		ci.IgnoreSize = true
	}
	if !b.opt.Compare.Modtime {
		ci.UseServerModTime = true
	}
	if b.opt.Compare.Size && !b.opt.Compare.Modtime && !b.opt.Compare.Checksum {
		ci.SizeOnly = true
	}

	return nil
}

// b.downloadHashOpt.downloadHash is true if we should attempt to compute hash by downloading when otherwise unavailable
type downloadHashOpt struct {
	downloadHash      bool
	downloadHashWarn  mutex.Once
	firstDownloadHash mutex.Once
}

func (b *bisyncRun) tryDownloadHash(ctx context.Context, o fs.DirEntry, hashVal string) (string, error) {
	if hashVal != "" || !b.downloadHashOpt.downloadHash {
		return hashVal, nil
	}
	obj, ok := o.(fs.Object)
	if !ok {
		fs.Infof(o, "failed to download hash -- not an fs.Object")
		return hashVal, fs.ErrorObjectNotFound
	}
	if o.Size() < 0 {
		b.downloadHashOpt.downloadHashWarn.Do(func() {
			fs.Log(o, Color(terminal.YellowFg, "Skipping hash download as checksum not reliable with files of unknown length."))
		})
		fs.Debugf(o, "Skipping hash download as checksum not reliable with files of unknown length.")
		return hashVal, hash.ErrUnsupported
	}

	b.downloadHashOpt.firstDownloadHash.Do(func() {
		fs.Infoc(obj.Fs().Name(), Color(terminal.Dim, "Downloading hashes..."))
	})
	tr := accounting.Stats(ctx).NewCheckingTransfer(o, "computing hash with --download-hash")
	defer func() {
		tr.Done(ctx, nil)
	}()

	sum, err := operations.HashSum(ctx, hash.MD5, false, true, obj)
	if err != nil {
		fs.Infof(o, "DownloadHash -- hash: %v, err: %v", sum, err)
	} else {
		fs.Debugf(o, "DownloadHash -- hash: %v", sum)
	}
	return sum, err
}
