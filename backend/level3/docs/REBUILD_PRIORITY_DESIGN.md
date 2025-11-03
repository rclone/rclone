# Rebuild Priority Option Design

**Date**: November 2, 2025  
**Purpose**: Design the `-o priority` option for rebuild command  
**Status**: Design refinement based on user feedback

---

## 🎯 Design Evolution

### Original Proposal (Too Vague):

```bash
-o priority=dirs     # Directories first (then what?)
-o priority=small    # Small files first (what about dirs?)
-o priority=large    # Large files first (what about dirs?)
```

**Problems**:
- ❌ Ambiguous (dirs first, then files in what order?)
- ❌ Doesn't specify complete ordering
- ❌ "small" - does it create dirs first or as-needed?

---

### Refined Proposal (Clear & Complete): ✅

```bash
-o priority=auto          # Smart default (recommended)
-o priority=dirs-small    # All dirs, then all files (smallest first)
-o priority=dirs          # Dirs first, then files (alphabetic)
-o priority=small         # Dirs as-needed, files by size (smallest first)
```

**Benefits**:
- ✅ **Single value** (mutually exclusive, clear semantics)
- ✅ **Complete specification** (no ambiguity)
- ✅ **Self-documenting names**
- ✅ **Covers different use cases**

---

## 📋 Detailed Behavior Specification

### Mode 1: `priority=auto` (Default) ⭐

**Behavior**: Intelligent automatic ordering

**Algorithm**:
```
1. Create all directories/buckets first (alphabetic)
2. Rebuild files in discovery order (as listed)
3. Optimize for: Balance of speed and user satisfaction
```

**Why auto?**
- ✅ Most users won't know which to choose
- ✅ Works well for most scenarios
- ✅ Can be optimized based on heuristics

**Default heuristic**:
```
If (total files < 1000):
    use dirs-small (fast completion for small datasets)
Else:
    use dirs (predictable, shows progress evenly)
```

**Example**:
```bash
$ rclone backend rebuild level3: odd

Auto-detected priority: dirs-small (1,247 files)

Phase 1: Creating directories...
  [====================] 100% (42/42 dirs)
  
Phase 2: Rebuilding files (smallest first)...
  [=====>              ] 25% (312/1,247 files)
  Current: small-config.json (1.2 KB)
```

---

### Mode 2: `priority=dirs-small` (Fast Visible Progress)

**Behavior**: All dirs first, then ALL files by size (smallest first)

**Algorithm**:
```
1. Phase 1: Create all directories/buckets (alphabetic)
   → /dir1/, /dir2/, /dir3/...
   
2. Phase 2: Rebuild all files (by size, smallest first)
   → dir1/tiny.txt (1 KB)
   → dir3/small.json (5 KB)
   → dir2/medium.doc (50 KB)
   → dir1/large-video.mp4 (500 MB)
   ...
```

**Use case**: 
- Fast visible progress (many small files complete quickly)
- User sees "80% complete" quickly (even if data remaining)
- Good for: Lots of small config files + few large media files

**Example**:
```
Progress: [=================>  ] 85% (1,060/1,247 files)
  But only 45% of data transferred (many large files pending)
```

**Pros**:
- ✅ Quick early wins (structure restored fast)
- ✅ High file count percentage early
- ✅ Feels faster (psychological)

**Cons**:
- ⚠️ File percentage doesn't match data percentage
- ⚠️ Large files at end (slow finish)

---

### Mode 3: `priority=dirs` (Predictable, Simple)

**Behavior**: Dirs first, then files in alphabetic order per directory

**Algorithm**:
```
1. Phase 1: Create all directories/buckets (alphabetic)
   → /dir1/, /dir2/, /dir3/...
   
2. Phase 2: Rebuild files directory-by-directory (alphabetic)
   → dir1/aaa.txt
   → dir1/bbb.doc
   → dir1/zzz.mp4
   → dir2/aaa.json
   → dir2/bbb.txt
   ...
```

**Use case**:
- Predictable progress
- One directory at a time
- Good for: Organized directory structures

**Example**:
```
Phase 1: Directories [====================] 100% (42/42)
Phase 2: Files in dir1/ [==========] 50% (25/50)
  Current: dir1/report.pdf
Phase 2: Files in dir2/ [====>     ] 20% (10/50)
  Current: dir2/config.json
```

**Pros**:
- ✅ Predictable (know what comes next)
- ✅ Directory-by-directory completion
- ✅ Easy to understand progress

**Cons**:
- ⚠️ Slow if first dir has large files
- ⚠️ Not optimized for quick wins

---

### Mode 4: `priority=small` (Incremental Completion)

**Behavior**: Create dirs as-needed, rebuild files by size (smallest first)

**Algorithm**:
```
1. Sort all files by size (smallest first)
2. For each file:
   a. Create parent directory if doesn't exist
   b. Rebuild file
   
Order:
   → Create /dir3/
   → Rebuild dir3/tiny.txt (1 KB)
   → Create /dir1/
   → Rebuild dir1/small.json (5 KB)
   → Create /dir2/
   → Rebuild dir2/medium.doc (50 KB)
   ...
```

**Use case**:
- Maximum early file completion
- Quick wins (many files done fast)
- Good for: Mixed structure with size variation

**Example**:
```
Rebuilding (smallest files first)...
  [=================>  ] 85% (1,060/1,247 files, 250 MB/2.3 GB)
  Current: large-video.mp4 (500 MB)
  
Many files complete, but significant data remaining
```

**Pros**:
- ✅ Highest file count percentage early
- ✅ Quick wins (psychological benefit)
- ✅ Useful files available sooner (configs, etc.)

**Cons**:
- ⚠️ Directory creation scattered
- ⚠️ File % != data % (misleading)
- ⚠️ Large files create long tail

---

## 📊 Comparison Table

| Mode | Dirs Created | File Order | File % Matches Data %? | Use Case |
|------|--------------|------------|----------------------|----------|
| **auto** | First | Smart | Varies | Default (best guess) |
| **dirs-small** | All first | By size (global) | No | Quick visible progress |
| **dirs** | All first | Alphabetic per-dir | Yes (roughly) | Predictable |
| **small** | As-needed | By size (global) | No | Maximum early completion |

---

## 🎯 Recommendation

### Default: `priority=auto`

**Benefits**:
- ✅ Works well for most users
- ✅ Can optimize based on dataset
- ✅ Users don't need to understand options

**Heuristic** (simple):
```go
if totalFiles < 500 {
    // Small dataset: dirs-small (fast completion feeling)
} else if largestFile > (totalSize / 10) {
    // Few large files: dirs (predictable)
} else {
    // Balanced: dirs-small (structure first, progress visible)
}
```

---

### Advanced Users Can Override:

```bash
# I have many small config files, want quick wins
$ rclone backend rebuild level3: -o priority=small

# I have large media library, want predictable progress  
$ rclone backend rebuild level3: -o priority=dirs

# I want structure first, then sorted completion
$ rclone backend rebuild level3: -o priority=dirs-small
```

---

## 💡 Additional Considerations

### Progress Display Per Mode:

**dirs-small**:
```
Phase 1: Directories    [====================] 100% (42/42)
Phase 2: Files          [======>             ] 30% (376/1,247)
         By size:       [===>                ] 15% (350 MB/2.3 GB)
         Current: medium-doc.pdf (25 MB)
```

**dirs**:
```
Phase 1: Directories    [====================] 100% (42/42)
Phase 2: dir1/          [==========          ] 50% (25/50)
         dir2/          [                    ]  0% (0/50)
         Overall:       [====>               ] 20% (250/1,247)
         Current: dir1/report.pdf (15 MB)
```

**small**:
```
Rebuilding smallest files first...
  Files:    [=================>  ] 85% (1,060/1,247)
  Data:     [======>             ] 30% (700 MB/2.3 GB)
  Dirs created: 38/42
  Current: large-movie.mp4 (500 MB)
```

---

## ✅ Design Decision

### Agree with Your Proposal: ✅

**Single-value option**:
```bash
-o priority=auto|dirs-small|dirs|small
```

**Not multiple options**:
```bash
# ❌ BAD (confusing):
-o priority=dirs -o priority=small
-o sort=size -o structure=dirs-first
```

**Rationale**:
- ✅ Clear semantics (mutually exclusive)
- ✅ Easy to understand
- ✅ No confusion about combining
- ✅ Simple to implement
- ✅ Standard pattern

---

## 🎨 Refined Option Specification

### Option Definition:

```go
type RebuildOptions struct {
    Priority    string  // auto|dirs-small|dirs|small
    CheckOnly   bool    // Analyze without rebuilding
    DryRun      bool    // Show what would be done
    Parallel    int     // Concurrent uploads (default: 4)
    MaxSize     int64   // Only files < this size
    Verify      bool    // Verify after rebuild (default: true)
    Force       bool    // Overwrite existing (default: false)
}
```

---

### Help Text:

```
Options:

-o priority=MODE
   Rebuild order:
   
   auto        Smart default based on dataset (recommended)
   dirs-small  All directories first, then files by size (smallest first)
               Best for: Quick visible progress, many small files
   dirs        All directories first, then files alphabetically per directory
               Best for: Predictable progress, organized structure
   small       Create directories as-needed, files by size (smallest first)
               Best for: Maximum early file completion, mixed sizes
   
   Default: auto

Examples:
   -o priority=dirs-small   # Structure first, small files prioritized
   -o priority=dirs         # Alphabetic, predictable
   -o priority=small        # Size-based, maximum early completion
```

---

## 🎯 Your Specific Values

### `auto` (Default):
✅ **Excellent choice**
- Let system decide based on dataset
- Best for most users
- Can improve heuristic over time

### `dirs-small`:
✅ **Very useful**
- Your description: "Create all dirs, then all files (smallest first)"
- Use case: Quick structure restoration + fast progress feeling
- **Recommendation**: Make this the default in `auto` mode

### `dirs`:
✅ **Good for predictability**
- Your description: "Dirs first, files alphabetic per directory"
- Use case: Organized restoration, one dir at a time
- Predictable completion per directory

### `small`:
✅ **Good for quick wins**
- Your description: "Dirs as-needed, files by size"
- Use case: Get most files back quickly
- Useful when have few large files

---

## 💡 Suggested Enhancement: Add `large`?

**Question**: Should we also have `-o priority=large`?

**Use case**: 
- User wants large files back first (media library)
- Opposite of `small`

**Behavior**:
```
Dirs as-needed, files by size (LARGEST first)
```

**Example**:
```bash
$ rclone backend rebuild level3: -o priority=large

Rebuilding largest files first...
  [===>                ] 15% (45/1,247 files, 1.8 GB/2.3 GB)
  Current: huge-backup.tar.gz (800 MB)
  
Only 15% of files, but 78% of data complete!
```

**Pros**:
- ✅ Complementary to `small`
- ✅ Useful for media libraries
- ✅ Get bulk data back fast

**Your decision**: Include `large` or not?

---

## ✅ Final Recommendation

### Option values:

```bash
-o priority=auto          # Default (smart heuristic)
-o priority=dirs-small    # All dirs, then files by size ↑
-o priority=dirs          # Dirs first, files alphabetic
-o priority=small         # Incremental dirs, files by size ↑
-o priority=large         # Incremental dirs, files by size ↓ (optional)
```

### Single-value constraint: ✅ **YES**

**Why**:
- ✅ Mutually exclusive (can't be both alphabetic AND size-sorted)
- ✅ Clear semantics
- ✅ No confusion
- ✅ Standard pattern

**Implementation**:
```go
priority := opt["priority"]
if priority == "" {
    priority = "auto"
}

switch priority {
case "auto":
    // Apply heuristic
case "dirs-small":
    // All dirs, then files by size
case "dirs":
    // Dirs, then files alphabetic
case "small":
    // Incremental dirs, files by size ascending
case "large":
    // Incremental dirs, files by size descending
default:
    return fmt.Errorf("invalid priority: %s (use: auto, dirs-small, dirs, small)", priority)
}
```

---

## 🎨 Example Usage

### Scenario 1: General Restore

```bash
$ rclone backend rebuild level3:
# Uses: priority=auto
# Behavior: Decides based on dataset (probably dirs-small)
```

---

### Scenario 2: Want Structure Back Fast

```bash
$ rclone backend rebuild level3: -o priority=dirs-small

Phase 1: All directories  [====================] 100% (42/42)
  ✅ Directory structure restored

Phase 2: Files (smallest first)
  [========>           ] 40% (500/1,247 files)
  Many small files done quickly!
```

---

### Scenario 3: Want Predictable Progress

```bash
$ rclone backend rebuild level3: -o priority=dirs

Rebuilding by directory (alphabetic)...
  [======              ] 30% completed
  
  ✅ /backup/ (50 files, 250 MB)
  ✅ /configs/ (120 files, 15 MB)
  ⏳ /media/ (30% - 15/50 files)
     Current: media/video3.mp4 (200 MB)
  ⏸️ /photos/ (not started)
```

---

### Scenario 4: Want Small Files Back Quickly

```bash
$ rclone backend rebuild level3: -o priority=small

Rebuilding smallest files first (dirs as-needed)...
  [=================>  ] 85% (1,060/1,247 files)
  Data: 30% (700 MB/2.3 GB)
  
  ✅ 1,000+ small files complete
  ⏳ 187 large files remaining
```

---

## 📊 When to Use Which Mode

| Mode | Best For | File/Data Progress | Speed Feeling |
|------|----------|-------------------|---------------|
| **auto** | Most users | Balanced | Good |
| **dirs-small** | Quick structure + progress | Files >> Data | Fast feeling |
| **dirs** | Organized libraries | Files ≈ Data | Steady |
| **small** | Config-heavy + few large | Files >> Data | Fast early |
| **large** | Media libraries | Data >> Files | Slow early, fast data |

---

## ✅ Advantages of Your Design

### 1. Single-Value Constraint ✅

**Your suggestion**: "one that can be added only one per call"

**Benefits**:
- ✅ No confusion (can't do `small AND large`)
- ✅ Clear semantics (one mode at a time)
- ✅ Easy to validate
- ✅ Matches other rclone options

**Implementation**:
```go
priority := opt["priority"]
// If user somehow specifies twice, last one wins
// This is standard rclone behavior
```

---

### 2. Descriptive Mode Names ✅

**Your names are better**:
- `dirs-small` - Immediately clear what it does
- `dirs` - Simple, predictable
- `small` - Size-focused

**vs. Original**:
- `dirs` - Ambiguous (then what?)
- `small` - Doesn't mention dirs

---

### 3. Complete Ordering Specification ✅

**Each mode fully specifies**:
- When dirs are created
- How files are ordered
- Complete workflow

**No ambiguity!**

---

## 🎯 Final Spec for Implementation

### Option: `priority`

**Type**: String (single value)

**Values**:
- `auto` - Smart default (recommended) ⭐
- `dirs-small` - All dirs, then files smallest-first
- `dirs` - Dirs first, files alphabetic per-dir
- `small` - Dirs as-needed, files smallest-first globally
- `large` - Dirs as-needed, files largest-first globally (optional)

**Default**: `auto`

**Validation**: Must be one of the above values

**Help Text**:
```
-o priority=MODE

  Determines rebuild order for optimal user experience.
  
  Modes:
    auto        Smart default based on your dataset (recommended)
    dirs-small  All directories first, then all files (smallest first)
                → Quick structure restoration + fast visible progress
    dirs        Directories first, then files alphabetically per directory
                → Predictable, organized completion
    small       Create directories as-needed, files by size (smallest first)
                → Maximum file count early, useful files first
    large       Create directories as-needed, files by size (largest first)
                → Bulk data transfer first, slower file count progress
  
  Default: auto
  
  Examples:
    -o priority=dirs-small    # Fast visible progress
    -o priority=dirs          # Predictable, organized
    -o priority=small         # Quick wins, config files first
```

---

## ✨ Summary

**Your design is superior**: ✅

**Why**:
1. ✅ **Single-value** (clear, no confusion)
2. ✅ **Complete specification** (each mode fully defined)
3. ✅ **Descriptive names** (self-documenting)
4. ✅ **Covers use cases** (different user needs)
5. ✅ **Has smart default** (`auto` for most users)

**Refinements made**:
- Added detailed behavior specs for each mode
- Added `auto` mode with heuristic
- Suggested optional `large` mode (your decision)
- Created complete examples
- Defined progress display per mode

**Ready for implementation**: ✅

**This design should go into the implementation when we build the rebuild command!**

---

**Excellent design thinking!** Your refinement makes the option much more useful and clear! 🎯

