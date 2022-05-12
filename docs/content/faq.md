---
title: "FAQ"
description: "Rclone Frequently Asked Questions"
---

# Frequently Asked Questions

### Do all cloud storage systems support all rclone commands ###

Yes they do.  All the rclone commands (e.g. `sync`, `copy`, etc.) will
work on all the remote storage systems.

### Can I copy the config from one machine to another ###

Sure!  Rclone stores all of its config in a single file.  If you want
to find this file, run `rclone config file` which will tell you where
it is.

See the [remote setup docs](/remote_setup/) for more info.

### How do I configure rclone on a remote / headless box with no browser? ###

This has now been documented in its own [remote setup page](/remote_setup/).

### Can rclone sync directly from drive to s3 ###

Rclone can sync between two remote cloud storage systems just fine.

Note that it effectively downloads the file and uploads it again, so
the node running rclone would need to have lots of bandwidth.

The syncs would be incremental (on a file by file basis).

e.g.

    rclone sync -i drive:Folder s3:bucket


### Using rclone from multiple locations at the same time ###

You can use rclone from multiple places at the same time if you choose
different subdirectory for the output, e.g.

```
Server A> rclone sync -i /tmp/whatever remote:ServerA
Server B> rclone sync -i /tmp/whatever remote:ServerB
```

If you sync to the same directory then you should use rclone copy
otherwise the two instances of rclone may delete each other's files, e.g.

```
Server A> rclone copy /tmp/whatever remote:Backup
Server B> rclone copy /tmp/whatever remote:Backup
```

The file names you upload from Server A and Server B should be
different in this case, otherwise some file systems (e.g. Drive) may
make duplicates.

### Why doesn't rclone support partial transfers / binary diffs like rsync? ###

Rclone stores each file you transfer as a native object on the remote
cloud storage system.  This means that you can see the files you
upload as expected using alternative access methods (e.g. using the
Google Drive web interface).  There is a 1:1 mapping between files on
your hard disk and objects created in the cloud storage system.

Cloud storage systems (at least none I've come across yet) don't
support partially uploading an object. You can't take an existing
object, and change some bytes in the middle of it.

It would be possible to make a sync system which stored binary diffs
instead of whole objects like rclone does, but that would break the
1:1 mapping of files on your hard disk to objects in the remote cloud
storage system.

All the cloud storage systems support partial downloads of content, so
it would be possible to make partial downloads work.  However to make
this work efficiently this would require storing a significant amount
of metadata, which breaks the desired 1:1 mapping of files to objects.

### Can rclone do bi-directional sync? ###

No, not at present.  rclone only does uni-directional sync from A ->
B. It may do in the future though since it has all the primitives - it
just requires writing the algorithm to do it.

### Can I use rclone with an HTTP proxy? ###

Yes. rclone will follow the standard environment variables for
proxies, similar to cURL and other programs.

In general the variables are called `http_proxy` (for services reached
over `http`) and `https_proxy` (for services reached over `https`).  Most
public services will be using `https`, but you may wish to set both.

The content of the variable is `protocol://server:port`.  The protocol
value is the one used to talk to the proxy server, itself, and is commonly
either `http` or `socks5`.

Slightly annoyingly, there is no _standard_ for the name; some applications
may use `http_proxy` but another one `HTTP_PROXY`.  The `Go` libraries
used by `rclone` will try both variations, but you may wish to set all
possibilities.  So, on Linux, you may end up with code similar to

    export http_proxy=http://proxyserver:12345
    export https_proxy=$http_proxy
    export HTTP_PROXY=$http_proxy
    export HTTPS_PROXY=$http_proxy

The `NO_PROXY` allows you to disable the proxy for specific hosts.
Hosts must be comma separated, and can contain domains or parts.
For instance "foo.com" also matches "bar.foo.com".

e.g.

    export no_proxy=localhost,127.0.0.0/8,my.host.name
    export NO_PROXY=$no_proxy

Note that the ftp backend does not support `ftp_proxy` yet.

### Rclone gives x509: failed to load system roots and no roots provided error ###

This means that `rclone` can't find the SSL root certificates.  Likely
you are running `rclone` on a NAS with a cut-down Linux OS, or
possibly on Solaris.

Rclone (via the Go runtime) tries to load the root certificates from
these places on Linux.

    "/etc/ssl/certs/ca-certificates.crt", // Debian/Ubuntu/Gentoo etc.
    "/etc/pki/tls/certs/ca-bundle.crt",   // Fedora/RHEL
    "/etc/ssl/ca-bundle.pem",             // OpenSUSE
    "/etc/pki/tls/cacert.pem",            // OpenELEC

So doing something like this should fix the problem.  It also sets the
time which is important for SSL to work properly.

```
mkdir -p /etc/ssl/certs/
curl -o /etc/ssl/certs/ca-certificates.crt https://raw.githubusercontent.com/bagder/ca-bundle/master/ca-bundle.crt
ntpclient -s -h pool.ntp.org
```

The two environment variables `SSL_CERT_FILE` and `SSL_CERT_DIR`, mentioned in the [x509 package](https://godoc.org/crypto/x509),
provide an additional way to provide the SSL root certificates.

Note that you may need to add the `--insecure` option to the `curl` command line if it doesn't work without.

```
curl --insecure -o /etc/ssl/certs/ca-certificates.crt https://raw.githubusercontent.com/bagder/ca-bundle/master/ca-bundle.crt
```

### Rclone gives Failed to load config file: function not implemented error ###

Likely this means that you are running rclone on Linux version not
supported by the go runtime, ie earlier than version 2.6.23.

See the [system requirements section in the go install
docs](https://golang.org/doc/install) for full details.

### All my uploaded docx/xlsx/pptx files appear as archive/zip ###

This is caused by uploading these files from a Windows computer which
hasn't got the Microsoft Office suite installed.  The easiest way to
fix is to install the Word viewer and the Microsoft Office
Compatibility Pack for Word, Excel, and PowerPoint 2007 and later
versions' file formats

### tcp lookup some.domain.com no such host ###

This happens when rclone cannot resolve a domain. Please check that
your DNS setup is generally working, e.g.

```
# both should print a long list of possible IP addresses
dig www.googleapis.com          # resolve using your default DNS
dig www.googleapis.com @8.8.8.8 # resolve with Google's DNS server
```

If you are using `systemd-resolved` (default on Arch Linux), ensure it
is at version 233 or higher. Previous releases contain a bug which
causes not all domains to be resolved properly.

Additionally with the `GODEBUG=netdns=` environment variable the Go
resolver decision can be influenced. This also allows to resolve certain
issues with DNS resolution. See the [name resolution section in the go docs](https://golang.org/pkg/net/#hdr-Name_Resolution).

### The total size reported in the stats for a sync is wrong and keeps changing

It is likely you have more than 10,000 files that need to be
synced. By default, rclone only gets 10,000 files ahead in a sync so as
not to use up too much memory. You can change this default with the
[--max-backlog](/docs/#max-backlog-n) flag.

### Rclone is using too much memory or appears to have a memory leak

Rclone is written in Go which uses a garbage collector.  The default
settings for the garbage collector mean that it runs when the heap
size has doubled.

However it is possible to tune the garbage collector to use less
memory by [setting GOGC](https://dave.cheney.net/tag/gogc) to a lower
value, say `export GOGC=20`.  This will make the garbage collector
work harder, reducing memory size at the expense of CPU usage.

The most common cause of rclone using lots of memory is a single
directory with thousands or millions of files in.  Rclone has to load
this entirely into memory as rclone objects.  Each rclone object takes
0.5k-1k of memory.

### Rclone changes fullwidth Unicode punctuation marks in file names

For example: On a Windows system, you have a file with name `Test：1.jpg`,
where `：` is the Unicode fullwidth colon symbol. When using rclone
to copy this to your Google Drive, you will notice that the file
gets renamed to `Test:1.jpg`, where `:` is the regular (halfwidth) colon.

The reason for such renames is the way rclone handles different
[restricted filenames](/overview/#restricted-filenames) on different
cloud storage systems. It tries to avoid ambiguous file names as
much and allow moving files between many cloud storage systems
transparently, by replacing invalid characters with similar looking
Unicode characters when transferring to one storage system, and replacing
back again when transferring to a different storage system where the
original characters are supported. When the same Unicode characters
are intentionally used in file names, this replacement strategy leads
to unwanted renames. Read more [here](/overview/#restricted-filenames-caveats).
