# RAID3 Backend - RAID 3 Implementation

## Purpose of This Document

This document explains **how RAID 3 works in principle**, both for traditional hardware RAID 3 and for the rclone `raid3` backend implementation. It focuses on the **fundamental concepts** of RAID 3 storage:

- **Byte-level striping** (how data is split across backends)
- **XOR parity calculation** (how redundancy is computed)
- **Reconstruction algorithms** (how missing data is recovered)
- **Hardware RAID 3 compliance** (how the implementation matches industry standards)

For **usage instructions**, **configuration options**, **backend commands** (rebuild, heal, status), and **operational details**, see the main [`README.md`](README.md) file.

This document serves as a **technical reference** for understanding the RAID 3 algorithm and how it's implemented in the rclone backend.

---

## Overview

The `raid3` backend implements RAID 3 storage with byte-level striping across three remotes:
- **Even remote**: Stores bytes at even indices (0, 2, 4, 6, ...)
- **Odd remote**: Stores bytes at odd indices (1, 3, 5, 7, ...)
- **Parity remote**: Stores XOR parity of even and odd bytes

## Data Distribution

### Even-Length Files (e.g., 14 bytes)

```
Original: [A, B, C, D, E, F, G, H, I, J, K, L, M, N]
           ↓ Split ↓
Even:     [A, C, E, G, I, K, M]      → 7 bytes
Odd:      [B, D, F, H, J, L, N]      → 7 bytes  
Parity:   [A^B, C^D, E^F, G^H, I^J, K^L, M^N] → 7 bytes (.parity-el)
```

### Odd-Length Files (e.g., 11 bytes)

```
Original: [A, B, C, D, E, F, G, H, I, J, K]
           ↓ Split ↓
Even:     [A, C, E, G, I, K]          → 6 bytes
Odd:      [B, D, F, H, J]             → 5 bytes
Parity:   [A^B, C^D, E^F, G^H, I^J, K] → 6 bytes (.parity-ol)
          └─────── XOR pairs ────┘  └─ last byte (no partner)
```

## Parity Calculation

### XOR Parity Logic

- For each pair: `parity[i] = even[i] XOR odd[i]`
- For odd-length files, last parity byte = last even byte (no XOR partner)
- Parity size always equals even particle size

### Filename Suffixes

Parity files are named with suffixes to indicate original data length:
- `.parity-el` - Even-length original data
- `.parity-ol` - Odd-length original data

This information is essential for correct reconstruction from parity, as the algorithm differs slightly for even-length vs odd-length files.

## Configuration

```ini
[raid3]
type = raid3
even = /path/to/backend1     # Even-indexed bytes
odd = /path/to/backend2      # Odd-indexed bytes
parity = /path/to/backend3   # XOR parity
```

Example with cloud storage:

```ini
[raid3]
type = raid3
even = s3:bucket1/data
odd = gdrive:backup/data
parity = dropbox:parity
```

## Example: "Hello, World!" (14 bytes)

```bash
Original data (hex):
48 65 6c 6c 6f 2c 20 57 6f 72 6c 64 21 0a
H  e  l  l  o  ,     W  o  r  l  d  !  \n

Even (7 bytes):
48 6c 6f 20 6f 6c 21
H  l  o     o  l  !

Odd (7 bytes):
65 6c 2c 57 72 64 0a
e  l  ,  W  r  d  \n

Parity (7 bytes) - saved as "test.txt.parity-el":
2d 00 43 77 1d 08 2b
│  │  │  │  │  │  └─ 0x21 XOR 0x0A = 0x2B
│  │  │  │  │  └──── 0x6C XOR 0x64 = 0x08
│  │  │  │  └─────── 0x6F XOR 0x72 = 0x1D
│  │  │  └────────── 0x20 XOR 0x57 = 0x77
│  │  └───────────── 0x6F XOR 0x2C = 0x43
│  └──────────────── 0x6C XOR 0x6C = 0x00
└─────────────────── 0x48 XOR 0x65 = 0x2D
```

## Verification

Upload a file:
```bash
echo "Hello, World!" > test.txt
rclone copy test.txt raid3:

# Verify particles
ls -l /path/to/backend1/test.txt         # 7 bytes (even)
ls -l /path/to/backend2/test.txt         # 7 bytes (odd)
ls -l /path/to/backend3/test.txt.parity-el  # 7 bytes (parity)

# Download and verify
rclone copy raid3:test.txt ./downloaded.txt
diff test.txt downloaded.txt  # Should be identical
```

## Operations

### Upload
- Splits data into even/odd bytes
- Calculates XOR parity
- Uploads to all three backends in parallel
- Parity file gets appropriate suffix (.parity-el or .parity-ol)

### Download
- **Normal mode** (all 3 backends available):
- Reads even and odd particles
- Validates sizes
- Merges bytes back to original
  - Parity is not needed but can be used for verification
- **Degraded mode** (1 backend missing):
  - Automatically reconstructs missing particle from the other two
  - If even missing: reconstructs from `odd + parity`
  - If odd missing: reconstructs from `even + parity`
  - If parity missing: uses `even + odd` (no reconstruction needed)
  - Transparent to users - operations succeed automatically
  - With `auto_heal=true` (default): queues missing particle for background upload

### Delete
- Removes all three particles (even, odd, parity)
- Searches for both .parity-el and .parity-ol suffixes

### List
- Shows union of files from even and odd backends
- Filters out parity files (hidden from user)
- Shows original (reconstructed) file sizes

## Reconstruction from Parity (Implemented)

The raid3 backend **fully implements** parity reconstruction. When any single backend fails, data can be reconstructed from the remaining two backends.

### Reconstruction Algorithms

**When even particle is lost:**
```
For byte i in reconstructed:
  if i is even:
    data[i] = parity[i/2] XOR odd[i/2]
  else:
    data[i] = odd[i/2]
```

**When odd particle is lost:**
```
For byte i in reconstructed:
  if i is even:
    data[i] = even[i/2]
  else:
    data[i] = parity[i/2] XOR even[i/2]
```

**When parity is lost:**
- No problem - can still reconstruct from even+odd
- Can regenerate parity from even+odd

### Implementation Status

✅ **Automatic reconstruction during reads** (degraded mode):
- Works transparently when accessing files
- Reconstructs missing particles on-the-fly
- No user intervention required

✅ **Rebuild command** (`rclone backend rebuild raid3:`):
- Complete restoration after backend replacement
- Rebuilds all missing particles on a new backend
- Supports check-only mode and dry-run

✅ **Heal command** (`rclone backend heal raid3:`):
- Proactively heals all degraded objects (2/3 particles)
- Reconstructs and uploads missing particles
- Works regardless of `auto_heal` setting

✅ **Auto-heal** (`auto_heal=true` by default):
- Automatically queues missing particles for upload during reads
- Background restoration of degraded files
- Also reconstructs missing directories during `List()` operations

## Benefits of RAID 3

- ✅ **Fault tolerance**: Can rebuild from single backend failure (fully implemented)
- ✅ **Parity storage**: Only ~50% overhead compared to full duplication (200% for full duplication)
- ✅ **Byte-level granularity**: More thorough than block-level RAID
- ✅ **Automatic recovery**: Degraded mode reads work transparently
- ✅ **Backend commands**: Rebuild and heal commands for maintenance

## Current Limitations

- ⚠️ **Memory buffering**: Currently loads entire files into memory during upload/download
  - Limits practical file size to ~500 MiB - 1 GB
  - See `README.md` for details and future streaming support plans
- ⚠️ **Update rollback**: Update operation rollback not working properly when `rollback=true`
  - Put and Move rollback work correctly
  - See `OPEN_QUESTIONS.md` Q1 for details
- ✅ **Move within backend**: Now supported (DirMove implemented)

## Error Handling - RAID 3 Compliance

The raid3 backend implements **hardware RAID 3 error handling**:

### Degraded Mode Behavior

**Hardware RAID 3 Standard**:
- **Reads**: Work in degraded mode (with N-1 drives) ✅
- **Writes**: Blocked in degraded mode (require all drives) ❌
- **Rationale**: Consistency over availability for writes

**raid3 Implementation**:
- **Reads**: Work with 2 of 3 backends (degraded mode) ✅
  - Automatic parity reconstruction for files
  - Heal background uploads for file particles
  - Automatic directory reconstruction when accessing directories (2/3 → 3/3)
  - Transparent to users
- **Writes**: Require all 3 backends (strict mode with health check) ❌
  - Pre-flight health check before Put/Update/Move
  - Fail immediately if any backend unavailable (5-second timeout)
  - Prevents creating partially-written or corrupted files
  - Blocks rclone's retry logic from creating degraded state
  - Clear error: "write blocked in degraded mode (RAID 3 policy)"
- **Deletes**: Best effort (idempotent) ✅
  - Succeed if any backends reachable
  - Ignore "not found" errors
  - Safe for cleanup

### Why Strict Writes?

**Industry Standard**:
- All hardware RAID 3 controllers block writes in degraded mode
- Linux MD RAID default behavior
- Proven approach for 30+ years

**Data Safety**:
- Prevents creating degraded files from the start
- Every file is fully replicated or not created
- No partial states or inconsistencies
- **Prevents corruption from partial updates**

**Performance**:
- Avoids reconstruction overhead for new files
- Heal only for pre-existing degraded files
- Better user experience
- Health check adds minimal overhead (+0.2s)

### Implementation Details

**Health Check Mechanism**:
```go
// Before each write operation (Put, Update, Move)
checkAllBackendsAvailable(ctx) {
    // Test all 3 backends with parallel List() calls
    // 5-second timeout per backend
    // Return error if ANY unavailable
}
```

**Why Health Check is Needed**:
- Prevents rclone's command-level retry logic from creating degraded files
- Detects unavailability BEFORE attempting write
- Fails on first attempt (no retries can bypass)
- Critical for Update operations (prevents corruption)

**Before Fix** (discovered during testing):
```
Attempt 1: Put fails ✅
Retry:     Put succeeds partially ❌
Result:    Degraded file created!
```

**After Fix** (health check):
```
Health Check: Backend unavailable detected ❌
Result:        Write blocked immediately
File:          Original preserved ✅
```

### Error Messages

**When backend unavailable during write** (Put, Update, Move):
```
ERROR: write blocked in degraded mode (RAID 3 policy): odd backend unavailable
```

**User action**: Fix backend, then retry operation

**When backend unavailable during read**:
```
INFO: Reconstructed file.txt from odd+parity (degraded mode)
INFO: Queued even particle for heal
```

**User action**: None - operation succeeds automatically

**When corruption detected** (Update validation):
```
ERROR: update failed: invalid particle sizes (even=11, odd=14) - FILE MAY BE CORRUPTED
```

**User action**: File may need rebuild. Use degraded mode read to reconstruct, then re-upload.

## Testing

Comprehensive testing included:
- ✅ Unit tests for split/merge
- ✅ Unit tests for XOR parity calculation  
- ✅ Unit tests for parity reconstruction logic
- ✅ End-to-end tests with even and odd length files
- ✅ MD5 hash verification
- ✅ Particle size validation
- ✅ Deletion of all three particles
- ✅ Degraded mode read tests (reconstruction from 2/3 particles)
- ✅ Rebuild command tests
- ✅ Heal command tests
- ✅ Auto-heal and auto-cleanup tests
- ✅ Directory reconstruction tests
- ✅ Error handling and rollback tests

---

## Related Documentation

- **`README.md`**: Complete user guide, configuration, commands, and usage examples
- **`TESTING.md`**: Testing strategy and test coverage details
- **`DESIGN_DECISIONS.md`**: Design choices and rationale
- **`OPEN_QUESTIONS.md`**: Known issues and future enhancements

