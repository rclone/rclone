#!/bin/bash
# Fetch the latest GUI dist from rclone/rclone-web GitHub releases.
#
# Downloads dist.zip from the latest release and extracts it to
# cmd/gui/dist/. Skips the download if the local tag matches.
#
# Requires: curl, unzip
# Optional: GITHUB_TOKEN (avoids API rate limits)

set -euo pipefail

REPO="rclone/rclone-web"
DEST="cmd/gui/dist"
TAG_FILE="${DEST}/.tag"

CURL_OPTS=(-sf)
if [ -n "${GITHUB_TOKEN:-}" ]; then
    CURL_OPTS+=(-H "Authorization: token ${GITHUB_TOKEN}")
fi

SLEEP_TIME=$(( 1 + RANDOM % 3 ))

# Get the latest release info
echo "Checking latest release of ${REPO}..."
for attempt in 1 2 3; do
    RELEASE_JSON=$(curl "${CURL_OPTS[@]}" \
        "https://api.github.com/repos/${REPO}/releases/latest") && break
    if [ "$attempt" -eq 3 ]; then
        echo "Error: failed to fetch release info from GitHub API" >&2
        exit 1
    fi
    echo "Warning: failed to fetch release info. Retrying in ${SLEEP_TIME}s... ($attempt/2)" >&2
    sleep "${SLEEP_TIME}"
done

TAG=$(echo "${RELEASE_JSON}" | python3 -c "import sys,json; print(json.load(sys.stdin)['tag_name'])")
ASSET_URL=$(echo "${RELEASE_JSON}" | python3 -c "
import sys, json
r = json.load(sys.stdin)
for a in r['assets']:
    if a['name'] == 'dist.zip':
        print(a['browser_download_url'])
        sys.exit(0)
print('', file=sys.stderr)
sys.exit(1)
") || {
    echo "Error: dist.zip asset not found in release ${TAG}" >&2
    exit 1
}

echo "Latest release: ${TAG}"

# Check if we already have this version
if [ -f "${TAG_FILE}" ] && [ "$(cat "${TAG_FILE}")" = "${TAG}" ]; then
    echo "Already up to date (${TAG})"
    exit 0
fi

# Download dist.zip
TMPFILE=$(mktemp /tmp/rclone-gui-dist.XXXXXX.zip)
trap 'rm -f "${TMPFILE}"' EXIT

echo "Downloading dist.zip from ${TAG}..."
for attempt in 1 2 3; do
    curl -sfL "${CURL_OPTS[@]}" -o "${TMPFILE}" "${ASSET_URL}" && break
    if [ "$attempt" -eq 3 ]; then
        echo "Error: failed to download dist.zip" >&2
        exit 1
    fi
    echo "Warning: failed to download dist.zip. Retrying in ${SLEEP_TIME}s... ($attempt/2)" >&2
    sleep "${SLEEP_TIME}"
done

# Extract
echo "Extracting to ${DEST}/..."
rm -rf "${DEST}"
mkdir -p "${DEST}"
unzip -q "${TMPFILE}" -d "${DEST}"

# Write tag for cache comparison
echo -n "${TAG}" > "${TAG_FILE}"

echo "Done. GUI dist updated to ${TAG}"
