#!/bin/bash
set -e
docker build -t rclone/xgo-cgofuse https://github.com/winfsp/cgofuse.git
docker images
docker push rclone/xgo-cgofuse
