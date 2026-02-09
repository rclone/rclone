---
title: "Raid3"
description: "Disaster‑tolerant virtual backend that distributes objects across three remotes and automatically recovers from single‑remote failures"
---

# {{< icon "fa fa-server" >}} Raid3

The `raid3` backend implements RAID 3 storage with byte-level data striping and XOR parity across three remotes. Data is split into even and odd byte indices, with parity calculated to enable rebuild from single backend failures.

## ⚠️ Status: Alpha / Experimental

This backend is in early development (Alpha stage) for testing and evaluation. Core operations (Put, Get, Delete, List) are implemented and tested. Degraded mode reads, automatic heal, and rebuild of failing remotes work.

## How It Works

When uploading a file, it is split at the byte level with XOR parity:
- **Even-indexed bytes** (0, 2, 4, 6, ...) go to the even remote
- **Odd-indexed bytes** (1, 3, 5, 7, ...) go to the odd remote
- **XOR parity** (even[i] XOR odd[i]) goes to the parity remote

For a file of N bytes, the even particle contains `ceil(N/2)` bytes and the odd particle contains `floor(N/2)` bytes, so the even particle size equals the odd size or is one byte larger.

When downloading, the backend retrieves both particles, validates particle sizes are correct, merges even and odd bytes back into the original data, and returns the reconstructed file. When one particle is missing (2/3 present), reads automatically reconstruct from the other two particles.

## Degraded Mode

The backend supports degraded mode (hardware RAID 3 compliant):
- **Reads** work with ANY 2 of 3 backends available (missing particles automatically reconstructed and restored in background)
- **Writes and deletes** require ALL 3 backends available (strict RAID 3 behavior)

With `auto_heal=true` (default), missing particles are queued for background upload and directories missing on one backend are automatically created during `List()` operations.

## Configuration

Here is an example of how to make a raid3 remote called `remote`. First run:

```console
rclone config
```

This will guide you through an interactive setup process:

```text
No remotes found, make a new one?
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / RAID 3 storage with byte-level data striping across three remotes
   \ "raid3"
[snip]
Storage> raid3
Remote for even-indexed bytes (indices 0, 2, 4, ...).
This should be in the form 'remote:path'.
Enter a string value. Press Enter for the default ("").
even> remote1:data/even
Remote for odd-indexed bytes (indices 1, 3, 5, ...).
This should be in the form 'remote:path'.
Enter a string value. Press Enter for the default ("").
odd> remote2:data/odd
Remote for parity data (XOR of even and odd bytes).
This should be in the form 'remote:path'.
Enter a string value. Press Enter for the default ("").
parity> remote3:data/parity
Automatically hide broken objects (only 1 particle) from listings.
Enter a boolean value (true or false). Press Enter for the default ("true").
auto_cleanup> true
Automatically reconstruct missing particles/directories (2/3 present).
Enter a boolean value (true or false). Press Enter for the default ("true").
auto_heal> true
Remote config
Configuration complete.
Options:
- type: raid3
- even: remote1:data/even
- odd: remote2:data/odd
- parity: remote3:data/parity
- auto_cleanup: true
- auto_heal: true
Keep this "remote" remote?
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
Current remotes:

Name                 Type
====                 ====
remote               raid3

e) Edit existing remote
n) New remote
d) Delete remote
r) Rename remote
c) Copy remote
s) Set configuration password
q) Quit config
e/n/d/r/c/s/q> q
```

Once configured you can then use `rclone` like this:

List directories in the raid3 remote:

```console
rclone lsd remote:
```

List all the files:

```console
rclone ls remote:
```

Copy a local directory to the raid3 remote:

```console
rclone copy /path/to/source remote:destination
```

## Auto-Cleanup and Auto-Heal

By default, raid3 provides two automatic features:

- **`auto_cleanup`** (default: `true`): Automatically hides and deletes orphaned items (1/3 particles) when all remotes are available
- **`auto_heal`** (default: `true`): Automatically reconstructs missing items (2/3 particles) during reads

### Object and Directory States

| Particles/Backends | State | auto_cleanup | auto_heal | Behavior |
|-------------------|-------|--------------|-----------|----------|
| **3/3** | Healthy | N/A | N/A | Normal operations |
| **2/3** | Degraded | N/A | ✅ Enabled | Reconstruct missing particle/directory |
| **2/3** | Degraded | N/A | ❌ Disabled | No automatic reconstruction (use `heal` command) |
| **1/3** | Orphaned | ✅ Enabled | N/A | Hide from listings, delete if accessed |
| **1/3** | Orphaned | ❌ Disabled | N/A | Show in listings, operations may fail |

**Terminology**: **Degraded** (2/3) = missing 1 particle, can reconstruct ✅. **Orphaned** (1/3) = missing 2 particles, cannot reconstruct ❌.

## CleanUp Command

When `auto_cleanup=true` (default), broken objects (1/3 particles) are automatically deleted from listings when all 3 remotes are available, or hidden when remotes are missing. With `auto_cleanup=false`, broken objects are visible but cannot be read.

To manually delete broken objects:

```console
rclone cleanup remote:path
```

This requires all 3 remotes to be available.

## Streaming and Large Files

With `use_streaming=true` (default), the backend uses a pipelined chunked approach that processes files in 2MB chunks, enabling efficient handling of large files without loading entire files into memory. Memory usage is bounded (~5MB for double buffering).

When `use_streaming=false`, files are loaded entirely into memory (legacy mode), limiting practical file size to ~500 MiB - 1 GB depending on available RAM.

## Feature Handling with Mixed Remotes

When using different remote types (e.g., mixing object storage like S3 with file storage like local filesystem), raid3 automatically intersects features from all three backends. Most features require **all backends** to support them (AND logic), ensuring compatibility across the union.

**Features requiring all backends** (AND logic):
- `BucketBased`, `SetTier`, `GetTier`, `ServerSideAcrossConfigs`, `PartialUploads`
- `Copy`, `Move`, `DirMove` operations
- `ReadMimeType`, `WriteMimeType`, `CanHaveEmptyDirectories`

**Features using best-effort** (OR logic, raid3-specific):
- Metadata features (`ReadMetadata`, `WriteMetadata`, `UserMetadata`, etc.) - work if any backend supports them

## Known Limitations

### Update Rollback Not Working Properly

Update operation rollback has issues when `rollback=true` (default); Put and Move rollback work correctly. Failed Update operations may not properly restore particles from temporary locations, leading to degraded files, mainly affecting backends without server-side Move support (e.g., S3/MinIO).

**Workarounds**:
- Use `rollback=false` for Update operations
- Ensure all backends support server-side Move
- Manually fix degraded files using the `heal` command

## Storage Efficiency

The backend enables single-backend failure rebuild using ~150% storage (50% overhead for parity):
- Even particle: ~50% of original file size
- Odd particle: ~50% of original file size
- Parity particle: ~50% of original file size
- Total: ~150% of original file size

<!-- autogenerated options start - DO NOT EDIT - instead edit fs.RegInfo in backend/raid3/raid3.go and run make backenddocs to verify --> <!-- markdownlint-disable-line line-length -->
### Standard options

Here are the Standard options specific to raid3 (RAID 3 storage with byte-level data striping across three remotes).

#### --raid3-even

Remote for even-indexed bytes (indices 0, 2, 4, ...).

This should be in the form 'remote:path'.

Properties:

- Config:      even
- Env Var:     RCLONE_RAID3_EVEN
- Type:        string
- Required:    true

#### --raid3-odd

Remote for odd-indexed bytes (indices 1, 3, 5, ...).

This should be in the form 'remote:path'.

Properties:

- Config:      odd
- Env Var:     RCLONE_RAID3_ODD
- Type:        string
- Required:    true

#### --raid3-parity

Remote for parity data (XOR of even and odd bytes).

This should be in the form 'remote:path'.

Properties:

- Config:      parity
- Env Var:     RCLONE_RAID3_PARITY
- Type:        string
- Required:    true

#### --raid3-timeout-mode

Timeout behavior for backend operations

Properties:

- Config:      timeout_mode
- Env Var:     RCLONE_RAID3_TIMEOUT_MODE
- Type:        string
- Default:     "standard"
- Examples:
  - "standard"
    - Use global timeout settings (best for local/file storage)
  - "balanced"
    - Moderate timeouts (3 retries, 30s) - good for reliable S3
  - "aggressive"
    - Fast failover (1 retry, 10s) - best for S3 degraded mode

#### --raid3-auto-cleanup

Automatically hide broken objects (only 1 particle) from listings

Properties:

- Config:      auto_cleanup
- Env Var:     RCLONE_RAID3_AUTO_CLEANUP
- Type:        bool
- Default:     true

#### --raid3-auto-heal

Automatically reconstruct missing particles/directories (2/3 present)

Properties:

- Config:      auto_heal
- Env Var:     RCLONE_RAID3_AUTO_HEAL
- Type:        bool
- Default:     true

#### --raid3-rollback

Automatically rollback successful operations if any particle operation fails (all-or-nothing guarantee)

Properties:

- Config:      rollback
- Env Var:     RCLONE_RAID3_ROLLBACK
- Type:        bool
- Default:     true

### Advanced options

Here are the Advanced options specific to raid3 (RAID 3 storage with byte-level data striping across three remotes).

#### --raid3-use-streaming

Use streaming processing path. When enabled, processes files in chunks instead of loading entire file into memory.

Properties:

- Config:      use_streaming
- Env Var:     RCLONE_RAID3_USE_STREAMING
- Type:        bool
- Default:     true

#### --raid3-chunk-size

Chunk size for streaming operations

Properties:

- Config:      chunk_size
- Env Var:     RCLONE_RAID3_CHUNK_SIZE
- Type:        SizeSuffix
- Default:     8Mi

#### --raid3-description

Description of the remote.

Properties:

- Config:      description
- Env Var:     RCLONE_RAID3_DESCRIPTION
- Type:        string
- Required:    false

### Metadata

Any metadata supported by the underlying remotes is read and written.

See the [metadata](/docs/#metadata) docs for more info.

## Backend commands

Here are the commands specific to the raid3 backend.

Run them with:

```console
rclone backend COMMAND remote:
```

The help below will explain what arguments each command takes.

See the [backend](/commands/rclone_backend/) command for more
info on how to pass options and arguments.

These can be run on a running backend using the rc command
[backend/command](/rc/#backend-command).

### status

Show backend health and rebuild guide

```console
rclone backend status remote: [options] [<arguments>+]
```

Shows the health status of all three backends and provides step-by-step
rebuild guidance if any backend is unavailable.

This is the primary diagnostic tool for raid3 - run this first when you
encounter errors or want to check backend health.

Usage:

    rclone backend status raid3:

Output includes:
  • Health status of all three backends (even, odd, parity)
  • Impact assessment (what operations work)
  • Complete rebuild guide for degraded mode
  • Step-by-step instructions for backend replacement

This command is mentioned in error messages when writes fail in degraded mode.


### rebuild

Rebuild missing particles on a replacement backend

```console
rclone backend rebuild remote: [options] [<arguments>+]
```

Rebuilds all missing particles on a backend after replacement.

Use this after replacing a failed backend with a new, empty backend. The rebuild
process reconstructs all missing particles using the other two backends and parity
information, restoring the raid3 backend to a fully healthy state.

Usage:

    rclone backend rebuild raid3: [even|odd|parity]
    
Auto-detects which backend needs rebuild if not specified:

    rclone backend rebuild raid3:

Options:

  -o check-only=true    Analyze what needs rebuild without actually rebuilding
  -o dry-run=true       Show what would be done without making changes
  -o priority=MODE      Rebuild order (auto, dirs-small, dirs, small)

Priority modes:
  auto        Smart default based on dataset (recommended)
  dirs-small  All directories first, then files by size (smallest first)
  dirs        Directories first, then files alphabetically per directory
  small       Create directories as-needed, files by size (smallest first)

Examples:

    # Check what needs rebuild
    rclone backend rebuild raid3: -o check-only=true
    
    # Rebuild with auto-detected backend
    rclone backend rebuild raid3:
    
    # Rebuild specific backend
    rclone backend rebuild raid3: odd
    
    # Rebuild with small files first
    rclone backend rebuild raid3: odd -o priority=small

The rebuild process will:
  1. Scan for missing particles
  2. Reconstruct data from other two backends
  3. Upload restored particles
  4. Show progress and ETA
  5. Verify integrity

Note: This is different from heal which happens automatically during
reads. Rebuild is a manual, complete restoration after backend replacement.


### heal

Heal all degraded objects (2/3 particles present)

```console
rclone backend heal remote: [options] [<arguments>+]
```

Scans the entire remote and heals any objects that have exactly 2 of 3 particles.

This is an explicit, admin-driven alternative to automatic heal on read.
Use this when you want to proactively heal all degraded objects rather than
waiting for them to be accessed during normal operations.

Usage:

    rclone backend heal raid3: [file_path]

Options:

  -o dry-run=true    Show what would be healed without making changes

The heal command will:
  1. Scan all objects in the remote
  2. Identify objects with exactly 2 of 3 particles (degraded state)
  3. Reconstruct and upload the missing particle
  4. Report summary of healed objects

Output includes:
  • Total files scanned
  • Number of healthy files (3/3 particles)
  • Number of healed files (2/3→3/3)
  • Number of unrebuildable files (≤1 particle)
  • List of unrebuildable objects (if any)

Examples:

    # Heal all degraded objects
    rclone backend heal raid3:

    # Heal a specific file
    rclone backend heal raid3: path/to/file.txt

    # Dry-run: See what would be healed without making changes
    rclone backend heal raid3: -o dry-run=true

    # Example output:
    # Heal Summary
    # ══════════════════════════════════════════
    # 
    # Files scanned:      100
    # Healthy (3/3):       85
    # Healed (2/3→3/3):   12
    # Unrebuildable (≤1): 3

Note: This is different from auto_heal which heals objects automatically
during reads. The heal command proactively heals all degraded objects at once,
which is useful for:
  • Periodic maintenance
  • After rebuilding from backend failures
  • Before important operations
  • When you want to ensure all objects are fully healthy

The heal command works regardless of the auto_heal setting - it's always
available as an explicit admin command.


<!-- autogenerated options stop -->

## Metadata

Any metadata supported by the underlying remotes is read and written.

See the [metadata](/docs/#metadata) docs for more info.
