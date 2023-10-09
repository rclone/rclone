// Package configflags defines the flags used by rclone.  It is
// decoupled into a separate package so it can be replaced.
package configflags

// Options set by command line flags
import (
	"log"
	"net"
	"os"
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
	cacheDir        string
	tempDir         string
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
	metadataSet     []string
	partialSuffix   string
)

// AddFlags adds the non filing system specific flags to the command
func AddFlags(ci *fs.ConfigInfo, flagSet *pflag.FlagSet) {
	rc.AddOption("main", ci)
	// NB defaults which aren't the zero for the type should be set in fs/config.go NewConfig
	flags.CountVarP(flagSet, &verbose, "verbose", "v", "Print lots more stuff (repeat for more)", "Logging,Important")
	flags.BoolVarP(flagSet, &quiet, "quiet", "q", false, "Print as little stuff as possible", "Logging")
	flags.DurationVarP(flagSet, &ci.ModifyWindow, "modify-window", "", ci.ModifyWindow, "Max time diff to be considered the same", "Copy")
	flags.IntVarP(flagSet, &ci.Checkers, "checkers", "", ci.Checkers, "Number of checkers to run in parallel", "Performance")
	flags.IntVarP(flagSet, &ci.Transfers, "transfers", "", ci.Transfers, "Number of file transfers to run in parallel", "Performance")
	flags.StringVarP(flagSet, &configPath, "config", "", config.GetConfigPath(), "Config file", "Config")
	flags.StringVarP(flagSet, &cacheDir, "cache-dir", "", config.GetCacheDir(), "Directory rclone will use for caching", "Config")
	flags.StringVarP(flagSet, &tempDir, "temp-dir", "", os.TempDir(), "Directory rclone will use for temporary files", "Config")
	flags.BoolVarP(flagSet, &ci.CheckSum, "checksum", "c", ci.CheckSum, "Check for changes with size & checksum (if available, or fallback to size only).", "Copy")
	flags.BoolVarP(flagSet, &ci.SizeOnly, "size-only", "", ci.SizeOnly, "Skip based on size only, not modtime or checksum", "Copy")
	flags.BoolVarP(flagSet, &ci.IgnoreTimes, "ignore-times", "I", ci.IgnoreTimes, "Don't skip files that match size and time - transfer all files", "Copy")
	flags.BoolVarP(flagSet, &ci.IgnoreExisting, "ignore-existing", "", ci.IgnoreExisting, "Skip all files that exist on destination", "Copy")
	flags.BoolVarP(flagSet, &ci.IgnoreErrors, "ignore-errors", "", ci.IgnoreErrors, "Delete even if there are I/O errors", "Sync")
	flags.BoolVarP(flagSet, &ci.DryRun, "dry-run", "n", ci.DryRun, "Do a trial run with no permanent changes", "Config,Important")
	flags.BoolVarP(flagSet, &ci.Interactive, "interactive", "i", ci.Interactive, "Enable interactive mode", "Config,Important")
	flags.DurationVarP(flagSet, &ci.ConnectTimeout, "contimeout", "", ci.ConnectTimeout, "Connect timeout", "Networking")
	flags.DurationVarP(flagSet, &ci.Timeout, "timeout", "", ci.Timeout, "IO idle timeout", "Networking")
	flags.DurationVarP(flagSet, &ci.ExpectContinueTimeout, "expect-continue-timeout", "", ci.ExpectContinueTimeout, "Timeout when using expect / 100-continue in HTTP", "Networking")
	flags.BoolVarP(flagSet, &dumpHeaders, "dump-headers", "", false, "Dump HTTP headers - may contain sensitive info", "Debugging")
	flags.BoolVarP(flagSet, &dumpBodies, "dump-bodies", "", false, "Dump HTTP headers and bodies - may contain sensitive info", "Debugging")
	flags.BoolVarP(flagSet, &ci.InsecureSkipVerify, "no-check-certificate", "", ci.InsecureSkipVerify, "Do not verify the server SSL certificate (insecure)", "Networking")
	flags.BoolVarP(flagSet, &ci.AskPassword, "ask-password", "", ci.AskPassword, "Allow prompt for password for encrypted configuration", "Config")
	flags.FVarP(flagSet, &ci.PasswordCommand, "password-command", "", "Command for supplying password for encrypted configuration", "Config")
	flags.BoolVarP(flagSet, &deleteBefore, "delete-before", "", false, "When synchronizing, delete files on destination before transferring", "Sync")
	flags.BoolVarP(flagSet, &deleteDuring, "delete-during", "", false, "When synchronizing, delete files during transfer", "Sync")
	flags.BoolVarP(flagSet, &deleteAfter, "delete-after", "", false, "When synchronizing, delete files on destination after transferring (default)", "Sync")
	flags.Int64VarP(flagSet, &ci.MaxDelete, "max-delete", "", -1, "When synchronizing, limit the number of deletes", "Sync")
	flags.FVarP(flagSet, &ci.MaxDeleteSize, "max-delete-size", "", "When synchronizing, limit the total size of deletes", "Sync")
	flags.BoolVarP(flagSet, &ci.TrackRenames, "track-renames", "", ci.TrackRenames, "When synchronizing, track file renames and do a server-side move if possible", "Sync")
	flags.StringVarP(flagSet, &ci.TrackRenamesStrategy, "track-renames-strategy", "", ci.TrackRenamesStrategy, "Strategies to use when synchronizing using track-renames hash|modtime|leaf", "Sync")
	flags.IntVarP(flagSet, &ci.LowLevelRetries, "low-level-retries", "", ci.LowLevelRetries, "Number of low level retries to do", "Config")
	flags.BoolVarP(flagSet, &ci.UpdateOlder, "update", "u", ci.UpdateOlder, "Skip files that are newer on the destination", "Copy")
	flags.BoolVarP(flagSet, &ci.UseServerModTime, "use-server-modtime", "", ci.UseServerModTime, "Use server modified time instead of object metadata", "Config")
	flags.BoolVarP(flagSet, &ci.NoGzip, "no-gzip-encoding", "", ci.NoGzip, "Don't set Accept-Encoding: gzip", "Networking")
	flags.IntVarP(flagSet, &ci.MaxDepth, "max-depth", "", ci.MaxDepth, "If set limits the recursion depth to this", "Filter")
	flags.BoolVarP(flagSet, &ci.IgnoreSize, "ignore-size", "", false, "Ignore size when skipping use modtime or checksum", "Copy")
	flags.BoolVarP(flagSet, &ci.IgnoreChecksum, "ignore-checksum", "", ci.IgnoreChecksum, "Skip post copy check of checksums", "Copy")
	flags.BoolVarP(flagSet, &ci.IgnoreCaseSync, "ignore-case-sync", "", ci.IgnoreCaseSync, "Ignore case when synchronizing", "Copy")
	flags.BoolVarP(flagSet, &ci.FixCase, "fix-case", "", ci.FixCase, "Force rename of case insensitive dest to match source", "Sync")
	flags.BoolVarP(flagSet, &ci.NoTraverse, "no-traverse", "", ci.NoTraverse, "Don't traverse destination file system on copy", "Copy")
	flags.BoolVarP(flagSet, &ci.CheckFirst, "check-first", "", ci.CheckFirst, "Do all the checks before starting transfers", "Copy")
	flags.BoolVarP(flagSet, &ci.NoCheckDest, "no-check-dest", "", ci.NoCheckDest, "Don't check the destination, copy regardless", "Copy")
	flags.BoolVarP(flagSet, &ci.NoUnicodeNormalization, "no-unicode-normalization", "", ci.NoUnicodeNormalization, "Don't normalize unicode characters in filenames", "Config")
	flags.BoolVarP(flagSet, &ci.NoUpdateModTime, "no-update-modtime", "", ci.NoUpdateModTime, "Don't update destination modtime if files identical", "Copy")
	flags.StringArrayVarP(flagSet, &ci.CompareDest, "compare-dest", "", nil, "Include additional comma separated server-side paths during comparison", "Copy")
	flags.StringArrayVarP(flagSet, &ci.CopyDest, "copy-dest", "", nil, "Implies --compare-dest but also copies files from paths into destination", "Copy")
	flags.StringVarP(flagSet, &ci.BackupDir, "backup-dir", "", ci.BackupDir, "Make backups into hierarchy based in DIR", "Sync")
	flags.StringVarP(flagSet, &ci.Suffix, "suffix", "", ci.Suffix, "Suffix to add to changed files", "Sync")
	flags.BoolVarP(flagSet, &ci.SuffixKeepExtension, "suffix-keep-extension", "", ci.SuffixKeepExtension, "Preserve the extension when using --suffix", "Sync")
	flags.BoolVarP(flagSet, &ci.UseListR, "fast-list", "", ci.UseListR, "Use recursive list if available; uses more memory but fewer transactions", "Listing")
	flags.Float64VarP(flagSet, &ci.TPSLimit, "tpslimit", "", ci.TPSLimit, "Limit HTTP transactions per second to this", "Networking")
	flags.IntVarP(flagSet, &ci.TPSLimitBurst, "tpslimit-burst", "", ci.TPSLimitBurst, "Max burst of transactions for --tpslimit", "Networking")
	flags.StringVarP(flagSet, &bindAddr, "bind", "", "", "Local address to bind to for outgoing connections, IPv4, IPv6 or name", "Networking")
	flags.StringVarP(flagSet, &disableFeatures, "disable", "", "", "Disable a comma separated list of features (use --disable help to see a list)", "Config")
	flags.StringVarP(flagSet, &ci.UserAgent, "user-agent", "", ci.UserAgent, "Set the user-agent to a specified string", "Networking")
	flags.BoolVarP(flagSet, &ci.Immutable, "immutable", "", ci.Immutable, "Do not modify files, fail if existing files have been modified", "Copy")
	flags.BoolVarP(flagSet, &ci.AutoConfirm, "auto-confirm", "", ci.AutoConfirm, "If enabled, do not request console confirmation", "Config")
	flags.IntVarP(flagSet, &ci.StatsFileNameLength, "stats-file-name-length", "", ci.StatsFileNameLength, "Max file name length in stats (0 for no limit)", "Logging")
	flags.FVarP(flagSet, &ci.LogLevel, "log-level", "", "Log level DEBUG|INFO|NOTICE|ERROR", "Logging")
	flags.FVarP(flagSet, &ci.StatsLogLevel, "stats-log-level", "", "Log level to show --stats output DEBUG|INFO|NOTICE|ERROR", "Logging")
	flags.FVarP(flagSet, &ci.BwLimit, "bwlimit", "", "Bandwidth limit in KiB/s, or use suffix B|K|M|G|T|P or a full timetable", "Networking")
	flags.FVarP(flagSet, &ci.BwLimitFile, "bwlimit-file", "", "Bandwidth limit per file in KiB/s, or use suffix B|K|M|G|T|P or a full timetable", "Networking")
	flags.FVarP(flagSet, &ci.BufferSize, "buffer-size", "", "In memory buffer size when reading files for each --transfer", "Performance")
	flags.FVarP(flagSet, &ci.StreamingUploadCutoff, "streaming-upload-cutoff", "", "Cutoff for switching to chunked upload if file size is unknown, upload starts after reaching cutoff or when file ends", "Copy")
	flags.FVarP(flagSet, &ci.Dump, "dump", "", "List of items to dump from: "+fs.DumpFlagsList, "Debugging")
	flags.FVarP(flagSet, &ci.MaxTransfer, "max-transfer", "", "Maximum size of data to transfer", "Copy")
	flags.DurationVarP(flagSet, &ci.MaxDuration, "max-duration", "", 0, "Maximum duration rclone will transfer data for", "Copy")
	flags.FVarP(flagSet, &ci.CutoffMode, "cutoff-mode", "", "Mode to stop transfers when reaching the max transfer limit HARD|SOFT|CAUTIOUS", "Copy")
	flags.IntVarP(flagSet, &ci.MaxBacklog, "max-backlog", "", ci.MaxBacklog, "Maximum number of objects in sync or check backlog", "Copy,Check")
	flags.IntVarP(flagSet, &ci.MaxStatsGroups, "max-stats-groups", "", ci.MaxStatsGroups, "Maximum number of stats groups to keep in memory, on max oldest is discarded", "Logging")
	flags.BoolVarP(flagSet, &ci.StatsOneLine, "stats-one-line", "", ci.StatsOneLine, "Make the stats fit on one line", "Logging")
	flags.BoolVarP(flagSet, &ci.StatsOneLineDate, "stats-one-line-date", "", ci.StatsOneLineDate, "Enable --stats-one-line and add current date/time prefix", "Logging")
	flags.StringVarP(flagSet, &ci.StatsOneLineDateFormat, "stats-one-line-date-format", "", ci.StatsOneLineDateFormat, "Enable --stats-one-line-date and use custom formatted date: Enclose date string in double quotes (\"), see https://golang.org/pkg/time/#Time.Format", "Logging")
	flags.BoolVarP(flagSet, &ci.ErrorOnNoTransfer, "error-on-no-transfer", "", ci.ErrorOnNoTransfer, "Sets exit code 9 if no files are transferred, useful in scripts", "Config")
	flags.BoolVarP(flagSet, &ci.Progress, "progress", "P", ci.Progress, "Show progress during transfer", "Logging")
	flags.BoolVarP(flagSet, &ci.ProgressTerminalTitle, "progress-terminal-title", "", ci.ProgressTerminalTitle, "Show progress on the terminal title (requires -P/--progress)", "Logging")
	flags.BoolVarP(flagSet, &ci.Cookie, "use-cookies", "", ci.Cookie, "Enable session cookiejar", "Networking")
	flags.BoolVarP(flagSet, &ci.UseMmap, "use-mmap", "", ci.UseMmap, "Use mmap allocator (see docs)", "Config")
	flags.StringArrayVarP(flagSet, &ci.CaCert, "ca-cert", "", ci.CaCert, "CA certificate used to verify servers", "Networking")
	flags.StringVarP(flagSet, &ci.ClientCert, "client-cert", "", ci.ClientCert, "Client SSL certificate (PEM) for mutual TLS auth", "Networking")
	flags.StringVarP(flagSet, &ci.ClientKey, "client-key", "", ci.ClientKey, "Client SSL private key (PEM) for mutual TLS auth", "Networking")
	flags.FVarP(flagSet, &ci.MultiThreadCutoff, "multi-thread-cutoff", "", "Use multi-thread downloads for files above this size", "Copy")
	flags.IntVarP(flagSet, &ci.MultiThreadStreams, "multi-thread-streams", "", ci.MultiThreadStreams, "Number of streams to use for multi-thread downloads", "Copy")
	flags.FVarP(flagSet, &ci.MultiThreadWriteBufferSize, "multi-thread-write-buffer-size", "", "In memory buffer size for writing when in multi-thread mode", "Copy")
	flags.FVarP(flagSet, &ci.MultiThreadChunkSize, "multi-thread-chunk-size", "", "Chunk size for multi-thread downloads / uploads, if not set by filesystem", "Copy")
	flags.BoolVarP(flagSet, &ci.UseJSONLog, "use-json-log", "", ci.UseJSONLog, "Use json log format", "Logging")
	flags.StringVarP(flagSet, &ci.OrderBy, "order-by", "", ci.OrderBy, "Instructions on how to order the transfers, e.g. 'size,descending'", "Copy")
	flags.StringArrayVarP(flagSet, &uploadHeaders, "header-upload", "", nil, "Set HTTP header for upload transactions", "Networking")
	flags.StringArrayVarP(flagSet, &downloadHeaders, "header-download", "", nil, "Set HTTP header for download transactions", "Networking")
	flags.StringArrayVarP(flagSet, &headers, "header", "", nil, "Set HTTP header for all transactions", "Networking")
	flags.StringArrayVarP(flagSet, &metadataSet, "metadata-set", "", nil, "Add metadata key=value when uploading", "Metadata")
	flags.BoolVarP(flagSet, &ci.RefreshTimes, "refresh-times", "", ci.RefreshTimes, "Refresh the modtime of remote files", "Copy")
	flags.BoolVarP(flagSet, &ci.NoConsole, "no-console", "", ci.NoConsole, "Hide console window (supported on Windows only)", "Config")
	flags.StringVarP(flagSet, &dscp, "dscp", "", "", "Set DSCP value to connections, value or name, e.g. CS1, LE, DF, AF21", "Networking")
	flags.DurationVarP(flagSet, &ci.FsCacheExpireDuration, "fs-cache-expire-duration", "", ci.FsCacheExpireDuration, "Cache remotes for this long (0 to disable caching)", "Config")
	flags.DurationVarP(flagSet, &ci.FsCacheExpireInterval, "fs-cache-expire-interval", "", ci.FsCacheExpireInterval, "Interval to check for expired remotes", "Config")
	flags.BoolVarP(flagSet, &ci.DisableHTTP2, "disable-http2", "", ci.DisableHTTP2, "Disable HTTP/2 in the global transport", "Networking")
	flags.BoolVarP(flagSet, &ci.HumanReadable, "human-readable", "", ci.HumanReadable, "Print numbers in a human-readable format, sizes with suffix Ki|Mi|Gi|Ti|Pi", "Config")
	flags.DurationVarP(flagSet, &ci.KvLockTime, "kv-lock-time", "", ci.KvLockTime, "Maximum time to keep key-value database locked by process", "Config")
	flags.BoolVarP(flagSet, &ci.DisableHTTPKeepAlives, "disable-http-keep-alives", "", ci.DisableHTTPKeepAlives, "Disable HTTP keep-alives and use each connection once.", "Networking")
	flags.BoolVarP(flagSet, &ci.Metadata, "metadata", "M", ci.Metadata, "If set, preserve metadata when copying objects", "Metadata,Copy")
	flags.BoolVarP(flagSet, &ci.ServerSideAcrossConfigs, "server-side-across-configs", "", ci.ServerSideAcrossConfigs, "Allow server-side operations (e.g. copy) to work across different configs", "Copy")
	flags.FVarP(flagSet, &ci.TerminalColorMode, "color", "", "When to show colors (and other ANSI codes) AUTO|NEVER|ALWAYS", "Config")
	flags.FVarP(flagSet, &ci.DefaultTime, "default-time", "", "Time to show if modtime is unknown for files and directories", "Config,Listing")
	flags.BoolVarP(flagSet, &ci.Inplace, "inplace", "", ci.Inplace, "Download directly to destination file instead of atomic download to temp/rename", "Copy")
	flags.StringVarP(flagSet, &partialSuffix, "partial-suffix", "", ci.PartialSuffix, "Add partial-suffix to temporary file name when --inplace is not used", "Copy")
	flags.FVarP(flagSet, &ci.MetadataMapper, "metadata-mapper", "", "Program to run to transforming metadata before upload", "Metadata")
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
	if dumpHeaders {
		ci.Dump |= fs.DumpHeaders
		fs.Logf(nil, "--dump-headers is obsolete - please use --dump headers instead")
	}
	if dumpBodies {
		ci.Dump |= fs.DumpBodies
		fs.Logf(nil, "--dump-bodies is obsolete - please use --dump bodies instead")
	}
	if ci.Dump != 0 && verbose < 2 && ci.LogLevel != fs.LogLevelDebug {
		fs.Logf(nil, "Automatically setting -vv as --dump is enabled")
		verbose = 2
	}

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
	if len(headers) != 0 {
		ci.Headers = ParseHeaders(headers)
	}
	if len(metadataSet) != 0 {
		ci.MetadataSet = make(fs.Metadata, len(metadataSet))
		for _, kv := range metadataSet {
			equal := strings.IndexRune(kv, '=')
			if equal < 0 {
				log.Fatalf("Failed to parse '%s' as metadata key=value.", kv)
			}
			ci.MetadataSet[strings.ToLower(kv[:equal])] = kv[equal+1:]
		}
		fs.Debugf(nil, "MetadataUpload %v", ci.MetadataSet)
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

	// Set path to cache dir
	if err := config.SetCacheDir(cacheDir); err != nil {
		log.Fatalf("--cache-dir: Failed to set %q as cache dir: %v", cacheDir, err)
	}

	// Set path to temp dir
	if err := config.SetTempDir(tempDir); err != nil {
		log.Fatalf("--temp-dir: Failed to set %q as temp dir: %v", tempDir, err)
	}

	// Set whether multi-thread-streams was set
	multiThreadStreamsFlag := pflag.Lookup("multi-thread-streams")
	ci.MultiThreadSet = multiThreadStreamsFlag != nil && multiThreadStreamsFlag.Changed

	if len(partialSuffix) > 16 {
		log.Fatalf("--partial-suffix: Expecting suffix length not greater than %d but got %d", 16, len(partialSuffix))
	}
	ci.PartialSuffix = partialSuffix

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
