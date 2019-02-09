#!/bin/bash
set -e

docker build -f js.cover.dockerfile -t js.cover.djherbis.times .
docker create --name js.cover.djherbis.times js.cover.djherbis.times
docker cp js.cover.djherbis.times:/go/src/github.com/djherbis/times/profile.cov .
docker rm -v js.cover.djherbis.times