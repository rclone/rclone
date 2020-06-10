---
title: "Crypt"
description: "Encryption overlay remote"
---

{{< icon "fa fa-lock" >}}Crypt
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
`s3:` then rclone will make encrypted bucket names too (if using file
name encryption) which may or may not be what you want.

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
[snip]
XX / Encrypt/Decrypt a remote
   \ "crypt"
[snip]
Storage> crypt
Remote to encrypt/decrypt.
Normally should contain a ':' and a path, eg "myremote:path/to/dir",
"myremote:bucket" or maybe "myremote:" (not recommended).
remote> remote:path
How to encrypt the filenames.
Choose a number from below, or type in your own value
 1 / Don't encrypt the file names.  Adds a ".bin" extension only.
   \ "off"
 2 / Encrypt the filenames see the docs for the details.
   \ "standard"
 3 / Very simple filename obfuscation.
   \ "obfuscate"
filename_encryption> 2
Option to either encrypt directory names or leave them intact.
Choose a number from below, or type in your own value
 1 / Encrypt directory names.
   \ "true"
 2 / Don't encrypt directory names, leave them intact.
   \ "false"
filename_encryption> 1
Password or pass phrase for encryption.
y) Yes type in my own password
g) Generate random password
y/g> y
Enter the password:
password:
Confirm the password:
password:
Password or pass phrase for salt. Optional but recommended.
Should be different to the previous password.
y) Yes type in my own password
g) Generate random password
n) No leave this optional password blank
y/g/n> g
Password strength in bits.
64 is just about memorable
128 is secure
1024 is the maximum
Bits> 128
Your password is: JAsJvRcgR-_veXNfy_sGmQ
Use this password?
y) Yes
n) No
y/n> y
Remote config
--------------------
[secret]
remote = remote:path
filename_encryption = standard
password = *** ENCRYPTED ***
password2 = *** ENCRYPTED ***
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

**Important** The password is stored in the config file is lightly
obscured so it isn't immediately obvious what it is.  It is in no way
secure unless you use config file encryption.

A long passphrase is recommended, or you can use a random one.

The obscured password is created by using AES-CTR with a static key, with
the salt stored verbatim at the beginning of the obscured password. This
static key is shared by between all versions of rclone.

If you reconfigure rclone with the same passwords/passphrases
elsewhere it will be compatible, but the obscured version will be different
due to the different salt.

Note that rclone does not encrypt

  * file length - this can be calculated within 16 bytes
  * modification time - used for syncing

## Specifying the remote ##

In normal use, make sure the remote has a `:` in. If you specify the
remote without a `:` then rclone will use a local directory of that
name.  So if you use a remote of `/path/to/secret/files` then rclone
will encrypt stuff to that directory.  If you use a remote of `name`
then rclone will put files in a directory called `name` in the current
directory.

If you specify the remote as `remote:path/to/dir` then rclone will
store encrypted files in `path/to/dir` on the remote. If you are using
file name encryption, then when you save files to
`secret:subdir/subfile` this will store them in the unencrypted path
`path/to/dir` but the `subdir/subpath` bit will be encrypted.

Note that unless you want encrypted bucket names (which are difficult
to manage because you won't know what directory they represent in web
interfaces etc), you should probably specify a bucket, eg
`remote:secretbucket` when using bucket based remotes such as S3,
Swift, Hubic, B2, GCS.

## Example ##

To test I made a little directory of files using "standard" file name
encryption.

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

If don't use file name encryption then the remote will look like this
- note the `.bin` extensions added to prevent the cloud provider
attempting to interpret the data.

```
$ rclone -q ls remote:path
       54 file0.txt.bin
       57 subdir/file3.txt.bin
       56 subdir/file2.txt.bin
       58 subdir/subsubdir/file4.txt.bin
       55 file1.txt.bin
```

### File name encryption modes ###

Here are some of the features of the file name encryption modes

Off

  * doesn't hide file names or directory structure
  * allows for longer file names (~246 characters)
  * can use sub paths and copy single files

Standard

  * file names encrypted
  * file names can't be as long (~143 characters)
  * can use sub paths and copy single files
  * directory structure visible
  * identical files names will have identical uploaded names
  * can use shortcuts to shorten the directory recursion

Obfuscation

This is a simple "rotate" of the filename, with each file having a rot
distance based on the filename. We store the distance at the beginning
of the filename. So a file called "hello" may become "53.jgnnq".

This is not a strong encryption of filenames, but it may stop automated
scanning tools from picking up on filename patterns. As such it's an
intermediate between "off" and "standard". The advantage is that it
allows for longer path segment names.

There is a possibility with some unicode based filenames that the
obfuscation is weak and may map lower case characters to upper case
equivalents.  You can not rely on this for strong protection.

  * file names very lightly obfuscated
  * file names can be longer than standard encryption
  * can use sub paths and copy single files
  * directory structure visible
  * identical files names will have identical uploaded names

Cloud storage systems have various limits on file name length and
total path length which you are more likely to hit using "Standard"
file name encryption.  If you keep your file names to below 156
characters in length then you should be OK on all providers.

There may be an even more secure file name encryption mode in the
future which will address the long file name problem.

### Directory name encryption ###
Crypt offers the option of encrypting dir names or leaving them intact.
There are two options:

True

Encrypts the whole file path including directory names
Example:
`1/12/123.txt` is encrypted to
`p0e52nreeaj0a5ea7s64m4j72s/l42g6771hnv3an9cgc8cr2n1ng/qgm4avr35m5loi1th53ato71v0`

False

Only encrypts file names, skips directory names
Example:
`1/12/123.txt` is encrypted to
`1/12/qgm4avr35m5loi1th53ato71v0`


### Modified time and hashes ###

Crypt stores modification times using the underlying remote so support
depends on that.

Hashes are not stored for crypt.  However the data integrity is
protected by an extremely strong crypto authenticator.

Note that you should use the `rclone cryptcheck` command to check the
integrity of a crypted remote instead of `rclone check` which can't
check the checksums properly.

{{< rem autogenerated options start" - DO NOT EDIT - instead edit fs.RegInfo in backend/crypt/crypt.go then run make backenddocs" >}}
### Standard Options

Here are the standard options specific to crypt (Encrypt/Decrypt a remote).

#### --crypt-remote

Remote to encrypt/decrypt.
Normally should contain a ':' and a path, eg "myremote:path/to/dir",
"myremote:bucket" or maybe "myremote:" (not recommended).

- Config:      remote
- Env Var:     RCLONE_CRYPT_REMOTE
- Type:        string
- Default:     ""

#### --crypt-filename-encryption

How to encrypt the filenames.

- Config:      filename_encryption
- Env Var:     RCLONE_CRYPT_FILENAME_ENCRYPTION
- Type:        string
- Default:     "standard"
- Examples:
    - "standard"
        - Encrypt the filenames see the docs for the details.
    - "obfuscate"
        - Very simple filename obfuscation.
    - "off"
        - Don't encrypt the file names.  Adds a ".bin" extension only.

#### --crypt-directory-name-encryption

Option to either encrypt directory names or leave them intact.

NB If filename_encryption is "off" then this option will do nothing.

- Config:      directory_name_encryption
- Env Var:     RCLONE_CRYPT_DIRECTORY_NAME_ENCRYPTION
- Type:        bool
- Default:     true
- Examples:
    - "true"
        - Encrypt directory names.
    - "false"
        - Don't encrypt directory names, leave them intact.

#### --crypt-password

Password or pass phrase for encryption.

**NB** Input to this must be obscured - see [rclone obscure](/commands/rclone_obscure/).

- Config:      password
- Env Var:     RCLONE_CRYPT_PASSWORD
- Type:        string
- Default:     ""

#### --crypt-password2

Password or pass phrase for salt. Optional but recommended.
Should be different to the previous password.

**NB** Input to this must be obscured - see [rclone obscure](/commands/rclone_obscure/).

- Config:      password2
- Env Var:     RCLONE_CRYPT_PASSWORD2
- Type:        string
- Default:     ""

### Advanced Options

Here are the advanced options specific to crypt (Encrypt/Decrypt a remote).

#### --crypt-show-mapping

For all files listed show how the names encrypt.

If this flag is set then for each file that the remote is asked to
list, it will log (at level INFO) a line stating the decrypted file
name and the encrypted file name.

This is so you can work out which encrypted names are which decrypted
names just in case you need to do something with the encrypted file
names, or for debugging purposes.

- Config:      show_mapping
- Env Var:     RCLONE_CRYPT_SHOW_MAPPING
- Type:        bool
- Default:     false

### Backend commands

Here are the commands specific to the crypt backend.

Run them with

    rclone backend COMMAND remote:

The help below will explain what arguments each command takes.

See [the "rclone backend" command](/commands/rclone_backend/) for more
info on how to pass options and arguments.

These can be run on a running backend using the rc command
[backend/command](/rc/#backend/command).

#### encode

Encode the given filename(s)

    rclone backend encode remote: [options] [<arguments>+]

This encodes the filenames given as arguments returning a list of
strings of the encoded results.

Usage Example:

    rclone backend encode crypt: file1 [file2...]
    rclone rc backend/command command=encode fs=crypt: file1 [file2...]


#### decode

Decode the given filename(s)

    rclone backend decode remote: [options] [<arguments>+]

This decodes the filenames given as arguments returning a list of
strings of the decoded results. It will return an error if any of the
inputs are invalid.

Usage Example:

    rclone backend decode crypt: encryptedfile1 [encryptedfile2...]
    rclone rc backend/command command=decode fs=crypt: encryptedfile1 [encryptedfile2...]


{{< rem autogenerated options stop >}}

## Backing up a crypted remote ##

If you wish to backup a crypted remote, it is recommended that you use
`rclone sync` on the encrypted files, and make sure the passwords are
the same in the new encrypted remote.

This will have the following advantages

  * `rclone sync` will check the checksums while copying
  * you can use `rclone check` between the encrypted remotes
  * you don't decrypt and encrypt unnecessarily

For example, let's say you have your original remote at `remote:` with
the encrypted version at `eremote:` with path `remote:crypt`.  You
would then set up the new remote `remote2:` and then the encrypted
version `eremote2:` with path `remote2:crypt` using the same passwords
as `eremote:`.

To sync the two remotes you would do

    rclone sync remote:crypt remote2:crypt

And to check the integrity you would do

    rclone check remote:crypt remote2:crypt

## File formats ##

### File encryption ###

Files are encrypted 1:1 source file to destination object.  The file
has a header and is divided into chunks.

#### Header ####

  * 8 bytes magic string `RCLONE\x00\x00`
  * 24 bytes Nonce (IV)

The initial nonce is generated from the operating systems crypto
strong random number generator.  The nonce is incremented for each
chunk read making sure each nonce is unique for each block written.
The chance of a nonce being re-used is minuscule.  If you wrote an
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

File names are encrypted segment by segment - the path is broken up
into `/` separated strings and these are encrypted individually.

File segments are padded using PKCS#7 to a multiple of 16 bytes
before encryption.

They are then encrypted with EME using AES with 256 bit key. EME
(ECB-Mix-ECB) is a wide-block encryption mode presented in the 2003
paper "A Parallelizable Enciphering Mode" by Halevi and Rogaway.

This makes for deterministic encryption which is what we want - the
same filename must encrypt to the same thing otherwise we can't find
it on the cloud storage system.

This means that

  * filenames with the same name will encrypt the same
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

Rclone uses `scrypt` with parameters `N=16384, r=8, p=1` with an
optional user supplied salt (password2) to derive the 32+32+16 = 80
bytes of key material required.  If the user doesn't supply a salt
then rclone uses an internal one.

`scrypt` makes it impractical to mount a dictionary attack on rclone
encrypted data.  For full protection against this you should always use
a salt.
