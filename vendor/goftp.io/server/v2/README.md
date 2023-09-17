# server

[![](http://gocover.io/_badge/gitea.com/goftp/server)](http://gocover.io/gitea.com/goftp/server)
[![](https://goreportcard.com/badge/gitea.com/goftp/server)](https://goreportcard.com/report/gitea.com/goftp/server)

A FTP server framework forked from [github.com/yob/graval](http://github.com/yob/graval) and changed a lot.

Full documentation for the package is available on [godoc](http://pkg.go.dev/goftp.io/server)

## Installation

    go get goftp.io/server/v2

## Usage

To boot a FTP server you will need to provide a driver that speaks to
your persistence layer - the required driver contract is in [the
documentation](http://pkg.go.dev/goftp.io/server).

Look at the [file driver](https://goftp.io/server/driver/file) to see
an example of how to build a backend.

There is a [sample ftp server](https://goftp.io/ftpd) as a demo. You can build it with this
command:

    go install goftp.io/ftpd

And finally, connect to the server with any FTP client and the following
details:

    host: 127.0.0.1
    port: 2121
    username: admin
    password: 123456

This uses the file driver mentioned above to serve files.

## Contact us

You can contact us via discord [https://discord.gg/ytmYqfNfqh](https://discord.gg/ytmYqfNfqh) or QQ群 972357369

## Contributors

see [https://gitea.com/goftp/server/graphs/contributors](https://gitea.com/goftp/server/graphs/contributors)

## Warning

FTP is an incredibly insecure protocol. Be careful about forcing users to authenticate
with an username or password that are important.

## License

This library is distributed under the terms of the MIT License. See the included file for
more detail.

## Contributing

All suggestions and patches welcome, preferably via a git repository I can pull from.
If this library proves useful to you, please let me know.

## Further Reading

There are a range of RFCs that together specify the FTP protocol. In chronological
order, the more useful ones are:

* [http://tools.ietf.org/rfc/rfc959.txt](http://tools.ietf.org/rfc/rfc959.txt)
* [http://tools.ietf.org/rfc/rfc1123.txt](http://tools.ietf.org/rfc/rfc1123.txt)
* [http://tools.ietf.org/rfc/rfc2228.txt](http://tools.ietf.org/rfc/rfc2228.txt)
* [http://tools.ietf.org/rfc/rfc2389.txt](http://tools.ietf.org/rfc/rfc2389.txt)
* [http://tools.ietf.org/rfc/rfc2428.txt](http://tools.ietf.org/rfc/rfc2428.txt)
* [http://tools.ietf.org/rfc/rfc3659.txt](http://tools.ietf.org/rfc/rfc3659.txt)
* [http://tools.ietf.org/rfc/rfc4217.txt](http://tools.ietf.org/rfc/rfc4217.txt)

For an english summary that's somewhat more legible than the RFCs, and provides
some commentary on what features are actually useful or relevant 24 years after
RFC959 was published:

* [http://cr.yp.to/ftp.html](http://cr.yp.to/ftp.html)

For a history lesson, check out Appendix III of RCF959. It lists the preceding
(obsolete) RFC documents that relate to file transfers, including the ye old
RFC114 from 1971, "A File Transfer Protocol"

This library is heavily based on [em-ftpd](https://github.com/yob/em-ftpd), an FTPd
framework with similar design goals within the ruby and EventMachine ecosystems. It
worked well enough, but you know, callbacks and event loops make me something
something.
