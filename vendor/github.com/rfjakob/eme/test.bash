#!/bin/bash -eu

go build
go test -v "$@"
go vet -all .
