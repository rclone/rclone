#!/usr/bin/env bash
#
# compare_heal.sh — thin wrapper: run the rs single-object heal integration test.
#
# Same flags as compare.sh, but always runs: test heal (uses `backend heal` for the repair step).
#
# Usage:
#   ./compare_heal.sh [--storage-type=local] [-v]
#

set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)
cd "${SCRIPT_DIR}" || exit 1

exec ./compare.sh "$@" test heal

