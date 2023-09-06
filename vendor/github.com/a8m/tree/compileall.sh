#!/bin/bash

go tool dist list >/dev/null || {
    echo 1>&2 "go tool dist list not supported - can't check compile"
    exit 0
}

failures=0
while read -r line; do
    parts=(${line//\// })
    export GOOS=${parts[0]}
    export GOARCH=${parts[1]}
    if go tool compile -V >/dev/null 2>&1 ; then
        echo Try GOOS=${GOOS} GOARCH=${GOARCH}
        if ! go install; then
            echo "*** Failed compiling GOOS=${GOOS} GOARCH=${GOARCH}"
            failures=$((failures+1))
        fi
    else
        echo Skipping GOOS=${GOOS} GOARCH=${GOARCH} as not supported
    fi
done < <(go tool dist list)

if [ $failures -ne 0 ]; then
    echo "*** $failures compile failures"
    exit 1
fi
