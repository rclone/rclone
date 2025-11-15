#!/usr/bin/env bash
#
# compare_level3_with_single_recover.sh
# -------------------------------------
# Recovery-focused harness for rclone level3 backends.
#
# This script simulates disk swaps for individual level3 remotes (even/odd/parity),
# runs `rclone backend rebuild`, and validates that the dataset is restored
# (or fails as expected) for both local and MinIO-backed configurations.
#
# Usage:
#   compare_level3_with_single_recover.sh [options] <command> [args]
#
# Commands:
#   start                 Start MinIO containers (requires --storage-type=minio).
#   stop                  Stop MinIO containers (requires --storage-type=minio).
#   teardown              Purge all data from the selected storage-type.
#   list                  Show available recovery scenarios.
#   test [name]           Run a named scenario (even|odd|parity). If omitted, runs all.
#
# Options:
#   --storage-type <local|minio>   Select backend pair (required for start/stop/test/teardown).
#   -v, --verbose                  Show stdout/stderr from rclone commands.
#   -h, --help                     Display this help text.
#
# Environment:
#   RCLONE_CONFIG   Path to rclone configuration file (default: $HOME/.config/rclone/rclone.conf).
#
# Safety guard: the script must be executed from $HOME/go/level3storage.
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

usage() {
  cat <<EOF
Usage: ${SCRIPT_NAME} [options] <command> [arguments]

Commands:
  start                      Start MinIO containers (requires --storage-type=minio).
  stop                       Stop MinIO containers (requires --storage-type=minio).
  teardown                   Purge all test data for the selected storage type.
  list                       Show available recovery scenarios.
  test [name]                Run the named scenario (even|odd|parity). Without a name, runs all.

Options:
  --storage-type <local|minio>   Select backend pair (required for start/stop/test/teardown).
  -v, --verbose                  Show stdout/stderr from rclone commands.
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
Available recovery scenarios:
  even    Simulate even backend swap and verify rebuild (success + failure cases).
  odd     Simulate odd backend swap and verify rebuild (success + failure cases).
  parity  Simulate parity backend swap and verify rebuild (success + failure cases).
EOF
}

remote_data_dir() {
  local backend="$1"
  case "${STORAGE_TYPE}" in
    local)
      case "${backend}" in
        even) echo "${LOCAL_LEVEL3_DIRS[0]}" ;;
        odd) echo "${LOCAL_LEVEL3_DIRS[1]}" ;;
        parity) echo "${LOCAL_LEVEL3_DIRS[2]}" ;;
        *) die "Unknown backend '${backend}'" ;;
      esac
      ;;
    minio)
      case "${backend}" in
        even) echo "${MINIO_LEVEL3_DIRS[0]}" ;;
        odd) echo "${MINIO_LEVEL3_DIRS[1]}" ;;
        parity) echo "${MINIO_LEVEL3_DIRS[2]}" ;;
        *) die "Unknown backend '${backend}'" ;;
      esac
      ;;
    *)
      die "Unsupported storage type '${STORAGE_TYPE}'"
      ;;
  esac
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

  if [[ "${STORAGE_TYPE}" == "minio" ]]; then
    stop_single_minio_container "${backend}"
  fi

  wipe_remote_directory "${dir}"

  if [[ "${STORAGE_TYPE}" == "minio" ]]; then
    start_single_minio_container "${backend}"
  fi
}

ensure_reference_copy() {
  local dataset_id="$1"
  local ref_dir
  ref_dir=$(mktemp -d) || die "Failed to create temp dir"
  if ! rclone_cmd copy "${LEVEL3_REMOTE}:${dataset_id}" "${ref_dir}" >/dev/null; then
    rm -rf "${ref_dir}"
    die "Failed to copy ${LEVEL3_REMOTE}:${dataset_id} to ${ref_dir} for reference"
  fi
  printf '%s\n' "${ref_dir}"
}

run_rebuild_command() {
  local backend="$1"
  capture_command "lvl_rebuild_${backend}" backend rebuild "${LEVEL3_REMOTE}:" "${backend}"
}

run_rebuild_success_scenario() {
  local backend="$1"
  purge_remote_root "${LEVEL3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"

  local dataset_id
  dataset_id=$(create_test_dataset "rebuild-${backend}-success") || die "Failed to create dataset for ${backend} success scenario"
  log "Success scenario dataset: ${LEVEL3_REMOTE}:${dataset_id}"

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
  verify_result=$(capture_command "lvl_check_${backend}" check "${LEVEL3_REMOTE}:${dataset_id}" ":local:${ref_dir}")
  IFS='|' read -r verify_status verify_stdout verify_stderr <<<"${verify_result}"
  print_if_verbose "check ${backend}" "${verify_stdout}" "${verify_stderr}"

  if [[ "${verify_status}" -ne 0 ]]; then
    log "rclone check failed after rebuild (status ${verify_status}). Outputs retained:"
    log "  ${verify_stdout}"
    log "  ${verify_stderr}"
    rm -rf "${ref_dir}"
    return 1
  fi

  local recovered_dir
  recovered_dir=$(mktemp -d) || { rm -rf "${ref_dir}"; return 1; }
  if ! rclone_cmd copy "${LEVEL3_REMOTE}:${dataset_id}" "${recovered_dir}" >/dev/null; then
    log "Failed to download rebuilt dataset for comparison."
    rm -rf "${ref_dir}" "${recovered_dir}"
    return 1
  fi

  if ! diff -qr "${ref_dir}" "${recovered_dir}" >/dev/null; then
    log "Rebuilt dataset differs from reference for backend '${backend}'."
    rm -rf "${ref_dir}" "${recovered_dir}"
    return 1
  fi

  rm -rf "${ref_dir}" "${recovered_dir}"
  log "Rebuild success scenario for '${backend}' completed; dataset retained for inspection."
  return 0
}

run_rebuild_failure_scenario() {
  local backend="$1"
  purge_remote_root "${LEVEL3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"

  local dataset_id
  dataset_id=$(create_test_dataset "rebuild-${backend}-failure") || die "Failed to create dataset for ${backend} failure scenario"
  log "Failure scenario dataset: ${LEVEL3_REMOTE}:${dataset_id}"

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
      log "Rebuild reported zero files rebuilt, indicating it could not recover '${backend}' with only one source."
    else
      log "Expected rebuild summary to report zero files rebuilt, but got:"
      log "${summary_content}"
      log "Stderr retained at ${lvl_stderr}"
      return 1
    fi
  fi

  local check_result check_status check_stdout check_stderr
  check_result=$(capture_command "lvl_failure_check_${backend}" check "${LEVEL3_REMOTE}:${dataset_id}" "${SINGLE_REMOTE}:${dataset_id}")
  IFS='|' read -r check_status check_stdout check_stderr <<<"${check_result}"
  print_if_verbose "check failure ${backend}" "${check_stdout}" "${check_stderr}"

  if [[ "${check_status}" -eq 0 ]]; then
    log "Expected rclone check to fail due to missing particles, but it succeeded."
    log "Outputs retained:"
    log "  ${check_stdout}"
    log "  ${check_stderr}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${check_stdout}" "${check_stderr}"
  log_info "scenario:${backend}" "Failure-mode dataset ${dataset_id} confirmed unrecoverable; cleaning up particles."
  cleanup_level3_dataset_raw "${dataset_id}"
  log_info "scenario:${backend}" "Failure-mode dataset ${dataset_id} removed from underlying remotes."
  log_info "scenario:${backend}" "Rebuild failure scenario for '${backend}' verified (check reported mismatch)."
  return 0
}

run_rebuild_scenarios() {
  local backend="$1"
  log_info "suite" "Running recovery scenarios for '${backend}' (${STORAGE_TYPE})"

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
      list_scenarios
      ;;
    test)
      set_remotes_for_storage_type
      [[ "${STORAGE_TYPE}" != "minio" ]] || ensure_minio_containers_ready
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

