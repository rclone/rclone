# Open Questions — rs Backend

This document tracks design gaps, limitations, and follow-up work for the Reed-Solomon (`rs`) virtual backend. When a topic is resolved, note the decision here or in code. **Last reviewed**: 2026-03-28. User-facing overview: [`README.md`](../README.md).

---

## High priority

### Q1: `use_spooling=false` write path

**Status**: Active — not implemented  
**Notes**: `Put` / `PutStream` return an error when `use_spooling` is false (`operations.go`). Streaming encode-and-upload without a full local spool file is still to be designed (memory bounds, stripe buffering, quorum on partial failure).

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

**Status**: Partially addressed — implementation requires every shard object  
**Notes**: `SetModTime` updates each shard in parallel via `Object.Update`. If a shard is missing or `NewObject` fails on one backend, the whole operation fails. Alternatives: allow degraded update (document inconsistency), or queue repair after a successful subset (risky).

### Q7: Backend-specific commands (`rclone backend …` on shard remotes)

**Status**: Active — not coordinated  
**Notes**: Only `status` and `heal` exist on the `rs` remote. Propagating tag/lifecycle/versioning commands to all shards in lockstep is undefined (compare community discussion for similar composite backends).

---

## Lower priority / documentation

### Q8: Integration and production hardening

**Status**: Active  
**Notes**: Expand automated tests (regression, fault injection, real S3/MinIO), document recovery runbooks, and clarify behavior under eventual consistency (listing vs. read-after-write).

### Q9: Hash / size surface vs. shard footers

**Status**: Monitoring  
**Notes**: Logical `Size()` and hashes come from the footer of a readable shard. If footers diverge across shards (corruption or buggy partial writes), behavior may be undefined; may warrant explicit cross-shard footer checks in `NewObject` or `status`.
