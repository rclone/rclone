# Open Questions — rs Backend

This document tracks design gaps, limitations, and follow-up work for the Reed-Solomon (`rs`) virtual backend. When a topic is resolved, note the decision here or in code. **Last reviewed**: 2026-04-10. User-facing overview: [`README.md`](../README.md).

---

## Decisions (record)

- **Quorum model**: directory and object metadata operations now follow quorum-style success semantics (with partial-failure logging), not strict all-shards success.
- **Two-phase retries**: operations use one parallel pass plus one bounded retry pass for failing shards.
- **Backend commands**: keep both `heal` (repair) and `degraded` (inspection/reporting).
- **Topology requirement**: `data_shards` must be strictly greater than `parity_shards` (**k > m**).
- **Interim directory delete safety**: only empty directories are removable; if any shard still has entries the operation fails.

## Risks / operational caveats

- **List merge complexity**: quorum listing can hide or delay visibility of minority-shard state; type conflicts (file vs directory) are especially risky.
- **Strict empty-dir rule**: requiring emptiness checks across shard remotes can fail in skewed namespaces and may require `degraded` + `heal` before cleanup.
- **Minority lag after quorum success**: reads can still observe stale shard state until healed (notably around delete/recreate races and footer divergence).

---

## High priority

### Q1: `use_spooling=false` write path

**Status**: Active — not implemented  
**Notes**: `Put` / `PutStream` return an error when `use_spooling` is false (`operations.go`). Forward Reed–Solomon encoding now streams from the logical `io.Reader` stripe-by-stripe in [`encode.go`](../encode.go) (bounded `O(k·S)` buffer); spooling still writes full shard files to disk before upload. Remaining work: pipe encoded output directly to shard `Put`s (memory bounds, backpressure, quorum on partial failure). Declared-size validation when `src.Size() >= 0` matches the chunker pattern (`incorrect upload size`).

---

## Medium priority

### Q2: `Fs.Move` / `Fs.Copy` / `DirMove`

**Status**: Active — not exposed on `rs` `Fs`  
**Notes**: Logical moves/copies must keep shard indices and footers consistent across all backends. Today users must use higher-level copy/delete or operate per-shard manually. Decide whether to implement coordinated multi-remote move/copy with rollback semantics similar to `Put`.

### Q3: Per-operation timeouts and `timeout_mode`

**Status**: Active — inherits per-remote behavior only  
**Notes**: Unlike some composite backends, there is no aggregated timeout or `timeout_mode` option. Slow or hung shards can dominate latency; consider documenting expectations or adding optional deadline propagation / fail-fast policy.

### Q4: Compression and optional transforms

**Status**: Active — not present  
**Notes**: Payloads are raw RS stripes with an EC footer. Optional compression (per shard or logical) would interact with `ContentLength`, hashes, and reconstruction; out of scope for the initial layout unless explicitly specified.

### Q5: Heal and large namespaces

**Status**: Active — operational scaling  
**Notes**: Full heal lists the union of object names across shards (`listAllObjectRemotes`). For very large buckets this may be heavy on API quotas and memory. Consider pagination, prefix arguments, or backend-specific listing strategies.

### Q6: `SetModTime` when shards are missing or backends disagree

**Status**: Partially addressed — quorum updates implemented, semantics still evolving  
**Notes**: `SetModTime` now succeeds at quorum with retries/logging, but cross-shard metadata convergence guarantees (and interaction with `heal`) still need explicit policy and docs.

### Q7: Backend-specific commands (`rclone backend …` on shard remotes)

**Status**: Active — not coordinated  
**Notes**: `status`, `heal`, and `degraded` exist on the `rs` remote. Propagating tag/lifecycle/versioning commands to all shards in lockstep is undefined (compare community discussion for similar composite backends).

### Q10: `rmdirs` / non-empty directory behavior under quorum listing

**Status**: Active — policy still open  
**Notes**: Backends may disagree on directory emptiness. Interim policy is conservative (fail if any shard still has entries), but long-term semantics for recursive removal under quorum visibility need design.

### Q11: Ordering and race semantics

**Status**: Active — not finalized  
**Notes**: Quorum-success writes/deletes can race with recreate/move patterns and concurrent writers. Decide whether to add explicit versioning/serialization or rely on eventual consistency + heal workflow.

### Q12: `degraded` / `heal` command detail level and scan costs

**Status**: Active — initial implementation only  
**Notes**: Need detailed design for output taxonomy, directory skew reporting (`lsd`), machine-readable output, prefix scoping, and large namespace scan behavior.

### Q13: Rollback strategy beyond `Put`

**Status**: Active — deferred  
**Notes**: `Put` has rollback support when quorum is not met. For directory/metadata/delete quorum paths, decide whether to add compensating rollback or keep `degraded` + `heal` as the primary convergence path.

---

## Lower priority / documentation

### Q8: Integration and production hardening

**Status**: Active  
**Notes**: Expand automated tests (regression, fault injection, real S3/MinIO), document recovery runbooks, and clarify behavior under eventual consistency (listing vs. read-after-write).

### Q9: Hash / size surface vs. shard footers

**Status**: Monitoring  
**Notes**: Logical `Size()` and hashes come from the footer of a readable shard. If footers diverge across shards (corruption or buggy partial writes), behavior may be undefined; may warrant explicit cross-shard footer checks in `NewObject` or `status`.
