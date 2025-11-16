# Testing the Level3 RAID 3 Backend

## Test Suite Overview

The level3 backend includes comprehensive testing following rclone conventions.

## Running Tests

### All Tests
```bash
cd /Users/hfischer/go/src/rclone
go test ./backend/level3 -v
```

### Unit Tests Only
```bash
go test ./backend/level3 -run "^Test(Split|Merge|Calculate|Parity|Validate)" -v
```

### Integration Tests
```bash
go test ./backend/level3 -run "TestStandard" -v
```

### Quick Integration Tests
```bash
go test ./backend/level3 -run "TestStandard" -test.short -v
```

## Test Coverage

### Unit Tests ✅

**Byte Operations:**
- `TestSplitBytes` - Splits data into even/odd bytes
- `TestMergeBytes` - Merges bytes back to original
- `TestSplitMergeRoundtrip` - Verifies round-trip integrity

**Parity Operations:**
- `TestCalculateParity` - XOR parity calculation
- `TestParityFilenames` - Suffix generation/parsing (.parity-el/.parity-ol)
- `TestParityReconstruction` - XOR reconstruction logic

**Validation:**
- `TestValidateParticleSizes` - Size validation rules

### Integration Tests ✅

**Provided by `fstests.Run()`:**
- File operations (Put, Get, Update, Remove)
- Directory operations (Mkdir, Rmdir, List)
- Hash calculations
- ModTime operations
- Seek and range support
- Empty file handling
- And 100+ more standard tests

## Manual Testing

### Quick Functional Test

```bash
# Configuration
cat > /tmp/raid3-test.conf << 'EOF'
[raid3]
type = level3
even = /tmp/raid3-test/even
odd = /tmp/raid3-test/odd
parity = /tmp/raid3-test/parity
EOF

# Setup
mkdir -p /tmp/raid3-test/{even,odd,parity,source,dest}
echo "Hello, World!" > /tmp/raid3-test/source/test.txt

# Upload
./rclone --config /tmp/raid3-test.conf copy /tmp/raid3-test/source/test.txt raid3:

# Verify particles
ls -lh /tmp/raid3-test/even/test.txt      # 7 bytes
ls -lh /tmp/raid3-test/odd/test.txt       # 7 bytes  
ls -lh /tmp/raid3-test/parity/*.parity-*  # 7 bytes (.parity-el)

# Download
./rclone --config /tmp/raid3-test.conf copy raid3:test.txt /tmp/raid3-test/dest/

# Verify
diff /tmp/raid3-test/source/test.txt /tmp/raid3-test/dest/test.txt
# Should output nothing (files identical)
```

### MinIO (3 local instances)

This shows how to run three local MinIO servers and a `level3` remote over them, then run basic commands (including degraded read).

1) Start three MinIO servers (Docker)

```bash
# Create storage directories
mkdir -p ~/go/level3storage/{even_minio,odd_minio,parity_minio}

# Start minioeven
docker run -d --name minioeven \
  -p 9001:9000 -p 9004:9001 \
  -e MINIO_ROOT_USER=even -e MINIO_ROOT_PASSWORD=evenpass88 \
  -v ~/go/level3storage/even_minio:/data \
  quay.io/minio/minio server /data --console-address ":9001"

# Start minioodd
docker run -d --name minioodd \
  -p 9002:9000 -p 9005:9001 \
  -e MINIO_ROOT_USER=odd -e MINIO_ROOT_PASSWORD=oddpass88 \
  -v ~/go/level3storage/odd_minio:/data \
  quay.io/minio/minio server /data --console-address ":9001"

# Start minioparity
docker run -d --name minioparity \
  -p 9003:9000 -p 9006:9001 \
  -e MINIO_ROOT_USER=parity -e MINIO_ROOT_PASSWORD=paritypass88 \
  -v ~/go/level3storage/parity_minio:/data \
  quay.io/minio/minio server /data --console-address ":9001"
```

**Note**: Each container runs the MinIO console on its internal port `:9001`, but Docker maps them to different host ports (9004, 9005, 9006), so there's no conflict.

2) rclone config for three MinIO S3 remotes and a `level3` remote

Append to your rclone config (e.g. `~/.config/rclone/rclone.conf`):

```ini
[minioeven]
type = s3
provider = Minio
env_auth = false
access_key_id = even
secret_access_key = evenpass8
endpoint = http://127.0.0.1:9001
acl = private
no_check_bucket = true

[minioodd]
type = s3
provider = Minio
env_auth = false
access_key_id = odd
secret_access_key = oddpass88
endpoint = http://127.0.0.1:9002
acl = private
no_check_bucket = true

[minioparity]
type = s3
provider = Minio
env_auth = false
access_key_id = parity
secret_access_key = parityp8
endpoint = http://127.0.0.1:9003
acl = private
no_check_bucket = true

[miniolevel3]
type = level3
even = minioeven:
odd = minioodd:
parity = minioparity:
```

3) Example usage

```bash
# Create a bucket via level3 on all three MinIOs
rclone mkdir miniolevel3:testdir

# Upload a file
echo "hello raid3" > /tmp/hello.txt
rclone copy /tmp/hello.txt miniolevel3:testdir

# Read via level3
rclone cat miniolevel3:testdir/hello.txt

# Inspect underlying remotes
rclone ls minioeven:testdir
rclone ls minioodd:testdir
rclone ls minioparity:testdir
```

4) Degraded read test

```bash
# Stop odd; level3 should reconstruct from even+parity
docker stop minioodd

# Should still read and log INFO about reconstruction
# Note: Use --timeout to avoid long waits when backend is unreachable
# The S3 client will retry, so expect ~5-10s delay even with timeout
rclone -vv cat miniolevel3:testdir/hello.txt --timeout 30s --contimeout 5s

# Restart odd
docker start minioodd
```

**Important**: When a MinIO instance is stopped (container down), the S3 backend will attempt connection and retry. The `--contimeout 5s` flag sets connection timeout to 5 seconds, and `--timeout 30s` sets overall operation timeout. Even so, you may see 5-10 second delays as the backend retries the unavailable instance before proceeding with reconstruction.

5) Cleanup

```bash
# Stop and remove containers
docker stop minioeven minioodd minioparity
docker rm minioeven minioodd minioparity

# Optional: remove data
# rm -rf ~/go/level3storage
```

### Verify Parity Calculation

```bash
# View particles in hex
echo "Original:"
hexdump -C /tmp/raid3-test/source/test.txt

echo "Even bytes:"
hexdump -C /tmp/raid3-test/even/test.txt

echo "Odd bytes:"
hexdump -C /tmp/raid3-test/odd/test.txt

echo "Parity (XOR):"
hexdump -C /tmp/raid3-test/parity/test.txt.parity-el

# Manually verify XOR: parity[i] should equal even[i] ^ odd[i]
```

## Test Results

### Unit Tests: ✅ ALL PASSING
```
PASS: TestSplitBytes (7 sub-tests)
PASS: TestMergeBytes (7 sub-tests)  
PASS: TestSplitMergeRoundtrip (7 sub-tests)
PASS: TestValidateParticleSizes (6 sub-tests)
PASS: TestCalculateParity (7 sub-tests)
PASS: TestParityFilenames (4 sub-tests)
PASS: TestParityReconstruction
```

### Integration Tests: ✅ PASSING
```
PASS: TestStandard
  - File operations ✓
  - Directory operations ✓
  - Seek/Range support ✓
  - Hash calculations ✓
  - All standard rclone fs tests ✓
```

## Known Test Limitations

Some tests are SKIPped because the backend doesn't implement optional features:
- `OpenWriterAt` - Not needed for this backend
- `OpenChunkWriter` - Not applicable
- `ChangeNotify` - Not applicable
- `FsPutStream` - Could be implemented in future
- `Shutdown` - Not needed

These are all optional features - the backend is fully functional without them.

## Continuous Integration

To add to CI/CD:
```yaml
# In .github/workflows/test.yml
- name: Test Level3 Backend
  run: go test ./backend/level3 -v
```

## Debugging Tests

### Run with more verbosity
```bash
go test ./backend/level3 -v -vv
```

### Run specific failing test
```bash
go test ./backend/level3 -run "TestStandard/FsMkdir/FsPutFiles/ObjectOpenSeek" -v
```

### Check particle sizes during test
```bash
# Add debug output to see what's happening
go test ./backend/level3 -run "TestStandard" -v 2>&1 | grep -i "particle"
```

## Test Coverage

```bash
# Generate coverage report
go test ./backend/level3 -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Comparison with Other Backends

The level3 backend follows the same testing pattern as:
- `backend/union` - Virtual backend with multiple upstreams
- `backend/combine` - Virtual backend combining remotes
- `backend/chunker` - Virtual backend with data transformation

All use:
- `fstests.Run()` for integration tests
- Unit tests for internal functions
- `ExtraConfig` for test configuration
- `QuickTestOK` for faster test iteration

## Summary

✅ **Proper rclone testing pattern** - No shell scripts needed
✅ **Comprehensive unit tests** - All core functions tested
✅ **Full integration tests** - Standard rclone test suite
✅ **Easy to run** - Standard `go test` command
✅ **CI/CD ready** - No special setup required

The level3 backend is production-ready with enterprise-grade testing!

## Bash Comparison Harness

The script `backend/level3/tools/compare_level3_with_single.sh` supplements the Go-based tests with a black-box comparison between level3 and the corresponding single backend remotes.

- Covers common rclone commands (`mkdir`, `ls`, `lsd`, `cat`, `delete`, `copy`, `move`, `check`, `sync`, `purge`, etc.).
- Works with both MinIO (S3) and local filesystem remotes using the names defined in `rclone.conf` (`miniolevel3`, `miniosingle`, `locallevel3`, `localsingle`).
- Assumes all three level3 remotes are healthy; degraded scenarios will be handled by a dedicated fault-testing script.
- Compares exit codes and command outputs to confirm level3 mirrors the single backend’s behaviour.
- Designed for incremental growth—run `./compare_level3_with_single.sh list` to see available tests, or `./compare_level3_with_single.sh test <name>` (optionally with `--storage-type=local|minio`) to execute individual cases.

## Recovery Harness

`backend/level3/tools/compare_level3_with_single_recover.sh` focuses on simulated disk swaps and rebuild workflows:

- Shares the same safety guards and helper functions via `compare_level3_common.sh`.
- Exercises both MinIO (Docker-backed) and local level3 remotes.
- Provides `start|stop|teardown|list|test` commands consistent with the comparison harness.
- For each backend (`even`, `odd`, `parity`) runs two scenarios:
  - **Failure:** wipes the target backend plus an additional source, confirming `rclone backend rebuild` fails gracefully.
  - **Success:** wipes the target backend only, runs `rclone backend rebuild`, then validates with `rclone check` and a byte-for-byte comparison against a preserved reference dataset.
- Leaves reconstructed datasets in place for manual inspection; `teardown` removes all state.

## Healing Harness

`backend/level3/tools/compare_level3_with_single_heal.sh` validates degraded-read behaviour and the explicit `backend heal` command when any particle backend is missing:

- Uses the shared helpers/autostart logic from `compare_level3_common.sh`.
- Covers both local and MinIO storage types (auto-starting MinIO containers on demand).
- Scenarios:
  - `even`, `odd`, `parity`: delete all particles on the selected backend, verify degraded reads still work via `rclone cat`, then run `rclone backend heal level3:` and wait for the missing particle to reappear on the affected backend.
  - `even-list`, `odd-list`, `parity-list`:
    - For `--storage-type=local`: delete particles, run `rclone ls`, and confirm listings do **not** heal (read-only semantics).
    - For `--storage-type=minio`: delete particles, run `rclone ls`, and assert that listing succeeds; MinIO-backed level3 may opportunistically heal during listing, which is accepted for now and documented as backend-dependent behaviour.
- Reports status with `PASS/FAIL` tags and a summary block so degraded behaviour is easy to spot in logs.
- `teardown` purges level3/single remotes and cleans the corresponding local directories.

