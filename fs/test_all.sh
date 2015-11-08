#!/bin/bash

go install

REMOTES="
TestSwift:
TestS3:
TestDrive:
TestGoogleCloudStorage:
TestDropbox:
TestAmazonCloudDrive:
TestOneDrive:
TestHubic:
"

function test_remote {
    args=$@
    echo "@go test $args"
    go test $args || {
        echo "*** test $args FAILED ***"
        exit 1
    }
}

test_remote
test_remote --subdir
for remote in $REMOTES; do
    test_remote --remote $remote
    test_remote --remote $remote --subdir
done

echo "All OK"
