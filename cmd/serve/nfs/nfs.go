//go:build unix

// Package nfs implements a server to serve a VFS remote over the NFSv3 protocol
//
// There is no authentication available on this server and it is
// served on the loopback interface by default.
//
// This is primarily used for mounting a VFS remote in macOS, where
// FUSE-mounting mechanisms are usually not available.
package nfs

import (
	"context"
	"strings"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/rclone/rclone/vfs/vfsflags"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// OptionsInfo descripts the Options in use
var OptionsInfo = fs.Options{{
	Name:    "addr",
	Default: "",
	Help:    "IPaddress:Port or :Port to bind server to",
}, {
	Name:    "nfs_cache_handle_limit",
	Default: 1000000,
	Help:    "max file handles cached simultaneously (min 5)",
}, {
	Name:    "nfs_cache_type",
	Default: cacheMemory,
	Help:    "Type of NFS handle cache to use",
}, {
	Name:    "nfs_cache_dir",
	Default: "",
	Help:    "The directory the NFS handle cache will use if set",
}}

func init() {
	fs.RegisterGlobalOptions(fs.OptionsInfo{Name: "nfs", Opt: &Opt, Options: OptionsInfo})
}

type handleCache = fs.Enum[handleCacheChoices]

const (
	cacheMemory handleCache = iota
	cacheDisk
	cacheSymlink
)

type handleCacheChoices struct{}

func (handleCacheChoices) Choices() []string {
	return []string{
		cacheMemory:  "memory",
		cacheDisk:    "disk",
		cacheSymlink: "symlink",
	}
}

// Options contains options for the NFS Server
type Options struct {
	ListenAddr     string      `config:"addr"`                   // Port to listen on
	HandleLimit    int         `config:"nfs_cache_handle_limit"` // max file handles cached by go-nfs CachingHandler
	HandleCache    handleCache `config:"nfs_cache_type"`         // what kind of handle cache to use
	HandleCacheDir string      `config:"nfs_cache_dir"`          // where the handle cache should be stored
}

// Opt is the default set of serve nfs options
var Opt Options

// AddFlags adds flags for serve nfs (and nfsmount)
func AddFlags(flagSet *pflag.FlagSet) {
	flags.AddFlagsFromOptions(flagSet, "", OptionsInfo)
}

func init() {
	vfsflags.AddFlags(Command.Flags())
	AddFlags(Command.Flags())
}

// Run the command
func Run(command *cobra.Command, args []string) {
	var f fs.Fs
	cmd.CheckArgs(1, 1, command, args)
	f = cmd.NewFsSrc(args)
	cmd.Run(false, true, command, func() error {
		s, err := NewServer(context.Background(), vfs.New(f, &vfscommon.Opt), &Opt)
		if err != nil {
			return err
		}
		return s.Serve()
	})
}

// Command is the definition of the command
var Command = &cobra.Command{
	Use:   "nfs remote:path",
	Short: `Serve the remote as an NFS mount`,
	Long: strings.ReplaceAll(`Create an NFS server that serves the given remote over the network.
	
This implements an NFSv3 server to serve any rclone remote via NFS.

The primary purpose for this command is to enable the [mount
command](/commands/rclone_mount/) on recent macOS versions where
installing FUSE is very cumbersome.

This server does not implement any authentication so any client will be
able to access the data. To limit access, you can use |serve nfs| on
the loopback address or rely on secure tunnels (such as SSH) or use
firewalling.

For this reason, by default, a random TCP port is chosen and the
loopback interface is used for the listening address by default;
meaning that it is only available to the local machine. If you want
other machines to access the NFS mount over local network, you need to
specify the listening address and port using the |--addr| flag.

Modifying files through the NFS protocol requires VFS caching. Usually
you will need to specify |--vfs-cache-mode| in order to be able to
write to the mountpoint (|full| is recommended). If you don't specify
VFS cache mode, the mount will be read-only.

|--nfs-cache-type| controls the type of the NFS handle cache. By
default this is |memory| where new handles will be randomly allocated
when needed. These are stored in memory. If the server is restarted
the handle cache will be lost and connected NFS clients will get stale
handle errors.

|--nfs-cache-type disk| uses an on disk NFS handle cache. Rclone
hashes the path of the object and stores it in a file named after the
hash. These hashes are stored on disk the directory controlled by
|--cache-dir| or the exact directory may be specified with
|--nfs-cache-dir|. Using this means that the NFS server can be
restarted at will without affecting the connected clients.

|--nfs-cache-type symlink| is similar to |--nfs-cache-type disk| in
that it uses an on disk cache, but the cache entries are held as
symlinks. Rclone will use the handle of the underlying file as the NFS
handle which improves performance. This sort of cache can't be backed
up and restored as the underlying handles will change. This is Linux
only.

|--nfs-cache-handle-limit| controls the maximum number of cached NFS
handles stored by the caching handler. This should not be set too low
or you may experience errors when trying to access files. The default
is |1000000|, but consider lowering this limit if the server's system
resource usage causes problems. This is only used by the |memory| type
cache.

To serve NFS over the network use following command:

    rclone serve nfs remote: --addr 0.0.0.0:$PORT --vfs-cache-mode=full

This specifies a port that can be used in the mount command. To mount
the server under Linux/macOS, use the following command:
    
    mount -t nfs -o port=$PORT,mountport=$PORT,tcp $HOSTNAME:/ path/to/mountpoint

Where |$PORT| is the same port number used in the |serve nfs| command
and |$HOSTNAME| is the network address of the machine that |serve nfs|
was run on.

This command is only available on Unix platforms.

`, "|", "`") + vfs.Help(),
	Annotations: map[string]string{
		"versionIntroduced": "v1.65",
		"groups":            "Filter",
		"status":            "Experimental",
	},
	Run: Run,
}
