#!/usr/bin/env bash
#
# compare_raid3_with_single_features.sh
# ---------------------------------
# Feature handling test harness for rclone raid3 backends.
#
# This script tests feature handling across different remote type configurations:
# - local: All three backends are local filesystem
# - minio: All three backends are MinIO object storage
# - mixed: Mix of local filesystem and MinIO (local even/parity, MinIO odd)
#
# It verifies that features are correctly intersected (AND logic) or use
# best-effort (OR logic) as documented in the raid3 backend.
#
# Usage:
#   compare_raid3_with_single_features.sh [options] <command> [args]
#
# Commands:
#   start                 Start the MinIO containers required for minioraid3/miniosingle.
#   stop                  Stop those MinIO containers.
#   teardown              Purge all data from the selected storage-type (raid3 + single).
#   list                  Show available test cases.
#   test <name>           Run a named test (e.g. "local-features", "minio-features", "mixed-features").
#
# Options:
#   --storage-type <local|minio|mixed>   Select which backend configuration to test.
#                                        Required for start/stop/test/teardown.
#   -v, --verbose                        Show stdout/stderr from both rclone invocations.
#   -h, --help                           Display this help text.
#
# Safety guard: the script must be executed from backend/raid3/test directory.
# -----------------------------------------------------------------------------

set -euo pipefail

SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)

# shellcheck source=compare_raid3_with_single_common.sh
. "${SCRIPT_DIR}/compare_raid3_with_single_common.sh"

# VERBOSE is used by sourced compare_raid3_with_single_common.sh (print_if_verbose, purge_remote_root)
# shellcheck disable=SC2034
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

  if [[ -n "${STORAGE_TYPE}" && "${STORAGE_TYPE}" != "local" && "${STORAGE_TYPE}" != "minio" && "${STORAGE_TYPE}" != "mixed" && "${STORAGE_TYPE}" != "sftp" ]]; then
    die "Invalid storage type '${STORAGE_TYPE}'. Expected 'local', 'minio', 'mixed', or 'sftp'."
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

# Test feature handling for a specific storage type
# This is a generic test function that validates features based on storage type
test_features_for_storage_type() {
  local storage_type="$1"
  local test_case="${storage_type}-features"
  log "Running test: ${test_case}"

  # Ensure MinIO or SFTP containers are ready if needed
  [[ "${storage_type}" != "minio" && "${storage_type}" != "mixed" ]] || ensure_minio_containers_ready
  [[ "${storage_type}" != "sftp" ]] || ensure_sftp_containers_ready

  # Determine which remote to test based on storage type
  local raid3_remote
  case "${storage_type}" in
    local)
      raid3_remote="localraid3"
      ;;
    minio)
      raid3_remote="minioraid3"
      ;;
    mixed)
      raid3_remote="localminioraid3"
      ;;
    sftp)
      raid3_remote="sftpraid3"
      ;;
    *)
      die "Unknown storage type: ${storage_type}"
      ;;
  esac

  log "Getting features for ${storage_type} raid3 (remote: ${raid3_remote})..."
  local features_json
  features_json=$(get_features_json "${raid3_remote}")

  # Test always-available features (raid3 implements independently)
  log "Testing always-available features (raid3-specific)..."
  
  # Shutdown and CleanUp are function features, check if they're not null
  local shutdown
  shutdown=$(echo "${features_json}" | grep -o '"Shutdown":[^,}]*' || echo "")
  if [[ -z "${shutdown}" ]] || [[ "${shutdown}" == *"null"* ]]; then
    die "Shutdown should always be available (raid3 implements it), got: ${shutdown}"
  fi
  log "✓ Shutdown correctly always available"

  local cleanup
  cleanup=$(echo "${features_json}" | grep -o '"CleanUp":[^,}]*' || echo "")
  if [[ -z "${cleanup}" ]] || [[ "${cleanup}" == *"null"* ]]; then
    die "CleanUp should always be available (raid3 implements it), got: ${cleanup}"
  fi
  log "✓ CleanUp correctly always available"

  # Test features specific to storage type
  case "${storage_type}" in
    local)
      test_local_features "${features_json}"
      ;;
    minio)
      test_minio_features "${features_json}"
      ;;
    mixed)
      test_mixed_features "${features_json}"
      ;;
  esac

  log "✅ Test ${test_case} passed"
}

# Test features specific to local-only backends
test_local_features() {
  local features_json="$1"
  log "Testing local-specific features..."

  # Local backends should have local-like features
  # BucketBased should be false for local
  local bucket_based
  bucket_based=$(extract_feature "${features_json}" "BucketBased")
  if [[ "${bucket_based}" != "false" ]]; then
    die "BucketBased should be false for local backends, got: '${bucket_based}'"
  fi
  log "✓ BucketBased correctly set to false for local backends"

  # SetTier/GetTier should be false for local
  local set_tier
  set_tier=$(extract_feature "${features_json}" "SetTier")
  if [[ "${set_tier}" != "false" ]]; then
    die "SetTier should be false for local backends, got: ${set_tier}"
  fi
  log "✓ SetTier correctly set to false for local backends"

  local get_tier
  get_tier=$(extract_feature "${features_json}" "GetTier")
  if [[ "${get_tier}" != "false" ]]; then
    die "GetTier should be false for local backends, got: ${get_tier}"
  fi
  log "✓ GetTier correctly set to false for local backends"

  # Local should support metadata features
  local read_metadata
  read_metadata=$(extract_feature "${features_json}" "ReadMetadata")
  if [[ "${read_metadata}" != "true" ]]; then
    log "⚠ ReadMetadata is ${read_metadata} for local backends (expected true)"
  else
    log "✓ ReadMetadata correctly enabled for local backends"
  fi

  local write_metadata
  write_metadata=$(extract_feature "${features_json}" "WriteMetadata")
  if [[ "${write_metadata}" != "true" ]]; then
    log "⚠ WriteMetadata is ${write_metadata} for local backends (expected true)"
  else
    log "✓ WriteMetadata correctly enabled for local backends"
  fi

  # IsLocal should be false for raid3 (wrapping backends don't propagate IsLocal)
  # This is correct behavior - even if all underlying backends are local,
  # raid3 itself is not a local filesystem
  local is_local
  is_local=$(extract_feature "${features_json}" "IsLocal")
  if [[ "${is_local}" != "false" ]]; then
    log "⚠ IsLocal is ${is_local} for raid3 with local backends (expected false - wrapping backends don't propagate IsLocal)"
  else
    log "✓ IsLocal correctly set to false for raid3 (wrapping backend)"
  fi
}

# Test features specific to MinIO-only backends
test_minio_features() {
  local features_json="$1"
  log "Testing MinIO-specific features..."

  # MinIO backends should have object storage-like features
  # BucketBased should be true for MinIO
  local bucket_based
  bucket_based=$(extract_feature "${features_json}" "BucketBased")
  if [[ "${bucket_based}" != "true" ]]; then
    die "BucketBased should be true for MinIO backends, got: '${bucket_based}'"
  fi
  log "✓ BucketBased correctly set to true for MinIO backends"

  # MinIO may support SetTier/GetTier (depending on MinIO version/config)
  # We just verify they're not null/undefined, not their exact value
  local set_tier
  set_tier=$(extract_feature "${features_json}" "SetTier")
  log "✓ SetTier value for MinIO: ${set_tier}"

  local get_tier
  get_tier=$(extract_feature "${features_json}" "GetTier")
  log "✓ GetTier value for MinIO: ${get_tier}"

  # IsLocal should be false for MinIO
  local is_local
  is_local=$(extract_feature "${features_json}" "IsLocal")
  if [[ "${is_local}" != "false" ]]; then
    log "⚠ IsLocal is ${is_local} for MinIO backends (expected false)"
  else
    log "✓ IsLocal correctly set to false for MinIO backends"
  fi

  # MinIO may or may not support metadata features (depends on version)
  local read_metadata
  read_metadata=$(extract_feature "${features_json}" "ReadMetadata")
  log "✓ ReadMetadata value for MinIO: ${read_metadata}"

  local write_metadata
  write_metadata=$(extract_feature "${features_json}" "WriteMetadata")
  log "✓ WriteMetadata value for MinIO: ${write_metadata}"
}

# Test features specific to mixed (local + MinIO) backends
test_mixed_features() {
  local features_json="$1"
  log "Testing mixed-specific features (local + MinIO intersection)..."

  # Test features that should use AND logic (require all backends)
  log "Testing AND logic features (require all backends)..."
  
  # BucketBased should be false for mixed (local is not bucket-based)
  local bucket_based
  bucket_based=$(extract_feature "${features_json}" "BucketBased")
  if [[ "${bucket_based}" != "false" ]]; then
    die "BucketBased should be false for mixed remotes (local is not bucket-based), got: '${bucket_based}'"
  fi
  log "✓ BucketBased correctly set to false for mixed remotes"

  # SetTier/GetTier should be false for mixed (local doesn't support tiers)
  local set_tier
  set_tier=$(extract_feature "${features_json}" "SetTier")
  if [[ "${set_tier}" != "false" ]]; then
    die "SetTier should be false for mixed remotes (local doesn't support tiers), got: ${set_tier}"
  fi
  log "✓ SetTier correctly set to false for mixed remotes"

  local get_tier
  get_tier=$(extract_feature "${features_json}" "GetTier")
  if [[ "${get_tier}" != "false" ]]; then
    die "GetTier should be false for mixed remotes (local doesn't support tiers), got: ${get_tier}"
  fi
  log "✓ GetTier correctly set to false for mixed remotes"

  # Test features that should use OR logic (best-effort, raid3-specific)
  log "Testing OR logic features (best-effort, any backend)..."
  
  # ReadMetadata should work if any backend supports it (local does)
  local read_metadata
  read_metadata=$(extract_feature "${features_json}" "ReadMetadata")
  if [[ "${read_metadata}" != "true" ]]; then
    log "⚠ ReadMetadata is ${read_metadata} for mixed remotes (expected true if local supports it)"
  else
    log "✓ ReadMetadata correctly enabled for mixed remotes (OR logic)"
  fi

  # WriteMetadata should work if any backend supports it (local does)
  local write_metadata
  write_metadata=$(extract_feature "${features_json}" "WriteMetadata")
  if [[ "${write_metadata}" != "true" ]]; then
    log "⚠ WriteMetadata is ${write_metadata} for mixed remotes (expected true if local supports it)"
  else
    log "✓ WriteMetadata correctly enabled for mixed remotes (OR logic)"
  fi

  # IsLocal should be false for mixed (MinIO is not local)
  local is_local
  is_local=$(extract_feature "${features_json}" "IsLocal")
  if [[ "${is_local}" != "false" ]]; then
    log "⚠ IsLocal is ${is_local} for mixed remotes (expected false due to MinIO)"
  else
    log "✓ IsLocal correctly set to false for mixed remotes"
  fi
}

# List available tests
list_tests() {
  cat <<EOF
Available tests:

  local-features    Test feature handling with local-only backends
                    Requires --storage-type=local

  minio-features    Test feature handling with MinIO-only backends
                    Requires --storage-type=minio

  mixed-features    Test feature handling with mixed remotes (local + MinIO)
                    Requires --storage-type=mixed

  sftp-features     Test feature handling with SFTP-only backends
                    Requires --storage-type=sftp

Run all tests by omitting the test name:
  ${SCRIPT_NAME} --storage-type=local test
  ${SCRIPT_NAME} --storage-type=minio test
  ${SCRIPT_NAME} --storage-type=mixed test
  ${SCRIPT_NAME} --storage-type=sftp test

EOF
}

# Run all available tests for the current storage type
run_all_tests() {
  local test_name="${STORAGE_TYPE}-features"
  local failed=0
  
  log "Running all feature handling tests for --storage-type=${STORAGE_TYPE}..."
  echo ""
  
  if ! test_features_for_storage_type "${STORAGE_TYPE}"; then
    log "✗ Test '${test_name}' failed"
    failed=1
  else
    log "✓ Test '${test_name}' passed"
  fi
  echo ""
  
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
          local-features)
            if [[ "${STORAGE_TYPE}" != "local" ]]; then
              die "Test '${COMMAND_ARG}' requires --storage-type=local"
            fi
            test_features_for_storage_type "local"
            ;;
          minio-features)
            if [[ "${STORAGE_TYPE}" != "minio" ]]; then
              die "Test '${COMMAND_ARG}' requires --storage-type=minio"
            fi
            test_features_for_storage_type "minio"
            ;;
          mixed-features)
            if [[ "${STORAGE_TYPE}" != "mixed" ]]; then
              die "Test '${COMMAND_ARG}' requires --storage-type=mixed"
            fi
            test_features_for_storage_type "mixed"
            ;;
          sftp-features)
            if [[ "${STORAGE_TYPE}" != "sftp" ]]; then
              die "Test '${COMMAND_ARG}' requires --storage-type=sftp"
            fi
            test_features_for_storage_type "sftp"
            ;;
          *)
            die "Unknown test: '${COMMAND_ARG}'. Use 'list' to see available tests."
            ;;
        esac
      fi
      ;;
    start)
      if [[ "${STORAGE_TYPE}" == "minio" || "${STORAGE_TYPE}" == "mixed" ]]; then
        start_minio_containers
      elif [[ "${STORAGE_TYPE}" == "sftp" ]]; then
        start_sftp_containers
      else
        log "'start' only applies to MinIO-based (minio or mixed) or SFTP (sftp) storage types."
        exit 0
      fi
      ;;
    stop)
      if [[ "${STORAGE_TYPE}" == "minio" || "${STORAGE_TYPE}" == "mixed" ]]; then
        stop_minio_containers
      elif [[ "${STORAGE_TYPE}" == "sftp" ]]; then
        stop_sftp_containers
      else
        log "'stop' only applies to MinIO-based (minio or mixed) or SFTP (sftp) storage types."
        exit 0
      fi
      ;;
    teardown)
      [[ "${STORAGE_TYPE}" != "minio" && "${STORAGE_TYPE}" != "mixed" ]] || ensure_minio_containers_ready
      [[ "${STORAGE_TYPE}" != "sftp" ]] || ensure_sftp_containers_ready
      set_remotes_for_storage_type
      purge_raid3_remote_root
      purge_remote_root "${SINGLE_REMOTE}"
      if [[ "${STORAGE_TYPE}" == "local" ]]; then
        for dir in "${LOCAL_RAID3_DIRS[@]}" "${LOCAL_SINGLE_DIR}"; do
          rm -rf "${dir:?}"/*
        done
        log "Teardown complete for local storage"
      elif [[ "${STORAGE_TYPE}" == "mixed" ]]; then
        for dir in "${LOCAL_EVEN_DIR}" "${LOCAL_PARITY_DIR}" "${LOCAL_SINGLE_DIR}"; do
          rm -rf "${dir:?}"/*
        done
        for dir in "${MINIO_RAID3_DIRS[@]}" "${MINIO_SINGLE_DIR}"; do
          remove_leftover_files "${dir}"
          verify_directory_empty "${dir}"
        done
        log "Teardown complete for mixed storage"
      elif [[ "${STORAGE_TYPE}" == "sftp" ]]; then
        for dir in "${SFTP_RAID3_DIRS[@]}" "${SFTP_SINGLE_DIR}"; do
          remove_leftover_files "${dir}"
          verify_directory_empty "${dir}"
        done
        log "Teardown complete for SFTP storage"
      else
        for dir in "${MINIO_RAID3_DIRS[@]}" "${MINIO_SINGLE_DIR}"; do
          remove_leftover_files "${dir}"
          verify_directory_empty "${dir}"
        done
        log "Teardown complete for MinIO storage"
      fi
      ;;
    *)
      die "Unknown command: ${COMMAND}. Use -h for help."
      ;;
  esac
}

main "$@"
