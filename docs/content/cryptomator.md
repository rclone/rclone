---
title: "Cryptomator"
description: "Encryption overlay remote with Cryptomator"
status: "Experimental"
---

# {{< icon "fa fa-user-secret" >}} Cryptomator

Rclone `cryptomator` remotes wrap other remotes containing a
cryptomator vault.

A remote of type `cryptomator` works almost exactly like [crypt](https://rclone.org/crypt),
but it is compatible with the cryptomator vault format.

For any information about the inner workings of the cryptomator architecture
please look here: 
[Cryptomator Architecture](https://docs.cryptomator.org/en/latest/security/architecture/).


## Configuration

Here is an example of how to make a remote called `secret`.

To use `cryptomator`, first set up the underlying remote. Follow the
`rclone config` instructions for the specific backend.

Before configuring the crypt remote, check the underlying remote is
working. In this example the underlying remote is called `remote`.
We will configure a path `path` within this remote to contain the
encrypted content. Anything inside `remote:path` will be encrypted
and anything outside will not.

Configure `cryptomator` using `rclone config`. In this example the `cryptomator`
remote is called `secret`, to differentiate it from the underlying
`remote`.

When you are done you can use the crypt remote named `secret` just
as you would with any other remote, e.g. `rclone copy D:\docs secret:\docs`,
and rclone will encrypt and decrypt as needed on the fly.
If you access the wrapped remote `remote:path` directly you will bypass
the encryption, and anything you read will be in encrypted form, and
anything you write will be unencrypted. To avoid issues it is best to
configure a dedicated path for encrypted content, and access it
exclusively through a crypt remote.

```
No remotes found, make a new one?
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> secret
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
XX / Treat a remote as Cryptomator Vault
   \ (cryptomator)
[snip]
Storage> cryptomator
** See help for crypt backend at: https://rclone.org/cryptomator/ **

Remote which contains the Cryptomator Vault
Normally should contain a ':' and a path, eg "myremote:path/to/dir",
"myremote:bucket" or maybe "myremote:" (not recommended).
Enter a string value. Press Enter for the default ("").
remote> remote:path
Password or pass phrase for encryption.
y) Yes type in my own password
g) Generate random password
y/g> y
Enter the password:
password:
Confirm the password:
password:
Configuration complete.
Options:
- type: cryptomator
- remote: remote:path
- password: *** ENCRYPTED ***
Keep this "secret" remote?
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d>
```

**Important** The cryptomator password stored in `rclone.conf` is lightly
obscured. That only protects it from cursory inspection. It is not
secure unless [configuration encryption](https://rclone.org/docs/#configuration-encryption) of `rclone.conf` is specified.

A long passphrase is recommended, or `rclone config` can generate a
random one.

Rclone does not encrypt

  * file length - this can be calculated within 16 bytes
  * modification time - used for syncing

### Specifying the remote

When configuring the remote to encrypt/decrypt, you may specify any
string that rclone accepts as a source/destination of other commands.

The primary use case is to specify the path into an already configured
remote (e.g. `remote:path/to/dir` or `remote:bucket`), such that
data in a remote untrusted location can be stored encrypted.

You may also specify a local filesystem path, such as
`/path/to/dir` on Linux, `C:\path\to\dir` on Windows. By creating
a crypt remote pointing to such a local filesystem path, you can
use rclone as a utility for pure local file encryption, for example
to keep encrypted files on a removable USB drive.

**Note**: A string which do not contain a `:` will by rclone be treated
as a relative path in the local filesystem. For example, if you enter
the name `remote` without the trailing `:`, it will be treated as
a subdirectory of the current directory with name "remote".

If a path `remote:path/to/dir` is specified, rclone stores encrypted
files in `path/to/dir` on the remote. With file name encryption, files
saved to `secret:subdir/subfile` are stored in the unencrypted path
`path/to/dir` but the `subdir/subpath` element is encrypted.

The path you specify does not have to exist, rclone will create
it when needed.

If you intend to use the wrapped remote both directly for keeping
unencrypted content, as well as through a crypt remote for encrypted
content, it is recommended to point the crypt remote to a separate
directory within the wrapped remote. If you use a bucket-based storage
system (e.g. Swift, S3, Google Compute Storage, B2) it is generally
advisable to wrap the crypt remote around a specific bucket (`s3:bucket`).
If wrapping around the entire root of the storage (`s3:`), and use the
optional file name encryption, rclone will encrypt the bucket name.

### Example

Create the following file structure

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

Copy these to the remote, and list them

```
$ rclone -q copy plaintext secret:
$ rclone -q ls secret:
        4 file0.txt
       17 file1.txt
       15 subdir/file2.txt
       27 subdir/file3.txt
       62 subdir/subsubdir/file4.txt
```

The cryptomator remote looks like

```
$ rclone -q ls remote:path
      333 masterkey.cryptomator
      287 vault.cryptomator
      163 d/OE/J2IK65QFG6ONYIHXXHC62JOWRBXPE2/MMN6UShRvhUIvALTk9sQKeIOWjw84vTQFQ==.c9r
      151 d/OE/J2IK65QFG6ONYIHXXHC62JOWRBXPE2/Wy9dPAmFNWX6z5vqfT6Aw20kcWiCZJdZCA==.c9r
      172 d/OE/J2IK65QFG6ONYIHXXHC62JOWRBXPE2/dirid.c9r
       36 d/OE/J2IK65QFG6ONYIHXXHC62JOWRBXPE2/E5nx1OfnHwGRkkodhhTD4VV5GB2E27YdpQ==.c9r/dir.c9r
      153 d/6O/OJXGDC6QT2EDZORS2HFL6E66FQM2OG/AxL3-O72A4GWXNGD4VjFrTnSjW9YN_WxCg==.c9r
      140 d/6O/OJXGDC6QT2EDZORS2HFL6E66FQM2OG/FA-hSHL-nvx2B5gCkpHYBAXtlA8jHKQzQA==.c9r
      172 d/7C/JG3J7CCJERWUPBVMBC3W4XVNMU6RWQ/dirid.c9r
      198 d/7C/JG3J7CCJERWUPBVMBC3W4XVNMU6RWQ/p0kKjHLESTwXrhXHaCTo_MB91t85m64a5Q==.c9r
       36 d/6O/OJXGDC6QT2EDZORS2HFL6E66FQM2OG/e-J7eVZQiqRR_XvYrF4AqmXPDQWQmw==.c9r/dir.c9r
```

The directory structure is preserved

```
$ rclone -q ls secret:subdir
       15 file2.txt
       27 file3.txt
       62 subsubdir/file4.txt
```

### Modified time and hashes

Cryptomator stores modification times using the underlying remote so support
depends on that.

Hashes are not stored for crypt. However the data integrity is
protected by AES GCM: 
See [File Content Encryption](https://docs.cryptomator.org/en/latest/security/architecture/#file-content-encryption)


{{< rem autogenerated options start" - DO NOT EDIT - instead edit fs.RegInfo in backend/cryptomator/cryptomator.go then run make backenddocs" >}}
### Standard options

Here are the Standard options specific to cryptomator (Treat a remote as Cryptomator Vault).

#### --cryptomator-remote

Remote which contains the Cryptomator Vault

Properties:

- Config:      remote
- Env Var:     RCLONE_CRYPTOMATOR_REMOTE
- Type:        string
- Required:    true

#### --cryptomator-password

Password for the Cryptomator Vault

**NB** Input to this must be obscured - see [rclone obscure](/commands/rclone_obscure/).

Properties:

- Config:      password
- Env Var:     RCLONE_CRYPTOMATOR_PASSWORD
- Type:        string
- Required:    true

### Metadata

Any metadata supported by the underlying remote is read and written

See the [metadata](/docs/#metadata) docs for more info.

{{< rem autogenerated options stop >}}
