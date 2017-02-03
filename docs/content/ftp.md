---
title: "FTP"
description: "Rclone docs for FTP backend"
date: "2017-01-01"
---

<i class="fa fa-file"></i> FTP
------------------------------

FTP support is provided via
[github.com/jlaffaye/ftp](https://godoc.org/github.com/jlaffaye/ftp)
package.

### Configuration ###

An Ftp backend only needs an Url and and username and password. With
anonymous FTP server you will need to use `anonymous` as username and
your email address as password.

Example:
```
[remote]
type = Ftp
username = anonymous
password = john.snow@example.org
url = ftp://ftp.kernel.org/pub
```

### Unsupported features ###

FTP backends does not support:

* Any hash mechanism
* Modified Time
* remote copy/move
