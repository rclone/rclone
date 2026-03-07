# Testing the RAID3 Backend

This document provides testing documentation for the raid3 backend. For user documentation, see [`../README.md`](../README.md). For technical RAID 3 details, see [`RAID3.md`](RAID3.md). For integration test setup and test layout, see [`../test/README.md`](../test/README.md).

---

## Test Types Overview

The raid3 backend provides three types of tests:

1. **Go tests provided by the raid3 backend** - Unit and integration tests in the `backend/raid3` package. Run with `go test ./backend/raid3 -v`. Includes core RAID 3 operations (split, merge, parity), degraded mode, and heal functionality.

2. **Shell-script based tests provided by the raid3 backend** - Bash integration test harnesses in `backend/raid3/test/`. Black-box testing for comparison, rebuild, heal, and error handling. For **local** (and **MinIO** where noted), rebuild, heal, and error scenarios are covered by Go tests; the bash scripts skip those and run only for **SFTP** (or minio/mixed for errors). See [`test/README.md`](../test/README.md).

3. **Go tests provided by the rclone project** - Comprehensive test suites (`fs/operations` and `fs/sync`) that validate the full `fs.Fs` interface. Run with `go test ./fs/operations -remote localraid3: -v` and `go test ./fs/sync -remote localraid3: -v`.

---

## ✅ Current Test Status

**Last Updated**: 2026-03

- **Backend Tests**: ✅ PASS - `go test ./backend/raid3 -v`
- **fs/sync Tests**: ✅ 96 PASS, 0 FAIL, 12 SKIP - `go test ./fs/sync -remote localraid3: -v`
- **fs/operations Tests**: ✅ All passing - `go test ./fs/operations -remote localraid3: -v`

---

## 🚀 Quick Start

### Backend Tests (Type 1)

```bash
# All tests (-parallel 1 avoids test interference from shared global state)
go test ./backend/raid3 -parallel 1 -v

# Unit tests only
go test ./backend/raid3 -run "^Test(Split|Merge|Calculate|Parity|Validate)" -v

# Integration tests
go test ./backend/raid3 -run "TestStandard" -v

# Race condition detection (see Race Detection section below)
go test ./backend/raid3 -race -v
```

### Rclone Framework Tests (Type 3)

```bash
# Setup (one-time)
cd backend/raid3/test && ./setup.sh

# Run from rclone root
export RCLONE_CONFIG=backend/raid3/test/tests.config
go test ./fs/operations -remote localraid3: -v
go test ./fs/sync -remote localraid3: -v
```

**Alternative** (inline config):
```bash
RCLONE_CONFIG=backend/raid3/test/tests.config \
  go test ./fs/operations -remote localraid3: -v
```

### Bash Integration Tests (Type 2)

See [`test/README.md`](../test/README.md) for complete documentation. Scripts include:
- `compare.sh` - Comparison harness (runs for local, minio, mixed, sftp)
- `compare_rebuild.sh` - Rebuild validation; **skips local/minio/mixed** (covered by Go); runs for **SFTP** only
- `compare_heal.sh` - Heal validation; **skips local/minio/mixed** (covered by Go); runs for **SFTP** only
- `compare_errors.sh` - Error handling; **skips local** (covered by Go); runs for minio, mixed, sftp
- `compare_all.sh` - Master script to run all tests across all backends
- `performance_test.sh` - Performance benchmarks (upload/download) for different file sizes and storage types
- `compression_bench.sh` - Compression ratio for local raid3 (requires `--storage-type=local`; config compression ≠ none)

---

## 📊 Test Organization

### Backend Tests

**Integration Tests:**
- `TestStandard` - Full suite with local temp dirs (70+ sub-tests, CI-ready)
- `TestIntegration` - Full suite with configured remote (requires `-remote` flag; see “Go tests: local vs MinIO” below)
- `TestFeatureHandlingWithMask` - Feature handling (Mask, local vs bucket-based); runs with temp dirs, TestRaid3Local, or TestRaid3Minio

**Unit Tests:**
- Byte operations: `TestSplitBytes`, `TestMergeBytes`, `TestSplitMergeRoundtrip`
- Validation: `TestValidateParticleSizes`
- Parity: `TestCalculateParity`, `TestParityFilenames`
- Reconstruction: `TestParityReconstruction`, `TestReconstructFromEvenAndParity`, `TestReconstructFromOddAndParity`, `TestSizeFormulaWithParity`

**Degraded Mode:**
- `TestIntegrationStyle_DegradedOpenAndSize` - Simulates backend failure
- `TestLargeDataQuick` - 1 MB file test

**Rebuild Tests** (`raid3_rebuild_test.go`):
- `TestRebuildEvenBackendSuccess`, `TestRebuildOddBackendSuccess`, `TestRebuildParityBackendSuccess` - Simulate disk swap (wipe one backend), run `backend rebuild`, verify restore and `operations.Check` (local)
- `TestRebuildEvenBackendFailure`, `TestRebuildOddBackendFailure`, `TestRebuildParityBackendFailure` - Two backends lost; rebuild reports 0 rebuilt, read fails (local)
- `TestRebuildMinioBackendSuccess` - MinIO/S3; requires `-remote TestRaid3Minio:` and Docker; purges sub-remote, runs rebuild, verifies. Bash rebuild script skips local/minio/mixed; runs for SFTP only.

**Heal Tests** (`raid3_heal_test.go`, `raid3_heal_command_test.go`):
- `TestHeal`, `TestHealEvenParticle`, `TestHealNoQueue`, `TestHealLargeFile` - Auto-heal on read (some skipped for streaming path)
- `TestHeal*DegradedReadThenRestore` (even, odd, parity) - Remove one particle, read (degraded), run `backend heal`, verify particle restored (local)
- `TestHeal*ListingDoesNotHeal` (even, odd, parity) - Remove particle, list only; assert listing does not heal (local)
- `TestHealMinioDegradedReadThenRestore`, `TestHealMinioListingSucceedsInDegradedMode` - Same flows for MinIO with `-remote TestRaid3Minio:`. Bash heal script skips local/minio/mixed; runs for SFTP only.

**Error Tests** (`raid3_errors_test.go`):
- `TestPutFailsWithUnavailableBackend`, `TestMoveFailsWithUnavailableBackend`, `TestUpdateFailsWithUnavailableBackend` - Table-driven for even/odd/parity (local; use `replaceDirWithFileForTest` to simulate unavailable backend)
- `TestMoveFailsWithRollbackDisabled`, `TestUpdateFailsWithRollbackDisabled` - rollback=false, one backend unavailable; operation must fail (local). Bash errors script skips local; runs for minio, mixed, sftp.

### Go tests: local vs MinIO

The raid3 Go tests can run against two fstest/testserver remotes (or with no remote, using temp dirs):

| Remote | How to run | What runs |
|--------|------------|-----------|
| **None** | `go test ./backend/raid3/... -v` | Unit tests and `TestFeatureHandlingWithMask` with in-process temp dirs; `TestIntegration` is skipped. |
| **TestRaid3Local** | `go test ./backend/raid3/... -remote TestRaid3Local: -v` | Full integration suite (`TestIntegration`) plus `TestFeatureHandlingWithMask`. No Docker; testserver uses local dirs. |
| **TestRaid3Minio** | `go test ./backend/raid3/... -remote TestRaid3Minio: -v` | `TestIntegration` is **skipped** (MinIO uses a config file that the generic fstest suite does not apply). `TestFeatureHandlingWithMask` and other raid3-specific tests run and cover MinIO. Requires Docker. |

So: use **TestRaid3Local** for the full integration run; use **TestRaid3Minio** for MinIO/S3 behaviour (feature handling, etc.). Feature handling is covered by `TestFeatureHandlingWithMask` for both remotes; the full `fs.Fs` suite runs only for TestRaid3Local.

**Quick reference:**
```bash
# Local (full integration + feature test; no Docker)
go test ./backend/raid3/... -remote TestRaid3Local: -v

# MinIO (feature test and other raid3 tests; Docker required)
go test ./backend/raid3/... -remote TestRaid3Minio: -v

# Feature test only (faster)
go test ./backend/raid3/... -run TestFeatureHandlingWithMask -remote TestRaid3Local: -v
go test ./backend/raid3/... -run TestFeatureHandlingWithMask -remote TestRaid3Minio: -v

# Rebuild tests (local temp dirs; even/odd/parity success + even failure)
go test ./backend/raid3/... -run TestRebuild -v

# Rebuild MinIO (requires Docker)
go test ./backend/raid3/... -run TestRebuildMinioBackendSuccess -remote TestRaid3Minio: -v
```

### Test Coverage

| Category | Tests | Coverage |
|----------|-------|----------|
| Integration | 2 | Full fs.Fs interface |
| Byte Operations | 3 | Core striping logic |
| Parity & Reconstruction | 6 | XOR calculation, degraded mode |
| Heal | 4 | Background uploads |
| Rebuild | 5 | even/odd/parity success, even failure (local); MinIO success (TestRaid3Minio) |
| **Total** | **20** | **Comprehensive** |

---

## Manual Testing

Use `setup.sh` from `test/` directory for test environment setup:

```bash
cd backend/raid3/test
./setup.sh

# Upload and verify
echo "Hello, World!" > test.txt
rclone --config tests.config copy test.txt localraid3:
rclone --config tests.config copy localraid3:test.txt downloaded.txt
diff test.txt downloaded.txt
```

### MinIO Testing

Start three MinIO servers (Docker):

```bash
docker run -d --name minioeven -p 9001:9000 -p 9004:9001 \
  -e MINIO_ROOT_USER=even -e MINIO_ROOT_PASSWORD=evenpass88 \
  -v ~/go/raid3storage/even_minio:/data \
  quay.io/minio/minio server /data --console-address ":9001"

docker run -d --name minioodd -p 9002:9000 -p 9005:9001 \
  -e MINIO_ROOT_USER=odd -e MINIO_ROOT_PASSWORD=oddpass88 \
  -v ~/go/raid3storage/odd_minio:/data \
  quay.io/minio/minio server /data --console-address ":9001"

docker run -d --name minioparity -p 9003:9000 -p 9006:9001 \
  -e MINIO_ROOT_USER=parity -e MINIO_ROOT_PASSWORD=paritypass88 \
  -v ~/go/raid3storage/parity_minio:/data \
  quay.io/minio/minio server /data --console-address ":9001"
```

Test degraded read:
```bash
docker stop minioodd
rclone -vv cat minioraid3:testdir/hello.txt --timeout 30s --contimeout 5s
docker start minioodd
```

**S3 / MinIO path and root:** The rclone test facility does **not** create data at the literal root of bucket-based remotes. For remotes like `TestS3:` or `TestRaid3Minio:`, `fstest.Run()` calls `fstest.RandomRemoteName()`, which appends a random subdirectory (e.g. `rclone-test-xxxxxxxxxxxx`). So the effective root is always one path segment (e.g. `TestS3:rclone-test-xxx`), and all object paths have at least two segments (bucket + key). The S3 backend’s `bucket.Split(path)` uses the first segment as the bucket name and the rest as the object Key; with that convention, the key is never empty, so the S3/MinIO integration tests never hit “Key must not be empty”. The raid3 MinIO rebuild test (`TestRebuildMinioBackendSuccess`) and the bash rebuild script follow the same idea: they use a path prefix (e.g. `rclone-rebuild-test` or `create_test_dataset`’s dataset id) so that all object paths have a first segment, and S3 never receives an empty key. The S3 backend does not skip this test; it is only when a test creates data at the true root (e.g. `minioeven:` with path `even-length.bin`) that the first segment becomes the “bucket” and the key is empty.

---

## 🐛 Debugging

**TestStandard fails**: Check sub-test error, verify temp directories are writable.

**Reconstruction fails**: Verify XOR parity (`TestCalculateParity`), split/merge logic (`TestSplitBytes`, `TestMergeBytes`), size formulas.

**Heal fails**: Check background workers, verify Shutdown() is called, check for goroutine leaks.

**Tests fail with "remote not found"**: Ensure `setup.sh` ran successfully, verify config file exists.

**MinIO connection errors**: Ensure MinIO servers are running (ports 9001, 9002, 9003).

---

## Race Condition Detection

The raid3 backend uses extensive concurrency (goroutines, errgroup, channels) for parallel operations. Race condition detection is critical to ensure thread safety.

### Running Tests with Race Detector

```bash
# Run all tests with race detector
go test ./backend/raid3 -race -v

# Run specific concurrent test with race detector
go test ./backend/raid3 -race -run TestConcurrentOperations -v

# Run all tests including race detection (slower but comprehensive)
go test ./backend/raid3 -race -timeout 10m -v
```

### Race Detection in CI/CD

For continuous integration, always run tests with the race detector:

```bash
# Full test suite with race detection
go test ./backend/raid3 -race -timeout 10m -count=1 -v
```

### What the Race Detector Checks

The race detector identifies:
- **Concurrent map access** - Multiple goroutines accessing maps without synchronization
- **Concurrent slice access** - Unsynchronized slice reads/writes
- **Shared variable access** - Variables accessed from multiple goroutines without locks
- **Channel synchronization issues** - Improper channel usage patterns

### Known Race-Safe Patterns

The following patterns are used throughout the codebase and are race-safe:
- **errgroup** - Used for coordinating parallel backend operations
- **sync.Mutex** - Protects shared state (e.g., `uploadQueue.pending` map)
- **Local variables** - Results stored in local variables before assignment to shared structs
- **Channel synchronization** - Channels used for goroutine coordination

### Example: Race-Safe Pattern

```go
// ✅ Race-safe: Local variables, then assign after Wait()
var evenExists, oddExists, parityExists bool
g, gCtx := errgroup.WithContext(ctx)
g.Go(func() error {
    _, err := f.even.NewObject(gCtx, remote)
    evenExists = (err == nil)
    return nil // Ignore errors for existence check
})
// ... similar for odd and parity ...
g.Wait()
// Now safe to assign to shared struct
pi.evenExists = evenExists
pi.oddExists = oddExists
pi.parityExists = parityExists
```

### TestConcurrentOperations

The `TestConcurrentOperations` test is specifically designed to stress-test concurrent operations:

```bash
# Run with race detector (recommended)
go test ./backend/raid3 -race -run TestConcurrentOperations -v

# Run without race detector (faster, but less safe)
go test ./backend/raid3 -run TestConcurrentOperations -v
```

This test verifies:
- Concurrent Put operations don't corrupt data
- Concurrent reads work correctly
- Heal queue handles concurrent uploads
- No race conditions in particle management
- Errgroup coordination works correctly

## Test Coverage Report

```bash
go test ./backend/raid3 -coverprofile=coverage.out
go tool cover -html=coverage.out
```

---

## Known Limitations

Some tests are SKIPped for optional features: `OpenWriterAt`, `OpenChunkWriter`, `ChangeNotify`, `FsPutStream`, `Shutdown`. These are optional rclone features not required for raid3.

---

## 🔗 Related Documentation

- [`../README.md`](../README.md) - User guide and usage examples
- [`RAID3.md`](RAID3.md) - Technical specification
- [`CLEAN_HEAL.md`](CLEAN_HEAL.md) - Self-maintenance guide
- [`../test/README.md`](../test/README.md) - Bash integration tests
