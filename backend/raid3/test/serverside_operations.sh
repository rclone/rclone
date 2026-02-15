#!/usr/bin/env bash
#
# serverside_operations.sh
# ------------------------
# Test script for server-side operations on rclone raid3 backends.
#
# Raid3 supports server-side Copy/Move/DirMove within the same backend only;
# cross-backend operations use stream copy. This script tests copy/move flows
# between raid3 and single backends and manages MinIO containers for minioraid3.
#
# For Go unit/integration tests (same-backend success, cross-backend rejection),
# see: run_serverside_go_tests.sh
#
# Usage:
#   serverside_operations.sh [options] <command> [args]
#
# Commands:
#   start                 Start the MinIO containers required for minioraid3/miniosingle.
#   stop                  Stop those MinIO containers.
#   teardown              Purge all data from both local and MinIO storage (raid3 + single).
#   list                  Show available test cases.
#   test <name>           Run a named test (e.g. "cp-upload"); uses both storage types where needed.
#
# Options:
#   -v, --verbose                  Show stdout/stderr from all rclone invocations.
#   -h, --help                     Display this help text.
#
# Environment:
#   RCLONE_CONFIG         Path to rclone configuration file.
#   RCLONE_TEST_TIMEOUT   Timeout in seconds for rclone commands during 'test' (default: 120). Exit 124 = timed out.
#   SKIP_CP_MINIO_TO_LOCAL  If set, skip cp-minio-to-local in full suite (MinIO upload can hang in some environments).
#
# Safety guard: the script must be executed from backend/raid3/test directory.
# -----------------------------------------------------------------------------

set -euo pipefail

SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)

# shellcheck source=backend/raid3/test/compare_raid3_with_single_common.sh
. "${SCRIPT_DIR}/compare_raid3_with_single_common.sh"

VERBOSE=0
COMMAND=""
COMMAND_ARG=""

# ---------------------------- helper functions ------------------------------

usage() {
  cat <<EOF
Usage: ${SCRIPT_NAME} [options] <command> [arguments]

Tests server-side Copy/Move on raid3 (same-backend only). Manages MinIO
containers for minioraid3. For Go tests (same-backend + cross-backend rejection),
run: ./run_serverside_go_tests.sh

Commands:
  start                      Start MinIO containers for raid3 tests (minioraid3/miniosingle).
  stop                       Stop MinIO containers for raid3 tests.
  teardown                   Purge all test data for both local and MinIO storage.
  list                       Show available tests.
  test <name>                Run the named test (e.g. "cp-upload"); uses both storage types where needed.

Options:
  -v, --verbose                  Show stdout/stderr from all rclone invocations.
  -h, --help                     Display this help.

Environment (for 'test'): RCLONE_TEST_TIMEOUT (seconds, default 120); SKIP_CP_MINIO_TO_LOCAL=1 to skip cp-minio-to-local when MinIO is flaky.

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
  cp-download        Copy objects from raid3 remote to local and compare with single.
  cp-upload          Upload from local to raid3 (local or minio); no cross-backend copy.
  cp-minio-to-local  Copy from minioraid3 to localraid3 (MinIO as source, local as dest).
  mv-local-to-minio  Move objects from localraid3 to minioraid3 (server-side move).
  cp-within-local    Server-side copy within localraid3 (src -> dst prefix).
  cp-within-minio    Server-side copy within minioraid3 (src -> dst prefix).
EOF
}

run_all_tests() {
  local tests=("cp-download" "cp-upload" "cp-minio-to-local" "mv-local-to-minio" "cp-within-local" "cp-within-minio")
  local name
  for name in "${tests[@]}"; do
    if [[ "${name}" == "cp-minio-to-local" && -n "${SKIP_CP_MINIO_TO_LOCAL:-}" ]]; then
      log_info "suite" "Skipping '${name}' (SKIP_CP_MINIO_TO_LOCAL is set)"
      continue
    fi
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

run_copy_download_test() {
  set_remotes_for_storage_type
  purge_raid3_remote_root
  purge_remote_root "${SINGLE_REMOTE}"
  log "Running copy-download test"

  local dataset_id
  if ! dataset_id=$(create_test_dataset "cp-download"); then
    log "Failed to set up dataset for copy-download test."
    return 1
  fi
  log "Dataset created: ${RAID3_REMOTE}:${dataset_id} and ${SINGLE_REMOTE}:${dataset_id} (retained for inspection)"

  # Verify dataset is visible on raid3 (same as compare_raid3_with_single; avoids spurious "directory not found")
  if ! rclone_cmd lsl "${RAID3_REMOTE}:${dataset_id}/file_root.txt" >/dev/null 2>&1; then
    log "Verification failed: ${RAID3_REMOTE}:${dataset_id}/file_root.txt not found after create_test_dataset"
    return 1
  fi

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
  set_remotes_for_storage_type
  purge_raid3_remote_root
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

  # Upload only to the raid3 remote (RAID3_REMOTE).
  local upload_result upload_status upload_stdout upload_stderr
  upload_result=$(capture_command "lvl_copy_upload" copy "${tempdir}" "${RAID3_REMOTE}:${dataset_id}")

  IFS='|' read -r upload_status upload_stdout upload_stderr <<<"${upload_result}"

  print_if_verbose "${RAID3_REMOTE} copy (upload)" "${upload_stdout}" "${upload_stderr}"

  if [[ "${upload_status}" -ne 0 ]]; then
    log "copy (upload) failed on ${RAID3_REMOTE} with status ${upload_status}"
    log "Outputs retained:"
    log "  ${upload_stdout}"
    log "  ${upload_stderr}"
    rm -rf "${tempdir}"
    return 1
  fi

  rm -f "${upload_stdout}" "${upload_stderr}"

  # Basic verification: download from RAID3_REMOTE and ensure we got data back.
  local tmp_lvl
  tmp_lvl=$(mktemp -d) || { rm -rf "${tempdir}"; return 1; }

  local verify_result verify_status verify_stdout verify_stderr
  verify_result=$(capture_command "lvl_verify_upload" copy "${RAID3_REMOTE}:${dataset_id}" "${tmp_lvl}")
  IFS='|' read -r verify_status verify_stdout verify_stderr <<<"${verify_result}"

  print_if_verbose "${RAID3_REMOTE} copy (verify download)" "${verify_stdout}" "${verify_stderr}"

  if [[ "${verify_status}" -ne 0 ]]; then
    log "Verification copy from ${RAID3_REMOTE} failed with status ${verify_status}"
    rm -rf "${tempdir}" "${tmp_lvl}"
    return 1
  fi

  rm -f "${verify_stdout}" "${verify_stderr}"

  # Ensure that something was actually downloaded.
  if ! find "${tmp_lvl}" -mindepth 1 -print -quit >/dev/null 2>&1; then
    log "Verification: no files downloaded from ${RAID3_REMOTE}:${dataset_id}"
    rm -rf "${tempdir}" "${tmp_lvl}"
    return 1
  fi

  rm -rf "${tempdir}" "${tmp_lvl}"

  log "copy-upload test completed. Dataset stored as ${dataset_id} on both remotes."

  # Cross-backend copy (localraid3 -> minioraid3) is skipped for now (MinIO 503 / slow).
  # Use test 'cp-minio-to-local' for the reverse direction (minioraid3 -> localraid3).

  return 0
}

# Copy minioraid3 -> localraid3 (MinIO as source, local as destination).
# Avoids MinIO as copy destination (no CreateBucket/503 on write); only reads from MinIO.
run_copy_minio_to_local_test() {
  log "Running cp-minio-to-local test (minioraid3 -> localraid3)"

  local prev_storage_type="${STORAGE_TYPE}"
  STORAGE_TYPE="minio"
  ensure_minio_containers_ready
  if ! wait_for_minio_backend_ready "even" || ! wait_for_minio_backend_ready "odd" || ! wait_for_minio_backend_ready "parity"; then
    log "MinIO backends did not become ready in time. Proceeding anyway."
  fi
  set_remotes_for_storage_type
  purge_raid3_remote_root
  purge_remote_root "${SINGLE_REMOTE}"
  STORAGE_TYPE="local"
  set_remotes_for_storage_type
  purge_raid3_remote_root
  purge_remote_root "${SINGLE_REMOTE}"
  STORAGE_TYPE="${prev_storage_type}"

  # Longer delay after purge so MinIO can settle (503 SlowDown on CreateBucket otherwise).
  sleep 15

  local tempdir
  tempdir=$(mktemp -d) || return 1
  printf 'minio-to-local root file\n' >"${tempdir}/file_root.txt"
  mkdir -p "${tempdir}/subdir"
  printf 'minio-to-local nested file\n' >"${tempdir}/subdir/file_nested.txt"

  local dataset_id
  dataset_id=$(date +minio-to-local-%Y%m%d%H%M%S-$((RANDOM % 10000)))

  # Upload to minioraid3 using same strategy as create_test_dataset (mkdir + copyto) for reliability with MinIO.
  # Retry each step on 503 SlowDown (MinIO rate limit after purge).
  STORAGE_TYPE="minio"
  local m2l_err m2l_retries=3 m2l_retry_delay=10
  m2l_err=$(mktemp) || { rm -rf "${tempdir}"; return 1; }
  # mkdir with retry
  local attempt=1
  while true; do
    if rclone_cmd mkdir "minioraid3:${dataset_id}" 2>"${m2l_err}"; then
      break
    fi
    if ! grep -q -e '503' -e 'SlowDown' "${m2l_err}" 2>/dev/null || [[ "${attempt}" -ge "${m2l_retries}" ]]; then
      log "cp-minio-to-local: mkdir minioraid3:${dataset_id} failed: $(cat "${m2l_err}" 2>/dev/null | head -5)"
      rm -f "${m2l_err}"; rm -rf "${tempdir}"
      return 1
    fi
    log "cp-minio-to-local: mkdir 503/SlowDown (attempt ${attempt}/${m2l_retries}), retrying in ${m2l_retry_delay}s"
    sleep "${m2l_retry_delay}"
    attempt=$((attempt + 1))
  done
  # copyto file_root.txt with retry
  attempt=1
  while true; do
    if rclone_cmd copyto "${tempdir}/file_root.txt" "minioraid3:${dataset_id}/file_root.txt" 2>"${m2l_err}"; then
      break
    fi
    if ! grep -q -e '503' -e 'SlowDown' "${m2l_err}" 2>/dev/null || [[ "${attempt}" -ge "${m2l_retries}" ]]; then
      log "cp-minio-to-local: upload file_root.txt failed: $(cat "${m2l_err}" 2>/dev/null | head -5)"
      rm -f "${m2l_err}"; rm -rf "${tempdir}"
      return 1
    fi
    log "cp-minio-to-local: upload file_root.txt 503/SlowDown (attempt ${attempt}/${m2l_retries}), retrying in ${m2l_retry_delay}s"
    sleep "${m2l_retry_delay}"
    attempt=$((attempt + 1))
  done
  # copyto subdir/file_nested.txt with retry
  attempt=1
  while true; do
    if rclone_cmd copyto "${tempdir}/subdir/file_nested.txt" "minioraid3:${dataset_id}/subdir/file_nested.txt" 2>"${m2l_err}"; then
      break
    fi
    if ! grep -q -e '503' -e 'SlowDown' "${m2l_err}" 2>/dev/null || [[ "${attempt}" -ge "${m2l_retries}" ]]; then
      log "cp-minio-to-local: upload subdir/file_nested.txt failed: $(cat "${m2l_err}" 2>/dev/null | head -5)"
      rm -f "${m2l_err}"; rm -rf "${tempdir}"
      return 1
    fi
    log "cp-minio-to-local: upload subdir/file_nested.txt 503/SlowDown (attempt ${attempt}/${m2l_retries}), retrying in ${m2l_retry_delay}s"
    sleep "${m2l_retry_delay}"
    attempt=$((attempt + 1))
  done
  rm -f "${m2l_err}"
  STORAGE_TYPE="${prev_storage_type}"

  # Copy minioraid3 -> localraid3 (destination is local, no MinIO write). Use timeout to avoid hang.
  local m2l_timeout=180
  local cross_result cross_status cross_stdout cross_stderr
  cross_result=$(capture_command_with_timeout "${m2l_timeout}" "m2l_copy" copy "minioraid3:${dataset_id}" "localraid3:${dataset_id}")
  IFS='|' read -r cross_status cross_stdout cross_stderr <<<"${cross_result}"
  print_if_verbose "minioraid3->localraid3 copy" "${cross_stdout}" "${cross_stderr}"
  if [[ "${cross_status}" -ne 0 ]]; then
    if [[ "${cross_status}" -eq 124 ]]; then
      log "cp-minio-to-local: copy minioraid3 -> localraid3 timed out (${m2l_timeout}s)"
    else
      log "cp-minio-to-local: copy minioraid3 -> localraid3 failed with status ${cross_status}"
    fi
    rm -f "${cross_stdout}" "${cross_stderr}"
    rm -rf "${tempdir}"
    return 1
  fi
  rm -f "${cross_stdout}" "${cross_stderr}"

  # Verify: download from localraid3 and diff with original.
  local dst_tmp
  dst_tmp=$(mktemp -d) || { rm -rf "${tempdir}"; return 1; }
  local verify_result verify_status verify_out verify_err
  verify_result=$(capture_command "m2l_verify" copy "localraid3:${dataset_id}" "${dst_tmp}")
  IFS='|' read -r verify_status verify_out verify_err <<<"${verify_result}"
  print_if_verbose "localraid3 copy (verify)" "${verify_out}" "${verify_err}"
  if [[ "${verify_status}" -ne 0 ]]; then
    log "cp-minio-to-local: verification download from localraid3 failed with status ${verify_status}"
    rm -f "${verify_out}" "${verify_err}"
    rm -rf "${tempdir}" "${dst_tmp}"
    return 1
  fi
  rm -f "${verify_out}" "${verify_err}"
  if ! diff -qr "${tempdir}" "${dst_tmp}" >/dev/null; then
    log "cp-minio-to-local: localraid3 content does not match original"
    rm -rf "${tempdir}" "${dst_tmp}"
    return 1
  fi
  rm -rf "${tempdir}" "${dst_tmp}"
  log "cp-minio-to-local test completed."
  return 0
}

run_move_local_to_minio_test() {
  log "Running move-local-to-minio test"

  # Ensure starting from a clean state.
  # This test specifically tests cross-backend operations (local -> minio)
  purge_remote_root "localraid3"
  
  # Ensure MinIO is ready
  local prev_storage_type="${STORAGE_TYPE}"
  STORAGE_TYPE="minio"
  ensure_minio_containers_ready
  STORAGE_TYPE="${prev_storage_type}"
  
  purge_remote_root "minioraid3"

  local tempdir
  tempdir=$(mktemp -d) || return 1

  # Create a small test dataset.
  printf 'move root file\n' >"${tempdir}/file_root.txt"
  mkdir -p "${tempdir}/subdir"
  printf 'move nested file\n' >"${tempdir}/subdir/file_nested.txt"

  local dataset_id
  dataset_id=$(date +move-local-to-minio-%Y%m%d%H%M%S-$((RANDOM % 10000)))

  # Upload to localraid3 only.
  local up_result up_status up_out up_err
  up_result=$(capture_command "mv_l2m_upload" copy "${tempdir}" "localraid3:${dataset_id}")
  IFS='|' read -r up_status up_out up_err <<<"${up_result}"
  print_if_verbose "localraid3 copy (upload for move)" "${up_out}" "${up_err}"

  if [[ "${up_status}" -ne 0 ]]; then
    log "move-local-to-minio: initial upload to localraid3 failed with status ${up_status}"
    rm -f "${up_out}" "${up_err}"
    rm -rf "${tempdir}"
    return 1
  fi
  rm -f "${up_out}" "${up_err}"

  # Perform server-side move from localraid3 to minioraid3.
  local mv_result mv_status mv_out mv_err
  mv_result=$(capture_command "mv_l2m_move" move "localraid3:${dataset_id}" "minioraid3:${dataset_id}")
  IFS='|' read -r mv_status mv_out mv_err <<<"${mv_result}"
  print_if_verbose "localraid3->minioraid3 move" "${mv_out}" "${mv_err}"

  if [[ "${mv_status}" -ne 0 ]]; then
    log "move-local-to-minio: server-side move failed with status ${mv_status}"
    rm -f "${mv_out}" "${mv_err}"
    rm -rf "${tempdir}"
    return 1
  fi
  rm -f "${mv_out}" "${mv_err}"

  # Verify that the source prefix on localraid3 is empty / gone.
  local src_ls_result src_ls_status src_ls_out src_ls_err
  src_ls_result=$(capture_command "mv_l2m_src_ls" ls "localraid3:${dataset_id}")
  IFS='|' read -r src_ls_status src_ls_out src_ls_err <<<"${src_ls_result}"
  print_if_verbose "localraid3 ls (post-move)" "${src_ls_out}" "${src_ls_err}"

  # It is acceptable if ls either fails with "not found" or succeeds with no output.
  # We only fail if there are still entries under the prefix.
  if grep -q . "${src_ls_out}" 2>/dev/null; then
    log "move-local-to-minio: source prefix localraid3:${dataset_id} still has objects after move"
    rm -f "${src_ls_out}" "${src_ls_err}"
    rm -rf "${tempdir}"
    return 1
  fi
  rm -f "${src_ls_out}" "${src_ls_err}"

  # Verify destination contents by downloading from minioraid3 and comparing with original local data.
  local dst_tmp
  dst_tmp=$(mktemp -d) || { rm -rf "${tempdir}"; return 1; }

  local dst_res dst_status dst_out dst_err
  dst_res=$(capture_command "mv_l2m_verify" copy "minioraid3:${dataset_id}" "${dst_tmp}")
  IFS='|' read -r dst_status dst_out dst_err <<<"${dst_res}"
  print_if_verbose "minioraid3 copy (verify move)" "${dst_out}" "${dst_err}"

  if [[ "${dst_status}" -ne 0 ]]; then
    log "move-local-to-minio: verification download from minioraid3 failed with status ${dst_status}"
    rm -f "${dst_out}" "${dst_err}"
    rm -rf "${tempdir}" "${dst_tmp}"
    return 1
  fi
  rm -f "${dst_out}" "${dst_err}"

  if ! diff -qr "${tempdir}" "${dst_tmp}" >/dev/null; then
    log "move-local-to-minio: destination content on minioraid3 does not match original data"
    rm -rf "${tempdir}" "${dst_tmp}"
    return 1
  fi

  rm -rf "${tempdir}" "${dst_tmp}"
  log "move-local-to-minio test completed."
  return 0
}

run_copy_within_localraid3_test() {
  log "Running copy-within-localraid3 test"

  set_remotes_for_storage_type
  purge_remote_root "localraid3"

  local tempdir
  tempdir=$(mktemp -d) || return 1

  # Create a small test dataset.
  printf 'within local root file\n' >"${tempdir}/file_root.txt"
  mkdir -p "${tempdir}/subdir"
  printf 'within local nested file\n' >"${tempdir}/subdir/file_nested.txt"

  local src_id dst_id
  src_id=$(date +cp-within-local-%Y%m%d%H%M%S-$((RANDOM % 10000)))
  dst_id="${src_id}-copy"

  # Upload to source prefix on localraid3.
  local up_res up_status up_out up_err
  up_res=$(capture_command "cp_within_local_upload" copy "${tempdir}" "localraid3:${src_id}")
  IFS='|' read -r up_status up_out up_err <<<"${up_res}"
  print_if_verbose "localraid3 copy (upload for within)" "${up_out}" "${up_err}"

  if [[ "${up_status}" -ne 0 ]]; then
    log "copy-within-localraid3: initial upload to localraid3 failed with status ${up_status}"
    rm -f "${up_out}" "${up_err}"
    rm -rf "${tempdir}"
    return 1
  fi
  rm -f "${up_out}" "${up_err}"

  # Perform server-side copy from src_id to dst_id on localraid3.
  local cp_res cp_status cp_out cp_err
  cp_res=$(capture_command "cp_within_local_copy" copy "localraid3:${src_id}" "localraid3:${dst_id}")
  IFS='|' read -r cp_status cp_out cp_err <<<"${cp_res}"
  print_if_verbose "localraid3 copy (within raid3)" "${cp_out}" "${cp_err}"

  if [[ "${cp_status}" -ne 0 ]]; then
    log "copy-within-localraid3: server-side copy failed with status ${cp_status}"
    rm -f "${cp_out}" "${cp_err}"
    rm -rf "${tempdir}"
    return 1
  fi
  rm -f "${cp_out}" "${cp_err}"

  # Verify both prefixes by downloading and comparing.
  local src_tmp dst_tmp
  src_tmp=$(mktemp -d) || { rm -rf "${tempdir}"; return 1; }
  dst_tmp=$(mktemp -d) || { rm -rf "${tempdir}" "${src_tmp}"; return 1; }

  local src_dl src_status src_out src_err
  local dst_dl dst_status dst_out dst_err
  src_dl=$(capture_command "cp_within_local_src_dl" copy "localraid3:${src_id}" "${src_tmp}")
  dst_dl=$(capture_command "cp_within_local_dst_dl" copy "localraid3:${dst_id}" "${dst_tmp}")
  IFS='|' read -r src_status src_out src_err <<<"${src_dl}"
  IFS='|' read -r dst_status dst_out dst_err <<<"${dst_dl}"

  print_if_verbose "localraid3 copy (verify src)" "${src_out}" "${src_err}"
  print_if_verbose "localraid3 copy (verify dst)" "${dst_out}" "${dst_err}"

  if [[ "${src_status}" -ne 0 || "${dst_status}" -ne 0 ]]; then
    log "copy-within-localraid3: verification copy failed (src=${src_status}, dst=${dst_status})"
    rm -f "${src_out}" "${src_err}" "${dst_out}" "${dst_err}"
    rm -rf "${tempdir}" "${src_tmp}" "${dst_tmp}"
    return 1
  fi

  rm -f "${src_out}" "${src_err}" "${dst_out}" "${dst_err}"

  if ! diff -qr "${src_tmp}" "${dst_tmp}" >/dev/null; then
    log "copy-within-localraid3: downloaded src/dst content differs"
    rm -rf "${tempdir}" "${src_tmp}" "${dst_tmp}"
    return 1
  fi

  rm -rf "${tempdir}" "${src_tmp}" "${dst_tmp}"
  log "copy-within-localraid3 test completed."
  return 0
}

run_copy_within_minioraid3_test() {
  log "Running copy-within-minioraid3 test"

  # Ensure MinIO is ready
  local prev_storage_type="${STORAGE_TYPE}"
  STORAGE_TYPE="minio"
  ensure_minio_containers_ready
  STORAGE_TYPE="${prev_storage_type}"
  
  purge_remote_root "minioraid3"

  local tempdir
  tempdir=$(mktemp -d) || return 1

  # Create a small test dataset.
  printf 'within minio root file\n' >"${tempdir}/file_root.txt"
  mkdir -p "${tempdir}/subdir"
  printf 'within minio nested file\n' >"${tempdir}/subdir/file_nested.txt"

  local src_id dst_id
  src_id=$(date +cp-within-minio-%Y%m%d%H%M%S-$((RANDOM % 10000)))
  dst_id="${src_id}-copy"

  # Upload to source prefix on minioraid3.
  local up_res up_status up_out up_err
  up_res=$(capture_command "cp_within_minio_upload" copy "${tempdir}" "minioraid3:${src_id}")
  IFS='|' read -r up_status up_out up_err <<<"${up_res}"
  print_if_verbose "minioraid3 copy (upload for within)" "${up_out}" "${up_err}"

  if [[ "${up_status}" -ne 0 ]]; then
    log "copy-within-minioraid3: initial upload to minioraid3 failed with status ${up_status}"
    rm -f "${up_out}" "${up_err}"
    rm -rf "${tempdir}"
    return 1
  fi
  rm -f "${up_out}" "${up_err}"

  # Perform server-side copy from src_id to dst_id on minioraid3.
  local cp_res cp_status cp_out cp_err
  cp_res=$(capture_command "cp_within_minio_copy" copy "minioraid3:${src_id}" "minioraid3:${dst_id}")
  IFS='|' read -r cp_status cp_out cp_err <<<"${cp_res}"
  print_if_verbose "minioraid3 copy (within raid3)" "${cp_out}" "${cp_err}"

  if [[ "${cp_status}" -ne 0 ]]; then
    log "copy-within-minioraid3: server-side copy failed with status ${cp_status}"
    rm -f "${cp_out}" "${cp_err}"
    rm -rf "${tempdir}"
    return 1
  fi
  rm -f "${cp_out}" "${cp_err}"

  # Verify both prefixes by downloading and comparing.
  local src_tmp dst_tmp
  src_tmp=$(mktemp -d) || { rm -rf "${tempdir}"; return 1; }
  dst_tmp=$(mktemp -d) || { rm -rf "${tempdir}" "${src_tmp}"; return 1; }

  local src_dl src_status src_out src_err
  local dst_dl dst_status dst_out dst_err
  src_dl=$(capture_command "cp_within_minio_src_dl" copy "minioraid3:${src_id}" "${src_tmp}")
  dst_dl=$(capture_command "cp_within_minio_dst_dl" copy "minioraid3:${dst_id}" "${dst_tmp}")
  IFS='|' read -r src_status src_out src_err <<<"${src_dl}"
  IFS='|' read -r dst_status dst_out dst_err <<<"${dst_dl}"

  print_if_verbose "minioraid3 copy (verify src)" "${src_out}" "${src_err}"
  print_if_verbose "minioraid3 copy (verify dst)" "${dst_out}" "${dst_err}"

  if [[ "${src_status}" -ne 0 || "${dst_status}" -ne 0 ]]; then
    log "copy-within-minioraid3: verification copy failed (src=${src_status}, dst=${dst_status})"
    rm -f "${src_out}" "${src_err}" "${dst_out}" "${dst_err}"
    rm -rf "${tempdir}" "${src_tmp}" "${dst_tmp}"
    return 1
  fi

  rm -f "${src_out}" "${src_err}" "${dst_out}" "${dst_err}"

  if ! diff -qr "${src_tmp}" "${dst_tmp}" >/dev/null; then
    log "copy-within-minioraid3: downloaded src/dst content differs"
    rm -rf "${tempdir}" "${src_tmp}" "${dst_tmp}"
    return 1
  fi

  rm -rf "${tempdir}" "${src_tmp}" "${dst_tmp}"
  log "copy-within-minioraid3 test completed."
  return 0
}
# Internal: run a test that requires a single storage type (local or minio).
# Sets STORAGE_TYPE for the duration of the test. Used by run_single_test.
run_pair_test_for_storage_type() {
  local test_func="$1"
  local storage_type="$2"
  local test_name="$3"
  STORAGE_TYPE="${storage_type}"
  if "${test_func}"; then
    pass_test "${test_name}" "Completed (${storage_type})."
    return 0
  else
    fail_test "${test_name}" "See details above (storage-type=${storage_type})."
    return 1
  fi
}

run_single_test() {
  local test_name="${COMMAND_ARG}"
  local test_func=""

  case "${test_name}" in
    cp-download)        test_func="run_copy_download_test" ;;
    cp-upload)          test_func="run_copy_upload_test" ;;
    cp-minio-to-local)  test_func="run_copy_minio_to_local_test" ;;
    mv-local-to-minio)  test_func="run_move_local_to_minio_test" ;;
    cp-within-local)    test_func="run_copy_within_localraid3_test" ;;
    cp-within-minio)    test_func="run_copy_within_minioraid3_test" ;;
    *) die "Unknown test '${test_name}'. Use '${SCRIPT_NAME} list' to see available tests." ;;
  esac

  # Tests that run against both local and minio pair (raid3 vs single)
  if [[ "${test_name}" == "cp-download" || "${test_name}" == "cp-upload" ]]; then
    if ! run_pair_test_for_storage_type "${test_func}" "local" "${test_name}"; then
      return 1
    fi
    if ! run_pair_test_for_storage_type "${test_func}" "minio" "${test_name}"; then
      return 1
    fi
    return 0
  fi

  # Tests that use a fixed backend: set STORAGE_TYPE so set_remotes_for_storage_type works if called
  if [[ "${test_name}" == "cp-within-local" ]]; then
    STORAGE_TYPE="local"
  elif [[ "${test_name}" == "cp-within-minio" || "${test_name}" == "mv-local-to-minio" || "${test_name}" == "cp-minio-to-local" ]]; then
    STORAGE_TYPE="minio"
  fi

  if "${test_func}"; then
    pass_test "${test_name}" "Completed."
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
  ensure_rclone_binary
  ensure_rclone_config

  case "${COMMAND}" in
    start)
      start_minio_containers
      ;;

    stop)
      stop_minio_containers
      ;;

    teardown)
      # Purge and clean both local and MinIO storage
      STORAGE_TYPE="local"
      set_remotes_for_storage_type
      purge_raid3_remote_root
      purge_remote_root "${SINGLE_REMOTE}"
      for dir in "${LOCAL_RAID3_DIRS[@]}" "${LOCAL_SINGLE_DIR}"; do
        remove_leftover_files "${dir}"
        verify_directory_empty "${dir}"
      done

      STORAGE_TYPE="minio"
      ensure_minio_containers_ready
      set_remotes_for_storage_type
      purge_raid3_remote_root
      purge_remote_root "${SINGLE_REMOTE}"
      sleep 3
      for dir in "${MINIO_RAID3_DIRS[@]}" "${MINIO_SINGLE_DIR}"; do
        remove_leftover_files "${dir}"
        verify_directory_empty "${dir}"
      done
      ;;

    list)
      list_tests
      ;;

    test)
      # All tests may use MinIO (cp-download/cp-upload run for both local and minio; mv and cp-within-minio need minio).
      # Set STORAGE_TYPE so ensure_minio_containers_ready (common script) does not hit unbound variable and actually starts MinIO.
      STORAGE_TYPE="minio"
      ensure_minio_containers_ready
      # Avoid indefinite hang on rclone (e.g. MinIO 503 or list hang). Exit 124 = timed out.
      export RCLONE_TEST_TIMEOUT="${RCLONE_TEST_TIMEOUT:-120}"
      # Skip cp-minio-to-local by default (MinIO 503 SlowDown in many environments). Set SKIP_CP_MINIO_TO_LOCAL=0 to run it.
      export SKIP_CP_MINIO_TO_LOCAL="${SKIP_CP_MINIO_TO_LOCAL:-1}"

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

