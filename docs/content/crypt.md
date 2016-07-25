---
title: "Crypt"
description: "Encryption overlay remote"
date: "2016-07-28"
---

<i class="fa fa-lock"></i>Crypt
----------------------------------------

The `crypt` remote encrypts and decrypts another remote.

To use it first set up the underlying remote following the config
instructions for that remote.  You can also use a local pathname
instead of a remote which will encrypt and decrypt from that directory
which might be useful for encrypting onto a USB stick for example.

First check your chosen remote is working - we'll call it
`remote:path` in these docs.  Note that anything inside `remote:path`
will be encrypted and anything outside won't.  This means that if you
are using a bucket based remote (eg S3, B2, swift) then you should
probably put the bucket in the remote `s3:bucket`. If you just use
`s3:` then rclone will make encrypted bucket names too which may or
may not be what you want.

Now configure `crypt` using `rclone config`. We will call this one
`secret` to differentiate it from the `remote`.

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> secret
Type of storage to configure.
Choose a number from below, or type in your own value
 1 / Amazon Drive
   \ "amazon cloud drive"
 2 / Amazon S3 (also Dreamhost, Ceph, Minio)
   \ "s3"
 3 / Backblaze B2
   \ "b2"
 4 / Dropbox
   \ "dropbox"
 5 / Encrypt/Decrypt a remote
   \ "crypt"
 6 / Google Cloud Storage (this is not Google Drive)
   \ "google cloud storage"
 7 / Google Drive
   \ "drive"
 8 / Hubic
   \ "hubic"
 9 / Local Disk
   \ "local"
10 / Microsoft OneDrive
   \ "onedrive"
11 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
   \ "swift"
12 / Yandex Disk
   \ "yandex"
Storage> 5
Remote to encrypt/decrypt.
remote> remote:path
Flatten the directory structure - more secure, less useful - see docs for tradeoffs.
Choose a number from below, or type in your own value
 1 / Don't flatten files (default) - good for unlimited files, but doesn't hide directory structure.
   \ "0"
 2 / Spread files over 1 directory good for <10,000 files.
   \ "1"
 3 / Spread files over 32 directories good for <320,000 files.
   \ "2"
 4 / Spread files over 1024 directories good for <10,000,000 files.
   \ "3"
 5 / Spread files over 32,768 directories good for <320,000,000 files.
   \ "4"
 6 / Spread files over 1,048,576 levels good for <10,000,000,000 files.
   \ "5"
flatten> 1
Password or pass phrase for encryption.
Enter the password:
password:
Confirm the password:
password:
Remote config
--------------------
[secret]
remote = remote:path
flatten = 0
password = 0_gtCJ422bzwAWP0UN2lggrjhA-sSg
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

**Important** The password is stored in the config file is lightly
obscured so it isn't immediately obvious what it is.  It is in no way
secure unless you use config file encryption.

A long passphrase is recommended, or you can use a random one.  Note
that if you reconfigure rclone with the same password/passphrase
elsewhere it will be compatible - all the secrets used are derived
from that one password/passphrase.

Note that rclone does not encrypt
  * file length - this can be calcuated within 16 bytes
  * modification time - used for syncing

## Example ##

To test I made a little directory of files

```
plaintext/
├── file0.txt
├── file1.txt
└── subdir
    ├── file2.txt
    ├── file3.txt
    └── subsubdir
        └── file4.txt
```

Copy these to the remote and list them back

```
$ rclone -q copy plaintext secret:
$ rclone -q ls secret:
        7 file1.txt
        6 file0.txt
        8 subdir/file2.txt
       10 subdir/subsubdir/file4.txt
        9 subdir/file3.txt
```

Now see what that looked like when encrypted

```
$ rclone -q ls remote:path
       55 hagjclgavj2mbiqm6u6cnjjqcg
       54 v05749mltvv1tf4onltun46gls
       57 86vhrsv86mpbtd3a0akjuqslj8/dlj7fkq4kdq72emafg7a7s41uo
       58 86vhrsv86mpbtd3a0akjuqslj8/7uu829995du6o42n32otfhjqp4/b9pausrfansjth5ob3jkdqd4lc
       56 86vhrsv86mpbtd3a0akjuqslj8/8njh1sk437gttmep3p70g81aps
```

Note that this retains the directory structure which means you can do this

```
$ rclone -q ls secret:subdir
        8 file2.txt
        9 file3.txt
       10 subsubdir/file4.txt
```

If you use the flattened flag then the listing will look and that last command will not work.

```
$ rclone -q ls remote:path
       56 t/tsdtcpdu6g9dpamn6poqc248tll9dj5ok78a363etmq8ushr821g
       57 g/gsrp2g0u85pgsi6kso74bjsrsafe11odpfln8qqpj6n9p20of0a0
       55 h/hagjclgavj2mbiqm6u6cnjjqcg
       58 4/4jsbao3dhi0jfoubt2oo493pbqmsshn92q01ddu7dg6428rlluhg
       54 v/v05749mltvv1tf4onltun46gls
```

### Flattened vs non-Flattened ###

Pros and cons of each

Flattened
  * hides directory structures
  * identical file names won't have identical encrypted names
  * can't use a sub path
    * doesn't work: `rclone copy crypt:sub/dir /tmp/recovered`
    * use: `rclone copy --include "/sub/dir/**" crypt: /tmp/recovered`
  * will always have to recurse through the entire directory structure
  * can't copy a single file directly
    * doesn't work: `rclone copy crypt:path/to/file /tmp/recovered`
    * use: `rclone copy --include "/path/to/file" crypt: /tmp/recovered`

Normal
  * can use sub paths and copy single files
  * directory structure visibile
  * identical files names will have identical uploaded names
  * can use shortcuts to shorten the directory recursion

You can swap between flattened levels without re-uploading your files.

## File formats ##

### File encryption ###

Files are encrypted 1:1 source file to destination object.  The file
has a header and is divided into chunks.

#### Header ####

  * 8 bytes magic string `RCLONE\x00\x00`
  * 24 bytes Nonce (IV)

The initial nonce is generated from the operating systems crypto
strong random number genrator.  The nonce is incremented for each
chunk read making sure each nonce is unique for each block written.
The chance of a nonce being re-used is miniscule.  If you wrote an
exabyte of data (10¹⁸ bytes) you would have a probability of
approximately 2×10⁻³² of re-using a nonce.

#### Chunk ####

Each chunk will contain 64kB of data, except for the last one which
may have less data.  The data chunk is in standard NACL secretbox
format. Secretbox uses XSalsa20 and Poly1305 to encrypt and
authenticate messages.

Each chunk contains:

  * 16 Bytes of Poly1305 authenticator
  * 1 - 65536 bytes XSalsa20 encrypted data

64k chunk size was chosen as the best performing chunk size (the
authenticator takes too much time below this and the performance drops
off due to cache effects above this).  Note that these chunks are
buffered in memory so they can't be too big.

This uses a 32 byte (256 bit key) key derived from the user password.

#### Examples ####

1 byte file will encrypt to

  * 32 bytes header
  * 17 bytes data chunk

49 bytes total

1MB (1048576 bytes) file will encrypt to

  * 32 bytes header
  * 16 chunks of 65568 bytes

1049120 bytes total (a 0.05% overhead).  This is the overhead for big
files.

### Name encryption ###

File names are encrypted by crypt.  These are either encrypted segment
by segment - the path is broken up into `/` separated strings and
these are encrypted individually, or if working in flattened mode the
whole path is encrypted `/`s and all.

First file names are padded using using PKCS#7 to a multiple of 16
bytes before encryption.

They are then encrypted with EME using AES with 256 bit key. EME
(ECB-Mix-ECB) is a wide-block encryption mode presented in the 2003
paper "A Parallelizable Enciphering Mode" by Halevi and Rogaway.

This makes for determinstic encryption which is what we want - the
same filename must encrypt to the same thing.

This means that

  * filenames with the same name will encrypt the same
    * (though we can use directory flattening to avoid this if required)
  * filenames which start the same won't have a common prefix

This uses a 32 byte key (256 bits) and a 16 byte (128 bits) IV both of
which are derived from the user password.

After encryption they are written out using a modified version of
standard `base32` encoding as described in RFC4648.  The standard
encoding is modified in two ways:

  * it becomes lower case (no-one likes upper case filenames!)
  * we strip the padding character `=`

`base32` is used rather than the more efficient `base64` so rclone can be
used on case insensitive remotes (eg Windows, Amazon Drive).

### Key derivation ###

Rclone uses `scrypt` with parameters `N=16384, r=8, p=1` with a fixed
salt to derive the 32+32+16 = 80 bytes of key material required.

`scrypt` makes it impractical to mount a dictionary attack on rclone
encrypted data.
