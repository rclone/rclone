#!/usr/bin/env bash
#
# compression_bench.sh
# --------------------
# Measures compression ratio for the raid3 backend (local storage only).
# For each test file size (4K, 40K, 400K, 4M, 40M, 4G), creates the same
# test file as performance_test.sh, uploads once to localraid3, and reports
# (even_particle + odd_particle) / original_size.
#
# Usage: run from backend/raid3/test
#   ./compression_bench.sh --storage-type=local [--config CONFIG]
#
# Requires: --storage-type=local (required). Config must have compression
# set to something other than 'none' for [localraid3]; otherwise the script
# exits with a warning.
#
# Requires: setup.sh has been run (config and _data dirs exist).
# -----------------------------------------------------------------------------

set -euo pipefail

SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATA_DIR="${DATA_DIR:-${SCRIPT_DIR}/_data}"
RCLONE_CONFIG_BASE="${SCRIPT_DIR}/rclone_raid3_integration_tests.config"
RCLONE_CMD="${RCLONE_BIN:-rclone}"
BENCH_PATH="compression_bench"

# File sizes: same as performance_test.sh
declare -a FILE_SIZE_LABELS=("4K" "40K" "400K" "4M" "40M" "4G")

get_file_size_bytes() {
  case "$1" in
    4K)   echo "4096" ;;
    40K)  echo "40960" ;;
    400K) echo "409600" ;;
    4M)   echo "4194304" ;;
    40M)  echo "41943040" ;;
    4G)   echo "4294967296" ;;
    *)    echo "0" ;;
  esac
}

# Same sample text and create_test_file as performance_test.sh (reproducible files)
PERF_TEST_SAMPLE_TEXT='The quick brown fox jumps over the lazy dog. Pack my box with five dozen liquor jugs.
How vexingly quick daft zebras jump. Sphinx of black quartz, judge my vow. Waltz, bad nymph, for quick jigs vex.
The five boxing wizards jump quickly. Grumpy wizards make toxic brew for the evil queen and jack.
Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore.
Raid three stripes bytes across even, odd, and parity. Each particle stores a ninety-byte footer.
Jagged peaks loom over misty valleys. Crimson leaves drift on autumn winds. Frost patterns bloom at dawn.
Binary streams flow through silicon paths. Checksums guard the integrity of every block and sector.
'
PERF_TEST_CHUNK_SIZE=65536
PERF_TEST_WRITE_CHUNK_SIZE=1048576

create_test_file() {
  local file_path="$1"
  local size_bytes="$2"
  local chunk_file="${file_path}.chunk"
  local actual_size=0

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

  local write_chunk="${file_path}.wchunk"
  : > "${write_chunk}"
  local j=0
  while [[ "${j}" -lt 16 ]]; do
    cat "${chunk_file}" >> "${write_chunk}"
    j=$((j + 1))
  done

  local full_1mb=$((size_bytes / PERF_TEST_WRITE_CHUNK_SIZE))
  local remainder=$((size_bytes % PERF_TEST_WRITE_CHUNK_SIZE))
  : > "${file_path}"
  local i=0
  while [[ "${i}" -lt "${full_1mb}" ]]; do
    cat "${write_chunk}" >> "${file_path}"
    i=$((i + 1))
  done
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

die() {
  echo "ERROR: $*" >&2
  exit 1
}

# Parse args: require --storage-type=local
STORAGE_TYPE=""
CONFIG_ARG=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --storage-type=*)
      STORAGE_TYPE="${1#*=}"
      ;;
    --storage-type)
      shift
      [[ $# -gt 0 ]] || die "--storage-type requires an argument"
      STORAGE_TYPE="$1"
      ;;
    --config)
      shift
      [[ $# -gt 0 ]] || die "--config requires an argument"
      CONFIG_ARG="$1"
      ;;
    --config=*)
      CONFIG_ARG="${1#*=}"
      ;;
    -h|--help)
      echo "Usage: ${SCRIPT_NAME} --storage-type=local [--config CONFIG]"
      echo "Measures compression ratio for raid3 (local only). Uses config's [localraid3] compression setting."
      echo "Requires compression != none in config; run from backend/raid3/test."
      exit 0
      ;;
    *)
      die "Unknown argument: $1. Use --storage-type=local."
      ;;
  esac
  shift
done

[[ -n "${STORAGE_TYPE}" ]] || die "Missing --storage-type. Use --storage-type=local."
[[ "${STORAGE_TYPE}" == "local" ]] || die "This script only supports --storage-type=local."

if [[ -n "${CONFIG_ARG}" ]]; then
  RCLONE_CONFIG_BASE="${CONFIG_ARG}"
fi

# Load env for LOCAL_EVEN_DIR, LOCAL_ODD_DIR
if [[ -f "${SCRIPT_DIR}/compare_raid3_env.sh" ]]; then
  # Pass script/data dir so compare_raid3_env.sh can use them (SC2097/SC2098)
  export SCRIPT_DIR DATA_DIR
  . "${SCRIPT_DIR}/compare_raid3_env.sh"
else
  LOCAL_EVEN_DIR="${DATA_DIR}/even_local"
  LOCAL_ODD_DIR="${DATA_DIR}/odd_local"
fi

# Must run from test directory so relative paths in config work
if [[ "$(pwd)" != "${SCRIPT_DIR}" ]]; then
  die "Run this script from the test directory: cd ${SCRIPT_DIR} && ./${SCRIPT_NAME} --storage-type=local"
fi

[[ -f "${RCLONE_CONFIG_BASE}" ]] || die "Config not found: ${RCLONE_CONFIG_BASE}. Run ./setup.sh first."

# Read compression value for [localraid3] from config
get_localraid3_compression() {
  awk '
    /^\[localraid3\]/ { in_lr=1; next }
    in_lr && /^compression = / { sub(/^compression = /,""); print; exit }
    in_lr && /^\[/ { exit }
  ' "${RCLONE_CONFIG_BASE}"
}

COMPRESSION_TYPE=$(get_localraid3_compression || true)
COMPRESSION_TYPE="${COMPRESSION_TYPE:-none}"
COMPRESSION_TYPE=$(echo "${COMPRESSION_TYPE}" | tr -d '[:space:]')

if [[ "${COMPRESSION_TYPE}" == "none" ]] || [[ -z "${COMPRESSION_TYPE}" ]]; then
  echo "WARNING: compression is set to 'none' (or missing) for [localraid3] in ${RCLONE_CONFIG_BASE}." >&2
  echo "This script measures compression ratio; set compression = snappy (or another type) in the config and try again." >&2
  exit 1
fi

if [[ -n "${RCLONE_BIN:-}" ]]; then
  [[ -x "${RCLONE_CMD}" ]] || die "RCLONE_BIN not executable: ${RCLONE_CMD}"
else
  command -v rclone &>/dev/null || die "rclone not found in PATH. Set RCLONE_BIN or install rclone."
fi

export RCLONE_CONFIG="${RCLONE_CONFIG_BASE}"

echo "=== raid3 compression benchmark (local only) ==="
echo "Config:       ${RCLONE_CONFIG_BASE}"
echo "Compression: ${COMPRESSION_TYPE}"
echo "Reference:   original file size vs (even_particle + odd_particle)"
echo ""

# Return (even + odd) bytes for a given bench subpath (e.g. 4K)
total_even_odd_bytes() {
  local subpath="$1"
  local even_base="${LOCAL_EVEN_DIR}"
  local odd_base="${LOCAL_ODD_DIR}"
  [[ "${even_base}" != /* ]] && even_base="${SCRIPT_DIR}/${even_base}"
  [[ "${odd_base}" != /* ]] && odd_base="${SCRIPT_DIR}/${odd_base}"
  local even_path="${even_base}/${BENCH_PATH}/${subpath}/test.bin"
  local odd_path="${odd_base}/${BENCH_PATH}/${subpath}/test.bin"
  local even_size=0 odd_size=0
  if [[ -f "${even_path}" ]]; then
    even_size=$(stat -f%z "${even_path}" 2>/dev/null || stat -c%s "${even_path}" 2>/dev/null)
  fi
  if [[ -f "${odd_path}" ]]; then
    odd_size=$(stat -f%z "${odd_path}" 2>/dev/null || stat -c%s "${odd_path}" 2>/dev/null)
  fi
  echo $((even_size + odd_size))
}

# Print header
printf "%-8s | %12s | %14s | %10s\n" "Size" "Original (B)" "Even+Odd (B)" "Ratio"
echo "----------|--------------|----------------|----------"

for label in "${FILE_SIZE_LABELS[@]}"; do
  file_size_bytes=$(get_file_size_bytes "${label}")
  [[ "${file_size_bytes}" -eq 0 ]] && continue

  temp_dir=$(mktemp -d) || die "Failed to create temp directory"
  test_file="${temp_dir}/test.bin"
  create_test_file "${test_file}" "${file_size_bytes}"
  actual_size=$(stat -f%z "${test_file}" 2>/dev/null || stat -c%s "${test_file}" 2>/dev/null)

  remote_subpath="${label}"
  remote_path="${BENCH_PATH}/${remote_subpath}/"
  ${RCLONE_CMD} purge "localraid3:${remote_path}" -q 2>/dev/null || true
  ${RCLONE_CMD} copy "${test_file}" "localraid3:${remote_path}" --ignore-times -q

  stored=$(total_even_odd_bytes "${remote_subpath}")
  rm -rf "${temp_dir}"

  if [[ "${stored}" -eq 0 ]]; then
    ratio="N/A"
  else
    ratio=$(LC_NUMERIC=C awk -v s="${stored}" -v o="${actual_size}" 'BEGIN {printf "%.3f", s/o}')
  fi
  printf "%-8s | %12d | %14d | %10s\n" "${label}" "${actual_size}" "${stored}" "${ratio}"
done

echo ""
echo "Done. Ratio = (even_particle + odd_particle) / original_file_size."
