#!/bin/bash -eu

go build
go test . "$@"
go tool vet -all -shadow .
