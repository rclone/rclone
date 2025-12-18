# Integration Tests - Bash Test Harnesses

## Purpose of This Document

This document provides **complete documentation** for the Bash-based integration test scripts in the `integration/` directory. It covers:

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
# Default working directory: ${HOME}/go/raid3storage
./setup.sh

# Or specify a custom directory
./setup.sh --workdir /path/to/your/test/directory
```

The setup script will:
- Create the working directory and all required subdirectories
- Generate the rclone configuration file: `${WORKDIR}/rclone_raid3_integration_tests.config`
- Store the working directory path in: `${HOME}/.rclone_raid3_integration_tests.workdir`

### 2. Run Tests

All test scripts must be run from the working directory:

```bash
# Change to the working directory
cd $(cat ${HOME}/.rclone_raid3_integration_tests.workdir)

# Run a test
./backend/raid3/integration/compare_raid3_with_single.sh --storage-type local test mkdir
```

## 📁 Test Scripts

| Script | Purpose | Commands |
|--------|---------|----------|
| **`setup.sh`** | Initial environment setup | `setup.sh [--workdir <path>]` |
| **`compare_raid3_with_single.sh`** | Black-box comparison tests | `start`, `stop`, `teardown`, `list`, `test <name>` |
| **`compare_raid3_with_single_rebuild.sh`** | Rebuild command validation | `start`, `stop`, `teardown`, `list`, `test <name>` |
| **`compare_raid3_with_single_heal.sh`** | Auto-heal functionality tests | `start`, `stop`, `teardown`, `list`, `test <name>` |
| **`compare_raid3_with_single_errors.sh`** | Error handling and rollback tests | `start`, `stop`, `teardown`, `list`, `test <name>` |

### Common Commands

All test scripts support these commands:

- **`list`** - Show available test cases
- **`test <name>`** - Run a named test (e.g., `test mkdir`)
- **`start`** - Start MinIO containers (requires `--storage-type=minio`)
- **`stop`** - Stop MinIO containers (requires `--storage-type=minio`)
- **`teardown`** - Purge all test data for the selected storage type

### Options

- **`--storage-type <local\|minio>`** - Select backend pair (required for most commands)
- **`-v, --verbose`** - Show detailed output from rclone invocations
- **`-h, --help`** - Display help text

## 🔧 Configuration

### Test Configuration File

The integration tests use a **strict, test-specific configuration file**:

**Location**: `${WORKDIR}/rclone_raid3_integration_tests.config`

Where `WORKDIR` is determined by reading `${HOME}/.rclone_raid3_integration_tests.workdir` (created by `setup.sh`).

The configuration file contains:
- Local storage remotes (localeven, localodd, localparity, localsingle)
- MinIO S3 remotes (minioeven, minioodd, minioparity, miniosingle)
- RAID3 remotes (localraid3, minioraid3)

**Important**: The test scripts **only** use this test-specific config file. They do not use your default `~/.config/rclone/rclone.conf`.

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

# Custom work directory (optional - can also use setup.sh --workdir)
WORKDIR="${HOME}/custom/raid3test"
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
# Run error handling tests
./compare_raid3_with_single_errors.sh --storage-type local test put-fail-even
./compare_raid3_with_single_errors.sh --storage-type minio test move-fail-odd -v
```

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
| Missing workdir file | Run `./setup.sh` |
| Missing working directory | Run `./setup.sh --workdir <path>` |
| Missing config file | Run `./setup.sh` |
| Wrong directory | Change to the directory shown in the error message |

## 📚 Additional Documentation

- **Setup and Configuration Details**: See [`../README.md`](../README.md#integration-test-scripts) for comprehensive setup instructions
- **Technical Details**: See [`../docs/TESTING.md`](../docs/TESTING.md#bash-integration-test-harnesses) for implementation details
- **Script Help**: Run any script with `-h` or `--help` for usage information

## 🔍 File Structure

```
integration/
├── README.md                              # This file
├── setup.sh                               # Initial setup script
├── compare_raid3_env.sh                  # Default environment variables
├── compare_raid3_env.local.sh            # Local overrides (not tracked)
├── compare_raid3_with_single_common.sh    # Shared helper functions
├── compare_raid3_with_single.sh          # Comparison tests
├── compare_raid3_with_single_rebuild.sh   # Rebuild tests
├── compare_raid3_with_single_heal.sh     # Heal tests
└── compare_raid3_with_single_errors.sh    # Error handling tests
```

## 🚀 Example Workflow

```bash
# 1. Initial setup (one-time)
cd /path/to/rclone/backend/raid3/integration
./setup.sh

# 2. Navigate to work directory
cd $(cat ${HOME}/.rclone_raid3_integration_tests.workdir)

# 3. List available tests
./backend/raid3/integration/compare_raid3_with_single.sh list

# 4. Run a comparison test
./backend/raid3/integration/compare_raid3_with_single.sh --storage-type local test mkdir

# 5. Run rebuild tests
./backend/raid3/integration/compare_raid3_with_single_rebuild.sh --storage-type local test even

# 6. Run heal tests
./backend/raid3/integration/compare_raid3_with_single_heal.sh --storage-type minio test odd -v

# 7. Clean up (optional)
./backend/raid3/integration/compare_raid3_with_single.sh --storage-type local teardown
```

## 💡 Tips

- **Verbose output**: Use `-v` flag to see detailed rclone command output
- **MinIO containers**: The scripts automatically start/stop MinIO containers when using `--storage-type=minio`
- **Test isolation**: Each test cleans up after itself, but you can use `teardown` to purge all data
- **Custom configuration**: Create `compare_raid3_env.local.sh` to override defaults without modifying tracked files
- **Quick help**: Run any script with `-h` or `--help` for command-specific help
