// TestBisync is a test engine for bisync test cases.
// See https://rclone.org/bisync/#testing for documentation.
// Test cases are organized in subdirs beneath ./testdata
// Results are compared against golden listings and log file.
package bisync_test

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
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
	"unicode/utf8"

	"github.com/rclone/rclone/cmd/bisync"
	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/sync"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/lib/terminal"
	"golang.org/x/text/unicode/norm"

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

var initDate = time.Date(2000, time.January, 1, 0, 0, 0, 0, bisync.TZ)

/* Useful Command Shortcuts */
// go test ./cmd/bisync -remote local -race
// go test ./cmd/bisync -remote local -golden
// go test ./cmd/bisync -remote local -case extended_filenames
// go run ./fstest/test_all -run '^TestBisync.*$' -timeout 3h -verbose -maxtries 5
// go run ./fstest/test_all -remotes local,TestCrypt:,TestDrive:,TestOneDrive:,TestOneDriveBusiness:,TestDropbox:,TestCryptDrive:,TestOpenDrive:,TestChunker:,:memory:,TestCryptNoEncryption:,TestCombine:DirA,TestFTPRclone:,TestWebdavRclone:,TestS3Rclone:,TestSFTPRclone:,TestSFTPRcloneSSH:,TestNextcloud:,TestChunkerNometaLocal:,TestChunkerChunk3bLocal:,TestChunkerLocal:,TestChunkerChunk3bNometaLocal:,TestStorj: -run '^TestBisync.*$' -timeout 3h -verbose -maxtries 5
// go test -timeout 3h -run '^TestBisync.*$' github.com/rclone/rclone/cmd/bisync -remote TestDrive:Bisync -v
// go test -timeout 3h -run '^TestBisyncRemoteRemote/basic$' github.com/rclone/rclone/cmd/bisync -remote TestDropbox:Bisync -v
// TestFTPProftpd:,TestFTPPureftpd:,TestFTPRclone:,TestFTPVsftpd:,TestHdfs:,TestS3Minio:,TestS3MinioEdge:,TestS3Rclone:,TestSeafile:,TestSeafileEncrypted:,TestSeafileV6:,TestSFTPOpenssh:,TestSFTPRclone:,TestSFTPRcloneSSH:,TestSia:,TestSwiftAIO:,TestWebdavNextcloud:,TestWebdavOwncloud:,TestWebdavRclone:

// logReplacements make modern test logs comparable with golden dir.
// It is a string slice of even length with this structure:
//
//	{`matching regular expression`, "mangled result string", ...}
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
	`^NOTICE: .*?: Forced to upload files to set modification times on this backend.$`, dropMe,
	`^INFO  : .*? Committing uploads - please wait...$`, dropMe,
	`^INFO  : .*?: src and dst identical but can't set mod time without deleting and re-uploading$`, dropMe,
	`^INFO  : .*?: src and dst identical but can't set mod time without re-uploading$`, dropMe,
	// ignore crypt info messages
	`^INFO  : .*?: Crypt detected! Using cryptcheck instead of check. \(Use --size-only or --ignore-checksum to disable\)$`, dropMe,
	// ignore drive info messages
	`^NOTICE:.*?Files of unknown size \(such as Google Docs\) do not sync reliably with --checksum or --size-only\. Consider using modtime instead \(the default\) or --drive-skip-gdocs.*?$`, dropMe,
	// ignore cache backend cache expired messages
	`^INFO  : .*cache expired.*$`, dropMe,
	// ignore "Implicitly create directory" messages (TestnStorage:)
	`^INFO  : .*Implicitly create directory.*$`, dropMe,
	// ignore differences in backend features
	`^.*?"HashType1":.*?$`, dropMe,
	`^.*?"HashType2":.*?$`, dropMe,
	`^.*?"SlowHashDetected":.*?$`, dropMe,
	`^.*? for same-side diffs on .*?$`, dropMe,
	`^.*?Downloading hashes.*?$`, dropMe,
	`^.*?Can't compare hashes, so using check --download.*?$`, dropMe,
	// ignore timestamps in directory time updates
	`^(INFO  : .*?: (Made directory with|Set directory) (metadata|modification time)).*$`, dropMe,
	// ignore sizes in directory time updates
	`^(NOTICE: .*?: Skipped set directory modification time as --dry-run is set).*$`, dropMe,
	// ignore sizes in directory metadata updates
	`^(NOTICE: .*?: Skipped update directory metadata as --dry-run is set).*$`, dropMe,
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
	`.* +.....Access test failed: Path[12] file not found in Path[12].*`,

	// Test case `resync` suffered from the order of queued copies.
	`(?:INFO  |NOTICE): - Path2    Resync will copy to Path1 +- .*`,

	// Test case `normalization` can have random order of fix-case files.
	`(?:INFO  |NOTICE): .*: Fixed case by renaming to: .*`,

	// order of files re-checked prior to a conflict rename
	`ERROR : .*: {hashtype} differ.*`,

	// Directory modification time setting can happen in any order
	`INFO  : .*: (Set directory modification time|Made directory with metadata).*`,
}

// Some log lines can contain Windows path separator that must be
// converted to "/" in every matching token to match golden logs.
var logLinesWithSlash = []string{
	`.*\(\d\d\)  :.*(fix-names|touch-glob|touch-copy|copy-file|copy-as|copy-dir|delete-file) `,
	`INFO  : - .*Path[12].* +.*Queue copy to.* Path[12].*`,
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
	argRemote1    string
	argRemote2    string
	noCompare     bool
	noCleanup     bool
	golden        bool
	debug         bool
	stopAt        int
	TestFn        bisync.TestFunc
	ignoreModtime bool // ignore modtimes when comparing final listings, for backends without support
}

var color = bisync.Color

// TestMain drives the tests
func TestMain(m *testing.M) {
	fstest.TestMain(m)
}

// Path1 is remote, Path2 is local
func TestBisyncRemoteLocal(t *testing.T) {
	if *fstest.RemoteName == *argRemote2 {
		t.Skip("path1 and path2 are the same remote")
	}
	_, remote, cleanup, err := fstest.RandomRemote()
	fs.Logf(nil, "remote: %v", remote)
	require.NoError(t, err)
	defer cleanup()
	testBisync(t, remote, *argRemote2)
}

// Path1 is local, Path2 is remote
func TestBisyncLocalRemote(t *testing.T) {
	if *fstest.RemoteName == *argRemote2 {
		t.Skip("path1 and path2 are the same remote")
	}
	_, remote, cleanup, err := fstest.RandomRemote()
	fs.Logf(nil, "remote: %v", remote)
	require.NoError(t, err)
	defer cleanup()
	testBisync(t, *argRemote2, remote)
}

// Path1 and Path2 are both different directories on remote
// (useful for testing server-side copy/move)
func TestBisyncRemoteRemote(t *testing.T) {
	_, remote, cleanup, err := fstest.RandomRemote()
	fs.Logf(nil, "remote: %v", remote)
	require.NoError(t, err)
	defer cleanup()
	testBisync(t, remote, remote)
}

// TestBisync is a test engine for bisync test cases.
func testBisync(t *testing.T, path1, path2 string) {
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
	bisync.Colors = true
	time.Local = bisync.TZ
	ci.FsCacheExpireDuration = 5 * time.Hour

	baseDir, err := os.Getwd()
	require.NoError(t, err, "get current directory")
	randName := time.Now().Format("150405") + random.String(2) // some bucket backends don't like dots, keep this short to avoid linux errors
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
		argRemote1: path1,
		argRemote2: path2,
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
				// if dir is empty, skip it (can happen due to gitignored files/dirs when checking out branch)
				if len(b.listDir(filepath.Join(b.dataRoot, testCase))) == 0 {
					continue
				}
				testList = append(testList, testCase)
			}
		}
	}
	require.False(t, b.stopAt > 0 && len(testList) > 1, "-stop-at is meaningful only for a single test")
	deadline, hasDeadline := t.Deadline()
	var maxRunDuration time.Duration

	for _, testCase := range testList {
		testCase = strings.ReplaceAll(testCase, "-", "_")
		testCase = strings.TrimPrefix(testCase, "test_")
		t.Run(testCase, func(childTest *testing.T) {
			startTime := time.Now()
			remaining := time.Until(deadline)
			if hasDeadline && (remaining < maxRunDuration || remaining < 10*time.Second) { // avoid starting tests we don't have time to finish
				childTest.Fatalf("test %v timed out - not enough time to start test (%v remaining, need %v for test)", testCase, remaining, maxRunDuration)
			}
			bCopy := *b
			bCopy.runTestCase(ctx, childTest, testCase)
			if time.Since(startTime) > maxRunDuration {
				maxRunDuration = time.Since(startTime)
			}
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

	if strings.Contains(b.replaceHex(b.path1), " ") || strings.Contains(b.replaceHex(b.path2), " ") {
		b.t.Skip("skipping as tests can't handle spaces config string")
	}

	b.sessionName = bilib.SessionName(b.fs1, b.fs2)
	b.testDir = b.ensureDir(b.dataRoot, "test_"+b.testCase, false)
	b.initDir = b.ensureDir(b.testDir, "initial", false)
	b.goldenDir = b.ensureDir(b.testDir, "golden", false)
	b.dataDir = b.ensureDir(b.testDir, "modfiles", true) // optional

	// normalize unicode so tets are runnable on macOS
	b.sessionName = norm.NFC.String(b.sessionName)
	b.goldenDir = norm.NFC.String(b.goldenDir)

	// For test stability, jam initial dates to a fixed past date.
	// Test cases that change files will touch specific files to fixed new dates.
	err = filepath.Walk(b.initDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			return os.Chtimes(path, initDate, initDate)
		}
		return err
	})
	require.NoError(b.t, err, "jamming initial dates")

	// copy to a new unique initdir and datadir so concurrent tests don't interfere with each other
	ctxNoDsStore, _ := ctxNoDsStore(ctx, b.t)
	makeUnique := func(label, oldPath string) (newPath string) {
		newPath = oldPath
		info, err := os.Stat(oldPath)
		if err == nil && info.IsDir() { // datadir is optional
			oldFs, err := cache.Get(ctx, oldPath)
			require.NoError(b.t, err)
			newPath = b.tempDir + "/" + label + "/" + "test_" + b.testCase + "-" + random.String(8)
			newFs, err := cache.Get(ctx, newPath)
			require.NoError(b.t, err)
			require.NoError(b.t, sync.CopyDir(ctxNoDsStore, newFs, oldFs, true), "setting up "+label)
		}
		return newPath
	}
	b.initDir = makeUnique("initdir", b.initDir)
	b.dataDir = makeUnique("datadir", b.dataDir)

	// Prepare initial content
	b.cleanupCase(ctx)
	fstest.CheckListingWithPrecision(b.t, b.fs1, []fstest.Item{}, []string{}, b.fs1.Precision()) // verify starting from empty
	fstest.CheckListingWithPrecision(b.t, b.fs2, []fstest.Item{}, []string{}, b.fs2.Precision())
	initFs, err := cache.Get(ctx, b.initDir)
	require.NoError(b.t, err)

	// verify pre-test equality (garbage in, garbage out!)
	srcObjs, srcDirs, err := walk.GetAll(ctxNoDsStore, initFs, "", false, -1)
	assert.NoError(b.t, err)
	items := []fstest.Item{}
	for _, obj := range srcObjs {
		require.False(b.t, strings.Contains(obj.Remote(), ".partial"))
		rc, err := operations.Open(ctxNoDsStore, obj)
		assert.NoError(b.t, err)
		bytes := make([]byte, obj.Size())
		_, err = rc.Read(bytes)
		assert.NoError(b.t, err)
		assert.NoError(b.t, rc.Close())
		item := fstest.NewItem(norm.NFC.String(obj.Remote()), string(bytes), obj.ModTime(ctxNoDsStore))
		items = append(items, item)
	}
	dirs := []string{}
	for _, dir := range srcDirs {
		dirs = append(dirs, norm.NFC.String(dir.Remote()))
	}
	fs.Logf(nil, "checking initFs %s", initFs)
	fstest.CheckListingWithPrecision(b.t, initFs, items, dirs, initFs.Precision())
	checkError(b.t, sync.CopyDir(ctxNoDsStore, b.fs1, initFs, true), "setting up path1")
	fs.Logf(nil, "checking Path1 %s", b.fs1)
	fstest.CheckListingWithPrecision(b.t, b.fs1, items, dirs, b.fs1.Precision())
	checkError(b.t, sync.CopyDir(ctxNoDsStore, b.fs2, initFs, true), "setting up path2")
	fs.Logf(nil, "checking path2 %s", b.fs2)
	fstest.CheckListingWithPrecision(b.t, b.fs2, items, dirs, b.fs2.Precision())

	// Create log file
	b.mkdir(b.workDir)
	b.logFile, err = os.Create(b.logPath)
	require.NoError(b.t, err, "creating log file")

	// Execute test scenario
	scenFile := filepath.Join(b.testDir, "scenario.txt")
	scenBuf, err := os.ReadFile(scenFile)
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
		fs.Logf(nil, "Store results to golden directory")
		b.storeGolden()
		return
	}

	errorCount := 0
	if b.noCompare {
		fs.Logf(nil, "Skip comparing results with golden directory")
		errorCount = -2
	} else {
		errorCount = b.compareResults()
	}

	if b.noCleanup {
		fs.Logf(nil, "Skip cleanup")
	} else {
		b.cleanupCase(ctx)
	}

	var msg string
	var passed bool
	switch errorCount {
	case 0:
		msg = color(terminal.GreenFg, fmt.Sprintf("TEST %s PASSED", b.testCase))
		passed = true
	case -2:
		msg = color(terminal.YellowFg, fmt.Sprintf("TEST %s SKIPPED", b.testCase))
		passed = true
	case -1:
		msg = color(terminal.RedFg, fmt.Sprintf("TEST %s FAILED - WRONG NUMBER OF FILES", b.testCase))
		passed = false
	default:
		msg = color(terminal.RedFg, fmt.Sprintf("TEST %s FAILED - %d MISCOMPARED FILES", b.testCase, errorCount))
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
	if bilib.IsLocalPath(remote) && !strings.HasPrefix(remote, ":") && !strings.Contains(remote, ",") {
		if remote != "" && !strings.HasPrefix(remote, "local") && *fstest.RemoteName != "" {
			b.t.Fatalf(`Missing ":" in remote %q. Use "local" to test with local filesystem.`, remote)
		}
		parent, err = cache.Get(ctx, b.tempDir)
		checkError(b.t, err, "parsing local tempdir %s", b.tempDir)

		path = filepath.Join(b.tempDir, b.testCase)
		path = filepath.Join(path, subdir)
	} else {
		last := remote[len(remote)-1]
		if last != ':' && last != '/' {
			remote += "/"
		}
		remote += b.randName
		parent, err = cache.Get(ctx, remote)
		checkError(b.t, err, "parsing remote %s", remote)
		checkError(b.t, operations.Mkdir(ctx, parent, subdir), "Mkdir "+subdir) // ensure dir exists (storj seems to need this)

		path = remote + "/" + b.testCase
		path += "/" + subdir
	}

	f, err = cache.Get(ctx, path)
	checkError(b.t, err, "parsing remote/subdir %s/%s", remote, subdir)
	path = bilib.FsPath(f)                                                                                                                // Make it canonical
	canon = bilib.StripHexString(bilib.CanonicalPath(strings.TrimSuffix(strings.TrimSuffix(path, `\`+subdir+`\`), "/"+subdir+"/"))) + "_" // account for possible connection string
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
	b.logPrintf("%s %s", color(terminal.CyanFg, b.stepStr), color(terminal.BlueFg, line))

	ci := fs.GetConfig(ctx)
	ciSave := *ci
	defer func() {
		*ci = ciSave
	}()
	ci.LogLevel = fs.LogLevelInfo
	if b.debug {
		ci.LogLevel = fs.LogLevelDebug
	}

	testFunc := func() {
		src := filepath.Join(b.dataDir, "file7.txt")

		for i := 0; i < 50; i++ {
			dst := "file" + fmt.Sprint(i) + ".txt"
			err := b.copyFile(ctx, src, b.replaceHex(b.path2), dst)
			if err != nil {
				fs.Errorf(src, "error copying file: %v", err)
			}
			dst = "file" + fmt.Sprint(100-i) + ".txt"
			err = b.copyFile(ctx, src, b.replaceHex(b.path1), dst)
			if err != nil {
				fs.Errorf(dst, "error copying file: %v", err)
			}
		}
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
		dir := ""
		if strings.HasPrefix(args[1], b.replaceHex(b.path1)) {
			fsrc = b.fs1
			dir = strings.TrimPrefix(args[1], b.replaceHex(b.path1))
		} else if strings.HasPrefix(args[1], b.replaceHex(b.path2)) {
			fsrc = b.fs2
			dir = strings.TrimPrefix(args[1], b.replaceHex(b.path2))
		} else {
			return fmt.Errorf("error parsing arg: %q (path1: %q, path2: %q)", args[1], b.path1, b.path2)
		}
		return purgeChildren(ctx, fsrc, dir)
	case "delete-file":
		b.checkArgs(args, 1, 1)
		dir, file := filepath.Split(args[1])
		if fsrc, err = cache.Get(ctx, dir); err != nil {
			return err
		}
		var obj fs.Object
		if obj, err = fsrc.NewObject(ctx, file); err != nil {
			return err
		}
		return operations.DeleteFile(ctx, obj)
	case "delete-glob":
		b.checkArgs(args, 2, 2)
		if fsrc, err = cache.Get(ctx, args[1]); err != nil {
			return err
		}
		return deleteFiles(ctx, fsrc, args[2])
	case "touch-glob":
		b.checkArgs(args, 3, 3)
		date, src, glob := args[1], args[2], args[3]
		if fsrc, err = cache.Get(ctx, b.replaceHex(src)); err != nil {
			return err
		}
		_, err = touchFiles(ctx, date, fsrc, src, glob)
		return err
	case "touch-copy":
		b.checkArgs(args, 3, 3)
		date, src, dst := args[1], args[2], args[3]
		dir, file := filepath.Split(src)
		if fsrc, err = cache.Get(ctx, dir); err != nil {
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
	case "copy-as-NFC":
		b.checkArgs(args, 3, 3)
		ci.NoUnicodeNormalization = true
		ci.FixCase = true
		return b.copyFile(ctx, args[1], norm.NFC.String(args[2]), norm.NFC.String(args[3]))
	case "copy-as-NFD":
		b.checkArgs(args, 3, 3)
		ci.NoUnicodeNormalization = true
		ci.FixCase = true
		return b.copyFile(ctx, args[1], norm.NFD.String(args[2]), norm.NFD.String(args[3]))
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
			ctxNoDsStore, _ := ctxNoDsStore(ctx, b.t)
			err = sync.CopyDir(ctxNoDsStore, fdst, fsrc, true)
		case "sync-dir":
			ctxNoDsStore, _ := ctxNoDsStore(ctx, b.t)
			err = sync.Sync(ctxNoDsStore, fdst, fsrc, true)
		}
		return err
	case "list-dirs":
		b.checkArgs(args, 1, 1)
		return b.listSubdirs(ctx, args[1], true)
	case "list-files":
		b.checkArgs(args, 1, 1)
		return b.listSubdirs(ctx, args[1], false)
	case "bisync":
		ci.NoUnicodeNormalization = false
		ci.IgnoreCaseSync = false
		// ci.FixCase = true
		return b.runBisync(ctx, args[1:])
	case "test-func":
		b.TestFn = testFunc
		return
	case "fix-names":
		// in case the local os converted any filenames
		ci.NoUnicodeNormalization = true
		ci.FixCase = true
		ci.IgnoreTimes = true
		reset := func() {
			ci.NoUnicodeNormalization = false
			ci.FixCase = false
			ci.IgnoreTimes = false
		}
		defer reset()
		b.checkArgs(args, 1, 1)
		var ok bool
		var remoteName string
		var remotePath string
		remoteName, remotePath, err = fspath.SplitFs(args[1])
		if err != nil {
			return err
		}
		if remoteName == "" {
			remoteName = "/"
		}

		fsrc, err = cache.Get(ctx, remoteName)
		if err != nil {
			return err
		}

		// DEBUG
		fs.Debugf(remotePath, "is NFC: %v", norm.NFC.IsNormalString(remotePath))
		fs.Debugf(remotePath, "is NFD: %v", norm.NFD.IsNormalString(remotePath))
		fs.Debugf(remotePath, "is valid UTF8: %v", utf8.ValidString(remotePath))

		// check if it's a dir, try moving it
		var leaf string
		_, leaf, err = fspath.Split(remotePath)
		if err == nil && leaf == "" {
			remotePath = args[1]
			fs.Debugf(remotePath, "attempting to fix directory")

			fixDirname := func(old, new string) {
				if new != old {
					oldName, err := cache.Get(ctx, old)
					if err != nil {
						fs.Errorf(old, "error getting Fs: %v", err)
						return
					}
					fs.Debugf(nil, "Attempting to move %s to %s", oldName.Root(), new)
					// Create random name to temporarily move dir to
					tmpDirName := strings.TrimSuffix(new, slash) + "-rclone-move-" + random.String(8)
					var tmpDirFs fs.Fs
					tmpDirFs, err = cache.Get(ctx, tmpDirName)
					if err != nil {
						fs.Errorf(tmpDirName, "error creating temp dir for move: %v", err)
					}
					if tmpDirFs == nil {
						return
					}
					err = sync.MoveDir(ctx, tmpDirFs, oldName, true, true)
					if err != nil {
						fs.Debugf(oldName, "error attempting to move folder: %v", err)
					}
					// now move the temp dir to real name
					fsrc, err = cache.Get(ctx, new)
					if err != nil {
						fs.Errorf(new, "error creating fsrc dir for move: %v", err)
					}
					if fsrc == nil {
						return
					}
					err = sync.MoveDir(ctx, fsrc, tmpDirFs, true, true)
					if err != nil {
						fs.Debugf(tmpDirFs, "error attempting to move folder to %s: %v", fsrc.Root(), err)
					}
				} else {
					fs.Debugf(nil, "old and new are equal. Skipping. %s (%s) %s (%s)", old, stringToHash(old), new, stringToHash(new))
				}
			}

			if norm.NFC.String(remotePath) != remotePath && norm.NFD.String(remotePath) != remotePath {
				fs.Debugf(remotePath, "This is neither fully NFD or NFC -- can't fix reliably!")
			}
			fixDirname(norm.NFC.String(remotePath), remotePath)
			fixDirname(norm.NFD.String(remotePath), remotePath)
			return
		}

		// if it's a file
		fs.Debugf(remotePath, "attempting to fix file -- filename hash: %s", stringToHash(leaf))
		fixFilename := func(old, new string) {
			ok, err := fs.FileExists(ctx, fsrc, old)
			if err != nil {
				fs.Debugf(remotePath, "error checking if file exists: %v", err)
			}
			fs.Debugf(old, "file exists: %v %s", ok, stringToHash(old))
			fs.Debugf(nil, "FILE old: %s new: %s equal: %v", old, new, old == new)
			fs.Debugf(nil, "HASH old: %s new: %s equal: %v", stringToHash(old), stringToHash(new), stringToHash(old) == stringToHash(new))
			if ok && new != old {
				fs.Debugf(new, "attempting to rename %s to %s", old, new)
				srcObj, err := fsrc.NewObject(ctx, old)
				if err != nil {
					fs.Errorf(old, "errorfinding srcObj - %v", err)
				}
				_, err = operations.MoveCaseInsensitive(ctx, fsrc, fsrc, new, old, false, srcObj)
				if err != nil {
					fs.Errorf(new, "error trying to rename %s to %s - %v", old, new, err)
				}
			}
		}

		// look for NFC version
		fixFilename(norm.NFC.String(remotePath), remotePath)
		// if it's in a subdir we just moved, the file and directory might have different encodings. Check for that.
		mixed := strings.TrimSuffix(norm.NFD.String(remotePath), norm.NFD.String(leaf)) + norm.NFC.String(leaf)
		fixFilename(mixed, remotePath)
		// Try NFD
		fixFilename(norm.NFD.String(remotePath), remotePath)
		// Try mixed in reverse
		mixed = strings.TrimSuffix(norm.NFC.String(remotePath), norm.NFC.String(leaf)) + norm.NFD.String(leaf)
		fixFilename(mixed, remotePath)
		// check if it's right now, error if not
		ok, err = fs.FileExists(ctx, fsrc, remotePath)
		if !ok || err != nil {
			fs.Logf(remotePath, "Can't find expected file %s (was it renamed by the os?) %v", args[1], err)
			return
		} else {
			// include hash of filename to make unicode form differences easier to see in logs
			fs.Debugf(remotePath, "verified file exists at correct path. filename hash: %s", stringToHash(leaf))
		}
		return
	default:
		return fmt.Errorf("unknown command: %q", args[0])
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

func (b *bisyncTest) checkPreReqs(ctx context.Context, opt *bisync.Options) (context.Context, *bisync.Options) {
	// check pre-requisites
	if b.testCase == "backupdir" && !(b.fs1.Features().IsLocal && b.fs2.Features().IsLocal) {
		b.t.Skip("backupdir test currently only works on local (it uses the workdir)")
	}
	if b.testCase == "volatile" && !(b.fs1.Features().IsLocal && b.fs2.Features().IsLocal) {
		b.t.Skip("skipping 'volatile' test on non-local as it requires uploading 100 files")
	}
	if strings.HasPrefix(b.fs1.String(), "Dropbox") || strings.HasPrefix(b.fs2.String(), "Dropbox") {
		fs.GetConfig(ctx).RefreshTimes = true // https://rclone.org/bisync/#notes-about-testing
	}
	if strings.HasPrefix(b.fs1.String(), "Dropbox") {
		b.fs1.Features().Disable("Copy") // https://github.com/rclone/rclone/issues/6199#issuecomment-1570366202
	}
	if strings.HasPrefix(b.fs2.String(), "Dropbox") {
		b.fs2.Features().Disable("Copy") // https://github.com/rclone/rclone/issues/6199#issuecomment-1570366202
	}
	if strings.HasPrefix(b.fs1.String(), "OneDrive") {
		b.fs1.Features().Disable("Copy") // API has longstanding bug for conflictBehavior=replace https://github.com/rclone/rclone/issues/4590
		b.fs1.Features().Disable("Move")
	}
	if strings.HasPrefix(b.fs2.String(), "OneDrive") {
		b.fs2.Features().Disable("Copy") // API has longstanding bug for conflictBehavior=replace https://github.com/rclone/rclone/issues/4590
		b.fs2.Features().Disable("Move")
	}
	if strings.Contains(strings.ToLower(fs.ConfigString(b.fs1)), "mailru") || strings.Contains(strings.ToLower(fs.ConfigString(b.fs2)), "mailru") {
		fs.GetConfig(ctx).TPSLimit = 10 // https://github.com/rclone/rclone/issues/7768#issuecomment-2060888980
	}
	if (!b.fs1.Features().CanHaveEmptyDirectories || !b.fs2.Features().CanHaveEmptyDirectories) && (b.testCase == "createemptysrcdirs" || b.testCase == "rmdirs") {
		b.t.Skip("skipping test as remote does not support empty dirs")
	}
	if b.fs1.Precision() == fs.ModTimeNotSupported || b.fs2.Precision() == fs.ModTimeNotSupported {
		if b.testCase != "nomodtime" {
			b.t.Skip("skipping test as at least one remote does not support setting modtime")
		}
		b.ignoreModtime = true
	}
	// test if modtimes are writeable
	testSetModtime := func(f fs.Fs) {
		in := bytes.NewBufferString("modtime_write_test")
		objinfo := object.NewStaticObjectInfo("modtime_write_test", initDate, int64(len("modtime_write_test")), true, nil, nil)
		obj, err := f.Put(ctx, in, objinfo)
		require.NoError(b.t, err)
		err = obj.SetModTime(ctx, initDate)
		if err == fs.ErrorCantSetModTime {
			if b.testCase != "nomodtime" {
				b.t.Skip("skipping test as at least one remote does not support setting modtime")
			}
		}
		err = obj.Remove(ctx)
		require.NoError(b.t, err)
	}
	testSetModtime(b.fs1)
	testSetModtime(b.fs2)

	if b.testCase == "normalization" || b.testCase == "extended_char_paths" || b.testCase == "extended_filenames" {
		// test whether remote is capable of running test
		const chars = "ě{chr:81}{chr:fe}{spc}áñhࢺ_測試Рускийěáñ👸🏼🧝🏾‍♀️💆🏿‍♂️🐨🤙🏼🤮🧑🏻‍🔧🧑‍🔬éö"
		testfilename1 := splitLine(norm.NFD.String(norm.NFC.String(chars)))[0]
		testfilename2 := splitLine(norm.NFC.String(norm.NFD.String(chars)))[0]
		tempDir, err := cache.Get(ctx, b.tempDir)
		require.NoError(b.t, err)
		preTest := func(f fs.Fs, testfilename string) string {
			in := bytes.NewBufferString(testfilename)
			objinfo := object.NewStaticObjectInfo(testfilename, initDate, int64(len(testfilename)), true, nil, nil)
			obj, err := f.Put(ctx, in, objinfo)
			if err != nil {
				b.t.Skipf("Fs is incapable of running test, skipping: %s (expected: \n%s (%s) actual: \n%s (%v))\n (fs: %s) \n", b.testCase, testfilename, detectEncoding(testfilename), "upload failed", err, f)
			}
			entries, err := f.List(ctx, "")
			assert.NoError(b.t, err)
			if entries.Len() == 1 && entries[0].Remote() != testfilename {
				diffStr, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{A: []string{testfilename}, B: []string{entries[0].Remote()}})
				// we can still deal with this as long as both remotes auto-convert the same way.
				b.t.Logf("Warning: this remote seems to auto-convert special characters (testcase: %s) (expected: \n%s (%s) actual: \n%s (%s))\n (fs: %s) \n%v", b.testCase, testfilename, detectEncoding(testfilename), entries[0].Remote(), detectEncoding(entries[0].Remote()), f, diffStr)
			}
			// test whether we can fix-case
			ctxFixCase, ci := fs.AddConfig(ctx)
			ci.FixCase = true
			transformedName := strings.ToLower(obj.Remote())
			src, err := tempDir.Put(ctx, in, object.NewStaticObjectInfo(transformedName, initDate, int64(len(transformedName)), true, nil, nil)) // local
			require.NoError(b.t, err)
			upperObj, err := operations.Copy(ctxFixCase, f, nil, transformedName, src)
			if err != nil || upperObj.Remote() != transformedName {
				b.t.Skipf("Fs is incapable of running test as can't fix-case, skipping: %s (expected: \n%s (%s) actual: \n%s (%v))\n (fs: %s) \n", b.testCase, transformedName, detectEncoding(transformedName), upperObj.Remote(), err, f)
			}
			require.NoError(b.t, src.Remove(ctx))
			require.NoError(b.t, obj.Remove(ctx))
			if obj != nil {
				require.NoError(b.t, upperObj.Remove(ctx))
			}
			return entries[0].Remote()
		}
		got1 := preTest(b.fs1, testfilename1)
		got1 += preTest(b.fs1, testfilename2)
		if b.fs1.Name() != b.fs2.Name() {
			got2 := preTest(b.fs2, testfilename1)
			got2 += preTest(b.fs2, testfilename2)
			if got1 != got2 {
				diffStr, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{A: []string{got1}, B: []string{got2}})
				b.t.Skipf("Fs is incapable of running test as the paths produce different results, skipping: %s (path1: \n%s (%s) path2: \n%s (%s))\n (fs1: %s fs2: %s) \n%v", b.testCase, got1, detectEncoding(got1), got2, got2, b.fs1, b.fs2, diffStr)
			}
		}
	}
	return ctx, opt
}

func (b *bisyncTest) runBisync(ctx context.Context, args []string) (err error) {
	opt := &bisync.Options{
		Workdir:       b.workDir,
		NoCleanup:     true,
		SaveQueues:    true,
		MaxDelete:     bisync.DefaultMaxDelete,
		CheckFilename: bisync.DefaultCheckFilename,
		CheckSync:     bisync.CheckSyncTrue,
		TestFn:        b.TestFn,
	}
	ctx, opt = b.checkPreReqs(ctx, opt)
	octx, ci := fs.AddConfig(ctx)
	fs1, fs2 := b.fs1, b.fs2

	addSubdir := func(path, subdir string) fs.Fs {
		remote := path + subdir
		f, err := cache.Get(ctx, remote)
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
		case "create-empty-src-dirs":
			opt.CreateEmptySrcDirs = true
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
		case "ignore-size":
			ci.IgnoreSize = true
		case "checksum":
			ci.CheckSum = true
			opt.Compare.DownloadHash = true // allows us to test crypt and the like
		case "compare-all":
			opt.CompareFlag = "size,modtime,checksum"
			opt.Compare.DownloadHash = true // allows us to test crypt and the like
		case "nomodtime":
			ci.CheckSum = true
			opt.CompareFlag = "size,checksum"
			opt.Compare.DownloadHash = true // allows us to test crypt and the like
		case "subdir":
			fs1 = addSubdir(b.replaceHex(b.path1), val)
			fs2 = addSubdir(b.replaceHex(b.path2), val)
		case "backupdir1":
			opt.BackupDir1 = val
		case "backupdir2":
			opt.BackupDir2 = val
		case "ignore-listing-checksum":
			opt.IgnoreListingChecksum = true
		case "no-norm":
			ci.NoUnicodeNormalization = true
			ci.IgnoreCaseSync = false
		case "norm":
			ci.NoUnicodeNormalization = false
			ci.IgnoreCaseSync = true
		case "fix-case":
			ci.NoUnicodeNormalization = false
			ci.IgnoreCaseSync = true
			ci.FixCase = true
		case "conflict-resolve":
			_ = opt.ConflictResolve.Set(val)
		case "conflict-loser":
			_ = opt.ConflictLoser.Set(val)
		case "conflict-suffix":
			opt.ConflictSuffixFlag = val
		case "resync-mode":
			_ = opt.ResyncMode.Set(val)
		default:
			return fmt.Errorf("invalid bisync option %q", arg)
		}
	}

	// set all dirs to a fixed date for test stability, as they are considered as of v1.66.
	jamDirTimes := func(f fs.Fs) {
		if f.Features().DirSetModTime == nil && f.Features().MkdirMetadata == nil {
			fs.Debugf(f, "Skipping jamDirTimes as remote does not support DirSetModTime or MkdirMetadata")
			return
		}
		err := walk.ListR(ctx, f, "", true, -1, walk.ListDirs, func(entries fs.DirEntries) error {
			var err error
			entries.ForDir(func(dir fs.Directory) {
				_, err = operations.SetDirModTime(ctx, f, dir, "", initDate)
			})
			return err
		})
		assert.NoError(b.t, err, "error jamming dirtimes")
	}
	jamDirTimes(fs1)
	jamDirTimes(fs2)

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
	fs.Debugf(nil, "copyFile %q to %q as %q", src, dst, asName)
	var fsrc, fdst fs.Fs
	var srcPath, srcFile, dstPath, dstFile string
	src = b.replaceHex(src)
	dst = b.replaceHex(dst)

	switch fsrc, err = fs.NewFs(ctx, src); err { // intentionally using NewFs here to avoid dircaching the parent
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
	if fdst, err = fs.NewFs(ctx, dstPath); err != nil { // intentionally using NewFs here to avoid dircaching the parent
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
	fs.Debugf(nil, "operations.CopyFile %q to %q as %q", srcFile, fdst.String(), dstFile)
	return operations.CopyFile(fctx, fdst, fsrc, dstFile, srcFile)
}

// listSubdirs is equivalent to `rclone lsf -R [--dirs-only]`
func (b *bisyncTest) listSubdirs(ctx context.Context, remote string, DirsOnly bool) error {
	f, err := cache.Get(ctx, remote)
	if err != nil {
		return err
	}

	opt := operations.ListJSONOpt{
		NoModTime:  true,
		NoMimeType: true,
		DirsOnly:   DirsOnly,
		Recurse:    true,
	}
	fmt := operations.ListFormat{}
	fmt.SetDirSlash(true)
	fmt.AddPath()
	printItem := func(item *operations.ListJSONItem) error {
		b.logPrintf("%s - filename hash: %s", fmt.Format(item), stringToHash(item.Name))
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
	if f.Precision() == fs.ModTimeNotSupported {
		return files, nil
	}

	date, err := time.ParseInLocation(touchDateFormat, dateStr, bisync.TZ)
	if err != nil {
		return files, fmt.Errorf("invalid date %q: %w", dateStr, err)
	}

	matcher, firstErr := filter.GlobPathToRegexp(glob, false)
	if firstErr != nil {
		return files, fmt.Errorf("invalid glob %q", glob)
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
		if err == fs.ErrorCantSetModTimeWithoutDelete || err == fs.ErrorCantSetModTime {
			// Workaround for dropbox, similar to --refresh-times
			err = nil
			buf := new(bytes.Buffer)
			size := obj.Size()
			separator := ""
			if size > 0 {
				filterCtx, fi := filter.AddConfig(ctx)
				err = fi.AddFile(remote) // limit Cat to only this file, not all files in dir
				if err != nil {
					return files, err
				}
				err = operations.Cat(filterCtx, f, buf, 0, size, []byte(separator))
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
		fs.Log(nil, divider)
		fs.Log(nil, color(terminal.RedFg, "MISCOMPARE - Number of Golden and Results files do not match:"))
		fs.Logf(nil, "  Golden count: %d", goldenNum)
		fs.Logf(nil, "  Result count: %d", resultNum)
		fs.Logf(nil, "  Golden files: %s", strings.Join(goldenFiles, ", "))
		fs.Logf(nil, "  Result files: %s", strings.Join(resultFiles, ", "))
	}

	for _, file := range goldenFiles {
		if !resultSet.Has(file) {
			errorCount++
			fs.Logf(nil, "  File found in Golden but not in Results:  %s", file)
		}
	}
	for _, file := range resultFiles {
		if !goldenSet.Has(file) {
			errorCount++
			fs.Logf(nil, "  File found in Results but not in Golden:  %s", file)
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
			require.NoError(b.t, os.WriteFile(goldenFile, []byte(goldenText), bilib.PermSecure))
			require.NoError(b.t, os.WriteFile(resultFile, []byte(resultText), bilib.PermSecure))
		}

		if goldenText == resultText || strings.Contains(resultText, ".DS_Store") {
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

		fs.Log(nil, divider)
		fs.Logf(nil, color(terminal.RedFg, "| MISCOMPARE  -Golden vs +Results for  %s"), file)
		for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
			fs.Logf(nil, "| %s", strings.TrimSpace(line))
		}
	}

	if errorCount > 0 {
		fs.Log(nil, divider)
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
	b.generateDebuggers()
	// Perform consistency checks
	files := b.listDir(b.workDir)
	require.NotEmpty(b.t, files, "nothing to store in golden dir")

	// Pass 1: validate files before storing
	for _, fileName := range files {
		if fileType(fileName) == "lock" {
			continue
		}
		if fileName == "backupdirs" {
			fs.Logf(nil, "skipping: %v", fileName)
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
		if fileName == "backupdirs" {
			fs.Logf(nil, "skipping: %v", fileName)
			continue
		}
		text := b.mangleResult(b.goldenDir, fileName, true)

		goldName := b.toGolden(fileName)
		goldPath := filepath.Join(b.goldenDir, goldName)
		err := os.WriteFile(goldPath, []byte(text), bilib.PermSecure)
		assert.NoError(b.t, err, "writing golden file %s", goldName)

		if goldName != fileName {
			origPath := filepath.Join(b.goldenDir, fileName)
			assert.NoError(b.t, os.Remove(origPath), "removing original file %s", fileName)
		}
	}
}

// mangleResult prepares test logs or listings for comparison
func (b *bisyncTest) mangleResult(dir, file string, golden bool) string {
	if file == "backupdirs" {
		return "skipping backupdirs"
	}
	buf, err := os.ReadFile(filepath.Join(dir, file))
	require.NoError(b.t, err)

	// normalize unicode so tets are runnable on macOS
	buf = norm.NFC.Bytes(buf)

	text := string(buf)

	switch fileType(strings.TrimSuffix(file, ".sav")) {
	case "queue":
		lines := strings.Split(text, eol)
		sort.Strings(lines)
		for i, line := range lines {
			lines[i] = normalizeEncoding(line)
		}
		return joinLines(lines)
	case "listing":
		return b.mangleListing(text, golden, file)
	case "log":
		// fall thru
	default:
		return text
	}

	// Adapt log lines to the golden way.
	// First replace filenames with whitespace
	// some backends (such as crypt) log them on multiple lines due to encoding differences, while others (local) do not
	wsrep := []string{
		"subdir with" + eol + "white space.txt/file2 with" + eol + "white space.txt", "subdir with white space.txt/file2 with white space.txt",
		"with\nwhite space", "with white space",
		"with\u0090white space", "with white space",
	}
	whitespaceJoiner := strings.NewReplacer(wsrep...)
	s := whitespaceJoiner.Replace(string(buf))

	lines := strings.Split(s, eol)
	pathReplacer := b.newReplacer(true)

	if b.fs1.Hashes() == hash.Set(hash.None) || b.fs2.Hashes() == hash.Set(hash.None) {
		logReplacements = append(logReplacements, `^.*{hashtype} differ.*$`, dropMe)
	}
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
func (b *bisyncTest) mangleListing(text string, golden bool, file string) string {
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

	// parse whether this is Path1 or Path2 (so we can apply per-Fs precision/hash settings)
	isPath1 := strings.Contains(file, ".path1.lst")
	f := b.fs2
	if isPath1 {
		f = b.fs1
	}

	// account for differences in backend features when comparing
	if !golden {
		for i, s := range lines {
			// Store hash as golden but ignore when comparing (only if no md5 support).
			match := regex.FindStringSubmatch(strings.TrimSpace(s))
			if match != nil && match[2] != "-" && (!b.fs1.Hashes().Contains(hash.MD5) || !b.fs2.Hashes().Contains(hash.MD5)) { // if hash is not empty and either side lacks md5
				lines[i] = match[1] + "-" + match[3] + match[4] // replace it with "-" for comparison purposes (see #5679)
			}
			// account for modtime precision
			lineRegex := regexp.MustCompile(`^(\S) +(-?\d+) (\S+) (\S+) (\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{9}[+-]\d{4}) (".+")$`)
			const timeFormat = "2006-01-02T15:04:05.000000000-0700"
			const lineFormat = "%s %8d %s %s %s %q\n"
			fields := lineRegex.FindStringSubmatch(strings.TrimSuffix(lines[i], "\n"))
			if fields != nil {
				sizeVal, sizeErr := strconv.ParseInt(fields[2], 10, 64)
				if sizeErr == nil {
					// account for filename encoding differences by normalizing to OS encoding
					fields[6] = normalizeEncoding(fields[6])
					timeStr := fields[5]
					if f.Precision() == fs.ModTimeNotSupported || b.ignoreModtime {
						lines[i] = fmt.Sprintf(lineFormat, fields[1], sizeVal, fields[3], fields[4], "-", fields[6])
						continue
					}
					timeVal, timeErr := time.ParseInLocation(timeFormat, timeStr, bisync.TZ)
					if timeErr == nil {
						timeRound := timeVal.Round(f.Precision() * 2)
						lines[i] = fmt.Sprintf(lineFormat, fields[1], sizeVal, fields[3], fields[4], timeRound, fields[6])
					}
				}
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
			"{path1/}", b.replaceHex(b.path1),
			"{path2/}", b.replaceHex(b.path2),
			"{session}", b.sessionName,
			"{/}", slash,
		}
		return strings.NewReplacer(rep...)
	}

	rep := []string{
		b.dataDir + slash, "{datadir/}",
		b.testDir + slash, "{testdir/}",
		b.workDir + slash, "{workdir/}",
		b.fs1.String(), "{path1String}",
		b.fs2.String(), "{path2String}",
		b.path1, "{path1/}",
		b.path2, "{path2/}",
		b.replaceHex(b.path1), "{path1/}",
		b.replaceHex(b.path2), "{path2/}",
		"//?/" + strings.TrimSuffix(strings.ReplaceAll(b.path1, slash, "/"), "/"), "{path1}", // fix windows-specific issue
		"//?/" + strings.TrimSuffix(strings.ReplaceAll(b.path2, slash, "/"), "/"), "{path2}",
		strings.TrimSuffix(b.path1, slash), "{path1}", // ensure it's still recognized without trailing slash
		strings.TrimSuffix(b.path2, slash), "{path2}",
		b.workDir, "{workdir}",
		b.sessionName, "{session}",
	}
	// convert all hash types to "{hashtype}"
	for _, ht := range hash.Supported().Array() {
		rep = append(rep, ht.String(), "{hashtype}")
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

	// normalize unicode so tets are runnable on macOS
	name = norm.NFC.String(name)

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
	files, err := os.ReadDir(dir)
	require.NoError(b.t, err)
	ignoreIt := func(file string) bool {
		ignoreList := []string{
			// ".lst-control", ".lst-dry-control", ".lst-old", ".lst-dry-old",
			".DS_Store",
		}
		for _, s := range ignoreList {
			if strings.Contains(file, s) {
				return true
			}
		}
		return false
	}
	for _, file := range files {
		if ignoreIt(file.Name()) {
			continue
		}
		names = append(names, filepath.Base(norm.NFC.String(file.Name())))
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
	case ".lst", ".lst-new", ".lst-err", ".lst-dry", ".lst-dry-new", ".lst-old", ".lst-dry-old", ".lst-control", ".lst-dry-control":
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
	fs.Log(nil, line)
	if b.logFile != nil {
		_, err := fmt.Fprintln(b.logFile, line)
		require.NoError(b.t, err, "writing log file")
	}
}

// account for filename encoding differences between remotes by normalizing to OS encoding
func normalizeEncoding(s string) string {
	if s == "" || s == "." {
		return s
	}
	nameVal, err := strconv.Unquote(s)
	if err != nil {
		nameVal = s
	}
	nameVal = filepath.Clean(nameVal)
	nameVal = encoder.OS.FromStandardPath(nameVal)
	return strconv.Quote(encoder.OS.ToStandardPath(filepath.ToSlash(nameVal)))
}

func stringToHash(s string) string {
	ht := hash.MD5
	hasher, err := hash.NewMultiHasherTypes(hash.NewHashSet(ht))
	if err != nil {
		fs.Errorf(s, "hash unsupported: %v", err)
	}

	_, err = hasher.Write([]byte(s))
	if err != nil {
		fs.Errorf(s, "failed to write to hasher: %v", err)
	}

	sum, err := hasher.SumString(ht, false)
	if err != nil {
		fs.Errorf(s, "hasher returned an error: %v", err)
	}
	return sum
}

func detectEncoding(s string) string {
	if norm.NFC.IsNormalString(s) && norm.NFD.IsNormalString(s) {
		return "BOTH"
	}
	if !norm.NFC.IsNormalString(s) && norm.NFD.IsNormalString(s) {
		return "NFD"
	}
	if norm.NFC.IsNormalString(s) && !norm.NFD.IsNormalString(s) {
		return "NFC"
	}
	return "OTHER"
}

// filters out those pesky macOS .DS_Store files, which are forbidden on Dropbox and just generally annoying
func ctxNoDsStore(ctx context.Context, t *testing.T) (context.Context, *filter.Filter) {
	ctxNoDsStore, fi := filter.AddConfig(ctx)
	err := fi.AddRule("- .DS_Store")
	require.NoError(t, err)
	err = fi.AddRule("- *.partial")
	require.NoError(t, err)
	err = fi.AddRule("+ **")
	require.NoError(t, err)
	return ctxNoDsStore, fi
}

func checkError(t *testing.T, err error, msgAndArgs ...interface{}) {
	if errors.Is(err, fs.ErrorCantUploadEmptyFiles) {
		t.Skipf("Skip test because remote cannot upload empty files")
	}
	assert.NoError(t, err, msgAndArgs...)
}

// for example, replaces TestS3{juk_h}:dir with TestS3,directory_markers=true:dir
// because NewFs needs the latter
func (b *bisyncTest) replaceHex(remote string) string {
	if bilib.HasHexString(remote) {
		remote = strings.ReplaceAll(remote, fs.ConfigString(b.parent1), fs.ConfigStringFull(b.parent1))
		remote = strings.ReplaceAll(remote, fs.ConfigString(b.parent2), fs.ConfigStringFull(b.parent2))
	}
	return remote
}
