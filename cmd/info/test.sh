#!/usr/bin/env zsh
#
# example usage: 
# $GOPATH/src/github.com/rclone/rclone/cmd/info/test.sh --list | \
#   parallel -P20 $GOPATH/src/github.com/rclone/rclone/cmd/info/test.sh

export PATH=$GOPATH/src/github.com/rclone/rclone:$PATH

typeset -A allRemotes
 allRemotes=(
  TestAmazonCloudDrive '--low-level-retries=2 --checkers=5'
  TestB2 ''
  TestBox ''
  TestDrive '--tpslimit=5'
  TestCrypt ''
  TestDropbox '--checkers=1'
  TestJottacloud ''
  TestMega ''
  TestOneDrive ''
  TestOpenDrive '--low-level-retries=2 --checkers=5'
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
  dir=$remote:infotest
  if [[ $remote = Local ]]; then
    dir=infotest
  fi
  rclone purge    $dir || :
  rclone info -vv $dir ${=allRemotes[$remote]} &> info-$remote.log
  rclone ls   -vv $dir &> info-$remote.list
done
