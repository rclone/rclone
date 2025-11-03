# Config Override & Health Check Solutions

## Question 1: Can we override global config locally in level3?

### ✅ YES - Using `fs.AddConfig()`

**Discovery**: Rclone has a built-in mechanism for this!

From `fs/config.go` lines 817-826:
```go
// AddConfig returns a mutable config structure based on a shallow
// copy of that found in ctx and returns a new context with that added
// to it.
func AddConfig(ctx context.Context) (context.Context, *ConfigInfo) {
    c := GetConfig(ctx)
    cCopy := new(ConfigInfo)
    *cCopy = *c
    newCtx := context.WithValue(ctx, configContextKey, cCopy)
    return newCtx, cCopy
}
```

**How it works**:
1. Creates a **shallow copy** of the current config
2. Returns a new context with the modified config
3. **Does NOT affect** global config or other backends
4. Changes are **context-scoped** (perfect for our use case)

### Implementation for level3

```go
// In level3.go NewFs function
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
    // Parse options
    opt := new(Options)
    err := configstruct.Set(m, opt)
    if err != nil {
        return nil, err
    }
    
    // Create a modified context with aggressive S3 timeouts
    newCtx, ci := fs.AddConfig(ctx)
    
    // Override settings for level3 operations
    ci.LowLevelRetries = 1           // Only 1 retry instead of 10
    ci.ConnectTimeout = fs.Duration(5 * time.Second)   // 5s instead of 60s
    ci.Timeout = fs.Duration(10 * time.Second)         // 10s instead of 5m
    
    fs.Logf(nil, "level3: Using aggressive timeouts (retries=%d, contimeout=%v, timeout=%v)", 
        ci.LowLevelRetries, ci.ConnectTimeout, ci.Timeout)
    
    // Use newCtx for all backend initialization
    f := &Fs{
        name: name,
        root: root,
        opt:  *opt,
    }
    
    // Initialize backends with modified context
    f.even, err = cache.Get(newCtx, opt.Even)
    if err != nil && err != fs.ErrorIsFile {
        return nil, fmt.Errorf("failed to create even remote: %w", err)
    }
    
    f.odd, err = cache.Get(newCtx, opt.Odd)
    if err != nil && err != fs.ErrorIsFile {
        return nil, fmt.Errorf("failed to create odd remote: %w", err)
    }
    
    f.parity, err = cache.Get(newCtx, opt.Parity)
    if err != nil && err != fs.ErrorIsFile {
        return nil, fmt.Errorf("failed to create parity remote: %w", err)
    }
    
    // ... rest of initialization
    
    return f, nil
}
```

### Also use in operations

```go
// In level3.go NewObject function
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
    // Create context with timeout for probing
    newCtx, ci := fs.AddConfig(ctx)
    ci.LowLevelRetries = 1  // Fast fail for probes
    
    // Use newCtx for all backend calls
    evenObj, errEven := f.even.NewObject(newCtx, remote)
    oddObj, errOdd := f.odd.NewObject(newCtx, remote)
    // ...
}
```

### Advantages

✅ **No changes to AWS SDK** - uses rclone's existing mechanism  
✅ **No architectural changes** - standard rclone pattern  
✅ **Context-scoped** - doesn't affect other backends  
✅ **Already tested** - used by other backends (see `fs/newfs_internal_test.go`)  
✅ **Clean and maintainable**

### Expected Result

With `LowLevelRetries = 1`:
- Current: 10 retries × ~10-20s = **2-5 minutes**
- With override: 1 retry × ~10-20s = **10-20 seconds**

**Verdict**: This is the **recommended solution** for Phase 2!

---

## Question 2: Can HeadBucket be used for health checking?

### ✅ YES - But with caveats

**What HeadBucket does**:
```go
// From AWS SDK v2
func (c *Client) HeadBucket(ctx context.Context, params *HeadBucketInput, 
    optFns ...func(*Options)) (*HeadBucketOutput, error)
```

- Sends HTTP `HEAD` request to bucket
- Returns `200 OK` if bucket exists and accessible
- Returns `403 Forbidden` if no permission
- Returns `404 Not Found` if bucket doesn't exist

### The Problem: Same Retry Behavior

**HeadBucket uses the SAME retry loop as other S3 operations**:

```go
// In AWS SDK v2 (simplified)
func (c *Client) HeadBucket(ctx context.Context, ...) {
    // Uses the SAME retry configuration
    return c.client.Do(ctx, &smithy.OperationInput{
        MaxAttempts: c.config.RetryMaxAttempts,  // Still 10 by default!
        // ...
    })
}
```

**Result**: HeadBucket will **still hang** for 2-5 minutes if backend is unavailable!

### When HeadBucket IS Useful

**Scenario 1**: Quick bucket existence check (when backend IS available)
```go
func (f *Fs) bucketExists(ctx context.Context, bucket string) bool {
    _, err := f.s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
        Bucket: aws.String(bucket),
    })
    return err == nil
}
```
- Fast: ~100-200ms
- Good for validation
- **But**: Still slow if backend down

**Scenario 2**: Combined with our config override
```go
func (f *Fs) healthCheck(ctx context.Context, backend string) (healthy bool, latency time.Duration) {
    // Use our modified context with LowLevelRetries=1
    newCtx, ci := fs.AddConfig(ctx)
    ci.LowLevelRetries = 1
    
    start := time.Now()
    _, err := f.s3Client.HeadBucket(newCtx, &s3.HeadBucketInput{
        Bucket: aws.String(backend),
    })
    latency = time.Since(start)
    
    return err == nil, latency
}
```
- With retry=1: ~10-20s timeout (acceptable)
- Returns latency info (useful for monitoring)
- Can cache results

### Comparison: HeadBucket vs HeadObject

| Operation | Purpose | Speed (available) | Speed (unavailable) | Use Case |
|-----------|---------|-------------------|---------------------|----------|
| **HeadBucket** | Check bucket exists | ~100ms | 2-5 min (10-20s*) | Bucket validation |
| **HeadObject** | Check object exists | ~100ms | 2-5 min (10-20s*) | File probing |
| **ListObjectsV2** | List objects | ~200ms | 2-5 min (10-20s*) | Directory listing |

(*) With `LowLevelRetries=1`

**Verdict**: HeadBucket is **useful** but **NOT a silver bullet**. It has the same timeout issues as other operations.

### Recommended Usage

```go
type Fs struct {
    // ... existing fields ...
    
    healthCache sync.Map  // map[string]*healthStatus
}

type healthStatus struct {
    healthy     bool
    lastChecked time.Time
    latency     time.Duration
}

// Background health checker (run periodically)
func (f *Fs) startHealthMonitor(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    go func() {
        for {
            select {
            case <-ticker.C:
                f.checkBackendHealth(ctx, "even", f.even)
                f.checkBackendHealth(ctx, "odd", f.odd)
                f.checkBackendHealth(ctx, "parity", f.parity)
            case <-ctx.Done():
                return
            }
        }
    }()
}

func (f *Fs) checkBackendHealth(ctx context.Context, name string, backend fs.Fs) {
    // Only check S3 backends
    s3Fs, ok := backend.(*s3.Fs)
    if !ok {
        return  // Not S3, assume healthy
    }
    
    // Use aggressive timeout context
    newCtx, ci := fs.AddConfig(ctx)
    ci.LowLevelRetries = 1
    
    start := time.Now()
    _, err := s3Fs.Client().HeadBucket(newCtx, &s3.HeadBucketInput{
        Bucket: aws.String(s3Fs.RootBucket()),
    })
    latency := time.Since(start)
    
    status := &healthStatus{
        healthy:     err == nil,
        lastChecked: time.Now(),
        latency:     latency,
    }
    
    f.healthCache.Store(name, status)
    
    if !status.healthy {
        fs.Logf(f, "Backend %s unhealthy: %v (latency: %v)", name, err, latency)
    }
}

// Use cached health status
func (f *Fs) isBackendHealthy(name string) bool {
    if status, ok := f.healthCache.Load(name); ok {
        health := status.(*healthStatus)
        // Consider stale after 60 seconds
        if time.Since(health.lastChecked) < 60*time.Second {
            return health.healthy
        }
    }
    return true  // Assume healthy if unknown
}
```

---

## Question 3: Can we use MinIO SDK's HealthCheck() with AWS SDK?

### ⚠️ POSSIBLE - But not recommended

### What MinIO SDK Provides

MinIO SDK has health check endpoints:
```go
// From minio-go v7
func (c *Client) HealthCheck(duration time.Duration) (bool, error)
```

**What it does**:
- Calls MinIO-specific health endpoint: `GET /minio/health/live`
- Very fast: ~50-100ms
- **MinIO-specific**: Not part of S3 API

### The Problem: Not S3-Compatible

```
AWS S3:        Does NOT have /minio/health/live endpoint
Wasabi:        Does NOT have /minio/health/live endpoint
Backblaze B2:  Does NOT have /minio/health/live endpoint
MinIO:         ✅ Has /minio/health/live endpoint
```

**Result**: Only works with actual MinIO servers, not AWS S3 or other providers.

### Hybrid Approach (If you ONLY use MinIO)

```go
package level3

import (
    "github.com/minio/minio-go/v7"
    "github.com/minio/minio-go/v7/pkg/credentials"
)

type Fs struct {
    // Existing AWS SDK fields
    even   fs.Fs
    odd    fs.Fs
    parity fs.Fs
    
    // NEW: MinIO clients for health checks only
    evenMinioClient   *minio.Client
    oddMinioClient    *minio.Client
    parityMinioClient *minio.Client
}

func (f *Fs) initMinioHealthCheckers(opt *Options) error {
    // Extract S3 config from rclone config
    evenS3, ok := f.even.(*s3.Fs)
    if !ok {
        return nil  // Not S3, skip
    }
    
    // Create MinIO client for health checks only
    client, err := minio.New(evenS3.Endpoint(), &minio.Options{
        Creds:  credentials.NewStaticV4(evenS3.AccessKey(), evenS3.SecretKey(), ""),
        Secure: true,
    })
    if err != nil {
        return err
    }
    
    f.evenMinioClient = client
    // Repeat for odd and parity...
    
    return nil
}

func (f *Fs) healthCheck(ctx context.Context, name string, client *minio.Client) (bool, error) {
    if client == nil {
        return true, nil  // Not MinIO, assume healthy
    }
    
    // MinIO SDK respects context timeout better
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    
    // Fast health check (50-100ms)
    healthy, err := client.HealthCheck(5 * time.Second)
    return healthy, err
}
```

### Pros and Cons

**Pros**:
- ✅ Very fast: 50-100ms (vs 10-20s with HeadBucket)
- ✅ MinIO SDK has better context handling
- ✅ Designed specifically for health checks

**Cons**:
- ❌ Only works with MinIO (not AWS S3, Wasabi, etc.)
- ❌ Adds MinIO SDK dependency
- ❌ Need to maintain two SDK configurations
- ❌ Complexity: managing two different clients

### Alternative: MinIO SDK for Everything (MinIO-only)

If you **only** use MinIO and never AWS S3:

```go
// Replace AWS SDK entirely with MinIO SDK
type Fs struct {
    evenClient   *minio.Client
    oddClient    *minio.Client
    parityClient *minio.Client
}

// All operations use MinIO SDK
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
    // MinIO SDK has better timeout handling
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()
    
    reader, err := f.evenClient.GetObject(ctx, bucket, object, minio.GetObjectOptions{})
    // ...
}
```

**Effort**: High (need to reimplement all S3 operations)  
**Benefit**: Fast timeouts (5-10s instead of 2-5 min)  
**Trade-off**: Lose AWS S3 compatibility

---

## Recommendation Matrix

| Approach | Effort | S3 Timeout | AWS S3 | MinIO | Other S3 | Verdict |
|----------|--------|------------|--------|-------|----------|---------|
| **1. fs.AddConfig()** | ✅ Low | ⚠️ 10-20s | ✅ Yes | ✅ Yes | ✅ Yes | ✅ **RECOMMENDED** |
| **2. HeadBucket health** | Medium | ⚠️ 10-20s | ✅ Yes | ✅ Yes | ✅ Yes | ⚠️ Same as #1 |
| **3. MinIO HealthCheck** | Medium | ✅ <1s | ❌ No | ✅ Yes | ❌ No | ⚠️ MinIO-only |
| **4. MinIO SDK full** | High | ✅ 5-10s | ❌ No | ✅ Yes | ⚠️ Maybe | ⚠️ Major rewrite |

## Final Recommendation

### For Tomorrow: Implement Option 1 (fs.AddConfig)

```go
// Minimal changes to level3.go
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
    // ... parse options ...
    
    // Override config for level3
    newCtx, ci := fs.AddConfig(ctx)
    ci.LowLevelRetries = 1
    ci.ConnectTimeout = fs.Duration(5 * time.Second)
    ci.Timeout = fs.Duration(10 * time.Second)
    
    // Use newCtx for everything
    f.even, err = cache.Get(newCtx, opt.Even)
    // ...
}

func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
    // Also use modified context for operations
    newCtx, ci := fs.AddConfig(ctx)
    ci.LowLevelRetries = 1
    
    evenObj, err := f.even.NewObject(newCtx, remote)
    // ...
}
```

**Expected result**:
- S3 degraded mode: **10-20 seconds** (down from 2-5 minutes)
- No AWS SDK changes ✅
- No architectural changes ✅
- Works with all S3 providers ✅

### Future Enhancement: Add HeadBucket Health Monitor

Once basic timeout is working, add background health checking:

```go
// Phase 3: Proactive health monitoring
func (f *Fs) startHealthMonitor(ctx context.Context) {
    // Check every 30 seconds
    // Cache results
    // Skip known-dead backends
}
```

**Expected result**:
- First access after failure: 10-20s (probe timeout)
- Subsequent accesses: **Instant** (use cached health status)

---

## Implementation Plan for Tomorrow

1. ✅ **Implement `fs.AddConfig()` in level3** (30 minutes)
   - Modify `NewFs()` to use modified context
   - Modify `NewObject()` to use modified context
   - Set `LowLevelRetries=1`, `ConnectTimeout=5s`, `Timeout=10s`

2. ✅ **Test with MinIO** (30 minutes)
   - Start 3 MinIO instances
   - Upload file with all 3 running
   - Stop one instance
   - Test `rclone cat` - should return in 10-20s

3. ✅ **Document the improvement** (15 minutes)
   - Update `TESTING.md` with new timeout behavior
   - Update `README.md` with S3 support status
   - Note: "S3 degraded mode: 10-20 second failover"

4. ⚠️ **Optional: Add HeadBucket health cache** (1-2 hours)
   - Only if time permits
   - Background health checker
   - Cache results for 30-60 seconds

**Total time**: 1-2 hours for basic improvement

---

**Date**: November 1, 2025  
**Status**: Ready to implement  
**Constraints**: No AWS SDK changes ✅, No architectural changes ✅

