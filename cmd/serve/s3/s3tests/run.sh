#!/usr/bin/env bash
# Run the ceph/s3-tests S3 compatibility suite against rclone serve s3.
#
# usage: run.sh [--update] [pytest args...]
#
# With no pytest args, runs the whole of test_s3.py and test_headers.py.
# Pass test ids (e.g. s3tests/functional/test_s3.py::test_bucket_list_empty)
# or pytest options (e.g. -k multipart) to run a subset.
#
# --update rewrites expected-pass.txt with the tests which passed.
#
# Environment variables:
#   RCLONE          rclone binary to test (default: build from this tree)
#   RCLONE_REMOTE   remote to serve (default: a temporary local directory)
#   RCLONE_ARGS     extra flags for rclone serve s3
#   S3TESTS_CACHE   where s3-tests and its venv are kept
#                   (default: ~/.cache/rclone-s3tests)
#   S3TESTS_OUT     where results are written (default: a temporary directory)
#   S3TESTS_TIMEOUT per test timeout in seconds (default: 120)

set -euo pipefail

# ceph/s3-tests was frozen here when the suite moved into ceph/ceph
# (src/test/rgw/s3-tests).
S3TESTS_REPO=https://github.com/ceph/s3-tests
S3TESTS_COMMIT=5b3987116eca0b4b8fa57d6b10f3db57bab196b6

HERE=$(cd "$(dirname "$0")" && pwd)
TOP=$(cd "$HERE/../../../.." && pwd)

UPDATE=""
if [ "${1:-}" = "--update" ]; then
    UPDATE="--update"
    shift
fi
if [ $# -eq 0 ]; then
    set -- s3tests/functional/test_s3.py s3tests/functional/test_headers.py
fi

CACHE=${S3TESTS_CACHE:-$HOME/.cache/rclone-s3tests}
OUT=${S3TESTS_OUT:-$(mktemp -d -t rclone-s3tests.XXXXXX)}
mkdir -p "$CACHE" "$OUT"

# Fetch s3-tests at the pinned commit
SRC=$CACHE/s3-tests-$S3TESTS_COMMIT
if [ ! -d "$SRC" ]; then
    echo "Fetching s3-tests $S3TESTS_COMMIT"
    git init -q "$SRC.tmp"
    git -C "$SRC.tmp" fetch -q --depth 1 "$S3TESTS_REPO" "$S3TESTS_COMMIT"
    git -C "$SRC.tmp" checkout -q FETCH_HEAD
    mv "$SRC.tmp" "$SRC"
fi

# Make a venv for the pinned requirements
VENV=$CACHE/venv-$(sha256sum "$HERE/requirements.txt" | cut -c1-12)
if [ ! -x "$VENV/bin/python" ]; then
    echo "Creating venv $VENV"
    python3 -m venv "$VENV.tmp"
    "$VENV.tmp/bin/pip" install -q -r "$HERE/requirements.txt"
    mv "$VENV.tmp" "$VENV"
fi

if [ -z "${RCLONE:-}" ]; then
    echo "Building rclone"
    RCLONE=$OUT/rclone
    (cd "$TOP" && go build -o "$RCLONE" .)
fi

REMOTE=${RCLONE_REMOTE:-$OUT/data}
if [ -z "${RCLONE_REMOTE:-}" ]; then
    rm -rf "$REMOTE"
    mkdir -p "$REMOTE"
fi

# The keys must match s3tests.conf.in
# shellcheck disable=SC2086
"$RCLONE" --config /notfound serve s3 --addr 127.0.0.1:0 \
    --auth-key '0555b35654ad1656d804,h7GhxuBLTrlhVUyxSPUKUV8r/2EI4ngqJxD7iBdBYLhwluN30JaT3Q==' \
    --auth-key 'NOPQRSTUVWXYZABCDEFG,nopqrstuvwxyzabcdefghijklmnabcdefghijklm' \
    --auth-key 'HIJKLMNOPQRSTUVWXYZA,opqrstuvwxyzabcdefghijklmnopqrstuvwxyzab' \
    ${RCLONE_ARGS:-} "$REMOTE" > "$OUT/rclone.log" 2>&1 &
RCLONE_PID=$!
trap 'kill $RCLONE_PID 2>/dev/null || true' EXIT

PORT=""
for _ in $(seq 100); do
    PORT=$(sed -n 's/.*Starting s3 server on \[http:\/\/127\.0\.0\.1:\([0-9]*\)\/\].*/\1/p' "$OUT/rclone.log")
    [ -n "$PORT" ] && break
    if ! kill -0 $RCLONE_PID 2>/dev/null; then
        cat "$OUT/rclone.log"
        echo "rclone serve s3 failed to start"
        exit 1
    fi
    sleep 0.1
done
if [ -z "$PORT" ]; then
    echo "Timed out waiting for rclone serve s3 to start"
    exit 1
fi
sed "s/@PORT@/$PORT/" "$HERE/s3tests.conf.in" > "$OUT/s3tests.conf"

echo "Running s3-tests against rclone serve s3 on port $PORT - results in $OUT"
cd "$SRC"
set +e
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=$HERE S3TEST_CONF=$OUT/s3tests.conf "$VENV/bin/python" -m pytest \
    -p no:cacheprovider -p rclone_shim \
    --timeout="${S3TESTS_TIMEOUT:-120}" -v -rfE \
    --junitxml="$OUT/junit.xml" "$@" > "$OUT/pytest.log" 2>&1
set -e
tail -1 "$OUT/pytest.log"

"$VENV/bin/python" "$HERE/report.py" $UPDATE --s3tests "$SRC" \
    "$OUT/junit.xml" "$HERE/expected-pass.txt" | tee "$OUT/report.txt"
