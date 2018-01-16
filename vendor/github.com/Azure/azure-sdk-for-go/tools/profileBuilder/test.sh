#!bin/bash
set -ev
if [[ ! "${TRAVIS_GO_VERSION}" < "1.9" ]]; then
    go test -v github.com/Azure/azure-sdk-for-go/tools/profileBuilder
fi
