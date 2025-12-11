# Timeout Mode

The raid3 backend supports configurable timeout behavior through the `timeout_mode` option, allowing users to optimize for different storage backends and use cases.

## Configuration Options

The `timeout_mode` option has three modes:

### Standard (Default)

**Best for**: Local filesystem storage

- Uses global rclone timeout settings
- Default retries: 10
- Connection timeout: 60 seconds
- Data timeout: 5 minutes
- **No changes to default behavior**

### Balanced

**Best for**: Reliable S3/MinIO backends

- Retries: 3
- Connection timeout: 15 seconds
- Data timeout: 30 seconds
- Degraded failover: ~30-60 seconds
- **Good balance of reliability and speed**

### Aggressive

**Best for**: Testing, degraded mode scenarios

- Retries: 1
- Connection timeout: 5 seconds
- Data timeout: 10 seconds
- Degraded failover: ~10-20 seconds
- **Fastest failover, best for degraded mode**

## Timeout Comparison

| Mode | Retries | ConnTimeout | Timeout | Degraded Failover | Use Case |
|------|---------|-------------|---------|-------------------|----------|
| **standard** | 10 (global) | 60s | 5m | 2-5 minutes | Local/file storage |
| **balanced** | 3 | 15s | 30s | ~30-60 seconds | Reliable S3 |
| **aggressive** | 1 | 5s | 10s | ~10-20 seconds | Degraded mode testing |

## Configuration Examples

### Local Storage (Standard)

```ini
[mylevel3local]
type = raid3
even = /mnt/disk1/even
odd = /mnt/disk2/odd
parity = /mnt/disk3/parity
timeout_mode = standard
```

### Reliable S3 (Balanced)

```ini
[mylevel3s3]
type = raid3
even = s3even:bucket
odd = s3odd:bucket
parity = s3parity:bucket
timeout_mode = balanced
```

### Testing/Degraded Mode (Aggressive)

```ini
[mylevel3test]
type = raid3
even = minioeven:
odd = minioodd:
parity = minioparity:
timeout_mode = aggressive
```

## Command Line

```bash
# Create with aggressive mode
rclone config create mylevel3 raid3 \
  even=/path/even \
  odd=/path/odd \
  parity=/path/parity \
  timeout_mode=aggressive
```

## Logging

The backend logs the selected mode:

- **Standard**: `DEBUG : raid3: Using standard timeout mode (global settings)`
- **Balanced**: `NOTICE: raid3: Using balanced timeout mode (retries=3, contimeout=15s, timeout=30s)`
- **Aggressive**: `NOTICE: raid3: Using aggressive timeout mode (retries=1, contimeout=5s, timeout=10s)`

## Benefits

- **User Control**: Choose speed/reliability trade-off
- **Safe Default**: Standard mode = no surprises
- **Clear Choices**: Mode names explain purpose
- **No Breaking Changes**: Default behavior unchanged
- **S3 Optimized**: Balanced/aggressive modes improve S3 degraded mode performance

## Performance Impact

**Standard Mode**: No change from default rclone behavior

**Balanced Mode**: 
- Faster failover in degraded mode (~30-60s vs 2-5 minutes)
- Still reliable for normal operations

**Aggressive Mode**:
- Fastest failover (~10-20s vs 2-5 minutes)
- Best for testing and degraded mode scenarios
- May be too aggressive for production use

## Related Documentation

For detailed implementation notes, see `_analysis/current/TIMEOUT_MODE_IMPLEMENTATION.md`.

For S3 timeout research, see `_analysis/research/S3_TIMEOUT_RESEARCH.md`.
