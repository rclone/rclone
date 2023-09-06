<img src="docs/gopher.png" alt="gopher" align="right" width="200"/>

HDFS for Go
===========

[![GoDoc](https://godoc.org/github.com/colinmarc/hdfs/web?status.svg)](https://godoc.org/github.com/colinmarc/hdfs/v2) [![build](https://github.com/colinmarc/hdfs/actions/workflows/tests.yml/badge.svg?branch=master)](https://github.com/colinmarc/hdfs/actions/workflows/tests.yml)

This is a native golang client for hdfs. It connects directly to the namenode using
the protocol buffers API.

It tries to be idiomatic by aping the stdlib `os` package, where possible, and
implements the interfaces from it, including `os.FileInfo` and `os.PathError`.

Here's what it looks like in action:

```go
client, _ := hdfs.New("namenode:8020")

file, _ := client.Open("/mobydick.txt")

buf := make([]byte, 59)
file.ReadAt(buf, 48847)

fmt.Println(string(buf))
// => Abominable are the tumblers into which he pours his poison.
```

For complete documentation, check out the [Godoc][1].

The `hdfs` Binary
-----------------

Along with the library, this repo contains a commandline client for HDFS. Like
the library, its primary aim is to be idiomatic, by enabling your favorite unix
verbs:


    $ hdfs --help
    Usage: hdfs COMMAND
    The flags available are a subset of the POSIX ones, but should behave similarly.

    Valid commands:
      ls [-lah] [FILE]...
      rm [-rf] FILE...
      mv [-fT] SOURCE... DEST
      mkdir [-p] FILE...
      touch [-amc] FILE...
      chmod [-R] OCTAL-MODE FILE...
      chown [-R] OWNER[:GROUP] FILE...
      cat SOURCE...
      head [-n LINES | -c BYTES] SOURCE...
      tail [-n LINES | -c BYTES] SOURCE...
      du [-sh] FILE...
      checksum FILE...
      get SOURCE [DEST]
      getmerge SOURCE DEST
      put SOURCE DEST

Since it doesn't have to wait for the JVM to start up, it's also a lot faster
`hadoop -fs`:

    $ time hadoop fs -ls / > /dev/null

    real  0m2.218s
    user  0m2.500s
    sys 0m0.376s

    $ time hdfs ls / > /dev/null

    real  0m0.015s
    user  0m0.004s
    sys 0m0.004s

Best of all, it comes with bash tab completion for paths!

Installing the commandline client
---------------------------------

Grab a tarball from the [releases page](https://github.com/colinmarc/hdfs/releases)
and unzip it wherever you like.

To configure the client, make sure one or both of these environment variables
point to your Hadoop configuration (`core-site.xml` and `hdfs-site.xml`). On
systems with Hadoop installed, they should already be set.

    $ export HADOOP_HOME="/etc/hadoop"
    $ export HADOOP_CONF_DIR="/etc/hadoop/conf"

To install tab completion globally on linux, copy or link the `bash_completion`
file which comes with the tarball into the right place:

    $ ln -sT bash_completion /etc/bash_completion.d/gohdfs

By default on non-kerberized clusters, the HDFS user is set to the
currently-logged-in user. You can override this with another environment
variable:

    $ export HADOOP_USER_NAME=username

Using the commandline client with Kerberos authentication
---------------------------------------------------------

Like `hadoop fs`, the commandline client expects a `ccache` file in the default
location: `/tmp/krb5cc_<uid>`. That means it should 'just work' to use `kinit`:

    $ kinit bob@EXAMPLE.com
    $ hdfs ls /

If that doesn't work, try setting the `KRB5CCNAME` environment variable to
wherever you have the `ccache` saved.

Compatibility
-------------

This library uses "Version 9" of the HDFS protocol, which means it should work
with hadoop distributions based on 2.2.x and above, as well as 3.x.

Acknowledgements
----------------

This library is heavily indebted to [snakebite][3].

[1]: https://godoc.org/github.com/colinmarc/hdfs
[2]: https://golang.org/doc/install
[3]: https://github.com/spotify/snakebite
