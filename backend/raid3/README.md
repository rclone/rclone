# RAID3 Backend - RAID 3 Storage

## Purpose of This Document

This document is the **main user guide** for the rclone `raid3` backend. It provides:

- **Complete usage instructions** - How to configure and use the raid3 backend
- **Feature documentation** - All available options, commands, and behaviors
- **Configuration examples** - Step-by-step setup guides
- **Backend commands** - Detailed documentation for `status`, `rebuild`, and `heal` commands
- **Error handling** - How the backend behaves in degraded mode
- **Testing** - Quick test examples and links to comprehensive testing documentation
- **Limitations** - Known issues and workarounds

For **technical details** about how RAID 3 works, see [`RAID3.md`](RAID3.md).  
For **testing documentation**, see [`TESTING.md`](TESTING.md).  
For **design decisions**, see [`DESIGN_DECISIONS.md`](DESIGN_DECISIONS.md).  
For **contributing guidelines**, see [`CONTRIBUTING.md`](CONTRIBUTING.md).

---

## ⚠️ Status: Alpha / Experimental

**This backend is in early development (Alpha stage)** and is provided for testing and evaluation purposes.

**What this means:**
- ✅ Core functionality is implemented and tested
- ✅ Basic operations (Put, Get, Delete, List) work correctly
- ✅ Degraded mode reads and automatic heal work
- ✅ Rebuild and heal commands are functional
- ⚠️ Some features have known limitations (see "Current Limitations" below)
- ⚠️ API and behavior may change in future versions
- ⚠️ **Not recommended for production use with critical data**

**Before testing:**
- Read the "Current Limitations" section below
- Start with small test files (< 100 MiB recommended due to memory buffering)
- Use the provided test setup for a safe testing environment (see "Getting Started with Test Environment" below)
- Report issues and feedback to help improve the backend

**Quick testing setup:** The easiest way to get started is using our integration test setup, which provides a ready-to-use environment with both local filesystem and MinIO (S3) examples. See the ["Getting Started with Test Environment"](#getting-started-with-test-environment) section below.

---

The `raid3` backend implements **RAID 3** storage with byte-level data striping and XOR parity across three remotes. Data is split into even and odd byte indices, with parity calculated to enable future rebuild from single backend failures.

## 🎯 RAID 3 Features

**Current Implementation:**
- ✅ Byte-level striping (even/odd bytes)
- ✅ XOR parity calculation and storage
- ✅ Parity files with length indicators (.parity-el/.parity-ol)
- ✅ All three particles uploaded/deleted in parallel
- ✅ **Automatic parity reconstruction** (degraded mode reads)
- ✅ **Automatic heal** (background particle restoration)

**Storage Efficiency:**
- Uses ~150% storage (50% overhead for parity)
- Better than full duplication (200% storage)
- Enables single-backend failure rebuild

**Degraded Mode (Hardware RAID 3 Compliant):**
- ✅ **Reads** work with ANY 2 of 3 backends available
- ✅ Missing particles are automatically reconstructed from parity
- ✅ **Heal** automatically restores missing particles in background
- ❌ **Writes** require ALL 3 backends available (strict RAID 3 behavior)
- ❌ **Deletes** require ALL 3 backends available (strict RAID 3 behavior)

## 🧹 Auto-Cleanup and Auto-Heal

**By default**, raid3 provides two automatic features for handling degraded states:

- **`auto_cleanup`**: Auto-deletes orphaned items (1/3 particles) when all remotes available, hides them when remotes missing
- **`auto_heal`**: Reconstructs missing items (2/3 particles - can reconstruct)

### Configuration Options

**Default** (recommended):
```bash
rclone config create myremote raid3 \
    even remote1: \
    odd remote2: \
    parity remote3:
# auto_cleanup=true, auto_heal=true, rollback=true (defaults)
```

**Conservative mode** (cleanup only, no auto-reconstruction):
```bash
# Same as default above, but add: auto_heal false
```

**Debugging mode** (no automatic changes):
```bash
# Same as default above, but add: auto_cleanup false auto_heal false
```

**Rollback**: `rollback=true` (default) provides all-or-nothing guarantee. Put/Move rollback fully working; Update rollback has issues (see Known Limitations).

### Behavior Matrix

| auto_cleanup | auto_heal | Behavior |
|--------------|-----------|----------|
| `true` (default) | `true` (default) | Auto-delete orphans (1/3) when all remotes available + Reconstruct degraded (2/3) - **Recommended** |
| `true` | `false` | Auto-delete orphans when all remotes available, but don't auto-reconstruct - **Conservative** |
| `false` | `true` | Show everything + Reconstruct degraded - **Debugging with healing** |
| `false` | `false` | Show everything, no changes - **Raw debugging mode** |

### CleanUp Command

**Important**: `auto_cleanup=true` **auto-deletes** broken objects (objects with only 1/3 particles - cannot be reconstructed) from listings when all 3 remotes are available. When one or more remotes are unavailable, broken objects are **hidden** (not deleted) to prevent data loss. With `auto_cleanup=false`, broken objects are **visible** in listings but cannot be read. Objects with 2/3 particles (degraded but reconstructable) are always shown regardless of `auto_cleanup` setting. To manually delete broken objects, you can run the cleanup command:

```bash
$ rclone cleanup myremote:mybucket
Scanning for broken objects...
Found 5 broken objects (total size: 3.0 KiB)
Cleaned up 5 broken objects (freed 3.0 KiB)
```

**Note**: Broken objects are physically deleted from the remotes when:
- `auto_cleanup=true` and all 3 remotes are available (automatic during `rclone ls` or similar listing operations)
- You explicitly run `rclone cleanup` (requires all 3 remotes available)

**When to use**:
- Cleaning up after backend failures
- Rebuilding from partial write operations
- Periodic maintenance
- Before switching from `auto_cleanup=false` to `auto_cleanup=true`

**What it removes**:
- Objects with only 1 particle (1/3 - cannot be reconstructed)
- Orphaned particles from failed operations
- Does NOT remove valid objects (2/3 or 3/3 particles)

### Backend Commands

Level3 provides several backend-specific commands for management and rebuild:

#### Status Command

Check backend health and get rebuild guidance:

```bash
rclone backend status raid3:
```

Shows:
- Health status of all three backends (even, odd, parity)
- Impact assessment (what operations work)
- Complete rebuild guide for degraded mode
- Step-by-step instructions for backend replacement

#### Rebuild Command

Rebuild all missing particles on a replacement backend after replacing a failed backend:

```bash
# Auto-detect which backend needs rebuild
rclone backend rebuild raid3:

# Or specify the backend
rclone backend rebuild raid3: odd

# Check what needs rebuild without actually rebuilding
rclone backend rebuild raid3: -o check-only=true
```

The rebuild process:
1. Scans for missing particles
2. Reconstructs data from other two backends
3. Uploads restored particles
4. Shows progress and ETA
5. Verifies integrity

**Note**: Rebuild is different from heal - it's a manual, complete restoration after backend replacement.

**Directory Handling:**
- Rebuild command processes files and their parent directories are automatically created during file uploads
- Empty directories that exist on 2/3 backends are automatically reconstructed by auto-heal when accessed (via `List()` operations)
- This ensures complete directory structure is maintained without requiring explicit directory rebuild commands

#### Heal Command

Proactively heal all degraded objects (objects with exactly 2 of 3 particles):

```bash
rclone backend heal raid3:
```

Scans all objects, identifies degraded ones (2/3 particles), reconstructs and uploads missing particles. Works regardless of `auto_heal` setting. For details, see [`docs/SELF_HEALING.md`](docs/SELF_HEALING.md).

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

**Status**: ⚠️ Known Issue

The Update operation rollback mechanism is not working properly when `rollback=true` (default). 

**What works**:
- ✅ Put rollback: Fully functional
- ✅ Move rollback: Fully functional
- ❌ Update rollback: Implementation exists but has issues

**Impact**:
- When Update operations fail with `rollback=true`, particles may not be properly restored from temporary locations
- This can lead to degraded files (missing particles) in some failure scenarios
- Affects mainly backends that don't support server-side Move operations (e.g., S3/MinIO that only support Copy)

**Workaround**:
- Use `rollback=false` for Update operations if you need to work around this limitation
- Or ensure all three underlying backends support server-side Move operations
- Monitor Update operations and manually fix degraded files if needed

**See also**: 
- `backend/raid3/OPEN_QUESTIONS.md` - Q1: Update Rollback Not Working Properly (active issue with detailed analysis)

---

### File Size Limitation

**IMPORTANT**: Level3 currently loads entire files into memory during upload/download operations.

**File Size Guidelines:**
- **Recommended**: Files up to **500 MiB**
- **Maximum**: ~1-2 GB (depends on available system RAM)
- **Memory Usage**: Approximately **3× file size** during operations
  - Example: 1 GB file requires ~3 GB RAM
  - Example: 100 MiB file requires ~300 MiB RAM

**Why This Limitation Exists:**
- Byte-level striping requires processing entire file
- XOR parity calculation needs all data
- Current implementation uses `io.ReadAll()` for simplicity

**Future Plans**:
- Streaming implementation with chunked processing (planned)
- Will support unlimited file sizes with constant memory (~32 MiB)
- Similar to S3's multipart upload approach

**Workarounds for Very Large Files (>1 GB):**
1. Use S3 backend directly (supports multipart uploads)
2. Use Union backend with multiple remotes
3. Split large files before uploading
4. Wait for raid3 streaming implementation

**Related**: See `docs/LARGE_FILE_ANALYSIS.md` for technical details.

## How It Works

### Data Splitting with Parity

When you upload a file, it is split at the byte level with XOR parity:
- **Even-indexed bytes** (0, 2, 4, 6, ...) go to the even remote
- **Odd-indexed bytes** (1, 3, 5, 7, ...) go to the odd remote
- **XOR parity** (even[i] XOR odd[i]) goes to the parity remote

**Example (7 bytes):**
```
Original data: [A, B, C, D, E, F, G]
               ↓  split with parity  ↓
Even remote:   [A, C, E, G]           (4 bytes)
Odd remote:    [B, D, F]              (3 bytes)
Parity remote: [A^B, C^D, E^F, G]    (4 bytes, saved as "file.parity-ol")
                └── XOR pairs ──┘ └─ last byte (no partner)
```

**Example (6 bytes):**
```
Original data: [A, B, C, D, E, F]
               ↓  split with parity  ↓
Even remote:   [A, C, E]              (3 bytes)
Odd remote:    [B, D, F]              (3 bytes)
Parity remote: [A^B, C^D, E^F]       (3 bytes, saved as "file.parity-el")
```

### Size Relationships

For a file of N bytes:
- Even particle (remote1): `ceil(N/2)` = `(N+1)/2` bytes
- Odd particle (remote2): `floor(N/2)` = `N/2` bytes  
- Even particle size = Odd particle size OR Odd particle size + 1

### Data Reconstruction

When downloading, the backend:
1. Retrieves both particles from both remotes
2. Validates that particle sizes are correct
3. Merges even and odd bytes back into the original data
4. Returns the reconstructed file

### Degraded Mode & Heal

When one particle is missing (2/3 present), reads automatically reconstruct from the other two particles. With `auto_heal=true` (default), missing particles are queued for background upload. Directories missing on one backend are automatically created during `List()` operations.

**Performance**: Normal reads 6-7s, degraded reads with heal 9-10s.

For details, see [`docs/SELF_HEALING.md`](docs/SELF_HEALING.md).

## Configuration

Use `rclone config` to set up a raid3 remote:

```
rclone config
n) New remote
name> myraid3
Type of storage> raid3
remote> /path/to/first/backend
remote2> /path/to/second/backend
```

Or edit the config file directly:

```ini
[myraid3]
type = raid3
even = /path/to/backend1       # Even-indexed bytes
odd = /path/to/backend2        # Odd-indexed bytes
parity = /path/to/backend3     # XOR parity
```

You can also use different cloud storage backends:

```ini
[myraid3]
type = raid3
even = s3:bucket1/data
odd = gdrive:backup/data
parity = dropbox:parity
```

## Usage Examples

### Upload (splits data):
```bash
rclone copy /local/files myraid3:
```
This splits each file and writes even bytes to remote1, odd bytes to remote2.

### Download (reconstructs data):
```bash
rclone copy myraid3: /local/destination
```
This reads both particles and reconstructs the original files.

### List files:
```bash
rclone ls myraid3:
```
Shows files with their original (reconstructed) sizes.

### Create directory:
```bash
rclone mkdir myraid3:newdir
```
Creates the directory in both remotes.

## Particle Validation

The backend performs integrity checks: size validation (ensures even particle size equals odd size or is one byte larger), existence checks (verifies both particles exist before attempting reconstruction), and clear error reporting if particles are missing or invalid.

### Limitations

#### Outdated Documentation Note
The following sections may contain outdated information about missing features. Parity is now implemented and single backend failure rebuild is supported through degraded mode reads and heal.

#### Memory Usage
- Files are buffered entirely in memory during upload/download
- Not suitable for very large files with limited RAM
- Consider file size vs. available memory

#### Moving Files Within Same Level3 Backend
Due to rclone's overlap detection, you cannot use `rclone move` within the same raid3 backend:

```bash
# This will NOT work:
rclone move myraid3:/file.txt myraid3:/subfolder/
# Error: can't sync or move files on overlapping remotes
```

**Workarounds:**
1. Move particles on each backend separately
2. Copy then delete
3. Use a temporary local directory

#### File-Level Operations (copyto, moveto)
Due to rclone's virtual backend architecture and cache behavior, single-file commands like `copyto` may create a directory instead of a file:

```bash
# This may create a directory with the file inside instead of the file itself:
rclone copyto myraid3:renamed_notes.txt notes_merged.txt
# Result: notes_merged.txt/ containing renamed_notes.txt (wrong!)

# Or may copy all files from the directory instead of just one file
```

**Recommended workaround - Use `cat` for single files:**
```bash
# Use cat instead - it works perfectly for single-file download
rclone cat myraid3:renamed_notes.txt > notes_merged.txt
# ✓ Creates notes_merged.txt as a file with the correct merged content
```

**Example:**
```bash
# Original file
echo "Hello, World!" > /tmp/raid3-test/source/test.txt

# Upload (splits data)
rclone copy /tmp/raid3-test/source/test.txt myraid3:

# Download single file using cat
rclone cat myraid3:test.txt > notes_merged.txt
# ✓ Creates a file called "notes_merged.txt" with merged content
```

## Error Handling (Hardware RAID 3 Compliant)

The raid3 backend follows **hardware RAID 3 behavior** for error handling:

| Operation | Degraded Mode | Policy |
|-----------|---------------|--------|
| **Read** | ✅ Supported | Works with ANY 2 of 3 backends (automatic reconstruction) |
| **Write** (Put, Update, Move) | ❌ Blocked | Requires ALL 3 backends (strict enforcement with pre-flight health check) |
| **Delete** | ✅ Supported | Best effort (idempotent, succeeds if any backend reachable) |

**Key Features**:
- **Pre-flight health check**: Tests all 3 backends before writes (5-second timeout, +0.2s overhead)
- **Automatic rollback**: When `rollback=true` (default), failed write operations are automatically rolled back (all-or-nothing guarantee)
- **Clear error messages**: `"write blocked in degraded mode (RAID 3 policy)"` when backend unavailable

**Rollback Status**:
- ✅ Put rollback: Fully working
- ✅ Move rollback: Fully working
- ⚠️ Update rollback: Not working properly (see Known Limitations)

For detailed error handling policy, rationale, and implementation details, see [`docs/STRICT_WRITE.md`](docs/STRICT_WRITE.md).

## Getting Started with Test Environment

The easiest way to explore the raid3 backend is using our integration test setup, which creates a complete working environment with pre-configured remotes. This setup is safe for experimentation and provides both local filesystem and MinIO (S3) examples.

### Step 1: Set Up the Test Environment

```bash
# Navigate to the rclone source directory containing the experimental raid3 backend
# This should be the directory where you have the rclone source with the raid3 backend
cd /path/to/rclone/with/raid3/backend

# Run the setup script (creates test directories and config file)
./backend/raid3/integration/setup.sh

# The setup script will:
# - Create working directory: ${HOME}/go/raid3storage (or custom path with --workdir)
# - Generate rclone config: ${WORKDIR}/rclone_raid3_integration_tests.config
# - Create all required subdirectories for local and MinIO storage
```

### Step 2: Navigate to the Test Directory

```bash
# Change to the working directory
cd $(cat ${HOME}/.rclone_raid3_integration_tests.workdir)

# The config file is automatically used by test scripts
# For manual use, specify it with --config
CONFIG="${PWD}/rclone_raid3_integration_tests.config"
```

### Step 3: Explore the Test Remotes

The test setup provides these remotes (in the generated config file):

**Local filesystem remotes:**
- `localraid3` - RAID3 backend using local directories
- `localsingle` - Single local backend (for comparison)

**MinIO (S3) remotes:**
- `minioraid3` - RAID3 backend using MinIO containers
- `miniosingle` - Single MinIO backend (for comparison)

### Step 4: Basic Operations with Test Setup

#### Using Local Filesystem Backend

```bash
# Set config path (e.g., /home/alice/go/raid3storage/rclone_raid3_integration_tests.config)
CONFIG="${PWD}/rclone_raid3_integration_tests.config"

# Create a test file
echo "Hello, RAID3!" > test.txt

# Upload to raid3 backend
rclone --config "${CONFIG}" copy test.txt localraid3:

# List files (parity files are hidden)
rclone --config "${CONFIG}" ls localraid3:

# Download and verify
rclone --config "${CONFIG}" copy localraid3:test.txt downloaded.txt
diff test.txt downloaded.txt  # Should be identical

# Inspect the particles (even, odd, parity)
ls -lh ${HOME}/go/raid3storage/even_local/
ls -lh ${HOME}/go/raid3storage/odd_local/
ls -lh ${HOME}/go/raid3storage/parity_local/  # Note .parity-el suffix
```

#### Using MinIO (S3) Backend

```bash
# Start MinIO containers (if not already running)
./backend/raid3/integration/compare_raid3_with_single.sh --storage-type minio start

# Upload to MinIO-based raid3
rclone --config "${CONFIG}" copy test.txt minioraid3:

# List and verify
rclone --config "${CONFIG}" ls minioraid3:
rclone --config "${CONFIG}" copy minioraid3:test.txt downloaded_minio.txt
diff test.txt downloaded_minio.txt
```

### Step 5: Explore Degraded Mode

```bash
# Simulate a backend failure by removing one particle
rm ${HOME}/go/raid3storage/even_local/test.txt

# Read still works! (reconstructs from odd + parity)
rclone --config "${CONFIG}" cat localraid3:test.txt

# With auto_heal enabled (default), the missing particle is queued for upload
# Check that it was restored
ls -lh ${HOME}/go/raid3storage/even_local/test.txt  # Should exist again
```

### Step 6: Test Backend Commands

```bash
# Check backend health
rclone --config "${CONFIG}" backend status localraid3:

# Heal all degraded objects
rclone --config "${CONFIG}" backend heal localraid3:

# Simulate rebuild scenario (remove all particles from one backend)
rm -rf ${HOME}/go/raid3storage/even_local/*
rclone --config "${CONFIG}" backend rebuild localraid3: even
```

### Step 7: Run Integration Tests

The test setup also includes comprehensive integration tests:

```bash
# List available tests
./backend/raid3/integration/compare_raid3_with_single.sh list

# Run a specific test
./backend/raid3/integration/compare_raid3_with_single.sh --storage-type local test mkdir

# Run heal tests
./backend/raid3/integration/compare_raid3_with_single_heal.sh --storage-type local test even

# Run rebuild tests
./backend/raid3/integration/compare_raid3_with_single_rebuild.sh --storage-type local test odd
```

### Test Environment Details

**Configuration file location:**
- `${WORKDIR}/rclone_raid3_integration_tests.config`
- Where `WORKDIR` defaults to `${HOME}/go/raid3storage`

**Local storage directories:**
- Even particles: `${WORKDIR}/even_local/`
- Odd particles: `${WORKDIR}/odd_local/`
- Parity particles: `${WORKDIR}/parity_local/`
- Single backend: `${WORKDIR}/single_local/`

**MinIO containers:**
- Automatically started/stopped by test scripts
- Ports: 9001 (even), 9002 (odd), 9003 (parity), 9004 (single)
- Data directories: `${WORKDIR}/even_minio/`, etc.

**For more details:** See [`backend/raid3/integration/README.md`](integration/README.md) for complete documentation.

### Additional Examples

#### Example: Verify Byte-Level Striping

```bash
# Create a file with known content
echo -n "ABCDEFGH" > test_stripe.txt  # 8 bytes

# Upload
rclone --config "${CONFIG}" copy test_stripe.txt localraid3:

# Check even bytes (A, C, E, G)
hexdump -C ${HOME}/go/raid3storage/even_local/test_stripe.txt
# Should show: 41 43 45 47 (A, C, E, G in hex)

# Check odd bytes (B, D, F, H)
hexdump -C ${HOME}/go/raid3storage/odd_local/test_stripe.txt
# Should show: 42 44 46 48 (B, D, F, H in hex)

# Check parity (XOR of pairs)
hexdump -C ${HOME}/go/raid3storage/parity_local/test_stripe.txt.parity-el
# Should show: 03 03 03 03 (A^B, C^D, E^F, G^H)
```

#### Example: Test Degraded Mode Reconstruction

```bash
# Upload a file
echo "Important data" > important.txt
rclone --config "${CONFIG}" copy important.txt localraid3:

# Simulate even backend failure
rm ${HOME}/go/raid3storage/even_local/important.txt

# Read still works (reconstructs from odd + parity)
rclone --config "${CONFIG}" cat localraid3:important.txt
# Output: Important data

# Verify reconstruction worked
rclone --config "${CONFIG}" copy localraid3:important.txt reconstructed.txt
diff important.txt reconstructed.txt  # Should be identical
```

For more examples, see [`integration/README.md`](integration/README.md).

---

## Testing

For comprehensive testing documentation, see [`TESTING.md`](TESTING.md).

### Integration Test Scripts

The `backend/raid3/integration/` directory contains comprehensive Bash-based integration test scripts for validating raid3 backend functionality. These scripts supplement the Go-based unit and integration tests with black-box testing scenarios.

**📚 For complete documentation, see [`backend/raid3/integration/README.md`](integration/README.md)**

**Quick Overview**:

- **`setup.sh`** - Initial setup script to create test environment and configuration
- **`compare_raid3_with_single.sh`** - Black-box comparison tests
- **`compare_raid3_with_single_rebuild.sh`** - Rebuild command validation
- **`compare_raid3_with_single_heal.sh`** - Auto-heal functionality tests
- **`compare_raid3_with_single_errors.sh`** - Error handling and rollback tests

**Platform Compatibility**: These scripts are Bash-based and work on Linux, macOS, WSL, Git Bash, and Cygwin. They will **not work natively on Windows** (cmd.exe or PowerShell).

**Quick Start**:

```bash
# 1. Initial setup (one-time)
./backend/raid3/integration/setup.sh

# 2. Navigate to work directory
cd $(cat ${HOME}/.rclone_raid3_integration_tests.workdir)

# 3. Run tests
./backend/raid3/integration/compare_raid3_with_single.sh --storage-type local test mkdir
```

For detailed setup instructions, configuration options, troubleshooting, and complete usage examples, see the [Integration Tests README](integration/README.md).

## Implementation Notes

The raid3 backend:
- Uses `errgroup` for parallel operations
- Implements byte-level splitting/merging functions
- Validates particle sizes on every read
- Computes hashes on reconstructed data
- Supports all standard fs operations (Put, Get, Update, Remove, etc.)

## Comparison with Duplicate Backend

| Feature | Duplicate | RAID3 (RAID 3) |
|---------|-----------|-----------------|
| **Number of backends** | 2 | 3 |
| **Data redundancy** | ✅ Full (identical copies) | ✅ With parity (XOR) |
| **Storage efficiency** | 50% (2x storage) | ~67% (1.5x storage) |
| **Single backend failure** | ✅ Still works | ✅ Degraded mode reads + heal |
| **Current status** | ✅ Fully redundant | ✅ Parity implemented |
| **Use case** | Backup, redundancy | Efficient fault-tolerance |
| **Read from** | Either backend | Any 2 of 3 (degraded mode) |
| **Write to** | Both (identical) | All 3 required (strict RAID 3) |
| **Parity** | ❌ None | ✅ XOR parity |
| **Single backend failure** | ✅ Still works | ✅ Degraded mode reads + heal |
| **Backend replacement** | ✅ Manual copy | ✅ Rebuild command available |

