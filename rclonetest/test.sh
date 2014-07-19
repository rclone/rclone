#!/bin/bash

go install

REMOTES="
memstore:
s3:
drive2:
gcs:
dropbox:
/tmp/z
"

function test_remote {
    args=$@
    rclonetest $args || {
        echo "*** rclonetest $args FAILED ***"
        exit 1
    }
}

for remote in $REMOTES; do
    test_remote $remote
    test_remote --subdir $remote
done
