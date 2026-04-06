// Package rs implements a virtual Reed-Solomon backend.
package rs

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/hash"
)

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "rs",
		Description: "Reed-Solomon virtual backend",
		NewFs:       NewFs,
		CommandHelp: commandHelp,
		Options: []fs.Option{{
			Name:     "remotes",
			Help:     "Comma-separated shard remotes in shard index order.",
			Required: true,
		}, {
			Name:     "data_shards",
			Help:     "Number of data shards (k).",
			Default:  4,
			Required: true,
		}, {
			Name:     "parity_shards",
			Help:     "Number of parity shards (m).",
			Default:  2,
			Required: true,
		}, {
			Name:     "use_spooling",
			Help:     "Spool shards to local disk before upload.",
			Default:  true,
			Advanced: true,
		}, {
			Name:     "staging_dir",
			Help:     "Directory for spooled shards. Empty means system temp.",
			Default:  "",
			Advanced: true,
		}, {
			Name:     "rollback",
			Help:     "Delete uploaded shards when write quorum is not met.",
			Default:  true,
			Advanced: true,
		}, {
			Name:     "max_parallel_uploads",
			Help:     "Maximum concurrent shard uploads during Put.",
			Default:  4,
			Advanced: true,
		}},
	})
}

var commandHelp = []fs.CommandHelp{{
	Name:  "status",
	Short: "Show RS backend health and quorum status",
}, {
	Name:  "heal",
	Short: "Scan objects and restore missing shards where at least k shards are readable",
	Long: `Scans logical objects (union of paths seen on shard remotes), and for each object
that is missing one or more shards but still has enough shards to reconstruct (>= data_shards),
reconstructs missing shard payloads and uploads them.

Without a path argument, all known objects are considered. With a path, only that logical
object is repaired (single-object repair).

This is an explicit, admin-driven repair. It does not run automatically on read.

Usage:

    rclone backend heal rs:
    rclone backend heal rs: path/to/file.bin

Options:

    -o dry-run=true    Report what would be healed without uploading shards

Examples:

    rclone backend heal rs:
    rclone backend heal rs: important.dat -o dry-run=true

Output includes counts (scanned / healed or would heal / skipped / failed) and per-object lines.
Objects with fewer than k good shards cannot be reconstructed and are reported as failed.
`,
	Opts: map[string]string{
		"dry-run": `If "true", only analyze and print "WOULD_HEAL" lines; no shard uploads.`,
	},
}}

// Options defines backend configuration.
type Options struct {
	Remotes            string `config:"remotes"`
	DataShards         int    `config:"data_shards"`
	ParityShards       int    `config:"parity_shards"`
	UseSpooling        bool   `config:"use_spooling"`
	StagingDir         string `config:"staging_dir"`
	Rollback           bool   `config:"rollback"`
	MaxParallelUploads int    `config:"max_parallel_uploads"`
}

// Fs represents an rs backend.
type Fs struct {
	name     string
	root     string
	opt      Options
	backends []fs.Fs
	features *fs.Features
	hashSet  hash.Set
}

// NewFs creates a new rs backend.
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	if err := configstruct.Set(m, opt); err != nil {
		return nil, err
	}
	if err := validateOptions(opt); err != nil {
		return nil, err
	}
	remoteList := parseRemotes(opt.Remotes)
	if len(remoteList) != opt.DataShards+opt.ParityShards {
		return nil, fmt.Errorf("rs: remotes count must equal data_shards + parity_shards (%d), got %d", opt.DataShards+opt.ParityShards, len(remoteList))
	}

	// When rclone targets a file (e.g. remote:path/to/object), some underlying backends
	// must be initialized on the parent directory rather than the file path.
	// We detect this via cache.Get returning fs.ErrorIsFile and then retry with root=dir(root).
	returnErrorIsFile := false
	backends := make([]fs.Fs, 0, len(remoteList))

	// Initialize shard 0 first so we can apply the "root points at file" fix consistently.
	shardPath0 := fspath.JoinRootPath(remoteList[0], root)
	firstBackend, err := cache.Get(ctx, shardPath0)
	if err != nil {
		isFileErr := errors.Is(err, fs.ErrorIsFile) || err.Error() == fs.ErrorIsFile.Error() || strings.Contains(err.Error(), fs.ErrorIsFile.Error())
		if isFileErr {
			adj := path.Dir(root)
			if adj == "." || adj == "/" {
				adj = ""
			}
			root = adj
			returnErrorIsFile = true

			shardPath0 = fspath.JoinRootPath(remoteList[0], root)
			firstBackend, err = cache.Get(ctx, shardPath0)
		}
		if err != nil {
			return nil, fmt.Errorf("rs: failed to initialize remote %q: %w", shardPath0, err)
		}
	}
	backends = append(backends, firstBackend)

	for _, remote := range remoteList[1:] {
		shardPath := fspath.JoinRootPath(remote, root)
		b, err := cache.Get(ctx, shardPath)
		if err != nil {
			return nil, fmt.Errorf("rs: failed to initialize remote %q: %w", shardPath, err)
		}
		backends = append(backends, b)
	}

	f := &Fs{
		name:     name,
		root:     root,
		opt:      *opt,
		backends: backends,
		hashSet:  hash.NewHashSet(hash.MD5, hash.SHA256),
	}
	f.features = (&fs.Features{}).Fill(ctx, f)
	if returnErrorIsFile {
		return f, fs.ErrorIsFile
	}
	return f, nil
}

func validateOptions(opt *Options) error {
	if opt.DataShards < 1 {
		return errors.New("rs: data_shards must be >= 1")
	}
	if opt.ParityShards < 1 {
		return errors.New("rs: parity_shards must be >= 1")
	}
	if opt.DataShards+opt.ParityShards > 255 {
		return errors.New("rs: data_shards + parity_shards must be <= 255")
	}
	if opt.MaxParallelUploads < 1 {
		opt.MaxParallelUploads = 1
	}
	return nil
}

func parseRemotes(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// Name returns the fs name for this remote.
func (f *Fs) Name() string { return f.name }

// Root returns the root path within the virtual backend.
func (f *Fs) Root() string { return f.root }

func (f *Fs) String() string { return fmt.Sprintf("RS root %q", f.root) }

// Features describes optional capabilities exposed by rs.
func (f *Fs) Features() *fs.Features { return f.features }

// Precision returns timestamp resolution for ModTime.
func (f *Fs) Precision() time.Duration { return time.Second }

// Hashes returns hash types supported for logical objects.
func (f *Fs) Hashes() hash.Set { return f.hashSet }

// Mkdir creates the directory on every shard backend.
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	for _, b := range f.backends {
		if err := b.Mkdir(ctx, dir); err != nil {
			return err
		}
	}
	return nil
}

// Rmdir removes the directory from every shard backend.
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	for _, b := range f.backends {
		if err := b.Rmdir(ctx, dir); err != nil {
			return err
		}
	}
	return nil
}
