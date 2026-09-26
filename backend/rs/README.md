# RS Backend (Work In Progress)

This backend implements a virtual Reed-Solomon layout over multiple remotes.

## Current Status

- Implemented:
  - Config parsing and validation (`k`, `m`, ordered shard remotes)
  - Enforced topology rule: `k > m` (`data_shards > parity_shards`)
  - Footer v1 (`RCLONERS`, 96-byte) with `Algorithm=SYMM`, `StripeSize`, `NumStripes`, `PayloadCRC32C`
  - **Virtual padding** on disk: trimmed data-shard payloads per stripe; parity shards full `S` bytes per stripe ([`payloadlayout.go`](payloadlayout.go))
  - Upload path: `use_spooling=false` streams shards by default; unknown-size sources use spooling for that transfer
  - **List metadata fast path**: size/ModTime from shard `List` when possible; coalesced footer read on lowest listing shard ([`docs/LIST_METADATA.md`](docs/LIST_METADATA.md), [`list_metadata.go`](list_metadata.go))
  - Quorum: **read/list at k**; **writes/metadata/namespace at `write_quorum`** (default `k+1`); transaction spec [`docs/QUORUM_TRANSACTIONS.md`](docs/QUORUM_TRANSACTIONS.md)
  - Two-phase operation retries (full pass + one fast retry for failing shards)
  - Read/reconstruct path from available shards
  - `status`, `heal`, `degraded`, and `verify` backend command plumbing
  - Same-layout server-side **`Copy`**, **`Move`**, and **`DirMove`** on the logical remote ([`move_copy.go`](move_copy.go)): per-shard delegated `Features().Copy` / `Move` / `DirMove` under write quorum (see [`docs/OPEN_QUESTIONS.md`](docs/OPEN_QUESTIONS.md) Q2)
  - Shell integration gate: [`test/compare_all.sh`](test/compare_all.sh) (local + MinIO: smoke, verify, heal, quorum_dirs, move_copy) — see [`test/README.md`](test/README.md)
- Not yet complete:
  - Production-scale heal orchestration (large namespaces, Q5) and broader hardening (Q15)

Open design questions and follow-ups are tracked in [`docs/OPEN_QUESTIONS.md`](docs/OPEN_QUESTIONS.md).

## fstest / CI (`TestRsLocal`)

The integration suite uses `fstest/testserver/init.d/TestRsLocal` (four local shard directories, `k=3`, `m=1`).

```bash
go test ./backend/rs/... -run '^TestStandard$' -count=1
go test ./backend/rs/... -remote TestRsLocal: -count=1
go run ./fstest/test_all -backends rs
```

## Configuration (Draft)

```ini
[myrs]
type = rs
remotes = remote1:,remote2:,remote3:,remote4:,remote5:,remote6:
data_shards = 4
parity_shards = 2
use_spooling = false
rollback = true
```

Notes:
- In v1, `len(remotes)` must equal `data_shards + parity_shards`.
- In v1, `data_shards` must be greater than `parity_shards` (`k > m`).
- Write commit requires at least `write_quorum` successful shard uploads (default `k+1`).
- Streaming `Open` uses parallel range reads per stripe; reconstruct `Open` probes shards in parallel once. **`List`**, **`NewObject`** (footer probe), **`Rmdir`** empty-dir checks, full-namespace heal listing, heal discovery / stripe reads / legacy `ReadAll` / healed shard **`Put`s** also fan out across shards in parallel where safe (same quorum and “lowest shard wins” rules as before).

## backend verify (integrity check)

Read-only integrity verification via the backend command:

`rclone backend verify rs:`

`rclone backend verify rs: path/to/file.bin -o hashes=true -o strict=true`

Default checks parse footers, validate WriteID group consistency, virtual-padding layout, and stream each shard payload against `PayloadCRC32C`. With `-o hashes=true`, stripe-wise reconstruction compares logical MD5/SHA256 to footer hashes. With `-o strict=true`, all k+m shards must be present in the winning WriteID group. Unlike `degraded` (presence-only), verify validates particle integrity. Unlike `heal`, verify makes no writes.
