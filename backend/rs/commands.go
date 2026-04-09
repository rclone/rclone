package rs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/klauspost/reedsolomon"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
)

// Command runs backend-specific commands (status, heal).
func (f *Fs) Command(ctx context.Context, name string, arg []string, opt map[string]string) (out any, err error) {
	switch name {
	case "status":
		return f.statusCommand(ctx, arg, opt)
	case "heal":
		return f.healCommand(ctx, arg, opt)
	case "degraded":
		return f.degradedCommand(ctx, arg, opt)
	default:
		return nil, fs.ErrorCommandNotFound
	}
}

func (f *Fs) statusCommand(ctx context.Context, arg []string, opt map[string]string) (any, error) {
	return f.statusText(ctx), nil
}

func (f *Fs) healCommand(ctx context.Context, arg []string, opt map[string]string) (any, error) {
	optMap := opt
	if optMap == nil {
		optMap = map[string]string{}
	}
	dryRun := optMap["dry-run"] == "true"

	// Single-object mode: when a logical object path is provided as the first
	// argument, repair only that object (no full-namespace scan).
	if len(arg) > 0 && strings.TrimSpace(arg[0]) != "" {
		remote := strings.TrimSpace(arg[0])
		missing, err := f.rebuildMissingShardsForObject(ctx, remote, dryRun)
		if err != nil {
			return nil, err
		}
		if dryRun {
			return fmt.Sprintf("RS heal dry-run for %s: would restore %d shard(s)", remote, missing), nil
		}
		return fmt.Sprintf("RS heal completed for %s: restored %d shard(s)", remote, missing), nil
	}

	remotes, err := f.listAllObjectRemotes(ctx)
	if err != nil {
		return nil, err
	}
	total := 0
	healed := 0
	wouldHeal := 0
	skipped := 0
	failed := 0
	var sb strings.Builder
	failedRemotes := make([]string, 0)
	for _, remote := range remotes {
		total++
		missing, err := f.rebuildMissingShardsForObject(ctx, remote, dryRun)
		if err != nil {
			failed++
			failedRemotes = append(failedRemotes, remote)
			sb.WriteString(fmt.Sprintf("FAIL %s: %v\n", remote, err))
			continue
		}
		if missing == 0 {
			skipped++
			continue
		}
		if dryRun {
			wouldHeal++
			sb.WriteString(fmt.Sprintf("WOULD_HEAL %s: would restore %d shard(s)\n", remote, missing))
		} else {
			healed++
			sb.WriteString(fmt.Sprintf("HEALED %s: restored %d shard(s)\n", remote, missing))
		}
	}
	report := "RS Heal Summary"
	if dryRun {
		report += " (dry-run)"
	}
	report += "\n========================================\n"
	if dryRun {
		report += fmt.Sprintf("Scanned: %d\nWould heal: %d\nHealthy/Skipped: %d\nFailed: %d\n", total, wouldHeal, skipped, failed)
	} else {
		report += fmt.Sprintf("Scanned: %d\nHealed: %d\nHealthy/Skipped: %d\nFailed: %d\n", total, healed, skipped, failed)
	}
	if sb.Len() > 0 {
		report += "\nDetails:\n" + sb.String()
	}
	if len(failedRemotes) > 0 {
		report += "\nFailed remotes:\n"
		for _, r := range failedRemotes {
			report += fmt.Sprintf("  - %s\n", r)
		}
	}
	return report, nil
}

type degradedStats struct {
	totalObjects    int
	healthyObjects  int
	degradedObjects int
}

func (f *Fs) degradedCommand(ctx context.Context, arg []string, opt map[string]string) (any, error) {
	sub := "summary"
	if len(arg) > 0 && strings.TrimSpace(arg[0]) != "" {
		sub = strings.ToLower(strings.TrimSpace(arg[0]))
	}
	switch sub {
	case "summary":
		return f.degradedSummary(ctx)
	case "ls":
		return f.degradedListObjects(ctx)
	case "lsd":
		return "RS degraded lsd: directory skew reporting is not implemented yet", nil
	default:
		return nil, fmt.Errorf("rs: unknown degraded subcommand %q (supported: summary, ls, lsd)", sub)
	}
}

func (f *Fs) degradedSummary(ctx context.Context) (any, error) {
	counts, stats, err := f.collectObjectPresence(ctx)
	if err != nil {
		return nil, err
	}
	_ = counts
	return fmt.Sprintf("RS Degraded Summary\n========================================\nTotal objects: %d\nHealthy: %d\nDegraded: %d\nQuorum: %d of %d\n", stats.totalObjects, stats.healthyObjects, stats.degradedObjects, f.writeQuorum(), len(f.backends)), nil
}

func (f *Fs) degradedListObjects(ctx context.Context) (any, error) {
	counts, stats, err := f.collectObjectPresence(ctx)
	if err != nil {
		return nil, err
	}
	var sb strings.Builder
	sb.WriteString("RS Degraded Objects\n========================================\n")
	sb.WriteString(fmt.Sprintf("Quorum: %d of %d\n", f.writeQuorum(), len(f.backends)))
	if stats.degradedObjects == 0 {
		sb.WriteString("No degraded objects found.\n")
		return sb.String(), nil
	}
	remotes := make([]string, 0, len(counts))
	for remote := range counts {
		if counts[remote] < f.writeQuorum() {
			remotes = append(remotes, remote)
		}
	}
	sort.Strings(remotes)
	for _, remote := range remotes {
		sb.WriteString(fmt.Sprintf("DEGRADED %s: present_shards=%d required=%d\n", remote, counts[remote], f.writeQuorum()))
	}
	return sb.String(), nil
}

func (f *Fs) collectObjectPresence(ctx context.Context) (map[string]int, degradedStats, error) {
	type shardResult struct {
		shard int
		objs  []string
		err   error
	}
	ch := make(chan shardResult, len(f.backends))
	for shard := range f.backends {
		shard := shard
		go func() {
			var objs []string
			err := operations.ListFn(ctx, f.backends[shard], func(o fs.Object) {
				objs = append(objs, o.Remote())
			})
			ch <- shardResult{shard: shard, objs: objs, err: err}
		}()
	}
	counts := map[string]int{}
	for i := 0; i < len(f.backends); i++ {
		res := <-ch
		if res.err != nil {
			if errors.Is(res.err, context.Canceled) || errors.Is(res.err, context.DeadlineExceeded) {
				return nil, degradedStats{}, res.err
			}
			fs.Logf(f, "rs: degraded scan shard=%d failed: %v", res.shard, res.err)
			continue
		}
		for _, remote := range res.objs {
			counts[remote]++
		}
	}
	stats := degradedStats{totalObjects: len(counts)}
	for _, n := range counts {
		if n >= f.writeQuorum() {
			stats.healthyObjects++
		} else {
			stats.degradedObjects++
		}
	}
	return counts, stats, nil
}

func (f *Fs) listAllObjectRemotes(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	uniq := map[string]struct{}{}
	for _, b := range f.backends {
		err := operations.ListFn(ctx, b, func(o fs.Object) {
			uniq[o.Remote()] = struct{}{}
		})
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			continue
		}
	}
	out := make([]string, 0, len(uniq))
	for r := range uniq {
		out = append(out, r)
	}
	sort.Strings(out)
	return out, nil
}

// rebuildMissingShardsForObject restores missing shards for one logical object.
// If dryRun is true, computes what would be restored and returns that count without uploading.
func (f *Fs) rebuildMissingShardsForObject(ctx context.Context, remote string, dryRun bool) (int, error) {
	k := f.opt.DataShards
	m := f.opt.ParityShards
	total := k + m

	shards := make([][]byte, total)
	missingIdx := make([]int, 0, total)
	var metaFooter *Footer
	for i, b := range f.backends {
		obj, err := b.NewObject(ctx, remote)
		if err != nil {
			missingIdx = append(missingIdx, i)
			continue
		}
		r, err := obj.Open(ctx)
		if err != nil {
			missingIdx = append(missingIdx, i)
			continue
		}
		all, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			missingIdx = append(missingIdx, i)
			continue
		}
		payload, ft, err := ExtractParticlePayload(all, i)
		if err != nil {
			missingIdx = append(missingIdx, i)
			continue
		}
		if metaFooter == nil {
			metaFooter = ft
		}
		shards[i] = payload
	}
	if len(missingIdx) == 0 {
		return 0, nil
	}
	if metaFooter == nil {
		return 0, fmt.Errorf("rs: no valid shards found for %q", remote)
	}
	reconstructed, err := reconstructMissingShards(shards, k, m, metaFooter.StripeSize, metaFooter.NumStripes)
	if err != nil {
		return 0, fmt.Errorf("rs: cannot reconstruct %q: %w", remote, err)
	}

	if dryRun {
		return len(missingIdx), nil
	}

	restored := 0
	mtime := time.Unix(metaFooter.Mtime, 0)
	for _, idx := range missingIdx {
		payload := reconstructed[idx]
		ft := NewRSFooter(metaFooter.ContentLength, metaFooter.MD5[:], metaFooter.SHA256[:], mtime, k, m, idx, metaFooter.StripeSize, metaFooter.NumStripes, crc32cChecksum(payload))
		fb, err := ft.MarshalBinary()
		if err != nil {
			return restored, err
		}
		blob := make([]byte, 0, len(payload)+len(fb))
		blob = append(blob, payload...)
		blob = append(blob, fb...)
		info := object.NewStaticObjectInfo(remote, mtime, int64(len(blob)), true, nil, nil)
		if _, err := f.backends[idx].Put(ctx, bytes.NewReader(blob), info); err != nil {
			return restored, fmt.Errorf("rs: upload rebuilt shard %d for %q failed: %w", idx, remote, err)
		}
		restored++
	}
	return restored, nil
}

func reconstructMissingShards(shards [][]byte, dataShards, parityShards int, stripeSize, numStripes uint32) ([][]byte, error) {
	k, m := dataShards, parityShards
	S := int(stripeSize)
	N := int(numStripes)
	if N == 0 {
		out := make([][]byte, k+m)
		return out, nil
	}
	out := make([][]byte, k+m)
	for i := range out {
		out[i] = make([]byte, N*S)
	}
	for t := 0; t < N; t++ {
		row := make([][]byte, k+m)
		for i := 0; i < k+m; i++ {
			if shards[i] != nil {
				row[i] = shards[i][t*S : (t+1)*S]
			}
		}
		cp := make([][]byte, k+m)
		copy(cp, row)
		fixed, err := reconstructInto(cp, k, m)
		if err != nil {
			return nil, err
		}
		for i := 0; i < k+m; i++ {
			copy(out[i][t*S:(t+1)*S], fixed[i])
		}
	}
	return out, nil
}

func reconstructInto(shards [][]byte, dataShards, parityShards int) ([][]byte, error) {
	available := 0
	for _, s := range shards {
		if s != nil {
			available++
		}
	}
	if available < dataShards {
		return nil, fmt.Errorf("insufficient shards: have %d need %d", available, dataShards)
	}
	enc, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		return nil, err
	}
	if err := enc.Reconstruct(shards); err != nil {
		return nil, err
	}
	return shards, nil
}
