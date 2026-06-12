# Open Questions — rs Backend

This document tracks design gaps, limitations, and follow-up work for the Reed-Solomon (`rs`) virtual backend. When a topic is resolved, note the decision here or in code. **Last reviewed**: 2026-06-10 (metadata authority, SetModTime hybrid, lazy list/NewObject). User-facing overview: [`README.md`](../README.md). Normative metadata policy: [`LIST_METADATA.md`](LIST_METADATA.md). Normative write/namespace policy: [`QUORUM_TRANSACTIONS.md`](QUORUM_TRANSACTIONS.md).

---

## Decisions (record)

- **Quorum model**: **read/list merge** uses **`k`** (`data_shards`) for directory health, per-name file/dir votes, and degraded healthy thresholds. **Writes** and namespace mutations (`Put`, `mkdir`/`rmdir`, `Copy`/`Move`/`DirMove`, `Remove`, `SetModTime`, …) follow **`write_quorum`** (default **k+1**). **Execute** all shards in the op set (no early exit); **commit** when `successes >= write_quorum`; laggards converged by **async heal**. See [`QUORUM_TRANSACTIONS.md`](QUORUM_TRANSACTIONS.md).
- **Namespace transactions**: preflight → execute → commit/rollback; **no separate metadata records** (rejected inode/dirent objects). Per-shard backing remotes remain authoritative; implementation Phases 1–3 pending.
- **Two-phase retries**: operations use one parallel pass plus one bounded retry pass for failing shards.
- **Backend commands**: keep both `heal` (repair) and `degraded` (inspection/reporting).
- **Topology requirement**: `data_shards` must be strictly greater than `parity_shards` (**k > m**). Reed–Solomon does not require this; it is v1 policy. With **k ≤ m**, two **disjoint** sets of **k** shards can each meet list/read quorum for the same path (e.g. `k=m=2`: particles for “v1” on shards 0–1 and “v2” on 2–3) while `List` still returns one name—reads follow lowest-index / data-shard join, so versions are ambiguous. **k > m** implies `2k > k+m`, so any two k-subsets overlap and cannot form fully separate version partitions.
- **Directory delete (target spec)**: `Rmdir` preflight requires `reachable >= write_quorum`; among **reachable** shards that have the dir, each must list **no children**; commit at `write_quorum` with rollback on failure. Minority unreachable shards or orphans may still hold data until **heal** (see [`QUORUM_TRANSACTIONS.md`](QUORUM_TRANSACTIONS.md)). *Code today is stricter on availability and weaker on commit bar—Phase 2.*
- **Server-side `Copy` / `Move` / `DirMove` on `rs`**: Implemented in [`move_copy.go`](../move_copy.go) via [`runQuorumTransaction`](../quorum_op.go) (preflight, commit at **`write_quorum`**, compensating rollback when **`rollback`** is enabled). The source object must be `*rs.Object` from a compatible `*rs.Fs` (`compatibleLayout`: same **k**, **m**, shard count); otherwise `fs.ErrorCantCopy` / `fs.ErrorCantMove` / `fs.ErrorCantDirMove`. **`Remove`** / **`SetModTime`** use the same framework; delete rollback is irreversible per spec.

## Risks / operational caveats

- **List merge complexity**: quorum listing can hide or delay visibility of minority-shard state; type conflicts (file vs directory) are especially risky.
- **Empty-dir under skew**: preflight emptiness is on the **reachable cohort**; orphans on laggard shards may not block logical `rmdir` after spec alignment—use `degraded` + `heal`.
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
**Notes**: [`move_copy.go`](../move_copy.go) implements `(*Fs).Copy`, `Move`, and `DirMove` as described in **Decisions** above. **Remaining gaps** (not “missing feature” but follow-up): (1) **Backend coverage**—if any shard lacks `Copy`/`Move`/`DirMove`, the whole operation fails that capability check for that path. (3) **S3/MinIO**—see Q14 (`DirMove` notices, post-`moveto` visibility). (4) **Cross-remote `rs` → `rs`**—only works when both sides are compatible `*Fs` layouts; arbitrary remotes still use normal rclone copy (re-encode), not `Fs.Copy`.

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

**Status**: Addressed — policy documented  
**Notes**: [`SetModTime`](../object.go) commits at **`write_quorum`** with per-shard strategy: shard `Object.SetModTime` (1s) on ModTime-capable backends; [`updateShardFooterMtime`](../object.go) only on `ModTimeNotSupported` shards. Rollback restores prior remote or footer mtime per shard. Logical **`ModTime()`** reads shard remotes when supported (footer not authoritative on those backends). **Heal** converges rebuilt shards to [`resolveHealReferenceModTime`](../list_metadata.go) from surviving remotes. Laggards after quorum success still need **`backend heal`**. Normative detail: [`LIST_METADATA.md`](LIST_METADATA.md), [`QUORUM_TRANSACTIONS.md`](QUORUM_TRANSACTIONS.md).

### Q7: `SetModTime` / `updateShardFooterMtime` — avoid loading full shard payload into memory

**Status**: Active — footer-fallback optimization only  
**Notes**: Primary `SetModTime` path uses shard `Object.SetModTime` (no payload I/O). The **footer fallback** for `ModTimeNotSupported` backends still [`ReadAll`s](../object.go) the payload before `Object.Update` — **O(payload)** RAM per affected shard.

**Possible directions** (footer fallback only):

- **Stream the payload**: pipe range-opened payload through to `Update` without `ReadAll`.
- **Server-side range copy + footer** where the wrapped `Fs` supports it.
- **Portable fallback**: keep current path when capabilities are missing.

### Q17: `SetModTime` — shard remote mtime only vs footer rewrite

**Status**: Resolved — hybrid policy implemented  
**Decision**: ModTime-capable shards use **remote `SetModTime` only** (footer `Mtime` may lag). `ModTimeNotSupported` shards use **footer rewrite**. Read/list authority: shard remote wins over footer when supported; heal writes matching remote + footer on rebuilt particles. See [`LIST_METADATA.md`](LIST_METADATA.md) and [`QUORUM_TRANSACTIONS.md`](QUORUM_TRANSACTIONS.md) SetModTime table. Remaining cost: footer fallback path (Q7).

### Q8: Parallel `Write`s to shard writers after each RS stripe (`encode.go`)

**Status**: Active — optional optimization / investigation  
**Notes**: Stripe-wise encoding in [`encode.go`](../encode.go) (`encodeLogicalToShardWriters`): after `enc.Encode` each shard’s **`S`-byte fragment** is written with a **sequential** loop over the per-shard `io.Writer`s (spool files or `io.PipeWriter`s feeding concurrent shard `Put`s in [`operations.go`](../operations.go)). **Idea:** run those **`Write`s in parallel** for a single stripe (e.g. `errgroup`), then **barrier** before reusing `stripeBuf` / `shards` for the next stripe. **Risks / trade-offs:** (1) **`io.Pipe`** backpressure unchanged in principle—slow `Put` still blocks that writer; parallelism might help when shards drain at different rates. (2) **Goroutine churn**: up to **k+m** tasks per stripe × stripe count—may need a cap or worker pool. (3) **Correctness**: fragments must not be overwritten until all shard `Write`s for that stripe complete (or copy fragment bytes). (4) **Measure first**—benchmark vs. today’s simple sequential loop; spooling to local disk may see little gain for small **`S`** writes.

### Q9: Backend-specific commands (`rclone backend …` on shard remotes)

**Status**: Active — not coordinated  
**Notes**: `status`, `heal`, and `degraded` exist on the `rs` remote. Propagating tag/lifecycle/versioning commands to all shards in lockstep is undefined (compare community discussion for similar composite backends).

### Q10: `rmdirs` / non-empty directory behavior under quorum listing

**Status**: Specified — implementation Phase 2  
**Notes**: Policy in [`QUORUM_TRANSACTIONS.md`](QUORUM_TRANSACTIONS.md): emptiness on **preflight cohort** (reachable shards that have the dir), `write_quorum` commit, compensating rollback. Recursive `rmdirs` inherits per-step `Rmdir` rules. Post-commit namespace convergence via **`backend heal`** (orphan purge, extra `Rmdir`, laggard `Mkdir`) and **`backend degraded lsd`** — see [`namespace_heal.go`](../namespace_heal.go).

### Q11: Ordering and race semantics

**Status**: Active — not finalized  
**Notes**: Quorum-success writes/deletes can race with recreate/move patterns and concurrent writers. Decide whether to add explicit versioning/serialization or rely on eventual consistency + heal workflow.

### Q12: `degraded` / `heal` command detail level and scan costs

**Status**: Active — initial implementation only  
**Notes**: Need detailed design for output taxonomy, directory skew reporting (`lsd`), machine-readable output, prefix scoping, and large namespace scan behavior.

### Q13: Rollback strategy beyond `Put`

**Status**: Implemented (Phases 1–4a)  
**Notes**: [`QUORUM_TRANSACTIONS.md`](QUORUM_TRANSACTIONS.md): **`rollback`** config applies to namespace and object ops (`Mkdir`↔`Rmdir`, inverse `DirMove`, copy dst remove, move inverse, `SetModTime` footer restore). **Successful** quorum commits still rely on **async heal** for laggards. `Remove` is irreversible once shard deletes succeed (preflight only before execute).

### Q14: MinIO / S3 shell-test observations (`compare.sh` + `compare_sequential.sh`)

**Status**: Logged — revisit behavior vs. harness expectations  
**Notes** (from `bash compare_sequential.sh --storage-type=minio` with Docker MinIO):

- **`quorum_dirs` / `lsd`**: After `mkdir` of a new empty directory, `lsd` on the `rs` remote did not list that directory on MinIO; the test treats this as **accepted** today. Follow up: quorum listing vs. S3 empty-prefix / eventual visibility, or tighten the test if this is a bug.
- **`moveto`**: On MinIO, the **source object can still be readable** after a successful `moveto` under current quorum semantics; the test logs this as **accepted**. Follow up: document expected client-visible semantics or align shards so post-move source reads fail consistently.
- **`DirMove`**: Phase 2 logs **`NOTICE`** lines such as `can't move directory - incompatible remotes` per shard while the overall `move_copy` test still **passes**. Follow up: whether `DirMove` should use a different strategy for S3 shard backends (server-side move limitations), reduce noisy notices, or document as expected.
- **`Copy` / `Move` return metadata**: [`newObjectAfterCopyMove`](../object.go) returns provisional size/ModTime from the source object (no destination footer read). Correct when shard backends preserve ModTime on server-side copy/move (local, MinIO). If S3 copy does not preserve shard ModTime, returned ModTime may not match destination remotes until `Open`/`NewObject` refresh — acceptable for sync paths that re-probe; document or tighten if integration tests show drift.

---

## Lower priority / documentation

### Q15: Integration and production hardening

**Status**: Active  
**Notes**: Expand automated tests (regression, fault injection, real S3/MinIO), document recovery runbooks, and clarify behavior under eventual consistency (listing vs. read-after-write).

### Q16: Hash / size surface vs. shard footers

**Status**: Monitoring  
**Notes**: Logical `Size()` prefers k data-shard remote sizes; footer `ContentLength` is fallback. **`Hash`** still requires a footer. Footer vs remote mtime skew after cheap `SetModTime` is benign for `ModTime()` on capable backends; heal converges missing shards. Cross-shard footer divergence (corruption) may still be undefined; may warrant explicit checks in `NewObject` or `status`.
