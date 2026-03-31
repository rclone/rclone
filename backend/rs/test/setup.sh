#!/usr/bin/env bash
#
# setup.sh — create _data layout and tests.config for rs shell integration tests.
#
# Usage: ./setup.sh
# Run from: backend/rs/test
#

set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)

# shellcheck source=compare_common.sh
. "${SCRIPT_DIR}/compare_common.sh"

log_info "setup" "Data directory: ${DATA_DIR}"
mkdir -p "${DATA_DIR}" || die "Failed to create ${DATA_DIR}"

log_info "setup" "Creating local shard directories (01_local …) and single_local"
for i in $(seq 1 "${RS_SHARD_TOTAL}"); do
  mkdir -p "${DATA_DIR}/$(printf '%02d' "${i}")_local"
done
mkdir -p "${DATA_DIR}/single_local"

log_info "setup" "Creating MinIO host data dirs (01_minio …) and single_minio (Docker bind mounts)"
for i in $(seq 1 "${RS_SHARD_TOTAL}"); do
  mkdir -p "${DATA_DIR}/$(printf '%02d' "${i}")_minio"
done
mkdir -p "${DATA_DIR}/single_minio"

CONFIG_FILE="${SCRIPT_DIR}/tests.config"
if create_rs_rclone_config "${CONFIG_FILE}" 0; then
  log_pass "setup" "Config ready: ${CONFIG_FILE}"
else
  if [[ ! -f "${CONFIG_FILE}" ]]; then
    die "Failed to create ${CONFIG_FILE}"
  fi
  log_info "setup" "Config unchanged (already present): ${CONFIG_FILE}"
fi

log_pass "setup" "Done. Example: ./compare.sh --storage-type=local test smoke"
log_info "setup" "MinIO example: ./manage.sh start --storage-type=minio && ./compare.sh --storage-type=minio test smoke"
