#!/usr/bin/env bash
# Thrash the VFS tests

set -e

# Optionally set the iterations with the first parameter
iterations=${1:-100}

base=$(dirname $(dirname $(realpath "$0")))
echo ${base}
run=${base}/bin/test-repeat.sh
echo ${run}

testdirs="
vfs
vfs/vfscache
vfs/vfscache/writeback
vfs/vfscache/downloaders
cmd/cmount
"

for testdir in ${testdirs}; do
    echo "Testing ${testdir} with ${iterations} iterations"
    cd ${base}/${testdir}
    ${run} -i=${iterations} -race -tags=cmount
done
