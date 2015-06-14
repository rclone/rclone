---
title: "Install"
description: "Rclone Installation"
date: "2015-06-12"
---

Install
-------

Rclone is a Go program and comes as a single binary file.

[Download](/downloads/) the relevant binary.

Or alternatively if you have Go installed use

    go get github.com/ncw/rclone

and this will build the binary in `$GOPATH/bin`.  If you have built
rclone before then you will want to update its dependencies first with
this (remove `-f` if using go < 1.4)

    go get -u -v -f github.com/ncw/rclone/...

See the [Usage section](/docs/) of the docs for how to use rclone, or
run `rclone -h`.
