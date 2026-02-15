#!/usr/bin/env bash
#
# compare_raid3_with_single_stacking.sh
# --------------------------------------
# Integration test for stacking virtual remotes with raid3.
#
# This script tests that wrapping a rclone backend with another rclone backend
# works correctly with raid3. Specifically, it tests:
# - crypt backend wrapping localsingle backend
# - crypt backend wrapping localraid3 backend
# - Upload to both crypt backends
# - Verification that encrypted particle files are created correctly
# - Download from both crypt backends
# - Verification that downloaded files are identical
#
# Usage:
#   compare_raid3_with_single_stacking.sh [options] <command> [args]
#
# Commands:
#   start                 Start the MinIO containers (not used for local storage type).
#   stop                  Stop those MinIO containers.
#   teardown              Purge all data from the selected storage-type.
#   list                  Show available test cases.
#   test <name>           Run a named test (e.g. "crypt" or "chunker").
#
# Options:
#   --storage-type <local|minio>   Select which backend pair to exercise.
#                                  Required for start/stop/test/teardown.
#                                  Both "local" and "minio" are supported.
#   -v, --verbose                  Show stdout/stderr from all rclone invocations.
#   -h, --help                     Display this help text.
#
# Environment:
#   RCLONE_CONFIG   Path to rclone configuration file.
#                   Defaults to test-specific config file.
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

# Crypt remote names (will be set based on storage type)
CRYPT_SINGLE_REMOTE=""
CRYPT_RAID3_REMOTE=""
# Underlying remotes (will be set based on storage type)
SINGLE_REMOTE=""
RAID3_REMOTE=""
# Individual backend remotes (will be set based on storage type)
EVEN_REMOTE=""
ODD_REMOTE=""
PARITY_REMOTE=""
# Chunker remote names for chunker test (will be set based on storage type)
CHUNKER_SINGLE_REMOTE=""
CHUNKER_RAID3_REMOTE=""

# ---------------------------- helper functions ------------------------------

# Set remotes based on storage type
set_stacking_remotes() {
  set_remotes_for_storage_type
  
  case "${STORAGE_TYPE}" in
    local)
      CRYPT_SINGLE_REMOTE="cryptlocalsingle"
      CRYPT_RAID3_REMOTE="cryptlocalraid3"
      EVEN_REMOTE="${LOCAL_EVEN_REMOTE}"
      ODD_REMOTE="${LOCAL_ODD_REMOTE}"
      PARITY_REMOTE="${LOCAL_PARITY_REMOTE}"
      ;;
    minio)
      CRYPT_SINGLE_REMOTE="cryptminiosingle"
      CRYPT_RAID3_REMOTE="cryptminioraid3"
      EVEN_REMOTE="${MINIO_EVEN_REMOTE}"
      ODD_REMOTE="${MINIO_ODD_REMOTE}"
      PARITY_REMOTE="${MINIO_PARITY_REMOTE}"
      ;;
    *)
      die "Unsupported storage type '${STORAGE_TYPE}' for crypt test"
      ;;
  esac
  
  # Verify that the crypt remotes exist in the config
  if ! grep -q "^\[${CRYPT_SINGLE_REMOTE}\]" "${RCLONE_CONFIG}" 2>/dev/null; then
    die "Crypt remote '${CRYPT_SINGLE_REMOTE}' not found in config file. Please run setup.sh to regenerate the config."
  fi
  if ! grep -q "^\[${CRYPT_RAID3_REMOTE}\]" "${RCLONE_CONFIG}" 2>/dev/null; then
    die "Crypt remote '${CRYPT_RAID3_REMOTE}' not found in config file. Please run setup.sh to regenerate the config."
  fi
}

# Set chunker remotes for chunker test (call after set_stacking_remotes)
set_stacking_chunker_remotes() {
  case "${STORAGE_TYPE}" in
    local)
      CHUNKER_SINGLE_REMOTE="chunkerlocalsingle"
      CHUNKER_RAID3_REMOTE="chunkerlocalraid3"
      ;;
    minio)
      CHUNKER_SINGLE_REMOTE="chunkerminiosingle"
      CHUNKER_RAID3_REMOTE="chunkerminioraid3"
      ;;
    *)
      die "Unsupported storage type '${STORAGE_TYPE}' for chunker test"
      ;;
  esac

  if ! grep -q "^\[${CHUNKER_SINGLE_REMOTE}\]" "${RCLONE_CONFIG}" 2>/dev/null; then
    die "Chunker remote '${CHUNKER_SINGLE_REMOTE}' not found in config file. Please run setup.sh to regenerate the config."
  fi
  if ! grep -q "^\[${CHUNKER_RAID3_REMOTE}\]" "${RCLONE_CONFIG}" 2>/dev/null; then
    die "Chunker remote '${CHUNKER_RAID3_REMOTE}' not found in config file. Please run setup.sh to regenerate the config."
  fi
}

usage() {
  cat <<EOF
Usage: ${SCRIPT_NAME} [options] <command> [arguments]

Commands:
  start                      Start MinIO containers (requires --storage-type=minio).
  stop                       Stop MinIO containers (requires --storage-type=minio).
  teardown                   Purge all test data for the selected storage type.
  list                       Show available tests.
  test <name>                Run the named test (e.g. "crypt" or "chunker").

Options:
  --storage-type <local|minio>   Select backend pair (required for start/stop/test/teardown).
                                  Both "local" and "minio" are supported.
  -v, --verbose                  Show stdout/stderr from all rclone invocations.
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
        if [[ "$1" == "-v" || "$1" == "--verbose" ]]; then
          VERBOSE=1
        elif [[ "$1" == "-" && $# -gt 1 && "$2" == "v" ]]; then
          # "test - v" typo: treat as -v (consume "-" and "v")
          shift
          shift
          VERBOSE=1
          continue
        elif [[ "$1" == "-" ]]; then
          # Skip bare "-"
          :
        elif [[ "${COMMAND}" == "test" && -z "${COMMAND_ARG}" ]]; then
          COMMAND_ARG="$1"
        else
          die "Unknown argument: $1 (use -v before the command, e.g. --storage-type minio -v test)"
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
  crypt    Test crypt backend wrapping localsingle and localraid3 backends.
           Uploads a file to both crypt backends, verifies encrypted particle
           files are created correctly, downloads from both, and verifies
           the downloaded files are identical.
  chunker  Test chunker over single/raid3; file is chunked into >=2 chunks.
           Uploads a file to both chunker remotes, verifies chunk+meta files
           in underlying storage, downloads from both, and verifies identical.
EOF
}

# ------------------------------ test helpers --------------------------------

# Get relative path for a directory (relative to test directory)
# This matches the logic used in create_rclone_config
get_relative_path() {
  local abs_path="$1"
  local test_dir="${SCRIPT_DIR}"
  # If path is within test directory, make it relative
  if [[ "${abs_path}" == "${test_dir}"/* ]]; then
    echo "${abs_path#"${test_dir}"/}"
  else
    # If path is outside test directory, keep absolute (shouldn't happen in normal case)
    echo "${abs_path}"
  fi
}

# Count files in underlying storage using rclone ls with the correct path
# This works correctly even when crypt encrypts directory names
# The local remotes need the path suffix to point to the data directory
count_files_in_remote() {
  local remote="$1"
  local path_suffix="$2"  # Optional path suffix (e.g., "_data/even_local")
  local count=0
  
  # Construct the full remote path
  local remote_path="${remote}:"
  if [[ -n "${path_suffix}" ]]; then
    remote_path="${remote}:${path_suffix}"
  fi
  
  # Use rclone ls to list all files recursively
  # This will show files even if directory names are encrypted
  # Suppress "directory not found" errors (expected when directories don't exist yet)
  # rclone ls format: "    <size> <path>" (spaces, then number, then space, then path)
  local ls_output
  ls_output=$(rclone_cmd ls "${remote_path}" 2>&1 | grep -v "directory not found" | grep -v "Failed to ls" | grep -v "^ERROR" | grep -v "^NOTICE" || echo "")
  # Count lines that contain numbers (file listings) - rclone ls outputs "    <size> <path>"
  # Exclude .DS_Store files (macOS Finder metadata files)
  if [[ -n "${ls_output}" ]]; then
    # Filter to only lines that contain a number (file size), which indicates a file listing
    # Exclude .DS_Store files which are macOS system files
    count=$(echo "${ls_output}" | grep -v "\.DS_Store" | grep -E "[0-9]" | grep -c . 2>/dev/null || echo "0")
  fi
  
  echo "${count}"
}

# Count particle files for raid3 storage
# Returns the total count of files across all three backends (even, odd, parity)
count_raid3_particles() {
  local even_count odd_count parity_count
  
  if [[ "${STORAGE_TYPE}" == "local" ]]; then
    # For local storage, need path suffixes
    local even_dir_rel odd_dir_rel parity_dir_rel
    even_dir_rel=$(get_relative_path "${LOCAL_EVEN_DIR}")
    odd_dir_rel=$(get_relative_path "${LOCAL_ODD_DIR}")
    parity_dir_rel=$(get_relative_path "${LOCAL_PARITY_DIR}")
    
    even_count=$(count_files_in_remote "${EVEN_REMOTE}" "${even_dir_rel}")
    odd_count=$(count_files_in_remote "${ODD_REMOTE}" "${odd_dir_rel}")
    parity_count=$(count_files_in_remote "${PARITY_REMOTE}" "${parity_dir_rel}")
  else
    # For minio (S3), no path suffixes needed
    even_count=$(count_files_in_remote "${EVEN_REMOTE}" "")
    odd_count=$(count_files_in_remote "${ODD_REMOTE}" "")
    parity_count=$(count_files_in_remote "${PARITY_REMOTE}" "")
  fi
  
  echo "$((even_count + odd_count + parity_count))"
}

# Count files for single storage
count_single_files() {
  # For local: localsingle is an alias remote, no path suffix needed
  # For minio: miniosingle is S3, no path suffix needed
  count_files_in_remote "${SINGLE_REMOTE}" ""
}

# Count files in a specific raid3 backend (even, odd, or parity)
count_raid3_backend_files() {
  local backend="$1"  # "even", "odd", or "parity"
  local remote dir_rel=""
  
  case "${backend}" in
    even) remote="${EVEN_REMOTE}" ;;
    odd) remote="${ODD_REMOTE}" ;;
    parity) remote="${PARITY_REMOTE}" ;;
    *) echo "0"; return 1 ;;
  esac
  
  if [[ "${STORAGE_TYPE}" == "local" ]]; then
    # For local storage, need path suffixes
    case "${backend}" in
      even) dir_rel=$(get_relative_path "${LOCAL_EVEN_DIR}") ;;
      odd) dir_rel=$(get_relative_path "${LOCAL_ODD_DIR}") ;;
      parity) dir_rel=$(get_relative_path "${LOCAL_PARITY_DIR}") ;;
    esac
  fi
  # For minio (S3), dir_rel remains empty
  
  count_files_in_remote "${remote}" "${dir_rel}"
}

run_stacking_test() {
  # Set remotes based on storage type
  set_stacking_remotes
  
  # For minio, ensure containers are ready
  if [[ "${STORAGE_TYPE}" == "minio" ]]; then
    log "Waiting for MinIO backends to be ready"
    wait_for_minio_backend_ready "even"
    wait_for_minio_backend_ready "odd"
    wait_for_minio_backend_ready "parity"
    # Wait for single MinIO backend
    local single_port="${MINIO_SINGLE_PORT}"
    local max_attempts=30
    local attempt=0
    while [[ ${attempt} -lt ${max_attempts} ]]; do
      if curl -s "http://127.0.0.1:${single_port}/minio/health/live" >/dev/null 2>&1; then
        break
      fi
      sleep 1
      attempt=$((attempt + 1))
    done
    if [[ ${attempt} -eq ${max_attempts} ]]; then
      log "WARNING: MinIO single backend may not be ready"
    fi
  fi
  
  # Purge remotes to ensure clean state for this test
  # Note: We only purge the composite remotes (crypt and raid3), not the individual
  # backends (even, odd, parity) as they are shared across all tests
  log "Purging remotes for clean test state"
  purge_remote_root "${CRYPT_SINGLE_REMOTE}"
  purge_remote_root "${CRYPT_RAID3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"
  purge_raid3_remote_root
  
  log "Running crypt test"
  
  # Count baseline files in underlying storage AFTER purge
  # After purging the composite remotes, the underlying storage should be empty
  # (or contain only files from other tests that use the underlying remotes directly)
  log "Counting baseline files in underlying storage after purge"
  
  # Debug: Show what rclone ls is finding (with correct paths)
  if (( VERBOSE )); then
    log "DEBUG: Checking what files exist in underlying remotes:"
    if [[ "${STORAGE_TYPE}" == "local" ]]; then
      local even_dir_rel odd_dir_rel parity_dir_rel
      even_dir_rel=$(get_relative_path "${LOCAL_EVEN_DIR}")
      odd_dir_rel=$(get_relative_path "${LOCAL_ODD_DIR}")
      parity_dir_rel=$(get_relative_path "${LOCAL_PARITY_DIR}")
      
      log "DEBUG: ${EVEN_REMOTE}:${even_dir_rel}"
      rclone_cmd ls "${EVEN_REMOTE}:${even_dir_rel}" 2>&1 | grep -v "directory not found" | grep -v "Failed to ls" | grep -v "^ERROR" | grep -v "^NOTICE" | head -20 || true
      log "DEBUG: ${ODD_REMOTE}:${odd_dir_rel}"
      rclone_cmd ls "${ODD_REMOTE}:${odd_dir_rel}" 2>&1 | grep -v "directory not found" | grep -v "Failed to ls" | grep -v "^ERROR" | grep -v "^NOTICE" | head -20 || true
      log "DEBUG: ${PARITY_REMOTE}:${parity_dir_rel}"
      rclone_cmd ls "${PARITY_REMOTE}:${parity_dir_rel}" 2>&1 | grep -v "directory not found" | grep -v "Failed to ls" | grep -v "^ERROR" | grep -v "^NOTICE" | head -20 || true
      log "DEBUG: ${SINGLE_REMOTE}:"
      rclone_cmd ls "${SINGLE_REMOTE}:" 2>&1 | grep -v "directory not found" | grep -v "Failed to ls" | grep -v "^ERROR" | grep -v "^NOTICE" | head -20 || true
    else
      # For minio, no path suffixes
      log "DEBUG: ${EVEN_REMOTE}:"
      rclone_cmd ls "${EVEN_REMOTE}:" 2>&1 | grep -v "directory not found" | grep -v "Failed to ls" | grep -v "^ERROR" | grep -v "^NOTICE" | head -20 || true
      log "DEBUG: ${ODD_REMOTE}:"
      rclone_cmd ls "${ODD_REMOTE}:" 2>&1 | grep -v "directory not found" | grep -v "Failed to ls" | grep -v "^ERROR" | grep -v "^NOTICE" | head -20 || true
      log "DEBUG: ${PARITY_REMOTE}:"
      rclone_cmd ls "${PARITY_REMOTE}:" 2>&1 | grep -v "directory not found" | grep -v "Failed to ls" | grep -v "^ERROR" | grep -v "^NOTICE" | head -20 || true
      log "DEBUG: ${SINGLE_REMOTE}:"
      rclone_cmd ls "${SINGLE_REMOTE}:" 2>&1 | grep -v "directory not found" | grep -v "Failed to ls" | grep -v "^ERROR" | grep -v "^NOTICE" | head -20 || true
    fi
  fi
  
  local single_baseline raid3_baseline
  local even_baseline odd_baseline parity_baseline
  single_baseline=$(count_single_files)
  raid3_baseline=$(count_raid3_particles)
  even_baseline=$(count_raid3_backend_files "even")
  odd_baseline=$(count_raid3_backend_files "odd")
  parity_baseline=$(count_raid3_backend_files "parity")
  
  log "Baseline after purge: ${single_baseline} file(s) in ${SINGLE_REMOTE}, ${raid3_baseline} file(s) in ${RAID3_REMOTE} backends (even: ${even_baseline}, odd: ${odd_baseline}, parity: ${parity_baseline})"
  
  # Create a test file to upload
  local tempdir
  tempdir=$(mktemp -d) || return 1
  
  local test_file="test_crypt_file.txt"
  local test_content="This is a test file for crypt backend wrapping single/raid3.
It contains multiple lines to ensure proper encryption and particle splitting.
Line 2: Testing crypt backend wrapping.
Line 3: Testing raid3 backend with crypt overlay.
Line 4: This file should be encrypted and split into particles correctly."
  
  printf '%s\n' "${test_content}" >"${tempdir}/${test_file}"
  
  local dataset_id
  dataset_id=$(date +crypt-%Y%m%d%H%M%S-$((RANDOM % 10000)))
  
  log "Uploading test file to ${CRYPT_SINGLE_REMOTE}:${dataset_id}"
  local single_upload_result single_upload_status single_upload_stdout single_upload_stderr
  single_upload_result=$(capture_command "cryptsingle_upload" copy "${tempdir}/${test_file}" "${CRYPT_SINGLE_REMOTE}:${dataset_id}/${test_file}")
  IFS='|' read -r single_upload_status single_upload_stdout single_upload_stderr <<<"${single_upload_result}"
  
  print_if_verbose "${CRYPT_SINGLE_REMOTE} upload" "${single_upload_stdout}" "${single_upload_stderr}"
  
  if [[ "${single_upload_status}" -ne 0 ]]; then
    log "Upload to ${CRYPT_SINGLE_REMOTE} failed with status ${single_upload_status}"
    rm -f "${single_upload_stdout}" "${single_upload_stderr}"
    rm -rf "${tempdir}"
    return 1
  fi
  rm -f "${single_upload_stdout}" "${single_upload_stderr}"
  
  log "Uploading test file to ${CRYPT_RAID3_REMOTE}:${dataset_id}"
  local raid3_upload_result raid3_upload_status raid3_upload_stdout raid3_upload_stderr
  raid3_upload_result=$(capture_command "cryptraid3_upload" copy "${tempdir}/${test_file}" "${CRYPT_RAID3_REMOTE}:${dataset_id}/${test_file}")
  IFS='|' read -r raid3_upload_status raid3_upload_stdout raid3_upload_stderr <<<"${raid3_upload_result}"
  
  print_if_verbose "${CRYPT_RAID3_REMOTE} upload" "${raid3_upload_stdout}" "${raid3_upload_stderr}"
  
  if [[ "${raid3_upload_status}" -ne 0 ]]; then
    log "Upload to ${CRYPT_RAID3_REMOTE} failed with status ${raid3_upload_status}"
    rm -f "${raid3_upload_stdout}" "${raid3_upload_stderr}"
    rm -rf "${tempdir}"
    return 1
  fi
  rm -f "${raid3_upload_stdout}" "${raid3_upload_stderr}"
  
  # Wait a moment for filesystem operations to complete
  sleep 1
  
  # Check that encrypted particle files exist in underlying storage
  # Count files AFTER upload and verify absolute counts
  log "Checking encrypted particle files in underlying storage"
  
  local single_file_count raid3_particle_count
  single_file_count=$(count_single_files)
  raid3_particle_count=$(count_raid3_particles)
  
  # Calculate expected counts (baseline + files added by this test)
  local expected_single=$((single_baseline + 1))
  local expected_raid3=$((raid3_baseline + 3))
  
  log "Found ${single_file_count} file(s) in ${SINGLE_REMOTE} (baseline: ${single_baseline}, expected: ${expected_single})"
  log "Found ${raid3_particle_count} file(s) in ${RAID3_REMOTE} backends (baseline: ${raid3_baseline}, expected: ${expected_raid3})"
  
  if [[ "${single_file_count}" -ne "${expected_single}" ]]; then
    log "Expected ${expected_single} encrypted file(s) in ${SINGLE_REMOTE} (baseline: ${single_baseline} + 1 from this test), found ${single_file_count}"
    rm -rf "${tempdir}"
    return 1
  fi
  
  if [[ "${raid3_particle_count}" -ne "${expected_raid3}" ]]; then
    log "Expected ${expected_raid3} encrypted particle file(s) in ${RAID3_REMOTE} backends (baseline: ${raid3_baseline} + 3 from this test), found ${raid3_particle_count}"
    log "Checking individual backends:"
    local even_after odd_after parity_after
    local expected_even expected_odd expected_parity
    even_after=$(count_raid3_backend_files "even")
    odd_after=$(count_raid3_backend_files "odd")
    parity_after=$(count_raid3_backend_files "parity")
    expected_even=$((even_baseline + 1))
    expected_odd=$((odd_baseline + 1))
    expected_parity=$((parity_baseline + 1))
    log "  even: ${even_after} (baseline: ${even_baseline}, expected: ${expected_even})"
    log "  odd: ${odd_after} (baseline: ${odd_baseline}, expected: ${expected_odd})"
    log "  parity: ${parity_after} (baseline: ${parity_baseline}, expected: ${expected_parity})"
    rm -rf "${tempdir}"
    return 1
  fi
  
  log "Particle file count verification passed"
  
  # Download from both crypt backends
  local tmp_single tmp_raid3
  tmp_single=$(mktemp -d) || { rm -rf "${tempdir}"; return 1; }
  tmp_raid3=$(mktemp -d) || { rm -rf "${tempdir}" "${tmp_single}"; return 1; }
  
  log "Downloading from ${CRYPT_SINGLE_REMOTE}:${dataset_id}"
  local single_download_result single_download_status single_download_stdout single_download_stderr
  single_download_result=$(capture_command "cryptsingle_download" copy "${CRYPT_SINGLE_REMOTE}:${dataset_id}" "${tmp_single}")
  IFS='|' read -r single_download_status single_download_stdout single_download_stderr <<<"${single_download_result}"
  
  print_if_verbose "${CRYPT_SINGLE_REMOTE} download" "${single_download_stdout}" "${single_download_stderr}"
  
  if [[ "${single_download_status}" -ne 0 ]]; then
    log "Download from ${CRYPT_SINGLE_REMOTE} failed with status ${single_download_status}"
    rm -f "${single_download_stdout}" "${single_download_stderr}"
    rm -rf "${tempdir}" "${tmp_single}" "${tmp_raid3}"
    return 1
  fi
  rm -f "${single_download_stdout}" "${single_download_stderr}"
  
  log "Downloading from ${CRYPT_RAID3_REMOTE}:${dataset_id}"
  local raid3_download_result raid3_download_status raid3_download_stdout raid3_download_stderr
  raid3_download_result=$(capture_command "cryptraid3_download" copy "${CRYPT_RAID3_REMOTE}:${dataset_id}" "${tmp_raid3}")
  IFS='|' read -r raid3_download_status raid3_download_stdout raid3_download_stderr <<<"${raid3_download_result}"
  
  print_if_verbose "${CRYPT_RAID3_REMOTE} download" "${raid3_download_stdout}" "${raid3_download_stderr}"
  
  if [[ "${raid3_download_status}" -ne 0 ]]; then
    log "Download from ${CRYPT_RAID3_REMOTE} failed with status ${raid3_download_status}"
    rm -f "${raid3_download_stdout}" "${raid3_download_stderr}"
    rm -rf "${tempdir}" "${tmp_single}" "${tmp_raid3}"
    return 1
  fi
  rm -f "${raid3_download_stdout}" "${raid3_download_stderr}"
  
  # Verify that the downloaded files are identical
  if ! diff -qr "${tmp_single}" "${tmp_raid3}" >/dev/null; then
    log "Downloaded files differ between ${CRYPT_SINGLE_REMOTE} and ${CRYPT_RAID3_REMOTE}"
    log "Files in ${tmp_single}:"
    find "${tmp_single}" -type f -exec ls -lh {} \;
    log "Files in ${tmp_raid3}:"
    find "${tmp_raid3}" -type f -exec ls -lh {} \;
    log "Diff output:"
    diff -qr "${tmp_single}" "${tmp_raid3}" || true
    rm -rf "${tempdir}" "${tmp_single}" "${tmp_raid3}"
    return 1
  fi
  
  # Verify the content matches the original
  if ! diff -q "${tempdir}/${test_file}" "${tmp_single}/${test_file}" >/dev/null 2>&1; then
    log "Downloaded file from ${CRYPT_SINGLE_REMOTE} does not match original"
    rm -rf "${tempdir}" "${tmp_single}" "${tmp_raid3}"
    return 1
  fi
  
  if ! diff -q "${tempdir}/${test_file}" "${tmp_raid3}/${test_file}" >/dev/null 2>&1; then
    log "Downloaded file from ${CRYPT_RAID3_REMOTE} does not match original"
    rm -rf "${tempdir}" "${tmp_single}" "${tmp_raid3}"
    return 1
  fi
  
  rm -rf "${tempdir}" "${tmp_single}" "${tmp_raid3}"
  log "crypt test completed successfully"
  return 0
}

# Chunker config: chunk_size=100 so a ~250-byte file yields 3 data chunks + 1 metadata (>=2 chunks)
run_chunker_test() {
  set_stacking_remotes
  set_stacking_chunker_remotes

  if [[ "${STORAGE_TYPE}" == "minio" ]]; then
    log "Waiting for MinIO backends to be ready"
    wait_for_minio_backend_ready "even"
    wait_for_minio_backend_ready "odd"
    wait_for_minio_backend_ready "parity"
    local single_port="${MINIO_SINGLE_PORT}"
    local max_attempts=30
    local attempt=0
    while [[ ${attempt} -lt ${max_attempts} ]]; do
      if curl -s "http://127.0.0.1:${single_port}/minio/health/live" >/dev/null 2>&1; then
        break
      fi
      sleep 1
      attempt=$((attempt + 1))
    done
    if [[ ${attempt} -eq ${max_attempts} ]]; then
      log "WARNING: MinIO single backend may not be ready"
    fi
  fi

  log "Purging remotes for clean test state"
  purge_remote_root "${CHUNKER_SINGLE_REMOTE}"
  purge_remote_root "${CHUNKER_RAID3_REMOTE}"
  purge_remote_root "${SINGLE_REMOTE}"
  purge_raid3_remote_root

  log "Running chunker test"

  log "Counting baseline files in underlying storage after purge"
  local single_baseline raid3_baseline
  single_baseline=$(count_single_files)
  raid3_baseline=$(count_raid3_particles)
  log "Baseline after purge: ${single_baseline} file(s) in ${SINGLE_REMOTE}, ${raid3_baseline} file(s) in ${RAID3_REMOTE} backends"

  local tempdir
  tempdir=$(mktemp -d) || return 1

  local test_file="test_chunker_file.txt"
  # chunk_size=100B: ensure file is >100 bytes so we get >=2 data chunks + 1 metadata
  local test_content="This is a test file for chunker (chunker over single/raid3).
It contains enough bytes to be split into at least 2 chunks (chunk_size=100B).
Line 2: Testing chunker backend wrapping.
Line 3: Testing raid3 backend with chunker overlay.
Line 4: This file should be chunked and stored as multiple chunk files plus metadata.
Padding: 0123456789 0123456789 0123456789 0123456789 0123456789 0123456789 0123456789 0123456789 end."
  printf '%s\n' "${test_content}" >"${tempdir}/${test_file}"

  local dataset_id
  dataset_id=$(date +chunker-%Y%m%d%H%M%S-$((RANDOM % 10000)))

  log "Uploading test file to ${CHUNKER_SINGLE_REMOTE}:${dataset_id}"
  local single_upload_result single_upload_status single_upload_stdout single_upload_stderr
  single_upload_result=$(capture_command "chunkersingle_upload" copy "${tempdir}/${test_file}" "${CHUNKER_SINGLE_REMOTE}:${dataset_id}/${test_file}")
  IFS='|' read -r single_upload_status single_upload_stdout single_upload_stderr <<<"${single_upload_result}"
  print_if_verbose "${CHUNKER_SINGLE_REMOTE} upload" "${single_upload_stdout}" "${single_upload_stderr}"
  if [[ "${single_upload_status}" -ne 0 ]]; then
    log "Upload to ${CHUNKER_SINGLE_REMOTE} failed with status ${single_upload_status}"
    rm -f "${single_upload_stdout}" "${single_upload_stderr}"
    rm -rf "${tempdir}"
    return 1
  fi
  rm -f "${single_upload_stdout}" "${single_upload_stderr}"

  log "Uploading test file to ${CHUNKER_RAID3_REMOTE}:${dataset_id}"
  local raid3_upload_result raid3_upload_status raid3_upload_stdout raid3_upload_stderr
  raid3_upload_result=$(capture_command "chunkerraid3_upload" copy "${tempdir}/${test_file}" "${CHUNKER_RAID3_REMOTE}:${dataset_id}/${test_file}")
  IFS='|' read -r raid3_upload_status raid3_upload_stdout raid3_upload_stderr <<<"${raid3_upload_result}"
  print_if_verbose "${CHUNKER_RAID3_REMOTE} upload" "${raid3_upload_stdout}" "${raid3_upload_stderr}"
  if [[ "${raid3_upload_status}" -ne 0 ]]; then
    log "Upload to ${CHUNKER_RAID3_REMOTE} failed with status ${raid3_upload_status}"
    rm -f "${raid3_upload_stdout}" "${raid3_upload_stderr}"
    rm -rf "${tempdir}"
    return 1
  fi
  rm -f "${raid3_upload_stdout}" "${raid3_upload_stderr}"

  sleep 1

  log "Checking chunk+metadata files in underlying storage"
  local single_file_count raid3_particle_count
  single_file_count=$(count_single_files)
  raid3_particle_count=$(count_raid3_particles)

  # With chunk_size=100B, file >100 bytes: at least 2 data chunks + 1 metadata = 3 files on single, 3*3 = 9 particles on raid3
  local min_single=$((single_baseline + 3))
  local min_raid3=$((raid3_baseline + 9))

  log "Found ${single_file_count} file(s) in ${SINGLE_REMOTE} (baseline: ${single_baseline}, min expected: ${min_single})"
  log "Found ${raid3_particle_count} file(s) in ${RAID3_REMOTE} backends (baseline: ${raid3_baseline}, min expected: ${min_raid3})"

  if [[ "${single_file_count}" -lt "${min_single}" ]]; then
    log "Expected at least ${min_single} chunk+meta file(s) in ${SINGLE_REMOTE} (>=2 chunks + 1 meta), found ${single_file_count}"
    rm -rf "${tempdir}"
    return 1
  fi
  if [[ "${raid3_particle_count}" -lt "${min_raid3}" ]]; then
    log "Expected at least ${min_raid3} particle file(s) in ${RAID3_REMOTE} backends, found ${raid3_particle_count}"
    rm -rf "${tempdir}"
    return 1
  fi
  log "Chunk file count verification passed"

  local tmp_single tmp_raid3
  tmp_single=$(mktemp -d) || { rm -rf "${tempdir}"; return 1; }
  tmp_raid3=$(mktemp -d) || { rm -rf "${tempdir}" "${tmp_single}"; return 1; }

  log "Downloading from ${CHUNKER_SINGLE_REMOTE}:${dataset_id}"
  single_download_result=$(capture_command "chunkersingle_download" copy "${CHUNKER_SINGLE_REMOTE}:${dataset_id}" "${tmp_single}")
  IFS='|' read -r single_download_status single_download_stdout single_download_stderr <<<"${single_download_result}"
  print_if_verbose "${CHUNKER_SINGLE_REMOTE} download" "${single_download_stdout}" "${single_download_stderr}"
  if [[ "${single_download_status}" -ne 0 ]]; then
    log "Download from ${CHUNKER_SINGLE_REMOTE} failed with status ${single_download_status}"
    rm -f "${single_download_stdout}" "${single_download_stderr}"
    rm -rf "${tempdir}" "${tmp_single}" "${tmp_raid3}"
    return 1
  fi
  rm -f "${single_download_stdout}" "${single_download_stderr}"

  log "Downloading from ${CHUNKER_RAID3_REMOTE}:${dataset_id}"
  raid3_download_result=$(capture_command "chunkerraid3_download" copy "${CHUNKER_RAID3_REMOTE}:${dataset_id}" "${tmp_raid3}")
  IFS='|' read -r raid3_download_status raid3_download_stdout raid3_download_stderr <<<"${raid3_download_result}"
  print_if_verbose "${CHUNKER_RAID3_REMOTE} download" "${raid3_download_stdout}" "${raid3_download_stderr}"
  if [[ "${raid3_download_status}" -ne 0 ]]; then
    log "Download from ${CHUNKER_RAID3_REMOTE} failed with status ${raid3_download_status}"
    rm -f "${raid3_download_stdout}" "${raid3_download_stderr}"
    rm -rf "${tempdir}" "${tmp_single}" "${tmp_raid3}"
    return 1
  fi
  rm -f "${raid3_download_stdout}" "${raid3_download_stderr}"

  if ! diff -qr "${tmp_single}" "${tmp_raid3}" >/dev/null; then
    log "Downloaded files differ between ${CHUNKER_SINGLE_REMOTE} and ${CHUNKER_RAID3_REMOTE}"
    diff -qr "${tmp_single}" "${tmp_raid3}" || true
    rm -rf "${tempdir}" "${tmp_single}" "${tmp_raid3}"
    return 1
  fi
  if ! diff -q "${tempdir}/${test_file}" "${tmp_single}/${test_file}" >/dev/null 2>&1; then
    log "Downloaded file from ${CHUNKER_SINGLE_REMOTE} does not match original"
    rm -rf "${tempdir}" "${tmp_single}" "${tmp_raid3}"
    return 1
  fi
  if ! diff -q "${tempdir}/${test_file}" "${tmp_raid3}/${test_file}" >/dev/null 2>&1; then
    log "Downloaded file from ${CHUNKER_RAID3_REMOTE} does not match original"
    rm -rf "${tempdir}" "${tmp_single}" "${tmp_raid3}"
    return 1
  fi

  rm -rf "${tempdir}" "${tmp_single}" "${tmp_raid3}"
  log "chunker test completed successfully"
  return 0
}

run_all_tests() {
  local tests=("crypt" "chunker")
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

run_single_test() {
  set_remotes_for_storage_type
  
  local test_name="${COMMAND_ARG}"
  local test_func=""
  
  case "${test_name}" in
    crypt) test_func="run_stacking_test" ;;
    chunker) test_func="run_chunker_test" ;;
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
  ensure_rclone_binary
  ensure_rclone_config

  # Prevent rclone from hanging with MinIO (purge, list, copy can block).
  if [[ "${STORAGE_TYPE}" == "minio" ]]; then
    export RCLONE_TEST_TIMEOUT="${RCLONE_TEST_TIMEOUT:-120}"
    if (( VERBOSE )); then
      log_info "main" "Rclone command timeout: ${RCLONE_TEST_TIMEOUT}s (exit 124 = timed out)"
    fi
  fi

  case "${COMMAND}" in
    start)
      if [[ "${STORAGE_TYPE}" != "minio" ]]; then
        log_info "start" "No MinIO containers needed for storage type '${STORAGE_TYPE}'"
        return 0
      fi
      STORAGE_TYPE="minio"
      start_minio_containers
      ;;
    
    stop)
      if [[ "${STORAGE_TYPE}" != "minio" ]]; then
        log_info "stop" "No MinIO containers to stop for storage type '${STORAGE_TYPE}'"
        return 0
      fi
      STORAGE_TYPE="minio"
      stop_minio_containers
      ;;
    
    teardown)
      set_stacking_remotes
      set_stacking_chunker_remotes
      # Only purge the contents of remotes; do not remove root directories
      # (crypt/chunker/single/raid3 are virtual or underlying storage used by tests)
      log "Purging contents of test remotes (crypt, chunker, single, raid3)"
      purge_remote_root "${CRYPT_SINGLE_REMOTE}"
      purge_remote_root "${CRYPT_RAID3_REMOTE}"
      purge_remote_root "${CHUNKER_SINGLE_REMOTE}"
      purge_remote_root "${CHUNKER_RAID3_REMOTE}"
      purge_remote_root "${SINGLE_REMOTE}"
      purge_raid3_remote_root
      ;;
    
    list)
      list_tests
      ;;
    
    test)
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
