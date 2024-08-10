package fs

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// Global
var (
	// globalConfig for rclone
	globalConfig = new(ConfigInfo)

	// Read a value from the config file
	//
	// This is a function pointer to decouple the config
	// implementation from the fs
	ConfigFileGet = func(section, key string) (string, bool) { return "", false }

	// Set a value into the config file and persist it
	//
	// This is a function pointer to decouple the config
	// implementation from the fs
	ConfigFileSet = func(section, key, value string) (err error) {
		return errors.New("no config file set handler")
	}

	// Check if the config file has the named section
	//
	// This is a function pointer to decouple the config
	// implementation from the fs
	ConfigFileHasSection = func(section string) bool { return false }

	// CountError counts an error.  If any errors have been
	// counted then rclone will exit with a non zero error code.
	//
	// This is a function pointer to decouple the config
	// implementation from the fs
	CountError = func(err error) error { return err }

	// ConfigProvider is the config key used for provider options
	ConfigProvider = "provider"

	// ConfigEdit is the config key used to show we wish to edit existing entries
	ConfigEdit = "config_fs_edit"
)

// ConfigOptionsInfo describes the Options in use
var ConfigOptionsInfo = Options{{
	Name:    "modify_window",
	Default: time.Nanosecond,
	Help:    "Max time diff to be considered the same",
	Groups:  "Copy",
}, {
	Name:    "checkers",
	Default: 8,
	Help:    "Number of checkers to run in parallel",
	Groups:  "Performance",
}, {
	Name:    "transfers",
	Default: 4,
	Help:    "Number of file transfers to run in parallel",
	Groups:  "Performance",
}, {
	Name:     "checksum",
	ShortOpt: "c",
	Default:  false,
	Help:     "Check for changes with size & checksum (if available, or fallback to size only)",
	Groups:   "Copy",
}, {
	Name:    "size_only",
	Default: false,
	Help:    "Skip based on size only, not modtime or checksum",
	Groups:  "Copy",
}, {
	Name:     "ignore_times",
	ShortOpt: "I",
	Default:  false,
	Help:     "Don't skip items that match size and time - transfer all unconditionally",
	Groups:   "Copy",
}, {
	Name:    "ignore_existing",
	Default: false,
	Help:    "Skip all files that exist on destination",
	Groups:  "Copy",
}, {
	Name:    "ignore_errors",
	Default: false,
	Help:    "Delete even if there are I/O errors",
	Groups:  "Sync",
}, {
	Name:     "dry_run",
	ShortOpt: "n",
	Default:  false,
	Help:     "Do a trial run with no permanent changes",
	Groups:   "Config,Important",
}, {
	Name:     "interactive",
	ShortOpt: "i",
	Default:  false,
	Help:     "Enable interactive mode",
	Groups:   "Config,Important",
}, {
	Name:    "contimeout",
	Default: 60 * time.Second,
	Help:    "Connect timeout",
	Groups:  "Networking",
}, {
	Name:    "timeout",
	Default: 5 * 60 * time.Second,
	Help:    "IO idle timeout",
	Groups:  "Networking",
}, {
	Name:    "expect_continue_timeout",
	Default: 1 * time.Second,
	Help:    "Timeout when using expect / 100-continue in HTTP",
	Groups:  "Networking",
}, {
	Name:    "no_check_certificate",
	Default: false,
	Help:    "Do not verify the server SSL certificate (insecure)",
	Groups:  "Networking",
}, {
	Name:    "ask_password",
	Default: true,
	Help:    "Allow prompt for password for encrypted configuration",
	Groups:  "Config",
}, {
	Name:    "password_command",
	Default: SpaceSepList{},
	Help:    "Command for supplying password for encrypted configuration",
	Groups:  "Config",
}, {
	Name:    "max_delete",
	Default: int64(-1),
	Help:    "When synchronizing, limit the number of deletes",
	Groups:  "Sync",
}, {
	Name:    "max_delete_size",
	Default: SizeSuffix(-1),
	Help:    "When synchronizing, limit the total size of deletes",
	Groups:  "Sync",
}, {
	Name:    "track_renames",
	Default: false,
	Help:    "When synchronizing, track file renames and do a server-side move if possible",
	Groups:  "Sync",
}, {
	Name:    "track_renames_strategy",
	Default: "hash",
	Help:    "Strategies to use when synchronizing using track-renames hash|modtime|leaf",
	Groups:  "Sync",
}, {
	Name:    "retries",
	Default: 3,
	Help:    "Retry operations this many times if they fail",
	Groups:  "Config",
}, {
	Name:    "retries_sleep",
	Default: time.Duration(0),
	Help:    "Interval between retrying operations if they fail, e.g. 500ms, 60s, 5m (0 to disable)",
	Groups:  "Config",
}, {
	Name:    "low_level_retries",
	Default: 10,
	Help:    "Number of low level retries to do",
	Groups:  "Config",
}, {
	Name:     "update",
	ShortOpt: "u",
	Default:  false,
	Help:     "Skip files that are newer on the destination",
	Groups:   "Copy",
}, {
	Name:    "use_server_modtime",
	Default: false,
	Help:    "Use server modified time instead of object metadata",
	Groups:  "Config",
}, {
	Name:    "no_gzip_encoding",
	Default: false,
	Help:    "Don't set Accept-Encoding: gzip",
	Groups:  "Networking",
}, {
	Name:    "max_depth",
	Default: -1,
	Help:    "If set limits the recursion depth to this",
	Groups:  "Filter",
}, {
	Name:    "ignore_size",
	Default: false,
	Help:    "Ignore size when skipping use modtime or checksum",
	Groups:  "Copy",
}, {
	Name:    "ignore_checksum",
	Default: false,
	Help:    "Skip post copy check of checksums",
	Groups:  "Copy",
}, {
	Name:    "ignore_case_sync",
	Default: false,
	Help:    "Ignore case when synchronizing",
	Groups:  "Copy",
}, {
	Name:    "fix_case",
	Default: false,
	Help:    "Force rename of case insensitive dest to match source",
	Groups:  "Sync",
}, {
	Name:    "no_traverse",
	Default: false,
	Help:    "Don't traverse destination file system on copy",
	Groups:  "Copy",
}, {
	Name:    "check_first",
	Default: false,
	Help:    "Do all the checks before starting transfers",
	Groups:  "Copy",
}, {
	Name:    "no_check_dest",
	Default: false,
	Help:    "Don't check the destination, copy regardless",
	Groups:  "Copy",
}, {
	Name:    "no_unicode_normalization",
	Default: false,
	Help:    "Don't normalize unicode characters in filenames",
	Groups:  "Config",
}, {
	Name:    "no_update_modtime",
	Default: false,
	Help:    "Don't update destination modtime if files identical",
	Groups:  "Copy",
}, {
	Name:    "no_update_dir_modtime",
	Default: false,
	Help:    "Don't update directory modification times",
	Groups:  "Copy",
}, {
	Name:    "compare_dest",
	Default: []string{},
	Help:    "Include additional server-side paths during comparison",
	Groups:  "Copy",
}, {
	Name:    "copy_dest",
	Default: []string{},
	Help:    "Implies --compare-dest but also copies files from paths into destination",
	Groups:  "Copy",
}, {
	Name:    "backup_dir",
	Default: "",
	Help:    "Make backups into hierarchy based in DIR",
	Groups:  "Sync",
}, {
	Name:    "suffix",
	Default: "",
	Help:    "Suffix to add to changed files",
	Groups:  "Sync",
}, {
	Name:    "suffix_keep_extension",
	Default: false,
	Help:    "Preserve the extension when using --suffix",
	Groups:  "Sync",
}, {
	Name:    "fast_list",
	Default: false,
	Help:    "Use recursive list if available; uses more memory but fewer transactions",
	Groups:  "Listing",
}, {
	Name:    "tpslimit",
	Default: 0.0,
	Help:    "Limit HTTP transactions per second to this",
	Groups:  "Networking",
}, {
	Name:    "tpslimit_burst",
	Default: 1,
	Help:    "Max burst of transactions for --tpslimit",
	Groups:  "Networking",
}, {
	Name:    "user_agent",
	Default: "rclone/" + Version,
	Help:    "Set the user-agent to a specified string",
	Groups:  "Networking",
}, {
	Name:    "immutable",
	Default: false,
	Help:    "Do not modify files, fail if existing files have been modified",
	Groups:  "Copy",
}, {
	Name:    "auto_confirm",
	Default: false,
	Help:    "If enabled, do not request console confirmation",
	Groups:  "Config",
}, {
	Name:    "stats_unit",
	Default: "bytes",
	Help:    "Show data rate in stats as either 'bits' or 'bytes' per second",
	Groups:  "Logging",
}, {
	Name:    "stats_file_name_length",
	Default: 45,
	Help:    "Max file name length in stats (0 for no limit)",
	Groups:  "Logging",
}, {
	Name:    "log_level",
	Default: LogLevelNotice,
	Help:    "Log level DEBUG|INFO|NOTICE|ERROR",
	Groups:  "Logging",
}, {
	Name:    "stats_log_level",
	Default: LogLevelInfo,
	Help:    "Log level to show --stats output DEBUG|INFO|NOTICE|ERROR",
	Groups:  "Logging",
}, {
	Name:    "bwlimit",
	Default: BwTimetable{},
	Help:    "Bandwidth limit in KiB/s, or use suffix B|K|M|G|T|P or a full timetable",
	Groups:  "Networking",
}, {
	Name:    "bwlimit_file",
	Default: BwTimetable{},
	Help:    "Bandwidth limit per file in KiB/s, or use suffix B|K|M|G|T|P or a full timetable",
	Groups:  "Networking",
}, {
	Name:    "buffer_size",
	Default: SizeSuffix(16 << 20),
	Help:    "In memory buffer size when reading files for each --transfer",
	Groups:  "Performance",
}, {
	Name:    "streaming_upload_cutoff",
	Default: SizeSuffix(100 * 1024),
	Help:    "Cutoff for switching to chunked upload if file size is unknown, upload starts after reaching cutoff or when file ends",
	Groups:  "Copy",
}, {
	Name:    "dump",
	Default: DumpFlags(0),
	Help:    "List of items to dump from: " + DumpFlagsList,
	Groups:  "Debugging",
}, {
	Name:    "max_transfer",
	Default: SizeSuffix(-1),
	Help:    "Maximum size of data to transfer",
	Groups:  "Copy",
}, {
	Name:    "max_duration",
	Default: time.Duration(0),
	Help:    "Maximum duration rclone will transfer data for",
	Groups:  "Copy",
}, {
	Name:    "cutoff_mode",
	Default: CutoffMode(0),
	Help:    "Mode to stop transfers when reaching the max transfer limit HARD|SOFT|CAUTIOUS",
	Groups:  "Copy",
}, {
	Name:    "max_backlog",
	Default: 10000,
	Help:    "Maximum number of objects in sync or check backlog",
	Groups:  "Copy,Check",
}, {
	Name:    "max_stats_groups",
	Default: 1000,
	Help:    "Maximum number of stats groups to keep in memory, on max oldest is discarded",
	Groups:  "Logging",
}, {
	Name:    "stats_one_line",
	Default: false,
	Help:    "Make the stats fit on one line",
	Groups:  "Logging",
}, {
	Name:    "stats_one_line_date",
	Default: false,
	Help:    "Enable --stats-one-line and add current date/time prefix",
	Groups:  "Logging",
}, {
	Name:    "stats_one_line_date_format",
	Default: "",
	Help:    "Enable --stats-one-line-date and use custom formatted date: Enclose date string in double quotes (\"), see https://golang.org/pkg/time/#Time.Format",
	Groups:  "Logging",
}, {
	Name:    "error_on_no_transfer",
	Default: false,
	Help:    "Sets exit code 9 if no files are transferred, useful in scripts",
	Groups:  "Config",
}, {
	Name:     "progress",
	ShortOpt: "P",
	Default:  false,
	Help:     "Show progress during transfer",
	Groups:   "Logging",
}, {
	Name:    "progress_terminal_title",
	Default: false,
	Help:    "Show progress on the terminal title (requires -P/--progress)",
	Groups:  "Logging",
}, {
	Name:    "use_cookies",
	Default: false,
	Help:    "Enable session cookiejar",
	Groups:  "Networking",
}, {
	Name:    "use_mmap",
	Default: false,
	Help:    "Use mmap allocator (see docs)",
	Groups:  "Config",
}, {
	Name:    "ca_cert",
	Default: []string{},
	Help:    "CA certificate used to verify servers",
	Groups:  "Networking",
}, {
	Name:    "client_cert",
	Default: "",
	Help:    "Client SSL certificate (PEM) for mutual TLS auth",
	Groups:  "Networking",
}, {
	Name:    "client_key",
	Default: "",
	Help:    "Client SSL private key (PEM) for mutual TLS auth",
	Groups:  "Networking",
}, {
	Name:    "multi_thread_cutoff",
	Default: SizeSuffix(256 * 1024 * 1024),
	Help:    "Use multi-thread downloads for files above this size",
	Groups:  "Copy",
}, {
	Name:    "multi_thread_streams",
	Default: 4,
	Help:    "Number of streams to use for multi-thread downloads",
	Groups:  "Copy",
}, {
	Name:    "multi_thread_write_buffer_size",
	Default: SizeSuffix(128 * 1024),
	Help:    "In memory buffer size for writing when in multi-thread mode",
	Groups:  "Copy",
}, {
	Name:    "multi_thread_chunk_size",
	Default: SizeSuffix(64 * 1024 * 1024),
	Help:    "Chunk size for multi-thread downloads / uploads, if not set by filesystem",
	Groups:  "Copy",
}, {
	Name:    "use_json_log",
	Default: false,
	Help:    "Use json log format",
	Groups:  "Logging",
}, {
	Name:    "order_by",
	Default: "",
	Help:    "Instructions on how to order the transfers, e.g. 'size,descending'",
	Groups:  "Copy",
}, {
	Name:    "refresh_times",
	Default: false,
	Help:    "Refresh the modtime of remote files",
	Groups:  "Copy",
}, {
	Name:    "no_console",
	Default: false,
	Help:    "Hide console window (supported on Windows only)",
	Groups:  "Config",
}, {
	Name:    "fs_cache_expire_duration",
	Default: 300 * time.Second,
	Help:    "Cache remotes for this long (0 to disable caching)",
	Groups:  "Config",
}, {
	Name:    "fs_cache_expire_interval",
	Default: 60 * time.Second,
	Help:    "Interval to check for expired remotes",
	Groups:  "Config",
}, {
	Name:    "disable_http2",
	Default: false,
	Help:    "Disable HTTP/2 in the global transport",
	Groups:  "Networking",
}, {
	Name:    "human_readable",
	Default: false,
	Help:    "Print numbers in a human-readable format, sizes with suffix Ki|Mi|Gi|Ti|Pi",
	Groups:  "Config",
}, {
	Name:    "kv_lock_time",
	Default: 1 * time.Second,
	Help:    "Maximum time to keep key-value database locked by process",
	Groups:  "Config",
}, {
	Name:    "disable_http_keep_alives",
	Default: false,
	Help:    "Disable HTTP keep-alives and use each connection once.",
	Groups:  "Networking",
}, {
	Name:     "metadata",
	ShortOpt: "M",
	Default:  false,
	Help:     "If set, preserve metadata when copying objects",
	Groups:   "Metadata,Copy",
}, {
	Name:    "server_side_across_configs",
	Default: false,
	Help:    "Allow server-side operations (e.g. copy) to work across different configs",
	Groups:  "Copy",
}, {
	Name:    "color",
	Default: TerminalColorMode(0),
	Help:    "When to show colors (and other ANSI codes) AUTO|NEVER|ALWAYS",
	Groups:  "Config",
}, {
	Name:    "default_time",
	Default: Time(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)),
	Help:    "Time to show if modtime is unknown for files and directories",
	Groups:  "Config,Listing",
}, {
	Name:    "inplace",
	Default: false,
	Help:    "Download directly to destination file instead of atomic download to temp/rename",
	Groups:  "Copy",
}, {
	Name:    "metadata_mapper",
	Default: SpaceSepList{},
	Help:    "Program to run to transforming metadata before upload",
	Groups:  "Metadata",
}, {
	Name:    "partial_suffix",
	Default: ".partial",
	Help:    "Add partial-suffix to temporary file name when --inplace is not used",
	Groups:  "Copy",
}}

// ConfigInfo is filesystem config options
type ConfigInfo struct {
	LogLevel                   LogLevel          `config:"log_level"`
	StatsLogLevel              LogLevel          `config:"stats_log_level"`
	UseJSONLog                 bool              `config:"use_json_log"`
	DryRun                     bool              `config:"dry_run"`
	Interactive                bool              `config:"interactive"`
	CheckSum                   bool              `config:"checksum"`
	SizeOnly                   bool              `config:"size_only"`
	IgnoreTimes                bool              `config:"ignore_times"`
	IgnoreExisting             bool              `config:"ignore_existing"`
	IgnoreErrors               bool              `config:"ignore_errors"`
	ModifyWindow               time.Duration     `config:"modify_window"`
	Checkers                   int               `config:"checkers"`
	Transfers                  int               `config:"transfers"`
	ConnectTimeout             time.Duration     `config:"contimeout"` // Connect timeout
	Timeout                    time.Duration     `config:"timeout"`    // Data channel timeout
	ExpectContinueTimeout      time.Duration     `config:"expect_continue_timeout"`
	Dump                       DumpFlags         `config:"dump"`
	InsecureSkipVerify         bool              `config:"no_check_certificate"` // Skip server certificate verification
	DeleteMode                 DeleteMode        `config:"delete_mode"`
	MaxDelete                  int64             `config:"max_delete"`
	MaxDeleteSize              SizeSuffix        `config:"max_delete_size"`
	TrackRenames               bool              `config:"track_renames"`          // Track file renames.
	TrackRenamesStrategy       string            `config:"track_renames_strategy"` // Comma separated list of strategies used to track renames
	Retries                    int               `config:"retries"`                // High-level retries
	RetriesInterval            time.Duration     `config:"retries_sleep"`
	LowLevelRetries            int               `config:"low_level_retries"`
	UpdateOlder                bool              `config:"update"`           // Skip files that are newer on the destination
	NoGzip                     bool              `config:"no_gzip_encoding"` // Disable compression
	MaxDepth                   int               `config:"max_depth"`
	IgnoreSize                 bool              `config:"ignore_size"`
	IgnoreChecksum             bool              `config:"ignore_checksum"`
	IgnoreCaseSync             bool              `config:"ignore_case_sync"`
	FixCase                    bool              `config:"fix_case"`
	NoTraverse                 bool              `config:"no_traverse"`
	CheckFirst                 bool              `config:"check_first"`
	NoCheckDest                bool              `config:"no_check_dest"`
	NoUnicodeNormalization     bool              `config:"no_unicode_normalization"`
	NoUpdateModTime            bool              `config:"no_update_modtime"`
	NoUpdateDirModTime         bool              `config:"no_update_dir_modtime"`
	DataRateUnit               string            `config:"stats_unit"`
	CompareDest                []string          `config:"compare_dest"`
	CopyDest                   []string          `config:"copy_dest"`
	BackupDir                  string            `config:"backup_dir"`
	Suffix                     string            `config:"suffix"`
	SuffixKeepExtension        bool              `config:"suffix_keep_extension"`
	UseListR                   bool              `config:"fast_list"`
	BufferSize                 SizeSuffix        `config:"buffer_size"`
	BwLimit                    BwTimetable       `config:"bwlimit"`
	BwLimitFile                BwTimetable       `config:"bwlimit_file"`
	TPSLimit                   float64           `config:"tpslimit"`
	TPSLimitBurst              int               `config:"tpslimit_burst"`
	BindAddr                   net.IP            `config:"bind_addr"`
	DisableFeatures            []string          `config:"disable"`
	UserAgent                  string            `config:"user_agent"`
	Immutable                  bool              `config:"immutable"`
	AutoConfirm                bool              `config:"auto_confirm"`
	StreamingUploadCutoff      SizeSuffix        `config:"streaming_upload_cutoff"`
	StatsFileNameLength        int               `config:"stats_file_name_length"`
	AskPassword                bool              `config:"ask_password"`
	PasswordCommand            SpaceSepList      `config:"password_command"`
	UseServerModTime           bool              `config:"use_server_modtime"`
	MaxTransfer                SizeSuffix        `config:"max_transfer"`
	MaxDuration                time.Duration     `config:"max_duration"`
	CutoffMode                 CutoffMode        `config:"cutoff_mode"`
	MaxBacklog                 int               `config:"max_backlog"`
	MaxStatsGroups             int               `config:"max_stats_groups"`
	StatsOneLine               bool              `config:"stats_one_line"`
	StatsOneLineDate           bool              `config:"stats_one_line_date"`        // If we want a date prefix at all
	StatsOneLineDateFormat     string            `config:"stats_one_line_date_format"` // If we want to customize the prefix
	ErrorOnNoTransfer          bool              `config:"error_on_no_transfer"`       // Set appropriate exit code if no files transferred
	Progress                   bool              `config:"progress"`
	ProgressTerminalTitle      bool              `config:"progress_terminal_title"`
	Cookie                     bool              `config:"use_cookies"`
	UseMmap                    bool              `config:"use_mmap"`
	CaCert                     []string          `config:"ca_cert"`     // Client Side CA
	ClientCert                 string            `config:"client_cert"` // Client Side Cert
	ClientKey                  string            `config:"client_key"`  // Client Side Key
	MultiThreadCutoff          SizeSuffix        `config:"multi_thread_cutoff"`
	MultiThreadStreams         int               `config:"multi_thread_streams"`
	MultiThreadSet             bool              `config:"multi_thread_set"`        // whether MultiThreadStreams was set (set in fs/config/configflags)
	MultiThreadChunkSize       SizeSuffix        `config:"multi_thread_chunk_size"` // Chunk size for multi-thread downloads / uploads, if not set by filesystem
	MultiThreadWriteBufferSize SizeSuffix        `config:"multi_thread_write_buffer_size"`
	OrderBy                    string            `config:"order_by"` // instructions on how to order the transfer
	UploadHeaders              []*HTTPOption     `config:"upload_headers"`
	DownloadHeaders            []*HTTPOption     `config:"download_headers"`
	Headers                    []*HTTPOption     `config:"headers"`
	MetadataSet                Metadata          `config:"metadata_set"` // extra metadata to write when uploading
	RefreshTimes               bool              `config:"refresh_times"`
	NoConsole                  bool              `config:"no_console"`
	TrafficClass               uint8             `config:"traffic_class"`
	FsCacheExpireDuration      time.Duration     `config:"fs_cache_expire_duration"`
	FsCacheExpireInterval      time.Duration     `config:"fs_cache_expire_interval"`
	DisableHTTP2               bool              `config:"disable_http2"`
	HumanReadable              bool              `config:"human_readable"`
	KvLockTime                 time.Duration     `config:"kv_lock_time"` // maximum time to keep key-value database locked by process
	DisableHTTPKeepAlives      bool              `config:"disable_http_keep_alives"`
	Metadata                   bool              `config:"metadata"`
	ServerSideAcrossConfigs    bool              `config:"server_side_across_configs"`
	TerminalColorMode          TerminalColorMode `config:"color"`
	DefaultTime                Time              `config:"default_time"` // time that directories with no time should display
	Inplace                    bool              `config:"inplace"`      // Download directly to destination file instead of atomic download to temp/rename
	PartialSuffix              string            `config:"partial_suffix"`
	MetadataMapper             SpaceSepList      `config:"metadata_mapper"`
}

func init() {
	// Set any values which aren't the zero for the type
	globalConfig.DeleteMode = DeleteModeDefault

	// Register the config and fill globalConfig with the defaults
	RegisterGlobalOptions(OptionsInfo{Name: "main", Opt: globalConfig, Options: ConfigOptionsInfo, Reload: globalConfig.Reload})

	// initial guess at log level from the flags
	globalConfig.LogLevel = initialLogLevel()
}

// Reload assumes the config has been edited and does what is necessary to make it live
func (ci *ConfigInfo) Reload(ctx context.Context) error {
	// Set -vv if --dump is in use
	if ci.Dump != 0 && ci.LogLevel != LogLevelDebug {
		Logf(nil, "Automatically setting -vv as --dump is enabled")
		ci.LogLevel = LogLevelDebug
	}

	// If --dry-run or -i then use NOTICE as minimum log level
	if (ci.DryRun || ci.Interactive) && ci.StatsLogLevel > LogLevelNotice {
		ci.StatsLogLevel = LogLevelNotice
	}

	// If --use-json-log then start the JSON logger
	if ci.UseJSONLog {
		InstallJSONLogger(ci.LogLevel)
	}

	// Check --compare-dest and --copy-dest
	if len(ci.CompareDest) > 0 && len(ci.CopyDest) > 0 {
		return fmt.Errorf("can't use --compare-dest with --copy-dest")
	}

	// Check --stats-one-line and dependent flags
	switch {
	case len(ci.StatsOneLineDateFormat) > 0:
		ci.StatsOneLineDate = true
		ci.StatsOneLine = true
	case ci.StatsOneLineDate:
		ci.StatsOneLineDateFormat = "2006/01/02 15:04:05 - "
		ci.StatsOneLine = true
	}

	// Check --partial-suffix
	if len(ci.PartialSuffix) > 16 {
		return fmt.Errorf("--partial-suffix: Expecting suffix length not greater than %d but got %d", 16, len(ci.PartialSuffix))
	}

	// Make sure some values are > 0
	nonZero := func(pi *int) {
		if *pi <= 0 {
			*pi = 1
		}
	}

	// Check --stats-unit
	if ci.DataRateUnit != "bits" && ci.DataRateUnit != "bytes" {
		Errorf(nil, "Unknown unit %q passed to --stats-unit. Defaulting to bytes.", ci.DataRateUnit)
		ci.DataRateUnit = "bytes"
	}

	// Check these are all > 0
	nonZero(&ci.Retries)
	nonZero(&ci.LowLevelRetries)
	nonZero(&ci.Transfers)
	nonZero(&ci.Checkers)

	return nil
}

// Initial logging level
//
// Perform a simple check for debug flags to enable debug logging during the flag initialization
func initialLogLevel() LogLevel {
	logLevel := LogLevelNotice
	for argIndex, arg := range os.Args {
		if strings.HasPrefix(arg, "-vv") && strings.TrimRight(arg, "v") == "-" {
			logLevel = LogLevelDebug
		}
		if arg == "--log-level=DEBUG" || (arg == "--log-level" && len(os.Args) > argIndex+1 && os.Args[argIndex+1] == "DEBUG") {
			logLevel = LogLevelDebug
		}
		if strings.HasPrefix(arg, "--verbose=") {
			if level, err := strconv.Atoi(arg[10:]); err == nil && level >= 2 {
				logLevel = LogLevelDebug
			}
		}
	}
	envValue, found := os.LookupEnv("RCLONE_LOG_LEVEL")
	if found && envValue == "DEBUG" {
		logLevel = LogLevelDebug
	}
	return logLevel
}

// TimeoutOrInfinite returns ci.Timeout if > 0 or infinite otherwise
func (ci *ConfigInfo) TimeoutOrInfinite() time.Duration {
	if ci.Timeout > 0 {
		return ci.Timeout
	}
	return ModTimeNotSupported
}

type configContextKeyType struct{}

// Context key for config
var configContextKey = configContextKeyType{}

// GetConfig returns the global or context sensitive context
func GetConfig(ctx context.Context) *ConfigInfo {
	if ctx == nil {
		return globalConfig
	}
	c := ctx.Value(configContextKey)
	if c == nil {
		return globalConfig
	}
	return c.(*ConfigInfo)
}

// CopyConfig copies the global config (if any) from srcCtx into
// dstCtx returning the new context.
func CopyConfig(dstCtx, srcCtx context.Context) context.Context {
	if srcCtx == nil {
		return dstCtx
	}
	c := srcCtx.Value(configContextKey)
	if c == nil {
		return dstCtx
	}
	return context.WithValue(dstCtx, configContextKey, c)
}

// AddConfig returns a mutable config structure based on a shallow
// copy of that found in ctx and returns a new context with that added
// to it.
func AddConfig(ctx context.Context) (context.Context, *ConfigInfo) {
	c := GetConfig(ctx)
	cCopy := new(ConfigInfo)
	*cCopy = *c
	newCtx := context.WithValue(ctx, configContextKey, cCopy)
	return newCtx, cCopy
}

// ConfigToEnv converts a config section and name, e.g. ("my-remote",
// "ignore-size") into an environment name
// "RCLONE_CONFIG_MY-REMOTE_IGNORE_SIZE"
func ConfigToEnv(section, name string) string {
	return "RCLONE_CONFIG_" + strings.ToUpper(section+"_"+strings.ReplaceAll(name, "-", "_"))
}

// OptionToEnv converts an option name, e.g. "ignore-size" into an
// environment name "RCLONE_IGNORE_SIZE"
func OptionToEnv(name string) string {
	return "RCLONE_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}
