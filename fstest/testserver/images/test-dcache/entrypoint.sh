#!/usr/bin/env bash
set -euo pipefail

target=/opt/dcache-source/packages/system-test/target
trap '"$target/bin/ctlcluster" stop' EXIT
trap 'exit 0' TERM INT

"$target/bin/ctlcluster" start
tail -n 40 -F "$target/dcache/var/log/dCacheDomain.log" &
while "$target/dcache/bin/dcache" status dCacheDomain >/dev/null; do
    sleep 2 &
    wait $!
done
exit 1
