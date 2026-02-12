# RAID3 Backend - RAID 3 Implementation

## Purpose of This Document

This document serves as a **technical reference** and explains **how RAID 3 works in principle**, both for traditional hardware RAID 3 and for the rclone `raid3` backend implementation. It focuses on the **fundamental concepts** of RAID 3 storage:

**Byte-level striping** (how data is split across backends), **XOR parity calculation** (how redundancy is computed), **Reconstruction algorithms** (how missing data is recovered) and **Hardware RAID 3 compliance** (how the implementation matches standards).

For **usage instructions**, **configuration options**, **backend commands** (rebuild, heal, status), and **operational details**, see the main [`../README.md`](../README.md) file.


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
- **Streaming mode:** Processes files in configurable chunks (default 2MB) using a pipelined approach. Reads chunks, splits into even/odd/parity particles, and uploads while reading the next chunk in parallel. Provides bounded memory usage (~5MB).
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

**When parity is lost:** No problem, can still reconstruct from even+odd.

### Implementation Status

Automatic reconstruction during reads (degraded mode) works transparently when accessing files, reconstructs missing particles on-the-fly, and requires no user intervention. The rebuild command (`rclone backend rebuild raid3:`) provides complete restoration after backend replacement, rebuilds all missing particles on a new backend, and supports check-only mode and dry-run. The heal command (`rclone backend heal raid3:`) proactively heals all degraded objects (2/3 particles), reconstructs and uploads missing particles, and works regardless of `auto_heal` setting. Auto-heal (`auto_heal=true` by default) automatically queues missing particles for upload during reads, provides background restoration of degraded files, and also reconstructs missing directories during `List()` operations.

## Benefits of RAID 3

RAID 3 provides fault tolerance by enabling rebuild from single backend failure (fully implemented), parity storage with only ~50% overhead compared to full duplication (200% for full duplication), byte-level granularity that is more thorough than block-level RAID, automatic recovery with degraded mode reads working transparently, and backend commands (rebuild and heal) for maintenance.

## Current Limitations

Files are processed in configurable chunks (default 2MB) using a pipelined approach, providing bounded memory usage (~5MB) and enabling efficient handling of very large files. Update rollback may have limitations when `rollback=true` (Put and Move rollback work correctly; see [`OPEN_QUESTIONS.md`](OPEN_QUESTIONS.md) Q1 for details). Move within backend is supported (DirMove implemented).

## Error Handling - RAID 3 Compliance

The raid3 backend implements **hardware RAID 3 error handling**:

### Degraded Mode Behavior

Hardware RAID 3 standard: reads work in degraded mode (with N-1 drives), writes are blocked in degraded mode (require all drives), with rationale being consistency over availability for writes. The raid3 implementation: reads work with 2 of 3 backends (degraded mode) with automatic parity reconstruction for files, heal background uploads for file particles, automatic directory reconstruction when accessing directories (2/3 → 3/3), and are transparent to users. Writes require all 3 backends (strict mode with health check) with pre-flight health check before Put/Update/Move, fail immediately if any backend unavailable (5-second timeout), prevent creating partially-written or corrupted files, block rclone's retry logic from creating degraded state, and provide clear error "write blocked in degraded mode (RAID 3 policy)". Deletes use best effort (idempotent), succeed if any backends reachable, ignore "not found" errors, and are safe for cleanup.

### Why Strict Writes?

Strict writes match the industry standard: all hardware RAID 3 controllers block writes in degraded mode, matching Linux MD RAID default behavior, and this is a proven approach used for 30+ years. Data safety: prevents creating degraded files from the start, ensures every file is fully replicated or not created, eliminates partial states or inconsistencies, and prevents corruption from partial updates. Performance: avoids reconstruction overhead for new files, heal only for pre-existing degraded files, provides better user experience, and health check adds minimal overhead (+0.2s).

---

## Related Documentation

- **`README.md`**: Complete user guide, configuration, commands, and usage examples
- **`TESTING.md`**: Testing strategy and test coverage details
- **`_analysis/DESIGN_DECISIONS.md`**: Design choices and rationale
- **`OPEN_QUESTIONS.md`**: Known issues and future enhancements

