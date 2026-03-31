#!/usr/bin/env bash
#
# compare_rs_env.sh
# -------------------
# Default environment for rs (Reed-Solomon) bash integration tests.
# Override values in compare_rs_env.local.sh (untracked) if needed.
#

# Script directory (set by scripts that source this file)
SCRIPT_DIR="${SCRIPT_DIR:-$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
DATA_DIR="${DATA_DIR:-${SCRIPT_DIR}/_data}"

# Fixed layout for shell suite v1: k=4, m=3 → seven local shard roots under _data/
RS_DATA_SHARDS="${RS_DATA_SHARDS:-4}"
RS_PARITY_SHARDS="${RS_PARITY_SHARDS:-3}"

# Virtual RS remote and reference single-backend remote (alias → plain tree)
RS_REMOTE="${RS_REMOTE:-localrs}"
RS_SINGLE_REMOTE="${RS_SINGLE_REMOTE:-rslocalsingle}"

# MinIO/S3 layout (used when --storage-type=minio). Ports 9201+ avoid raid3 defaults (9001–9004).
RS_REMOTE_MINIO="${RS_REMOTE_MINIO:-minirs}"
RS_SINGLE_REMOTE_MINIO="${RS_SINGLE_REMOTE_MINIO:-rsminiosingle}"
RS_MINIO_BUCKET="${RS_MINIO_BUCKET:-rsint}"

MINIO_RS_USER="${MINIO_RS_USER:-rsminio}"
MINIO_RS_PASS="${MINIO_RS_PASS:-rsminio88}"
# Host S3 API ports: shard i uses MINIO_RS_FIRST_S3_PORT + i - 1; single uses FIRST + RS_SHARD_TOTAL.
MINIO_RS_FIRST_S3_PORT="${MINIO_RS_FIRST_S3_PORT:-9201}"
MINIO_RS_FIRST_CONSOLE_PORT="${MINIO_RS_FIRST_CONSOLE_PORT:-19301}"
MINIO_RS_CONTAINER_PREFIX="${MINIO_RS_CONTAINER_PREFIX:-rsminio}"
MINIO_RS_SINGLE_CONTAINER_NAME="${MINIO_RS_SINGLE_CONTAINER_NAME:-rsminiosingle}"
MINIO_RS_SHARD_REMOTE_PREFIX="${MINIO_RS_SHARD_REMOTE_PREFIX:-minirs}"
MINIO_RS_SINGLE_REMOTE_NAME="${MINIO_RS_SINGLE_REMOTE_NAME:-minirssingle}"

MINIO_IMAGE="${MINIO_IMAGE:-minio/minio:RELEASE.2025-09-07T16-13-09Z}"

# Per-shard local remote names (localrs01 …) and directory names (01_local …) are
# derived in setup.sh / compare_common.sh from RS_SHARD_TOTAL.
