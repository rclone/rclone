# Testing the RAID3 Backend

This document provides testing documentation for the raid3 backend. For user documentation, see [`../README.md`](../README.md). For technical RAID 3 details, see [`RAID3.md`](RAID3.md). For integration test setup, see [`../integration/README.md`](../integration/README.md).

---

## Test Types Overview

The raid3 backend provides three types of tests:

1. **Go tests provided by the raid3 backend** - Unit and integration tests in the `backend/raid3` package. Run with `go test ./backend/raid3 -v`. Includes core RAID 3 operations (split, merge, parity), degraded mode, and heal functionality.

2. **Shell-script based tests provided by the raid3 backend** - Bash integration test harnesses in `backend/raid3/integration/`. Black-box testing for comparison, rebuild, heal, and error handling scenarios. See [`integration/README.md`](../integration/README.md).

3. **Go tests provided by the rclone project** - Comprehensive test suites (`fs/operations` and `fs/sync`) that validate the full `fs.Fs` interface. Run with `go test ./fs/operations -remote localraid3: -v` and `go test ./fs/sync -remote localraid3: -v`.

---

## ✅ Current Test Status

**Last Updated**: 2025-12-25

- **Backend Tests**: ✅ 66 PASS, 0 FAIL, 3 SKIP - `go test ./backend/raid3 -v`
- **fs/sync Tests**: ✅ 96 PASS, 0 FAIL, 12 SKIP - `go test ./fs/sync -remote localraid3: -v`
- **fs/operations Tests**: ✅ All passing - `go test ./fs/operations -remote localraid3: -v`

---

## 🚀 Quick Start

### Backend Tests (Type 1)

```bash
# All tests
go test ./backend/raid3 -v

# Unit tests only
go test ./backend/raid3 -run "^Test(Split|Merge|Calculate|Parity|Validate)" -v

# Integration tests
go test ./backend/raid3 -run "TestStandard" -v
```

### Rclone Framework Tests (Type 3)

```bash
# Setup (one-time)
cd backend/raid3/integration && ./setup.sh

# Run from rclone root
export RCLONE_CONFIG=$(cat ${HOME}/.rclone_raid3_integration_tests.workdir)/rclone_raid3_integration_tests.config
go test ./fs/operations -remote localraid3: -v
go test ./fs/sync -remote localraid3: -v
```

**Alternative** (inline config):
```bash
RCLONE_CONFIG=$(cat ${HOME}/.rclone_raid3_integration_tests.workdir)/rclone_raid3_integration_tests.config \
  go test ./fs/operations -remote localraid3: -v
```

### Bash Integration Tests (Type 2)

See [`integration/README.md`](../integration/README.md) for complete documentation. Scripts include:
- `compare_raid3_with_single.sh` - Comparison harness
- `compare_raid3_with_single_rebuild.sh` - Rebuild validation
- `compare_raid3_with_single_heal.sh` - Heal validation
- `compare_raid3_with_single_errors.sh` - Error handling

---

## 📊 Test Organization

### Backend Tests

**Integration Tests:**
- `TestStandard` - Full suite with local temp dirs (70+ sub-tests, CI-ready)
- `TestIntegration` - Full suite with configured remote (requires `-remote` flag)

**Unit Tests:**
- Byte operations: `TestSplitBytes`, `TestMergeBytes`, `TestSplitMergeRoundtrip`
- Validation: `TestValidateParticleSizes`
- Parity: `TestCalculateParity`, `TestParityFilenames`
- Reconstruction: `TestParityReconstruction`, `TestReconstructFromEvenAndParity`, `TestReconstructFromOddAndParity`, `TestSizeFormulaWithParity`

**Degraded Mode:**
- `TestIntegrationStyle_DegradedOpenAndSize` - Simulates backend failure
- `TestLargeDataQuick` - 1 MB file test

**Heal Tests:**
- `TestHeal`, `TestHealEvenParticle` - Automatic particle restoration
- `TestHealNoQueue` - Fast shutdown optimization
- `TestHealLargeFile` - 100 KB stress test

### Test Coverage

| Category | Tests | Coverage |
|----------|-------|----------|
| Integration | 2 | Full fs.Fs interface |
| Byte Operations | 3 | Core striping logic |
| Parity & Reconstruction | 6 | XOR calculation, degraded mode |
| Heal | 4 | Background uploads |
| **Total** | **16** | **Comprehensive** |

---

## Manual Testing

Use `setup.sh` from `integration/` directory for test environment setup:

```bash
cd backend/raid3/integration
./setup.sh
cd $(cat ${HOME}/.rclone_raid3_integration_tests.workdir)

# Upload and verify
echo "Hello, World!" > test.txt
rclone --config rclone_raid3_integration_tests.config copy test.txt localraid3:
rclone --config rclone_raid3_integration_tests.config copy localraid3:test.txt downloaded.txt
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

---

## 🐛 Debugging

**TestStandard fails**: Check sub-test error, verify temp directories are writable.

**Reconstruction fails**: Verify XOR parity (`TestCalculateParity`), split/merge logic (`TestSplitBytes`, `TestMergeBytes`), size formulas.

**Heal fails**: Check background workers, verify Shutdown() is called, check for goroutine leaks.

**Tests fail with "remote not found"**: Ensure `setup.sh` ran successfully, verify config file exists.

**MinIO connection errors**: Ensure MinIO servers are running (ports 9001, 9002, 9003).

---

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
- [`../integration/README.md`](../integration/README.md) - Bash integration tests
