# RS backend — bash integration tests

Black-box tests for the Reed-Solomon (`rs`) virtual backend, structured like `backend/raid3/test/`.

## Quick start

```bash
cd backend/rs/test
./setup.sh
./compare.sh --storage-type=local test smoke
```

## Full local gate (recommended before pushing)

From the **repository root**, build the binaries that include the `rs` backend and `rsverify`, then run the full shell suite:

```bash
cd /path/to/rclone
go build -o rclone .
go build -o rsverify ./cmd/rsverify

cd backend/rs/test
./setup.sh
./compare_all.sh
```

`compare_all.sh` matches the **raid3** orchestrator pattern: **by default** it runs that sequence for **`local`**, then again for **`MinIO`** (Docker). Use **`--storage-type=local`** or **`--storage-type=minio`** to run only one backend.

For each `--storage-type`, in order:

1. **`test verify`** — `smoke` (rcat, cat, shard checks) plus **`rsverify check`** on all particles.
2. **`test heal`** — `smoke`, delete the **last-shard** particle, **`rclone cat`** (degraded read), **`rclone backend heal`** (single-object), then **`rsverify check`**.

Optional pause between steps: `COMPARE_ALL_SLEEP_BETWEEN_TESTS=1` (default; set to `0` to disable).

### Individual tests

| Command | What it checks |
|---------|----------------|
| `./compare.sh --storage-type=local test smoke` | Basic upload/read and shard files on disk. |
| `./compare.sh --storage-type=minio test smoke` | Same checks via S3 (Docker MinIO; **`lsl`** per shard). |
| `./compare.sh --storage-type=local test verify` | Smoke + **`rsverify check`** (needs `rsverify` binary). |
| `./compare.sh --storage-type=minio test verify` | Smoke + download particles to temp + **`rsverify check`**. |
| `./compare.sh --storage-type=local test heal` | Smoke + drop last shard + heal (single-object) + **`rsverify`**. |
| `./compare.sh --storage-type=minio test heal` | Same via **`rclone deletefile`** on last shard + heal (single-object) + **`rsverify`**. |
| `./compare_heal.sh -v --storage-type=local` | Same as **`test heal`** (wrapper for parity with raid3 naming). |

## Configuration

- **setup.sh** creates `_data/01_local` … `_data/07_local`, `_data/single_local`, the MinIO bind-mount dirs **`_data/01_minio` … `_data/07_minio`, `_data/single_minio`**, and **`tests.config`** with **both** local and MinIO remotes (k=4, m=3 by default).
- **compare.sh** supports **`--storage-type=local`** (filesystem shards) and **`--storage-type=minio`** (one MinIO container per shard; same tests as local).

### MinIO (Docker)

Ports default to **9201–9208** (seven shards + single) so they do not overlap raid3’s MinIO defaults (9001–9004). Override with **`MINIO_RS_FIRST_S3_PORT`** / **`MINIO_RS_FIRST_CONSOLE_PORT`** in `compare_rs_env.local.sh` if needed.

```bash
cd backend/rs/test
./setup.sh
./manage.sh start --storage-type=minio
RCLONE_BINARY=/path/to/rclone RSVERIFY_BINARY=/path/to/rsverify ./compare_all.sh --storage-type=minio
./manage.sh stop --storage-type=minio   # optional
```

`compare.sh` starts containers automatically if they are missing; **`manage.sh`** is for explicit start/stop/recreate.

Optional overrides: create `compare_rs_env.local.sh` to adjust paths, `RS_DATA_SHARDS` / `RS_PARITY_SHARDS` (regenerate **`tests.config`** after changes), or **`MINIO_IMAGE`** (same idea as `backend/raid3/test`). If you still have an old **`tests.minio.config`**, you can remove it; MinIO remotes now live in **`tests.config`**.

## Environment variables

| Variable | Purpose |
|----------|---------|
| **`RCLONE_BINARY`** | Path to `rclone` if not using the repo root `./rclone` or `PATH`. |
| **`RSVERIFY_BINARY`** | Path to `rsverify` if not using repo root `./rsverify` or `PATH`. |
| **`COMPARE_ALL_SLEEP_BETWEEN_TESTS`** | Seconds to sleep between `compare_all` suites (default `1`). |
| **`MINIO_IMAGE`** | MinIO Docker image (default pinned release; see `compare_rs_env.sh`). |
| **`RS_MINIO_BUCKET`** | S3 bucket name used on every shard (default `rsint`). |
| **`MINIO_RS_FIRST_S3_PORT`** | First host port for shard 1 API (default `9201`). |
