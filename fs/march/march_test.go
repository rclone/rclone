// Internal tests for march

package march

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/list"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/mockdir"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/unicode/norm"
)

// Some times used in the tests
var (
	t1 = fstest.Time("2001-02-03T04:05:06.499999999Z")
)

func TestMain(m *testing.M) {
	fstest.TestMain(m)
}

type marchTester struct {
	ctx        context.Context // internal context for controlling go-routines
	cancel     func()          // cancel the context
	srcOnly    fs.DirEntries
	dstOnly    fs.DirEntries
	match      fs.DirEntries
	entryMutex sync.Mutex
	errorMu    sync.Mutex // Mutex covering the error variables
	err        error
	noRetryErr error
	fatalErr   error
	noTraverse bool
}

// DstOnly have an object which is in the destination only
func (mt *marchTester) DstOnly(dst fs.DirEntry) (recurse bool) {
	mt.entryMutex.Lock()
	mt.dstOnly = append(mt.dstOnly, dst)
	mt.entryMutex.Unlock()

	switch dst.(type) {
	case fs.Object:
		return false
	case fs.Directory:
		return true
	default:
		panic("Bad object in DirEntries")
	}
}

// SrcOnly have an object which is in the source only
func (mt *marchTester) SrcOnly(src fs.DirEntry) (recurse bool) {
	mt.entryMutex.Lock()
	mt.srcOnly = append(mt.srcOnly, src)
	mt.entryMutex.Unlock()

	switch src.(type) {
	case fs.Object:
		return false
	case fs.Directory:
		return true
	default:
		panic("Bad object in DirEntries")
	}
}

// Match is called when src and dst are present, so sync src to dst
func (mt *marchTester) Match(ctx context.Context, dst, src fs.DirEntry) (recurse bool) {
	mt.entryMutex.Lock()
	mt.match = append(mt.match, src)
	mt.entryMutex.Unlock()

	switch src.(type) {
	case fs.Object:
		return false
	case fs.Directory:
		// Do the same thing to the entire contents of the directory
		_, ok := dst.(fs.Directory)
		if ok {
			return true
		}
		// FIXME src is dir, dst is file
		err := errors.New("can't overwrite file with directory")
		fs.Errorf(dst, "%v", err)
		mt.processError(err)
	default:
		panic("Bad object in DirEntries")
	}
	return false
}

func (mt *marchTester) processError(err error) {
	if err == nil {
		return
	}
	mt.errorMu.Lock()
	defer mt.errorMu.Unlock()
	switch {
	case fserrors.IsFatalError(err):
		if !mt.aborting() {
			fs.Errorf(nil, "Cancelling sync due to fatal error: %v", err)
			mt.cancel()
		}
		mt.fatalErr = err
	case fserrors.IsNoRetryError(err):
		mt.noRetryErr = err
	default:
		mt.err = err
	}
}

func (mt *marchTester) currentError() error {
	mt.errorMu.Lock()
	defer mt.errorMu.Unlock()
	if mt.fatalErr != nil {
		return mt.fatalErr
	}
	if mt.err != nil {
		return mt.err
	}
	return mt.noRetryErr
}

func (mt *marchTester) aborting() bool {
	return mt.ctx.Err() != nil
}

func TestMarch(t *testing.T) {
	for _, test := range []struct {
		what        string
		fileSrcOnly []string
		dirSrcOnly  []string
		fileDstOnly []string
		dirDstOnly  []string
		fileMatch   []string
		dirMatch    []string
		noTraverse  bool
		fastList    bool
	}{
		{
			what:        "source only",
			fileSrcOnly: []string{"test", "test2", "test3", "sub dir/test4"},
			dirSrcOnly:  []string{"sub dir"},
		},
		{
			what:      "identical",
			fileMatch: []string{"test", "test2", "sub dir/test3", "sub dir/sub sub dir/test4"},
			dirMatch:  []string{"sub dir", "sub dir/sub sub dir"},
		},
		{
			what:        "typical sync",
			fileSrcOnly: []string{"srcOnly", "srcOnlyDir/sub"},
			dirSrcOnly:  []string{"srcOnlyDir"},
			fileMatch:   []string{"match", "matchDir/match file"},
			dirMatch:    []string{"matchDir"},
			fileDstOnly: []string{"dstOnly", "dstOnlyDir/sub"},
			dirDstOnly:  []string{"dstOnlyDir"},
		},
		{
			what:        "no traverse source only",
			fileSrcOnly: []string{"test", "test2", "test3", "sub dir/test4"},
			dirSrcOnly:  []string{"sub dir"},
			noTraverse:  true,
		},
		{
			what:       "no traverse identical",
			fileMatch:  []string{"test", "test2", "sub dir/test3", "sub dir/sub sub dir/test4"},
			noTraverse: true,
		},
		{
			what:        "no traverse typical sync",
			fileSrcOnly: []string{"srcOnly", "srcOnlyDir/sub"},
			fileMatch:   []string{"match", "matchDir/match file"},
			noTraverse:  true,
		},
		{
			what:        "fast list source only",
			fileSrcOnly: []string{"test", "test2", "test3", "sub dir/test4"},
			dirSrcOnly:  []string{"sub dir"},
			fastList:    true,
		},
		{
			what:      "fast list identical",
			fileMatch: []string{"test", "test2", "sub dir/test3", "sub dir/sub sub dir/test4"},
			dirMatch:  []string{"sub dir", "sub dir/sub sub dir"},
			fastList:  true,
		},
		{
			what:        "fast list typical sync",
			fileSrcOnly: []string{"srcOnly", "srcOnlyDir/sub"},
			dirSrcOnly:  []string{"srcOnlyDir"},
			fileMatch:   []string{"match", "matchDir/match file"},
			dirMatch:    []string{"matchDir"},
			fileDstOnly: []string{"dstOnly", "dstOnlyDir/sub"},
			dirDstOnly:  []string{"dstOnlyDir"},
			fastList:    true,
		},
	} {
		t.Run(fmt.Sprintf("TestMarch-%s", test.what), func(t *testing.T) {
			r := fstest.NewRun(t)

			var srcOnly []fstest.Item
			var dstOnly []fstest.Item
			var match []fstest.Item

			ctx, cancel := context.WithCancel(context.Background())

			for _, f := range test.fileSrcOnly {
				srcOnly = append(srcOnly, r.WriteFile(f, "hello world", t1))
			}
			for _, f := range test.fileDstOnly {
				dstOnly = append(dstOnly, r.WriteObject(ctx, f, "hello world", t1))
			}
			for _, f := range test.fileMatch {
				match = append(match, r.WriteBoth(ctx, f, "hello world", t1))
			}

			ctx, ci := fs.AddConfig(ctx)
			ci.UseListR = test.fastList

			fi := filter.GetConfig(ctx)

			// Local backend doesn't implement ListR, so monkey patch it for this test
			if test.fastList && r.Flocal.Features().ListR == nil {
				r.Flocal.Features().ListR = func(ctx context.Context, dir string, callback fs.ListRCallback) error {
					r.Flocal.Features().ListR = nil // disable ListR to avoid infinite recursion
					return walk.ListR(ctx, r.Flocal, dir, true, -1, walk.ListAll, callback)
				}
				defer func() {
					r.Flocal.Features().ListR = nil
				}()
			}

			mt := &marchTester{
				ctx:        ctx,
				cancel:     cancel,
				noTraverse: test.noTraverse,
			}
			m := &March{
				Ctx:           ctx,
				Fdst:          r.Fremote,
				Fsrc:          r.Flocal,
				Dir:           "",
				NoTraverse:    test.noTraverse,
				Callback:      mt,
				DstIncludeAll: fi.Opt.DeleteExcluded,
			}

			mt.processError(m.Run(ctx))
			mt.cancel()
			err := mt.currentError()
			require.NoError(t, err)

			precision := fs.GetModifyWindow(ctx, r.Fremote, r.Flocal)
			fstest.CompareItems(t, mt.srcOnly, srcOnly, test.dirSrcOnly, precision, "srcOnly")
			fstest.CompareItems(t, mt.dstOnly, dstOnly, test.dirDstOnly, precision, "dstOnly")
			fstest.CompareItems(t, mt.match, match, test.dirMatch, precision, "match")
		})
	}
}

// matchPair is a matched pair of direntries returned by matchListings
type matchPair struct {
	src, dst fs.DirEntry
}

func TestMatchListings(t *testing.T) {
	var (
		a    = mockobject.Object("a")
		A    = mockobject.Object("A")
		b    = mockobject.Object("b")
		c    = mockobject.Object("c")
		d    = mockobject.Object("d")
		uE1  = mockobject.Object("é") // one of the unicode E characters
		uE2  = mockobject.Object("é")  // a different unicode E character
		dirA = mockdir.New("A")
		dirb = mockdir.New("b")
	)

	for _, test := range []struct {
		what       string
		input      fs.DirEntries // pairs of input src, dst
		srcOnly    fs.DirEntries
		dstOnly    fs.DirEntries
		matches    []matchPair // pairs of output
		transforms []matchTransformFn
	}{
		{
			what: "only src or dst",
			input: fs.DirEntries{
				a, nil,
				b, nil,
				c, nil,
				d, nil,
			},
			srcOnly: fs.DirEntries{
				a, b, c, d,
			},
		},
		{
			what: "typical sync #1",
			input: fs.DirEntries{
				a, nil,
				b, b,
				nil, c,
				nil, d,
			},
			srcOnly: fs.DirEntries{
				a,
			},
			dstOnly: fs.DirEntries{
				c, d,
			},
			matches: []matchPair{
				{b, b},
			},
		},
		{
			what: "typical sync #2",
			input: fs.DirEntries{
				a, a,
				b, b,
				nil, c,
				d, d,
			},
			dstOnly: fs.DirEntries{
				c,
			},
			matches: []matchPair{
				{a, a},
				{b, b},
				{d, d},
			},
		},
		{
			what: "One duplicate",
			input: fs.DirEntries{
				A, A,
				a, a,
				a, nil,
				b, b,
			},
			matches: []matchPair{
				{A, A},
				{a, a},
				{b, b},
			},
		},
		{
			what: "Two duplicates",
			input: fs.DirEntries{
				a, a,
				a, a,
				a, nil,
			},
			matches: []matchPair{
				{a, a},
			},
		},
		{
			what: "Case insensitive duplicate - no transform",
			input: fs.DirEntries{
				a, a,
				A, A,
			},
			matches: []matchPair{
				{A, A},
				{a, a},
			},
		},
		{
			what: "Case insensitive duplicate - transform to lower case",
			input: fs.DirEntries{
				a, A,
				A, a,
			},
			matches: []matchPair{
				{a, A}, // the first duplicate will be returned with a stable sort
			},
			transforms: []matchTransformFn{strings.ToLower},
		},
		{
			what: "Unicode near-duplicate that becomes duplicate with normalization",
			input: fs.DirEntries{
				uE1, uE1,
				uE2, uE2,
			},
			matches: []matchPair{
				{uE1, uE1},
			},
			transforms: []matchTransformFn{norm.NFC.String},
		},
		{
			what: "Unicode near-duplicate with no normalization",
			input: fs.DirEntries{
				uE1, uE1,
				uE2, uE2,
			},
			matches: []matchPair{
				{uE1, uE1},
				{uE2, uE2},
			},
		},
		{
			what: "File and directory are not duplicates - srcOnly",
			input: fs.DirEntries{
				dirA, nil,
				A, nil,
			},
			srcOnly: fs.DirEntries{
				dirA,
				A,
			},
		},
		{
			what: "File and directory are not duplicates - matches",
			input: fs.DirEntries{
				dirA, dirA,
				A, A,
			},
			matches: []matchPair{
				{dirA, dirA},
				{A, A},
			},
		},
		{
			what: "Sync with directory #1",
			input: fs.DirEntries{
				dirA, nil,
				A, nil,
				b, b,
				nil, c,
				nil, d,
			},
			srcOnly: fs.DirEntries{
				dirA,
				A,
			},
			dstOnly: fs.DirEntries{
				c, d,
			},
			matches: []matchPair{
				{b, b},
			},
		},
		{
			what: "Sync with 2 directories",
			input: fs.DirEntries{
				dirA, dirA,
				A, nil,
				nil, dirb,
				nil, b,
			},
			srcOnly: fs.DirEntries{
				A,
			},
			dstOnly: fs.DirEntries{
				dirb,
				b,
			},
			matches: []matchPair{
				{dirA, dirA},
			},
		},
		{
			what: "Sync with duplicate files and dirs",
			input: fs.DirEntries{
				dirA, A,
				A, dirA,
			},
			matches: []matchPair{
				{dirA, dirA},
				{A, A},
			},
		},
	} {
		t.Run(fmt.Sprintf("TestMatchListings-%s", test.what), func(t *testing.T) {
			ctx := context.Background()
			var wg sync.WaitGroup

			// Skeleton March for testing
			m := March{
				Ctx:        context.Background(),
				transforms: test.transforms,
			}

			// Make a channel to send the source (0) or dest (1) using a list.Sorter
			makeChan := func(offset int) <-chan fs.DirEntry {
				out := make(chan fs.DirEntry)
				key := m.dstKey
				if offset == 0 {
					key = m.srcKey
				}
				ls, err := list.NewSorter(ctx, nil, list.SortToChan(out), key)
				require.NoError(t, err)
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := 0; i < len(test.input); i += 2 {
						entry := test.input[i+offset]
						if entry != nil {
							require.NoError(t, ls.Add(fs.DirEntries{entry}))
						}
					}
					require.NoError(t, ls.Send())
					ls.CleanUp()
					close(out)
				}()
				return out
			}

			var srcOnly fs.DirEntries
			srcOnlyFn := func(entry fs.DirEntry) {
				srcOnly = append(srcOnly, entry)
			}
			var dstOnly fs.DirEntries
			dstOnlyFn := func(entry fs.DirEntry) {
				dstOnly = append(dstOnly, entry)
			}
			var matches []matchPair
			matchFn := func(dst, src fs.DirEntry) {
				matches = append(matches, matchPair{dst: dst, src: src})
			}

			err := m.matchListings(makeChan(0), makeChan(1), srcOnlyFn, dstOnlyFn, matchFn)
			require.NoError(t, err)
			wg.Wait()
			assert.Equal(t, test.srcOnly, srcOnly, test.what, "srcOnly differ")
			assert.Equal(t, test.dstOnly, dstOnly, test.what, "dstOnly differ")
			assert.Equal(t, test.matches, matches, test.what, "matches differ")

			// now swap src and dst
			srcOnly, dstOnly, matches = nil, nil, nil
			err = m.matchListings(makeChan(0), makeChan(1), srcOnlyFn, dstOnlyFn, matchFn)
			require.NoError(t, err)
			wg.Wait()
			assert.Equal(t, test.srcOnly, srcOnly, test.what, "srcOnly differ")
			assert.Equal(t, test.dstOnly, dstOnly, test.what, "dstOnly differ")
			assert.Equal(t, test.matches, matches, test.what, "matches differ")
		})
	}
}
