# PR Notes: RAID3 Backend Contribution

## Summary

This PR adds a new disaster‑tolerant virtual backend `raid3` that implements RAID-3-style byte-level striping with XOR parity across three underlying remotes (two data, one parity). The backend can read with one remote missing and writes each object as three correlated objects on the underlying remotes.

## Behavior

- Byte-level striping with XOR parity across three remotes (two data stripes + one parity stripe).
- Degraded reads supported when any single remote is unavailable; writes require all three remotes.
- Automatic reconstruction of missing stripes on read; optional rebuild/repair operations for restoring redundancy.
- Streaming I/O for large objects; objects are not fully buffered in memory.
- Periodic cleanup to remove orphaned or incomplete stripes created by failed operations.
- Backend commands: `status`, `rebuild`, `heal` (see `backend/raid3/commands.go`).

## Code quality

- `golangci-lint run ./backend/raid3/...` passes with no issues.
- Code structure follows the pattern of existing virtual backends (e.g. `union`, `chunker`) where applicable.
- Backend Go tests: `go test ./backend/raid3`; bash integration tests in `backend/raid3/test/` run separately (see test/README.md).

## Documentation

- User-facing backend docs: `backend/raid3/docs/README.md`.
- Detailed design: `backend/raid3/docs/RAID3.md`.
- Open design questions and edge cases: `backend/raid3/docs/OPEN_QUESTIONS.md`.
- `docs/content/_index.md` and top-level `README.md` updated to mention `raid3`.

## Testing

- Standard `fstest` suite runs for `raid3`; all tests pass.
- Backend-specific integration tests in `backend/raid3/test` (compare against a single remote, failure scenarios) pass.
- `golangci-lint` run as part of the CI job for `backend/raid3` passes.

