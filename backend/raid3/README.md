# RAID3 Backend - RAID 3 Storage

This document is the main user guide for the rclone `raid3` backend, providing usage instructions, configuration examples, backend commands (`status`, `rebuild`, `heal`), error handling details, and known limitations. For technical RAID 3 details, testing documentation, and design decisions, see [`docs/RAID3.md`](docs/RAID3.md), [`docs/TESTING.md`](docs/TESTING.md), and [`_analysis/DESIGN_DECISIONS.md`](_analysis/DESIGN_DECISIONS.md). For the improvement plan and task tracking, see [`_analysis/IMPROVEMENT_PLAN.md`](_analysis/IMPROVEMENT_PLAN.md).

---

## ⚠️ Status: Alpha / Experimental

This backend is in early development (Alpha stage) for testing and evaluation. Core operations (Put, Get, Delete, List) are implemented and tested. Degraded mode reads, automatic heal, and rebuild of failing remotes work. The easiest way to get started is using the integration test setup (see ["Getting Started with Test Environment"](#getting-started-with-test-environment) below), which provides a ready-to-use environment with local filesystem and MinIO (S3) examples. Before testing, read the "Current Limitations" section and start with small test files.

The `raid3` backend implements RAID 3 storage with byte-level data striping and XOR parity across three remotes. Data is split into even and odd byte indices, with parity calculated to enable rebuild from single backend failures.

## 🎯 RAID 3 Features

The backend implements byte-level striping (even/odd bytes), XOR parity calculation and storage with length indicators (.parity-el/.parity-ol), parallel upload/delete of all three particles, automatic parity reconstruction during degraded mode reads, and automatic heal with background particle restoration. Degraded mode (hardware RAID 3 compliant): reads work with ANY 2 of 3 backends available (missing particles automatically reconstructed and restored in background), while writes and deletes require ALL 3 backends available (strict RAID 3 behavior). Storage efficiency: enables single-backend failure rebuild using ~150% storage (50% overhead for parity).

## 🧹 Auto-Cleanup and Auto-Heal

By default, raid3 provides two automatic features: `auto_cleanup` auto-deletes orphaned items (1/3 particles) when all remotes available, and `auto_heal` reconstructs missing items (2/3 particles). Configuration: default (`auto_cleanup=true`, `auto_heal=true`) auto-deletes orphans and reconstructs degraded (recommended); conservative mode (`auto_cleanup=true`, `auto_heal=false`) auto-deletes orphans but doesn't auto-reconstruct; debugging mode (`auto_cleanup=false`, `auto_heal=false`) shows everything with no automatic changes. `rollback=true` (default) provides all-or-nothing guarantee; Put/Move rollback fully working, Update rollback has issues (see Known Limitations).

```bash
rclone config create myremote raid3 \
    even remote1: \
    odd remote2: \
    parity remote3:
# auto_cleanup=true, auto_heal=true, rollback=true (defaults)
```



### CleanUp Command

When `auto_cleanup=true` (default), broken objects (1/3 particles) are automatically deleted from listings when all 3 remotes are available, or hidden when remotes are missing. With `auto_cleanup=false`, broken objects are visible but cannot be read. Objects with 2/3 particles are always shown. To manually delete broken objects, run `rclone cleanup myremote:mybucket` (requires all 3 remotes available).

### Backend Commands

The raid3 backend provides three management commands:

**Status** (`rclone backend status raid3:`): Shows health status of all three backends and impact assessment (what operations work). When in degraded mode, provides complete rebuild guidance with step-by-step instructions for backend replacement.

**Rebuild** (`rclone backend rebuild raid3:` or `rclone backend rebuild raid3: odd`): Rebuilds all missing particles on a replacement backend after replacing a failed backend. The process scans for missing particles, reconstructs data from the other two backends, uploads restored particles with progress/ETA, and verifies integrity. Parent directories are automatically created during file uploads; empty directories are reconstructed by auto-heal when accessed. Use `-o check-only=true` to check what needs rebuild without actually rebuilding.

**Heal** (`rclone backend heal raid3:`): Proactively heals all degraded objects (2/3 particles) by scanning, identifying, reconstructing, and uploading missing particles. Works regardless of `auto_heal` setting. For details, see [`docs/CLEAN_HEAL.md`](docs/CLEAN_HEAL.md).

### Object and Directory States

| Particles/Backends | State | auto_cleanup | auto_heal | Behavior |
|-------------------|-------|--------------|-----------|----------|
| **3/3** | Healthy | N/A | N/A | Normal operations |
| **2/3** | Degraded | N/A | ✅ Enabled | Reconstruct missing particle/directory |
| **2/3** | Degraded | N/A | ❌ Disabled | No automatic reconstruction (use `heal` command or `rebuild`) |
| **1/3** | Orphaned | ✅ Enabled | N/A | Hide from listings, delete if accessed |
| **1/3** | Orphaned | ❌ Disabled | N/A | Show in listings, operations may fail |

**Terminology**: **Degraded** (2/3) = missing 1 particle, can reconstruct ✅. **Orphaned** (1/3) = missing 2 particles, cannot reconstruct ❌.

## ⚠️ Current Limitations

### Update Rollback Not Working Properly

Update operation rollback has issues when `rollback=true` (default); Put and Move rollback work correctly. Failed Update operations may not properly restore particles from temporary locations, leading to degraded files, mainly affecting backends without server-side Move support (e.g., S3/MinIO). Workarounds: use `rollback=false` for Update operations, ensure all backends support server-side Move, or manually fix degraded files. See [`docs/OPEN_QUESTIONS.md`](docs/OPEN_QUESTIONS.md) Q1 for details.

### File Size Limitation

With `use_streaming=true` (default), the backend uses a pipelined chunked approach that processes files in 2MB chunks, enabling efficient handling of large files without loading entire files into memory. Memory usage is bounded (~5MB for double buffering). For very large files (>10GB), performance may be slightly slower than concurrent uploads, but the implementation is more reliable and simpler. When `use_streaming=false`, files are loaded entirely into memory (legacy mode), limiting practical file size to ~500 MiB - 1 GB depending on available RAM.

## How It Works

When uploading a file, it is split at the byte level with XOR parity: even-indexed bytes (0, 2, 4, 6, ...) go to the even remote, odd-indexed bytes (1, 3, 5, 7, ...) go to the odd remote, and XOR parity (even[i] XOR odd[i]) goes to the parity remote. For a file of N bytes, the even particle contains `ceil(N/2)` bytes and the odd particle contains `floor(N/2)` bytes, so the even particle size equals the odd size or is one byte larger.

**Streaming Processing** (default, `use_streaming=true`): Files are processed in 2MB chunks using a pipelined approach. The backend reads chunks, splits them into even/odd/parity particles, and uploads them sequentially while reading the next chunk in parallel. This provides bounded memory usage (~5MB) and enables handling of very large files efficiently.

**Buffered Processing** (`use_streaming=false`): Legacy mode that loads entire files into memory before processing. Use only for small files or when streaming is not available.

When downloading, the backend retrieves both particles, validates particle sizes are correct, merges even and odd bytes back into the original data, and returns the reconstructed file. When one particle is missing (2/3 present), reads automatically reconstruct from the other two particles. With `auto_heal=true` (default), missing particles are queued for background upload and directories missing on one backend are automatically created during `List()` operations. Performance: normal reads 6-7s, degraded reads with heal 9-10s. For details, see [`docs/CLEAN_HEAL.md`](docs/CLEAN_HEAL.md).

## Configuration

Configure a raid3 remote using `rclone config` or by editing the config file directly:

```ini
[myraid3]
type = raid3
even = /path/to/backend1       # Even-indexed bytes (or s3:bucket1/data)
odd = /path/to/backend2        # Odd-indexed bytes (or gdrive:backup/data)
parity = /path/to/backend3     # XOR parity (or dropbox:parity)
```

All three remotes can use different storage backends (local filesystem, S3, Google Drive, Dropbox, etc.).

## Feature Handling with Mixed Remotes

When using different remote types (e.g., mixing object storage like S3 with file storage like local filesystem), raid3 automatically intersects features from all three backends. Most features require **all backends** to support them (AND logic), ensuring compatibility across the union. However, raid3 uses **best-effort** logic for metadata features (OR logic), allowing metadata operations to work if **any backend** supports them, since operations check per-backend support before calling.

**Features requiring all backends** (AND logic):
- `BucketBased`, `SetTier`, `GetTier`, `ServerSideAcrossConfigs`, `PartialUploads`
- `Copy`, `Move`, `DirMove` operations
- `ReadMimeType`, `WriteMimeType`, `CanHaveEmptyDirectories`

**Features using best-effort** (OR logic, raid3-specific):
- `ReadMetadata`, `WriteMetadata`, `UserMetadata` (object metadata)
- `ReadDirMetadata`, `WriteDirMetadata`, `UserDirMetadata` (directory metadata)
- `DirSetModTime`, `MkdirMetadata` (directory operations)
- `CaseInsensitive` (more permissive: any case-insensitive backend makes the whole union case-insensitive)

**Always available** (raid3 implements independently):
- `Shutdown` (waits for heal uploads to complete)
- `CleanUp` (removes broken objects with 1/3 particles)

**Example**: Mixing S3 (object storage) with local filesystem will disable `BucketBased` and tier features (`SetTier`, `GetTier`) since local filesystem doesn't support them, but metadata features will work if either backend supports them.

## Usage Examples

Standard rclone commands work with raid3: `rclone copy /local/files myraid3:` splits files and uploads particles, `rclone copy myraid3: /local/destination` reconstructs files from particles, `rclone ls myraid3:` shows files with original sizes, and `rclone mkdir myraid3:newdir` creates directories on all remotes.

## Particle Validation

The backend performs integrity checks including size validation (ensures even particle size equals odd size or is one byte larger), existence checks (verifies both particles exist before attempting reconstruction), and clear error reporting if particles are missing or invalid.

### Limitations

Due to rclone's virtual backend architecture and cache behavior, `copyto` for single-file downloads (remote → local) may create a directory instead of a file. For single-file downloads, use `rclone cat myraid3:file.txt > output.txt` instead. For single-file uploads (local → remote), `copyto` works correctly: `rclone copyto localfile.txt myraid3:file.txt`. See [`docs/OPEN_QUESTIONS.md`](docs/OPEN_QUESTIONS.md) for details on this current limitation.

## Error Handling (Hardware RAID 3 Compliant)

The raid3 backend follows hardware RAID 3 behavior: reads work with ANY 2 of 3 backends (automatic reconstruction), writes (Put, Update, Move) require ALL 3 backends with strict enforcement via pre-flight health check (5-second timeout, +0.2s overhead), and deletes use best effort (idempotent, succeeds if any backend reachable). When `rollback=true` (default), failed write operations are automatically rolled back (all-or-nothing guarantee). Put and Move rollback are fully working; Update rollback has issues (see Known Limitations). For detailed error handling policy, rationale, and implementation details, see [`docs/STRICT_WRITE_POLICY.md`](docs/STRICT_WRITE_POLICY.md).

## Getting Started with Test Environment

The easiest way to explore the raid3 backend is using the integration test setup, which creates a complete working environment with pre-configured remotes (both local filesystem and MinIO/S3 examples). Run `cd backend/raid3/test && ./setup.sh` to create the test data directories and generate the config file. The config file will be created at `backend/raid3/test/rclone_raid3_integration_tests.config`.

The setup provides `localraid3` and `minioraid3` remotes for testing. Basic operations work as expected: upload splits files, download reconstructs them, and degraded mode can be tested by removing particles. Backend commands (`status`, `heal`, `rebuild`) can be tested using these remotes. For MinIO, start containers with `./compare_raid3_with_single.sh --storage-type minio start`. Integration tests are available via the scripts in `test/` directory. See [`test/README.md`](test/README.md) for complete documentation.

---

## Testing

For comprehensive testing documentation, see [`docs/TESTING.md`](docs/TESTING.md). The `test/` directory contains Bash-based integration test scripts (`setup.sh`, `compare_raid3_with_single.sh`, `compare_raid3_with_single_rebuild.sh`, `compare_raid3_with_single_heal.sh`, `compare_raid3_with_single_errors.sh`, `compare_raid3_with_single_all.sh`) that supplement Go-based unit and integration tests with black-box testing scenarios. These scripts work on Linux, macOS, WSL, Git Bash, and Cygwin (not natively on Windows). See [`test/README.md`](test/README.md) for complete documentation. The `compare_raid3_with_single_all.sh` script runs all test suites across all RAID3 backends with minimal output (pass/fail only).

## Implementation Notes

The raid3 backend uses a pipelined chunked approach for streaming uploads (default), implements byte-level splitting/merging functions, validates particle sizes on every read, computes hashes on reconstructed data, supports all standard fs operations (Put, Get, Update, Remove, etc.), and uses `errgroup` for parallel operations where appropriate. The streaming implementation processes files in 2MB chunks, providing bounded memory usage and efficient handling of large files.

**Code Organization**: The backend is organized into dedicated files for clarity:
- `particles.go` - Core particle operations (SplitBytes, MergeBytes, CalculateParity, reconstruction functions)
- `streammerger.go` - StreamMerger type for merging even and odd particle streams during reads
- `streamsplitter.go` - StreamSplitter type for splitting input streams into even, odd, and parity particles during writes
- `raid3.go` - Main Fs implementation and filesystem operations
- `object.go` - Object implementation and operations


