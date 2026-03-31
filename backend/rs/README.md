# RS Backend (Work In Progress)

This backend implements a virtual Reed-Solomon layout over multiple remotes.

## Current Status

- Implemented:
  - Config parsing and validation (`k`, `m`, ordered shard remotes)
  - Footer v2 (`RCLONE/EC`) with `Algorithm=RS`, `StripeSize`, `PayloadCRC32C`
  - Upload path with `use_spooling=true`
  - Write quorum policy: `k+1`
  - Read/reconstruct path from available shards
  - Basic `status` and `heal` command plumbing
- Not yet complete:
  - `use_spooling=false` streaming write path
  - Full production-ready heal orchestration and integration coverage

Open design questions and follow-ups are tracked in [`docs/OPEN_QUESTIONS.md`](docs/OPEN_QUESTIONS.md).

## fstest / CI (`TestRsLocal`)

The integration suite uses `fstest/testserver/init.d/TestRsLocal` (four local shard directories, `k=2`, `m=2`).

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
use_spooling = true
rollback = true
max_parallel_uploads = 4
```

Notes:
- In v1, `len(remotes)` must equal `data_shards + parity_shards`.
- Write commit requires at least `k+1` successful shard uploads.

## rsverify (developer tool)

The standalone helper `rsverify` lives under `cmd/rsverify`. Build with:

`go build -o rsverify ./cmd/rsverify`

Subcommands: `encode`, `decode`, `check`, `footer` (see `rsverify --help`). Defaults match rclone particle layout (EC v2 footer unless `encode --footer=false`).
