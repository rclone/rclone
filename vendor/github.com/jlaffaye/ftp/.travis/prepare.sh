#!/bin/sh -e

case "$1" in
  proftpd)
    mkdir -p /etc/proftpd/conf.d/
    cp $TRAVIS_BUILD_DIR/.travis/proftpd.conf /etc/proftpd/conf.d/
    ;;
  vsftpd)
    cp $TRAVIS_BUILD_DIR/.travis/vsftpd.conf /etc/vsftpd.conf
    ;;
  *)
    echo "unknown software: $1"
    exit 1
esac

mkdir --mode 0777 -p /var/ftp/incoming

apt-get install -qq "$1"
