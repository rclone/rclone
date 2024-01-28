#!/usr/bin/env bash
#
# End-to-end tests for "rclone gitannex". This script runs the `git-annex
# testremote` suite against "rclone gitannex" in an ephemeral git-annex repo.
#
# Assumptions:
#
#   * This system has an rclone remote configured named "git-annex-builtin-test-remote".
#
#   * If it uses rclone's "local" backend, /tmp/git-annex-builtin-test-remote exists.

set -e

TEST_DIR="$(realpath "$(mktemp -d)")"
mkdir "$TEST_DIR/bin"

function cleanup()
{
    rm -rf "$TEST_DIR"
}

trap cleanup EXIT

RCLONE_DIR="$(git rev-parse --show-toplevel)"

rm -rf /tmp/git-annex-builtin-test-remote/*

set -x

pushd "$RCLONE_DIR"
go build -o "$TEST_DIR/bin" ./

ln -s "$(realpath "$TEST_DIR/bin/rclone")" "$TEST_DIR/bin/git-annex-remote-rclone-builtin"
popd

pushd "$TEST_DIR"

git init
git annex init

REMOTE_NAME=git-annex-builtin-test-remote
PREFIX=/tmp/git-annex-builtin-test-remote

PATH="$PATH:$TEST_DIR/bin" git annex initremote $REMOTE_NAME \
    type=external externaltype=rclone-builtin encryption=none \
    rcloneremotename=$REMOTE_NAME \
    rcloneprefix="$PREFIX"

PATH="$PATH:$(realpath bin)" git annex testremote $REMOTE_NAME

popd
rm -rf "$TEST_DIR"
