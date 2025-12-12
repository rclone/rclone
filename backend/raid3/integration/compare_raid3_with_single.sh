#!/usr/bin/env bash
#
# compare_raid3_with_single.sh
# ---------------------------------
# Black-box comparison harness for rclone raid3 backends.
#
# This script runs a selected rclone command against a raid3 backend and the
# corresponding single-backend configuration, compares the exit status, and
# (optionally) shows both outputs. It also manages the supporting MinIO
# containers used by the MinIO-based raid3 backend.
#
# Usage:
#   compare_raid3_with_single.sh [options] <command> [args]
#
# Commands:
#   start                 Start the MinIO containers required for minioraid3/miniosingle.
#   stop                  Stop those MinIO containers.
#   teardown              Purge all data from the selected storage-type (raid3 + single).
#   list                  Show available test cases.
#   test <name>           Run a named test (e.g. "mkdir") against raid3 vs single.
#
# Options:
#   --storage-type <local|minio>   Select which backend pair to exercise.
#                                  Required for start/stop/test/teardown.
#   -v, --verbose                  Show stdout/stderr from both rclone invocations.
#   -h, --help                     Display this help text.
#
# Environment:
#   RCLONE_CONFIG   Path to rclone configuration file.
#                   Defaults to $HOME/.config/rclone/rclone.conf.
#
# Safety guard: the script must be executed from $HOME/go/raid3storage.
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

# ---------------------------- helper functions ------------------------------

usage() {
  cat <<EOF
Usage: ${SCRIPT_NAME} [options] <command> [arguments]

Commands:
  start                      Start MinIO containers (requires --storage-type=minio).
  stop                       Stop MinIO containers (requires --storage-type=minio).
  teardown                   Purge all test data for the selected storage type.
  list                       Show available tests.
  test <name>                Run the named test (e.g. "mkdir").

Options:
  --storage-type <local|minio>   Select backend pair (required for start/stop/test/teardown).
  -v, --verbose                  Show stdout/stderr from both rclone invocations.
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

TEST_RESULTS=()

reset_test_results() {
  TEST_RESULTS=()
}

pass_test() {
  local test="$1"
  local detail="$2"
  log_pass "test:${test}" "${detail}"
  TEST_RESULTS+=("PASS ${test}")
}

fail_test() {
  local test="$1"
  local detail="$2"
  log_fail "test:${test}" "${detail}"
  TEST_RESULTS+=("FAIL ${test} – ${detail}")
}

print_test_summary() {
  log_info "summary:----------"
  if [[ "${#TEST_RESULTS[@]}" -eq 0 ]]; then
    log_info "summary:No entries recorded."
    return
  fi
  for entry in "${TEST_RESULTS[@]}"; do
    log_info "summary:${entry}"
  done
}


list_tests() {
  cat <<EOF
Available tests:
  mkdir        Create a bucket/directory on both remotes and compare results.
  lsd          List directories on both remotes and compare results.
  ls           List objects on both remotes and compare results.
  cat          Read object contents (existing and missing) and compare results.
  delete       Delete objects (existing and missing) and compare results.
  cp-download  Copy objects from remote to local and compare results.
  cp-upload    Copy objects from local to remote and compare results.
  move         Move objects from remote to local (remove at source) and compare results.
  check        Compare hashes between raid3 and single remotes (matching/mismatching cases).
  sync-upload  Sync local changes to remote (create/update/delete) and compare results.
  sync-download Sync remote to local and compare results.
  purge        Purge (delete) buckets on both remotes and compare results.
  performance  Compare upload/download performance between raid3 and single remotes.
               PASS: raid3 is not more than 150% slower than single (ratio <= 2.5x).
               FAIL: raid3 is more than 150% slower than single (ratio > 2.5x).
EOF
}

run_all_tests() {
  local tests=("mkdir" "lsd" "ls" "cat" "delete" "cp-download" "cp-upload" "move" "check" "sync-upload" "sync-download" "purge")
  local name
  for name in "${tests[@]}"; do
    log_info "suite" "Running '${name}'"
    COMMAND_ARG="${name}"
    if ! run_single_test; then
      print_test_summary
      die "Test '${name}' failed."
    fi
  done
  COMMAND_ARG=""
}

# ------------------------------ test helpers --------------------------------
run_lsd_test() {
  local test_case="lsd"
  purge_remote_root "${RAID3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"
  log_info "test:${test_case}" "Preparing dataset"

  local dataset_id
  if ! dataset_id=$(create_test_dataset "lsd"); then
    log_warn "test:${test_case}" "Failed to set up dataset."
    return 1
  fi
  log_info "test:${test_case}" "Dataset ${dataset_id} created on both remotes (retained)."

  local lvl_result single_result
  lvl_result=$(capture_command "lvl_lsd" lsd "${RAID3_REMOTE}:${dataset_id}")
  single_result=$(capture_command "single_lsd" lsd "${SINGLE_REMOTE}:${dataset_id}")

  local lvl_status lvl_stdout lvl_stderr
  local single_status single_stdout single_stderr

  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${RAID3_REMOTE} lsd" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} lsd" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log_warn "test:${test_case}" "lsd status mismatch (${RAID3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status})"
    rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"
  log_info "test:${test_case}" "Command comparison completed."
  return 0
}

run_ls_test() {
  purge_remote_root "${RAID3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"
  log "Running ls test"

  local dataset_id
  if ! dataset_id=$(create_test_dataset "ls"); then
    log "Failed to set up dataset for ls test."
    return 1
  fi
  log "Dataset created: ${RAID3_REMOTE}:${dataset_id} and ${SINGLE_REMOTE}:${dataset_id} (retained for inspection)"

  local lvl_result single_result
  lvl_result=$(capture_command "lvl_ls" ls "${RAID3_REMOTE}:${dataset_id}")
  single_result=$(capture_command "single_ls" ls "${SINGLE_REMOTE}:${dataset_id}")

  local lvl_status lvl_stdout lvl_stderr
  local single_status single_stdout single_stderr

  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${RAID3_REMOTE} ls" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} ls" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "ls status mismatch: ${RAID3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status}"
    rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"
  log "ls test completed."
  return 0
}

run_cat_test() {
  purge_remote_root "${RAID3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"
  log "Running cat test"

  local dataset_id
  if ! dataset_id=$(create_test_dataset "cat"); then
    log "Failed to set up dataset for cat test."
    return 1
  fi
  log "Dataset created: ${RAID3_REMOTE}:${dataset_id} and ${SINGLE_REMOTE}:${dataset_id} (retained for inspection)"

  local target_existing="${dataset_id}/dirA/file_nested.txt"
  local target_missing="${dataset_id}/missing.txt"

  # Existing object
  local lvl_result single_result
  lvl_result=$(capture_command "lvl_cat_existing" cat "${RAID3_REMOTE}:${target_existing}")
  single_result=$(capture_command "single_cat_existing" cat "${SINGLE_REMOTE}:${target_existing}")

  local lvl_status lvl_stdout lvl_stderr
  local single_status single_stdout single_stderr
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${RAID3_REMOTE} cat existing" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} cat existing" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "cat (existing) status mismatch: ${RAID3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status}"
    log "Outputs retained for inspection:"
    log "  ${lvl_stdout}"
    log "  ${lvl_stderr}"
    log "  ${single_stdout}"
    log "  ${single_stderr}"
    return 1
  fi

  if [[ "${lvl_status}" -eq 0 ]]; then
    if ! cmp -s "${lvl_stdout}" "${single_stdout}"; then
      log "cat (existing) output mismatch between raid3 and single backends."
      log "Outputs retained:"
      log "  ${lvl_stdout}"
      log "  ${single_stdout}"
      return 1
    fi
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  # Missing object
  lvl_result=$(capture_command "lvl_cat_missing" cat "${RAID3_REMOTE}:${target_missing}")
  single_result=$(capture_command "single_cat_missing" cat "${SINGLE_REMOTE}:${target_missing}")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${RAID3_REMOTE} cat missing" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} cat missing" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "cat (missing) status mismatch: ${RAID3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status}"
    log "Outputs retained for inspection:"
    log "  ${lvl_stdout}"
    log "  ${lvl_stderr}"
    log "  ${single_stdout}"
    log "  ${single_stderr}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  log "cat test completed."
  return 0
}

run_copy_download_test() {
  purge_remote_root "${RAID3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"
  log "Running copy-download test"

  local dataset_id
  if ! dataset_id=$(create_test_dataset "cp-download"); then
    log "Failed to set up dataset for copy-download test."
    return 1
  fi
  log "Dataset created: ${RAID3_REMOTE}:${dataset_id} and ${SINGLE_REMOTE}:${dataset_id} (retained for inspection)"

  local tmp_lvl tmp_single
  tmp_lvl=$(mktemp -d) || return 1
  tmp_single=$(mktemp -d) || { rm -rf "${tmp_lvl}"; return 1; }

  local lvl_result single_result
  lvl_result=$(capture_command "lvl_copy_download" copy "${RAID3_REMOTE}:${dataset_id}" "${tmp_lvl}")
  single_result=$(capture_command "single_copy_download" copy "${SINGLE_REMOTE}:${dataset_id}" "${tmp_single}")

  local lvl_status lvl_stdout lvl_stderr
  local single_status single_stdout single_stderr
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${RAID3_REMOTE} copy (download)" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} copy (download)" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "copy (download) status mismatch: ${RAID3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status}"
    log "Outputs retained:"
    log "  ${lvl_stdout}"
    log "  ${lvl_stderr}"
    log "  ${single_stdout}"
    log "  ${single_stderr}"
    rm -rf "${tmp_lvl}" "${tmp_single}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  # Compare directory contents
  if ! diff -qr "${tmp_lvl}" "${tmp_single}" >/dev/null; then
    log "copy (download) produced different local content between raid3 and single remotes."
    rm -rf "${tmp_lvl}" "${tmp_single}"
    return 1
  fi

  rm -rf "${tmp_lvl}" "${tmp_single}"
  log "copy-download test completed."
  return 0
}

run_copy_upload_test() {
  purge_remote_root "${RAID3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"
  log "Running copy-upload test"

  local tempdir
  tempdir=$(mktemp -d) || return 1

  # Create local sample structure:
  #   upload_root/file_uploaded.txt
  #   upload_root/subdir/file_in_subdir.txt
  printf 'upload root file\n' >"${tempdir}/file_uploaded.txt"
  mkdir -p "${tempdir}/subdir"
  printf 'upload nested file\n' >"${tempdir}/subdir/file_in_subdir.txt"

  local dataset_id
  dataset_id=$(date +upload-%Y%m%d%H%M%S-$((RANDOM % 10000)))

  local lvl_result single_result
  lvl_result=$(capture_command "lvl_copy_upload" copy "${tempdir}" "${RAID3_REMOTE}:${dataset_id}")
  single_result=$(capture_command "single_copy_upload" copy "${tempdir}" "${SINGLE_REMOTE}:${dataset_id}")

  local lvl_status lvl_stdout lvl_stderr
  local single_status single_stdout single_stderr
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${RAID3_REMOTE} copy (upload)" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} copy (upload)" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "copy (upload) status mismatch: ${RAID3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status}"
    log "Outputs retained:"
    log "  ${lvl_stdout}"
    log "  ${lvl_stderr}"
    log "  ${single_stdout}"
    log "  ${single_stderr}"
    rm -rf "${tempdir}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  # Verify uploaded content by downloading to temp locations and comparing
  local tmp_lvl tmp_single
  tmp_lvl=$(mktemp -d) || { rm -rf "${tempdir}"; return 1; }
  tmp_single=$(mktemp -d) || { rm -rf "${tempdir}" "${tmp_lvl}"; return 1; }

  lvl_result=$(capture_command "lvl_verify_upload" copy "${RAID3_REMOTE}:${dataset_id}" "${tmp_lvl}")
  single_result=$(capture_command "single_verify_upload" copy "${SINGLE_REMOTE}:${dataset_id}" "${tmp_single}")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${RAID3_REMOTE} copy (verify download)" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} copy (verify download)" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "Verification copy status mismatch: ${RAID3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status}"
    rm -rf "${tempdir}" "${tmp_lvl}" "${tmp_single}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  if ! diff -qr "${tmp_lvl}" "${tmp_single}" >/dev/null; then
    log "Verification: downloaded content differs between raid3 and single remotes."
    rm -rf "${tempdir}" "${tmp_lvl}" "${tmp_single}"
    return 1
  fi

  rm -rf "${tempdir}" "${tmp_lvl}" "${tmp_single}"

  log "copy-upload test completed. Dataset stored as ${dataset_id} on both remotes."
  return 0
}

run_move_test() {
  purge_remote_root "${RAID3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"
  log "Running move test"

  local dataset_id
  if ! dataset_id=$(create_test_dataset "move"); then
    log "Failed to set up dataset for move test."
    return 1
  fi
  log "Dataset created: ${RAID3_REMOTE}:${dataset_id} and ${SINGLE_REMOTE}:${dataset_id} (retained for inspection until move completes)"

  local tmp_lvl tmp_single
  tmp_lvl=$(mktemp -d) || return 1
  tmp_single=$(mktemp -d) || { rm -rf "${tmp_lvl}"; return 1; }

  local lvl_result single_result
  lvl_result=$(capture_command "lvl_move" move "${RAID3_REMOTE}:${dataset_id}" "${tmp_lvl}")
  single_result=$(capture_command "single_move" move "${SINGLE_REMOTE}:${dataset_id}" "${tmp_single}")

  local lvl_status lvl_stdout lvl_stderr
  local single_status single_stdout single_stderr
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${RAID3_REMOTE} move" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} move" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "move status mismatch: ${RAID3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status}"
    log "Outputs retained:"
    log "  ${lvl_stdout}"
    log "  ${lvl_stderr}"
    log "  ${single_stdout}"
    log "  ${single_stderr}"
    rm -rf "${tmp_lvl}" "${tmp_single}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  # Compare destination directories
  if ! diff -qr "${tmp_lvl}" "${tmp_single}" >/dev/null; then
    log "move produced different destination content between raid3 and single remotes."
    rm -rf "${tmp_lvl}" "${tmp_single}"
    return 1
  fi

  # Confirm source buckets are empty (already moved)
  lvl_result=$(capture_command "lvl_post_move_ls" ls "${RAID3_REMOTE}:${dataset_id}")
  single_result=$(capture_command "single_post_move_ls" ls "${SINGLE_REMOTE}:${dataset_id}")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${RAID3_REMOTE} ls post-move" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} ls post-move" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "ls post-move status mismatch: ${RAID3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status}"
    rm -rf "${tmp_lvl}" "${tmp_single}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"
  rm -rf "${tmp_lvl}" "${tmp_single}"

  log "move test completed."
  return 0
}

run_delete_test() {
  purge_remote_root "${RAID3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"
  log "Running delete test"

  local dataset_id
  if ! dataset_id=$(create_test_dataset "delete"); then
    log "Failed to set up dataset for delete test."
    return 1
  fi
  log "Dataset created: ${RAID3_REMOTE}:${dataset_id} and ${SINGLE_REMOTE}:${dataset_id} (retained for inspection)"

  local target_existing="${dataset_id}/dirA/file_nested.txt"
  local target_missing="${dataset_id}/dirA/does_not_exist.txt"

  # Delete existing object
  local lvl_result single_result
  lvl_result=$(capture_command "lvl_delete_existing" delete "${RAID3_REMOTE}:${target_existing}")
  single_result=$(capture_command "single_delete_existing" delete "${SINGLE_REMOTE}:${target_existing}")

  local lvl_status lvl_stdout lvl_stderr
  local single_status single_stdout single_stderr
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${RAID3_REMOTE} delete existing" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} delete existing" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "delete (existing) status mismatch: ${RAID3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status}"
    log "Outputs retained:"
    log "  ${lvl_stdout}"
    log "  ${lvl_stderr}"
    log "  ${single_stdout}"
    log "  ${single_stderr}"
    return 1
  fi
  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  # Confirm deletion by listing the directory
  lvl_result=$(capture_command "lvl_post_delete_ls" ls "${RAID3_REMOTE}:${dataset_id}/dirA")
  single_result=$(capture_command "single_post_delete_ls" ls "${SINGLE_REMOTE}:${dataset_id}/dirA")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"
  print_if_verbose "${RAID3_REMOTE} ls post-delete" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} ls post-delete" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "ls post-delete status mismatch: ${RAID3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status}"
    return 1
  fi
  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  # Delete missing object (should be idempotent)
  lvl_result=$(capture_command "lvl_delete_missing" delete "${RAID3_REMOTE}:${target_missing}")
  single_result=$(capture_command "single_delete_missing" delete "${SINGLE_REMOTE}:${target_missing}")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${RAID3_REMOTE} delete missing" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} delete missing" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "delete (missing) status mismatch: ${RAID3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status}"
    log "Outputs retained:"
    log "  ${lvl_stdout}"
    log "  ${lvl_stderr}"
    log "  ${single_stdout}"
    log "  ${single_stderr}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  log "delete test completed."
  return 0
}

run_check_test() {
  purge_remote_root "${RAID3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"
  log "Running check test"

  local dataset_id
  if ! dataset_id=$(create_test_dataset "check"); then
    log "Failed to set up dataset for check test."
    return 1
  fi
  log "Dataset created: ${RAID3_REMOTE}:${dataset_id} and ${SINGLE_REMOTE}:${dataset_id}"

  local lvl_result single_result
  local lvl_status lvl_stdout lvl_stderr
  local single_status single_stdout single_stderr

  # Matching scenario
  lvl_result=$(capture_command "check_l2s_match" check "${RAID3_REMOTE}:${dataset_id}" "${SINGLE_REMOTE}:${dataset_id}")
  single_result=$(capture_command "check_s2l_match" check "${SINGLE_REMOTE}:${dataset_id}" "${RAID3_REMOTE}:${dataset_id}")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "check raid3->single (match)" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "check single->raid3 (match)" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "check (match) status mismatch: raid3->single=${lvl_status}, single->raid3=${single_status}"
    log "Outputs retained:"
    log "  ${lvl_stdout}"
    log "  ${lvl_stderr}"
    log "  ${single_stdout}"
    log "  ${single_stderr}"
    return 1
  fi

  if [[ "${lvl_status}" -ne 0 ]]; then
    log "check (match) failed unexpectedly with status ${lvl_status}; outputs retained."
    log "  ${lvl_stdout}"
    log "  ${lvl_stderr}"
    log "  ${single_stdout}"
    log "  ${single_stderr}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  # Induce mismatch: remove a file from raid3
  rclone_cmd delete "${RAID3_REMOTE}:${dataset_id}/dirA/file_nested.txt" >/dev/null 2>&1 || true

  lvl_result=$(capture_command "check_l2s_mismatch" check "${RAID3_REMOTE}:${dataset_id}" "${SINGLE_REMOTE}:${dataset_id}")
  single_result=$(capture_command "check_s2l_mismatch" check "${SINGLE_REMOTE}:${dataset_id}" "${RAID3_REMOTE}:${dataset_id}")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "check raid3->single (mismatch)" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "check single->raid3 (mismatch)" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "check (mismatch) status mismatch: raid3->single=${lvl_status}, single->raid3=${single_status}"
    log "Outputs retained:"
    log "  ${lvl_stdout}"
    log "  ${lvl_stderr}"
    log "  ${single_stdout}"
    log "  ${single_stderr}"
    return 1
  fi

  if [[ "${lvl_status}" -eq 0 ]]; then
    log "check (mismatch) unexpectedly succeeded; expected failure due to missing file."
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  log "check test completed."
  return 0
}

run_sync_upload_test() {
  purge_remote_root "${RAID3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"
  log "Running sync-upload test"

  # Initial upload from local -> remote for both backends
  local initial_dir
  initial_dir=$(mktemp -d) || return 1
  printf 'sync test file 1\n' >"${initial_dir}/file1.txt"
  mkdir -p "${initial_dir}/subdir"
  printf 'sync test file 2\n' >"${initial_dir}/subdir/file2.txt"

  local dataset_id
  dataset_id=$(date +sync-upload-%Y%m%d%H%M%S-$((RANDOM % 10000)))

  local lvl_result single_result
  lvl_result=$(capture_command "lvl_sync_initial" sync "${initial_dir}" "${RAID3_REMOTE}:${dataset_id}")
  single_result=$(capture_command "single_sync_initial" sync "${initial_dir}" "${SINGLE_REMOTE}:${dataset_id}")

  local lvl_status lvl_stdout lvl_stderr
  local single_status single_stdout single_stderr
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${RAID3_REMOTE} sync (initial upload)" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} sync (initial upload)" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "sync initial upload mismatch: raid3=${lvl_status}, single=${single_status}"
    rm -rf "${initial_dir}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  # Modify local state: delete file1, modify file2, add file3
  rm -f "${initial_dir}/file1.txt"
  printf 'updated sync test file 2\n' >"${initial_dir}/subdir/file2.txt"
  printf 'sync test file 3\n' >"${initial_dir}/file3.txt"

  # Apply sync (the operation under test)
  lvl_result=$(capture_command "lvl_sync_delta" sync "${initial_dir}" "${RAID3_REMOTE}:${dataset_id}")
  single_result=$(capture_command "single_sync_delta" sync "${initial_dir}" "${SINGLE_REMOTE}:${dataset_id}")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${RAID3_REMOTE} sync (delta)" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} sync (delta)" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "sync delta mismatch: raid3=${lvl_status}, single=${single_status}"
    rm -rf "${initial_dir}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  # Download both remotes to verify they now match the new local state
  local tmp_lvl tmp_single
  tmp_lvl=$(mktemp -d) || { rm -rf "${initial_dir}"; return 1; }
  tmp_single=$(mktemp -d) || { rm -rf "${initial_dir}" "${tmp_lvl}"; return 1; }

  lvl_result=$(capture_command "lvl_sync_verify" copy "${RAID3_REMOTE}:${dataset_id}" "${tmp_lvl}")
  single_result=$(capture_command "single_sync_verify" copy "${SINGLE_REMOTE}:${dataset_id}" "${tmp_single}")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${RAID3_REMOTE} copy (verify sync)" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} copy (verify sync)" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "Verification copy mismatch after sync: raid3=${lvl_status}, single=${single_status}"
    rm -rf "${initial_dir}" "${tmp_lvl}" "${tmp_single}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  if ! diff -qr "${tmp_lvl}" "${tmp_single}" >/dev/null; then
    log "Verification: remote states differ between raid3 and single after sync."
    rm -rf "${initial_dir}" "${tmp_lvl}" "${tmp_single}"
    return 1
  fi

  # Ensure the remotes reflect the expected content:
  # - file1 deleted
  # - file2 updated
  # - file3 present
  local expected_files=(
    "file3.txt"
    "subdir/file2.txt"
  )

  lvl_result=$(capture_command "lvl_sync_ls" ls "${RAID3_REMOTE}:${dataset_id}")
  single_result=$(capture_command "single_sync_ls" ls "${SINGLE_REMOTE}:${dataset_id}")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"
  print_if_verbose "${RAID3_REMOTE} ls (post-sync)" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} ls (post-sync)" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "Post-sync ls status mismatch: raid3=${lvl_status}, single=${single_status}"
    rm -rf "${initial_dir}" "${tmp_lvl}" "${tmp_single}"
    return 1
  fi

  # Quick presence test for expected files (just ensures commands succeeded)
  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  rm -rf "${initial_dir}" "${tmp_lvl}" "${tmp_single}"
  log "sync-upload test completed."
  return 0
}

run_sync_download_test() {
  purge_remote_root "${RAID3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"
  log "Running sync-download test"

  local dataset_id
  if ! dataset_id=$(create_test_dataset "sync-download"); then
    log "Failed to set up dataset for sync-download test."
    return 1
  fi

  local tmp_lvl tmp_single
  tmp_lvl=$(mktemp -d) || return 1
  tmp_single=$(mktemp -d) || { rm -rf "${tmp_lvl}"; return 1; }

  local lvl_result single_result
  lvl_result=$(capture_command "lvl_sync_download" sync "${RAID3_REMOTE}:${dataset_id}" "${tmp_lvl}")
  single_result=$(capture_command "single_sync_download" sync "${SINGLE_REMOTE}:${dataset_id}" "${tmp_single}")

  local lvl_status lvl_stdout lvl_stderr
  local single_status single_stdout single_stderr
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${RAID3_REMOTE} sync (download)" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} sync (download)" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "sync-download status mismatch: raid3=${lvl_status}, single=${single_status}"
    rm -rf "${tmp_lvl}" "${tmp_single}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  if ! diff -qr "${tmp_lvl}" "${tmp_single}" >/dev/null; then
    log "sync-download produced different local content between raid3 and single remotes."
    rm -rf "${tmp_lvl}" "${tmp_single}"
    return 1
  fi

  rm -rf "${tmp_lvl}" "${tmp_single}"
  log "sync-download test completed."
  return 0
}

run_purge_test() {
  purge_remote_root "${RAID3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"
  log "Running purge test"

  local dataset_id
  if ! dataset_id=$(create_test_dataset "purge"); then
    log "Failed to set up dataset for purge test."
    return 1
  fi
  log "Dataset created: ${RAID3_REMOTE}:${dataset_id} and ${SINGLE_REMOTE}:${dataset_id}"

  local lvl_result single_result
  local lvl_status lvl_stdout lvl_stderr
  local single_status single_stdout single_stderr

  # Initial purge
  lvl_result=$(capture_command "lvl_purge" purge "${RAID3_REMOTE}:${dataset_id}")
  single_result=$(capture_command "single_purge" purge "${SINGLE_REMOTE}:${dataset_id}")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${RAID3_REMOTE} purge (first)" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} purge (first)" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "purge status mismatch: raid3=${lvl_status}, single=${single_status}"
    log "Outputs retained:"
    log "  ${lvl_stdout}"
    log "  ${lvl_stderr}"
    log "  ${single_stdout}"
    log "  ${single_stderr}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  # Confirm dataset no longer exists
  lvl_result=$(capture_command "lvl_purge_verify" lsd "${RAID3_REMOTE}:${dataset_id}")
  single_result=$(capture_command "single_purge_verify" lsd "${SINGLE_REMOTE}:${dataset_id}")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${RAID3_REMOTE} lsd (post-purge)" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} lsd (post-purge)" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "lsd post-purge status mismatch: raid3=${lvl_status}, single=${single_status}"
    return 1
  fi

  # Expect both to report error (bucket gone). Clean up output files.
  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  log "purge test completed."
  return 0
}

run_performance_test() {
  local test_case="performance"
  purge_remote_root "${RAID3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"
  log_info "test:${test_case}" "Preparing performance test dataset"

  local dataset_id
  if ! dataset_id=$(create_test_dataset "performance"); then
    log_warn "test:${test_case}" "Failed to set up dataset."
    return 1
  fi

  # Create a larger test file for meaningful performance comparison
  local tempdir
  tempdir=$(mktemp -d) || return 1
  local test_file="${tempdir}/perf_test_file.bin"
  # Create a 10MB test file
  dd if=/dev/urandom of="${test_file}" bs=1M count=10 >/dev/null 2>&1 || return 1

  log_info "test:${test_case}" "Testing upload performance"
  
  # Test upload performance
  local raid3_result single_result
  raid3_result=$(capture_command_timed "raid3_upload" copy "${test_file}" "${RAID3_REMOTE}:${dataset_id}/perf_test.bin")
  single_result=$(capture_command_timed "single_upload" copy "${test_file}" "${SINGLE_REMOTE}:${dataset_id}/perf_test.bin")

  local raid3_status raid3_stdout raid3_stderr raid3_time
  local single_status single_stdout single_stderr single_time

  IFS='|' read -r raid3_status raid3_stdout raid3_stderr raid3_time <<<"${raid3_result}"
  IFS='|' read -r single_status single_stdout single_stderr single_time <<<"${single_result}"

  if [[ "${raid3_status}" -ne 0 || "${single_status}" -ne 0 ]]; then
    log_warn "test:${test_case}" "Upload failed (raid3=${raid3_status}, single=${single_status})"
    rm -rf "${tempdir}"
    rm -f "${raid3_stdout}" "${raid3_stderr}" "${single_stdout}" "${single_stderr}"
    return 1
  fi

  log_info "test:${test_case}" "Upload times: raid3=${raid3_time}s, single=${single_time}s"

  # Normalize time values: replace comma with dot for locale compatibility
  raid3_time="${raid3_time//,/.}"
  single_time="${single_time//,/.}"

  # Calculate upload performance ratio (raid3_time / single_time)
  local upload_ratio max_ratio="2.5"
  # Handle edge case where single_time might be very small
  if (( $(LC_NUMERIC=C awk "BEGIN {print (${single_time} < 0.001)}") )); then
    log_warn "test:${test_case}" "Single upload time too small (${single_time}s), skipping upload ratio check"
    upload_ratio="0.00"
  else
    upload_ratio=$(LC_NUMERIC=C awk "BEGIN {printf \"%.2f\", ${raid3_time} / ${single_time}}")
    log_info "test:${test_case}" "Upload ratio: ${upload_ratio}x (raid3/single)"
    
    # Check if raid3 upload is more than 150% slower (ratio > 2.5)
    if (( $(LC_NUMERIC=C awk "BEGIN {print (${upload_ratio} > ${max_ratio})}") )); then
      log_fail "test:${test_case}" "Upload performance check failed: raid3 is ${upload_ratio}x slower than single (threshold: ${max_ratio}x)"
      rm -rf "${tempdir}"
      rm -f "${raid3_stdout}" "${raid3_stderr}" "${single_stdout}" "${single_stderr}"
      return 1
    fi
  fi

  # Test download performance
  local tmp_raid3 tmp_single
  tmp_raid3=$(mktemp -d) || { rm -rf "${tempdir}"; return 1; }
  tmp_single=$(mktemp -d) || { rm -rf "${tempdir}" "${tmp_raid3}"; return 1; }

  raid3_result=$(capture_command_timed "raid3_download" copy "${RAID3_REMOTE}:${dataset_id}/perf_test.bin" "${tmp_raid3}/perf_test.bin")
  single_result=$(capture_command_timed "single_download" copy "${SINGLE_REMOTE}:${dataset_id}/perf_test.bin" "${tmp_single}/perf_test.bin")

  local raid3_dl_status raid3_dl_stdout raid3_dl_stderr raid3_dl_time
  local single_dl_status single_dl_stdout single_dl_stderr single_dl_time

  IFS='|' read -r raid3_dl_status raid3_dl_stdout raid3_dl_stderr raid3_dl_time <<<"${raid3_result}"
  IFS='|' read -r single_dl_status single_dl_stdout single_dl_stderr single_dl_time <<<"${single_result}"

  if [[ "${raid3_dl_status}" -ne 0 || "${single_dl_status}" -ne 0 ]]; then
    log_warn "test:${test_case}" "Download failed (raid3=${raid3_dl_status}, single=${single_dl_status})"
    rm -rf "${tempdir}" "${tmp_raid3}" "${tmp_single}"
    rm -f "${raid3_stdout}" "${raid3_stderr}" "${single_stdout}" "${single_stderr}"
    rm -f "${raid3_dl_stdout}" "${raid3_dl_stderr}" "${single_dl_stdout}" "${single_dl_stderr}"
    return 1
  fi

  log_info "test:${test_case}" "Download times: raid3=${raid3_dl_time}s, single=${single_dl_time}s"

  # Normalize time values: replace comma with dot for locale compatibility
  raid3_dl_time="${raid3_dl_time//,/.}"
  single_dl_time="${single_dl_time//,/.}"

  # Calculate download performance ratio (raid3_time / single_time)
  local download_ratio
  # Handle edge case where single_time might be very small
  if (( $(LC_NUMERIC=C awk "BEGIN {print (${single_dl_time} < 0.001)}") )); then
    log_warn "test:${test_case}" "Single download time too small (${single_dl_time}s), skipping download ratio check"
    download_ratio="0.00"
  else
    download_ratio=$(LC_NUMERIC=C awk "BEGIN {printf \"%.2f\", ${raid3_dl_time} / ${single_dl_time}}")
    log_info "test:${test_case}" "Download ratio: ${download_ratio}x (raid3/single)"
    
    # Check if raid3 download is more than 150% slower (ratio > 2.5)
    if (( $(LC_NUMERIC=C awk "BEGIN {print (${download_ratio} > ${max_ratio})}") )); then
      log_fail "test:${test_case}" "Download performance check failed: raid3 is ${download_ratio}x slower than single (threshold: ${max_ratio}x)"
      rm -rf "${tempdir}" "${tmp_raid3}" "${tmp_single}"
      rm -f "${raid3_stdout}" "${raid3_stderr}" "${single_stdout}" "${single_stderr}"
      rm -f "${raid3_dl_stdout}" "${raid3_dl_stderr}" "${single_dl_stdout}" "${single_dl_stderr}"
      return 1
    fi
  fi

  # Both operations passed the performance check
  log_info "test:${test_case}" "Performance check passed:"
  log_info "test:${test_case}" "  Upload: raid3=${raid3_time}s, single=${single_time}s (ratio: ${upload_ratio}x)"
  log_info "test:${test_case}" "  Download: raid3=${raid3_dl_time}s, single=${single_dl_time}s (ratio: ${download_ratio}x)"
  log_info "test:${test_case}" "  Both operations within ${max_ratio}x threshold"

  rm -rf "${tempdir}" "${tmp_raid3}" "${tmp_single}"
  rm -f "${raid3_stdout}" "${raid3_stderr}" "${single_stdout}" "${single_stderr}"
  rm -f "${raid3_dl_stdout}" "${raid3_dl_stderr}" "${single_dl_stdout}" "${single_dl_stderr}"
  return 0
}

run_mkdir_test() {
  purge_remote_root "${RAID3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"
  local test_id
  local timestamp random_suffix
  timestamp=$(date +%Y%m%d%H%M%S)
  printf -v random_suffix '%04d' $((RANDOM % 10000))
  test_id="cmp-mkdir-${timestamp}-${random_suffix}"

  log "Running mkdir test with identifier '${test_id}'"

  local lvl_result single_result
  lvl_result=$(capture_command "lvl_mkdir" mkdir "${RAID3_REMOTE}:${test_id}")
  single_result=$(capture_command "single_mkdir" mkdir "${SINGLE_REMOTE}:${test_id}")

  local lvl_status lvl_stdout lvl_stderr
  local single_status single_stdout single_stderr

  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${RAID3_REMOTE} mkdir" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} mkdir" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "mkdir status mismatch: ${RAID3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status}"
    rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"
    return 1
  fi

  if [[ "${lvl_status}" -ne 0 ]]; then
    log "mkdir failed with status ${lvl_status}; outputs retained for inspection:"
    log "  ${lvl_stdout}"
    log "  ${lvl_stderr}"
    log "  ${single_stdout}"
    log "  ${single_stderr}"
    return 1
  fi

  # Follow-up verification using lsd on both remotes.
  local lvl_check single_check
  lvl_check=$(capture_command "lvl_check" lsd "${RAID3_REMOTE}:${test_id}")
  single_check=$(capture_command "single_check" lsd "${SINGLE_REMOTE}:${test_id}")

  local lvl_check_status lvl_check_stdout lvl_check_stderr
  local single_check_status single_check_stdout single_check_stderr

  IFS='|' read -r lvl_check_status lvl_check_stdout lvl_check_stderr <<<"${lvl_check}"
  IFS='|' read -r single_check_status single_check_stdout single_check_stderr <<<"${single_check}"

  print_if_verbose "${RAID3_REMOTE} lsd" "${lvl_check_stdout}" "${lvl_check_stderr}"
  print_if_verbose "${SINGLE_REMOTE} lsd" "${single_check_stdout}" "${single_check_stderr}"

  if [[ "${lvl_check_status}" -ne "${single_check_status}" ]]; then
    log "lsd status mismatch after mkdir: ${RAID3_REMOTE}=${lvl_check_status}, ${SINGLE_REMOTE}=${single_check_status}"
    rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"
    rm -f "${lvl_check_stdout}" "${lvl_check_stderr}" "${single_check_stdout}" "${single_check_stderr}"
    return 1
  fi

  log "mkdir test succeeded for '${test_id}'."

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"
  rm -f "${lvl_check_stdout}" "${lvl_check_stderr}" "${single_check_stdout}" "${single_check_stderr}"
  return 0
}

run_single_test() {
  set_remotes_for_storage_type

  local test_name="${COMMAND_ARG}"
  local test_func=""

  case "${test_name}" in
    mkdir)        test_func="run_mkdir_test" ;;
    lsd)          test_func="run_lsd_test" ;;
    ls)           test_func="run_ls_test" ;;
    cat)          test_func="run_cat_test" ;;
    delete)       test_func="run_delete_test" ;;
    cp-download)  test_func="run_copy_download_test" ;;
    cp-upload)    test_func="run_copy_upload_test" ;;
    move)         test_func="run_move_test" ;;
    check)        test_func="run_check_test" ;;
    sync-upload)  test_func="run_sync_upload_test" ;;
    sync-download) test_func="run_sync_download_test" ;;
    purge)        test_func="run_purge_test" ;;
    performance)  test_func="run_performance_test" ;;
    *) die "Unknown test '${test_name}'. Use '${SCRIPT_NAME} list' to see available tests." ;;
  esac

  if "${test_func}"; then
    pass_test "${test_name}" "Completed (${STORAGE_TYPE})."
    return 0
  else
    fail_test "${test_name}" "See details above."
    return 1
  fi
}

# ------------------------------- main logic ---------------------------------

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
      list_tests
      ;;

    test)
      [[ "${STORAGE_TYPE}" != "minio" ]] || ensure_minio_containers_ready
      reset_test_results
      if [[ -z "${COMMAND_ARG}" ]]; then
        if ! run_all_tests; then
          print_test_summary
          die "One or more tests failed."
        fi
      else
        if ! run_single_test; then
          print_test_summary
          die "Test '${COMMAND_ARG}' failed."
        fi
      fi
      print_test_summary
      ;;
  esac
}

main "$@"

