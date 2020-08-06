#!/bin/bash
# Thrash the VFS tests

set -e

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

testdirs="
cmd/cmount
"

for testdir in ${testdirs}; do
    echo "Testing ${testdir}"
    cd ${base}/${testdir}
    ${run} -c=100 -race -tags=cmount
done
