---
title: "Swift"
description: "Swift"
date: "2014-04-26"
---

Swift refers to [Openstack Object Storage](http://www.openstack.org/software/openstack-storage/).
Commercial implementations of that being:

  * [Rackspace Cloud Files](http://www.rackspace.com/cloud/files/)
  * [Memset Memstore](http://www.memset.com/cloud/storage/)

Paths are specified as `remote:container` or `remote:`

Here is an example of making a swift configuration.  First run

    rclone config

This will guide you through an interactive setup process.

```
No remotes found - make a new one
n) New remote
q) Quit config
n/q> n
name> remote
What type of source is it?
Choose a number from below
 1) swift
 2) s3
 3) local
 4) drive
type> 1
User name to log in.
user> user_name
API key or password.
key> password_or_api_key
Authentication URL for server.
Choose a number from below, or type in your own value
 * Rackspace US
 1) https://auth.api.rackspacecloud.com/v1.0
 * Rackspace UK
 2) https://lon.auth.api.rackspacecloud.com/v1.0
 * Rackspace v2
 3) https://identity.api.rackspacecloud.com/v2.0
 * Memset Memstore UK
 4) https://auth.storage.memset.com/v1.0
 * Memset Memstore UK v2
 5) https://auth.storage.memset.com/v2.0
auth> 1
Remote config
--------------------
[remote]
user = user_name
key = password_or_api_key
auth = https://auth.api.rackspacecloud.com/v1.0
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

This remote is called `remote` and can now be used like this

See all containers

    rclone lsd remote:

Make a new container

    rclone mkdir remote:container

List the contents of a container

    rclone ls remote:container

Sync `/home/local/directory` to the remote container, deleting any
excess files in the container.

    rclone sync /home/local/directory remote:container

Modified time
-------------

The modified time is stored as metadata on the object as
`X-Object-Meta-Mtime` as floating point since the epoch accurate to 1
ns.

This is a defacto standard (used in the official python-swiftclient
amongst others) for storing the modification time for an object.
