# Email Draft: RAID Backend Summary for Nick Craig-Wood

**Subject**: RAID Backend Progress Update - RAID 3 Implementation for rclone

---

Hi Nick,

Following up on our earlier conversation, I want to provide a technical summary of the RAID-backend implementation. For now, we're using the temporary name `level3` in the codebase, but we're considering renaming it (more on that below).

I should also mention that this project has been developed with the help of AI assistance, and this email was also prepared with AI support, as I am not a native speaker.

## How the RAID Backend Works

The RAID-backend implements **RAID 3 storage** with byte-level data striping across three remotes. Data is split at the byte level, which provides the finest granularity possible. Parity is calculated as `parity[i] = even[i] XOR odd[i]`, enabling reconstruction of any missing particle from the other two. Storage overhead is 150% (50% parity), which is better than full duplication at 200%. The processing could be applied hierarchically if required, leading to many smaller particles should this be preferred. It's very fast, flexible, and almost stateless.

**Handling Incomplete Data**

The backend handles incomplete data according to RAID 3 semantics. When reading data, if only 2 of 3 particles are available, the missing particle is automatically reconstructed from the available data and parity. This degraded mode read works transparently - users get their data immediately while the backend reconstructs the missing piece in the background.

However, writing is not allowed when only 2 of 3 backends are available. This strict policy prevents corruption from partial writes and matches hardware RAID 3 controller behavior. The backend performs a pre-flight health check before each write operation and fails immediately if any backend is unavailable, providing error messages about the degraded state.

Delete operations use a best-effort approach - they succeed if any backends are reachable and ignore "not found" errors.

The backend hides orphaned objects (those with only 1 particle) from listings, though they are not physically deleted until explicitly cleaned up. For reconstructing missing particles, the backend provides two explicit commands: `backend heal` scans and heals all degraded objects across the entire remote, while `backend rebuild` reconstructs all missing particles on a replacement backend after a failed remote has been replaced.

## Why RAID 3

RAID 3 is deprecated for hard drives due to the parity disk bottleneck caused by mechanical seek times. However, for cloud storage, this bottleneck doesn't apply, to my understanding. Cloud storage uses pure network I/O with no seek time or rotational latency, eliminating the mechanical head repositioning that makes RAID 3 problematic for hard drives. All three remotes are written to concurrently in parallel transfers, and byte-level granularity provides finer reconstruction granularity than RAID 5's block-level striping. Additionally, RAID 3 offers the least complexity.

We've documented this analysis in `backend/level3/docs/RAID3_VS_RAID5_ANALYSIS.md`. This finding emerged from our work on Particle Cloud Security technology.

## How We Test

Our testing approach uses both Go-based unit and integration tests, as well as Bash-based comparison harnesses for black-box validation.

The Go-based testing includes a `fstests.Run()` integration test suite covering standard rclone tests, unit tests for byte splitting, merging, parity calculation, and validation, plus dedicated tests for the rebuild and heal commands (`level3_rebuild_test.go` and `level3_heal_command_test.go`).

The Bash-based comparison harnesses provide black-box testing by comparing the RAID-backend behavior against single backends.

We test the RAID-backend in two configurations: first, backed by three local filesystem remotes, and second, backed by three MinIO object storage instances running as Docker containers.

The main script `compare_level3_with_single.sh` compares the RAID-backend against single backends for major rclone commands including `mkdir`, `ls`, `cat`, `copy`, `move`, `check`, and `purge`, asserting identical exit codes and comparing outputs. Additional scripts test `backend rebuild` with simulated disk swaps (`compare_level3_with_single_recover.sh`) and degraded reads with the explicit `backend heal` command (`compare_level3_with_single_heal.sh`). These scripts use centralized configuration.

## What's Implemented

The core functionality includes byte-level striping and XOR parity calculation, parallel uploads to all three backends, degraded mode reads that work with 2 of 3 backends, and automatic reconstruction from parity. The backend provides three explicit commands: `backend status` for health diagnostics, `backend rebuild` for reconstructing all missing particles on a replacement backend after a failed remote has been replaced, and `backend heal` for scanning and healing all degraded objects across the entire remote. Auto-cleanup hides orphaned objects from listings, though they are not physically deleted until explicitly cleaned up. Timeout modes for S3/MinIO backends are configurable (standard, balanced, aggressive) to optimize for different network conditions. Detailed design decisions and implementation notes are documented in `backend/level3/DESIGN_DECISIONS.md` and `backend/level3/docs/IMPLEMENTATION_COMPLETE.md`.

## What's Missing

Streaming support has not been started. The current implementation loads entire files into memory using `io.ReadAll()`. Streaming with chunked striping is planned but not yet implemented and is documented in the README.

Native implementations for `Purge` and `ListR` are deferred. The backend currently falls back to `List + Delete + Rmdir` for purging. Both are deferred as future optimizations.

Compression is under consideration but not implemented. We are considering Snappy compression as used by Kubernetes due to its speed. Analysis is documented in `docs/COMPRESSION_ANALYSIS.md`.

The question of mixed file/object storage remains uncertain. The current implementation works with both file-based (local) and object-based (S3) remotes individually, but mixing different storage types has not been tested at all. This needs discussion on whether it should be explicitly supported or discouraged.

## Potential Rename

We're considering renaming `level3` to `raid` or `raid3` for clarity after discussing the subject.

## Major Open Issues for Discussion

Several questions need discussion. Testing has been limited to artificial settings with controlled datasets; we haven't tested metadata handling, large-scale operations, or production workloads. Key questions include: auto-heal behavior (currently `auto_heal` defaults to `false`, requiring explicit `backend heal` command due to inconsistent behavior across backend types - is explicit healing the right approach?), whether we should explicitly support mixing local filesystem and object storage remotes or recommend homogeneous backends, naming of the project, using compression, chunking, and using custom tags in object storage to store the hash of the original data.


I'd very much appreciate your thoughts on these topics.

Would it be possible for an rclone community member to review the current status before we attempt an official contribution? I have a GitHub repository where the code is available. Alternatively, should we conduct more extensive testing with real-world backends first?

Thanks for your encouragement on this project. Happy to discuss any of these points in more detail.

Best regards,
Harald

PS. This is the first time I've been coding with the support of AI. I am willing to dive deep into rclone when contributing this new backend and eventually the PCS technology. For now it seems to be the right choice to show what we want to offer to the project.

