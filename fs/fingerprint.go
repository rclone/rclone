package fs

import (
	"context"
	"fmt"
	"strings"

	"github.com/rclone/rclone/fs/hash"
)

// Fingerprint produces a unique-ish string for an object.
//
// This is for detecting whether an object has changed since we last
// saw it, not for checking object identity between two different
// remotes - operations.Equal should be used for that.
//
// If fast is set then Fingerprint will only include attributes where
// usually another operation is not required to fetch them. For
// example if fast is set then this won't include hashes on the local
// backend.
func Fingerprint(ctx context.Context, o ObjectInfo, fast bool) string {
	var (
		out      strings.Builder
		f        = o.Fs()
		features = f.Features()
	)
	fmt.Fprintf(&out, "%d", o.Size())
	// Whether we want to do a slow operation or not
	//
	//  fast     true  false true  false
	//  opIsSlow true  true  false false
	//  do Op    false true  true  true
	//
	// If !fast (slow) do the operation or if !OpIsSlow ==
	// OpIsFast do the operation.
	//
	// Eg don't do this for S3 where modtimes are expensive
	if !fast || !features.SlowModTime {
		if f.Precision() != ModTimeNotSupported {
			fmt.Fprintf(&out, ",%v", o.ModTime(ctx).UTC())
		}
	}
	// Eg don't do this for SFTP/local where hashes are expensive?
	if !fast || !features.SlowHash {
		hashType := f.Hashes().GetOne()
		if hashType != hash.None {
			hash, err := o.Hash(ctx, hashType)
			if err == nil {
				fmt.Fprintf(&out, ",%v", hash)
			}
		}
	}
	return out.String()
}
