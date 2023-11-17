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
