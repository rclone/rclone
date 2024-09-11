// Package configflags defines the flags used by rclone.  It is
// decoupled into a separate package so it can be replaced.
package configflags

// Options set by command line flags
import (
	"context"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/flags"
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
)

// AddFlags adds the non filing system specific flags to the command
func AddFlags(ci *fs.ConfigInfo, flagSet *pflag.FlagSet) {
	flags.AddFlagsFromOptions(flagSet, "", fs.ConfigOptionsInfo)

	// Add flags we haven't converted into options yet
	flags.CountVarP(flagSet, &verbose, "verbose", "v", "Print lots more stuff (repeat for more)", "Logging,Important")
	flags.BoolVarP(flagSet, &quiet, "quiet", "q", false, "Print as little stuff as possible", "Logging")
	flags.StringVarP(flagSet, &configPath, "config", "", config.GetConfigPath(), "Config file", "Config")
	flags.StringVarP(flagSet, &cacheDir, "cache-dir", "", config.GetCacheDir(), "Directory rclone will use for caching", "Config")
	flags.StringVarP(flagSet, &tempDir, "temp-dir", "", os.TempDir(), "Directory rclone will use for temporary files", "Config")
	flags.BoolVarP(flagSet, &dumpHeaders, "dump-headers", "", false, "Dump HTTP headers - may contain sensitive info", "Debugging")
	flags.BoolVarP(flagSet, &dumpBodies, "dump-bodies", "", false, "Dump HTTP headers and bodies - may contain sensitive info", "Debugging")
	flags.BoolVarP(flagSet, &deleteBefore, "delete-before", "", false, "When synchronizing, delete files on destination before transferring", "Sync")
	flags.BoolVarP(flagSet, &deleteDuring, "delete-during", "", false, "When synchronizing, delete files during transfer", "Sync")
	flags.BoolVarP(flagSet, &deleteAfter, "delete-after", "", false, "When synchronizing, delete files on destination after transferring (default)", "Sync")
	flags.StringVarP(flagSet, &bindAddr, "bind", "", "", "Local address to bind to for outgoing connections, IPv4, IPv6 or name", "Networking")
	flags.StringVarP(flagSet, &disableFeatures, "disable", "", "", "Disable a comma separated list of features (use --disable help to see a list)", "Config")
	flags.StringArrayVarP(flagSet, &uploadHeaders, "header-upload", "", nil, "Set HTTP header for upload transactions", "Networking")
	flags.StringArrayVarP(flagSet, &downloadHeaders, "header-download", "", nil, "Set HTTP header for download transactions", "Networking")
	flags.StringArrayVarP(flagSet, &headers, "header", "", nil, "Set HTTP header for all transactions", "Networking")
	flags.StringArrayVarP(flagSet, &metadataSet, "metadata-set", "", nil, "Add metadata key=value when uploading", "Metadata")
	flags.StringVarP(flagSet, &dscp, "dscp", "", "", "Set DSCP value to connections, value or name, e.g. CS1, LE, DF, AF21", "Networking")
}

// ParseHeaders converts the strings passed in via the header flags into HTTPOptions
func ParseHeaders(headers []string) []*fs.HTTPOption {
	opts := []*fs.HTTPOption{}
	for _, header := range headers {
		parts := strings.SplitN(header, ":", 2)
		if len(parts) == 1 {
			fs.Fatalf(nil, "Failed to parse '%s' as an HTTP header. Expecting a string like: 'Content-Encoding: gzip'", header)
		}
		option := &fs.HTTPOption{
			Key:   strings.TrimSpace(parts[0]),
			Value: strings.TrimSpace(parts[1]),
		}
		opts = append(opts, option)
	}
	return opts
}

// SetFlags sets flags which aren't part of the config system
func SetFlags(ci *fs.ConfigInfo) {
	// Process obsolete --dump-headers and --dump-bodies flags
	if dumpHeaders {
		ci.Dump |= fs.DumpHeaders
		fs.Logf(nil, "--dump-headers is obsolete - please use --dump headers instead")
	}
	if dumpBodies {
		ci.Dump |= fs.DumpBodies
		fs.Logf(nil, "--dump-bodies is obsolete - please use --dump bodies instead")
	}

	// Process -v flag
	if verbose >= 2 {
		ci.LogLevel = fs.LogLevelDebug
	} else if verbose >= 1 {
		ci.LogLevel = fs.LogLevelInfo
	}

	// Process -q flag
	if quiet {
		if verbose > 0 {
			fs.Fatalf(nil, "Can't set -v and -q")
		}
		ci.LogLevel = fs.LogLevelError
	}

	// Can't set log level, -v, -q
	logLevelFlag := pflag.Lookup("log-level")
	if logLevelFlag != nil && logLevelFlag.Changed {
		if verbose > 0 {
			fs.Fatalf(nil, "Can't set -v and --log-level")
		}
		if quiet {
			fs.Fatalf(nil, "Can't set -q and --log-level")
		}
	}

	// Process --delete-before, --delete-during and --delete-after
	switch {
	case deleteBefore && (deleteDuring || deleteAfter),
		deleteDuring && deleteAfter:
		fs.Fatalf(nil, `Only one of --delete-before, --delete-during or --delete-after can be used.`)
	case deleteBefore:
		ci.DeleteMode = fs.DeleteModeBefore
	case deleteDuring:
		ci.DeleteMode = fs.DeleteModeDuring
	case deleteAfter:
		ci.DeleteMode = fs.DeleteModeAfter
	default:
		ci.DeleteMode = fs.DeleteModeDefault
	}

	// Process --bind into IP address
	if bindAddr != "" {
		addrs, err := net.LookupIP(bindAddr)
		if err != nil {
			fs.Fatalf(nil, "--bind: Failed to parse %q as IP address: %v", bindAddr, err)
		}
		if len(addrs) != 1 {
			fs.Fatalf(nil, "--bind: Expecting 1 IP address for %q but got %d", bindAddr, len(addrs))
		}
		ci.BindAddr = addrs[0]
	}

	// Process --disable
	if disableFeatures != "" {
		if disableFeatures == "help" {
			fs.Fatalf(nil, "Possible backend features are: %s\n", strings.Join(new(fs.Features).List(), ", "))
		}
		ci.DisableFeatures = strings.Split(disableFeatures, ",")
	}

	// Process --headers-upload, --headers-download, --headers
	if len(uploadHeaders) != 0 {
		ci.UploadHeaders = ParseHeaders(uploadHeaders)
	}
	if len(downloadHeaders) != 0 {
		ci.DownloadHeaders = ParseHeaders(downloadHeaders)
	}
	if len(headers) != 0 {
		ci.Headers = ParseHeaders(headers)
	}

	// Process --metadata-set
	if len(metadataSet) != 0 {
		ci.MetadataSet = make(fs.Metadata, len(metadataSet))
		for _, kv := range metadataSet {
			equal := strings.IndexRune(kv, '=')
			if equal < 0 {
				fs.Fatalf(nil, "Failed to parse '%s' as metadata key=value.", kv)
			}
			ci.MetadataSet[strings.ToLower(kv[:equal])] = kv[equal+1:]
		}
		fs.Debugf(nil, "MetadataUpload %v", ci.MetadataSet)
	}

	// Process --dscp
	if len(dscp) != 0 {
		if value, ok := parseDSCP(dscp); ok {
			ci.TrafficClass = value << 2
		} else {
			fs.Fatalf(nil, "--dscp: Invalid DSCP name: %v", dscp)
		}
	}

	// Process --config path
	if err := config.SetConfigPath(configPath); err != nil {
		fs.Fatalf(nil, "--config: Failed to set %q as config path: %v", configPath, err)
	}

	// Process --cache-dir path
	if err := config.SetCacheDir(cacheDir); err != nil {
		fs.Fatalf(nil, "--cache-dir: Failed to set %q as cache dir: %v", cacheDir, err)
	}

	// Process --temp-dir path
	if err := config.SetTempDir(tempDir); err != nil {
		fs.Fatalf(nil, "--temp-dir: Failed to set %q as temp dir: %v", tempDir, err)
	}

	// Process --multi-thread-streams - set whether multi-thread-streams was set
	multiThreadStreamsFlag := pflag.Lookup("multi-thread-streams")
	ci.MultiThreadSet = multiThreadStreamsFlag != nil && multiThreadStreamsFlag.Changed

	// Reload any changes
	if err := ci.Reload(context.Background()); err != nil {
		fs.Fatalf(nil, "Failed to reload config changes: %v", err)
	}
}

// parseDSCP converts DSCP names to value
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
