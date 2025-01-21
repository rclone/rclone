---
title: "Cryptomator"
description: "Rclone docs for Cryptomator"
status: Experimental
---

# Cryptomator

Rclone `cryptomator` remotes wrap other remotes containing a
[Cryptomator](https://cryptomator.org)-format vault.

For information on the Cryptomator vault format and how it encrypts files,
please read:
[Cryptomator Architecture](https://docs.cryptomator.org/security/architecture/).

The `cryptomator` remote is **experimental**. It has received only limited
testing against the official Cryptomator clients. Use with caution.

Known issues:

- Rclone cannot yet parse old vault formats (before version 8).
- Rclone ignores symlinks in cryptomator vaults.

## Configuration

Here is an example of how to make a remote called `secret`.

To use `cryptomator`, first set up the underlying remote. Follow the
`rclone config` instructions for the specific backend.

Before configuring the cryptomator remote, check the underlying remote is
working. In this example the underlying remote is called `remote`.
We will configure a path `path` within this remote to contain the
encrypted content. Anything inside `remote:path` will be encrypted
and anything outside will not.

Configure `cryptomator` using `rclone config`. In this example the
`cryptomator` remote is called `secret`, to differentiate it from the
underlying `remote`.

When you are done you can use the cryptomator remote named `secret` just
as you would with any other remote, e.g. `rclone copy D:\docs secret:\docs`,
and rclone will encrypt and decrypt as needed on the fly.
If you access the wrapped remote `remote:path` directly you will bypass
the encryption, and anything you read will be in encrypted form, and
anything you write will be unencrypted. To avoid issues it is best to
configure a dedicated path for encrypted content, and access it
exclusively through a cryptomator remote.

```text
No remotes found, make a new one?
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n

Enter name for new remote.
name> secret

Option Storage.
Type of storage to configure.
Choose a number from below, or type in your own value.
[snip]
XX / Encrypt/Decrypt Cryptomator-format vaults - Tier 4
   \ (cryptomator)
[snip]
Storage> cryptomator

Option remote.
Remote to use as a Cryptomator vault.
Normally should contain a ':' and a path, e.g. "myremote:path/to/dir",
"myremote:bucket" or maybe "myremote:" (not recommended).
Enter a value.
remote> remote:path

Option password.
Password for Cryptomator vault.
Choose an alternative below.
y) Yes, type in my own password
g) Generate random password
y/g> y
Enter the password:
password:
Confirm the password:
password:

Edit advanced config?
y) Yes
n) No (default)
y/n> n

Configuration complete.
Options:
- type: cryptomator
- remote: remote:path
- password: *** ENCRYPTED ***
Keep this "secret" remote?
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d>
```

**Important** The cryptomator password stored in `rclone.conf` is lightly
obscured. That only protects it from cursory inspection. It is not
secure unless [configuration encryption](https://rclone.org/docs/#configuration-encryption)
of `rclone.conf` is specified.

A long passphrase is recommended, or `rclone config` can generate a
random one.

### Specifying the remote

When configuring the remote to encrypt/decrypt, you may specify any
string that rclone accepts as a source/destination of other commands.

The primary use case is to specify the path into an already configured
remote (e.g. `remote:path/to/dir` or `remote:bucket`), such that
data in a remote untrusted location can be stored encrypted.

You may also specify a local filesystem path, such as
`/path/to/dir` on Linux, `C:\path\to\dir` on Windows. By creating
a cryptomator remote pointing to such a local filesystem path, you can
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

If you intend to use the wrapped remote both directly for keeping unencrypted
content, as well as through a cryptomator remote for encrypted content, it is
recommended to point the cryptomator remote to a separate directory within the
wrapped remote. If you use a bucket-based storage system (e.g. Swift, S3,
Google Compute Storage, B2) it is necessary to wrap the cryptomator remote
around a specific bucket (`s3:bucket`). Otherwise, rclone will attempt to
create configuration files in the root of the storage (`s3:`).

### Example

Create the following file structure.

```text
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

```console
$ rclone -q copy plaintext secret:
$ rclone -q ls secret:
        6 file0.txt
        7 file1.txt
        8 subdir/file2.txt
        9 subdir/file3.txt
       10 subdir/subsubdir/file4.txt
```

The cryptomator vault looks like

```console
$ rclone -q ls remote:path
      333 masterkey.cryptomator
      283 vault.cryptomator
      104 d/KE/32SOK74WWKLZYJPR2KDINSPOW6KCF4/1tlc1uDSBOm1WbV83-682WMWkF_CBwzs2Q==.c9r
      132 d/KE/32SOK74WWKLZYJPR2KDINSPOW6KCF4/dirid.c9r
      105 d/KE/32SOK74WWKLZYJPR2KDINSPOW6KCF4/u85LJU0T8u7kour8CmukHpz9bUHc0ykRaw==.c9r
       36 d/KE/32SOK74WWKLZYJPR2KDINSPOW6KCF4/YOv9E2fAfW3X4B9jY6prXszoDZosardIsA==.c9r/dir.c9r
      106 d/M3/GSTDC7WEJHVDKTXFU4IYOCK4JZPL7Q/V87lwDcGAfL6kA0QJf24o_dLiRgjvRdfGQ==.c9r
      132 d/M3/GSTDC7WEJHVDKTXFU4IYOCK4JZPL7Q/dirid.c9r
      103 d/QK/GIR7IOTE5GB3VDRDKRZETC5RHCAXGQ/Q5L14GbiODO_U0GKprmnEe81wx6ZjDeb8g==.c9r
      102 d/QK/GIR7IOTE5GB3VDRDKRZETC5RHCAXGQ/y2mmVT_4X58i3n4C06_nxzotUnxVk8vX2Q==.c9r
       36 d/QK/GIR7IOTE5GB3VDRDKRZETC5RHCAXGQ/M0eewPxxsKq2ObhUJDOUZnnRCgE77g==.c9r/dir.c9r
```

The directory structure is preserved

```console
$ rclone -q ls secret:subdir
        8 file2.txt
        9 file3.txt
       10 subsubdir/file4.txt
```

### Modification times and hashes

Cryptomator stores modification times using the underlying remote so support
depends on that.

Hashes are not stored for cryptomator. However the data integrity is
protected by the cryptography itself.

<!-- autogenerated options start - DO NOT EDIT - instead edit fs.RegInfo in backend/cryptomator/cryptomator.go and run make backenddocs to verify --> <!-- markdownlint-disable-line line-length -->
### Standard options

Here are the Standard options specific to cryptomator (Encrypt/Decrypt Cryptomator-format vaults).

#### --cryptomator-remote

Remote to use as a Cryptomator vault.

Normally should contain a ':' and a path, e.g. "myremote:path/to/dir",
"myremote:bucket" or maybe "myremote:" (not recommended).

Properties:

- Config:      remote
- Env Var:     RCLONE_CRYPTOMATOR_REMOTE
- Type:        string
- Required:    true

#### --cryptomator-password

Password for Cryptomator vault.

**NB** Input to this must be obscured - see [rclone obscure](/commands/rclone_obscure/).

Properties:

- Config:      password
- Env Var:     RCLONE_CRYPTOMATOR_PASSWORD
- Type:        string
- Required:    true

### Advanced options

Here are the Advanced options specific to cryptomator (Encrypt/Decrypt Cryptomator-format vaults).

#### --cryptomator-auto-create

Whether to automatically create a vault if none exists at the underlying remote.

Properties:

- Config:      auto_create
- Env Var:     RCLONE_CRYPTOMATOR_AUTO_CREATE
- Type:        bool
- Default:     false

#### --cryptomator-auto-create-shortening-threshold

If auto create is enabled, the setting to use for minimum filename length that triggers out-of-band storage of the filename.

This vault setting is normally read out of the vault config file on the remote. It can only be configured when the vault is created, because changing it may make the vault unreadable. Therefore this option has no effect if auto create is disabled, or it is enabled and the the vault already exists.

The out-of-band filename storage works by replacing a file whose name is too long with a directory named using the SHA1 hash of the encrypted filename, and storying the original encrypted filename in a text file. For details, read the Cryptomator documentation on [Name Shortening](https://docs.cryptomator.org/security/vault/#name-shortening).

Properties:

- Config:      auto_create_shortening_threshold
- Env Var:     RCLONE_CRYPTOMATOR_AUTO_CREATE_SHORTENING_THRESHOLD
- Type:        int
- Default:     220

#### --cryptomator-auto-create-vault-format-version

If auto create is enabled, the version of the vault format to use.

This vault setting is normally read out of the vault config file on the remote. It can only be configured when the vault is created, because changing it may make the vault unreadable. Therefore this option has no effect if auto create is disabled, or it is enabled and the the vault already exists.

It is not recommended to intentionally choose an old vault format version. This option is mainly useful for testing Rclone itself. Rclone supports vault formats 8 and 7.

Properties:

- Config:      auto_create_vault_format_version
- Env Var:     RCLONE_CRYPTOMATOR_AUTO_CREATE_VAULT_FORMAT_VERSION
- Type:        uint
- Default:     8

#### --cryptomator-description

Description of the remote.

Properties:

- Config:      description
- Env Var:     RCLONE_CRYPTOMATOR_DESCRIPTION
- Type:        string
- Required:    false

### Metadata

Any metadata supported by the underlying remote is read and written.

See the [metadata](/docs/#metadata) docs for more info.

<!-- autogenerated options stop -->

## Backing up an encrypted remote

If you wish to backup an encrypted remote, it is recommended that you use
`rclone sync` on the encrypted files, and make sure the passwords are
the same in the new encrypted remote.

This will have the following advantages

- `rclone sync` will check the checksums while copying
- you can use `rclone check` between the encrypted remotes
- you don't decrypt and encrypt unnecessarily

For example, let's say you have your original remote at `remote:` with
the encrypted version at `eremote:` with path `remote:cryptomator`.  You
would then set up the new remote `remote2:` and then the encrypted
version `eremote2:` with path `remote2:cryptomator` using the same
passwords as `eremote:`.

To sync the two remotes you would do

```console
rclone sync --interactive remote:cryptomator remote2:cryptomator
```

## Limitations of Cryptomator encryption

Cryptomator encrypts, and will detect external modification to:

- file contents
- file names
- the parent of a directory

Cryptomator does not encrypt

- file length
- filename length
- modification time - used for syncing
- how many entries there are in a directory, and whether each one is a file or
  a directory

Cryptomator cannot detect if a file or directory has been copied, moved, or
deleted by someone with access to the underlying storage. However, such an
adversary would have to guess which file to tamper with from the above
unencrypted attributes.
