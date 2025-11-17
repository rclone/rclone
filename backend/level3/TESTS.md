# Level3 Backend Tests

This document provides an overview of the test suite for the level3 RAID 3 backend.

---

## 🛡️ Error Handling Policy (Hardware RAID 3 Compliant)

The level3 backend follows hardware RAID 3 behavior:

- **Reads**: Work with 2 of 3 backends (best effort) ✅
- **Writes**: Require all 3 backends (strict) ❌  
- **Deletes**: Work with any backends (best effort, idempotent) ✅

This ensures data consistency while maximizing read availability.

---

## 📊 Test Organization

### Integration Tests

**`TestIntegration`** - Full suite with configured remote
- Runs rclone's comprehensive integration tests
- Requires `-remote` flag with configured level3 remote
- Tests real cloud storage backends (S3, GCS, etc.)
- Usage: `go test -remote level3config: ./backend/level3/`

**`TestStandard`** - Full suite with local temp dirs (CI)
- Primary test for CI/CD pipelines
- Creates three temp directories (even, odd, parity)
- Runs 70+ sub-tests covering all rclone operations
- No external dependencies required
- **This is the main test to run for development**

---

### Unit Tests - Core Operations

**Byte Operations**:
- `TestSplitBytes` - Byte-level striping (even/odd indices)
- `TestMergeBytes` - Reconstruction from even/odd slices
- `TestSplitMergeRoundtrip` - Verifies split/merge are perfect inverses

**Validation**:
- `TestValidateParticleSizes` - Validates even/odd size relationships

**Parity Operations**:
- `TestCalculateParity` - XOR parity calculation
- `TestParityFilenames` - Parity naming (.parity-el/.parity-ol)

**Reconstruction**:
- `TestParityReconstruction` - Basic XOR reconstruction
- `TestReconstructFromEvenAndParity` - Full file reconstruction (odd missing)
- `TestReconstructFromOddAndParity` - Full file reconstruction (even missing)
- `TestSizeFormulaWithParity` - Size calculation in degraded mode

---

### Integration Tests - Degraded Mode

**`TestIntegrationStyle_DegradedOpenAndSize`**
- Simulates real backend failure by deleting particles
- Verifies reads work via reconstruction
- Tests correct size reporting in degraded mode

**`TestLargeDataQuick`**
- Tests with 1 MB file
- Ensures implementation scales to realistic sizes
- Verifies performance is acceptable

---

### Self-Healing Tests

**`TestSelfHealing`**
- Odd particle automatic restoration
- Verifies background upload queue
- Validates restored particle correctness

**`TestSelfHealingEvenParticle`**
- Even particle automatic restoration
- Ensures symmetry in self-healing

**`TestSelfHealingNoQueue`**
- Verifies fast Shutdown() when no healing needed
- Tests Solution D (hybrid) optimization
- Ensures <100ms exit when healthy

**`TestSelfHealingLargeFile`**
- Self-healing with 100 KB file
- Stress-tests memory and upload handling

**`TestSelfHealingShutdownTimeout`** (skipped)
- Would test 60-second timeout in Shutdown()
- Requires mocked slow backend (future enhancement)

---

## 🚀 Running Tests

### Run All Tests
```bash
go test ./backend/level3/...
```

### Run Specific Test
```bash
go test -run TestSelfHealing ./backend/level3/
```

### Run with Verbose Output
```bash
go test -v ./backend/level3/...
```

### Run Only Unit Tests (skip integration)
```bash
go test -run 'Test(Split|Merge|Validate|Calculate|Parity|Reconstruct|Size)' ./backend/level3/
```

### Run Only Self-Healing Tests
```bash
go test -run TestSelfHealing ./backend/level3/
```

### Run Integration Tests Only
```bash
go test -run TestStandard ./backend/level3/
```

---

## 📈 Test Coverage

| Category | Tests | Lines | Coverage |
|----------|-------|-------|----------|
| Integration | 2 | 70+ sub-tests | Full fs.Fs interface |
| Byte Operations | 3 | ~150 | Core striping logic |
| Validation | 1 | ~30 | Size validation |
| Parity | 2 | ~100 | XOR calculation |
| Reconstruction | 4 | ~200 | Degraded mode |
| Self-Healing | 4 | ~250 | Background uploads |
| **Total** | **16** | **~800** | **Comprehensive** |

---

## ⏱️ Test Performance

| Test Category | Duration | Notes |
|---------------|----------|-------|
| Unit tests | <0.01s | Fast, run frequently |
| Integration | 0.07s | Comprehensive, run before commit |
| Self-healing | <0.01s | Fast, includes background workers |
| Large file | 0.01s | 1 MB test, acceptable performance |
| **Total** | **~0.37s** | **Entire suite** |

---

## 🎯 Test Philosophy

### What We Test:

1. **Core RAID 3 Math** - Striping, merging, XOR parity
2. **Data Integrity** - Round-trip, reconstruction correctness
3. **Edge Cases** - Empty files, single bytes, odd/even lengths
4. **Degraded Mode** - All combinations of missing particles
5. **Self-Healing** - Background uploads, deduplication, shutdown
6. **Performance** - Large files, acceptable execution time
7. **Integration** - Full rclone compatibility

### What We Don't Test (Yet):

1. Network failures during upload/download
2. Concurrent operations (multiple readers/writers)
3. Very large files (>100 MB)
4. Shutdown timeout with slow backends (requires mocking)
5. Retry logic for failed self-healing uploads
6. Parity particle self-healing

---

## 🔍 Test Documentation Standard

Each test follows this structure:

```go
// TestXxx tests [WHAT: one-sentence description].
//
// [WHY: 1-2 sentences explaining importance/context]
//
// This test verifies:
//   - [Behavior 1]
//   - [Behavior 2]
//   - [Edge case]
//
// Failure indicates: [What's broken / impact]
func TestXxx(t *testing.T) {
    // Test implementation
}
```

This ensures every test is:
- **Self-documenting**: Clear purpose without reading code
- **Debuggable**: "Failure indicates:" helps diagnose issues
- **Maintainable**: Explains "why" not just "what"

---

## 🐛 Debugging Failed Tests

### If `TestStandard` Fails:

1. Check which sub-test failed (e.g., `FsPutFiles/ObjectOpen`)
2. Look at error message for specific operation
3. Check if all three temp directories are writable
4. Verify no file system permissions issues

### If Reconstruction Tests Fail:

1. Check XOR parity calculation (`TestCalculateParity`)
2. Verify split/merge logic (`TestSplitBytes`, `TestMergeBytes`)
3. Check size formulas (`TestSizeFormulaWithParity`)
4. Look for off-by-one errors in byte indices

### If Self-Healing Tests Fail:

1. Check if background workers started correctly
2. Verify Shutdown() is being called
3. Check for goroutine leaks or panics
4. Verify file system write permissions
5. Look at timing - is Shutdown() timing out?

---

## 📚 Adding New Tests

When adding new tests, follow these guidelines:

1. **Add doc comment** using the standard structure
2. **Choose appropriate section** (unit, integration, self-healing)
3. **Test both success and failure paths**
4. **Include edge cases** (empty, single byte, odd/even lengths)
5. **Verify error messages** are helpful
6. **Keep tests fast** (<1 second if possible)
7. **Clean up resources** (use `t.TempDir()`, defer cleanup)

Example:

```go
// TestNewFeature tests [description].
//
// [Importance/context]
//
// This test verifies:
//   - [Expected behavior]
//
// Failure indicates: [Impact]
func TestNewFeature(t *testing.T) {
    // Setup
    ctx := context.Background()
    
    // Test
    result, err := newFeature()
    
    // Verify
    require.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

---

## ✅ Test Checklist Before Commit

- [ ] All tests pass: `go test ./backend/level3/...`
- [ ] Build succeeds: `go build`
- [ ] New tests have doc comments
- [ ] Edge cases are covered
- [ ] No test takes >1 second
- [ ] Temp resources are cleaned up
- [ ] Documentation updated if behavior changes

---

## 🔗 Related Documentation

- `README.md` - User guide and usage examples
- `TESTING.md` - Manual testing with MinIO and Bash harnesses
- `RAID3.md` - Technical specification
- `docs/SELF_HEALING_IMPLEMENTATION.md` - Self-healing details

---

**Last Updated**: November 16, 2025  
**Status**: ✅ Comprehensive coverage - see `TESTING.md` for latest test information

