#!/usr/bin/env bash
#
# compare_raid3_with_single_all.sh
# ----------------------------------
# Master test script that runs all integration tests across all RAID3 backends.
#
# This script runs all test suites:
#   - compare_raid3_with_single.sh (with local, minio, mixed)
#   - compare_raid3_with_single_heal.sh (with local, minio, mixed)
#   - compare_raid3_with_single_errors.sh (with minio only)
#   - compare_raid3_with_single_rebuild.sh (with local, minio, mixed)
#   - compare_raid3_with_single_features.sh (with mixed only)
#   - compare_raid3_with_single_stacking.sh (with local, minio)
#   - serverside_operations.sh (with local, minio)
#   - performance_test.sh (with local, minio)
#
# Usage:
#   compare_raid3_with_single_all.sh [options]
#
# Options:
#   --storage-type <local|minio|mixed>   Filter tests to run only for specified storage type.
#                                        If not specified, runs all applicable storage types.
#   -v, --verbose                        Show detailed output from individual test scripts
#   -h, --help                          Display this help text
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

# Source common script to get ensure_rclone_binary and other helper functions
# shellcheck source=backend/raid3/test/compare_raid3_with_single_common.sh
. "${SCRIPT_DIR}/compare_raid3_with_single_common.sh"

VERBOSE=0
STORAGE_TYPE_FILTER=""  # If set, only run tests for this storage type

# Test scripts and their storage types
# Format: "script_name:storage_type1,storage_type2,..."
TEST_SCRIPTS=(
  "compare_raid3_with_single.sh:local,minio,mixed"
  "compare_raid3_with_single_heal.sh:local,minio,mixed"
  "compare_raid3_with_single_errors.sh:minio"
  "compare_raid3_with_single_rebuild.sh:local,minio,mixed"
  "compare_raid3_with_single_features.sh:mixed"
  "compare_raid3_with_single_stacking.sh:local,minio"
  "serverside_operations.sh:local,minio"
  "performance_test.sh:local,minio"
)

# ---------------------------- helper functions ------------------------------

usage() {
  cat <<EOF
Usage: ${SCRIPT_NAME} [options]

Options:
  --storage-type <local|minio|mixed>   Filter tests to run only for specified storage type.
                                        If not specified, runs all applicable storage types.
  -v, --verbose                        Show detailed output from individual test scripts
  -h, --help                          Display this help text

This script runs all integration tests across all RAID3 backends:
  - compare_raid3_with_single.sh (local, minio, mixed)
  - compare_raid3_with_single_heal.sh (local, minio, mixed)
  - compare_raid3_with_single_errors.sh (minio only)
  - compare_raid3_with_single_rebuild.sh (local, minio, mixed)
  - compare_raid3_with_single_features.sh (mixed only)
  - compare_raid3_with_single_stacking.sh (local, minio)
  - serverside_operations.sh (local, minio)
  - performance_test.sh (local, minio)

Each test suite is run with the appropriate storage types, and only
pass/fail status is shown unless --verbose is used.

If --storage-type is specified, only tests that support that storage type
will be run.
EOF
}

log() {
  echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*" >&2
}

log_test_start() {
  local script_name="$1"
  local storage_type="$2"
  log "▶ Starting: ${script_name} (${storage_type})"
}

log_test_result() {
  local script_name="$1"
  local storage_type="$2"
  local status="$3"
  if [[ "${status}" == "PASS" ]]; then
    log "✓ PASS: ${script_name} (${storage_type})"
  else
    log "✗ FAIL: ${script_name} (${storage_type})"
  fi
}

run_test_script() {
  local script_path="$1"
  local storage_type="$2"
  local script_name
  script_name=$(basename "${script_path}")
  
  log_test_start "${script_name}" "${storage_type}"
  
  local cmd_args=("--storage-type" "${storage_type}" "test")
  if [[ "${VERBOSE}" -eq 1 ]]; then
    cmd_args+=("-v")
  fi
  
  # Redirect output based on verbose mode
  if [[ "${VERBOSE}" -eq 1 ]]; then
    if "${script_path}" "${cmd_args[@]}"; then
      log_test_result "${script_name}" "${storage_type}" "PASS"
      return 0
    else
      log_test_result "${script_name}" "${storage_type}" "FAIL"
      return 1
    fi
  else
    # Suppress output; on failure re-run with verbose to show reason
    if "${script_path}" "${cmd_args[@]}" >/dev/null 2>&1; then
      log_test_result "${script_name}" "${storage_type}" "PASS"
      return 0
    else
      log_test_result "${script_name}" "${storage_type}" "FAIL"
      log "Re-running with verbose output to show failure reason:"
      cmd_args+=("-v")
      "${script_path}" "${cmd_args[@]}" || true
      return 1
    fi
  fi
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --storage-type)
        shift
        [[ $# -gt 0 ]] || { echo "ERROR: --storage-type requires an argument" >&2; usage >&2; exit 1; }
        STORAGE_TYPE_FILTER="$1"
        if [[ "${STORAGE_TYPE_FILTER}" != "local" && "${STORAGE_TYPE_FILTER}" != "minio" && "${STORAGE_TYPE_FILTER}" != "mixed" ]]; then
          echo "ERROR: Invalid storage type '${STORAGE_TYPE_FILTER}'. Expected 'local', 'minio', or 'mixed'." >&2
          usage >&2
          exit 1
        fi
        shift
        ;;
      --storage-type=*)
        STORAGE_TYPE_FILTER="${1#*=}"
        if [[ "${STORAGE_TYPE_FILTER}" != "local" && "${STORAGE_TYPE_FILTER}" != "minio" && "${STORAGE_TYPE_FILTER}" != "mixed" ]]; then
          echo "ERROR: Invalid storage type '${STORAGE_TYPE_FILTER}'. Expected 'local', 'minio', or 'mixed'." >&2
          usage >&2
          exit 1
        fi
        shift
        ;;
      -v|--verbose)
        VERBOSE=1
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
}

# ensure_workdir is now provided by compare_raid3_with_single_common.sh
# (removed local definition to avoid conflicts)

# ------------------------------- main logic ---------------------------------

main() {
  parse_args "$@"
  ensure_workdir
  ensure_rclone_binary
  
  log "=========================================="
  log "Running all RAID3 integration tests"
  if [[ -n "${STORAGE_TYPE_FILTER}" ]]; then
    log "Storage type filter: ${STORAGE_TYPE_FILTER}"
  fi
  log "=========================================="
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
    
    script_path="${SCRIPT_DIR}/${script_name}"
    
    if [[ ! -f "${script_path}" ]]; then
      log "✗ ERROR: Test script not found: ${script_path}"
      failed_tests=$((failed_tests + ${#storage_types_array[@]}))
      total_tests=$((total_tests + ${#storage_types_array[@]}))
      continue
    fi
    
    # Make script executable
    chmod +x "${script_path}"
    
    # Run test for each storage type
    for storage_type in "${storage_types_array[@]}"; do
      # Filter by storage type if specified
      if [[ -n "${STORAGE_TYPE_FILTER}" && "${storage_type}" != "${STORAGE_TYPE_FILTER}" ]]; then
        continue
      fi
      
      total_tests=$((total_tests + 1))
      
      # For features script, we need to pass the test name
      if [[ "${script_name}" == "compare_raid3_with_single_features.sh" ]]; then
        # Features script requires test name, run mixed-features test
        if [[ "${storage_type}" == "mixed" ]]; then
          log_test_start "${script_name}" "${storage_type}"
          local cmd_args=("--storage-type" "${storage_type}" "test" "mixed-features")
          if [[ "${VERBOSE}" -eq 1 ]]; then
            cmd_args+=("-v")
          fi
          
          if [[ "${VERBOSE}" -eq 1 ]]; then
            if "${script_path}" "${cmd_args[@]}"; then
              log_test_result "${script_name}" "${storage_type}" "PASS"
              passed_tests=$((passed_tests + 1))
            else
              log_test_result "${script_name}" "${storage_type}" "FAIL"
              failed_tests=$((failed_tests + 1))
              failed_test_list+=("${script_name} (${storage_type})")
            fi
          else
            if "${script_path}" "${cmd_args[@]}" >/dev/null 2>&1; then
              log_test_result "${script_name}" "${storage_type}" "PASS"
              passed_tests=$((passed_tests + 1))
            else
              log_test_result "${script_name}" "${storage_type}" "FAIL"
              failed_tests=$((failed_tests + 1))
              failed_test_list+=("${script_name} (${storage_type})")
              log "Re-running with verbose output to show failure reason:"
              cmd_args+=("-v")
              "${script_path}" "${cmd_args[@]}" || true
            fi
          fi
        else
          # Skip non-mixed storage types for features script
          log "⊘ SKIP: ${script_name} (${storage_type}) - features test only supports mixed"
          total_tests=$((total_tests - 1))
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
  log "=========================================="
  log "Test Summary"
  log "=========================================="
  log "Total tests: ${total_tests}"
  log "Passed: ${passed_tests}"
  log "Failed: ${failed_tests}"
  echo ""
  
  if [[ ${failed_tests} -gt 0 ]]; then
    log "Failed tests:"
    for failed_test in "${failed_test_list[@]}"; do
      log "  - ${failed_test}"
    done
    echo ""
    exit 1
  else
    log "All tests passed! ✓"
    echo ""
    exit 0
  fi
}

# Run main function
main "$@"
