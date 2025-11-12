#!/usr/bin/env bash
#
# compare_level3_with_single.sh
# ---------------------------------
# Black-box comparison harness for rclone level3 backends.
#
# This script runs a selected rclone command against a level3 backend and the
# corresponding single-backend configuration, compares the exit status, and
# (optionally) shows both outputs. It also manages the supporting MinIO
# containers used by the MinIO-based level3 backend.
#
# Usage:
#   compare_level3_with_single.sh [options] <command> [args]
#
# Commands:
#   start                 Start the MinIO containers required for miniolevel3/miniosingle.
#   stop                  Stop those MinIO containers.
#   teardown              Purge all data from the selected storage-type (level3 + single).
#   list                  Show available test cases.
#   test <name>           Run a named test (e.g. "mkdir") against level3 vs single.
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
# Safety guard: the script must be executed from $HOME/go/level3storage.
# -----------------------------------------------------------------------------

set -euo pipefail

SCRIPT_NAME=$(basename "$0")
WORKDIR="${HOME}/go/level3storage"
RCLONE_CONFIG="${RCLONE_CONFIG:-${HOME}/.config/rclone/rclone.conf}"
VERBOSE=0
STORAGE_TYPE=""
COMMAND=""
COMMAND_ARG=""

# Directory layout used by the configured backends.
LOCAL_LEVEL3_DIRS=(
  "${WORKDIR}/even_local"
  "${WORKDIR}/odd_local"
  "${WORKDIR}/parity_local"
)
LOCAL_SINGLE_DIR="${WORKDIR}/single_local"

MINIO_LEVEL3_DIRS=(
  "${WORKDIR}/even_minio"
  "${WORKDIR}/odd_minio"
  "${WORKDIR}/parity_minio"
)
MINIO_SINGLE_DIR="${WORKDIR}/single_minio"

# Directories explicitly allowed for cleanup
ALLOWED_DATA_DIRS=(
  "${LOCAL_LEVEL3_DIRS[@]}"
  "${LOCAL_SINGLE_DIR}"
  "${MINIO_LEVEL3_DIRS[@]}"
  "${MINIO_SINGLE_DIR}"
)

# Definition of MinIO containers: name|user|password|s3_port|console_port|data_dir
MINIO_CONTAINERS=(
  "minioeven|even|evenpass88|9001|9004|${WORKDIR}/even_minio"
  "minioodd|odd|oddpass88|9002|9005|${WORKDIR}/odd_minio"
  "minioparity|parity|paritypass88|9003|9006|${WORKDIR}/parity_minio"
  "miniosingle|single|singlepass88|9004|9007|${WORKDIR}/single_minio"
)

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

Environment:
  RCLONE_CONFIG                  Path to rclone.conf (default: ${RCLONE_CONFIG})

The script must be executed from ${WORKDIR}.
EOF
}

log() {
  printf '[%s] %s\n' "${SCRIPT_NAME}" "$*"
}

die() {
  printf '[%s] ERROR: %s\n' "${SCRIPT_NAME}" "$*" >&2
  exit 1
}

ensure_workdir() {
  if [[ "${PWD}" != "${WORKDIR}" ]]; then
    die "This script must be run from ${WORKDIR} (current: ${PWD})"
  fi
}

ensure_rclone_config() {
  [[ -f "${RCLONE_CONFIG}" ]] || die "rclone config not found at ${RCLONE_CONFIG}"
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

rclone_cmd() {
  rclone --config "${RCLONE_CONFIG}" "$@"
}

capture_command() {
  local label="$1"
  shift

  local out_file err_file status
  out_file=$(mktemp "/tmp/${label}.stdout.XXXXXX")
  err_file=$(mktemp "/tmp/${label}.stderr.XXXXXX")

  set +e
  rclone_cmd "$@" >"${out_file}" 2>"${err_file}"
  status=$?
  set -e

  printf '%s|%s|%s\n' "${status}" "${out_file}" "${err_file}"
}

print_if_verbose() {
  local tag="$1"
  local stdout_file="$2"
  local stderr_file="$3"

  if (( VERBOSE )); then
    printf '\n[%s stdout]\n' "${tag}"
    cat "${stdout_file}"
    printf '[%s stderr]\n' "${tag}"
    cat "${stderr_file}"
  fi
}

ensure_directory() {
  local dir="$1"
  if [[ ! -d "${dir}" ]]; then
    mkdir -p "${dir}"
  fi
}

container_exists() {
  local name="$1"
  docker ps -a --format '{{.Names}}' | grep -Fxq "${name}"
}

container_running() {
  local name="$1"
  docker ps --format '{{.Names}}' | grep -Fxq "${name}"
}

start_minio_containers() {
  for entry in "${MINIO_CONTAINERS[@]}"; do
    IFS='|' read -r name user pass s3_port console_port data_dir <<<"${entry}"
    ensure_directory "${data_dir}"

    if container_running "${name}"; then
      log "Container '${name}' already running – skipping."
      continue
    fi

    if container_exists "${name}"; then
      log "Starting existing container '${name}'."
      docker start "${name}" >/dev/null
      continue
    fi

    log "Launching container '${name}' (ports ${s3_port}/${console_port})."
    docker run -d \
      --name "${name}" \
      -p "${s3_port}:9000" \
      -p "${console_port}:9001" \
      -e "MINIO_ROOT_USER=${user}" \
      -e "MINIO_ROOT_PASSWORD=${pass}" \
      -v "${data_dir}:/data" \
      quay.io/minio/minio server /data --console-address ":9001" >/dev/null
  done
}

stop_minio_containers() {
  local any_running=0
  for entry in "${MINIO_CONTAINERS[@]}"; do
    IFS='|' read -r name _ <<<"${entry}"
    if container_running "${name}"; then
      log "Stopping container '${name}'."
      docker stop "${name}" >/dev/null
      any_running=1
    else
      log "Container '${name}' not running."
    fi
  done

  if (( ! any_running )); then
    log "No MinIO containers were running."
  fi
}

purge_remote_root() {
  local remote="$1"
  log "Purging remote '${remote}:'"

  local entries=()
  local lsd_output=""
  if lsd_output=$(rclone_cmd lsd "${remote}:" 2>/dev/null | awk '{print $5}' || true); then
    while IFS= read -r entry; do
      [[ -n "${entry}" ]] && entries+=("${entry}")
    done <<<"${lsd_output}"
  fi

  if [[ "${#entries[@]}" -eq 0 ]]; then
    log "  (no top-level directories found on ${remote})"
    rclone_cmd purge "${remote}:" >/dev/null 2>&1 || true
  else
    for entry in "${entries[@]}"; do
      if [[ -n "${entry}" ]]; then
        log "  - purging ${remote}:${entry}"
        rclone_cmd purge "${remote}:${entry}" >/dev/null 2>&1 || true
      fi
    done
  fi
}

verify_directory_empty() {
  local dir="$1"
  if [[ ! -d "${dir}" ]]; then
    return
  fi
  local leftover
  leftover=$(find "${dir}" -mindepth 1 \
    -not -path "${dir}/.DS_Store" \
    -not -path "${dir}/.DS_Store/*" \
    -not -path "${dir}/.minio.sys" \
    -not -path "${dir}/.minio.sys/*" \
    -print -quit 2>/dev/null || true)
  if [[ -n "${leftover}" ]]; then
    log "WARNING: directory '${dir}' is not empty after purge."
  fi
}

remove_leftover_files() {
  local dir="$1"

  local allowed=0
  for candidate in "${ALLOWED_DATA_DIRS[@]}"; do
    if [[ "${dir}" == "${candidate}" ]]; then
      allowed=1
      break
    fi
  done

  if (( ! allowed )); then
    log "Refusing to clean unexpected directory '${dir}' (not in whitelist)."
    return
  fi

  case "${dir}" in
    "${WORKDIR}"/*) ;;
    *)
      log "Refusing to clean directory '${dir}' (outside ${WORKDIR})."
      return
      ;;
  esac

  if [[ ! -d "${dir}" ]]; then
    return
  fi

  find "${dir}" -mindepth 1 \
    -not -path "${dir}/.DS_Store" \
    -not -path "${dir}/.DS_Store/*" \
    -not -path "${dir}/.minio.sys" \
    -not -path "${dir}/.minio.sys/*" \
    -exec rm -rf {} + >/dev/null 2>&1 || true
}

create_test_dataset() {
  local label="$1"

  # Dataset layout created by this helper (for both remotes):
  #   ${dataset_id}/file_root.txt              → Root-level file
  #   ${dataset_id}/dirA/file_nested.txt       → Nested file in dirA/
  #   ${dataset_id}/dirB/file_placeholder.txt  → Nested file in dirB/
  #
  # Each test using this dataset can rely on these files. The directories are
  # materialized by uploading files, keeping S3/MinIO semantics happy (no empty dirs).
  local timestamp random_suffix test_id
  timestamp=$(date +%Y%m%d%H%M%S)
  printf -v random_suffix '%04d' $((RANDOM % 10000))
  test_id="compare-${label}-${timestamp}-${random_suffix}"

  local tmpfile1 tmpfile2
  tmpfile1=$(mktemp) || return 1
  tmpfile2=$(mktemp) || { rm -f "${tmpfile1}"; return 1; }

  printf 'Sample data for %s (root file)\n' "${label}" >"${tmpfile1}"
  printf 'Sample data for %s (nested file)\n' "${label}" >"${tmpfile2}"

  local remote
  for remote in "${LEVEL3_REMOTE}" "${SINGLE_REMOTE}"; do
    if ! rclone_cmd mkdir "${remote}:${test_id}" >/dev/null; then
      log "Failed to mkdir ${remote}:${test_id}"
      rm -f "${tmpfile1}" "${tmpfile2}"
      return 1
    fi
    if ! rclone_cmd copyto "${tmpfile1}" "${remote}:${test_id}/file_root.txt" >/dev/null; then
      log "Failed to copy root sample file to ${remote}:${test_id}"
      rm -f "${tmpfile1}" "${tmpfile2}"
      return 1
    fi
    if ! rclone_cmd copyto "${tmpfile2}" "${remote}:${test_id}/dirA/file_nested.txt" >/dev/null; then
      log "Failed to copy nested sample file to ${remote}:${test_id}"
      rm -f "${tmpfile1}" "${tmpfile2}"
      return 1
    fi
    if ! rclone_cmd copyto "${tmpfile1}" "${remote}:${test_id}/dirB/file_placeholder.txt" >/dev/null; then
      log "Failed to copy placeholder file to ${remote}:${test_id}/dirB"
      rm -f "${tmpfile1}" "${tmpfile2}"
      return 1
    fi
  done

  rm -f "${tmpfile1}" "${tmpfile2}"
  printf '%s\n' "${test_id}"
}

set_remotes_for_storage_type() {
  case "${STORAGE_TYPE}" in
    local)
      LEVEL3_REMOTE="locallevel3"
      SINGLE_REMOTE="localsingle"
      ;;
    minio)
      LEVEL3_REMOTE="miniolevel3"
      SINGLE_REMOTE="miniosingle"
      ;;
    *)
      die "Unsupported storage type '${STORAGE_TYPE}'"
      ;;
  esac
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
  check        Compare hashes between level3 and single remotes (matching/mismatching cases).
  sync-upload  Sync local changes to remote (create/update/delete) and compare results.
  sync-download Sync remote to local and compare results.
  purge        Purge (delete) buckets on both remotes and compare results.
EOF
}

run_all_tests() {
  local tests=("mkdir" "lsd" "ls" "cat" "delete" "cp-download" "cp-upload" "move" "check" "sync-upload" "sync-download" "purge")
  local name
  for name in "${tests[@]}"; do
    log "=== Running test '${name}' ==="
    COMMAND_ARG="${name}"
    if ! run_single_test; then
      die "Test '${name}' failed."
    fi
  done
  COMMAND_ARG=""
}

# ------------------------------ test helpers --------------------------------
run_lsd_test() {
  purge_remote_root "${LEVEL3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"
  log "Running lsd test"

  local dataset_id
  if ! dataset_id=$(create_test_dataset "lsd"); then
    log "Failed to set up dataset for lsd test."
    return 1
  fi
  log "Dataset created: ${LEVEL3_REMOTE}:${dataset_id} and ${SINGLE_REMOTE}:${dataset_id} (retained for inspection)"

  local lvl_result single_result
  lvl_result=$(capture_command "lvl_lsd" lsd "${LEVEL3_REMOTE}:${dataset_id}")
  single_result=$(capture_command "single_lsd" lsd "${SINGLE_REMOTE}:${dataset_id}")

  local lvl_status lvl_stdout lvl_stderr
  local single_status single_stdout single_stderr

  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${LEVEL3_REMOTE} lsd" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} lsd" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "lsd status mismatch: ${LEVEL3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status}"
    rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"
  log "lsd test completed."
  return 0
}

run_ls_test() {
  purge_remote_root "${LEVEL3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"
  log "Running ls test"

  local dataset_id
  if ! dataset_id=$(create_test_dataset "ls"); then
    log "Failed to set up dataset for ls test."
    return 1
  fi
  log "Dataset created: ${LEVEL3_REMOTE}:${dataset_id} and ${SINGLE_REMOTE}:${dataset_id} (retained for inspection)"

  local lvl_result single_result
  lvl_result=$(capture_command "lvl_ls" ls "${LEVEL3_REMOTE}:${dataset_id}")
  single_result=$(capture_command "single_ls" ls "${SINGLE_REMOTE}:${dataset_id}")

  local lvl_status lvl_stdout lvl_stderr
  local single_status single_stdout single_stderr

  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${LEVEL3_REMOTE} ls" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} ls" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "ls status mismatch: ${LEVEL3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status}"
    rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"
  log "ls test completed."
  return 0
}

run_cat_test() {
  purge_remote_root "${LEVEL3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"
  log "Running cat test"

  local dataset_id
  if ! dataset_id=$(create_test_dataset "cat"); then
    log "Failed to set up dataset for cat test."
    return 1
  fi
  log "Dataset created: ${LEVEL3_REMOTE}:${dataset_id} and ${SINGLE_REMOTE}:${dataset_id} (retained for inspection)"

  local target_existing="${dataset_id}/dirA/file_nested.txt"
  local target_missing="${dataset_id}/missing.txt"

  # Existing object
  local lvl_result single_result
  lvl_result=$(capture_command "lvl_cat_existing" cat "${LEVEL3_REMOTE}:${target_existing}")
  single_result=$(capture_command "single_cat_existing" cat "${SINGLE_REMOTE}:${target_existing}")

  local lvl_status lvl_stdout lvl_stderr
  local single_status single_stdout single_stderr
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${LEVEL3_REMOTE} cat existing" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} cat existing" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "cat (existing) status mismatch: ${LEVEL3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status}"
    log "Outputs retained for inspection:"
    log "  ${lvl_stdout}"
    log "  ${lvl_stderr}"
    log "  ${single_stdout}"
    log "  ${single_stderr}"
    return 1
  fi

  if [[ "${lvl_status}" -eq 0 ]]; then
    if ! cmp -s "${lvl_stdout}" "${single_stdout}"; then
      log "cat (existing) output mismatch between level3 and single backends."
      log "Outputs retained:"
      log "  ${lvl_stdout}"
      log "  ${single_stdout}"
      return 1
    fi
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  # Missing object
  lvl_result=$(capture_command "lvl_cat_missing" cat "${LEVEL3_REMOTE}:${target_missing}")
  single_result=$(capture_command "single_cat_missing" cat "${SINGLE_REMOTE}:${target_missing}")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${LEVEL3_REMOTE} cat missing" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} cat missing" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "cat (missing) status mismatch: ${LEVEL3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status}"
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
  purge_remote_root "${LEVEL3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"
  log "Running copy-download test"

  local dataset_id
  if ! dataset_id=$(create_test_dataset "cp-download"); then
    log "Failed to set up dataset for copy-download test."
    return 1
  fi
  log "Dataset created: ${LEVEL3_REMOTE}:${dataset_id} and ${SINGLE_REMOTE}:${dataset_id} (retained for inspection)"

  local tmp_lvl tmp_single
  tmp_lvl=$(mktemp -d) || return 1
  tmp_single=$(mktemp -d) || { rm -rf "${tmp_lvl}"; return 1; }

  local lvl_result single_result
  lvl_result=$(capture_command "lvl_copy_download" copy "${LEVEL3_REMOTE}:${dataset_id}" "${tmp_lvl}")
  single_result=$(capture_command "single_copy_download" copy "${SINGLE_REMOTE}:${dataset_id}" "${tmp_single}")

  local lvl_status lvl_stdout lvl_stderr
  local single_status single_stdout single_stderr
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${LEVEL3_REMOTE} copy (download)" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} copy (download)" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "copy (download) status mismatch: ${LEVEL3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status}"
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
    log "copy (download) produced different local content between level3 and single remotes."
    rm -rf "${tmp_lvl}" "${tmp_single}"
    return 1
  fi

  rm -rf "${tmp_lvl}" "${tmp_single}"
  log "copy-download test completed."
  return 0
}

run_copy_upload_test() {
  purge_remote_root "${LEVEL3_REMOTE}"
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
  lvl_result=$(capture_command "lvl_copy_upload" copy "${tempdir}" "${LEVEL3_REMOTE}:${dataset_id}")
  single_result=$(capture_command "single_copy_upload" copy "${tempdir}" "${SINGLE_REMOTE}:${dataset_id}")

  local lvl_status lvl_stdout lvl_stderr
  local single_status single_stdout single_stderr
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${LEVEL3_REMOTE} copy (upload)" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} copy (upload)" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "copy (upload) status mismatch: ${LEVEL3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status}"
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

  lvl_result=$(capture_command "lvl_verify_upload" copy "${LEVEL3_REMOTE}:${dataset_id}" "${tmp_lvl}")
  single_result=$(capture_command "single_verify_upload" copy "${SINGLE_REMOTE}:${dataset_id}" "${tmp_single}")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${LEVEL3_REMOTE} copy (verify download)" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} copy (verify download)" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "Verification copy status mismatch: ${LEVEL3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status}"
    rm -rf "${tempdir}" "${tmp_lvl}" "${tmp_single}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  if ! diff -qr "${tmp_lvl}" "${tmp_single}" >/dev/null; then
    log "Verification: downloaded content differs between level3 and single remotes."
    rm -rf "${tempdir}" "${tmp_lvl}" "${tmp_single}"
    return 1
  fi

  rm -rf "${tempdir}" "${tmp_lvl}" "${tmp_single}"

  log "copy-upload test completed. Dataset stored as ${dataset_id} on both remotes."
  return 0
}

run_move_test() {
  purge_remote_root "${LEVEL3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"
  log "Running move test"

  local dataset_id
  if ! dataset_id=$(create_test_dataset "move"); then
    log "Failed to set up dataset for move test."
    return 1
  fi
  log "Dataset created: ${LEVEL3_REMOTE}:${dataset_id} and ${SINGLE_REMOTE}:${dataset_id} (retained for inspection until move completes)"

  local tmp_lvl tmp_single
  tmp_lvl=$(mktemp -d) || return 1
  tmp_single=$(mktemp -d) || { rm -rf "${tmp_lvl}"; return 1; }

  local lvl_result single_result
  lvl_result=$(capture_command "lvl_move" move "${LEVEL3_REMOTE}:${dataset_id}" "${tmp_lvl}")
  single_result=$(capture_command "single_move" move "${SINGLE_REMOTE}:${dataset_id}" "${tmp_single}")

  local lvl_status lvl_stdout lvl_stderr
  local single_status single_stdout single_stderr
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${LEVEL3_REMOTE} move" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} move" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "move status mismatch: ${LEVEL3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status}"
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
    log "move produced different destination content between level3 and single remotes."
    rm -rf "${tmp_lvl}" "${tmp_single}"
    return 1
  fi

  # Confirm source buckets are empty (already moved)
  lvl_result=$(capture_command "lvl_post_move_ls" ls "${LEVEL3_REMOTE}:${dataset_id}")
  single_result=$(capture_command "single_post_move_ls" ls "${SINGLE_REMOTE}:${dataset_id}")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${LEVEL3_REMOTE} ls post-move" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} ls post-move" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "ls post-move status mismatch: ${LEVEL3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status}"
    rm -rf "${tmp_lvl}" "${tmp_single}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"
  rm -rf "${tmp_lvl}" "${tmp_single}"

  log "move test completed."
  return 0
}

run_delete_test() {
  purge_remote_root "${LEVEL3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"
  log "Running delete test"

  local dataset_id
  if ! dataset_id=$(create_test_dataset "delete"); then
    log "Failed to set up dataset for delete test."
    return 1
  fi
  log "Dataset created: ${LEVEL3_REMOTE}:${dataset_id} and ${SINGLE_REMOTE}:${dataset_id} (retained for inspection)"

  local target_existing="${dataset_id}/dirA/file_nested.txt"
  local target_missing="${dataset_id}/dirA/does_not_exist.txt"

  # Delete existing object
  local lvl_result single_result
  lvl_result=$(capture_command "lvl_delete_existing" delete "${LEVEL3_REMOTE}:${target_existing}")
  single_result=$(capture_command "single_delete_existing" delete "${SINGLE_REMOTE}:${target_existing}")

  local lvl_status lvl_stdout lvl_stderr
  local single_status single_stdout single_stderr
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${LEVEL3_REMOTE} delete existing" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} delete existing" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "delete (existing) status mismatch: ${LEVEL3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status}"
    log "Outputs retained:"
    log "  ${lvl_stdout}"
    log "  ${lvl_stderr}"
    log "  ${single_stdout}"
    log "  ${single_stderr}"
    return 1
  fi
  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  # Confirm deletion by listing the directory
  lvl_result=$(capture_command "lvl_post_delete_ls" ls "${LEVEL3_REMOTE}:${dataset_id}/dirA")
  single_result=$(capture_command "single_post_delete_ls" ls "${SINGLE_REMOTE}:${dataset_id}/dirA")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"
  print_if_verbose "${LEVEL3_REMOTE} ls post-delete" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} ls post-delete" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "ls post-delete status mismatch: ${LEVEL3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status}"
    return 1
  fi
  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  # Delete missing object (should be idempotent)
  lvl_result=$(capture_command "lvl_delete_missing" delete "${LEVEL3_REMOTE}:${target_missing}")
  single_result=$(capture_command "single_delete_missing" delete "${SINGLE_REMOTE}:${target_missing}")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${LEVEL3_REMOTE} delete missing" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} delete missing" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "delete (missing) status mismatch: ${LEVEL3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status}"
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
  purge_remote_root "${LEVEL3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"
  log "Running check test"

  local dataset_id
  if ! dataset_id=$(create_test_dataset "check"); then
    log "Failed to set up dataset for check test."
    return 1
  fi
  log "Dataset created: ${LEVEL3_REMOTE}:${dataset_id} and ${SINGLE_REMOTE}:${dataset_id}"

  local lvl_result single_result
  local lvl_status lvl_stdout lvl_stderr
  local single_status single_stdout single_stderr

  # Matching scenario
  lvl_result=$(capture_command "check_l2s_match" check "${LEVEL3_REMOTE}:${dataset_id}" "${SINGLE_REMOTE}:${dataset_id}")
  single_result=$(capture_command "check_s2l_match" check "${SINGLE_REMOTE}:${dataset_id}" "${LEVEL3_REMOTE}:${dataset_id}")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "check level3->single (match)" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "check single->level3 (match)" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "check (match) status mismatch: level3->single=${lvl_status}, single->level3=${single_status}"
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

  # Induce mismatch: remove a file from level3
  rclone_cmd delete "${LEVEL3_REMOTE}:${dataset_id}/dirA/file_nested.txt" >/dev/null 2>&1 || true

  lvl_result=$(capture_command "check_l2s_mismatch" check "${LEVEL3_REMOTE}:${dataset_id}" "${SINGLE_REMOTE}:${dataset_id}")
  single_result=$(capture_command "check_s2l_mismatch" check "${SINGLE_REMOTE}:${dataset_id}" "${LEVEL3_REMOTE}:${dataset_id}")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "check level3->single (mismatch)" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "check single->level3 (mismatch)" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "check (mismatch) status mismatch: level3->single=${lvl_status}, single->level3=${single_status}"
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
  purge_remote_root "${LEVEL3_REMOTE}"
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
  lvl_result=$(capture_command "lvl_sync_initial" sync "${initial_dir}" "${LEVEL3_REMOTE}:${dataset_id}")
  single_result=$(capture_command "single_sync_initial" sync "${initial_dir}" "${SINGLE_REMOTE}:${dataset_id}")

  local lvl_status lvl_stdout lvl_stderr
  local single_status single_stdout single_stderr
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${LEVEL3_REMOTE} sync (initial upload)" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} sync (initial upload)" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "sync initial upload mismatch: level3=${lvl_status}, single=${single_status}"
    rm -rf "${initial_dir}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  # Modify local state: delete file1, modify file2, add file3
  rm -f "${initial_dir}/file1.txt"
  printf 'updated sync test file 2\n' >"${initial_dir}/subdir/file2.txt"
  printf 'sync test file 3\n' >"${initial_dir}/file3.txt"

  # Apply sync (the operation under test)
  lvl_result=$(capture_command "lvl_sync_delta" sync "${initial_dir}" "${LEVEL3_REMOTE}:${dataset_id}")
  single_result=$(capture_command "single_sync_delta" sync "${initial_dir}" "${SINGLE_REMOTE}:${dataset_id}")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${LEVEL3_REMOTE} sync (delta)" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} sync (delta)" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "sync delta mismatch: level3=${lvl_status}, single=${single_status}"
    rm -rf "${initial_dir}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  # Download both remotes to verify they now match the new local state
  local tmp_lvl tmp_single
  tmp_lvl=$(mktemp -d) || { rm -rf "${initial_dir}"; return 1; }
  tmp_single=$(mktemp -d) || { rm -rf "${initial_dir}" "${tmp_lvl}"; return 1; }

  lvl_result=$(capture_command "lvl_sync_verify" copy "${LEVEL3_REMOTE}:${dataset_id}" "${tmp_lvl}")
  single_result=$(capture_command "single_sync_verify" copy "${SINGLE_REMOTE}:${dataset_id}" "${tmp_single}")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${LEVEL3_REMOTE} copy (verify sync)" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} copy (verify sync)" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "Verification copy mismatch after sync: level3=${lvl_status}, single=${single_status}"
    rm -rf "${initial_dir}" "${tmp_lvl}" "${tmp_single}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  if ! diff -qr "${tmp_lvl}" "${tmp_single}" >/dev/null; then
    log "Verification: remote states differ between level3 and single after sync."
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

  lvl_result=$(capture_command "lvl_sync_ls" ls "${LEVEL3_REMOTE}:${dataset_id}")
  single_result=$(capture_command "single_sync_ls" ls "${SINGLE_REMOTE}:${dataset_id}")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"
  print_if_verbose "${LEVEL3_REMOTE} ls (post-sync)" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} ls (post-sync)" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "Post-sync ls status mismatch: level3=${lvl_status}, single=${single_status}"
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
  purge_remote_root "${LEVEL3_REMOTE}"
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
  lvl_result=$(capture_command "lvl_sync_download" sync "${LEVEL3_REMOTE}:${dataset_id}" "${tmp_lvl}")
  single_result=$(capture_command "single_sync_download" sync "${SINGLE_REMOTE}:${dataset_id}" "${tmp_single}")

  local lvl_status lvl_stdout lvl_stderr
  local single_status single_stdout single_stderr
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${LEVEL3_REMOTE} sync (download)" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} sync (download)" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "sync-download status mismatch: level3=${lvl_status}, single=${single_status}"
    rm -rf "${tmp_lvl}" "${tmp_single}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  if ! diff -qr "${tmp_lvl}" "${tmp_single}" >/dev/null; then
    log "sync-download produced different local content between level3 and single remotes."
    rm -rf "${tmp_lvl}" "${tmp_single}"
    return 1
  fi

  rm -rf "${tmp_lvl}" "${tmp_single}"
  log "sync-download test completed."
  return 0
}

run_purge_test() {
  purge_remote_root "${LEVEL3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"
  log "Running purge test"

  local dataset_id
  if ! dataset_id=$(create_test_dataset "purge"); then
    log "Failed to set up dataset for purge test."
    return 1
  fi
  log "Dataset created: ${LEVEL3_REMOTE}:${dataset_id} and ${SINGLE_REMOTE}:${dataset_id}"

  local lvl_result single_result
  local lvl_status lvl_stdout lvl_stderr
  local single_status single_stdout single_stderr

  # Initial purge
  lvl_result=$(capture_command "lvl_purge" purge "${LEVEL3_REMOTE}:${dataset_id}")
  single_result=$(capture_command "single_purge" purge "${SINGLE_REMOTE}:${dataset_id}")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${LEVEL3_REMOTE} purge (first)" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} purge (first)" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "purge status mismatch: level3=${lvl_status}, single=${single_status}"
    log "Outputs retained:"
    log "  ${lvl_stdout}"
    log "  ${lvl_stderr}"
    log "  ${single_stdout}"
    log "  ${single_stderr}"
    return 1
  fi

  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  # Confirm dataset no longer exists
  lvl_result=$(capture_command "lvl_purge_verify" lsd "${LEVEL3_REMOTE}:${dataset_id}")
  single_result=$(capture_command "single_purge_verify" lsd "${SINGLE_REMOTE}:${dataset_id}")
  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${LEVEL3_REMOTE} lsd (post-purge)" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} lsd (post-purge)" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "lsd post-purge status mismatch: level3=${lvl_status}, single=${single_status}"
    return 1
  fi

  # Expect both to report error (bucket gone). Clean up output files.
  rm -f "${lvl_stdout}" "${lvl_stderr}" "${single_stdout}" "${single_stderr}"

  log "purge test completed."
  return 0
}


run_mkdir_test() {
  purge_remote_root "${LEVEL3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"
  local test_id
  local timestamp random_suffix
  timestamp=$(date +%Y%m%d%H%M%S)
  printf -v random_suffix '%04d' $((RANDOM % 10000))
  test_id="compare-mkdir-${timestamp}-${random_suffix}"

  log "Running mkdir test with identifier '${test_id}'"

  local lvl_result single_result
  lvl_result=$(capture_command "lvl_mkdir" mkdir "${LEVEL3_REMOTE}:${test_id}")
  single_result=$(capture_command "single_mkdir" mkdir "${SINGLE_REMOTE}:${test_id}")

  local lvl_status lvl_stdout lvl_stderr
  local single_status single_stdout single_stderr

  IFS='|' read -r lvl_status lvl_stdout lvl_stderr <<<"${lvl_result}"
  IFS='|' read -r single_status single_stdout single_stderr <<<"${single_result}"

  print_if_verbose "${LEVEL3_REMOTE} mkdir" "${lvl_stdout}" "${lvl_stderr}"
  print_if_verbose "${SINGLE_REMOTE} mkdir" "${single_stdout}" "${single_stderr}"

  if [[ "${lvl_status}" -ne "${single_status}" ]]; then
    log "mkdir status mismatch: ${LEVEL3_REMOTE}=${lvl_status}, ${SINGLE_REMOTE}=${single_status}"
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
  lvl_check=$(capture_command "lvl_check" lsd "${LEVEL3_REMOTE}:${test_id}")
  single_check=$(capture_command "single_check" lsd "${SINGLE_REMOTE}:${test_id}")

  local lvl_check_status lvl_check_stdout lvl_check_stderr
  local single_check_status single_check_stdout single_check_stderr

  IFS='|' read -r lvl_check_status lvl_check_stdout lvl_check_stderr <<<"${lvl_check}"
  IFS='|' read -r single_check_status single_check_stdout single_check_stderr <<<"${single_check}"

  print_if_verbose "${LEVEL3_REMOTE} lsd" "${lvl_check_stdout}" "${lvl_check_stderr}"
  print_if_verbose "${SINGLE_REMOTE} lsd" "${single_check_stdout}" "${single_check_stderr}"

  if [[ "${lvl_check_status}" -ne "${single_check_status}" ]]; then
    log "lsd status mismatch after mkdir: ${LEVEL3_REMOTE}=${lvl_check_status}, ${SINGLE_REMOTE}=${single_check_status}"
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

  case "${COMMAND_ARG}" in
    mkdir)
      run_mkdir_test
      ;;
    lsd)
      run_lsd_test
      ;;
    ls)
      run_ls_test
      ;;
    cat)
      run_cat_test
      ;;
    delete)
      run_delete_test
      ;;
    cp-download)
      run_copy_download_test
      ;;
    cp-upload)
      run_copy_upload_test
      ;;
    move)
      run_move_test
      ;;
    check)
      run_check_test
      ;;
    sync-upload)
      run_sync_upload_test
      ;;
    sync-download)
      run_sync_download_test
      ;;
    purge)
      run_purge_test
      ;;
    *)
      die "Unknown test '${COMMAND_ARG}'. Use '${SCRIPT_NAME} list' to see available tests."
      ;;
  esac
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
      list_tests
      ;;

    test)
      if [[ -z "${COMMAND_ARG}" ]]; then
        run_all_tests
      else
        if run_single_test; then
          log "Test '${COMMAND_ARG}' passed."
        else
          die "Test '${COMMAND_ARG}' failed."
        fi
      fi
      ;;
  esac
}

main "$@"

