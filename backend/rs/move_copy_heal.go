package rs

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// healCopyMoveArtifacts purges *.rs-tmp-* staging files and restores or purges *.rs-bak-* look-asides
// left by interrupted atomic copy/move operations.
func (f *Fs) healCopyMoveArtifacts(ctx context.Context, scope string, objSets []map[string]struct{}, dryRun bool) (namespaceHealStats, string, error) {
	stats := namespaceHealStats{}
	var details strings.Builder

	artifacts := map[string]struct{}{}
	for _, set := range objSets {
		for remote := range set {
			if _, _, _, ok := parseCopyMoveArtifact(remote); ok {
				artifacts[remote] = struct{}{}
			}
		}
	}
	names := make([]string, 0, len(artifacts))
	for remote := range artifacts {
		names = append(names, remote)
	}
	sort.Strings(names)

	k := f.opt.DataShards
	m := f.opt.ParityShards
	bakByBase := map[string][]string{}

	for _, remote := range names {
		base, kind, _, ok := parseCopyMoveArtifact(remote)
		if !ok || !remoteMatchesScope(base, scope) {
			continue
		}
		switch kind {
		case copyMoveArtifactTmp:
			for shard, set := range objSets {
				if _, ok := set[remote]; !ok {
					continue
				}
				if dryRun {
					stats.orphansPurged++
					details.WriteString(fmt.Sprintf("WOULD_PURGE_COPYMOVE_TMP shard=%d %s\n", shard, remote))
					continue
				}
				if err := removeShardObject(ctx, f.backends[shard], remote); err != nil {
					stats.failed++
					details.WriteString(fmt.Sprintf("FAIL_PURGE_COPYMOVE_TMP shard=%d %s: %v\n", shard, remote, err))
					continue
				}
				delete(objSets[shard], remote)
				stats.orphansPurged++
				details.WriteString(fmt.Sprintf("PURGED_COPYMOVE_TMP shard=%d %s\n", shard, remote))
			}
		case copyMoveArtifactBak:
			bakByBase[base] = append(bakByBase[base], remote)
		}
	}

	bases := make([]string, 0, len(bakByBase))
	for base := range bakByBase {
		bases = append(bases, base)
	}
	sort.Strings(bases)

	for _, base := range bases {
		bakRemotes := bakByBase[base]
		sort.Strings(bakRemotes)
		logicalOK := false
		if _, err := probeAndSelectWriteIDGroup(ctx, f, base, nil, k, m); err == nil {
			logicalOK = true
		}
		for _, bakRemote := range bakRemotes {
			for shard, set := range objSets {
				if _, ok := set[bakRemote]; !ok {
					continue
				}
				if logicalOK {
					if dryRun {
						stats.orphansPurged++
						details.WriteString(fmt.Sprintf("WOULD_PURGE_COPYMOVE_BAK shard=%d %s\n", shard, bakRemote))
						continue
					}
					if err := removeShardObject(ctx, f.backends[shard], bakRemote); err != nil {
						stats.failed++
						details.WriteString(fmt.Sprintf("FAIL_PURGE_COPYMOVE_BAK shard=%d %s: %v\n", shard, bakRemote, err))
						continue
					}
					delete(objSets[shard], bakRemote)
					stats.orphansPurged++
					details.WriteString(fmt.Sprintf("PURGED_COPYMOVE_BAK shard=%d %s\n", shard, bakRemote))
					continue
				}
				if dryRun {
					stats.orphansPurged++
					details.WriteString(fmt.Sprintf("WOULD_RESTORE_COPYMOVE_BAK shard=%d %s -> %s\n", shard, bakRemote, base))
					continue
				}
				b := f.backends[shard]
				if err := shardRestoreDst(ctx, b, base, bakRemote, ""); err != nil {
					stats.failed++
					details.WriteString(fmt.Sprintf("FAIL_RESTORE_COPYMOVE_BAK shard=%d %s: %v\n", shard, bakRemote, err))
					continue
				}
				delete(objSets[shard], bakRemote)
				stats.orphansPurged++
				details.WriteString(fmt.Sprintf("RESTORED_COPYMOVE_BAK shard=%d %s -> %s\n", shard, bakRemote, base))
			}
		}
	}

	return stats, strings.TrimRight(details.String(), "\n"), nil
}

func mergeNamespaceHealStats(dst *namespaceHealStats, src namespaceHealStats) {
	dst.orphansPurged += src.orphansPurged
	dst.mkdirs += src.mkdirs
	dst.rmdirs += src.rmdirs
	dst.skipped += src.skipped
	dst.failed += src.failed
}

func appendHealDetails(details, extra string) string {
	extra = strings.TrimSpace(extra)
	if extra == "" {
		return details
	}
	if details == "" {
		return extra
	}
	return details + "\n" + extra
}
