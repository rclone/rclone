#!/bin/bash
set -e

script() {
    if [ "${TRAVIS_PULL_REQUEST}" == "false" ];
    then
        COVERALLS_PARALLEL=true

        if [ ! -z "$JS" ];
        then
            bash js.cover.sh
        else
            go test -covermode=count -coverprofile=profile.cov
        fi

        go get github.com/axw/gocov/gocov github.com/mattn/goveralls golang.org/x/tools/cmd/cover
        $HOME/gopath/bin/goveralls --coverprofile=profile.cov -service=travis-ci
    fi

    if [ -z "$JS" ];
    then
        go get golang.org/x/lint/golint && golint ./...
        go vet
        go test -bench=.* -v ./...
    fi
}

"$@"