package rs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/reedsolomon"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	"golang.org/x/sync/errgroup"
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
		var sb strings.Builder
		missing, err := f.rebuildMissingShardsForObject(ctx, remote, dryRun)
		if err != nil {
			return nil, err
		}
		if dryRun {
			sb.WriteString(fmt.Sprintf("RS heal dry-run for %s: would restore %d shard(s)\n", remote, missing))
		} else {
			sb.WriteString(fmt.Sprintf("RS heal completed for %s: restored %d shard(s)\n", remote, missing))
		}
		nsStats, nsDetails, err := f.healNamespace(ctx, remote, dryRun)
		if err != nil {
			return nil, err
		}
		sb.WriteString(formatNamespaceHealSummary(nsStats, dryRun))
		if nsDetails != "" {
			sb.WriteString("\nNamespace details:\n")
			sb.WriteString(nsDetails)
		}
		return strings.TrimRight(sb.String(), "\n"), nil
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
	nsStats, nsDetails, err := f.healNamespace(ctx, "", dryRun)
	if err != nil {
		return nil, err
	}
	report += "\nNamespace convergence:\n"
	report += formatNamespaceHealSummary(nsStats, dryRun)
	if nsDetails != "" {
		report += "\nNamespace details:\n" + nsDetails
	}
	return report, nil
}

func formatNamespaceHealSummary(stats namespaceHealStats, dryRun bool) string {
	if dryRun {
		return fmt.Sprintf("Orphans would purge: %d\nDirs would mkdir: %d\nDirs would rmdir: %d\nFailed: %d\n",
			stats.orphansPurged, stats.mkdirs, stats.rmdirs, stats.failed)
	}
	return fmt.Sprintf("Orphans purged: %d\nDirs mkdir: %d\nDirs rmdir: %d\nFailed: %d\n",
		stats.orphansPurged, stats.mkdirs, stats.rmdirs, stats.failed)
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
		return f.degradedListDirectories(ctx)
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
	ns, err := f.collectNamespaceVotes(ctx)
	if err != nil {
		return nil, err
	}
	k := f.readQuorum()
	total := len(f.backends)
	var dirSkew, dirExtra int
	for _, v := range ns.dirVotes {
		switch classifyDirectory(v, k, total) {
		case dirClassSkew:
			dirSkew++
		case dirClassExtra:
			dirExtra++
		}
	}
	return fmt.Sprintf("RS Degraded Summary\n========================================\nTotal objects: %d\nHealthy objects: %d\nDegraded objects: %d\nDirectory skew: %d\nExtra directories: %d\nRead quorum (k): %d\nWrite quorum: %d of %d\n",
		stats.totalObjects, stats.healthyObjects, stats.degradedObjects, dirSkew, dirExtra, k, f.writeQuorum(), total), nil
}

func (f *Fs) degradedListObjects(ctx context.Context) (any, error) {
	counts, stats, err := f.collectObjectPresence(ctx)
	if err != nil {
		return nil, err
	}
	var sb strings.Builder
	sb.WriteString("RS Degraded Objects\n========================================\n")
	sb.WriteString(fmt.Sprintf("Read quorum (k): %d\nWrite quorum: %d of %d\n", f.readQuorum(), f.writeQuorum(), len(f.backends)))
	if stats.degradedObjects == 0 {
		sb.WriteString("No degraded objects found.\n")
		return sb.String(), nil
	}
	remotes := make([]string, 0, len(counts))
	for remote := range counts {
		if counts[remote] < f.readQuorum() {
			remotes = append(remotes, remote)
		}
	}
	sort.Strings(remotes)
	for _, remote := range remotes {
		sb.WriteString(fmt.Sprintf("DEGRADED %s: present_shards=%d required=%d\n", remote, counts[remote], f.readQuorum()))
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
		if n >= f.readQuorum() {
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
	n := len(f.backends)
	type listFnRes struct {
		objs []string
		err  error
	}
	results := make([]listFnRes, n)
	g, gctx := errgroup.WithContext(ctx)
	for i := range f.backends {
		i := i
		g.Go(func() error {
			var objs []string
			err := operations.ListFn(gctx, f.backends[i], func(o fs.Object) {
				objs = append(objs, o.Remote())
			})
			results[i].objs = objs
			results[i].err = err
			if err != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
				return err
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	uniq := map[string]struct{}{}
	for i := 0; i < n; i++ {
		if results[i].err != nil {
			if errors.Is(results[i].err, context.Canceled) || errors.Is(results[i].err, context.DeadlineExceeded) {
				return nil, results[i].err
			}
			continue
		}
		for _, r := range results[i].objs {
			uniq[r] = struct{}{}
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
//
// For stripe-encoded particles (StripeSize and NumStripes both non-zero), discovery uses
// footer-only reads and reconstruction is stripe-wise with range reads, so peak memory is
// O(|missing|·N·S + (k+m)·S) instead of full ReadAll of every present shard plus full (k+m) buffers.
// Full-particle PayloadCRC32C is not verified on reads in that path (same tradeoff as streaming Open).
// Empty logical objects (NumStripes==0) use the legacy buffered path with ExtractParticlePayload.
func (f *Fs) rebuildMissingShardsForObject(ctx context.Context, remote string, dryRun bool) (int, error) {
	k := f.opt.DataShards
	m := f.opt.ParityShards
	total := k + m

	// Quirky Fs with no backends (e.g. command-only tests): match legacy no-op heal behavior.
	if len(f.backends) == 0 {
		return 0, nil
	}

	metaFooter, missing, err := f.discoverHealShardPresence(ctx, remote, total)
	if err != nil {
		return 0, err
	}
	missingIdx := indicesWhereTrue(missing)
	if len(missingIdx) == 0 {
		return 0, nil
	}

	if metaFooter.NumStripes == 0 || metaFooter.StripeSize == 0 {
		return f.rebuildMissingShardsLegacyBuffered(ctx, remote, dryRun, k, m, total, metaFooter, missing, missingIdx)
	}
	return f.rebuildMissingShardsStripeWise(ctx, remote, dryRun, k, m, total, metaFooter, missing, missingIdx)
}

// healShardGather holds per-shard probe data for discoverHealShardPresence (parallel gather, index-ordered reduce).
type healShardGather struct {
	skip   bool // unusable shard for reduce (same cases as sequential continue before metaFooter logic)
	sz     int64
	footer *Footer
}

// discoverHealShardPresence returns a reference footer from the first readable shard and a
// missing[] slice (true = shard cannot be used for reconstruction). Footer-only reads do not
// verify PayloadCRC32C over the full payload.
func (f *Fs) discoverHealShardPresence(ctx context.Context, remote string, total int) (*Footer, []bool, error) {
	gather := make([]healShardGather, total)
	g, gctx := errgroup.WithContext(ctx)
	for i := 0; i < total; i++ {
		i := i
		g.Go(func() error {
			obj, err := f.backends[i].NewObject(gctx, remote)
			if err != nil {
				gather[i].skip = true
				return nil
			}
			sz := obj.Size()
			if sz < FooterSize {
				gather[i].skip = true
				return nil
			}
			ft, err := readFooterFromParticle(gctx, obj)
			if err != nil {
				gather[i].skip = true
				return nil
			}
			gather[i].sz = sz
			gather[i].footer = ft
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, nil, err
	}

	missing := make([]bool, total)
	var metaFooter *Footer
	for i := 0; i < total; i++ {
		g := gather[i]
		if g.skip {
			missing[i] = true
			continue
		}
		sz := g.sz
		ft := g.footer
		if metaFooter == nil {
			metaFooter = ft
		} else if !footerCompatibleForStripeRead(metaFooter, ft, i) {
			missing[i] = true
			continue
		}
		if ft.NumStripes == 0 || ft.StripeSize == 0 {
			if sz != int64(FooterSize) {
				missing[i] = true
			}
			continue
		}
		if sz != ExpectedParticleSize(metaFooter.ContentLength, i, f.opt.DataShards, f.opt.ParityShards, int(ft.StripeSize), true) {
			missing[i] = true
		}
	}
	if metaFooter == nil {
		return nil, nil, fmt.Errorf("rs: no valid shards found for %q", remote)
	}
	return metaFooter, missing, nil
}

func indicesWhereTrue(flags []bool) []int {
	out := make([]int, 0, len(flags))
	for i, v := range flags {
		if v {
			out = append(out, i)
		}
	}
	return out
}

// rebuildMissingShardsLegacyBuffered is the pre-stripe heal path: full ReadAll per present shard,
// ExtractParticlePayload (CRC), and reconstructMissingShards. Used for NumStripes==0 (empty logical).
func (f *Fs) rebuildMissingShardsLegacyBuffered(ctx context.Context, remote string, dryRun bool, k, m, total int, metaFooter *Footer, missing []bool, missingIdx []int) (int, error) {
	shards := make([][]byte, total)
	g, gctx := errgroup.WithContext(ctx)
	for i := 0; i < total; i++ {
		if missing[i] {
			continue
		}
		i := i
		g.Go(func() error {
			obj, err := f.backends[i].NewObject(gctx, remote)
			if err != nil {
				return fmt.Errorf("rs: heal legacy shard %d: %w", i, err)
			}
			r, err := obj.Open(gctx)
			if err != nil {
				return fmt.Errorf("rs: heal legacy shard %d: %w", i, err)
			}
			all, err := io.ReadAll(r)
			_ = r.Close()
			if err != nil {
				return fmt.Errorf("rs: heal legacy shard %d: %w", i, err)
			}
			payload, _, err := ExtractParticlePayload(all, i)
			if err != nil {
				return fmt.Errorf("rs: heal legacy shard %d: %w", i, err)
			}
			shards[i] = payload
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return 0, err
	}
	reconstructed, err := reconstructMissingShards(shards, k, m, metaFooter.ContentLength, metaFooter.StripeSize, metaFooter.NumStripes)
	if err != nil {
		return 0, fmt.Errorf("rs: cannot reconstruct %q: %w", remote, err)
	}
	if dryRun {
		return len(missingIdx), nil
	}
	return f.putHealedMissingShards(ctx, remote, k, m, metaFooter, missing, missingIdx, reconstructed)
}

// rebuildMissingShardsStripeWise reconstructs only missing shard payloads using per-stripe range reads.
func (f *Fs) rebuildMissingShardsStripeWise(ctx context.Context, remote string, dryRun bool, k, m, total int, metaFooter *Footer, missing []bool, missingIdx []int) (int, error) {
	N := int(metaFooter.NumStripes)
	S := int64(metaFooter.StripeSize)
	intS := int(S)
	if N <= 0 || intS <= 0 {
		return 0, fmt.Errorf("rs: invalid stripe metadata for %q", remote)
	}
	presentCount := 0
	for i := 0; i < total; i++ {
		if !missing[i] {
			presentCount++
		}
	}
	if presentCount < k {
		return 0, fmt.Errorf("rs: not enough shards to reconstruct %q: have %d need %d", remote, presentCount, k)
	}

	if dryRun {
		if err := f.healStripeDryRunProbe(ctx, remote, k, m, total, metaFooter.ContentLength, S, intS, missing); err != nil {
			return 0, fmt.Errorf("rs: cannot reconstruct %q: %w", remote, err)
		}
		return len(missingIdx), nil
	}

	cl := metaFooter.ContentLength
	out := make([][]byte, total)
	for _, idx := range missingIdx {
		out[idx] = make([]byte, ShardPayloadLen(cl, idx, k, m, intS))
	}
	rowBuf := make([]byte, total*intS)
	row := make([][]byte, total)

	for t := 0; t < N; t++ {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		logLen := StripeLogicalLen(k, S, cl, t)
		for i := 0; i < total; i++ {
			row[i] = nil
		}
		for i := 0; i < total; i++ {
			if missing[i] {
				continue
			}
			row[i] = rowBuf[i*intS : (i+1)*intS]
		}
		if err := readStripeFragmentsParallel(ctx, f, remote, t, k, cl, S, logLen, row, false); err != nil {
			return 0, fmt.Errorf("rs: heal stripe %d: %w", t, err)
		}
		if _, err := reconstructInto(row, k, m); err != nil {
			return 0, fmt.Errorf("rs: cannot reconstruct %q stripe %d: %w", remote, t, err)
		}
		for _, idx := range missingIdx {
			frag := row[idx]
			if len(frag) != intS {
				return 0, fmt.Errorf("rs: heal shard %d stripe %d: unexpected fragment length %d", idx, t, len(frag))
			}
			writeShardStripeFragment(out[idx], idx, k, intS, t, cl, logLen, frag)
		}
	}

	return f.putHealedMissingShards(ctx, remote, k, m, metaFooter, missing, missingIdx, out)
}

// healStripeDryRunProbe runs a single-stripe reconstruct to verify heal would succeed without allocating full outputs.
func (f *Fs) healStripeDryRunProbe(ctx context.Context, remote string, k, m, total int, contentLength int64, S int64, intS int, missing []bool) error {
	rowBuf := make([]byte, total*intS)
	row := make([][]byte, total)
	for i := 0; i < total; i++ {
		row[i] = nil
	}
	for i := 0; i < total; i++ {
		if missing[i] {
			continue
		}
		row[i] = rowBuf[i*intS : (i+1)*intS]
	}
	logLen := StripeLogicalLen(k, S, contentLength, 0)
	if err := readStripeFragmentsParallel(ctx, f, remote, 0, k, contentLength, S, logLen, row, false); err != nil {
		return err
	}
	_, err := reconstructInto(row, k, m)
	return err
}

type healPutJob struct {
	idx  int
	blob []byte
	info fs.ObjectInfo
}

func (f *Fs) putHealedMissingShards(ctx context.Context, remote string, k, m int, metaFooter *Footer, missing []bool, missingIdx []int, reconstructed [][]byte) (int, error) {
	mtime, err := f.resolveHealReferenceModTime(ctx, remote, missing)
	if err != nil {
		mtime = time.Unix(0, metaFooter.Mtime).Truncate(time.Second)
	}
	jobs := make([]healPutJob, 0, len(missingIdx))
	for _, idx := range missingIdx {
		payload := reconstructed[idx]
		ft := NewRSFooter(metaFooter.ContentLength, metaFooter.MD5[:], metaFooter.SHA256[:], mtime, k, m, idx, metaFooter.StripeSize, metaFooter.NumStripes, crc32cChecksum(payload))
		fb, err := ft.MarshalBinary()
		if err != nil {
			return len(jobs), err
		}
		blob := make([]byte, 0, len(payload)+len(fb))
		blob = append(blob, payload...)
		blob = append(blob, fb...)
		info := object.NewStaticObjectInfo(remote, mtime, int64(len(blob)), true, nil, nil)
		jobs = append(jobs, healPutJob{idx: idx, blob: blob, info: info})
	}
	errs := make([]error, len(jobs))
	var wg sync.WaitGroup
	for j := range jobs {
		wg.Add(1)
		j := j
		go func() {
			defer wg.Done()
			_, errs[j] = f.backends[jobs[j].idx].Put(ctx, bytes.NewReader(jobs[j].blob), jobs[j].info)
		}()
	}
	wg.Wait()
	for j := 0; j < len(jobs); j++ {
		if errs[j] != nil {
			return j, fmt.Errorf("rs: upload rebuilt shard %d for %q failed: %w", jobs[j].idx, remote, errs[j])
		}
	}
	return len(jobs), nil
}

func reconstructMissingShards(shards [][]byte, dataShards, parityShards int, contentLength int64, stripeSize, numStripes uint32) ([][]byte, error) {
	k, m := dataShards, parityShards
	S := int(stripeSize)
	N := int(numStripes)
	if N == 0 {
		out := make([][]byte, k+m)
		return out, nil
	}
	out := make([][]byte, k+m)
	for i := range out {
		out[i] = make([]byte, ShardPayloadLen(contentLength, i, k, m, S))
	}
	for t := 0; t < N; t++ {
		logLen := StripeLogicalLen(k, int64(S), contentLength, t)
		row := make([][]byte, k+m)
		for i := 0; i < k+m; i++ {
			if shards[i] != nil {
				frag := shardStripeFragmentFromPayload(shards[i], i, k, S, t, contentLength, logLen)
				if frag == nil {
					return nil, fmt.Errorf("rs: shard %d stripe %d: fragment out of range", i, t)
				}
				buf := make([]byte, S)
				copy(buf, frag)
				row[i] = buf
			}
		}
		cp := make([][]byte, k+m)
		copy(cp, row)
		fixed, err := reconstructInto(cp, k, m)
		if err != nil {
			return nil, err
		}
		for i := 0; i < k+m; i++ {
			writeShardStripeFragment(out[i], i, k, S, t, contentLength, logLen, fixed[i])
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

// ReconstructMissingShardPayloadsStripeWiseForTest rebuilds payloads only for missingIdx using
// the same stripe-wise algorithm as heal (sparse output: non-missing entries are nil).
// Present shards must supply virtual-padding payloads in shards[i].
func ReconstructMissingShardPayloadsStripeWiseForTest(shards [][]byte, k, m int, contentLength int64, stripeSize, numStripes uint32, missingIdx []int) ([][]byte, error) {
	total := k + m
	missing := make([]bool, total)
	for _, i := range missingIdx {
		missing[i] = true
	}
	N := int(numStripes)
	intS := int(stripeSize)
	if N <= 0 || intS <= 0 {
		return nil, fmt.Errorf("invalid stripe metadata")
	}
	out := make([][]byte, total)
	for _, idx := range missingIdx {
		out[idx] = make([]byte, ShardPayloadLen(contentLength, idx, k, m, intS))
	}
	rowBuf := make([]byte, total*intS)
	row := make([][]byte, total)
	for t := 0; t < N; t++ {
		logLen := StripeLogicalLen(k, int64(intS), contentLength, t)
		for i := 0; i < total; i++ {
			row[i] = nil
		}
		for i := 0; i < total; i++ {
			if missing[i] {
				continue
			}
			if shards[i] == nil {
				return nil, fmt.Errorf("present shard %d is nil", i)
			}
			frag := shardStripeFragmentFromPayload(shards[i], i, k, intS, t, contentLength, logLen)
			if frag == nil {
				return nil, fmt.Errorf("present shard %d stripe %d: fragment out of range", i, t)
			}
			row[i] = rowBuf[i*intS : (i+1)*intS]
			clear(row[i])
			copy(row[i], frag)
		}
		if _, err := reconstructInto(row, k, m); err != nil {
			return nil, err
		}
		for _, idx := range missingIdx {
			frag := row[idx]
			if len(frag) != intS {
				return nil, fmt.Errorf("shard %d stripe %d: fragment length %d", idx, t, len(frag))
			}
			writeShardStripeFragment(out[idx], idx, k, intS, t, contentLength, logLen, frag)
		}
	}
	return out, nil
}
