// Package nbd provides a network block device server
package nbd

import (
	"bufio"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log"
	"math/bits"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rclone/gonbdserver/nbd"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/serve/proxy/proxyflags"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/lib/systemd"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/rclone/rclone/vfs/vfsflags"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const logPrefix = "nbd"

// OptionsInfo descripts the Options in use
var OptionsInfo = fs.Options{{
	Name:    "addr",
	Default: "localhost:10809",
	Help:    "IPaddress:Port or :Port to bind server to",
}, {
	Name:    "min_block_size",
	Default: fs.SizeSuffix(512), // FIXME
	Help:    "Minimum block size to advertise",
}, {
	Name:    "preferred_block_size",
	Default: fs.SizeSuffix(4096), // FIXME this is the max according to nbd-client
	Help:    "Preferred block size to advertise",
}, {
	Name:    "max_block_size",
	Default: fs.SizeSuffix(1024 * 1024), // FIXME,
	Help:    "Maximum block size to advertise",
}, {
	Name:    "create",
	Default: fs.SizeSuffix(-1),
	Help:    "If the destination does not exist, create it with this size",
}, {
	Name:    "chunk_size",
	Default: fs.SizeSuffix(0),
	Help:    "If creating the destination use this chunk size. Must be a power of 2.",
}, {
	Name:    "resize",
	Default: fs.SizeSuffix(-1),
	Help:    "If the destination exists, resize it to this size",
}}

// name := flag.String("name", "default", "Export name")
// description := flag.String("description", "The default export", "Export description")

// Options required for nbd server
type Options struct {
	ListenAddr         string        `config:"addr"` // Port to listen on
	MinBlockSize       fs.SizeSuffix `config:"min_block_size"`
	PreferredBlockSize fs.SizeSuffix `config:"preferred_block_size"`
	MaxBlockSize       fs.SizeSuffix `config:"max_block_size"`
	Create             fs.SizeSuffix `config:"create"`
	ChunkSize          fs.SizeSuffix `config:"chunk_size"`
	Resize             fs.SizeSuffix `config:"resize"`
}

func init() {
	fs.RegisterGlobalOptions(fs.OptionsInfo{Name: "nbd", Opt: &Opt, Options: OptionsInfo})
}

// Opt is options set by command line flags
var Opt Options

// AddFlags adds flags for the nbd
func AddFlags(flagSet *pflag.FlagSet, Opt *Options) {
	flags.AddFlagsFromOptions(flagSet, "", OptionsInfo)
}

func init() {
	flagSet := Command.Flags()
	vfsflags.AddFlags(flagSet)
	proxyflags.AddFlags(flagSet)
	AddFlags(flagSet, &Opt)
}

//go:embed nbd.md
var helpText string

// Command definition for cobra
var Command = &cobra.Command{
	Use:   "nbd remote:path",
	Short: `Serve the remote over NBD.`,
	Long:  helpText + vfs.Help(),
	Annotations: map[string]string{
		"versionIntroduced": "v1.65",
		"status":            "experimental",
	},
	Run: func(command *cobra.Command, args []string) {
		// FIXME could serve more than one nbd?
		cmd.CheckArgs(1, 1, command, args)
		f, leaf := cmd.NewFsFile(args[0])

		cmd.Run(false, true, command, func() error {
			s, err := run(context.Background(), f, leaf, Opt)
			if err != nil {
				log.Fatal(err)
			}

			defer systemd.Notify()()
			// FIXME
			_ = s
			s.Wait()
			return nil
		})
	},
}

// NBD contains everything to run the server
type NBD struct {
	f                fs.Fs
	leaf             string
	vfs              *vfs.VFS // don't use directly, use getVFS
	opt              Options
	wg               sync.WaitGroup
	sessionWaitGroup sync.WaitGroup
	logRd            *io.PipeReader
	logWr            *io.PipeWriter
	log2ChunkSize    uint
	readOnly         bool // Set for read only by vfs config

	backendFactory backendFactory
}

// interface for creating backend factories
type backendFactory interface {
	newBackend(ctx context.Context, ec *nbd.ExportConfig) (nbd.Backend, error)
}

// Create and start the server for nbd either on directory f or using file leaf in f
func run(ctx context.Context, f fs.Fs, leaf string, opt Options) (s *NBD, err error) {
	s = &NBD{
		f:        f,
		leaf:     leaf,
		opt:      opt,
		vfs:      vfs.New(f, &vfscommon.Opt),
		readOnly: vfscommon.Opt.ReadOnly,
	}

	if opt.ChunkSize != 0 {
		if set := bits.OnesCount64(uint64(opt.ChunkSize)); set != 1 {
			return nil, fmt.Errorf("--chunk-size must be a power of 2 (counted %d bits set)", set)
		}
		s.log2ChunkSize = uint(bits.TrailingZeros64(uint64(opt.ChunkSize)))
		fs.Debugf(logPrefix, "Using ChunkSize %v (%v), Log2ChunkSize %d", opt.ChunkSize, fs.SizeSuffix(1<<s.log2ChunkSize), s.log2ChunkSize)
	}
	if !vfscommon.Opt.ReadOnly && vfscommon.Opt.CacheMode < vfscommon.CacheModeWrites {
		return nil, errors.New("need --vfs-cache-mode writes or full when serving read/write")
	}

	// Create the backend factory
	if leaf != "" {
		s.backendFactory, err = s.newFileBackendFactory(ctx)
	} else {
		s.backendFactory, err = s.newChunkedBackendFactory(ctx)
	}
	if err != nil {
		return nil, err
	}
	nbd.RegisterBackend("rclone", s.backendFactory.newBackend)
	fs.Debugf(logPrefix, "Registered backends: %v", nbd.GetBackendNames())

	var (
		protocol = "tcp"
		addr     = Opt.ListenAddr
	)
	if strings.HasPrefix(addr, "unix://") || filepath.IsAbs(addr) {
		protocol = "unix"
		addr = strings.TrimPrefix(addr, "unix://")

	}

	ec := nbd.ExportConfig{
		Name:               "default",
		Description:        fs.ConfigString(f),
		Driver:             "rclone",
		ReadOnly:           vfscommon.Opt.ReadOnly,
		Workers:            8,     // should this be --checkers or a new config flag FIXME
		TLSOnly:            false, // FIXME
		MinimumBlockSize:   uint64(Opt.MinBlockSize),
		PreferredBlockSize: uint64(Opt.PreferredBlockSize),
		MaximumBlockSize:   uint64(Opt.MaxBlockSize),
		DriverParameters: nbd.DriverParametersConfig{
			"sync": "false",
			"path": "/tmp/diskimage",
		},
	}

	// Make a logger to feed gonbdserver's logs into rclone's logging system
	s.logRd, s.logWr = io.Pipe()
	go func() {
		scanner := bufio.NewScanner(s.logRd)
		for scanner.Scan() {
			line := scanner.Text()
			if s, ok := strings.CutPrefix(line, "[DEBUG] "); ok {
				fs.Debugf(logPrefix, "%s", s)
			} else if s, ok := strings.CutPrefix(line, "[INFO] "); ok {
				fs.Infof(logPrefix, "%s", s)
			} else if s, ok := strings.CutPrefix(line, "[WARN] "); ok {
				fs.Logf(logPrefix, "%s", s)
			} else if s, ok := strings.CutPrefix(line, "[ERROR] "); ok {
				fs.Errorf(logPrefix, "%s", s)
			} else if s, ok := strings.CutPrefix(line, "[CRIT] "); ok {
				fs.Errorf(logPrefix, "%s", s)
			} else {
				fs.Infof(logPrefix, "%s", line)
			}
		}
		if err := scanner.Err(); err != nil {
			fs.Errorf(logPrefix, "Log writer failed: %v", err)
		}
	}()
	logger := log.New(s.logWr, "", 0)

	ci := fs.GetConfig(ctx)
	dump := ci.Dump & (fs.DumpHeaders | fs.DumpBodies | fs.DumpAuth | fs.DumpRequests | fs.DumpResponses)
	var serverConfig = nbd.ServerConfig{
		Protocol:      protocol,               // protocol it should listen on (in net.Conn form)
		Address:       addr,                   // address to listen on
		DefaultExport: "default",              // name of default export
		Exports:       []nbd.ExportConfig{ec}, // array of configurations of exported items
		//TLS:             nbd.TLSConfig{},        // TLS configuration
		DisableNoZeroes: false,     // Disable NoZereos extension FIXME
		Debug:           dump != 0, // Verbose debug
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		// FIXME contexts
		nbd.StartServer(ctx, ctx, &s.sessionWaitGroup, logger, serverConfig)
	}()

	return s, nil
}

// Wait for the server to finish
func (s *NBD) Wait() {
	s.wg.Wait()
	_ = s.logWr.Close()
	_ = s.logRd.Close()
}
