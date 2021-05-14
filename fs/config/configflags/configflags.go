// Package configflags defines the flags used by rclone.  It is
// decoupled into a separate package so it can be replaced.
package configflags

// Options set by command line flags
import (
	"log"
	"net"
	"strconv"
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
	configPath      string
	dumpHeaders     bool
	dumpBodies      bool
	deleteBefore    bool
	deleteDuring    bool
	deleteAfter     bool
	bindAddr        string
	disableFeatures string
	dscp            string
	uploadHeaders   []string
	downloadHeaders []string
	headers         []string
)

// AddFlags adds the non filing system specific flags to the command
func AddFlags(ci *fs.ConfigInfo, flagSet *pflag.FlagSet) {
	rc.AddOption("main", ci)
	// NB defaults which aren't the zero for the type should be set in fs/config.go NewConfig
	flags.CountVarP(flagSet, &verbose, "verbose", "v", "Print lots more stuff (repeat for more)")
	flags.BoolVarP(flagSet, &quiet, "quiet", "q", false, "Print as little stuff as possible")
	flags.DurationVarP(flagSet, &ci.ModifyWindow, "modify-window", "", ci.ModifyWindow, "Max time diff to be considered the same")
	flags.IntVarP(flagSet, &ci.Checkers, "checkers", "", ci.Checkers, "Number of checkers to run in parallel.")
	flags.IntVarP(flagSet, &ci.Transfers, "transfers", "", ci.Transfers, "Number of file transfers to run in parallel.")
	flags.StringVarP(flagSet, &configPath, "config", "", config.GetConfigPath(), "Config file.")
	flags.StringVarP(flagSet, &config.CacheDir, "cache-dir", "", config.CacheDir, "Directory rclone will use for caching.")
	flags.BoolVarP(flagSet, &ci.CheckSum, "checksum", "c", ci.CheckSum, "Skip based on checksum (if available) & size, not mod-time & size")
	flags.BoolVarP(flagSet, &ci.SizeOnly, "size-only", "", ci.SizeOnly, "Skip based on size only, not mod-time or checksum")
	flags.BoolVarP(flagSet, &ci.IgnoreTimes, "ignore-times", "I", ci.IgnoreTimes, "Don't skip files that match size and time - transfer all files")
	flags.BoolVarP(flagSet, &ci.IgnoreExisting, "ignore-existing", "", ci.IgnoreExisting, "Skip all files that exist on destination")
	flags.BoolVarP(flagSet, &ci.IgnoreErrors, "ignore-errors", "", ci.IgnoreErrors, "delete even if there are I/O errors")
	flags.BoolVarP(flagSet, &ci.DryRun, "dry-run", "n", ci.DryRun, "Do a trial run with no permanent changes")
	flags.BoolVarP(flagSet, &ci.Interactive, "interactive", "i", ci.Interactive, "Enable interactive mode")
	flags.DurationVarP(flagSet, &ci.ConnectTimeout, "contimeout", "", ci.ConnectTimeout, "Connect timeout")
	flags.DurationVarP(flagSet, &ci.Timeout, "timeout", "", ci.Timeout, "IO idle timeout")
	flags.DurationVarP(flagSet, &ci.ExpectContinueTimeout, "expect-continue-timeout", "", ci.ExpectContinueTimeout, "Timeout when using expect / 100-continue in HTTP")
	flags.BoolVarP(flagSet, &dumpHeaders, "dump-headers", "", false, "Dump HTTP headers - may contain sensitive info")
	flags.BoolVarP(flagSet, &dumpBodies, "dump-bodies", "", false, "Dump HTTP headers and bodies - may contain sensitive info")
	flags.BoolVarP(flagSet, &ci.InsecureSkipVerify, "no-check-certificate", "", ci.InsecureSkipVerify, "Do not verify the server SSL certificate. Insecure.")
	flags.BoolVarP(flagSet, &ci.AskPassword, "ask-password", "", ci.AskPassword, "Allow prompt for password for encrypted configuration.")
	flags.FVarP(flagSet, &ci.PasswordCommand, "password-command", "", "Command for supplying password for encrypted configuration.")
	flags.BoolVarP(flagSet, &deleteBefore, "delete-before", "", false, "When synchronizing, delete files on destination before transferring")
	flags.BoolVarP(flagSet, &deleteDuring, "delete-during", "", false, "When synchronizing, delete files during transfer")
	flags.BoolVarP(flagSet, &deleteAfter, "delete-after", "", false, "When synchronizing, delete files on destination after transferring (default)")
	flags.Int64VarP(flagSet, &ci.MaxDelete, "max-delete", "", -1, "When synchronizing, limit the number of deletes")
	flags.BoolVarP(flagSet, &ci.TrackRenames, "track-renames", "", ci.TrackRenames, "When synchronizing, track file renames and do a server-side move if possible")
	flags.StringVarP(flagSet, &ci.TrackRenamesStrategy, "track-renames-strategy", "", ci.TrackRenamesStrategy, "Strategies to use when synchronizing using track-renames hash|modtime|leaf")
	flags.IntVarP(flagSet, &ci.LowLevelRetries, "low-level-retries", "", ci.LowLevelRetries, "Number of low level retries to do.")
	flags.BoolVarP(flagSet, &ci.UpdateOlder, "update", "u", ci.UpdateOlder, "Skip files that are newer on the destination.")
	flags.BoolVarP(flagSet, &ci.UseServerModTime, "use-server-modtime", "", ci.UseServerModTime, "Use server modified time instead of object metadata")
	flags.BoolVarP(flagSet, &ci.NoGzip, "no-gzip-encoding", "", ci.NoGzip, "Don't set Accept-Encoding: gzip.")
	flags.IntVarP(flagSet, &ci.MaxDepth, "max-depth", "", ci.MaxDepth, "If set limits the recursion depth to this.")
	flags.BoolVarP(flagSet, &ci.IgnoreSize, "ignore-size", "", false, "Ignore size when skipping use mod-time or checksum.")
	flags.BoolVarP(flagSet, &ci.IgnoreChecksum, "ignore-checksum", "", ci.IgnoreChecksum, "Skip post copy check of checksums.")
	flags.BoolVarP(flagSet, &ci.IgnoreCaseSync, "ignore-case-sync", "", ci.IgnoreCaseSync, "Ignore case when synchronizing")
	flags.BoolVarP(flagSet, &ci.NoTraverse, "no-traverse", "", ci.NoTraverse, "Don't traverse destination file system on copy.")
	flags.BoolVarP(flagSet, &ci.CheckFirst, "check-first", "", ci.CheckFirst, "Do all the checks before starting transfers.")
	flags.BoolVarP(flagSet, &ci.NoCheckDest, "no-check-dest", "", ci.NoCheckDest, "Don't check the destination, copy regardless.")
	flags.BoolVarP(flagSet, &ci.NoUnicodeNormalization, "no-unicode-normalization", "", ci.NoUnicodeNormalization, "Don't normalize unicode characters in filenames.")
	flags.BoolVarP(flagSet, &ci.NoUpdateModTime, "no-update-modtime", "", ci.NoUpdateModTime, "Don't update destination mod-time if files identical.")
	flags.StringArrayVarP(flagSet, &ci.CompareDest, "compare-dest", "", nil, "Include additional comma separated server-side paths during comparison.")
	flags.StringArrayVarP(flagSet, &ci.CopyDest, "copy-dest", "", nil, "Implies --compare-dest but also copies files from paths into destination.")
	flags.StringVarP(flagSet, &ci.BackupDir, "backup-dir", "", ci.BackupDir, "Make backups into hierarchy based in DIR.")
	flags.StringVarP(flagSet, &ci.Suffix, "suffix", "", ci.Suffix, "Suffix to add to changed files.")
	flags.BoolVarP(flagSet, &ci.SuffixKeepExtension, "suffix-keep-extension", "", ci.SuffixKeepExtension, "Preserve the extension when using --suffix.")
	flags.BoolVarP(flagSet, &ci.UseListR, "fast-list", "", ci.UseListR, "Use recursive list if available. Uses more memory but fewer transactions.")
	flags.Float64VarP(flagSet, &ci.TPSLimit, "tpslimit", "", ci.TPSLimit, "Limit HTTP transactions per second to this.")
	flags.IntVarP(flagSet, &ci.TPSLimitBurst, "tpslimit-burst", "", ci.TPSLimitBurst, "Max burst of transactions for --tpslimit.")
	flags.StringVarP(flagSet, &bindAddr, "bind", "", "", "Local address to bind to for outgoing connections, IPv4, IPv6 or name.")
	flags.StringVarP(flagSet, &disableFeatures, "disable", "", "", "Disable a comma separated list of features.  Use --disable help to see a list.")
	flags.StringVarP(flagSet, &ci.UserAgent, "user-agent", "", ci.UserAgent, "Set the user-agent to a specified string. The default is rclone/ version")
	flags.BoolVarP(flagSet, &ci.Immutable, "immutable", "", ci.Immutable, "Do not modify files. Fail if existing files have been modified.")
	flags.BoolVarP(flagSet, &ci.AutoConfirm, "auto-confirm", "", ci.AutoConfirm, "If enabled, do not request console confirmation.")
	flags.IntVarP(flagSet, &ci.StatsFileNameLength, "stats-file-name-length", "", ci.StatsFileNameLength, "Max file name length in stats. 0 for no limit")
	flags.FVarP(flagSet, &ci.LogLevel, "log-level", "", "Log level DEBUG|INFO|NOTICE|ERROR")
	flags.FVarP(flagSet, &ci.StatsLogLevel, "stats-log-level", "", "Log level to show --stats output DEBUG|INFO|NOTICE|ERROR")
	flags.FVarP(flagSet, &ci.BwLimit, "bwlimit", "", "Bandwidth limit in KiByte/s, or use suffix B|K|M|G|T|P or a full timetable.")
	flags.FVarP(flagSet, &ci.BwLimitFile, "bwlimit-file", "", "Bandwidth limit per file in KiByte/s, or use suffix B|K|M|G|T|P or a full timetable.")
	flags.FVarP(flagSet, &ci.BufferSize, "buffer-size", "", "In memory buffer size when reading files for each --transfer.")
	flags.FVarP(flagSet, &ci.StreamingUploadCutoff, "streaming-upload-cutoff", "", "Cutoff for switching to chunked upload if file size is unknown. Upload starts after reaching cutoff or when file ends.")
	flags.FVarP(flagSet, &ci.Dump, "dump", "", "List of items to dump from: "+fs.DumpFlagsList)
	flags.FVarP(flagSet, &ci.MaxTransfer, "max-transfer", "", "Maximum size of data to transfer.")
	flags.DurationVarP(flagSet, &ci.MaxDuration, "max-duration", "", 0, "Maximum duration rclone will transfer data for.")
	flags.FVarP(flagSet, &ci.CutoffMode, "cutoff-mode", "", "Mode to stop transfers when reaching the max transfer limit HARD|SOFT|CAUTIOUS")
	flags.IntVarP(flagSet, &ci.MaxBacklog, "max-backlog", "", ci.MaxBacklog, "Maximum number of objects in sync or check backlog.")
	flags.IntVarP(flagSet, &ci.MaxStatsGroups, "max-stats-groups", "", ci.MaxStatsGroups, "Maximum number of stats groups to keep in memory. On max oldest is discarded.")
	flags.BoolVarP(flagSet, &ci.StatsOneLine, "stats-one-line", "", ci.StatsOneLine, "Make the stats fit on one line.")
	flags.BoolVarP(flagSet, &ci.StatsOneLineDate, "stats-one-line-date", "", ci.StatsOneLineDate, "Enables --stats-one-line and add current date/time prefix.")
	flags.StringVarP(flagSet, &ci.StatsOneLineDateFormat, "stats-one-line-date-format", "", ci.StatsOneLineDateFormat, "Enables --stats-one-line-date and uses custom formatted date. Enclose date string in double quotes (\"). See https://golang.org/pkg/time/#Time.Format")
	flags.BoolVarP(flagSet, &ci.ErrorOnNoTransfer, "error-on-no-transfer", "", ci.ErrorOnNoTransfer, "Sets exit code 9 if no files are transferred, useful in scripts")
	flags.BoolVarP(flagSet, &ci.Progress, "progress", "P", ci.Progress, "Show progress during transfer.")
	flags.BoolVarP(flagSet, &ci.ProgressTerminalTitle, "progress-terminal-title", "", ci.ProgressTerminalTitle, "Show progress on the terminal title. Requires -P/--progress.")
	flags.BoolVarP(flagSet, &ci.Cookie, "use-cookies", "", ci.Cookie, "Enable session cookiejar.")
	flags.BoolVarP(flagSet, &ci.UseMmap, "use-mmap", "", ci.UseMmap, "Use mmap allocator (see docs).")
	flags.StringVarP(flagSet, &ci.CaCert, "ca-cert", "", ci.CaCert, "CA certificate used to verify servers")
	flags.StringVarP(flagSet, &ci.ClientCert, "client-cert", "", ci.ClientCert, "Client SSL certificate (PEM) for mutual TLS auth")
	flags.StringVarP(flagSet, &ci.ClientKey, "client-key", "", ci.ClientKey, "Client SSL private key (PEM) for mutual TLS auth")
	flags.FVarP(flagSet, &ci.MultiThreadCutoff, "multi-thread-cutoff", "", "Use multi-thread downloads for files above this size.")
	flags.IntVarP(flagSet, &ci.MultiThreadStreams, "multi-thread-streams", "", ci.MultiThreadStreams, "Max number of streams to use for multi-thread downloads.")
	flags.BoolVarP(flagSet, &ci.UseJSONLog, "use-json-log", "", ci.UseJSONLog, "Use json log format.")
	flags.StringVarP(flagSet, &ci.OrderBy, "order-by", "", ci.OrderBy, "Instructions on how to order the transfers, e.g. 'size,descending'")
	flags.StringArrayVarP(flagSet, &uploadHeaders, "header-upload", "", nil, "Set HTTP header for upload transactions")
	flags.StringArrayVarP(flagSet, &downloadHeaders, "header-download", "", nil, "Set HTTP header for download transactions")
	flags.StringArrayVarP(flagSet, &headers, "header", "", nil, "Set HTTP header for all transactions")
	flags.BoolVarP(flagSet, &ci.RefreshTimes, "refresh-times", "", ci.RefreshTimes, "Refresh the modtime of remote files.")
	flags.BoolVarP(flagSet, &ci.NoConsole, "no-console", "", ci.NoConsole, "Hide console window. Supported on Windows only.")
	flags.StringVarP(flagSet, &dscp, "dscp", "", "", "Set DSCP value to connections. Can be value or names, eg. CS1, LE, DF, AF21.")
	flags.DurationVarP(flagSet, &ci.FsCacheExpireDuration, "fs-cache-expire-duration", "", ci.FsCacheExpireDuration, "cache remotes for this long (0 to disable caching)")
	flags.DurationVarP(flagSet, &ci.FsCacheExpireInterval, "fs-cache-expire-interval", "", ci.FsCacheExpireInterval, "interval to check for expired remotes")
	flags.BoolVarP(flagSet, &ci.DisableHTTP2, "disable-http2", "", ci.DisableHTTP2, "Disable HTTP/2 in the global transport.")
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
func SetFlags(ci *fs.ConfigInfo) {
	if verbose >= 2 {
		ci.LogLevel = fs.LogLevelDebug
	} else if verbose >= 1 {
		ci.LogLevel = fs.LogLevelInfo
	}
	if (ci.DryRun || ci.Interactive) && ci.StatsLogLevel > fs.LogLevelNotice {
		ci.StatsLogLevel = fs.LogLevelNotice
	}
	if quiet {
		if verbose > 0 {
			log.Fatalf("Can't set -v and -q")
		}
		ci.LogLevel = fs.LogLevelError
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
	if ci.UseJSONLog {
		logrus.AddHook(fsLog.NewCallerHook())
		logrus.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.999999-07:00",
		})
		logrus.SetLevel(logrus.DebugLevel)
		switch ci.LogLevel {
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
		ci.Dump |= fs.DumpHeaders
		fs.Logf(nil, "--dump-headers is obsolete - please use --dump headers instead")
	}
	if dumpBodies {
		ci.Dump |= fs.DumpBodies
		fs.Logf(nil, "--dump-bodies is obsolete - please use --dump bodies instead")
	}

	switch {
	case deleteBefore && (deleteDuring || deleteAfter),
		deleteDuring && deleteAfter:
		log.Fatalf(`Only one of --delete-before, --delete-during or --delete-after can be used.`)
	case deleteBefore:
		ci.DeleteMode = fs.DeleteModeBefore
	case deleteDuring:
		ci.DeleteMode = fs.DeleteModeDuring
	case deleteAfter:
		ci.DeleteMode = fs.DeleteModeAfter
	default:
		ci.DeleteMode = fs.DeleteModeDefault
	}

	if len(ci.CompareDest) > 0 && len(ci.CopyDest) > 0 {
		log.Fatalf(`Can't use --compare-dest with --copy-dest.`)
	}

	switch {
	case len(ci.StatsOneLineDateFormat) > 0:
		ci.StatsOneLineDate = true
		ci.StatsOneLine = true
	case ci.StatsOneLineDate:
		ci.StatsOneLineDateFormat = "2006/01/02 15:04:05 - "
		ci.StatsOneLine = true
	}

	if bindAddr != "" {
		addrs, err := net.LookupIP(bindAddr)
		if err != nil {
			log.Fatalf("--bind: Failed to parse %q as IP address: %v", bindAddr, err)
		}
		if len(addrs) != 1 {
			log.Fatalf("--bind: Expecting 1 IP address for %q but got %d", bindAddr, len(addrs))
		}
		ci.BindAddr = addrs[0]
	}

	if disableFeatures != "" {
		if disableFeatures == "help" {
			log.Fatalf("Possible backend features are: %s\n", strings.Join(new(fs.Features).List(), ", "))
		}
		ci.DisableFeatures = strings.Split(disableFeatures, ",")
	}

	if len(uploadHeaders) != 0 {
		ci.UploadHeaders = ParseHeaders(uploadHeaders)
	}
	if len(downloadHeaders) != 0 {
		ci.DownloadHeaders = ParseHeaders(downloadHeaders)
	}
	if len(headers) != 0 {
		ci.Headers = ParseHeaders(headers)
	}
	if len(dscp) != 0 {
		if value, ok := parseDSCP(dscp); ok {
			ci.TrafficClass = value << 2
		} else {
			log.Fatalf("--dscp: Invalid DSCP name: %v", dscp)
		}
	}

	// Set path to configuration file
	if err := config.SetConfigPath(configPath); err != nil {
		log.Fatalf("--config: Failed to set %q as config path: %v", configPath, err)
	}

	// Set whether multi-thread-streams was set
	multiThreadStreamsFlag := pflag.Lookup("multi-thread-streams")
	ci.MultiThreadSet = multiThreadStreamsFlag != nil && multiThreadStreamsFlag.Changed

	// Make sure some values are > 0
	nonZero := func(pi *int) {
		if *pi <= 0 {
			*pi = 1
		}
	}
	nonZero(&ci.LowLevelRetries)
	nonZero(&ci.Transfers)
	nonZero(&ci.Checkers)
}

// parseHeaders converts DSCP names to value
func parseDSCP(dscp string) (uint8, bool) {
	if s, err := strconv.ParseUint(dscp, 10, 6); err == nil {
		return uint8(s), true
	}
	dscp = strings.ToUpper(dscp)
	switch dscp {
	case "BE":
		fallthrough
	case "DF":
		fallthrough
	case "CS0":
		return 0x00, true
	case "CS1":
		return 0x08, true
	case "AF11":
		return 0x0A, true
	case "AF12":
		return 0x0C, true
	case "AF13":
		return 0x0E, true
	case "CS2":
		return 0x10, true
	case "AF21":
		return 0x12, true
	case "AF22":
		return 0x14, true
	case "AF23":
		return 0x16, true
	case "CS3":
		return 0x18, true
	case "AF31":
		return 0x1A, true
	case "AF32":
		return 0x1C, true
	case "AF33":
		return 0x1E, true
	case "CS4":
		return 0x20, true
	case "AF41":
		return 0x22, true
	case "AF42":
		return 0x24, true
	case "AF43":
		return 0x26, true
	case "CS5":
		return 0x28, true
	case "EF":
		return 0x2E, true
	case "CS6":
		return 0x30, true
	case "LE":
		return 0x01, true
	default:
		return 0, false
	}
}
