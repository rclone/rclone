# Level3 Timeout Mode Option - Design Document

## User's Excellent Suggestion

Add a level3-specific option to let users choose timeout behavior:
1. **Standard** - Use global config (good for local/file storage)
2. **Balanced** - Some retries but faster than default (good for reliable S3)
3. **Aggressive** - Fastest failover (good for degraded mode testing)

## Design

### Option 1: Enum-Style (Recommended)

```go
// In level3.go

type Options struct {
    Even          string `config:"even"`
    Odd           string `config:"odd"`
    Parity        string `config:"parity"`
    TimeoutMode   string `config:"timeout_mode"`  // NEW
}

func init() {
    fs.Register(&fs.RegInfo{
        Name:        "level3",
        Description: "Level3 RAID3-like backend with byte-level striping and parity",
        NewFs:       NewFs,
        Options: []fs.Option{
            {
                Name:     "even",
                Help:     "Remote for even-indexed bytes",
                Required: true,
            },
            {
                Name:     "odd",
                Help:     "Remote for odd-indexed bytes",
                Required: true,
            },
            {
                Name:     "parity",
                Help:     "Remote for parity bytes",
                Required: true,
            },
            {
                Name:     "timeout_mode",
                Help:     "Timeout behavior for backend operations",
                Default:  "standard",
                Examples: []fs.OptionExample{
                    {
                        Value: "standard",
                        Help:  "Use global timeout settings (best for local/file storage)",
                    },
                    {
                        Value: "balanced",
                        Help:  "Moderate timeouts (3 retries, 30s timeout) - good for reliable S3",
                    },
                    {
                        Value: "aggressive",
                        Help:  "Fast failover (1 retry, 10s timeout) - best for S3 degraded mode",
                    },
                },
                Advanced: false,  // Important enough to show in basic config
            },
        },
    })
}
```

### Implementation in NewFs

```go
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
    // Parse options
    opt := new(Options)
    err := configstruct.Set(m, opt)
    if err != nil {
        return nil, err
    }
    
    // Apply timeout mode
    ctx = applyTimeoutMode(ctx, opt.TimeoutMode)
    
    f := &Fs{
        name: name,
        root: root,
        opt:  *opt,
    }
    
    // Initialize backends with modified context
    f.even, err = cache.Get(ctx, opt.Even)
    // ... rest of initialization
    
    return f, nil
}

// applyTimeoutMode creates a context with timeout settings based on mode
func applyTimeoutMode(ctx context.Context, mode string) context.Context {
    switch mode {
    case "standard":
        // Don't modify context - use global settings
        fs.Debugf(nil, "level3: Using standard timeout mode (global settings)")
        return ctx
        
    case "balanced":
        newCtx, ci := fs.AddConfig(ctx)
        ci.LowLevelRetries = 3
        ci.ConnectTimeout = fs.Duration(15 * time.Second)
        ci.Timeout = fs.Duration(30 * time.Second)
        fs.Logf(nil, "level3: Using balanced timeout mode (retries=%d, contimeout=%v, timeout=%v)",
            ci.LowLevelRetries, ci.ConnectTimeout, ci.Timeout)
        return newCtx
        
    case "aggressive":
        newCtx, ci := fs.AddConfig(ctx)
        ci.LowLevelRetries = 1
        ci.ConnectTimeout = fs.Duration(5 * time.Second)
        ci.Timeout = fs.Duration(10 * time.Second)
        fs.Logf(nil, "level3: Using aggressive timeout mode (retries=%d, contimeout=%v, timeout=%v)",
            ci.LowLevelRetries, ci.ConnectTimeout, ci.Timeout)
        return newCtx
        
    default:
        fs.Errorf(nil, "level3: Unknown timeout_mode %q, using standard", mode)
        return ctx
    }
}
```

### Store Context in Fs for Operations

```go
type Fs struct {
    name     string
    root     string
    opt      Options
    even     fs.Fs
    odd      fs.Fs
    parity   fs.Fs
    hashSet  hash.Set
    features *fs.Features
    
    // NEW: Store the configured context for operations
    opCtx    context.Context
}

func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
    // ... parse options ...
    
    // Apply timeout mode
    opCtx := applyTimeoutMode(ctx, opt.TimeoutMode)
    
    f := &Fs{
        name:  name,
        root:  root,
        opt:   *opt,
        opCtx: opCtx,  // Store for later use
    }
    
    // Initialize backends with modified context
    f.even, err = cache.Get(opCtx, opt.Even)
    // ...
    
    return f, nil
}

// Use in operations
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
    // Merge operation context with our timeout settings
    // This preserves cancellation from the operation while using our timeouts
    mergedCtx := mergeContexts(ctx, f.opCtx)
    
    evenObj, errEven := f.even.NewObject(mergedCtx, remote)
    oddObj, errOdd := f.odd.NewObject(mergedCtx, remote)
    // ...
}

// mergeContexts combines cancellation from opCtx with config from configCtx
func mergeContexts(opCtx, configCtx context.Context) context.Context {
    // Copy config from configCtx to opCtx
    return fs.CopyConfig(opCtx, configCtx)
}
```

## Configuration Examples

### Example 1: Local Storage (Standard Mode)

```ini
[level3local]
type = level3
even = /mnt/disk1/even
odd = /mnt/disk2/odd
parity = /mnt/disk3/parity
timeout_mode = standard
```

**Behavior**:
- Uses global config (default: 10 retries)
- Fast operations (~instant)
- No unnecessary timeouts

### Example 2: Reliable S3 (Balanced Mode)

```ini
[level3s3]
type = level3
even = s3even:bucket
odd = s3odd:bucket
parity = s3parity:bucket
timeout_mode = balanced
```

**Behavior**:
- 3 retries (instead of 10)
- 30s timeout (instead of 5 minutes)
- Good balance: ~30-60s failover in degraded mode
- More forgiving than aggressive

### Example 3: Testing/Degraded Mode (Aggressive Mode)

```ini
[level3minio]
type = level3
even = minioeven:bucket
odd = minioodd:bucket
parity = minioparity:bucket
timeout_mode = aggressive
```

**Behavior**:
- 1 retry only
- 10s timeout
- Fast failover: ~10-20s in degraded mode
- Best for testing and known-unreliable backends

## Timeout Comparison Table

| Mode | Retries | ConnectTimeout | Timeout | Degraded Failover | Use Case |
|------|---------|----------------|---------|-------------------|----------|
| **standard** | 10 (global) | 60s (global) | 5m (global) | 2-5 minutes | Local/file storage |
| **balanced** | 3 | 15s | 30s | ~30-60 seconds | Reliable S3 |
| **aggressive** | 1 | 5s | 10s | ~10-20 seconds | Degraded mode, testing |

## User Experience

### During `rclone config`

```
Current remotes:

Name                 Type
====                 ====
minioeven            s3
minioodd             s3
minioparity          s3

e) Edit existing remote
n) New remote
d) Delete remote
r) Rename remote
c) Copy remote
s) Set configuration password
q) Quit config
e/n/d/r/c/s/q> n

Enter name for new remote.
name> level3test

Option Storage.
Type of storage to configure.
Choose a number from below, or type in your own value.
...
XX / Level3 RAID3-like backend
   \ (level3)
...
Storage> level3

Option even.
Remote for even-indexed bytes.
Enter a value. Press Enter to leave empty.
even> minioeven:

Option odd.
Remote for odd-indexed bytes.
Enter a value. Press Enter to leave empty.
odd> minioodd:

Option parity.
Remote for parity bytes.
Enter a value. Press Enter to leave empty.
parity> minioparity:

Option timeout_mode.
Timeout behavior for backend operations.
Choose a number from below, or type in your own string value.
Press Enter for the default (standard).
 1 / Use global timeout settings (best for local/file storage)
   \ (standard)
 2 / Moderate timeouts (3 retries, 30s timeout) - good for reliable S3
   \ (balanced)
 3 / Fast failover (1 retry, 10s timeout) - best for S3 degraded mode
   \ (aggressive)
timeout_mode> 3

Edit advanced config?
y) Yes
n) No (default)
y/n> n

Configuration complete.
```

### Runtime Logging

```bash
$ rclone copy source.txt level3test:

# With standard mode (local storage)
2025/11/01 10:00:00 DEBUG : level3: Using standard timeout mode (global settings)

# With balanced mode (S3)
2025/11/01 10:00:00 NOTICE: level3: Using balanced timeout mode (retries=3, contimeout=15s, timeout=30s)

# With aggressive mode (testing)
2025/11/01 10:00:00 NOTICE: level3: Using aggressive timeout mode (retries=1, contimeout=5s, timeout=10s)
```

## Alternative: Numeric Values (More Flexible)

If users want even more control:

```go
type Options struct {
    Even               string        `config:"even"`
    Odd                string        `config:"odd"`
    Parity             string        `config:"parity"`
    TimeoutMode        string        `config:"timeout_mode"`
    CustomRetries      int           `config:"custom_retries"`       // NEW
    CustomConnTimeout  fs.Duration   `config:"custom_contimeout"`    // NEW
    CustomTimeout      fs.Duration   `config:"custom_timeout"`       // NEW
}

// In init()
{
    Name:     "custom_retries",
    Help:     "Number of retries (only used if timeout_mode=custom)",
    Default:  1,
    Advanced: true,
},
{
    Name:     "custom_contimeout",
    Help:     "Connection timeout (only used if timeout_mode=custom)",
    Default:  fs.Duration(5 * time.Second),
    Advanced: true,
},
{
    Name:     "custom_timeout",
    Help:     "Operation timeout (only used if timeout_mode=custom)",
    Default:  fs.Duration(10 * time.Second),
    Advanced: true,
},

// In applyTimeoutMode()
case "custom":
    newCtx, ci := fs.AddConfig(ctx)
    ci.LowLevelRetries = opt.CustomRetries
    ci.ConnectTimeout = opt.CustomConnTimeout
    ci.Timeout = opt.CustomTimeout
    fs.Logf(nil, "level3: Using custom timeout mode (retries=%d, contimeout=%v, timeout=%v)",
        ci.LowLevelRetries, ci.ConnectTimeout, ci.Timeout)
    return newCtx
```

**Pros**: Maximum flexibility  
**Cons**: More complex for users

## Recommendation

### Implement: Enum-Style (3 modes)

**Why**:
1. ✅ **Simple for users** - clear choices
2. ✅ **Covers 95% of use cases**
3. ✅ **Self-documenting** - mode names explain purpose
4. ✅ **Safe defaults** - standard mode = no surprises
5. ✅ **Easy to test** - three well-defined scenarios

**Implementation order**:
1. Add `timeout_mode` option to level3
2. Implement `applyTimeoutMode()` function
3. Store context in `Fs.opCtx`
4. Use in `NewObject()` and other operations
5. Test all three modes
6. Document in README.md and TESTING.md

**Estimated time**: 1-2 hours

### Future Enhancement: Add "custom" mode

If users request more control, add a 4th mode:
- `custom` - uses `custom_retries`, `custom_contimeout`, `custom_timeout` options
- Only implement if there's demand

## Documentation Updates

### README.md

```markdown
## Configuration

### Timeout Modes

Level3 supports three timeout modes to optimize for different storage types:

- **standard** (default): Uses global rclone timeout settings. Best for local/file storage.
- **balanced**: Moderate timeouts (3 retries, 30s). Good for reliable S3 providers.
- **aggressive**: Fast failover (1 retry, 10s). Best for S3 degraded mode testing.

Example for S3 with fast failover:
```ini
[mylevel3]
type = level3
even = s3even:bucket
odd = s3odd:bucket
parity = s3parity:bucket
timeout_mode = aggressive
```

### TESTING.md

```markdown
## Timeout Modes for S3 Testing

When testing with MinIO, use `timeout_mode = aggressive` for faster degraded mode testing:

```ini
[miniolevel3]
type = level3
even = minioeven:
odd = minioodd:
parity = minioparity:
timeout_mode = aggressive  # Fast failover for testing
```

This reduces degraded mode timeout from 2-5 minutes to 10-20 seconds.
```

## Summary

**User's suggestion**: ✅ Excellent idea!

**Recommended implementation**:
- Add `timeout_mode` option with 3 choices: `standard`, `balanced`, `aggressive`
- Default to `standard` (no behavior change for existing users)
- Users can opt-in to faster timeouts when using S3
- Clear, simple, self-documenting

**Benefits**:
1. ✅ No breaking changes (default = standard)
2. ✅ Flexibility for different storage types
3. ✅ Users control their own timeout/reliability trade-off
4. ✅ Easy to understand and configure
5. ✅ Solves the "one size doesn't fit all" problem

Ready to implement tomorrow?

