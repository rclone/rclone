#!/bin/bash
set -e

if [ "${TRAVIS_PULL_REQUEST}" = "true" ]; then
    exit 0
fi

if [ "${TEST_REAL_SERVER}" = "rackspace" ] && [ ! -z "${RACKSPACE_APIKEY}" ]; then
    echo "Running tests pointing to Rackspace"
    export SWIFT_API_KEY=$RACKSPACE_APIKEY
    export SWIFT_API_USER=$RACKSPACE_USER
    export SWIFT_AUTH_URL=$RACKSPACE_AUTH
    go test ./...
fi

if [ "${TEST_REAL_SERVER}" = "memset" ] && [ ! -z "${MEMSET_APIKEY}" ]; then
    echo "Running tests pointing to Memset"
    export SWIFT_API_KEY=$MEMSET_APIKEY
    export SWIFT_API_USER=$MEMSET_USER
    export SWIFT_AUTH_URL=$MEMSET_AUTH
    go test
fi
