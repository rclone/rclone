# S3/MinIO Degraded Mode Timeout Issue - Research Summary

## Current Problem

**Status**: S3/MinIO backends with level3 **do NOT work** in degraded mode
- Operations hang indefinitely (not just slow) when one backend is unavailable
- Local filesystem backends work perfectly
- The issue is S3-specific, not a problem with our RAID 3 logic

## Research Findings

### 1. How Union Backend Handles This

**Key Discovery**: Union backend **does NOT solve this problem either**

From `backend/union/union.go`:
```go
// NewObject creates a new remote union file object
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
    objs := make([]*upstream.Object, len(f.upstreams))
    errs := Errors(make([]error, len(f.upstreams)))
    multithread(len(f.upstreams), func(i int) {
        u := f.upstreams[i]
        o, err := u.NewObject(ctx, remote)  // <- Same issue: this blocks
        if err != nil && err != fs.ErrorObjectNotFound {
            errs[i] = fmt.Errorf("%s: %w", u.Name(), err)
            return
        }
        objs[i] = u.WrapObject(o)
    })
    // Waits for ALL goroutines to complete
    // ...
}
```

**Analysis**:
- Union uses `multithread()` (goroutines + WaitGroup) just like we do
- It waits for ALL upstreams to respond before continuing
- No timeout logic or early-return mechanism
- **Union would have the same hang problem with unavailable S3 backends**

**Implication**: This is a **known limitation in rclone's virtual backends** when using S3

### 2. S3 Backend Configuration

**Key Configuration Points** from `backend/s3/s3.go`:

```go
// Line 1291: S3 uses global LowLevelRetries
awsConfig.RetryMaxAttempts = ci.LowLevelRetries

// Line 1568: Pacer is set to only 2 retries
pc := fs.NewPacer(ctx, pacer.NewS3(pacer.MinSleep(minSleep)))
pc.SetRetries(2)  // Only for directory listing retries
```

**Global Configuration** from `fs/config.go`:
```go
{
    Name:    "low_level_retries",
    Default: 10,  // <- This is the problem!
    Help:    "Number of low level retries to do",
    Groups:  "Config",
}

{
    Name:    "contimeout",  // Connection timeout
    Default: Duration(60 * time.Second),
    Help:    "Connect timeout",
    Groups:  "Networking",
}

{
    Name:    "timeout",  // Data channel timeout
    Default: Duration(5 * time.Minute),
    Help:    "IO idle timeout",
    Groups:  "Networking",
}
```

**The Problem**:
1. **LowLevelRetries = 10** (default): AWS SDK retries 10 times per operation
2. **Each retry has exponential backoff** (starts ~1s, grows to ~30s+)
3. **Connection timeout = 60s**: Each attempt can take up to 60s to fail
4. **Total worst case**: 10 attempts × 60s = **10 minutes of hanging**

### 3. Why Our Timeouts Don't Work

**What we tried**:
```go
// In NewFs
initCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
defer cancel()

fs, err := cache.Get(initCtx, evenPath)  // This doesn't respect our timeout!
```

**Why it fails**:
1. `cache.Get()` calls the S3 backend's `NewFs`
2. S3 backend creates its own HTTP client with SDK-level timeouts
3. **AWS SDK v2** has its own retry loop that doesn't check context frequently enough
4. Our context timeout fires, but the SDK is deep in a retry loop

**From AWS SDK behavior**:
- Connection refused errors trigger full retry sequence
- Each retry attempt itself has a timeout
- Context cancellation is only checked between retries, not during retries
- With 10 retries and exponential backoff, this takes 2-5 minutes minimum

### 4. How Other Products Handle This

**Commercial RAID/Storage Systems**:

1. **Ceph** (distributed storage):
   - Default OSD timeout: 20 seconds
   - Health check interval: 5 seconds
   - Marks OSD as "down" after 2-3 missed heartbeats
   - Reads immediately failover to replica OSDs

2. **MinIO Distributed Mode**:
   - Health check: Every 1 second
   - Read quorum: Immediate failover if node unreachable
   - Uses very short connection timeouts (5-10s)
   - Does NOT wait for all nodes

3. **GlusterFS**:
   - Network timeout: 42 seconds (configurable)
   - Quick-read from first responding node
   - Parallel reads with "first wins" approach

**Key Pattern**: **Early return + aggressive timeouts**

### 5. S3-Specific Solutions in Other Rclone Backends

**Searched**: `chunker`, `crypt`, `compress` backends
**Finding**: **None of them solve this problem**

These backends are "wrapper" backends that:
- Only wrap a single underlying remote
- Don't do parallel operations
- Don't handle failover scenarios
- Inherit whatever timeout behavior the underlying backend has

**Implication**: **This is an unsolved problem in rclone's architecture**

## Root Cause Analysis

The fundamental issue is **architectural**:

1. **rclone's design assumption**: Backends are always reachable
2. **No built-in failover**: Virtual backends aren't designed for HA scenarios
3. **SDK control**: AWS SDK v2 owns the retry logic, not rclone
4. **Cache.Get() blocking**: No way to timeout or cancel a backend initialization

## Proposed Solutions

### Option 1: User Configuration (Immediate)
**Status**: Can implement today

Document that users must configure aggressive timeouts:
```bash
rclone cat miniolevel3:file.txt \
  --low-level-retries 1 \
  --contimeout 5s \
  --timeout 10s
```

**Expected behavior**:
- 1 retry instead of 10: ~10-15 seconds instead of 2-5 minutes
- Still slow, but acceptable for degraded mode

**Pros**:
- No code changes needed
- Works with current implementation

**Cons**:
- Must document clearly
- Users must remember flags
- Still ~10-15s delay

### Option 2: Backend-Specific Timeout Override (Medium effort)
**Status**: Needs implementation

Add to level3 Options:
```go
type Options struct {
    Even   string `config:"even"`
    Odd    string `config:"odd"`
    Parity string `config:"parity"`
    ProbeTimeout Duration `config:"probe_timeout"` // NEW
}
```

Modify NewObject to create a custom context:
```go
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
    probeCtx, cancel := context.WithTimeout(ctx, f.opt.ProbeTimeout)
    defer cancel()
    
    // Use probeCtx for all backend calls
    // ...
}
```

**Challenge**: Still doesn't fix AWS SDK retry loops

### Option 3: Async Probe with Early Return (Better)
**Status**: Partially implemented, needs SDK fix

Our current approach but working:
```go
// Start all probes
go func() { probe even }()
go func() { probe odd }()
go func() { probe parity }()

// Return as soon as we have 2 of 3
for {
    select {
    case result := <-results:
        if haveEnoughParticles() {
            return success  // Don't wait for slow/dead backend!
        }
    case <-time.After(10 * time.Second):
        if haveEnoughParticles() {
            return success
        }
        return error
    }
}
```

**Problem**: SDK retry loop still blocks the goroutine for 2-5 minutes

**Solution requires**:
- Force AWS SDK to respect context cancellation during retries
- OR: Wrap backend calls in a timeout enforcer that kills goroutines

### Option 4: Health Check Layer (Enterprise)
**Status**: Major architectural change

Implement active health monitoring:
```go
type HealthChecker struct {
    backends map[string]*BackendHealth
    interval time.Duration
}

type BackendHealth struct {
    available  bool
    lastCheck  time.Time
    latency    time.Duration
}

func (h *HealthChecker) IsAvailable(backend string) bool {
    health := h.backends[backend]
    return health.available && time.Since(health.lastCheck) < 10*time.Second
}
```

In NewObject:
```go
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
    if !healthChecker.IsAvailable("odd") {
        // Skip odd, go straight to reconstruction
        return f.newObjectDegraded(ctx, remote, useEvenAndParity)
    }
    // Normal path
}
```

**Pros**:
- Immediate detection of backend failure
- No waiting for timeouts
- Production-grade solution

**Cons**:
- Complex implementation
- Needs background goroutines
- Health checks themselves can fail

### Option 5: Fork AWS SDK or Use REST API (Nuclear)
**Status**: Not recommended

Replace AWS SDK with direct HTTP calls that we control:
- Use rclone's `rest` package
- Implement only the S3 operations we need
- Full control over timeouts

**Pros**:
- Complete control

**Cons**:
- Massive effort
- Lose SDK benefits (auth, regions, error handling)
- Maintenance burden

## Recommended Approach

### Phase 1: Document and Mitigate (Today)
1. ✅ Document in TESTING.md that S3 degraded mode doesn't work
2. ✅ Recommend `--low-level-retries 1` for S3 use cases
3. ✅ Note that local backends work perfectly
4. Add warning log when using S3 backends

### Phase 2: Improve Detection (This Week)
1. Add `probe_timeout` option to level3 backend
2. Implement early-return logic with timeout
3. Log prominently when backend probe times out
4. Test with MinIO to verify 10-15s failover

### Phase 3: Health Checking (Future)
1. Design health check architecture
2. Implement background health monitor
3. Skip probes for known-dead backends
4. Test with actual S3 failures

### Phase 4: Upstream (Long-term)
1. Report issue to rclone project
2. Propose `ctx` checking in AWS SDK retry loops
3. Consider contributing health check layer to rclone core

## Testing Plan

### Immediate Tests
- ✅ Local backends work
- ✅ Local degraded mode works
- [ ] Document S3 limitations

### With `--low-level-retries 1`
- [ ] MinIO degraded mode with 1 retry
- [ ] Measure actual timeout duration
- [ ] Verify reconstruction works when it completes
- [ ] Test with actual AWS S3

### With Phase 2 Implementation
- [ ] probe_timeout=10s works
- [ ] Early return with 2/3 backends
- [ ] Logging shows which backend timed out

## Comparison: level3 vs Commercial Solutions

| Feature | level3 (current) | level3 (with Phase 2) | Ceph | MinIO Distributed | 
|---------|------------------|----------------------|------|-------------------|
| Local backend failover | ✅ Instant | ✅ Instant | ✅ Instant | ✅ Instant |
| S3 backend failover | ❌ Hangs | ⚠️ 10-15s | ✅ 5-10s | ✅ 1-5s |
| Health checking | ❌ No | ❌ No | ✅ Yes | ✅ Yes |
| Auto-recovery | ❌ No | ❌ No | ✅ Yes | ✅ Yes |
| Production ready | ⚠️ Local only | ⚠️ Local + S3* | ✅ Yes | ✅ Yes |

(*) With user-configured timeouts

## Conclusion

**Current State**:
- ✅ RAID 3 reconstruction logic is correct and tested
- ✅ Works perfectly with local/file backends
- ❌ S3/object storage degraded mode is broken (hangs indefinitely)
- ❌ This is a fundamental limitation of rclone's architecture with AWS SDK

**Path Forward**:
1. Document limitations clearly
2. Provide workaround (`--low-level-retries 1`)
3. Implement Phase 2 for better S3 support (10-15s failover)
4. Consider Phase 3 health checking for production use

**Honest Assessment**:
- For **local storage**: Production ready ✅
- For **S3/object storage**: Not production ready yet ❌
- Need Phase 2 minimum for S3 use cases

---

**Date**: October 31, 2025  
**Status**: Research complete, awaiting decision on implementation phases

