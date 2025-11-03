# Phase 2 Implementation & Alternative SDK Analysis

## Question 1: How Would Phase 2 (`probe_timeout`) Work?

### The Initial Idea (Overly Optimistic)

I originally suggested adding a `probe_timeout` option like this:

```go
type Options struct {
    Even         string   `config:"even"`
    Odd          string   `config:"odd"`
    Parity       string   `config:"parity"`
    ProbeTimeout Duration `config:"probe_timeout"`  // NEW
}
```

And using it like:
```go
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
    probeCtx, cancel := context.WithTimeout(ctx, f.opt.ProbeTimeout)
    defer cancel()
    
    evenObj, err := f.even.NewObject(probeCtx, remote)
    // ...
}
```

### Why This Doesn't Actually Work

After researching `cache.Get()` behavior (lines 175-199 in `fs/cache/cache.go`):

```go
func Get(ctx context.Context, fsString string) (f fs.Fs, err error) {
    // CRITICAL: Creates a NEW context disconnected from the caller's context!
    newCtx := context.Background()  // <-- Ignores our timeout!
    newCtx = fs.CopyConfig(newCtx, ctx)
    newCtx = filter.CopyConfig(newCtx, ctx)
    f, err = GetFn(newCtx, fsString, fs.NewFs)
    // ...
}
```

**Problem**: The cache **deliberately disconnects** from the calling context because:
1. Cached backends are long-lived (across multiple requests)
2. Can't have them tied to a cancelled context from one operation
3. Preserves config but drops cancellation/timeout

**Result**: Our `context.WithTimeout()` is **ignored** during backend initialization.

### What WOULD Work: Direct Backend Calls

Once the backend is initialized, we CAN use timeouts on operations:

```go
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
    // Backend already initialized from cache, so context timeout DOES work
    probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()
    
    // These respect the timeout because they're operations, not initialization
    evenObj, err := f.even.NewObject(probeCtx, remote)
    oddObj, err := f.odd.NewObject(probeCtx, remote)
    // ...
}
```

**BUT**: Even this has problems...

### The AWS SDK v2 Retry Loop Problem

Even when we pass a timeout context to `NewObject()`, the AWS SDK does this:

```
User calls: NewObject(ctx with 10s timeout)
  └─> S3 backend receives context
      └─> AWS SDK starts HeadObject operation
          ├─> Attempt 1: Connect... [timeout after 60s]
          ├─> Check context... [still valid at 10s mark, but we're inside connect()]
          ├─> Attempt 2: Connect... [exponential backoff]
          ├─> Check context... [finally cancelled, but we're 2 minutes in]
          └─> Eventually returns error
```

**The issue**: Context checking happens **between retries**, not during the connect/read operations themselves.

### Phase 2: What We CAN Improve

Given these constraints, here's what Phase 2 can realistically achieve:

#### Improvement 1: Aggressive Default Retries

```go
// In level3 NewFs initialization
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
    // ... parse options ...
    
    // For S3 backends, override the global low-level retries
    // Create a child context with aggressive retry settings
    if isS3Backend(opt.Even) || isS3Backend(opt.Odd) || isS3Backend(opt.Parity) {
        fs.Logf(nil, "level3: S3 backends detected, using aggressive timeouts")
        
        // Create custom config with lower retries
        ci := fs.GetConfig(ctx)
        newCi := *ci
        newCi.LowLevelRetries = 1  // Only 1 retry instead of 10
        newCi.ConnectTimeout = fs.Duration(5 * time.Second)
        newCi.Timeout = fs.Duration(10 * time.Second)
        
        ctx = fs.AddConfig(ctx, &newCi)
    }
    
    // Now initialize backends with this context
    f.even, err = cache.Get(ctx, opt.Even)
    // ...
}
```

**Result**: ~10-15 seconds instead of 2-5 minutes

#### Improvement 2: Parallel Probe with Timeout Wrapper

```go
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
    type result struct {
        name string
        obj  fs.Object
        err  error
    }
    
    results := make(chan result, 3)
    timeout := 15 * time.Second
    
    // Start all probes
    probeWithTimeout := func(name string, backend fs.Fs) {
        done := make(chan result, 1)
        
        go func() {
            obj, err := backend.NewObject(ctx, remote)
            done <- result{name, obj, err}
        }()
        
        select {
        case res := <-done:
            results <- res
        case <-time.After(timeout):
            results <- result{name, nil, fmt.Errorf("probe timeout after %v", timeout)}
        }
    }
    
    go probeWithTimeout("even", f.even)
    go probeWithTimeout("odd", f.odd)
    go probeWithTimeout("parity", f.parity)
    
    // Collect results and return early if we have 2 of 3
    // ...
}
```

**Result**: Even if SDK hangs, we time out and proceed with available backends

#### Improvement 3: Backend Health Cache

```go
type Fs struct {
    // ... existing fields ...
    
    backendHealth sync.Map  // map[string]*healthStatus
}

type healthStatus struct {
    available   bool
    lastChecked time.Time
    lastError   error
}

func (f *Fs) isBackendHealthy(name string) bool {
    if status, ok := f.backendHealth.Load(name); ok {
        health := status.(*healthStatus)
        // Consider healthy if checked in last 30 seconds
        if time.Since(health.lastChecked) < 30*time.Second {
            return health.available
        }
    }
    return true  // Assume healthy if unknown
}

func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
    // Skip probing backends we know are down
    if !f.isBackendHealthy("odd") {
        fs.Debugf(f, "Skipping odd backend (known to be down), using reconstruction")
        // Go straight to even+parity reconstruction
    }
    
    // Normal probe logic...
}
```

### Phase 2 Summary: What We Can Achieve

**Realistic improvements**:
- ✅ Reduce timeout from 2-5 minutes to 10-15 seconds (via LowLevelRetries=1)
- ✅ Early return when 2 of 3 backends respond (don't wait for dead backend)
- ✅ Cache known-dead backends to skip future probes
- ✅ Prominent logging when operating in degraded mode

**What we CANNOT fix**:
- ❌ Can't make S3 backend initialization respect context timeout (cache.Get limitation)
- ❌ Can't make AWS SDK check context during connect/read operations
- ❌ Still 10-15 seconds delay on first access to dead backend

**Verdict**: Phase 2 is **worthwhile** but not a complete solution.

---

## Question 2: Could We Use Another S3 SDK?

### Option A: IBM COS SDK for Go

**Repository**: https://github.com/IBM/ibm-cos-sdk-go

**Analysis**: 
- **It's a fork of AWS SDK Go v1** (not v2)
- IBM maintains it for their Cloud Object Storage
- **Same architecture**: Uses http.Client, has retry loops
- **Same problems**: Context handling, retry behavior

**Code comparison**:
```go
// AWS SDK v2 (what rclone uses now)
awsConfig.RetryMaxAttempts = ci.LowLevelRetries

// IBM SDK (v1 fork)
sess := session.Must(session.NewSession(&aws.Config{
    MaxRetries: aws.Int(ci.LowLevelRetries),
}))
```

**Verdict**: ❌ **Same fundamental issues**, and it's older (v1 vs v2)

### Option B: MinIO Go SDK

**Repository**: https://github.com/minio/minio-go

**Architecture**: 
- Independent implementation (not AWS fork)
- Designed specifically for MinIO and S3-compatible storage
- Lighter weight, fewer dependencies

**Timeout handling**:
```go
// MinIO SDK
client, err := minio.New(endpoint, &minio.Options{
    Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
    Secure: true,
})

// Context is passed to every operation
object, err := client.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
```

**Key difference**: MinIO SDK is **more context-aware**:
1. Doesn't have AWS's multi-service complexity
2. Simpler retry logic (easier to control)
3. Better context.Context integration

**BUT**: Would require **major refactoring**:
```diff
// Current rclone S3 backend
- Uses AWS SDK v2 with smithy framework
- ~1500 lines of AWS-specific code
- Handles 20+ S3 providers (AWS, GCS, Wasabi, etc.)

// Would need to become
+ Dual implementation (AWS SDK for AWS, MinIO SDK for MinIO)
+ OR: Complete rewrite using MinIO SDK only
+ Lose some AWS-specific features
+ Need to test with all 20+ providers
```

**Testing**: Would MinIO SDK actually be faster?

I can test this hypothesis:
```go
// Test 1: AWS SDK v2 with unavailable endpoint
start := time.Now()
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
_, err := s3Client.HeadObject(ctx, &s3.HeadObjectInput{...})
fmt.Printf("AWS SDK: %v, error: %v\n", time.Since(start), err)

// Test 2: MinIO SDK with same endpoint
start := time.Now()
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
_, err := minioClient.StatObject(ctx, bucket, object, minio.StatObjectOptions{})
fmt.Printf("MinIO SDK: %v, error: %v\n", time.Since(start), err)
```

**Expected results** (based on SDK architecture):
- AWS SDK: ~60-120 seconds (ignores 5s timeout due to retry loop)
- MinIO SDK: ~5-10 seconds (better context handling)

### Option C: Direct HTTP/REST Calls

**What rclone already has**: `rest` package (`lib/rest`)

```go
// Instead of AWS SDK
import "github.com/rclone/rclone/lib/rest"

client := rest.NewClient(fshttp.NewClient(ctx))

// We control everything
resp, err := client.Call(ctx, &rest.Opts{
    Method:  "HEAD",
    Path:    "/bucket/object",
    Options: []rest.Option{
        rest.Timeout(5 * time.Second),  // Full control
    },
})
```

**Advantages**:
- ✅ Complete control over timeouts
- ✅ No retry loops unless we add them
- ✅ Context cancellation works perfectly

**Disadvantages**:
- ❌ Lose AWS authentication (Signature V4)
- ❌ Lose region detection
- ❌ Lose multipart upload handling
- ❌ Lose 20+ provider-specific quirks
- ❌ Basically reimplementing S3 backend from scratch

### Option D: Hybrid Approach

Use **different SDKs for different operations**:

```go
// For time-critical probes: MinIO SDK or REST
func (f *Fs) probeParticle(ctx context.Context, backend string, remote string) (exists bool, err error) {
    // Use lightweight MinIO SDK just for HEAD requests
    exists, err = f.minioProbe(ctx, backend, remote)
    return
}

// For actual data operations: AWS SDK (full features)
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
    // Use AWS SDK for GetObject (benefits from multipart, retries, etc.)
    return o.awsOpen(ctx, options...)
}
```

**Feasibility**: Medium effort
- Keep AWS SDK for rclone's S3 backend operations
- Add MinIO SDK just for level3's fast probing
- Best of both worlds?

### Recommendation Matrix

| Solution | Effort | S3 Timeout | Compatibility | Production Ready |
|----------|--------|------------|---------------|------------------|
| Current (AWS SDK v2) | None | ❌ 2-5 min | ✅ All S3 | ⚠️ Local only |
| Phase 2 (tuned AWS) | Low | ⚠️ 10-15s | ✅ All S3 | ⚠️ Acceptable |
| MinIO SDK full | High | ✅ 5-10s | ⚠️ MinIO only | ⚠️ Limited |
| MinIO SDK hybrid | Medium | ✅ 5-10s | ✅ All S3 | ✅ Good |
| REST direct | Very High | ✅ <5s | ⚠️ Basic S3 | ❌ Too complex |
| IBM SDK | High | ❌ Same | ✅ All S3 | ❌ No benefit |

## My Recommendation

### For Your Use Case

**If using MinIO exclusively**:
- Consider **MinIO SDK hybrid** approach
- Fast probes with MinIO SDK (5-10s)
- Keep AWS SDK for data operations
- Medium effort, good payoff

**If using multiple S3 providers**:
- Implement **Phase 2** improvements
- 10-15s is acceptable for most use cases
- Lower effort, works everywhere
- Document the limitation

**If production critical**:
- Implement **Phase 3** (health checking)
- Monitor backend availability actively
- Skip known-dead backends immediately
- High effort but proper solution

### Code Prototype: MinIO SDK Hybrid

I can implement this if you're interested:

```go
package level3

import (
    "github.com/minio/minio-go/v7"
    "github.com/minio/minio-go/v7/pkg/credentials"
)

type Fs struct {
    // Existing fields
    even   fs.Fs
    odd    fs.Fs
    parity fs.Fs
    
    // NEW: Fast probers for S3 backends
    evenProber   *fastProbe
    oddProber    *fastProbe  
    parityProber *fastProbe
}

type fastProbe struct {
    isS3        bool
    minioClient *minio.Client
    bucket      string
    region      string
}

func newFastProbe(backend fs.Fs) (*fastProbe, error) {
    // Detect if this is an S3 backend
    if s3Fs, ok := backend.(*s3.Fs); ok {
        // Extract S3 config
        client, err := minio.New(s3Fs.Endpoint(), &minio.Options{
            Creds:  credentials.NewStaticV4(s3Fs.AccessKey(), s3Fs.SecretKey(), ""),
            Region: s3Fs.Region(),
        })
        
        return &fastProbe{
            isS3:        true,
            minioClient: client,
            bucket:      s3Fs.RootBucket(),
            region:      s3Fs.Region(),
        }, nil
    }
    
    return &fastProbe{isS3: false}, nil
}

func (p *fastProbe) exists(ctx context.Context, remote string) (bool, error) {
    if !p.isS3 {
        // Fall back to regular method
        return false, nil
    }
    
    // Use MinIO SDK for fast probe
    _, err := p.minioClient.StatObject(ctx, p.bucket, remote, minio.StatObjectOptions{})
    if err != nil {
        if minio.ToErrorResponse(err).Code == "NoSuchKey" {
            return false, nil
        }
        return false, err
    }
    return true, nil
}
```

**Estimated effort**: 2-3 days to implement and test

## Next Steps

1. **Decide on approach**:
   - Quick fix: Phase 2 with AWS SDK tuning
   - Better fix: MinIO SDK hybrid
   - Proper fix: Phase 3 health checking

2. **Test current timeout**:
   - Measure actual AWS SDK timeout with `--low-level-retries 1`
   - Confirm 10-15 second range

3. **Prototype if interested**:
   - I can implement MinIO SDK hybrid approach
   - Test with your 3 local MinIO instances
   - Compare timeout behavior

What's your preference?

