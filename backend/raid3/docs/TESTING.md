# Testing the RAID3 Backend

This document provides comprehensive testing documentation for the raid3 backend, covering test suite overview, unit tests, integration tests, bash integration tests, test organization, running tests, and writing tests. For user documentation, see [`../README.md`](../README.md). For technical RAID 3 details, see [`RAID3.md`](RAID3.md). For integration test setup and usage, see [`../integration/README.md`](../integration/README.md).

---

## Test Suite Overview

The raid3 backend includes comprehensive testing following rclone conventions. The backend follows hardware RAID 3 behavior: reads work with 2 of 3 backends (best effort), writes require all 3 backends (strict), deletes work with any backends (best effort, idempotent). This ensures data consistency while maximizing read availability.

---

## 🚀 Running Tests

```bash
# All tests
go test ./backend/raid3 -v

# Unit tests only
go test ./backend/raid3 -run "^Test(Split|Merge|Calculate|Parity|Validate)" -v

# Integration tests
go test ./backend/raid3 -run "TestStandard" -v

# Quick integration tests
go test ./backend/raid3 -run "TestStandard" -test.short -v

# Specific test
go test -run TestHeal ./backend/raid3/

# Verbose output
go test -v ./backend/raid3/...
```

---

## 📊 Test Organization

### Integration Tests

**`TestIntegration`** - Full suite with configured remote. Runs rclone's comprehensive integration tests, requires `-remote` flag with configured raid3 remote, tests real cloud storage backends (S3, GCS, etc.). Usage: `go test -remote raid3config: ./backend/raid3/`

**`TestStandard`** - Full suite with local temp dirs (CI). Primary test for CI/CD pipelines, creates three temp directories (even, odd, parity), runs 70+ sub-tests covering all rclone operations, no external dependencies required. This is the main test to run for development.

---

### Unit Tests - Core Operations

**Byte Operations:**
- `TestSplitBytes` - Byte-level striping (even/odd indices)
- `TestMergeBytes` - Reconstruction from even/odd slices
- `TestSplitMergeRoundtrip` - Verifies split/merge are perfect inverses

**Validation:**
- `TestValidateParticleSizes` - Validates even/odd size relationships

**Parity Operations:**
- `TestCalculateParity` - XOR parity calculation
- `TestParityFilenames` - Parity naming (.parity-el/.parity-ol)

**Reconstruction:**
- `TestParityReconstruction` - Basic XOR reconstruction
- `TestReconstructFromEvenAndParity` - Full file reconstruction (odd missing)
- `TestReconstructFromOddAndParity` - Full file reconstruction (even missing)
- `TestSizeFormulaWithParity` - Size calculation in degraded mode

---

### Integration Tests - Degraded Mode

**`TestIntegrationStyle_DegradedOpenAndSize`** - Simulates real backend failure by deleting particles, verifies reads work via reconstruction, tests correct size reporting in degraded mode.

**`TestLargeDataQuick`** - Tests with 1 MB file, ensures implementation scales to realistic sizes, verifies performance is acceptable.

---

### Heal Tests

**`TestHeal`** - Odd particle automatic restoration, verifies background upload queue, validates restored particle correctness.

**`TestHealEvenParticle`** - Even particle automatic restoration, ensures symmetry in heal.

**`TestHealNoQueue`** - Verifies fast Shutdown() when no heal needed, tests Solution D (hybrid) optimization, ensures <100ms exit when healthy.

**`TestHealLargeFile`** - Heal with 100 KB file, stress-tests memory and upload handling.

**`TestHealShutdownTimeout`** (skipped) - Would test 60-second timeout in Shutdown(), requires mocked slow backend (future enhancement).

---

## 📈 Test Coverage

| Category | Tests | Lines | Coverage |
|----------|-------|-------|----------|
| Integration | 2 | 70+ sub-tests | Full fs.Fs interface |
| Byte Operations | 3 | ~150 | Core striping logic |
| Validation | 1 | ~30 | Size validation |
| Parity | 2 | ~100 | XOR calculation |
| Reconstruction | 4 | ~200 | Degraded mode |
| Heal | 4 | ~250 | Background uploads |
| **Total** | **16** | **~800** | **Comprehensive** |

---

## ⏱️ Test Performance

| Test Category | Duration | Notes |
|---------------|----------|-------|
| Unit tests | <0.01s | Fast, run frequently |
| Integration | 0.07s | Comprehensive, run before commit |
| Heal | <0.01s | Fast, includes background workers |
| Large file | 0.01s | 1 MB test, acceptable performance |
| **Total** | **~0.37s** | **Entire suite** |

---

## 🎯 Test Philosophy

We test: core RAID 3 math (striping, merging, XOR parity), data integrity (round-trip, reconstruction correctness), edge cases (empty files, single bytes, odd/even lengths), degraded mode (all combinations of missing particles), heal (background uploads, deduplication, shutdown), performance (large files, acceptable execution time), and integration (full rclone compatibility). Not yet tested: network failures during upload/download, concurrent operations (multiple readers/writers), very large files (>100 MB), shutdown timeout with slow backends (requires mocking), retry logic for failed heal uploads, parity particle heal.

---

## 🔍 Test Documentation Standard

Each test follows this structure with doc comments describing what it tests, why it's important, what it verifies, and what failure indicates. This ensures every test is self-documenting (clear purpose without reading code), debuggable (failure indicates helps diagnose issues), and maintainable (explains why not just what).

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

---

## 🐛 Debugging Failed Tests

The standard Go test flags `-v` and `-vv` are supported for increased verbosity.

**If `TestStandard` fails**: Check which sub-test failed, look at error message for specific operation, verify all three temp directories are writable, check for file system permissions issues.

**If reconstruction tests fail**: Check XOR parity calculation (`TestCalculateParity`), verify split/merge logic (`TestSplitBytes`, `TestMergeBytes`), check size formulas (`TestSizeFormulaWithParity`), look for off-by-one errors in byte indices.

**If heal tests fail**: Check if background workers started correctly, verify Shutdown() is being called, check for goroutine leaks or panics, verify file system write permissions, check timing (is Shutdown() timing out?).

---

## Test Coverage

```bash
# Generate coverage report
go test ./backend/raid3 -coverprofile=coverage.out
go tool cover -html=coverage.out
```

---

## Manual Testing

Use the `setup.sh` script from the `integration/` directory for easy test environment setup. See [`integration/README.md`](integration/README.md) for complete details.

```bash
# Quick setup using setup.sh
cd backend/raid3/integration
./setup.sh
cd $(cat ${HOME}/.rclone_raid3_integration_tests.workdir)

# Upload a test file
echo "Hello, World!" > test.txt
rclone --config rclone_raid3_integration_tests.config copy test.txt localraid3:

# Verify particles were created (7 bytes each)
ls -lh ${WORKDIR}/local/even/test.txt
ls -lh ${WORKDIR}/local/odd/test.txt
ls -lh ${WORKDIR}/local/parity/*.parity-*

# Download and verify
rclone --config rclone_raid3_integration_tests.config copy localraid3:test.txt downloaded.txt
diff test.txt downloaded.txt  # Should output nothing (files identical)
```

### MinIO (3 local instances)

Run three local MinIO servers and a `raid3` remote over them, then run basic commands (including degraded read). Note: If you've already run `setup.sh`, the MinIO data directories are already created and you can skip the directory creation step.

**1) Start three MinIO servers (Docker)**

```bash
# Create storage directories (skip if you've run setup.sh)
mkdir -p ~/go/raid3storage/{even_minio,odd_minio,parity_minio}

# Start minioeven
docker run -d --name minioeven \
  -p 9001:9000 -p 9004:9001 \
  -e MINIO_ROOT_USER=even -e MINIO_ROOT_PASSWORD=evenpass88 \
  -v ~/go/raid3storage/even_minio:/data \
  quay.io/minio/minio server /data --console-address ":9001"

# Start minioodd
docker run -d --name minioodd \
  -p 9002:9000 -p 9005:9001 \
  -e MINIO_ROOT_USER=odd -e MINIO_ROOT_PASSWORD=oddpass88 \
  -v ~/go/raid3storage/odd_minio:/data \
  quay.io/minio/minio server /data --console-address ":9001"

# Start minioparity
docker run -d --name minioparity \
  -p 9003:9000 -p 9006:9001 \
  -e MINIO_ROOT_USER=parity -e MINIO_ROOT_PASSWORD=paritypass88 \
  -v ~/go/raid3storage/parity_minio:/data \
  quay.io/minio/minio server /data --console-address ":9001"
```

Note: Each container runs the MinIO console on its internal port `:9001`, but Docker maps them to different host ports (9004, 9005, 9006), so there's no conflict.

**2) rclone config**

You can use the config file created by `setup.sh` (located at `${WORKDIR}/rclone_raid3_integration_tests.config`) for your own tests. Otherwise, add the content of that config file to your own rclone config (e.g. `~/.config/rclone/rclone.conf`).

**3) Degraded read test**

```bash
docker stop minioodd  # Stop odd; raid3 should reconstruct from even+parity
rclone -vv cat minioraid3:testdir/hello.txt --timeout 30s --contimeout 5s
docker start minioodd
```

Important: When a MinIO instance is stopped, the S3 backend will attempt connection and retry. The `--contimeout 5s` flag sets connection timeout to 5 seconds, and `--timeout 30s` sets overall operation timeout. Even so, you may see 5-10 second delays as the backend retries the unavailable instance before proceeding with reconstruction.

**5) Cleanup**

```bash
docker stop minioeven minioodd minioparity
docker rm minioeven minioodd minioparity
# Optional: rm -rf ~/go/raid3storage
```

### Verify Parity Calculation

```bash
# View particles in hex and manually verify XOR: parity[i] should equal even[i] ^ odd[i]
hexdump -C /tmp/raid3-test/source/test.txt  # Original
hexdump -C /tmp/raid3-test/even/test.txt     # Even bytes
hexdump -C /tmp/raid3-test/odd/test.txt      # Odd bytes
hexdump -C /tmp/raid3-test/parity/test.txt.parity-el  # Parity (XOR)
```

---

## Known Test Limitations

Some tests are SKIPped because the backend doesn't implement optional features: `OpenWriterAt` (not needed), `OpenChunkWriter` (not applicable), `ChangeNotify` (not applicable), `FsPutStream` (could be implemented in future), `Shutdown` (not needed). These are all optional features.

---

## Continuous Integration

To add to CI/CD, add to `.github/workflows/test.yml`:

```yaml
- name: Test RAID3 Backend
  run: go test ./backend/raid3 -v
```

---

## Bash Integration Test Harnesses

The `backend/raid3/integration/` directory contains comprehensive Bash-based integration test scripts that supplement the Go-based tests with black-box testing scenarios. For complete documentation, setup instructions, and usage examples, see [`backend/raid3/integration/README.md`](integration/README.md).

The integration test suite includes: **Comparison Harness** (`compare_raid3_with_single.sh`) - black-box comparison between raid3 and single backend remotes, covers common rclone commands, compares exit codes and command outputs to confirm raid3 mirrors single backend behavior. **Rebuild Harness** (`compare_raid3_with_single_rebuild.sh`) - validates `rclone backend rebuild` command after simulated backend replacement, tests rebuild scenarios for each backend (even, odd, parity), validates with `rclone check` and byte-for-byte comparison. **Heal Harness** (`compare_raid3_with_single_heal.sh`) - validates degraded-read behavior and explicit `backend heal` command, tests automatic particle reconstruction, covers directory reconstruction scenarios. **Error Handling Harness** (`compare_raid3_with_single_errors.sh`) - validates error handling, rollback behavior, and degraded mode write blocking, tests health check functionality. All scripts share common infrastructure via `compare_raid3_with_single_common.sh` and support both local filesystem and MinIO (S3) storage backends.

---

## Summary

The raid3 backend follows proper rclone testing pattern (no shell scripts needed for core functionality), includes comprehensive unit tests (all core functions tested), full integration tests (standard rclone test suite), bash integration tests (additional validation for complex scenarios), is easy to run (standard `go test` command), and is CI/CD ready (no special setup required). The backend is production-ready with enterprise-grade testing.

---

## 🔗 Related Documentation

Related documentation: `README.md` for user guide and usage examples, [`RAID3.md`](RAID3.md) for technical specification, [`CLEAN_HEAL.md`](CLEAN_HEAL.md) for self-maintenance guide (healing and cleanup).
