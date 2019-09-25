# Copyright 2016 Canonical Ltd.
# Licensed under the LGPLv3, see LICENCE file for details.

default: check

check:
	go test

docs:
	godoc2md github.com/juju/ansiterm > README.md
	sed -i 's|\[godoc-link-here\]|[![GoDoc](https://godoc.org/github.com/juju/ansiterm?status.svg)](https://godoc.org/github.com/juju/ansiterm)|' README.md 


.PHONY: default check docs
