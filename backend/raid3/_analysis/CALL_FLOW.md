# raid3 Backend - Complete Call Flow Documentation

## Purpose of This Document

This document traces the complete call flow for all major rclone operations in the raid3 backend, showing function calls from user commands to underlying storage backends. It is organized into:

1. **Common Parts** - Shared infrastructure used across all commands
2. **Command-Specific Parts** - Detailed flows for each operation

---

## Table of Contents

1. [Common Infrastructure](#common-infrastructure)
2. [Put (Upload)](#put-upload)
3. [Get/Open (Download)](#getopen-download)
4. [List (ls)](#list-ls)
5. [Remove (Delete)](#remove-delete)
6. [Update (Modify)](#update-modify)
7. [Move (Rename)](#move-rename)
8. [Mkdir (Create Directory)](#mkdir-create-directory)
9. [Rmdir (Remove Directory)](#rmdir-remove-directory)
10. [Heal Command](#heal-command)
11. [Rebuild Command](#rebuild-command)
12. [Status Command](#status-command)

---

## Common Infrastructure

### Backend Initialization

**File**: `raid3.go:264` - `NewFs()`

```
User: rclone config / rclone copy ...
    │
    └─> backend/raid3.NewFs(ctx, name, root, configmap)
        │
        ├─> Parse config (even, odd, parity remotes)
        ├─> Apply timeout mode to context
        ├─> Create three underlying fs.Fs instances (parallel)
        │   ├─> cache.Get(evenPath)
        │   ├─> cache.Get(oddPath)
        │   └─> cache.Get(parityPath)
        ├─> Initialize heal infrastructure (if auto_heal enabled)
        │   ├─> Create uploadQueue
        │   ├─> Start background workers (uploadWorkers goroutines)
        │   └─> Setup context for cancellation
        └─> Return *raid3.Fs
```

**Key Functions**:
- `NewFs()` - Main constructor
- `applyTimeoutMode()` - Applies timeout settings based on mode
- Background heal workers started in `NewFs()` if `auto_heal` is enabled

---

### Health Check (Write Operations)

**File**: `raid3.go:646` - `checkAllBackendsAvailable()`

**Used by**: Put, Update, Move, Mkdir

```go
func (f *Fs) checkAllBackendsAvailable(ctx context.Context) error
```

**Flow**:
```
checkAllBackendsAvailable(ctx)
    │
    ├─> Create timeout context (5 seconds)
    ├─> Parallel checks (3 goroutines):
    │   ├─> Backend 1: List() + Mkdir(test)
    │   ├─> Backend 2: List() + Mkdir(test)
    │   └─> Backend 3: List() + Mkdir(test)
    │
    └─> Return error if ANY backend unavailable
```

**Purpose**: Enforces strict RAID 3 write policy (all 3 backends must be available for writes)

---

### Data Splitting & Parity Calculation

**File**: `particles.go`

#### SplitBytes
**File**: `particles.go:26`

```go
func SplitBytes(data []byte) (even []byte, odd []byte)
```

**Algorithm**:
- `even[i] = data[2*i]` (indices 0, 2, 4, 6, ...)
- `odd[i] = data[2*i+1]` (indices 1, 3, 5, 7, ...)

**Example**:
```
Input:  [A, B, C, D, E, F, G]  (7 bytes)
Output:
  even: [A, C, E, G]           (4 bytes)
  odd:  [B, D, F]              (3 bytes)
```

#### CalculateParity
**File**: `particles.go:64`

```go
func CalculateParity(even []byte, odd []byte) []byte
```

**Algorithm**:
- For each pair: `parity[i] = even[i] XOR odd[i]`
- If odd length: `parity[last] = even[last]` (no XOR partner)

**Example**:
```
even:  [0x48, 0x6C, 0x6F]  (H, l, o)
odd:   [0x65, 0x6C, 0x2C]  (e, l, ,)
parity: [0x2D, 0x00, 0x43]  (XOR results)
```

#### MergeBytes
**File**: `particles.go:44`

```go
func MergeBytes(even []byte, odd []byte) ([]byte, error)
```

**Algorithm**: Interleaves even and odd bytes back into original data

---

### Parallel Operation Pattern

**Used by**: Put, Update, Move, Remove, Mkdir, Rmdir

All write operations use `golang.org/x/sync/errgroup` for parallel execution:

```go
g, gCtx := errgroup.WithContext(ctx)

g.Go(func() error {
    // Operation on even backend
    return f.even.Operation(gCtx, ...)
})

g.Go(func() error {
    // Operation on odd backend
    return f.odd.Operation(gCtx, ...)
})

g.Go(func() error {
    // Operation on parity backend
    return f.parity.Operation(gCtx, ...)
})

return g.Wait() // Waits for all, returns first error
```

**Behavior**:
- All 3 operations happen in parallel
- If ANY fails, context is cancelled
- Other operations are automatically cancelled
- Returns first error encountered

---

### Disable Retries for Writes

**File**: `helpers.go:57` - `disableRetriesForWrites()`

**Used by**: Put, Update, Move

```go
func (f *Fs) disableRetriesForWrites(ctx context.Context) context.Context
```

**Purpose**: Prevents rclone's retry logic from creating degraded files by setting a context flag

---

### Rollback Mechanisms

**File**: `helpers.go`

#### rollbackPut
**File**: `helpers.go:121`

Deletes successfully uploaded particles if Put fails partway through.

#### rollbackUpdate
**File**: `helpers.go:132`

Removes temporary particles if Update fails.

#### rollbackMoves
**File**: `helpers.go:185`

Reverses successful moves if Move fails partway through.

---

## Put (Upload)

**User Command**: `rclone copy file.txt raid3:`

**File**: `raid3.go:1067` - `(*Fs).Put()`

### Call Flow

```
User: rclone copy file.txt raid3:
    │
    └─> fs/operations.Copy()
        │
        └─> backend/raid3.(*Fs).Put(ctx, reader, objectInfo, options)
            │
            ├─> STEP 1: Health Check [LINE 1070]
            │   └─> checkAllBackendsAvailable(ctx)
            │       └─> Tests all 3 backends (parallel)
            │
            ├─> STEP 2: Disable Retries [LINE 1076]
            │   └─> disableRetriesForWrites(ctx)
            │
            ├─> STEP 3: Read File [LINE 1079]
            │   └─> io.ReadAll(in)
            │       └─> Loads entire file into memory
            │
            ├─> STEP 4: Split Data [LINE 1085]
            │   └─> particles.SplitBytes(data)
            │       ├─> evenData = bytes[0, 2, 4, 6, ...]
            │       └─> oddData = bytes[1, 3, 5, 7, ...]
            │
            ├─> STEP 5: Calculate Parity [LINE 1088]
            │   └─> particles.CalculateParity(evenData, oddData)
            │       └─> parityData = even XOR odd
            │
            ├─> STEP 6: Prepare ObjectInfo [LINES 1094-1106]
            │   ├─> Create particleObjectInfo for even
            │   ├─> Create particleObjectInfo for odd
            │   └─> Create particleObjectInfo for parity
            │       └─> GetParityFilename() adds .parity-el or .parity-ol suffix
            │
            ├─> STEP 7: Parallel Upload [LINES 1122-1161]
            │   └─> errgroup.WithContext(ctx)
            │       ├─> Goroutine 1: f.even.Put(evenData)
            │       ├─> Goroutine 2: f.odd.Put(oddData)
            │       └─> Goroutine 3: f.parity.Put(parityData)
            │
            ├─> STEP 8: Wait for All [LINE 1163]
            │   └─> errgroup.Wait()
            │       └─> Returns first error if any
            │
            ├─> STEP 9: Error Handling [LINES 1111-1120]
            │   └─> defer rollbackPut() (if enabled and error occurred)
            │
            └─> STEP 10: Return Object [LINES 1168-1171]
                └─> Returns raid3.Object
```

### Key Points

- **Memory Usage**: Entire file buffered in memory (see [`../docs/OPEN_QUESTIONS.md`](../docs/OPEN_QUESTIONS.md) Q13 for streaming)
- **Error Handling**: If any upload fails, others are cancelled via errgroup
- **Rollback**: If enabled, successfully uploaded particles are deleted on error

---

## Get/Open (Download)

**User Command**: `rclone copy raid3:file.txt /dest` or `rclone cat raid3:file.txt`

**File**: `object.go:206` - `(*Object).Open()`

### Call Flow

```
User: rclone copy raid3:file.txt /dest
    │
    └─> fs/operations.Copy()
        │
        ├─> backend/raid3.(*Fs).NewObject(ctx, remote) [raid3.go:1001]
        │   │
        │   ├─> Probe all 3 particles (parallel)
        │   │   ├─> f.even.NewObject(remote)
        │   │   ├─> f.odd.NewObject(remote)
        │   │   └─> f.parity.NewObject(parityName) [tries both suffixes]
        │   │
        │   └─> Return Object if ≥2 particles present
        │
        └─> backend/raid3.(*Object).Open(ctx, options) [object.go:206]
            │
            ├─> STEP 1: Fetch Particles (Parallel) [LINES 217-229]
            │   ├─> Goroutine 1: f.even.NewObject(remote)
            │   └─> Goroutine 2: f.odd.NewObject(remote)
            │
            ├─> STEP 2: Check Particle Availability [LINE 240]
            │   │
            │   ├─> CASE A: Both even and odd present [LINES 240-269]
            │   │   ├─> Open even particle
            │   │   ├─> Open odd particle
            │   │   ├─> Read both particles
            │   │   └─> MergeBytes(evenData, oddData)
            │   │
            │   └─> CASE B: One particle missing (degraded mode) [LINES 270-370]
            │       │
            │       ├─> Find parity particle (try both suffixes)
            │       │
            │       ├─> If even missing:
            │       │   ├─> Read odd + parity
            │       │   ├─> ReconstructFromOddAndParity()
            │       │   └─> Queue even for heal (if auto_heal enabled)
            │       │
            │       └─> If odd missing:
            │           ├─> Read even + parity
            │           ├─> ReconstructFromEvenAndParity()
            │           └─> Queue odd for heal (if auto_heal enabled)
            │
            └─> STEP 3: Return ReadCloser [LINE 370+]
                └─> Returns io.ReadCloser with merged/reconstructed data
```

### Key Points

- **Degraded Mode**: Works with 2 of 3 particles (automatic reconstruction)
- **Auto-Heal**: Missing particle queued for background upload if `auto_heal` enabled
- **Reconstruction**: Uses XOR parity to rebuild missing data particle

### Reconstruction Functions

**File**: `particles.go`

- `ReconstructFromEvenAndParity()` - Rebuilds odd from even+parity
- `ReconstructFromOddAndParity()` - Rebuilds even from odd+parity
- `ReconstructFromEvenAndOdd()` - Rebuilds parity from even+odd

---

## List (ls)

**User Command**: `rclone ls raid3:` or `rclone lsd raid3:`

**File**: `raid3.go:844` - `(*Fs).List()`

### Call Flow

```
User: rclone ls raid3:
    │
    └─> backend/raid3.(*Fs).List(ctx, dir)
        │
        ├─> STEP 1: Parallel List [LINES 853-866]
        │   ├─> Goroutine 1: f.even.List(dir)
        │   ├─> Goroutine 2: f.odd.List(dir)
        │   └─> Goroutine 3: f.parity.List(dir)
        │
        ├─> STEP 2: Collect Results [LINES 869-884]
        │   ├─> entriesEven, errEven
        │   ├─> entriesOdd, errOdd
        │   └─> entriesParity, errParity (errors ignored)
        │
        ├─> STEP 3: Handle Degraded Mode [LINES 887-900]
        │   │
        │   ├─> If even fails:
        │   │   └─> Try odd (degraded mode)
        │   │
        │   ├─> If both fail:
        │   │   ├─> Check if orphaned directory (exists only on parity)
        │   │   └─> Cleanup if auto_cleanup enabled
        │   │
        │   └─> If odd fails:
        │       └─> Use even (degraded mode)
        │
        ├─> STEP 4: Merge Entries [LINES 920-960]
        │   ├─> Combine even and odd entries
        │   ├─> Filter out parity files (hidden from user)
        │   ├─> Filter out broken objects (if auto_cleanup enabled)
        │   └─> Handle directory reconstruction (if auto_heal enabled)
        │
        └─> STEP 5: Return DirEntries
            └─> Returns unified list (parity files hidden)
```

### Key Points

- **Parity Files Hidden**: Files with `.parity-el` or `.parity-ol` suffix are filtered out
- **Directory Reconstruction**: Missing directories created on failed backend if `auto_heal` enabled
- **Broken Object Cleanup**: Objects with only 1 particle hidden if `auto_cleanup` enabled

---

## Remove (Delete)

**User Command**: `rclone delete raid3:file.txt`

**File**: `object.go:735` - `(*Object).Remove()`

### Call Flow

```
User: rclone delete raid3:file.txt
    │
    └─> backend/raid3.(*Object).Remove(ctx)
        │
        └─> errgroup.WithContext(ctx)
            ├─> Goroutine 1: f.even.NewObject() → Remove()
            ├─> Goroutine 2: f.odd.NewObject() → Remove()
            └─> Goroutine 3: f.parity.NewObject() → Remove()
                │
                └─> Tries both parity suffixes (.parity-el, .parity-ol)
```

### Key Points

- **Best-Effort Policy**: Ignores "not found" errors (idempotent)
- **Parallel Execution**: All 3 deletions happen simultaneously
- **Parity Handling**: Tries both parity filename variants

---

## Update (Modify)

**User Command**: `rclone copy --update file.txt raid3:`

**File**: `object.go:421` - `(*Object).Update()`

### Call Flow

```
User: rclone copy --update file.txt raid3:
    │
    └─> backend/raid3.(*Object).Update(ctx, reader, objectInfo, options)
        │
        ├─> STEP 1: Health Check [LINE 424]
        │   └─> checkAllBackendsAvailable(ctx)
        │
        ├─> STEP 2: Disable Retries [LINE 429]
        │   └─> disableRetriesForWrites(ctx)
        │
        ├─> STEP 3: Read & Process Data [LINES 432-445]
        │   ├─> io.ReadAll(in)
        │   ├─> SplitBytes(data)
        │   ├─> CalculateParity(evenData, oddData)
        │   └─> GetParityFilename(remote, isOddLength)
        │
        ├─> STEP 4: Choose Update Strategy [LINE 448]
        │   │
        │   ├─> If rollback disabled [LINE 450]:
        │   │   └─> updateInPlace() [object.go:458]
        │   │       └─> Direct Update on particle objects
        │   │
        │   └─> If rollback enabled [LINE 454]:
        │       └─> updateWithRollback() [object.go:514]
        │           ├─> Move particles to temp names
        │           ├─> Upload new particles
        │           ├─> Remove temp particles
        │           └─> Rollback on error
        │
        └─> STEP 5: Parallel Update
            └─> errgroup (3 goroutines for even/odd/parity)
```

### Key Points

- **Two Strategies**: In-place update vs. move-to-temp (rollback-safe)
- **Rollback Support**: If enabled, uses move-to-temp pattern for atomic updates
- **Particle Size Validation**: Validates sizes after update to detect corruption

---

## Move (Rename)

**User Command**: `rclone moveto raid3:old.txt raid3:new.txt`

**File**: `raid3.go:1403` - `(*Fs).Move()`

### Call Flow

```
User: rclone moveto raid3:old.txt raid3:new.txt
    │
    └─> backend/raid3.(*Fs).Move(ctx, srcObject, newRemote)
        │
        ├─> STEP 1: Validate Source [LINES 1405-1408]
        │   └─> Check src is raid3.Object
        │
        ├─> STEP 2: Determine Parity Names [LINES 1418-1431]
        │   ├─> Find source parity filename (try both suffixes)
        │   └─> Generate destination parity filename
        │
        ├─> STEP 3: Health Check [LINE 1435]
        │   └─> checkAllBackendsAvailable(ctx)
        │
        ├─> STEP 4: Disable Retries [LINE 1452]
        │   └─> disableRetriesForWrites(ctx)
        │
        ├─> STEP 5: Parallel Move [LINES 1454-1480]
        │   └─> errgroup.WithContext(ctx)
        │       ├─> Goroutine 1: f.even.Move(src, dst)
        │       ├─> Goroutine 2: f.odd.Move(src, dst)
        │       └─> Goroutine 3: f.parity.Move(srcParity, dstParity)
        │
        └─> STEP 6: Rollback on Error [LINES 1482-1495]
            └─> rollbackMoves() if any move failed
```

### Key Points

- **Parity Filename Handling**: Both source and destination parity names must be determined
- **Rollback**: If enabled, reverses successful moves if any fails
- **Strict Policy**: All 3 moves must succeed (RAID 3 compliance)

---

## Mkdir (Create Directory)

**User Command**: `rclone mkdir raid3:newdir`

**File**: `raid3.go:1175` - `(*Fs).Mkdir()`

### Call Flow

```
User: rclone mkdir raid3:newdir
    │
    └─> backend/raid3.(*Fs).Mkdir(ctx, dir)
        │
        ├─> STEP 1: Health Check [LINE 1178]
        │   └─> checkAllBackendsAvailable(ctx)
        │
        └─> STEP 2: Parallel Mkdir [LINES 1182-1206]
            └─> errgroup.WithContext(ctx)
                ├─> Goroutine 1: f.even.Mkdir(dir)
                ├─> Goroutine 2: f.odd.Mkdir(dir)
                └─> Goroutine 3: f.parity.Mkdir(dir)
```

### Key Points

- **Strict Policy**: All 3 backends must be available
- **Parallel Execution**: All 3 mkdirs happen simultaneously

---

## Rmdir (Remove Directory)

**User Command**: `rclone rmdir raid3:dir`

**File**: `raid3.go:1212` - `(*Fs).Rmdir()`

### Call Flow

```
User: rclone rmdir raid3:dir
    │
    └─> backend/raid3.(*Fs).Rmdir(ctx, dir)
        │
        └─> errgroup.WithContext(ctx)
            ├─> Goroutine 1: f.even.Rmdir(dir)
            ├─> Goroutine 2: f.odd.Rmdir(dir)
            └─> Goroutine 3: f.parity.Rmdir(dir)
```

### Key Points

- **Best-Effort Policy**: Ignores "not found" and "not empty" errors
- **Idempotent**: Can be called multiple times safely

---

## Heal Command

**User Command**: `rclone backend heal raid3:` or `rclone backend heal raid3: file.txt`

**File**: `commands.go:386` - `healCommand()`

### Call Flow (Single File)

```
User: rclone backend heal raid3: file.txt
    │
    └─> backend/raid3.(*Fs).healCommand(ctx, ["file.txt"], opt)
        │
        ├─> STEP 1: Inspect File [LINE 392]
        │   └─> particleInfoForObject(ctx, remote) [particles.go:319]
        │       ├─> Check even particle
        │       ├─> Check odd particle
        │       └─> Check parity particle (both suffixes)
        │
        ├─> STEP 2: Determine Status [LINES 407-428]
        │   │
        │   ├─> If 3/3 particles: ✅ Healthy (no action)
        │   │
        │   ├─> If 2/3 particles: Heal needed
        │   │   └─> healObject(ctx, particleInfo) [commands.go:496]
        │   │       │
        │   │       ├─> If missing parity:
        │   │       │   └─> healParityFromData() [commands.go:518]
        │   │       │       ├─> Read even + odd
        │   │       │       ├─> CalculateParity()
        │   │       │       └─> uploadParticle("parity")
        │   │       │
        │   │       ├─> If missing even:
        │   │       │   └─> healDataFromParity(ctx, remote, "even") [commands.go:560]
        │   │       │       ├─> Read odd + parity
        │   │       │       ├─> ReconstructFromOddAndParity()
        │   │       │       └─> uploadParticle("even")
        │   │       │
        │   │       └─> If missing odd:
        │   │           └─> healDataFromParity(ctx, remote, "odd")
        │   │               ├─> Read even + parity
        │   │               ├─> ReconstructFromEvenAndParity()
        │   │               └─> uploadParticle("odd")
        │   │
        │   └─> If ≤1/3 particles: ❌ Unrebuildable
        │
        └─> STEP 3: Return Report
            └─> Summary string with status
```

### Call Flow (All Files)

```
User: rclone backend heal raid3:
    │
    └─> backend/raid3.(*Fs).healCommand(ctx, [], opt)
        │
        ├─> STEP 1: Enumerate All Objects [LINES 437-443]
        │   └─> operations.ListFn(ctx, f, callback)
        │       └─> Collects all object remotes
        │
        ├─> STEP 2: Process Each Object [LINES 448-474]
        │   ├─> particleInfoForObject(ctx, remote)
        │   ├─> Count particles (0, 1, 2, or 3)
        │   │
        │   ├─> If 3/3: healthy++
        │   │
        │   ├─> If 2/3: healObject() → healed++
        │   │
        │   └─> If ≤1/3: unrebuildable++
        │
        └─> STEP 3: Return Summary Report
            └─> Statistics and list of unrebuildable objects
```

### Key Functions

#### healObject
**File**: `commands.go:496`

Heals a single object when exactly 2 of 3 particles exist.

#### healParityFromData
**File**: `commands.go:518`

Reconstructs missing parity from even+odd particles.

#### healDataFromParity
**File**: `commands.go:560`

Reconstructs missing data particle (even or odd) from the other data particle + parity.

#### uploadParticle
**File**: `heal.go:113`

Uploads a single particle to its backend (synchronous, not queued).

### Key Points

- **Requires 2/3 Particles**: Cannot heal if ≤1 particle present
- **Synchronous Upload**: Uses direct upload (not background queue)
- **Comprehensive Report**: Shows healthy, healed, and unrebuildable counts

---

## Rebuild Command

**User Command**: `rclone backend rebuild raid3: [even|odd|parity]`

**File**: `commands.go:204` - `rebuildCommand()`

### Call Flow

```
User: rclone backend rebuild raid3: odd
    │
    └─> backend/raid3.(*Fs).rebuildCommand(ctx, ["odd"], opt)
        │
        ├─> STEP 1: Detect Target Backend [LINES 220-240]
        │   ├─> If specified: Use provided backend name
        │   └─> If not: Auto-detect by scanning for missing particles
        │
        ├─> STEP 2: Scan for Missing Particles [LINES 250-280]
        │   └─> scanParticles(ctx, "") [particles.go:355]
        │       └─> Lists all objects, checks particle presence
        │
        ├─> STEP 3: Filter & Sort [LINES 290-320]
        │   ├─> Filter objects missing target backend particle
        │   └─> Sort by priority mode (auto, dirs-small, dirs, small)
        │
        ├─> STEP 4: Rebuild Loop [LINES 330-370]
        │   │
        │   └─> For each object:
        │       ├─> Determine which particles exist
        │       │
        │       ├─> If missing data particle (even/odd):
        │       │   └─> reconstructDataParticle() [particles.go:210]
        │       │       ├─> Read existing data + parity
        │       │       ├─> Reconstruct missing data
        │       │       └─> Upload to target backend
        │       │
        │       └─> If missing parity:
        │           └─> reconstructParityParticle() [particles.go:173]
        │               ├─> Read even + odd
        │               ├─> CalculateParity()
        │               └─> Upload to target backend
        │
        └─> STEP 5: Return Summary
            └─> Statistics (files rebuilt, data transferred, duration)
```

### Key Points

- **Complete Restoration**: Rebuilds entire backend after replacement
- **Priority Modes**: Different sorting strategies (directories first, small files first, etc.)
- **Progress Reporting**: Shows ETA and transfer speed

---

## Status Command

**User Command**: `rclone backend status raid3:`

**File**: `commands.go:43` - `statusCommand()`

### Call Flow

```
User: rclone backend status raid3:
    │
    └─> backend/raid3.(*Fs).statusCommand(ctx, opt)
        │
        ├─> STEP 1: Check All Backends [LINES 60-100]
        │   ├─> Test even backend (List + Mkdir)
        │   ├─> Test odd backend (List + Mkdir)
        │   └─> Test parity backend (List + Mkdir)
        │
        ├─> STEP 2: Determine Health Status [LINES 110-150]
        │   ├─> Count available backends (0, 1, 2, or 3)
        │   ├─> Identify which backend is unavailable
        │   └─> Assess impact (reads/writes)
        │
        ├─> STEP 3: Generate Report [LINES 160-200]
        │   ├─> Backend status (✅ Available / ❌ Unavailable)
        │   ├─> Impact assessment
        │   ├─> Rebuild guide (if degraded)
        │   └─> Step-by-step instructions
        │
        └─> STEP 4: Return Status String
            └─> Formatted report
```

### Key Points

- **Diagnostic Tool**: Primary tool for checking backend health
- **Rebuild Guidance**: Provides step-by-step rebuild instructions
- **Impact Assessment**: Shows what operations work in current state

---

## Background Heal Infrastructure

**File**: `heal.go`

### Auto-Heal on Read

When `auto_heal` is enabled and a degraded read occurs:

```
Object.Open() [object.go:206]
    │
    ├─> Detects missing particle (degraded mode)
    ├─> Reconstructs data
    └─> queueParticleUpload() [heal.go:147]
        │
        └─> Adds job to uploadQueue
            │
            └─> Background worker picks up job
                └─> uploadParticle() [heal.go:113]
```

### Background Workers

**File**: `heal.go:79` - `backgroundUploader()`

```
NewFs() [raid3.go:264]
    │
    └─> Starts N background workers (uploadWorkers goroutines)
        │
        └─> Each worker:
            ├─> Reads from uploadQueue.jobs channel
            ├─> Calls uploadParticle()
            └─> Marks job as done
```

**Configuration**: Number of workers controlled by backend options (default: 1)

---

## Code Locations Reference

| Component | File | Lines |
|-----------|------|-------|
| **Common** | | |
| NewFs (initialization) | `raid3.go` | 264-400 |
| Health check | `raid3.go` | 646-720 |
| Split bytes | `particles.go` | 26-41 |
| Calculate parity | `particles.go` | 64-79 |
| Merge bytes | `particles.go` | 44-60 |
| Disable retries | `helpers.go` | 57-88 |
| Rollback functions | `helpers.go` | 121-220 |
| **Commands** | | |
| Put | `raid3.go` | 1067-1172 |
| Open (Get) | `object.go` | 206-420 |
| List | `raid3.go` | 844-1000 |
| Remove | `object.go` | 735-771 |
| Update | `object.go` | 421-733 |
| Move | `raid3.go` | 1403-1570 |
| Mkdir | `raid3.go` | 1175-1209 |
| Rmdir | `raid3.go` | 1212-1273 |
| Heal command | `commands.go` | 386-493 |
| Rebuild command | `commands.go` | 204-384 |
| Status command | `commands.go` | 43-202 |
| **Heal Infrastructure** | | |
| Background workers | `heal.go` | 79-110 |
| Upload particle | `heal.go` | 113-145 |
| Queue particle | `heal.go` | 147-177 |
| Heal object | `commands.go` | 496-515 |
| Heal parity from data | `commands.go` | 518-557 |
| Heal data from parity | `commands.go` | 560-647 |

---

## Related Documentation

- [`README.md`](../README.md) - User guide
- [`RAID3.md`](../docs/RAID3.md) - Technical details on RAID 3
- [`TESTING.md`](../docs/TESTING.md) - Testing documentation
- [`OPEN_QUESTIONS.md`](../docs/OPEN_QUESTIONS.md) - Known limitations and future work
- [`STRICT_WRITE_POLICY.md`](../docs/STRICT_WRITE_POLICY.md) - Write policy details
- [`CLEAN_HEAL.md`](../docs/CLEAN_HEAL.md) - Self-maintenance documentation (healing and cleanup)

---

**Last Updated**: 2025-01-XX
