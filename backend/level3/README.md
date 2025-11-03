# Level3 Backend - RAID 3 Storage

The `level3` backend implements **RAID 3** storage with byte-level data striping and XOR parity across three remotes. Data is split into even and odd byte indices, with parity calculated to enable future recovery from single backend failures.

## 🎯 RAID 3 Features

**Current Implementation:**
- ✅ Byte-level striping (even/odd bytes)
- ✅ XOR parity calculation and storage
- ✅ Parity files with length indicators (.parity-el/.parity-ol)
- ✅ All three particles uploaded/deleted in parallel
- ✅ **Automatic parity reconstruction** (degraded mode reads)
- ✅ **Automatic self-healing** (background particle restoration)

**Storage Efficiency:**
- Uses ~150% storage (50% overhead for parity)
- Better than full duplication (200% storage)
- Enables single-backend failure recovery

**Degraded Mode (Hardware RAID 3 Compliant):**
- ✅ **Reads** work with ANY 2 of 3 backends available
- ✅ Missing particles are automatically reconstructed from parity
- ✅ **Self-healing** automatically restores missing particles in background
- ❌ **Writes** require ALL 3 backends available (strict RAID 3 behavior)
- ✅ **Deletes** work with ANY backends available (idempotent)

## ⚠️ Current Limitations

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
4. Wait for level3 streaming implementation

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

### Degraded Mode & Self-Healing

**Automatic Reconstruction:**
When one data particle (even or odd) is missing but parity is available:
1. Backend detects the missing particle
2. Reads the available data particle + parity particle
3. Reconstructs the missing data using XOR operation
4. Returns the complete file to the user
5. **Queues the missing particle for background upload (self-healing)**

**Self-Healing Process:**
1. Missing particle is calculated during reconstruction
2. Upload is queued to a background worker
3. User gets data immediately (6-7 seconds with aggressive timeout)
4. Background worker uploads the missing particle (2-3 seconds)
5. Command waits for upload to complete before exiting (~9-10 seconds total)

**Example:**
```bash
$ rclone cat level3:file.txt
2025/11/02 10:00:00 INFO  : file.txt: Reconstructed from even+parity (degraded mode)
2025/11/02 10:00:00 INFO  : level3: Queued odd particle for self-healing upload: file.txt
Hello World!
2025/11/02 10:00:07 INFO  : level3: Waiting for 1 self-healing upload(s) to complete...
2025/11/02 10:00:10 INFO  : level3: Self-healing upload completed for file.txt (odd)
2025/11/02 10:00:10 INFO  : level3: Self-healing complete
```

**Performance:**
- **Normal operation** (all particles healthy): 6-7 seconds
- **Degraded mode with self-healing**: 9-10 seconds (6-7s read + 2-3s upload)
- **No delay** when all particles are available (Shutdown exits immediately)

## Configuration

Use `rclone config` to set up a level3 remote:

```
rclone config
n) New remote
name> mylevel3
Type of storage> level3
remote> /path/to/first/backend
remote2> /path/to/second/backend
```

Or edit the config file directly:

```ini
[mylevel3]
type = level3
even = /path/to/backend1       # Even-indexed bytes
odd = /path/to/backend2        # Odd-indexed bytes
parity = /path/to/backend3     # XOR parity
```

You can also use different cloud storage backends:

```ini
[mylevel3]
type = level3
even = s3:bucket1/data
odd = gdrive:backup/data
parity = dropbox:parity
```

## Usage Examples

### Upload (splits data):
```bash
rclone copy /local/files mylevel3:
```
This splits each file and writes even bytes to remote1, odd bytes to remote2.

### Download (reconstructs data):
```bash
rclone copy mylevel3: /local/destination
```
This reads both particles and reconstructs the original files.

### List files:
```bash
rclone ls mylevel3:
```
Shows files with their original (reconstructed) sizes.

### Create directory:
```bash
rclone mkdir mylevel3:newdir
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

#### Data Loss Risk
**Critical:** If either remote fails, **ALL data is lost**. This backend provides:
- ❌ No redundancy
- ❌ No parity (yet - planned for future)
- ❌ No recovery from single backend failure

#### Memory Usage
- Files are buffered entirely in memory during upload/download
- Not suitable for very large files with limited RAM
- Consider file size vs. available memory

#### Moving Files Within Same Level3 Backend
Due to rclone's overlap detection, you cannot use `rclone move` within the same level3 backend:

```bash
# This will NOT work:
rclone move mylevel3:/file.txt mylevel3:/subfolder/
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
rclone copyto mylevel3:renamed_notes.txt notes_merged.txt
# Result: notes_merged.txt/ containing renamed_notes.txt (wrong!)

# Or may copy all files from the directory instead of just one file
```

**Recommended workaround - Use `cat` for single files:**
```bash
# Use cat instead - it works perfectly for single-file download
rclone cat mylevel3:renamed_notes.txt > notes_merged.txt
# ✓ Creates notes_merged.txt as a file with the correct merged content
```

**Example:**
```bash
# Original file
echo "Hello, World!" > /tmp/level3-test/source/test.txt

# Upload (splits data)
rclone copy /tmp/level3-test/source/test.txt mylevel3:

# Download single file using cat
rclone cat mylevel3:test.txt > notes_merged.txt
# ✓ Creates a file called "notes_merged.txt" with merged content
```

## Error Handling (Hardware RAID 3 Compliant)

The level3 backend follows **hardware RAID 3 behavior** for error handling:

### Read Operations ✅ Degraded Mode Supported
- Work with **ANY 2 of 3** backends available
- Automatically reconstruct missing particles from parity
- Self-healing restores missing particles in background
- **Example**: If odd backend is down, reads use even + parity

### Write Operations ❌ All Backends Required (Strict Enforcement)
- **Require ALL 3 backends** available (Put, Update, Move)
- **Pre-flight health check** before each write operation
- Fail immediately if any backend unavailable
- **Do NOT create partially-written files or corrupted data**
- Matches hardware RAID 3 controller behavior

**Implementation**:
- Health check tests all 3 backends before write (5-second timeout)
- Clear error message: `"write blocked in degraded mode (RAID 3 policy)"`
- Prevents corruption from rclone's retry logic
- Overhead: +0.2 seconds (acceptable for safety)

### Delete Operations ✅ Best Effort
- Succeed if any backends are reachable
- Ignore "not found" errors (idempotent)
- Safe to delete files with missing particles

### Rationale

This policy ensures **data consistency** while maximizing **read availability**:

**Why strict writes with health check?**
- Prevents creating degraded files from the start
- **Prevents corruption** from partial updates on retries
- Matches industry-standard RAID 3 behavior
- Avoids performance degradation (every new file needing reconstruction)
- Fails fast with clear error messages

**Why best-effort deletes?**
- Missing particle = already deleted (same end state)
- Idempotent delete is user-friendly
- Can't make state worse by deleting

**Example workflow**:
```bash
# Normal operation - all backends up
$ rclone copy local:file.txt level3:
✅ Success - all 3 particles created (with health check: ~1.2s)

# One backend goes down
$ rclone copy local:newfile.txt level3:
❌ Error: write blocked in degraded mode (RAID 3 policy): odd backend unavailable
# Fails in ~5 seconds (health check timeout)

# But reads still work!
$ rclone cat level3:file.txt
✅ Success - reconstructed from even+parity (~7s)
INFO: Self-healing upload completed

# Updates also blocked in degraded mode (prevents corruption)
$ echo "new data" | rclone rcat level3:file.txt
❌ Error: update blocked in degraded mode (RAID 3 policy): odd backend unavailable
# Original file preserved - NO CORRUPTION!

# Deletes still work!
$ rclone delete level3:file.txt
✅ Success - deleted from available backends
```

## Testing

### Quick Test with Three Local Directories

```bash
# Set up config
cat > /tmp/raid3-test.conf << EOF
[raid3]
type = level3
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

## Implementation Notes

The level3 backend:
- Uses `errgroup` for parallel operations
- Implements byte-level splitting/merging functions
- Validates particle sizes on every read
- Computes hashes on reconstructed data
- Supports all standard fs operations (Put, Get, Update, Remove, etc.)

## Comparison with Duplicate Backend

| Feature | Duplicate | Level3 (RAID 3) |
|---------|-----------|-----------------|
| **Number of backends** | 2 | 3 |
| **Data redundancy** | ✅ Full (identical copies) | ⏳ With parity (future) |
| **Storage efficiency** | 50% (2x storage) | 67% (1.5x storage) |
| **Single backend failure** | ✅ Still works | ⏳ Future (parity reconstruction) |
| **Current status** | ✅ Fully redundant | ⚠️ Needs both even+odd |
| **Use case** | Backup, redundancy | Efficient fault-tolerance |
| **Read from** | Either backend | Both even+odd required |
| **Write to** | Both (identical) | All 3 (striped+parity) |
| **Parity** | ❌ None | ✅ XOR parity |

