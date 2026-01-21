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

### Common Commands

All test scripts support these commands:

- **`list`** - Show available test cases
- **`test <name>`** - Run a named test (e.g., `test mkdir`)
- **`start`** - Start MinIO containers (requires `--storage-type=minio`)
- **`stop`** - Stop MinIO containers (requires `--storage-type=minio`)
- **`teardown`** - Purge all test data for the selected storage type

### Options

- **`--storage-type <local\|minio\|mixed>`** - Select backend pair (required for most commands)
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
- RAID3 remotes:
  - `localraid3` - All local file-based backends
  - `minioraid3` - All MinIO S3 object-based backends
  - `localminioraid3` - Mixed file/object backends (local even/parity, MinIO odd) for testing heterogeneous storage scenarios

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
- Failure cases (missing source backends)
- Success cases (complete rebuild)
- Validation with `rclone check` and byte-for-byte comparison

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

Validates feature handling when mixing different remote types (local filesystem + MinIO S3).

**What it tests**:
- Feature intersection with mixed remotes (AND logic for most features)
- Best-effort features (OR logic for metadata, raid3-specific)
- Always-available features (Shutdown, CleanUp - raid3 implements independently)
- Verifies features are correctly disabled when mixing incompatible backends

**Usage**:
```bash
# Run all feature handling tests (mixed only - requires local + MinIO)
./compare_raid3_with_single_features.sh --storage-type mixed test

# Run a specific test
./compare_raid3_with_single_features.sh --storage-type mixed test mixed-features

# With verbose output
./compare_raid3_with_single_features.sh --storage-type mixed test -v
./compare_raid3_with_single_features.sh --storage-type mixed test mixed-features -v
```

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

## 💡 Tips

- **Verbose output**: Use `-v` flag to see detailed rclone command output
- **MinIO containers**: The scripts automatically start/stop MinIO containers when using `--storage-type=minio`
- **Test isolation**: Each test cleans up after itself, but you can use `teardown` to purge all data
- **Custom configuration**: Create `compare_raid3_env.local.sh` to override defaults without modifying tracked files
- **Quick help**: Run any script with `-h` or `--help` for command-specific help
