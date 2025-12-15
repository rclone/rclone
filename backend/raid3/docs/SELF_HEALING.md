# Heal

The raid3 backend automatically reconstructs and restores missing particles during read operations, providing transparent heal similar to commercial RAID systems.

## How It Works

### File Heal

When reading a file in degraded mode (one particle missing):

1. **Detection**: The backend detects that one particle is missing (even, odd, or parity)
2. **Reconstruction**: Reads the two available particles and reconstructs the missing data using XOR parity
3. **Background Upload**: Queues the missing particle for automatic upload in the background
4. **Immediate Return**: Returns the reconstructed data to the user immediately (non-blocking)

### Directory Heal

When accessing a directory that exists on 2/3 backends (via `List()` operation):

1. **Detection**: The backend detects the directory exists on exactly 2 backends during `List()` operation
2. **Automatic Creation**: If `auto_heal=true` (default), automatically creates the missing directory on the third backend
3. **Transparent**: Directory is immediately available on all 3 backends (2/3 → 3/3)
4. **No User Action**: Happens automatically during normal directory access - no manual intervention needed

**Note**: Directory reconstruction complements the rebuild command:
- **Rebuild command**: Rebuilds file particles (parent directories created automatically during file uploads)
- **Auto-heal during access**: Reconstructs empty directories that exist on 2/3 backends when accessed

## Features

- **Automatic**: Enabled by default (`auto_heal=true`) - can be disabled if needed
- **Transparent**: Works automatically during normal read operations
- **Non-Blocking**: Reads return immediately, upload happens in background
- **Deduplication**: Multiple reads of the same file don't create duplicate uploads
- **Graceful Shutdown**: Waits for pending uploads to complete before exit
- **Explicit Heal Command**: `rclone backend heal raid3:` to proactively heal all degraded objects

## Performance

| Scenario | Read Time | Upload Time | Total Time |
|----------|-----------|-------------|------------|
| All particles healthy | 6-7s | 0s | **6-7s** |
| One particle missing | 6-7s | 2-3s | **9-10s** |

## Architecture

The implementation uses a background worker pattern:

- **Upload Queue**: Thread-safe queue for pending uploads
- **Background Workers**: 2 concurrent workers process upload jobs
- **Lifecycle**: Workers run for the lifetime of the Fs instance
- **Shutdown**: Waits up to 60 seconds for pending uploads to complete

## Example

```bash
$ rclone cat raid3:file.txt
2025/11/02 10:00:00 INFO  : file.txt: Reconstructed from even+parity (degraded mode)
2025/11/02 10:00:00 INFO  : raid3: Queued odd particle for heal upload: file.txt
Hello World!
2025/11/02 10:00:07 INFO  : raid3: Waiting for 1 heal upload(s) to complete...
2025/11/02 10:00:10 INFO  : raid3: Heal complete
```

## Configuration

Auto-heal is enabled by default (`auto_heal=true`) but can be disabled:

```ini
[raid3]
type = raid3
even = remote1:
odd = remote2:
parity = remote3:
auto_heal = false  # Disable automatic healing
```

When `auto_heal=false`:
- Files can still be read in degraded mode (reconstruction works)
- Missing particles are NOT automatically uploaded
- Use the explicit `heal` command to restore degraded objects

The backend uses 2 concurrent background workers by default (hardcoded, not configurable).

## Explicit Heal Command

In addition to automatic healing during reads, you can proactively heal all degraded objects:

```bash
rclone backend heal raid3:
```

This command:
- Scans all objects in the remote
- Identifies objects with exactly 2 of 3 particles (degraded state)
- Reconstructs and uploads missing particles
- Reports a summary of healed objects

**When to use**:
- Periodic maintenance
- After rebuilding from backend failures
- Before important operations
- When you want to ensure all objects are fully healthy

**Note**: The `heal` command works regardless of the `auto_heal` setting - it's always available as an explicit admin command.

## Related Documentation

For S3 timeout research related to degraded mode performance, see `_analysis/research/S3_TIMEOUT_RESEARCH.md`.
