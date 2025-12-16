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
- ✅ **Deletes** work with ANY backends available (idempotent)

## 🧹 Auto-Cleanup and Auto-Heal

**By default**, raid3 provides two automatic features for handling degraded states:

- **`auto_cleanup`**: Hides/deletes orphaned items (1/3 particles - cannot reconstruct)
- **`auto_heal`**: Reconstructs missing items (2/3 particles - can reconstruct)

### Configuration Options

**Default behavior** (recommended - full automation):
```bash
rclone config create myremote raid3 \
    even remote1: \
    odd remote2: \
    parity remote3:
# auto_cleanup defaults to true
# auto_heal defaults to true
# rollback defaults to true (all-or-nothing guarantee)
```

**Conservative mode** (cleanup only, no auto-reconstruction):
```bash
rclone config create myremote raid3 \
    even remote1: \
    odd remote2: \
    parity remote3: \
    auto_cleanup true \
    auto_heal false
# Hides broken items but doesn't auto-reconstruct missing particles
# rollback still enabled by default (all-or-nothing guarantee)
```

**Debugging mode** (see everything, no automatic changes):
```bash
rclone config create myremote raid3 \
    even remote1: \
    odd remote2: \
    parity remote3: \
    auto_cleanup false \
    auto_heal false
# Shows all objects/directories including broken ones
# No automatic reconstruction or cleanup
# rollback still enabled by default (all-or-nothing guarantee)
```

**Rollback Configuration:**

The `rollback` option controls whether write operations (Put, Update, Move) automatically rollback successful particle operations if any particle operation fails. This provides an **all-or-nothing guarantee** - either all particles are written/moved/updated, or none are.

- **`rollback=true`** (default): Automatically rollback on failure. Prevents partial operations that would create degraded files. Recommended for production use.
  - ✅ **Put rollback**: Fully working
  - ✅ **Move rollback**: Fully working
  - ⚠️ **Update rollback**: Currently not working properly (see Known Limitations below)
- **`rollback=false`**: No automatic rollback. Failed operations may leave partial files. Only use for debugging or special cases.

**Example with rollback disabled** (debugging):
```bash
rclone config create myremote raid3 \
    even remote1: \
    odd remote2: \
    parity remote3: \
    rollback false
# Allows partial operations for debugging purposes
```

### Behavior Matrix

| auto_cleanup | auto_heal | Behavior |
|--------------|-----------|----------|
| `true` (default) | `true` (default) | Hide orphans (1/3) + Reconstruct degraded (2/3) - **Recommended** |
| `true` | `false` | Hide orphans, but don't auto-reconstruct - **Conservative** |
| `false` | `true` | Show everything + Reconstruct degraded - **Debugging with healing** |
| `false` | `false` | Show everything, no changes - **Raw debugging mode** |

### CleanUp Command

**Important**: `auto_cleanup=true` only **hides** broken objects from listings. To **actually delete** them, you must run the cleanup command:

```bash
$ rclone cleanup myremote:mybucket
Scanning for broken objects...
Found 5 broken objects (total size: 3.0 KiB)
Cleaned up 5 broken objects (freed 3.0 KiB)
```

**Note**: Broken objects are physically deleted from the remotes only when:
- You explicitly run `rclone cleanup`
- You delete/remove objects (best-effort cleanup during delete operations)

**When to use**:
- Cleaning up after backend failures
- Rebuilding from partial write operations
- Periodic maintenance
- Before switching from `auto_cleanup=false` to `auto_cleanup=true`

**What it removes**:
- Objects with only 1 particle (even, odd, or parity)
- Orphaned particles from failed operations
- Does NOT remove valid objects (2+ particles)

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

The heal command:
- Scans all objects in the remote
- Identifies objects with exactly 2 of 3 particles (degraded state)
- Reconstructs and uploads the missing particle
- Reports summary of healed objects

**Example output**:
```
Heal Summary
══════════════════════════════════════════

Files scanned:      100
Healthy (3/3):       85
Healed (2/3→3/3):   12
Unrebuildable (≤1): 3
```

**When to use**:
- Periodic maintenance
- After rebuilding from backend failures
- Before important operations
- When you want to ensure all objects are fully healthy

**Note**: The heal command works regardless of the `auto_heal` setting - it's always available as an explicit admin command. This is different from `auto_heal` which heals objects automatically during reads.

### Object and Directory States

| Particles/Backends | State | auto_cleanup | auto_heal | Behavior |
|-------------------|-------|--------------|-----------|----------|
| **3/3** | Healthy | N/A | N/A | Normal operations |
| **2/3** | Degraded | N/A | ✅ Enabled | Reconstruct missing particle/directory |
| **2/3** | Degraded | N/A | ❌ Disabled | No automatic reconstruction (use `heal` command or `rebuild`) |
| **1/3** | Orphaned | ✅ Enabled | N/A | Hide from listings, delete if accessed |
| **1/3** | Orphaned | ❌ Disabled | N/A | Show in listings, operations may fail |

**Terminology**:
- **Degraded** (2/3): Missing 1 particle/directory - **can reconstruct** ✅
- **Orphaned** (1/3): Missing 2 particles/directories - **cannot reconstruct** ❌

**Examples**:

**File with 2/3 particles** (degraded, reconstructable):
```bash
# auto_heal=true (default)
$ rclone cat raid3:file.txt
# ✅ Reads successfully (reconstructs from even+odd or parity)
# ✅ Queues missing particle for upload (heal)

# auto_heal=false
$ rclone cat raid3:file.txt  
# ✅ Reads successfully (reconstructs from available particles)
# ❌ Does NOT queue heal upload
```

**Directory on 2/3 backends** (degraded, reconstructable):
```bash
# auto_heal=true (default)
$ rclone ls raid3:mydir
# ✅ Lists contents
# ✅ Automatically creates missing directory on 3rd backend during access (2/3 → 3/3)
#    This happens transparently - no manual intervention needed

# auto_heal=false
$ rclone ls raid3:mydir
# ✅ Lists contents  
# ❌ Does NOT create missing directory (directory remains degraded)
```

**Note**: Directory reconstruction via auto-heal complements the rebuild command:
- **Rebuild command**: Rebuilds file particles (parent directories created automatically during file uploads)
- **Auto-heal during access**: Reconstructs empty directories that exist on 2/3 backends when accessed

**File with 1/3 particles** (orphaned, cannot reconstruct):
```bash
# auto_cleanup=true (default)
$ rclone ls raid3:
# ✅ File hidden from listing (not shown)

# auto_cleanup=false (debugging)
$ rclone ls raid3:
# ✅ File shown in listing
$ rclone cat raid3:file.txt
# ❌ Fails: cannot reconstruct from 1 particle
```

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

**Automatic File Reconstruction:**
When one data particle (even or odd) is missing but parity is available:
1. Backend detects the missing particle
2. Reads the available data particle + parity particle
3. Reconstructs the missing data using XOR operation
4. Returns the complete file to the user
5. **Queues the missing particle for background upload (heal)**

**Automatic Directory Reconstruction:**
When a directory exists on 2/3 backends and is accessed (via `List()` operation):
1. Backend detects the directory exists on exactly 2 backends
2. If `auto_heal=true` (default), automatically creates the missing directory on the third backend
3. Directory is immediately available on all 3 backends (2/3 → 3/3)
4. This happens transparently during normal directory access operations

**Note**: Directory reconstruction complements the rebuild command:
- **Rebuild command**: Rebuilds file particles on a replacement backend (handles files and their parent directories)
- **Auto-heal during access**: Reconstructs empty directories that exist on 2/3 backends (handles directory structure)

**Heal Process (Files):**
1. Missing particle is calculated during reconstruction
2. Upload is queued to a background worker
3. User gets data immediately (6-7 seconds with aggressive timeout)
4. Background worker uploads the missing particle (2-3 seconds)
5. Command waits for upload to complete before exiting (~9-10 seconds total)

**Example:**
```bash
$ rclone cat raid3:file.txt
2025/11/02 10:00:00 INFO  : file.txt: Reconstructed from even+parity (degraded mode)
2025/11/02 10:00:00 INFO  : raid3: Queued odd particle for heal upload: file.txt
Hello World!
2025/11/02 10:00:07 INFO  : raid3: Waiting for 1 heal upload(s) to complete...
2025/11/02 10:00:10 INFO  : raid3: Heal upload completed for file.txt (odd)
2025/11/02 10:00:10 INFO  : raid3: Heal complete
```

**Performance:**
- **Normal operation** (all particles healthy): 6-7 seconds
- **Degraded mode with heal**: 9-10 seconds (6-7s read + 2-3s upload)
- **No delay** when all particles are available (Shutdown exits immediately)

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

The backend performs integrity checks:

1. **Size validation**: Ensures even particle size equals odd size or is one byte larger
2. **Existence check**: Verifies both particles exist before attempting reconstruction
3. **Error reporting**: Clear error messages if particles are missing or invalid

## Behavior Details

### Upload Operations
- Reads entire file into memory
- Splits into even/odd bytes
- Writes both particles in parallel to both remotes

### Download Operations
- Retrieves both particles in parallel
- Validates particle sizes
- Merges bytes back into original sequence
- Returns reconstructed data

### Hash Calculation
- Must reconstruct entire file to calculate hash
- Hashes are computed on the merged data, not particles
- Transparent to the user - appears as normal file hash

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

### Read Operations ✅ Degraded Mode Supported
- Work with **ANY 2 of 3** backends available
- Automatically reconstruct missing particles from parity
- Heal restores missing particles in background
- **Example**: If odd backend is down, reads use even + parity

### Write Operations ❌ All Backends Required (Strict Enforcement)
- **Require ALL 3 backends** available (Put, Update, Move)
- **Pre-flight health check** before each write operation
- Fail immediately if any backend unavailable
- **Do NOT create partially-written files or corrupted data**
- **Automatic rollback** (enabled by default) ensures all-or-nothing guarantee
- Matches hardware RAID 3 controller behavior

**Implementation**:
- Health check tests all 3 backends before write (5-second timeout)
- Clear error message: `"write blocked in degraded mode (RAID 3 policy)"`
- Prevents corruption from rclone's retry logic
- **Rollback mechanism**: If any particle operation fails, all successful operations are automatically rolled back
- Overhead: +0.2 seconds for health check (acceptable for safety)

### Delete Operations ✅ Best Effort
- Succeed if any backends are reachable
- Ignore "not found" errors (idempotent)
- Safe to delete files with missing particles

### Rollback Mechanism ✅ All-or-Nothing Guarantee

The raid3 backend implements automatic rollback for write operations (Put, Update, Move) when `rollback=true` (default). This ensures an **all-or-nothing guarantee**: either all particles are successfully written/moved/updated, or none are.

**Current Status**:
- ✅ **Put rollback**: Fully working - automatically removes uploaded particles on failure
- ✅ **Move rollback**: Fully working - automatically moves particles back to original locations on failure
- ⚠️ **Update rollback**: **Not working properly** - implementation exists but has issues (see Known Limitations)

**How it works**:

1. **Put Operation**: Tracks successfully uploaded particles. If any particle upload fails, all uploaded particles are automatically removed.
2. **Move Operation**: Tracks successfully moved particles. If any particle move fails, all moved particles are automatically moved back to their original locations.
3. **Update Operation**: Uses move-to-temp pattern - original particles are moved to temporary locations before updating. If update fails, original particles should be restored from temp locations. **⚠️ Currently not working properly - rollback restoration may fail.**

**Benefits**:
- ✅ Prevents partial operations that would create degraded files
- ✅ Ensures data consistency - no corrupted or partially-written files
- ✅ Automatic cleanup on failure - no manual intervention needed
- ✅ Best-effort rollback - logs errors but doesn't fail if rollback itself encounters issues

**Example**:
```bash
# Attempt to move file when one backend becomes unavailable mid-operation
$ rclone move raid3:file.txt raid3:moved/file.txt
❌ Error: move failed - odd backend unavailable

# With rollback enabled (default):
✅ Original file still exists at source (rollback restored it)
✅ No file exists at destination (partial moves were rolled back)
✅ All-or-nothing guarantee maintained - no degraded files created

# With rollback disabled:
⚠️ Some particles may have moved (partial move occurred)
⚠️ File may exist at destination in degraded state
⚠️ Original file may be partially missing
```

### Rationale

This policy ensures **data consistency** while maximizing **read availability**:

**Why strict writes with health check?**
- Prevents creating degraded files from the start
- **Prevents corruption** from partial updates on retries
- Matches industry-standard RAID 3 behavior
- Avoids performance degradation (every new file needing reconstruction)
- Fails fast with clear error messages

**Why automatic rollback?**
- **All-or-nothing guarantee**: Either all particles are written/moved/updated, or none are
- Prevents partial operations that would create degraded files
- Ensures data consistency even if backends fail during operations
- Automatically cleans up successful particle operations if any particle operation fails
- Can be disabled for debugging purposes (not recommended for production)

**Why best-effort deletes?**
- Missing particle = already deleted (same end state)
- Idempotent delete is user-friendly
- Can't make state worse by deleting

**Example workflow**:
```bash
# Normal operation - all backends up
$ rclone copy local:file.txt raid3:
✅ Success - all 3 particles created (with health check: ~1.2s)

# One backend goes down
$ rclone copy local:newfile.txt level3:
❌ Error: write blocked in degraded mode (RAID 3 policy): odd backend unavailable
# Fails in ~5 seconds (health check timeout)

# But reads still work!
$ rclone cat raid3:file.txt
✅ Success - reconstructed from even+parity (~7s)
INFO: Heal upload completed

# Updates also blocked in degraded mode (prevents corruption)
$ echo "new data" | rclone rcat raid3:file.txt
❌ Error: update blocked in degraded mode (RAID 3 policy): odd backend unavailable
# Original file preserved - NO CORRUPTION!

# If update starts but backend fails mid-operation:
$ rclone copyto updated.txt raid3:file.txt
❌ Error: update failed - parity backend unavailable
# ⚠️ NOTE: Update rollback is currently not working properly
# With rollback enabled, restoration may fail in some scenarios
# See "Known Limitations" section above for details

# Deletes still work!
$ rclone delete raid3:file.txt
✅ Success - deleted from available backends
```

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

### More Examples: Playing with the Test Setup

Here are more examples to explore raid3 features using the test environment:

#### Example 1: Verify Byte-Level Striping

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

#### Example 2: Test Degraded Mode Reconstruction

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

#### Example 3: Test Auto-Heal

```bash
# Upload a file
echo "Test heal" > heal_test.txt
rclone --config "${CONFIG}" copy heal_test.txt localraid3:

# Remove odd particle (simulate failure)
rm ${HOME}/go/raid3storage/odd_local/heal_test.txt

# Read triggers auto-heal (with auto_heal=true, default)
rclone --config "${CONFIG}" cat localraid3:heal_test.txt

# Wait a moment, then verify particle was restored
sleep 2
ls -lh ${HOME}/go/raid3storage/odd_local/heal_test.txt  # Should exist
```

#### Example 4: Test Rebuild Command

```bash
# Upload several files
for i in {1..5}; do
  echo "File $i" > file$i.txt
  rclone --config "${CONFIG}" copy file$i.txt localraid3:
done

# Simulate complete backend replacement (remove all even particles)
rm -rf ${HOME}/go/raid3storage/even_local/*

# Rebuild the even backend
rclone --config "${CONFIG}" backend rebuild localraid3: even

# Verify files were rebuilt
rclone --config "${CONFIG}" ls localraid3:  # Should show all 5 files
ls ${HOME}/go/raid3storage/even_local/      # Should have 5 particles
```

#### Example 5: Test Heal Command

```bash
# Upload files
for i in {1..3}; do
  echo "Data $i" > data$i.txt
  rclone --config "${CONFIG}" copy data$i.txt localraid3:
done

# Degrade files (remove parity particles)
rm ${HOME}/go/raid3storage/parity_local/*.parity-*

# Heal all degraded objects
rclone --config "${CONFIG}" backend heal localraid3:

# Verify parity particles were restored
ls ${HOME}/go/raid3storage/parity_local/  # Should have 3 .parity-* files
```

#### Example 6: Compare RAID3 vs Single Backend

```bash
# Upload same file to both
echo "Comparison test" > compare.txt
rclone --config "${CONFIG}" copy compare.txt localraid3:
rclone --config "${CONFIG}" copy compare.txt localsingle:

# Compare directory structures
echo "=== RAID3 (3 particles) ==="
ls -lh ${HOME}/go/raid3storage/even_local/
ls -lh ${HOME}/go/raid3storage/odd_local/
ls -lh ${HOME}/go/raid3storage/parity_local/

echo "=== Single (1 copy) ==="
ls -lh ${HOME}/go/raid3storage/single_local/

# Both should have the same logical content
rclone --config "${CONFIG}" cat localraid3:compare.txt
rclone --config "${CONFIG}" cat localsingle:compare.txt
```

#### Example 7: Test Error Handling (Degraded Writes)

```bash
# Upload a file first
echo "Test file" > test.txt
rclone --config "${CONFIG}" copy test.txt localraid3:

# Simulate backend failure (remove even directory access)
# Note: This is just for testing - in real scenarios, the backend would be unavailable
chmod 000 ${HOME}/go/raid3storage/even_local

# Try to upload (should fail with clear error)
echo "New data" | rclone --config "${CONFIG}" rcat localraid3:newfile.txt
# Expected: ERROR: write blocked in degraded mode (RAID 3 policy)

# Restore access
chmod 755 ${HOME}/go/raid3storage/even_local

# Now upload should work
echo "New data" | rclone --config "${CONFIG}" rcat localraid3:newfile.txt
```

#### Example 8: Inspect Parity Files

```bash
# Upload files of different lengths
echo -n "Even" > even.txt      # 4 bytes (even length)
echo -n "Odd!" > odd.txt       # 4 bytes (even length, but let's make it odd)
echo -n "X" > odd.txt          # 1 byte (odd length)

rclone --config "${CONFIG}" copy even.txt localraid3:
rclone --config "${CONFIG}" copy odd.txt localraid3:

# Check parity file suffixes
ls ${HOME}/go/raid3storage/parity_local/
# Should show: even.txt.parity-el (even length)
#              odd.txt.parity-ol (odd length)

# The suffix indicates how to reconstruct the file
```

---

## Testing

### Quick Test with Three Local Directories

```bash
# Set up config
cat > /tmp/raid3-test.conf << EOF
[raid3]
type = raid3
even = /tmp/raid3-test/even
odd = /tmp/raid3-test/odd
parity = /tmp/raid3-test/parity
EOF

# Create test directories
mkdir -p /tmp/raid3-test/{even,odd,parity,source,dest}

# Create test data (even-length and odd-length)
echo "Hello, World!" > /tmp/raid3-test/source/test_even.txt  # 14 bytes
echo "Test file!" > /tmp/raid3-test/source/test_odd.txt      # 11 bytes

# Upload with RAID 3 striping and parity
rclone --config /tmp/raid3-test.conf copy /tmp/raid3-test/source/ raid3: -v

# Verify particles
echo "=== Even bytes ==="
ls -lh /tmp/raid3-test/even/          # 7 bytes and 6 bytes
hexdump -C /tmp/raid3-test/even/test_even.txt

echo "=== Odd bytes ==="
ls -lh /tmp/raid3-test/odd/           # 7 bytes and 5 bytes  
hexdump -C /tmp/raid3-test/odd/test_even.txt

echo "=== Parity (with suffixes) ==="
ls -lh /tmp/raid3-test/parity/        # .parity-el and .parity-ol files
hexdump -C /tmp/raid3-test/parity/test_even.txt.parity-el

# Download (reconstructs from even+odd, ignores parity)
rclone --config /tmp/raid3-test.conf copy raid3: /tmp/raid3-test/dest/ -v

# Verify reconstruction
diff /tmp/raid3-test/source/test_even.txt /tmp/raid3-test/dest/test_even.txt
# ✓ Files are identical!

# Verify MD5 hashes
md5sum /tmp/raid3-test/source/*.txt
md5sum /tmp/raid3-test/dest/*.txt
# Hashes should match perfectly
```

### Automated Test Script

A comprehensive test script is available at `/tmp/test-raid3.sh` that tests:
- ✅ Upload with byte striping
- ✅ Parity calculation and suffix assignment
- ✅ File listing (parity files hidden)
- ✅ Download and reconstruction
- ✅ MD5 hash verification
- ✅ Deletion of all three particles

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

