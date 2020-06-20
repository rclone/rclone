// Package configflags defines the flags used by rclone.  It is
// decoupled into a separate package so it can be replaced.
package configflags

// Options set by command line flags
import (
	"log"
	"net"
	"path/filepath"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/flags"
	fsLog "github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/fs/rc"
	"github.com/sirupsen/logrus"
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
	uploadHeaders   []string
	downloadHeaders []string
	headers         []string
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
	flags.BoolVarP(flagSet, &fs.Config.Interactive, "interactive", "i", fs.Config.Interactive, "Enable interactive mode")
	flags.DurationVarP(flagSet, &fs.Config.ConnectTimeout, "contimeout", "", fs.Config.ConnectTimeout, "Connect timeout")
	flags.DurationVarP(flagSet, &fs.Config.Timeout, "timeout", "", fs.Config.Timeout, "IO idle timeout")
	flags.DurationVarP(flagSet, &fs.Config.ExpectContinueTimeout, "expect-continue-timeout", "", fs.Config.ExpectContinueTimeout, "Timeout when using expect / 100-continue in HTTP")
	flags.BoolVarP(flagSet, &dumpHeaders, "dump-headers", "", false, "Dump HTTP headers - may contain sensitive info")
	flags.BoolVarP(flagSet, &dumpBodies, "dump-bodies", "", false, "Dump HTTP headers and bodies - may contain sensitive info")
	flags.BoolVarP(flagSet, &fs.Config.InsecureSkipVerify, "no-check-certificate", "", fs.Config.InsecureSkipVerify, "Do not verify the server SSL certificate. Insecure.")
	flags.BoolVarP(flagSet, &fs.Config.AskPassword, "ask-password", "", fs.Config.AskPassword, "Allow prompt for password for encrypted configuration.")
	flags.FVarP(flagSet, &fs.Config.PasswordCommand, "password-command", "", "Command for supplying password for encrypted configuration.")
	flags.BoolVarP(flagSet, &deleteBefore, "delete-before", "", false, "When synchronizing, delete files on destination before transferring")
	flags.BoolVarP(flagSet, &deleteDuring, "delete-during", "", false, "When synchronizing, delete files during transfer")
	flags.BoolVarP(flagSet, &deleteAfter, "delete-after", "", false, "When synchronizing, delete files on destination after transferring (default)")
	flags.Int64VarP(flagSet, &fs.Config.MaxDelete, "max-delete", "", -1, "When synchronizing, limit the number of deletes")
	flags.BoolVarP(flagSet, &fs.Config.TrackRenames, "track-renames", "", fs.Config.TrackRenames, "When synchronizing, track file renames and do a server side move if possible")
	flags.StringVarP(flagSet, &fs.Config.TrackRenamesStrategy, "track-renames-strategy", "", fs.Config.TrackRenamesStrategy, "Strategies to use when synchronizing using track-renames hash|modtime|leaf")
	flags.IntVarP(flagSet, &fs.Config.LowLevelRetries, "low-level-retries", "", fs.Config.LowLevelRetries, "Number of low level retries to do.")
	flags.BoolVarP(flagSet, &fs.Config.UpdateOlder, "update", "u", fs.Config.UpdateOlder, "Skip files that are newer on the destination.")
	flags.BoolVarP(flagSet, &fs.Config.UseServerModTime, "use-server-modtime", "", fs.Config.UseServerModTime, "Use server modified time instead of object metadata")
	flags.BoolVarP(flagSet, &fs.Config.NoGzip, "no-gzip-encoding", "", fs.Config.NoGzip, "Don't set Accept-Encoding: gzip.")
	flags.IntVarP(flagSet, &fs.Config.MaxDepth, "max-depth", "", fs.Config.MaxDepth, "If set limits the recursion depth to this.")
	flags.BoolVarP(flagSet, &fs.Config.IgnoreSize, "ignore-size", "", false, "Ignore size when skipping use mod-time or checksum.")
	flags.BoolVarP(flagSet, &fs.Config.IgnoreChecksum, "ignore-checksum", "", fs.Config.IgnoreChecksum, "Skip post copy check of checksums.")
	flags.BoolVarP(flagSet, &fs.Config.IgnoreCaseSync, "ignore-case-sync", "", fs.Config.IgnoreCaseSync, "Ignore case when synchronizing")
	flags.BoolVarP(flagSet, &fs.Config.NoTraverse, "no-traverse", "", fs.Config.NoTraverse, "Don't traverse destination file system on copy.")
	flags.BoolVarP(flagSet, &fs.Config.CheckFirst, "check-first", "", fs.Config.CheckFirst, "Do all the checks before starting transfers.")
	flags.BoolVarP(flagSet, &fs.Config.NoCheckDest, "no-check-dest", "", fs.Config.NoCheckDest, "Don't check the destination, copy regardless.")
	flags.BoolVarP(flagSet, &fs.Config.NoUnicodeNormalization, "no-unicode-normalization", "", fs.Config.NoUnicodeNormalization, "Don't normalize unicode characters in filenames.")
	flags.BoolVarP(flagSet, &fs.Config.NoUpdateModTime, "no-update-modtime", "", fs.Config.NoUpdateModTime, "Don't update destination mod-time if files identical.")
	flags.StringVarP(flagSet, &fs.Config.CompareDest, "compare-dest", "", fs.Config.CompareDest, "Include additional server-side path during comparison.")
	flags.StringVarP(flagSet, &fs.Config.CopyDest, "copy-dest", "", fs.Config.CopyDest, "Implies --compare-dest but also copies files from path into destination.")
	flags.StringVarP(flagSet, &fs.Config.BackupDir, "backup-dir", "", fs.Config.BackupDir, "Make backups into hierarchy based in DIR.")
	flags.StringVarP(flagSet, &fs.Config.Suffix, "suffix", "", fs.Config.Suffix, "Suffix to add to changed files.")
	flags.BoolVarP(flagSet, &fs.Config.SuffixKeepExtension, "suffix-keep-extension", "", fs.Config.SuffixKeepExtension, "Preserve the extension when using --suffix.")
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
	flags.FVarP(flagSet, &fs.Config.BwLimitFile, "bwlimit-file", "", "Bandwidth limit per file in kBytes/s, or use suffix b|k|M|G or a full timetable.")
	flags.FVarP(flagSet, &fs.Config.BufferSize, "buffer-size", "", "In memory buffer size when reading files for each --transfer.")
	flags.FVarP(flagSet, &fs.Config.StreamingUploadCutoff, "streaming-upload-cutoff", "", "Cutoff for switching to chunked upload if file size is unknown. Upload starts after reaching cutoff or when file ends.")
	flags.FVarP(flagSet, &fs.Config.Dump, "dump", "", "List of items to dump from: "+fs.DumpFlagsList)
	flags.FVarP(flagSet, &fs.Config.MaxTransfer, "max-transfer", "", "Maximum size of data to transfer.")
	flags.DurationVarP(flagSet, &fs.Config.MaxDuration, "max-duration", "", 0, "Maximum duration rclone will transfer data for.")
	flags.FVarP(flagSet, &fs.Config.CutoffMode, "cutoff-mode", "", "Mode to stop transfers when reaching the max transfer limit HARD|SOFT|CAUTIOUS")
	flags.IntVarP(flagSet, &fs.Config.MaxBacklog, "max-backlog", "", fs.Config.MaxBacklog, "Maximum number of objects in sync or check backlog.")
	flags.IntVarP(flagSet, &fs.Config.MaxStatsGroups, "max-stats-groups", "", fs.Config.MaxStatsGroups, "Maximum number of stats groups to keep in memory. On max oldest is discarded.")
	flags.BoolVarP(flagSet, &fs.Config.StatsOneLine, "stats-one-line", "", fs.Config.StatsOneLine, "Make the stats fit on one line.")
	flags.BoolVarP(flagSet, &fs.Config.StatsOneLineDate, "stats-one-line-date", "", fs.Config.StatsOneLineDate, "Enables --stats-one-line and add current date/time prefix.")
	flags.StringVarP(flagSet, &fs.Config.StatsOneLineDateFormat, "stats-one-line-date-format", "", fs.Config.StatsOneLineDateFormat, "Enables --stats-one-line-date and uses custom formatted date. Enclose date string in double quotes (\"). See https://golang.org/pkg/time/#Time.Format")
	flags.BoolVarP(flagSet, &fs.Config.ErrorOnNoTransfer, "error-on-no-transfer", "", fs.Config.ErrorOnNoTransfer, "Sets exit code 9 if no files are transferred, useful in scripts")
	flags.BoolVarP(flagSet, &fs.Config.Progress, "progress", "P", fs.Config.Progress, "Show progress during transfer.")
	flags.BoolVarP(flagSet, &fs.Config.Cookie, "use-cookies", "", fs.Config.Cookie, "Enable session cookiejar.")
	flags.BoolVarP(flagSet, &fs.Config.UseMmap, "use-mmap", "", fs.Config.UseMmap, "Use mmap allocator (see docs).")
	flags.StringVarP(flagSet, &fs.Config.CaCert, "ca-cert", "", fs.Config.CaCert, "CA certificate used to verify servers")
	flags.StringVarP(flagSet, &fs.Config.ClientCert, "client-cert", "", fs.Config.ClientCert, "Client SSL certificate (PEM) for mutual TLS auth")
	flags.StringVarP(flagSet, &fs.Config.ClientKey, "client-key", "", fs.Config.ClientKey, "Client SSL private key (PEM) for mutual TLS auth")
	flags.FVarP(flagSet, &fs.Config.MultiThreadCutoff, "multi-thread-cutoff", "", "Use multi-thread downloads for files above this size.")
	flags.IntVarP(flagSet, &fs.Config.MultiThreadStreams, "multi-thread-streams", "", fs.Config.MultiThreadStreams, "Max number of streams to use for multi-thread downloads.")
	flags.BoolVarP(flagSet, &fs.Config.UseJSONLog, "use-json-log", "", fs.Config.UseJSONLog, "Use json log format.")
	flags.StringVarP(flagSet, &fs.Config.OrderBy, "order-by", "", fs.Config.OrderBy, "Instructions on how to order the transfers, eg 'size,descending'")
	flags.StringArrayVarP(flagSet, &uploadHeaders, "header-upload", "", nil, "Set HTTP header for upload transactions")
	flags.StringArrayVarP(flagSet, &downloadHeaders, "header-download", "", nil, "Set HTTP header for download transactions")
	flags.StringArrayVarP(flagSet, &headers, "header", "", nil, "Set HTTP header for all transactions")
	flags.BoolVarP(flagSet, &fs.Config.RefreshTimes, "refresh-times", "", fs.Config.RefreshTimes, "Refresh the modtime of remote files.")
}

// ParseHeaders converts the strings passed in via the header flags into HTTPOptions
func ParseHeaders(headers []string) []*fs.HTTPOption {
	opts := []*fs.HTTPOption{}
	for _, header := range headers {
		parts := strings.SplitN(header, ":", 2)
		if len(parts) == 1 {
			log.Fatalf("Failed to parse '%s' as an HTTP header. Expecting a string like: 'Content-Encoding: gzip'", header)
		}
		option := &fs.HTTPOption{
			Key:   strings.TrimSpace(parts[0]),
			Value: strings.TrimSpace(parts[1]),
		}
		opts = append(opts, option)
	}
	return opts
}

// SetFlags converts any flags into config which weren't straight forward
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
	if fs.Config.UseJSONLog {
		logrus.AddHook(fsLog.NewCallerHook())
		logrus.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.999999-07:00",
		})
		logrus.SetLevel(logrus.DebugLevel)
		switch fs.Config.LogLevel {
		case fs.LogLevelEmergency, fs.LogLevelAlert:
			logrus.SetLevel(logrus.PanicLevel)
		case fs.LogLevelCritical:
			logrus.SetLevel(logrus.FatalLevel)
		case fs.LogLevelError:
			logrus.SetLevel(logrus.ErrorLevel)
		case fs.LogLevelWarning, fs.LogLevelNotice:
			logrus.SetLevel(logrus.WarnLevel)
		case fs.LogLevelInfo:
			logrus.SetLevel(logrus.InfoLevel)
		case fs.LogLevelDebug:
			logrus.SetLevel(logrus.DebugLevel)
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

	if fs.Config.CompareDest != "" && fs.Config.CopyDest != "" {
		log.Fatalf(`Can't use --compare-dest with --copy-dest.`)
	}

	switch {
	case len(fs.Config.StatsOneLineDateFormat) > 0:
		fs.Config.StatsOneLineDate = true
		fs.Config.StatsOneLine = true
	case fs.Config.StatsOneLineDate:
		fs.Config.StatsOneLineDateFormat = "2006/01/02 15:04:05 - "
		fs.Config.StatsOneLine = true
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

	if len(uploadHeaders) != 0 {
		fs.Config.UploadHeaders = ParseHeaders(uploadHeaders)
	}
	if len(downloadHeaders) != 0 {
		fs.Config.DownloadHeaders = ParseHeaders(downloadHeaders)
	}
	if len(headers) != 0 {
		fs.Config.Headers = ParseHeaders(headers)
	}

	// Make the config file absolute
	configPath, err := filepath.Abs(config.ConfigPath)
	if err == nil {
		config.ConfigPath = configPath
	}

	// Set whether multi-thread-streams was set
	multiThreadStreamsFlag := pflag.Lookup("multi-thread-streams")
	fs.Config.MultiThreadSet = multiThreadStreamsFlag != nil && multiThreadStreamsFlag.Changed

}
