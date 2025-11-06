# Backend-Specific Commands Analysis for level3

**Date**: November 4, 2025  
**Purpose**: Research backend-specific commands to determine if level3 should support them  
**Context**: When all three remotes (even, odd, parity) use the same backend type, should level3 expose backend commands?

---

## 🎯 Research Question

**Should level3 support backend-specific commands when all three remotes are the same backend type?**

### Possible Approaches:
1. **Do not support** backend commands at all
2. **Support a subset** of commands (abstraction layer)
3. **Support all** commands (pass-through)

---

## 📊 Inventory of Backend Commands

### Major Storage Backends

#### **S3 (Amazon S3 and compatible)**
```
Commands: 7
1. restore               - Restore objects from GLACIER/INTELLIGENT-TIERING
2. restore-status        - Show restore status for archived objects
3. list-multipart-uploads - List unfinished multipart uploads
4. cleanup              - Remove old multipart uploads
5. cleanup-hidden       - Remove old versions of files
6. versioning           - Set/get versioning for bucket
7. set                  - Update config parameters (access_key, secret_key, etc.)
```

**Category**: Storage management, version control, lifecycle

---

#### **Google Drive**
```
Commands: 11
1. get                  - Get config parameters
2. set                  - Update config parameters (service_account, chunk_size)
3. shortcut             - Create shortcuts from files/directories
4. drives               - List Shared Drives (Team Drives)
5. untrash              - Restore files from trash
6. copyid               - Copy files by ID
7. moveid               - Move files by ID
8. exportformats        - Dump export formats (debug)
9. importformats        - Dump import formats (debug)
10. query               - List files using Google Drive query language
11. rescue              - Rescue or delete orphaned files
```

**Category**: ID-based operations, Drive-specific features, metadata

---

#### **B2 (Backblaze B2)**
```
Commands: 3
1. lifecycle            - Manage lifecycle rules
2. cleanup              - Remove old multipart uploads
3. cleanup-hidden       - Remove old file versions
```

**Category**: Version control, cleanup

---

### Virtual/Wrapper Backends

#### **Crypt**
```
Commands: 2
1. encode               - Encode file names
2. decode               - Decode file names
```

**Category**: Filename encryption/decryption

---

#### **Cache**
```
Commands: 1
1. stats                - Show cache statistics
```

**Category**: Performance monitoring

---

#### **Hasher**
```
Commands: 5
1. drop                 - Drop hash cache
2. dump                 - Dump hash database
3. fulldump             - Full dump of hash database
4. import               - Import a SUM file
5. stickyimport         - Fast import of SUM file
```

**Category**: Hash management

---

### Specialized Backends

#### **Local**
```
Commands: 1
1. noop                 - No operation (test command)
```

**Category**: Testing

---

#### **HTTP**
```
Commands: 1
1. set                  - Update config parameters
```

**Category**: Configuration

---

#### **DOI (Digital Object Identifier)**
```
Commands: 2
1. metadata             - Get DOI metadata
2. set                  - Update config
```

**Category**: Metadata retrieval

---

#### **PikPak**
```
Commands: 2
1. addurl               - Add URL for download
2. decompress           - Decompress archived files
```

**Category**: Cloud download service features

---

#### **NetStorage (Akamai)**
```
Commands: 2
1. du                   - Disk usage
2. symlink              - Create symlink
```

**Category**: File system operations

---

#### **Oracle Object Storage**
```
Commands: 0 (but has command.go file for future)
```

---

## 🔍 Command Categories Analysis

### Category 1: **Configuration Management** (8 backends)
**Commands**: `get`, `set`

**Purpose**: Update runtime configuration (credentials, chunk sizes, etc.)

**Examples**:
- S3: `set` - Update access_key, secret_key, session_token
- Drive: `get/set` - Update service_account_file, chunk_size
- HTTP: `set` - Update headers, URL
- DOI: `set` - Update config

**Common Pattern**: All provide runtime config updates without restart

---

### Category 2: **Cleanup/Maintenance** (S3, B2)
**Commands**: `cleanup`, `cleanup-hidden`

**Purpose**: Remove old multipart uploads, old file versions

**Examples**:
- S3: `cleanup` - Remove uploads > 24h old
- S3: `cleanup-hidden` - Remove old versions
- B2: Same commands, similar purpose

**Common Pattern**: Housekeeping for versioned/multipart storage

---

### Category 3: **Version/Lifecycle Management** (S3, B2)
**Commands**: `versioning`, `lifecycle`

**Purpose**: Manage object versions and lifecycle rules

**Examples**:
- S3: `versioning` - Enable/disable versioning
- B2: `lifecycle` - Set lifecycle rules
- S3: `restore` - Restore from GLACIER

**Common Pattern**: Backend-specific storage tier/version management

---

### Category 4: **Backend-Specific Features** (Drive, PikPak, NetStorage)
**Commands**: `shortcut`, `copyid`, `drives`, `untrash`, `addurl`, `decompress`, `symlink`

**Purpose**: Features unique to the backend

**Examples**:
- Drive: `shortcut` - Google Drive shortcuts
- Drive: `drives` - List Team Drives
- Drive: `copyid/moveid` - Operations by file ID
- PikPak: `addurl` - Cloud download
- NetStorage: `symlink` - Akamai symlinks

**Common Pattern**: No abstraction possible (too specific)

---

### Category 5: **Diagnostic/Debug** (Drive, Hasher, Cache)
**Commands**: `exportformats`, `importformats`, `stats`, `dump`, `query`

**Purpose**: Inspect backend state, performance, metadata

**Examples**:
- Drive: `query` - Drive query language
- Hasher: `dump` - Dump hash database
- Cache: `stats` - Cache statistics
- Drive: `exportformats` - Debug export formats

**Common Pattern**: Read-only inspection

---

### Category 6: **Data Management** (Drive, Hasher)
**Commands**: `rescue`, `untrash`, `import`

**Purpose**: Data recovery, import/export

**Examples**:
- Drive: `rescue` - Find orphaned files
- Drive: `untrash` - Restore from trash
- Hasher: `import` - Import SUM file

**Common Pattern**: Recovery operations

---

## 🧩 Commonalities Across Backends

### Pattern 1: **Configuration Commands** ✅
**Commands**: `get`, `set`  
**Backends**: S3, Drive, HTTP, DOI (8 total)  
**Abstraction Potential**: **HIGH**

**Why**: All backends need runtime config updates

**Possible level3 Implementation**:
```bash
# Update all three remotes' config simultaneously
rclone backend set level3: -o access_key=XXX -o secret_key=YYY

# Would call:
#   f.even.Command("set", ..., {access_key:XXX, secret_key:YYY})
#   f.odd.Command("set", ..., {access_key:XXX, secret_key:YYY})
#   f.parity.Command("set", ..., {access_key:XXX, secret_key:YYY})
```

**Benefit**: Keep all three remotes' credentials in sync!

---

### Pattern 2: **Cleanup Commands** ✅
**Commands**: `cleanup`, `cleanup-hidden`  
**Backends**: S3, B2 (2 total)  
**Abstraction Potential**: **MEDIUM**

**Why**: Multipart/versioning cleanup is common in cloud storage

**Possible level3 Implementation**:
```bash
# Clean up all three remotes
rclone backend cleanup level3:bucket/path -o max-age=7d

# Would call cleanup on all three remotes in parallel
```

**Benefit**: Single command cleans all three remotes

---

### Pattern 3: **Version Management** ⚠️
**Commands**: `versioning`, `lifecycle`, `restore`  
**Backends**: S3, B2  
**Abstraction Potential**: **LOW**

**Why**: Very backend-specific (GLACIER, B2 tiers, etc.)

**Problem for level3**:
- Particles don't have independent versions
- Versioning would need to track even+odd+parity as a set
- Complexity: High
- Benefit: Unclear

**Recommendation**: **Do not support** in level3

---

### Pattern 4: **Backend-Specific Features** ❌
**Commands**: `shortcut`, `copyid`, `drives`, `addurl`, etc.  
**Backends**: Drive, PikPak, NetStorage  
**Abstraction Potential**: **NONE**

**Why**: Too specific to individual backend

**Examples**:
- Drive `shortcut` - Google Drive only
- PikPak `addurl` - Cloud downloader only
- NetStorage `symlink` - Akamai only

**Recommendation**: **Cannot support** (no common abstraction)

---

### Pattern 5: **Diagnostic Commands** ⚠️
**Commands**: `stats`, `dump`, `query`  
**Backends**: Cache, Hasher, Drive  
**Abstraction Potential**: **MEDIUM** (read-only)

**Why**: Inspecting state is useful

**Possible level3 Implementation**:
```bash
# Get stats from all three remotes
rclone backend stats level3:

# Returns:
{
  "even": {...},
  "odd": {...},
  "parity": {...}
}
```

**Benefit**: Unified view of all three remotes

---

## 💡 Analysis: What Works for level3?

### ✅ **Commands That Make Sense for level3**

#### 1. **Configuration: `get` / `set`**
**Reason**: Keep all three remotes' config synchronized

**Use Cases**:
- Rotate credentials across all three remotes
- Update chunk sizes uniformly
- Change timeout settings

**Implementation**: Broadcast to all three remotes

**Priority**: **HIGH** ⭐⭐⭐

---

#### 2. **Cleanup: `cleanup`**
**Reason**: Clean up orphaned particles on all remotes

**Use Cases**:
- Remove old multipart uploads from failed level3 uploads
- Clean up after interrupted operations

**Implementation**: Parallel cleanup on all three remotes

**Priority**: **MEDIUM** ⭐⭐

---

#### 3. **Diagnostics: Custom level3 commands**
**Reason**: level3-specific health and status

**Use Cases**:
- `status` - Check backend health (✅ **already implemented!**)
- `rebuild` - Rebuild missing particles (✅ **already implemented!**)
- `verify` - Verify parity integrity (future)

**Implementation**: level3-specific logic (not pass-through)

**Priority**: **HIGH** ⭐⭐⭐ (already done!)

---

### ❌ **Commands That Don't Make Sense for level3**

#### 1. **Version Management** (`versioning`, `lifecycle`, `restore`)
**Reason**: Particle-level versioning is complex and unclear benefit

**Problems**:
- Need to version even+odd+parity as a set
- Which particle gets the version marker?
- Restoration requires all three particles' versions to match
- GLACIER restore would need all three particles restored

**Recommendation**: **Do not support**

---

#### 2. **Backend-Specific Features** (`shortcut`, `copyid`, `drives`, etc.)
**Reason**: No common abstraction possible

**Problems**:
- Each backend's features are unique
- No way to generalize across backends
- Users should use backend directly for these

**Recommendation**: **Do not support**

---

#### 3. **ID-Based Operations** (`copyid`, `moveid`)
**Reason**: level3 has particles, not single objects with IDs

**Problems**:
- level3 object = even particle + odd particle + parity particle
- Each particle might have different ID
- No single "object ID" exists

**Recommendation**: **Do not support**

---

## 🎯 Recommendations for level3

### **Option A: Do Not Support Backend Commands** ⚠️

**Pros**:
- Simple implementation (no code needed)
- No complexity
- No maintenance burden

**Cons**:
- Users can't update credentials easily
- Can't clean up all three remotes at once
- Less convenient

**Verdict**: **Not recommended** - Missing useful functionality

---

### **Option B: Support Subset (Abstraction Layer)** ✅ **RECOMMENDED**

**Support These Commands**:

1. **`set`** - Update config on all three remotes
   - Implementation: Broadcast to all three
   - Priority: **HIGH** ⭐⭐⭐
   
2. **`get`** - Get config from backends
   - Implementation: Return from first remote (or all three)
   - Priority: **MEDIUM** ⭐⭐

3. **`cleanup`** - Clean up orphaned uploads
   - Implementation: Parallel cleanup on all three
   - Priority: **MEDIUM** ⭐⭐

**Custom level3 Commands** (already implemented):
- **`status`** ✅ - Backend health check
- **`rebuild`** ✅ - Rebuild missing particles

**Pros**:
- ✅ Useful functionality
- ✅ Maintains abstraction
- ✅ Reasonable complexity
- ✅ Solves real problems (credential rotation, cleanup)

**Cons**:
- ⚠️ Some implementation effort
- ⚠️ Need to handle errors from multiple remotes

**Verdict**: **RECOMMENDED** ⭐⭐⭐

---

### **Option C: Support All Commands** (Pass-Through) ❌

**Pros**:
- Complete feature parity

**Cons**:
- ❌ Doesn't make sense (versioning on particles?)
- ❌ Complex error handling
- ❌ Breaks level3 abstraction
- ❌ Confusing for users (which remote gets the command?)
- ❌ High maintenance burden

**Verdict**: **Not recommended**

---

## 📋 Implementation Plan (if Option B chosen)

### Phase 1: Configuration Commands ⭐⭐⭐
**Priority**: HIGH  
**Commands**: `set`, `get`

**Implementation**:
```go
func (f *Fs) Command(ctx context.Context, name string, arg []string, opt map[string]string) (out any, err error) {
    switch name {
    case "status":  // ✅ Already implemented
        return f.statusCommand(ctx, opt)
    case "rebuild": // ✅ Already implemented
        return f.rebuildCommand(ctx, arg, opt)
    
    // NEW COMMANDS:
    case "set":
        return f.setCommand(ctx, arg, opt)
    case "get":
        return f.getCommand(ctx, arg, opt)
    case "cleanup":
        return f.cleanupCommand(ctx, arg, opt)
    default:
        return nil, fs.ErrorCommandNotFound
    }
}

func (f *Fs) setCommand(ctx context.Context, arg []string, opt map[string]string) (out any, err error) {
    // Check if underlying backends support Command
    evenCmd := f.even.Features().Command
    oddCmd := f.odd.Features().Command
    parityCmd := f.parity.Features().Command
    
    if evenCmd == nil || oddCmd == nil || parityCmd == nil {
        return nil, errors.New("not all backends support commands")
    }
    
    // Check if all backends are same type
    evenType := f.even.Name()
    if f.odd.Name() != evenType || f.parity.Name() != evenType {
        return nil, errors.New("all three backends must be same type for set command")
    }
    
    // Call set on all three backends in parallel
    g, gCtx := errgroup.WithContext(ctx)
    var evenOut, oddOut, parityOut any
    
    g.Go(func() error {
        var err error
        evenOut, err = evenCmd(gCtx, "set", arg, opt)
        return err
    })
    
    g.Go(func() error {
        var err error
        oddOut, err = oddCmd(gCtx, "set", arg, opt)
        return err
    })
    
    g.Go(func() error {
        var err error
        parityOut, err = parityCmd(gCtx, "set", arg, opt)
        return err
    })
    
    if err := g.Wait(); err != nil {
        return nil, fmt.Errorf("set failed on one or more backends: %w", err)
    }
    
    // Return combined results
    return map[string]any{
        "even":   evenOut,
        "odd":    oddOut,
        "parity": parityOut,
    }, nil
}
```

**Testing**:
- Test with S3 backends (all three S3)
- Test with mixed backends (should error)
- Test with backends that don't support `set`

**Estimated Effort**: 4-6 hours

---

### Phase 2: Cleanup Commands ⭐⭐
**Priority**: MEDIUM  
**Commands**: `cleanup`

**Implementation**: Similar to `set`, broadcast to all three remotes

**Estimated Effort**: 2-3 hours

---

### Phase 3: Documentation
**Tasks**:
- Update README with supported commands
- Add examples to docs
- Update commandHelp

**Estimated Effort**: 1-2 hours

---

## 🎯 Final Recommendation

### **Implement Option B: Support Subset of Commands** ⭐⭐⭐

**Rationale**:

1. **Useful Functionality**:
   - `set` solves credential rotation problem
   - `cleanup` helps with housekeeping
   - Already have `status` and `rebuild`

2. **Maintains Abstraction**:
   - Only support commands that make sense for RAID 3
   - Don't expose particle-level details
   - Keep level3 as unified interface

3. **Reasonable Complexity**:
   - ~10-15 hours total effort
   - Clean implementation
   - Good test coverage possible

4. **User Benefit**:
   - Easier management of all three remotes
   - Better than manual operations on each remote
   - Consistent with level3 philosophy (unified access)

---

## 📊 Summary Table

| Command | Backends | level3 Support? | Priority | Effort |
|---------|----------|-----------------|----------|--------|
| **Configuration** |
| `get` | S3, Drive, HTTP, DOI | ✅ **YES** | HIGH ⭐⭐⭐ | 3h |
| `set` | S3, Drive, HTTP, DOI | ✅ **YES** | HIGH ⭐⭐⭐ | 4h |
| **Cleanup** |
| `cleanup` | S3, B2 | ✅ **YES** | MEDIUM ⭐⭐ | 3h |
| `cleanup-hidden` | S3, B2 | ⚠️ Maybe | LOW ⭐ | 2h |
| **Version Management** |
| `versioning` | S3, B2 | ❌ **NO** | - | - |
| `lifecycle` | B2 | ❌ **NO** | - | - |
| `restore` | S3 | ❌ **NO** | - | - |
| **Backend-Specific** |
| `shortcut`, `copyid`, `drives`, etc. | Various | ❌ **NO** | - | - |
| **Diagnostics** |
| `status` | level3 | ✅ **DONE** ✅ | HIGH ⭐⭐⭐ | 0h (complete) |
| `rebuild` | level3 | ✅ **DONE** ✅ | HIGH ⭐⭐⭐ | 0h (complete) |

**Total Effort**: ~10-15 hours for recommended commands

---

## 🔗 Related Documents

- `USER_CENTRIC_RECOVERY.md` - Existing `status` and `rebuild` commands
- `RAID3_VS_RAID5_ANALYSIS.md` - Why RAID 3 architecture is sound
- `level3.go` - Current Command implementation

---

**Conclusion**: level3 should support a **carefully chosen subset** of backend commands (`set`, `get`, `cleanup`) that maintain the RAID 3 abstraction while providing useful management functionality. Commands that don't make sense for particle-based storage (versioning, ID-based operations, backend-specific features) should not be supported.

**Next Steps**: Decide whether to implement Option B, and if so, proceed with Phase 1 (configuration commands).



