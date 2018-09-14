#!/bin/bash

# Example script for git-bisect -run
# Run from the project root

set -e

# Compile
make
rclone version

# Test whatever it is that is going wrong
truncate -s 10M /tmp/10M
rclone delete azure:rclone-test1/10M || true
rclone --retries 1 copyto -vv /tmp/10M azure:rclone-test1/10M --azureblob-upload-cutoff 1M
