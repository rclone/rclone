# Open Questions — rs Backend

This document tracks design gaps, limitations, and follow-up work for the Reed-Solomon (`rs`) virtual backend. When a topic is resolved, note the decision here or in code. **Last reviewed**: 2026-06-11 (quorum/listing, virtual padding, list metadata fast path synced with code and `docs/content/rs.md`). User-facing overview: [`README.md`](../README.md).

---

## Decisions (record)

- **Quorum model**: **read/list merge** uses **`k`** (`data_shards`) for directory health, per-name file/dir votes, and degraded healthy thresholds. **Writes** and namespace mutations (`Put`, `mkdir`/`rmdir`, `Copy`/`Move`/`DirMove`, `Remove`, `SetModTime`, …) follow **`write_quorum`** (default **k+1**). Operations use two-phase parallel passes with partial-failure logging, not strict all-shards success.
- **Two-phase retries**: operations use one parallel pass plus one bounded retry pass for failing shards.
- **Backend commands**: keep both `heal` (repair) and `degraded` (inspection/reporting).
- **Topology requirement**: `data_shards` must be strictly greater than `parity_shards` (**k > m**).
- **Interim directory delete safety**: only empty directories are removable; if any shard still has entries the operation fails.
- **Server-side `Copy` / `Move` / `DirMove` on `rs`**: Implemented in [`move_copy.go`](../move_copy.go). Each shard uses the wrapped remote’s `Features().Copy`, `Move` (or `Copy`+`Remove` if `Move` is nil), or `DirMove`, under [`runTwoPhaseQuorumOp`](../rs.go) and **`write_quorum`**. The source object must be `*rs.Object` from an `*rs.Fs` whose **`data_shards`**, **`parity_shards`**, and shard count match** the destination `*rs.Fs` (`compatibleLayout`); otherwise rclone gets `fs.ErrorCantCopy` / `fs.ErrorCantMove` / `fs.ErrorCantDirMove`. There is **no** `Put`-style rollback if a copy/move succeeds on a quorum of shards but leaves others inconsistent—see Q13 and operational notes in [`docs/content/rs.md`](../../../docs/content/rs.md).

## Risks / operational caveats

- **List merge complexity**: quorum listing can hide or delay visibility of minority-shard state; type conflicts (file vs directory) are especially risky.
- **Strict empty-dir rule**: requiring emptiness checks across shard remotes can fail in skewed namespaces and may require `degraded` + `heal` before cleanup.
- **Minority lag after quorum success**: reads can still observe stale shard state until healed (notably around delete/recreate races and footer divergence).

---

## High priority

### Q1: `use_spooling=false` write path

**Status**: Addressed for default-off streaming + unknown-size auto-spool  
**Notes**: Default `use_spooling=false` streams encoded shard particles directly to shard `Put`s when `src.Size() >= 0` (no local shard files), reusing quorum/rollback/size checks. Unknown-size inputs (`src.Size() < 0`, e.g. `rcat`) automatically use the spooling path for that `Put` only (INFO log). Forward Reed–Solomon encoding remains stripe-by-stripe in [`encode.go`](../encode.go) (`O(k·S)` buffer). Optional future work: capability-gated `PutStream` on all shards when every backend supports unknown-length uploads.

---

## Medium priority

### Q2: `Fs.Move` / `Fs.Copy` / `DirMove`

**Status**: Addressed — shard-aligned server-side implementation  
**Notes**: [`move_copy.go`](../move_copy.go) implements `(*Fs).Copy`, `Move`, and `DirMove` as described in **Decisions** above. **Remaining gaps** (not “missing feature” but follow-up): (1) **partial quorum / rollback**—unlike `Put`, these paths do not remove successful shard side-effects when quorum fails mid-flight (Q13). (2) **Backend coverage**—if any shard lacks `Copy`/`Move`/`DirMove`, the whole operation fails that capability check for that path. (3) **S3/MinIO**—see Q14 (`DirMove` notices, post-`moveto` visibility). (4) **Cross-remote `rs` → `rs`**—only works when both sides are compatible `*Fs` layouts; arbitrary remotes still use normal rclone copy (re-encode), not `Fs.Copy`.

### Q3: Per-operation timeouts and `timeout_mode`

**Status**: Active — inherits per-remote behavior only  
**Notes**: Unlike some composite backends, there is no aggregated timeout or `timeout_mode` option. Slow or hung shards can dominate latency; consider documenting expectations or adding optional deadline propagation / fail-fast policy.

### Q4: Compression and optional transforms

**Status**: Rejected — removed from footer v1 format  
**Notes**: Footer v1 has no compression fields. Payloads are raw RS stripe bytes plus the `RCLONERS` footer. Any future transform (compression or otherwise) would need a new footer version and explicit design.

### Q5: Heal and large namespaces

**Status**: Active — operational scaling  
**Notes**: Full heal lists the union of object names across shards (`listAllObjectRemotes`). For very large buckets this may be heavy on API quotas and memory. Consider pagination, prefix arguments, or backend-specific listing strategies.

### Q6: `SetModTime` when shards are missing or backends disagree

**Status**: Partially addressed — quorum updates implemented, semantics still evolving  
**Notes**: `SetModTime` now succeeds at quorum with retries/logging, but cross-shard metadata convergence guarantees (and interaction with `heal`) still need explicit policy and docs.

### Q7: `SetModTime` / `updateShardFooterMtime` — avoid loading full shard payload into memory

**Status**: Active — future optimization  
**Notes**: Today [`updateShardFooterMtime`](../object.go) range-opens the payload then **`ReadAll`s it into a `[]byte`** before `MultiReader` + `Object.Update`, so large shard payloads use **O(payload)** RAM in the rclone process per shard update.

**Possible directions** (may combine with Q6 docs once designed):

- **Stream the payload**: pipe `Open(RangeOption{… payload only})` through to `Update` without materializing the full payload in one buffer (still one logical `Update` per shard if the backend accepts streaming `Put`/`Update`).
- **Rename + server-side range copy + footer** (where the wrapped `Fs` supports it): e.g. rename original → temp, server-side copy payload bytes into a new object under the original name, append or write new footer — avoids client-side download of the payload on rich remotes (S3-style multipart copy, etc.).
- **Portable fallback**: keep or simplify the current path when optional interfaces / `Features` do not expose server-side copy or atomic rename semantics.

**Blockers**: `rs` only sees generic `fs.Fs` / `fs.Object`; not every backend offers server-side byte-range copy or append; any fast path needs **capability checks** and a **safe fallback**.

### Q8: Parallel `Write`s to shard writers after each RS stripe (`encode.go`)

**Status**: Active — optional optimization / investigation  
**Notes**: Stripe-wise encoding in [`encode.go`](../encode.go) (`encodeLogicalToShardWriters`): after `enc.Encode` each shard’s **`S`-byte fragment** is written with a **sequential** loop over the per-shard `io.Writer`s (spool files or `io.PipeWriter`s feeding concurrent shard `Put`s in [`operations.go`](../operations.go)). **Idea:** run those **`Write`s in parallel** for a single stripe (e.g. `errgroup`), then **barrier** before reusing `stripeBuf` / `shards` for the next stripe. **Risks / trade-offs:** (1) **`io.Pipe`** backpressure unchanged in principle—slow `Put` still blocks that writer; parallelism might help when shards drain at different rates. (2) **Goroutine churn**: up to **k+m** tasks per stripe × stripe count—may need a cap or worker pool. (3) **Correctness**: fragments must not be overwritten until all shard `Write`s for that stripe complete (or copy fragment bytes). (4) **Measure first**—benchmark vs. today’s simple sequential loop; spooling to local disk may see little gain for small **`S`** writes.

### Q9: Backend-specific commands (`rclone backend …` on shard remotes)

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

### Q14: MinIO / S3 shell-test observations (`compare.sh` + `compare_sequential.sh`)

**Status**: Logged — revisit behavior vs. harness expectations  
**Notes** (from `bash compare_sequential.sh --storage-type=minio` with Docker MinIO):

- **`quorum_dirs` / `lsd`**: After `mkdir` of a new empty directory, `lsd` on the `rs` remote did not list that directory on MinIO; the test treats this as **accepted** today. Follow up: quorum listing vs. S3 empty-prefix / eventual visibility, or tighten the test if this is a bug.
- **`moveto`**: On MinIO, the **source object can still be readable** after a successful `moveto` under current quorum semantics; the test logs this as **accepted**. Follow up: document expected client-visible semantics or align shards so post-move source reads fail consistently.
- **`DirMove`**: Phase 2 logs **`NOTICE`** lines such as `can't move directory - incompatible remotes` per shard while the overall `move_copy` test still **passes**. Follow up: whether `DirMove` should use a different strategy for S3 shard backends (server-side move limitations), reduce noisy notices, or document as expected.

---

## Lower priority / documentation

### Q15: Integration and production hardening

**Status**: Active  
**Notes**: Expand automated tests (regression, fault injection, real S3/MinIO), document recovery runbooks, and clarify behavior under eventual consistency (listing vs. read-after-write).

### Q16: Hash / size surface vs. shard footers

**Status**: Monitoring  
**Notes**: Logical `Size()` and hashes come from the footer of a readable shard. If footers diverge across shards (corruption or buggy partial writes), behavior may be undefined; may warrant explicit cross-shard footer checks in `NewObject` or `status`.
