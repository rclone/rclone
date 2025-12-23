#!/usr/bin/env bash
#
# setup.sh
# --------
# Setup script for RAID3 integration tests.
#
# This script initializes the test environment by:
# 1. Creating a working directory for test data
# 2. Creating all required subdirectories (local and MinIO data directories)
# 3. Generating the rclone configuration file
# 4. Storing the working directory path in $HOME/.rclone_raid3_integration_tests.workdir
#
# Usage:
#   setup.sh [--workdir <path>]
#
# Options:
#   --workdir <path>   Specify the working directory (default: ${HOME}/go/raid3storage)
#
# The script is idempotent and can be run multiple times safely.
# -----------------------------------------------------------------------------

set -euo pipefail

# Check if running on native Windows (cmd.exe/PowerShell)
# Note: Git Bash (OSTYPE=msys) and Cygwin (OSTYPE=cygwin) are allowed as they provide Unix-like environments
if [[ -n "${WINDIR:-}" ]] || [[ -n "${SYSTEMROOT:-}" ]]; then
  # Check if we're in WSL, Git Bash, or Cygwin (these should work)
  if [[ "${OSTYPE:-}" != "msys" ]] && [[ "${OSTYPE:-}" != "cygwin" ]] && [[ ! -f /proc/version ]] && [[ ! -d /usr/bin ]]; then
    cat >&2 <<EOF
ERROR: This script cannot run natively on Windows (cmd.exe or PowerShell).

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

SCRIPT_NAME=$(basename "$0")
SCRIPT_DIR=$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)

# Default workdir
DEFAULT_WORKDIR="${HOME}/go/raid3storage"
WORKDIR="${DEFAULT_WORKDIR}"

# Parse arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
    --workdir)
      shift
      [[ $# -gt 0 ]] || { echo "ERROR: --workdir requires an argument" >&2; exit 1; }
      WORKDIR="$1"
      ;;
    --workdir=*)
      WORKDIR="${1#*=}"
      ;;
    -h|--help)
      cat <<EOF
Usage: ${SCRIPT_NAME} [--workdir <path>]

Setup the RAID3 integration test environment.

Options:
  --workdir <path>   Specify the working directory (default: ${DEFAULT_WORKDIR})
  -h, --help         Display this help message

This script will:
  1. Create the working directory and all required subdirectories
  2. Create the rclone configuration file: \${WORKDIR}/rclone_raid3_integration_tests.config
  3. Store the working directory path in: \${HOME}/.rclone_raid3_integration_tests.workdir

The script is idempotent and safe to run multiple times.
EOF
      exit 0
      ;;
    *)
      echo "ERROR: Unknown option: $1" >&2
      echo "Use --help for usage information." >&2
      exit 1
      ;;
  esac
  shift
done

# Resolve absolute path
if [[ -d "${WORKDIR}" ]]; then
  WORKDIR=$(cd -- "${WORKDIR}" && pwd)
elif [[ -d "$(dirname "${WORKDIR}")" ]]; then
  # Parent exists, we'll create the directory
  WORKDIR=$(cd -- "$(dirname "${WORKDIR}")" && pwd)/$(basename "${WORKDIR}")
else
  echo "ERROR: Parent directory of workdir does not exist: $(dirname "${WORKDIR}")" >&2
  exit 1
fi

# Source common.sh to get environment variables and helper functions
# We need to set WORKDIR, SCRIPT_DIR, and SCRIPT_NAME before sourcing so it uses our values
export WORKDIR
export SCRIPT_DIR
export SCRIPT_NAME
# shellcheck source=backend/raid3/integration/compare_raid3_with_single_common.sh
. "${SCRIPT_DIR}/compare_raid3_with_single_common.sh"

# Create working directory
log_info "setup" "Creating working directory: ${WORKDIR}"
mkdir -p "${WORKDIR}" || die "Failed to create working directory: ${WORKDIR}"

# Create all required subdirectories
log_info "setup" "Creating local storage directories"
mkdir -p "${LOCAL_EVEN_DIR}" "${LOCAL_ODD_DIR}" "${LOCAL_PARITY_DIR}" "${LOCAL_SINGLE_DIR}" || \
  die "Failed to create local storage directories"

log_info "setup" "Creating MinIO data directories"
mkdir -p "${MINIO_EVEN_DIR}" "${MINIO_ODD_DIR}" "${MINIO_PARITY_DIR}" "${MINIO_SINGLE_DIR}" || \
  die "Failed to create MinIO data directories"

# Create the rclone config file
CONFIG_FILE="${WORKDIR}/rclone_raid3_integration_tests.config"
log_info "setup" "Creating rclone configuration file: ${CONFIG_FILE}"

# Check if config file exists and if it contains the mixed remote
# If it doesn't have the mixed remote, we'll regenerate it
NEEDS_REGENERATION=0
if [[ -f "${CONFIG_FILE}" ]]; then
  if ! grep -q "^\[localminioraid3\]" "${CONFIG_FILE}"; then
    log_info "setup" "Config file exists but missing mixed remote - will regenerate"
    NEEDS_REGENERATION=1
  fi
fi

# Use the create_rclone_config function from common.sh
# Call with force=1 if file doesn't exist or needs regeneration, otherwise force=0 for idempotent behavior
if [[ ${NEEDS_REGENERATION} -eq 1 ]]; then
  if create_rclone_config "${CONFIG_FILE}" 1; then
    log_pass "setup" "Config file regenerated successfully: ${CONFIG_FILE}"
  else
    die "Failed to regenerate config file: ${CONFIG_FILE}"
  fi
elif create_rclone_config "${CONFIG_FILE}" 0; then
  log_pass "setup" "Config file created successfully: ${CONFIG_FILE}"
else
  # If it failed because file exists, that's fine (idempotent)
  if [[ -f "${CONFIG_FILE}" ]]; then
    log_warn "setup" "Config file already exists: ${CONFIG_FILE}"
    log_warn "setup" "Skipping config file creation (idempotent behavior)"
  else
    die "Failed to create config file: ${CONFIG_FILE}"
  fi
fi

# Store workdir path in $HOME/.rclone_raid3_integration_tests.workdir
WORKDIR_FILE="${HOME}/.rclone_raid3_integration_tests.workdir"
log_info "setup" "Storing working directory path: ${WORKDIR_FILE}"
echo "${WORKDIR}" > "${WORKDIR_FILE}" || die "Failed to write workdir file: ${WORKDIR_FILE}"

log_pass "setup" "Setup completed successfully!"
log_info "setup" "Working directory: ${WORKDIR}"
log_info "setup" "Config file: ${CONFIG_FILE}"
log_info "setup" "Workdir file: ${WORKDIR_FILE}"
log_info "setup" ""
log_info "setup" "You can now run the integration tests from: ${WORKDIR}"
log_info "setup" "Example: cd ${WORKDIR} && ./backend/raid3/integration/compare_raid3_with_single.sh test mkdir --storage-type=local"
