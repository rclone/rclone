#!/usr/bin/env bash
#
# Test all the remotes against restic integration test
# Run with: screen -S restic-test -L ./restic-test.sh

remotes="
TestAzureBlob:
TestB2:
TestBox:
TestCache:
TestCryptDrive:
TestCryptSwift:
TestDrive:
TestDropbox:
TestFichier:
TestFTP:
TestGoogleCloudStorage:
TestNetStorage:
TestOneDrive:
TestPcloud:
TestQingStor:
TestS3:
TestSftp:
TestSwift:
TestWebdav:
TestYandex:
"

# TestOss:
# TestMega:

for remote in $remotes; do
    echo `date -Is` $remote starting
    go test -remote $remote -v -timeout 30m 2>&1 | tee restic-test.$remote.log
    echo `date -Is` $remote ending
done
