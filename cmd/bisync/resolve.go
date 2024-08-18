package bisync

import (
	"context"
	"fmt"
	"math"
	"mime"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/terminal"
)

// Prefer describes strategies for resolving sync conflicts
type Prefer = fs.Enum[preferChoices]

// Supported --conflict-resolve strategies
const (
	PreferNone Prefer = iota
	PreferPath1
	PreferPath2
	PreferNewer
	PreferOlder
	PreferLarger
	PreferSmaller
)

type preferChoices struct{}

func (preferChoices) Choices() []string {
	return []string{
		PreferNone:    "none",
		PreferNewer:   "newer",
		PreferOlder:   "older",
		PreferLarger:  "larger",
		PreferSmaller: "smaller",
		PreferPath1:   "path1",
		PreferPath2:   "path2",
	}
}

func (preferChoices) Type() string {
	return "string"
}

// ConflictResolveList is a list of --conflict-resolve flag choices used in the help
var ConflictResolveList = Opt.ConflictResolve.Help()

// ConflictLoserAction describes possible actions to take on the loser of a sync conflict
type ConflictLoserAction = fs.Enum[conflictLoserChoices]

// Supported --conflict-loser actions
const (
	ConflictLoserSkip     ConflictLoserAction = iota // Reserved as zero but currently unused
	ConflictLoserNumber                              // file.conflict1, file.conflict2, file.conflict3, etc.
	ConflictLoserPathname                            // file.path1, file.path2
	ConflictLoserDelete                              // delete the loser, keep winner only
)

type conflictLoserChoices struct{}

func (conflictLoserChoices) Choices() []string {
	return []string{
		ConflictLoserNumber:   "num",
		ConflictLoserPathname: "pathname",
		ConflictLoserDelete:   "delete",
	}
}

func (conflictLoserChoices) Type() string {
	return "ConflictLoserAction"
}

// ConflictLoserList is a list of --conflict-loser flag choices used in the help
var ConflictLoserList = Opt.ConflictLoser.Help()

func (b *bisyncRun) setResolveDefaults(ctx context.Context) error {
	if b.opt.ConflictLoser == ConflictLoserSkip {
		b.opt.ConflictLoser = ConflictLoserNumber
	}
	if b.opt.ConflictSuffixFlag == "" {
		b.opt.ConflictSuffixFlag = "conflict"
	}
	suffixes := strings.Split(b.opt.ConflictSuffixFlag, ",")
	if len(suffixes) == 1 {
		b.opt.ConflictSuffix1 = suffixes[0]
		b.opt.ConflictSuffix2 = suffixes[0]
	} else if len(suffixes) == 2 {
		b.opt.ConflictSuffix1 = suffixes[0]
		b.opt.ConflictSuffix2 = suffixes[1]
	} else {
		return fmt.Errorf("--conflict-suffix cannot have more than 2 comma-separated values. Received %v: %v", len(suffixes), suffixes)
	}
	// replace glob variables, if any
	t := time.Now() // capture static time here so it is the same for all files throughout this run
	b.opt.ConflictSuffix1 = bilib.AppyTimeGlobs(b.opt.ConflictSuffix1, t)
	b.opt.ConflictSuffix2 = bilib.AppyTimeGlobs(b.opt.ConflictSuffix2, t)

	// append dot (intentionally allow more than one)
	b.opt.ConflictSuffix1 = "." + b.opt.ConflictSuffix1
	b.opt.ConflictSuffix2 = "." + b.opt.ConflictSuffix2

	// checks and warnings
	if (b.opt.ConflictResolve == PreferNewer || b.opt.ConflictResolve == PreferOlder) && (b.fs1.Precision() == fs.ModTimeNotSupported || b.fs2.Precision() == fs.ModTimeNotSupported) {
		fs.Logf(nil, Color(terminal.YellowFg, "WARNING: ignoring --conflict-resolve %s as at least one remote does not support modtimes."), b.opt.ConflictResolve.String())
		b.opt.ConflictResolve = PreferNone
	} else if (b.opt.ConflictResolve == PreferNewer || b.opt.ConflictResolve == PreferOlder) && !b.opt.Compare.Modtime {
		fs.Logf(nil, Color(terminal.YellowFg, "WARNING: ignoring --conflict-resolve %s as --compare does not include modtime."), b.opt.ConflictResolve.String())
		b.opt.ConflictResolve = PreferNone
	}
	if (b.opt.ConflictResolve == PreferLarger || b.opt.ConflictResolve == PreferSmaller) && !b.opt.Compare.Size {
		fs.Logf(nil, Color(terminal.YellowFg, "WARNING: ignoring --conflict-resolve %s as --compare does not include size."), b.opt.ConflictResolve.String())
		b.opt.ConflictResolve = PreferNone
	}

	return nil
}

type (
	renames map[string]renamesInfo // [originalName]newName (remember the originalName may have an alias)
	// the newName may be the same as the old name (if winner), but should not be blank, unless we're deleting.
	// the oldNames may not match each other, if we're normalizing case or unicode
	// all names should be "remotes" (relative names, without base path)
	renamesInfo struct {
		path1 namePair
		path2 namePair
	}
)
type namePair struct {
	oldName string
	newName string
}

func (b *bisyncRun) resolve(ctxMove context.Context, path1, path2, file, alias string, renameSkipped, copy1to2, copy2to1 *bilib.Names, ds1, ds2 *deltaSet) error {
	winningPath := 0
	if b.opt.ConflictResolve != PreferNone {
		winningPath = b.conflictWinner(ds1, ds2, file, alias)
		if winningPath > 0 {
			fs.Infof(file, Color(terminal.GreenFg, "The winner is: Path%d"), winningPath)
		} else {
			fs.Infof(file, Color(terminal.RedFg, "A winner could not be determined.")) //nolint:govet
		}
	}

	suff1 := b.opt.ConflictSuffix1 // copy to new var to make sure our changes here don't persist
	suff2 := b.opt.ConflictSuffix2
	if b.opt.ConflictLoser == ConflictLoserPathname && b.opt.ConflictSuffix1 == b.opt.ConflictSuffix2 {
		// numerate, but not if user supplied two different suffixes
		suff1 += "1"
		suff2 += "2"
	}

	r := renamesInfo{
		path1: namePair{
			oldName: file,
			newName: SuffixName(ctxMove, file, suff1),
		},
		path2: namePair{
			oldName: alias,
			newName: SuffixName(ctxMove, alias, suff2),
		},
	}

	// handle auto-numbering
	// note that we still queue copies for both files, whether or not we renamed
	// we also set these for ConflictLoserDelete in case there is no winner.
	if b.opt.ConflictLoser == ConflictLoserNumber || b.opt.ConflictLoser == ConflictLoserDelete {
		num := b.numerate(ctxMove, 1, file, alias)
		switch winningPath {
		case 1: // keep path1, rename path2
			r.path1.newName = r.path1.oldName
			r.path2.newName = SuffixName(ctxMove, r.path2.oldName, b.opt.ConflictSuffix2+fmt.Sprint(num))
		case 2: // keep path2, rename path1
			r.path1.newName = SuffixName(ctxMove, r.path1.oldName, b.opt.ConflictSuffix1+fmt.Sprint(num))
			r.path2.newName = r.path2.oldName
		default: // no winner, so rename both to different numbers (unless suffixes are already different)
			if b.opt.ConflictSuffix1 == b.opt.ConflictSuffix2 {
				r.path1.newName = SuffixName(ctxMove, r.path1.oldName, b.opt.ConflictSuffix1+fmt.Sprint(num))
				// let's just make sure num + 1 is available...
				num2 := b.numerate(ctxMove, num+1, file, alias)
				r.path2.newName = SuffixName(ctxMove, r.path2.oldName, b.opt.ConflictSuffix2+fmt.Sprint(num2))
			} else {
				// suffixes are different, so numerate independently
				num = b.numerateSingle(ctxMove, 1, file, alias, 1)
				r.path1.newName = SuffixName(ctxMove, r.path1.oldName, b.opt.ConflictSuffix1+fmt.Sprint(num))
				num = b.numerateSingle(ctxMove, 1, file, alias, 2)
				r.path2.newName = SuffixName(ctxMove, r.path2.oldName, b.opt.ConflictSuffix2+fmt.Sprint(num))
			}
		}
	}

	// when winningPath == 0 (no winner), we ignore settings and rename both, do not delete
	// note also that deletes and renames are mutually exclusive -- we never delete one path and rename the other.
	if b.opt.ConflictLoser == ConflictLoserDelete && winningPath == 1 {
		// delete 2, copy 1 to 2
		err = b.delete(ctxMove, r.path2, path2, path1, b.fs2, 2, 1, renameSkipped)
		if err != nil {
			return err
		}
		r.path2.newName = ""
		// copy the one that wasn't deleted
		b.indent("Path1", r.path1.oldName, "Queue copy to Path2")
		copy1to2.Add(r.path1.oldName)
	} else if b.opt.ConflictLoser == ConflictLoserDelete && winningPath == 2 {
		// delete 1, copy 2 to 1
		err = b.delete(ctxMove, r.path1, path1, path2, b.fs1, 1, 2, renameSkipped)
		if err != nil {
			return err
		}
		r.path1.newName = ""
		// copy the one that wasn't deleted
		b.indent("Path2", r.path2.oldName, "Queue copy to Path1")
		copy2to1.Add(r.path2.oldName)
	} else {
		err = b.rename(ctxMove, r.path1, path1, path2, b.fs1, 1, 2, winningPath, copy1to2, renameSkipped)
		if err != nil {
			return err
		}
		err = b.rename(ctxMove, r.path2, path2, path1, b.fs2, 2, 1, winningPath, copy2to1, renameSkipped)
		if err != nil {
			return err
		}
	}

	b.renames[r.path1.oldName] = r // note map index is path1's oldName, which may be different from path2 if aliases
	return nil
}

// SuffixName adds the current --conflict-suffix to the remote, obeying
// --suffix-keep-extension if set
// It is a close cousin of operations.SuffixName, but we don't want to
// use ci.Suffix for this because it might be used for --backup-dir.
func SuffixName(ctx context.Context, remote, suffix string) string {
	if suffix == "" {
		return remote
	}
	ci := fs.GetConfig(ctx)
	if ci.SuffixKeepExtension {
		var (
			base  = remote
			exts  = ""
			first = true
			ext   = path.Ext(remote)
		)
		for ext != "" {
			// Look second and subsequent extensions in mime types.
			// If they aren't found then don't keep it as an extension.
			if !first && mime.TypeByExtension(ext) == "" {
				break
			}
			base = base[:len(base)-len(ext)]
			exts = ext + exts
			first = false
			ext = path.Ext(base)
		}
		return base + suffix + exts
	}
	return remote + suffix
}

// NotEmpty checks whether set is not empty
func (r renames) NotEmpty() bool {
	return len(r) > 0
}

func (ri *renamesInfo) getNames(is1to2 bool) (srcOldName, srcNewName, dstOldName, dstNewName string) {
	if is1to2 {
		return ri.path1.oldName, ri.path1.newName, ri.path2.oldName, ri.path2.newName
	}
	return ri.path2.oldName, ri.path2.newName, ri.path1.oldName, ri.path1.newName
}

// work out the lowest number that neither side has, return it for suffix
func (b *bisyncRun) numerate(ctx context.Context, startnum int, file, alias string) int {
	for i := startnum; i < math.MaxInt; i++ {
		iStr := fmt.Sprint(i)
		if !ls1.has(SuffixName(ctx, file, b.opt.ConflictSuffix1+iStr)) &&
			!ls1.has(SuffixName(ctx, alias, b.opt.ConflictSuffix1+iStr)) &&
			!ls2.has(SuffixName(ctx, file, b.opt.ConflictSuffix2+iStr)) &&
			!ls2.has(SuffixName(ctx, alias, b.opt.ConflictSuffix2+iStr)) {
			// make sure it still holds true with suffixes switched (it should)
			if !ls1.has(SuffixName(ctx, file, b.opt.ConflictSuffix2+iStr)) &&
				!ls1.has(SuffixName(ctx, alias, b.opt.ConflictSuffix2+iStr)) &&
				!ls2.has(SuffixName(ctx, file, b.opt.ConflictSuffix1+iStr)) &&
				!ls2.has(SuffixName(ctx, alias, b.opt.ConflictSuffix1+iStr)) {
				fs.Debugf(file, "The first available suffix is: %s", iStr)
				return i
			}
		}
	}
	return 0 // not really possible, as no one has 9223372036854775807 conflicts, and if they do, they have bigger problems
}

// like numerate, but consider only one side's suffix (for when suffixes are different)
func (b *bisyncRun) numerateSingle(ctx context.Context, startnum int, file, alias string, path int) int {
	lsA, lsB := ls1, ls2
	suffix := b.opt.ConflictSuffix1
	if path == 2 {
		lsA, lsB = ls2, ls1
		suffix = b.opt.ConflictSuffix2
	}
	for i := startnum; i < math.MaxInt; i++ {
		iStr := fmt.Sprint(i)
		if !lsA.has(SuffixName(ctx, file, suffix+iStr)) &&
			!lsA.has(SuffixName(ctx, alias, suffix+iStr)) &&
			!lsB.has(SuffixName(ctx, file, suffix+iStr)) &&
			!lsB.has(SuffixName(ctx, alias, suffix+iStr)) {
			fs.Debugf(file, "The first available suffix is: %s", iStr)
			return i
		}
	}
	return 0 // not really possible, as no one has 9223372036854775807 conflicts, and if they do, they have bigger problems
}

func (b *bisyncRun) rename(ctx context.Context, thisNamePair namePair, thisPath, thatPath string, thisFs fs.Fs, thisPathNum, thatPathNum, winningPath int, q, renameSkipped *bilib.Names) error {
	if winningPath == thisPathNum {
		b.indent(fmt.Sprintf("!Path%d", thisPathNum), thisPath+thisNamePair.newName, fmt.Sprintf("Not renaming Path%d copy, as it was determined the winner", thisPathNum))
	} else {
		skip := operations.SkipDestructive(ctx, thisNamePair.oldName, "rename")
		if !skip {
			b.indent(fmt.Sprintf("!Path%d", thisPathNum), thisPath+thisNamePair.newName, fmt.Sprintf("Renaming Path%d copy", thisPathNum))
			ctx = b.setBackupDir(ctx, thisPathNum) // in case already a file with new name
			if err = operations.MoveFile(ctx, thisFs, thisFs, thisNamePair.newName, thisNamePair.oldName); err != nil {
				err = fmt.Errorf("%s rename failed for %s: %w", thisPath, thisPath+thisNamePair.oldName, err)
				b.critical = true
				return err
			}
		} else {
			renameSkipped.Add(thisNamePair.oldName) // (due to dry-run, not equality)
		}
	}
	b.indent(fmt.Sprintf("!Path%d", thisPathNum), thatPath+thisNamePair.newName, fmt.Sprintf("Queue copy to Path%d", thatPathNum))
	q.Add(thisNamePair.newName)
	return nil
}

func (b *bisyncRun) delete(ctx context.Context, thisNamePair namePair, thisPath, thatPath string, thisFs fs.Fs, thisPathNum, thatPathNum int, renameSkipped *bilib.Names) error {
	skip := operations.SkipDestructive(ctx, thisNamePair.oldName, "delete")
	if !skip {
		b.indent(fmt.Sprintf("!Path%d", thisPathNum), thisPath+thisNamePair.oldName, fmt.Sprintf("Deleting Path%d copy", thisPathNum))
		ctx = b.setBackupDir(ctx, thisPathNum)
		ci := fs.GetConfig(ctx)
		var backupDir fs.Fs
		if ci.BackupDir != "" {
			backupDir, err = operations.BackupDir(ctx, thisFs, thisFs, thisNamePair.oldName)
			if err != nil {
				b.critical = true
				return err
			}
		}
		obj, err := thisFs.NewObject(ctx, thisNamePair.oldName)
		if err != nil {
			b.critical = true
			return err
		}
		if err = operations.DeleteFileWithBackupDir(ctx, obj, backupDir); err != nil {
			err = fmt.Errorf("%s delete failed for %s: %w", thisPath, thisPath+thisNamePair.oldName, err)
			b.critical = true
			return err
		}
	} else {
		renameSkipped.Add(thisNamePair.oldName) // (due to dry-run, not equality)
	}
	return nil
}

func (b *bisyncRun) conflictWinner(ds1, ds2 *deltaSet, remote1, remote2 string) int {
	switch b.opt.ConflictResolve {
	case PreferPath1:
		return 1
	case PreferPath2:
		return 2
	case PreferNewer, PreferOlder:
		t1, t2 := ds1.time[remote1], ds2.time[remote2]
		return b.resolveNewerOlder(t1, t2, remote1, remote2, b.opt.ConflictResolve)
	case PreferLarger, PreferSmaller:
		s1, s2 := ds1.size[remote1], ds2.size[remote2]
		return b.resolveLargerSmaller(s1, s2, remote1, remote2, b.opt.ConflictResolve)
	default:
		return 0
	}
}

// returns the winning path number, or 0 if winner can't be determined
func (b *bisyncRun) resolveNewerOlder(t1, t2 time.Time, remote1, remote2 string, prefer Prefer) int {
	if fs.GetModifyWindow(b.octx, b.fs1, b.fs2) == fs.ModTimeNotSupported {
		fs.Infof(remote1, "Winner cannot be determined as at least one path lacks modtime support.")
		return 0
	}
	if t1.IsZero() || t2.IsZero() {
		fs.Infof(remote1, "Winner cannot be determined as at least one modtime is missing. Path1: %v, Path2: %v", t1, t2)
		return 0
	}
	if t1.After(t2) {
		if prefer == PreferNewer {
			fs.Infof(remote1, "Path1 is newer. Path1: %v, Path2: %v, Difference: %s", t1.Local(), t2.Local(), t1.Sub(t2))
			return 1
		} else if prefer == PreferOlder {
			fs.Infof(remote1, "Path2 is older. Path1: %v, Path2: %v, Difference: %s", t1.Local(), t2.Local(), t1.Sub(t2))
			return 2
		}
	} else if t1.Before(t2) {
		if prefer == PreferNewer {
			fs.Infof(remote1, "Path2 is newer. Path1: %v, Path2: %v, Difference: %s", t1.Local(), t2.Local(), t2.Sub(t1))
			return 2
		} else if prefer == PreferOlder {
			fs.Infof(remote1, "Path1 is older. Path1: %v, Path2: %v, Difference: %s", t1.Local(), t2.Local(), t2.Sub(t1))
			return 1
		}
	}
	if t1.Equal(t2) {
		fs.Infof(remote1, "Winner cannot be determined as times are equal. Path1: %v, Path2: %v, Difference: %s", t1.Local(), t2.Local(), t2.Sub(t1))
		return 0
	}
	fs.Errorf(remote1, "Winner cannot be determined. Path1: %v, Path2: %v", t1.Local(), t2.Local()) // shouldn't happen unless prefer is of wrong type
	return 0
}

// returns the winning path number, or 0 if winner can't be determined
func (b *bisyncRun) resolveLargerSmaller(s1, s2 int64, remote1, remote2 string, prefer Prefer) int {
	if s1 < 0 || s2 < 0 {
		fs.Infof(remote1, "Winner cannot be determined as at least one size is unknown. Path1: %v, Path2: %v", s1, s2)
		return 0
	}
	if s1 > s2 {
		if prefer == PreferLarger {
			fs.Infof(remote1, "Path1 is larger. Path1: %v, Path2: %v, Difference: %v", s1, s2, s1-s2)
			return 1
		} else if prefer == PreferSmaller {
			fs.Infof(remote1, "Path2 is smaller. Path1: %v, Path2: %v, Difference: %v", s1, s2, s1-s2)
			return 2
		}
	} else if s1 < s2 {
		if prefer == PreferLarger {
			fs.Infof(remote1, "Path2 is larger. Path1: %v, Path2: %v, Difference: %v", s1, s2, s2-s1)
			return 2
		} else if prefer == PreferSmaller {
			fs.Infof(remote1, "Path1 is smaller. Path1: %v, Path2: %v, Difference: %v", s1, s2, s2-s1)
			return 1
		}
	}
	if s1 == s2 {
		fs.Infof(remote1, "Winner cannot be determined as sizes are equal. Path1: %v, Path2: %v, Difference: %v", s1, s2, s1-s2)
		return 0
	}
	fs.Errorf(remote1, "Winner cannot be determined. Path1: %v, Path2: %v", s1, s2) // shouldn't happen unless prefer is of wrong type
	return 0
}
