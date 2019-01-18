#!/bin/bash
set -e
docker build -t rclone/xgo-cgofuse https://github.com/billziss-gh/cgofuse.git
docker images
docker push rclone/xgo-cgofuse
