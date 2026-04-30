#!/bin/bash
# Fetch the latest GUI dist.zip from rclone/rclone-web GitHub releases.
#
# Downloads dist.zip from the latest release to cmd/gui/dist.zip and
# records the tag in cmd/gui/dist.tag. Both files are committed to the
# repo so that builds are reproducible and `go build` works on a fresh
# clone without needing to fetch anything.
#
# Skips the download when both dist.zip and dist.tag exist and the tag
# matches the latest release.
#
# Requires: curl

set -euo pipefail

REPO="rclone/rclone-web"
DEST_DIR="cmd/gui"
ZIP_FILE="${DEST_DIR}/dist.zip"
TAG_FILE="${DEST_DIR}/dist.tag"

COMMIT=0
for arg in "$@"; do
    case "$arg" in
        --commit) COMMIT=1 ;;
        *) echo "Unknown argument: $arg" >&2; exit 1 ;;
    esac
done

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

# Skip only when both the zip and the tag are present and the tag matches.
# If only the tag exists (e.g. someone deleted the zip), force a re-download.
if [ -f "${ZIP_FILE}" ] && [ -f "${TAG_FILE}" ] && [ "$(cat "${TAG_FILE}")" = "${TAG}" ]; then
    echo "Already up to date (${TAG})"
    exit 0
fi

# Download dist.zip directly to its final location, via a temp file so a
# failed download doesn't leave a partial file behind.
TMPFILE=$(mktemp "${DEST_DIR}/.dist.zip.XXXXXX")
trap 'rm -f "${TMPFILE}"' EXIT

echo "Downloading dist.zip from ${TAG}..."
curl -L "${CURL_OPTS[@]}" "${AUTH_HEADER[@]}" -o "${TMPFILE}" "${ASSET_URL}" || {
    echo "Error: failed to download dist.zip" >&2
    exit 1
}

mv "${TMPFILE}" "${ZIP_FILE}"
chmod 644 "${ZIP_FILE}"
echo -n "${TAG}" > "${TAG_FILE}"

echo "Done. ${ZIP_FILE} updated to ${TAG}"

if [ "${COMMIT}" -eq 1 ]; then
    git add "${ZIP_FILE}" "${TAG_FILE}"
    if git diff --cached --quiet -- "${ZIP_FILE}" "${TAG_FILE}"; then
        echo "No changes to commit (zip and tag are identical to HEAD)"
    else
        git commit -m "gui: update embedded release to ${TAG}" -- "${ZIP_FILE}" "${TAG_FILE}"
    fi
fi
