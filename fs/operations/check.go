package operations

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/march"
	"github.com/rclone/rclone/lib/readers"
)

// checkFn is the type of the checking function used in CheckFn()
//
// It should check the two objects (a, b) and return if they differ
// and whether the hash was used.
//
// If there are differences then this should Errorf the difference and
// the reason but return with err = nil. It should not CountError in
// this case.
type checkFn func(ctx context.Context, a, b fs.Object) (differ bool, noHash bool, err error)

// CheckOpt contains options for the Check functions
type CheckOpt struct {
	Fdst, Fsrc   fs.Fs     // fses to check
	Check        checkFn   // function to use for checking
	OneWay       bool      // one way only?
	Combined     io.Writer // a file with file names with leading sigils
	MissingOnSrc io.Writer // files only in the destination
	MissingOnDst io.Writer // files only in the source
	Match        io.Writer // matching files
	Differ       io.Writer // differing files
	Error        io.Writer // files with errors of some kind
}

// checkMarch is used to march over two Fses in the same way as
// sync/copy
type checkMarch struct {
	ioMu            sync.Mutex
	wg              sync.WaitGroup
	tokens          chan struct{}
	differences     int32
	noHashes        int32
	srcFilesMissing int32
	dstFilesMissing int32
	matches         int32
	opt             CheckOpt
}

// report outputs the fileName to out if required and to the combined log
func (c *checkMarch) report(o fs.DirEntry, out io.Writer, sigil rune) {
	if out != nil {
		c.ioMu.Lock()
		_, _ = fmt.Fprintf(out, "%v\n", o)
		c.ioMu.Unlock()
	}
	if c.opt.Combined != nil {
		c.ioMu.Lock()
		_, _ = fmt.Fprintf(c.opt.Combined, "%c %v\n", sigil, o)
		c.ioMu.Unlock()
	}
}

// DstOnly have an object which is in the destination only
func (c *checkMarch) DstOnly(dst fs.DirEntry) (recurse bool) {
	switch dst.(type) {
	case fs.Object:
		if c.opt.OneWay {
			return false
		}
		err := errors.Errorf("File not in %v", c.opt.Fsrc)
		fs.Errorf(dst, "%v", err)
		_ = fs.CountError(err)
		atomic.AddInt32(&c.differences, 1)
		atomic.AddInt32(&c.srcFilesMissing, 1)
		c.report(dst, c.opt.MissingOnSrc, '-')
	case fs.Directory:
		// Do the same thing to the entire contents of the directory
		if c.opt.OneWay {
			return false
		}
		return true
	default:
		panic("Bad object in DirEntries")
	}
	return false
}

// SrcOnly have an object which is in the source only
func (c *checkMarch) SrcOnly(src fs.DirEntry) (recurse bool) {
	switch src.(type) {
	case fs.Object:
		err := errors.Errorf("File not in %v", c.opt.Fdst)
		fs.Errorf(src, "%v", err)
		_ = fs.CountError(err)
		atomic.AddInt32(&c.differences, 1)
		atomic.AddInt32(&c.dstFilesMissing, 1)
		c.report(src, c.opt.MissingOnDst, '+')
	case fs.Directory:
		// Do the same thing to the entire contents of the directory
		return true
	default:
		panic("Bad object in DirEntries")
	}
	return false
}

// check to see if two objects are identical using the check function
func (c *checkMarch) checkIdentical(ctx context.Context, dst, src fs.Object) (differ bool, noHash bool, err error) {
	tr := accounting.Stats(ctx).NewCheckingTransfer(src)
	defer func() {
		tr.Done(err)
	}()
	if sizeDiffers(src, dst) {
		err = errors.Errorf("Sizes differ")
		fs.Errorf(src, "%v", err)
		return true, false, nil
	}
	if fs.Config.SizeOnly {
		return false, false, nil
	}
	return c.opt.Check(ctx, dst, src)
}

// Match is called when src and dst are present, so sync src to dst
func (c *checkMarch) Match(ctx context.Context, dst, src fs.DirEntry) (recurse bool) {
	switch srcX := src.(type) {
	case fs.Object:
		dstX, ok := dst.(fs.Object)
		if ok {
			if SkipDestructive(ctx, src, "check") {
				return false
			}
			c.wg.Add(1)
			c.tokens <- struct{}{} // put a token to limit concurrency
			go func() {
				defer func() {
					<-c.tokens // get the token back to free up a slot
					c.wg.Done()
				}()
				differ, noHash, err := c.checkIdentical(ctx, dstX, srcX)
				if err != nil {
					fs.Errorf(src, "%v", err)
					_ = fs.CountError(err)
					c.report(src, c.opt.Error, '!')
				} else if differ {
					atomic.AddInt32(&c.differences, 1)
					err := errors.New("files differ")
					// the checkFn has already logged the reason
					_ = fs.CountError(err)
					c.report(src, c.opt.Differ, '*')
				} else {
					atomic.AddInt32(&c.matches, 1)
					c.report(src, c.opt.Match, '=')
					if noHash {
						atomic.AddInt32(&c.noHashes, 1)
						fs.Debugf(dstX, "OK - could not check hash")
					} else {
						fs.Debugf(dstX, "OK")
					}
				}
			}()
		} else {
			err := errors.Errorf("is file on %v but directory on %v", c.opt.Fsrc, c.opt.Fdst)
			fs.Errorf(src, "%v", err)
			_ = fs.CountError(err)
			atomic.AddInt32(&c.differences, 1)
			atomic.AddInt32(&c.dstFilesMissing, 1)
			c.report(src, c.opt.MissingOnDst, '+')
		}
	case fs.Directory:
		// Do the same thing to the entire contents of the directory
		_, ok := dst.(fs.Directory)
		if ok {
			return true
		}
		err := errors.Errorf("is file on %v but directory on %v", c.opt.Fdst, c.opt.Fsrc)
		fs.Errorf(dst, "%v", err)
		_ = fs.CountError(err)
		atomic.AddInt32(&c.differences, 1)
		atomic.AddInt32(&c.srcFilesMissing, 1)
		c.report(dst, c.opt.MissingOnSrc, '-')

	default:
		panic("Bad object in DirEntries")
	}
	return false
}

// CheckFn checks the files in fsrc and fdst according to Size and
// hash using checkFunction on each file to check the hashes.
//
// checkFunction sees if dst and src are identical
//
// it returns true if differences were found
// it also returns whether it couldn't be hashed
func CheckFn(ctx context.Context, opt *CheckOpt) error {
	if opt.Check == nil {
		return errors.New("internal error: nil check function")
	}
	c := &checkMarch{
		tokens: make(chan struct{}, fs.Config.Checkers),
		opt:    *opt,
	}

	// set up a march over fdst and fsrc
	m := &march.March{
		Ctx:      ctx,
		Fdst:     c.opt.Fdst,
		Fsrc:     c.opt.Fsrc,
		Dir:      "",
		Callback: c,
	}
	fs.Debugf(c.opt.Fdst, "Waiting for checks to finish")
	err := m.Run()
	c.wg.Wait() // wait for background go-routines

	if c.dstFilesMissing > 0 {
		fs.Logf(c.opt.Fdst, "%d files missing", c.dstFilesMissing)
	}
	if c.srcFilesMissing > 0 {
		fs.Logf(c.opt.Fsrc, "%d files missing", c.srcFilesMissing)
	}

	fs.Logf(c.opt.Fdst, "%d differences found", accounting.Stats(ctx).GetErrors())
	if errs := accounting.Stats(ctx).GetErrors(); errs > 0 {
		fs.Logf(c.opt.Fdst, "%d errors while checking", errs)
	}
	if c.noHashes > 0 {
		fs.Logf(c.opt.Fdst, "%d hashes could not be checked", c.noHashes)
	}
	if c.matches > 0 {
		fs.Logf(c.opt.Fdst, "%d matching files", c.matches)
	}
	if c.differences > 0 {
		return errors.Errorf("%d differences found", c.differences)
	}
	return err
}

// Check the files in fsrc and fdst according to Size and hash
func Check(ctx context.Context, opt *CheckOpt) error {
	optCopy := *opt
	optCopy.Check = func(ctx context.Context, dst, src fs.Object) (differ bool, noHash bool, err error) {
		same, ht, err := CheckHashes(ctx, src, dst)
		if err != nil {
			return true, false, err
		}
		if ht == hash.None {
			return false, true, nil
		}
		if !same {
			err = errors.Errorf("%v differ", ht)
			fs.Errorf(src, "%v", err)
			return true, false, nil
		}
		return false, false, nil
	}

	return CheckFn(ctx, &optCopy)
}

// CheckEqualReaders checks to see if in1 and in2 have the same
// content when read.
//
// it returns true if differences were found
func CheckEqualReaders(in1, in2 io.Reader) (differ bool, err error) {
	const bufSize = 64 * 1024
	buf1 := make([]byte, bufSize)
	buf2 := make([]byte, bufSize)
	for {
		n1, err1 := readers.ReadFill(in1, buf1)
		n2, err2 := readers.ReadFill(in2, buf2)
		// check errors
		if err1 != nil && err1 != io.EOF {
			return true, err1
		} else if err2 != nil && err2 != io.EOF {
			return true, err2
		}
		// err1 && err2 are nil or io.EOF here
		// process the data
		if n1 != n2 || !bytes.Equal(buf1[:n1], buf2[:n2]) {
			return true, nil
		}
		// if both streams finished the we have finished
		if err1 == io.EOF && err2 == io.EOF {
			break
		}
	}
	return false, nil
}

// CheckIdenticalDownload checks to see if dst and src are identical
// by reading all their bytes if necessary.
//
// it returns true if differences were found
func CheckIdenticalDownload(ctx context.Context, dst, src fs.Object) (differ bool, err error) {
	err = Retry(src, fs.Config.LowLevelRetries, func() error {
		differ, err = checkIdenticalDownload(ctx, dst, src)
		return err
	})
	return differ, err
}

// Does the work for CheckIdenticalDownload
func checkIdenticalDownload(ctx context.Context, dst, src fs.Object) (differ bool, err error) {
	in1, err := dst.Open(ctx)
	if err != nil {
		return true, errors.Wrapf(err, "failed to open %q", dst)
	}
	tr1 := accounting.Stats(ctx).NewTransfer(dst)
	defer func() {
		tr1.Done(nil) // error handling is done by the caller
	}()
	in1 = tr1.Account(ctx, in1).WithBuffer() // account and buffer the transfer

	in2, err := src.Open(ctx)
	if err != nil {
		return true, errors.Wrapf(err, "failed to open %q", src)
	}
	tr2 := accounting.Stats(ctx).NewTransfer(dst)
	defer func() {
		tr2.Done(nil) // error handling is done by the caller
	}()
	in2 = tr2.Account(ctx, in2).WithBuffer() // account and buffer the transfer

	// To assign err variable before defer.
	differ, err = CheckEqualReaders(in1, in2)
	return
}

// CheckDownload checks the files in fsrc and fdst according to Size
// and the actual contents of the files.
func CheckDownload(ctx context.Context, opt *CheckOpt) error {
	optCopy := *opt
	optCopy.Check = func(ctx context.Context, a, b fs.Object) (differ bool, noHash bool, err error) {
		differ, err = CheckIdenticalDownload(ctx, a, b)
		if err != nil {
			return true, true, errors.Wrap(err, "failed to download")
		}
		return differ, false, nil
	}
	return CheckFn(ctx, &optCopy)
}
