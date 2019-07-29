---
title: "Install"
description: "Rclone Installation"
date: "2018-08-28"
---

# Install #

Rclone is a Go program and comes as a single binary file.

## Quickstart ##

  * [Download](/downloads/) the relevant binary.
  * Extract the `rclone` or `rclone.exe` binary from the archive
  * Run `rclone config` to setup. See [rclone config docs](/docs/) for more details.

See below for some expanded Linux / macOS instructions.

See the [Usage section](/docs/) of the docs for how to use rclone, or
run `rclone -h`.

## Script installation ##

To install rclone on Linux/macOS/BSD systems, run:

    curl https://rclone.org/install.sh | sudo bash

For beta installation, run:

    curl https://rclone.org/install.sh | sudo bash -s beta

Note that this script checks the version of rclone installed first and
won't re-download if not needed.

## Linux installation from precompiled binary ##

Fetch and unpack

    curl -O https://downloads.rclone.org/rclone-current-linux-amd64.zip
    unzip rclone-current-linux-amd64.zip
    cd rclone-*-linux-amd64

Copy binary file

    sudo cp rclone /usr/bin/
    sudo chown root:root /usr/bin/rclone
    sudo chmod 755 /usr/bin/rclone
    
Install manpage

    sudo mkdir -p /usr/local/share/man/man1
    sudo cp rclone.1 /usr/local/share/man/man1/
    sudo mandb 

Run `rclone config` to setup. See [rclone config docs](/docs/) for more details.

    rclone config

## macOS installation from precompiled binary ##

Download the latest version of rclone.

    cd && curl -O https://downloads.rclone.org/rclone-current-osx-amd64.zip

Unzip the download and cd to the extracted folder.

    unzip -a rclone-current-osx-amd64.zip && cd rclone-*-osx-amd64

Move rclone to your $PATH. You will be prompted for your password.

    sudo mkdir -p /usr/local/bin
    sudo mv rclone /usr/local/bin/

(the `mkdir` command is safe to run, even if the directory already exists).

Remove the leftover files.

    cd .. && rm -rf rclone-*-osx-amd64 rclone-current-osx-amd64.zip

Run `rclone config` to setup. See [rclone config docs](/docs/) for more details.

    rclone config

## Install from source ##

Make sure you have at least [Go](https://golang.org/) 1.7
installed.  [Download go](https://golang.org/dl/) if necessary.  The
latest release is recommended. Then

    git clone https://github.com/rclone/rclone.git
    cd rclone
    go build
    ./rclone version

You can also build and install rclone in the
[GOPATH](https://github.com/golang/go/wiki/GOPATH) (which defaults to
`~/go`) with:

    go get -u -v github.com/rclone/rclone

and this will build the binary in `$GOPATH/bin` (`~/go/bin/rclone` by
default) after downloading the source to
`$GOPATH/src/github.com/rclone/rclone` (`~/go/src/github.com/rclone/rclone`
by default).

## Installation with Ansible ##

This can be done with [Stefan Weichinger's ansible
role](https://github.com/stefangweichinger/ansible-rclone).

Instructions

  1. `git clone https://github.com/stefangweichinger/ansible-rclone.git` into your local roles-directory
  2. add the role to the hosts you want rclone installed to:
    
```
    - hosts: rclone-hosts
      roles:
          - rclone
```
