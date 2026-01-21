#!/usr/bin/env bash
#
# compare_raid3_with_single_features.sh
# ---------------------------------
# Feature handling test harness for rclone raid3 backends.
#
# This script tests feature handling when mixing different remote types
# (local filesystem + MinIO S3). It verifies that features are correctly
# intersected (AND logic) or use best-effort (OR logic) as documented.
#
# Usage:
#   compare_raid3_with_single_features.sh [options] <command> [args]
#
# Commands:
#   start                 Start the MinIO containers required for minioraid3/miniosingle.
#   stop                  Stop those MinIO containers.
#   teardown              Purge all data from the selected storage-type (raid3 + single).
#   list                  Show available test cases.
#   test <name>           Run a named test (e.g. "mixed-features").
#
# Options:
#   --storage-type <local|minio|mixed>   Select which backend pair to exercise.
#                                        Required for start/stop/test/teardown.
#   -v, --verbose                        Show stdout/stderr from both rclone invocations.
#   -h, --help                           Display this help text.
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

# ---------------------------- helper functions ------------------------------

usage() {
  cat <<EOF
Usage: ${SCRIPT_NAME} [options] <command> [arguments]

Commands:
  start                      Start MinIO containers (requires --storage-type=minio or mixed).
  stop                       Stop MinIO containers (requires --storage-type=minio or mixed).
  teardown                   Purge all test data for the selected storage type.
  list                       Show available tests.
  test <name>                Run the named test (e.g. "mixed-features").

Options:
  --storage-type <local|minio|mixed>   Select backend pair (required for start/stop/test/teardown).
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
        # If this looks like an option (starts with -), it's an unknown option
        if [[ "$1" =~ ^- ]]; then
          die "Unknown option: $1"
        fi
        # Otherwise, if command is "test" and we don't have a test name yet, use this as the test name
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

# Extract a feature value from JSON output
# The JSON structure is: { "Features": { "FeatureName": true/false, ... } }
extract_feature() {
  local json="$1"
  local feature="$2"
  # Extract the feature value from the Features object
  # Look for "FeatureName": value (where value can be true, false, or null)
  # Handle tabs and spaces in JSON
  local value
  # Use sed to extract the value, handling any whitespace (tabs, spaces)
  value=$(echo "${json}" | sed -n 's/.*"'"${feature}"'":[[:space:]]*\([^,}]*\).*/\1/p' | head -1 | tr -d '[:space:]\t')
  if [[ -z "${value}" ]]; then
    echo "null"
  else
    echo "${value}"
  fi
}

# Get features as JSON
get_features_json() {
  local remote="$1"
  # Use rclone_cmd to ensure correct config file is used
  rclone_cmd backend features "${remote}:" --json 2>/dev/null || echo "{}"
}

# Test feature handling with mixed remotes
test_mixed_features() {
  local test_case="mixed-features"
  log "Running test: ${test_case}"

  if [[ "${STORAGE_TYPE}" != "mixed" ]]; then
    die "Test '${test_case}' requires --storage-type=mixed"
  fi

  [[ "${STORAGE_TYPE}" != "minio" && "${STORAGE_TYPE}" != "mixed" ]] || ensure_minio_containers_ready

  # Get features for different configurations
  log "Getting features for local-only raid3..."
  local local_features_json
  local_features_json=$(get_features_json "localraid3")

  log "Getting features for MinIO-only raid3..."
  local minio_features_json
  minio_features_json=$(get_features_json "minioraid3")

  log "Getting features for mixed raid3 (local + MinIO)..."
  local mixed_features_json
  mixed_features_json=$(get_features_json "localminioraid3")

  # Test features that should use AND logic (require all backends)
  log "Testing AND logic features (require all backends)..."
  
  # BucketBased should be false for mixed (local is not bucket-based)
  local mixed_bucket_based
  mixed_bucket_based=$(extract_feature "${mixed_features_json}" "BucketBased")
  if [[ "${mixed_bucket_based}" != "false" ]]; then
    die "BucketBased should be false for mixed remotes (local is not bucket-based), got: '${mixed_bucket_based}'"
  fi
  log "✓ BucketBased correctly set to false for mixed remotes"

  # SetTier/GetTier should be false for mixed (local doesn't support tiers)
  local mixed_set_tier
  mixed_set_tier=$(extract_feature "${mixed_features_json}" "SetTier")
  if [[ "${mixed_set_tier}" != "false" ]]; then
    die "SetTier should be false for mixed remotes (local doesn't support tiers), got: ${mixed_set_tier}"
  fi
  log "✓ SetTier correctly set to false for mixed remotes"

  local mixed_get_tier
  mixed_get_tier=$(extract_feature "${mixed_features_json}" "GetTier")
  if [[ "${mixed_get_tier}" != "false" ]]; then
    die "GetTier should be false for mixed remotes (local doesn't support tiers), got: ${mixed_get_tier}"
  fi
  log "✓ GetTier correctly set to false for mixed remotes"

  # Test features that should use OR logic (best-effort, raid3-specific)
  log "Testing OR logic features (best-effort, any backend)..."
  
  # ReadMetadata should work if any backend supports it
  local mixed_read_metadata
  mixed_read_metadata=$(extract_feature "${mixed_features_json}" "ReadMetadata")
  # Local supports ReadMetadata, so mixed should also support it
  if [[ "${mixed_read_metadata}" != "true" ]]; then
    log "⚠ ReadMetadata is ${mixed_read_metadata} for mixed remotes (expected true if local supports it)"
  else
    log "✓ ReadMetadata correctly enabled for mixed remotes (OR logic)"
  fi

  # WriteMetadata should work if any backend supports it
  local mixed_write_metadata
  mixed_write_metadata=$(extract_feature "${mixed_features_json}" "WriteMetadata")
  # Local supports WriteMetadata, so mixed should also support it
  if [[ "${mixed_write_metadata}" != "true" ]]; then
    log "⚠ WriteMetadata is ${mixed_write_metadata} for mixed remotes (expected true if local supports it)"
  else
    log "✓ WriteMetadata correctly enabled for mixed remotes (OR logic)"
  fi

  # Test always-available features (raid3 implements independently)
  log "Testing always-available features (raid3-specific)..."
  
  # Shutdown and CleanUp are function features, check if they're not null
  local mixed_shutdown
  mixed_shutdown=$(echo "${mixed_features_json}" | grep -o '"Shutdown":[^,}]*' || echo "")
  if [[ -z "${mixed_shutdown}" ]] || [[ "${mixed_shutdown}" == *"null"* ]]; then
    die "Shutdown should always be available (raid3 implements it), got: ${mixed_shutdown}"
  fi
  log "✓ Shutdown correctly always available"

  local mixed_cleanup
  mixed_cleanup=$(echo "${mixed_features_json}" | grep -o '"CleanUp":[^,}]*' || echo "")
  if [[ -z "${mixed_cleanup}" ]] || [[ "${mixed_cleanup}" == *"null"* ]]; then
    die "CleanUp should always be available (raid3 implements it), got: ${mixed_cleanup}"
  fi
  log "✓ CleanUp correctly always available"

  log "✅ Test ${test_case} passed"
}

# List available tests
list_tests() {
  cat <<EOF
Available tests:

  mixed-features    Test feature handling with mixed remotes (local + MinIO)
                    Requires --storage-type=mixed

Run all tests by omitting the test name:
  ${SCRIPT_NAME} --storage-type=mixed test

EOF
}

# Run all available tests
run_all_tests() {
  local tests=("mixed-features")
  local test_name
  local failed=0
  
  log "Running all feature handling tests..."
  echo ""
  
  for test_name in "${tests[@]}"; do
    COMMAND_ARG="${test_name}"
    if ! test_mixed_features; then
      log "✗ Test '${test_name}' failed"
      failed=1
    else
      log "✓ Test '${test_name}' passed"
    fi
    echo ""
  done
  
  COMMAND_ARG=""
  return ${failed}
}

# Main execution
main() {
  parse_args "$@"

  # Verify we're in the right directory
  ensure_workdir

  # Ensure rclone binary and config are available
  ensure_rclone_binary
  ensure_rclone_config

  case "${COMMAND}" in
    list)
      list_tests
      ;;
    test)
      if [[ -z "${COMMAND_ARG}" ]]; then
        # Run all tests when no test name is provided
        if ! run_all_tests; then
          die "One or more tests failed."
        fi
      else
        # Run a single named test
        case "${COMMAND_ARG}" in
          mixed-features)
            test_mixed_features
            ;;
          *)
            die "Unknown test: '${COMMAND_ARG}'. Use 'list' to see available tests."
            ;;
        esac
      fi
      ;;
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
      if [[ "${STORAGE_TYPE}" == "local" ]]; then
        for dir in "${LOCAL_RAID3_DIRS[@]}" "${LOCAL_SINGLE_DIR}"; do
          rm -rf "${dir:?}"/*
        done
        log "Teardown complete for local storage"
      elif [[ "${STORAGE_TYPE}" == "mixed" ]]; then
        # Mixed: clean local even/parity dirs and MinIO odd dir
        for dir in "${LOCAL_EVEN_DIR}" "${LOCAL_PARITY_DIR}" "${LOCAL_SINGLE_DIR}"; do
          rm -rf "${dir:?}"/*
        done
        # MinIO cleanup would require API calls, skip for now
        log "Teardown complete for mixed storage (local dirs only)"
      else
        for dir in "${MINIO_RAID3_DIRS[@]}" "${MINIO_SINGLE_DIR}"; do
          # MinIO cleanup would require API calls, skip for now
          :
        done
        log "Teardown complete for MinIO storage (containers not cleaned)"
      fi
      ;;
    *)
      die "Unknown command: ${COMMAND}. Use -h for help."
      ;;
  esac
}

main "$@"
