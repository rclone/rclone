---
title: "Install"
description: "Rclone Installation"
date: "2016-03-28"
---

Install
-------

Rclone is a Go program and comes as a single binary file.

[Download](/downloads/) the relevant binary.

Or alternatively if you have Go 1.5+ installed use

    go get github.com/ncw/rclone

and this will build the binary in `$GOPATH/bin`.  If you have built
rclone before then you will want to update its dependencies first with
this

    go get -u -v github.com/ncw/rclone/...

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

installation per ansible-role "ansible-rclone"
-------

    Usage:

    1. clone this repo into your local roles-directory
    2. add role to the hosts you want rclone installed to:
    
```
    - hosts: rclone-hosts
      roles:
          - rclone
```
