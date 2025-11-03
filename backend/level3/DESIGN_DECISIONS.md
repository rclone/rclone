# Design Decisions - Level3 Backend

**Purpose**: Document key design decisions, rationale, and alternatives considered  
**Format**: Lightweight ADR (Architecture Decision Record) style  
**Last Updated**: November 2, 2025

---

## How to Use This Document

### Adding a New Decision:
1. Add a new section with sequential number (DD-XXX)
2. Include: Context, Decision, Rationale, Alternatives, Status
3. Link to detailed docs in `docs/` if available

### Format Template:
```markdown
## DD-XXX: [Title]
**Date**: YYYY-MM-DD  
**Status**: Accepted | Proposed | Deprecated  
**Context**: What problem are we solving?  
**Decision**: What did we decide?  
**Rationale**: Why this choice?  
**Alternatives**: What else was considered?  
**Consequences**: Trade-offs and implications  
**References**: Links to detailed docs
```

---

## Decisions

### DD-001: Hardware RAID 3 Compliance for Error Handling
**Date**: 2025-11-02  
**Status**: ✅ Accepted  

**Context**: How should level3 handle unavailable backends? Should writes succeed with 2 of 3 backends (high availability) or require all 3 (strict)?

**Decision**: **Strict writes (all 3 backends required)**, matching hardware RAID 3 behavior:
- Reads: Work with 2 of 3 (degraded mode) ✅
- Writes: Require all 3 (strict mode) ❌
- Deletes: Best effort (idempotent) ✅

**Rationale**:
- Matches hardware RAID 3 controllers (industry standard)
- Prevents creating degraded files from the start
- Avoids performance degradation (reconstruction overhead)
- Ensures data consistency

**Alternatives Considered**:
- **Option B**: Configurable (default strict, optional degraded writes)
- **Option C**: Always allow degraded writes (high availability)

**Consequences**:
- ✅ Data consistency guaranteed
- ✅ No corruption possible
- ❌ Writes fail when backend unavailable (expected RAID behavior)
- ✅ Simple implementation

**References**: `docs/ERROR_HANDLING_POLICY.md`, `docs/DECISION_SUMMARY.md`

---

### DD-002: Pre-flight Health Check for Write Operations
**Date**: 2025-11-02  
**Status**: ✅ Accepted  

**Context**: Discovered that rclone's retry logic could bypass strict write policy, creating degraded/corrupted files on retries.

**Decision**: Add **pre-flight health check** before all write operations:
- `checkAllBackendsAvailable(ctx)` with 5-second timeout
- Tests all 3 backends before Put/Update/Move
- Fails immediately with clear error if any unavailable

**Rationale**:
- Prevents rclone's retry logic from creating partial files
- Critical for preventing Update corruption
- Fast failure (5 seconds) vs hanging (minutes)
- Clear error messages

**Alternatives Considered**:
- Disable retries globally (affects all operations)
- Add rollback logic (complex)
- Trust errgroup alone (insufficient - retries bypass it)

**Consequences**:
- ✅ Prevents all corruption scenarios
- ✅ Fast failure in degraded mode
- ⚠️ +0.2s overhead per write operation (acceptable)
- ✅ Simple to implement

**References**: `docs/STRICT_WRITE_FIX.md`, `docs/FIXES_COMPLETE.md`

---

### DD-003: Timeout Modes for S3/MinIO
**Date**: 2025-11-01  
**Status**: ✅ Accepted  

**Context**: S3 operations hang for 2-5 minutes when backend unavailable due to AWS SDK retry logic.

**Decision**: Add `timeout_mode` option with three presets:
- **standard**: Use global settings (local/file storage)
- **balanced**: 3 retries, 30s timeout (reliable S3)
- **aggressive**: 1 retry, 10s timeout (degraded mode S3) ⭐ Default for MinIO

**Rationale**:
- Reduces failover time from 5 minutes to 6-7 seconds (aggressive)
- User can choose based on their backend type
- Uses `fs.AddConfig()` for local override

**Alternatives Considered**:
- Modify AWS SDK (rejected - too invasive)
- Use different S3 SDK (rejected - architectural change)
- Global timeout override (rejected - affects all backends)

**Consequences**:
- ✅ Fast degraded mode (6-7 seconds)
- ✅ User choice for different scenarios
- ⚠️ Configuration complexity (3 modes to understand)
- ✅ No architectural changes

**References**: `docs/TIMEOUT_MODE_IMPLEMENTATION.md`, `docs/S3_TIMEOUT_RESEARCH.md`

---

### DD-004: Self-Healing with Background Workers
**Date**: 2025-11-01  
**Status**: ✅ Accepted  

**Context**: When reading in degraded mode, should missing particles be restored automatically?

**Decision**: **Solution D (Hybrid Auto-detect)** with background workers:
- Queue missing particles for upload during reads
- Background workers process uploads asynchronously
- Shutdown waits for uploads ONLY if queue non-empty
- Deduplication prevents duplicate uploads

**Rationale**:
- No delay when no healing needed (fast exit)
- Reliable completion when healing needed
- Background workers don't block operations
- Graceful shutdown ensures uploads complete

**Alternatives Considered**:
- **A**: Block until upload complete (slow)
- **B**: Fire-and-forget (unreliable)
- **C**: User-initiated rebuild (manual)

**Consequences**:
- ✅ Automatic healing (no user intervention)
- ✅ Fast when not needed (immediate exit)
- ✅ Reliable when needed (waits for uploads)
- ⚠️ Background workers add complexity
- ✅ Graceful shutdown within 60 seconds

**References**: `docs/SELF_HEALING_IMPLEMENTATION.md`, `docs/SELF_HEALING_RESEARCH.md`

---

### DD-005: Parity Filename Suffixes
**Date**: 2025-10-31  
**Status**: ✅ Accepted  

**Context**: How to distinguish parity files from original files and encode odd/even length information?

**Decision**: Use suffixes:
- `.parity-el` - Even-length original
- `.parity-ol` - Odd-length original

**Rationale**:
- Distinctive suffixes (no collision with real files)
- Encodes essential reconstruction information
- Simple to parse and validate

**Alternatives Considered**:
- `.parity` only (rejected - doesn't encode length)
- `.el` and `.ol` only (rejected - too generic)
- Metadata in separate file (rejected - too complex)

**Consequences**:
- ✅ Clear identification of parity files
- ✅ Reconstruction possible without even/odd particles
- ✅ Simple implementation

**References**: `RAID3.md`

---

### DD-006: Byte-Level Striping (not block-level)
**Date**: 2025-10-31  
**Status**: ✅ Accepted  

**Context**: What granularity for data striping - byte, block, or other?

**Decision**: **Byte-level striping**:
- Even indices (0, 2, 4, ...) → even remote
- Odd indices (1, 3, 5, ...) → odd remote

**Rationale**:
- True RAID 3 behavior (byte-level)
- Simple algorithm (no block alignment)
- Works with any file size
- Even distribution for small files

**Alternatives Considered**:
- Block-level striping (RAID 5 style) - more complex
- Variable-size chunks - unnecessary complexity

**Consequences**:
- ✅ Simple implementation
- ✅ Works with any file size
- ⚠️ Entire file must be in memory (acceptable for most use cases)
- ✅ Perfect 50/50 distribution

**References**: `RAID3.md`

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

**Rationale**:
- Mathematically correct for XOR reconstruction
- Parity size = even size (consistent)
- Enables reconstruction from any 2 particles

**Alternatives Considered**:
- Pad odd data with zero (rejected - changes data)
- Store last byte separately (rejected - too complex)

**Consequences**:
- ✅ Correct reconstruction math
- ✅ Simple implementation
- ✅ Works for all file sizes

**References**: `RAID3.md`

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

**Rationale**:
- Clear purpose for each test
- Helps debugging (know what's broken)
- Improves maintainability
- Self-documenting code

**Alternatives Considered**:
- Minimal comments (rejected - hard to maintain)
- External test documentation only (rejected - comments better)

**Consequences**:
- ✅ Self-documenting tests
- ✅ Easier debugging
- ✅ Better maintainability
- ⚠️ More verbose test files

**References**: `docs/TEST_DOCUMENTATION_PROPOSAL.md`, `TESTS.md`

---

## 🤔 Open Questions

### OQ-001: Backend Help Command Behavior
**Date**: 2025-11-02  
**Status**: 🔴 Open  

**Question**: How should `rclone backend help level3:` behave?

**Options**:
- **A**: Aggregated information (like `union` backend)
- **B**: Per-remote information (like `combine` backend)
- **C**: Level3-specific help (custom)

**Considerations**:
- Union: Shows combined capabilities
- Combine: Shows per-remote details
- Level3: Has 3 remotes with specific roles (even/odd/parity)

**Investigation Needed**:
- Check union backend implementation
- Check combine backend implementation
- Determine what's most useful for users

**Decision Deadline**: None (low priority)

**Owner**: TBD

---

### OQ-002: Streaming Support (Large Files)
**Date**: TBD  
**Status**: 🔴 Open  

**Question**: Should level3 support streaming for large files instead of loading entire file into memory?

**Current**: Loads entire file into memory for splitting

**Pros of streaming**:
- Handle files larger than RAM
- Better memory efficiency
- Scalability

**Cons**:
- More complex implementation
- Hash calculation requires full read anyway
- Parity requires all data

**Decision Needed**: Determine if memory buffering is acceptable for target use cases

---

### OQ-003: Move with Degraded Source Files
**Date**: 2025-11-02  
**Status**: 🟡 Tentative (current behavior documented)  

**Question**: Should Move work when source file has missing particles?

**Current Behavior**: Move succeeds, propagates degraded state to new location

**Options**:
- **A**: Fail (require all source particles) - strict
- **B**: Allow (current) - flexible
- **C**: Reconstruct first, then move - smart but slow

**Current Decision**: Keep Option B (allow)

**Reconsider?**: Only if users report issues

---

## 📝 Decision Process

### When to Create a Decision Record:

✅ **DO create** for:
- Architectural choices with multiple viable options
- Trade-offs between competing concerns (safety vs performance)
- Behaviors that affect users or data integrity
- Anything that might need justification later

❌ **DON'T create** for:
- Obvious implementation details
- Trivial choices with single correct answer
- Internal refactoring with no external impact

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

**Total Decisions Documented**: 8  
**Status**: All accepted and implemented  
**Open Questions**: 3 (low priority)  

This document provides a quick reference for understanding WHY the level3 backend works the way it does.

For detailed implementation notes, see files in `docs/` directory.

