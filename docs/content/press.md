---
title: "Press"
description: "Compression Remote"
date: "2019-05-12"
---

Press (Experimental)
-----------------------------------------

The `press` remote adds compression to another remote. It is best used with remotes containing
many large compressible files or on top of other remotes like crypt.

Please read the [warnings](#warnings) before using this remote.

To use this remote, all you need to do is specify another remote and a compression mode to use:

```
Current remotes:

Name                 Type
====                 ====
remote_to_press      sometype

e) Edit existing remote
$ rclone config
n) New remote
d) Delete remote
r) Rename remote
c) Copy remote
s) Set configuration password
q) Quit config
e/n/d/r/c/s/q> n
name> press
...
 8 / Compress a remote
   \ "press"
...
Storage> press
** See help for press backend at: https://rclone.org/press/ **

Remote to compress.
Enter a string value. Press Enter for the default ("")
remote> remote_to_press
Compression mode. XZ compression mode requires the xz binary to be in PATH.
Enter a string value. Press Enter for the default ("gzip-min").
Choose a number from below, or type in your own value
 1 / Fast, real-time compression with reasonable compression ratios.
   \ "lz4"
 2 / Google's compression algorithm. Slightly faster and larger than LZ4.
   \ "snappy"
 3 / Standard gzip compression with fastest parameters.
   \ "gzip-min"
 4 / Standard gzip compression with default parameters.
   \ "gzip-default"
 5 / Slow but powerful compression with reasonable speed.
   \ "xz-min"
 6 / Slowest but best compression.
   \ "xz-default"
compression_mode> gzip-min
```

### Compression Modes
Currently there are four compression algorithms supported: lz4, snappy, gzip, and xz.
Gzip and xz are further divided into two modes: "min" with less compression and "default" with more.
Currently, xz modes are only supported if there is an xz binary in your system's $PATH.
Depending on your operating system, the methods of installing this binary vary. This may be changed in
future updates.

### Warnings

#### Filetype
If you open a remote wrapped by press, you will see that there are many files with an extension corresponding to
the compression algorithm you chose. These files, with the exception of snappy files, are standard files that
can be opened by various archive programs, but they have some hidden metadata that allows them to be used by rclone.
While you may download and decompress these files at will, do **not** upload any compressed files to a wrapped remote
through any other means than rclone. This will upload files that do not contain metadata and **will** cause unexpected behavior.

#### Experimental
This remote is currently **experimental**. Things may break and data may be lost. Anything you do with this remote is
at your own risk. Please understand the risks associated with using experimental code and don't use this remote in
critical applications.