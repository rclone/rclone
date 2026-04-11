#!/bin/bash
# Fetch the latest GUI dist from rclone/rclone-web GitHub releases.
#
# Downloads dist.zip from the latest release and extracts it to
# cmd/gui/dist/. Skips the download if the local tag matches.
#
# Requires: curl, unzip

set -euo pipefail

REPO="rclone/rclone-web"
DEST="cmd/gui/dist"
TAG_FILE="${DEST}/.tag"

CURL_OPTS=(-fSs --retry 5 --retry-delay 2 --retry-all-errors)

# Use GITHUB_TOKEN (or GH_TOKEN) if present so that GitHub API calls are
# authenticated. Unauthenticated calls are limited to 60/hour per source IP,
# which is regularly exhausted on shared GitHub Actions runners.
AUTH_HEADER=()
TOKEN="${GITHUB_TOKEN:-${GH_TOKEN:-}}"
if [ -n "${TOKEN}" ]; then
    AUTH_HEADER=(-H "Authorization: Bearer ${TOKEN}")
fi

# Get the latest release info
echo "Checking latest release of ${REPO}..."
RELEASE_JSON=$(curl "${CURL_OPTS[@]}" "${AUTH_HEADER[@]}" \
    -H "Accept: application/vnd.github+json" \
    "https://api.github.com/repos/${REPO}/releases/latest") || {
    echo "Error: failed to fetch release info from GitHub API" >&2
    exit 1
}

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
curl -L "${CURL_OPTS[@]}" "${AUTH_HEADER[@]}" -o "${TMPFILE}" "${ASSET_URL}" || {
    echo "Error: failed to download dist.zip" >&2
    exit 1
}

# Extract
echo "Extracting to ${DEST}/..."
rm -rf "${DEST}"
mkdir -p "${DEST}"
unzip -q "${TMPFILE}" -d "${DEST}"

# Write tag for cache comparison
echo -n "${TAG}" > "${TAG_FILE}"

echo "Done. GUI dist updated to ${TAG}"
