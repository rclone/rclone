#!/usr/bin/env bash
#
# compare_all.sh — master script for rs shell integration tests (same idea as raid3/compare_all.sh).
#
# By default runs the full suite for each storage backend: local, then minio.
# Use --storage-type to run only one backend.
#
# Per storage type, runs (in order):
#   1. compare.sh test verify   (smoke + rsverify)
#   2. compare.sh test heal     (smoke + drop shard + heal (single-object) + rsverify)
#
# Usage:
#   ./compare_all.sh [-v] [--storage-type=local|minio]
#   ./compare_all.sh test [options]    (optional "test" is ignored; same as raid3)
#
# Environment:
#   COMPARE_ALL_SLEEP_BETWEEN_TESTS  Seconds to sleep between steps (default: 1). Set to 0 to disable.
#

set -euo pipefail

SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)

# shellcheck source=compare_common.sh
. "${SCRIPT_DIR}/compare_common.sh"

VERBOSE=0
STORAGE_TYPE_FILTER=""
SLEEP_BETWEEN="${COMPARE_ALL_SLEEP_BETWEEN_TESTS:-1}"

usage() {
  cat <<EOF
Usage: ${SCRIPT_NAME} [-v] [--storage-type=local|minio]
       ${SCRIPT_NAME} test [options]   (optional "test" is ignored)

By default runs verify + heal (single-object repair) for each storage type: local, then minio.
Pass --storage-type to run only that backend.

  -v, --verbose              Show rclone output from compare.sh
  --storage-type <t>         Run only local or minio (default: both)
  -h, --help                 This help

MinIO requires Docker; see ./manage.sh start --storage-type=minio

Environment:
  COMPARE_ALL_SLEEP_BETWEEN_TESTS   Pause between steps in seconds (default: 1; 0 = off)

Must be run from: ${SCRIPT_DIR}
EOF
}

for arg in "$@"; do
  if [[ "${arg}" == "-h" || "${arg}" == "--help" ]]; then
    usage
    exit 0
  fi
done

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      -v | --verbose)
        VERBOSE=1
        shift
        ;;
      --storage-type)
        shift
        [[ $# -gt 0 ]] || die "--storage-type requires a value"
        STORAGE_TYPE_FILTER="$1"
        shift
        ;;
      --storage-type=*)
        STORAGE_TYPE_FILTER="${1#*=}"
        shift
        ;;
      test)
        shift
        ;;
      *)
        die "Unknown option: $1 (try --help)"
        ;;
    esac
  done
}

run_compare() {
  local storage_type="$1"
  shift
  if [[ "${VERBOSE}" -eq 1 ]]; then
    "${SCRIPT_DIR}/compare.sh" -v --storage-type="${storage_type}" "$@"
  else
    "${SCRIPT_DIR}/compare.sh" --storage-type="${storage_type}" "$@"
  fi
}

main() {
  parse_args "$@"

  if [[ -n "${STORAGE_TYPE_FILTER}" && "${STORAGE_TYPE_FILTER}" != "local" && "${STORAGE_TYPE_FILTER}" != "minio" ]]; then
    die "Invalid --storage-type '${STORAGE_TYPE_FILTER}'. Expected local or minio."
  fi

  local storage_types=()
  if [[ -n "${STORAGE_TYPE_FILTER}" ]]; then
    storage_types=("${STORAGE_TYPE_FILTER}")
  else
    storage_types=(local minio)
  fi

  ensure_workdir
  ensure_rclone_config

  log_info "${SCRIPT_NAME}" "=========================================="
  log_info "${SCRIPT_NAME}" "Running rs integration tests"
  if [[ -n "${STORAGE_TYPE_FILTER}" ]]; then
    log_info "${SCRIPT_NAME}" "Storage filter: ${STORAGE_TYPE_FILTER} only"
  else
    log_info "${SCRIPT_NAME}" "Storage types: ${storage_types[*]} (verify + heal (single-object repair) each)"
  fi
  if [[ "${SLEEP_BETWEEN}" != "0" ]]; then
    log_info "${SCRIPT_NAME}" "Sleep between steps: ${SLEEP_BETWEEN}s (set COMPARE_ALL_SLEEP_BETWEEN_TESTS=0 to disable)"
  fi
  log_info "${SCRIPT_NAME}" "=========================================="
  echo ""

  local failed=()
  local st

  for st in "${storage_types[@]}"; do
    log_info "${SCRIPT_NAME}" ">>> --storage-type=${st}"
    if ! run_compare "${st}" test verify; then
      failed+=("verify (${st})")
    fi
    if [[ "${SLEEP_BETWEEN}" != "0" ]]; then
      sleep "${SLEEP_BETWEEN}"
    fi
    if ! run_compare "${st}" test heal; then
      failed+=("heal (${st})")
    fi
    if [[ "${SLEEP_BETWEEN}" != "0" ]]; then
      sleep "${SLEEP_BETWEEN}"
    fi
    echo ""
  done

  log_info "${SCRIPT_NAME}" "=========================================="
  log_info "${SCRIPT_NAME}" "Summary"
  log_info "${SCRIPT_NAME}" "=========================================="

  if [[ ${#failed[@]} -gt 0 ]]; then
    log_fail "${SCRIPT_NAME}" "Failed (${#failed[@]}):"
    local x
    for x in "${failed[@]}"; do
      log_fail "${SCRIPT_NAME}" "  - ${x}"
    done
    exit 1
  fi

  log_pass "${SCRIPT_NAME}" "All suites passed (${#storage_types[@]} storage type(s) × verify + heal (single-object repair))"
}

main "$@"
