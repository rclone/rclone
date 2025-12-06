# Bash Tests for Skipped Go Tests: Analysis

**Date**: December 4, 2025  
**Question**: Would it make sense to implement skipped tests as bash-based tests?

**Answer**: ✅ **YES - Highly Recommended**

---

## 🎯 Skipped Tests That Could Be Implemented

1. **`TestMoveFailsWithUnavailableBackend`** - Move fails when backend unavailable
2. **`TestUpdateFailsWithUnavailableBackend`** - Update fails when backend unavailable  
3. **`TestSelfHealingShutdownTimeout`** - Shutdown timeout with slow backend

---

## ✅ Why Bash Tests Make Sense

### 1. **Infrastructure Already Exists**

The tools directory already has:
- ✅ MinIO container management (`start_minio_containers`, `stop_minio_containers`)
- ✅ Individual container control (`stop_single_minio_container`, `start_single_minio_container`)
- ✅ Test framework (`compare_level3_common.sh`)
- ✅ Pattern already used in `compare_level3_with_single_recover.sh`

**Example from recover script**:
```bash
simulate_disk_swap() {
  local backend="$1"
  if [[ "${STORAGE_TYPE}" == "minio" ]]; then
    stop_single_minio_container "${backend}"  # ✅ Already implemented!
  fi
  # ... test logic ...
}
```

### 2. **Go Test Comments Recommend It**

The skipped tests explicitly mention MinIO:

```go
// TestMoveFailsWithUnavailableBackend
// NOTE: Testing Move with truly unavailable backend is complex because:
// 1. NewFs may fail with unavailable backend (can't create test Fs)
// 2. chmod doesn't reliably make local backend unavailable
// 3. Need to mock backend behavior (complex)
//
// For comprehensive Move failure testing, use interactive tests with MinIO
// where you can stop a backend and verify Move fails.
t.Skip("Move failure with unavailable backend requires mocked backends or MinIO testing")
```

**The Go tests are literally telling us to use MinIO!**

### 3. **Real Network Failures**

Bash tests with MinIO provide:
- ✅ Real network failures (connection refused)
- ✅ Real service unavailability (container stopped)
- ✅ Realistic timeout behavior
- ✅ Tests actual rclone binary behavior

### 4. **Consistent with Existing Patterns**

The tools directory already has:
- `compare_level3_with_single.sh` - Basic operations
- `compare_level3_with_single_heal.sh` - Heal command testing
- `compare_level3_with_single_recover.sh` - Rebuild with stopped containers

Adding error/failure tests would complete the suite.

---

## 📋 Proposed Implementation

### New Script: `compare_level3_with_single_errors.sh`

**Purpose**: Test error handling when backends are unavailable

**Scenarios**:

1. **Move Fails with Unavailable Backend**
   ```bash
   # Scenario: move-fail-even
   - Create file on level3
   - Stop even MinIO container
   - Attempt rclone move
   - Verify: Move fails with appropriate error
   - Verify: Original file unchanged
   - Start container
   ```

2. **Update Fails with Unavailable Backend**
   ```bash
   # Scenario: update-fail-odd
   - Create file on level3
   - Stop odd MinIO container
   - Attempt rclone copy (update)
   - Verify: Update fails with appropriate error
   - Verify: Original file unchanged
   - Start container
   ```

3. **Shutdown Timeout with Slow Backend** (optional)
   ```bash
   # Scenario: shutdown-timeout
   - Create degraded file (missing particle)
   - Trigger read (queues self-healing)
   - Stop backend mid-upload (simulate slow backend)
   - Verify: Shutdown waits appropriately
   - Verify: Timeout behavior
   ```

---

## 🛠️ Implementation Details

### Using Existing Infrastructure

**Container Management** (already exists):
```bash
# Stop a backend
stop_single_minio_container "even"

# Test operation
rclone move level3:file.txt level3:renamed.txt

# Verify failure
if [[ $? -eq 0 ]]; then
    log_fail "move" "Move should have failed with unavailable backend"
fi

# Restore backend
start_single_minio_container "even"
```

**Test Pattern** (follow existing scripts):
```bash
run_move_fail_scenario() {
    local backend="$1"
    log_info "suite" "Running move-fail scenario '${backend}' (${STORAGE_TYPE})"
    
    # Only works with MinIO (can stop containers)
    if [[ "${STORAGE_TYPE}" != "minio" ]]; then
        record_result "PASS" "move-fail-${backend}" "Skipped for local (requires MinIO)"
        return 0
    fi
    
    # Setup
    purge_remote_root "${LEVEL3_REMOTE}"
    create_test_file "move-test.txt"
    
    # Stop backend
    stop_single_minio_container "${backend}"
    
    # Attempt move
    result=$(capture_command "move" move "${LEVEL3_REMOTE}:move-test.txt" "${LEVEL3_REMOTE}:moved.txt")
    # ... verify failure ...
    
    # Restore backend
    start_single_minio_container "${backend}"
}
```

---

## 📊 Comparison: Go Tests vs Bash Tests

| Aspect | Go Tests (Skipped) | Bash Tests (Proposed) |
|--------|-------------------|----------------------|
| **Backend Type** | Local (non-existent paths) | MinIO (real containers) |
| **Failure Simulation** | Can't create Fs | Stop container (real failure) |
| **Move Testing** | ❌ Skipped | ✅ Possible |
| **Update Testing** | ❌ Skipped | ✅ Possible |
| **Realism** | ⚠️ Limited | ✅ High (real network failures) |
| **Setup Complexity** | ✅ Simple | ⚠️ Requires Docker |
| **Execution Speed** | ✅ Fast | ⚠️ Slower (container ops) |
| **CI/CD Integration** | ✅ Easy | ⚠️ Requires Docker |

---

## ✅ Advantages of Bash Tests

1. **Real Network Failures**
   - Tests actual connection refused errors
   - Tests real timeout behavior
   - More realistic than mocked failures

2. **Tests Actual Binary**
   - Tests rclone command-line interface
   - Tests error message formatting
   - Tests user-facing behavior

3. **Completes Test Coverage**
   - Covers scenarios Go tests can't
   - Validates error messages
   - Tests end-to-end workflows

4. **Consistent with Existing Tools**
   - Follows same pattern as heal/recover scripts
   - Uses same infrastructure
   - Maintains consistency

---

## ⚠️ Considerations

### 1. **MinIO Only**

These tests would only work with MinIO (can't stop local filesystem):
- ✅ MinIO: Can stop containers
- ❌ Local: Can't simulate unavailable backend

**Solution**: Skip for local (like heal script does, or did before we removed skip)

### 2. **Test Execution Time**

Bash tests are slower:
- Container start/stop operations
- Network timeouts
- Process spawning

**Impact**: Acceptable for integration tests

### 3. **CI/CD Requirements**

Requires Docker:
- ✅ Already used by other scripts
- ✅ Standard in CI/CD environments
- ⚠️ Adds dependency

---

## 🎯 Recommendation

**✅ YES - Implement as bash tests**

**Reasons**:
1. ✅ Infrastructure already exists
2. ✅ Go tests explicitly recommend MinIO
3. ✅ Provides real network failure testing
4. ✅ Completes test coverage
5. ✅ Consistent with existing tools
6. ✅ Tests actual rclone binary behavior

**Implementation Plan**:
1. ✅ Create `compare_level3_with_single_errors.sh`
2. ✅ Implement `move-fail-even`, `move-fail-odd`, `move-fail-parity` scenarios
3. ✅ Implement `update-fail-even`, `update-fail-odd`, `update-fail-parity` scenarios
4. ✅ Use existing `stop_single_minio_container` / `start_single_minio_container`
5. ✅ Follow pattern from `compare_level3_with_single_recover.sh`

**Status**: ✅ **IMPLEMENTED** - December 4, 2025

**This would provide comprehensive error testing that Go tests cannot achieve!**

---

## ✅ Implementation Complete

**Created**: `compare_level3_with_single_errors.sh`

**Implemented Scenarios**:
1. ✅ `move-fail-even` - Stop even backend, verify Move fails
2. ✅ `move-fail-odd` - Stop odd backend, verify Move fails  
3. ✅ `move-fail-parity` - Stop parity backend, verify Move fails
4. ✅ `update-fail-even` - Stop even backend, verify Update fails
5. ✅ `update-fail-odd` - Stop odd backend, verify Update fails
6. ✅ `update-fail-parity` - Stop parity backend, verify Update fails

**Features**:
- ✅ Uses existing MinIO container infrastructure
- ✅ Follows same pattern as heal/recover scripts
- ✅ Skips gracefully for local backends (can't stop filesystem)
- ✅ Verifies operations fail with appropriate error codes
- ✅ Verifies original files remain unchanged (no partial operations)
- ✅ Verifies new/moved files don't exist (complete failure)
- ✅ Tests actual rclone binary behavior
- ✅ Provides detailed test results and summaries

**Usage**:
```bash
# List scenarios
./compare_level3_with_single_errors.sh list

# Run all scenarios
./compare_level3_with_single_errors.sh --storage-type minio test

# Run specific scenario
./compare_level3_with_single_errors.sh --storage-type minio test move-fail-even
```

**This implementation provides comprehensive error testing that Go tests cannot achieve!**

