#!/usr/bin/env bash
#
# compare_level3_with_single_errors.sh
# ------------------------------------
# Error handling validation harness for the rclone level3 backend.
#
# This script tests that write operations (Move, Update) properly fail when
# backends are unavailable, following RAID 3 strict write policy. Works with
# MinIO-backed level3 configurations, stopping containers to simulate backend
# unavailability.
#
# Usage:
#   compare_level3_with_single_errors.sh [options] <command> [args]
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
# Safety guard: must be executed from $HOME/go/level3storage.
# -----------------------------------------------------------------------------

set -euo pipefail

SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=backend/level3/tools/compare_level3_common.sh
. "${SCRIPT_DIR}/compare_level3_common.sh"

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

Environment:
  RCLONE_CONFIG                  Path to rclone.conf (default: ${RCLONE_CONFIG})

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
  move-fail-even     Stop even backend and verify Move fails.
  move-fail-odd      Stop odd backend and verify Move fails.
  move-fail-parity   Stop parity backend and verify Move fails.
  update-fail-even   Stop even backend and verify Update fails.
  update-fail-odd    Stop odd backend and verify Update fails.
  update-fail-parity Stop parity backend and verify Update fails.
EOF
}

ERROR_RESULTS=()

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

  purge_remote_root "${LEVEL3_REMOTE}"
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
  check_result=$(capture_command "check_before" ls "${LEVEL3_REMOTE}:${test_file}")
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
  move_result=$(capture_command "move_attempt" move "${LEVEL3_REMOTE}:${test_file}" "${LEVEL3_REMOTE}:${new_file}")
  IFS='|' read -r move_status move_stdout move_stderr <<<"${move_result}"
  print_if_verbose "move attempt" "${move_stdout}" "${move_stderr}"

  # Verify move failed (non-zero exit status) BEFORE restoring backend
  if [[ "${move_status}" -eq 0 ]]; then
    # Restore backend before returning error
    log_info "scenario:move-fail-${backend}" "Restoring '${backend}' backend before error exit."
    start_single_minio_container "${backend}"
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
  # This catches partial moves that create degraded files at new location
  
  # Verify original file still exists (move should not have partially succeeded)
  # Note: This check might fail if we need the down backend, but we check with available backends
  local check_result check_status check_stdout check_stderr
  check_result=$(capture_command "check_after" ls "${LEVEL3_REMOTE}:${test_file}")
  IFS='|' read -r check_status check_stdout check_stderr <<<"${check_result}"
  
  # Verify new file does NOT exist (move should have failed completely)
  # Note: level3 can list files in degraded mode (2/3 particles), so we check this carefully
  local new_check_result new_check_status new_check_stdout new_check_stderr
  new_check_result=$(capture_command "check_new" ls "${LEVEL3_REMOTE}:${new_file}")
  IFS='|' read -r new_check_status new_check_stdout new_check_stderr <<<"${new_check_result}"
  
  # Restore backend now (after checking state)
  log_info "scenario:move-fail-${backend}" "Restoring '${backend}' backend."
  start_single_minio_container "${backend}"

  # Wait for container to be ready (reuse port variable set earlier)
  if ! wait_for_minio_port "${port}"; then
    log_warn "scenario:move-fail-${backend}" "Backend '${backend}' may not be fully ready, but continuing."
  fi

  # Now verify the checks we did while backend was down
  if [[ "${check_status}" -ne 0 ]]; then
    record_error_result "FAIL" "move-fail-${backend}" "Original file disappeared after failed move (partial move occurred)."
    rm -f "${check_stdout}" "${check_stderr}" "${new_check_stdout}" "${new_check_stderr}" "${move_stdout}" "${move_stderr}"
    return 1
  fi

  if [[ "${new_check_status}" -eq 0 ]]; then
    # New file exists - this indicates partial move
    # This is a known limitation: Move operations don't rollback completed moves
    # See: backend/level3/docs/ERROR_HANDLING_POLICY.md line 182
    # "Limitation: Already-completed operations aren't undone!"
    #
    # The Move implementation should prevent moves when backend is unavailable
    # (via checkAllBackendsAvailable), but if a race condition occurs or the
    # check passes but backend fails during move, partial moves can occur.
    #
    # This test correctly detects this scenario. The move command failed (non-zero exit),
    # but some particles were already moved before the failure.
    
    local file_name=$(basename "${new_file}")
    local file_dir=$(dirname "${new_file}")
    local particles_moved=""
    
    # Check odd backend (if even was stopped)
    if [[ "${backend}" != "odd" ]]; then
      if object_exists_in_backend "odd" "${file_dir}" "${file_name}"; then
        particles_moved="${particles_moved} odd"
      fi
    fi
    
    # Check parity backend
    if [[ "${backend}" != "parity" ]]; then
      local parity_name
      # Parity files have suffixes, but we can check if any parity file exists
      # For now, just note that we detected the file
      particles_moved="${particles_moved} (parity-check-skipped)"
    fi
    
    log_warn "move" "⚠️  PARTIAL MOVE DETECTED: New file exists after failed move."
    log_warn "move" "This is a known limitation - Move operations don't rollback completed moves."
    log_warn "move" "See: backend/level3/docs/ERROR_HANDLING_POLICY.md (Rollback Strategy for Move)"
    log_note "move" "File detected at: ${new_file}"
    if [[ -n "${particles_moved}" ]]; then
      log_note "move" "Particles that appear to have moved:${particles_moved}"
    fi
    
    # Clean up the partially moved file
    log_info "cleanup" "Removing partially moved file at ${new_file}"
    rclone_cmd delete "${LEVEL3_REMOTE}:${new_file}" >/dev/null 2>&1 || true
    
    # For now, we document this as a known limitation rather than a hard failure
    # The test successfully detected the partial move scenario
    record_error_result "FAIL" "move-fail-${backend}" "New file exists after failed move (partial move occurred - known limitation: no rollback)."
    log_note "move" "Move command correctly returned non-zero exit code."
    log_note "move" "However, partial move occurred due to lack of rollback mechanism."
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

  purge_remote_root "${LEVEL3_REMOTE}"
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
  get_result=$(capture_command "get_original" cat "${LEVEL3_REMOTE}:${test_file}")
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
  update_result=$(capture_command "update_attempt" copyto "${updated_content}" "${LEVEL3_REMOTE}:${test_file}")
  IFS='|' read -r update_status update_stdout update_stderr <<<"${update_result}"
  print_if_verbose "update attempt" "${update_stdout}" "${update_stderr}"

  # Restore backend
  log_info "scenario:update-fail-${backend}" "Restoring '${backend}' backend."
  start_single_minio_container "${backend}"

  # Wait for container to be ready
  local port
  case "${backend}" in
    even) port="${MINIO_EVEN_PORT}" ;;
    odd) port="${MINIO_ODD_PORT}" ;;
    parity) port="${MINIO_PARITY_PORT}" ;;
  esac
  if ! wait_for_minio_port "${port}"; then
    log_warn "scenario:update-fail-${backend}" "Backend '${backend}' may not be fully ready, but continuing."
  fi

  # Verify update failed (non-zero exit status)
  if [[ "${update_status}" -eq 0 ]]; then
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
  verify_result=$(capture_command "verify_original" cat "${LEVEL3_REMOTE}:${test_file}")
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

run_error_scenario() {
  local scenario="$1"
  case "${scenario}" in
    move-fail-even) run_move_fail_scenario "even" ;;
    move-fail-odd) run_move_fail_scenario "odd" ;;
    move-fail-parity) run_move_fail_scenario "parity" ;;
    update-fail-even) run_update_fail_scenario "even" ;;
    update-fail-odd) run_update_fail_scenario "odd" ;;
    update-fail-parity) run_update_fail_scenario "parity" ;;
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
      purge_remote_root "${LEVEL3_REMOTE}"
      purge_remote_root "${SINGLE_REMOTE}"
      if [[ "${STORAGE_TYPE}" == "local" ]]; then
        for dir in "${LOCAL_LEVEL3_DIRS[@]}" "${LOCAL_SINGLE_DIR}"; do
          remove_leftover_files "${dir}"
          verify_directory_empty "${dir}"
        done
      else
        for dir in "${MINIO_LEVEL3_DIRS[@]}" "${MINIO_SINGLE_DIR}"; do
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

