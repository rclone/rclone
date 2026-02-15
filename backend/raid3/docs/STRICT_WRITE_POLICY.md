# Strict Write Policy

The raid3 backend enforces a strict write policy that blocks all write operations when any backend is unavailable, matching hardware RAID 3 behavior.

## Policy

Read operations work in degraded mode with 2 of 3 backends available (reconstruction). All write operations (Put, Update, Move/Rename) and delete operations (Remove, Rmdir, Purge, CleanUp) are blocked in degraded mode and require all 3 backends available.

## Why Strict Writes?

Strict writes ensure data consistency by preventing creation of files with missing particles, avoid corruption by preventing partial updates, match hardware RAID 3 controller behavior for compliance, and maintain structural integrity by ensuring directories and metadata stay synchronized across all backends.

## How It Works

Before every write operation, the backend performs a pre-flight health check that tests all 3 backends in parallel: `List()` operation to check connectivity, `Mkdir()` on a test directory to verify write capability, and `Rmdir()` to clean up the test directory. The check uses a 5-second timeout per backend, returns an error immediately if any backend is unavailable or read-only, and provides actionable error messages with rebuild guidance.

## Safety Features

After Update operations, the backend validates particle sizes to detect corruption. If particle sizes are invalid, it returns an error indicating the file may be corrupted.

## Related Documentation

For error handling policy, see [ERROR_HANDLING.md](ERROR_HANDLING.md).

