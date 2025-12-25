#!/bin/bash
# Convenience script to run fs/sync tests against RAID3 backend
# Automatically finds the test config file and navigates to rclone root
#
# Usage:
#   ./test_fs_sync.sh [remote] [additional-go-test-flags...]
#
# Examples:
#   ./test_fs_sync.sh localraid3: -v
#   ./test_fs_sync.sh localraid3: -v -run TestCopy
#   ./test_fs_sync.sh  # Uses default remote (localraid3:)
#
# Environment variables:
#   RCLONE_CONFIG - Path to rclone config file (optional, auto-detected if not set)

set -e

# Get remote name (default to localraid3:)
REMOTE="${1:-localraid3:}"
shift || true

# Get rclone config from environment or use default
if [ -z "$RCLONE_CONFIG" ]; then
    WORKDIR_FILE="${HOME}/.rclone_raid3_integration_tests.workdir"
    if [ -f "$WORKDIR_FILE" ]; then
        WORKDIR=$(cat "$WORKDIR_FILE")
        RCLONE_CONFIG="${WORKDIR}/rclone_raid3_integration_tests.config"
    else
        echo "Error: RCLONE_CONFIG not set and workdir file not found at $WORKDIR_FILE"
        echo "Please set RCLONE_CONFIG or run backend/raid3/integration/setup.sh"
        exit 1
    fi
fi

if [ ! -f "$RCLONE_CONFIG" ]; then
    echo "Error: Config file not found: $RCLONE_CONFIG"
    exit 1
fi

# Export RCLONE_CONFIG and run the tests
export RCLONE_CONFIG

echo "Running fs/sync tests against remote: $REMOTE"
echo "Using config: $RCLONE_CONFIG"
echo ""

# Change to rclone root directory (assuming script is in backend/raid3/)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RCLONE_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$RCLONE_ROOT"

# Run the tests with provided flags
go test ./fs/sync -remote "$REMOTE" "$@"

