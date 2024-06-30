#!/usr/bin/env bash

set -e

# Extract just the America/New_York timezone from
tzinfo=$(go env GOROOT)/lib/time/zoneinfo.zip

rm -rf tzdata
mkdir tzdata
cd tzdata
unzip ${tzinfo} America/New_York

cd ..
# Make the embedded assets
go run generate_tzdata.go

# tidy up
rm -rf tzdata
