This directory contains scripts to start and stop servers for testing.

The commands are named after the remotes in use.  They should be
executable files with the following parameters:

    start  - starts the server
    stop   - stops the server
    status - returns non-zero exit code if the server is not running

These will be called automatically by test_all if that remote is
required.

When start is run it should output config parameters for that remote.
If a `_connect` parameter is output then that will be used for a
connection test.  For example if `_connect=127.0.0.1:80` then a TCP
connection will be made to `127.0.0.1:80` and only when that succeeds
will the test continue.

`run.bash` contains boilerplate to be included in a bash script for
interpreting the command line parameters.

`docker.bash` contains library functions to help with docker
implementations.

## TODO

- sftpd - https://github.com/panubo/docker-sshd ?
- openstack swift - https://github.com/bouncestorage/docker-swift
- ceph - https://github.com/ceph/cn
- other ftp servers

