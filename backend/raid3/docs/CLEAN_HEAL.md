# Self-Maintenance

The raid3 backend automatically maintains system health through two complementary mechanisms: healing reconstructs and restores missing particles for degraded files (2/3 → 3/3 particles), while cleanup removes orphaned files that cannot be reconstructed (1/3 particles). Both features work together to prevent accumulation of broken state and maintain data integrity. This maintenance is automatically triggered by regular usage of the backend (healing during read operations, cleanup during list operations). All maintenance operations work on whichever remote is affected (even, odd, or parity). Terminology: degraded (2/3) means missing 1 particle and can be reconstructed, while orphaned (1/3) means missing 2 particles and cannot be reconstructed.

---

## Healing (2/3 → 3/3)

Healing reconstructs missing particles for degraded files that can still be recovered. When reading a file in degraded mode (one particle missing), the backend detects which particle is missing (even, odd, or parity), reads the two available particles, and reconstructs the complete file data using XOR parity. The missing particle data is extracted from the reconstructed file and queued for background upload (non-blocking operation). The read operation returns the reconstructed file data immediately without waiting for the upload to complete. Background worker threads process the queued uploads asynchronously, restoring the missing particle to its backend. When the rclone process exits, it waits for pending uploads to complete (up to 60 seconds timeout) to ensure data integrity before shutdown. For directories, when accessing a directory that exists on 2/3 backends, the backend detects this during `List()` operations. If `auto_heal=true` (default), the backend automatically creates the missing directory on the third backend, making it immediately available on all 3 backends (2/3 → 3/3). This happens automatically during normal directory access with no manual intervention needed. Note: Automatic healing applies to the specific file or directory being accessed during normal operations; it does not scan the entire remote for degraded objects.

### Explicit Heal Command

In addition to automatic healing during reads, you can proactively heal all degraded objects using `rclone backend heal raid3:`. This command scans all objects in the remote, identifies objects with exactly 2 of 3 particles, reconstructs and uploads missing particles. It's useful for periodic maintenance. The `heal` command works regardless of the `auto_heal` setting and is always available as an explicit admin command. It also triggers cleanup as a side effect when `auto_cleanup=true`, since it uses `List()` operations which automatically clean up orphaned files (1/3 particles) during the scan.

---

## Cleanup (1/3 → 0/3)

Cleanup removes orphaned files that cannot be reconstructed. These files may be created during failed rollback operations (especially Update rollback issues), partial write operations that fail, or manual file creation outside raid3. When `auto_cleanup=false`, orphaned files are visible in listings even though they cannot be downloaded. You can manually clean up orphaned files using `rclone cleanup raid3:`, which recursively scans from the specified path (or root if no path specified). The `cleanup` command requires all 3 remotes to be available and is useful for periodic maintenance.


