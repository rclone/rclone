#!/usr/bin/env zsh
#
# example usage: 
# $GOPATH/src/github.com/rclone/rclone/cmd/info/test.sh --list | \
#   parallel -P20 $GOPATH/src/github.com/rclone/rclone/cmd/info/test.sh

export PATH=$GOPATH/src/github.com/rclone/rclone:$PATH

typeset -A allRemotes
allRemotes=(
  TestB2 ''
  TestBox ''
  TestDrive '--tpslimit=5'
  TestCrypt ''
  TestDropbox '--checkers=1'
  TestGCS ''
  TestJottacloud ''
  TestKoofr ''
  TestMega ''
  TestOneDrive ''
  TestOpenDrive '--low-level-retries=4 --checkers=5'
  TestPcloud '--low-level-retries=2 --timeout=15s'
  TestS3 ''
  Local ''
)

set -euo pipefail

if [[ $# -eq 0 ]]; then
  set -- ${(k)allRemotes[@]}
elif [[ $1 = --list ]]; then
  printf '%s\n' ${(k)allRemotes[@]}
  exit 0
fi

for remote; do
  case $remote in
    Local)
      l=Local$(uname)
      export RCLONE_CONFIG_${l:u}_TYPE=local
      dir=$l:infotest;;
    TestGCS)
      dir=$remote:$GCS_BUCKET/infotest;;
    *)
      dir=$remote:infotest;;
  esac

  rclone purge    $dir || :
  rclone info -vv $dir --write-json=info-$remote.json ${=allRemotes[$remote]:-} &> info-$remote.log
  rclone ls   -vv $dir &> info-$remote.list
done
