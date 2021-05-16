// TestBisync is a test engine for bisync test cases.
// See https://rclone.org/bisync/#testing for documentation.
// Test cases are organized in subdirs beneath ./testdata
// Results are compared against golden listings and log file.
package bisync_test

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/cmd/bisync"
	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/sync"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/random"

	"github.com/pkg/errors"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/rclone/rclone/backend/all" // for integration tests
)

const (
	touchDateFormat = "2006-01-02"
	goldenCanonBase = "_testdir_"
	logFileName     = "test.log"
	dropMe          = "*** [DROP THIS LINE] ***"
	eol             = "\n"
	slash           = string(os.PathSeparator)
	fixSlash        = (runtime.GOOS == "windows")
)

// logReplacements make modern test logs comparable with golden dir.
// It is a string slice of even length with this structure:
//    {`matching regular expression`, "mangled result string", ...}
var logReplacements = []string{
	// skip syslog facility markers
	`^(<[1-9]>)(INFO  |ERROR |NOTICE|DEBUG ):(.*)$`, "$2:$3",
	// skip log prefixes
	`^\d+/\d\d/\d\d \d\d:\d\d:\d\d(?:\.\d{6})? `, "",
	// ignore rclone info messages
	`^INFO  : .*?: (Deleted|Copied |Moved |Updated ).*$`, dropMe,
	`^NOTICE: .*?: Replacing invalid UTF-8 characters in "[^"]*"$`, dropMe,
	// ignore rclone debug messages
	`^DEBUG : .*$`, dropMe,
	// ignore dropbox info messages
	`^NOTICE: too_many_(requests|write_operations)/\.*: Too many requests or write operations.*$`, dropMe,
	`^NOTICE: Dropbox root .*?: Forced to upload files to set modification times on this backend.$`, dropMe,
	`^INFO  : .*?: src and dst identical but can't set mod time without deleting and re-uploading$`, dropMe,
}

// Some dry-run messages differ depending on the particular remote.
var dryrunReplacements = []string{
	`^(NOTICE: file5.txt: Skipped) (copy|update modification time) (as --dry-run is set [(]size \d+[)])$`,
	`$1 copy (or update modification time) $3`,
}

// Some groups of log lines may appear unordered because rclone applies
// many operations in parallel to boost performance.
var logHoppers = []string{
	// Test case `dry-run` produced log mismatches due to non-deterministic
	// order of captured dry-run info messages.
	`NOTICE: \S+?: Skipped (?:copy|move|delete|copy \(or [^)]+\)|update modification time) as --dry-run is set \(size \d+\)`,

	// Test case `extended-filenames` detected difference in order of files
	// with extended unicode names between Windows and Unix or GDrive,
	// but the order is in fact not important for success.
	`(?:INFO  |NOTICE): - Path[12] +File (?:was deleted|is new|is newer|is OLDER) +- .*`,

	// Test case `check-access-filters` detected listing miscompares due
	// to indeterminate order of rclone operations in presence of multiple
	// subdirectories. The order inconsistency initially showed up in the
	// listings and triggered reordering of log messages, but the actual
	// files will in fact match.
	`ERROR : - +Access test failed: Path[12] file not found in Path[12] - .*`,

	// Test case `resync` suffered from the order of queued copies.
	`(?:INFO  |NOTICE): - Path2    Resync will copy to Path1 +- .*`,
}

// Some log lines can contain Windows path separator that must be
// converted to "/" in every matching token to match golden logs.
var logLinesWithSlash = []string{
	`\(\d\d\)  : (touch-glob|touch-copy|copy-file|copy-as|copy-dir|delete-file) `,
	`INFO  : - Path[12] +Queue copy to Path[12] `,
	`INFO  : Synching Path1 .*? with Path2 `,
	`INFO  : Validating listings for `,
}
var regexFixSlash = regexp.MustCompile("^(" + strings.Join(logLinesWithSlash, "|") + ")")

// Command line flags for bisync test
var (
	argTestCase  = flag.String("case", "", "Bisync test case to run")
	argRemote2   = flag.String("remote2", "", "Path2 for bisync tests")
	argNoCompare = flag.Bool("no-compare", false, "Do not compare test results with golden")
	argNoCleanup = flag.Bool("no-cleanup", false, "Keep test files")
	argGolden    = flag.Bool("golden", false, "Store results as golden")
	argDebug     = flag.Bool("debug", false, "Print debug messages")
	argStopAt    = flag.Int("stop-at", 0, "Stop after given test step")
	// Flag -refresh-times helps with Dropbox tests failing with message
	// "src and dst identical but can't set mod time without deleting and re-uploading"
	argRefreshTimes = flag.Bool("refresh-times", false, "Force refreshing the target modtime, useful for Dropbox (default: false)")
)

// bisyncTest keeps all test data in a single place
type bisyncTest struct {
	// per-test state
	t           *testing.T
	step        int
	stopped     bool
	stepStr     string
	testCase    string
	sessionName string
	// test dirs
	testDir    string
	dataDir    string
	initDir    string
	goldenDir  string
	workDir    string
	fs1        fs.Fs
	path1      string
	canonPath1 string
	fs2        fs.Fs
	path2      string
	canonPath2 string
	// test log
	logDir  string
	logPath string
	logFile *os.File
	// global state
	dataRoot string
	randName string
	tempDir  string
	parent1  fs.Fs
	parent2  fs.Fs
	// global flags
	argRemote1 string
	argRemote2 string
	noCompare  bool
	noCleanup  bool
	golden     bool
	debug      bool
	stopAt     int
}

// TestBisync is a test engine for bisync test cases.
func TestBisync(t *testing.T) {
	ctx := context.Background()
	fstest.Initialise()

	ci := fs.GetConfig(ctx)
	ciSave := *ci
	defer func() {
		*ci = ciSave
	}()
	if *argRefreshTimes {
		ci.RefreshTimes = true
	}

	baseDir, err := os.Getwd()
	require.NoError(t, err, "get current directory")
	randName := "bisync." + time.Now().Format("150405-") + random.String(5)
	tempDir := filepath.Join(os.TempDir(), randName)
	workDir := filepath.Join(tempDir, "workdir")

	b := &bisyncTest{
		// per-test state
		t: t,
		// global state
		tempDir:  tempDir,
		randName: randName,
		workDir:  workDir,
		dataRoot: filepath.Join(baseDir, "testdata"),
		logDir:   filepath.Join(tempDir, "logs"),
		logPath:  filepath.Join(workDir, logFileName),
		// global flags
		argRemote1: *fstest.RemoteName,
		argRemote2: *argRemote2,
		noCompare:  *argNoCompare,
		noCleanup:  *argNoCleanup,
		golden:     *argGolden,
		debug:      *argDebug,
		stopAt:     *argStopAt,
	}

	b.mkdir(b.tempDir)
	b.mkdir(b.logDir)

	fnHandle := atexit.Register(func() {
		if atexit.Signalled() {
			b.cleanupAll()
		}
	})
	defer func() {
		b.cleanupAll()
		atexit.Unregister(fnHandle)
	}()

	argCase := *argTestCase
	if argCase == "" {
		argCase = "all"
		if testing.Short() {
			// remote tests can be long, help with "go test -short"
			argCase = "basic"
		}
	}

	testList := strings.Split(argCase, ",")
	if strings.ToLower(argCase) == "all" {
		testList = nil
		for _, testCase := range b.listDir(b.dataRoot) {
			if strings.HasPrefix(testCase, "test_") {
				testList = append(testList, testCase)
			}
		}
	}
	require.False(t, b.stopAt > 0 && len(testList) > 1, "-stop-at is meaningful only for a single test")

	for _, testCase := range testList {
		testCase = strings.ReplaceAll(testCase, "-", "_")
		testCase = strings.TrimPrefix(testCase, "test_")
		t.Run(testCase, func(childTest *testing.T) {
			b.runTestCase(ctx, childTest, testCase)
		})
	}
}

func (b *bisyncTest) cleanupAll() {
	if b.noCleanup {
		return
	}
	ctx := context.Background()
	if b.parent1 != nil {
		_ = operations.Purge(ctx, b.parent1, "")
	}
	if b.parent2 != nil {
		_ = operations.Purge(ctx, b.parent2, "")
	}
	_ = os.RemoveAll(b.tempDir)
}

func (b *bisyncTest) runTestCase(ctx context.Context, t *testing.T, testCase string) {
	b.t = t
	b.testCase = testCase
	var err error

	b.fs1, b.parent1, b.path1, b.canonPath1 = b.makeTempRemote(ctx, b.argRemote1, "path1")
	b.fs2, b.parent2, b.path2, b.canonPath2 = b.makeTempRemote(ctx, b.argRemote2, "path2")

	b.sessionName = bilib.SessionName(b.fs1, b.fs2)
	b.testDir = b.ensureDir(b.dataRoot, "test_"+b.testCase, false)
	b.initDir = b.ensureDir(b.testDir, "initial", false)
	b.goldenDir = b.ensureDir(b.testDir, "golden", false)
	b.dataDir = b.ensureDir(b.testDir, "modfiles", true) // optional

	// For test stability, jam initial dates to a fixed past date.
	// Test cases that change files will touch specific files to fixed new dates.
	initDate := time.Date(2000, time.January, 1, 0, 0, 0, 0, bisync.TZ)
	err = filepath.Walk(b.initDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			return os.Chtimes(path, initDate, initDate)
		}
		return err
	})
	require.NoError(b.t, err, "jamming initial dates")

	// Prepare initial content
	b.cleanupCase(ctx)
	initFs, err := fs.NewFs(ctx, b.initDir)
	require.NoError(b.t, err)
	require.NoError(b.t, sync.CopyDir(ctx, b.fs1, initFs, true), "setting up path1")
	require.NoError(b.t, sync.CopyDir(ctx, b.fs2, initFs, true), "setting up path2")

	// Create log file
	b.mkdir(b.workDir)
	b.logFile, err = os.Create(b.logPath)
	require.NoError(b.t, err, "creating log file")

	// Execute test scenario
	scenFile := filepath.Join(b.testDir, "scenario.txt")
	scenBuf, err := ioutil.ReadFile(scenFile)
	scenReplacer := b.newReplacer(false)
	require.NoError(b.t, err)
	b.step = 0
	b.stopped = false
	for _, line := range strings.Split(string(scenBuf), "\n") {
		comment := strings.Index(line, "#")
		if comment != -1 {
			line = line[:comment]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			if b.golden {
				// Keep empty lines in golden logs
				_, _ = b.logFile.WriteString("\n")
			}
			continue
		}

		b.step++
		b.stepStr = fmt.Sprintf("(%02d)  :", b.step)
		line = scenReplacer.Replace(line)
		if err = b.runTestStep(ctx, line); err != nil {
			require.Failf(b.t, "test step failed", "step %d failed: %v", b.step, err)
			return
		}
		if b.stopAt > 0 && b.step >= b.stopAt {
			comment := ""
			if b.golden {
				comment = " (ignoring -golden)"
			}
			b.logPrintf("Stopping after step %d%s", b.step, comment)
			b.stopped = true
			b.noCleanup = true
			b.noCompare = true
			break
		}
	}

	// Perform post-run activities
	require.NoError(b.t, b.logFile.Close(), "flushing test log")
	b.logFile = nil

	savedLog := b.testCase + ".log"
	err = bilib.CopyFile(b.logPath, filepath.Join(b.logDir, savedLog))
	require.NoError(b.t, err, "saving log file %s", savedLog)

	if b.golden && !b.stopped {
		log.Printf("Store results to golden directory")
		b.storeGolden()
		return
	}

	errorCount := 0
	if b.noCompare {
		log.Printf("Skip comparing results with golden directory")
		errorCount = -2
	} else {
		errorCount = b.compareResults()
	}

	if b.noCleanup {
		log.Printf("Skip cleanup")
	} else {
		b.cleanupCase(ctx)
	}

	var msg string
	var passed bool
	switch errorCount {
	case 0:
		msg = fmt.Sprintf("TEST %s PASSED", b.testCase)
		passed = true
	case -2:
		msg = fmt.Sprintf("TEST %s SKIPPED", b.testCase)
		passed = true
	case -1:
		msg = fmt.Sprintf("TEST %s FAILED - WRONG NUMBER OF FILES", b.testCase)
		passed = false
	default:
		msg = fmt.Sprintf("TEST %s FAILED - %d MISCOMPARED FILES", b.testCase, errorCount)
		buckets := b.fs1.Features().BucketBased || b.fs2.Features().BucketBased
		passed = false
		if b.testCase == "rmdirs" && buckets {
			msg += " (expected failure on bucket remotes)"
			passed = true
		}
	}
	b.t.Log(msg)
	if !passed {
		b.t.FailNow()
	}
}

// makeTempRemote creates temporary folder and makes a filesystem
// if a local path is provided, it's ignored (the test will run under system temp)
func (b *bisyncTest) makeTempRemote(ctx context.Context, remote, subdir string) (f, parent fs.Fs, path, canon string) {
	var err error
	if bilib.IsLocalPath(remote) {
		if remote != "" && remote != "local" {
			b.t.Fatalf(`Missing ":" in remote %q. Use "local" to test with local filesystem.`, remote)
		}
		parent, err = fs.NewFs(ctx, b.tempDir)
		require.NoError(b.t, err, "parsing %s", b.tempDir)

		path = filepath.Join(b.tempDir, b.testCase)
		canon = bilib.CanonicalPath(path) + "_"
		path = filepath.Join(path, subdir)
	} else {
		last := remote[len(remote)-1]
		if last != ':' && last != '/' {
			remote += "/"
		}
		remote += b.randName
		parent, err = fs.NewFs(ctx, remote)
		require.NoError(b.t, err, "parsing %s", remote)

		path = remote + "/" + b.testCase
		canon = bilib.CanonicalPath(path) + "_"
		path += "/" + subdir
	}

	f, err = fs.NewFs(ctx, path)
	require.NoError(b.t, err, "parsing %s/%s", remote, subdir)
	path = bilib.FsPath(f) // Make it canonical

	if f.Precision() == fs.ModTimeNotSupported {
		b.t.Skipf("modification time support is missing on %s", subdir)
	}
	return
}

func (b *bisyncTest) cleanupCase(ctx context.Context) {
	// Silence "directory not found" errors from the ftp backend
	_ = bilib.CaptureOutput(func() {
		_ = operations.Purge(ctx, b.fs1, "")
	})
	_ = bilib.CaptureOutput(func() {
		_ = operations.Purge(ctx, b.fs2, "")
	})
	_ = os.RemoveAll(b.workDir)
	accounting.Stats(ctx).ResetCounters()
}

func (b *bisyncTest) runTestStep(ctx context.Context, line string) (err error) {
	var fsrc, fdst fs.Fs
	accounting.Stats(ctx).ResetErrors()
	b.logPrintf("%s %s", b.stepStr, line)

	ci := fs.GetConfig(ctx)
	ciSave := *ci
	defer func() {
		*ci = ciSave
	}()
	ci.LogLevel = fs.LogLevelInfo
	if b.debug {
		ci.LogLevel = fs.LogLevelDebug
	}

	args := splitLine(line)
	switch args[0] {
	case "test":
		b.checkArgs(args, 1, 0)
		return nil
	case "copy-listings":
		b.checkArgs(args, 1, 1)
		return b.saveTestListings(args[1], true)
	case "move-listings":
		b.checkArgs(args, 1, 1)
		return b.saveTestListings(args[1], false)
	case "purge-children":
		b.checkArgs(args, 1, 1)
		if fsrc, err = fs.NewFs(ctx, args[1]); err != nil {
			return err
		}
		return purgeChildren(ctx, fsrc, "")
	case "delete-file":
		b.checkArgs(args, 1, 1)
		dir, file := filepath.Split(args[1])
		if fsrc, err = fs.NewFs(ctx, dir); err != nil {
			return err
		}
		var obj fs.Object
		if obj, err = fsrc.NewObject(ctx, file); err != nil {
			return err
		}
		return operations.DeleteFile(ctx, obj)
	case "delete-glob":
		b.checkArgs(args, 2, 2)
		if fsrc, err = fs.NewFs(ctx, args[1]); err != nil {
			return err
		}
		return deleteFiles(ctx, fsrc, args[2])
	case "touch-glob":
		b.checkArgs(args, 3, 3)
		date, src, glob := args[1], args[2], args[3]
		if fsrc, err = fs.NewFs(ctx, src); err != nil {
			return err
		}
		_, err = touchFiles(ctx, date, fsrc, src, glob)
		return err
	case "touch-copy":
		b.checkArgs(args, 3, 3)
		date, src, dst := args[1], args[2], args[3]
		dir, file := filepath.Split(src)
		if fsrc, err = fs.NewFs(ctx, dir); err != nil {
			return err
		}
		if _, err = touchFiles(ctx, date, fsrc, dir, file); err != nil {
			return err
		}
		return b.copyFile(ctx, src, dst, "")
	case "copy-file":
		b.checkArgs(args, 2, 2)
		return b.copyFile(ctx, args[1], args[2], "")
	case "copy-as":
		b.checkArgs(args, 3, 3)
		return b.copyFile(ctx, args[1], args[2], args[3])
	case "copy-dir", "sync-dir":
		b.checkArgs(args, 2, 2)
		if fsrc, err = cache.Get(ctx, args[1]); err != nil {
			return err
		}
		if fdst, err = cache.Get(ctx, args[2]); err != nil {
			return err
		}
		switch args[0] {
		case "copy-dir":
			err = sync.CopyDir(ctx, fdst, fsrc, true)
		case "sync-dir":
			err = sync.Sync(ctx, fdst, fsrc, true)
		}
		return err
	case "list-dirs":
		b.checkArgs(args, 1, 1)
		return b.listSubdirs(ctx, args[1])
	case "bisync":
		return b.runBisync(ctx, args[1:])
	default:
		return errors.Errorf("unknown command: %q", args[0])
	}
}

// splitLine splits scenario line into tokens and performs
// substitutions that involve whitespace or control chars.
func splitLine(line string) (args []string) {
	for _, s := range strings.Fields(line) {
		b := []byte(whitespaceReplacer.Replace(s))
		b = regexChar.ReplaceAllFunc(b, func(b []byte) []byte {
			c, _ := strconv.ParseUint(string(b[5:7]), 16, 8)
			return []byte{byte(c)}
		})
		args = append(args, string(b))
	}
	return
}

var whitespaceReplacer = strings.NewReplacer(
	"{spc}", " ",
	"{tab}", "\t",
	"{eol}", eol,
)
var regexChar = regexp.MustCompile(`\{chr:([0-9a-f]{2})\}`)

// checkArgs verifies the number of the test command arguments
func (b *bisyncTest) checkArgs(args []string, min, max int) {
	cmd := args[0]
	num := len(args) - 1
	if min == max && num != min {
		b.t.Fatalf("%q must have strictly %d args", cmd, min)
	}
	if min > 0 && num < min {
		b.t.Fatalf("%q must have at least %d args", cmd, min)
	}
	if max > 0 && num > max {
		b.t.Fatalf("%q must have at most %d args", cmd, max)
	}
}

func (b *bisyncTest) runBisync(ctx context.Context, args []string) (err error) {
	opt := &bisync.Options{
		Workdir:       b.workDir,
		NoCleanup:     true,
		SaveQueues:    true,
		MaxDelete:     bisync.DefaultMaxDelete,
		CheckFilename: bisync.DefaultCheckFilename,
		CheckSync:     bisync.CheckSyncTrue,
	}
	octx, ci := fs.AddConfig(ctx)
	fs1, fs2 := b.fs1, b.fs2

	addSubdir := func(path, subdir string) fs.Fs {
		remote := path + subdir
		f, err := fs.NewFs(ctx, remote)
		require.NoError(b.t, err, "parsing remote %q", remote)
		return f
	}

	for _, arg := range args {
		val := ""
		pos := strings.Index(arg, "=")
		if pos > 0 {
			arg, val = arg[:pos], arg[pos+1:]
		}
		switch arg {
		case "resync":
			opt.Resync = true
		case "dry-run":
			ci.DryRun = true
			opt.DryRun = true
		case "force":
			opt.Force = true
		case "remove-empty-dirs":
			opt.RemoveEmptyDirs = true
		case "check-sync-only":
			opt.CheckSync = bisync.CheckSyncOnly
		case "no-check-sync":
			opt.CheckSync = bisync.CheckSyncFalse
		case "check-access":
			opt.CheckAccess = true
		case "check-filename":
			opt.CheckFilename = val
		case "filters-file":
			opt.FiltersFile = val
		case "max-delete":
			opt.MaxDelete, err = strconv.Atoi(val)
			require.NoError(b.t, err, "parsing max-delete=%q", val)
		case "size-only":
			ci.SizeOnly = true
		case "subdir":
			fs1 = addSubdir(b.path1, val)
			fs2 = addSubdir(b.path2, val)
		default:
			return errors.Errorf("invalid bisync option %q", arg)
		}
	}

	output := bilib.CaptureOutput(func() {
		err = bisync.Bisync(octx, fs1, fs2, opt)
	})

	_, _ = os.Stdout.Write(output)
	_, _ = b.logFile.Write(output)

	if err != nil {
		b.logPrintf("Bisync error: %v", err)
	}
	return nil
}

// saveTestListings creates a copy of test artifacts with given prefix
// including listings (.lst*), queues (.que) and filters (.flt, .flt.md5)
func (b *bisyncTest) saveTestListings(prefix string, keepSource bool) (err error) {
	count := 0
	for _, srcFile := range b.listDir(b.workDir) {
		switch fileType(srcFile) {
		case "listing", "queue", "filters":
			// fall thru
		default:
			continue
		}
		count++
		dstFile := fmt.Sprintf("%s.%s.sav", prefix, b.toGolden(srcFile))
		src := filepath.Join(b.workDir, srcFile)
		dst := filepath.Join(b.workDir, dstFile)
		if err = bilib.CopyFile(src, dst); err != nil {
			return
		}
		if keepSource {
			continue
		}
		if err = os.Remove(src); err != nil {
			return
		}
	}
	if count == 0 {
		err = errors.New("listings not found")
	}
	return
}

func (b *bisyncTest) copyFile(ctx context.Context, src, dst, asName string) (err error) {
	var fsrc, fdst fs.Fs
	var srcPath, srcFile, dstPath, dstFile string

	switch fsrc, err = cache.Get(ctx, src); err {
	case fs.ErrorIsFile:
		// ok
	case nil:
		return errors.New("source must be a file")
	default:
		return err
	}

	if _, srcPath, err = fspath.SplitFs(src); err != nil {
		return err
	}
	srcFile = path.Base(srcPath)

	if dstPath, dstFile, err = fspath.Split(dst); err != nil {
		return err
	}
	if dstPath == "" {
		return errors.New("invalid destination")
	}
	if dstFile != "" {
		dstPath = dst // force directory
	}
	if fdst, err = cache.Get(ctx, dstPath); err != nil {
		return err
	}

	if asName != "" {
		dstFile = asName
	} else {
		dstFile = srcFile
	}

	fctx, fi := filter.AddConfig(ctx)
	if err := fi.AddFile(srcFile); err != nil {
		return err
	}
	return operations.CopyFile(fctx, fdst, fsrc, dstFile, srcFile)
}

// listSubdirs is equivalent to `rclone lsf -R --dirs-only`
func (b *bisyncTest) listSubdirs(ctx context.Context, remote string) error {
	f, err := fs.NewFs(ctx, remote)
	if err != nil {
		return err
	}
	opt := operations.ListJSONOpt{
		NoModTime:  true,
		NoMimeType: true,
		DirsOnly:   true,
		Recurse:    true,
	}
	fmt := operations.ListFormat{}
	fmt.SetDirSlash(true)
	fmt.AddPath()
	printItem := func(item *operations.ListJSONItem) error {
		b.logPrintf("%s", fmt.Format(item))
		return nil
	}
	return operations.ListJSON(ctx, f, "", &opt, printItem)
}

// purgeChildren deletes child files and purges subdirs under given path.
// Note: this cannot be done with filters.
func purgeChildren(ctx context.Context, f fs.Fs, dir string) error {
	entries, firstErr := f.List(ctx, dir)
	if firstErr != nil {
		return firstErr
	}
	for _, entry := range entries {
		var err error
		switch dirObj := entry.(type) {
		case fs.Object:
			fs.Debugf(dirObj, "Remove file")
			err = dirObj.Remove(ctx)
		case fs.Directory:
			fs.Debugf(dirObj, "Purge subdir")
			err = operations.Purge(ctx, f, dirObj.Remote())
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// deleteFiles deletes a group of files by the name pattern.
func deleteFiles(ctx context.Context, f fs.Fs, glob string) error {
	fctx, fi := filter.AddConfig(ctx)
	if err := fi.Add(true, glob); err != nil {
		return err
	}
	if err := fi.Add(false, "/**"); err != nil {
		return err
	}
	return operations.Delete(fctx, f)
}

// touchFiles sets modification time on a group of files.
// Returns names of touched files and/or error.
// Note: `rclone touch` can touch only single file, doesn't support filters.
func touchFiles(ctx context.Context, dateStr string, f fs.Fs, dir, glob string) ([]string, error) {
	files := []string{}

	date, err := time.ParseInLocation(touchDateFormat, dateStr, bisync.TZ)
	if err != nil {
		return files, errors.Wrapf(err, "invalid date %q", dateStr)
	}

	matcher, firstErr := filter.GlobToRegexp(glob, false)
	if firstErr != nil {
		return files, errors.Errorf("invalid glob %q", glob)
	}

	entries, firstErr := f.List(ctx, "")
	if firstErr != nil {
		return files, firstErr
	}

	for _, entry := range entries {
		obj, isFile := entry.(fs.Object)
		if !isFile {
			continue
		}
		remote := obj.Remote()
		if !matcher.MatchString(remote) {
			continue
		}
		files = append(files, dir+remote)

		fs.Debugf(obj, "Set modification time %s", dateStr)
		err := obj.SetModTime(ctx, date)
		if err == fs.ErrorCantSetModTimeWithoutDelete {
			// Workaround for dropbox, similar to --refresh-times
			err = nil
			buf := new(bytes.Buffer)
			size := obj.Size()
			if size > 0 {
				err = operations.Cat(ctx, f, buf, 0, size)
			}
			info := object.NewStaticObjectInfo(remote, date, size, true, nil, f)
			if err == nil {
				_ = obj.Remove(ctx)
				_, err = f.Put(ctx, buf, info)
			}
		}
		if firstErr == nil {
			firstErr = err
		}
	}

	return files, firstErr
}

// compareResults validates scenario results against golden dir
func (b *bisyncTest) compareResults() int {
	goldenFiles := b.listDir(b.goldenDir)
	resultFiles := b.listDir(b.workDir)

	// Adapt test file names to their golden counterparts
	renamed := false
	for _, fileName := range resultFiles {
		goldName := b.toGolden(fileName)
		if goldName != fileName {
			filePath := filepath.Join(b.workDir, fileName)
			goldPath := filepath.Join(b.workDir, goldName)
			require.NoError(b.t, os.Rename(filePath, goldPath))
			renamed = true
		}
	}
	if renamed {
		resultFiles = b.listDir(b.workDir)
	}

	goldenSet := bilib.ToNames(goldenFiles)
	resultSet := bilib.ToNames(resultFiles)
	goldenNum := len(goldenFiles)
	resultNum := len(resultFiles)
	errorCount := 0
	const divider = "----------------------------------------------------------"

	if goldenNum != resultNum {
		log.Print(divider)
		log.Printf("MISCOMPARE - Number of Golden and Results files do not match:")
		log.Printf("  Golden count: %d", goldenNum)
		log.Printf("  Result count: %d", resultNum)
		log.Printf("  Golden files: %s", strings.Join(goldenFiles, ", "))
		log.Printf("  Result files: %s", strings.Join(resultFiles, ", "))
	}

	for _, file := range goldenFiles {
		if !resultSet.Has(file) {
			errorCount++
			log.Printf("  File found in Golden but not in Results:  %s", file)
		}
	}
	for _, file := range resultFiles {
		if !goldenSet.Has(file) {
			errorCount++
			log.Printf("  File found in Results but not in Golden:  %s", file)
		}
	}

	for _, file := range goldenFiles {
		if !resultSet.Has(file) {
			continue
		}

		goldenText := b.mangleResult(b.goldenDir, file, false)
		resultText := b.mangleResult(b.workDir, file, false)

		if fileType(file) == "log" {
			// save mangled logs so difference is easier on eyes
			goldenFile := filepath.Join(b.logDir, "mangled.golden.log")
			resultFile := filepath.Join(b.logDir, "mangled.result.log")
			require.NoError(b.t, ioutil.WriteFile(goldenFile, []byte(goldenText), bilib.PermSecure))
			require.NoError(b.t, ioutil.WriteFile(resultFile, []byte(resultText), bilib.PermSecure))
		}

		if goldenText == resultText {
			continue
		}
		errorCount++

		diff := difflib.UnifiedDiff{
			A:       difflib.SplitLines(goldenText),
			B:       difflib.SplitLines(resultText),
			Context: 0,
		}
		text, err := difflib.GetUnifiedDiffString(diff)
		require.NoError(b.t, err, "diff failed")

		log.Print(divider)
		log.Printf("| MISCOMPARE  -Golden vs +Results for  %s", file)
		for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
			log.Printf("| %s", strings.TrimSpace(line))
		}
	}

	if errorCount > 0 {
		log.Print(divider)
	}
	if errorCount == 0 && goldenNum != resultNum {
		return -1
	}
	return errorCount
}

// storeGolden will store workdir files to the golden directory.
// Golden results will have adapted file names and contain
// generic strings instead of local or cloud paths.
func (b *bisyncTest) storeGolden() {
	// Perform consistency checks
	files := b.listDir(b.workDir)
	require.NotEmpty(b.t, files, "nothing to store in golden dir")

	// Pass 1: validate files before storing
	for _, fileName := range files {
		if fileType(fileName) == "lock" {
			continue
		}
		goldName := b.toGolden(fileName)
		if goldName != fileName {
			targetPath := filepath.Join(b.workDir, goldName)
			exists := bilib.FileExists(targetPath)
			require.False(b.t, exists, "golden name overlap for file %s", fileName)
		}
		text := b.mangleResult(b.workDir, fileName, true)
		if fileType(fileName) == "log" {
			require.NotEmpty(b.t, text, "incorrect golden log %s", fileName)
		}
	}

	// Pass 2: perform a verbatim copy
	_ = os.RemoveAll(b.goldenDir)
	require.NoError(b.t, bilib.CopyDir(b.workDir, b.goldenDir))

	// Pass 3: adapt file names and content
	for _, fileName := range files {
		if fileType(fileName) == "lock" {
			continue
		}
		text := b.mangleResult(b.goldenDir, fileName, true)

		goldName := b.toGolden(fileName)
		goldPath := filepath.Join(b.goldenDir, goldName)
		err := ioutil.WriteFile(goldPath, []byte(text), bilib.PermSecure)
		assert.NoError(b.t, err, "writing golden file %s", goldName)

		if goldName != fileName {
			origPath := filepath.Join(b.goldenDir, fileName)
			assert.NoError(b.t, os.Remove(origPath), "removing original file %s", fileName)
		}
	}
}

// mangleResult prepares test logs or listings for comparison
func (b *bisyncTest) mangleResult(dir, file string, golden bool) string {
	buf, err := ioutil.ReadFile(filepath.Join(dir, file))
	require.NoError(b.t, err)
	text := string(buf)

	switch fileType(strings.TrimSuffix(file, ".sav")) {
	case "queue":
		lines := strings.Split(text, eol)
		sort.Strings(lines)
		return joinLines(lines)
	case "listing":
		return mangleListing(text, golden)
	case "log":
		// fall thru
	default:
		return text
	}

	// Adapt log lines to the golden way.
	lines := strings.Split(string(buf), eol)
	pathReplacer := b.newReplacer(true)

	rep := logReplacements
	if b.testCase == "dry_run" {
		rep = append(rep, dryrunReplacements...)
	}
	repFrom := make([]*regexp.Regexp, len(rep)/2)
	repTo := make([]string, len(rep)/2)
	for i := 0; i < len(rep); i += 2 {
		repFrom[i/2] = regexp.MustCompile(rep[i])
		repTo[i/2] = rep[i+1]
	}

	hoppers := make([]*regexp.Regexp, len(logHoppers))
	dampers := make([][]string, len(logHoppers))
	for i, regex := range logHoppers {
		hoppers[i] = regexp.MustCompile("^" + regex + "$")
	}

	// The %q format doubles backslashes, hence "{1,2}"
	regexBackslash := regexp.MustCompile(`\\{1,2}`)

	emptyCount := 0
	maxEmpty := 0
	if b.golden {
		maxEmpty = 2
	}

	result := make([]string, 0, len(lines))
	for _, s := range lines {
		// Adapt file paths
		s = pathReplacer.Replace(strings.TrimSpace(s))

		// Apply regular expression replacements
		for i := 0; i < len(repFrom); i++ {
			s = repFrom[i].ReplaceAllString(s, repTo[i])
		}
		s = strings.TrimSpace(s)
		if s == dropMe {
			continue
		}

		if fixSlash && regexFixSlash.MatchString(s) {
			s = regexBackslash.ReplaceAllString(s, "/")
		}

		// Sort consecutive groups of naturally unordered lines.
		// Any such group must end before the log ends or it might be lost.
		absorbed := false
		for i := 0; i < len(dampers); i++ {
			match := false
			if s != "" && !absorbed {
				match = hoppers[i].MatchString(s)
			}
			if match {
				dampers[i] = append(dampers[i], s)
				absorbed = true
			} else if len(dampers[i]) > 0 {
				sort.Strings(dampers[i])
				result = append(result, dampers[i]...)
				dampers[i] = nil
			}
		}
		if absorbed {
			continue
		}

		// Skip empty lines unless storing to golden
		if s == "" {
			if emptyCount < maxEmpty {
				result = append(result, "")
			}
			emptyCount++
			continue
		}
		result = append(result, s)
		emptyCount = 0
	}

	return joinLines(result)
}

// mangleListing sorts listing lines before comparing.
func mangleListing(text string, golden bool) string {
	lines := strings.Split(text, eol)

	hasHeader := len(lines) > 0 && strings.HasPrefix(lines[0], bisync.ListingHeader)
	if hasHeader {
		lines = lines[1:]
	}

	// Split line in 4 groups:    (flag, size)(hash.)( .id., .......modtime....... )(name).
	regex := regexp.MustCompile(`^([^ ] +\d+ )([^ ]+)( [^ ]+ [\d-]+T[\d:.]+[\d+-]+ )(".+")$`)

	getFile := func(s string) string {
		if match := regex.FindStringSubmatch(strings.TrimSpace(s)); match != nil {
			if name, err := strconv.Unquote(match[4]); err == nil {
				return name
			}
		}
		return s
	}

	sort.SliceStable(lines, func(i, j int) bool {
		return getFile(lines[i]) < getFile(lines[j])
	})

	// Store hash as golden but ignore when comparing.
	if !golden {
		for i, s := range lines {
			match := regex.FindStringSubmatch(strings.TrimSpace(s))
			if match != nil && match[2] != "-" {
				lines[i] = match[1] + "-" + match[3] + match[4]
			}
		}
	}

	text = joinLines(lines)
	if hasHeader && golden {
		text = bisync.ListingHeader + " test\n" + text
	}
	return text
}

// joinLines joins text lines dropping empty lines at the beginning and at the end
func joinLines(lines []string) string {
	text := strings.Join(lines, eol)
	text = strings.TrimLeft(text, eol)
	text = strings.TrimRight(text, eol)
	if text != "" {
		text += eol
	}
	return text
}

// newReplacer can create two kinds of string replacers.
// If mangle is false, it will substitute macros in test scenario.
// If true then mangle paths in test log to match with golden log.
func (b *bisyncTest) newReplacer(mangle bool) *strings.Replacer {
	if !mangle {
		rep := []string{
			"{datadir/}", b.dataDir + slash,
			"{testdir/}", b.testDir + slash,
			"{workdir/}", b.workDir + slash,
			"{path1/}", b.path1,
			"{path2/}", b.path2,
			"{session}", b.sessionName,
			"{/}", slash,
		}
		return strings.NewReplacer(rep...)
	}

	rep := []string{
		b.dataDir + slash, "{datadir/}",
		b.testDir + slash, "{testdir/}",
		b.workDir + slash, "{workdir/}",
		b.path1, "{path1/}",
		b.path2, "{path2/}",
		b.sessionName, "{session}",
	}
	if fixSlash {
		prep := []string{}
		for i := 0; i < len(rep); i += 2 {
			// A hack for backslashes doubled by the go format "%q".
			doubled := strings.ReplaceAll(rep[i], "\\", "\\\\")
			if rep[i] != doubled {
				prep = append(prep, doubled, rep[i+1])
			}
		}
		// Put longer patterns first to ensure correct translation.
		rep = append(prep, rep...)
	}
	return strings.NewReplacer(rep...)
}

// toGolden makes a result file name golden.
// It replaces each canonical path separately instead of using the
// session name to allow for subdirs in the extended-char-paths case.
func (b *bisyncTest) toGolden(name string) string {
	name = strings.ReplaceAll(name, b.canonPath1, goldenCanonBase)
	name = strings.ReplaceAll(name, b.canonPath2, goldenCanonBase)
	name = strings.TrimSuffix(name, ".sav")
	return name
}

func (b *bisyncTest) mkdir(dir string) {
	require.NoError(b.t, os.MkdirAll(dir, os.ModePerm))
}

func (b *bisyncTest) ensureDir(parent, dir string, optional bool) string {
	path := filepath.Join(parent, dir)
	if !optional {
		info, err := os.Stat(path)
		require.NoError(b.t, err, "%s must exist", path)
		require.True(b.t, info.IsDir(), "%s must be a directory", path)
	}
	return path
}

func (b *bisyncTest) listDir(dir string) (names []string) {
	files, err := ioutil.ReadDir(dir)
	require.NoError(b.t, err)
	for _, file := range files {
		names = append(names, filepath.Base(file.Name()))
	}
	// Sort files to ensure comparability.
	sort.Strings(names)
	return
}

// fileType detects test artifact type.
// Notes:
// - "filtersfile.txt" will NOT be recognized as a filters file
// - only "test.log" will be recognized as a test log file
func fileType(fileName string) string {
	if fileName == logFileName {
		return "log"
	}
	switch filepath.Ext(fileName) {
	case ".lst", ".lst-new", ".lst-err", ".lst-dry", ".lst-dry-new":
		return "listing"
	case ".que":
		return "queue"
	case ".lck":
		return "lock"
	case ".flt":
		return "filters"
	}
	if strings.HasSuffix(fileName, ".flt.md5") {
		return "filters"
	}
	return "other"
}

// logPrintf prints a message to stdout and to the test log
func (b *bisyncTest) logPrintf(text string, args ...interface{}) {
	line := fmt.Sprintf(text, args...)
	log.Print(line)
	if b.logFile != nil {
		_, err := fmt.Fprintln(b.logFile, line)
		require.NoError(b.t, err, "writing log file")
	}
}
