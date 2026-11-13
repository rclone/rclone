#!/usr/bin/env bash
set -euo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/../../../.." && pwd)
cd "$ROOT"
for option in "${!RCLONE_@}"; do
    if [[ $option != RCLONE_TESTSERVER_DCACHE_IMAGE ]]; then
        unset "$option"
    fi
done
umask 077
WORK=$(mktemp -d "${TMPDIR:-/tmp}/rclone-dcache.XXXXXXXX")
export RCLONE_CONFIG=$WORK/rclone.conf
export STATE_DIR=$WORK/state
export RCLONE_TESTSERVER_DCACHE_NAME=rclone-test-webdav-dcache-${WORK##*.}
SERVER=$ROOT/fstest/testserver/init.d/TestWebdavDcache
echo "dCache fixture: $RCLONE_TESTSERVER_DCACHE_NAME" >&2

cleanup() {
    local rc=$?
    trap - EXIT
    if "$SERVER" reset; then
        rm -rf "$WORK"
    else
        echo "dCache cleanup failed; preserving configuration and state in $WORK" >&2
        printf 'Retry cleanup: STATE_DIR=%q RCLONE_TESTSERVER_DCACHE_NAME=%q %q reset\n' \
            "$STATE_DIR" "$RCLONE_TESTSERVER_DCACHE_NAME" "$SERVER" >&2
        if (( rc == 0 )); then rc=1; fi
    fi
    exit "$rc"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

printf '[TestWebdavDcache]\ntype = webdav\n' > "$RCLONE_CONFIG"
go build -o "$WORK/rclone" .
export PATH=$WORK:$PATH
"$SERVER" start > "$WORK/server.env"
address=$(python3 - "$WORK/server.env" "$RCLONE_CONFIG" <<'PY'
import configparser
import pathlib
import sys
import urllib.parse

fields = {}
for line in pathlib.Path(sys.argv[1]).read_text().splitlines():
    key, separator, value = line.partition("=")
    if separator and key in {"type", "vendor", "url", "user", "pass"}:
        fields[key] = value
if len(fields) != 5:
    raise RuntimeError("incomplete dCache fixture configuration")
config = configparser.ConfigParser(interpolation=None)
config["TestWebdavDcache"] = fields
with open(sys.argv[2], "w") as output:
    config.write(output)
print(urllib.parse.urlsplit(fields["url"]).hostname)
PY
)
export NO_PROXY="${NO_PROXY:-${no_proxy:-}}"
export NO_PROXY="${NO_PROXY:+$NO_PROXY,}$address"
export no_proxy=$NO_PROXY

go run ./fstest/test_all -remotes TestWebdavDcache: -backends webdav -n 2 -maxtries 1 "$@"
