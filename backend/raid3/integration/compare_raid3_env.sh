#!/usr/bin/env bash
#
# compare_raid3_env.sh
# ----------------------
# Default environment for raid3 bash-based comparison/recovery/heal scripts.
# You can create compare_raid3_env.local.sh with custom values to override
# defaults; the scripts will automatically source that file if present.
#

# Root directory where test data lives
WORKDIR="${WORKDIR:-${HOME}/go/raid3storage}"

# rclone configuration file
RCLONE_CONFIG="${RCLONE_CONFIG:-${HOME}/.config/rclone/rclone.conf}"

# Local raid3 backend directories
LOCAL_EVEN_DIR="${LOCAL_EVEN_DIR:-${WORKDIR}/even_local}"
LOCAL_ODD_DIR="${LOCAL_ODD_DIR:-${WORKDIR}/odd_local}"
LOCAL_PARITY_DIR="${LOCAL_PARITY_DIR:-${WORKDIR}/parity_local}"
LOCAL_SINGLE_DIR="${LOCAL_SINGLE_DIR:-${WORKDIR}/single_local}"

# Local remote names as defined in rclone.conf
LOCAL_EVEN_REMOTE="${LOCAL_EVEN_REMOTE:-localeven}"
LOCAL_ODD_REMOTE="${LOCAL_ODD_REMOTE:-localodd}"
LOCAL_PARITY_REMOTE="${LOCAL_PARITY_REMOTE:-localparity}"
LOCAL_SINGLE_REMOTE="${LOCAL_SINGLE_REMOTE:-localsingle}"

# Main remote names for comparison scripts
# Default: localraid3 / minioraid3
# Can be overridden via RAID3_REMOTE environment variable if your config uses different names

# MinIO-backed raid3 backend directories
MINIO_EVEN_DIR="${MINIO_EVEN_DIR:-${WORKDIR}/even_minio}"
MINIO_ODD_DIR="${MINIO_ODD_DIR:-${WORKDIR}/odd_minio}"
MINIO_PARITY_DIR="${MINIO_PARITY_DIR:-${WORKDIR}/parity_minio}"
MINIO_SINGLE_DIR="${MINIO_SINGLE_DIR:-${WORKDIR}/single_minio}"

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


