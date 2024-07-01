#!/usr/bin/env bash

# An example script to run when bisecting go with git bisect -run when
# looking for an rclone regression

# Run this from the go root

set -e

# Compile the go version
cd src
./make.bash || exit 125

# Make sure we are using it
source ~/bin/use-go1.11
go version

# Compile rclone
cd ~/go/src/github.com/rclone/rclone
make

# run the failing test
go run -race race.go
