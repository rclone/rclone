#!/bin/bash

go tool dist list >/dev/null || {
    echo 1>&2 "go tool dist list not supported - can't check compile"
    exit 0
}

while read -r line; do
    parts=(${line//\// })
    export GOOS=${parts[0]}
    export GOARCH=${parts[1]}
    echo Try GOOS=${GOOS} GOARCH=${GOARCH}
    go install
done < <(go tool dist list)
