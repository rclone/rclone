#!/bin/bash

set -eu

go build
go test -bench=.
