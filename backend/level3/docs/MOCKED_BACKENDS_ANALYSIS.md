# Mocked Backends in Rclone: Analysis

**Date**: December 4, 2025  
**Question**: Do other rclone backends use mocked backends for testing unavailable backends?

---

## 🔍 Summary: **No, Most Backends Don't Use Mocked Backends**

After analyzing the rclone codebase, **most backends do NOT use mocked backends** for testing unavailable backend scenarios. Here's what I found:

---

## 📊 Findings by Backend Type

### 1. **Multi-Backend Backends** (Union, Combine, Level3)

**Union Backend**:
- ❌ **No mocked backends**
- ❌ **No tests for unavailable upstreams**
- ✅ Handles errors gracefully (continues with available backends)
- ✅ Uses real local/Memory backends in tests

**Combine Backend**:
- ❌ **No mocked backends**
- ❌ **No tests for unavailable upstreams**
- ✅ Uses real backends in tests

**Level3 Backend**:
- ❌ **No mocked backends** (tests are skipped)
- ✅ Uses **non-existent paths** to simulate unavailable backends (for Put tests)
- ⚠️ Some tests skipped: `TestMoveFailsWithUnavailableBackend`, `TestUpdateFailsWithUnavailableBackend`
- ✅ Uses **MinIO** for real integration testing

---

### 2. **Single-Backend Backends** (SMB, S3, etc.)

**SMB Backend**:
- ✅ Has **custom mocks** (`mockFs` in `filepool_test.go`)
- ⚠️ But mocks are for **internal interfaces**, not unavailable backends
- Purpose: Test file pool logic, not backend unavailability

**S3 Backend**:
- ❌ **No mocked backends**
- ✅ Uses real S3/MinIO for testing
- ✅ Tests error handling with real network failures

**Other Backends**:
- ❌ **No standard mocking framework** (no `testify/mock`, `gomock`)
- ✅ Most use real backends or skip complex scenarios

---

## 🛠️ Techniques Used Instead of Mocking

### Technique 1: Non-Existent Paths (Level3)

**How it works**:
```go
// Simulate unavailable backend with non-existent path
evenDir := "/nonexistent/path/even"  // Backend can't be created
oddDir := t.TempDir()                // Available
parityDir := t.TempDir()             // Available

f, err := level3.NewFs(ctx, "test", "", configmap.Simple{
    "even": evenDir,   // Will fail operations
    "odd": oddDir,
    "parity": parityDir,
})
```

**Pros**:
- ✅ Simple to implement
- ✅ Works for Put operations (health check fails)
- ✅ No mocking framework needed

**Cons**:
- ❌ Doesn't work for Move/Update (need existing Fs)
- ❌ Can't simulate mid-operation failures
- ❌ Limited to initial setup failures

**Used in**: `TestPutFailsWithUnavailableBackend`, `TestHealthCheckEnforcesStrictWrites`

---

### Technique 2: Real Backends with Error Handling (Union, Combine)

**How it works**:
- Use real backends (local, memory)
- Test graceful degradation
- Errors are collected and handled

**Example** (Union):
```go
// Union continues with available backends
errs := Errors(make([]error, len(f.upstreams)))
multithread(len(f.upstreams), func(i int) {
    u := f.upstreams[i]
    entries, err := u.List(ctx, dir)
    if err != nil {
        errs[i] = fmt.Errorf("%s: %w", u.Name(), err)
        return
    }
    // ...
})
// Continues even if some backends fail
```

**Pros**:
- ✅ Tests real behavior
- ✅ No mocking needed
- ✅ Works for all operations

**Cons**:
- ⚠️ Can't easily simulate unavailable backends
- ⚠️ Tests focus on graceful degradation, not strict failures

---

### Technique 3: MinIO Integration Testing (Level3)

**How it works**:
- Use MinIO (S3-compatible) in Docker containers
- Start/stop containers to simulate failures
- Test real network/service failures

**Example** (from shell scripts):
```bash
# Start MinIO containers
docker-compose up -d

# Stop one container to simulate failure
docker stop minio-odd

# Run tests - backend is truly unavailable
rclone backend heal level3:
```

**Pros**:
- ✅ Real network failures
- ✅ Tests actual service unavailability
- ✅ Most realistic testing

**Cons**:
- ⏱️ Slower (requires Docker)
- 🔧 More complex setup
- 🐳 Requires container infrastructure

---

### Technique 4: Custom Internal Mocks (SMB)

**How it works**:
- Create custom mock structs for internal interfaces
- Not for unavailable backends, but for testing internal logic

**Example** (SMB):
```go
type mockFs struct {
    putConnectionCalled bool
    putConnectionErr    error
    getConnectionErr    error
}

func (m *mockFs) getConnection(ctx context.Context, share string) (*conn, error) {
    if m.getConnectionErr != nil {
        return nil, m.getConnectionErr
    }
    return &conn{}, nil
}
```

**Purpose**: Test file pool logic, not backend unavailability

---

## 📋 Comparison Table

| Backend | Mocked Backends? | Technique | Tests Unavailable? |
|---------|------------------|-----------|-------------------|
| **Level3** | ❌ No | Non-existent paths, MinIO | ⚠️ Partial (some skipped) |
| **Union** | ❌ No | Real backends, graceful errors | ❌ No |
| **Combine** | ❌ No | Real backends | ❌ No |
| **SMB** | ✅ Custom mocks | Internal interface mocks | ❌ No (not for unavailable) |
| **S3** | ❌ No | Real S3/MinIO | ✅ Yes (network failures) |
| **Chunker** | ❌ No | Real backends | ❌ No |
| **Crypt** | ❌ No | Real backends | ❌ No |

---

## 🎯 Why Mocked Backends Are Rare

### 1. **Rclone's Architecture**

Rclone backends use the **`fs.Fs` interface**, which is:
- Backend-agnostic
- Hard to mock (complex interface)
- Better tested with real backends

### 2. **Testing Philosophy**

Rclone prefers:
- ✅ **Real backends** (local, memory, MinIO)
- ✅ **Integration testing** over unit testing with mocks
- ✅ **fstests.Run()** for comprehensive testing

### 3. **Complexity**

Mocking `fs.Fs` would require:
- Mocking 50+ methods
- Complex state management
- Maintaining mock implementations
- Not worth the effort for most cases

---

## 💡 What Level3 Could Do

### Option 1: Keep Current Approach ✅ **Recommended**

**Current**:
- Use non-existent paths for Put tests (works)
- Use MinIO for integration testing (realistic)
- Skip complex Move/Update tests (documented)

**Pros**:
- ✅ Simple and maintainable
- ✅ Real integration testing with MinIO
- ✅ Covers most scenarios

**Cons**:
- ⚠️ Some tests skipped
- ⚠️ Can't test mid-operation failures

---

### Option 2: Create Custom Mock Backend

**Would require**:
```go
type mockBackend struct {
    fs.Fs
    shouldFailPut bool
    shouldFailMove bool
    // ...
}

func (m *mockBackend) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo) (fs.Object, error) {
    if m.shouldFailPut {
        return nil, errors.New("simulated failure")
    }
    return m.Fs.Put(ctx, in, src)
}
```

**Pros**:
- ✅ Could test Move/Update failures
- ✅ More control over failure scenarios

**Cons**:
- ⏱️ Significant development effort
- 🔧 Maintenance burden
- 📊 Not used by other backends (inconsistent)

---

### Option 3: Use MinIO for All Tests

**Would require**:
- Docker setup in Go tests
- Container management
- Slower test execution

**Pros**:
- ✅ Most realistic testing
- ✅ Tests real network failures

**Cons**:
- ⏱️ Slower tests
- 🐳 Requires Docker
- 🔧 More complex CI/CD setup

---

## 📊 Conclusion

**Answer**: **No, other rclone backends generally do NOT use mocked backends.**

**Patterns observed**:
1. **Multi-backend backends** (union, combine) don't test unavailable backends - they handle errors gracefully
2. **Level3 is unique** - it needs strict failure testing (RAID 3 policy)
3. **Most backends** use real backends or skip complex scenarios
4. **SMB has mocks** but for internal interfaces, not unavailable backends
5. **No standard mocking framework** is used across rclone

**Level3's approach is reasonable**:
- ✅ Uses non-existent paths (works for Put)
- ✅ Uses MinIO for integration testing
- ✅ Skips complex Move/Update tests (documented)
- ✅ Matches rclone's testing philosophy

**Recommendation**: Keep current approach. The skipped tests are documented and the important scenarios (Put failures) are covered. Adding mocked backends would be inconsistent with rclone's testing patterns and add significant complexity.

