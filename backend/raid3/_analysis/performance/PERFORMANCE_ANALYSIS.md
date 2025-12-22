# RAID3 Performance Analysis

**Date**: 2024-12-19 (Updated: 2025-12-22)  
**Status**: Analysis Complete - Streaming Implemented  
**Issue**: RAID3 backend is 3x slower than standard rclone

## Executive Summary

The RAID3 backend implementation has several performance bottlenecks that cause it to be approximately 3 times slower than a standard rclone backend. The main issues are:

1. **Memory buffering** - Legacy mode (`use_streaming=false`) loads entire files into memory; streaming mode (`use_streaming=true`, default) uses bounded memory (~5MB)
2. **Health check overhead** - Every write operation performs expensive health checks
3. **Multiple backend operations** - Inherent 3x overhead from RAID3 design
4. **List operation overhead** - Additional particle existence checks per object
5. ~~**No streaming support**~~ ✅ **IMPLEMENTED** - Streaming support is now available (pipelined chunked approach, default)

## Detailed Bottleneck Analysis

### 1. Memory Buffering (Critical Impact - Partially Resolved)

#### Issue
**Legacy Mode** (`use_streaming=false`): All file operations load entire files into memory before processing.

**Streaming Mode** (`use_streaming=true`, **default**): ✅ **IMPLEMENTED** - Files are processed in 2MB chunks using a pipelined approach with bounded memory usage (~5MB).

**Legacy Mode Implementation** (when `use_streaming=false`):
```go
data, err := io.ReadAll(in)  // Loads entire file into memory
evenData, oddData := SplitBytes(data)
parityData := CalculateParity(evenData, oddData)
```

**Streaming Mode Implementation** (when `use_streaming=true`, default):
```go
// Reads 2MB chunks, splits, uploads sequentially while reading next chunk
readChunkSize := int64(f.opt.ChunkSize) * 2  // ~2MB
// Double buffering: ~5MB total memory usage
```

#### Impact

**Legacy Mode** (`use_streaming=false`):
- **Memory usage**: For a file of size N, uses ~3N memory (original + even + odd + merged)
- **Latency**: Must wait for entire file to be read before processing can start
- **Scalability**: Cannot handle files larger than available memory

**Streaming Mode** (`use_streaming=true`, default):
- **Memory usage**: Bounded at ~5MB regardless of file size (double buffering)
- **Latency**: Can start processing immediately, pipelined reading/uploading
- **Scalability**: Can handle files much larger than available memory

#### Standard rclone behavior
- Uses `PutStream` when available for streaming uploads
- Uses temporary files for large uploads when streaming not available
- Supports range reads for partial file access

#### Files Affected

**Streaming Mode** (default):
- `backend/raid3/raid3.go:putStreaming()` - Pipelined chunked uploads
- `backend/raid3/object.go:updateStreaming()` - Pipelined chunked updates
- `backend/raid3/object.go:openStreaming()` - Uses StreamMerger (still buffers particles)

**Legacy Mode** (`use_streaming=false`):
- `backend/raid3/raid3.go:putBuffered()` - Loads entire file
- `backend/raid3/object.go:openBuffered()` - Loads entire particles
- `backend/raid3/object.go:updateBuffered()` - Loads entire file

### 2. Health Check Overhead (High Impact)

#### Issue
`checkAllBackendsAvailable()` is called before every write operation:

**Called from**:
- `Put()` - `raid3.go:1065`
- `Update()` - `object.go:424`
- `Move()` - `raid3.go:1383`
- `Mkdir()` - `raid3.go:1173`
- `SetModTime()` - `object.go:163`

**Implementation** (`raid3.go:641-736`):
```go
func (f *Fs) checkAllBackendsAvailable(ctx context.Context) error {
    checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    // For each backend:
    //   1. List() operation
    //   2. Mkdir() on test path
    //   3. Rmdir() cleanup
}
```

#### Impact
- **Latency**: Adds 15-30ms minimum overhead per write (even when backends are healthy)
- **Network overhead**: 3x List() + 3x Mkdir() operations per write
- **Blocking**: Blocks writes unnecessarily when backends are known to be available
- **No caching**: Health status is checked fresh every time

#### Optimization Opportunity
- Cache health status with TTL (e.g., 1-5 seconds)
- Skip check if recently verified
- Make async/non-blocking for known-good backends
- Only check when actually needed (not on every write)

### 3. Multiple Backend Operations (Medium Impact - Inherent)

#### Issue
Every operation touches 3 backends instead of 1:

- **Reads**: Must read from 2-3 backends (even + odd, or even/odd + parity)
- **Writes**: Must write to all 3 backends (even + odd + parity)
- **Lists**: Must query all 3 backends and merge results
- **Moves**: Must move on all 3 backends

#### Impact
- **Network latency**: 3x round-trips minimum
- **I/O operations**: 3x disk/network operations
- **Concurrency**: While operations are concurrent, network overhead is still 3x

#### Note
This is inherent to RAID3 design (data redundancy). However, operations are already concurrent using `errgroup`, which is good. The overhead is unavoidable but could be optimized with:
- Better connection pooling
- Pipelining where possible
- Reducing redundant operations

### 4. List Operation Overhead (Medium Impact)

#### Issue
`List()` operation has additional overhead when `auto_cleanup` is enabled:

**Implementation** (`raid3.go:839-993`):
1. Queries all 3 backends concurrently (good)
2. Merges results (good)
3. For each object, calls `countParticlesSync()` if `auto_cleanup` enabled (line 962)
4. `countParticlesSync()` makes 3 additional `NewObject()` calls per object

**Code**:
```go
if f.opt.AutoCleanup {
    particleCount := f.countParticlesSync(ctx, e.Remote())  // 3x NewObject() calls
    if particleCount < 2 {
        continue  // Skip broken objects
    }
}
```

#### Impact
- **For N objects**: Makes 3N additional backend calls
- **Worst case**: 6-9x backend operations per object (3 for List + 3 for countParticlesSync)
- **Scales poorly**: Gets worse with more objects

#### Optimization Opportunity
- Cache particle counts per object
- Batch particle existence checks
- Skip `countParticlesSync()` for objects that are known to be healthy
- Only check when actually needed (not for every object in every list)

### 5. Streaming Support ✅ **IMPLEMENTED** (2025-12-22)

#### Status
✅ **Streaming writes are now implemented** using a pipelined chunked approach (default: `use_streaming=true`).

**Put() / Update()**: ✅ **IMPLEMENTED** - Processes files in 2MB chunks
```go
// Pipelined chunked approach
readChunkSize := int64(f.opt.ChunkSize) * 2  // ~2MB
// Read chunk → Split → Upload sequentially → Read next chunk in parallel
```

**Open()**: ⚠️ **Partially implemented** - Still uses `StreamMerger` which buffers particles
- Uses `StreamMerger` to merge even + odd streams
- Still reads entire particles into memory before merging
- Future optimization: True streaming merge (read and merge on-the-fly)

#### Impact

**Streaming Writes** (Put/Update):
- ✅ **Memory**: Bounded at ~5MB regardless of file size
- ✅ **Latency**: Can start uploading immediately
- ✅ **Throughput**: Pipelined reading/uploading

**Streaming Reads** (Open):
- ⚠️ **Memory**: Still buffers particles (future optimization)
- ✅ **Latency**: Can start reading immediately (particles read concurrently)
- ⚠️ **Throughput**: Merging happens after particles are read (future optimization)

#### Implementation Details
- **Default**: `use_streaming=true` (pipelined chunked approach)
- **Chunk size**: 2MB read chunks (produces ~1MB per particle)
- **Memory usage**: ~5MB for double buffering
- **Architecture**: Sequential particle uploads with pipelined reading

## Performance Measurements Needed

To quantify the impact, we need benchmarks comparing:

### Test Cases
1. **Small files** (< 1MB): 1000 files, 10KB each
2. **Medium files** (1-100MB): 100 files, 10MB each
3. **Large files** (> 100MB): 10 files, 100MB each

### Metrics
- **Throughput**: MB/s transfer rate
- **Latency**: Time to first byte, time to complete
- **Memory**: Peak memory usage
- **CPU**: CPU usage during operations
- **Network**: Number of backend operations

### Comparison
- RAID3 backend vs single backend (same underlying storage)
- RAID3 with different file sizes
- RAID3 with different operations (Put, Open, List, Update)

## Optimization Opportunities

### High Priority (Biggest Impact)

#### 1. Streaming Reads (Open()) ⚠️ **PARTIALLY IMPLEMENTED**
**Current**: Uses `StreamMerger` which buffers particles before merging
**Status**: ✅ Particles are read concurrently, but merging happens after buffering
**Proposed**: Stream particles concurrently, merge on-the-fly without buffering
**Benefit**: 
- Reduce memory usage further (currently buffers particles)
- Start returning data immediately (currently waits for particles)
- Support files larger than memory (currently limited by particle size)

**Implementation**:
- Enhance `StreamMerger` to merge on-the-fly without buffering entire particles
- Use buffered channels to pipeline read/merge operations
- Support range reads at particle level (read only needed particles)

#### 2. Streaming Writes (Put()) ✅ **IMPLEMENTED** (2025-12-22)
**Status**: ✅ **COMPLETE** - Pipelined chunked approach implemented
**Implementation**: 
- Reads input in 2MB chunks
- Splits bytes on-the-fly
- Uploads particles sequentially while reading next chunk in parallel
- Bounded memory usage (~5MB)

**Future Optimization**:
- Consider concurrent particle uploads (currently sequential for simplicity)
- Optimize chunk size based on backend characteristics

#### 3. Health Check Optimization
**Current**: Checks all backends before every write
**Proposed**: Cache health status with TTL, skip if recently verified
**Benefit**:
- Reduce write latency by 15-30ms
- Reduce network overhead by 3x per write
- Only check when actually needed

**Implementation**:
- Add health status cache with TTL (1-5 seconds)
- Skip check if status is cached and recent
- Make check async/non-blocking for known-good backends
- Only check when backends are actually needed

### Medium Priority

#### 4. List Optimization
**Current**: Calls `countParticlesSync()` for every object when `auto_cleanup` enabled
**Proposed**: Cache particle counts, batch checks, skip for known-healthy objects
**Benefit**:
- Reduce List() overhead by 3x per object
- Scale better with large directories

#### 5. Range Read Optimization
**Current**: Reads entire particles even for range reads
**Proposed**: Read only needed byte ranges from particles
**Benefit**:
- Reduce I/O for partial reads
- Improve latency for range requests

### Low Priority (Nice to Have)

#### 6. Memory Pool
**Proposed**: Reuse buffers for temporary allocations
**Benefit**: Reduce GC pressure, improve performance

#### 7. Connection Pooling
**Proposed**: Reuse connections to backends
**Benefit**: Reduce connection overhead

## Implementation Complexity

### Easy (1-2 days)
- Health check caching
- List optimization (caching particle counts)

### Medium (3-5 days)
- Streaming reads (Open()) - Enhance StreamMerger for true streaming
- Range read optimization

### Hard (1-2 weeks)
- ~~Streaming writes (Put())~~ ✅ **COMPLETE** (2025-12-22)
- ~~Full streaming support~~ ✅ **MOSTLY COMPLETE** (writes done, reads partially done)

## Success Metrics

### ✅ Achieved (2025-12-22)
- **Memory**: ✅ Support files much larger than available memory (streaming writes implemented)
- **Memory Usage**: ✅ Bounded at ~5MB for streaming mode (vs ~3N for legacy mode)

### 🎯 Remaining Goals
- **Throughput**: Match or exceed 80% of single backend throughput
- **Latency**: Reduce write latency by 50% (remove health check overhead)
- **CPU**: Reduce CPU usage by 30% (fewer allocations)
- **Streaming Reads**: True streaming merge without buffering particles

## Files Requiring Changes

### Critical Path
- `backend/raid3/raid3.go` - Put(), health checks, List()
- `backend/raid3/object.go` - Open(), Update()
- `backend/raid3/particles.go` - Reconstruction functions

### Supporting
- `backend/raid3/helpers.go` - Utility functions
- `backend/raid3/heal.go` - Heal operations

## Conclusion

The RAID3 backend has several performance bottlenecks, with significant progress made:

### ✅ Resolved (2025-12-22)
1. ~~**Memory buffering**~~ - **Streaming writes implemented** (default: `use_streaming=true`)
   - Pipelined chunked approach with bounded memory (~5MB)
   - Can handle files much larger than available memory
   - Legacy mode (`use_streaming=false`) still available for small files

### ⚠️ Partially Resolved
2. **Streaming reads** - Particles read concurrently, but merging still buffers
   - Future optimization: True streaming merge without buffering

### ❌ Remaining Issues
3. **Health check overhead** - Unnecessary checks on every write
   - Optimization opportunity: Cache health status with TTL
4. **List operation overhead** - Additional particle existence checks per object
   - Optimization opportunity: Cache particle counts, batch checks

### Performance Status
- **Streaming writes**: ✅ Implemented and working (default)
- **Streaming reads**: ⚠️ Partially implemented (particles read concurrently, but merged after buffering)
- **Overall**: Implementation is functional and can handle large files efficiently with streaming enabled

### Future Optimization Work
1. ✅ ~~Streaming writes~~ - **COMPLETE** (2025-12-22)
2. Enhance streaming reads (true streaming merge without buffering)
3. Health check optimization (easiest win)
4. List optimization (scalability)

## References

- Standard rclone streaming: `fs/operations/operations.go:1362-1479`
- PutStream examples: `backend/compress/compress.go:563-611`
- Multi-threaded copy: `fs/operations/multithread.go:66-132`
