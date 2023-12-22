package bisync

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/rclone/rclone/backend/crypt"
	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/cmd/check"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
)

var hashType hash.Type
var fsrc, fdst fs.Fs
var fcrypt *crypt.Fs

// WhichCheck determines which CheckFn we should use based on the Fs types
// It is more robust and accurate than Check because
// it will fallback to CryptCheck or DownloadCheck instead of --size-only!
// it returns the *operations.CheckOpt with the CheckFn set.
func WhichCheck(ctx context.Context, opt *operations.CheckOpt) *operations.CheckOpt {
	ci := fs.GetConfig(ctx)
	common := opt.Fsrc.Hashes().Overlap(opt.Fdst.Hashes())

	// note that ci.IgnoreChecksum doesn't change the behavior of Check -- it's just a way to opt-out of cryptcheck/download
	if common.Count() > 0 || ci.SizeOnly || ci.IgnoreChecksum {
		// use normal check
		opt.Check = CheckFn
		return opt
	}

	FsrcCrypt, srcIsCrypt := opt.Fsrc.(*crypt.Fs)
	FdstCrypt, dstIsCrypt := opt.Fdst.(*crypt.Fs)

	if (srcIsCrypt && dstIsCrypt) || (!srcIsCrypt && dstIsCrypt) {
		// if both are crypt or only dst is crypt
		hashType = FdstCrypt.UnWrap().Hashes().GetOne()
		if hashType != hash.None {
			// use cryptcheck
			fsrc = opt.Fsrc
			fdst = opt.Fdst
			fcrypt = FdstCrypt
			fs.Infof(fdst, "Crypt detected! Using cryptcheck instead of check. (Use --size-only or --ignore-checksum to disable)")
			opt.Check = CryptCheckFn
			return opt
		}
	} else if srcIsCrypt && !dstIsCrypt {
		// if only src is crypt
		hashType = FsrcCrypt.UnWrap().Hashes().GetOne()
		if hashType != hash.None {
			// use reverse cryptcheck
			fsrc = opt.Fdst
			fdst = opt.Fsrc
			fcrypt = FsrcCrypt
			fs.Infof(fdst, "Crypt detected! Using cryptcheck instead of check. (Use --size-only or --ignore-checksum to disable)")
			opt.Check = ReverseCryptCheckFn
			return opt
		}
	}

	// if we've gotten this far, niether check or cryptcheck will work, so use --download
	fs.Infof(fdst, "Can't compare hashes, so using check --download for safety. (Use --size-only or --ignore-checksum to disable)")
	opt.Check = DownloadCheckFn
	return opt
}

// CheckFn is a slightly modified version of Check
func CheckFn(ctx context.Context, dst, src fs.Object) (differ bool, noHash bool, err error) {
	same, ht, err := operations.CheckHashes(ctx, src, dst)
	if err != nil {
		return true, false, err
	}
	if ht == hash.None {
		return false, true, nil
	}
	if !same {
		err = fmt.Errorf("%v differ", ht)
		fs.Errorf(src, "%v", err)
		return true, false, nil
	}
	return false, false, nil
}

// CryptCheckFn is a slightly modified version of CryptCheck
func CryptCheckFn(ctx context.Context, dst, src fs.Object) (differ bool, noHash bool, err error) {
	cryptDst := dst.(*crypt.Object)
	underlyingDst := cryptDst.UnWrap()
	underlyingHash, err := underlyingDst.Hash(ctx, hashType)
	if err != nil {
		return true, false, fmt.Errorf("error reading hash from underlying %v: %w", underlyingDst, err)
	}
	if underlyingHash == "" {
		return false, true, nil
	}
	cryptHash, err := fcrypt.ComputeHash(ctx, cryptDst, src, hashType)
	if err != nil {
		return true, false, fmt.Errorf("error computing hash: %w", err)
	}
	if cryptHash == "" {
		return false, true, nil
	}
	if cryptHash != underlyingHash {
		err = fmt.Errorf("hashes differ (%s:%s) %q vs (%s:%s) %q", fdst.Name(), fdst.Root(), cryptHash, fsrc.Name(), fsrc.Root(), underlyingHash)
		fs.Debugf(src, err.Error())
		// using same error msg as CheckFn so integration tests match
		err = fmt.Errorf("%v differ", hashType)
		fs.Errorf(src, err.Error())
		return true, false, nil
	}
	return false, false, nil
}

// ReverseCryptCheckFn is like CryptCheckFn except src and dst are switched
// result: src is crypt, dst is non-crypt
func ReverseCryptCheckFn(ctx context.Context, dst, src fs.Object) (differ bool, noHash bool, err error) {
	return CryptCheckFn(ctx, src, dst)
}

// DownloadCheckFn is a slightly modified version of Check with --download
func DownloadCheckFn(ctx context.Context, a, b fs.Object) (differ bool, noHash bool, err error) {
	differ, err = operations.CheckIdenticalDownload(ctx, a, b)
	if err != nil {
		return true, true, fmt.Errorf("failed to download: %w", err)
	}
	return differ, false, nil
}

// check potential conflicts (to avoid renaming if already identical)
func (b *bisyncRun) checkconflicts(ctxCheck context.Context, filterCheck *filter.Filter, fs1, fs2 fs.Fs) (bilib.Names, error) {
	matches := bilib.Names{}
	if filterCheck.HaveFilesFrom() {
		fs.Debugf(nil, "There are potential conflicts to check.")

		opt, close, checkopterr := check.GetCheckOpt(b.fs1, b.fs2)
		if checkopterr != nil {
			b.critical = true
			b.retryable = true
			fs.Debugf(nil, "GetCheckOpt error: %v", checkopterr)
			return matches, checkopterr
		}
		defer close()

		opt.Match = new(bytes.Buffer)

		opt = WhichCheck(ctxCheck, opt)

		fs.Infof(nil, "Checking potential conflicts...")
		check := operations.CheckFn(ctxCheck, opt)
		fs.Infof(nil, "Finished checking the potential conflicts. %s", check)

		//reset error count, because we don't want to count check errors as bisync errors
		accounting.Stats(ctxCheck).ResetErrors()

		//return the list of identical files to check against later
		if len(fmt.Sprint(opt.Match)) > 0 {
			matches = bilib.ToNames(strings.Split(fmt.Sprint(opt.Match), "\n"))
		}
		if matches.NotEmpty() {
			fs.Debugf(nil, "The following potential conflicts were determined to be identical. %v", matches)
		} else {
			fs.Debugf(nil, "None of the conflicts were determined to be identical.")
		}

	}
	return matches, nil
}

// WhichEqual is similar to WhichCheck, but checks a single object.
// Returns true if the objects are equal, false if they differ or if we don't know
func WhichEqual(ctx context.Context, src, dst fs.Object, Fsrc, Fdst fs.Fs) bool {
	opt, close, checkopterr := check.GetCheckOpt(Fsrc, Fdst)
	if checkopterr != nil {
		fs.Debugf(nil, "GetCheckOpt error: %v", checkopterr)
	}
	defer close()

	opt = WhichCheck(ctx, opt)
	differ, noHash, err := opt.Check(ctx, dst, src)
	if err != nil {
		fs.Errorf(src, "failed to check: %v", err)
		return false
	}
	if noHash {
		fs.Errorf(src, "failed to check as hash is missing")
		return false
	}
	return !differ
}

// Replaces the standard Equal func with one that also considers checksum
// Note that it also updates the modtime the same way as Sync
func (b *bisyncRun) EqualFn(ctx context.Context) context.Context {
	ci := fs.GetConfig(ctx)
	ci.CheckSum = false // force checksum off so modtime is evaluated if needed
	// modtime and size settings should already be set correctly for Equal
	var equalFn operations.EqualFn = func(ctx context.Context, src fs.ObjectInfo, dst fs.Object) bool {
		fs.Debugf(src, "evaluating...")
		equal := false
		logger, _ := operations.GetLogger(ctx)
		// temporarily unset logger, we don't want Equal to duplicate it
		noop := func(ctx context.Context, sigil operations.Sigil, src, dst fs.DirEntry, err error) {
			fs.Debugf(src, "equal skipped")
		}
		ctxNoLogger := operations.WithLogger(ctx, noop)

		timeSizeEqualFn := func() (equal bool, skipHash bool) { return operations.Equal(ctxNoLogger, src, dst), false } // normally use Equal()
		if b.opt.ResyncMode == PreferOlder || b.opt.ResyncMode == PreferLarger || b.opt.ResyncMode == PreferSmaller {
			timeSizeEqualFn = func() (equal bool, skipHash bool) { return b.resyncTimeSizeEqual(ctxNoLogger, src, dst) } // but override for --resync-mode older, larger, smaller
		}
		skipHash := false // (note that we might skip it anyway based on compare/ht settings)
		equal, skipHash = timeSizeEqualFn()
		if equal && !skipHash {
			whichHashType := func(f fs.Info) hash.Type {
				ht := getHashType(f.Name())
				if ht == hash.None && b.opt.Compare.SlowHashSyncOnly && !b.opt.Resync {
					ht = f.Hashes().GetOne()
				}
				return ht
			}
			srcHash, _ := src.Hash(ctx, whichHashType(src.Fs()))
			dstHash, _ := dst.Hash(ctx, whichHashType(dst.Fs()))
			srcHash, _ = tryDownloadHash(ctx, src, srcHash)
			dstHash, _ = tryDownloadHash(ctx, dst, dstHash)
			equal = !hashDiffers(srcHash, dstHash, whichHashType(src.Fs()), whichHashType(dst.Fs()), src.Size(), dst.Size())
		}
		if equal {
			logger(ctx, operations.Match, src, dst, nil)
			fs.Debugf(src, "EqualFn: files are equal")
			return true
		}
		logger(ctx, operations.Differ, src, dst, nil)
		fs.Debugf(src, "EqualFn: files are NOT equal")
		return false
	}
	return operations.WithEqualFn(ctx, equalFn)
}

func (b *bisyncRun) resyncTimeSizeEqual(ctxNoLogger context.Context, src fs.ObjectInfo, dst fs.Object) (equal bool, skipHash bool) {
	switch b.opt.ResyncMode {
	case PreferLarger, PreferSmaller:
		// note that arg order is path1, path2, regardless of src/dst
		path1, path2 := b.resyncWhichIsWhich(src, dst)
		if sizeDiffers(path1.Size(), path2.Size()) {
			winningPath := b.resolveLargerSmaller(path1.Size(), path2.Size(), path1.Remote(), path2.Remote(), b.opt.ResyncMode)
			// don't need to check/update modtime here, as sizes definitely differ and something will be transferred
			return b.resyncWinningPathToEqual(winningPath), b.resyncWinningPathToEqual(winningPath) // skip hash check if true
		}
		// sizes equal or don't know, so continue to checking time/hash, if applicable
		return operations.Equal(ctxNoLogger, src, dst), false // note we're back to src/dst, not path1/path2
	case PreferOlder:
		// note that arg order is path1, path2, regardless of src/dst
		path1, path2 := b.resyncWhichIsWhich(src, dst)
		if timeDiffers(ctxNoLogger, path1.ModTime(ctxNoLogger), path2.ModTime(ctxNoLogger), path1.Fs(), path2.Fs()) {
			winningPath := b.resolveNewerOlder(path1.ModTime(ctxNoLogger), path2.ModTime(ctxNoLogger), path1.Remote(), path2.Remote(), b.opt.ResyncMode)
			// if src is winner, proceed with equal to check size/hash and possibly just update dest modtime instead of transferring
			if !b.resyncWinningPathToEqual(winningPath) {
				return operations.Equal(ctxNoLogger, src, dst), false // note we're back to src/dst, not path1/path2
			}
			// if dst is winner (and definitely unequal), do not proceed further as we want dst to overwrite src regardless of size difference, and we do not want dest modtime updated
			return true, true
		}
		// times equal or don't know, so continue to checking size/hash, if applicable
	}
	return operations.Equal(ctxNoLogger, src, dst), false // note we're back to src/dst, not path1/path2
}
