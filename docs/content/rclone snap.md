---
title: "Install"
description: "Rclone Snap Installation"
date: "2016-03-28"
---

# Install #

Rclone is a Go program and comes as a single binary file.

## Quickstart ##

  * sudo snap install rclone --classic 
  * install Snapd on your distro using the instructions below
  * Run `rclone config` to setup. See [rclone config docs](http://rclone.org/docs/) for more details.

See below for some expanded Linux / macOS instructions.

See the [Usage section](/docs/) of the docs for how to use rclone, or
run `rclone -h`.

## Linux installation from Ubuntu Snap binary ##

Installing Snapd if its not installed, since this is needed in order to install snaps

Arch

sudo pacman -S snapd

# enable the snapd systemd service:
sudo systemctl enable --now snapd.socket
Debian

# On Sid:
sudo apt install snapd
Fedora

sudo dnf copr enable zyga/snapcore
sudo dnf install snapd

# enable the snapd systemd service:
sudo systemctl enable --now snapd.service

# SELinux support is in beta, so currently:
sudo setenforce 0

# to persist, edit /etc/selinux/config
to set SELINUX=permissive and reboot.
Gentoo

Install the gentoo-snappy overlay. https://github.com/zyga/gentoo-snappy

OpenEmbedded/Yocto

Install the snap meta layer. https://github.com/morphis/meta-snappy/blob/master/README.md

openSUSE

sudo zypper addrepo http://download.opensuse.org/repositories/system:/snappy/openSUSE_Leap_42.2/ snappy
sudo zypper install snapd
OpenWrt

Enable the snap-openwrt feed.

Ubuntu

sudo apt install snapd
 

Run `rclone config` to setup. See [rclone config docs](http://rclone.org/docs/) for more details.

    rclone config


