#!/usr/bin/env bash
#
# compare_raid3_with_single_all.sh
# ----------------------------------
# Master test script that runs all integration tests across all RAID3 backends.
#
# This script runs all integration test suites (serverside_operations.sh excluded for now; see backend/raid3/docs/OPEN_ISSUES.md):
#   - compare_raid3_with_single.sh (with local, minio, mixed)
#   - compare_raid3_with_single_heal.sh (with local, minio, mixed)
#   - compare_raid3_with_single_errors.sh (with minio only)
#   - compare_raid3_with_single_rebuild.sh (with local, minio, mixed)
#   - compare_raid3_with_single_features.sh (with mixed only)
#   - compare_raid3_with_single_stacking.sh (with local, minio)
#   - performance_test.sh (with local, minio; uses scenario all-but-4G)
#
# Usage:
#   compare_raid3_with_single_all.sh [options]
#
# Options:
#   -v, --verbose         Show detailed output from individual test scripts
#   --storage-type <t>    Run only with given backend: local, minio, or mixed.
#                         If not supplied, runs all storage types for each test.
#   -h, --help            Display this help text
#
# Environment:
#   RCLONE_CONFIG   Path to rclone configuration file.
#                   Defaults to $HOME/.config/rclone/rclone.conf.
#
# Safety guard: the script must be executed from backend/raid3/test directory.
# -----------------------------------------------------------------------------

set -euo pipefail

SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)

# Handle -h/--help early so usage works even when rclone binary is not built
for arg in "$@"; do
  if [[ "${arg}" == "-h" || "${arg}" == "--help" ]]; then
    cat <<EOF
Usage: ${SCRIPT_NAME} [options]

Options:
  -v, --verbose         Show detailed output from individual test scripts
  --storage-type <t>    Run only with given backend: local, minio, or mixed.
                        If not supplied, runs all storage types for each test.
  -h, --help            Display this help text

This script runs all integration tests across all RAID3 backends
(serverside_operations.sh excluded for now; see backend/raid3/docs/OPEN_ISSUES.md):
  - compare_raid3_with_single.sh (local, minio, mixed)
  - compare_raid3_with_single_heal.sh (local, minio, mixed)
  - compare_raid3_with_single_errors.sh (minio only)
  - compare_raid3_with_single_rebuild.sh (local, minio, mixed)
  - compare_raid3_with_single_features.sh (mixed only)
  - compare_raid3_with_single_stacking.sh (local, minio)
  - performance_test.sh (local, minio; scenario all-but-4G)

Each test suite is run with the appropriate storage types, and only
pass/fail status is shown unless --verbose is used.
EOF
    exit 0
  fi
done

# Source common script to get ensure_rclone_binary and other helper functions
# shellcheck source=compare_raid3_with_single_common.sh
. "${SCRIPT_DIR}/compare_raid3_with_single_common.sh"

VERBOSE=0
STORAGE_TYPE_FILTER=""

# Test scripts and their storage types
# Format: "script_name:storage_type1,storage_type2,..."
TEST_SCRIPTS=(
  "compare_raid3_with_single.sh:local,minio,mixed"
  "compare_raid3_with_single_heal.sh:local,minio,mixed"
  "compare_raid3_with_single_errors.sh:minio"
  "compare_raid3_with_single_rebuild.sh:local,minio,mixed"
  "compare_raid3_with_single_features.sh:local,minio,mixed"
  "compare_raid3_with_single_stacking.sh:local,minio"
  "performance_test.sh:local,minio"
)

# ---------------------------- helper functions ------------------------------

usage() {
  cat <<EOF
Usage: ${SCRIPT_NAME} [options]

Options:
  -v, --verbose         Show detailed output from individual test scripts
  --storage-type <t>    Run only with given backend: local, minio, or mixed.
                        If not supplied, runs all storage types for each test.
  -h, --help            Display this help text

This script runs all integration tests across all RAID3 backends
(serverside_operations.sh excluded for now; see backend/raid3/docs/OPEN_ISSUES.md):
  - compare_raid3_with_single.sh (local, minio, mixed)
  - compare_raid3_with_single_heal.sh (local, minio, mixed)
  - compare_raid3_with_single_errors.sh (minio only)
  - compare_raid3_with_single_rebuild.sh (local, minio, mixed)
  - compare_raid3_with_single_features.sh (mixed only)
  - compare_raid3_with_single_stacking.sh (local, minio)
  - performance_test.sh (local, minio; scenario all-but-4G)

Each test suite is run with the appropriate storage types, and only
pass/fail status is shown unless --verbose is used.
EOF
}

# run_test_script runs a test script for the given storage type.
# Optional extra arguments are passed after "test" (e.g. for features: "mixed-features").
# Returns 0 on success, 1 on failure. Uses common's log_info/log_pass/log_fail.
run_test_script() {
  local script_path="$1"
  local storage_type="$2"
  shift 2
  local script_name
  script_name=$(basename "${script_path}")

  log_info "all" "Starting: ${script_name} (${storage_type})"

  local cmd_args=("--storage-type" "${storage_type}" "test")
  if [[ $# -gt 0 ]]; then
    cmd_args+=("$@")
  fi
  if [[ "${VERBOSE}" -eq 1 ]]; then
    cmd_args+=("-v")
  fi

  if [[ "${VERBOSE}" -eq 1 ]]; then
    if "${script_path}" "${cmd_args[@]}"; then
      log_pass "${script_name} (${storage_type})"
      return 0
    else
      log_fail "${script_name} (${storage_type})"
      return 1
    fi
  else
    if "${script_path}" "${cmd_args[@]}" >/dev/null 2>&1; then
      log_pass "${script_name} (${storage_type})"
      return 0
    else
      log_fail "${script_name} (${storage_type})"
      return 1
    fi
  fi
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      -v|--verbose)
        VERBOSE=1
        shift
        ;;
      --storage-type)
        shift
        [[ $# -gt 0 ]] || { echo "Missing argument for --storage-type" >&2; usage >&2; exit 1; }
        STORAGE_TYPE_FILTER="$1"
        shift
        ;;
      --storage-type=*)
        STORAGE_TYPE_FILTER="${1#*=}"
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        echo "Unknown option: $1" >&2
        usage >&2
        exit 1
        ;;
    esac
  done
  if [[ -n "${STORAGE_TYPE_FILTER}" && "${STORAGE_TYPE_FILTER}" != "local" && "${STORAGE_TYPE_FILTER}" != "minio" && "${STORAGE_TYPE_FILTER}" != "mixed" ]]; then
    echo "Invalid --storage-type '${STORAGE_TYPE_FILTER}'. Expected local, minio, or mixed." >&2
    usage >&2
    exit 1
  fi
}

# ensure_workdir is now provided by compare_raid3_with_single_common.sh
# (removed local definition to avoid conflicts)

# ------------------------------- main logic ---------------------------------

main() {
  parse_args "$@"
  ensure_workdir
  ensure_rclone_binary

  # Prevent any single rclone command from hanging (raid3 can block on List/mkdir/copy/sync).
  export RCLONE_TEST_TIMEOUT="${RCLONE_TEST_TIMEOUT:-120}"
  if [[ -n "${RCLONE_TEST_TIMEOUT}" ]]; then
    log_info "all" "Rclone command timeout: ${RCLONE_TEST_TIMEOUT}s (exit 124 = timed out)"
  fi

  ensure_rclone_config

  log_info "all" "=========================================="
  log_info "all" "Running all RAID3 integration tests"
  [[ -n "${STORAGE_TYPE_FILTER}" ]] && log_info "all" "Storage type filter: ${STORAGE_TYPE_FILTER} only"
  log_info "all" "=========================================="
  echo ""

  local total_tests=0
  local passed_tests=0
  local failed_tests=0
  local failed_test_list=()

  # Process each test script
  for test_config in "${TEST_SCRIPTS[@]}"; do
    # Split script name and storage types
    local script_name="${test_config%%:*}"
    local storage_types_str="${test_config#*:}"

    # Split storage types into array
    local storage_types_array=()
    IFS=',' read -ra storage_types_array <<< "${storage_types_str}"

    # Filter to requested storage type if --storage-type was set
    if [[ -n "${STORAGE_TYPE_FILTER}" ]]; then
      local filtered=()
      for st in "${storage_types_array[@]}"; do
        [[ "${st}" == "${STORAGE_TYPE_FILTER}" ]] && filtered+=("${st}")
      done
      if [[ ${#filtered[@]} -gt 0 ]]; then
        storage_types_array=("${filtered[@]}")
      else
        storage_types_array=()
      fi
      [[ ${#storage_types_array[@]} -eq 0 ]] && continue
    fi

    script_path="${SCRIPT_DIR}/${script_name}"

    if [[ ! -f "${script_path}" ]]; then
      log_fail "all" "Test script not found: ${script_path}"
      failed_tests=$((failed_tests + ${#storage_types_array[@]}))
      total_tests=$((total_tests + ${#storage_types_array[@]}))
      continue
    fi

    # Make script executable
    chmod +x "${script_path}"

    # Run test for each storage type
    for storage_type in "${storage_types_array[@]}"; do
      total_tests=$((total_tests + 1))

      # Features script only supports mixed; pass test name "mixed-features"
      if [[ "${script_name}" == "compare_raid3_with_single_features.sh" ]]; then
        if [[ "${storage_type}" == "mixed" ]]; then
          if run_test_script "${script_path}" "mixed" "mixed-features"; then
            passed_tests=$((passed_tests + 1))
          else
            failed_tests=$((failed_tests + 1))
            failed_test_list+=("${script_name} (${storage_type})")
          fi
        else
          log_info "all" "SKIP: ${script_name} (${storage_type}) - features test only supports mixed"
          total_tests=$((total_tests - 1))
        fi
      # Performance test uses scenario all-but-4G (skip 4G file size)
      elif [[ "${script_name}" == "performance_test.sh" ]]; then
        if run_test_script "${script_path}" "${storage_type}" "all-but-4G"; then
          passed_tests=$((passed_tests + 1))
        else
          failed_tests=$((failed_tests + 1))
          failed_test_list+=("${script_name} (${storage_type})")
        fi
      else
        if run_test_script "${script_path}" "${storage_type}"; then
          passed_tests=$((passed_tests + 1))
        else
          failed_tests=$((failed_tests + 1))
          failed_test_list+=("${script_name} (${storage_type})")
        fi
      fi
    done

    echo ""
  done

  # Print summary
  log_info "all" "=========================================="
  log_info "all" "Test Summary"
  log_info "all" "=========================================="
  log_info "all" "Total tests: ${total_tests}"
  log_info "all" "Passed: ${passed_tests}"
  log_info "all" "Failed: ${failed_tests}"
  echo ""

  if [[ ${failed_tests} -gt 0 ]]; then
    log_info "all" "Failed tests:"
    for failed_test in "${failed_test_list[@]}"; do
      log_info "all" "  - ${failed_test}"
    done
    echo ""
    exit 1
  else
    log_pass "all" "All tests passed"
    echo ""
    exit 0
  fi
}

# Run main function
main "$@"

