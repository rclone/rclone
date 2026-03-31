#!/usr/bin/env bash
#
# manage.sh — start/stop MinIO containers for rs shell tests (see backend/raid3/test/manage.sh).
#
# Usage:
#   ./manage.sh --storage-type=minio start|stop|recreate
#
# Must be run from: backend/rs/test
#

set -euo pipefail

SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)

# shellcheck source=compare_common.sh
. "${SCRIPT_DIR}/compare_common.sh"

STORAGE_TYPE=""
COMMAND=""

usage() {
  cat <<EOF
Usage: ${SCRIPT_NAME} --storage-type=minio <command>

Commands:
  start      Start MinIO containers (ports ${MINIO_RS_FIRST_S3_PORT}-$((MINIO_RS_FIRST_S3_PORT + RS_SHARD_TOTAL))).
  stop       Stop MinIO containers.
  recreate   Stop, remove, and recreate containers (fixes broken state).

Options:
  --storage-type minio   Required (only minio is supported for this script).

The script must be run from ${SCRIPT_DIR}
EOF
}

parse_args() {
  if [[ $# -eq 0 ]]; then
    usage
    exit 0
  fi
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --storage-type)
        shift
        [[ $# -gt 0 ]] || die "--storage-type requires a value"
        STORAGE_TYPE="$1"
        ;;
      --storage-type=*)
        STORAGE_TYPE="${1#*=}"
        ;;
      -h | --help)
        usage
        exit 0
        ;;
      start | stop | recreate)
        [[ -z "${COMMAND}" ]] || die "Multiple commands"
        COMMAND="$1"
        ;;
      *)
        die "Unknown argument: $1"
        ;;
    esac
    shift
  done
  [[ -n "${COMMAND}" ]] || die "No command (start|stop|recreate)"
  [[ "${STORAGE_TYPE}" == "minio" ]] || die "Only --storage-type=minio is supported"
}

main() {
  parse_args "$@"
  ensure_workdir
  check_docker_available

  case "${COMMAND}" in
    start)
      start_minio_rs_containers
      ;;
    stop)
      stop_minio_rs_containers
      ;;
    recreate)
      stop_minio_rs_containers
      remove_minio_rs_containers
      start_minio_rs_containers
      ;;
  esac
}

main "$@"
