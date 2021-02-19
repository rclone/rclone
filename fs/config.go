package fs

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// Global
var (
	// globalConfig for rclone
	globalConfig = NewConfig()

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

	// CountError counts an error.  If any errors have been
	// counted then rclone will exit with a non zero error code.
	//
	// This is a function pointer to decouple the config
	// implementation from the fs
	CountError = func(err error) error { return err }

	// ConfigProvider is the config key used for provider options
	ConfigProvider = "provider"
)

// ConfigInfo is filesystem config options
type ConfigInfo struct {
	LogLevel               LogLevel
	StatsLogLevel          LogLevel
	UseJSONLog             bool
	DryRun                 bool
	Interactive            bool
	CheckSum               bool
	SizeOnly               bool
	IgnoreTimes            bool
	IgnoreExisting         bool
	IgnoreErrors           bool
	ModifyWindow           time.Duration
	Checkers               int
	Transfers              int
	ConnectTimeout         time.Duration // Connect timeout
	Timeout                time.Duration // Data channel timeout
	ExpectContinueTimeout  time.Duration
	Dump                   DumpFlags
	InsecureSkipVerify     bool // Skip server certificate verification
	DeleteMode             DeleteMode
	MaxDelete              int64
	TrackRenames           bool   // Track file renames.
	TrackRenamesStrategy   string // Comma separated list of strategies used to track renames
	LowLevelRetries        int
	UpdateOlder            bool // Skip files that are newer on the destination
	NoGzip                 bool // Disable compression
	MaxDepth               int
	IgnoreSize             bool
	IgnoreChecksum         bool
	IgnoreCaseSync         bool
	NoTraverse             bool
	CheckFirst             bool
	NoCheckDest            bool
	NoUnicodeNormalization bool
	NoUpdateModTime        bool
	DataRateUnit           string
	CompareDest            []string
	CopyDest               []string
	BackupDir              string
	Suffix                 string
	SuffixKeepExtension    bool
	UseListR               bool
	BufferSize             SizeSuffix
	BwLimit                BwTimetable
	BwLimitFile            BwTimetable
	TPSLimit               float64
	TPSLimitBurst          int
	BindAddr               net.IP
	DisableFeatures        []string
	UserAgent              string
	Immutable              bool
	AutoConfirm            bool
	StreamingUploadCutoff  SizeSuffix
	StatsFileNameLength    int
	AskPassword            bool
	PasswordCommand        SpaceSepList
	UseServerModTime       bool
	MaxTransfer            SizeSuffix
	MaxDuration            time.Duration
	CutoffMode             CutoffMode
	MaxBacklog             int
	MaxStatsGroups         int
	StatsOneLine           bool
	StatsOneLineDate       bool   // If we want a date prefix at all
	StatsOneLineDateFormat string // If we want to customize the prefix
	ErrorOnNoTransfer      bool   // Set appropriate exit code if no files transferred
	Progress               bool
	ProgressTerminalTitle  bool
	Cookie                 bool
	UseMmap                bool
	CaCert                 string // Client Side CA
	ClientCert             string // Client Side Cert
	ClientKey              string // Client Side Key
	MultiThreadCutoff      SizeSuffix
	MultiThreadStreams     int
	MultiThreadSet         bool   // whether MultiThreadStreams was set (set in fs/config/configflags)
	OrderBy                string // instructions on how to order the transfer
	UploadHeaders          []*HTTPOption
	DownloadHeaders        []*HTTPOption
	Headers                []*HTTPOption
	RefreshTimes           bool
	NoConsole              bool
	TrafficClass           uint8
}

// NewConfig creates a new config with everything set to the default
// value.  These are the ultimate defaults and are overridden by the
// config module.
func NewConfig() *ConfigInfo {
	c := new(ConfigInfo)

	// Set any values which aren't the zero for the type
	c.LogLevel = LogLevelNotice
	c.StatsLogLevel = LogLevelInfo
	c.ModifyWindow = time.Nanosecond
	c.Checkers = 8
	c.Transfers = 4
	c.ConnectTimeout = 60 * time.Second
	c.Timeout = 5 * 60 * time.Second
	c.ExpectContinueTimeout = 1 * time.Second
	c.DeleteMode = DeleteModeDefault
	c.MaxDelete = -1
	c.LowLevelRetries = 10
	c.MaxDepth = -1
	c.DataRateUnit = "bytes"
	c.BufferSize = SizeSuffix(16 << 20)
	c.UserAgent = "rclone/" + Version
	c.StreamingUploadCutoff = SizeSuffix(100 * 1024)
	c.MaxStatsGroups = 1000
	c.StatsFileNameLength = 45
	c.AskPassword = true
	c.TPSLimitBurst = 1
	c.MaxTransfer = -1
	c.MaxBacklog = 10000
	// We do not want to set the default here. We use this variable being empty as part of the fall-through of options.
	//	c.StatsOneLineDateFormat = "2006/01/02 15:04:05 - "
	c.MultiThreadCutoff = SizeSuffix(250 * 1024 * 1024)
	c.MultiThreadStreams = 4

	c.TrackRenamesStrategy = "hash"

	return c
}

// TimeoutOrInfinite returns ci.Timeout if > 0 or infinite otherwise
func (c *ConfigInfo) TimeoutOrInfinite() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
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

// ConfigToEnv converts a config section and name, e.g. ("myremote",
// "ignore-size") into an environment name
// "RCLONE_CONFIG_MYREMOTE_IGNORE_SIZE"
func ConfigToEnv(section, name string) string {
	return "RCLONE_CONFIG_" + strings.ToUpper(strings.Replace(section+"_"+name, "-", "_", -1))
}

// OptionToEnv converts an option name, e.g. "ignore-size" into an
// environment name "RCLONE_IGNORE_SIZE"
func OptionToEnv(name string) string {
	return "RCLONE_" + strings.ToUpper(strings.Replace(name, "-", "_", -1))
}
