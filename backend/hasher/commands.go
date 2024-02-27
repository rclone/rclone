package hasher

import (
	"context"
	"errors"
	"fmt"
	"path"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/kv"
)

// Command the backend to run a named command
//
// The command run is name
// args may be used to read arguments from
// opts may be used to read optional arguments from
//
// The result should be capable of being JSON encoded
// If it is a string or a []string it will be shown to the user
// otherwise it will be JSON encoded and shown to the user like that
func (f *Fs) Command(ctx context.Context, name string, arg []string, opt map[string]string) (out interface{}, err error) {
	switch name {
	case "drop":
		return nil, f.db.Stop(true)
	case "dump", "fulldump":
		return nil, f.dbDump(ctx, name == "fulldump", "")
	case "import", "stickyimport":
		sticky := name == "stickyimport"
		if len(arg) != 2 {
			return nil, errors.New("please provide checksum type and path to sum file")
		}
		return nil, f.dbImport(ctx, arg[0], arg[1], sticky)
	default:
		return nil, fs.ErrorCommandNotFound
	}
}

var commandHelp = []fs.CommandHelp{{
	Name:  "drop",
	Short: "Drop cache",
	Long: `Completely drop checksum cache.
Usage Example:
    rclone backend drop hasher:
`,
}, {
	Name:  "dump",
	Short: "Dump the database",
	Long:  "Dump cache records covered by the current remote",
}, {
	Name:  "fulldump",
	Short: "Full dump of the database",
	Long:  "Dump all cache records in the database",
}, {
	Name:  "import",
	Short: "Import a SUM file",
	Long: `Amend hash cache from a SUM file and bind checksums to files by size/time.
Usage Example:
    rclone backend import hasher:subdir md5 /path/to/sum.md5
`,
}, {
	Name:  "stickyimport",
	Short: "Perform fast import of a SUM file",
	Long: `Fill hash cache from a SUM file without verifying file fingerprints.
Usage Example:
    rclone backend stickyimport hasher:subdir md5 remote:path/to/sum.md5
`,
}}

func (f *Fs) dbDump(ctx context.Context, full bool, root string) error {
	if root == "" {
		remoteFs, err := cache.Get(ctx, f.opt.Remote)
		if err != nil {
			return err
		}
		root = fspath.JoinRootPath(remoteFs.Root(), f.Root())
	}
	if f.db == nil {
		if f.opt.MaxAge == 0 {
			fs.Errorf(f, "db not found. (disabled with max_age = 0)")
		} else {
			fs.Errorf(f, "db not found.")
		}
		return kv.ErrInactive
	}
	op := &kvDump{
		full: full,
		root: root,
		path: f.db.Path(),
		fs:   f,
	}
	err := f.db.Do(false, op)
	if err == kv.ErrEmpty {
		fs.Infof(op.path, "empty")
		err = nil
	}
	return err
}

func (f *Fs) dbImport(ctx context.Context, hashName, sumRemote string, sticky bool) error {
	var hashType hash.Type
	if err := hashType.Set(hashName); err != nil {
		return err
	}
	if hashType == hash.None {
		return errors.New("please provide a valid hash type")
	}
	if !f.suppHashes.Contains(hashType) {
		return errors.New("unsupported hash type")
	}
	if !f.keepHashes.Contains(hashType) {
		fs.Infof(nil, "Need not import hashes of this type")
		return nil
	}

	_, sumPath, err := fspath.SplitFs(sumRemote)
	if err != nil {
		return err
	}
	sumFs, err := cache.Get(ctx, sumRemote)
	switch err {
	case fs.ErrorIsFile:
		// ok
	case nil:
		return fmt.Errorf("not a file: %s", sumRemote)
	default:
		return err
	}

	sumObj, err := sumFs.NewObject(ctx, path.Base(sumPath))
	if err != nil {
		return fmt.Errorf("cannot open sum file: %w", err)
	}
	hashes, err := operations.ParseSumFile(ctx, sumObj)
	if err != nil {
		return fmt.Errorf("failed to parse sum file: %w", err)
	}

	if sticky {
		rootPath := f.Fs.Root()
		for remote, hashVal := range hashes {
			key := path.Join(rootPath, remote)
			hashSums := operations.HashSums{hashName: hashVal}
			if err := f.putRawHashes(ctx, key, anyFingerprint, hashSums); err != nil {
				fs.Errorf(nil, "%s: failed to import: %v", remote, err)
			}
		}
		fs.Infof(nil, "Summary: %d checksum(s) imported", len(hashes))
		return nil
	}

	const longImportThreshold = 100
	if len(hashes) > longImportThreshold {
		fs.Infof(nil, "Importing %d checksums. Please wait...", len(hashes))
	}

	doneCount := 0
	err = operations.ListFn(ctx, f, func(obj fs.Object) {
		remote := obj.Remote()
		hash := hashes[remote]
		hashes[remote] = "" // mark as handled
		o, ok := obj.(*Object)
		if ok && hash != "" {
			if err := o.putHashes(ctx, hashMap{hashType: hash}); err != nil {
				fs.Errorf(nil, "%s: failed to import: %v", remote, err)
			}
			accounting.Stats(ctx).NewCheckingTransfer(obj, "importing").Done(ctx, err)
			doneCount++
		}
	})
	if err != nil {
		fs.Errorf(nil, "Import failed: %v", err)
	}
	skipCount := 0
	for remote, emptyOrDone := range hashes {
		if emptyOrDone != "" {
			fs.Infof(nil, "Skip vanished object: %s", remote)
			skipCount++
		}
	}
	fs.Infof(nil, "Summary: %d imported, %d skipped", doneCount, skipCount)
	return err
}
