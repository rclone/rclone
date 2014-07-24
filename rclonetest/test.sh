#!/bin/bash

go install

REMOTES="
TestSwift:
TestS3:
TestDrive:
TestGoogleCloudStorage:
TestDropbox:
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
