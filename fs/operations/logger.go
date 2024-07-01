package operations

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/spf13/pflag"
)

// Sigil represents the rune (-+=*!?) used by Logger to categorize files by their match/differ/missing status.
type Sigil rune

// String converts sigil to more human-readable string
func (sigil Sigil) String() string {
	switch sigil {
	case '-':
		return "MissingOnSrc"
	case '+':
		return "MissingOnDst"
	case '=':
		return "Match"
	case '*':
		return "Differ"
	case '!':
		return "Error"
	// case '.':
	// 	return "Completed"
	case '?':
		return "Other"
	}
	return "unknown"
}

// Writer directs traffic from sigil -> LoggerOpt.Writer
func (sigil Sigil) Writer(opt LoggerOpt) io.Writer {
	switch sigil {
	case '-':
		return opt.MissingOnSrc
	case '+':
		return opt.MissingOnDst
	case '=':
		return opt.Match
	case '*':
		return opt.Differ
	case '!':
		return opt.Error
	}
	return nil
}

// Sigil constants
const (
	MissingOnSrc  Sigil = '-'
	MissingOnDst  Sigil = '+'
	Match         Sigil = '='
	Differ        Sigil = '*'
	TransferError Sigil = '!'
	Other         Sigil = '?' // reserved but not currently used
)

// LoggerFn uses fs.DirEntry instead of fs.Object so it can include Dirs
// For LoggerFn example, see bisync.WriteResults() or sync.SyncLoggerFn()
// Usage example: s.logger(ctx, operations.Differ, src, dst, nil)
type LoggerFn func(ctx context.Context, sigil Sigil, src, dst fs.DirEntry, err error)
type loggerContextKey struct{}
type loggerOptContextKey struct{}

var loggerKey = loggerContextKey{}
var loggerOptKey = loggerOptContextKey{}

// LoggerOpt contains options for the Sync Logger functions
// TODO: refactor Check in here too?
type LoggerOpt struct {
	// Fdst, Fsrc   fs.Fs         // fses to check
	// Check        checkFn       // function to use for checking
	// OneWay       bool          // one way only?
	LoggerFn      LoggerFn      // function to use for logging
	Combined      io.Writer     // a file with file names with leading sigils
	MissingOnSrc  io.Writer     // files only in the destination
	MissingOnDst  io.Writer     // files only in the source
	Match         io.Writer     // matching files
	Differ        io.Writer     // differing files
	Error         io.Writer     // files with errors of some kind
	DestAfter     io.Writer     // files that exist on the destination post-sync
	JSON          *bytes.Buffer // used by bisync to read/write struct as JSON
	DeleteModeOff bool          //affects whether Logger expects MissingOnSrc to be deleted

	// lsf options for destAfter
	ListFormat ListFormat
	JSONOpt    ListJSONOpt
	LJ         *listJSON
	Format     string
	TimeFormat string
	Separator  string
	DirSlash   bool
	// Recurse   bool
	HashType  hash.Type
	FilesOnly bool
	DirsOnly  bool
	Csv       bool
	Absolute  bool
}

// WithLogger stores logger in ctx and returns a copy of ctx in which loggerKey = logger
func WithLogger(ctx context.Context, logger LoggerFn) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// WithLoggerOpt stores loggerOpt in ctx and returns a copy of ctx in which loggerOptKey = loggerOpt
func WithLoggerOpt(ctx context.Context, loggerOpt LoggerOpt) context.Context {
	return context.WithValue(ctx, loggerOptKey, loggerOpt)
}

// GetLogger attempts to retrieve LoggerFn from context, returns it if found, otherwise returns no-op function
func GetLogger(ctx context.Context) (LoggerFn, bool) {
	logger, ok := ctx.Value(loggerKey).(LoggerFn)
	if !ok {
		logger = func(ctx context.Context, sigil Sigil, src, dst fs.DirEntry, err error) {}
	}
	return logger, ok
}

// GetLoggerOpt attempts to retrieve LoggerOpt from context, returns it if found, otherwise returns NewLoggerOpt()
func GetLoggerOpt(ctx context.Context) LoggerOpt {
	loggerOpt, ok := ctx.Value(loggerOptKey).(LoggerOpt)
	if ok {
		return loggerOpt
	}
	return NewLoggerOpt()
}

// WithSyncLogger starts a new logger with the options passed in and saves it to ctx for retrieval later
func WithSyncLogger(ctx context.Context, opt LoggerOpt) context.Context {
	ctx = WithLoggerOpt(ctx, opt)
	return WithLogger(ctx, func(ctx context.Context, sigil Sigil, src, dst fs.DirEntry, err error) {
		if opt.LoggerFn != nil {
			opt.LoggerFn(ctx, sigil, src, dst, err)
		} else {
			SyncFprintf(opt.Combined, "%c %s\n", sigil, dst.Remote())
		}
	})
}

// NewLoggerOpt returns a new LoggerOpt struct with defaults
func NewLoggerOpt() LoggerOpt {
	opt := LoggerOpt{
		Combined:     new(bytes.Buffer),
		MissingOnSrc: new(bytes.Buffer),
		MissingOnDst: new(bytes.Buffer),
		Match:        new(bytes.Buffer),
		Differ:       new(bytes.Buffer),
		Error:        new(bytes.Buffer),
		DestAfter:    new(bytes.Buffer),
		JSON:         new(bytes.Buffer),
	}
	return opt
}

// Winner predicts which side (src or dst) should end up winning out on the dst.
type Winner struct {
	Obj  fs.DirEntry // the object that should exist on dst post-sync, if any
	Side string      // whether the winning object was from the src or dst
	Err  error       // whether there's an error preventing us from predicting winner correctly (not whether there was a sync error more generally)
}

// WinningSide can be called in a LoggerFn to predict what the dest will look like post-sync
//
// This attempts to account for every case in which dst (intentionally) does not match src after a sync.
//
// Known issues / cases we can't confidently predict yet:
//
//	--max-duration / CutoffModeHard
//	--compare-dest / --copy-dest (because equal() is called multiple times for the same file)
//	server-side moves of an entire dir at once (because we never get the individual file objects in the dir)
//	High-level retries, because there would be dupes (use --retries 1 to disable)
//	Possibly some error scenarios
func WinningSide(ctx context.Context, sigil Sigil, src, dst fs.DirEntry, err error) Winner {
	winner := Winner{nil, "none", nil}
	opt := GetLoggerOpt(ctx)
	ci := fs.GetConfig(ctx)

	if err == fs.ErrorIsDir {
		winner.Err = err
		if sigil == MissingOnSrc {
			if (opt.DeleteModeOff || ci.DryRun) && dst != nil {
				winner.Obj = dst
				winner.Side = "dst" // whatever's on dst will remain so after DryRun
				return winner
			}
			return winner // none, because dst should just get deleted
		}
		if sigil == MissingOnDst && ci.DryRun {
			return winner // none, because it does not currently exist on dst, and will still not exist after DryRun
		} else if ci.DryRun && dst != nil {
			winner.Obj = dst
			winner.Side = "dst"
		} else if src != nil {
			winner.Obj = src
			winner.Side = "src"
		}
		return winner
	}

	_, srcOk := src.(fs.Object)
	_, dstOk := dst.(fs.Object)
	if !srcOk && !dstOk {
		return winner // none, because we don't have enough info to continue.
	}

	switch sigil {
	case MissingOnSrc:
		if opt.DeleteModeOff || ci.DryRun { // i.e. it's a copy, not sync (or it's a DryRun)
			winner.Obj = dst
			winner.Side = "dst" // whatever's on dst will remain so after DryRun
			return winner
		}
		return winner // none, because dst should just get deleted
	case Match, Differ, MissingOnDst:
		if sigil == MissingOnDst && ci.DryRun {
			return winner // none, because it does not currently exist on dst, and will still not exist after DryRun
		}
		winner.Obj = src
		winner.Side = "src" // presume dst will end up matching src unless changed below
		if sigil == Match && (ci.SizeOnly || ci.CheckSum || ci.IgnoreSize || ci.UpdateOlder || ci.NoUpdateModTime) {
			winner.Obj = dst
			winner.Side = "dst" // ignore any differences with src because of user flags
		}
		if ci.IgnoreTimes {
			winner.Obj = src
			winner.Side = "src" // copy src to dst unconditionally
		}
		if (sigil == Match || sigil == Differ) && (ci.IgnoreExisting || ci.Immutable) {
			winner.Obj = dst
			winner.Side = "dst" // dst should remain unchanged if it already exists (and we know it does because it's Match or Differ)
		}
		if ci.DryRun {
			winner.Obj = dst
			winner.Side = "dst" // dst should remain unchanged after DryRun (note that we handled MissingOnDst earlier)
		}
		return winner
	case TransferError:
		winner.Obj = dst
		winner.Side = "dst" // usually, dst should not change if there's an error
		if dst == nil {
			winner.Obj = src
			winner.Side = "src" // but if for some reason we have a src and not a dst, go with it
		}
		if winner.Obj != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, errors.New("max transfer duration reached as set by --max-duration")) {
				winner.Err = err // we can't confidently predict what survives if CutoffModeHard
			}
			return winner // we know at least one of the objects
		}
	}
	// should only make it this far if it's TransferError and both src and dst are nil
	winner.Side = "none"
	winner.Err = fmt.Errorf("unknown case -- can't determine winner. %v", err)
	fs.Debugf(winner.Obj, "%v", winner.Err)
	return winner
}

// SetListFormat sets opt.ListFormat for destAfter
// TODO: possibly refactor duplicate code from cmd/lsf, where this is mostly copied from
func (opt *LoggerOpt) SetListFormat(ctx context.Context, cmdFlags *pflag.FlagSet) {
	// Work out if the separatorFlag was supplied or not
	separatorFlag := cmdFlags.Lookup("separator")
	separatorFlagSupplied := separatorFlag != nil && separatorFlag.Changed
	// Default the separator to , if using CSV
	if opt.Csv && !separatorFlagSupplied {
		opt.Separator = ","
	}

	var list ListFormat
	list.SetSeparator(opt.Separator)
	list.SetCSV(opt.Csv)
	list.SetDirSlash(opt.DirSlash)
	list.SetAbsolute(opt.Absolute)
	var JSONOpt = ListJSONOpt{
		NoModTime:  true,
		NoMimeType: true,
		DirsOnly:   opt.DirsOnly,
		FilesOnly:  opt.FilesOnly,
		// Recurse:    opt.Recurse,
	}

	for _, char := range opt.Format {
		switch char {
		case 'p':
			list.AddPath()
		case 't':
			list.AddModTime(opt.TimeFormat)
			JSONOpt.NoModTime = false
		case 's':
			list.AddSize()
		case 'h':
			list.AddHash(opt.HashType)
			JSONOpt.ShowHash = true
			JSONOpt.HashTypes = []string{opt.HashType.String()}
		case 'i':
			list.AddID()
		case 'm':
			list.AddMimeType()
			JSONOpt.NoMimeType = false
		case 'e':
			list.AddEncrypted()
			JSONOpt.ShowEncrypted = true
		case 'o':
			list.AddOrigID()
			JSONOpt.ShowOrigIDs = true
		case 'T':
			list.AddTier()
		case 'M':
			list.AddMetadata()
			JSONOpt.Metadata = true
		default:
			fs.Errorf(nil, "unknown format character %q", char)
		}
	}
	opt.ListFormat = list
	opt.JSONOpt = JSONOpt
}

// NewListJSON makes a new *listJSON for destAfter
func (opt *LoggerOpt) NewListJSON(ctx context.Context, fdst fs.Fs, remote string) {
	opt.LJ, _ = newListJSON(ctx, fdst, remote, &opt.JSONOpt)
	//fs.Debugf(nil, "%v", opt.LJ)
}

// JSONEntry returns a *ListJSONItem for destAfter
func (opt *LoggerOpt) JSONEntry(ctx context.Context, entry fs.DirEntry) (*ListJSONItem, error) {
	return opt.LJ.entry(ctx, entry)
}

// PrintDestAfter writes a *ListJSONItem to opt.DestAfter
func (opt *LoggerOpt) PrintDestAfter(ctx context.Context, sigil Sigil, src, dst fs.DirEntry, err error) {
	entry := WinningSide(ctx, sigil, src, dst, err)
	if entry.Obj != nil {
		JSONEntry, _ := opt.JSONEntry(ctx, entry.Obj)
		_, _ = fmt.Fprintln(opt.DestAfter, opt.ListFormat.Format(JSONEntry))
	}
}
