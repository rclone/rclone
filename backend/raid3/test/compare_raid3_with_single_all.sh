#!/usr/bin/env bash
#
# compare_raid3_with_single_all.sh
# ----------------------------------
# Master test script that runs all integration tests across all RAID3 backends.
#
# This script runs all 4 test suites:
#   - compare_raid3_with_single.sh (with local, minio, mixed)
#   - compare_raid3_with_single_heal.sh (with local, minio, mixed)
#   - compare_raid3_with_single_errors.sh (with minio only)
#   - compare_raid3_with_single_rebuild.sh (with local, minio, mixed)
#
# Usage:
#   compare_raid3_with_single_all.sh [options]
#
# Options:
#   -v, --verbose    Show detailed output from individual test scripts
#   -h, --help       Display this help text
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

VERBOSE=0

# Storage types to test
STORAGE_TYPES=("local" "minio" "mixed")

# Test scripts and their storage types
# Format: "script_name:storage_type1,storage_type2,..."
TEST_SCRIPTS=(
  "compare_raid3_with_single.sh:local,minio,mixed"
  "compare_raid3_with_single_heal.sh:local,minio,mixed"
  "compare_raid3_with_single_errors.sh:minio"
  "compare_raid3_with_single_rebuild.sh:local,minio,mixed"
)

# ---------------------------- helper functions ------------------------------

usage() {
  cat <<EOF
Usage: ${SCRIPT_NAME} [options]

Options:
  -v, --verbose    Show detailed output from individual test scripts
  -h, --help       Display this help text

This script runs all integration tests across all RAID3 backends:
  - compare_raid3_with_single.sh (local, minio, mixed)
  - compare_raid3_with_single_heal.sh (local, minio, mixed)
  - compare_raid3_with_single_errors.sh (minio only)
  - compare_raid3_with_single_rebuild.sh (local, minio, mixed)

Each test suite is run with the appropriate storage types, and only
pass/fail status is shown unless --verbose is used.
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
  local script_name=$(basename "${script_path}")
  
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
    # Suppress output, only show errors
    if "${script_path}" "${cmd_args[@]}" >/dev/null 2>&1; then
      log_test_result "${script_name}" "${storage_type}" "PASS"
      return 0
    else
      log_test_result "${script_name}" "${storage_type}" "FAIL"
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

ensure_workdir() {
  # Script must be run from the test directory (where this script is located)
  if [[ "$(pwd)" != "${SCRIPT_DIR}" ]]; then
    echo "Error: This script must be run from ${SCRIPT_DIR}" >&2
    echo "Current directory: $(pwd)" >&2
    exit 1
  fi
}

# ------------------------------- main logic ---------------------------------

main() {
  parse_args "$@"
  ensure_workdir
  
  log "=========================================="
  log "Running all RAID3 integration tests"
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
      total_tests=$((total_tests + 1))
      
      if run_test_script "${script_path}" "${storage_type}"; then
        passed_tests=$((passed_tests + 1))
      else
        failed_tests=$((failed_tests + 1))
        failed_test_list+=("${script_name} (${storage_type})")
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

