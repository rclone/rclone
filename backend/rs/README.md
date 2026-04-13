# RS Backend (Work In Progress)

This backend implements a virtual Reed-Solomon layout over multiple remotes.

## Current Status

- Implemented:
  - Config parsing and validation (`k`, `m`, ordered shard remotes)
  - Enforced topology rule: `k > m` (`data_shards > parity_shards`)
  - Footer v3 (`RCLONE/EC`, 102-byte) with `Algorithm=RS`, `StripeSize`, `NumStripes`, `PayloadCRC32C`
  - Upload path: `use_spooling=false` streams shards by default; unknown-size sources use spooling for that transfer
  - Default `use_spooling=false`: direct shard streaming for known-size sources; unknown-size `Put` auto-spools for that transfer only
  - Quorum policy for writes/metadata/namespace ops (`write_quorum`, default `k+1`)
  - Two-phase operation retries (full pass + one fast retry for failing shards)
  - Read/reconstruct path from available shards
  - `status`, `heal`, and `degraded` backend command plumbing
  - Same-layout server-side **`Copy`**, **`Move`**, and **`DirMove`** on the logical remote ([`move_copy.go`](move_copy.go)): per-shard delegated `Features().Copy` / `Move` / `DirMove` under write quorum (see [`docs/OPEN_QUESTIONS.md`](docs/OPEN_QUESTIONS.md) Q2)
- Not yet complete:
  - Full production-ready heal orchestration and integration coverage

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

## rsverify (developer tool)

The standalone helper `rsverify` lives under `cmd/rsverify`. Build with:

`go build -o rsverify ./cmd/rsverify`

Subcommands: `encode`, `decode`, `check`, `footer` (see `rsverify --help`). Defaults match rclone particle layout (EC footer v3 unless `encode --footer=false`). Use `encode --stripe-size` / rs `stripe_fragment_size` to control the RS fragment size **S**.
