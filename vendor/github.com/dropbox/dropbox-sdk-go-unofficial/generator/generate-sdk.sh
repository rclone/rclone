#! /usr/bin/env bash
set -euo pipefail

if [[ $# -gt 1 ]]; then
    echo "$0: Not expecting any command-line arguments, got $#." 1>&2
    exit 1
fi

loc=$(realpath -e $0)
base_dir=$(dirname "$loc")
spec_dir="$base_dir/dropbox-api-spec"
gen_dir=$(dirname ${base_dir})/dropbox

stone -v -a :all go_types.stoneg.py "$gen_dir" "$spec_dir"/*.stone
stone -v -a :all go_client.stoneg.py "$gen_dir" "$spec_dir"/*.stone

# Update SDK and API spec versions
sdk_version=${1:-"4.2.0"}
pushd ${spec_dir}
spec_version=$(git rev-parse --short HEAD)
popd

sed -i.bak -e "s/UNKNOWN SDK VERSION/${sdk_version}/" \
    -e "s/UNKNOWN SPEC VERSION/${spec_version}/" ${gen_dir}/sdk.go
rm ${gen_dir}/sdk.go.bak
goimports -l -w ${gen_dir}
