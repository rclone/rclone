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

linux binary downloaded files install example
-------

    unzip rclone-v1.17-linux-amd64.zip
    cd rclone-v1.17-linux-amd64
    #copy binary file
    sudo cp rclone /usr/sbin/
    sudo chown root:root /usr/sbin/rclone
    sudo chmod 755 /usr/sbin/rclone
    #install manpage
    sudo mkdir -p /usr/local/share/man/man1
    sudo cp rclone.1 /usr/local/share/man/man1/
    sudo mandb 
