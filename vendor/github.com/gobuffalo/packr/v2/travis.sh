#!/bin/sh

go get -t ./...
go install -v ./packr2
packr2 -v clean
packr2 -v
go test -v -timeout=5s -race ./...
