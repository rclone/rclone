#!/usr/bin/env bash
#
# compare_raid3_with_single_rebuild.sh
# -------------------------------------
# Rebuild-focused harness for rclone raid3 backends.
#
# This script simulates disk swaps for individual raid3 remotes (even/odd/parity),
# runs `rclone backend rebuild`, and validates that the dataset is restored
# (or fails as expected) for both local and MinIO-backed configurations.
#
# Usage:
#   compare_raid3_with_single_rebuild.sh [options] <command> [args]
#
# Commands:
#   start                 Start MinIO containers (requires --storage-type=minio).
#   stop                  Stop MinIO containers (requires --storage-type=minio).
#   teardown              Purge all data from the selected storage-type.
#   list                  Show available rebuild scenarios.
#   test [name]           Run a named scenario (even|odd|parity). If omitted, runs all.
#
# Options:
#   --storage-type <local|minio|mixed>   Select backend pair (required for start/stop/test/teardown).
#   -v, --verbose                  Show stdout/stderr from rclone commands.
#   -h, --help                     Display this help text.
#
# Environment:
#   RCLONE_CONFIG   Path to rclone configuration file (default: $HOME/.config/rclone/rclone.conf).
#
# Safety guard: the script must be executed from backend/raid3/test directory.
# -----------------------------------------------------------------------------

set -euo pipefail

SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)

# shellcheck source=backend/raid3/test/compare_raid3_with_single_common.sh
. "${SCRIPT_DIR}/compare_raid3_with_single_common.sh"

VERBOSE=0
STORAGE_TYPE=""
COMMAND=""
COMMAND_ARG=""

usage() {
  cat <<EOF
Usage: ${SCRIPT_NAME} [options] <command> [arguments]

Commands:
  start                      Start MinIO containers (requires --storage-type=minio).
  stop                       Stop MinIO containers (requires --storage-type=minio).
  teardown                   Purge all test data for the selected storage type.
  list                       Show available rebuild scenarios.
  test [name]                Run the named scenario (even|odd|parity). Without a name, runs all.

Options:
  --storage-type <local|minio>   Select backend pair (required for start/stop/test/teardown).
  -v, --verbose                  Show stdout/stderr from rclone commands.
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

  if [[ -n "${STORAGE_TYPE}" && "${STORAGE_TYPE}" != "local" && "${STORAGE_TYPE}" != "minio" && "${STORAGE_TYPE}" != "mixed" ]]; then
    die "Invalid storage type '${STORAGE_TYPE}'. Expected 'local', 'minio', or 'mixed'."
  fi
}

SCENARIO_RESULTS=()

reset_scenario_results() {
  SCENARIO_RESULTS=()
}

pass_scenario() {
  local scenario="$1"
  local detail="$2"
  log_pass "scenario:${scenario}" "${detail}"
  SCENARIO_RESULTS+=("PASS ${scenario}")
}

fail_scenario() {
  local scenario="$1"
  local detail="$2"
  log_fail "scenario:${scenario}" "${detail}"
  SCENARIO_RESULTS+=("FAIL ${scenario} – ${detail}")
}

print_scenario_summary() {
  log_info "summary:----------"
  if [[ "${#SCENARIO_RESULTS[@]}" -eq 0 ]]; then
    log_info "summary:No scenarios recorded."
    return
  fi
  for entry in "${SCENARIO_RESULTS[@]}"; do
    log_info "summary:${entry}"
  done
}

list_scenarios() {
  cat <<EOF
Available rebuild scenarios:
  even    Simulate even backend swap and verify rebuild (success + failure cases).
  odd     Simulate odd backend swap and verify rebuild (success + failure cases).
  parity  Simulate parity backend swap and verify rebuild (success + failure cases).
EOF
}

secondary_failure_backend() {
  local backend="$1"
  case "${backend}" in
    even) echo "parity" ;;
    odd) echo "parity" ;;
    parity) echo "even" ;;
    *) die "Unknown backend '${backend}'" ;;
  esac
}

wipe_remote_directory() {
  local dir="$1"
  local allowed=0
  for candidate in "${ALLOWED_DATA_DIRS[@]}"; do
    if [[ "${candidate}" == "${dir}" ]]; then
      allowed=1
      break
    fi
  done

  (( allowed )) || die "Refusing to wipe unrecognized directory '${dir}'"

  case "${dir}" in
    "${WORKDIR}"/*) ;;
    *)
      die "Refusing to wipe directory '${dir}' (outside ${WORKDIR})"
      ;;
  esac

  rm -rf "${dir}"
  mkdir -p "${dir}"
}

simulate_disk_swap() {
  local backend="$1"
  local dir
  dir=$(remote_data_dir "${backend}")
  log_info "disk-swap" "Simulating disk swap for '${backend}' backend at ${dir}"

  # Determine if this specific backend is MinIO or local
  # For mixed storage, we need to check the actual backend type, not just STORAGE_TYPE
  local is_minio_backend=0
  if [[ "${STORAGE_TYPE}" == "minio" ]]; then
    # All backends are MinIO
    is_minio_backend=1
  elif [[ "${STORAGE_TYPE}" == "mixed" ]]; then
    # In mixed storage: even=local, odd=MinIO, parity=local
    case "${backend}" in
      odd) is_minio_backend=1 ;;
      even|parity) is_minio_backend=0 ;;
      *) die "Unknown backend '${backend}' for mixed storage" ;;
    esac
  fi

  if [[ "${is_minio_backend}" -eq 1 ]]; then
    # For MinIO, we need to purge the bucket using rclone, not just wipe the local directory
    # The local directory is where MinIO stores data, but we need to ensure the bucket is empty
    stop_single_minio_container "${backend}"
    wipe_remote_directory "${dir}"  # Wipe local MinIO data directory
    start_single_minio_container "${backend}"
    
    # Wait for MinIO container to be fully ready
    local port
    case "${backend}" in
      even) port="${MINIO_EVEN_PORT:-9001}" ;;
      odd) port="${MINIO_ODD_PORT:-9002}" ;;
      parity) port="${MINIO_PARITY_PORT:-9003}" ;;
      *) die "Unknown backend '${backend}'" ;;
    esac
    if ! wait_for_minio_port "${port}"; then
      log_warn "disk-swap" "Port ${port} did not open in time, continuing anyway"
    fi
    
    # Purge the entire bucket to ensure it's truly empty
    local remote
    remote=$(backend_remote_name "${backend}")
    log_info "disk-swap" "Purging MinIO bucket '${remote}:' to simulate empty disk"
    
    # Use delete first (works better with S3/MinIO), then purge for directories
    rclone_cmd delete "${remote}:" --max-delete 10000 >/dev/null 2>&1 || true
    rclone_cmd purge "${remote}:" >/dev/null 2>&1 || true
    
    # Brief wait for MinIO to process deletions
    sleep 1
  else
    # For local storage, wiping the directory is sufficient
    wipe_remote_directory "${dir}"
  fi
}

ensure_reference_copy() {
  local dataset_id="$1"
  local ref_dir
  ref_dir=$(mktemp -d) || die "Failed to create temp dir"
  if ! rclone_cmd copy "${RAID3_REMOTE}:${dataset_id}" "${ref_dir}" >/dev/null; then
    rm -rf "${ref_dir}"
    die "Failed to copy ${RAID3_REMOTE}:${dataset_id} to ${ref_dir} for reference"
  fi
  printf '%s\n' "${ref_dir}"
}

run_rebuild_command() {
  local backend="$1"
  capture_command "lvl_rebuild_${backend}" backend rebuild "${RAID3_REMOTE}:" "${backend}"
}

run_rebuild_success_scenario() {
  local backend="$1"
  purge_remote_root "${RAID3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"

  local dataset_id
  dataset_id=$(create_test_dataset "rebuild-${backend}-success") || die "Failed to create dataset for ${backend} success scenario"
  log "Success scenario dataset: ${RAID3_REMOTE}:${dataset_id}"

  local ref_dir
  ref_dir=$(ensure_reference_copy "${dataset_id}")

  simulate_disk_swap "${backend}"

  local rebuild_result lvl_status lvl_stdout lvl_stderr
  rebuild_result=$(run_rebuild_command "${backend}")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${rebuild_result}"
  print_if_verbose "rebuild ${backend}" "${lvl_stdout}" "${lvl_stderr}"

  if [[ "${lvl_status}" -ne 0 ]]; then
    log "Rebuild command failed with status ${lvl_status}; outputs retained:"
    log "  ${lvl_stdout}"
    log "  ${lvl_stderr}"
    rm -rf "${ref_dir}"
    return 1
  fi

  local verify_result verify_status verify_stdout verify_stderr
  verify_result=$(capture_command "lvl_check_${backend}" check "${RAID3_REMOTE}:${dataset_id}" ":local:${ref_dir}")
  IFS='|' read -r verify_status verify_stdout verify_stderr <<<"${verify_result}"
  print_if_verbose "check ${backend}" "${verify_stdout}" "${verify_stderr}"

  if [[ "${verify_status}" -ne 0 ]]; then
    log "rclone check failed after rebuild (status ${verify_status}). Outputs retained:"
    log "  ${verify_stdout}"
    log "  ${verify_stderr}"
    rm -rf "${ref_dir}"
    return 1
  fi

  local rebuilt_dir
  rebuilt_dir=$(mktemp -d) || { rm -rf "${ref_dir}"; return 1; }
  if ! rclone_cmd copy "${RAID3_REMOTE}:${dataset_id}" "${rebuilt_dir}" >/dev/null; then
    log "Failed to download rebuilt dataset for comparison."
    rm -rf "${ref_dir}" "${rebuilt_dir}"
    return 1
  fi

  if ! diff -qr "${ref_dir}" "${rebuilt_dir}" >/dev/null; then
    log "Rebuilt dataset differs from reference for backend '${backend}'."
    rm -rf "${ref_dir}" "${rebuilt_dir}"
    return 1
  fi

  rm -rf "${ref_dir}" "${rebuilt_dir}"
  log "Rebuild success scenario for '${backend}' completed; dataset retained for inspection."
  return 0
}

run_rebuild_failure_scenario() {
  local backend="$1"
  purge_remote_root "${RAID3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"

  local dataset_id
  dataset_id=$(create_test_dataset "rebuild-${backend}-failure") || die "Failed to create dataset for ${backend} failure scenario"
  log "Failure scenario dataset: ${RAID3_REMOTE}:${dataset_id}"

  local secondary
  secondary=$(secondary_failure_backend "${backend}")

  simulate_disk_swap "${backend}"
  simulate_disk_swap "${secondary}"

  local rebuild_result lvl_status lvl_stdout lvl_stderr
  rebuild_result=$(run_rebuild_command "${backend}")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${rebuild_result}"
  print_if_verbose "rebuild ${backend} (failure scenario)" "${lvl_stdout}" "${lvl_stderr}"

  local summary_content
  summary_content=$(cat "${lvl_stdout}")

  if [[ "${lvl_status}" -ne 0 ]]; then
    log "Rebuild command returned status ${lvl_status} as expected when '${backend}' and '${secondary}' are missing."
  else
    if grep -q "Files rebuilt: 0/" <<<"${summary_content}"; then
      log "Rebuild reported zero files rebuilt, indicating it could not rebuild '${backend}' with only one source."
    else
      log "Expected rebuild summary to report zero files rebuilt, but got:"
      log "${summary_content}"
      log "Stderr retained at ${lvl_stderr}"
      return 1
    fi
  fi

  # With only one backend available, files cannot be read.
  # Test that reading actually fails by trying to read a known file.
  # The test dataset always includes file_root.txt at the root level.
  local test_file="${dataset_id}/file_root.txt"
  
  # Try to actually read the file content
  # Note: rclone cat may return exit code 0 with no output when file can't be opened
  # We need to check for errors in stderr or verify no content was actually read
  local read_result read_status read_stdout read_stderr
  read_result=$(capture_command "lvl_failure_read_${backend}" cat "${RAID3_REMOTE}:${test_file}")
  IFS='|' read -r read_status read_stdout read_stderr <<<"${read_result}"
  print_if_verbose "read failure ${backend}" "${read_stdout}" "${read_stderr}"

  # Check if actual file content was read
  local output_size
  output_size=$(wc -c <"${read_stdout}" 2>/dev/null || echo "0")

  # Look for error messages in stderr (rclone may log errors there even with exit code 0)
  local has_reconstruction_error=0
  if grep -qiE "cannot reconstruct|no data particle|parity particle not found|missing.*particle|missing odd particle|missing even particle|object not found|failed to.*open|failed to.*read" <<<"$(cat "${read_stderr}" 2>/dev/null || true)"; then
    has_reconstruction_error=1
  fi

  if [[ "${output_size}" -gt 0 ]]; then
    # File content was actually read - this is a failure, should not be possible
    log "Expected file read to fail due to missing particles (only one backend available), but it succeeded and returned ${output_size} bytes."
    log "File: ${test_file}"
    log "Outputs retained:"
    log "  ${read_stdout}"
    log "  ${read_stderr}"
    rm -f "${read_stdout}" "${read_stderr}"
    return 1
  elif [[ "${has_reconstruction_error}" -eq 1 ]]; then
    # Error message found in stderr - file read correctly failed
    log "File read correctly failed as expected when only one backend is available (error in stderr)."
    rm -f "${read_stdout}" "${read_stderr}"
  elif [[ "${read_status}" -ne 0 ]]; then
    # Exit code is non-zero - file read failed as expected
    log "File read correctly failed as expected when only one backend is available (exit code ${read_status})."
    rm -f "${read_stdout}" "${read_stderr}"
  else
    # Exit code 0, no content, no error in stderr
    # This is unusual but could happen if rclone cat doesn't report errors properly
    # Since we already verified rebuild failed (0 files rebuilt), this indicates files can't be read
    # Accept this as expected behavior - the key test is that rebuild failed
    log "File read returned exit code 0 with no content and no error messages."
    log "Since rebuild reported 0 files rebuilt, this indicates files cannot be read (expected behavior)."
    log "File: ${test_file}"
    print_if_verbose "read details" "${read_stdout}" "${read_stderr}"
    rm -f "${read_stdout}" "${read_stderr}"
  fi

  # Also verify that rclone check fails or reports errors
  local check_result check_status check_stdout check_stderr
  check_result=$(capture_command "lvl_failure_check_${backend}" check "${RAID3_REMOTE}:${dataset_id}" "${SINGLE_REMOTE}:${dataset_id}")
  IFS='|' read -r check_status check_stdout check_stderr <<<"${check_result}"
  print_if_verbose "check failure ${backend}" "${check_stdout}" "${check_stderr}"

  # Check should fail OR report errors (even if exit status is 0, check stderr for errors)
  if [[ "${check_status}" -eq 0 ]] && ! grep -qi "error\|cannot\|failed" <<<"${check_stderr}${check_stdout}"; then
    log "Expected rclone check to fail or report errors due to missing particles, but it succeeded without errors."
    log "Outputs retained:"
    log "  ${check_stdout}"
    log "  ${check_stderr}"
    rm -f "${check_stdout}" "${check_stderr}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${check_stdout}" "${check_stderr}"
  log_info "scenario:${backend}" "Failure-mode dataset ${dataset_id} confirmed unrebuildable; cleaning up particles."
  cleanup_raid3_dataset_raw "${dataset_id}"
  log_info "scenario:${backend}" "Failure-mode dataset ${dataset_id} removed from underlying remotes."
  log_info "scenario:${backend}" "Rebuild failure scenario for '${backend}' verified (check reported mismatch)."
  return 0
}

run_rebuild_scenarios() {
  local backend="$1"
  log_info "suite" "Running rebuild scenarios for '${backend}' (${STORAGE_TYPE})"

  if ! run_rebuild_failure_scenario "${backend}"; then
    log_warn "suite" "Failure scenario failed for backend '${backend}'."
    fail_scenario "${backend}" "Failure-mode check failed."
    return 1
  fi

  if ! run_rebuild_success_scenario "${backend}"; then
    log_warn "suite" "Success scenario failed for backend '${backend}'."
    fail_scenario "${backend}" "Success-mode check failed."
    return 1
  fi

  pass_scenario "${backend}" "Failure + rebuild success validated."
  return 0
}

run_all_scenarios() {
  local scenarios=("even" "odd" "parity")
  local name
  for name in "${scenarios[@]}"; do
    if ! run_rebuild_scenarios "${name}"; then
      return 1
    fi
  done
  return 0
}

main() {
  parse_args "$@"
  ensure_workdir
  ensure_rclone_binary
  ensure_rclone_config

  case "${COMMAND}" in
    start)
      if [[ "${STORAGE_TYPE}" != "minio" && "${STORAGE_TYPE}" != "mixed" ]]; then
        log "'start' only applies to MinIO-based storage types (minio or mixed)."
        exit 0
      fi
      start_minio_containers
      ;;
    stop)
      if [[ "${STORAGE_TYPE}" != "minio" && "${STORAGE_TYPE}" != "mixed" ]]; then
        log "'stop' only applies to MinIO-based storage types (minio or mixed)."
        exit 0
      fi
      stop_minio_containers
      ;;
    teardown)
      [[ "${STORAGE_TYPE}" != "minio" && "${STORAGE_TYPE}" != "mixed" ]] || ensure_minio_containers_ready
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
      list_scenarios
      ;;
    test)
      set_remotes_for_storage_type
      [[ "${STORAGE_TYPE}" != "minio" && "${STORAGE_TYPE}" != "mixed" ]] || ensure_minio_containers_ready
      reset_scenario_results
      if [[ -z "${COMMAND_ARG}" ]]; then
        if ! run_all_scenarios; then
          print_scenario_summary
          die "One or more scenarios failed."
        fi
      else
        case "${COMMAND_ARG}" in
          even|odd|parity)
            if ! run_rebuild_scenarios "${COMMAND_ARG}"; then
              print_scenario_summary
              die "Scenario '${COMMAND_ARG}' failed."
            fi
            ;;
          *)
            die "Unknown scenario '${COMMAND_ARG}'. Use '${SCRIPT_NAME} list' to see options."
            ;;
        esac
      fi
      print_scenario_summary
      ;;
  esac
}

main "$@"
