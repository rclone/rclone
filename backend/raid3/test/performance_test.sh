#!/usr/bin/env bash
#
# performance_test.sh
# -------------------
# Performance test script for rclone raid3 backend.
#
# This script benchmarks upload/download performance across different
# configurations (miniosingle/rclone, minioraid3/rclone, miniosingle/mc)
# using different file sizes (4K, 40K, 400K, 4M, 40M, 4G).
#
# Usage:
#   performance_test.sh [options] <command>
#
# Commands:
#   start                 Start MinIO containers
#   stop                  Stop MinIO containers
#   teardown              Purge all test data
#   list                  Show available test configurations
#   test                  Run all performance tests
#
# Options:
#   --storage-type <type>    Select storage type: 'minio', 'local', or 'sftp'
#   -v, --verbose            Verbose output
#   -c, --compression        Use Snappy compression for raid3 remotes (regenerates config)
#   --skip-mc                Skip mc tests (if mc not available, minio only)
#   --skip-cp                Skip cp tests (local only)
#   --iterations N            Number of iterations (default: 11)
#   -h, --help               Display this help text
#
# Safety guard: the script must be executed from backend/raid3/test directory.
# -----------------------------------------------------------------------------

# Ensure we're running with bash (not sh)
if [[ -z "${BASH_VERSION:-}" ]]; then
  echo "Error: This script requires bash. Please run with: bash $0" >&2
  exit 1
fi

set -euo pipefail

SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)

# shellcheck source=compare_raid3_with_single_common.sh
# shellcheck disable=SC1091
. "${SCRIPT_DIR}/compare_raid3_with_single_common.sh"

VERBOSE=0
SKIP_MC=0
SKIP_CP=0
ITERATIONS=11
STORAGE_TYPE=""
COMMAND=""
# Test scenario: "all", "all-but-4G", or a single size (4K, 40K, 400K, 4M, 40M, 4G). Set by parse_args for command 'test'.
TEST_SCENARIO=""
# Sizes actually run in this test (set by run_performance_tests for print_results_table)
declare -a SIZES_RUN=()

# Test configurations (set based on STORAGE_TYPE after parsing args)
# Format: config_name|remote_or_basedir|tool
declare -a CONFIGS=()

# MinIO configurations
declare -a MINIO_CONFIGS=(
  "miniosingle-rclone|miniosingle|rclone"
  "minioraid3-rclone|minioraid3|rclone"
  "miniosingle-mc|miniosingle|mc"
)

# Local configurations: cp single, rclone single, rclone raid3
declare -a LOCAL_CONFIGS=(
  "localsingle-cp|LOCAL_SINGLE_DIR|cp"
  "localsingle-rclone|localsingle|rclone"
  "localraid3-rclone|localraid3|rclone"
)

# SFTP configurations (rclone only; no mc for SFTP)
declare -a SFTP_CONFIGS=(
  "sftpsingle-rclone|sftpsingle|rclone"
  "sftpraid3-rclone|sftpraid3|rclone"
)

# File sizes in bytes (using regular array)
declare -a FILE_SIZE_LABELS=("4K" "40K" "400K" "4M" "40M" "4G")

# Helper function to get file size in bytes
get_file_size_bytes() {
  case "$1" in
    "4K") echo "4096" ;;
    "40K") echo "40960" ;;
    "400K") echo "409600" ;;
    "4M") echo "4194304" ;;
    "40M") echo "41943040" ;;
    "4G") echo "4294967296" ;;
    *) echo "0" ;;
  esac
}

# Operations
declare -a OPERATIONS=("upload" "download")

# Results storage: using environment variables to avoid associative arrays
# (for compatibility with older bash versions that don't support declare -A)
# Format: RESULT_DURATION_<key>, RESULT_BYTES_<key>, RESULT_STATUS_<key>, RESULT_SIZE_<key>
# where key is sanitized (e.g., "miniosingle-rclone_4K_upload").
# SIZE is (even_particle + odd_particle) / original_file_size, 3 decimals; local raid3 only.
#
# How speed is calculated:
#   - Time: wall time from just before the copy command starts until it returns (upload or
#     download). So it is "total time from start of read/transfer until done".
#   - Bytes: we always use the original (uncompressed) test file size as the reference.
#     Speed = original_file_size_bytes / elapsed_seconds. So the table shows throughput in
#     "logical" (uncompressed) bytes per second, not actual bytes transferred (e.g. compressed).
#   - Size column (raid3 local only): (even_particle_file_size + odd_particle_file_size) /
#     original_file_size. Particle sizes are the on-disk sizes (compressed payload + footer
#     when compression is enabled). So with compression the ratio is < 1.

# Helper to sanitize key for use in variable name (replace hyphens with underscores)
sanitize_key() {
  echo "$1" | tr '-' '_' | tr -cd '[:alnum:]_'
}

# Store result (using eval to set variable dynamically)
store_result() {
  local key="$1"
  local type="$2"  # DURATION, BYTES, or STATUS
  local value="$3"
  local sanitized
  sanitized=$(sanitize_key "${key}")
  local var_name="RESULT_${type}_${sanitized}"
  # Use eval to set and export the variable (safe because we sanitized the key)
  # shellcheck disable=SC2163
  eval "export ${var_name}=\"${value}\""
}

# Get result
get_result() {
  local key="$1"
  local type="$2"  # DURATION, BYTES, STATUS, or SIZE
  local default="${3:-}"
  local sanitized
  sanitized=$(sanitize_key "${key}")
  local var_name="RESULT_${type}_${sanitized}"
  # Use indirect variable reference
  eval "echo \"\${${var_name}:-${default}}\""
}

# ---------------------------- helper functions ------------------------------

usage() {
  cat <<EOF
Usage: ${SCRIPT_NAME} [options] <command> [arguments]

Performance test for rclone raid3 backend.

Commands:
  start                 Start MinIO containers for performance tests (requires --storage-type=minio).
  stop                  Stop MinIO containers (requires --storage-type=minio).
  teardown              Purge all test data (requires --storage-type).
  list                  Show available test configurations.
  test [scenario]       Run performance tests (requires --storage-type). Optional scenario:
                          all           Run all file sizes (default).
                          all-but-4G    Run all file sizes except 4G.
                          4K|40K|400K|4M|40M|4G   Run only this file size.

Options:
  --storage-type <type>     Select storage type: 'minio', 'local', or 'sftp'.
  -v, --verbose             Show verbose output from commands.
  -c, --compression         Use Snappy compression for raid3 remotes (regenerates config).
  --skip-mc                 Skip mc tests (if mc command not available, minio only).
  --skip-cp                 Skip cp tests (local only).
  -n, --iterations N        Number of iterations per test (default: 11, minimum: 2, first discarded).
  -h, --help                Display this help text.

Storage types:
  minio                 Tests with MinIO S3 backends (requires Docker):
                          - miniosingle using rclone
                          - minioraid3 using rclone
                          - miniosingle using mc command

  local                 Tests with local filesystem backends:
                          - localsingle using cp command
                          - localsingle using rclone
                          - localraid3 using rclone

With file sizes: 4K, 40K, 400K, 4M, 40M, 4G
Each test runs ${ITERATIONS} iterations (first discarded, remaining averaged).

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
      -c|--compression)
        export RAID3_COMPRESSION=snappy
        ;;
      --skip-mc)
        SKIP_MC=1
        ;;
      --skip-cp)
        SKIP_CP=1
        ;;
      -n=*)
        ITERATIONS="${1#*=}"
        if ! [[ "${ITERATIONS}" =~ ^[0-9]+$ ]] || [[ "${ITERATIONS}" -lt 2 ]]; then
          die "Invalid iterations value: ${ITERATIONS}. Must be >= 2 (minimum 2 runs, first discarded)."
        fi
        ;;
      -n|--iterations)
        shift
        [[ $# -gt 0 ]] || die "--iterations requires an argument"
        ITERATIONS="$1"
        if ! [[ "${ITERATIONS}" =~ ^[0-9]+$ ]] || [[ "${ITERATIONS}" -lt 2 ]]; then
          die "Invalid iterations value: ${ITERATIONS}. Must be >= 2 (minimum 2 runs, first discarded)."
        fi
        ;;
      --iterations=*)
        ITERATIONS="${1#*=}"
        if ! [[ "${ITERATIONS}" =~ ^[0-9]+$ ]] || [[ "${ITERATIONS}" -lt 2 ]]; then
          die "Invalid iterations value: ${ITERATIONS}. Must be >= 2 (minimum 2 runs, first discarded)."
        fi
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      start|stop|teardown|list)
        if [[ -n "${COMMAND}" ]]; then
          die "Multiple commands provided: '${COMMAND}' and '$1'"
        fi
        COMMAND="$1"
        ;;
      test)
        if [[ -n "${COMMAND}" ]]; then
          die "Multiple commands provided: '${COMMAND}' and '$1'"
        fi
        COMMAND="test"
        ;;
      all|all-but-4G|4K|40K|400K|4M|40M|4G)
        if [[ "${COMMAND}" == "test" ]]; then
          TEST_SCENARIO="$1"
        else
          die "Unknown argument: $1. See --help."
        fi
        ;;
      *)
        die "Unknown argument: $1. See --help."
        ;;
    esac
    shift
  done

  [[ -n "${COMMAND}" ]] || die "No command specified. See --help."

  if [[ "${COMMAND}" == "test" && -z "${TEST_SCENARIO}" ]]; then
    TEST_SCENARIO="all"
  fi

  case "${COMMAND}" in
    start|stop|teardown|test)
      [[ -n "${STORAGE_TYPE}" ]] || die "--storage-type must be provided for '${COMMAND}'"
      ;;
  esac

  if [[ -n "${STORAGE_TYPE}" && "${STORAGE_TYPE}" != "minio" && "${STORAGE_TYPE}" != "local" && "${STORAGE_TYPE}" != "sftp" ]]; then
    die "Invalid storage type '${STORAGE_TYPE}'. Expected 'minio', 'local', or 'sftp'."
  fi

  # Set CONFIGS based on storage type
  if [[ "${STORAGE_TYPE}" == "minio" ]]; then
    CONFIGS=("${MINIO_CONFIGS[@]}")
  elif [[ "${STORAGE_TYPE}" == "local" ]]; then
    CONFIGS=("${LOCAL_CONFIGS[@]}")
  elif [[ "${STORAGE_TYPE}" == "sftp" ]]; then
    CONFIGS=("${SFTP_CONFIGS[@]}")
  fi
}

# Check if mc command is available
check_mc_available() {
  if ! command -v mc >/dev/null 2>&1; then
    return 1
  fi
  return 0
}

# Setup mc alias for miniosingle
setup_mc_alias() {
  local alias_name="testsingle"
  local endpoint="http://127.0.0.1:${MINIO_SINGLE_PORT}"
  local access_key="single"
  local secret_key="singlepass88"
  local err_file
  err_file=$(mktemp "/tmp/mc_alias_setup.stderr.XXXXXX")
  
  # Remove existing alias if present
  mc alias remove "${alias_name}" >/dev/null 2>&1 || true
  
  # Set new alias
  if ! mc alias set "${alias_name}" "${endpoint}" "${access_key}" "${secret_key}" >/dev/null 2>"${err_file}"; then
    local error_msg
    error_msg=$(cat "${err_file}")
    log_warn "mc" "Failed to set mc alias '${alias_name}': ${error_msg}"
    rm -f "${err_file}"
    return 1
  fi
  
  rm -f "${err_file}"
  echo "${alias_name}"
}

# Cleanup mc alias
cleanup_mc_alias() {
  local alias_name="$1"
  mc alias remove "${alias_name}" >/dev/null 2>&1 || true
}

# Structured sample text (words, sentences) used to fill test files; varied lines add a little randomness.
PERF_TEST_SAMPLE_TEXT='The quick brown fox jumps over the lazy dog. Pack my box with five dozen liquor jugs.
How vexingly quick daft zebras jump. Sphinx of black quartz, judge my vow. Waltz, bad nymph, for quick jigs vex.
The five boxing wizards jump quickly. Grumpy wizards make toxic brew for the evil queen and jack.
Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore.
Raid three stripes bytes across even, odd, and parity. Each particle stores a ninety-byte footer.
Jagged peaks loom over misty valleys. Crimson leaves drift on autumn winds. Frost patterns bloom at dawn.
Binary streams flow through silicon paths. Checksums guard the integrity of every block and sector.
'
PERF_TEST_CHUNK_SIZE=65536

# Create test file of exact size filled with structured text (words, sentences).
# Uses a 64KB chunk of repeated sample text; replicates it by appending in a loop
# (dd count=N only copies the input once when the input file is smaller than N blocks).
# For large files we use a 1MB write chunk to keep the loop count reasonable (e.g. 4096 for 4GB).
PERF_TEST_WRITE_CHUNK_SIZE=1048576

create_test_file() {
  local file_path="$1"
  local size_bytes="$2"
  local chunk_file="${file_path}.chunk"
  local actual_size=0

  # Build one chunk (64KB) by repeating sample text; inject a little randomness every few appends
  : > "${chunk_file}"
  local append_count=0
  while [[ "${actual_size}" -lt "${PERF_TEST_CHUNK_SIZE}" ]]; do
    printf '%s' "${PERF_TEST_SAMPLE_TEXT}" >> "${chunk_file}"
    append_count=$((append_count + 1))
    if [[ $((append_count % 7)) -eq 0 ]]; then
      printf ' %s ' "${RANDOM}" >> "${chunk_file}"
    fi
    actual_size=$(stat -f%z "${chunk_file}" 2>/dev/null || stat -c%s "${chunk_file}" 2>/dev/null)
  done
  truncate -s "${PERF_TEST_CHUNK_SIZE}" "${chunk_file}" 2>/dev/null || {
    head -c "${PERF_TEST_CHUNK_SIZE}" "${chunk_file}" > "${chunk_file}.tmp" && mv "${chunk_file}.tmp" "${chunk_file}"
  }

  # Build a 1MB write chunk (16 x 64KB) to reduce loop iterations for large files
  local write_chunk="${file_path}.wchunk"
  : > "${write_chunk}"
  local j=0
  while [[ "${j}" -lt 16 ]]; do
    cat "${chunk_file}" >> "${write_chunk}"
    j=$((j + 1))
  done

  # Replicate write chunk (or small chunk for tiny files) to reach target size
  local full_1mb=$((size_bytes / PERF_TEST_WRITE_CHUNK_SIZE))
  local remainder=$((size_bytes % PERF_TEST_WRITE_CHUNK_SIZE))
  : > "${file_path}"
  local i=0
  while [[ "${i}" -lt "${full_1mb}" ]]; do
    cat "${write_chunk}" >> "${file_path}"
    i=$((i + 1))
  done
  # Remainder: use 64KB chunk for the last partial megabyte
  local remain_chunks=$((remainder / PERF_TEST_CHUNK_SIZE))
  local remain_bytes=$((remainder % PERF_TEST_CHUNK_SIZE))
  j=0
  while [[ "${j}" -lt "${remain_chunks}" ]]; do
    cat "${chunk_file}" >> "${file_path}"
    j=$((j + 1))
  done
  if [[ "${remain_bytes}" -gt 0 ]]; then
    head -c "${remain_bytes}" "${chunk_file}" >> "${file_path}"
  fi
  rm -f "${chunk_file}" "${write_chunk}"
}

# Run single performance test with rclone
run_rclone_test() {
  local remote="$1"
  local operation="$2"
  local local_file="$3"
  local remote_path="$4"
  
  local start_time end_time elapsed status err_file
  start_time=$(date +%s.%N)
  err_file=$(mktemp)
  
  # Use more retries for copy so large-file or raid3 operations don't fail on transient timeouts
  set +e
  if [[ "${operation}" == "upload" ]]; then
    rclone_cmd --retries 5 copy "${local_file}" "${remote}:${remote_path}" >/dev/null 2>"${err_file}"
    status=$?
  else
    rclone_cmd --retries 5 copy "${remote}:${remote_path}" "${local_file}" >/dev/null 2>"${err_file}"
    status=$?
  fi
  set -e
  
  if [[ "${status}" -ne 0 ]]; then
    if (( VERBOSE )); then
      log_warn "test" "rclone ${operation} failed (exit ${status}): $(cat "${err_file}" 2>/dev/null | head -10)"
    fi
    if [[ -s "${err_file}" ]]; then
      cat "${err_file}" >&2
    fi
  fi
  rm -f "${err_file}"
  
  end_time=$(date +%s.%N)
  elapsed=$(LC_NUMERIC=C awk -v start="${start_time}" -v end="${end_time}" 'BEGIN {printf "%.6f", end - start}')
  
  printf '%s|%s\n' "${status}" "${elapsed}"
}

# Run single performance test with mc
run_mc_test() {
  local alias_name="$1"
  local operation="$2"
  local local_file="$3"
  local remote_path="$4"
  
  local start_time end_time elapsed status
  local err_file
  err_file=$(mktemp "/tmp/mc_test_${operation}.stderr.XXXXXX")
  
  start_time=$(date +%s.%N)
  
  set +e
  if [[ "${operation}" == "upload" ]]; then
    if (( VERBOSE )); then
      mc cp "${local_file}" "${alias_name}/${remote_path}" 2>"${err_file}"
    else
      mc cp "${local_file}" "${alias_name}/${remote_path}" 1>/dev/null 2>"${err_file}"
    fi
    status=$?
  else
    if (( VERBOSE )); then
      mc cp "${alias_name}/${remote_path}" "${local_file}" 2>"${err_file}"
    else
      mc cp "${alias_name}/${remote_path}" "${local_file}" 1>/dev/null 2>"${err_file}"
    fi
    status=$?
  fi
  set -e
  
  end_time=$(date +%s.%N)
  elapsed=$(LC_NUMERIC=C awk -v start="${start_time}" -v end="${end_time}" 'BEGIN {printf "%.6f", end - start}')
  
  # If failed, show error (always, not just in verbose mode for debugging)
  if [[ "${status}" -ne 0 ]]; then
    local error_msg
    error_msg=$(cat "${err_file}")
    log_warn "mc" "mc ${operation} failed (${alias_name}/${remote_path}): ${error_msg}"
  fi
  
  rm -f "${err_file}"
  
  printf '%s|%s\n' "${status}" "${elapsed}"
}

# Run single performance test with cp (local filesystem)
run_cp_test() {
  local base_dir="$1"
  local operation="$2"
  local local_file="$3"
  local remote_path="$4"
  
  local target_path="${base_dir}/${remote_path}"
  local start_time end_time elapsed status
  local err_file
  err_file=$(mktemp "/tmp/cp_test_${operation}.stderr.XXXXXX")
  
  start_time=$(date +%s.%N)
  
  set +e
  if [[ "${operation}" == "upload" ]]; then
    # Ensure target directory exists
    mkdir -p "$(dirname "${target_path}")" 2>"${err_file}"
    if (( VERBOSE )); then
      cp "${local_file}" "${target_path}" 2>>"${err_file}"
    else
      cp "${local_file}" "${target_path}" >/dev/null 2>>"${err_file}"
    fi
    status=$?
  else
    if (( VERBOSE )); then
      cp "${target_path}" "${local_file}" 2>"${err_file}"
    else
      cp "${target_path}" "${local_file}" >/dev/null 2>"${err_file}"
    fi
    status=$?
  fi
  set -e
  
  end_time=$(date +%s.%N)
  elapsed=$(LC_NUMERIC=C awk -v start="${start_time}" -v end="${end_time}" 'BEGIN {printf "%.6f", end - start}')
  
  # If failed, show error
  if [[ "${status}" -ne 0 ]]; then
    local error_msg
    error_msg=$(cat "${err_file}")
    log_warn "cp" "cp ${operation} failed (${target_path}): ${error_msg}"
  fi
  
  rm -f "${err_file}"
  
  printf '%s|%s\n' "${status}" "${elapsed}"
}

# Run test suite: N iterations (first discarded), each iteration measures both upload and download.
# Upload and download are measured in one step per iteration for efficiency and comparable conditions.
run_test_suite() {
  local config_name="$1"
  local remote_or_alias="$2"
  local tool="$3"
  local file_size_label="$4"
  local file_size_bytes="$5"
  
  local test_key_base="${config_name}_${file_size_label}"
  local test_key_upload="${test_key_base}_upload"
  local test_key_download="${test_key_base}_download"
  local temp_dir
  temp_dir=$(mktemp -d) || die "Failed to create temp directory"
  
  # Create test file
  local test_file="${temp_dir}/test_${file_size_label}.bin"
  if (( VERBOSE )); then
    log_info "test" "Creating test file: ${file_size_label} (${file_size_bytes} bytes)"
  fi
  create_test_file "${test_file}" "${file_size_bytes}"
  
  # Verify file was created correctly
  local actual_size
  actual_size=$(stat -f%z "${test_file}" 2>/dev/null || stat -c%s "${test_file}" 2>/dev/null)
  if [[ "${actual_size}" -ne "${file_size_bytes}" ]]; then
    log_warn "test" "Test file size mismatch: ${actual_size} != ${file_size_bytes}"
    rm -rf "${temp_dir}"
    return 1
  fi
  
  # Remote path for test (same path used for upload and download)
  local remote_path="perf-test/${test_key_base}/test.bin"
  local download_file="${temp_dir}/downloaded.bin"
  
  # Setup mc alias if needed
  local mc_alias=""
  local mc_path=""
  local cp_base_dir=""
  if [[ "${tool}" == "mc" ]]; then
    if ! mc_alias=$(setup_mc_alias); then
      log_warn "test" "Failed to setup mc alias, skipping test"
      rm -rf "${temp_dir}"
      return 1
    fi
    local mc_bucket="perftest"
    mc_path="${mc_bucket}/${remote_path}"
    remote_or_alias="${mc_alias}"
  elif [[ "${tool}" == "cp" ]]; then
    cp_base_dir="${LOCAL_SINGLE_DIR}"
  fi
  
  # Cleanup remote before test
  if [[ "${tool}" == "rclone" ]]; then
    rclone_cmd purge "${remote_or_alias}:perf-test/${test_key_base}/" >/dev/null 2>&1 || true
    rclone_cmd mkdir "${remote_or_alias}:perf-test/${test_key_base}/" >/dev/null 2>&1 || true
  elif [[ "${tool}" == "mc" ]]; then
    local mc_bucket="perftest"
    mc mb "${remote_or_alias}/${mc_bucket}" >/dev/null 2>&1 || true
    mc rm --recursive --force "${remote_or_alias}/${mc_path%/*}/" >/dev/null 2>&1 || true
  elif [[ "${tool}" == "cp" ]]; then
    rm -rf "${cp_base_dir}/perf-test/${test_key_base}" 2>/dev/null || true
    mkdir -p "${cp_base_dir}/perf-test/${test_key_base}" 2>/dev/null || true
  fi
  
  local -a upload_durations=()
  local -a download_durations=()
  local upload_all_passed=1 download_all_passed=1
  local upload_valid_count=0 download_valid_count=0
  
  if (( VERBOSE )); then
    log_info "test" "Running ${ITERATIONS} iterations for ${config_name} ${file_size_label} (upload + download per iteration)"
  fi
  
  for ((i=1; i<=ITERATIONS; i++)); do
    local result status elapsed upload_elapsed download_elapsed
    
    # Upload
    if [[ "${tool}" == "rclone" ]]; then
      result=$(run_rclone_test "${remote_or_alias}" "upload" "${test_file}" "${remote_path}")
    elif [[ "${tool}" == "mc" ]]; then
      result=$(run_mc_test "${remote_or_alias}" "upload" "${test_file}" "${mc_path}")
    else
      result=$(run_cp_test "${cp_base_dir}" "upload" "${test_file}" "${remote_path}")
    fi
    result=$(printf '%s' "${result}" | tail -n1)
    IFS='|' read -r status upload_elapsed <<<"${result}"
    if ! [[ "${status}" =~ ^[0-9]+$ ]]; then status=1; fi
    local upload_ok=0
    [[ "${status}" -eq 0 ]] && upload_ok=1
    [[ "${status}" -ne 0 ]] && upload_all_passed=0
    
    # Download (data is already there from upload above)
    if [[ "${tool}" == "rclone" ]]; then
      result=$(run_rclone_test "${remote_or_alias}" "download" "${download_file}" "${remote_path}")
    elif [[ "${tool}" == "mc" ]]; then
      result=$(run_mc_test "${remote_or_alias}" "download" "${download_file}" "${mc_path}")
    else
      result=$(run_cp_test "${cp_base_dir}" "download" "${download_file}" "${remote_path}")
    fi
    result=$(printf '%s' "${result}" | tail -n1)
    IFS='|' read -r status download_elapsed <<<"${result}"
    if ! [[ "${status}" =~ ^[0-9]+$ ]]; then status=1; fi
    local download_ok=0
    [[ "${status}" -eq 0 ]] && download_ok=1
    if [[ "${status}" -ne 0 ]]; then
      download_all_passed=0
      if (( VERBOSE )) && [[ "${tool}" == "rclone" ]]; then
        log_warn "test" "Download failed at iteration ${i} (exit ${status})"
      fi
    fi
    
    # Discard first iteration, store rest
    if [[ $i -gt 1 ]]; then
      if [[ "${upload_ok}" -eq 1 ]]; then
        upload_durations+=("${upload_elapsed}")
        upload_valid_count=$((upload_valid_count + 1))
      fi
      if [[ "${download_ok}" -eq 1 ]]; then
        download_durations+=("${download_elapsed}")
        download_valid_count=$((download_valid_count + 1))
      fi
      if (( VERBOSE )); then
        log_info "test" "  Iteration ${i}: upload ${upload_elapsed}s, download ${download_elapsed}s"
      fi
    else
      if (( VERBOSE )); then
        log_info "test" "  Iteration ${i} (discarded): upload ${upload_elapsed}s, download ${download_elapsed}s"
      fi
    fi
  done

  # For local + localraid3: compute (even+odd size)/original from particle files on disk
  if [[ "${STORAGE_TYPE}" == "local" ]] && [[ "${config_name}" == "localraid3-rclone" ]]; then
    local even_base odd_base even_path odd_path even_size odd_size ratio rel_even rel_odd
    local lsl_even lsl_odd rclone_even_path rclone_odd_path script_prefix
    if [[ "${LOCAL_EVEN_DIR}" == "${SCRIPT_DIR}"/* ]]; then
      rel_even="${LOCAL_EVEN_DIR#"${SCRIPT_DIR}/"}"
      even_base="${PWD}/${rel_even}"
    elif [[ "${LOCAL_EVEN_DIR}" != /* ]]; then
      even_base="${PWD}/${LOCAL_EVEN_DIR}"
    else
      even_base="${LOCAL_EVEN_DIR}"
    fi
    if [[ "${LOCAL_ODD_DIR}" == "${SCRIPT_DIR}"/* ]]; then
      rel_odd="${LOCAL_ODD_DIR#"${SCRIPT_DIR}/"}"
      odd_base="${PWD}/${rel_odd}"
    elif [[ "${LOCAL_ODD_DIR}" != /* ]]; then
      odd_base="${PWD}/${LOCAL_ODD_DIR}"
    else
      odd_base="${LOCAL_ODD_DIR}"
    fi
    even_path="${even_base}/perf-test/${test_key_base}/test.bin"
    odd_path="${odd_base}/perf-test/${test_key_base}/test.bin"
    even_size=""
    odd_size=""
    if [[ -f "${even_path}" ]]; then
      even_size=$(stat -f%z "${even_path}" 2>/dev/null || stat -c%s "${even_path}" 2>/dev/null)
    fi
    if [[ -z "${even_size}" ]] && [[ -d "${even_path}" ]]; then
      even_path=$(find "${even_path}" -type f 2>/dev/null | head -1)
      [[ -n "${even_path}" ]] && even_size=$(stat -f%z "${even_path}" 2>/dev/null || stat -c%s "${even_path}" 2>/dev/null)
    fi
    if [[ -z "${even_size}" ]]; then
      even_path=$(find "${even_base}/perf-test/${test_key_base}" -maxdepth 2 -type f 2>/dev/null | head -1)
      [[ -n "${even_path}" ]] && even_size=$(stat -f%z "${even_path}" 2>/dev/null || stat -c%s "${even_path}" 2>/dev/null)
    fi
    if [[ -f "${odd_path}" ]]; then
      odd_size=$(stat -f%z "${odd_path}" 2>/dev/null || stat -c%s "${odd_path}" 2>/dev/null)
    fi
    if [[ -z "${odd_size}" ]] && [[ -d "${odd_path}" ]]; then
      odd_path=$(find "${odd_path}" -type f 2>/dev/null | head -1)
      [[ -n "${odd_path}" ]] && odd_size=$(stat -f%z "${odd_path}" 2>/dev/null || stat -c%s "${odd_path}" 2>/dev/null)
    fi
    if [[ -z "${odd_size}" ]]; then
      odd_path=$(find "${odd_base}/perf-test/${test_key_base}" -maxdepth 2 -type f 2>/dev/null | head -1)
      [[ -n "${odd_path}" ]] && odd_size=$(stat -f%z "${odd_path}" 2>/dev/null || stat -c%s "${odd_path}" 2>/dev/null)
    fi
    if [[ -z "${even_size}" ]] || [[ -z "${odd_size}" ]]; then
      rclone_even_path="${rel_even:-${LOCAL_EVEN_DIR}}/perf-test/${test_key_base}/"
      rclone_odd_path="${rel_odd:-${LOCAL_ODD_DIR}}/perf-test/${test_key_base}/"
      script_prefix="${SCRIPT_DIR}/"
      if [[ "${rclone_even_path}" == /* ]]; then
        rclone_even_path="${rclone_even_path#"${script_prefix}"}"
      fi
      if [[ "${rclone_odd_path}" == /* ]]; then
        rclone_odd_path="${rclone_odd_path#"${script_prefix}"}"
      fi
      lsl_even=$(rclone_cmd lsl "${LOCAL_EVEN_REMOTE}:${rclone_even_path}" 2>/dev/null | head -5)
      lsl_odd=$(rclone_cmd lsl "${LOCAL_ODD_REMOTE}:${rclone_odd_path}" 2>/dev/null | head -5)
      if [[ -n "${lsl_even}" ]] && [[ -z "${even_size}" ]]; then
        even_size=$(echo "${lsl_even}" | awk '{ sum += $1 } END { print sum+0 }')
      fi
      if [[ -n "${lsl_odd}" ]] && [[ -z "${odd_size}" ]]; then
        odd_size=$(echo "${lsl_odd}" | awk '{ sum += $1 } END { print sum+0 }')
      fi
    fi
    if [[ -n "${even_size}" ]] && [[ -n "${odd_size}" ]] && [[ "${file_size_bytes}" -gt 0 ]]; then
      ratio=$(LC_NUMERIC=C awk -v e="${even_size}" -v o="${odd_size}" -v orig="${file_size_bytes}" 'BEGIN {printf "%.3f", (e + o) / orig}')
      store_result "${test_key_base}" "SIZE" "${ratio}"
      if (( VERBOSE )); then
        log_info "test" "Size ratio ${test_key_base}: ${ratio} (even+odd)/original"
      fi
    elif (( VERBOSE )); then
      log_warn "test" "Size ratio ${test_key_base}: could not get sizes (even_path=${even_path} even_size=${even_size:-empty} odd_path=${odd_path} odd_size=${odd_size:-empty})"
    fi
  fi

  # Cleanup mc alias if used
  if [[ -n "${mc_alias}" ]]; then
    cleanup_mc_alias "${mc_alias}"
  fi

  # Compute averages and store upload results
  if [[ ${#upload_durations[@]} -gt 0 ]]; then
    local sum=0.0
    local duration
    for duration in "${upload_durations[@]}"; do
      sum=$(LC_NUMERIC=C awk -v s="${sum}" -v d="${duration}" 'BEGIN {printf "%.6f", s + d}')
    done
    local avg_upload
    avg_upload=$(LC_NUMERIC=C awk -v s="${sum}" -v c="${#upload_durations[@]}" 'BEGIN {printf "%.6f", s / c}')
    local upload_speed
    upload_speed=$(LC_NUMERIC=C awk -v bytes="${file_size_bytes}" -v dur="${avg_upload}" 'BEGIN {printf "%.0f", bytes / dur}')
    store_result "${test_key_upload}" "DURATION" "${avg_upload}"
    store_result "${test_key_upload}" "BYTES" "${file_size_bytes}"
    if [[ "${upload_all_passed}" -eq 1 ]]; then
      store_result "${test_key_upload}" "STATUS" "OK"
    elif [[ ${upload_valid_count} -gt 0 ]]; then
      store_result "${test_key_upload}" "STATUS" "PARTIAL"
    else
      store_result "${test_key_upload}" "STATUS" "FAILED"
    fi
    if (( VERBOSE )); then
      log_info "test" "${test_key_upload}: average ${avg_upload}s, $(format_speed "${upload_speed}")"
    fi
  else
    store_result "${test_key_upload}" "DURATION" ""
    store_result "${test_key_upload}" "BYTES" "${file_size_bytes}"
    store_result "${test_key_upload}" "STATUS" "FAILED"
  fi
  
  # Compute averages and store download results
  if [[ ${#download_durations[@]} -gt 0 ]]; then
    local sum=0.0
    local duration
    for duration in "${download_durations[@]}"; do
      sum=$(LC_NUMERIC=C awk -v s="${sum}" -v d="${duration}" 'BEGIN {printf "%.6f", s + d}')
    done
    local avg_download
    avg_download=$(LC_NUMERIC=C awk -v s="${sum}" -v c="${#download_durations[@]}" 'BEGIN {printf "%.6f", s / c}')
    local download_speed
    download_speed=$(LC_NUMERIC=C awk -v bytes="${file_size_bytes}" -v dur="${avg_download}" 'BEGIN {printf "%.0f", bytes / dur}')
    store_result "${test_key_download}" "DURATION" "${avg_download}"
    store_result "${test_key_download}" "BYTES" "${file_size_bytes}"
    if [[ "${download_all_passed}" -eq 1 ]]; then
      store_result "${test_key_download}" "STATUS" "OK"
    elif [[ ${download_valid_count} -gt 0 ]]; then
      store_result "${test_key_download}" "STATUS" "PARTIAL"
    else
      store_result "${test_key_download}" "STATUS" "FAILED"
    fi
    if (( VERBOSE )); then
      log_info "test" "${test_key_download}: average ${avg_download}s, $(format_speed "${download_speed}")"
    fi
  else
    store_result "${test_key_download}" "DURATION" ""
    store_result "${test_key_download}" "BYTES" "${file_size_bytes}"
    store_result "${test_key_download}" "STATUS" "FAILED"
  fi

  if [[ ${#upload_durations[@]} -eq 0 ]] && [[ ${#download_durations[@]} -eq 0 ]]; then
    log_warn "test" "No valid durations for ${test_key_base} (all iterations failed)"
    rm -rf "${temp_dir}"
    if [[ "${tool}" == "rclone" ]]; then
      rclone_cmd purge "${remote_or_alias}:perf-test/${test_key_base}/" >/dev/null 2>&1 || true
    elif [[ "${tool}" == "mc" ]]; then
      mc rm --recursive --force "${remote_or_alias}/${mc_path%/*}/" >/dev/null 2>&1 || true
    elif [[ "${tool}" == "cp" ]]; then
      rm -rf "${cp_base_dir}/perf-test/${test_key_base}" 2>/dev/null || true
    fi
    return 1
  fi
  
  rm -rf "${temp_dir}"
  if [[ "${tool}" == "rclone" ]]; then
    rclone_cmd purge "${remote_or_alias}:perf-test/${test_key_base}/" >/dev/null 2>&1 || true
  elif [[ "${tool}" == "mc" ]]; then
    mc rm --recursive --force "${remote_or_alias}/${mc_path%/*}/" >/dev/null 2>&1 || true
  elif [[ "${tool}" == "cp" ]]; then
    rm -rf "${cp_base_dir}/perf-test/${test_key_base}" 2>/dev/null || true
  fi
  
  return 0
}

# Format speed in human-readable format
format_speed() {
  local bytes_per_sec="$1"
  local result
  
  if [[ "${bytes_per_sec}" -lt 1024 ]]; then
    printf "%d B/s" "${bytes_per_sec}"
  elif [[ "${bytes_per_sec}" -lt 1048576 ]]; then
    result=$(LC_NUMERIC=C awk -v b="${bytes_per_sec}" 'BEGIN {printf "%.2f", b / 1024}')
    printf "%s KB/s" "${result}"
  elif [[ "${bytes_per_sec}" -lt 1073741824 ]]; then
    result=$(LC_NUMERIC=C awk -v b="${bytes_per_sec}" 'BEGIN {printf "%.2f", b / 1048576}')
    printf "%s MB/s" "${result}"
  else
    result=$(LC_NUMERIC=C awk -v b="${bytes_per_sec}" 'BEGIN {printf "%.2f", b / 1073741824}')
    printf "%s GB/s" "${result}"
  fi
}

# Choose a single unit (B, KB, MB, GB) for a column from a list of speed values (bytes/sec).
# Returns the unit that fits the maximum value so all values in the column use the same unit.
choose_speed_unit() {
  local max=0
  local v
  for v in "$@"; do
    if [[ -n "${v}" ]] && [[ "${v}" =~ ^[0-9]+$ ]] && [[ "${v}" -gt "${max}" ]]; then
      max="${v}"
    fi
  done
  if [[ "${max}" -lt 1024 ]]; then
    echo "B"
  elif [[ "${max}" -lt 1048576 ]]; then
    echo "KB"
  elif [[ "${max}" -lt 1073741824 ]]; then
    echo "MB"
  else
    echo "GB"
  fi
}

# Format speed in a specific unit (B, KB, MB, GB). bytes_per_sec is in bytes/sec.
format_speed_in_unit() {
  local bytes_per_sec="$1"
  local unit="$2"
  local result
  
  if [[ -z "${bytes_per_sec}" ]] || [[ "${bytes_per_sec}" == "0" ]]; then
    echo "N/A"
    return
  fi
  case "${unit}" in
    B)   printf "%d B/s" "${bytes_per_sec}" ;;
    KB)  result=$(LC_NUMERIC=C awk -v b="${bytes_per_sec}" 'BEGIN {printf "%.2f", b / 1024}'); printf "%s KB/s" "${result}" ;;
    MB)  result=$(LC_NUMERIC=C awk -v b="${bytes_per_sec}" 'BEGIN {printf "%.2f", b / 1048576}'); printf "%s MB/s" "${result}" ;;
    GB)  result=$(LC_NUMERIC=C awk -v b="${bytes_per_sec}" 'BEGIN {printf "%.2f", b / 1073741824}'); printf "%s GB/s" "${result}" ;;
    *)   format_speed "${bytes_per_sec}" ;;
  esac
}

# Format duration
format_duration() {
  local seconds="$1"
  # Format to 3 decimal places using awk to ensure proper formatting
  local formatted
  formatted=$(LC_NUMERIC=C awk -v s="${seconds}" 'BEGIN {printf "%.3f", s}')
  printf "%ss" "${formatted}"
}

# List available test configurations
list_tests() {
  cat <<EOF
Available test configurations:

  --storage-type=minio (requires Docker):
    miniosingle-rclone   MinIO single backend using rclone
    minioraid3-rclone    MinIO RAID3 backend using rclone
    miniosingle-mc       MinIO single backend using mc command

  --storage-type=local:
    localsingle-cp       Local single backend using cp command
    localsingle-rclone   Local single backend using rclone
    localraid3-rclone    Local RAID3 backend using rclone

File sizes tested:
  4K                  4 kilobytes (4096 bytes)
  40K                 40 kilobytes (40960 bytes)
  400K                400 kilobytes (409600 bytes)
  4M                  4 megabytes (4194304 bytes)
  40M                 40 megabytes (41943040 bytes)
  4G                  4 gigabytes (4294967296 bytes)

Test scenarios (for 'test' command):
  test                or  test all           Run all file sizes.
  test all-but-4G                           Run all file sizes except 4G.
  test <size>         e.g. test 4M           Run only the given file size (4K|40K|400K|4M|40M|4G).

Operations tested:
  upload               Upload performance (measured each iteration)
  download             Download performance (measured each iteration)

Each (config, size) runs ${ITERATIONS} iterations; each iteration measures both upload and
download in one step (first iteration discarded, remaining averaged).
EOF
}

# Print results table
print_results_table() {
  local -a table_sizes=()
  if [[ ${#SIZES_RUN[@]} -gt 0 ]]; then
    table_sizes=("${SIZES_RUN[@]}")
  else
    table_sizes=("${FILE_SIZE_LABELS[@]}")
  fi

  echo
  echo "Performance Test Results"
  echo "========================"
  echo

  # Group by file size
  for size_label in "${table_sizes[@]}"; do
    echo "File Size: ${size_label}"
    printf '=%.0s' {1..50}
    echo
    echo
    
    # Print header (Config column 13 chars; add Size only for local storage)
    if [[ "${STORAGE_TYPE}" == "local" ]]; then
      printf "%-13s | %-8s | %-12s | %-12s | %-12s | %-7s\n" \
        "Config" "Status" "Upload" "Download" "Average" "Size"
      printf "%-13s-+-%-8s-+-%-12s-+-%-12s-+-%-12s-+-%-7s\n" \
        "$(printf '%.0s-' {1..13})" \
        "$(printf '%.0s-' {1..8})" \
        "$(printf '%.0s-' {1..12})" \
        "$(printf '%.0s-' {1..12})" \
        "$(printf '%.0s-' {1..12})" \
        "$(printf '%.0s-' {1..7})"
    else
      printf "%-13s | %-8s | %-12s | %-12s | %-12s\n" \
        "Config" "Status" "Upload" "Download" "Average"
      printf "%-13s-+-%-8s-+-%-12s-+-%-12s-+-%-12s\n" \
        "$(printf '%.0s-' {1..13})" \
        "$(printf '%.0s-' {1..8})" \
        "$(printf '%.0s-' {1..12})" \
        "$(printf '%.0s-' {1..12})" \
        "$(printf '%.0s-' {1..12})"
    fi
    
    # Print results for each config based on storage type
    local -a config_order=()
    if [[ "${STORAGE_TYPE}" == "minio" ]]; then
      # Order: mc single, rclone single, rclone raid3
      config_order=("miniosingle-mc|mc single" "miniosingle-rclone|rclone single" "minioraid3-rclone|rclone raid3")
    elif [[ "${STORAGE_TYPE}" == "local" ]]; then
      # Order: cp single, rclone single, rclone raid3
      config_order=("localsingle-cp|cp single" "localsingle-rclone|rclone single" "localraid3-rclone|rclone raid3")
    fi
    
    # First pass: collect all speed values (bytes/sec) for this file-size block to choose one unit per column
    local -a upload_speeds=() download_speeds=() avg_speeds=()
    for config_entry in "${config_order[@]}"; do
      IFS='|' read -r config_name display_name <<<"${config_entry}"
      if [[ "${config_name}" == "miniosingle-mc" && "${SKIP_MC}" -eq 1 ]]; then
        continue
      fi
      if [[ "${config_name}" == "localsingle-cp" && "${SKIP_CP}" -eq 1 ]]; then
        continue
      fi
      local upload_key="${config_name}_${size_label}_upload"
      local download_key="${config_name}_${size_label}_download"
      local ud ub dd db
      ud=$(get_result "${upload_key}" "DURATION" "")
      ub=$(get_result "${upload_key}" "BYTES" "")
      dd=$(get_result "${download_key}" "DURATION" "")
      db=$(get_result "${download_key}" "BYTES" "")
      local us=0 ds=0 av=0
      if [[ -n "${ud}" ]] && [[ -n "${ub}" ]]; then
        us=$(LC_NUMERIC=C awk -v b="${ub}" -v d="${ud}" 'BEGIN {printf "%.0f", b / d}')
      fi
      if [[ -n "${dd}" ]] && [[ -n "${db}" ]]; then
        ds=$(LC_NUMERIC=C awk -v b="${db}" -v d="${dd}" 'BEGIN {printf "%.0f", b / d}')
      fi
      if [[ "${us}" -gt 0 ]] && [[ "${ds}" -gt 0 ]]; then
        av=$(LC_NUMERIC=C awk -v u="${us}" -v d="${ds}" 'BEGIN {printf "%.0f", (u + d) / 2}')
      elif [[ "${us}" -gt 0 ]]; then
        av="${us}"
      elif [[ "${ds}" -gt 0 ]]; then
        av="${ds}"
      fi
      upload_speeds+=("${us}")
      download_speeds+=("${ds}")
      avg_speeds+=("${av}")
    done
    
    # Choose one unit per column so all values in that column use the same unit
    local upload_unit download_unit avg_unit
    upload_unit=$(choose_speed_unit "${upload_speeds[@]}")
    download_unit=$(choose_speed_unit "${download_speeds[@]}")
    avg_unit=$(choose_speed_unit "${avg_speeds[@]}")
    
    # Second pass: print each row using the chosen units
    for config_entry in "${config_order[@]}"; do
      IFS='|' read -r config_name display_name <<<"${config_entry}"
      if [[ "${config_name}" == "miniosingle-mc" && "${SKIP_MC}" -eq 1 ]]; then
        continue
      fi
      if [[ "${config_name}" == "localsingle-cp" && "${SKIP_CP}" -eq 1 ]]; then
        continue
      fi
      local upload_key="${config_name}_${size_label}_upload"
      local download_key="${config_name}_${size_label}_download"
      local upload_duration upload_bytes upload_status upload_speed upload_speed_fmt
      local download_duration download_bytes download_status download_speed download_speed_fmt
      local avg_speed avg_speed_fmt overall_status
      upload_duration=$(get_result "${upload_key}" "DURATION" "")
      upload_bytes=$(get_result "${upload_key}" "BYTES" "")
      upload_status=$(get_result "${upload_key}" "STATUS" "FAILED")
      download_duration=$(get_result "${download_key}" "DURATION" "")
      download_bytes=$(get_result "${download_key}" "BYTES" "")
      download_status=$(get_result "${download_key}" "STATUS" "FAILED")
      if [[ "${upload_status}" == "FAILED" ]] || [[ "${download_status}" == "FAILED" ]]; then
        overall_status="FAILED"
      elif [[ "${upload_status}" == "PARTIAL" ]] || [[ "${download_status}" == "PARTIAL" ]]; then
        overall_status="PARTIAL"
      elif [[ "${upload_status}" == "SKIPPED" ]] && [[ "${download_status}" == "SKIPPED" ]]; then
        overall_status="SKIPPED"
      else
        overall_status="OK"
      fi
      if [[ -n "${upload_duration}" ]] && [[ -n "${upload_bytes}" ]]; then
        upload_speed=$(LC_NUMERIC=C awk -v b="${upload_bytes}" -v d="${upload_duration}" 'BEGIN {printf "%.0f", b / d}')
        upload_speed_fmt=$(format_speed_in_unit "${upload_speed}" "${upload_unit}")
      else
        upload_speed_fmt="N/A"
        upload_speed=0
      fi
      if [[ -n "${download_duration}" ]] && [[ -n "${download_bytes}" ]]; then
        download_speed=$(LC_NUMERIC=C awk -v b="${download_bytes}" -v d="${download_duration}" 'BEGIN {printf "%.0f", b / d}')
        download_speed_fmt=$(format_speed_in_unit "${download_speed}" "${download_unit}")
      else
        download_speed_fmt="N/A"
        download_speed=0
      fi
      if [[ "${upload_speed}" -gt 0 ]] && [[ "${download_speed}" -gt 0 ]]; then
        avg_speed=$(LC_NUMERIC=C awk -v u="${upload_speed}" -v d="${download_speed}" 'BEGIN {printf "%.0f", (u + d) / 2}')
        avg_speed_fmt=$(format_speed_in_unit "${avg_speed}" "${avg_unit}")
      elif [[ "${upload_speed}" -gt 0 ]]; then
        avg_speed_fmt=$(format_speed_in_unit "${upload_speed}" "${avg_unit}")
      elif [[ "${download_speed}" -gt 0 ]]; then
        avg_speed_fmt=$(format_speed_in_unit "${download_speed}" "${avg_unit}")
      else
        avg_speed_fmt="N/A"
      fi
      # Size column: (even+odd particle size)/original, local only, 3 decimal places
      local size_ratio=""
      if [[ "${STORAGE_TYPE}" == "local" ]]; then
        size_ratio=$(get_result "${config_name}_${size_label}" "SIZE" "")
        [[ -z "${size_ratio}" ]] && size_ratio="-"
      fi
      if [[ "${STORAGE_TYPE}" == "local" ]]; then
        printf "%-13s | %-8s | %-12s | %-12s | %-12s | %-7s\n" \
          "${display_name}" "${overall_status}" "${upload_speed_fmt}" "${download_speed_fmt}" "${avg_speed_fmt}" "${size_ratio}"
      else
        printf "%-13s | %-8s | %-12s | %-12s | %-12s\n" \
          "${display_name}" "${overall_status}" "${upload_speed_fmt}" "${download_speed_fmt}" "${avg_speed_fmt}"
      fi
    done
    
    echo
  done
}

# Run performance tests
run_performance_tests() {
  # Resolve which file sizes to run from TEST_SCENARIO
  local -a sizes_to_run=()
  case "${TEST_SCENARIO}" in
    all)
      sizes_to_run=("${FILE_SIZE_LABELS[@]}")
      ;;
    all-but-4G)
      for label in "${FILE_SIZE_LABELS[@]}"; do
        [[ "${label}" != "4G" ]] && sizes_to_run+=("${label}")
      done
      ;;
    4K|40K|400K|4M|40M|4G)
      sizes_to_run=("${TEST_SCENARIO}")
      ;;
    *)
      die "Invalid test scenario: '${TEST_SCENARIO}'. Use: all, all-but-4G, or 4K|40K|400K|4M|40M|4G"
      ;;
  esac
  SIZES_RUN=("${sizes_to_run[@]}")

  if (( VERBOSE )); then
    log_info "test" "Starting performance tests (storage-type=${STORAGE_TYPE}, scenario=${TEST_SCENARIO})"
    log_info "test" "Iterations per test: ${ITERATIONS} (first discarded)"
  fi
  
  # Storage-type specific setup
  if [[ "${STORAGE_TYPE}" == "minio" ]]; then
    # Ensure MinIO containers are ready
    if (( VERBOSE )); then
      log_info "test" "Ensuring MinIO containers are ready"
    fi
    if ! ensure_minio_containers_ready; then
      die "MinIO containers are not ready. Please run '${SCRIPT_NAME} start --storage-type=minio' first."
    fi
    
    # Check mc availability
    if [[ "${SKIP_MC}" -eq 0 ]]; then
      if ! check_mc_available; then
        log_warn "test" "mc command not found. Use --skip-mc to skip mc tests."
        SKIP_MC=1
      fi
    fi
  elif [[ "${STORAGE_TYPE}" == "local" ]]; then
    # Ensure local directories exist
    if (( VERBOSE )); then
      log_info "test" "Ensuring local directories exist"
    fi
    ensure_directory "${LOCAL_SINGLE_DIR}"
    for dir in "${LOCAL_RAID3_DIRS[@]}"; do
      ensure_directory "${dir}"
    done
    
    # Ensure rclone config and binary are available (regenerate config when --compression set)
    if [[ -n "${RAID3_COMPRESSION:-}" ]] && [[ "${RAID3_COMPRESSION}" == "snappy" || "${RAID3_COMPRESSION}" == "zstd" ]]; then
      log_info "test" "Regenerating config with compression = ${RAID3_COMPRESSION} for raid3 remotes"
      create_rclone_config "${TEST_SPECIFIC_CONFIG}" 1 || true
    else
      ensure_rclone_config
    fi
    ensure_rclone_binary
  elif [[ "${STORAGE_TYPE}" == "sftp" ]]; then
    if (( VERBOSE )); then
      log_info "test" "Ensuring SFTP containers are ready"
    fi
    if ! ensure_sftp_containers_ready; then
      die "SFTP containers are not ready. Please run '${SCRIPT_NAME} start --storage-type=sftp' first."
    fi
  fi
  
  # Run all test suites
  local total_tests=0
  local passed_tests=0
  local failed_tests=0
  local partial_tests=0
  
  for config_entry in "${CONFIGS[@]}"; do
    IFS='|' read -r config_name remote tool <<<"${config_entry}"
    
    # Skip mc if requested or not available (minio only)
    if [[ "${tool}" == "mc" && "${SKIP_MC}" -eq 1 ]]; then
      if (( VERBOSE )); then
        log_info "test" "Skipping ${config_name} (mc not available or --skip-mc)"
      fi
      continue
    fi
    
    # Skip cp if requested (local only)
    if [[ "${tool}" == "cp" && "${SKIP_CP}" -eq 1 ]]; then
      if (( VERBOSE )); then
        log_info "test" "Skipping ${config_name} (--skip-cp)"
      fi
      continue
    fi
    
    for size_label in "${sizes_to_run[@]}"; do
      local size_bytes
      size_bytes=$(get_file_size_bytes "${size_label}")
      
      total_tests=$((total_tests + 2))
      if (( VERBOSE )); then
        log_info "test" "Running test: ${config_name} ${size_label} (upload + download per iteration)"
      fi
      
      if run_test_suite "${config_name}" "${remote}" "${tool}" "${size_label}" "${size_bytes}"; then
        for operation in "${OPERATIONS[@]}"; do
          local test_key="${config_name}_${size_label}_${operation}"
          local status
          status=$(get_result "${test_key}" "STATUS" "FAILED")
          if [[ "${status}" == "OK" ]]; then
            passed_tests=$((passed_tests + 1))
          elif [[ "${status}" == "PARTIAL" ]]; then
            partial_tests=$((partial_tests + 1))
            log_warn "test" "Test completed with partial failures: ${config_name} ${size_label} ${operation}"
          else
            failed_tests=$((failed_tests + 1))
            log_warn "test" "Test failed: ${config_name} ${size_label} ${operation}"
          fi
        done
      else
        failed_tests=$((failed_tests + 2))
        log_warn "test" "Test failed: ${config_name} ${size_label} (upload and download)"
      fi
    done
  done
  
  # Print results table and summary counts only when verbose (harmonized with other test scripts)
  if (( VERBOSE )); then
    print_results_table
    log_info "summary" "Total tests: ${total_tests}"
    log_info "summary" "Passed: ${passed_tests}"
    log_info "summary" "Partial: ${partial_tests}"
    log_info "summary" "Failed: ${failed_tests}"
  fi

  if [[ ${failed_tests} -eq 0 && ${partial_tests} -eq 0 ]]; then
    log_pass "test" "All performance tests completed successfully"
    return 0
  elif [[ ${failed_tests} -gt 0 ]]; then
    log_fail "test" "Some tests failed (${failed_tests} failed, ${partial_tests} partial)"
    return 1
  else
    log_warn "test" "Some tests had partial failures (${partial_tests} partial)"
    return 0
  fi
}

# Main command dispatcher
main() {
  case "${COMMAND}" in
    start)
      if [[ "${STORAGE_TYPE}" == "minio" ]]; then
        start_minio_containers
      elif [[ "${STORAGE_TYPE}" == "local" ]]; then
        if (( VERBOSE )); then
          log_info "start" "Ensuring local directories exist"
        fi
        ensure_directory "${LOCAL_SINGLE_DIR}"
        for dir in "${LOCAL_RAID3_DIRS[@]}"; do
          ensure_directory "${dir}"
        done
        if (( VERBOSE )); then
          log_info "start" "Local directories ready"
        fi
      elif [[ "${STORAGE_TYPE}" == "sftp" ]]; then
        start_sftp_containers
      fi
      ;;
      
    stop)
      if [[ "${STORAGE_TYPE}" == "minio" ]]; then
        stop_minio_containers
      elif [[ "${STORAGE_TYPE}" == "local" ]]; then
        if (( VERBOSE )); then
          log_info "stop" "No containers to stop for storage type 'local'"
        fi
      elif [[ "${STORAGE_TYPE}" == "sftp" ]]; then
        stop_sftp_containers
      fi
      ;;
      
    teardown)
      if [[ "${STORAGE_TYPE}" == "minio" ]]; then
        ensure_minio_containers_ready
        
        # Purge remotes
        purge_remote_root "miniosingle"
        purge_remote_root "minioraid3"
        
        # Clean up data directories
        for dir in "${MINIO_RAID3_DIRS[@]}" "${MINIO_SINGLE_DIR}"; do
          remove_leftover_files "${dir}"
          verify_directory_empty "${dir}"
        done
        if (( VERBOSE )); then
          log_info "teardown" "Teardown completed"
        fi
      elif [[ "${STORAGE_TYPE}" == "local" ]]; then
        if (( VERBOSE )); then
          log_info "teardown" "Purging local remotes"
        fi
        purge_remote_root "localsingle"
        purge_remote_root "localraid3"
        remove_leftover_files "${LOCAL_SINGLE_DIR}"
        verify_directory_empty "${LOCAL_SINGLE_DIR}"
        for dir in "${LOCAL_RAID3_DIRS[@]}"; do
          remove_leftover_files "${dir}"
          verify_directory_empty "${dir}"
        done
        if (( VERBOSE )); then
          log_info "teardown" "Teardown completed"
        fi
      elif [[ "${STORAGE_TYPE}" == "sftp" ]]; then
        if ! ensure_sftp_containers_ready; then
          log_warn "teardown" "SFTP containers not running; skipping purge"
        else
          purge_remote_root "sftpsingle"
          purge_remote_root "sftpraid3"
        fi
        for dir in "${SFTP_RAID3_DIRS[@]}" "${SFTP_SINGLE_DIR}"; do
          remove_leftover_files "${dir}"
          verify_directory_empty "${dir}"
        done
        if (( VERBOSE )); then
          log_info "teardown" "Teardown completed (sftp)"
        fi
      fi
      ;;
      
    list)
      list_tests
      ;;
      
    test)
      run_performance_tests
      ;;
  esac
}

# Parse arguments and run
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  parse_args "$@"
  main
fi
