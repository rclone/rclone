package bisync

import (
	"context"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/sync/trackrenames"
)

func (b *bisyncRun) trackRenamesPreflight() bool {
	return b.opt.MaxDeleteRenamesAware && !b.opt.Force
}

func (b *bisyncRun) trackedRenameExemptions(ctx context.Context, ds1, ds2 *deltaSet) (path1, path2 int, err error) {
	if ds1.exceedsDeletes() {
		path1, err = b.trackedRenameExemptionsForPath(ctx, ds1, ds2, true)
		if err != nil {
			return 0, 0, err
		}
	}
	if ds2.exceedsDeletes() {
		path2, err = b.trackedRenameExemptionsForPath(ctx, ds2, ds1, false)
		if err != nil {
			return 0, 0, err
		}
	}
	return path1, path2, nil
}

func (b *bisyncRun) trackedRenameExemptionsForPath(ctx context.Context, changed, other *deltaSet, path1 bool) (int, error) {
	fsrc, fdst := b.fs1, b.fs2
	srcListing, dstListing := b.march.ls1, b.march.ls2
	if !path1 {
		fsrc, fdst = b.fs2, b.fs1
		srcListing, dstListing = b.march.ls2, b.march.ls1
	}

	hashType, modifyWindow, unsupported := trackrenames.CheckCapabilities(ctx, fsrc, fdst, b.trackRenamesStrategy)
	if len(unsupported) > 0 {
		for _, reason := range unsupported {
			fs.Infof(fdst, "Not excluding tracked renames from --max-delete because %s", reason)
		}
		return 0, nil
	}
	if b.trackRenamesStrategy.UsesHash() && (srcListing.hash != hashType || dstListing.hash != hashType) {
		fs.Infof(fdst, "Not excluding tracked renames from --max-delete because the listings do not contain the common %s hash", hashType)
		return 0, nil
	}

	sources := make([]trackrenames.Candidate, 0)
	for _, name := range changed.sort() {
		if changed.deltas[name] != deltaNew || b.aliases.Alias(name) != name {
			continue
		}
		if _, found := other.deltas[name]; found || dstListing.has(name) {
			continue
		}
		if candidate, ok := listingCandidate(srcListing, name); ok {
			sources = append(sources, candidate)
		}
	}
	if len(sources) == 0 {
		return 0, nil
	}

	destinations := make([]trackrenames.Candidate, 0)
	for _, name := range changed.sort() {
		if changed.deltas[name] != deltaDeleted || b.aliases.Alias(name) != name {
			continue
		}
		if _, found := other.deltas[name]; found || srcListing.has(name) || !dstListing.has(name) {
			continue
		}
		if candidate, ok := listingCandidate(dstListing, name); ok {
			destinations = append(destinations, candidate)
		}
	}

	return trackrenames.CountGuaranteedMatches(b.trackRenamesStrategy, modifyWindow, sources, destinations), nil
}

func listingCandidate(listing *fileList, remote string) (trackrenames.Candidate, bool) {
	info := listing.get(remote)
	if info == nil || info.flags == "d" {
		return trackrenames.Candidate{}, false
	}
	return trackrenames.Candidate{
		Remote:  remote,
		Size:    info.size,
		ModTime: info.time,
		Hash:    info.hash,
	}, true
}
