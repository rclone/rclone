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

- **Automatic**: No user configuration needed - always enabled
- **Transparent**: Works automatically during normal read operations
- **Non-Blocking**: Reads return immediately, upload happens in background
- **Deduplication**: Multiple reads of the same file don't create duplicate uploads
- **Graceful Shutdown**: Waits for pending uploads to complete before exit

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

Heal is always enabled - no configuration needed.

The backend uses 2 concurrent background workers by default (hardcoded, not configurable).

## Related Documentation

For detailed implementation notes, see `_analysis/current/SELF_HEALING_IMPLEMENTATION.md`.
