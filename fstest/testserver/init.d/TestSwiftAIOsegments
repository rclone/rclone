#!/bin/bash

set -e

NAME=swift-aio-segments
PORT=28632

. $(dirname "$0")/docker.bash

start() {
    docker run --rm -d --name ${NAME} \
           -p 127.0.0.1:${PORT}:8080 \
           bouncestorage/swift-aio
    
    echo type=swift
    echo env_auth=false
    echo user=test:tester
    echo key=testing
    echo auth=http://127.0.0.1:${PORT}/auth/v1.0
    echo use_segments_container=false
    echo _connect=127.0.0.1:${PORT}
}

. $(dirname "$0")/run.bash
