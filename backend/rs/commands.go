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
	reconstructed, err := reconstructMissingShards(shards, k, m)
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
		ft := NewRSFooter(metaFooter.ContentLength, metaFooter.MD5[:], metaFooter.SHA256[:], mtime, k, m, idx, metaFooter.StripeSize, crc32cChecksum(payload))
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

func reconstructMissingShards(shards [][]byte, dataShards, parityShards int) ([][]byte, error) {
	cp := make([][]byte, len(shards))
	copy(cp, shards)
	returned, err := reconstructInto(cp, dataShards, parityShards)
	if err != nil {
		return nil, err
	}
	return returned, nil
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
