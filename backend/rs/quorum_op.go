package rs

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/rclone/rclone/fs"
	"golang.org/x/sync/errgroup"
)

// quorumTransaction runs preflight → execute (all targets, two-phase retry) → commit or compensating rollback.
// See backend/rs/docs/QUORUM_TRANSACTIONS.md.
type quorumTransaction struct {
	OpName        string
	Remote        string
	Shards        []int // nil or empty = all backends 0..n-1
	Forward       func(context.Context, int) error
	Rollback      func(context.Context, int) error // nil = no compensating rollback
	SkipPreflight bool
	// CommitError overrides the default quorum-not-met error after rollback (e.g. DirMove → ErrorDirExists).
	CommitError func(quorumOpResult, int) error
}

// preflightReachableShards probes each backend with List at root and returns indices that respond.
// ErrorDirNotFound counts as reachable. Fails when len(reachable) < write_quorum.
func (f *Fs) preflightReachableShards(ctx context.Context) ([]int, error) {
	g, gctx := errgroup.WithContext(ctx)
	type probe struct {
		shard     int
		reachable bool
		err       error
	}
	out := make([]probe, len(f.backends))
	for i := range f.backends {
		i := i
		g.Go(func() error {
			_, err := f.backends[i].List(gctx, "")
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					out[i] = probe{shard: i, err: err}
					return err
				}
				if errors.Is(err, fs.ErrorDirNotFound) {
					out[i] = probe{shard: i, reachable: true}
					return nil
				}
				fs.Logf(f, "rs: preflight shard=%d unreachable: %v", i, err)
				out[i] = probe{shard: i, reachable: false}
				return nil
			}
			out[i] = probe{shard: i, reachable: true}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	reachable := make([]int, 0, len(f.backends))
	for _, p := range out {
		if p.reachable {
			reachable = append(reachable, p.shard)
		}
	}
	if len(reachable) < f.writeQuorum() {
		return reachable, fmt.Errorf("rs: insufficient reachable remotes for quorum: available=%d required=%d", len(reachable), f.writeQuorum())
	}
	return reachable, nil
}

// rmdirPreflightHadDir lists dir on each reachable shard and returns shards that have an empty directory.
// Unreachable shards are not probed. ErrorDirNotFound means the shard is omitted from the op set.
func (f *Fs) rmdirPreflightHadDir(ctx context.Context, dir string, reachable []int) ([]int, error) {
	type listDirRes struct {
		hasDir bool
	}
	out := make([]listDirRes, len(reachable))
	g, gctx := errgroup.WithContext(ctx)
	for j, shard := range reachable {
		j, shard := j, shard
		g.Go(func() error {
			entries, err := f.backends[shard].List(gctx, dir)
			if err != nil {
				if errors.Is(err, fs.ErrorDirNotFound) {
					return nil
				}
				return fmt.Errorf("rs: shard %d: failed to verify dir state for %q: %w", shard, dir, err)
			}
			if len(entries) > 0 {
				return fmt.Errorf("rs: shard %d: %w for %q", shard, fs.ErrorDirectoryNotEmpty, dir)
			}
			out[j].hasDir = true
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	hadDir := make([]int, 0, len(reachable))
	for j, shard := range reachable {
		if out[j].hasDir {
			hadDir = append(hadDir, shard)
		}
	}
	return hadDir, nil
}

// mkdirPreflight fails when the path is visible as a file on >= readQuorum reachable shards.
func (f *Fs) mkdirPreflight(ctx context.Context, dir string, reachable []int) error {
	if dir == "" {
		return nil
	}
	var fileVotes int
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	for _, shard := range reachable {
		shard := shard
		g.Go(func() error {
			_, err := f.backends[shard].NewObject(gctx, dir)
			if err == nil || errors.Is(err, fs.ErrorIsFile) {
				mu.Lock()
				fileVotes++
				mu.Unlock()
			}
			// ErrorObjectNotFound and directory-path probe errors are not file votes.
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	if fileVotes >= f.readQuorum() {
		return fs.ErrorIsFile
	}
	return nil
}

func shardHasDir(ctx context.Context, b fs.Fs, dir string) (bool, error) {
	_, err := b.List(ctx, dir)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrorDirNotFound) {
		return false, nil
	}
	return false, err
}

func shardHasFile(ctx context.Context, b fs.Fs, remote string) (bool, error) {
	_, err := b.NewObject(ctx, remote)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrorObjectNotFound) {
		return false, nil
	}
	return false, err
}

func shardPathOccupied(ctx context.Context, b fs.Fs, remote string) (bool, error) {
	hasDir, err := shardHasDir(ctx, b, remote)
	if err != nil || hasDir {
		return hasDir, err
	}
	return shardHasFile(ctx, b, remote)
}

// dirmovePreflight requires src as directory on >= write_quorum reachable shards with dst absent there.
func (f *Fs) dirmovePreflight(ctx context.Context, srcFs *Fs, srcRemote, dstRemote string, reachable []int) error {
	var eligible, srcVotes int
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	for _, shard := range reachable {
		shard := shard
		g.Go(func() error {
			hasSrc, err := shardHasDir(gctx, srcFs.backends[shard], srcRemote)
			if err != nil {
				return fmt.Errorf("rs: shard %d: failed to verify source dir %q: %w", shard, srcRemote, err)
			}
			occupied, err := shardPathOccupied(gctx, f.backends[shard], dstRemote)
			if err != nil {
				return fmt.Errorf("rs: shard %d: failed to verify destination %q: %w", shard, dstRemote, err)
			}
			mu.Lock()
			if hasSrc {
				srcVotes++
				if !occupied {
					eligible++
				}
			}
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	wq := f.writeQuorum()
	if eligible >= wq {
		return nil
	}
	if srcVotes < wq {
		return fs.ErrorDirNotFound
	}
	return fs.ErrorDirExists
}

// runQuorumTransaction executes tx and returns nil when successes >= write_quorum.
// On failed commit with rollback enabled and Rollback set, runs compensating rollback on successful shards.
func (f *Fs) runQuorumTransaction(ctx context.Context, tx quorumTransaction) error {
	shards := tx.Shards
	if len(shards) == 0 {
		shards = make([]int, len(f.backends))
		for i := range f.backends {
			shards[i] = i
		}
	}
	if !tx.SkipPreflight {
		if _, err := f.preflightReachableShards(ctx); err != nil {
			return err
		}
	}
	result := f.runTwoPhaseQuorumOp(ctx, tx.OpName, tx.Remote, shards, tx.Forward)
	wq := f.writeQuorum()
	if result.Successes >= wq {
		return nil
	}
	if f.opt.Rollback && tx.Rollback != nil {
		successes := quorumSuccessIndices(shards, result.Failures)
		if err := f.runCompensatingRollback(ctx, tx.OpName, tx.Remote, successes, tx.Rollback); err != nil {
			fs.Logf(f, "rs: %s %q rollback incomplete: %v", tx.OpName, tx.Remote, err)
		}
	}
	if tx.CommitError != nil {
		return tx.CommitError(result, wq)
	}
	return fmt.Errorf("rs: %s quorum not met for %q: successes=%d required=%d", tx.OpName, tx.Remote, result.Successes, wq)
}

func quorumSuccessIndices(shards []int, failures map[int]shardFailure) []int {
	out := make([]int, 0, len(shards))
	for _, shard := range shards {
		if _, failed := failures[shard]; !failed {
			out = append(out, shard)
		}
	}
	return out
}

// runCompensatingRollback runs rollbackFn on each shard in successes (best-effort, parallel).
func (f *Fs) runCompensatingRollback(ctx context.Context, opName, remote string, successes []int, rollbackFn func(context.Context, int) error) error {
	if len(successes) == 0 || rollbackFn == nil {
		return nil
	}
	var firstErr error
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, shard := range successes {
		shard := shard
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := rollbackFn(ctx, shard); err != nil {
				fs.Logf(f, "rs: %s %q rollback shard=%d failed: %v", opName, remote, shard, err)
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return firstErr
}
