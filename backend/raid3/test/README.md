# Integration Tests - Bash Test Harnesses

## Purpose of This Document

This document provides **complete documentation** for the Bash-based integration test scripts in the `test/` directory. It covers:

- **Quick start guide** - How to set up and run tests quickly
- **Test script descriptions** - What each script does and when to use it
- **Configuration** - How to customize test settings
- **Usage examples** - Common workflows and commands
- **Troubleshooting** - Common issues and solutions
- **Platform compatibility** - Supported environments

These scripts supplement the Go-based unit and integration tests with black-box testing scenarios for complex real-world use cases.

For **main testing documentation**, see [`../docs/TESTING.md`](../docs/TESTING.md).  
For **user documentation**, see [`../README.md`](../README.md).

---

This directory contains comprehensive Bash-based integration test scripts for validating the raid3 backend functionality. These scripts supplement the Go-based unit and integration tests with black-box testing scenarios.

## 📋 Quick Start

### 1. Initial Setup

Run the setup script to create the test environment:

```bash
cd backend/raid3/test
./setup.sh
```

The setup script will:
- Create the `_data` subdirectory and all required subdirectories within it
- Generate the rclone configuration file: `rclone_raid3_integration_tests.config` in the test directory

### 2. Run Tests

All test scripts must be run from the test directory:

```bash
# Change to the test directory
cd backend/raid3/test

# Run a test
./compare_raid3_with_single.sh --storage-type local test mkdir
```

### Quick test: sftpsingle remote

To verify the SFTP remotes work (Docker with atmoz/sftp required). On ARM64 (e.g. Apple Silicon) you may see a platform mismatch warning (linux/amd64 vs linux/arm64); the image runs under emulation and is fine for these tests.

```bash
cd backend/raid3/test

# Ensure config and _data dirs exist (includes even_sftp, odd_sftp, parity_sftp, single_sftp)
./setup.sh

# Start the four SFTP containers (ports 2221–2224)
./compare_raid3_with_single.sh start --storage-type=sftp

# Check that sftpsingle works: list root (should be empty or show existing dirs)
rclone --config rclone_raid3_integration_tests.config lsd sftpsingle:

# Create a test file and copy it to sftpsingle
echo "hello sftp" > /tmp/hello.txt
rclone --config rclone_raid3_integration_tests.config copy /tmp/hello.txt sftpsingle:test/

# List again to see test/
rclone --config rclone_raid3_integration_tests.config ls sftpsingle:

# Run one comparison test (raid3 vs single) with SFTP
./compare_raid3_with_single.sh --storage-type=sftp test mkdir

# Stop SFTP containers when done
./compare_raid3_with_single.sh stop --storage-type=sftp
```

## 📁 Test Scripts

| Script | Purpose | Commands |
|--------|---------|----------|
| **`setup.sh`** | Initial environment setup | `setup.sh` |
| **`compare_raid3_with_single.sh`** | Black-box comparison tests | `start`, `stop`, `teardown`, `list`, `test <name>` |
| **`compare_raid3_with_single_rebuild.sh`** | Rebuild command validation | `start`, `stop`, `teardown`, `list`, `test <name>` |
| **`compare_raid3_with_single_heal.sh`** | Auto-heal functionality tests | `start`, `stop`, `teardown`, `list`, `test <name>` |
| **`compare_raid3_with_single_errors.sh`** | Error handling and rollback tests | `start`, `stop`, `teardown`, `list`, `test <name>` |
| **`compare_raid3_with_single_features.sh`** | Feature handling with mixed remotes | `start`, `stop`, `teardown`, `list`, `test <name>` |
| **`compare_raid3_with_single_all.sh`** | Run all test suites across all backends | `[-v]`, `[-h]` |
| **`performance_test.sh`** | Performance benchmarks (upload/download) for different file sizes and storage types | `start`, `stop`, `teardown`, `list`, `test [all\|all-but-4G\|4K\|…]` |
| **`compression_bench.sh`** | Measures compression ratio for local raid3 (requires `--storage-type=local`; config compression ≠ none) | `--storage-type=local` |

### Common Commands

All test scripts support these commands:

- **`list`** - Show available test cases
- **`test <name>`** - Run a named test (e.g., `test mkdir`)
- **`start`** - Start MinIO or SFTP containers (requires `--storage-type=minio` or `--storage-type=sftp`)
- **`stop`** - Stop MinIO or SFTP containers (requires `--storage-type=minio` or `--storage-type=sftp`)
- **`teardown`** - Purge all test data for the selected storage type

### Options

- **`--storage-type <local\|minio\|mixed\|sftp>`** - Select backend pair (required for most commands)
- **`-v, --verbose`** - Show detailed output from rclone invocations
- **`-h, --help`** - Display help text

## 🔧 Configuration

### Test Configuration File

The integration tests use a **strict, test-specific configuration file**:

**Location**: `backend/raid3/test/rclone_raid3_integration_tests.config`

The config file is created in the test directory by `setup.sh`.

The configuration file contains:
- Local storage remotes (localeven, localodd, localparity, localsingle)
- MinIO S3 remotes (minioeven, minioodd, minioparity, miniosingle)
- SFTP remotes (sftpeven, sftpodd, sftpparity, sftpsingle) for atmoz/sftp Docker containers
- RAID3 remotes:
  - `localraid3` - All local file-based backends
  - `minioraid3` - All MinIO S3 object-based backends
  - `localminioraid3` - Mixed file/object backends (local even/parity, MinIO odd) for testing heterogeneous storage scenarios
  - `sftpraid3` - All SFTP backends (one container per shard: even, odd, parity, single)

**Important**: 
- The test scripts **only** use this test-specific config file. They do not use your default `~/.config/rclone/rclone.conf`.
- Tests **verify that the config file exists** before running. If the config file is missing, tests will exit with an error message directing you to run `setup.sh`.

### Customizing Test Configuration

You can override default settings by creating `compare_raid3_env.local.sh`:

```bash
# Create override file
cat > compare_raid3_env.local.sh << 'EOF'
#!/usr/bin/env bash
# Custom MinIO ports (if default ports conflict)
MINIO_EVEN_PORT=9101
MINIO_ODD_PORT=9102
MINIO_PARITY_PORT=9103
MINIO_SINGLE_PORT=9104

# Custom data directory (optional)
DATA_DIR="${SCRIPT_DIR}/custom_data"
EOF

# Run setup again to apply changes
./setup.sh
```

This file is automatically sourced by all test scripts if present. **Do not commit this file** to version control.

## 🎯 Test Script Details

### Comparison Tests (`compare_raid3_with_single.sh`)

Black-box comparison between raid3 and single backend remotes.

**What it tests**:
- Common rclone commands: `mkdir`, `ls`, `lsd`, `cat`, `delete`, `copy`, `move`, `check`, `sync`, `purge`
- Exit codes and command outputs
- Ensures raid3 mirrors single backend behavior

**Usage**:
```bash
# List available tests
./compare_raid3_with_single.sh list

# Run a specific test
./compare_raid3_with_single.sh --storage-type local test mkdir
./compare_raid3_with_single.sh --storage-type minio test copy -v
```

### Rebuild Tests (`compare_raid3_with_single_rebuild.sh`)

Validates the `rclone backend rebuild` command after simulated backend replacement.

**What it tests**:
- Rebuild scenarios for each backend (even, odd, parity)
- Failure cases (missing source backends; rebuild reports 0 files rebuilt)
- Success cases (complete rebuild; post-rebuild `rclone check` passes with correct logical sizes and content)
- Byte-for-byte comparison of rebuilt dataset against reference copy

**Usage**:
```bash
# Run rebuild tests
./compare_raid3_with_single_rebuild.sh --storage-type local test even
./compare_raid3_with_single_rebuild.sh --storage-type minio test odd -v
```

### Heal Tests (`compare_raid3_with_single_heal.sh`)

Validates degraded-read behavior and the explicit `backend heal` command.

**What it tests**:
- Degraded mode reads (2/3 particles available)
- Automatic particle reconstruction
- Explicit `rclone backend heal` command
- Directory reconstruction behavior

**Usage**:
```bash
# Run heal tests
./compare_raid3_with_single_heal.sh --storage-type local test even
./compare_raid3_with_single_heal.sh --storage-type minio test parity-list -v
```

### Error Handling Tests (`compare_raid3_with_single_errors.sh`)

Validates error handling, rollback behavior, and degraded mode write blocking.

**What it tests**:
- Write operations blocked in degraded mode
- Rollback behavior for Put, Move, Update operations
- Error messages and recovery
- Health check functionality

**Usage**:
```bash
# Run error handling tests (minio only - requires containers to stop)
./compare_raid3_with_single_errors.sh --storage-type minio test put-fail-even
./compare_raid3_with_single_errors.sh --storage-type minio test move-fail-odd -v
```

### Feature Handling Tests (`compare_raid3_with_single_features.sh`)

Validates feature handling across different remote type configurations.

**What it tests**:
- **local**: Feature handling when all three backends are local filesystem
- **minio**: Feature handling when all three backends are MinIO object storage
- **mixed**: Feature intersection when mixing local and MinIO backends (AND logic for most features)
- Best-effort features (OR logic for metadata, raid3-specific)
- Always-available features (Shutdown, CleanUp - raid3 implements independently)
- Verifies features are correctly intersected/disabled when mixing incompatible backends

**Usage**:
```bash
# Run all feature handling tests for a storage type
./compare_raid3_with_single_features.sh --storage-type local test
./compare_raid3_with_single_features.sh --storage-type minio test
./compare_raid3_with_single_features.sh --storage-type mixed test

# Run a specific test
./compare_raid3_with_single_features.sh --storage-type local test local-features
./compare_raid3_with_single_features.sh --storage-type minio test minio-features
./compare_raid3_with_single_features.sh --storage-type mixed test mixed-features

# With verbose output
./compare_raid3_with_single_features.sh --storage-type mixed test -v
./compare_raid3_with_single_features.sh --storage-type mixed test mixed-features -v
```

**What it tests**:
- **local**: Verifies features when all three backends are local filesystem
- **minio**: Verifies features when all three backends are MinIO object storage
- **mixed**: Verifies feature intersection when mixing local and MinIO backends

### Master Test Script (`compare_raid3_with_single_all.sh`)

Runs all test suites across all RAID3 backends with minimal output (pass/fail only).

**What it does**:
- Runs `compare_raid3_with_single.sh` with local, minio, and mixed storage types
- Runs `compare_raid3_with_single_heal.sh` with local, minio, and mixed storage types
- Runs `compare_raid3_with_single_errors.sh` with minio only
- Runs `compare_raid3_with_single_rebuild.sh` with local, minio, and mixed storage types
- Runs `compare_raid3_with_single_features.sh` with mixed storage type only
- Provides summary of all test results

**Usage**:
```bash
# Run all tests with minimal output (default)
./compare_raid3_with_single_all.sh

# Run all tests with verbose output
./compare_raid3_with_single_all.sh --verbose

# Show help
./compare_raid3_with_single_all.sh --help
```

**Output**: Shows only pass/fail status for each test combination, with a final summary. Use `--verbose` to see detailed output from individual test scripts.

## 🖥️ Platform Compatibility

**Supported Platforms**:
- ✅ Linux
- ✅ macOS
- ✅ WSL (Windows Subsystem for Linux)
- ✅ Git Bash (Windows)
- ✅ Cygwin (Windows)

**Not Supported**:
- ❌ Native Windows (cmd.exe or PowerShell)

The scripts will detect native Windows and provide instructions to use WSL, Git Bash, or Cygwin.

## ⚠️ Error Messages

The test scripts provide clear error messages if the environment is not set up:

| Error | Solution |
|-------|----------|
| Missing config file | Run `./setup.sh` to create `rclone_raid3_integration_tests.config` |
| Wrong directory | Change to `backend/raid3/test` directory |

**Note**: All test scripts verify that `rclone_raid3_integration_tests.config` exists before executing any tests. If the config file is missing, the script will exit immediately with a clear error message indicating that you need to run `setup.sh` first.

## 📚 Additional Documentation

- **Setup and Configuration Details**: See [`../README.md`](../README.md#integration-test-scripts) for comprehensive setup instructions
- **Technical Details**: See [`../docs/TESTING.md`](../docs/TESTING.md#bash-integration-test-harnesses) for implementation details
- **Script Help**: Run any script with `-h` or `--help` for usage information

## 🔍 File Structure

```
test/
├── README.md                              # This file
├── setup.sh                               # Initial setup script
├── compare_raid3_env.sh                  # Default environment variables
├── compare_raid3_env.local.sh            # Local overrides (not tracked)
├── compare_raid3_with_single_common.sh    # Shared helper functions
├── compare_raid3_with_single.sh          # Comparison tests
├── compare_raid3_with_single_rebuild.sh   # Rebuild tests
├── compare_raid3_with_single_heal.sh     # Heal tests
├── compare_raid3_with_single_errors.sh    # Error handling tests
├── compare_raid3_with_single_features.sh  # Feature handling tests
├── compare_raid3_with_single_all.sh       # Master script (runs all tests)
├── rclone_raid3_integration_tests.config  # Config file (created by setup.sh)
└── _data/                                 # Test data directory (created by setup.sh, gitignored)
    ├── even_local/
    ├── odd_local/
    ├── single_local/
    ├── parity_local/
    ├── even_minio/
    ├── odd_minio/
    ├── single_minio/
    └── parity_minio/
```

## 🚀 Example Workflow

```bash
# 1. Initial setup (one-time)
cd backend/raid3/test
./setup.sh

# 2. List available tests
./compare_raid3_with_single.sh list

# 3. Run a comparison test
./compare_raid3_with_single.sh --storage-type local test mkdir

# 4. Run rebuild tests
./compare_raid3_with_single_rebuild.sh --storage-type local test even

# 5. Run heal tests
./compare_raid3_with_single_heal.sh --storage-type minio test odd -v

# 6. Run feature handling tests
./compare_raid3_with_single_features.sh --storage-type mixed test mixed-features

# 7. Run all tests at once (recommended for CI/validation)
./compare_raid3_with_single_all.sh

# 7. Clean up (optional)
./compare_raid3_with_single.sh --storage-type local teardown
```

## 🔧 Debugging sync-upload timeout (MinIO + multipart)

If `sync-upload` fails with "timed out after 120s (possible raid3 hang)" when using `--storage-type=minio`:

### Root cause

- **Raid3** uploads via a **streaming** path: it does not know the object size up front, so it passes size `-1` to the underlying S3 backends.
- **rclone S3** uses **multipart upload** whenever size is unknown (`size < 0`) or ≥ `upload_cutoff`. So every raid3 upload to MinIO goes through multipart (CreateMultipartUpload + parts).
- **MinIO** has a known history of multipart issues: CreateMultipartUpload or PutObjectPart can hang or timeout (see [minio/minio#9608](https://github.com/minio/minio/issues/9608), [rclone forum](https://forum.rclone.org/t/trouble-uploading-multi-part-files-to-s3-minio/42941)). The last log line before the hang is often:  
  `NOTICE: S3 bucket …: Streaming uploads using chunk size 5Mi will have maximum file size of 48.828Gi`.

So the timeout is almost certainly **MinIO blocking on multipart** (CreateMultipartUpload or first part), not raid3 itself.

### What we already do

- **upload_cutoff = 5G** is set on all MinIO S3 remotes in the test config. That only avoids multipart when the backend sees a **known** size &lt; 5G. Raid3’s streaming path always sends unknown size, so multipart is still used for sync-upload.
- **MinIO image** is pinned to `RELEASE.2025-09-07T16-13-09Z` (newest on Docker Hub with multipart bugfixes). Override with `MINIO_IMAGE=minio/minio:latest` if needed.
- **Timeouts** (e.g. 120s) and stderr dumps on timeout so the run fails clearly instead of hanging forever.
- **cp-upload** and **sync-upload** use a 120s timeout and one retry for the raid3 upload when using MinIO/mixed; on failure, MinIO container logs are written to a temp file for inspection (see “Analyzing MinIO Docker logs” below).

### Workarounds

1. **Run sync-upload against local only** (avoids MinIO):  
   `./compare_raid3_with_single.sh test sync-upload --storage-type=local`
2. **Try a different MinIO version**: set `MINIO_IMAGE=quay.io/minio/minio:latest` (or another tag), recreate containers (`stop` then `start --storage-type=minio`), then rerun the test.
3. **Increase timeout** for a slow environment:  
   `RCLONE_TEST_TIMEOUT=300 ./compare_raid3_with_single.sh test sync-upload --storage-type=minio`

### Getting more detail

1. **Run with `-v`** so the last 30 lines of rclone stderr are printed on timeout:  
   `./compare_raid3_with_single.sh test sync-upload --storage-type=minio -v`

2. **Reproduce manually** to capture full output:
   ```bash
   cd backend/raid3/test
   export RCLONE_CONFIG="${PWD}/rclone_raid3_integration_tests.config"
   ./setup.sh  # ensure MinIO is running
   ./compare_raid3_with_single.sh start --storage-type=minio  # if needed
   mkdir -p /tmp/sync-debug && echo "f1" > /tmp/sync-debug/f1.txt && echo "f2" > /tmp/sync-debug/sub/f2.txt
   rclone sync /tmp/sync-debug minioraid3:sync-debug-test -vv 2>&1 | tee sync_initial.log
   rm /tmp/sync-debug/f1.txt && echo "f2 updated" > /tmp/sync-debug/sub/f2.txt && echo "f3" > /tmp/sync-debug/f3.txt
   rclone sync /tmp/sync-debug minioraid3:sync-debug-test -vv 2>&1 | tee sync_delta.log
   # If it hangs, sync_delta.log shows where (last line = last operation before hang)
   ```

3. **Reproduce with MinIO request trace** to see which S3 call MinIO was handling when a hang occurs:
   ```bash
   ./repro_minio_timeout_with_trace.sh 10
   ```
   This runs cp-upload 10 times with `mc admin trace` on all three raid3 MinIO backends. On failure, the trace is saved to `/tmp/minio_trace_repro/trace.log`; inspect the last lines for the final request before the hang (e.g. `s3.NewMultipartUpload`, `s3.PutObjectPart`). Requires `mc` (MinIO Client).

### Analyzing MinIO Docker logs

When a test fails with `--storage-type=minio` (e.g. timeout 124, or status mismatch), the script may write MinIO container logs to a temp file and log its path, e.g.:

```text
[compare_raid3_with_single.sh] WARN minio-logs MinIO container logs (last 150 lines each) saved to: /tmp/minio_logs_cp-upload.XXXXXX
```

**Inspect that file:** `cat /tmp/minio_logs_cp-upload.*` (or the path shown).

**Or capture logs manually** for a specific container (e.g. the one raid3 uses for even/odd/parity):

```bash
docker logs minioeven  2>&1 | tail -200
docker logs minioodd   2>&1 | tail -200
docker logs minioparity 2>&1 | tail -200
docker logs miniosingle 2>&1 | tail -200
```

**What to look for:**

- **CreateMultipartUpload** – request may be stuck or very slow; last MinIO log line before a hang is often the S3 API handler for that call.
- **PutObjectPart** – similar; multipart upload part writes can block.
- **Timeout / context canceled** – client gave up; check rclone stderr (and `-v` output) for the last operation.
- **Connection reset / refused** – MinIO restarted or became unavailable; check for OOM or panic in the full log.

For **cp-upload** and **sync-upload**, raid3 uses the streaming path (unknown size), so S3 uses multipart; MinIO’s multipart handling is the usual suspect when the test times out.

## MinIO containers exit immediately

If MinIO containers start then stop right away (Docker Desktop shows them stopping when you press Play):

1. **See why:** Run `docker logs minioeven` (or any of `minioodd`, `minioparity`, `miniosingle`). The last lines usually show the error.

2. **"Unknown xl meta version 3" (or similar):** The data on disk was written by a **newer** MinIO than the image you're running. You must wipe the MinIO data dirs so the current image can start with empty storage:
   ```bash
   cd backend/raid3/test
   ./compare_raid3_with_single.sh stop --storage-type=minio
   docker rm -f minioeven minioodd minioparity miniosingle 2>/dev/null || true
   rm -rf _data/even_minio _data/odd_minio _data/parity_minio _data/single_minio
   ./compare_raid3_with_single.sh start --storage-type=minio
   ```

3. **Other errors:** Remove containers and optionally wipe data, then start again:
   ```bash
   cd backend/raid3/test
   ./compare_raid3_with_single.sh stop --storage-type=minio
   docker rm -f minioeven minioodd minioparity miniosingle 2>/dev/null || true
   rm -rf _data/even_minio _data/odd_minio _data/parity_minio _data/single_minio
   ./compare_raid3_with_single.sh start --storage-type=minio
   ```

4. **Try a different image:** If the pinned image fails on your host (e.g. architecture), try the latest image:
   ```bash
   MINIO_IMAGE=minio/minio:latest ./compare_raid3_with_single.sh start --storage-type=minio
   ```

5. **Ports in use:** Ensure nothing else is using ports 9001–9004 (e.g. `lsof -i :9001` on the host).

## 💡 Tips

- **Verbose output**: Use `-v` flag to see detailed rclone command output
- **MinIO containers**: The scripts automatically start/stop MinIO containers when using `--storage-type=minio`
- **Test isolation**: Each test cleans up after itself, but you can use `teardown` to purge all data
- **Custom configuration**: Create `compare_raid3_env.local.sh` to override defaults without modifying tracked files
- **Quick help**: Run any script with `-h` or `--help` for command-specific help
