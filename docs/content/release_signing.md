---
title: "Release Signing"
description: "How the release is signed and how to check the signature."
---

# Release signing

The hashes of the binary artefacts of the rclone release are signed
with a public PGP/GPG key. This can be verified manually as described
below.

The same mechanism is also used by [rclone selfupdate](/commands/rclone_selfupdate/)
to verify that the release has not been tampered with before the new
update is installed. This checks the SHA256 hash and the signature
with a public key compiled into the rclone binary.

## Release signing key

You may obtain the release signing key from:

- From [KEYS](/KEYS) on this website - this file contains all past signing keys also.
- The git repository hosted on GitHub - https://github.com/rclone/rclone/blob/master/docs/content/KEYS
- `gpg --keyserver hkps://keys.openpgp.org --search nick@craig-wood.com`
- `gpg --keyserver hkps://keyserver.ubuntu.com --search nick@craig-wood.com`
- https://www.craig-wood.com/nick/pub/pgp-key.txt

After importing the key, verify that the fingerprint of one of the
keys matches: `FBF737ECE9F8AB18604BD2AC93935E02FF3B54FA` as this key is used for signing.

We recommend that you cross-check the fingerprint shown above through
the domains listed below. By cross-checking the integrity of the
fingerprint across multiple domains you can be confident that you
obtained the correct key.

- The [source for this page on GitHub](https://github.com/rclone/rclone/blob/master/docs/content/release_signing.md).
- Through DNS `dig key.rclone.org txt`

If you find anything that doesn't not match, please contact the
developers at once.

## How to verify the release

In the release directory you will see the release files and some files called `MD5SUMS`, `SHA1SUMS` and `SHA256SUMS`.

```
$ rclone lsf --http-url https://downloads.rclone.org/v1.63.1 :http:
MD5SUMS
SHA1SUMS
SHA256SUMS
rclone-v1.63.1-freebsd-386.zip
rclone-v1.63.1-freebsd-amd64.zip
...
rclone-v1.63.1-windows-arm64.zip
rclone-v1.63.1.tar.gz
version.txt
```

The `MD5SUMS`, `SHA1SUMS` and `SHA256SUMS` contain hashes of the
binary files in the release directory along with a signature.

For example:

```
$ rclone cat --http-url https://downloads.rclone.org/v1.63.1 :http:SHA256SUMS
-----BEGIN PGP SIGNED MESSAGE-----
Hash: SHA1

f6d1b2d7477475ce681bdce8cb56f7870f174cb6b2a9ac5d7b3764296ea4a113  rclone-v1.63.1-freebsd-386.zip
7266febec1f01a25d6575de51c44ddf749071a4950a6384e4164954dff7ac37e  rclone-v1.63.1-freebsd-amd64.zip
...
66ca083757fb22198309b73879831ed2b42309892394bf193ff95c75dff69c73  rclone-v1.63.1-windows-amd64.zip
bbb47c16882b6c5f2e8c1b04229378e28f68734c613321ef0ea2263760f74cd0  rclone-v1.63.1-windows-arm64.zip
-----BEGIN PGP SIGNATURE-----

iF0EARECAB0WIQT79zfs6firGGBL0qyTk14C/ztU+gUCZLVKJQAKCRCTk14C/ztU
+pZuAJ0XJ+QWLP/3jCtkmgcgc4KAwd/rrwCcCRZQ7E+oye1FPY46HOVzCFU3L7g=
=8qrL
-----END PGP SIGNATURE-----
```

### Download the files

The first step is to download the binary and SUMs file and verify that
the SUMs you have downloaded match. Here we download
`rclone-v1.63.1-windows-amd64.zip` - choose the binary (or binaries)
appropriate to your architecture. We've also chosen the `SHA256SUMS`
as these are the most secure. You could verify the other types of hash
also for extra security. `rclone selfupdate` verifies just the
`SHA256SUMS`.

```
$ mkdir /tmp/check
$ cd /tmp/check
$ rclone copy --http-url https://downloads.rclone.org/v1.63.1 :http:SHA256SUMS .
$ rclone copy --http-url https://downloads.rclone.org/v1.63.1 :http:rclone-v1.63.1-windows-amd64.zip .
```

### Verify the signatures

First verify the signatures on the SHA256 file.

Import the key. See above for ways to verify this key is correct.

```
$ gpg --keyserver keyserver.ubuntu.com --receive-keys FBF737ECE9F8AB18604BD2AC93935E02FF3B54FA
gpg: key 93935E02FF3B54FA: public key "Nick Craig-Wood <nick@craig-wood.com>" imported
gpg: Total number processed: 1
gpg:               imported: 1
```

Then check the signature:

```
$ gpg --verify SHA256SUMS 
gpg: Signature made Mon 17 Jul 2023 15:03:17 BST
gpg:                using DSA key FBF737ECE9F8AB18604BD2AC93935E02FF3B54FA
gpg: Good signature from "Nick Craig-Wood <nick@craig-wood.com>" [ultimate]
```

Verify the signature was good and is using the fingerprint shown above.

Repeat for `MD5SUMS` and `SHA1SUMS` if desired.

### Verify the hashes

Now that we know the signatures on the hashes are OK we can verify the
binaries match the hashes, completing the verification.

```
$ sha256sum -c SHA256SUMS 2>&1 | grep OK
rclone-v1.63.1-windows-amd64.zip: OK
```

Or do the check with rclone

```
$ rclone hashsum sha256 -C SHA256SUMS rclone-v1.63.1-windows-amd64.zip 
2023/09/11 10:53:58 NOTICE: SHA256SUMS: improperly formatted checksum line 0
2023/09/11 10:53:58 NOTICE: SHA256SUMS: improperly formatted checksum line 1
2023/09/11 10:53:58 NOTICE: SHA256SUMS: improperly formatted checksum line 49
2023/09/11 10:53:58 NOTICE: SHA256SUMS: 4 warning(s) suppressed...
= rclone-v1.63.1-windows-amd64.zip
2023/09/11 10:53:58 NOTICE: Local file system at /tmp/check: 0 differences found
2023/09/11 10:53:58 NOTICE: Local file system at /tmp/check: 1 matching files
```

### Verify signatures and hashes together

You can verify the signatures and hashes in one command line like this:

```
$ gpg --decrypt SHA256SUMS | sha256sum -c --ignore-missing
gpg: Signature made Mon 17 Jul 2023 15:03:17 BST
gpg:                using DSA key FBF737ECE9F8AB18604BD2AC93935E02FF3B54FA
gpg: Good signature from "Nick Craig-Wood <nick@craig-wood.com>" [ultimate]
gpg:                 aka "Nick Craig-Wood <nick@memset.com>" [unknown]
rclone-v1.63.1-windows-amd64.zip: OK
```
