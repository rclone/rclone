# Virtual Backend File Organization Analysis

## Question
Are there other rclone virtual backends which split the core functionality into more than 5 files?

## Answer: Yes, Several Backends Have More Than 5 Files

### Virtual Backends (that wrap other backends)

| Backend | Total Files | Main Files (non-test) | Organization Pattern |
|---------|-------------|----------------------|---------------------|
| **union** | 24 files | 3 main + subdirectories | Main files + policy/ + upstream/ + common/ |
| **cache** | 12 files | 7 main files | Single directory, multiple concerns |
| **hasher** | 6 files | 4 main files | Single directory, feature separation |
| **crypt** | 7 files | 2 main files | Main + cipher separation |
| **chunker** | 3 files | 1 main file | Single file |
| **combine** | 3 files | 1 main file | Single file |
| **compress** | 3 files | 1 main file | Single file |
| **alias** | 2 files | 1 main file | Single file |

## Detailed Analysis

### 1. Union Backend (24 files total)

**Structure:**
```
union/
├── union.go              # Main Fs implementation
├── entry.go              # Object and Directory types
├── errors.go             # Error types and utilities
├── common/
│   └── options.go        # Shared options
├── policy/
│   ├── policy.go        # Policy interface
│   ├── all.go           # All policy implementations
│   ├── epall.go         # EPALL policy
│   ├── epff.go          # EPFF policy
│   ├── eplfs.go         # EPLFS policy
│   ├── eplno.go         # EPLNO policy
│   ├── eplus.go         # EPLUS policy
│   ├── epmfs.go         # EPMFS policy
│   ├── eprand.go        # EPRAND policy
│   ├── ff.go            # FF policy
│   ├── lfs.go           # LFS policy
│   ├── lno.go           # LNO policy
│   ├── lus.go           # LUS policy
│   ├── mfs.go           # MFS policy
│   ├── newest.go        # NEWEST policy
│   └── rand.go          # RAND policy
└── upstream/
    └── upstream.go      # Upstream Fs wrapper
```

**Main package files:** 3 (union.go, entry.go, errors.go)
**Subdirectories:** policy/ (16 files), upstream/ (1 file), common/ (1 file)

**Pattern:** Main functionality in root, complex features in subdirectories

### 2. Cache Backend (12 files, 7 main)

**Structure:**
```
cache/
├── cache.go              # Main Fs implementation
├── directory.go          # Directory operations
├── handle.go             # Handle management
├── object.go             # Object operations
├── plex.go               # Plex integration
├── storage_memory.go      # Memory storage backend
├── storage_persistent.go  # Persistent storage backend
├── cache_test.go
├── cache_internal_test.go
├── cache_upload_test.go
├── cache_unsupported.go
└── utils_test.go
```

**Main files:** 7 (cache.go, directory.go, handle.go, object.go, plex.go, storage_memory.go, storage_persistent.go)

**Pattern:** Single directory, separation by concern (Fs, Directory, Object, Storage backends)

### 3. Hasher Backend (6 files, 4 main)

**Structure:**
```
hasher/
├── hasher.go             # Main Fs implementation
├── commands.go           # Backend commands
├── kv.go                 # Key-value storage
├── object.go             # Object operations
├── hasher_test.go
└── hasher_internal_test.go
```

**Main files:** 4 (hasher.go, commands.go, kv.go, object.go)

**Pattern:** Single directory, feature-based separation

### 4. Crypt Backend (7 files, 2 main)

**Structure:**
```
crypt/
├── crypt.go              # Main Fs and Object implementation
├── cipher.go             # Encryption/decryption logic
├── crypt_test.go
├── crypt_internal_test.go
├── cipher_test.go
└── ... (other test files)
```

**Main files:** 2 (crypt.go, cipher.go)

**Pattern:** Separation of filesystem logic from cryptographic operations

## Comparison with Level3

### Current Level3 Structure
```
level3/
├── level3.go             # ~4000 lines - ALL functionality
├── level3_test.go
├── level3_internal_test.go
└── ... (other test files)
```

**Main files:** 1 (level3.go)

### Proposed Level3 Structure (from analysis)
```
level3/
├── level3.go             # ~500 lines - Main Fs, registration
├── object.go             # ~800 lines - Object and Directory
├── commands.go           # ~1000 lines - Commands (status, rebuild, heal)
├── particles.go          # ~600 lines - Particle operations
├── healing.go            # ~400 lines - Self-healing infrastructure
└── helpers.go            # ~300 lines - Utility functions
```

**Main files:** 6 (proposed)

## Key Findings

### 1. Union Backend - Most Complex Organization
- **24 files total** across main package and subdirectories
- Uses subdirectories for complex features (policies, upstreams)
- Main package has 3 core files
- **Pattern:** Root files for core, subdirectories for features

### 2. Cache Backend - Single Directory, Multiple Files
- **7 main files** in single directory
- Clear separation by concern:
  - Main Fs (cache.go)
  - Directory operations (directory.go)
  - Object operations (object.go)
  - Storage backends (storage_memory.go, storage_persistent.go)
  - Features (handle.go, plex.go)
- **Pattern:** Feature-based file organization in single directory

### 3. Hasher Backend - Moderate Complexity
- **4 main files** in single directory
- Separation: Main Fs, Commands, Storage, Objects
- **Pattern:** Feature-based separation

### 4. Crypt Backend - Minimal Split
- **2 main files**
- Clear separation: Filesystem vs Cryptography
- **Pattern:** Domain separation (fs logic vs crypto logic)

## Recommendations for Level3

### Option 1: Follow Cache Pattern (Recommended)
- **7 files** in single directory
- Similar complexity to cache backend
- Clear separation by concern
- No subdirectories needed

**Structure:**
```
level3/
├── level3.go             # Main Fs, registration, core operations
├── object.go             # Object and Directory types
├── commands.go           # Backend commands
├── particles.go          # Particle operations
├── healing.go            # Self-healing infrastructure
├── helpers.go            # Utility functions
└── errors.go             # Error types (if needed)
```

### Option 2: Follow Union Pattern (If Complexity Grows)
- Use subdirectories for complex features
- Keep core in root directory
- Only if level3 grows significantly more complex

**Structure (if needed later):**
```
level3/
├── level3.go             # Main Fs
├── object.go             # Object and Directory
├── commands.go           # Commands
├── particles/
│   ├── operations.go     # Split, merge, parity
│   └── reconstruction.go # Reconstruction logic
└── healing/
    ├── queue.go          # Upload queue
    └── workers.go        # Background workers
```

## Conclusion

**Yes, there are virtual backends with more than 5 files:**

1. **Union**: 24 files (3 main + subdirectories) - Most complex
2. **Cache**: 12 files (7 main) - Single directory, multiple concerns
3. **Hasher**: 6 files (4 main) - Feature-based separation

**Level3's proposed 6-file structure is:**
- ✅ Reasonable compared to cache (7 files)
- ✅ Similar complexity to hasher (4 files)
- ✅ More organized than current single-file approach
- ✅ Follows established patterns in rclone

The proposed 6-file split for level3 is well-justified and follows patterns used by other complex virtual backends in rclone.
