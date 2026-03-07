#!/usr/bin/env bash
#
# manage.sh
# ---------
# Manages the test environment: start/stop containers, teardown data, recreate containers.
#
# Use this script for maintenance operations. To run comparison tests, use compare.sh.
#
# Usage:
#   manage.sh [options] <command>
#
# Commands:
#   start      Start MinIO/SFTP containers (requires --storage-type=minio, mixed, or sftp).
#   stop       Stop MinIO/SFTP containers (requires --storage-type=minio, mixed, or sftp).
#   teardown   Purge all test data for the selected storage type (raid3 + single).
#   recreate   Stop, remove, and recreate MinIO/SFTP containers (fixes broken container state).
#
# Options:
#   --storage-type <local|minio|mixed|sftp>   Required for all commands.
#   -h, --help                                Display this help.
#
# Safety guard: the script must be executed from backend/raid3/test directory.
# -----------------------------------------------------------------------------

set -euo pipefail

SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)

# shellcheck source=compare_common.sh
. "${SCRIPT_DIR}/compare_common.sh"

STORAGE_TYPE=""
COMMAND=""

usage() {
  cat <<EOF
Usage: ${SCRIPT_NAME} [options] <command>

Commands:
  start      Start MinIO/SFTP containers (requires --storage-type=minio, mixed, or sftp).
  stop       Stop MinIO/SFTP containers (requires --storage-type=minio, mixed, or sftp).
  teardown   Purge all test data for the selected storage type (raid3 + single).
  recreate   Stop, remove, and recreate MinIO/SFTP containers (fixes broken container state).

Options:
  --storage-type <local|minio|mixed|sftp>   Required for all commands.
  -h, --help                                Display this help.

The script must be executed from ${WORKDIR}.
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
        [[ $# -gt 0 ]] || die "--storage-type requires an argument"
        STORAGE_TYPE="$1"
        ;;
      --storage-type=*)
        STORAGE_TYPE="${1#*=}"
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      start|stop|teardown|recreate)
        if [[ -n "${COMMAND}" ]]; then
          die "Multiple commands provided: '${COMMAND}' and '$1'"
        fi
        COMMAND="$1"
        ;;
      *)
        die "Unknown argument: $1"
        ;;
    esac
    shift
  done

  [[ -n "${COMMAND}" ]] || die "No command specified. See --help."
  [[ -n "${STORAGE_TYPE}" ]] || die "--storage-type must be provided for '${COMMAND}'."

  if [[ "${STORAGE_TYPE}" != "local" && "${STORAGE_TYPE}" != "minio" && "${STORAGE_TYPE}" != "mixed" && "${STORAGE_TYPE}" != "sftp" ]]; then
    die "Invalid storage type '${STORAGE_TYPE}'. Expected 'local', 'minio', 'mixed', or 'sftp'."
  fi
}

main() {
  parse_args "$@"
  ensure_workdir
  ensure_rclone_binary
  ensure_rclone_config

  # Teardown uses rclone (purge); prevent hangs for minio/mixed/sftp
  if [[ "${STORAGE_TYPE}" == "minio" || "${STORAGE_TYPE}" == "mixed" ]]; then
    export RCLONE_TEST_TIMEOUT="${RCLONE_TEST_TIMEOUT:-120}"
  fi
  if [[ "${STORAGE_TYPE}" == "sftp" ]]; then
    export RCLONE_TEST_TIMEOUT="${RCLONE_TEST_TIMEOUT:-120}"
  fi

  case "${COMMAND}" in
    start)
      if [[ "${STORAGE_TYPE}" == "minio" || "${STORAGE_TYPE}" == "mixed" ]]; then
        start_minio_containers
      elif [[ "${STORAGE_TYPE}" == "sftp" ]]; then
        start_sftp_containers
      else
        log_info "main" "'start' only applies to MinIO-based (minio or mixed) or SFTP (sftp) storage types."
        exit 0
      fi
      ;;

    stop)
      if [[ "${STORAGE_TYPE}" == "minio" || "${STORAGE_TYPE}" == "mixed" ]]; then
        stop_minio_containers
      elif [[ "${STORAGE_TYPE}" == "sftp" ]]; then
        stop_sftp_containers
      else
        log_info "main" "'stop' only applies to MinIO-based (minio or mixed) or SFTP (sftp) storage types."
        exit 0
      fi
      ;;

    recreate)
      if [[ "${STORAGE_TYPE}" == "local" ]]; then
        log_info "main" "'recreate' does not apply to local storage (no containers). Use for --storage-type=minio, mixed, or sftp."
        exit 0
      fi
      if [[ "${STORAGE_TYPE}" == "minio" || "${STORAGE_TYPE}" == "mixed" ]]; then
        check_docker_available
        log_info "main" "Recreating MinIO containers (stop → remove → start)..."
        stop_minio_containers
        remove_minio_containers
        start_minio_containers
        log_info "main" "MinIO containers recreated."
      elif [[ "${STORAGE_TYPE}" == "sftp" ]]; then
        check_docker_available
        log_info "main" "Recreating SFTP containers (stop → remove → start)..."
        stop_sftp_containers
        remove_sftp_containers
        start_sftp_containers
        log_info "main" "SFTP containers recreated."
      fi
      ;;

    teardown)
      [[ "${STORAGE_TYPE}" != "minio" && "${STORAGE_TYPE}" != "mixed" ]] || ensure_minio_containers_ready
      [[ "${STORAGE_TYPE}" != "sftp" ]] || ensure_sftp_containers_ready
      set_remotes_for_storage_type
      purge_raid3_remote_root
      purge_remote_root "${SINGLE_REMOTE}"
      if [[ "${STORAGE_TYPE}" == "minio" || "${STORAGE_TYPE}" == "mixed" ]]; then
        sleep 3
      fi
      if [[ "${STORAGE_TYPE}" == "sftp" ]]; then
        sleep 2
      fi
      if [[ "${STORAGE_TYPE}" == "local" ]]; then
        for dir in "${LOCAL_RAID3_DIRS[@]}" "${LOCAL_SINGLE_DIR}"; do
          remove_leftover_files "${dir}"
          verify_directory_empty "${dir}"
        done
      elif [[ "${STORAGE_TYPE}" == "mixed" ]]; then
        for dir in "${LOCAL_EVEN_DIR}" "${LOCAL_PARITY_DIR}" "${LOCAL_SINGLE_DIR}"; do
          remove_leftover_files "${dir}"
          verify_directory_empty "${dir}"
        done
        for dir in "${MINIO_ODD_DIR}" "${MINIO_SINGLE_DIR}"; do
          remove_leftover_files "${dir}"
          verify_directory_empty "${dir}"
        done
      elif [[ "${STORAGE_TYPE}" == "sftp" ]]; then
        for dir in "${SFTP_RAID3_DIRS[@]}" "${SFTP_SINGLE_DIR}"; do
          remove_leftover_files "${dir}"
          verify_directory_empty "${dir}"
        done
      else
        for dir in "${MINIO_RAID3_DIRS[@]}" "${MINIO_SINGLE_DIR}"; do
          remove_leftover_files "${dir}"
          verify_directory_empty "${dir}"
        done
      fi
      ;;
  esac
}

main "$@"
