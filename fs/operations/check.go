package operations

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/march"
	"github.com/rclone/rclone/lib/readers"
	"golang.org/x/text/unicode/norm"
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
	differences     atomic.Int32
	noHashes        atomic.Int32
	srcFilesMissing atomic.Int32
	dstFilesMissing atomic.Int32
	matches         atomic.Int32
	opt             CheckOpt
}

// report outputs the fileName to out if required and to the combined log
func (c *checkMarch) report(o fs.DirEntry, out io.Writer, sigil rune) {
	c.reportFilename(o.String(), out, sigil)
}

func (c *checkMarch) reportFilename(filename string, out io.Writer, sigil rune) {
	if out != nil {
		SyncFprintf(out, "%s\n", filename)
	}
	if c.opt.Combined != nil {
		SyncFprintf(c.opt.Combined, "%c %s\n", sigil, filename)
	}
}

// DstOnly have an object which is in the destination only
func (c *checkMarch) DstOnly(dst fs.DirEntry) (recurse bool) {
	switch dst.(type) {
	case fs.Object:
		if c.opt.OneWay {
			return false
		}
		err := fmt.Errorf("file not in %v", c.opt.Fsrc)
		fs.Errorf(dst, "%v", err)
		_ = fs.CountError(err)
		c.differences.Add(1)
		c.srcFilesMissing.Add(1)
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
		err := fmt.Errorf("file not in %v", c.opt.Fdst)
		fs.Errorf(src, "%v", err)
		_ = fs.CountError(err)
		c.differences.Add(1)
		c.dstFilesMissing.Add(1)
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
	ci := fs.GetConfig(ctx)
	tr := accounting.Stats(ctx).NewCheckingTransfer(src, "checking")
	defer func() {
		tr.Done(ctx, err)
	}()
	if sizeDiffers(ctx, src, dst) {
		err = fmt.Errorf("sizes differ")
		fs.Errorf(src, "%v", err)
		return true, false, nil
	}
	if ci.SizeOnly {
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
					c.differences.Add(1)
					err := errors.New("files differ")
					// the checkFn has already logged the reason
					_ = fs.CountError(err)
					c.report(src, c.opt.Differ, '*')
				} else {
					c.matches.Add(1)
					c.report(src, c.opt.Match, '=')
					if noHash {
						c.noHashes.Add(1)
						fs.Debugf(dstX, "OK - could not check hash")
					} else {
						fs.Debugf(dstX, "OK")
					}
				}
			}()
		} else {
			err := fmt.Errorf("is file on %v but directory on %v", c.opt.Fsrc, c.opt.Fdst)
			fs.Errorf(src, "%v", err)
			_ = fs.CountError(err)
			c.differences.Add(1)
			c.dstFilesMissing.Add(1)
			c.report(src, c.opt.MissingOnDst, '+')
		}
	case fs.Directory:
		// Do the same thing to the entire contents of the directory
		_, ok := dst.(fs.Directory)
		if ok {
			return true
		}
		err := fmt.Errorf("is file on %v but directory on %v", c.opt.Fdst, c.opt.Fsrc)
		fs.Errorf(dst, "%v", err)
		_ = fs.CountError(err)
		c.differences.Add(1)
		c.srcFilesMissing.Add(1)
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
	ci := fs.GetConfig(ctx)
	if opt.Check == nil {
		return errors.New("internal error: nil check function")
	}
	c := &checkMarch{
		tokens: make(chan struct{}, ci.Checkers),
		opt:    *opt,
	}

	// set up a march over fdst and fsrc
	m := &march.March{
		Ctx:                    ctx,
		Fdst:                   c.opt.Fdst,
		Fsrc:                   c.opt.Fsrc,
		Dir:                    "",
		Callback:               c,
		NoTraverse:             ci.NoTraverse,
		NoUnicodeNormalization: ci.NoUnicodeNormalization,
	}
	fs.Debugf(c.opt.Fdst, "Waiting for checks to finish")
	err := m.Run(ctx)
	c.wg.Wait() // wait for background go-routines

	return c.reportResults(ctx, err)
}

func (c *checkMarch) reportResults(ctx context.Context, err error) error {
	if c.dstFilesMissing.Load() > 0 {
		fs.Logf(c.opt.Fdst, "%d files missing", c.dstFilesMissing.Load())
	}
	if c.srcFilesMissing.Load() > 0 {
		entity := "files"
		if c.opt.Fsrc == nil {
			entity = "hashes"
		}
		fs.Logf(c.opt.Fsrc, "%d %s missing", c.srcFilesMissing.Load(), entity)
	}

	fs.Logf(c.opt.Fdst, "%d differences found", accounting.Stats(ctx).GetErrors())
	if errs := accounting.Stats(ctx).GetErrors(); errs > 0 {
		fs.Logf(c.opt.Fdst, "%d errors while checking", errs)
	}
	if c.noHashes.Load() > 0 {
		fs.Logf(c.opt.Fdst, "%d hashes could not be checked", c.noHashes.Load())
	}
	if c.matches.Load() > 0 {
		fs.Logf(c.opt.Fdst, "%d matching files", c.matches.Load())
	}
	if err != nil {
		return err
	}
	if c.differences.Load() > 0 {
		// Return an already counted error so we don't double count this error too
		err = fserrors.FsError(fmt.Errorf("%d differences found", c.differences.Load()))
		fserrors.Count(err)
		return err
	}
	return nil
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
			err = fmt.Errorf("%v differ", ht)
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
	ci := fs.GetConfig(ctx)
	err = Retry(ctx, src, ci.LowLevelRetries, func() error {
		differ, err = checkIdenticalDownload(ctx, dst, src)
		return err
	})
	return differ, err
}

// Does the work for CheckIdenticalDownload
func checkIdenticalDownload(ctx context.Context, dst, src fs.Object) (differ bool, err error) {
	var in1, in2 io.ReadCloser
	in1, err = Open(ctx, dst)
	if err != nil {
		return true, fmt.Errorf("failed to open %q: %w", dst, err)
	}
	tr1 := accounting.Stats(ctx).NewTransfer(dst, nil)
	defer func() {
		tr1.Done(ctx, nil) // error handling is done by the caller
	}()
	in1 = tr1.Account(ctx, in1).WithBuffer() // account and buffer the transfer

	in2, err = Open(ctx, src)
	if err != nil {
		return true, fmt.Errorf("failed to open %q: %w", src, err)
	}
	tr2 := accounting.Stats(ctx).NewTransfer(dst, nil)
	defer func() {
		tr2.Done(ctx, nil) // error handling is done by the caller
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
			return true, true, fmt.Errorf("failed to download: %w", err)
		}
		return differ, false, nil
	}
	return CheckFn(ctx, &optCopy)
}

// ApplyTransforms handles --no-unicode-normalization and --ignore-case-sync for CheckSum
// so that it matches behavior of Check (where it's handled by March)
func ApplyTransforms(ctx context.Context, s string) string {
	ci := fs.GetConfig(ctx)
	if !ci.NoUnicodeNormalization {
		s = norm.NFC.String(s)
	}
	if ci.IgnoreCaseSync {
		s = strings.ToLower(s)
	}
	return s
}

// CheckSum checks filesystem hashes against a SUM file
func CheckSum(ctx context.Context, fsrc, fsum fs.Fs, sumFile string, hashType hash.Type, opt *CheckOpt, download bool) error {
	var options CheckOpt
	if opt != nil {
		options = *opt
	} else {
		// default options for hashsum -c
		options.Combined = os.Stdout
	}
	// CheckSum treats Fsrc and Fdst specially:
	options.Fsrc = nil  // no file system here, corresponds to the sum list
	options.Fdst = fsrc // denotes the file system to check
	opt = &options      // override supplied argument

	if !download && (hashType == hash.None || !opt.Fdst.Hashes().Contains(hashType)) {
		return fmt.Errorf("%s: hash type is not supported by file system: %s", hashType, opt.Fdst)
	}

	if sumFile == "" {
		return fmt.Errorf("not a sum file: %s", fsum)
	}
	sumObj, err := fsum.NewObject(ctx, sumFile)
	if err != nil {
		return fmt.Errorf("cannot open sum file: %w", err)
	}
	hashes, err := ParseSumFile(ctx, sumObj)
	if err != nil {
		return fmt.Errorf("failed to parse sum file: %w", err)
	}

	ci := fs.GetConfig(ctx)
	c := &checkMarch{
		tokens: make(chan struct{}, ci.Checkers),
		opt:    *opt,
	}
	lastErr := ListFn(ctx, opt.Fdst, func(obj fs.Object) {
		c.checkSum(ctx, obj, download, hashes, hashType)
	})
	c.wg.Wait() // wait for background go-routines

	// make census of unhandled sums
	fi := filter.GetConfig(ctx)
	for filename, hash := range hashes {
		if hash == "" { // the sum has been successfully consumed
			continue
		}
		if !fi.IncludeRemote(filename) { // the file was filtered out
			continue
		}
		// filesystem missed the file, sum wasn't consumed
		err := fmt.Errorf("file not in %v", opt.Fdst)
		fs.Errorf(filename, "%v", err)
		_ = fs.CountError(err)
		if lastErr == nil {
			lastErr = err
		}
		c.dstFilesMissing.Add(1)
		c.reportFilename(filename, opt.MissingOnDst, '+')
	}

	return c.reportResults(ctx, lastErr)
}

// checkSum checks single object against golden hashes
func (c *checkMarch) checkSum(ctx context.Context, obj fs.Object, download bool, hashes HashSums, hashType hash.Type) {
	normalizedRemote := ApplyTransforms(ctx, obj.Remote())
	c.ioMu.Lock()
	sumHash, sumFound := hashes[normalizedRemote]
	hashes[normalizedRemote] = "" // mark sum as consumed
	c.ioMu.Unlock()

	if !sumFound && c.opt.OneWay {
		return
	}

	var err error
	tr := accounting.Stats(ctx).NewCheckingTransfer(obj, "hashing")
	defer tr.Done(ctx, err)

	if !sumFound {
		err = errors.New("sum not found")
		_ = fs.CountError(err)
		fs.Errorf(obj, "%v", err)
		c.differences.Add(1)
		c.srcFilesMissing.Add(1)
		c.report(obj, c.opt.MissingOnSrc, '-')
		return
	}

	if !download {
		var objHash string
		objHash, err = obj.Hash(ctx, hashType)
		c.matchSum(ctx, sumHash, objHash, obj, err, hashType)
		return
	}

	c.wg.Add(1)
	c.tokens <- struct{}{} // put a token to limit concurrency
	go func() {
		var (
			objHash string
			err     error
			in      io.ReadCloser
		)
		defer func() {
			c.matchSum(ctx, sumHash, objHash, obj, err, hashType)
			<-c.tokens // get the token back to free up a slot
			c.wg.Done()
		}()
		if in, err = Open(ctx, obj); err != nil {
			return
		}
		tr := accounting.Stats(ctx).NewTransfer(obj, nil)
		in = tr.Account(ctx, in).WithBuffer() // account and buffer the transfer
		defer func() {
			tr.Done(ctx, nil) // will close the stream
		}()
		hashVals, err2 := hash.StreamTypes(in, hash.NewHashSet(hashType))
		if err2 != nil {
			err = err2 // pass to matchSum
			return
		}
		objHash = hashVals[hashType]
	}()
}

// matchSum sums up the results of hashsum matching for an object
func (c *checkMarch) matchSum(ctx context.Context, sumHash, objHash string, obj fs.Object, err error, hashType hash.Type) {
	switch {
	case err != nil:
		_ = fs.CountError(err)
		fs.Errorf(obj, "Failed to calculate hash: %v", err)
		c.report(obj, c.opt.Error, '!')
	case sumHash == "":
		err = errors.New("duplicate file")
		_ = fs.CountError(err)
		fs.Errorf(obj, "%v", err)
		c.report(obj, c.opt.Error, '!')
	case objHash == "":
		fs.Debugf(nil, "%v = %s (sum)", hashType, sumHash)
		fs.Debugf(obj, "%v - could not check hash (%v)", hashType, c.opt.Fdst)
		c.noHashes.Add(1)
		c.matches.Add(1)
		c.report(obj, c.opt.Match, '=')
	case objHash == sumHash:
		fs.Debugf(obj, "%v = %s OK", hashType, sumHash)
		c.matches.Add(1)
		c.report(obj, c.opt.Match, '=')
	default:
		err = errors.New("files differ")
		_ = fs.CountError(err)
		fs.Debugf(nil, "%v = %s (sum)", hashType, sumHash)
		fs.Debugf(obj, "%v = %s (%v)", hashType, objHash, c.opt.Fdst)
		fs.Errorf(obj, "%v", err)
		c.differences.Add(1)
		c.report(obj, c.opt.Differ, '*')
	}
}

// HashSums represents a parsed SUM file
type HashSums map[string]string

// ParseSumFile parses a hash SUM file and returns hashes as a map
func ParseSumFile(ctx context.Context, sumFile fs.Object) (HashSums, error) {
	rd, err := Open(ctx, sumFile)
	if err != nil {
		return nil, err
	}
	parser := bufio.NewReader(rd)

	const maxWarn = 3
	numWarn := 0

	re := regexp.MustCompile(`^([^ ]+) [ *](.+)$`)
	hashes := HashSums{}
	for lineNo := 0; true; lineNo++ {
		lineBytes, _, err := parser.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		line := string(lineBytes)
		if line == "" {
			continue
		}

		fields := re.FindStringSubmatch(ApplyTransforms(ctx, line))
		if fields == nil {
			numWarn++
			if numWarn <= maxWarn {
				fs.Logf(sumFile, "improperly formatted checksum line %d", lineNo)
			}
			continue
		}

		sum, file := fields[1], fields[2]
		if hashes[file] != "" {
			numWarn++
			if numWarn <= maxWarn {
				fs.Logf(sumFile, "duplicate file on checksum line %d", lineNo)
			}
			continue
		}

		// We've standardised on lower case checksums in rclone internals.
		hashes[file] = strings.ToLower(sum)
	}

	if numWarn > maxWarn {
		fs.Logf(sumFile, "%d warning(s) suppressed...", numWarn-maxWarn)
	}
	if err = rd.Close(); err != nil {
		return nil, err
	}
	return hashes, nil
}
