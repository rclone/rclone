#!/usr/bin/env bash
#
# compare_raid3_env.sh
# ----------------------
# Default environment for raid3 bash-based comparison/recovery/heal scripts.
# You can create compare_raid3_env.local.sh with custom values to override
# defaults; the scripts will automatically source that file if present.
#

# Script directory (set by scripts that source this file)
# Data directory is relative to script directory
SCRIPT_DIR="${SCRIPT_DIR:-$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
DATA_DIR="${DATA_DIR:-${SCRIPT_DIR}/_data}"

# rclone configuration file
RCLONE_CONFIG="${RCLONE_CONFIG:-${HOME}/.config/rclone/rclone.conf}"

# Local raid3 backend directories (relative to test/_data)
LOCAL_EVEN_DIR="${LOCAL_EVEN_DIR:-${DATA_DIR}/even_local}"
LOCAL_ODD_DIR="${LOCAL_ODD_DIR:-${DATA_DIR}/odd_local}"
LOCAL_PARITY_DIR="${LOCAL_PARITY_DIR:-${DATA_DIR}/parity_local}"
LOCAL_SINGLE_DIR="${LOCAL_SINGLE_DIR:-${DATA_DIR}/single_local}"

# Local remote names as defined in rclone.conf
LOCAL_EVEN_REMOTE="${LOCAL_EVEN_REMOTE:-localeven}"
LOCAL_ODD_REMOTE="${LOCAL_ODD_REMOTE:-localodd}"
LOCAL_PARITY_REMOTE="${LOCAL_PARITY_REMOTE:-localparity}"
LOCAL_SINGLE_REMOTE="${LOCAL_SINGLE_REMOTE:-localsingle}"

# Main remote names for comparison scripts
# Default: localraid3 / minioraid3
# Can be overridden via RAID3_REMOTE environment variable if your config uses different names

# MinIO-backed raid3 backend directories (relative to test/_data)
MINIO_EVEN_DIR="${MINIO_EVEN_DIR:-${DATA_DIR}/even_minio}"
MINIO_ODD_DIR="${MINIO_ODD_DIR:-${DATA_DIR}/odd_minio}"
MINIO_PARITY_DIR="${MINIO_PARITY_DIR:-${DATA_DIR}/parity_minio}"
MINIO_SINGLE_DIR="${MINIO_SINGLE_DIR:-${DATA_DIR}/single_minio}"

# MinIO remote names as defined in rclone.conf
MINIO_EVEN_REMOTE="${MINIO_EVEN_REMOTE:-minioeven}"
MINIO_ODD_REMOTE="${MINIO_ODD_REMOTE:-minioodd}"
MINIO_PARITY_REMOTE="${MINIO_PARITY_REMOTE:-minioparity}"
MINIO_SINGLE_REMOTE="${MINIO_SINGLE_REMOTE:-miniosingle}"

# MinIO container names and S3 ports
MINIO_EVEN_NAME="${MINIO_EVEN_NAME:-minioeven}"
MINIO_ODD_NAME="${MINIO_ODD_NAME:-minioodd}"
MINIO_PARITY_NAME="${MINIO_PARITY_NAME:-minioparity}"
MINIO_SINGLE_NAME="${MINIO_SINGLE_NAME:-miniosingle}"

MINIO_EVEN_PORT="${MINIO_EVEN_PORT:-9001}"
MINIO_ODD_PORT="${MINIO_ODD_PORT:-9002}"
MINIO_PARITY_PORT="${MINIO_PARITY_PORT:-9003}"
MINIO_SINGLE_PORT="${MINIO_SINGLE_PORT:-9004}"

# MinIO Docker image (Docker Hub). Use RELEASE.2025-09-07 or newer for multipart bugfixes
# (fixes for AbortMultipartUpload idempotency, conditional checks write for multipart).
# RELEASE.2025-10-15 not yet on Docker Hub; use MINIO_IMAGE=... to override if needed.
MINIO_IMAGE="${MINIO_IMAGE:-minio/minio:RELEASE.2025-09-07T16-13-09Z}"

# SFTP-backed raid3 backend directories (relative to test/_data), using atmoz/sftp
SFTP_EVEN_DIR="${SFTP_EVEN_DIR:-${DATA_DIR}/even_sftp}"
SFTP_ODD_DIR="${SFTP_ODD_DIR:-${DATA_DIR}/odd_sftp}"
SFTP_PARITY_DIR="${SFTP_PARITY_DIR:-${DATA_DIR}/parity_sftp}"
SFTP_SINGLE_DIR="${SFTP_SINGLE_DIR:-${DATA_DIR}/single_sftp}"

# SFTP remote names as defined in rclone.conf
SFTP_EVEN_REMOTE="${SFTP_EVEN_REMOTE:-sftpeven}"
SFTP_ODD_REMOTE="${SFTP_ODD_REMOTE:-sftpodd}"
SFTP_PARITY_REMOTE="${SFTP_PARITY_REMOTE:-sftpparity}"
SFTP_SINGLE_REMOTE="${SFTP_SINGLE_REMOTE:-sftpsingle}"

# SFTP container names and SSH/SFTP host ports (container listens on 22)
SFTP_EVEN_NAME="${SFTP_EVEN_NAME:-sftpeven}"
SFTP_ODD_NAME="${SFTP_ODD_NAME:-sftpodd}"
SFTP_PARITY_NAME="${SFTP_PARITY_NAME:-sftpparity}"
SFTP_SINGLE_NAME="${SFTP_SINGLE_NAME:-sftpsingle}"

SFTP_EVEN_PORT="${SFTP_EVEN_PORT:-2221}"
SFTP_ODD_PORT="${SFTP_ODD_PORT:-2222}"
SFTP_PARITY_PORT="${SFTP_PARITY_PORT:-2223}"
SFTP_SINGLE_PORT="${SFTP_SINGLE_PORT:-2224}"

# SFTP remote root path. Containers mount host dirs at /home/<user>/data; we use a "base"
# subdir on the host so rclone paths are data/base/<id>. setup.sh creates "base" under each.
# Override via SFTP_REMOTE_ROOT in compare_raid3_env.local.sh if needed.
SFTP_REMOTE_ROOT="${SFTP_REMOTE_ROOT:-data/base}"

# Legacy: SFTP_BASE_PATH (e.g. /pub) was used before we switched to data/base mount.
SFTP_BASE_PATH="${SFTP_BASE_PATH:-/pub}"

# SFTP Docker image (atmoz/sftp).
# On ARM64 (e.g. Apple Silicon) you may see "platform (linux/amd64) does not match (linux/arm64)";
# that is expected—Docker runs the image via emulation and it works for these tests.
# To use a multi-arch image instead, set SFTP_IMAGE in compare_raid3_env.local.sh (e.g. a fork with arm64).
SFTP_IMAGE="${SFTP_IMAGE:-atmoz/sftp}"

