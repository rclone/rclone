# Integration Test System

This directory contains the integration test harness for rclone that runs tests across all supported backends.

## Overview

The test system allows running the complete rclone test suite against any configured remote backend. This ensures that rclone works correctly with real cloud storage providers, not just mock implementations.

## Configuration

### config.yaml

The `config.yaml` file defines which backends to test and their specific configurations. Each backend entry includes:

- `backend`: The backend name (e.g., "s3", "drive", "dropbox")
- `remote`: The test remote name from your rclone config (e.g., "TestS3:", "TestDrive:")
- `fastlist`: Whether to test with the -fast-list flag
- `ignore`: List of tests to skip for this backend
- `maxfile`: Maximum file size for testing
- Other backend-specific options

### Automatic Backend Discovery (TestRemote Field)

Starting from commits b96eb52 and ef0f1ed, backends can now self-register their test configuration using the `TestRemote` field in `fs.RegInfo`. This eliminates the need to manually maintain `config.yaml` for simple backend configurations.

#### How It Works

1. **Backend Declaration**: Backends declare their test remote in their `init()` function:
   ```go
   fs.Register(&fs.RegInfo{
       Name:        "s3",
       Description: "Amazon S3",
       TestRemote:  "TestS3:",  // <-- Self-declared test remote
       NewFs:       NewFs,
       // ...
   })
   ```

2. **Automatic Discovery**: The `AddBackendsFromRegistry()` function scans all registered backends and automatically includes those with a `TestRemote` configured.

3. **Fallback**: If a remote is specified with `-remotes` but not found in `config.yaml`, the system automatically uses the backend's `TestRemote` value.

#### Benefits

- **Reduced Maintenance**: No need to update `config.yaml` for simple backend additions
- **Self-Documenting**: Test configuration lives alongside backend code
- **Consistency**: Ensures backend developers specify their test remote
- **Backward Compatible**: `config.yaml` still works and takes precedence

#### Coverage

As of commit ef0f1ed, **55 backends** have `TestRemote` configured, covering all actively tested backends. Backends without `TestRemote` are typically:
- Wrapper backends (alias, hasher) that don't have standalone remotes
- Backends not currently tested (commented out in config.yaml)
- Deprecated or rate-limited backends

## Running Tests

### Run all configured backends
```bash
go run ./fstest/test_all
```

### Run specific backends
```bash
go run ./fstest/test_all -remotes TestS3:,TestDrive:
```

### Run specific tests
```bash
go run ./fstest/test_all -tests fs/operations,fs/sync
```

### Run with backend filter
```bash
go run ./fstest/test_all -backends s3,drive
```

### Clean up test artifacts
```bash
go run ./fstest/test_all -clean
```

## Test Configuration Options

- `-maxtries`: Number of times to retry failed tests (default: 5)
- `-n`: Maximum number of tests to run concurrently (default: 20)
- `-timeout`: Maximum time per test (default: 60m)
- `-race`: Run tests with race detector
- `-verbose`: Enable verbose logging
- `-dry-run`: Print commands without executing

## Backend Requirements

For a backend to be included in integration testing:

1. **Required**: Have a configured remote in your rclone config matching the test remote name
2. **Recommended**: Set `TestRemote` field in the backend's `RegInfo` struct
3. **Optional**: Add entry to `config.yaml` for advanced configuration (ignore lists, special flags, etc.)

## Adding a New Backend to Tests

### Simple Approach (Recommended)

Add the `TestRemote` field to your backend's registration:

```go
func init() {
    fs.Register(&fs.RegInfo{
        Name:        "mybackend",
        Description: "My Storage Backend",
        TestRemote:  "TestMyBackend:",  // Add this line
        NewFs:       NewFs,
        // ...
    })
}
```

The backend will be automatically discovered and tested.

### Advanced Approach

If your backend needs special test configuration (ignore certain tests, custom flags, etc.), add an entry to `config.yaml`:

```yaml
backends:
 - backend: "mybackend"
   remote: "TestMyBackend:"
   fastlist: true
   ignore:
     - TestSomeFailingTest
   maxfile: 1k
```

## Implementation Details

The test system is implemented across several files:

- **test_all.go**: Main test runner and command-line interface
- **config.yaml**: Backend configurations and test specifications
- **fstest/runs/config.go**: Configuration parsing and backend filtering
  - `FilterBackendsByRemotes()`: Filter by remote names
  - `FilterBackendsByBackends()`: Filter by backend names
  - `AddBackendsFromRegistry()`: Auto-discover backends with TestRemote
- **fs/registry.go**: Backend registration system with TestRemote field

## History

- Initial implementation: Test system with config.yaml
- Commit b96eb52: Added TestRemote infrastructure to RegInfo
- Commit 3fda281: Added TestRemote to 6 popular backends
- Commit ef0f1ed: Comprehensively added TestRemote to all 46 tested backends
- Total: 55 backends now support automated test discovery

## See Also

- [Backend implementation guide](../../docs/content/contributing.md)
- [Testing guide](../../CONTRIBUTING.md)
- [Integration test site](https://pub.rclone.org/integration-tests/)
