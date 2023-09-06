#!/bin/bash
# Run the swift tests against an openstack server from a swift all in
# one docker image

set -e

NAME=swift-aio
HOST=127.0.0.1
PORT=8294
AUTH=v1

case $AUTH in
    v1)
        export SWIFT_AUTH_URL="http://${HOST}:${PORT}/auth/v1.0"
        export SWIFT_API_USER='test:tester'
        export SWIFT_API_KEY='testing'
        ;;
    v2)
        # NB v2 auth doesn't work for unknown reasons!
        export SWIFT_AUTH_URL="http://${HOST}:${PORT}/auth/v2.0"
        export SWIFT_TENANT='tester'
        export SWIFT_API_USER='test'
        export SWIFT_API_KEY='testing'
        ;;
    *)
        echo "Bad AUTH %AUTH"
        exit 1
        ;;
esac


echo "Starting test server"
docker run --rm -d --name ${NAME} -p ${HOST}:${PORT}:8080 bouncestorage/swift-aio

function cleanup {
    echo "Killing test server"
    docker kill ${NAME}
}

trap cleanup EXIT

echo -n "Waiting for test server to startup"
tries=30
while [[ $tries -gt 0 ]]; do
    echo -n "."
    STATUS_RECEIVED=$(curl -s -o /dev/null -L -w ''%{http_code}'' ${SWIFT_AUTH_URL} || true)
    if [[ ${STATUS_RECEIVED} -ge 200 ]]; then
        break
    fi
    let tries-=1
    sleep 1
done
echo "OK"

echo "Running tests"
go test -v

