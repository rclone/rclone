package rs

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
)

type namespaceVotes struct {
	dirVotes    map[string]int
	objectVotes map[string]int
}

type namespaceDirClass int

const (
	dirClassHealthy namespaceDirClass = iota
	dirClassSkew
	dirClassExtra
)

func parentDirsOf(remote string) []string {
	if remote == "" || !strings.Contains(remote, "/") {
		return nil
	}
	parts := strings.Split(remote, "/")
	out := make([]string, 0, len(parts)-1)
	for i := 1; i < len(parts); i++ {
		out = append(out, strings.Join(parts[:i], "/"))
	}
	return out
}

func (f *Fs) collectObjectVotes(ctx context.Context) (map[string]int, error) {
	counts, _, err := f.collectObjectPresence(ctx)
	return counts, err
}

func (f *Fs) collectShardObjectSets(ctx context.Context) ([]map[string]struct{}, error) {
	n := len(f.backends)
	sets := make([]map[string]struct{}, n)
	type res struct {
		shard int
		set   map[string]struct{}
		err   error
	}
	ch := make(chan res, n)
	for shard := range f.backends {
		shard := shard
		go func() {
			set := map[string]struct{}{}
			err := operations.ListFn(ctx, f.backends[shard], func(o fs.Object) {
				set[o.Remote()] = struct{}{}
			})
			ch <- res{shard: shard, set: set, err: err}
		}()
	}
	for i := 0; i < n; i++ {
		r := <-ch
		if r.err != nil {
			if errors.Is(r.err, context.Canceled) || errors.Is(r.err, context.DeadlineExceeded) {
				return nil, r.err
			}
			fs.Logf(f, "rs: namespace object scan shard=%d failed: %v", r.shard, r.err)
			sets[r.shard] = map[string]struct{}{}
			continue
		}
		sets[r.shard] = r.set
	}
	return sets, nil
}

func listShardRootDirectories(ctx context.Context, b fs.Fs) (map[string]struct{}, error) {
	set := map[string]struct{}{}
	entries, err := b.List(ctx, "")
	if err != nil {
		if errors.Is(err, fs.ErrorDirNotFound) {
			return set, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if d, ok := e.(fs.Directory); ok {
			set[d.Remote()] = struct{}{}
		}
	}
	return set, nil
}

func shardHasObjectsUnder(objSet map[string]struct{}, dir string) bool {
	prefix := dir + "/"
	for remote := range objSet {
		if strings.HasPrefix(remote, prefix) {
			return true
		}
	}
	return false
}

func (f *Fs) shardHasDirectoryAt(ctx context.Context, shard int, objSet map[string]struct{}, dir string) (bool, error) {
	if shardHasObjectsUnder(objSet, dir) {
		return true, nil
	}
	has, err := shardHasDir(ctx, f.backends[shard], dir)
	if err != nil {
		return false, err
	}
	return has, nil
}

func (f *Fs) collectNamespaceVotes(ctx context.Context) (namespaceVotes, error) {
	objSets, err := f.collectShardObjectSets(ctx)
	if err != nil {
		return namespaceVotes{}, err
	}
	objectVotes := make(map[string]int)
	for _, set := range objSets {
		for remote := range set {
			objectVotes[remote]++
		}
	}

	union := map[string]struct{}{}
	for shard := range f.backends {
		rootDirs, err := listShardRootDirectories(ctx, f.backends[shard])
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return namespaceVotes{}, err
			}
			fs.Logf(f, "rs: namespace root dir scan shard=%d failed: %v", shard, err)
			continue
		}
		for dir := range rootDirs {
			union[dir] = struct{}{}
		}
	}
	for remote := range objectVotes {
		for _, parent := range parentDirsOf(remote) {
			union[parent] = struct{}{}
		}
	}

	dirVotes := make(map[string]int, len(union))
	for dir := range union {
		votes := 0
		for shard := range f.backends {
			has, err := f.shardHasDirectoryAt(ctx, shard, objSets[shard], dir)
			if err != nil {
				fs.Logf(f, "rs: namespace dir probe shard=%d dir=%q failed: %v", shard, dir, err)
				continue
			}
			if has {
				votes++
			}
		}
		dirVotes[dir] = votes
	}
	return namespaceVotes{dirVotes: dirVotes, objectVotes: objectVotes}, nil
}

func classifyDirectory(votes, k, total int) namespaceDirClass {
	if votes == 0 {
		return dirClassHealthy
	}
	if votes >= k && votes < total {
		return dirClassSkew
	}
	if votes >= k {
		return dirClassHealthy
	}
	return dirClassExtra
}

func dirMatchesScope(dir, scope string) bool {
	if scope == "" {
		return true
	}
	if dir == scope || strings.HasPrefix(dir, scope+"/") {
		return true
	}
	for _, parent := range parentDirsOf(scope) {
		if dir == parent {
			return true
		}
	}
	return false
}

func remoteMatchesScope(remote, scope string) bool {
	if scope == "" {
		return true
	}
	if remote == scope {
		return true
	}
	dir := path.Dir(scope)
	if dir != "." && dir != "" {
		return strings.HasPrefix(remote, dir+"/")
	}
	return false
}

type namespaceHealStats struct {
	orphansPurged int
	mkdirs        int
	rmdirs        int
	skipped       int
	failed        int
}

func (f *Fs) degradedListDirectories(ctx context.Context) (string, error) {
	votes, err := f.collectNamespaceVotes(ctx)
	if err != nil {
		return "", err
	}
	k := f.readQuorum()
	total := len(f.backends)
	var sb strings.Builder
	sb.WriteString("RS Degraded Directories\n========================================\n")
	sb.WriteString(fmt.Sprintf("Read quorum (k): %d\nWrite quorum: %d of %d\n", k, f.writeQuorum(), total))

	dirs := make([]string, 0, len(votes.dirVotes))
	for dir := range votes.dirVotes {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	var skew, extra int
	for _, dir := range dirs {
		v := votes.dirVotes[dir]
		switch classifyDirectory(v, k, total) {
		case dirClassSkew:
			skew++
			sb.WriteString(fmt.Sprintf("SKEW %s: dirVotes=%d required=%d total_shards=%d\n", dir, v, k, total))
		case dirClassExtra:
			extra++
			sb.WriteString(fmt.Sprintf("EXTRA %s: dirVotes=%d required=%d\n", dir, v, k))
		}
	}
	if skew == 0 && extra == 0 {
		sb.WriteString("No degraded directory skew found.\n")
	} else {
		sb.WriteString(fmt.Sprintf("\nSummary: skew=%d extra=%d\n", skew, extra))
	}
	return sb.String(), nil
}

func (f *Fs) healNamespace(ctx context.Context, scope string, dryRun bool) (namespaceHealStats, string, error) {
	votes, err := f.collectNamespaceVotes(ctx)
	if err != nil {
		return namespaceHealStats{}, "", err
	}
	objSets, err := f.collectShardObjectSets(ctx)
	if err != nil {
		return namespaceHealStats{}, "", err
	}

	k := f.readQuorum()
	stats := namespaceHealStats{}
	var details strings.Builder

	cmStats, cmDetails, err := f.healCopyMoveArtifacts(ctx, scope, objSets, dryRun)
	if err != nil {
		return namespaceHealStats{}, "", err
	}
	mergeNamespaceHealStats(&stats, cmStats)
	if cmDetails != "" {
		details.WriteString(cmDetails)
		details.WriteString("\n")
	}

	orphans := make([]string, 0)
	for remote, n := range votes.objectVotes {
		if !remoteMatchesScope(remote, scope) {
			continue
		}
		if n < k {
			orphans = append(orphans, remote)
		}
	}
	sort.Strings(orphans)
	for _, remote := range orphans {
		for shard := range f.backends {
			if _, ok := objSets[shard][remote]; !ok {
				continue
			}
			if dryRun {
				stats.orphansPurged++
				details.WriteString(fmt.Sprintf("WOULD_PURGE_ORPHAN shard=%d %s\n", shard, remote))
				continue
			}
			obj, err := f.backends[shard].NewObject(ctx, remote)
			if err != nil {
				stats.failed++
				details.WriteString(fmt.Sprintf("FAIL_PURGE_ORPHAN shard=%d %s: %v\n", shard, remote, err))
				continue
			}
			if err := obj.Remove(ctx); err != nil {
				stats.failed++
				details.WriteString(fmt.Sprintf("FAIL_PURGE_ORPHAN shard=%d %s: %v\n", shard, remote, err))
				continue
			}
			delete(objSets[shard], remote)
			stats.orphansPurged++
			details.WriteString(fmt.Sprintf("PURGED_ORPHAN shard=%d %s\n", shard, remote))
		}
	}

	dirs := make([]string, 0, len(votes.dirVotes))
	for dir := range votes.dirVotes {
		if dirMatchesScope(dir, scope) {
			dirs = append(dirs, dir)
		}
	}
	sort.Strings(dirs)

	for _, dir := range dirs {
		v := votes.dirVotes[dir]
		if v >= k {
			continue
		}
		for shard := range f.backends {
			has, err := f.shardHasDirectoryAt(ctx, shard, objSets[shard], dir)
			if err != nil || !has {
				continue
			}
			prefix := dir + "/"
			for remote := range objSets[shard] {
				if remote == dir || strings.HasPrefix(remote, prefix) {
					if dryRun {
						stats.orphansPurged++
						details.WriteString(fmt.Sprintf("WOULD_PURGE_UNDER_DIR shard=%d %s\n", shard, remote))
						continue
					}
					obj, err := f.backends[shard].NewObject(ctx, remote)
					if err != nil {
						continue
					}
					if err := obj.Remove(ctx); err == nil {
						delete(objSets[shard], remote)
						stats.orphansPurged++
						details.WriteString(fmt.Sprintf("PURGED_UNDER_DIR shard=%d %s\n", shard, remote))
					}
				}
			}
			hasMarker, _ := shardHasDir(ctx, f.backends[shard], dir)
			if !hasMarker {
				continue
			}
			if dryRun {
				stats.rmdirs++
				details.WriteString(fmt.Sprintf("WOULD_RMDIR shard=%d %s\n", shard, dir))
				continue
			}
			if err := f.backends[shard].Rmdir(ctx, dir); err != nil {
				stats.failed++
				details.WriteString(fmt.Sprintf("FAIL_RMDIR shard=%d %s: %v\n", shard, dir, err))
				continue
			}
			stats.rmdirs++
			details.WriteString(fmt.Sprintf("RMDIR shard=%d %s\n", shard, dir))
		}
	}

	for _, dir := range dirs {
		v := votes.dirVotes[dir]
		if v < k {
			continue
		}
		for shard := range f.backends {
			has, err := f.shardHasDirectoryAt(ctx, shard, objSets[shard], dir)
			if err != nil || has {
				continue
			}
			if dryRun {
				stats.mkdirs++
				details.WriteString(fmt.Sprintf("WOULD_MKDIR shard=%d %s\n", shard, dir))
				continue
			}
			if err := f.backends[shard].Mkdir(ctx, dir); err != nil {
				stats.failed++
				details.WriteString(fmt.Sprintf("FAIL_MKDIR shard=%d %s: %v\n", shard, dir, err))
				continue
			}
			stats.mkdirs++
			details.WriteString(fmt.Sprintf("MKDIR shard=%d %s\n", shard, dir))
		}
	}

	return stats, details.String(), nil
}
