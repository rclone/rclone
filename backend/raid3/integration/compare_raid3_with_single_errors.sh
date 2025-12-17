#!/usr/bin/env bash
#
# compare_raid3_with_single_errors.sh
# ------------------------------------
# Error handling validation harness for the rclone raid3 backend.
#
# This script tests that write operations (Move, Update) properly fail when
# backends are unavailable, following RAID 3 strict write policy. Tests verify
# that rollback mechanism prevents partial operations (all-or-nothing guarantee).
# Rollback-disabled tests use rclone connection strings (remote,rollback=false:path)
# to test behavior without rollback, without requiring a second remote configuration.
# Works with MinIO-backed raid3 configurations, stopping containers to simulate
# backend unavailability.
#
# Usage:
#   compare_raid3_with_single_errors.sh [options] <command> [args]
#
# Commands:
#   start                 Start MinIO containers (requires --storage-type=minio).
#   stop                  Stop MinIO containers (requires --storage-type=minio).
#   teardown              Purge datasets and local/MinIO directories.
#   list                  Show available error scenarios.
#   test [scenario]       Run all or a named scenario.
#
# Options:
#   --storage-type <local|minio>   Backend pair to exercise (required for start/stop/test/teardown).
#   -v, --verbose                  Show stdout/stderr from rclone operations.
#   -h, --help                     Display this help text.
#
# Environment:
#   RCLONE_CONFIG   Path to rclone configuration file (defaults to ~/.config/rclone/rclone.conf).
#
# Safety guard: must be executed from $HOME/go/raid3storage.
# -----------------------------------------------------------------------------

set -euo pipefail

SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)

# shellcheck source=backend/raid3/integration/compare_raid3_with_single_common.sh
. "${SCRIPT_DIR}/compare_raid3_with_single_common.sh"

VERBOSE=0
STORAGE_TYPE=""
COMMAND=""
COMMAND_ARG=""

TARGET_OBJECT="file_root.txt"

usage() {
  cat <<EOF
Usage: ${SCRIPT_NAME} [options] <command> [arguments]

Commands:
  start                      Start MinIO containers (requires --storage-type=minio).
  stop                       Stop MinIO containers (requires --storage-type=minio).
  teardown                   Purge datasets for the selected storage type.
  list                       Show available error scenarios.
  test [scenario]            Run all scenarios or a single one.

Options:
  --storage-type <local|minio>   Backend pair (required for start/stop/test/teardown).
  -v, --verbose                  Show stdout/stderr from rclone operations.
  -h, --help                     Display this help.

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
      -v|--verbose)
        VERBOSE=1
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      start|stop|teardown|list|test)
        if [[ -n "${COMMAND}" ]]; then
          die "Multiple commands provided: '${COMMAND}' and '$1'"
        fi
        COMMAND="$1"
        ;;
      *)
        if [[ "${COMMAND}" == "test" && -z "${COMMAND_ARG}" ]]; then
          COMMAND_ARG="$1"
        else
          die "Unknown argument: $1"
        fi
        ;;
    esac
    shift
  done

  [[ -n "${COMMAND}" ]] || die "No command specified. See --help."

  case "${COMMAND}" in
    start|stop|teardown|test)
      [[ -n "${STORAGE_TYPE}" ]] || die "--storage-type must be provided for '${COMMAND}'"
      ;;
  esac

  if [[ -n "${STORAGE_TYPE}" && "${STORAGE_TYPE}" != "local" && "${STORAGE_TYPE}" != "minio" ]]; then
    die "Invalid storage type '${STORAGE_TYPE}'. Expected 'local' or 'minio'."
  fi
}

print_scenarios() {
  cat <<EOF
Available error scenarios:
  move-fail-even              Stop even backend and verify Move fails (with rollback).
  move-fail-odd               Stop odd backend and verify Move fails (with rollback).
  move-fail-parity            Stop parity backend and verify Move fails (with rollback).
  update-fail-even            Stop even backend and verify Update fails (with rollback).
  update-fail-odd             Stop odd backend and verify Update fails (with rollback).
  update-fail-parity          Stop parity backend and verify Update fails (with rollback).
  rollback-disabled-move-fail-even     Stop even backend and verify Move fails (rollback disabled, partial moves allowed).
  rollback-disabled-move-fail-odd      Stop odd backend and verify Move fails (rollback disabled, partial moves allowed).
  rollback-disabled-move-fail-parity   Stop parity backend and verify Move fails (rollback disabled, partial moves allowed).
  rollback-disabled-update-fail-even   Stop even backend and verify Update fails (rollback disabled, partial updates allowed).
  rollback-disabled-update-fail-odd    Stop odd backend and verify Update fails (rollback disabled, partial updates allowed).
  rollback-disabled-update-fail-parity Stop parity backend and verify Update fails (rollback disabled, partial updates allowed).
EOF
}

ERROR_RESULTS=()

# Helper to construct rollback-disabled remote path using connection string syntax
# Uses rclone connection string: remote,rollback=false:path
get_rollback_disabled_path() {
  local path="$1"
  echo "${RAID3_REMOTE},rollback=false:${path}"
}

reset_error_results() {
  ERROR_RESULTS=()
}

record_error_result() {
  local status="$1"
  local scenario="$2"
  local detail="$3"
  ERROR_RESULTS+=("${status} ${scenario} - ${detail}")
  case "${status}" in
    PASS) log_pass "scenario:${scenario}" "${detail}" ;;
    FAIL) log_fail "scenario:${scenario}" "${detail}" ;;
  esac
}

print_error_summary() {
  log_info "summary:----------"
  if [[ "${#ERROR_RESULTS[@]}" -eq 0 ]]; then
    log_info "summary:No scenarios recorded."
    return
  fi
  for entry in "${ERROR_RESULTS[@]}"; do
    log_info "summary:${entry}"
  done
}

run_move_fail_scenario() {
  local backend="$1"
  log_info "suite" "Running move-fail scenario '${backend}' (${STORAGE_TYPE})"

  # These tests require MinIO to simulate unavailable backends
  if [[ "${STORAGE_TYPE}" != "minio" ]]; then
    record_error_result "PASS" "move-fail-${backend}" "Skipped for local backend (requires MinIO to stop containers)."
    return 0
  fi

  # Ensure all backends are ready before starting (important after previous scenario restored a backend)
  log_info "scenario:move-fail-${backend}" "Verifying all backends are ready before starting scenario."
  for check_backend in even odd parity; do
    if ! wait_for_minio_backend_ready "${check_backend}" 2>/dev/null; then
      log_warn "scenario:move-fail-${backend}" "Backend '${check_backend}' may not be ready, but continuing."
    fi
  done

  purge_remote_root "${RAID3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"

  # Create a test file
  local dataset_id
  dataset_id=$(create_test_dataset "move-fail-${backend}") || {
    record_error_result "FAIL" "move-fail-${backend}" "Failed to create dataset."
    return 1
  }
  log_info "scenario:move-fail-${backend}" "Dataset ${dataset_id} created."

  local test_file="${dataset_id}/${TARGET_OBJECT}"
  local new_file="${dataset_id}/moved_${TARGET_OBJECT}"

  # Verify file exists before move attempt
  local check_result check_status check_stdout check_stderr
  check_result=$(capture_command "check_before" ls "${RAID3_REMOTE}:${test_file}")
  IFS='|' read -r check_status check_stdout check_stderr <<<"${check_result}"
  if [[ "${check_status}" -ne 0 ]]; then
    record_error_result "FAIL" "move-fail-${backend}" "Test file does not exist before move attempt."
    rm -f "${check_stdout}" "${check_stderr}"
    return 1
  fi
  rm -f "${check_stdout}" "${check_stderr}"

  # Stop the backend to simulate unavailability
  log_info "scenario:move-fail-${backend}" "Stopping '${backend}' backend to simulate unavailability."
  stop_single_minio_container "${backend}"

  # Wait for container to fully stop and verify it's unreachable
  sleep 3
  local port
  case "${backend}" in
    even) port="${MINIO_EVEN_PORT}" ;;
    odd) port="${MINIO_ODD_PORT}" ;;
    parity) port="${MINIO_PARITY_PORT}" ;;
  esac
  
  # Verify port is actually closed (backend is unreachable)
  local retries=10
  while (( retries > 0 )); do
    if ! nc -z localhost "${port}" >/dev/null 2>&1; then
      break  # Port is closed, backend is down
    fi
    log_info "scenario:move-fail-${backend}" "Waiting for backend port ${port} to close..."
    sleep 1
    ((retries--))
  done
  
  if nc -z localhost "${port}" >/dev/null 2>&1; then
    log_warn "scenario:move-fail-${backend}" "Backend port ${port} still open after stop attempt."
  else
    log_info "scenario:move-fail-${backend}" "Backend port ${port} confirmed closed."
  fi

  # Attempt move - should fail
  local move_result move_status move_stdout move_stderr
  move_result=$(capture_command "move_attempt" move "${RAID3_REMOTE}:${test_file}" "${RAID3_REMOTE}:${new_file}")
  IFS='|' read -r move_status move_stdout move_stderr <<<"${move_result}"
  print_if_verbose "move attempt" "${move_stdout}" "${move_stderr}"

  # Verify move failed (non-zero exit status) BEFORE restoring backend
  if [[ "${move_status}" -eq 0 ]]; then
    # Restore backend before returning error
    log_info "scenario:move-fail-${backend}" "Restoring '${backend}' backend before error exit."
    start_single_minio_container "${backend}"
    # Wait for backend to be ready (for next scenario)
    if wait_for_minio_port "${port}" 2>/dev/null; then
      wait_for_minio_backend_ready "${backend}" 2>/dev/null || true
    fi
    record_error_result "FAIL" "move-fail-${backend}" "Move succeeded when it should have failed (backend '${backend}' was unavailable)."
    log_note "move" "Move stdout: ${move_stdout}"
    log_note "move" "Move stderr: ${move_stderr}"
    rm -f "${move_stdout}" "${move_stderr}"
    return 1
  fi

  # Verify error message indicates degraded mode or backend unavailability
  local error_message
  error_message=$(cat "${move_stderr}" 2>/dev/null || echo "")
  if [[ -z "${error_message}" ]]; then
    error_message=$(cat "${move_stdout}" 2>/dev/null || echo "")
  fi

  if [[ -n "${error_message}" ]]; then
    if ! echo "${error_message}" | grep -qiE "(degraded|unavailable|failed|error|cannot|blocked)"; then
      log_note "move" "Move failed (good), but error message may not clearly indicate backend unavailability."
      log_note "move" "Error message: ${error_message}"
    fi
  fi

  # IMPORTANT: Check file state WHILE backend is still down
  # With rollback enabled (default), failed moves should leave no trace at destination
  
  # Verify original file still exists (move should not have partially succeeded)
  # Note: This check might fail if we need the down backend, but we check with available backends
  local check_result check_status check_stdout check_stderr
  check_result=$(capture_command "check_after" ls "${RAID3_REMOTE}:${test_file}")
  IFS='|' read -r check_status check_stdout check_stderr <<<"${check_result}"
  
  # Verify new file does NOT exist (rollback should have cleaned up any partial moves)
  # Note: raid3 can list files in degraded mode (2/3 particles), so we check this carefully
  local new_check_result new_check_status new_check_stdout new_check_stderr
  new_check_result=$(capture_command "check_new" ls "${RAID3_REMOTE}:${new_file}")
  IFS='|' read -r new_check_status new_check_stdout new_check_stderr <<<"${new_check_result}"
  
  # Restore backend now (after checking state)
  log_info "scenario:move-fail-${backend}" "Restoring '${backend}' backend."
  start_single_minio_container "${backend}"

  # Wait for container port to be open
  if ! wait_for_minio_port "${port}"; then
    log_warn "scenario:move-fail-${backend}" "Backend port ${port} did not open in time."
  fi

  # Wait for MinIO to be fully ready (not just port open, but S3 API ready)
  if ! wait_for_minio_backend_ready "${backend}"; then
    log_warn "scenario:move-fail-${backend}" "Backend '${backend}' may not be fully ready, but continuing."
  else
    log_info "scenario:move-fail-${backend}" "Backend '${backend}' confirmed ready."
  fi

  # Now verify the checks we did while backend was down
  if [[ "${check_status}" -ne 0 ]]; then
    record_error_result "FAIL" "move-fail-${backend}" "Original file disappeared after failed move (partial move occurred)."
    rm -f "${check_stdout}" "${check_stderr}" "${new_check_stdout}" "${new_check_stderr}" "${move_stdout}" "${move_stderr}"
    return 1
  fi

  if [[ "${new_check_status}" -eq 0 ]]; then
    # New file exists - this indicates partial move occurred despite rollback
    # With rollback enabled (default), this should NOT happen
    # If it does, it means rollback failed or was disabled
    log_fail "move" "Partial move detected: New file exists after failed move (rollback should have prevented this)."
    log_note "move" "File detected at: ${new_file}"
    log_note "move" "This indicates rollback did not work correctly or was disabled."
    
    # Clean up the partially moved file
    log_info "cleanup" "Removing partially moved file at ${new_file}"
    rclone_cmd delete "${RAID3_REMOTE}:${new_file}" >/dev/null 2>&1 || true
    
    record_error_result "FAIL" "move-fail-${backend}" "New file exists after failed move (rollback should have prevented partial move)."
    rm -f "${check_stdout}" "${check_stderr}" "${new_check_stdout}" "${new_check_stderr}" "${move_stdout}" "${move_stderr}"
    
    return 1
  fi

  rm -f "${check_stdout}" "${check_stderr}" "${new_check_stdout}" "${new_check_stderr}" "${move_stdout}" "${move_stderr}"

  record_error_result "PASS" "move-fail-${backend}" "Move correctly failed with unavailable '${backend}' backend."
  return 0
}

run_update_fail_scenario() {
  local backend="$1"
  log_info "suite" "Running update-fail scenario '${backend}' (${STORAGE_TYPE})"

  # These tests require MinIO to simulate unavailable backends
  if [[ "${STORAGE_TYPE}" != "minio" ]]; then
    record_error_result "PASS" "update-fail-${backend}" "Skipped for local backend (requires MinIO to stop containers)."
    return 0
  fi

  # Ensure all backends are ready before starting (important after previous scenario restored a backend)
  log_info "scenario:update-fail-${backend}" "Verifying all backends are ready before starting scenario."
  for check_backend in even odd parity; do
    if ! wait_for_minio_backend_ready "${check_backend}" 2>/dev/null; then
      log_warn "scenario:update-fail-${backend}" "Backend '${check_backend}' may not be ready, but continuing."
    fi
  done

  purge_remote_root "${RAID3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"

  # Create a test file
  local dataset_id
  dataset_id=$(create_test_dataset "update-fail-${backend}") || {
    record_error_result "FAIL" "update-fail-${backend}" "Failed to create dataset."
    return 1
  }
  log_info "scenario:update-fail-${backend}" "Dataset ${dataset_id} created."

  local test_file="${dataset_id}/${TARGET_OBJECT}"

  # Get original file content for verification
  local original_content
  original_content=$(mktemp) || {
    record_error_result "FAIL" "update-fail-${backend}" "Failed to create temp file for original content."
    return 1
  }
  local get_result get_status get_stdout get_stderr
  get_result=$(capture_command "get_original" cat "${RAID3_REMOTE}:${test_file}")
  IFS='|' read -r get_status get_stdout get_stderr <<<"${get_result}"
  if [[ "${get_status}" -ne 0 ]]; then
    record_error_result "FAIL" "update-fail-${backend}" "Failed to read original file content."
    rm -f "${get_stdout}" "${get_stderr}" "${original_content}"
    return 1
  fi
  cp "${get_stdout}" "${original_content}"
  rm -f "${get_stdout}" "${get_stderr}"

  # Stop the backend to simulate unavailability
  log_info "scenario:update-fail-${backend}" "Stopping '${backend}' backend to simulate unavailability."
  stop_single_minio_container "${backend}"

  # Wait a moment for container to fully stop
  sleep 2

  # Create updated content
  local updated_content
  updated_content=$(mktemp) || {
    record_error_result "FAIL" "update-fail-${backend}" "Failed to create temp file for updated content."
    rm -f "${original_content}"
    return 1
  }
  echo "Updated content for update-fail-${backend} test" > "${updated_content}"

  # Attempt update (using copy as update) - should fail
  local update_result update_status update_stdout update_stderr
  update_result=$(capture_command "update_attempt" copyto "${updated_content}" "${RAID3_REMOTE}:${test_file}")
  IFS='|' read -r update_status update_stdout update_stderr <<<"${update_result}"
  print_if_verbose "update attempt" "${update_stdout}" "${update_stderr}"

  # Restore backend
  log_info "scenario:update-fail-${backend}" "Restoring '${backend}' backend."
  start_single_minio_container "${backend}"

  # Wait for container port to be open
  local port
  case "${backend}" in
    even) port="${MINIO_EVEN_PORT}" ;;
    odd) port="${MINIO_ODD_PORT}" ;;
    parity) port="${MINIO_PARITY_PORT}" ;;
  esac
  if ! wait_for_minio_port "${port}"; then
    log_warn "scenario:update-fail-${backend}" "Backend port ${port} did not open in time."
  fi

  # Wait for MinIO to be fully ready (not just port open, but S3 API ready)
  if ! wait_for_minio_backend_ready "${backend}"; then
    log_warn "scenario:update-fail-${backend}" "Backend '${backend}' may not be fully ready, but continuing."
  else
    log_info "scenario:update-fail-${backend}" "Backend '${backend}' confirmed ready."
  fi

  # Verify update failed (non-zero exit status)
  if [[ "${update_status}" -eq 0 ]]; then
    # Backend was already restored earlier, but ensure it's ready for next scenario
    if wait_for_minio_port "${port}" 2>/dev/null; then
      wait_for_minio_backend_ready "${backend}" 2>/dev/null || true
    fi
    record_error_result "FAIL" "update-fail-${backend}" "Update succeeded when it should have failed (backend '${backend}' was unavailable)."
    log_note "update" "Update stdout: ${update_stdout}"
    log_note "update" "Update stderr: ${update_stderr}"
    rm -f "${update_stdout}" "${update_stderr}" "${original_content}" "${updated_content}"
    return 1
  fi

  # Verify error message indicates degraded mode or backend unavailability
  local error_message
  error_message=$(cat "${update_stderr}" 2>/dev/null || echo "")
  if [[ -z "${error_message}" ]]; then
    error_message=$(cat "${update_stdout}" 2>/dev/null || echo "")
  fi

  if [[ -n "${error_message}" ]]; then
    if ! echo "${error_message}" | grep -qiE "(degraded|unavailable|failed|error|cannot)"; then
      log_note "update" "Update failed (good), but error message may not clearly indicate backend unavailability."
      log_note "update" "Error message: ${error_message}"
    fi
  fi

  # Verify original file content unchanged (update should not have partially succeeded)
  local verify_result verify_status verify_stdout verify_stderr
  verify_result=$(capture_command "verify_original" cat "${RAID3_REMOTE}:${test_file}")
  IFS='|' read -r verify_status verify_stdout verify_stderr <<<"${verify_result}"
  if [[ "${verify_status}" -ne 0 ]]; then
    record_error_result "FAIL" "update-fail-${backend}" "Original file missing after failed update."
    rm -f "${verify_stdout}" "${verify_stderr}" "${update_stdout}" "${update_stderr}" "${original_content}" "${updated_content}"
    return 1
  fi

  if ! cmp -s "${original_content}" "${verify_stdout}"; then
    record_error_result "FAIL" "update-fail-${backend}" "Original file content changed after failed update (partial update occurred)."
    rm -f "${verify_stdout}" "${verify_stderr}" "${update_stdout}" "${update_stderr}" "${original_content}" "${updated_content}"
    return 1
  fi

  rm -f "${verify_stdout}" "${verify_stderr}" "${update_stdout}" "${update_stderr}" "${original_content}" "${updated_content}"

  record_error_result "PASS" "update-fail-${backend}" "Update correctly failed with unavailable '${backend}' backend."
  return 0
}

run_move_fail_scenario_no_rollback() {
  local backend="$1"
  log_info "suite" "Running rollback-disabled move-fail scenario '${backend}' (${STORAGE_TYPE})"

  # These tests require MinIO to simulate unavailable backends
  if [[ "${STORAGE_TYPE}" != "minio" ]]; then
    record_error_result "PASS" "rollback-disabled-move-fail-${backend}" "Skipped for local backend (requires MinIO to stop containers)."
    return 0
  fi

  # Ensure all backends are ready before starting (important after previous scenario restored a backend)
  log_info "scenario:rollback-disabled-move-fail-${backend}" "Verifying all backends are ready before starting scenario."
  for check_backend in even odd parity; do
    if ! wait_for_minio_backend_ready "${check_backend}" 2>/dev/null; then
      log_warn "scenario:rollback-disabled-move-fail-${backend}" "Backend '${check_backend}' may not be ready, but continuing."
    fi
  done

  purge_remote_root "${RAID3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"

  # Create a test file using the regular remote (rollback enabled by default)
  local dataset_id
  dataset_id=$(create_test_dataset "rollback-disabled-move-fail-${backend}") || {
    record_error_result "FAIL" "rollback-disabled-move-fail-${backend}" "Failed to create dataset."
    return 1
  }
  log_info "scenario:rollback-disabled-move-fail-${backend}" "Dataset ${dataset_id} created (will use connection string with rollback=false)."

  local test_file="${dataset_id}/${TARGET_OBJECT}"
  local new_file="${dataset_id}/moved_${TARGET_OBJECT}"

  # Verify file exists before move attempt (using connection string with rollback=false)
  local test_file_path new_file_path
  test_file_path=$(get_rollback_disabled_path "${test_file}")
  new_file_path=$(get_rollback_disabled_path "${new_file}")

  local check_result check_status check_stdout check_stderr
  check_result=$(capture_command "check_before" ls "${test_file_path}")
  IFS='|' read -r check_status check_stdout check_stderr <<<"${check_result}"
  if [[ "${check_status}" -ne 0 ]]; then
    record_error_result "FAIL" "rollback-disabled-move-fail-${backend}" "Test file does not exist before move attempt."
    rm -f "${check_stdout}" "${check_stderr}"
    return 1
  fi
  rm -f "${check_stdout}" "${check_stderr}"

  # Stop the backend to simulate unavailability
  log_info "scenario:rollback-disabled-move-fail-${backend}" "Stopping '${backend}' backend to simulate unavailability."
  stop_single_minio_container "${backend}"

  # Wait for container to fully stop
  sleep 3
  local port
  case "${backend}" in
    even) port="${MINIO_EVEN_PORT}" ;;
    odd) port="${MINIO_ODD_PORT}" ;;
    parity) port="${MINIO_PARITY_PORT}" ;;
  esac
  
  # Verify port is actually closed
  local retries=10
  while (( retries > 0 )); do
    if ! nc -z localhost "${port}" >/dev/null 2>&1; then
      break
    fi
    log_info "scenario:rollback-disabled-move-fail-${backend}" "Waiting for backend port ${port} to close..."
    sleep 1
    ((retries--))
  done

  # Attempt move using connection string with rollback=false - should fail
  local move_result move_status move_stdout move_stderr
  move_result=$(capture_command "move_attempt" move "${test_file_path}" "${new_file_path}")
  IFS='|' read -r move_status move_stdout move_stderr <<<"${move_result}"
  print_if_verbose "move attempt" "${move_stdout}" "${move_stderr}"

  # Verify move failed (non-zero exit status)
  if [[ "${move_status}" -eq 0 ]]; then
    start_single_minio_container "${backend}"
    # Wait for backend to be ready (for next scenario)
    if wait_for_minio_port "${port}" 2>/dev/null; then
      wait_for_minio_backend_ready "${backend}" 2>/dev/null || true
    fi
    record_error_result "FAIL" "rollback-disabled-move-fail-${backend}" "Move succeeded when it should have failed (backend '${backend}' was unavailable)."
    rm -f "${move_stdout}" "${move_stderr}"
    return 1
  fi

  # Restore backend now
  log_info "scenario:rollback-disabled-move-fail-${backend}" "Restoring '${backend}' backend."
  start_single_minio_container "${backend}"
  if ! wait_for_minio_port "${port}"; then
    log_warn "scenario:rollback-disabled-move-fail-${backend}" "Backend port ${port} did not open in time."
  fi

  # Wait for MinIO to be fully ready (not just port open, but S3 API ready)
  if ! wait_for_minio_backend_ready "${backend}"; then
    log_warn "scenario:rollback-disabled-move-fail-${backend}" "Backend '${backend}' may not be fully ready, but continuing."
  else
    log_info "scenario:rollback-disabled-move-fail-${backend}" "Backend '${backend}' confirmed ready."
  fi

  # With rollback disabled, partial moves are expected
  # Check if new file exists (partial move occurred) using connection string
  local new_check_result new_check_status new_check_stdout new_check_stderr
  new_check_result=$(capture_command "check_new" ls "${new_file_path}")
  IFS='|' read -r new_check_status new_check_stdout new_check_stderr <<<"${new_check_result}"

  # Verify original file - may or may not exist depending on which particles moved
  local check_result check_status check_stdout check_stderr
  check_result=$(capture_command "check_after" ls "${test_file_path}")
  IFS='|' read -r check_status check_stdout check_stderr <<<"${check_result}"

  if [[ "${new_check_status}" -eq 0 ]]; then
    # Partial move occurred - this is expected with rollback disabled
    log_info "rollback-disabled" "Partial move detected: New file exists after failed move (expected with rollback=false)."
    
    # Clean up the partially moved file using connection string
    log_info "cleanup" "Removing partially moved file at ${new_file}"
    rclone_cmd delete "${new_file_path}" >/dev/null 2>&1 || true
    
    record_error_result "PASS" "rollback-disabled-move-fail-${backend}" "Move correctly failed and partial move occurred (expected with rollback=false)."
    rm -f "${check_stdout}" "${check_stderr}" "${new_check_stdout}" "${new_check_stderr}" "${move_stdout}" "${move_stderr}"
    return 0
  else
    # No partial move - this might happen if move failed early before any particles moved
    log_info "rollback-disabled" "No partial move detected (move failed before any particles moved)."
    record_error_result "PASS" "rollback-disabled-move-fail-${backend}" "Move correctly failed with no partial move (failed early)."
    rm -f "${check_stdout}" "${check_stderr}" "${new_check_stdout}" "${new_check_stderr}" "${move_stdout}" "${move_stderr}"
    return 0
  fi
}

run_update_fail_scenario_no_rollback() {
  local backend="$1"
  log_info "suite" "Running rollback-disabled update-fail scenario '${backend}' (${STORAGE_TYPE})"

  # These tests require MinIO to simulate unavailable backends
  if [[ "${STORAGE_TYPE}" != "minio" ]]; then
    record_error_result "PASS" "rollback-disabled-update-fail-${backend}" "Skipped for local backend (requires MinIO to stop containers)."
    return 0
  fi

  # Ensure all backends are ready before starting (important after previous scenario restored a backend)
  log_info "scenario:rollback-disabled-update-fail-${backend}" "Verifying all backends are ready before starting scenario."
  for check_backend in even odd parity; do
    if ! wait_for_minio_backend_ready "${check_backend}" 2>/dev/null; then
      log_warn "scenario:rollback-disabled-update-fail-${backend}" "Backend '${check_backend}' may not be ready, but continuing."
    fi
  done

  purge_remote_root "${RAID3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"

  # Create a test file using the regular remote (rollback enabled by default)
  local dataset_id
  dataset_id=$(create_test_dataset "rollback-disabled-update-fail-${backend}") || {
    record_error_result "FAIL" "rollback-disabled-update-fail-${backend}" "Failed to create dataset."
    return 1
  }
  log_info "scenario:rollback-disabled-update-fail-${backend}" "Dataset ${dataset_id} created (will use connection string with rollback=false)."

  local test_file="${dataset_id}/${TARGET_OBJECT}"

  # Get original file content for verification (using connection string with rollback=false)
  local test_file_path
  test_file_path=$(get_rollback_disabled_path "${test_file}")

  local original_content
  original_content=$(mktemp) || {
    record_error_result "FAIL" "rollback-disabled-update-fail-${backend}" "Failed to create temp file for original content."
    return 1
  }

  local get_result get_status get_stdout get_stderr
  get_result=$(capture_command "get_original" cat "${test_file_path}")
  IFS='|' read -r get_status get_stdout get_stderr <<<"${get_result}"
  if [[ "${get_status}" -ne 0 ]]; then
    record_error_result "FAIL" "rollback-disabled-update-fail-${backend}" "Failed to read original file content."
    rm -f "${get_stdout}" "${get_stderr}" "${original_content}"
    return 1
  fi
  cp "${get_stdout}" "${original_content}"
  rm -f "${get_stdout}" "${get_stderr}"

  # Stop the backend to simulate unavailability
  log_info "scenario:rollback-disabled-update-fail-${backend}" "Stopping '${backend}' backend to simulate unavailability."
  stop_single_minio_container "${backend}"

  # Wait a moment for container to fully stop
  sleep 2

  # Create updated content
  local updated_content
  updated_content=$(mktemp) || {
    record_error_result "FAIL" "rollback-disabled-update-fail-${backend}" "Failed to create temp file for updated content."
    rm -f "${original_content}"
    return 1
  }
  echo "Updated content for rollback-disabled update-fail-${backend} test" > "${updated_content}"

  # Attempt update (using copy as update) with connection string rollback=false - should fail
  local update_result update_status update_stdout update_stderr
  update_result=$(capture_command "update_attempt" copyto "${updated_content}" "${test_file_path}")
  IFS='|' read -r update_status update_stdout update_stderr <<<"${update_result}"
  print_if_verbose "update attempt" "${update_stdout}" "${update_stderr}"

  # Restore backend
  log_info "scenario:rollback-disabled-update-fail-${backend}" "Restoring '${backend}' backend."
  start_single_minio_container "${backend}"

  # Wait for container port to be open
  local port
  case "${backend}" in
    even) port="${MINIO_EVEN_PORT}" ;;
    odd) port="${MINIO_ODD_PORT}" ;;
    parity) port="${MINIO_PARITY_PORT}" ;;
  esac
  if ! wait_for_minio_port "${port}"; then
    log_warn "scenario:rollback-disabled-update-fail-${backend}" "Backend port ${port} did not open in time."
  fi

  # Wait for MinIO to be fully ready (not just port open, but S3 API ready)
  if ! wait_for_minio_backend_ready "${backend}"; then
    log_warn "scenario:rollback-disabled-update-fail-${backend}" "Backend '${backend}' may not be fully ready, but continuing."
  else
    log_info "scenario:rollback-disabled-update-fail-${backend}" "Backend '${backend}' confirmed ready."
  fi

  # Verify update failed (non-zero exit status)
  if [[ "${update_status}" -eq 0 ]]; then
    # Backend was already restored earlier, but ensure it's ready for next scenario
    if wait_for_minio_port "${port}" 2>/dev/null; then
      wait_for_minio_backend_ready "${backend}" 2>/dev/null || true
    fi
    record_error_result "FAIL" "rollback-disabled-update-fail-${backend}" "Update succeeded when it should have failed (backend '${backend}' was unavailable)."
    rm -f "${update_stdout}" "${update_stderr}" "${original_content}" "${updated_content}"
    return 1
  fi

  # With rollback disabled, partial updates may occur
  # Verify file still exists and check if content changed using connection string
  local verify_result verify_status verify_stdout verify_stderr
  verify_result=$(capture_command "verify_original" cat "${test_file_path}")
  IFS='|' read -r verify_status verify_stdout verify_stderr <<<"${verify_result}"
  
  if [[ "${verify_status}" -ne 0 ]]; then
    record_error_result "FAIL" "rollback-disabled-update-fail-${backend}" "File missing after failed update."
    rm -f "${verify_stdout}" "${verify_stderr}" "${update_stdout}" "${update_stderr}" "${original_content}" "${updated_content}"
    return 1
  fi

  # Check if content changed (partial update may have occurred)
  if ! cmp -s "${original_content}" "${verify_stdout}"; then
    # Partial update occurred - this is expected with rollback disabled
    log_info "rollback-disabled" "Partial update detected: File content changed after failed update (expected with rollback=false)."
    record_error_result "PASS" "rollback-disabled-update-fail-${backend}" "Update correctly failed and partial update occurred (expected with rollback=false)."
    rm -f "${verify_stdout}" "${verify_stderr}" "${update_stdout}" "${update_stderr}" "${original_content}" "${updated_content}"
    return 0
  else
    # No partial update - content unchanged
    log_info "rollback-disabled" "No partial update detected (update failed before any changes)."
    record_error_result "PASS" "rollback-disabled-update-fail-${backend}" "Update correctly failed with no partial update (failed early)."
    rm -f "${verify_stdout}" "${verify_stderr}" "${update_stdout}" "${update_stderr}" "${original_content}" "${updated_content}"
    return 0
  fi
}

run_error_scenario() {
  local scenario="$1"
  case "${scenario}" in
    move-fail-even) run_move_fail_scenario "even" ;;
    move-fail-odd) run_move_fail_scenario "odd" ;;
    move-fail-parity) run_move_fail_scenario "parity" ;;
    update-fail-even) run_update_fail_scenario "even" ;;
    update-fail-odd) run_update_fail_scenario "odd" ;;
    update-fail-parity) run_update_fail_scenario "parity" ;;
    rollback-disabled-move-fail-even) run_move_fail_scenario_no_rollback "even" ;;
    rollback-disabled-move-fail-odd) run_move_fail_scenario_no_rollback "odd" ;;
    rollback-disabled-move-fail-parity) run_move_fail_scenario_no_rollback "parity" ;;
    rollback-disabled-update-fail-even) run_update_fail_scenario_no_rollback "even" ;;
    rollback-disabled-update-fail-odd) run_update_fail_scenario_no_rollback "odd" ;;
    rollback-disabled-update-fail-parity) run_update_fail_scenario_no_rollback "parity" ;;
    *)
      record_error_result "FAIL" "${scenario}" "Unknown scenario."
      return 1
      ;;
  esac
}

run_all_error_scenarios() {
  local scenarios=("move-fail-even" "move-fail-odd" "move-fail-parity" "update-fail-even" "update-fail-odd" "update-fail-parity")
  local name
  for name in "${scenarios[@]}"; do
    if ! run_error_scenario "${name}"; then
      return 1
    fi
  done
  return 0
}

main() {
  parse_args "$@"
  ensure_workdir
  ensure_rclone_config

  case "${COMMAND}" in
    start)
      if [[ "${STORAGE_TYPE}" != "minio" ]]; then
        log "'start' only applies to the MinIO storage type."
        exit 0
      fi
      start_minio_containers
      ;;
    stop)
      if [[ "${STORAGE_TYPE}" != "minio" ]]; then
        log "'stop' only applies to the MinIO storage type."
        exit 0
      fi
      stop_minio_containers
      ;;
    teardown)
      [[ "${STORAGE_TYPE}" != "minio" ]] || ensure_minio_containers_ready
      set_remotes_for_storage_type
      purge_remote_root "${RAID3_REMOTE}"
      purge_remote_root "${SINGLE_REMOTE}"
      if [[ "${STORAGE_TYPE}" == "local" ]]; then
        for dir in "${LOCAL_RAID3_DIRS[@]}" "${LOCAL_SINGLE_DIR}"; do
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
    list)
      print_scenarios
      ;;
    test)
      set_remotes_for_storage_type
      [[ "${STORAGE_TYPE}" != "minio" ]] || ensure_minio_containers_ready
      reset_error_results
      if [[ -z "${COMMAND_ARG}" ]]; then
        if ! run_all_error_scenarios; then
          print_error_summary
          die "One or more error scenarios failed."
        fi
      else
        if ! run_error_scenario "${COMMAND_ARG}"; then
          print_error_summary
          die "Scenario '${COMMAND_ARG}' failed."
        fi
      fi
      print_error_summary
      ;;
  esac
}

main "$@"

