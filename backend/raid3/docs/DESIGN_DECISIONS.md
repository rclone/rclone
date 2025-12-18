# Design Decisions - raid3 Backend

This document records key architectural and design decisions made during the development of the raid3 backend, serving as a decision log (historical record of why certain choices were made), rationale documentation (explanation of trade-offs and alternatives considered), reference for maintainers (understanding the reasoning behind current implementation), and future guidance (context for similar decisions that may arise). Format: Lightweight ADR (Architecture Decision Record) style. Last Updated: December 8, 2025. For user documentation, see [`README.md`](README.md). For open questions and pending decisions, see [`OPEN_QUESTIONS.md`](OPEN_QUESTIONS.md).

---

## How to Use This Document

To add a new decision, add a new section with sequential number (DD-XXX), include Context, Decision, Rationale, Alternatives, and Status, and link to detailed docs in `docs/` if available. Format: DD-XXX: [Title], Date: YYYY-MM-DD, Status: Accepted | Proposed | Deprecated, Context: What problem are we solving?, Decision: What did we decide?, Rationale: Why this choice?, Alternatives: What else was considered?, Consequences: Trade-offs and implications, References: Links to detailed docs.

---

## Decisions

### DD-001: Hardware RAID 3 Compliance for Error Handling
**Date**: 2025-11-02  
**Status**: ✅ Accepted  

**Context**: How should raid3 handle unavailable backends? Should writes succeed with 2 of 3 backends (high availability) or require all 3 (strict)?

**Decision**: Strict writes (all 3 backends required), matching hardware RAID 3 behavior: reads work with 2 of 3 (degraded mode), writes require all 3 (strict mode), deletes use best effort (idempotent).

**Rationale**: Matches hardware RAID 3 controllers (industry standard), prevents creating degraded files from the start, avoids performance degradation (reconstruction overhead), and ensures data consistency. **Alternatives Considered**: Option B (configurable with default strict, optional degraded writes), Option C (always allow degraded writes for high availability). **Consequences**: Data consistency guaranteed, no corruption possible, writes fail when backend unavailable (expected RAID behavior), and simple implementation.

**References**: `docs/ERROR_HANDLING.md`

---

### DD-002: Pre-flight Health Check for Write Operations
**Date**: 2025-11-02  
**Status**: ✅ Accepted  

**Context**: Discovered that rclone's retry logic could bypass strict write policy, creating degraded/corrupted files on retries.

**Decision**: Add pre-flight health check before all write operations: `checkAllBackendsAvailable(ctx)` with 5-second timeout, tests all 3 backends before Put/Update/Move, fails immediately with clear error if any unavailable.

**Rationale**: Prevents rclone's retry logic from creating partial files, critical for preventing Update corruption, provides fast failure (5 seconds) vs hanging (minutes), and provides clear error messages. **Alternatives Considered**: Disable retries globally (affects all operations), add rollback logic (complex), trust errgroup alone (insufficient - retries bypass it). **Consequences**: Prevents all corruption scenarios, fast failure in degraded mode, +0.2s overhead per write operation (acceptable), and simple to implement.

**References**: `docs/STRICT_WRITE_POLICY.md`, `docs/FIXES_COMPLETE.md`

---

### DD-003: Timeout Modes for S3/MinIO
**Date**: 2025-11-01  
**Status**: ✅ Accepted  

**Context**: S3 operations hang for 2-5 minutes when backend unavailable due to AWS SDK retry logic.

**Decision**: Add `timeout_mode` option with three presets: standard (use global settings for local/file storage), balanced (3 retries, 30s timeout for reliable S3), aggressive (1 retry, 10s timeout for degraded mode S3, default for MinIO).

**Rationale**: Reduces failover time from 5 minutes to 6-7 seconds (aggressive), allows user to choose based on their backend type, and uses `fs.AddConfig()` for local override. **Alternatives Considered**: Modify AWS SDK (rejected - too invasive), use different S3 SDK (rejected - architectural change), global timeout override (rejected - affects all backends). **Consequences**: Fast degraded mode (6-7 seconds), user choice for different scenarios, configuration complexity (3 modes to understand), and no architectural changes.

**References**: `docs/TIMEOUT_MODE.md`, `docs/S3_TIMEOUT_RESEARCH.md`

---

### DD-004: Auto-Heal with Background Workers
**Date**: 2025-11-01  
**Status**: ✅ Accepted  

**Context**: When reading in degraded mode, should missing particles be restored automatically?

**Decision**: Solution D (Hybrid Auto-detect) with background workers: queue missing particles for upload during reads (controlled by `auto_heal` option), background workers process uploads asynchronously, shutdown waits for uploads only if queue non-empty, deduplication prevents duplicate uploads.

**Rationale**: No delay when no healing needed (fast exit), reliable completion when healing needed, background workers don't block operations, and graceful shutdown ensures uploads complete. **Alternatives Considered**: Option A (block until upload complete - slow), Option B (fire-and-forget - unreliable), Option C (user-initiated rebuild - manual). **Consequences**: Automatic healing (no user intervention when `auto_heal=true`), fast when not needed (immediate exit), reliable when needed (waits for uploads), background workers add complexity, graceful shutdown within 60 seconds, configurable via `auto_heal` option (default: `true`), and explicit `heal` command available for proactive healing.

**Implementation Notes**: Implemented as `auto_heal` configuration option (default: `true`), background workers handle upload queue (`heal.go`), explicit `rclone backend heal raid3:` command available for manual healing, directory reconstruction also supported during `List()` operations.

**References**: `docs/CLEAN_HEAL.md`, `docs/SELF_HEALING_RESEARCH.md`, `README.md` (Auto-Heal section)

---

### DD-005: Parity Filename Suffixes
**Date**: 2025-10-31  
**Status**: ✅ Accepted  

**Context**: How to distinguish parity files from original files and encode odd/even length information?

**Decision**: Use suffixes: `.parity-el` for even-length original, `.parity-ol` for odd-length original.

**Rationale**: Distinctive suffixes (no collision with real files), encodes essential reconstruction information, and simple to parse and validate. **Alternatives Considered**: `.parity` only (rejected - doesn't encode length), `.el` and `.ol` only (rejected - too generic), metadata in separate file (rejected - too complex). **Consequences**: Clear identification of parity files, reconstruction possible without even/odd particles, and simple implementation.

**References**: [`RAID3.md`](RAID3.md)

---

### DD-006: Byte-Level Striping (not block-level)
**Date**: 2025-10-31  
**Status**: ✅ Accepted  

**Context**: What granularity for data striping - byte, block, or other?

**Decision**: Byte-level striping: even indices (0, 2, 4, ...) → even remote, odd indices (1, 3, 5, ...) → odd remote.

**Rationale**: True RAID 3 behavior (byte-level), simple algorithm (no block alignment), works with any file size, and even distribution for small files. **Alternatives Considered**: Block-level striping (RAID 5 style - more complex), variable-size chunks (unnecessary complexity). **Consequences**: Simple implementation, works with any file size, and perfect 50/50 distribution.

**References**: [`RAID3.md`](RAID3.md)

---

### DD-007: XOR Parity for Odd-Length Files
**Date**: 2025-10-31  
**Status**: ✅ Accepted  

**Context**: How to handle parity calculation when file has odd number of bytes (last even byte has no odd partner)?

**Decision**: Last parity byte = last even byte (no XOR):
```
Even:   [A, C, E, G]  (4 bytes)
Odd:    [B, D, F]     (3 bytes)
Parity: [A^B, C^D, E^F, G]  (4 bytes)
                        ↑ no XOR partner
```

**Rationale**: Mathematically correct for XOR reconstruction, parity size equals even size (consistent), enables reconstruction from any 2 particles. **Alternatives Considered**: Pad odd data with zero (rejected - changes data), store last byte separately (rejected - too complex). **Consequences**: Correct reconstruction math, simple implementation, works for all file sizes.

**References**: [`RAID3.md`](RAID3.md)

---

### DD-008: Test Documentation Structure
**Date**: 2025-11-02  
**Status**: ✅ Accepted  

**Context**: How to document tests so other developers understand what they do and why they exist?

**Decision**: Structured doc comments with format:
```go
// TestName tests [what].
//
// [Context paragraph - why this test exists]
//
// This test verifies:
//   - Point 1
//   - Point 2
//
// Failure indicates: [what's broken]
```

**Rationale**: Clear purpose for each test, helps debugging (know what's broken), improves maintainability, self-documenting code. **Alternatives Considered**: Minimal comments (rejected - hard to maintain), external test documentation only (rejected - comments better). **Consequences**: Self-documenting tests, easier debugging, better maintainability, more verbose test files.

**References**: `TEST_DOCUMENTATION_PROPOSAL.md`, [`TESTING.md`](TESTING.md)

---

## 🤔 Open Questions

### OQ-001: Backend Help Command Behavior
**Date**: 2025-11-02  
**Status**: 🔴 Open  

**Question**: How should `rclone backend help raid3:` behave?

**Options**: A (aggregated information like `union` backend), B (per-remote information like `combine` backend), C (raid3-specific help - custom). **Considerations**: Union shows combined capabilities, Combine shows per-remote details, raid3 has 3 remotes with specific roles (even/odd/parity). **Investigation Needed**: Check union backend implementation, check combine backend implementation, determine what's most useful for users. **Decision Deadline**: None (low priority). **Owner**: TBD

---

### OQ-002: Streaming Support (Large Files)
**Date**: 2025-11-03  
**Status**: 🔴 Open  

**Question**: Should raid3 support streaming for large files instead of loading entire file into memory?

**Current**: Loads entire file into memory for splitting

**Pros of streaming**: Handle files larger than RAM, better memory efficiency, scalability. **Cons**: More complex implementation, hash calculation requires full read anyway, parity requires all data. **Decision Needed**: Determine if memory buffering is acceptable for target use cases.

---

### OQ-003: Move with Degraded Source Files
**Date**: 2025-11-02  
**Status**: 🟡 Tentative (current behavior documented)  

**Question**: Should Move work when source file has missing particles?

**Current Behavior**: Move succeeds, propagates degraded state to new location

**Options**: A (fail - require all source particles, strict), B (allow - current, flexible), C (reconstruct first then move - smart but slow). **Current Decision**: Keep Option B (allow). **Reconsider?**: Only if users report issues.

---

## 📝 Decision Process

### When to Create a Decision Record

Create decision records for architectural choices with multiple viable options, trade-offs between competing concerns (safety vs performance), behaviors that affect users or data integrity, and anything that might need justification later. Don't create records for obvious implementation details, trivial choices with single correct answer, or internal refactoring with no external impact.

### Decision Template (Copy This):

```markdown
### DD-XXX: [Title]
**Date**: YYYY-MM-DD  
**Status**: Proposed | Accepted | Deprecated  

**Context**: [What problem? What constraints?]

**Decision**: [What did we decide?]

**Rationale**: [Why this choice?]

**Alternatives Considered**:
- Option A: [description]
- Option B: [description]

**Consequences**:
- ✅ Benefit 1
- ✅ Benefit 2
- ⚠️ Trade-off 1
- ❌ Limitation 1

**References**: [Links to detailed docs]
```

---

## 🎯 Summary

Total decisions documented: 8. Status: All accepted and implemented. Open questions: 3 (low priority). This document provides a quick reference for understanding why the raid3 backend works the way it does. For detailed implementation notes, see files in `docs/` directory.

