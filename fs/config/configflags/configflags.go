// Package configflags defines the flags used by rclone.  It is
// decoupled into a separate package so it can be replaced.
package configflags

// Options set by command line flags
import (
	"log"
	"net"
	"path/filepath"
	"strings"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config"
	"github.com/ncw/rclone/fs/config/flags"
	"github.com/ncw/rclone/fs/rc"
	"github.com/spf13/pflag"
)

var (
	// these will get interpreted into fs.Config via SetFlags() below
	verbose         int
	quiet           bool
	dumpHeaders     bool
	dumpBodies      bool
	deleteBefore    bool
	deleteDuring    bool
	deleteAfter     bool
	bindAddr        string
	disableFeatures string
)

// AddFlags adds the non filing system specific flags to the command
func AddFlags(flagSet *pflag.FlagSet) {
	rc.AddOption("main", fs.Config)
	// NB defaults which aren't the zero for the type should be set in fs/config.go NewConfig
	flags.CountVarP(flagSet, &verbose, "verbose", "v", "Print lots more stuff (repeat for more)")
	flags.BoolVarP(flagSet, &quiet, "quiet", "q", false, "Print as little stuff as possible")
	flags.DurationVarP(flagSet, &fs.Config.ModifyWindow, "modify-window", "", fs.Config.ModifyWindow, "Max time diff to be considered the same")
	flags.IntVarP(flagSet, &fs.Config.Checkers, "checkers", "", fs.Config.Checkers, "Number of checkers to run in parallel.")
	flags.IntVarP(flagSet, &fs.Config.Transfers, "transfers", "", fs.Config.Transfers, "Number of file transfers to run in parallel.")
	flags.StringVarP(flagSet, &config.ConfigPath, "config", "", config.ConfigPath, "Config file.")
	flags.StringVarP(flagSet, &config.CacheDir, "cache-dir", "", config.CacheDir, "Directory rclone will use for caching.")
	flags.BoolVarP(flagSet, &fs.Config.CheckSum, "checksum", "c", fs.Config.CheckSum, "Skip based on checksum (if available) & size, not mod-time & size")
	flags.BoolVarP(flagSet, &fs.Config.SizeOnly, "size-only", "", fs.Config.SizeOnly, "Skip based on size only, not mod-time or checksum")
	flags.BoolVarP(flagSet, &fs.Config.IgnoreTimes, "ignore-times", "I", fs.Config.IgnoreTimes, "Don't skip files that match size and time - transfer all files")
	flags.BoolVarP(flagSet, &fs.Config.IgnoreExisting, "ignore-existing", "", fs.Config.IgnoreExisting, "Skip all files that exist on destination")
	flags.BoolVarP(flagSet, &fs.Config.IgnoreErrors, "ignore-errors", "", fs.Config.IgnoreErrors, "delete even if there are I/O errors")
	flags.BoolVarP(flagSet, &fs.Config.DryRun, "dry-run", "n", fs.Config.DryRun, "Do a trial run with no permanent changes")
	flags.DurationVarP(flagSet, &fs.Config.ConnectTimeout, "contimeout", "", fs.Config.ConnectTimeout, "Connect timeout")
	flags.DurationVarP(flagSet, &fs.Config.Timeout, "timeout", "", fs.Config.Timeout, "IO idle timeout")
	flags.BoolVarP(flagSet, &dumpHeaders, "dump-headers", "", false, "Dump HTTP bodies - may contain sensitive info")
	flags.BoolVarP(flagSet, &dumpBodies, "dump-bodies", "", false, "Dump HTTP headers and bodies - may contain sensitive info")
	flags.BoolVarP(flagSet, &fs.Config.InsecureSkipVerify, "no-check-certificate", "", fs.Config.InsecureSkipVerify, "Do not verify the server SSL certificate. Insecure.")
	flags.BoolVarP(flagSet, &fs.Config.AskPassword, "ask-password", "", fs.Config.AskPassword, "Allow prompt for password for encrypted configuration.")
	flags.BoolVarP(flagSet, &deleteBefore, "delete-before", "", false, "When synchronizing, delete files on destination before transferring")
	flags.BoolVarP(flagSet, &deleteDuring, "delete-during", "", false, "When synchronizing, delete files during transfer")
	flags.BoolVarP(flagSet, &deleteAfter, "delete-after", "", false, "When synchronizing, delete files on destination after transferring (default)")
	flags.IntVar64P(flagSet, &fs.Config.MaxDelete, "max-delete", "", -1, "When synchronizing, limit the number of deletes")
	flags.BoolVarP(flagSet, &fs.Config.TrackRenames, "track-renames", "", fs.Config.TrackRenames, "When synchronizing, track file renames and do a server side move if possible")
	flags.IntVarP(flagSet, &fs.Config.LowLevelRetries, "low-level-retries", "", fs.Config.LowLevelRetries, "Number of low level retries to do.")
	flags.BoolVarP(flagSet, &fs.Config.UpdateOlder, "update", "u", fs.Config.UpdateOlder, "Skip files that are newer on the destination.")
	flags.BoolVarP(flagSet, &fs.Config.UseServerModTime, "use-server-modtime", "", fs.Config.UseServerModTime, "Use server modified time instead of object metadata")
	flags.BoolVarP(flagSet, &fs.Config.NoGzip, "no-gzip-encoding", "", fs.Config.NoGzip, "Don't set Accept-Encoding: gzip.")
	flags.IntVarP(flagSet, &fs.Config.MaxDepth, "max-depth", "", fs.Config.MaxDepth, "If set limits the recursion depth to this.")
	flags.BoolVarP(flagSet, &fs.Config.IgnoreSize, "ignore-size", "", false, "Ignore size when skipping use mod-time or checksum.")
	flags.BoolVarP(flagSet, &fs.Config.IgnoreChecksum, "ignore-checksum", "", fs.Config.IgnoreChecksum, "Skip post copy check of checksums.")
	flags.BoolVarP(flagSet, &fs.Config.NoTraverse, "no-traverse", "", fs.Config.NoTraverse, "Don't traverse destination file system on copy.")
	flags.BoolVarP(flagSet, &fs.Config.NoUpdateModTime, "no-update-modtime", "", fs.Config.NoUpdateModTime, "Don't update destination mod-time if files identical.")
	flags.StringVarP(flagSet, &fs.Config.BackupDir, "backup-dir", "", fs.Config.BackupDir, "Make backups into hierarchy based in DIR.")
	flags.StringVarP(flagSet, &fs.Config.Suffix, "suffix", "", fs.Config.Suffix, "Suffix for use with --backup-dir.")
	flags.BoolVarP(flagSet, &fs.Config.UseListR, "fast-list", "", fs.Config.UseListR, "Use recursive list if available. Uses more memory but fewer transactions.")
	flags.Float64VarP(flagSet, &fs.Config.TPSLimit, "tpslimit", "", fs.Config.TPSLimit, "Limit HTTP transactions per second to this.")
	flags.IntVarP(flagSet, &fs.Config.TPSLimitBurst, "tpslimit-burst", "", fs.Config.TPSLimitBurst, "Max burst of transactions for --tpslimit.")
	flags.StringVarP(flagSet, &bindAddr, "bind", "", "", "Local address to bind to for outgoing connections, IPv4, IPv6 or name.")
	flags.StringVarP(flagSet, &disableFeatures, "disable", "", "", "Disable a comma separated list of features.  Use help to see a list.")
	flags.StringVarP(flagSet, &fs.Config.UserAgent, "user-agent", "", fs.Config.UserAgent, "Set the user-agent to a specified string. The default is rclone/ version")
	flags.BoolVarP(flagSet, &fs.Config.Immutable, "immutable", "", fs.Config.Immutable, "Do not modify files. Fail if existing files have been modified.")
	flags.BoolVarP(flagSet, &fs.Config.AutoConfirm, "auto-confirm", "", fs.Config.AutoConfirm, "If enabled, do not request console confirmation.")
	flags.IntVarP(flagSet, &fs.Config.StatsFileNameLength, "stats-file-name-length", "", fs.Config.StatsFileNameLength, "Max file name length in stats. 0 for no limit")
	flags.FVarP(flagSet, &fs.Config.LogLevel, "log-level", "", "Log level DEBUG|INFO|NOTICE|ERROR")
	flags.FVarP(flagSet, &fs.Config.StatsLogLevel, "stats-log-level", "", "Log level to show --stats output DEBUG|INFO|NOTICE|ERROR")
	flags.FVarP(flagSet, &fs.Config.BwLimit, "bwlimit", "", "Bandwidth limit in kBytes/s, or use suffix b|k|M|G or a full timetable.")
	flags.FVarP(flagSet, &fs.Config.BufferSize, "buffer-size", "", "In memory buffer size when reading files for each --transfer.")
	flags.FVarP(flagSet, &fs.Config.StreamingUploadCutoff, "streaming-upload-cutoff", "", "Cutoff for switching to chunked upload if file size is unknown. Upload starts after reaching cutoff or when file ends.")
	flags.FVarP(flagSet, &fs.Config.Dump, "dump", "", "List of items to dump from: "+fs.DumpFlagsList)
	flags.FVarP(flagSet, &fs.Config.MaxTransfer, "max-transfer", "", "Maximum size of data to transfer.")
	flags.IntVarP(flagSet, &fs.Config.MaxBacklog, "max-backlog", "", fs.Config.MaxBacklog, "Maximum number of objects in sync or check backlog.")
	flags.BoolVarP(flagSet, &fs.Config.StatsOneLine, "stats-one-line", "", fs.Config.StatsOneLine, "Make the stats fit on one line.")
	flags.BoolVarP(flagSet, &fs.Config.Progress, "progress", "P", fs.Config.Progress, "Show progress during transfer.")
	flags.BoolVarP(flagSet, &fs.Config.Cookie, "use-cookies", "", fs.Config.Cookie, "Enable session cookiejar.")
}

// SetFlags converts any flags into config which weren't straight foward
func SetFlags() {
	if verbose >= 2 {
		fs.Config.LogLevel = fs.LogLevelDebug
	} else if verbose >= 1 {
		fs.Config.LogLevel = fs.LogLevelInfo
	}
	if quiet {
		if verbose > 0 {
			log.Fatalf("Can't set -v and -q")
		}
		fs.Config.LogLevel = fs.LogLevelError
	}
	logLevelFlag := pflag.Lookup("log-level")
	if logLevelFlag != nil && logLevelFlag.Changed {
		if verbose > 0 {
			log.Fatalf("Can't set -v and --log-level")
		}
		if quiet {
			log.Fatalf("Can't set -q and --log-level")
		}
	}

	if dumpHeaders {
		fs.Config.Dump |= fs.DumpHeaders
		fs.Logf(nil, "--dump-headers is obsolete - please use --dump headers instead")
	}
	if dumpBodies {
		fs.Config.Dump |= fs.DumpBodies
		fs.Logf(nil, "--dump-bodies is obsolete - please use --dump bodies instead")
	}

	switch {
	case deleteBefore && (deleteDuring || deleteAfter),
		deleteDuring && deleteAfter:
		log.Fatalf(`Only one of --delete-before, --delete-during or --delete-after can be used.`)
	case deleteBefore:
		fs.Config.DeleteMode = fs.DeleteModeBefore
	case deleteDuring:
		fs.Config.DeleteMode = fs.DeleteModeDuring
	case deleteAfter:
		fs.Config.DeleteMode = fs.DeleteModeAfter
	default:
		fs.Config.DeleteMode = fs.DeleteModeDefault
	}

	if fs.Config.IgnoreSize && fs.Config.SizeOnly {
		log.Fatalf(`Can't use --size-only and --ignore-size together.`)
	}

	if fs.Config.Suffix != "" && fs.Config.BackupDir == "" {
		log.Fatalf(`Can only use --suffix with --backup-dir.`)
	}

	if bindAddr != "" {
		addrs, err := net.LookupIP(bindAddr)
		if err != nil {
			log.Fatalf("--bind: Failed to parse %q as IP address: %v", bindAddr, err)
		}
		if len(addrs) != 1 {
			log.Fatalf("--bind: Expecting 1 IP address for %q but got %d", bindAddr, len(addrs))
		}
		fs.Config.BindAddr = addrs[0]
	}

	if disableFeatures != "" {
		if disableFeatures == "help" {
			log.Fatalf("Possible backend features are: %s\n", strings.Join(new(fs.Features).List(), ", "))
		}
		fs.Config.DisableFeatures = strings.Split(disableFeatures, ",")
	}

	// Make the config file absolute
	configPath, err := filepath.Abs(config.ConfigPath)
	if err == nil {
		config.ConfigPath = configPath
	}
}
