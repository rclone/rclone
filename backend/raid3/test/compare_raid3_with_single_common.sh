# Shared helpers for raid3 comparison and rebuild scripts.
# This file is sourced by compare_raid3_with_single*.sh variants.
# shellcheck shell=bash

# Check if running on native Windows (cmd.exe/PowerShell)
# Note: Git Bash (OSTYPE=msys) and Cygwin (OSTYPE=cygwin) are allowed as they provide Unix-like environments
if [[ -n "${WINDIR:-}" ]] || [[ -n "${SYSTEMROOT:-}" ]]; then
  # Check if we're in WSL, Git Bash, or Cygwin (these should work)
  if [[ "${OSTYPE:-}" != "msys" ]] && [[ "${OSTYPE:-}" != "cygwin" ]] && [[ ! -f /proc/version ]] && [[ ! -d /usr/bin ]]; then
    cat >&2 <<EOF
ERROR: These integration test scripts cannot run natively on Windows (cmd.exe or PowerShell).

These Bash-based integration test scripts require a Unix-like environment.
To run on Windows, please use one of the following options:

1. Windows Subsystem for Linux (WSL)
   - Install WSL from Microsoft Store
   - Run the scripts from within a WSL terminal

2. Git Bash
   - Install Git for Windows (includes Git Bash)
   - Run the scripts from Git Bash terminal

3. Cygwin
   - Install Cygwin
   - Run the scripts from Cygwin terminal

For more information, see the README.md documentation.
EOF
    exit 1
  fi
fi

# Determine script directory so we can locate optional env overrides.
SCRIPT_DIR=${SCRIPT_DIR:-$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)}

# Data directory is relative to script directory
DATA_DIR="${DATA_DIR:-${SCRIPT_DIR}/_data}"

# For backward compatibility, set WORKDIR to SCRIPT_DIR (used in some cleanup functions)
WORKDIR="${SCRIPT_DIR}"

# Initialize VERBOSE if not set (used by purge_remote_root and print_if_verbose)
VERBOSE="${VERBOSE:-0}"

# Load default environment (required – tracked in git).
if [[ ! -f "${SCRIPT_DIR}/compare_raid3_env.sh" ]]; then
  printf '[%s] ERROR: Missing required env file: %s\n' "${SCRIPT_NAME:-compare_raid3_with_single_common.sh}" "${SCRIPT_DIR}/compare_raid3_env.sh" >&2
  exit 1
fi
# shellcheck source=/dev/null
. "${SCRIPT_DIR}/compare_raid3_env.sh"

# Allow user-specific overrides without touching the tracked file (optional).
if [[ -f "${SCRIPT_DIR}/compare_raid3_env.local.sh" ]]; then
  # shellcheck source=/dev/null
  . "${SCRIPT_DIR}/compare_raid3_env.local.sh"
fi

# Resolve rclone config file - only use test-specific config file
# This is the strict approach: tests only use the config file created by setup.sh
# IMPORTANT: Set this AFTER sourcing env files to ensure it takes precedence
# and cannot be overridden by compare_raid3_env.local.sh
TEST_SPECIFIC_CONFIG="${SCRIPT_DIR}/rclone_raid3_integration_tests.config"
RCLONE_CONFIG="${TEST_SPECIFIC_CONFIG}"

# Directory layout used by the configured backends. All variables below are
# expected to come from compare_raid3_env.sh (or its local override).
LOCAL_RAID3_DIRS=(
  "${LOCAL_EVEN_DIR}"
  "${LOCAL_ODD_DIR}"
  "${LOCAL_PARITY_DIR}"
)
# Document that LOCAL_SINGLE_DIR comes from env (compare_raid3_env.sh)
# shellcheck disable=SC2269
LOCAL_SINGLE_DIR="${LOCAL_SINGLE_DIR}"
LOCAL_RAID3_REMOTES=(
  "${LOCAL_EVEN_REMOTE}"
  "${LOCAL_ODD_REMOTE}"
  "${LOCAL_PARITY_REMOTE}"
)

MINIO_RAID3_DIRS=(
  "${MINIO_EVEN_DIR}"
  "${MINIO_ODD_DIR}"
  "${MINIO_PARITY_DIR}"
)
# Document that MINIO_SINGLE_DIR comes from env (compare_raid3_env.sh)
# shellcheck disable=SC2269
MINIO_SINGLE_DIR="${MINIO_SINGLE_DIR}"
MINIO_RAID3_REMOTES=(
  "${MINIO_EVEN_REMOTE}"
  "${MINIO_ODD_REMOTE}"
  "${MINIO_PARITY_REMOTE}"
)
MINIO_S3_PORTS=(
  "${MINIO_EVEN_PORT}"
  "${MINIO_ODD_PORT}"
  "${MINIO_PARITY_PORT}"
  "${MINIO_SINGLE_PORT}"
)

# Directories explicitly allowed for cleanup
ALLOWED_DATA_DIRS=(
  "${LOCAL_RAID3_DIRS[@]}"
  "${LOCAL_SINGLE_DIR}"
  "${MINIO_RAID3_DIRS[@]}"
  "${MINIO_SINGLE_DIR}"
)

# Definition of MinIO containers: name|user|password|s3_port|console_port|data_dir
MINIO_CONTAINERS=(
  "${MINIO_EVEN_NAME}|${MINIO_EVEN_USER:-even}|${MINIO_EVEN_PASS:-evenpass88}|${MINIO_EVEN_PORT}|9004|${MINIO_EVEN_DIR}"
  "${MINIO_ODD_NAME}|${MINIO_ODD_USER:-odd}|${MINIO_ODD_PASS:-oddpass88}|${MINIO_ODD_PORT}|9005|${MINIO_ODD_DIR}"
  "${MINIO_PARITY_NAME}|${MINIO_PARITY_USER:-parity}|${MINIO_PARITY_PASS:-paritypass88}|${MINIO_PARITY_PORT}|9006|${MINIO_PARITY_DIR}"
  "${MINIO_SINGLE_NAME}|${MINIO_SINGLE_USER:-single}|${MINIO_SINGLE_PASS:-singlepass88}|${MINIO_SINGLE_PORT}|9007|${MINIO_SINGLE_DIR}"
)

log_tag() {
  local tag="$1"
  shift
  printf '[%s] %s %s\n' "${SCRIPT_NAME}" "${tag}" "$*"
}

log_info() {
  log_tag "INFO" "$*"
}

log_warn() {
  log_tag "WARN" "$*"
}

log_pass() {
  log_tag "PASS" "$*"
}

log_fail() {
  log_tag "FAIL" "$*"
}

log_note() {
  log_tag "NOTE" "$*"
}

log() {
  log_info "$*"
}

die() {
  local prefix="[${SCRIPT_NAME}] ERROR:"
  # Print each argument on a new line
  for msg in "$@"; do
    printf '%s %s\n' "${prefix}" "${msg}" >&2
    prefix="[${SCRIPT_NAME}]"
  done
  exit 1
}

ensure_workdir() {
  # Check if script directory exists
  if [[ ! -d "${SCRIPT_DIR}" ]]; then
    die "Integration test directory does not exist: ${SCRIPT_DIR}" \
        "Please ensure you are running from the correct location."
  fi
  
  # Check if we're running from the correct directory
  if [[ "${PWD}" != "${SCRIPT_DIR}" ]]; then
    die "This script must be run from ${SCRIPT_DIR} (current: ${PWD})" \
        "Please change to the test directory: cd ${SCRIPT_DIR}"
  fi
}

ensure_rclone_config() {
  # Verify that rclone_raid3_integration_tests.config exists
  if [[ ! -f "${RCLONE_CONFIG}" ]]; then
    die "Integration test configuration file not found: ${RCLONE_CONFIG}" \
        "" \
        "The test-specific config file is missing. You need to run the setup script first." \
        "" \
        "Please run:" \
        "  cd ${SCRIPT_DIR}" \
        "  ./setup.sh" \
        "" \
        "This will create the configuration file: ${RCLONE_CONFIG}"
  fi
  log_info "config" "Using rclone config: ${RCLONE_CONFIG}"
}

create_rclone_config() {
  local config_file="${1:-${TEST_SPECIFIC_CONFIG}}"
  local force="${2:-0}"
  
  if [[ -f "${config_file}" && "${force}" -eq 0 ]]; then
    log_warn "config" "Config file already exists: ${config_file}"
    log_warn "config" "Skipping config file creation (idempotent behavior)"
    return 1
  fi
  
  # Ensure directory exists
  local config_dir
  config_dir=$(dirname "${config_file}")
  if [[ ! -d "${config_dir}" ]]; then
    mkdir -p "${config_dir}" || die "Failed to create config directory: ${config_dir}"
  fi
  
  log_info "config" "Creating rclone config file: ${config_file}"
  
  # Convert absolute paths to relative paths (relative to test directory)
  # This ensures the config file is portable across different machines
  make_relative_path() {
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
  
  local LOCAL_EVEN_DIR_REL LOCAL_ODD_DIR_REL LOCAL_PARITY_DIR_REL LOCAL_SINGLE_DIR_REL
  LOCAL_EVEN_DIR_REL=$(make_relative_path "${LOCAL_EVEN_DIR}")
  LOCAL_ODD_DIR_REL=$(make_relative_path "${LOCAL_ODD_DIR}")
  LOCAL_PARITY_DIR_REL=$(make_relative_path "${LOCAL_PARITY_DIR}")
  LOCAL_SINGLE_DIR_REL=$(make_relative_path "${LOCAL_SINGLE_DIR}")
  
  # Obscure passwords for crypt backends
  # rclone crypt backend requires passwords to be obscured in the config file
  # Use test passwords that are consistent across test runs
  local CRYPT_PASSWORD="testpassword123"
  local CRYPT_SALT="testsalt456"
  local CRYPT_PASSWORD_OBSCURED CRYPT_SALT_OBSCURED
  
  # Try to find rclone binary for obscuring passwords
  # We need to obscure passwords before writing to config file
  local rclone_bin=""
  if [[ -n "${RCLONE_BINARY:-}" ]] && [[ -x "${RCLONE_BINARY}" ]]; then
    rclone_bin="${RCLONE_BINARY}"
  elif command -v rclone >/dev/null 2>&1; then
    rclone_bin="rclone"
  else
    # Try to find rclone binary in repo root (same logic as find_rclone_binary but without die)
    local repo_root
    repo_root=$(cd "${SCRIPT_DIR}/../../.." && pwd)
    if [[ -f "${repo_root}/rclone.go" ]] || [[ -f "${repo_root}/Makefile" ]]; then
      if [[ -f "${repo_root}/rclone" ]] && [[ -x "${repo_root}/rclone" ]]; then
        rclone_bin="${repo_root}/rclone"
      elif [[ -f "${repo_root}/rclone.exe" ]] && [[ -x "${repo_root}/rclone.exe" ]]; then
        rclone_bin="${repo_root}/rclone.exe"
      fi
    fi
  fi
  
  # Obscure passwords using rclone obscure command
  if [[ -n "${rclone_bin}" ]]; then
    CRYPT_PASSWORD_OBSCURED=$(echo -n "${CRYPT_PASSWORD}" | "${rclone_bin}" obscure - 2>/dev/null || echo "")
    CRYPT_SALT_OBSCURED=$(echo -n "${CRYPT_SALT}" | "${rclone_bin}" obscure - 2>/dev/null || echo "")
    
    # Verify that obscuring worked
    if [[ -z "${CRYPT_PASSWORD_OBSCURED}" ]] || [[ -z "${CRYPT_SALT_OBSCURED}" ]]; then
      log_warn "config" "Failed to obscure crypt passwords, but continuing (rclone may obscure them automatically)"
      CRYPT_PASSWORD_OBSCURED="${CRYPT_PASSWORD}"
      CRYPT_SALT_OBSCURED="${CRYPT_SALT}"
    elif [[ "${CRYPT_PASSWORD_OBSCURED}" == "${CRYPT_PASSWORD}" ]] || [[ "${CRYPT_SALT_OBSCURED}" == "${CRYPT_SALT}" ]]; then
      log_warn "config" "Password obscuring may have failed (output same as input), but continuing"
    fi
  else
    # If rclone is not available, we cannot obscure passwords
    # This will cause the crypt backend to fail, but we'll let the user know
    log_warn "config" "Cannot obscure crypt passwords (rclone not available)"
    log_warn "config" "The crypt backends will not work until passwords are obscured"
    log_warn "config" "Please run: rclone obscure 'testpassword123' and rclone obscure 'testsalt456'"
    log_warn "config" "Then update the config file manually, or regenerate after building rclone"
    CRYPT_PASSWORD_OBSCURED="${CRYPT_PASSWORD}"
    CRYPT_SALT_OBSCURED="${CRYPT_SALT}"
  fi
  
  # Generate config file content
  cat > "${config_file}" <<EOF
# rclone configuration file for raid3 integration tests
# Generated by: ${SCRIPT_NAME:-compare_raid3_with_single_common.sh}
# Generated on: $(date -u +"%Y-%m-%d %H:%M:%S UTC")
#
# This config file contains remotes for testing the raid3 backend.
# You can edit this file to customize remote names or paths.
# NOTE: Paths are relative to the test directory (backend/raid3/test)
#
# Local storage remotes
[${LOCAL_EVEN_REMOTE}]
type = local

[${LOCAL_ODD_REMOTE}]
type = local

[${LOCAL_PARITY_REMOTE}]
type = local

# RAID3 remote using local storage
[localraid3]
type = raid3
even = ${LOCAL_EVEN_REMOTE}:${LOCAL_EVEN_DIR_REL}
odd = ${LOCAL_ODD_REMOTE}:${LOCAL_ODD_DIR_REL}
parity = ${LOCAL_PARITY_REMOTE}:${LOCAL_PARITY_DIR_REL}
timeout_mode = aggressive
auto_cleanup = true
auto_heal = false

# Single local remote (alias type)
[${LOCAL_SINGLE_REMOTE}]
type = alias
remote = ${LOCAL_SINGLE_DIR_REL}

# Crypt backends for stacking tests
# cryptlocalsingle wraps localsingle
[cryptlocalsingle]
type = crypt
remote = ${LOCAL_SINGLE_REMOTE}:
filename_encryption = standard
directory_name_encryption = true
password = ${CRYPT_PASSWORD_OBSCURED}
password2 = ${CRYPT_SALT_OBSCURED}

# cryptlocalraid3 wraps localraid3
[cryptlocalraid3]
type = crypt
remote = localraid3:
filename_encryption = standard
directory_name_encryption = true
password = ${CRYPT_PASSWORD_OBSCURED}
password2 = ${CRYPT_SALT_OBSCURED}

# Chunker backends for stacking_chunker test (chunk_size=100B so test file splits into >=2 chunks)
[chunkerlocalsingle]
type = chunker
remote = ${LOCAL_SINGLE_REMOTE}:
chunk_size = 100B
hash_type = md5

[chunkerlocalraid3]
type = chunker
remote = localraid3:
chunk_size = 100B
hash_type = md5

# MinIO S3 remotes
[${MINIO_EVEN_REMOTE}]
type = s3
provider = Minio
env_auth = false
access_key_id = ${MINIO_EVEN_USER:-even}
secret_access_key = ${MINIO_EVEN_PASS:-evenpass88}
endpoint = http://127.0.0.1:${MINIO_EVEN_PORT}
acl = private
no_check_bucket = false
max_retries = 1

[${MINIO_ODD_REMOTE}]
type = s3
provider = Minio
env_auth = false
access_key_id = ${MINIO_ODD_USER:-odd}
secret_access_key = ${MINIO_ODD_PASS:-oddpass88}
endpoint = http://127.0.0.1:${MINIO_ODD_PORT}
acl = private
no_check_bucket = false
max_retries = 1

[${MINIO_PARITY_REMOTE}]
type = s3
provider = Minio
env_auth = false
access_key_id = ${MINIO_PARITY_USER:-parity}
secret_access_key = ${MINIO_PARITY_PASS:-paritypass88}
endpoint = http://127.0.0.1:${MINIO_PARITY_PORT}
acl = private
no_check_bucket = false
max_retries = 1

# RAID3 remote using MinIO storage
[minioraid3]
type = raid3
even = ${MINIO_EVEN_REMOTE}:
odd = ${MINIO_ODD_REMOTE}:
parity = ${MINIO_PARITY_REMOTE}:
timeout_mode = aggressive
auto_cleanup = true
auto_heal = false

# RAID3 remote using local and minio storage (mixed file/object backend)
[localminioraid3]
type = raid3
even = ${LOCAL_EVEN_REMOTE}:${LOCAL_EVEN_DIR_REL}
odd = ${MINIO_ODD_REMOTE}:
parity = ${LOCAL_PARITY_REMOTE}:${LOCAL_PARITY_DIR_REL}
timeout_mode = aggressive
auto_cleanup = true
auto_heal = false

[${MINIO_SINGLE_REMOTE}]
type = s3
provider = Minio
env_auth = false
access_key_id = ${MINIO_SINGLE_USER:-single}
secret_access_key = ${MINIO_SINGLE_PASS:-singlepass88}
endpoint = http://127.0.0.1:${MINIO_SINGLE_PORT}
acl = private
no_check_bucket = false
max_retries = 1

# Crypt backends for stacking tests with MinIO
# cryptminiosingle wraps miniosingle
[cryptminiosingle]
type = crypt
remote = ${MINIO_SINGLE_REMOTE}:
filename_encryption = standard
directory_name_encryption = true
password = ${CRYPT_PASSWORD_OBSCURED}
password2 = ${CRYPT_SALT_OBSCURED}

# cryptminioraid3 wraps minioraid3
[cryptminioraid3]
type = crypt
remote = minioraid3:
filename_encryption = standard
directory_name_encryption = true
password = ${CRYPT_PASSWORD_OBSCURED}
password2 = ${CRYPT_SALT_OBSCURED}

# Chunker backends for stacking_chunker test with MinIO (use explicit bucket for S3)
[chunkerminiosingle]
type = chunker
remote = ${MINIO_SINGLE_REMOTE}:chunker
chunk_size = 100B
hash_type = md5

[chunkerminioraid3]
type = chunker
remote = minioraid3:chunker
chunk_size = 100B
hash_type = md5

EOF
  
  log_pass "config" "Config file created successfully: ${config_file}"
  log_note "config" "You can now run integration tests. The config file will be used automatically."
  return 0
}

# Find the rclone binary - requires local build in repo root
find_rclone_binary() {
  # Find repository root by looking for rclone.go or Makefile
  # Script is in backend/raid3/test, so repo root is 3 levels up
  local repo_root
  repo_root=$(cd "${SCRIPT_DIR}/../../.." && pwd)
  
  # Check if this looks like the repo root
  if [[ ! -f "${repo_root}/rclone.go" ]] && [[ ! -f "${repo_root}/Makefile" ]]; then
    # Couldn't find repo root
    die "Could not find repository root (looking for rclone.go or Makefile)" \
        "Expected repository root at: ${repo_root}" \
        "Please ensure you are running the tests from the rclone repository."
  fi
  
  # Check for local rclone binary in repo root (where 'go build' puts it)
  # We ONLY use the repo root version to ensure tests use the locally compiled version
  if [[ -f "${repo_root}/rclone" ]] && [[ -x "${repo_root}/rclone" ]]; then
    echo "${repo_root}/rclone"
    return 0
  elif [[ -f "${repo_root}/rclone.exe" ]] && [[ -x "${repo_root}/rclone.exe" ]]; then
    echo "${repo_root}/rclone.exe"
    return 0
  fi
  
  # Binary not found in repo root - provide helpful error message
  die "Local rclone binary not found in repository root: ${repo_root}" \
      "The integration tests require a locally built rclone binary in the repository root." \
      "" \
      "Please compile rclone first:" \
      "  cd ${repo_root}" \
      "  go build" \
      "" \
      "This will create the binary at: ${repo_root}/rclone" \
      "" \
      "Note: The tests use the binary from the repository root, not from \$GOPATH/bin," \
      "to ensure you're testing the version you're actively developing."
}

# Cache the rclone binary path (can be overridden via RCLONE_BINARY env var)
RCLONE_BINARY="${RCLONE_BINARY:-$(find_rclone_binary)}"

ensure_rclone_binary() {
  # Verify that the rclone binary exists and is executable
  if [[ ! -f "${RCLONE_BINARY}" ]]; then
    die "Rclone binary not found: ${RCLONE_BINARY}" \
        "The integration tests require a locally built rclone binary." \
        "" \
        "Please compile rclone first:" \
        "  cd $(cd "${SCRIPT_DIR}/../../.." && pwd)" \
        "  go build"
  fi
  if [[ ! -x "${RCLONE_BINARY}" ]]; then
    die "Rclone binary is not executable: ${RCLONE_BINARY}" \
        "Please check the file permissions."
  fi
  log_info "binary" "Using rclone binary: ${RCLONE_BINARY}"
}

rclone_cmd() {
  # Use --retries 1 for faster failure in tests (avoid 3 retries causing long delays)
  # Use local rclone binary from repo root if available, otherwise system rclone
  "${RCLONE_BINARY}" --config "${RCLONE_CONFIG}" --retries 1 "$@"
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

capture_command_timed() {
  local label="$1"
  shift

  local out_file err_file status
  out_file=$(mktemp "/tmp/${label}.stdout.XXXXXX")
  err_file=$(mktemp "/tmp/${label}.stderr.XXXXXX")

  local start_time end_time elapsed
  start_time=$(date +%s.%N)
  
  set +e
  rclone_cmd "$@" >"${out_file}" 2>"${err_file}"
  status=$?
  set -e
  
  end_time=$(date +%s.%N)
  # Force LC_NUMERIC=C to ensure dot as decimal separator
  elapsed=$(LC_NUMERIC=C awk "BEGIN {printf \"%.3f\", ${end_time} - ${start_time}}")

  printf '%s|%s|%s|%s\n' "${status}" "${out_file}" "${err_file}" "${elapsed}"
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

wait_for_minio_port() {
  local port="$1"
  local retries=30
  local delay=1
  while (( retries > 0 )); do
    if nc -z localhost "${port}" >/dev/null 2>&1; then
      return 0
    fi
    sleep "${delay}"
    (( retries-- ))
  done
  return 1
}

# Wait for MinIO backend to be ready by attempting a simple S3 operation
# This verifies MinIO is not just listening on the port, but actually ready to accept requests
wait_for_minio_backend_ready() {
  local backend="$1"
  local remote
  case "${backend}" in
    even) remote="${MINIO_EVEN_REMOTE}" ;;
    odd) remote="${MINIO_ODD_REMOTE}" ;;
    parity) remote="${MINIO_PARITY_REMOTE}" ;;
    *) return 1 ;;
  esac

  local retries=30
  local delay=1
  while (( retries > 0 )); do
    # Attempt a simple S3 operation (ls) to verify MinIO is ready
    # Capture both stdout and stderr to check for success or acceptable errors
    local output
    output=$(rclone_cmd ls "${remote}:" 2>&1)
    local status=$?
    
    # Success (status 0) means backend is ready
    if [[ ${status} -eq 0 ]]; then
      return 0
    fi
    
    # ErrorDirNotFound is also acceptable (backend is ready, just empty)
    if echo "${output}" | grep -qiE "(directory not found|bucket.*not found|no such bucket)"; then
      return 0
    fi
    
    # Connection errors mean backend is not ready yet - keep retrying
    if echo "${output}" | grep -qiE "(connection reset|connection refused|no route to host|timeout)"; then
      sleep "${delay}"
      (( retries-- ))
      continue
    fi
    
    # Other errors might indicate backend is ready but has issues - accept as ready
    # (better to proceed than wait forever)
    return 0
  done
  return 1
}

minio_container_for_backend() {
  local backend="$1"
  case "${backend}" in
    even) echo "minioeven" ;;
    odd) echo "minioodd" ;;
    parity) echo "minioparity" ;;
    *) echo "" ;;
  esac
}

stop_single_minio_container() {
  local backend="$1"
  local name
  name=$(minio_container_for_backend "${backend}")
  [[ -n "${name}" ]] || return
  if container_running "${name}"; then
    log_info "docker" "Stopping container '${name}' for backend '${backend}'."
    docker stop "${name}" >/dev/null
  fi
}

# Stop or disable a backend to simulate unavailability
# Handles both MinIO (stop container) and local (rename directory) backends
stop_backend() {
  local backend="$1"
  
  # Determine if this specific backend is MinIO or local
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
    # MinIO backend: stop the container
    stop_single_minio_container "${backend}"
  else
    # Local backend: make the directory unavailable
    # For local backends, we need to cause actual errors, not just "directory not found"
    # Strategy: Replace directory with a file - this causes "not a directory" errors
    local dir
    dir=$(remote_data_dir "${backend}")
    if [[ -d "${dir}" ]]; then
      local backup_dir="${dir}.disabled"
      log_info "backend" "Disabling local backend '${backend}' by renaming directory: ${dir} -> ${backup_dir}"
      
      # Rename directory to backup location
      mv "${dir}" "${backup_dir}" 2>/dev/null || {
        log_warn "backend" "Failed to rename directory ${dir}, trying alternative method"
        # Alternative: Remove all permissions to make it inaccessible
        chmod 000 "${dir}" 2>/dev/null || true
        return
      }
      
      # Create a file with the same name - this will cause "not a directory" errors
      # which RAID3 will treat as a real error (not acceptable like ErrorDirNotFound)
      touch "${dir}" 2>/dev/null || {
        log_warn "backend" "Failed to create blocking file at ${dir}"
        # Fallback: restore directory and try permission method
        mv "${backup_dir}" "${dir}" 2>/dev/null || true
        chmod 000 "${dir}" 2>/dev/null || true
      }
    fi
  fi
}

# Start or enable a backend after it was stopped/disabled
# Handles both MinIO (start container) and local (restore directory) backends
start_backend() {
  local backend="$1"
  
  # Determine if this specific backend is MinIO or local
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
    # MinIO backend: start the container
    start_single_minio_container "${backend}"
  else
    # Local backend: restore the directory
    local dir
    dir=$(remote_data_dir "${backend}")
    local backup_dir="${dir}.disabled"
    
    # Remove the blocking file if it exists
    if [[ -f "${dir}" ]]; then
      log_info "backend" "Removing blocking file: ${dir}"
      rm -f "${dir}" 2>/dev/null || true
    fi
    
    # Restore directory from backup
    if [[ -d "${backup_dir}" ]]; then
      log_info "backend" "Restoring local backend '${backend}' by renaming directory: ${backup_dir} -> ${dir}"
      mv "${backup_dir}" "${dir}" 2>/dev/null || {
        log_warn "backend" "Failed to restore directory ${backup_dir}"
      }
    elif [[ ! -d "${dir}" ]]; then
      # Directory doesn't exist - might have been made inaccessible via chmod
      if [[ -e "${dir}" ]]; then
        # File or inaccessible directory exists - restore permissions
        log_info "backend" "Restoring local backend '${backend}' by restoring permissions: ${dir}"
        chmod 755 "${dir}" 2>/dev/null || true
      else
        # Doesn't exist - create it
        log_info "backend" "Creating local backend '${backend}' directory: ${dir}"
        mkdir -p "${dir}" 2>/dev/null || true
        chmod 755 "${dir}" 2>/dev/null || true
      fi
    fi
  fi
}

start_single_minio_container() {
  local backend="$1"
  local name
  name=$(minio_container_for_backend "${backend}")
  [[ -n "${name}" ]] || return
  if container_exists "${name}"; then
    log_info "docker" "Starting container '${name}' for backend '${backend}'."
    docker start "${name}" >/dev/null
  else
    # Fallback to launching via start_minio_containers (ensures config).
    log_info "docker" "Container '${name}' missing; launching all MinIO containers."
    start_minio_containers
  fi
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

ensure_minio_containers_ready() {
  # For mixed storage type, we need MinIO for the odd backend
  if [[ "${STORAGE_TYPE}" != "minio" && "${STORAGE_TYPE}" != "mixed" ]]; then
    return 0
  fi

  local entry started=0
  for entry in "${MINIO_CONTAINERS[@]}"; do
    IFS='|' read -r name _ _ _ _ data_dir <<<"${entry}"
    ensure_directory "${data_dir}"
    if container_running "${name}"; then
      log_info "autostart" "Container '${name}' already running."
      continue
    fi
    started=1
    if container_exists "${name}"; then
      log_info "autostart" "Starting container '${name}'."
      docker start "${name}" >/dev/null || return 1
    else
      log_info "autostart" "Container '${name}' missing; launching full MinIO set."
      start_minio_containers
      started=0
      break
    fi
  done

  # Wait for S3 ports to come online
  local idx=0
  for entry in "${MINIO_CONTAINERS[@]}"; do
    IFS='|' read -r name _ _ _ _ _ <<<"${entry}"
    local port="${MINIO_S3_PORTS[idx]}"
    log_info "autostart" "Waiting for ${name} (port ${port})..."
    if ! wait_for_minio_port "${port}"; then
      log_fail "autostart" "Port ${port} for ${name} did not open in time."
      return 1
    fi
    ((idx++))
  done

  if (( started )); then
    log_info "autostart" "MinIO containers are ready."
  else
    log_info "autostart" "All MinIO containers already running."
  fi
  return 0
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

# Purge only the contents of the remote root, never the root itself
# (so that local dirs like even_local, odd_local, etc. are not removed)
purge_remote_root() {
  local remote="$1"
  log "Purging remote '${remote}:'"

  local entry
  local lsf_output
  lsf_output=$(rclone_cmd lsf "${remote}:" 2>/dev/null | grep -v '^$' || true)
  if [[ -z "${lsf_output}" ]]; then
    if (( VERBOSE )); then
      log "  (no top-level entries on ${remote})"
    fi
    return 0
  fi
  while IFS= read -r entry; do
    entry="${entry%/}"
    [[ -z "${entry}" ]] && continue
    if (( VERBOSE )); then
      log "  - purging ${remote}:${entry}"
    fi
    rclone_cmd purge "${remote}:${entry}" >/dev/null 2>&1 || true
  done <<<"${lsf_output}"
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

cleanup_raid3_dataset_raw() {
  local dataset_id="$1"
  case "${STORAGE_TYPE}" in
    local)
      local idx dir
      for dir in "${LOCAL_RAID3_DIRS[@]}"; do
        if [[ -d "${dir}/${dataset_id}" ]]; then
          rm -rf "${dir:?}/${dataset_id}"
        fi
      done
      ;;
    minio)
      local remote
      for remote in "${MINIO_RAID3_REMOTES[@]}"; do
        rclone_cmd purge "${remote}:${dataset_id}" >/dev/null 2>&1 || true
      done
      ;;
    mixed)
      # Mixed: even and parity are local, odd is MinIO
      if [[ -d "${LOCAL_EVEN_DIR}/${dataset_id}" ]]; then
        rm -rf "${LOCAL_EVEN_DIR:?}/${dataset_id}"
      fi
      if [[ -d "${LOCAL_PARITY_DIR}/${dataset_id}" ]]; then
        rm -rf "${LOCAL_PARITY_DIR:?}/${dataset_id}"
      fi
      rclone_cmd purge "${MINIO_ODD_REMOTE}:${dataset_id}" >/dev/null 2>&1 || true
      ;;
    *)
      ;;
  esac
}

backend_remote_name() {
  local backend="$1"
  case "${STORAGE_TYPE}" in
    local)
      case "${backend}" in
        even) echo "${LOCAL_RAID3_REMOTES[0]}" ;;
        odd) echo "${LOCAL_RAID3_REMOTES[1]}" ;;
        parity) echo "${LOCAL_RAID3_REMOTES[2]}" ;;
        *) die "Unknown backend '${backend}'" ;;
      esac
      ;;
    minio)
      case "${backend}" in
        even) echo "${MINIO_RAID3_REMOTES[0]}" ;;
        odd) echo "${MINIO_RAID3_REMOTES[1]}" ;;
        parity) echo "${MINIO_RAID3_REMOTES[2]}" ;;
        *) die "Unknown backend '${backend}'" ;;
      esac
      ;;
    mixed)
      case "${backend}" in
        even) echo "${LOCAL_RAID3_REMOTES[0]}" ;;
        odd) echo "${MINIO_RAID3_REMOTES[1]}" ;;
        parity) echo "${LOCAL_RAID3_REMOTES[2]}" ;;
        *) die "Unknown backend '${backend}'" ;;
      esac
      ;;
    *)
      die "Unsupported storage type '${STORAGE_TYPE}'"
      ;;
  esac
}

remote_data_dir() {
  local backend="$1"
  case "${STORAGE_TYPE}" in
    local)
      case "${backend}" in
        even) echo "${LOCAL_RAID3_DIRS[0]}" ;;
        odd) echo "${LOCAL_RAID3_DIRS[1]}" ;;
        parity) echo "${LOCAL_RAID3_DIRS[2]}" ;;
        *) die "Unknown backend '${backend}'" ;;
      esac
      ;;
    minio)
      case "${backend}" in
        even) echo "${MINIO_RAID3_DIRS[0]}" ;;
        odd) echo "${MINIO_RAID3_DIRS[1]}" ;;
        parity) echo "${MINIO_RAID3_DIRS[2]}" ;;
        *) die "Unknown backend '${backend}'" ;;
      esac
      ;;
    mixed)
      case "${backend}" in
        even) echo "${LOCAL_RAID3_DIRS[0]}" ;;
        odd) echo "${MINIO_RAID3_DIRS[1]}" ;;
        parity) echo "${LOCAL_RAID3_DIRS[2]}" ;;
        *) die "Unknown backend '${backend}'" ;;
      esac
      ;;
    *)
      die "Unsupported storage type '${STORAGE_TYPE}'"
      ;;
  esac
}

remove_dataset_from_backend() {
  local backend="$1"
  local dataset_id="$2"
  case "${STORAGE_TYPE}" in
    local)
      local dir
      dir=$(remote_data_dir "${backend}")
      rm -rf "${dir:?}/${dataset_id}"
      ;;
    minio)
      local remote
      remote=$(backend_remote_name "${backend}")
      rclone_cmd purge "${remote}:${dataset_id}" >/dev/null 2>&1 || true
      ;;
    mixed)
      # Mixed: even and parity are local, odd is MinIO
      case "${backend}" in
        even|parity)
          local dir
          dir=$(remote_data_dir "${backend}")
          rm -rf "${dir:?}/${dataset_id}"
          ;;
        odd)
          local remote
          remote=$(backend_remote_name "${backend}")
          rclone_cmd purge "${remote}:${dataset_id}" >/dev/null 2>&1 || true
          ;;
        *)
          ;;
      esac
      ;;
    *)
      ;;
  esac
}

object_exists_in_backend() {
  local backend="$1"
  local dataset_id="$2"
  local relative_path="$3"
  case "${STORAGE_TYPE}" in
    local)
      local dir
      dir=$(remote_data_dir "${backend}")
      [[ -f "${dir}/${dataset_id}/${relative_path}" ]]
      ;;
    minio)
      local remote
      remote=$(backend_remote_name "${backend}")
      rclone_cmd lsl "${remote}:${dataset_id}/${relative_path}" >/dev/null 2>&1
      ;;
    mixed)
      # Mixed: even and parity are local, odd is MinIO
      case "${backend}" in
        even|parity)
          local dir
          dir=$(remote_data_dir "${backend}")
          [[ -f "${dir}/${dataset_id}/${relative_path}" ]]
          ;;
        odd)
          local remote
          remote=$(backend_remote_name "${backend}")
          rclone_cmd lsl "${remote}:${dataset_id}/${relative_path}" >/dev/null 2>&1
          ;;
        *)
          return 1
          ;;
      esac
      ;;
    *)
      return 1
      ;;
  esac
}

wait_for_object_in_backend() {
  local backend="$1"
  local dataset_id="$2"
  local relative_path="$3"
  local attempts=20
  local delay=1
  while (( attempts > 0 )); do
    if object_exists_in_backend "${backend}" "${dataset_id}" "${relative_path}"; then
      return 0
    fi
    sleep "${delay}"
    ((attempts--))
  done
  return 1
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
  test_id="cmp-${label}-${timestamp}-${random_suffix}"

  local tmpfile1 tmpfile2
  tmpfile1=$(mktemp) || return 1
  tmpfile2=$(mktemp) || { rm -f "${tmpfile1}"; return 1; }

  printf 'Sample data for %s (root file)\n' "${label}" >"${tmpfile1}"
  printf 'Sample data for %s (nested file)\n' "${label}" >"${tmpfile2}"

  local remote
  for remote in "${RAID3_REMOTE}" "${SINGLE_REMOTE}"; do
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
      # Allow generic override via RAID3_REMOTE environment variable
      RAID3_REMOTE="${RAID3_REMOTE:-localraid3}"
      SINGLE_REMOTE="${SINGLE_REMOTE:-localsingle}"
      ;;
    minio)
      # Allow generic override via RAID3_REMOTE environment variable
      RAID3_REMOTE="${RAID3_REMOTE:-minioraid3}"
      SINGLE_REMOTE="${SINGLE_REMOTE:-miniosingle}"
      ;;
    mixed)
      # Mixed storage: local for even/parity, MinIO for odd
      RAID3_REMOTE="${RAID3_REMOTE:-localminioraid3}"
      SINGLE_REMOTE="${SINGLE_REMOTE:-localsingle}"
      ;;
    *)
      die "Unsupported storage type '${STORAGE_TYPE}'"
      ;;
  esac
}