# Timeout Mode

The raid3 backend supports configurable timeout behavior through the `timeout_mode` option, allowing users to optimize for different storage backends and use cases.

## Configuration Options

The `timeout_mode` option has three modes. Standard (default) is best for local filesystem storage, uses global rclone timeout settings, and makes no changes to default behavior (safe default, no breaking changes). Balanced is best for reliable S3/MinIO backends, provides faster failover in degraded mode (~30-60s vs 2-5 minutes), offers a good balance of reliability and speed, and is optimized for S3/MinIO backends. Aggressive is best for testing and degraded mode scenarios, provides fastest failover (~10-20s vs 2-5 minutes), but may be too aggressive for production use.

Timeout values: standard uses 10 retries (global), 60s ConnTimeout, 5m Timeout, 2-5 minutes degraded failover, for local/file storage; balanced uses 3 retries, 15s ConnTimeout, 30s Timeout, ~30-60 seconds degraded failover, for reliable S3; aggressive uses 1 retry, 5s ConnTimeout, 10s Timeout, ~10-20 seconds degraded failover, for degraded mode testing.

## Configuration Examples

The `timeout_mode` option applies to all three remotes (even, odd, parity) uniformly.

```ini
[mylevel3]
type = raid3
even = s3even:bucket
odd = s3odd:bucket          # Same timeout_mode applies
parity = s3parity:bucket    # Same timeout_mode applies
timeout_mode = balanced     # Options: standard, balanced, aggressive
```

## Command Line

```bash
# Create with timeout mode (applies to all three remotes)
rclone config create mylevel3 raid3 \
  even=/path/even \
  odd=/path/odd \
  parity=/path/parity \
  timeout_mode=aggressive
```

## Logging

The backend logs the selected mode: Standard logs `DEBUG : raid3: Using standard timeout mode (global settings)`, Balanced logs `NOTICE: raid3: Using balanced timeout mode (retries=3, contimeout=15s, timeout=30s)`, and Aggressive logs `NOTICE: raid3: Using aggressive timeout mode (retries=1, contimeout=5s, timeout=10s)`.

## Related Documentation

For S3 timeout research, see `_analysis/research/S3_TIMEOUT_RESEARCH.md`.
