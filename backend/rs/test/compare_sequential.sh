#!/usr/bin/env bash
#
# compare_sequential.sh — run each compare.sh test in order; continue after failures
# and print a summary (handy to see which step breaks).
#
# Usage:
#   ./compare_sequential.sh [--storage-type=local|minio] [-v]
#
# Requires: cwd = this directory, ./setup.sh (tests.config), repo-root rclone binary
# for verify/heal (rsverify). MinIO: Docker + ./manage.sh start --storage-type=minio
#
set -u

SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)
cd "${SCRIPT_DIR}" || exit 1

# shellcheck source=compare_common.sh
. "${SCRIPT_DIR}/compare_common.sh"

VERBOSE=0
STORAGE_TYPE="local"
# Same order as documented in compare.sh list; smoke first to isolate rcat/upload issues.
TESTS=(smoke verify heal quorum_dirs move_copy)

usage() {
  cat <<EOF
Usage: ${SCRIPT_NAME} [-v] [--storage-type=local|minio]

Runs, in order: ${TESTS[*]}

Unlike a single failing compare_all run, this script always runs every test and
reports which names failed at the end (exit 1 if any failed).

EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h | --help)
      usage
      exit 0
      ;;
    -v | --verbose)
      VERBOSE=1
      shift
      ;;
    --storage-type)
      shift
      [[ $# -gt 0 ]] || die "--storage-type requires a value"
      STORAGE_TYPE="$1"
      shift
      ;;
    --storage-type=*)
      STORAGE_TYPE="${1#*=}"
      shift
      ;;
    *)
      die "Unknown option: $1 (try --help)"
      ;;
  esac
done

[[ "${STORAGE_TYPE}" == "local" || "${STORAGE_TYPE}" == "minio" ]] || die "Only --storage-type=local or minio (got: ${STORAGE_TYPE})"

ensure_workdir
ensure_rclone_config

if [[ "${STORAGE_TYPE}" == "minio" ]]; then
  RCLONE_BINARY=$(find_rclone_binary)
  export RCLONE_BINARY
  ensure_minio_rs_containers_ready || die "MinIO is not ready (start Docker or run ./manage.sh start --storage-type=minio)"
fi

log_info "${SCRIPT_NAME}" "Running tests one by one (--storage-type=${STORAGE_TYPE})"

failed=()
for t in "${TESTS[@]}"; do
  log_info "${SCRIPT_NAME}" "---------- test ${t} ----------"
  if [[ "${VERBOSE}" -eq 1 ]]; then
    ./compare.sh -v --storage-type="${STORAGE_TYPE}" test "${t}"
  else
    ./compare.sh --storage-type="${STORAGE_TYPE}" test "${t}"
  fi
  rc=$?
  if [[ "${rc}" -ne 0 ]]; then
    log_fail "${SCRIPT_NAME}" "FAILED: ${t} (exit ${rc})"
    failed+=("${t}")
  else
    log_pass "${SCRIPT_NAME}" "passed: ${t}"
  fi
done

echo ""
log_info "${SCRIPT_NAME}" "========== summary =========="
if [[ ${#failed[@]} -eq 0 ]]; then
  log_pass "${SCRIPT_NAME}" "All ${#TESTS[@]} test(s) passed."
  exit 0
fi
log_fail "${SCRIPT_NAME}" "Failed ${#failed[@]} test(s):"
for t in "${failed[@]}"; do
  log_fail "${SCRIPT_NAME}" "  - ${t}"
done
exit 1
